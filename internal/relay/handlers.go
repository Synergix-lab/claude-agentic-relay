package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-relay/internal/db"
	"agent-relay/internal/ingest"
	"agent-relay/internal/models"
	"agent-relay/internal/vault"

	"github.com/mark3labs/mcp-go/mcp"
)

type Handlers struct {
	db           *db.DB
	registry     *SessionRegistry
	ingester     *ingest.Ingester
	vaultWatcher *vault.Watcher
	events       *EventBus
}

func NewHandlers(db *db.DB, registry *SessionRegistry, ingester *ingest.Ingester, vaultWatcher *vault.Watcher, events *EventBus) *Handlers {
	return &Handlers{db: db, registry: registry, ingester: ingester, vaultWatcher: vaultWatcher, events: events}
}

// HandleWhoami finds the caller's Claude Code session by grepping transcripts for a unique salt.
func (h *Handlers) HandleWhoami(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	salt := req.GetString("salt", "")
	if salt == "" {
		return mcp.NewToolResultError("salt is required"), nil
	}
	if len(salt) < 5 {
		return mcp.NewToolResultError("salt too short — use at least 3 random words"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return mcp.NewToolResultError("cannot determine home dir"), nil
	}

	claudeDir := filepath.Join(home, ".claude", "projects")
	var matchFile string

	// Walk all .jsonl transcript files
	_ = filepath.Walk(claudeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if matchFile != "" {
			return filepath.SkipAll
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		// Scan from the end — salt is in recent lines. Read last 64KB.
		stat, _ := f.Stat()
		offset := stat.Size() - 65536
		if offset < 0 {
			offset = 0
		}
		f.Seek(offset, 0)

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), salt) {
				matchFile = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	if matchFile == "" {
		return mcp.NewToolResultError("salt not found in any transcript — make sure you wrote the salt in your conversation before calling whoami"), nil
	}

	// Extract session ID from filename (UUID.jsonl)
	base := filepath.Base(matchFile)
	sessionID := strings.TrimSuffix(base, ".jsonl")

	return resultJSON(map[string]any{
		"session_id":      sessionID,
		"transcript_path": matchFile,
	})
}

func (h *Handlers) HandleRegisterAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	name := strings.ToLower(req.GetString("name", ""))
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	role := req.GetString("role", "")
	description := req.GetString("description", "")
	reportsTo := optionalStringLower(req.GetString("reports_to", ""))
	profileSlug := optionalString(req.GetString("profile_slug", ""))
	isExecutive := req.GetBool("is_executive", false)
	sessionID := optionalString(req.GetString("session_id", ""))
	interestTags := req.GetString("interest_tags", "[]")
	maxContextBytes := req.GetInt("max_context_bytes", 16384)

	agent, isRespawn, err := h.db.RegisterAgent(project, name, role, description, reportsTo, profileSlug, isExecutive, sessionID, interestTags, maxContextBytes)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to register agent: %v", err)), nil
	}

	// Auto-create admin team + add executive agent (fixes broadcast permission UX)
	var autoAdminTeam *string
	if isExecutive {
		adminTeam, _ := h.db.GetTeam(project, "leadership")
		if adminTeam == nil {
			adminTeam, _ = h.db.CreateTeam("Leadership", "leadership", project, "Auto-created admin team for executive agents", "admin", nil, nil)
		}
		if adminTeam != nil {
			_ = h.db.AddTeamMember(adminTeam.ID, name, project, "admin")
			autoAdminTeam = &adminTeam.Slug
		}
	}

	// Register the session for push notifications
	if sess := sessionFromContext(ctx); sess != nil {
		h.registry.Register(project, name, sess.SessionID())
	}

	// Build session_context for the response (Phase 2: boot-in-register)
	sessionCtx := h.buildSessionContext(project, name, profileSlug)
	sessionCtx["is_respawn"] = isRespawn

	action := "register"
	if isRespawn {
		action = "respawn"
	}
	h.events.Emit(MCPEvent{Type: "register", Action: action, Agent: name, Project: project, Label: role})

	resp := map[string]any{
		"agent":           agent,
		"session_context": sessionCtx,
	}
	if autoAdminTeam != nil {
		resp["auto_admin_team"] = *autoAdminTeam
		resp["hint"] = "You were auto-added to the 'leadership' admin team (broadcast enabled). Use send_message(to='*') to broadcast."
	}
	return resultJSON(resp)
}

func (h *Handlers) HandleSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	from := resolveAgent(ctx, req)
	to := strings.ToLower(req.GetString("to", ""))
	msgType := req.GetString("type", "notification")
	subject := req.GetString("subject", "")
	content := req.GetString("content", "")
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}

	metadata := req.GetString("metadata", "{}")
	replyTo := optionalString(req.GetString("reply_to", ""))
	conversationID := optionalString(req.GetString("conversation_id", ""))
	priority := mapPriority(req.GetString("priority", "P2"))
	ttlSeconds := req.GetInt("ttl_seconds", 3600)

	// Support "to": "conversation:<id>" shorthand
	if conversationID == nil && strings.HasPrefix(to, "conversation:") {
		cid := strings.TrimPrefix(to, "conversation:")
		conversationID = &cid
	}

	// Touch sender's last_seen
	_ = h.db.TouchAgent(project, from)

	if conversationID != nil {
		// Conversation message — validate membership
		isMember, err := h.db.IsConversationMember(*conversationID, from)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to check membership: %v", err)), nil
		}
		if !isMember {
			return mcp.NewToolResultError("you are not a member of this conversation"), nil
		}
		to = "" // no single recipient for conversation messages
	} else if to == "" {
		return mcp.NewToolResultError("to is required (or provide conversation_id)"), nil
	}

	// Permission check: only enforce when teams are configured (bypass for "user" — always reachable)
	if conversationID == nil && to != "*" && to != "user" && !strings.HasPrefix(to, "team:") {
		hasTeams, _ := h.db.HasTeams(project)
		if hasTeams {
			allowed, err := h.db.CanMessage(project, from, to)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("permission check failed: %v", err)), nil
			}
			if !allowed {
				return mcp.NewToolResultError(fmt.Sprintf("not authorized to message '%s' — no shared team, reports_to chain, or notify channel", to)), nil
			}
		}
	}

	// Team addressing: to="team:slug" → fan out to team members + team_inbox
	if strings.HasPrefix(to, "team:") {
		teamSlug := strings.TrimPrefix(to, "team:")
		team, err := h.db.ResolveTeamSlug(project, teamSlug)
		if err != nil || team == nil {
			return mcp.NewToolResultError(fmt.Sprintf("team '%s' not found", teamSlug)), nil
		}

		msg, err := h.db.InsertMessage(project, from, to, msgType, subject, content, metadata, priority, ttlSeconds, replyTo, conversationID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
		}

		// Add to team inbox
		_ = h.db.AddToTeamInbox(team.ID, msg.ID)

		// Create deliveries for team members
		members, _ := h.db.GetTeamMemberNames(team.ID)
		var recipients []string
		for _, member := range members {
			if member != from {
				recipients = append(recipients, member)
				h.registry.Notify(project, member, from, subject, msg.ID)
			}
		}
		_ = h.db.CreateDeliveries(msg.ID, project, recipients)

		return resultJSON(msg)
	}

	// Broadcast permission: when teams exist, only admin team members can broadcast
	if to == "*" {
		hasTeams, _ := h.db.HasTeams(project)
		if hasTeams {
			allowed, _ := h.db.CanMessage(project, from, "*")
			if !allowed {
				return mcp.NewToolResultError("broadcast requires membership in an 'admin' type team. Fix: register with is_executive=true (auto-creates admin team), or manually: create_team(type='admin') then add_team_member()"), nil
			}
		}
	}

	msg, err := h.db.InsertMessage(project, from, to, msgType, subject, content, metadata, priority, ttlSeconds, replyTo, conversationID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	// Create deliveries
	recipients, _ := h.db.ResolveRecipients(project, to, from, conversationID)
	_ = h.db.CreateDeliveries(msg.ID, project, recipients)

	// Push notification
	if conversationID != nil {
		h.notifyConversation(project, *conversationID, from, subject, msg.ID)
	} else if to == "*" {
		h.registry.NotifyBroadcast(project, from, subject, msg.ID)
	} else {
		h.registry.Notify(project, to, from, subject, msg.ID)
	}

	return resultJSON(msg)
}

func (h *Handlers) HandleGetInbox(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	unreadOnly := req.GetBool("unread_only", true)
	limit := req.GetInt("limit", 10)
	fullContent := req.GetBool("full_content", false)
	budgetMode := req.GetBool("apply_budget", false)

	_ = h.db.TouchAgent(project, agent)

	// Expire stale messages before querying
	h.db.ExpireMessages()

	messages, err := h.db.GetInbox(project, agent, unreadOnly, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get inbox: %v", err)), nil
	}
	if messages == nil {
		messages = []models.Message{}
	}

	// Apply context budget pruning if requested
	if budgetMode && len(messages) > 0 {
		agentObj, _ := h.db.GetAgent(project, agent)
		if agentObj != nil {
			var tags []string
			json.Unmarshal([]byte(agentObj.InterestTags), &tags)
			messages = applyBudget(messages, tags, agentObj.MaxContextBytes)
		}
	}

	formatted := make([]map[string]any, len(messages))
	for i, m := range messages {
		content := m.Content
		if !fullContent && len(content) > 300 {
			content = content[:300] + "..."
		}
		entry := map[string]any{
			"id":         m.ID,
			"from":       m.From,
			"to":         m.To,
			"type":       m.Type,
			"subject":    m.Subject,
			"content":    content,
			"created_at": m.CreatedAt,
			"priority":   m.Priority,
		}
		if m.ReplyTo != nil {
			entry["reply_to"] = *m.ReplyTo
		}
		if m.ConversationID != nil {
			entry["conversation_id"] = *m.ConversationID
		}
		if m.DeliveryID != nil {
			entry["delivery_id"] = *m.DeliveryID
		}
		if m.DeliveryState != nil {
			entry["delivery_state"] = *m.DeliveryState
		}
		formatted[i] = entry
	}

	return resultJSON(map[string]any{
		"agent":    agent,
		"count":    len(messages),
		"messages": formatted,
	})
}

func (h *Handlers) HandleAckDelivery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	deliveryID := req.GetString("delivery_id", "")
	if deliveryID == "" {
		return mcp.NewToolResultError("delivery_id is required"), nil
	}
	if err := h.db.AcknowledgeDelivery(deliveryID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to acknowledge delivery: %v", err)), nil
	}
	return resultJSON(map[string]any{"acknowledged": deliveryID})
}

func (h *Handlers) HandleGetThread(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID := req.GetString("message_id", "")
	if messageID == "" {
		return mcp.NewToolResultError("message_id is required"), nil
	}

	messages, err := h.db.GetThread(messageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get thread: %v", err)), nil
	}
	if messages == nil {
		messages = []models.Message{}
	}

	return resultJSON(map[string]any{
		"count":    len(messages),
		"messages": messages,
	})
}

func (h *Handlers) HandleListAgents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)

	agents, err := h.db.ListAgents(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list agents: %v", err)), nil
	}
	if agents == nil {
		agents = []models.Agent{}
	}

	// Enrich with live activity from ingester
	var sessions []ingest.SessionState
	if h.ingester != nil {
		sessions = h.ingester.GetSessions()
	}
	sessionByID := make(map[string]ingest.SessionState)
	for _, s := range sessions {
		sessionByID[s.SessionID] = s
	}

	type agentWithActivity struct {
		models.Agent
		Activity     string `json:"activity,omitempty"`
		ActivityTool string `json:"activity_tool,omitempty"`
	}

	result := make([]agentWithActivity, 0, len(agents))
	for _, a := range agents {
		aa := agentWithActivity{Agent: a}
		if a.SessionID != nil {
			if s, ok := sessionByID[*a.SessionID]; ok {
				aa.Activity = string(s.Activity)
				aa.ActivityTool = s.Tool
			}
		}
		result = append(result, aa)
	}

	return resultJSON(map[string]any{
		"count":  len(result),
		"agents": result,
	})
}

func (h *Handlers) HandleMarkRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)

	// Support marking a whole conversation as read
	convID := req.GetString("conversation_id", "")
	if convID != "" {
		if err := h.db.MarkConversationRead(convID, agent); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to mark conversation read: %v", err)), nil
		}
		return resultJSON(map[string]any{
			"conversation_id": convID,
			"marked_read":     true,
		})
	}

	ids := req.GetStringSlice("message_ids", nil)
	if len(ids) == 0 {
		return mcp.NewToolResultError("message_ids or conversation_id is required"), nil
	}

	count, err := h.db.MarkRead(ids, agent, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to mark read: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"marked_read": count,
	})
}

func (h *Handlers) HandleCreateConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	title := req.GetString("title", "")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	rawMembers := req.GetStringSlice("members", nil)
	if len(rawMembers) == 0 {
		return mcp.NewToolResultError("at least one other member is required"), nil
	}
	members := make([]string, len(rawMembers))
	for i, m := range rawMembers {
		members[i] = strings.ToLower(m)
	}

	// Ensure creator is included in members
	found := false
	for _, m := range members {
		if m == agent {
			found = true
			break
		}
	}
	if !found {
		members = append([]string{agent}, members...)
	}

	conv, err := h.db.CreateConversation(project, title, agent, members)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create conversation: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"conversation": conv,
		"members":      members,
	})
}

func (h *Handlers) HandleListConversations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)

	convs, err := h.db.ListConversations(project, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list conversations: %v", err)), nil
	}
	if convs == nil {
		convs = []models.ConversationSummary{}
	}

	return resultJSON(map[string]any{
		"agent":         agent,
		"count":         len(convs),
		"conversations": convs,
	})
}

func (h *Handlers) HandleGetConversationMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	convID := req.GetString("conversation_id", "")
	if convID == "" {
		return mcp.NewToolResultError("conversation_id is required"), nil
	}
	limit := req.GetInt("limit", 50)

	// Verify membership
	isMember, err := h.db.IsConversationMember(convID, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to check membership: %v", err)), nil
	}
	if !isMember {
		return mcp.NewToolResultError("you are not a member of this conversation"), nil
	}

	messages, err := h.db.GetConversationMessages(convID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get messages: %v", err)), nil
	}
	if messages == nil {
		messages = []models.Message{}
	}

	// Auto-mark conversation as read when fetching messages
	_ = h.db.MarkConversationRead(convID, agent)

	format := req.GetString("format", "full")

	formatted := make([]map[string]any, len(messages))
	for i, m := range messages {
		entry := map[string]any{
			"id":         m.ID,
			"from":       m.From,
			"type":       m.Type,
			"subject":    m.Subject,
			"created_at": m.CreatedAt,
		}
		if m.ReplyTo != nil {
			entry["reply_to"] = *m.ReplyTo
		}
		switch format {
		case "compact":
			// metadata only — no content
		case "digest":
			c := m.Content
			if len(c) > 200 {
				c = c[:200] + "..."
			}
			entry["content"] = c
		default: // "full"
			entry["content"] = m.Content
			entry["metadata"] = m.Metadata
		}
		formatted[i] = entry
	}

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"count":           len(formatted),
		"format":          format,
		"messages":        formatted,
	})
}

func (h *Handlers) HandleInviteToConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	convID := req.GetString("conversation_id", "")
	if convID == "" {
		return mcp.NewToolResultError("conversation_id is required"), nil
	}
	invitee := strings.ToLower(req.GetString("agent_name", ""))
	if invitee == "" {
		return mcp.NewToolResultError("agent_name is required"), nil
	}

	// Verify inviter is a member
	isMember, err := h.db.IsConversationMember(convID, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to check membership: %v", err)), nil
	}
	if !isMember {
		return mcp.NewToolResultError("you are not a member of this conversation"), nil
	}

	if err := h.db.AddConversationMember(convID, invitee); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to invite: %v", err)), nil
	}

	// Notify the invitee
	h.registry.Notify(project, invitee, agent, fmt.Sprintf("You were invited to conversation: %s", convID), "")

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"invited":         invitee,
	})
}

func (h *Handlers) HandleLeaveConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	convID := req.GetString("conversation_id", "")
	if convID == "" {
		return mcp.NewToolResultError("conversation_id is required"), nil
	}

	isMember, err := h.db.IsConversationMember(convID, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to check membership: %v", err)), nil
	}
	if !isMember {
		return mcp.NewToolResultError("you are not a member of this conversation"), nil
	}

	if err := h.db.LeaveConversation(convID, agent); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to leave: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "conversation",
		Action:  "left",
		Agent:   agent,
		Project: project,
		Label:   convID,
	})

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"left":            agent,
	})
}

func (h *Handlers) HandleArchiveConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	convID := req.GetString("conversation_id", "")
	if convID == "" {
		return mcp.NewToolResultError("conversation_id is required"), nil
	}

	if err := h.db.ArchiveConversation(convID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to archive: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "conversation",
		Action:  "archived",
		Agent:   resolveAgent(ctx, req),
		Project: project,
		Label:   convID,
	})

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"archived":        true,
	})
}

func (h *Handlers) notifyConversation(project, conversationID, senderName, subject, messageID string) {
	members, err := h.db.GetConversationMembers(conversationID)
	if err != nil {
		return
	}
	for _, m := range members {
		if m.AgentName != senderName {
			h.registry.Notify(project, m.AgentName, senderName, subject, messageID)
		}
	}
}

// resolveProject returns the project from the explicit `project` tool parameter,
// falling back to the HTTP context default (from ?project= URL param).
func resolveProject(ctx context.Context, req mcp.CallToolRequest) string {
	if p := req.GetString("project", ""); p != "" {
		return p
	}
	return ProjectFromContext(ctx)
}

// resolveAgent returns the agent name from the explicit `as` tool parameter,
// falling back to the HTTP context default (from ?agent= URL param).
// Names are lowercased for case-insensitive matching.
func resolveAgent(ctx context.Context, req mcp.CallToolRequest) string {
	if as := req.GetString("as", ""); as != "" {
		return strings.ToLower(as)
	}
	return AgentFromContext(ctx)
}

// helpers

func resultJSON(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalStringLower(s string) *string {
	if s == "" {
		return nil
	}
	l := strings.ToLower(s)
	return &l
}

// normalizeJSONArrayParam handles profile parameters that can be either a JSON string
// (e.g. "[\"a\",\"b\"]") or a native JSON array from the MCP client. Returns a JSON string.
func normalizeJSONArrayParam(req mcp.CallToolRequest, key string) string {
	// First try as string (the documented format)
	if s := req.GetString(key, ""); s != "" {
		// Validate it's valid JSON
		var check json.RawMessage
		if json.Unmarshal([]byte(s), &check) == nil {
			return s
		}
		// Not valid JSON — wrap as a single-element array
		b, _ := json.Marshal([]string{s})
		return string(b)
	}
	// Try to extract the raw argument value — it might be a native array
	if args := req.GetArguments(); args != nil {
		if raw, exists := args[key]; exists {
			// Re-marshal whatever the MCP client sent (array, object, etc.)
			b, err := json.Marshal(raw)
			if err == nil {
				return string(b)
			}
		}
	}
	return "[]"
}

// mapPriority normalizes MACP aliases to P0-P3.
func mapPriority(p string) string {
	switch strings.ToLower(p) {
	case "interrupt", "p0":
		return "P0"
	case "steering", "p1":
		return "P1"
	case "advisory", "p2", "":
		return "P2"
	case "info", "p3":
		return "P3"
	default:
		return "P2"
	}
}

func sessionFromContext(ctx context.Context) clientSession {
	if sess, ok := ctx.Value(sessionKey).(clientSession); ok {
		return sess
	}
	return nil
}

type clientSession interface {
	SessionID() string
}

const sessionKey contextKey = "mcp_session"

// --- Memory handlers ---

func (h *Handlers) HandleSetMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	key := req.GetString("key", "")
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}
	value := req.GetString("value", "")
	if value == "" {
		return mcp.NewToolResultError("value is required"), nil
	}
	scope := req.GetString("scope", "project")
	confidence := req.GetString("confidence", "stated")
	layer := req.GetString("layer", "behavior")
	tags := req.GetStringSlice("tags", nil)
	tagsJSON := db.TagsToJSON(tags)

	mem, err := h.db.SetMemory(project, agent, key, value, tagsJSON, scope, confidence, layer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set memory: %v", err)), nil
	}

	result := map[string]any{
		"memory": mem,
	}
	action := "set"
	if mem.ConflictWith != nil {
		result["conflict"] = true
		result["message"] = fmt.Sprintf("Conflict detected: key '%s' already exists with a different value. Both versions preserved. Use resolve_conflict to pick the truth.", key)
		action = "conflict"
	}
	h.events.Emit(MCPEvent{Type: "memory", Action: action, Agent: agent, Project: project, Label: key})

	return resultJSON(result)
}

func (h *Handlers) HandleGetMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	key := req.GetString("key", "")
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}
	scope := req.GetString("scope", "")

	memories, err := h.db.GetMemory(project, agent, key, scope)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get memory: %v", err)), nil
	}
	if memories == nil {
		memories = []models.Memory{}
	}

	result := map[string]any{
		"key":      key,
		"count":    len(memories),
		"memories": memories,
	}
	if len(memories) > 1 {
		result["conflict"] = true
		result["message"] = "Multiple values exist for this key. Use resolve_conflict to pick the truth."
	}

	return resultJSON(result)
}

func (h *Handlers) HandleSearchMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	query := req.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	scope := req.GetString("scope", "")
	tags := req.GetStringSlice("tags", nil)
	limit := req.GetInt("limit", 20)

	memories, err := h.db.SearchMemory(project, agent, query, tags, scope, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search memories: %v", err)), nil
	}
	if memories == nil {
		memories = []models.Memory{}
	}

	// Truncate values for compact response
	truncated := make([]map[string]any, len(memories))
	for i, m := range memories {
		val := m.Value
		if len(val) > 300 {
			val = val[:300] + "..."
		}
		truncated[i] = map[string]any{
			"id":         m.ID,
			"key":        m.Key,
			"value":      val,
			"tags":       m.Tags,
			"scope":      m.Scope,
			"agent_name": m.AgentName,
			"confidence": m.Confidence,
			"version":    m.Version,
			"updated_at": m.UpdatedAt,
			"conflict":   m.ConflictWith != nil,
		}
	}

	return resultJSON(map[string]any{
		"query":    query,
		"count":    len(truncated),
		"memories": truncated,
	})
}

func (h *Handlers) HandleListMemories(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	scope := req.GetString("scope", "")
	agentFilter := req.GetString("agent", "")
	tags := req.GetStringSlice("tags", nil)
	limit := req.GetInt("limit", 50)

	// Bug fix: scope=agent must be filtered by the calling agent to prevent leaking
	// other agents' private memories. If no explicit agent filter, use the caller's identity.
	if scope == "agent" && agentFilter == "" {
		agentFilter = resolveAgent(ctx, req)
	}

	memories, err := h.db.ListMemories(project, scope, agentFilter, tags, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list memories: %v", err)), nil
	}
	if memories == nil {
		memories = []models.Memory{}
	}

	// Truncate values for compact response
	truncated := make([]map[string]any, len(memories))
	for i, m := range memories {
		val := m.Value
		if len(val) > 200 {
			val = val[:200] + "..."
		}
		truncated[i] = map[string]any{
			"id":         m.ID,
			"key":        m.Key,
			"value":      val,
			"tags":       m.Tags,
			"scope":      m.Scope,
			"project":    m.Project,
			"agent_name": m.AgentName,
			"confidence": m.Confidence,
			"version":    m.Version,
			"updated_at": m.UpdatedAt,
			"conflict":   m.ConflictWith != nil,
		}
	}

	return resultJSON(map[string]any{
		"count":    len(truncated),
		"memories": truncated,
	})
}

func (h *Handlers) HandleDeleteMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	key := req.GetString("key", "")
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}
	scope := req.GetString("scope", "project")

	if err := h.db.DeleteMemory(project, agent, key, scope); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete memory: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"deleted": true,
		"key":     key,
		"scope":   scope,
	})
}

func (h *Handlers) HandleResolveConflict(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	key := req.GetString("key", "")
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}
	chosenValue := req.GetString("chosen_value", "")
	if chosenValue == "" {
		return mcp.NewToolResultError("chosen_value is required"), nil
	}
	scope := req.GetString("scope", "project")

	winner, err := h.db.ResolveConflict(project, agent, key, chosenValue, scope)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve conflict: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "memory", Action: "resolve", Agent: agent, Project: project, Label: key})

	return resultJSON(map[string]any{
		"resolved": true,
		"memory":   winner,
	})
}

// --- Profile handlers ---

func (h *Handlers) HandleRegisterProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	slug := req.GetString("slug", "")
	if slug == "" {
		return mcp.NewToolResultError("slug is required"), nil
	}
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	role := req.GetString("role", "")
	contextPack := req.GetString("context_pack", "")
	soulKeys := normalizeJSONArrayParam(req, "soul_keys")
	skills := normalizeJSONArrayParam(req, "skills")
	vaultPaths := normalizeJSONArrayParam(req, "vault_paths")

	profile, err := h.db.RegisterProfile(project, slug, name, role, contextPack, soulKeys, skills, vaultPaths)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to register profile: %v", err)), nil
	}
	return resultJSON(profile)
}

func (h *Handlers) HandleGetProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	slug := req.GetString("slug", "")
	if slug == "" {
		return mcp.NewToolResultError("slug is required"), nil
	}

	profile, err := h.db.GetProfile(project, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get profile: %v", err)), nil
	}
	if profile == nil {
		return mcp.NewToolResultError(fmt.Sprintf("profile not found: %s", slug)), nil
	}
	return resultJSON(profile)
}

func (h *Handlers) HandleListProfiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)

	profiles, err := h.db.ListProfiles(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list profiles: %v", err)), nil
	}
	if profiles == nil {
		profiles = []models.Profile{}
	}

	return resultJSON(map[string]any{
		"count":    len(profiles),
		"profiles": profiles,
	})
}

// --- Task handlers ---

func (h *Handlers) HandleDispatchTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	profile := req.GetString("profile", "")
	if profile == "" {
		return mcp.NewToolResultError("profile is required"), nil
	}
	title := req.GetString("title", "")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}
	description := req.GetString("description", "")
	priority := req.GetString("priority", "P2")
	parentTaskID := optionalString(req.GetString("parent_task_id", ""))
	boardID := optionalString(req.GetString("board_id", ""))
	goalID := optionalString(req.GetString("goal_id", ""))

	// Auto-create "human" profile if dispatching to it for the first time
	if profile == "human" {
		existing, _ := h.db.GetProfile(project, "human")
		if existing == nil {
			_, _ = h.db.RegisterProfile(project, "human", "Human Operator",
				"Tasks that require human action (API keys, approvals, purchases, manual config)",
				"You are the human operator. Complete these tasks outside the relay.",
				"[]", "[]", "[]")
		}
	}

	// Auto-create a default "backlog" board if none specified and none exist
	var autoBoard *models.Board
	if boardID == nil {
		boards, _ := h.db.ListBoards(project)
		if len(boards) == 0 {
			autoBoard, _ = h.db.CreateBoard(project, "Backlog", "backlog", "Auto-created default board", agent)
			if autoBoard != nil {
				boardID = &autoBoard.ID
			}
		} else {
			// Use the first existing board as default
			boardID = &boards[0].ID
		}
	}

	task, err := h.db.DispatchTask(project, profile, agent, title, description, priority, parentTaskID, boardID, goalID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to dispatch task: %v", err)), nil
	}

	// Push notification for P0/P1 tasks
	if priority == "P0" || priority == "P1" {
		h.registry.NotifyProfile(project, profile, agent, fmt.Sprintf("[%s] %s", priority, title), task.ID)
	}

	h.events.Emit(MCPEvent{Type: "task", Action: "dispatch", Agent: agent, Project: project, Target: profile, Label: title})

	resp := map[string]any{"task": task}
	if autoBoard != nil {
		resp["auto_board"] = autoBoard
		resp["hint"] = fmt.Sprintf("Auto-created 'backlog' board (id: %s) since no boards existed.", autoBoard.ID)
	}

	// Dedup warning: check for similar active tasks on same profile
	similar, _ := h.db.FindSimilarTasks(project, profile, title)
	if len(similar) > 0 {
		// Filter out the task we just created
		var dupes []map[string]string
		for _, s := range similar {
			if s.ID != task.ID {
				dupes = append(dupes, map[string]string{"id": s.ID, "title": s.Title, "status": s.Status})
			}
		}
		if len(dupes) > 0 {
			resp["warning"] = fmt.Sprintf("Found %d similar active task(s) on profile '%s'", len(dupes), profile)
			resp["similar"] = dupes
		}
	}

	return resultJSON(resp)
}

func (h *Handlers) HandleClaimTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.db.ClaimTask(taskID, agent, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to claim task: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "task", Action: "claim", Agent: agent, Project: project, Label: task.Title})
	return resultJSON(task)
}

func (h *Handlers) HandleStartTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.db.StartTask(taskID, agent, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start task: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "task", Action: "start", Agent: agent, Project: project, Label: task.Title})
	return resultJSON(task)
}

func (h *Handlers) HandleCompleteTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}
	result := optionalString(req.GetString("result", ""))

	task, err := h.db.CompleteTask(taskID, agent, project, result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to complete task: %v", err)), nil
	}

	h.events.Emit(MCPEvent{Type: "task", Action: "complete", Agent: agent, Project: project, Target: task.DispatchedBy, Label: task.Title})

	// Notify dispatcher
	h.registry.Notify(project, task.DispatchedBy, agent, fmt.Sprintf("Task done: %s", task.Title), task.ID)

	// If this task has a parent, check if all sibling subtasks are now complete
	if task.ParentTaskID != nil {
		allDone, total, doneCount := h.db.CheckSubtasksComplete(*task.ParentTaskID, project)
		if allDone {
			parent, _ := h.db.GetTask(*task.ParentTaskID, project)
			if parent != nil {
				h.registry.Notify(project, parent.DispatchedBy, agent,
					fmt.Sprintf("All %d subtasks complete for: %s", total, parent.Title), parent.ID)
				// Also notify the assigned agent on the parent task
				if parent.AssignedTo != nil && *parent.AssignedTo != parent.DispatchedBy {
					h.registry.Notify(project, *parent.AssignedTo, agent,
						fmt.Sprintf("All %d subtasks complete for your task: %s", total, parent.Title), parent.ID)
				}
			}
		} else {
			// Partial progress notification to parent dispatcher
			parent, _ := h.db.GetTask(*task.ParentTaskID, project)
			if parent != nil {
				h.registry.Notify(project, parent.DispatchedBy, agent,
					fmt.Sprintf("Subtask done (%d/%d): %s → %s", doneCount, total, task.Title, parent.Title), parent.ID)
			}
		}
	}

	return resultJSON(task)
}

func (h *Handlers) HandleBlockTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}
	reason := optionalString(req.GetString("reason", ""))

	task, err := h.db.BlockTask(taskID, agent, project, reason)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to block task: %v", err)), nil
	}

	h.events.Emit(MCPEvent{Type: "task", Action: "block", Agent: agent, Project: project, Target: task.DispatchedBy, Label: task.Title})

	// Notify dispatcher — blocked is critical
	reasonStr := ""
	if reason != nil {
		reasonStr = ": " + *reason
	}
	h.registry.Notify(project, task.DispatchedBy, agent, fmt.Sprintf("BLOCKED: %s%s", task.Title, reasonStr), task.ID)

	// Phase 4: Bubble notification up parent chain
	if task.ParentTaskID != nil {
		parentChain, _ := h.db.GetParentChain(taskID, project)
		for _, parent := range parentChain {
			h.registry.Notify(project, parent.DispatchedBy, agent,
				fmt.Sprintf("Subtask blocked: '%s' → %s%s", task.Title, parent.Title, reasonStr), task.ID)
		}
	}

	return resultJSON(task)
}

func (h *Handlers) HandleCancelTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}
	reason := optionalString(req.GetString("reason", ""))

	task, err := h.db.CancelTask(taskID, agent, project, reason)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to cancel task: %v", err)), nil
	}

	// Notify dispatcher
	reasonStr := ""
	if reason != nil {
		reasonStr = ": " + *reason
	}
	h.registry.Notify(project, task.DispatchedBy, agent, fmt.Sprintf("Task cancelled: %s%s", task.Title, reasonStr), task.ID)

	// If this task has a parent, check if all sibling subtasks are now complete (cancelled counts)
	if task.ParentTaskID != nil {
		allDone, total, doneCount := h.db.CheckSubtasksComplete(*task.ParentTaskID, project)
		if allDone {
			parent, _ := h.db.GetTask(*task.ParentTaskID, project)
			if parent != nil {
				h.registry.Notify(project, parent.DispatchedBy, agent,
					fmt.Sprintf("All %d subtasks resolved for: %s", total, parent.Title), parent.ID)
			}
		} else {
			parent, _ := h.db.GetTask(*task.ParentTaskID, project)
			if parent != nil {
				h.registry.Notify(project, parent.DispatchedBy, agent,
					fmt.Sprintf("Subtask cancelled (%d/%d resolved): %s → %s", doneCount, total, task.Title, parent.Title), parent.ID)
			}
		}
	}

	return resultJSON(task)
}

func (h *Handlers) HandleArchiveTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	status := req.GetString("status", "")
	boardID := req.GetString("board_id", "")

	count, err := h.db.ArchiveTasks(project, status, boardID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to archive tasks: %v", err)), nil
	}

	msg := fmt.Sprintf("Archived %d tasks", count)
	if status != "" {
		msg += fmt.Sprintf(" (status=%s)", status)
	}
	if boardID != "" {
		msg += fmt.Sprintf(" (board=%s)", boardID)
	}
	return mcp.NewToolResultText(msg), nil
}

func (h *Handlers) HandleGetTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	taskID := req.GetString("task_id", "")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}
	includeSubtasks := req.GetBool("include_subtasks", false)

	var task *models.Task
	var err error
	if includeSubtasks {
		task, err = h.db.GetTaskWithSubtasks(taskID, project)
	} else {
		task, err = h.db.GetTask(taskID, project)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get task: %v", err)), nil
	}
	if task == nil {
		return mcp.NewToolResultError("task not found"), nil
	}

	// Include goal ancestry if task has a goal_id
	if task.GoalID != nil && *task.GoalID != "" {
		ancestry, _ := h.db.GetGoalAncestry(*task.GoalID, project)
		goal, _ := h.db.GetGoal(*task.GoalID, project)
		if goal != nil {
			if ancestry == nil {
				ancestry = []models.Goal{}
			}
			goalChain := append(ancestry, *goal)
			resp := map[string]any{
				"task":          task,
				"goal_ancestry": goalChain,
			}
			return resultJSON(resp)
		}
	}

	return resultJSON(task)
}

func (h *Handlers) HandleListTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	status := req.GetString("status", "")
	profile := req.GetString("profile", "")
	priority := req.GetString("priority", "")
	assignedTo := req.GetString("assigned_to", "")
	boardID := req.GetString("board_id", "")
	limit := req.GetInt("limit", 50)

	tasks, err := h.db.ListTasks(project, status, profile, priority, assignedTo, boardID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list tasks: %v", err)), nil
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	return resultJSON(map[string]any{
		"count": len(tasks),
		"tasks": tasks,
	})
}

// --- File locks ---

func (h *Handlers) HandleClaimFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	filePaths := req.GetString("file_paths", "[]")
	ttlSeconds := req.GetInt("ttl_seconds", 1800)

	lock, err := h.db.ClaimFiles(project, agent, filePaths, ttlSeconds)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to claim files: %v", err)), nil
	}

	// Auto-broadcast steering message
	subject := fmt.Sprintf("%s claimed files", agent)
	content := fmt.Sprintf("%s is now editing: %s", agent, filePaths)
	h.db.InsertMessage(project, agent, "*", "notification", subject, content, fmt.Sprintf(`{"tags":["file-lock"],"file_paths":%s}`, filePaths), "P1", 0, nil, nil)

	return resultJSON(lock)
}

func (h *Handlers) HandleReleaseFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	filePaths := req.GetString("file_paths", "[]")

	if err := h.db.ReleaseFiles(project, agent, filePaths); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to release files: %v", err)), nil
	}

	// Auto-broadcast info message
	subject := fmt.Sprintf("%s released files", agent)
	content := fmt.Sprintf("%s released: %s", agent, filePaths)
	h.db.InsertMessage(project, agent, "*", "notification", subject, content, fmt.Sprintf(`{"tags":["file-lock"],"file_paths":%s}`, filePaths), "P3", 3600, nil, nil)

	return resultJSON(map[string]any{"released": filePaths})
}

func (h *Handlers) HandleListLocks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)

	locks, err := h.db.ListFileLocks(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list locks: %v", err)), nil
	}
	if locks == nil {
		locks = []models.FileLock{}
	}

	return resultJSON(map[string]any{
		"count": len(locks),
		"locks": locks,
	})
}

// --- Agent lifecycle ---

func (h *Handlers) HandleDeactivateAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	name := strings.ToLower(req.GetString("name", ""))
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := h.db.DeactivateAgent(project, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to deactivate agent: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "register", Action: "deactivate", Agent: name, Project: project})

	return resultJSON(map[string]any{
		"deactivated": true,
		"agent":       name,
	})
}

func (h *Handlers) HandleCreateBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	name := req.GetString("name", "")
	slug := req.GetString("slug", "")
	if name == "" || slug == "" {
		return mcp.NewToolResultError("name and slug are required"), nil
	}
	description := req.GetString("description", "")

	board, err := h.db.CreateBoard(project, name, slug, description, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create board: %v", err)), nil
	}
	return resultJSON(board)
}

func (h *Handlers) HandleListBoards(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	boards, err := h.db.ListBoards(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list boards: %v", err)), nil
	}
	if boards == nil {
		boards = []models.Board{}
	}
	return resultJSON(boards)
}

func (h *Handlers) HandleArchiveBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	boardID := req.GetString("board_id", "")
	if boardID == "" {
		return mcp.NewToolResultError("board_id is required"), nil
	}

	if err := h.db.ArchiveBoard(project, boardID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to archive board: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Board %s archived (with all its tasks)", boardID)), nil
}

func (h *Handlers) HandleDeleteBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	boardID := req.GetString("board_id", "")
	if boardID == "" {
		return mcp.NewToolResultError("board_id is required"), nil
	}

	if err := h.db.DeleteBoard(project, boardID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete board: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Board %s permanently deleted", boardID)), nil
}

func (h *Handlers) HandleDeleteAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	name := strings.ToLower(req.GetString("name", ""))
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := h.db.DeleteAgent(project, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete agent: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"deleted": true,
		"agent":   name,
	})
}

func (h *Handlers) HandleDeleteProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	if err := h.db.DeleteProject(project); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete project: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"deleted": true,
		"project": project,
	})
}

func (h *Handlers) HandleSleepAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)

	if err := h.db.SleepAgent(project, agent); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sleep agent: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "register", Action: "sleep", Agent: agent, Project: project})

	return resultJSON(map[string]any{
		"status": "sleeping",
		"agent":  agent,
	})
}

// --- Find profiles by skill ---

func (h *Handlers) HandleFindProfiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	tag := req.GetString("skill_tag", "")
	if tag == "" {
		return mcp.NewToolResultError("skill_tag is required"), nil
	}

	profiles, err := h.db.FindProfilesBySkillTag(project, tag)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find profiles: %v", err)), nil
	}
	if profiles == nil {
		profiles = []models.Profile{}
	}

	return resultJSON(map[string]any{
		"skill_tag": tag,
		"count":     len(profiles),
		"profiles":  profiles,
	})
}

// --- Goals ---

func (h *Handlers) HandleCreateGoal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	goalType := req.GetString("type", "agent_goal")
	title := req.GetString("title", "")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}
	description := req.GetString("description", "")
	parentGoalID := optionalString(req.GetString("parent_goal_id", ""))
	ownerAgent := optionalString(req.GetString("owner_agent", ""))

	goal, err := h.db.CreateGoal(project, goalType, title, description, agent, ownerAgent, parentGoalID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create goal: %v", err)), nil
	}
	h.events.Emit(MCPEvent{Type: "goal", Action: "create", Agent: agent, Project: project, Label: title})
	return resultJSON(map[string]any{
		"goal": goal,
		"hint": "Goals are objectives, NOT tasks. To create actionable work items, use dispatch_task() and link them via goal_id. Goals track progress by counting linked tasks.",
	})
}

func (h *Handlers) HandleListGoals(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	goalType := req.GetString("type", "")
	status := req.GetString("status", "")
	ownerAgent := optionalString(req.GetString("owner_agent", ""))
	limit := req.GetInt("limit", 50)

	goals, err := h.db.ListGoals(project, goalType, status, ownerAgent, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list goals: %v", err)), nil
	}
	if goals == nil {
		goals = []models.Goal{}
	}

	// Enrich with progress
	type goalWithProgress struct {
		models.Goal
		TotalTasks int     `json:"total_tasks"`
		DoneTasks  int     `json:"done_tasks"`
		Progress   float64 `json:"progress"`
	}
	enriched := make([]goalWithProgress, 0, len(goals))
	for _, g := range goals {
		total, done := h.db.GetGoalProgress(g.ID, project)
		var progress float64
		if total > 0 {
			progress = float64(done) / float64(total)
		}
		enriched = append(enriched, goalWithProgress{Goal: g, TotalTasks: total, DoneTasks: done, Progress: progress})
	}

	return resultJSON(map[string]any{
		"count": len(enriched),
		"goals": enriched,
	})
}

func (h *Handlers) HandleGetGoal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	goalID := req.GetString("goal_id", "")
	if goalID == "" {
		return mcp.NewToolResultError("goal_id is required"), nil
	}

	gwp, err := h.db.GetGoalWithProgress(goalID, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get goal: %v", err)), nil
	}
	if gwp == nil {
		return mcp.NewToolResultError("goal not found"), nil
	}
	return resultJSON(gwp)
}

func (h *Handlers) HandleUpdateGoal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	goalID := req.GetString("goal_id", "")
	if goalID == "" {
		return mcp.NewToolResultError("goal_id is required"), nil
	}
	title := optionalString(req.GetString("title", ""))
	description := optionalString(req.GetString("description", ""))
	status := optionalString(req.GetString("status", ""))

	goal, err := h.db.UpdateGoal(goalID, project, title, description, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update goal: %v", err)), nil
	}
	return resultJSON(goal)
}

func (h *Handlers) HandleGetGoalCascade(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)

	cascade, err := h.db.GetGoalCascade(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get goal cascade: %v", err)), nil
	}
	return resultJSON(cascade)
}

// --- Session context ---

func (h *Handlers) buildSessionContext(project, agentName string, profileSlug *string) map[string]any {
	result := map[string]any{}

	// Profile
	if profileSlug != nil && *profileSlug != "" {
		profile, err := h.db.GetProfile(project, *profileSlug)
		if err == nil && profile != nil {
			result["profile"] = profile
		}
	}

	// Tasks
	assignedToMe, dispatchedByMe, _ := h.db.GetAgentTasks(project, agentName)
	if assignedToMe == nil {
		assignedToMe = []models.Task{}
	}
	if dispatchedByMe == nil {
		dispatchedByMe = []models.Task{}
	}
	// Build goal context for assigned tasks that have goal_id
	goalContext := map[string]any{}
	for _, t := range assignedToMe {
		if t.GoalID != nil && *t.GoalID != "" {
			if _, seen := goalContext[*t.GoalID]; !seen {
				ancestry, _ := h.db.GetGoalAncestry(*t.GoalID, project)
				goal, _ := h.db.GetGoal(*t.GoalID, project)
				if goal != nil {
					if ancestry == nil {
						ancestry = []models.Goal{}
					}
					goalContext[*t.GoalID] = append(ancestry, *goal)
				}
			}
		}
	}

	result["pending_tasks"] = map[string]any{
		"assigned_to_me":  assignedToMe,
		"dispatched_by_me": dispatchedByMe,
	}
	if len(goalContext) > 0 {
		result["goal_context"] = goalContext
	}

	// Unread messages (full content, not truncated)
	unread, err := h.db.GetInbox(project, agentName, true, 50)
	if err != nil || unread == nil {
		unread = []models.Message{}
	}
	result["unread_messages"] = unread

	// Active conversations
	convs, err := h.db.ListConversations(project, agentName)
	if err != nil || convs == nil {
		convs = []models.ConversationSummary{}
	}
	result["active_conversations"] = convs

	// Relevant memories (agent-scope + project-scope)
	memories, err := h.db.ListMemories(project, "", agentName, nil, 20)
	if err != nil || memories == nil {
		memories = []models.Memory{}
	}
	result["relevant_memories"] = memories

	// Vault context: auto-inject docs based on profile vault_paths
	if profileSlug != nil && *profileSlug != "" {
		profile, _ := h.db.GetProfile(project, *profileSlug)
		if profile != nil && profile.VaultPaths != "" && profile.VaultPaths != "[]" {
			var paths []string
			if err := json.Unmarshal([]byte(profile.VaultPaths), &paths); err == nil && len(paths) > 0 {
				// Resolve {slug} template
				resolved := make([]string, len(paths))
				for i, p := range paths {
					resolved[i] = strings.ReplaceAll(p, "{slug}", *profileSlug)
				}

				// Max ~4000 bytes (conservative estimate for ~4000 tokens)
				maxBytes := 16000 // ~4000 tokens at ~4 bytes/token
				docs, _ := h.db.GetVaultDocsByPaths(project, resolved, maxBytes)
				if len(docs) > 0 {
					type vaultCtx struct {
						Path    string `json:"path"`
						Title   string `json:"title"`
						Content string `json:"content"`
					}
					vaultDocs := make([]vaultCtx, len(docs))
					for i, d := range docs {
						vaultDocs[i] = vaultCtx{Path: d.Path, Title: d.Title, Content: d.Content}
					}
					result["vault_context"] = vaultDocs
				}
			}
		}
	}

	return result
}

func (h *Handlers) HandleGetSessionContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	profileSlugParam := optionalString(req.GetString("profile_slug", ""))

	_ = h.db.TouchAgent(project, agent)

	// Auto-detect profile from agent if not provided
	if profileSlugParam == nil {
		a, err := h.db.GetAgent(project, agent)
		if err == nil && a != nil && a.ProfileSlug != nil {
			profileSlugParam = a.ProfileSlug
		}
	}

	sessionCtx := h.buildSessionContext(project, agent, profileSlugParam)
	sessionCtx["agent"] = agent
	sessionCtx["project"] = project

	return resultJSON(sessionCtx)
}

// --- Soul RAG ---

func (h *Handlers) HandleQueryContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	query := req.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	limit := req.GetInt("limit", 10)

	// Source 1: memories via FTS5
	memories, err := h.db.SearchMemory(project, agent, query, nil, "", limit)
	if err != nil {
		memories = []models.Memory{}
	}

	// Truncate memory values
	memResults := make([]map[string]any, len(memories))
	for i, m := range memories {
		val := m.Value
		if len(val) > 500 {
			val = val[:500] + "..."
		}
		memResults[i] = map[string]any{
			"type":       "memory",
			"key":        m.Key,
			"value":      val,
			"scope":      m.Scope,
			"agent_name": m.AgentName,
			"confidence": m.Confidence,
			"updated_at": m.UpdatedAt,
		}
	}

	// Source 2: completed tasks (implicit knowledge)
	doneTasks, err := h.db.ListTasks(project, "done", "", "", "", "", limit)
	if err != nil {
		doneTasks = []models.Task{}
	}

	// Filter tasks by relevance (simple keyword matching on title+description+result)
	taskResults := make([]map[string]any, 0)
	queryLower := strings.ToLower(query)
	for _, t := range doneTasks {
		searchable := strings.ToLower(t.Title + " " + t.Description)
		if t.Result != nil {
			searchable += " " + strings.ToLower(*t.Result)
		}
		// Simple relevance: check if any query word appears
		words := strings.Fields(queryLower)
		match := false
		for _, w := range words {
			if strings.Contains(searchable, w) {
				match = true
				break
			}
		}
		if match {
			entry := map[string]any{
				"type":         "task_result",
				"task_id":      t.ID,
				"title":        t.Title,
				"profile":      t.ProfileSlug,
				"completed_at": t.CompletedAt,
			}
			if t.Result != nil {
				r := *t.Result
				if len(r) > 500 {
					r = r[:500] + "..."
				}
				entry["result"] = r
			}
			taskResults = append(taskResults, entry)
		}
	}

	// Combine and return
	allResults := append(memResults, taskResults...)

	return resultJSON(map[string]any{
		"query":   query,
		"count":   len(allResults),
		"results": allResults,
	})
}

// --- Teams + Orgs Handlers ---

func (h *Handlers) HandleCreateOrg(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	slug := req.GetString("slug", "")
	description := req.GetString("description", "")

	if name == "" || slug == "" {
		return mcp.NewToolResultError("name and slug are required"), nil
	}

	org, err := h.db.CreateOrg(name, slug, description)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create org: %v", err)), nil
	}
	return resultJSON(org)
}

func (h *Handlers) HandleListOrgs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	orgs, err := h.db.ListOrgs()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list orgs: %v", err)), nil
	}
	if orgs == nil {
		orgs = []models.Org{}
	}
	return resultJSON(map[string]any{"count": len(orgs), "orgs": orgs})
}

func (h *Handlers) HandleCreateTeam(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	name := req.GetString("name", "")
	slug := req.GetString("slug", "")
	description := req.GetString("description", "")
	teamType := req.GetString("type", "regular")
	orgID := optionalString(req.GetString("org_id", ""))
	parentTeamID := optionalString(req.GetString("parent_team_id", ""))

	if name == "" || slug == "" {
		return mcp.NewToolResultError("name and slug are required"), nil
	}

	// Validate type
	switch teamType {
	case "regular", "admin", "bot":
	default:
		return mcp.NewToolResultError("type must be 'regular', 'admin', or 'bot'"), nil
	}

	team, err := h.db.CreateTeam(name, slug, project, description, teamType, orgID, parentTeamID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create team: %v", err)), nil
	}
	return resultJSON(team)
}

func (h *Handlers) HandleListTeams(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)

	teams, err := h.db.ListTeams(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list teams: %v", err)), nil
	}
	if teams == nil {
		teams = []models.Team{}
	}

	// Enrich with members
	result := make([]map[string]any, 0, len(teams))
	for _, t := range teams {
		members, _ := h.db.GetTeamMembers(t.ID)
		if members == nil {
			members = []models.TeamMember{}
		}
		result = append(result, map[string]any{
			"team":    t,
			"members": members,
		})
	}

	return resultJSON(map[string]any{"count": len(result), "teams": result})
}

func (h *Handlers) HandleAddTeamMember(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	teamSlug := req.GetString("team", "")
	agentName := strings.ToLower(req.GetString("agent_name", ""))
	role := req.GetString("role", "member")

	if teamSlug == "" || agentName == "" {
		return mcp.NewToolResultError("team and agent_name are required"), nil
	}

	// Validate role
	switch role {
	case "admin", "lead", "member", "observer":
	default:
		return mcp.NewToolResultError("role must be 'admin', 'lead', 'member', or 'observer'"), nil
	}

	team, err := h.db.GetTeam(project, teamSlug)
	if err != nil || team == nil {
		return mcp.NewToolResultError(fmt.Sprintf("team '%s' not found", teamSlug)), nil
	}

	if err := h.db.AddTeamMember(team.ID, agentName, project, role); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add member: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"team":       teamSlug,
		"agent_name": agentName,
		"role":       role,
		"added":      true,
	})
}

func (h *Handlers) HandleRemoveTeamMember(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	teamSlug := req.GetString("team", "")
	agentName := strings.ToLower(req.GetString("agent_name", ""))

	if teamSlug == "" || agentName == "" {
		return mcp.NewToolResultError("team and agent_name are required"), nil
	}

	team, err := h.db.GetTeam(project, teamSlug)
	if err != nil || team == nil {
		return mcp.NewToolResultError(fmt.Sprintf("team '%s' not found", teamSlug)), nil
	}

	if err := h.db.RemoveTeamMember(team.ID, agentName); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to remove member: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"team":       teamSlug,
		"agent_name": agentName,
		"removed":    true,
	})
}

func (h *Handlers) HandleGetTeamInbox(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	teamSlug := req.GetString("team", "")
	limit := req.GetInt("limit", 50)

	if teamSlug == "" {
		return mcp.NewToolResultError("team is required"), nil
	}

	team, err := h.db.GetTeam(project, teamSlug)
	if err != nil || team == nil {
		return mcp.NewToolResultError(fmt.Sprintf("team '%s' not found", teamSlug)), nil
	}

	msgs, err := h.db.GetTeamInbox(team.ID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get team inbox: %v", err)), nil
	}
	if msgs == nil {
		msgs = []models.Message{}
	}

	return resultJSON(map[string]any{
		"team":     teamSlug,
		"count":    len(msgs),
		"messages": msgs,
	})
}

func (h *Handlers) HandleAddNotifyChannel(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	agent := resolveAgent(ctx, req)
	target := strings.ToLower(req.GetString("target", ""))

	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	if err := h.db.AddNotifyChannel(agent, project, target); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add notify channel: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"agent":  agent,
		"target": target,
		"added":  true,
	})
}

// --- Vault ---

func (h *Handlers) HandleRegisterVault(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	path := req.GetString("path", "")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	// Verify path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("path not accessible: %v", err)), nil
	}
	if !info.IsDir() {
		return mcp.NewToolResultError("path must be a directory"), nil
	}

	// Save to DB
	if err := h.db.RegisterVault(project, path); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to register vault: %v", err)), nil
	}

	// Start indexing + watching
	cfg := models.VaultConfig{Path: path, Project: project}
	if h.vaultWatcher != nil {
		h.vaultWatcher.AddVault(cfg)
	}

	// Get stats
	count, totalSize, _ := h.db.GetVaultStats(project)

	// Check if any profiles exist with empty vault_paths
	profiles, _ := h.db.ListProfiles(project)
	var emptyVaultProfiles []string
	for _, p := range profiles {
		if p.VaultPaths == "" || p.VaultPaths == "[]" {
			emptyVaultProfiles = append(emptyVaultProfiles, p.Slug)
		}
	}

	resp := map[string]any{
		"registered":   true,
		"project":      project,
		"path":         path,
		"docs_indexed": count,
		"total_bytes":  totalSize,
	}
	if len(emptyVaultProfiles) > 0 {
		resp["hint"] = fmt.Sprintf("Profiles with empty vault_paths: %v. Consider updating them with register_profile to auto-inject vault docs at boot.", emptyVaultProfiles)
	}
	return resultJSON(resp)
}

func (h *Handlers) HandleSearchVault(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	query := req.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	limit := req.GetInt("limit", 10)

	var tags []string
	if tagsJSON := req.GetString("tags", ""); tagsJSON != "" && tagsJSON != "[]" {
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
	}

	results, err := h.db.SearchVault(project, query, tags, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}
	if results == nil {
		results = []models.VaultSearchResult{}
	}

	return resultJSON(map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
}

func (h *Handlers) HandleGetVaultDoc(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	path := req.GetString("path", "")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	doc, err := h.db.GetVaultDoc(project, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get vault doc: %v", err)), nil
	}
	if doc == nil {
		return mcp.NewToolResultError(fmt.Sprintf("vault doc not found: %s", path)), nil
	}

	return resultJSON(doc)
}

func (h *Handlers) HandleListVaultDocs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := resolveProject(ctx, req)
	limit := req.GetInt("limit", 100)

	var tags []string
	if tagsJSON := req.GetString("tags", ""); tagsJSON != "" && tagsJSON != "[]" {
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
	}

	docs, err := h.db.ListVaultDocs(project, tags, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list vault docs: %v", err)), nil
	}
	if docs == nil {
		docs = []models.VaultDoc{}
	}

	// Return metadata only (strip content)
	type docMeta struct {
		Path      string `json:"path"`
		Title     string `json:"title"`
		Owner     string `json:"owner"`
		Status    string `json:"status"`
		Tags      string `json:"tags"`
		SizeBytes int    `json:"size_bytes"`
		UpdatedAt string `json:"updated_at"`
	}
	metas := make([]docMeta, len(docs))
	for i, d := range docs {
		metas[i] = docMeta{
			Path: d.Path, Title: d.Title, Owner: d.Owner,
			Status: d.Status, Tags: d.Tags, SizeBytes: d.SizeBytes,
			UpdatedAt: d.UpdatedAt,
		}
	}

	count, totalSize, _ := h.db.GetVaultStats(project)

	return resultJSON(map[string]any{
		"count":       len(metas),
		"total_docs":  count,
		"total_bytes": totalSize,
		"docs":        metas,
	})
}
