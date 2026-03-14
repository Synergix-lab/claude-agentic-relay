package models

// Cycle defines a reusable execution template for agent spawns.
type Cycle struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	TTL       int    `json:"ttl"` // minutes
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
