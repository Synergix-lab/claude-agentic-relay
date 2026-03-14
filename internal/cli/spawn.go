package cli

import (
	"fmt"
	"strings"
	"time"
)

func runChildren(args []string) {
	project, rest := parseProject(args)

	// Parse optional flags
	status := "all"
	agent := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "-s", "--status":
			if i+1 < len(rest) {
				status = rest[i+1]
				i++
			}
		default:
			if agent == "" {
				agent = rest[i]
			}
		}
	}

	d := openDB()
	defer func() { _ = d.Close() }()

	children := d.ListSpawnChildren(agent, project, status)

	if len(children) == 0 {
		fmt.Println("no spawned children found")
		return
	}

	fmt.Printf("%s children in %s:\n\n", bold(fmt.Sprintf("%d", len(children))), project)

	for _, c := range children {
		id, _ := c["id"].(string)
		profile, _ := c["profile"].(string)
		st, _ := c["status"].(string)
		parent, _ := c["parent_agent"].(string)
		startedAt, _ := c["started_at"].(string)

		shortID := id
		if len(id) > 8 {
			shortID = id[:8]
		}

		statusIcon := "⏳"
		switch st {
		case "finished":
			statusIcon = "✓"
		case "killed":
			statusIcon = "✗"
		}

		duration := ""
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			duration = time.Since(t).Truncate(time.Second).String()
		}

		fmt.Printf("  %s %s  %s  parent=%s  %s  %s\n",
			statusIcon, shortID, bold(profile), parent, st, duration)

		if errMsg, ok := c["error"].(string); ok && errMsg != "" {
			fmt.Printf("    error: %s\n", errMsg)
		}
	}
}

func runSchedules(args []string) {
	project, rest := parseProject(args)

	agent := ""
	if len(rest) > 0 {
		agent = rest[0]
	}

	d := openDB()
	defer func() { _ = d.Close() }()

	var schedules []map[string]any
	if agent != "" {
		schedules = d.ListSchedulesByAgent(project, agent)
	} else {
		schedules = d.ListSchedulesByProject(project)
	}

	if len(schedules) == 0 {
		fmt.Println("no schedules found")
		return
	}

	fmt.Printf("%s schedules in %s:\n\n", bold(fmt.Sprintf("%d", len(schedules))), project)

	for _, s := range schedules {
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		agentName, _ := s["agent_name"].(string)
		cronExpr, _ := s["cron_expr"].(string)
		ttl, _ := s["ttl"].(string)
		enabled, _ := s["enabled"].(bool)

		shortID := id
		if len(id) > 8 {
			shortID = id[:8]
		}

		enabledStr := "enabled"
		if !enabled {
			enabledStr = "disabled"
		}

		fmt.Printf("  %s  %s  agent=%s  cron=%q  ttl=%s  %s\n",
			shortID, bold(name), agentName, cronExpr, ttl, enabledStr)
	}
}

func runHistory(args []string) {
	project, rest := parseProject(args)

	agent := ""
	if len(rest) > 0 {
		agent = rest[0]
	}

	d := openDB()
	defer func() { _ = d.Close() }()

	history := d.GetCycleHistory(project, agent, 20)

	if len(history) == 0 {
		fmt.Println("no cycle history found")
		return
	}

	fmt.Printf("last %d cycles in %s:\n\n", len(history), project)

	for _, h := range history {
		agentName, _ := h["agent_name"].(string)
		cycle, _ := h["cycle_name"].(string)
		durationMs, _ := h["duration_ms"].(int64)
		success, _ := h["success"].(bool)
		createdAt, _ := h["created_at"].(string)
		inputTokens, _ := h["input_tokens"].(int64)
		outputTokens, _ := h["output_tokens"].(int64)

		icon := "✓"
		if !success {
			icon = "✗"
		}

		// Trim timestamp
		ts := createdAt
		if len(ts) > 19 {
			ts = ts[:19]
		}

		duration := time.Duration(durationMs) * time.Millisecond

		parts := []string{
			fmt.Sprintf("%s %s", icon, bold(agentName)),
			cycle,
			duration.Truncate(time.Second).String(),
		}

		if inputTokens > 0 || outputTokens > 0 {
			parts = append(parts, fmt.Sprintf("tokens=%d/%d", inputTokens, outputTokens))
		}

		fmt.Printf("  %s  %s\n", ts, strings.Join(parts, "  "))

		if errMsg, ok := h["error"].(string); ok && errMsg != "" {
			fmt.Printf("    error: %s\n", errMsg)
		}
	}
}
