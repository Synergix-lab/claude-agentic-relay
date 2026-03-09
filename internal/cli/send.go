package cli

import (
	"fmt"
	"os"
	"strings"
)

func runSend(args []string) {
	project, rest := parseProject(args)

	if len(rest) < 3 {
		fmt.Fprintln(os.Stderr, "usage: agent-relay send [-p project] <from> <to> <message>")
		os.Exit(1)
	}

	from := rest[0]
	to := rest[1]
	content := strings.Join(rest[2:], " ")

	// Derive subject from first few words.
	subject := deriveSubject(content)

	// Detect type: question if ends with "?"
	msgType := "notification"
	if strings.HasSuffix(strings.TrimSpace(content), "?") {
		msgType = "question"
	}

	// Write directly to DB (WAL supports concurrent readers + single writer).
	d := openDBReadWrite()
	defer d.Close()

	msg, err := d.InsertMessage(project, from, to, msgType, subject, content, "{}", "P2", nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ok → %s (id:%s)\n", to, msg.ID[:8])
}

// deriveSubject takes the first ~5 words of a message as a subject line.
func deriveSubject(content string) string {
	words := strings.Fields(content)
	if len(words) <= 5 {
		return content
	}
	return strings.Join(words[:5], " ") + "..."
}
