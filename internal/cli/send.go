package cli

import (
	"fmt"
	"os"
	"strings"
)

func runSend(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: agent-relay send <from> <to> <message>")
		os.Exit(1)
	}

	from := args[0]
	to := args[1]
	content := strings.Join(args[2:], " ")

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

	msg, err := d.InsertMessage(from, to, msgType, subject, content, "{}", nil, nil)
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
