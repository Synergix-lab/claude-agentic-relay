package relay

import (
	"context"
	"fmt"
	"strings"

	"agent-relay/internal/spawn"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleSpawn spawns a child agent process (fork + exec).
// Supports two modes:
//   - Legacy: profile + prompt (raw prompt passed to claude)
//   - Agent OS: profile + cycle (relay assembles the full context)
func (h *Handlers) HandleSpawn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	profile := req.GetString("profile", "")
	prompt := req.GetString("prompt", "")
	cycle := req.GetString("cycle", "")
	taskID := req.GetString("task_id", "")
	ttl := req.GetString("ttl", "10m")
	allowedTools := req.GetString("allowed_tools", "")

	if agent == "" {
		return mcp.NewToolResultError("agent identity required (use 'as' parameter or register first)"), nil
	}
	if profile == "" {
		return mcp.NewToolResultError("profile is required"), nil
	}

	// Quota check: spawns
	if qErr := h.db.CheckQuotaError(project, agent, "spawns"); qErr != "" {
		return mcp.NewToolResultError(qErr), nil
	}

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("spawn not available: claude binary not found"), nil
	}

	var childID string
	var err error

	if cycle != "" {
		// Agent OS mode: relay assembles full context from profile + cycle + task
		childID, err = h.spawnMgr.SpawnWithContext(project, profile, cycle, taskID)
	} else {
		// Legacy mode: raw prompt
		if prompt == "" {
			return mcp.NewToolResultError("either 'prompt' or 'cycle' is required"), nil
		}
		childID, err = h.spawnMgr.Spawn(agent, project, profile, prompt, ttl, allowedTools)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("spawn failed: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "spawn",
		Action:  "start",
		Agent:   agent,
		Project: project,
		Label:   fmt.Sprintf("child %s (profile: %s, cycle: %s)", childID[:8], profile, cycle),
	})

	return h.resultJSONTracked(project, agent, "spawn", map[string]any{
		"child_id": childID,
		"profile":  profile,
		"cycle":    cycle,
		"status":   "running",
		"message":  fmt.Sprintf("Child agent spawned with profile '%s'. Use list_children to monitor.", profile),
	})
}

// HandleKillChild terminates a running child agent.
func (h *Handlers) HandleKillChild(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	childID := req.GetString("child_id", "")

	if childID == "" {
		return mcp.NewToolResultError("child_id is required"), nil
	}

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("spawn not available"), nil
	}

	if err := h.spawnMgr.KillChild(childID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("kill failed: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "spawn",
		Action:  "kill",
		Agent:   agent,
		Project: project,
		Label:   fmt.Sprintf("killed child %s", childID[:8]),
	})

	return h.resultJSONTracked(project, agent, "kill_child", map[string]any{
		"child_id": childID,
		"status":   "killed",
	})
}

// HandleListChildren lists spawned child agents.
func (h *Handlers) HandleListChildren(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	status := req.GetString("status", "all")

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("spawn not available"), nil
	}

	children := h.spawnMgr.ListChildren(agent, project, status)
	if children == nil {
		children = []map[string]any{}
	}

	return h.resultJSONTracked(project, agent, "list_children", map[string]any{
		"children":     children,
		"active_count": h.spawnMgr.ActiveCount(project),
	})
}

// HandleSchedule creates or updates a cron schedule.
func (h *Handlers) HandleSchedule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	name := req.GetString("name", "")
	cronExpr := req.GetString("cron_expr", "")
	prompt := req.GetString("prompt", "")
	ttl := req.GetString("ttl", "10m")
	cycle := req.GetString("cycle", "")
	allowedTools := req.GetString("allowed_tools", "")

	var missing []string
	if agent == "" {
		missing = append(missing, "as (caller agent identity)")
	}
	if name == "" {
		missing = append(missing, "name")
	}
	if cronExpr == "" {
		missing = append(missing, "cron_expr")
	}
	if cycle == "" && prompt == "" {
		missing = append(missing, "cycle or prompt")
	}
	if len(missing) > 0 {
		return mcp.NewToolResultError("missing required fields: " + strings.Join(missing, ", ")), nil
	}

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("scheduler not available"), nil
	}

	scheduleID := uuid.New().String()

	if err := h.spawnMgr.Schedule(scheduleID, agent, project, name, cronExpr, prompt, ttl, cycle, allowedTools); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("schedule failed: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "schedule",
		Action:  "create",
		Agent:   agent,
		Project: project,
		Label:   fmt.Sprintf("%s @ %s", name, cronExpr),
	})

	return h.resultJSONTracked(project, agent, "schedule", map[string]any{
		"schedule_id": scheduleID,
		"name":        name,
		"cron_expr":   cronExpr,
		"ttl":         ttl,
		"message":     fmt.Sprintf("Schedule '%s' created. Agent will execute on cron: %s", name, cronExpr),
	})
}

// HandleUnschedule removes a cron schedule.
func (h *Handlers) HandleUnschedule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	scheduleID := req.GetString("schedule_id", "")

	if scheduleID == "" {
		return mcp.NewToolResultError("schedule_id is required"), nil
	}

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("scheduler not available"), nil
	}

	h.spawnMgr.Unschedule(scheduleID)

	h.events.Emit(MCPEvent{
		Type:    "schedule",
		Action:  "delete",
		Agent:   agent,
		Project: project,
		Label:   scheduleID[:8],
	})

	return h.resultJSONTracked(project, agent, "unschedule", map[string]any{
		"schedule_id": scheduleID,
		"status":      "removed",
	})
}

// HandleListSchedules lists cron schedules.
func (h *Handlers) HandleListSchedules(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)

	var schedules []map[string]any
	if agent != "" {
		schedules = h.db.ListSchedulesByAgent(project, agent)
	} else {
		schedules = h.db.ListSchedulesByProject(project)
	}
	if schedules == nil {
		schedules = []map[string]any{}
	}

	schedulerRunning := false
	jobCount := 0
	if h.spawnMgr != nil {
		sched := h.spawnMgr.GetScheduler()
		schedulerRunning = sched.IsRunning()
		jobCount = sched.JobCount()
	}

	return h.resultJSONTracked(project, agent, "list_schedules", map[string]any{
		"schedules":         schedules,
		"scheduler_running": schedulerRunning,
		"total_jobs":        jobCount,
	})
}

// HandleTriggerCycle manually triggers a scheduled cycle.
func (h *Handlers) HandleTriggerCycle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agent := resolveAgent(ctx, req)
	project := resolveProject(ctx, req)
	scheduleID := req.GetString("schedule_id", "")

	if scheduleID == "" {
		return mcp.NewToolResultError("schedule_id is required"), nil
	}

	if h.spawnMgr == nil {
		return mcp.NewToolResultError("scheduler not available"), nil
	}

	if err := h.spawnMgr.TriggerCycle(scheduleID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("trigger failed: %v", err)), nil
	}

	h.events.Emit(MCPEvent{
		Type:    "schedule",
		Action:  "trigger",
		Agent:   agent,
		Project: project,
		Label:   fmt.Sprintf("manual trigger %s", scheduleID[:8]),
	})

	return h.resultJSONTracked(project, agent, "trigger_cycle", map[string]any{
		"schedule_id": scheduleID,
		"status":      "triggered",
		"message":     "Cycle triggered. It will execute asynchronously.",
	})
}

// SetSpawnManager sets the spawn manager on the handlers (called after construction).
func (h *Handlers) SetSpawnManager(mgr *spawn.Manager) {
	h.spawnMgr = mgr
}
