package relay

import "github.com/mark3labs/mcp-go/mcp"

// asParam is added to every tool that uses agent identity.
var asParam = mcp.WithString("as", mcp.Description("Override agent identity from URL"))

// projectParam is added to every tool that needs project scoping.
// It allows overriding the default ?project= from the URL,
// so agents can switch projects without changing the MCP connection.
var projectParam = mcp.WithString("project", mcp.Description("Override project from URL"))

func whoamiTool() mcp.Tool {
	return mcp.NewTool(
		"whoami",
		mcp.WithDescription("Identify your Claude Code session. Generate a unique salt (3+ random words), include it in your message, then call this tool. Returns session_id for register_agent."),
		mcp.WithString("salt", mcp.Description("Unique string you generated (3+ random words, e.g. 'purple-falcon-nebula')"), mcp.Required()),
	)
}

func registerAgentTool() mcp.Tool {
	return mcp.NewTool(
		"register_agent",
		mcp.WithDescription("Register/re-register an agent. Returns session_context. If is_executive=true, auto-joins 'leadership' admin team (enables broadcast)."),
		projectParam,
		mcp.WithString("name", mcp.Description("Unique agent name (re-registering updates)"), mcp.Required()),
		mcp.WithString("role", mcp.Description("Role description")),
		mcp.WithString("description", mcp.Description("Current work focus")),
		mcp.WithString("reports_to", mcp.Description("Parent agent name (org hierarchy)")),
		mcp.WithBoolean("is_executive", mcp.Description("Executive flag (enables broadcast)")),
		mcp.WithString("profile_slug", mcp.Description("Profile archetype to run")),
		mcp.WithString("session_id", mcp.Description("$CLAUDE_SESSION_ID for activity tracking")),
		mcp.WithString("interest_tags", mcp.Description("JSON array of tags for budget filtering")),
		mcp.WithNumber("max_context_bytes", mcp.Description("Max bytes for budget-pruned inbox (default: 16384)")),
	)
}

func sendMessageTool() mcp.Tool {
	return mcp.NewTool(
		"send_message",
		mcp.WithDescription("Send a message. Recipients: agent name, '*' (broadcast, requires admin team), 'team:<slug>', or set conversation_id."),
		asParam,
		projectParam,
		mcp.WithString("to", mcp.Description("Recipient: agent name, '*', or 'team:<slug>'"), mcp.Required()),
		mcp.WithString("type",
			mcp.Description("Message type"),
			mcp.Enum("question", "response", "notification", "code-snippet", "task", "user_question"),
		),
		mcp.WithString("subject", mcp.Description("Message subject line"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Message body content"), mcp.Required()),
		mcp.WithString("reply_to", mcp.Description("Message ID for threading")),
		mcp.WithString("metadata", mcp.Description("JSON metadata")),
		mcp.WithString("conversation_id", mcp.Description("Target conversation (overrides to)")),
		mcp.WithString("priority",
			mcp.Description("P0=interrupt, P1=steering, P2=advisory (default), P3=info"),
			mcp.Enum("P0", "P1", "P2", "P3", "interrupt", "steering", "advisory", "info"),
		),
		mcp.WithNumber("ttl_seconds", mcp.Description("TTL in seconds (default: 3600, 0=never)")),
	)
}

func getInboxTool() mcp.Tool {
	return mcp.NewTool(
		"get_inbox",
		mcp.WithDescription("Get inbox messages (sent to agent or broadcast)."),
		asParam,
		projectParam,
		mcp.WithBoolean("unread_only", mcp.Description("Only return unread messages (default: true)")),
		mcp.WithNumber("limit", mcp.Description("Max number of messages to return (default: 10).")),
		mcp.WithBoolean("full_content", mcp.Description("Full content instead of 300-char truncation (default: false)")),
		mcp.WithBoolean("apply_budget", mcp.Description("Budget-prune by priority/tags/freshness (default: false)")),
	)
}

func ackDeliveryTool() mcp.Tool {
	return mcp.NewTool(
		"ack_delivery",
		mcp.WithDescription("Acknowledge a message delivery (surfaced → acknowledged)."),
		mcp.WithString("delivery_id", mcp.Description("Delivery ID to acknowledge"), mcp.Required()),
	)
}

func getThreadTool() mcp.Tool {
	return mcp.NewTool(
		"get_thread",
		mcp.WithDescription("Get a complete message thread."),
		projectParam,
		mcp.WithString("message_id", mcp.Description("Any message ID in the thread"), mcp.Required()),
	)
}

func listAgentsTool() mcp.Tool {
	return mcp.NewTool(
		"list_agents",
		mcp.WithDescription("List agents with status and activity."),
		projectParam,
	)
}

func markReadTool() mcp.Tool {
	return mcp.NewTool(
		"mark_read",
		mcp.WithDescription("Mark messages as read."),
		asParam,
		projectParam,
		mcp.WithArray("message_ids",
			mcp.Description("List of message IDs to mark as read"),
			mcp.WithStringItems(),
		),
		mcp.WithString("conversation_id", mcp.Description("Mark entire conversation as read")),
	)
}

func createConversationTool() mcp.Tool {
	return mcp.NewTool(
		"create_conversation",
		mcp.WithDescription("Create a multi-agent conversation."),
		asParam,
		projectParam,
		mcp.WithString("title", mcp.Description("Conversation title"), mcp.Required()),
		mcp.WithArray("members",
			mcp.Description("Agent names to include (you are added automatically)"),
			mcp.Required(),
			mcp.WithStringItems(),
		),
	)
}

func listConversationsTool() mcp.Tool {
	return mcp.NewTool(
		"list_conversations",
		mcp.WithDescription("List your conversations with unread counts."),
		asParam,
		projectParam,
	)
}

func getConversationMessagesTool() mcp.Tool {
	return mcp.NewTool(
		"get_conversation_messages",
		mcp.WithDescription("Get conversation messages (chronological)."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max number of messages to return (default: 50)")),
		mcp.WithString("format", mcp.Description("'full' (default), 'compact' (metadata only), 'digest' (metadata + 200 chars)")),
		mcp.WithBoolean("full_content", mcp.Description("Full content in 'full' format (default: true)")),
	)
}

func inviteToConversationTool() mcp.Tool {
	return mcp.NewTool(
		"invite_to_conversation",
		mcp.WithDescription("Invite an agent to a conversation."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
		mcp.WithString("agent_name", mcp.Description("Agent name to invite"), mcp.Required()),
	)
}

func leaveConversationTool() mcp.Tool {
	return mcp.NewTool(
		"leave_conversation",
		mcp.WithDescription("Leave a conversation."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
	)
}

func archiveConversationTool() mcp.Tool {
	return mcp.NewTool(
		"archive_conversation",
		mcp.WithDescription("Archive a conversation (hidden from all members)."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
	)
}

// --- Memory tools ---

func setMemoryTool() mcp.Tool {
	return mcp.NewTool(
		"set_memory",
		mcp.WithDescription("Store knowledge in persistent memory. Duplicate keys at same scope create a conflict (use resolve_conflict)."),
		asParam,
		projectParam,
		mcp.WithString("key", mcp.Description("Memory key"), mcp.Required()),
		mcp.WithString("value", mcp.Description("Knowledge to store"), mcp.Required()),
		mcp.WithArray("tags", mcp.Description("Tags for filtering"), mcp.WithStringItems()),
		mcp.WithString("scope",
			mcp.Description("agent (private) / project (shared) / global"),
			mcp.Enum("agent", "project", "global"),
		),
		mcp.WithString("confidence",
			mcp.Description("Provenance"),
			mcp.Enum("stated", "inferred", "observed"),
		),
		mcp.WithString("layer",
			mcp.Description("constraints (hard rules) / behavior (defaults) / context (ephemeral)"),
			mcp.Enum("constraints", "behavior", "context"),
		),
	)
}

func getMemoryTool() mcp.Tool {
	return mcp.NewTool(
		"get_memory",
		mcp.WithDescription("Get memory by key (cascades: agent → project → global). Returns all conflicting values if any."),
		asParam,
		projectParam,
		mcp.WithString("key", mcp.Description("Memory key"), mcp.Required()),
		mcp.WithString("scope",
			mcp.Description("Skip cascade, search specific scope"),
			mcp.Enum("agent", "project", "global"),
		),
	)
}

func searchMemoryTool() mcp.Tool {
	return mcp.NewTool(
		"search_memory",
		mcp.WithDescription("Full-text search across memories (cross-scope, respects privacy)."),
		asParam,
		projectParam,
		mcp.WithString("query", mcp.Description("Search query"), mcp.Required()),
		mcp.WithArray("tags", mcp.Description("Filter by tags"), mcp.WithStringItems()),
		mcp.WithString("scope",
			mcp.Description("Limit to scope"),
			mcp.Enum("agent", "project", "global"),
		),
		mcp.WithNumber("limit", mcp.Description("Max results to return (default: 20)")),
	)
}

func listMemoriesTool() mcp.Tool {
	return mcp.NewTool(
		"list_memories",
		mcp.WithDescription("Browse memories with filtering (key, truncated value, tags)."),
		asParam,
		projectParam,
		mcp.WithString("scope",
			mcp.Description("Filter by scope"),
			mcp.Enum("agent", "project", "global"),
		),
		mcp.WithArray("tags", mcp.Description("Filter by tags"), mcp.WithStringItems()),
		mcp.WithString("agent", mcp.Description("Filter by author")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 50)")),
	)
}

func deleteMemoryTool() mcp.Tool {
	return mcp.NewTool(
		"delete_memory",
		mcp.WithDescription("Soft-delete (archive) a memory."),
		asParam,
		projectParam,
		mcp.WithString("key", mcp.Description("Memory key to archive"), mcp.Required()),
		mcp.WithString("scope",
			mcp.Description("Scope to delete from"),
			mcp.Enum("agent", "project", "global"),
		),
	)
}

func resolveConflictTool() mcp.Tool {
	return mcp.NewTool(
		"resolve_conflict",
		mcp.WithDescription("Resolve a memory conflict. Rejected version is archived."),
		asParam,
		projectParam,
		mcp.WithString("key", mcp.Description("The conflicted memory key"), mcp.Required()),
		mcp.WithString("chosen_value", mcp.Description("The value to keep (can be one of the existing values or a new one)"), mcp.Required()),
		mcp.WithString("scope",
			mcp.Description("Scope where the conflict exists"),
			mcp.Enum("agent", "project", "global"),
		),
	)
}

// --- Profile tools ---

func registerProfileTool() mcp.Tool {
	return mcp.NewTool(
		"register_profile",
		mcp.WithDescription("Create/update a profile archetype (role + context pack + skills)."),
		projectParam,
		mcp.WithString("slug", mcp.Description("Profile identifier"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Display name"), mcp.Required()),
		mcp.WithString("role", mcp.Description("Role description")),
		mcp.WithString("context_pack", mcp.Description("Markdown: soul, skills, working style")),
		mcp.WithString("soul_keys", mcp.Description("Memory keys to load at boot (JSON array)")),
		mcp.WithString("skills", mcp.Description("Skill objects (JSON array)")),
		mcp.WithString("vault_paths", mcp.Description("Vault paths to auto-inject at boot (globs, {slug} resolved)")),
	)
}

func getProfileTool() mcp.Tool {
	return mcp.NewTool(
		"get_profile",
		mcp.WithDescription("Get a profile with full context pack."),
		projectParam,
		mcp.WithString("slug", mcp.Description("Profile slug to retrieve"), mcp.Required()),
	)
}

func listProfilesTool() mcp.Tool {
	return mcp.NewTool(
		"list_profiles",
		mcp.WithDescription("List all profiles."),
		projectParam,
	)
}

func findProfilesTool() mcp.Tool {
	return mcp.NewTool(
		"find_profiles",
		mcp.WithDescription("Find profiles by skill tag."),
		projectParam,
		mcp.WithString("skill_tag", mcp.Description("Skill tag to search for"), mcp.Required()),
	)
}

// --- Task tools ---

func dispatchTaskTool() mcp.Tool {
	return mcp.NewTool(
		"dispatch_task",
		mcp.WithDescription("Dispatch a task to a profile (pending → claim → start → complete). Use profile='human' for human actions. Auto-assigns to first board if none specified."),
		asParam,
		projectParam,
		mcp.WithString("profile", mcp.Description("Target profile slug ('human' for user tasks)"), mcp.Required()),
		mcp.WithString("title", mcp.Description("Task title"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Detailed task description")),
		mcp.WithString("priority",
			mcp.Description("Task priority"),
			mcp.Enum("P0", "P1", "P2", "P3"),
		),
		mcp.WithString("parent_task_id", mcp.Description("Parent task ID (subtask)")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("goal_id", mcp.Description("Linked goal ID")),
	)
}

func claimTaskTool() mcp.Tool {
	return mcp.NewTool(
		"claim_task",
		mcp.WithDescription("Claim a pending task (→ accepted)."),
		asParam,
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID to claim"), mcp.Required()),
	)
}

func startTaskTool() mcp.Tool {
	return mcp.NewTool(
		"start_task",
		mcp.WithDescription("Start a task (→ in-progress). Can skip accepted."),
		asParam,
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID to start"), mcp.Required()),
	)
}

func completeTaskTool() mcp.Tool {
	return mcp.NewTool(
		"complete_task",
		mcp.WithDescription("Complete a task (→ done). Notifies dispatcher."),
		asParam,
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID to complete"), mcp.Required()),
		mcp.WithString("result", mcp.Description("Task output/result")),
	)
}

func blockTaskTool() mcp.Tool {
	return mcp.NewTool(
		"block_task",
		mcp.WithDescription("Block a task with reason (→ blocked). Notifies dispatcher + parent chain."),
		asParam,
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID to block"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Why the task is blocked")),
	)
}

func cancelTaskTool() mcp.Tool {
	return mcp.NewTool(
		"cancel_task",
		mcp.WithDescription("Cancel a task (→ cancelled). Notifies dispatcher."),
		asParam,
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID to cancel"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Why the task is being cancelled")),
	)
}

func getTaskTool() mcp.Tool {
	return mcp.NewTool(
		"get_task",
		mcp.WithDescription("Get full task details (optionally with subtasks)."),
		projectParam,
		mcp.WithString("task_id", mcp.Description("Task ID"), mcp.Required()),
		mcp.WithBoolean("include_subtasks", mcp.Description("Include subtasks (max depth 3)")),
	)
}

func listTasksTool() mcp.Tool {
	return mcp.NewTool(
		"list_tasks",
		mcp.WithDescription("List tasks with filtering (sorted by priority)."),
		asParam,
		projectParam,
		mcp.WithString("status",
			mcp.Description("Filter by status"),
			mcp.Enum("pending", "accepted", "in-progress", "done", "blocked", "cancelled"),
		),
		mcp.WithString("profile", mcp.Description("Filter by profile slug")),
		mcp.WithString("priority",
			mcp.Description("Filter by priority"),
			mcp.Enum("P0", "P1", "P2", "P3"),
		),
		mcp.WithString("assigned_to", mcp.Description("Filter by assigned agent")),
		mcp.WithString("board_id", mcp.Description("Filter by board ID")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 50)")),
	)
}

// --- Boards ---

func createBoardTool() mcp.Tool {
	return mcp.NewTool(
		"create_board",
		mcp.WithDescription("Create a task board."),
		asParam,
		projectParam,
		mcp.WithString("name", mcp.Description("Board display name"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Board slug (unique per project)"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Board description")),
	)
}

func listBoardsTool() mcp.Tool {
	return mcp.NewTool(
		"list_boards",
		mcp.WithDescription("List task boards."),
		projectParam,
	)
}

func archiveBoardTool() mcp.Tool {
	return mcp.NewTool(
		"archive_board",
		mcp.WithDescription("Archive a board and its tasks (hidden, data preserved)."),
		asParam,
		projectParam,
		mcp.WithString("board_id", mcp.Description("Board ID to archive"), mcp.Required()),
	)
}

func deleteBoardTool() mcp.Tool {
	return mcp.NewTool(
		"delete_board",
		mcp.WithDescription("Delete an archived board (must archive first). Tasks kept."),
		asParam,
		projectParam,
		mcp.WithString("board_id", mcp.Description("Board ID (must be archived)"), mcp.Required()),
	)
}

func archiveTasksTool() mcp.Tool {
	return mcp.NewTool(
		"archive_tasks",
		mcp.WithDescription("Archive done/cancelled tasks (soft-delete from listings)."),
		asParam,
		projectParam,
		mcp.WithString("status", mcp.Description("'done', 'cancelled', or empty for both"), mcp.Enum("done", "cancelled", "")),
		mcp.WithString("board_id", mcp.Description("Limit to board (empty = all)")),
	)
}

// --- Goals ---

func createGoalTool() mcp.Tool {
	return mcp.NewTool(
		"create_goal",
		mcp.WithDescription("Create a goal in the cascade (mission → project_goal → agent_goal). Link tasks via goal_id. Progress = linked task completion."),
		asParam,
		projectParam,
		mcp.WithString("type",
			mcp.Description("Goal level in the cascade"),
			mcp.Enum("mission", "project_goal", "agent_goal"),
			mcp.Required(),
		),
		mcp.WithString("title", mcp.Description("Goal title"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Goal description")),
		mcp.WithString("parent_goal_id", mcp.Description("Parent goal ID")),
		mcp.WithString("owner_agent", mcp.Description("Owner agent name")),
	)
}

func listGoalsTool() mcp.Tool {
	return mcp.NewTool(
		"list_goals",
		mcp.WithDescription("List goals with progress."),
		projectParam,
		mcp.WithString("type",
			mcp.Description("Filter by goal type"),
			mcp.Enum("mission", "project_goal", "agent_goal"),
		),
		mcp.WithString("status",
			mcp.Description("Filter by status"),
			mcp.Enum("active", "completed", "paused"),
		),
		mcp.WithString("owner_agent", mcp.Description("Filter by owner")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 50)")),
	)
}

func getGoalTool() mcp.Tool {
	return mcp.NewTool(
		"get_goal",
		mcp.WithDescription("Get goal with ancestry, progress, and children."),
		projectParam,
		mcp.WithString("goal_id", mcp.Description("Goal ID"), mcp.Required()),
	)
}

func updateGoalTool() mcp.Tool {
	return mcp.NewTool(
		"update_goal",
		mcp.WithDescription("Update a goal's title, description, or status."),
		asParam,
		projectParam,
		mcp.WithString("goal_id", mcp.Description("Goal ID to update"), mcp.Required()),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("status",
			mcp.Description("New status"),
			mcp.Enum("active", "completed", "paused"),
		),
	)
}

func getGoalCascadeTool() mcp.Tool {
	return mcp.NewTool(
		"get_goal_cascade",
		mcp.WithDescription("Get full goal hierarchy tree with progress."),
		projectParam,
	)
}

// --- Vault tools ---

func registerVaultTool() mcp.Tool {
	return mcp.NewTool(
		"register_vault",
		mcp.WithDescription("Register a vault (markdown docs folder). Indexes .md files, watches for changes. One per project, re-register replaces."),
		projectParam,
		mcp.WithString("path", mcp.Description("Absolute path to vault directory"), mcp.Required()),
	)
}

func searchVaultTool() mcp.Tool {
	return mcp.NewTool(
		"search_vault",
		mcp.WithDescription("Search vault docs (FTS5). Use get_vault_doc for full content."),
		projectParam,
		mcp.WithString("query", mcp.Description("FTS5 query (words, OR, \"phrases\")"), mcp.Required()),
		mcp.WithString("tags", mcp.Description("JSON array of tags")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 10)")),
	)
}

func getVaultDocTool() mcp.Tool {
	return mcp.NewTool(
		"get_vault_doc",
		mcp.WithDescription("Get full vault document content by path."),
		projectParam,
		mcp.WithString("path", mcp.Description("Path relative to vault root"), mcp.Required()),
	)
}

func listVaultDocsTool() mcp.Tool {
	return mcp.NewTool(
		"list_vault_docs",
		mcp.WithDescription("List vault documents (metadata only)."),
		projectParam,
		mcp.WithString("tags", mcp.Description("JSON array of tags")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 100)")),
	)
}

// --- File locks ---

func claimFilesTool() mcp.Tool {
	return mcp.NewTool(
		"claim_files",
		mcp.WithDescription("Claim files you're editing (broadcasts lock to all agents)."),
		asParam,
		projectParam,
		mcp.WithString("file_paths", mcp.Description("JSON array of file paths"), mcp.Required()),
		mcp.WithNumber("ttl_seconds", mcp.Description("Claim duration (default: 1800)")),
	)
}

func releaseFilesTool() mcp.Tool {
	return mcp.NewTool(
		"release_files",
		mcp.WithDescription("Release claimed files."),
		asParam,
		projectParam,
		mcp.WithString("file_paths", mcp.Description("JSON array of file paths"), mcp.Required()),
	)
}

func listLocksTool() mcp.Tool {
	return mcp.NewTool(
		"list_locks",
		mcp.WithDescription("List active file locks."),
		projectParam,
	)
}

// --- Agent lifecycle ---

func deactivateAgentTool() mcp.Tool {
	return mcp.NewTool(
		"deactivate_agent",
		mcp.WithDescription("Permanently deactivate an agent. Re-register to restore. For temporary pause, use sleep_agent."),
		projectParam,
		mcp.WithString("name", mcp.Description("Agent name to deactivate"), mcp.Required()),
	)
}

func deleteAgentTool() mcp.Tool {
	return mcp.NewTool(
		"delete_agent",
		mcp.WithDescription("Soft-delete an agent (hidden from UI, re-register to restore)."),
		projectParam,
		mcp.WithString("name", mcp.Description("Agent name to delete"), mcp.Required()),
	)
}

func sleepAgentTool() mcp.Tool {
	return mcp.NewTool(
		"sleep_agent",
		mcp.WithDescription("Sleep agent (status='sleeping', messages queued). Re-register to wake."),
		asParam,
		projectParam,
	)
}

// --- Project lifecycle ---

func deleteProjectTool() mcp.Tool {
	return mcp.NewTool(
		"delete_project",
		mcp.WithDescription("IRREVERSIBLE: Delete a project and ALL its data."),
		mcp.WithString("project", mcp.Description("Project name to delete"), mcp.Required()),
	)
}

// --- Project onboarding ---

func createProjectTool() mcp.Tool {
	return mcp.NewTool(
		"create_project",
		mcp.WithDescription("Create a project. Returns an onboarding plan to execute (org, vault, profiles, goals, board setup)."),
		mcp.WithString("name", mcp.Description("Project name (lowercase, no spaces)"), mcp.Required()),
		mcp.WithString("description", mcp.Description("One-line project description")),
		mcp.WithString("cwd", mcp.Description("Project root path (for vault)")),
		mcp.WithBoolean("interactive", mcp.Description("Wait for approval at each phase (default: false)")),
	)
}

// --- Soul RAG ---

func queryContextTool() mcp.Tool {
	return mcp.NewTool(
		"query_context",
		mcp.WithDescription("Query relevant memories and task results for context loading."),
		asParam,
		projectParam,
		mcp.WithString("query", mcp.Description("Context query"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max results (default: 10)")),
	)
}

// --- Session context ---

func getSessionContextTool() mcp.Tool {
	return mcp.NewTool(
		"get_session_context",
		mcp.WithDescription("Boot context: profile, tasks, unread messages (index), conversations, memories (index). One call replaces 5-8."),
		asParam,
		projectParam,
		mcp.WithString("profile_slug", mcp.Description("Profile slug (auto-detected if omitted)")),
	)
}

// --- Teams + Orgs tools ---

func createOrgTool() mcp.Tool {
	return mcp.NewTool(
		"create_org",
		mcp.WithDescription("Create an organization (groups teams)."),
		asParam,
		projectParam,
		mcp.WithString("name", mcp.Description("Organization name"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Unique slug (e.g. 'acme-corp')"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Organization description")),
	)
}

func listOrgsTool() mcp.Tool {
	return mcp.NewTool(
		"list_orgs",
		mcp.WithDescription("List all organizations."),
		asParam,
		projectParam,
	)
}

func createTeamTool() mcp.Tool {
	return mcp.NewTool(
		"create_team",
		mcp.WithDescription("Create a team (controls messaging permissions). Types: regular, admin (broadcast), bot."),
		asParam,
		projectParam,
		mcp.WithString("name", mcp.Description("Team name"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Team slug (unique per project)"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Team description")),
		mcp.WithString("type", mcp.Description("regular (default) / admin / bot")),
		mcp.WithString("org_id", mcp.Description("Organization ID")),
		mcp.WithString("parent_team_id", mcp.Description("Parent team ID")),
	)
}

func listTeamsTool() mcp.Tool {
	return mcp.NewTool(
		"list_teams",
		mcp.WithDescription("List teams with members."),
		asParam,
		projectParam,
	)
}

func addTeamMemberTool() mcp.Tool {
	return mcp.NewTool(
		"add_team_member",
		mcp.WithDescription("Add agent to team."),
		asParam,
		projectParam,
		mcp.WithString("team", mcp.Description("Team slug"), mcp.Required()),
		mcp.WithString("agent_name", mcp.Description("Agent name to add"), mcp.Required()),
		mcp.WithString("role", mcp.Description("admin / lead / member (default) / observer")),
	)
}

func removeTeamMemberTool() mcp.Tool {
	return mcp.NewTool(
		"remove_team_member",
		mcp.WithDescription("Remove agent from team."),
		asParam,
		projectParam,
		mcp.WithString("team", mcp.Description("Team slug"), mcp.Required()),
		mcp.WithString("agent_name", mcp.Description("Agent name to remove"), mcp.Required()),
	)
}

func getTeamInboxTool() mcp.Tool {
	return mcp.NewTool(
		"get_team_inbox",
		mcp.WithDescription("Get team inbox messages."),
		asParam,
		projectParam,
		mcp.WithString("team", mcp.Description("Team slug"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max messages (default: 50)")),
	)
}

func addNotifyChannelTool() mcp.Tool {
	return mcp.NewTool(
		"add_notify_channel",
		mcp.WithDescription("Allow messaging target agent outside team boundaries."),
		asParam,
		projectParam,
		mcp.WithString("target", mcp.Description("Target agent name"), mcp.Required()),
	)
}
