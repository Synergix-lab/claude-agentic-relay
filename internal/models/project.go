package models

type Project struct {
	Name       string `json:"name"`
	PlanetType string `json:"planet_type"`
	CreatedAt  string `json:"created_at"`
}

type ProjectInfo struct {
	Name        string `json:"name"`
	PlanetType  string `json:"planet_type"`
	CreatedAt   string `json:"created_at"`
	AgentCount  int    `json:"agent_count"`
	OnlineCount int    `json:"online_count"`
	TotalTasks  int    `json:"total_tasks"`
	ActiveTasks int    `json:"active_tasks"`
	DoneTasks   int    `json:"done_tasks"`
	Tokens24h   int64  `json:"tokens_24h"`
}
