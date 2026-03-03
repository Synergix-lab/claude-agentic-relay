package relay

import "github.com/mark3labs/mcp-go/mcp"

// asParam is added to every tool that uses agent identity.
var asParam = mcp.WithString("as", mcp.Description("Act as this agent (overrides the default identity from the connection URL). Use this when managing multiple agents from a single session."))

// projectParam is added to every tool that needs project scoping.
// It allows overriding the default ?project= from the URL,
// so agents can switch projects without changing the MCP connection.
var projectParam = mcp.WithString("project", mcp.Description("Project namespace (overrides the default from the connection URL). Agents, messages, and conversations are isolated per project."))

func registerAgentTool() mcp.Tool {
	return mcp.NewTool(
		"register_agent",
		mcp.WithDescription("Register an agent with the relay. Call this once per agent at startup to announce their presence."),
		projectParam,
		mcp.WithString("name", mcp.Description("Unique agent name (e.g. 'backend', 'frontend')"), mcp.Required()),
		mcp.WithString("role", mcp.Description("Agent role description (e.g. 'FastAPI backend developer')")),
		mcp.WithString("description", mcp.Description("What this agent is currently working on")),
		mcp.WithString("reports_to", mcp.Description("Name of the agent this one reports to (for org hierarchy)")),
	)
}

func sendMessageTool() mcp.Tool {
	return mcp.NewTool(
		"send_message",
		mcp.WithDescription("Send a message to another agent. Use '*' as recipient for broadcast. Set conversation_id to send to a conversation (all members will see it)."),
		asParam,
		projectParam,
		mcp.WithString("to", mcp.Description("Recipient agent name, or '*' for broadcast. Ignored when conversation_id is set."), mcp.Required()),
		mcp.WithString("type",
			mcp.Description("Message type"),
			mcp.Enum("question", "response", "notification", "code-snippet", "task", "user_question"),
		),
		mcp.WithString("subject", mcp.Description("Message subject line"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Message body content"), mcp.Required()),
		mcp.WithString("reply_to", mcp.Description("Message ID to reply to (for threading)")),
		mcp.WithString("metadata", mcp.Description("JSON string of additional metadata")),
		mcp.WithString("conversation_id", mcp.Description("Send message to a conversation instead of a single agent")),
	)
}

func getInboxTool() mcp.Tool {
	return mcp.NewTool(
		"get_inbox",
		mcp.WithDescription("Get messages from an agent's inbox. Returns messages sent to them or broadcast (excluding their own broadcasts)."),
		asParam,
		projectParam,
		mcp.WithBoolean("unread_only", mcp.Description("Only return unread messages (default: true)")),
		mcp.WithNumber("limit", mcp.Description("Max number of messages to return (default: 10). Content is truncated to 300 chars — use get_thread for full messages.")),
	)
}

func getThreadTool() mcp.Tool {
	return mcp.NewTool(
		"get_thread",
		mcp.WithDescription("Get a complete thread of messages starting from any message in the thread."),
		projectParam,
		mcp.WithString("message_id", mcp.Description("Any message ID in the thread"), mcp.Required()),
	)
}

func listAgentsTool() mcp.Tool {
	return mcp.NewTool(
		"list_agents",
		mcp.WithDescription("List all registered agents and their status."),
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
		mcp.WithString("conversation_id", mcp.Description("Mark all messages in a conversation as read (alternative to message_ids)")),
	)
}

func createConversationTool() mcp.Tool {
	return mcp.NewTool(
		"create_conversation",
		mcp.WithDescription("Create a multi-agent conversation. All members will see messages sent to it."),
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
		mcp.WithDescription("List conversations you are a member of, with unread counts."),
		asParam,
		projectParam,
	)
}

func getConversationMessagesTool() mcp.Tool {
	return mcp.NewTool(
		"get_conversation_messages",
		mcp.WithDescription("Get messages from a conversation, ordered chronologically."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max number of messages to return (default: 50)")),
	)
}

func inviteToConversationTool() mcp.Tool {
	return mcp.NewTool(
		"invite_to_conversation",
		mcp.WithDescription("Add an agent to an existing conversation."),
		asParam,
		projectParam,
		mcp.WithString("conversation_id", mcp.Description("The conversation ID"), mcp.Required()),
		mcp.WithString("agent_name", mcp.Description("Agent name to invite"), mcp.Required()),
	)
}
