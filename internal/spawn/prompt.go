package spawn

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-relay/internal/db"
)

// SpawnContext is the fully-assembled context object that a spawned agent receives.
// The agent opens its eyes knowing everything — zero boot calls needed.
type SpawnContext struct {
	Identity  *ContextIdentity  `json:"identity"`
	Task      *ContextTask      `json:"task,omitempty"`
	Knowledge *ContextKnowledge `json:"knowledge"`
	Team      *ContextTeam      `json:"team,omitempty"`
	Inbox     []ContextMessage  `json:"inbox,omitempty"`
	Rules     *ContextRules     `json:"rules"`
}

type ContextIdentity struct {
	Profile     string `json:"profile"`
	Project     string `json:"project"`
	Role        string `json:"role"`
	ContextPack string `json:"context_pack,omitempty"`
	ReportsTo   string `json:"reports_to,omitempty"`
}

type ContextTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Acceptance  string `json:"acceptance,omitempty"`
	Priority    string `json:"priority"`
}

type ContextKnowledge struct {
	Conventions []string `json:"conventions,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
	Lessons     []string `json:"lessons,omitempty"`
	LastCycle   string   `json:"last_cycle,omitempty"`
}

type ContextTeam struct {
	Agents       []ContextAgent `json:"agents,omitempty"`
	PendingTasks int            `json:"pending_tasks"`
	BlockedTasks int            `json:"blocked_tasks"`
}

type ContextAgent struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
}

type ContextMessage struct {
	From     string `json:"from"`
	Subject  string `json:"subject"`
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"`
}

type ContextRules struct {
	Cycle string `json:"cycle,omitempty"`
}

// SpawnMode controls how much context is injected into the agent prompt.
type SpawnMode string

const (
	// ModeHeadless is for automated spawns (cron, fire, triggers).
	// Lean context: constraints + task-relevant FTS + vault index only.
	ModeHeadless SpawnMode = "headless"

	// ModeInteractive is for manual connections (user launches Claude + connects to relay).
	// Full context: all memories, full vault docs, all inbox.
	ModeInteractive SpawnMode = "interactive"
)

// BuildSpawnContext assembles the full context for a spawned agent.
// Mode controls the injection strategy: headless = lean, interactive = everything.
func BuildSpawnContext(database *db.DB, project, profileSlug, cycleName string, taskID string, mode SpawnMode) (*SpawnContext, error) {
	if mode == "" {
		mode = ModeHeadless
	}

	// 1. IDENTITY — load profile
	profile, err := database.GetProfile(project, profileSlug)
	if err != nil {
		return nil, fmt.Errorf("load profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("profile %q not found in project %q", profileSlug, project)
	}

	ctx := &SpawnContext{
		Identity: &ContextIdentity{
			Profile:    profile.Slug,
			Project:    project,
			Role:       profile.Role,
			ContextPack: profile.ContextPack,
		},
		Knowledge: &ContextKnowledge{},
		Rules:     &ContextRules{},
	}

	// 2. TASK — load if specified
	if taskID != "" {
		task, err := database.GetTask(taskID, project)
		if err == nil && task != nil {
			ctx.Task = &ContextTask{
				ID:          task.ID,
				Title:       task.Title,
				Description: task.Description,
				Priority:    task.Priority,
			}
		}
	}

	// 3. KNOWLEDGE — strategy differs by mode

	// 3a. Constraints — always ALL, both modes
	constraints, err := database.GetMemoriesByLayer(project, profileSlug, "constraints")
	if err == nil {
		for _, m := range constraints {
			ctx.Knowledge.Constraints = append(ctx.Knowledge.Constraints,
				fmt.Sprintf("[%s] %s", m.Key, m.Value))
		}
	}

	if mode == ModeInteractive {
		// Interactive: load ALL behavior + context memories
		for _, layer := range []string{"behavior", "context"} {
			mems, err := database.GetMemoriesByLayer(project, profileSlug, layer)
			if err == nil {
				for _, m := range mems {
					ctx.Knowledge.Lessons = append(ctx.Knowledge.Lessons,
						fmt.Sprintf("[%s] %s", m.Key, m.Value))
				}
			}
		}
	} else {
		// Headless: FTS search scoped to task/cycle, skip context + constraints layers
		searchQuery := ""
		if ctx.Task != nil {
			searchQuery = ctx.Task.Title + " " + ctx.Task.Description
		} else if cycleName != "" {
			searchQuery = cycleName
		}
		if searchQuery != "" {
			memories, err := database.SearchMemory(project, profileSlug, searchQuery, nil, "", 10)
			if err == nil {
				for _, m := range memories {
					if m.Layer == "context" || m.Layer == "constraints" {
						continue
					}
					ctx.Knowledge.Lessons = append(ctx.Knowledge.Lessons,
						fmt.Sprintf("[%s] %s", m.Key, m.Value))
				}
			}
		}
	}

	// 3b. Vault docs
	if mode == ModeInteractive {
		// Interactive: load full content from profile's vault_paths
		if profile.VaultPaths != "" && profile.VaultPaths != "[]" {
			var paths []string
			if err := json.Unmarshal([]byte(profile.VaultPaths), &paths); err == nil {
				resolved := make([]string, len(paths))
				for i, p := range paths {
					resolved[i] = strings.ReplaceAll(p, "{slug}", profileSlug)
				}
				docs, err := database.GetVaultDocsByPaths(project, resolved, 0)
				if err == nil {
					for _, d := range docs {
						ctx.Knowledge.Conventions = append(ctx.Knowledge.Conventions,
							fmt.Sprintf("--- %s ---\n%s", d.Path, d.Content))
					}
				}
			}
		}
	} else {
		// Headless: FTS search vault for task/cycle-relevant docs, return paths only
		vaultQuery := ""
		if ctx.Task != nil {
			vaultQuery = ctx.Task.Title
		} else if cycleName != "" {
			vaultQuery = cycleName
		} else {
			vaultQuery = profileSlug
		}
		if vaultQuery != "" {
			results, err := database.SearchVault(project, vaultQuery, nil, 5)
			if err == nil {
				for _, r := range results {
					ctx.Knowledge.Conventions = append(ctx.Knowledge.Conventions,
						fmt.Sprintf("`%s` — %s", r.Path, r.Title))
				}
			}
		}
	}

	// 3c. Last cycle info from cycle_history
	history := database.GetCycleHistory(project, profileSlug, 1)
	if len(history) > 0 {
		h := history[0]
		createdAt, _ := h["created_at"].(string)
		cycleName, _ := h["cycle_name"].(string)
		durationMs, _ := h["duration_ms"].(int64)
		success, _ := h["success"].(bool)
		status := "success"
		if !success {
			status = "failed"
		}
		ctx.Knowledge.LastCycle = fmt.Sprintf("%s: %s (%dms, %s)", createdAt, cycleName, durationMs, status)
	}

	// 4. CYCLE PROMPT — from cycles table
	if cycleName != "" {
		cycle, _ := database.GetCycle(project, cycleName)
		if cycle != nil {
			ctx.Rules.Cycle = cycle.Prompt
		}
	}

	// 5. INBOX — unread messages for this profile (pruning/TTL handled by DB)
	inbox, err := database.GetInbox(project, profileSlug, true, 0)
	if err == nil {
		for _, msg := range inbox {
			ctx.Inbox = append(ctx.Inbox, ContextMessage{
				From:     msg.From,
				Subject:  msg.Subject,
				Content:  msg.Content,
				Priority: msg.Priority,
			})
		}
	}

	// 6. TEAM — for managers (agents with reports_to)
	agents, err := database.ListAgents(project)
	if err == nil && len(agents) > 0 {
		// Check if this profile is a manager (has anyone reporting to it)
		isManager := false
		for _, a := range agents {
			if a.ReportsTo != nil && *a.ReportsTo == profileSlug {
				isManager = true
				break
			}
		}

		if isManager {
			team := &ContextTeam{}
			for _, a := range agents {
				team.Agents = append(team.Agents, ContextAgent{
					Name:     a.Name,
					Status:   a.Status,
					LastSeen: a.LastSeen,
				})
			}

			// Count pending + blocked tasks
			pendingTasks, _ := database.ListTasks(project, "pending", "", "", "", "", 0, false)
			blockedTasks, _ := database.ListTasks(project, "blocked", "", "", "", "", 0, false)
			team.PendingTasks = len(pendingTasks)
			team.BlockedTasks = len(blockedTasks)

			ctx.Team = team
		}
	}

	return ctx, nil
}

// FormatPrompt converts a SpawnContext into the prompt string passed to claude --headless.
func FormatPrompt(ctx *SpawnContext) string {
	var b strings.Builder

	b.WriteString("You are an autonomous agent spawned by the relay OS.\n\n")

	// --- Identity ---
	b.WriteString("## Identity\n\n")
	b.WriteString(fmt.Sprintf("- **Profile:** %s\n", ctx.Identity.Profile))
	b.WriteString(fmt.Sprintf("- **Project:** %s\n", ctx.Identity.Project))
	b.WriteString(fmt.Sprintf("- **Role:** %s\n", ctx.Identity.Role))
	if ctx.Identity.ReportsTo != "" {
		b.WriteString(fmt.Sprintf("- **Reports to:** %s\n", ctx.Identity.ReportsTo))
	}
	if ctx.Identity.ContextPack != "" {
		b.WriteString(fmt.Sprintf("\n%s\n", ctx.Identity.ContextPack))
	}
	b.WriteString("\n")

	// --- Boot: register first ---
	b.WriteString("## Boot: Register First\n\n")
	b.WriteString(fmt.Sprintf("**Step 0 — before anything else**, call:\n```\nregister_agent(name: %q, role: %q, project: %q", ctx.Identity.Profile, ctx.Identity.Role, ctx.Identity.Project))
	if ctx.Identity.ReportsTo != "" {
		b.WriteString(fmt.Sprintf(", reports_to: %q", ctx.Identity.ReportsTo))
	}
	b.WriteString(fmt.Sprintf(", profile_slug: %q)\n```\n", ctx.Identity.Profile))
	b.WriteString("This connects your session to the relay. Without it you cannot send/receive messages, claim tasks, or ACK notifications.\n\n")

	// --- Relay identity reminder ---
	b.WriteString(fmt.Sprintf("Pass `as: %q` and `project: %q` on **every** relay tool call (except register_agent which uses `name`).\n\n", ctx.Identity.Profile, ctx.Identity.Project))

	// --- Task ---
	if ctx.Task != nil {
		b.WriteString("## Task\n\n")
		b.WriteString(fmt.Sprintf("**[%s] %s**\n\n", ctx.Task.Priority, ctx.Task.Title))
		b.WriteString(ctx.Task.Description)
		b.WriteString("\n")
		if ctx.Task.Acceptance != "" {
			b.WriteString(fmt.Sprintf("\n**Acceptance criteria:** %s\n", ctx.Task.Acceptance))
		}
		b.WriteString("\n")
	}

	// --- Cycle instructions ---
	if ctx.Rules.Cycle != "" {
		b.WriteString("## Cycle Instructions\n\n")
		b.WriteString(ctx.Rules.Cycle)
		b.WriteString("\n\n")
	}

	// --- Constraints (non-negotiable rules) ---
	if len(ctx.Knowledge.Constraints) > 0 {
		b.WriteString("## Constraints (non-negotiable)\n\n")
		for _, c := range ctx.Knowledge.Constraints {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
		b.WriteString("\n")
	}

	// --- Knowledge ---
	if len(ctx.Knowledge.Conventions) > 0 || len(ctx.Knowledge.Lessons) > 0 || ctx.Knowledge.LastCycle != "" {
		b.WriteString("## Knowledge\n\n")

		if len(ctx.Knowledge.Conventions) > 0 {
			b.WriteString("### Conventions (use `get_vault_doc` to load)\n\n")
			for _, c := range ctx.Knowledge.Conventions {
				b.WriteString(fmt.Sprintf("- %s\n", c))
			}
			b.WriteString("\n")
		}

		if len(ctx.Knowledge.Lessons) > 0 {
			b.WriteString("### Lessons Learned\n\n")
			for _, l := range ctx.Knowledge.Lessons {
				b.WriteString(fmt.Sprintf("- %s\n", l))
			}
			b.WriteString("\n")
		}

		if ctx.Knowledge.LastCycle != "" {
			b.WriteString(fmt.Sprintf("### Last Cycle\n\n%s\n\n", ctx.Knowledge.LastCycle))
		}
	}

	// --- Inbox ---
	if len(ctx.Inbox) > 0 {
		b.WriteString("## Inbox\n\n")
		for _, m := range ctx.Inbox {
			prio := ""
			if m.Priority != "" {
				prio = fmt.Sprintf(" (%s)", m.Priority)
			}
			b.WriteString(fmt.Sprintf("**From %s%s:** %s\n", m.From, prio, m.Subject))
			if m.Content != "" {
				b.WriteString(fmt.Sprintf("> %s\n", m.Content))
			}
			b.WriteString("\n")
		}
	}

	// --- Team ---
	if ctx.Team != nil && len(ctx.Team.Agents) > 0 {
		b.WriteString("## Team\n\n")
		for _, a := range ctx.Team.Agents {
			b.WriteString(fmt.Sprintf("- **%s** — %s (last seen: %s)\n", a.Name, a.Status, a.LastSeen))
		}
		b.WriteString(fmt.Sprintf("\nPending tasks: %d | Blocked tasks: %d\n\n", ctx.Team.PendingTasks, ctx.Team.BlockedTasks))
	}

	b.WriteString("When done: persist what you learned via `set_memory`, then exit.\n")

	return b.String()
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

