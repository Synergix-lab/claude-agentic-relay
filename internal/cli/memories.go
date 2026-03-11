package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

func runMemories(args []string) {
	project, args := parseProject(args)

	// Parse flags
	var searchQuery, agentFilter, tagFilter string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--search":
			if i+1 < len(args) {
				searchQuery = args[i+1]
				i++
			}
		case "-a", "--agent":
			if i+1 < len(args) {
				agentFilter = args[i+1]
				i++
			}
		case "-t", "--tag":
			if i+1 < len(args) {
				tagFilter = args[i+1]
				i++
			}
		}
	}

	d := openDB()
	defer func() { _ = d.Close() }()

	// Show stats header
	total, conflicts, err := d.MemoryStats(project)
	if err == nil && total > 0 {
		header := fmt.Sprintf("%d memories", total)
		if conflicts > 0 {
			header += fmt.Sprintf(" (%d conflicts)", conflicts)
		}
		if project != "default" {
			header += fmt.Sprintf(" [%s]", project)
		}
		fmt.Println(bold(header))
		fmt.Println()
	}

	var tags []string
	if tagFilter != "" {
		tags = []string{tagFilter}
	}

	var memories []struct {
		Key        string
		Value      string
		Scope      string
		Agent      string
		Confidence string
		Version    int
		Tags       string
		UpdatedAt  string
		Conflict   bool
	}

	if searchQuery != "" {
		// Full-text search
		mems, err := d.SearchMemory(project, "", searchQuery, tags, "", 50)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for _, m := range mems {
			memories = append(memories, struct {
				Key        string
				Value      string
				Scope      string
				Agent      string
				Confidence string
				Version    int
				Tags       string
				UpdatedAt  string
				Conflict   bool
			}{m.Key, m.Value, m.Scope, m.AgentName, m.Confidence, m.Version, m.Tags, m.UpdatedAt, m.ConflictWith != nil})
		}
	} else {
		// List with filters
		mems, err := d.ListMemories(project, "", agentFilter, tags, 50)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for _, m := range mems {
			memories = append(memories, struct {
				Key        string
				Value      string
				Scope      string
				Agent      string
				Confidence string
				Version    int
				Tags       string
				UpdatedAt  string
				Conflict   bool
			}{m.Key, m.Value, m.Scope, m.AgentName, m.Confidence, m.Version, m.Tags, m.UpdatedAt, m.ConflictWith != nil})
		}
	}

	if len(memories) == 0 {
		fmt.Println(dim("No memories found."))
		return
	}

	rows := [][]string{{"KEY", "SCOPE", "AGENT", "VALUE", "TAGS", "AGE"}}
	for _, m := range memories {
		val := m.Value
		if len(val) > 40 {
			val = val[:37] + "..."
		}

		key := m.Key
		if m.Conflict {
			key = key + " !"
		}

		tagsStr := ""
		var tagSlice []string
		_ = json.Unmarshal([]byte(m.Tags), &tagSlice)
		if len(tagSlice) > 0 {
			for i, t := range tagSlice {
				if i > 0 {
					tagsStr += ","
				}
				tagsStr += t
			}
		}

		rows = append(rows, []string{
			truncate(key, 24),
			m.Scope,
			truncate(m.Agent, 14),
			val,
			truncate(tagsStr, 16),
			timeAgo(m.UpdatedAt),
		})
	}

	printTable(rows)
}
