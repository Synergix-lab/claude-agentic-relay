package relay

import (
	"context"
	"net/http"
)

type contextKey string

const agentNameKey contextKey = "agent_name"
const projectKey contextKey = "project_name"

// HTTPContextFunc extracts the agent name from the ?agent= query parameter
// and the project from the ?project= query parameter, injecting both into the request context.
func HTTPContextFunc(ctx context.Context, r *http.Request) context.Context {
	agent := r.URL.Query().Get("agent")
	if agent == "" {
		agent = "anonymous"
	}
	project := r.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	ctx = context.WithValue(ctx, agentNameKey, agent)
	return context.WithValue(ctx, projectKey, project)
}

// AgentFromContext retrieves the agent name from the context.
func AgentFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(agentNameKey).(string); ok {
		return v
	}
	return "anonymous"
}

// ProjectFromContext retrieves the project name from the context.
func ProjectFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(projectKey).(string); ok {
		return v
	}
	return "default"
}
