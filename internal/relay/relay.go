package relay

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"agent-relay/internal/db"
	"agent-relay/internal/web"

	"github.com/mark3labs/mcp-go/server"
)

// Relay is the main struct that wires together the MCP server, DB, and notifications.
type Relay struct {
	MCPServer  *server.MCPServer
	HTTP       *server.StreamableHTTPServer
	DB         *db.DB
	Registry   *SessionRegistry
	httpServer *http.Server
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

	// Register all tools
	mcpSrv.AddTools(
		server.ServerTool{Tool: registerAgentTool(), Handler: handlers.HandleRegisterAgent},
		server.ServerTool{Tool: sendMessageTool(), Handler: handlers.HandleSendMessage},
		server.ServerTool{Tool: getInboxTool(), Handler: handlers.HandleGetInbox},
		server.ServerTool{Tool: getThreadTool(), Handler: handlers.HandleGetThread},
		server.ServerTool{Tool: listAgentsTool(), Handler: handlers.HandleListAgents},
		server.ServerTool{Tool: markReadTool(), Handler: handlers.HandleMarkRead},
		server.ServerTool{Tool: createConversationTool(), Handler: handlers.HandleCreateConversation},
		server.ServerTool{Tool: listConversationsTool(), Handler: handlers.HandleListConversations},
		server.ServerTool{Tool: getConversationMessagesTool(), Handler: handlers.HandleGetConversationMessages},
		server.ServerTool{Tool: inviteToConversationTool(), Handler: handlers.HandleInviteToConversation},
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

// ListenAndServe starts a composite HTTP server that serves:
//   - /mcp     → MCP Streamable HTTP handler
//   - /api/*   → REST API for the web UI
//   - /*       → Embedded static files (web UI)
func (r *Relay) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	// MCP handler
	mux.Handle("/mcp", r.HTTP)

	// REST API
	mux.HandleFunc("/api/", r.ServeAPI)

	// Embedded static files
	staticFS, err := fs.Sub(web.StaticFiles, "static")
	if err != nil {
		log.Fatalf("failed to create sub FS: %v", err)
	}
	mux.Handle("/", http.FileServerFS(staticFS))

	r.httpServer = &http.Server{Addr: addr, Handler: mux}
	return r.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (r *Relay) Shutdown(ctx context.Context) error {
	if r.httpServer != nil {
		return r.httpServer.Shutdown(ctx)
	}
	return nil
}
