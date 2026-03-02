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
	if to == "" {
		return mcp.NewToolResultError("to is required"), nil
	}

	msgType := req.GetString("type", "notification")
	subject := req.GetString("subject", "")
	content := req.GetString("content", "")
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}

	metadata := req.GetString("metadata", "{}")
	replyTo := optionalString(req.GetString("reply_to", ""))

	// Touch sender's last_seen
	_ = h.db.TouchAgent(from)

	msg, err := h.db.InsertMessage(from, to, msgType, subject, content, metadata, replyTo)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	// Push notification
	if to == "*" {
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
	ids := req.GetStringSlice("message_ids", nil)
	if len(ids) == 0 {
		return mcp.NewToolResultError("message_ids is required"), nil
	}

	count, err := h.db.MarkRead(ids, agent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to mark read: %v", err)), nil
	}

	return resultJSON(map[string]any{
		"marked_read": count,
	})
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
