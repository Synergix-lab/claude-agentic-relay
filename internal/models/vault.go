package models

type VaultDoc struct {
	Path      string `json:"path"`
	Project   string `json:"project"`
	Title     string `json:"title"`
	Owner     string `json:"owner"`
	Status    string `json:"status"`
	Tags      string `json:"tags"` // JSON array
	Content   string `json:"content"`
	SizeBytes int    `json:"size_bytes"`
	UpdatedAt string `json:"updated_at"`
	IndexedAt string `json:"indexed_at"`
}

type VaultSearchResult struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Owner   string  `json:"owner"`
	Tags    string  `json:"tags"`
	Excerpt string  `json:"excerpt"`
	Score   float64 `json:"score"`
}

type VaultConfig struct {
	Path    string `json:"path"`
	Project string `json:"project"`
}
