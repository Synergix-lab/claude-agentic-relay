package models

type Agent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	RegisteredAt string `json:"registered_at"`
	LastSeen     string `json:"last_seen"`
}
