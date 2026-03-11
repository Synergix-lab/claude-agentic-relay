package cli

import (
	"fmt"
	"os"
	"strings"
)

func runThread(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: agent-relay thread <message-id>")
		os.Exit(1)
	}

	messageID := args[0]
	d := openDB()
	defer func() { _ = d.Close() }()

	// Support short ID prefixes (e.g. "75cd52c8" instead of full UUID).
	if !strings.Contains(messageID, "-") || len(messageID) < 36 {
		fullID, err := d.FindMessageByPrefix(messageID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		messageID = fullID
	}

	messages, err := d.GetThread(messageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		fmt.Println("thread not found")
		return
	}

	fmt.Printf("thread: %d messages\n\n", len(messages))
	for _, m := range messages {
		fmt.Printf("  %s %s → %s  [%s]  (%s)\n", dim(m.ID[:8]), bold(m.From), m.To, m.Type, timeAgo(m.CreatedAt))
		fmt.Printf("  %s: %s\n", m.Subject, m.Content)
		fmt.Println()
	}
}
