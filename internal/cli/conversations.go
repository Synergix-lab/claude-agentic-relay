package cli

import (
	"fmt"
	"os"
)

func runConversations(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: agent-relay conversations <agent>")
		os.Exit(1)
	}

	agentName := args[0]

	d := openDB()
	defer d.Close()

	convs, err := d.ListConversations(agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(convs) == 0 {
		fmt.Printf("No conversations for %s\n", agentName)
		return
	}

	rows := [][]string{{"ID", "TITLE", "MEMBERS", "UNREAD", "CREATED"}}
	for _, c := range convs {
		rows = append(rows, []string{
			c.ID[:8],
			truncate(c.Title, 30),
			fmt.Sprintf("%d", c.MemberCount),
			fmt.Sprintf("%d", c.UnreadCount),
			timeAgo(c.CreatedAt),
		})
	}

	printTable(rows)
}
