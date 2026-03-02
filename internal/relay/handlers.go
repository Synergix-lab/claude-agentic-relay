package relay

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-relay/internal/db"
	"agent-relay/internal/models"

	"github.com/mark3labs/mcp-go/mcp"
)

type Handlers struct {
	db       *db.DB
	registry *SessionRegistry
}

func NewHandlers(db *db.DB, registry *SessionRegistry) *Handlers {
	return &Handlers{db: db, registry: registry}
}

func (h *Handlers) HandleRegisterAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	role := req.GetString("role", "")
	description := req.GetString("description", "")

	agent, err := h.db.RegisterAgent(name, role, description)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to register agent: %v", err)), nil
	}

	// Register the session for push notifications
	if sess := sessionFromContext(ctx); sess != nil {
		h.registry.Register(name, sess.SessionID())
	}

	return resultJSON(agent)
}

func (h *Handlers) HandleSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	from := AgentFromContext(ctx)
	to := req.GetString("to", "")
	msgType := req.GetString("type", "notification")
	subject := req.GetString("subject", "")
	content := req.GetString("content", "")
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}

	metadata := req.GetString("metadata", "{}")
	replyTo := optionalString(req.GetString("reply_to", ""))
	conversationID := optionalString(req.GetString("conversation_id", ""))

	// Touch sender's last_seen
	_ = h.db.TouchAgent(from)

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

	msg, err := h.db.InsertMessage(from, to, msgType, subject, content, metadata, replyTo, conversationID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	// Push notification
	if conversationID != nil {
		h.notifyConversation(*conversationID, from, subject, msg.ID)
	} else if to == "*" {
		h.registry.NotifyBroadcast(from, subject, msg.ID)
	} else {
		h.registry.Notify(to, from, subject, msg.ID)
	}

	return resultJSON(msg)
}

func (h *Handlers) HandleGetInbox(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := AgentFromContext(ctx)
	unreadOnly := req.GetBool("unread_only", true)
	limit := req.GetInt("limit", 50)

	_ = h.db.TouchAgent(agent)

	messages, err := h.db.GetInbox(agent, unreadOnly, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get inbox: %v", err)), nil
	}
	if messages == nil {
		messages = []models.Message{}
	}

	return resultJSON(map[string]any{
		"agent":    agent,
		"count":    len(messages),
		"messages": messages,
	})
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
	agents, err := h.db.ListAgents()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list agents: %v", err)), nil
	}
	if agents == nil {
		agents = []models.Agent{}
	}

	return resultJSON(map[string]any{
		"count":  len(agents),
		"agents": agents,
	})
}

func (h *Handlers) HandleMarkRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := AgentFromContext(ctx)

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

	count, err := h.db.MarkRead(ids, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to mark read: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"marked_read": count,
	})
}

func (h *Handlers) HandleCreateConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := AgentFromContext(ctx)
	title := req.GetString("title", "")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	members := req.GetStringSlice("members", nil)
	if len(members) == 0 {
		return mcp.NewToolResultError("at least one other member is required"), nil
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

	conv, err := h.db.CreateConversation(title, agent, members)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create conversation: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"conversation": conv,
		"members":      members,
	})
}

func (h *Handlers) HandleListConversations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := AgentFromContext(ctx)

	convs, err := h.db.ListConversations(agent)
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
	agent := AgentFromContext(ctx)
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

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"count":           len(messages),
		"messages":        messages,
	})
}

func (h *Handlers) HandleInviteToConversation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := AgentFromContext(ctx)
	convID := req.GetString("conversation_id", "")
	if convID == "" {
		return mcp.NewToolResultError("conversation_id is required"), nil
	}
	invitee := req.GetString("agent_name", "")
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
	h.registry.Notify(invitee, agent, fmt.Sprintf("You were invited to conversation: %s", convID), "")

	return resultJSON(map[string]any{
		"conversation_id": convID,
		"invited":         invitee,
	})
}

func (h *Handlers) notifyConversation(conversationID, senderName, subject, messageID string) {
	members, err := h.db.GetConversationMembers(conversationID)
	if err != nil {
		return
	}
	for _, m := range members {
		if m.AgentName != senderName {
			h.registry.Notify(m.AgentName, senderName, subject, messageID)
		}
	}
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
