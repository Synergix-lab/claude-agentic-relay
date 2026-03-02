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
	case "status":
		runStatus()
	case "agents":
		runAgents()
	case "inbox":
		runInbox(rest)
	case "send":
		runSend(rest)
	case "thread":
		runThread(rest)
	case "stats":
		runStats()
	case "conversations":
		runConversations(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
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
	fmt.Print(`usage: agent-relay <command>

commands:
  serve               start the relay server
  status              relay status & summary
  agents              list registered agents
  inbox <agent>       show unread messages for agent
  send <from> <to> <msg>  send a message
  thread <id>         show full message thread
  conversations <agent>  list conversations for agent
  stats               global statistics

flags:
  --version           show version
  --help              show this help
`)
}
