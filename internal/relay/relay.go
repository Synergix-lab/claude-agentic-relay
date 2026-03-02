package relay

import (
	"agent-relay/internal/db"

	"github.com/mark3labs/mcp-go/server"
)

// Relay is the main struct that wires together the MCP server, DB, and notifications.
type Relay struct {
	MCPServer *server.MCPServer
	HTTP      *server.StreamableHTTPServer
	DB        *db.DB
	Registry  *SessionRegistry
}

// New creates a fully wired Relay with all tools registered.
func New(database *db.DB) *Relay {
	mcpSrv := server.NewMCPServer(
		"agent-relay",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithLogging(),
		server.WithRecovery(),
	)

	registry := NewSessionRegistry(mcpSrv)
	handlers := NewHandlers(database, registry)

	// Register all 6 tools
	mcpSrv.AddTools(
		server.ServerTool{Tool: registerAgentTool(), Handler: handlers.HandleRegisterAgent},
		server.ServerTool{Tool: sendMessageTool(), Handler: handlers.HandleSendMessage},
		server.ServerTool{Tool: getInboxTool(), Handler: handlers.HandleGetInbox},
		server.ServerTool{Tool: getThreadTool(), Handler: handlers.HandleGetThread},
		server.ServerTool{Tool: listAgentsTool(), Handler: handlers.HandleListAgents},
		server.ServerTool{Tool: markReadTool(), Handler: handlers.HandleMarkRead},
	)

	httpSrv := server.NewStreamableHTTPServer(
		mcpSrv,
		server.WithHTTPContextFunc(HTTPContextFunc),
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	return &Relay{
		MCPServer: mcpSrv,
		HTTP:      httpSrv,
		DB:        database,
		Registry:  registry,
	}
}
