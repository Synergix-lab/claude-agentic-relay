package cli

import (
	"fmt"
	"os"

	"agent-relay/internal/db"
)

// Run dispatches CLI subcommands. args[0] is the subcommand name.
func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "help":
		printUsage()
	case "init":
		runInit(rest)
	case "status":
		runStatus()
	case "agents":
		runAgents(rest)
	case "inbox":
		runInbox(rest)
	case "send":
		runSend(rest)
	case "thread":
		runThread(rest)
	case "stats":
		runStats(rest)
	case "conversations":
		runConversations(rest)
	case "memories":
		runMemories(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// parseProject extracts -p <value> from args. Returns (project, remaining args).
func parseProject(args []string) (string, []string) {
	project := "default"
	var rest []string
	for i := 0; i < len(args); i++ {
		if (args[i] == "-p" || args[i] == "--project") && i+1 < len(args) {
			project = args[i+1]
			i++ // skip value
		} else {
			rest = append(rest, args[i])
		}
	}
	return project, rest
}

// openDB opens the database in read-only mode, printing a user-friendly
// error and exiting if the DB doesn't exist.
func openDB() *db.DB {
	d, err := db.NewReadOnly()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return d
}

// openDBSafe opens the database in read-only mode, returning an error
// instead of exiting (for commands that can degrade gracefully).
func openDBSafe() (*db.DB, error) {
	return db.NewReadOnly()
}

// openDBReadWrite opens the database in read-write mode (for send).
func openDBReadWrite() *db.DB {
	d, err := db.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return d
}

func printUsage() {
	fmt.Print(`usage: agent-relay <command> [-p <project>]

commands:
  init [project] [flags]       create .mcp.json for Claude Code
    --global                   write to ~/.claude/.mcp.json instead
    --port <port>              relay port (default: 8090)
    --host <host>              relay host (default: localhost)
  serve                        start the relay server
  status                       relay status & summary
  agents [-p project]          list registered agents
  inbox [-p project] <agent>   show unread messages for agent
  send [-p project] <from> <to> <msg>  send a message
  thread <id>                  show full message thread
  conversations [-p project] <agent>  list conversations for agent
  stats [-p project]           global statistics
  memories [-p project]        list persistent memories
    -s <query>                 search memories (FTS5)
    -a <agent>                 filter by agent
    -t <tag>                   filter by tag

flags:
  -p, --project <name>   filter by project (default: "default")
  --version              show version
  --help                 show this help
`)
}
