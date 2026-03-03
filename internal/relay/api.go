package relay

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"agent-relay/internal/models"
)

// ServeAPI handles REST API requests for the web UI.
func (r *Relay) ServeAPI(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(req.URL.Path, "/api")

	switch {
	case path == "/projects" && req.Method == http.MethodGet:
		r.apiGetProjects(w)
	case path == "/agents" && req.Method == http.MethodGet:
		r.apiGetAgents(w, req)
	case path == "/org" && req.Method == http.MethodGet:
		r.apiGetOrgTree(w, req)
	case path == "/conversations" && req.Method == http.MethodGet:
		r.apiGetConversations(w, req)
	case strings.HasPrefix(path, "/conversations/") && strings.HasSuffix(path, "/messages") && req.Method == http.MethodGet:
		r.apiGetConversationMessages(w, path)
	case path == "/messages/all" && req.Method == http.MethodGet:
		r.apiGetAllMessages(w, req)
	case path == "/messages/latest" && req.Method == http.MethodGet:
		r.apiGetLatestMessages(w, req)
	case path == "/user-response" && req.Method == http.MethodPost:
		r.apiPostUserResponse(w, req)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

// projectFromRequest extracts the ?project= query parameter, defaulting to "default".
func projectFromRequest(req *http.Request) string {
	p := req.URL.Query().Get("project")
	if p == "" {
		return "default"
	}
	return p
}

func (r *Relay) apiGetProjects(w http.ResponseWriter) {
	projects, err := r.DB.ListProjects()
	if err != nil {
		http.Error(w, `{"error":"failed to list projects"}`, http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []string{}
	}
	writeJSON(w, projects)
}

type apiAgent struct {
	Name         string  `json:"name"`
	Role         string  `json:"role"`
	Description  string  `json:"description"`
	LastSeen     string  `json:"last_seen"`
	RegisteredAt string  `json:"registered_at"`
	Online       bool    `json:"online"`
	ReportsTo    *string `json:"reports_to,omitempty"`
}

func (r *Relay) apiGetAgents(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)

	agents, err := r.DB.ListAgents(project)
	if err != nil {
		http.Error(w, `{"error":"failed to list agents"}`, http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	result := make([]apiAgent, 0, len(agents))
	for _, a := range agents {
		online := false
		if t, err := time.Parse(time.RFC3339, a.LastSeen); err == nil {
			online = now.Sub(t) < 5*time.Minute
		}
		result = append(result, apiAgent{
			Name:         a.Name,
			Role:         a.Role,
			Description:  a.Description,
			LastSeen:     a.LastSeen,
			RegisteredAt: a.RegisteredAt,
			Online:       online,
			ReportsTo:    a.ReportsTo,
		})
	}

	writeJSON(w, result)
}

func (r *Relay) apiGetConversations(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)

	convs, err := r.DB.ListAllConversations(project)
	if err != nil {
		http.Error(w, `{"error":"failed to list conversations"}`, http.StatusInternalServerError)
		return
	}

	if convs == nil {
		convs = make([]models.ConversationWithMembers, 0)
	}

	writeJSON(w, convs)
}

func (r *Relay) apiGetConversationMessages(w http.ResponseWriter, path string) {
	// path: /conversations/{id}/messages
	trimmed := strings.TrimPrefix(path, "/conversations/")
	convID, _, _ := strings.Cut(trimmed, "/")
	if convID == "" {
		http.Error(w, `{"error":"missing conversation id"}`, http.StatusBadRequest)
		return
	}

	msgs, err := r.DB.GetConversationMessages(convID, 200)
	if err != nil {
		http.Error(w, `{"error":"failed to get messages"}`, http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = make([]models.Message, 0)
	}

	writeJSON(w, msgs)
}

func (r *Relay) apiGetAllMessages(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)

	msgs, err := r.DB.GetAllRecentMessages(project, 500)
	if err != nil {
		http.Error(w, `{"error":"failed to get messages"}`, http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = make([]models.Message, 0)
	}

	writeJSON(w, msgs)
}

func (r *Relay) apiGetLatestMessages(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)
	since := req.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().Add(-30 * time.Second).Format("2006-01-02T15:04:05.000000Z")
	}

	msgs, err := r.DB.GetMessagesSince(project, since, 100)
	if err != nil {
		http.Error(w, `{"error":"failed to get messages"}`, http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = make([]models.Message, 0)
	}

	writeJSON(w, msgs)
}

// apiGetOrgTree returns the agent hierarchy as a nested tree structure.
func (r *Relay) apiGetOrgTree(w http.ResponseWriter, req *http.Request) {
	project := projectFromRequest(req)

	agents, err := r.DB.GetOrgTree(project)
	if err != nil {
		http.Error(w, `{"error":"failed to get org tree"}`, http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()

	type orgNode struct {
		Name    string     `json:"name"`
		Role    string     `json:"role"`
		Online  bool       `json:"online"`
		Reports []*orgNode `json:"reports"`
	}

	// Build a map of nodes and track children
	nodeMap := make(map[string]*orgNode, len(agents))
	for _, a := range agents {
		online := false
		if t, err := time.Parse(time.RFC3339, a.LastSeen); err == nil {
			online = now.Sub(t) < 5*time.Minute
		}
		nodeMap[a.Name] = &orgNode{
			Name:    a.Name,
			Role:    a.Role,
			Online:  online,
			Reports: []*orgNode{},
		}
	}

	// Build tree
	var roots []*orgNode
	for _, a := range agents {
		node := nodeMap[a.Name]
		if a.ReportsTo != nil {
			if parent, ok := nodeMap[*a.ReportsTo]; ok {
				parent.Reports = append(parent.Reports, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	if roots == nil {
		roots = []*orgNode{}
	}

	writeJSON(w, roots)
}

// apiPostUserResponse handles user responses from the web UI to agent questions.
func (r *Relay) apiPostUserResponse(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Project string `json:"project"`
		To      string `json:"to"`
		Content string `json:"content"`
		ReplyTo string `json:"reply_to"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if body.To == "" || body.Content == "" {
		http.Error(w, `{"error":"to and content are required"}`, http.StatusBadRequest)
		return
	}
	if body.Project == "" {
		body.Project = "default"
	}

	replyTo := optionalString(body.ReplyTo)

	msg, err := r.DB.InsertMessage(body.Project, "user", body.To, "response", "User response", body.Content, "{}", replyTo, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to send response"}`, http.StatusInternalServerError)
		return
	}

	// Push notification to the target agent
	r.Registry.Notify(body.Project, body.To, "user", "User response", msg.ID)

	writeJSON(w, map[string]any{"ok": true, "message_id": msg.ID})
}

func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"encode failed"}`, http.StatusInternalServerError)
	}
}
