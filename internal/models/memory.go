package models

// Memory represents a persistent piece of agent knowledge.
type Memory struct {
	ID           string  `json:"id"`
	Key          string  `json:"key"`
	Value        string  `json:"value"`
	Tags         string  `json:"tags"`  // JSON array of strings
	Scope        string  `json:"scope"` // "agent", "project", "global"
	Project      string  `json:"project"`
	AgentName    string  `json:"agent_name"`
	Confidence   string  `json:"confidence"` // "stated", "inferred", "observed"
	Version      int     `json:"version"`
	Supersedes   *string `json:"supersedes,omitempty"`    // previous version's memory ID
	ConflictWith *string `json:"conflict_with,omitempty"` // conflicting memory ID
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	ArchivedAt   *string `json:"archived_at,omitempty"`
	ArchivedBy   *string `json:"archived_by,omitempty"`
	Layer        string  `json:"layer"` // "constraints", "behavior", "context"
}
