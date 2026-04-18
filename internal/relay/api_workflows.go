package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agent-relay/internal/db"
)

// --- Workflow API endpoints ---

// GET /api/workflows?project=X
func (r *Relay) apiGetWorkflows(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)
	workflows, err := r.DB.ListWorkflows(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list workflows failed", err)
		return
	}
	if workflows == nil {
		workflows = []db.Workflow{}
	}
	writeJSON(w, workflows)
}

// GET /api/workflows/:id
func (r *Relay) apiGetWorkflow(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/workflows/")
	id = strings.TrimSuffix(id, "/")
	wf, err := r.DB.GetWorkflow(id)
	if err != nil {
		apiError(w, http.StatusNotFound, "workflow not found", err)
		return
	}
	writeJSON(w, wf)
}

// POST /api/workflows
func (r *Relay) apiCreateWorkflow(w http.ResponseWriter, req *http.Request) {
	// Decode into a raw map first so we can warn on unknown fields (e.g. the
	// common mistake of sending trigger_type/definition from a stale docs draft).
	var raw map[string]any
	if err := json.NewDecoder(req.Body).Decode(&raw); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}
	var unknown []string
	for k := range raw {
		switch k {
		case "project", "name", "description", "nodes", "edges":
			// known fields, ignore
		default:
			unknown = append(unknown, k)
		}
	}

	body := struct {
		Project     string `json:"project"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Nodes       string `json:"nodes"`
		Edges       string `json:"edges"`
	}{
		Project:     asString(raw["project"]),
		Name:        asString(raw["name"]),
		Description: asString(raw["description"]),
		Nodes:       asString(raw["nodes"]),
		Edges:       asString(raw["edges"]),
	}

	if body.Project == "" {
		body.Project = "default"
	}
	if body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if body.Nodes == "" {
		body.Nodes = "[]"
	}
	if body.Edges == "" {
		body.Edges = "[]"
	}

	wf, err := r.DB.CreateWorkflow(body.Project, body.Name, body.Description, body.Nodes, body.Edges)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create workflow failed", err)
		return
	}

	// Attach a warning header so curl callers see unknown-field hints without
	// breaking the JSON contract for UI clients.
	if len(unknown) > 0 {
		w.Header().Set("Warning", "299 - \"unknown fields ignored: "+strings.Join(unknown, ", ")+"\"")
	}

	b, _ := json.Marshal(wf)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// asString tolerantly coerces any value to a string (empty on nil/type mismatch).
// Used when decoding workflow payloads where clients may send number/string.
func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// PUT /api/workflows/:id
func (r *Relay) apiUpdateWorkflow(w http.ResponseWriter, req *http.Request, path string) {
	id := strings.TrimPrefix(path, "/workflows/")
	id = strings.TrimSuffix(id, "/")

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Nodes       string `json:"nodes"`
		Edges       string `json:"edges"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON", err)
		return
	}

	// Handle enable/disable toggle
	if body.Enabled != nil {
		r.DB.ToggleWorkflow(id, *body.Enabled)
	}

	wf, err := r.DB.UpdateWorkflow(id, body.Name, body.Description, body.Nodes, body.Edges)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "update workflow failed", err)
		return
	}
	writeJSON(w, wf)
}

// DELETE /api/workflows/:id
func (r *Relay) apiDeleteWorkflow(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/workflows/")
	id = strings.TrimSuffix(id, "/")
	if err := r.DB.DeleteWorkflow(id); err != nil {
		apiError(w, http.StatusInternalServerError, "delete workflow failed", err)
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "deleted"})
}

// POST /api/workflows/:id/execute
func (r *Relay) apiExecuteWorkflow(w http.ResponseWriter, req *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/workflows/"), "/execute")

	wf, err := r.DB.GetWorkflow(id)
	if err != nil {
		apiError(w, http.StatusNotFound, "workflow not found", err)
		return
	}

	// Parse optional trigger meta from request body
	meta := make(map[string]string)
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		if len(bodyBytes) > 0 {
			var parsed struct {
				Meta map[string]string `json:"meta"`
			}
			if err := json.Unmarshal(bodyBytes, &parsed); err == nil && parsed.Meta != nil {
				meta = parsed.Meta
			}
		}
	}

	if r.WorkflowEngine == nil {
		http.Error(w, `{"error":"workflow engine not available"}`, http.StatusServiceUnavailable)
		return
	}

	run, err := r.WorkflowEngine.Execute(req.Context(), wf, "manual", meta)
	if err != nil {
		// Run was created but failed — return the run with error
		if run != nil {
			writeJSON(w, run)
			return
		}
		apiError(w, http.StatusInternalServerError, "execute failed", err)
		return
	}

	writeJSON(w, run)
}

// GET /api/workflows/:id/runs?limit=N
func (r *Relay) apiGetWorkflowRuns(w http.ResponseWriter, req *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/workflows/"), "/runs")

	limit := 20
	if l := req.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}

	runs, err := r.DB.ListWorkflowRuns(id, limit)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list runs failed", err)
		return
	}
	if runs == nil {
		runs = []db.WorkflowRun{}
	}
	writeJSON(w, runs)
}

// GET /api/workflow-runs/:id
func (r *Relay) apiGetWorkflowRunDetail(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/workflow-runs/")
	id = strings.TrimSuffix(id, "/")

	run, err := r.DB.GetWorkflowRun(id)
	if err != nil {
		apiError(w, http.StatusNotFound, "run not found", err)
		return
	}

	nodeRuns, err := r.DB.ListNodeRuns(id)
	if err != nil {
		nodeRuns = []db.WorkflowNodeRun{}
	}

	writeJSON(w, map[string]any{
		"run":       run,
		"node_runs": nodeRuns,
	})
}

// --- Custom Events API ---

// GET /api/custom-events?project=X
func (r *Relay) apiGetCustomEvents(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)
	events, err := r.DB.ListCustomEvents(project)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "list custom events failed", err)
		return
	}
	if events == nil {
		events = []db.CustomEvent{}
	}
	writeJSON(w, events)
}

// POST /api/custom-events
func (r *Relay) apiCreateCustomEvent(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project     string `json:"project"`
		Name        string `json:"name"`
		Description string `json:"description"`
		MetaFields  string `json:"meta_fields"` // JSON array: ["branch","status"]
		Icon        string `json:"icon"`
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
	evt, err := r.DB.UpsertCustomEvent(body.Project, body.Name, body.Description, body.MetaFields, body.Icon)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "create custom event failed", err)
		return
	}
	b, _ := json.Marshal(evt)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
}

// DELETE /api/custom-events/:id
func (r *Relay) apiDeleteCustomEvent(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/custom-events/")
	id = strings.TrimSuffix(id, "/")
	r.DB.DeleteCustomEvent(id)
	writeJSON(w, map[string]any{"id": id, "status": "deleted"})
}

// parseInt is a small helper for query param parsing.
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
