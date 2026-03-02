package relay

import (
	"context"
	"net/http"
)

type contextKey string

const agentNameKey contextKey = "agent_name"

// HTTPContextFunc extracts the agent name from the ?agent= query parameter
// and injects it into the request context.
func HTTPContextFunc(ctx context.Context, r *http.Request) context.Context {
	agent := r.URL.Query().Get("agent")
	if agent == "" {
		agent = "anonymous"
	}
	return context.WithValue(ctx, agentNameKey, agent)
}

// AgentFromContext retrieves the agent name from the context.
func AgentFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(agentNameKey).(string); ok {
		return v
	}
	return "anonymous"
}
