package cli

import (
	"fmt"
	"os"
	"time"
)

func runStats(args []string) {
	project, _ := parseProject(args)

	d := openDB()
	defer d.Close()

	stats, err := d.GetStats(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if project != "default" {
		fmt.Printf("project: %s\n", project)
	}

	// Uptime from oldest agent registration.
	if stats.OldestAgent != "" {
		fmt.Printf("uptime: %s\n", uptimeSince(stats.OldestAgent))
	}

	fmt.Printf("agents: %d registered\n", stats.Agents)
	fmt.Printf("messages: %d total, %d unread\n", stats.Messages, stats.Unread)
	fmt.Printf("threads: %d\n", stats.Threads)
}

func uptimeSince(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000000Z", rfc3339)
		if err != nil {
			return "unknown"
		}
	}

	d := time.Since(t)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
