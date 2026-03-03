package cli

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func runStatus() {
	port := "8090"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	running := isListening(port)

	if running {
		fmt.Printf("relay: %s (:%s)\n", bold("running"), port)
	} else {
		fmt.Printf("relay: %s\n", "stopped")
	}

	d, err := openDBSafe()
	if err != nil {
		fmt.Println("db: not found")
		return
	}
	defer d.Close()

	stats, err := d.GetGlobalStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stats: %v\n", err)
		return
	}

	agents, _ := d.ListAgents("default")
	if len(agents) > 0 {
		var online, offline []string
		for _, a := range agents {
			if isOnline(a.LastSeen) {
				online = append(online, a.Name)
			} else {
				offline = append(offline, a.Name)
			}
		}

		parts := []string{}
		if len(online) > 0 {
			parts = append(parts, fmt.Sprintf("%d online (%s)", len(online), strings.Join(online, ", ")))
		}
		if len(offline) > 0 {
			parts = append(parts, fmt.Sprintf("%d offline", len(offline)))
		}
		fmt.Printf("agents: %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Printf("agents: 0\n")
	}

	fmt.Printf("unread: %d messages\n", stats.Unread)

	// Show projects
	projects, _ := d.ListProjects()
	if len(projects) > 0 {
		fmt.Printf("projects: %s\n", strings.Join(projects, ", "))
	}
}

func isListening(port string) bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
