package models

type Profile struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	ContextPack string  `json:"context_pack"`
	SoulKeys    string  `json:"soul_keys"`   // JSON array
	Skills      string  `json:"skills"`      // JSON array of skill objects
	VaultPaths  string  `json:"vault_paths"` // JSON array of glob patterns
	Project     string  `json:"project"`
	OrgID       *string `json:"org_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}
