package cli

import (
	"fmt"
	"os"
)

func runInbox(args []string) {
	project, rest := parseProject(args)

	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: agent-relay inbox [-p project] <agent>")
		os.Exit(1)
	}

	agent := rest[0]
	d := openDB()
	defer func() { _ = d.Close() }()

	messages, err := d.GetInbox(project, agent, true, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		fmt.Printf("no unread messages for %s\n", agent)
		return
	}

	fmt.Printf("%d unread:\n", len(messages))
	for _, m := range messages {
		fmt.Printf("  [%s] %s → %q  (%s)  id:%s\n",
			m.Type,
			m.From,
			truncate(m.Subject, 50),
			timeAgo(m.CreatedAt),
			m.ID[:8],
		)
	}
}
