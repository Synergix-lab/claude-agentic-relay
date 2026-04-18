package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-relay/internal/db"
	"agent-relay/internal/models"
)

func (r *Relay) apiGetSpawnChildren(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	agent := req.URL.Query().Get("agent")
	status := req.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	children := r.DB.ListSpawnChildren(agent, project, status)
	if children == nil {
		children = []map[string]any{}
	}
	b, _ := json.Marshal(children)
	w.Write(b)
}

func (r *Relay) apiKillSpawnChild(w http.ResponseWriter, path string) {
	// /spawn/children/{id}/kill
	parts := strings.Split(strings.TrimPrefix(path, "/spawn/children/"), "/")
	if len(parts) < 2 {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	childID := parts[0]

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"spawn not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := r.SpawnMgr.KillChild(childID); err != nil {
		apiError(w, http.StatusNotFound, "kill failed", err)
		return
	}
	b, _ := json.Marshal(map[string]any{"child_id": childID, "status": "killed"})
	w.Write(b)
}

func (r *Relay) apiSpawnChild(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Agent        string `json:"agent"`
		Project      string `json:"project"`
		Profile      string `json:"profile"`
		Prompt       string `json:"prompt"`
		TTL          string `json:"ttl"`
		AllowedTools string `json:"allowed_tools"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.TTL == "" {
		body.TTL = "10m"
	}
	if body.Profile == "" || body.Prompt == "" || body.Agent == "" {
		http.Error(w, `{"error":"agent, profile, and prompt are required"}`, http.StatusBadRequest)
		return
	}

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"spawn not available"}`, http.StatusServiceUnavailable)
		return
	}

	childID, err := r.SpawnMgr.Spawn(body.Agent, body.Project, body.Profile, body.Prompt, body.TTL, body.AllowedTools)
	if err != nil {
		apiError(w, http.StatusConflict, "spawn failed", err)
		return
	}

	b, _ := json.Marshal(map[string]any{"child_id": childID, "profile": body.Profile, "status": "running"})
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

func (r *Relay) apiGetSpawnChild(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/spawn/children/")
	id = strings.TrimSuffix(id, "/")

	child := r.DB.GetSpawnChild(id)
	if child == nil {
		http.Error(w, `{"error":"child not found"}`, http.StatusNotFound)
		return
	}
	b, _ := json.Marshal(child)
	w.Write(b)
}

func (r *Relay) apiGetSchedules(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	agent := req.URL.Query().Get("agent")

	var schedules []map[string]any
	if agent != "" {
		schedules = r.DB.ListSchedulesByAgent(project, agent)
	} else {
		schedules = r.DB.ListSchedulesByProject(project)
	}
	if schedules == nil {
		schedules = []map[string]any{}
	}
	b, _ := json.Marshal(schedules)
	w.Write(b)
}

func (r *Relay) apiCreateSchedule(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Agent        string `json:"agent"`
		Project      string `json:"project"`
		Name         string `json:"name"`
		CronExpr     string `json:"cron_expr"`
		Prompt       string `json:"prompt"`
		TTL          string `json:"ttl"`
		Cycle        string `json:"cycle"`
		AllowedTools string `json:"allowed_tools"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.TTL == "" {
		body.TTL = "10m"
	}

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"scheduler not available"}`, http.StatusServiceUnavailable)
		return
	}

	id := body.Name + "-" + body.Agent // simple deterministic ID
	if err := r.SpawnMgr.Schedule(id, body.Agent, body.Project, body.Name, body.CronExpr, body.Prompt, body.TTL, body.Cycle, body.AllowedTools); err != nil {
		apiError(w, http.StatusBadRequest, "schedule failed", err)
		return
	}

	b, _ := json.Marshal(map[string]any{"schedule_id": id, "status": "created"})
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

func (r *Relay) apiTriggerSchedule(w http.ResponseWriter, path string) {
	// /schedules/{id}/trigger
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/schedules/"), "/trigger")

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"scheduler not available"}`, http.StatusServiceUnavailable)
		return
	}

	if err := r.SpawnMgr.TriggerCycle(id); err != nil {
		apiError(w, http.StatusNotFound, "trigger failed", err)
		return
	}

	b, _ := json.Marshal(map[string]any{"schedule_id": id, "status": "triggered"})
	w.Write(b)
}

func (r *Relay) apiGetSchedule(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/schedules/")
	id = strings.TrimSuffix(id, "/")

	schedule := r.DB.GetSchedule(id)
	if schedule == nil {
		http.Error(w, `{"error":"schedule not found"}`, http.StatusNotFound)
		return
	}
	b, _ := json.Marshal(schedule)
	w.Write(b)
}

func (r *Relay) apiUpdateSchedule(w http.ResponseWriter, req *http.Request, path string) {
	id := strings.TrimPrefix(path, "/schedules/")
	id = strings.TrimSuffix(id, "/")

	existing := r.DB.GetSchedule(id)
	if existing == nil {
		http.Error(w, `{"error":"schedule not found"}`, http.StatusNotFound)
		return
	}

	var body struct {
		CronExpr     *string `json:"cron_expr"`
		Prompt       *string `json:"prompt"`
		TTL          *string `json:"ttl"`
		Cycle        *string `json:"cycle"`
		AllowedTools *string `json:"allowed_tools"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"scheduler not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Merge with existing values
	cronExpr, _ := existing["cron_expr"].(string)
	prompt, _ := existing["prompt"].(string)
	ttl, _ := existing["ttl"].(string)
	cycle, _ := existing["cycle"].(string)
	allowedTools, _ := existing["allowed_tools"].(string)
	agentName, _ := existing["agent_name"].(string)
	project, _ := existing["project"].(string)
	name, _ := existing["name"].(string)

	if body.CronExpr != nil {
		cronExpr = *body.CronExpr
	}
	if body.Prompt != nil {
		prompt = *body.Prompt
	}
	if body.TTL != nil {
		ttl = *body.TTL
	}
	if body.Cycle != nil {
		cycle = *body.Cycle
	}
	if body.AllowedTools != nil {
		allowedTools = *body.AllowedTools
	}

	if err := r.SpawnMgr.Schedule(id, agentName, project, name, cronExpr, prompt, ttl, cycle, allowedTools); err != nil {
		apiError(w, http.StatusBadRequest, "update failed", err)
		return
	}

	b, _ := json.Marshal(map[string]any{"schedule_id": id, "status": "updated"})
	w.Write(b)
}

func (r *Relay) apiDeleteSchedule(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/schedules/")

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"scheduler not available"}`, http.StatusServiceUnavailable)
		return
	}

	r.SpawnMgr.Unschedule(id)
	b, _ := json.Marshal(map[string]any{"schedule_id": id, "status": "deleted"})
	w.Write(b)
}

func (r *Relay) apiGetCycleHistory(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	agent := req.URL.Query().Get("agent")

	history := r.DB.GetCycleHistory(project, agent, 50)
	if history == nil {
		history = []map[string]any{}
	}
	b, _ := json.Marshal(history)
	w.Write(b)
}

// --- Triggers (event-driven spawn rules) ---

func (r *Relay) apiGetTriggers(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	event := req.URL.Query().Get("event")

	var triggers []db.Trigger
	if event != "" {
		triggers = r.DB.ListTriggers(project, event)
	} else {
		triggers = r.DB.ListAllTriggers(project)
	}
	if triggers == nil {
		triggers = []db.Trigger{}
	}
	b, _ := json.Marshal(triggers)
	w.Write(b)
}

func (r *Relay) apiCreateTrigger(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project         string `json:"project"`
		Event           string `json:"event"`
		MatchRules      string `json:"match_rules"`
		ProfileSlug     string `json:"profile_slug"`
		Cycle           string `json:"cycle"`
		MaxDuration     string `json:"max_duration"`
		CooldownSeconds *int   `json:"cooldown_seconds"` // nil = default 60s, *0 = disabled
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Event == "" || body.ProfileSlug == "" || body.Cycle == "" {
		http.Error(w, `{"error":"event, profile_slug, and cycle are required"}`, http.StatusBadRequest)
		return
	}

	trigger, err := r.DB.UpsertTrigger(body.Project, body.Event, body.MatchRules, body.ProfileSlug, body.Cycle, body.MaxDuration, body.CooldownSeconds)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create trigger failed", err)
		return
	}

	b, _ := json.Marshal(trigger)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

func (r *Relay) apiDeleteTrigger(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/triggers/")
	id = strings.TrimSuffix(id, "/")
	r.DB.DeleteTrigger(id)
	b, _ := json.Marshal(map[string]any{"id": id, "status": "deleted"})
	w.Write(b)
}

// --- Agent OS spawn (profile + cycle → assembled context) ---

func (r *Relay) apiSpawnWithContext(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project string `json:"project"`
		Profile string `json:"profile"`
		Cycle   string `json:"cycle"`
		TaskID  string `json:"task_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Profile == "" {
		http.Error(w, `{"error":"profile is required"}`, http.StatusBadRequest)
		return
	}

	if r.SpawnMgr == nil {
		http.Error(w, `{"error":"spawn not available"}`, http.StatusServiceUnavailable)
		return
	}

	childID, err := r.SpawnMgr.SpawnWithContext(body.Project, body.Profile, body.Cycle, body.TaskID)
	if err != nil {
		apiError(w, http.StatusConflict, "spawn failed", err)
		return
	}

	b, _ := json.Marshal(map[string]any{
		"child_id": childID,
		"profile":  body.Profile,
		"cycle":    body.Cycle,
		"status":   "running",
	})
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// --- Webhook receiver ---

// POST /api/webhooks/:project/:event
func (r *Relay) apiWebhook(w http.ResponseWriter, req *http.Request, path string) {
	// path = /webhooks/<project>/<event>
	parts := strings.SplitN(strings.TrimPrefix(path, "/webhooks/"), "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, `{"error":"path must be /webhooks/:project/:event"}`, http.StatusBadRequest)
		return
	}
	project := parts[0]
	event := parts[1]

	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20)) // 1MB max
	if err != nil {
		apiError(w, http.StatusBadRequest, "read body failed", err)
		return
	}

	meta := flattenJSON(body)
	if meta == nil {
		meta = map[string]string{}
	}

	fires, skipped := r.Handlers.fireTriggersSync(project, event, meta)
	if fires == nil {
		fires = []webhookResult{}
	}
	if skipped == nil {
		skipped = []webhookSkipped{}
	}

	b, _ := json.Marshal(map[string]any{"fires": fires, "skipped": skipped})
	w.Write(b)
}

// --- Signal handlers (convenience endpoint) ---

// POST /api/signal-handlers — creates a trigger with event = "signal:<name>"
func (r *Relay) apiCreateSignalHandler(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project     string `json:"project"`
		Signal      string `json:"signal"` // e.g. "interrupt", "alert"
		MatchRules  string `json:"match_rules"`
		ProfileSlug string `json:"profile_slug"`
		Cycle       string `json:"cycle"`
		MaxDuration string `json:"max_duration"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Signal == "" || body.ProfileSlug == "" || body.Cycle == "" {
		http.Error(w, `{"error":"signal, profile_slug, and cycle are required"}`, http.StatusBadRequest)
		return
	}

	event := "signal:" + body.Signal
	trigger, err := r.DB.UpsertTrigger(body.Project, event, body.MatchRules, body.ProfileSlug, body.Cycle, body.MaxDuration, nil)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create signal handler failed", err)
		return
	}

	b, _ := json.Marshal(trigger)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// --- Trigger history ---

// GET /api/trigger-history?project=X&limit=N
func (r *Relay) apiGetTriggerHistory(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	limit := 50
	if l := req.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	history := r.DB.GetTriggerHistory(project, limit)
	if history == nil {
		history = []db.TriggerFire{}
	}
	b, _ := json.Marshal(history)
	w.Write(b)
}

// --- Poll Triggers ---

// GET /api/poll-triggers?project=X
func (r *Relay) apiGetPollTriggers(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	triggers, err := r.DB.ListPollTriggers(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list poll triggers failed", err)
		return
	}
	if triggers == nil {
		triggers = []models.PollTrigger{}
	}
	b, _ := json.Marshal(triggers)
	w.Write(b)
}

// POST /api/poll-triggers
func (r *Relay) apiCreatePollTrigger(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project        string `json:"project"`
		Name           string `json:"name"`
		URL            string `json:"url"`
		Headers        string `json:"headers"`
		ConditionPath  string `json:"condition_path"`
		ConditionOp    string `json:"condition_op"`
		ConditionValue string `json:"condition_value"`
		PollInterval   string `json:"poll_interval"`
		FireEvent      string `json:"fire_event"`
		FireMeta       string `json:"fire_meta"`
		Cooldown       int    `json:"cooldown_seconds"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Name == "" || body.URL == "" || body.ConditionPath == "" || body.ConditionOp == "" || body.FireEvent == "" {
		http.Error(w, `{"error":"name, url, condition_path, condition_op, and fire_event are required"}`, http.StatusBadRequest)
		return
	}
	if body.PollInterval == "" {
		body.PollInterval = "5m"
	}

	pt, err := r.DB.UpsertPollTrigger(body.Project, body.Name, body.URL, body.Headers,
		body.ConditionPath, body.ConditionOp, body.ConditionValue, body.PollInterval,
		body.FireEvent, body.FireMeta, body.Cooldown)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create poll trigger failed", err)
		return
	}
	b, _ := json.Marshal(pt)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// DELETE /api/poll-triggers/:id
func (r *Relay) apiDeletePollTrigger(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/poll-triggers/")
	id = strings.TrimSuffix(id, "/")
	if err := r.DB.DeletePollTrigger(id); err != nil {
		apiError(w, http.StatusInternalServerError, "delete poll trigger failed", err)
		return
	}
	b, _ := json.Marshal(map[string]any{"id": id, "status": "deleted"})
	w.Write(b)
}

// POST /api/poll-triggers/:id/test
func (r *Relay) apiTestPollTrigger(w http.ResponseWriter, path string) {
	// path = /poll-triggers/<id>/test
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/poll-triggers/"), "/test")
	matched, value, err := r.Handlers.PollOnceByID(id)
	if err != nil {
		// Include the underlying cause so the UI can show what actually failed
		// (network error, JSONPath miss, condition mismatch, etc.) rather than
		// a generic "poll test failed".
		b, _ := json.Marshal(map[string]any{
			"error":   "poll test failed",
			"details": err.Error(),
		})
		w.WriteHeader(http.StatusBadRequest)
		w.Write(b)
		return
	}
	b, _ := json.Marshal(map[string]any{
		"matched": matched,
		"value":   value,
	})
	w.Write(b)
}

// --- Skills ---

// GET /api/skills?project=X
func (r *Relay) apiGetSkills(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	// If agent slug is given, return skills linked to that profile with proficiency
	if agent := req.URL.Query().Get("agent"); agent != "" {
		profile, err := r.DB.GetProfile(project, agent)
		if err != nil || profile == nil {
			writeJSON(w, []any{})
			return
		}
		links, err := r.DB.GetProfileSkillLinks(profile.ID)
		if err != nil || links == nil {
			writeJSON(w, []any{})
			return
		}
		// Add agent field so the frontend can filter
		for i := range links {
			links[i]["agent"] = agent
		}
		b, _ := json.Marshal(links)
		w.Write(b)
		return
	}

	skills, err := r.DB.ListSkills(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list skills failed", err)
		return
	}
	if skills == nil {
		skills = []models.Skill{}
	}
	b, _ := json.Marshal(skills)
	w.Write(b)
}

// POST /api/skills
func (r *Relay) apiCreateSkill(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project     string `json:"project"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Tags        string `json:"tags"`
		Agent       string `json:"agent"`       // optional: auto-link to this profile
		Proficiency int    `json:"proficiency"` // 1-5, mapped to text
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	skill, err := r.DB.UpsertSkill(body.Project, body.Name, body.Description, body.Tags)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create skill failed", err)
		return
	}

	// Auto-link to profile if agent is specified
	if body.Agent != "" {
		profile, err := r.DB.GetProfile(body.Project, body.Agent)
		if err == nil && profile != nil {
			prof := "capable"
			if body.Proficiency >= 5 {
				prof = "expert"
			} else if body.Proficiency >= 3 {
				prof = "capable"
			} else if body.Proficiency >= 1 {
				prof = "learning"
			}
			_ = r.DB.LinkProfileSkill(profile.ID, skill.ID, prof)
		}
	}

	b, _ := json.Marshal(skill)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// GET /api/skills/:name/profiles?project=X
func (r *Relay) apiGetSkillProfiles(w http.ResponseWriter, req *http.Request, path string) {
	// path = /skills/<name>/profiles
	name := strings.TrimSuffix(strings.TrimPrefix(path, "/skills/"), "/profiles")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	links, err := r.DB.GetSkillProfileLinks(project, name)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "get skill profiles failed", err)
		return
	}
	if links == nil {
		links = []map[string]any{}
	}
	b, _ := json.Marshal(map[string]any{"skill": name, "profiles": links})
	w.Write(b)
}

// --- Quotas ---

// GET /api/quotas?project=X
func (r *Relay) apiGetQuotas(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	quotas, err := r.DB.ListAgentQuotas(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list quotas failed", err)
		return
	}
	if quotas == nil {
		quotas = []models.AgentQuota{}
	}
	b, _ := json.Marshal(quotas)
	w.Write(b)
}

// GET /api/quotas/:agent?project=X
func (r *Relay) apiGetAgentQuota(w http.ResponseWriter, req *http.Request, path string) {
	agent := strings.TrimPrefix(path, "/quotas/")
	agent = strings.TrimSuffix(agent, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	usage, err := r.DB.GetQuotaUsage(project, agent)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "get quota failed", err)
		return
	}
	b, _ := json.Marshal(usage)
	w.Write(b)
}

// PUT /api/quotas/:agent
func (r *Relay) apiSetAgentQuota(w http.ResponseWriter, req *http.Request, path string) {
	agent := strings.TrimPrefix(path, "/quotas/")
	agent = strings.TrimSuffix(agent, "/")

	var body struct {
		Project            string `json:"project"`
		MaxTokensPerDay    int64  `json:"max_tokens_per_day"`
		MaxMessagesPerHour int64  `json:"max_messages_per_hour"`
		MaxTasksPerHour    int64  `json:"max_tasks_per_hour"`
		MaxSpawnsPerHour   int64  `json:"max_spawns_per_hour"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}

	if err := r.DB.SetAgentQuota(body.Project, agent, body.MaxTokensPerDay, body.MaxMessagesPerHour, body.MaxTasksPerHour, body.MaxSpawnsPerHour); err != nil {
		apiError(w, http.StatusInternalServerError, "set quota failed", err)
		return
	}

	// Echo back the resulting quota so the caller can verify what was stored
	// (previously the response only said "updated" even when all fields were 0).
	quota, _ := r.DB.GetAgentQuota(body.Project, agent)
	if quota == nil {
		writeJSON(w, map[string]any{"agent": agent, "status": "updated"})
		return
	}
	writeJSON(w, quota)
}

// --- Service Discovery ---

// GET /api/discover?project=X&skill=Y&active_only=true
func (r *Relay) apiDiscover(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	skill := req.URL.Query().Get("skill")
	if skill == "" {
		http.Error(w, `{"error":"skill parameter is required"}`, http.StatusBadRequest)
		return
	}

	agents, err := r.DB.FindActiveAgentsBySkill(project, skill)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "discover failed", err)
		return
	}
	if agents == nil {
		agents = []models.Agent{}
	}

	// Also include profiles that have this skill (may not have active agents)
	profiles, _ := r.DB.FindProfilesBySkill(project, skill)
	if profiles == nil {
		profiles = []models.Profile{}
	}

	b, _ := json.Marshal(map[string]any{
		"skill":    skill,
		"agents":   agents,
		"profiles": profiles,
	})
	w.Write(b)
}

// --- Privilege Escalation ---

// GET /api/elevations?project=X
func (r *Relay) apiGetElevations(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	elevations, err := r.DB.ListActiveElevations(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list elevations failed", err)
		return
	}
	if elevations == nil {
		elevations = []models.Elevation{}
	}
	b, _ := json.Marshal(elevations)
	w.Write(b)
}

// POST /api/elevations
func (r *Relay) apiGrantElevation(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project   string `json:"project"`
		Agent     string `json:"agent"`
		Role      string `json:"role"` // admin, lead
		GrantedBy string `json:"granted_by"`
		Reason    string `json:"reason"`
		Duration  string `json:"duration"` // e.g. "1h", "30m"
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Agent == "" || body.Role == "" || body.GrantedBy == "" {
		http.Error(w, `{"error":"agent, role, and granted_by are required"}`, http.StatusBadRequest)
		return
	}
	if body.Duration == "" {
		body.Duration = "1h"
	}

	duration, err := time.ParseDuration(body.Duration)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid duration format", err)
		return
	}

	elevation, err := r.DB.GrantElevation(body.Project, body.Agent, body.Role, body.GrantedBy, body.Reason, duration)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "grant elevation failed", err)
		return
	}
	b, _ := json.Marshal(elevation)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// DELETE /api/elevations/:id
func (r *Relay) apiRevokeElevation(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/elevations/")
	id = strings.TrimSuffix(id, "/")
	if err := r.DB.RevokeElevation(id); err != nil {
		apiError(w, http.StatusInternalServerError, "revoke elevation failed", err)
		return
	}
	b, _ := json.Marshal(map[string]any{"id": id, "status": "revoked"})
	w.Write(b)
}

// --- Agent management ---

// DELETE /api/agents/:name?project=X
func (r *Relay) apiDeactivateAgent(w http.ResponseWriter, req *http.Request, path string) {
	name := strings.TrimPrefix(path, "/agents/")
	name = strings.TrimSuffix(name, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	if err := r.DB.DeactivateAgent(project, name); err != nil {
		apiError(w, http.StatusInternalServerError, "deactivate agent failed", err)
		return
	}
	writeJSON(w, map[string]any{"agent": name, "status": "deactivated"})
}

// --- Profile management ---

// POST /api/profiles
func (r *Relay) apiCreateProfile(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project      string `json:"project"`
		Slug         string `json:"slug"`
		Name         string `json:"name"`
		Role         string `json:"role"`
		ContextPack  string `json:"context_pack"`
		SoulKeys     string `json:"soul_keys"`
		Skills       string `json:"skills"`
		VaultPaths   string `json:"vault_paths"`
		AllowedTools string `json:"allowed_tools"`
		PoolSize     int    `json:"pool_size"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Slug == "" {
		http.Error(w, `{"error":"slug is required"}`, http.StatusBadRequest)
		return
	}

	var opts []db.ProfileOption
	if body.AllowedTools != "" {
		opts = append(opts, db.WithAllowedTools(body.AllowedTools))
	}
	if body.PoolSize > 0 {
		opts = append(opts, db.WithPoolSize(body.PoolSize))
	}

	profile, err := r.DB.RegisterProfile(body.Project, body.Slug, body.Name, body.Role, body.ContextPack, body.SoulKeys, body.Skills, body.VaultPaths, opts...)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create profile failed", err)
		return
	}
	b, _ := json.Marshal(profile)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// PUT /api/profiles/:slug?project=X
func (r *Relay) apiUpdateProfile(w http.ResponseWriter, req *http.Request, path string) {
	slug := strings.TrimPrefix(path, "/profiles/")
	slug = strings.TrimSuffix(slug, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	// Merge semantics: decode into a raw map so absent fields keep the current
	// value. A prior bug wiped context_pack/soul_keys/skills/vault_paths to
	// empty strings when the caller sent only {name, role}.
	var raw map[string]any
	if err := json.NewDecoder(req.Body).Decode(&raw); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}

	existing, err := r.DB.GetProfile(project, slug)
	if err != nil || existing == nil {
		apiError(w, http.StatusNotFound, "profile not found", err)
		return
	}

	// Start from current values, overlay only the fields that were sent.
	name := existing.Name
	role := existing.Role
	contextPack := existing.ContextPack
	soulKeys := existing.SoulKeys
	skills := existing.Skills
	vaultPaths := existing.VaultPaths
	allowedTools := existing.AllowedTools
	poolSize := existing.PoolSize

	if v, ok := raw["name"].(string); ok {
		name = v
	}
	if v, ok := raw["role"].(string); ok {
		role = v
	}
	if v, ok := raw["context_pack"].(string); ok {
		contextPack = v
	}
	if v, ok := raw["soul_keys"].(string); ok {
		soulKeys = v
	}
	if v, ok := raw["skills"].(string); ok {
		skills = v
	}
	if v, ok := raw["vault_paths"].(string); ok {
		vaultPaths = v
	}
	if v, ok := raw["allowed_tools"].(string); ok {
		allowedTools = v
	}
	if v, ok := raw["pool_size"].(float64); ok && int(v) > 0 {
		poolSize = int(v)
	}

	var opts []db.ProfileOption
	if allowedTools != "" {
		opts = append(opts, db.WithAllowedTools(allowedTools))
	}
	if poolSize > 0 {
		opts = append(opts, db.WithPoolSize(poolSize))
	}

	profile, err := r.DB.RegisterProfile(project, slug, name, role, contextPack, soulKeys, skills, vaultPaths, opts...)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "update profile failed", err)
		return
	}
	writeJSON(w, profile)
}

// DELETE /api/profiles/:slug?project=X
func (r *Relay) apiDeleteProfile(w http.ResponseWriter, req *http.Request, path string) {
	slug := strings.TrimPrefix(path, "/profiles/")
	slug = strings.TrimSuffix(slug, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	if err := r.DB.DeleteProfile(project, slug); err != nil {
		apiError(w, http.StatusInternalServerError, "delete profile failed", err)
		return
	}
	writeJSON(w, map[string]any{"slug": slug, "status": "deleted"})
}

// DELETE /api/skills/:name?project=X
func (r *Relay) apiDeleteSkill(w http.ResponseWriter, req *http.Request, path string) {
	name := strings.TrimPrefix(path, "/skills/")
	name = strings.TrimSuffix(name, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	if err := r.DB.DeleteSkill(project, name); err != nil {
		apiError(w, http.StatusInternalServerError, "delete skill failed", err)
		return
	}
	writeJSON(w, map[string]any{"name": name, "status": "deleted"})
}

// DELETE /api/quotas/:agent?project=X
func (r *Relay) apiDeleteQuota(w http.ResponseWriter, req *http.Request, path string) {
	agent := strings.TrimPrefix(path, "/quotas/")
	agent = strings.TrimSuffix(agent, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}

	if err := r.DB.DeleteQuota(project, agent); err != nil {
		apiError(w, http.StatusInternalServerError, "delete quota failed", err)
		return
	}
	writeJSON(w, map[string]any{"agent": agent, "status": "deleted"})
}

// --- Cycles CRUD ---

func (r *Relay) apiGetCycles(w http.ResponseWriter, req *http.Request) {
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	cycles, err := r.DB.ListCycles(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list cycles", err)
		return
	}
	if cycles == nil {
		cycles = []models.Cycle{}
	}
	writeJSON(w, cycles)
}

func (r *Relay) apiGetCycle(w http.ResponseWriter, req *http.Request, path string) {
	name := strings.TrimPrefix(path, "/cycles/")
	name = strings.TrimSuffix(name, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	cycle, err := r.DB.GetCycle(project, name)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "get cycle", err)
		return
	}
	if cycle == nil {
		apiError(w, http.StatusNotFound, "cycle not found", nil)
		return
	}
	writeJSON(w, cycle)
}

func (r *Relay) apiCreateCycle(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project string `json:"project"`
		Name    string `json:"name"`
		Prompt  string `json:"prompt"`
		TTL     int    `json:"ttl"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	if body.Name == "" {
		apiError(w, http.StatusBadRequest, "name required", nil)
		return
	}
	cycle, err := r.DB.UpsertCycle(body.Project, body.Name, body.Prompt, body.TTL)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create cycle", err)
		return
	}
	writeJSON(w, cycle)
}

func (r *Relay) apiUpdateCycle(w http.ResponseWriter, req *http.Request, path string) {
	name := strings.TrimPrefix(path, "/cycles/")
	name = strings.TrimSuffix(name, "/")
	var body struct {
		Project string `json:"project"`
		Prompt  string `json:"prompt"`
		TTL     int    `json:"ttl"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}
	cycle, err := r.DB.UpsertCycle(body.Project, name, body.Prompt, body.TTL)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "update cycle", err)
		return
	}
	writeJSON(w, cycle)
}

func (r *Relay) apiDeleteCycle(w http.ResponseWriter, req *http.Request, path string) {
	name := strings.TrimPrefix(path, "/cycles/")
	name = strings.TrimSuffix(name, "/")
	project := req.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	if err := r.DB.DeleteCycle(project, name); err != nil {
		apiError(w, http.StatusInternalServerError, "delete cycle", err)
		return
	}
	writeJSON(w, map[string]any{"name": name, "status": "deleted"})
}
