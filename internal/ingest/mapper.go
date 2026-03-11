package ingest

var toolActivityMap = map[string]Activity{
	// Typing — writing/editing files
	"write_file":   ActivityTyping,
	"edit_file":    ActivityTyping,
	"str_replace":  ActivityTyping,
	"create_file":  ActivityTyping,
	"Write":        ActivityTyping,
	"Edit":         ActivityTyping,
	"NotebookEdit": ActivityTyping,

	// Reading — reading/searching files
	"read_file":    ActivityReading,
	"search_files": ActivityReading,
	"list_files":   ActivityReading,
	"Read":         ActivityReading,
	"Glob":         ActivityReading,
	"Grep":         ActivityReading,
	"LSP":          ActivityReading,

	// Terminal — shell commands
	"bash":     ActivityTerminal,
	"terminal": ActivityTerminal,
	"Bash":     ActivityTerminal,

	// Browsing — web access
	"web_search": ActivityBrowsing,
	"web_fetch":  ActivityBrowsing,
	"WebSearch":  ActivityBrowsing,
	"WebFetch":   ActivityBrowsing,

	// Thinking — meta/orchestration tools
	"ToolSearch":      ActivityThinking,
	"Agent":           ActivityThinking,
	"AskUserQuestion": ActivityWaiting,
	"Skill":           ActivityThinking,
	"EnterPlanMode":   ActivityThinking,
	"ExitPlanMode":    ActivityThinking,
	"EnterWorktree":   ActivityThinking,
	"TaskCreate":      ActivityThinking,
	"TaskUpdate":      ActivityThinking,
	"TaskGet":         ActivityThinking,
	"TaskList":        ActivityThinking,
	"TaskOutput":      ActivityReading,
	"TaskStop":        ActivityTerminal,
}

// MapToolToActivity returns the activity for a known tool.
// Unknown tools (including MCP tools) return "thinking" — an active tool is never idle.
func MapToolToActivity(toolName string) Activity {
	if a, ok := toolActivityMap[toolName]; ok {
		return a
	}
	return ActivityThinking
}
