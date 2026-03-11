package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var isTTY = isTerminal()

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ANSI helpers — only emit codes when stdout is a terminal.

func bold(s string) string {
	if !isTTY {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func dim(s string) string {
	if !isTTY {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

// timeAgo formats an RFC3339 timestamp as a human-readable relative time.
func timeAgo(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		// try the microsecond format used by InsertMessage
		t, err = time.Parse("2006-01-02T15:04:05.000000Z", rfc3339)
		if err != nil {
			return rfc3339
		}
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// StaleThreshold is the duration after which an agent is considered offline.
const StaleThreshold = 5 * time.Minute

// isOnline returns true if the agent's last_seen is within the stale threshold.
func isOnline(rfc3339 string) bool {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000000Z", rfc3339)
		if err != nil {
			return false
		}
	}
	return time.Since(t) < StaleThreshold
}

func statusIndicator(rfc3339 string) string {
	if isOnline(rfc3339) {
		if isTTY {
			return "\033[32m●\033[0m" // green dot
		}
		return "online"
	}
	if isTTY {
		return "\033[90m○\033[0m" // dim circle
	}
	return "offline"
}

// truncate shortens a string to n characters, adding "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// printTable prints rows as an aligned table.
// Each row is a slice of strings; the first row is treated as the header.
func printTable(rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Compute column widths.
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if i < cols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header.
	header := rows[0]
	parts := make([]string, cols)
	for i, cell := range header {
		parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
	}
	fmt.Println(bold(strings.Join(parts, "  ")))

	// Print data rows.
	for _, row := range rows[1:] {
		parts := make([]string, cols)
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
		fmt.Println(strings.Join(parts, "  "))
	}
}
