package relay

import (
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/server"
)

// SessionRegistry tracks connected agent sessions for push notifications.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string][]string // agentName → []sessionID
	mcpSrv   *server.MCPServer
}

func NewSessionRegistry(mcpSrv *server.MCPServer) *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string][]string),
		mcpSrv:   mcpSrv,
	}
}

// Register associates a session ID with an agent name.
func (r *SessionRegistry) Register(agentName, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[agentName] = append(r.sessions[agentName], sessionID)
}

// Unregister removes a session ID from an agent.
func (r *SessionRegistry) Unregister(agentName, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.sessions[agentName]
	for i, id := range ids {
		if id == sessionID {
			r.sessions[agentName] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(r.sessions[agentName]) == 0 {
		delete(r.sessions, agentName)
	}
}

// Notify sends a notification to all sessions of the given agent.
func (r *SessionRegistry) Notify(agentName, from, subject, messageID string) {
	r.mu.RLock()
	sessionIDs := make([]string, len(r.sessions[agentName]))
	copy(sessionIDs, r.sessions[agentName])
	r.mu.RUnlock()

	params := map[string]any{
		"from":       from,
		"subject":    subject,
		"message_id": messageID,
	}

	for _, sid := range sessionIDs {
		if err := r.mcpSrv.SendNotificationToSpecificClient(sid, "notifications/message", params); err != nil {
			log.Printf("notify %s (session %s): %v", agentName, sid, err)
		}
	}
}

// NotifyBroadcast sends a notification to all connected agents except the sender.
func (r *SessionRegistry) NotifyBroadcast(senderName, subject, messageID string) {
	r.mu.RLock()
	agents := make(map[string][]string)
	for name, ids := range r.sessions {
		if name != senderName {
			agents[name] = append([]string{}, ids...)
		}
	}
	r.mu.RUnlock()

	params := map[string]any{
		"from":       senderName,
		"subject":    subject,
		"message_id": messageID,
	}

	for _, sessionIDs := range agents {
		for _, sid := range sessionIDs {
			if err := r.mcpSrv.SendNotificationToSpecificClient(sid, "notifications/message", params); err != nil {
				log.Printf("broadcast notify (session %s): %v", sid, err)
			}
		}
	}
}
