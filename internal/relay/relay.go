package relay

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"agent-relay/internal/db"
	"agent-relay/internal/ingest"
	"agent-relay/internal/vault"
	"agent-relay/internal/web"

	"github.com/mark3labs/mcp-go/server"
)

// Relay is the main struct that wires together the MCP server, DB, and notifications.
type Relay struct {
	MCPServer    *server.MCPServer
	HTTP         *server.StreamableHTTPServer
	DB           *db.DB
	Registry     *SessionRegistry
	Ingester     *ingest.Ingester
	VaultWatcher *vault.Watcher
	Events       *EventBus
	httpServer   *http.Server
}

// New creates a fully wired Relay with all tools registered.
func New(database *db.DB, ingester *ingest.Ingester, vaultWatcher *vault.Watcher) *Relay {
	mcpSrv := server.NewMCPServer(
		"agent-relay",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithLogging(),
		server.WithRecovery(),
	)

	events := NewEventBus()
	registry := NewSessionRegistry(mcpSrv)
	handlers := NewHandlers(database, registry, ingester, vaultWatcher, events)

	// Register all tools
	mcpSrv.AddTools(
		server.ServerTool{Tool: whoamiTool(), Handler: handlers.HandleWhoami},
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
		server.ServerTool{Tool: leaveConversationTool(), Handler: handlers.HandleLeaveConversation},
		server.ServerTool{Tool: archiveConversationTool(), Handler: handlers.HandleArchiveConversation},
		// Memory tools
		server.ServerTool{Tool: setMemoryTool(), Handler: handlers.HandleSetMemory},
		server.ServerTool{Tool: getMemoryTool(), Handler: handlers.HandleGetMemory},
		server.ServerTool{Tool: searchMemoryTool(), Handler: handlers.HandleSearchMemory},
		server.ServerTool{Tool: listMemoriesTool(), Handler: handlers.HandleListMemories},
		server.ServerTool{Tool: deleteMemoryTool(), Handler: handlers.HandleDeleteMemory},
		server.ServerTool{Tool: resolveConflictTool(), Handler: handlers.HandleResolveConflict},
		// Profile tools
		server.ServerTool{Tool: registerProfileTool(), Handler: handlers.HandleRegisterProfile},
		server.ServerTool{Tool: getProfileTool(), Handler: handlers.HandleGetProfile},
		server.ServerTool{Tool: listProfilesTool(), Handler: handlers.HandleListProfiles},
		server.ServerTool{Tool: findProfilesTool(), Handler: handlers.HandleFindProfiles},
		// Task tools
		server.ServerTool{Tool: dispatchTaskTool(), Handler: handlers.HandleDispatchTask},
		server.ServerTool{Tool: claimTaskTool(), Handler: handlers.HandleClaimTask},
		server.ServerTool{Tool: startTaskTool(), Handler: handlers.HandleStartTask},
		server.ServerTool{Tool: completeTaskTool(), Handler: handlers.HandleCompleteTask},
		server.ServerTool{Tool: blockTaskTool(), Handler: handlers.HandleBlockTask},
		server.ServerTool{Tool: cancelTaskTool(), Handler: handlers.HandleCancelTask},
		server.ServerTool{Tool: getTaskTool(), Handler: handlers.HandleGetTask},
		server.ServerTool{Tool: listTasksTool(), Handler: handlers.HandleListTasks},
		server.ServerTool{Tool: archiveTasksTool(), Handler: handlers.HandleArchiveTasks},
		// Boards
		server.ServerTool{Tool: createBoardTool(), Handler: handlers.HandleCreateBoard},
		server.ServerTool{Tool: listBoardsTool(), Handler: handlers.HandleListBoards},
		server.ServerTool{Tool: archiveBoardTool(), Handler: handlers.HandleArchiveBoard},
		server.ServerTool{Tool: deleteBoardTool(), Handler: handlers.HandleDeleteBoard},
		// Goals
		server.ServerTool{Tool: createGoalTool(), Handler: handlers.HandleCreateGoal},
		server.ServerTool{Tool: listGoalsTool(), Handler: handlers.HandleListGoals},
		server.ServerTool{Tool: getGoalTool(), Handler: handlers.HandleGetGoal},
		server.ServerTool{Tool: updateGoalTool(), Handler: handlers.HandleUpdateGoal},
		server.ServerTool{Tool: getGoalCascadeTool(), Handler: handlers.HandleGetGoalCascade},
		// Vault
		server.ServerTool{Tool: registerVaultTool(), Handler: handlers.HandleRegisterVault},
		server.ServerTool{Tool: searchVaultTool(), Handler: handlers.HandleSearchVault},
		server.ServerTool{Tool: getVaultDocTool(), Handler: handlers.HandleGetVaultDoc},
		server.ServerTool{Tool: listVaultDocsTool(), Handler: handlers.HandleListVaultDocs},
		// Agent lifecycle
		server.ServerTool{Tool: deactivateAgentTool(), Handler: handlers.HandleDeactivateAgent},
		server.ServerTool{Tool: deleteAgentTool(), Handler: handlers.HandleDeleteAgent},
		server.ServerTool{Tool: sleepAgentTool(), Handler: handlers.HandleSleepAgent},
		// Soul RAG
		server.ServerTool{Tool: queryContextTool(), Handler: handlers.HandleQueryContext},
		// Session context
		server.ServerTool{Tool: getSessionContextTool(), Handler: handlers.HandleGetSessionContext},
		// Teams + Orgs
		server.ServerTool{Tool: createOrgTool(), Handler: handlers.HandleCreateOrg},
		server.ServerTool{Tool: listOrgsTool(), Handler: handlers.HandleListOrgs},
		server.ServerTool{Tool: createTeamTool(), Handler: handlers.HandleCreateTeam},
		server.ServerTool{Tool: listTeamsTool(), Handler: handlers.HandleListTeams},
		server.ServerTool{Tool: addTeamMemberTool(), Handler: handlers.HandleAddTeamMember},
		server.ServerTool{Tool: removeTeamMemberTool(), Handler: handlers.HandleRemoveTeamMember},
		server.ServerTool{Tool: getTeamInboxTool(), Handler: handlers.HandleGetTeamInbox},
		server.ServerTool{Tool: addNotifyChannelTool(), Handler: handlers.HandleAddNotifyChannel},
	)

	httpSrv := server.NewStreamableHTTPServer(
		mcpSrv,
		server.WithHTTPContextFunc(HTTPContextFunc),
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	return &Relay{
		MCPServer:    mcpSrv,
		HTTP:         httpSrv,
		DB:           database,
		Registry:     registry,
		Ingester:     ingester,
		VaultWatcher: vaultWatcher,
		Events:       events,
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
