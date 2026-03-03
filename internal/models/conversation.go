package models

type Conversation struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	CreatedBy  string  `json:"created_by"`
	CreatedAt  string  `json:"created_at"`
	ArchivedAt *string `json:"archived_at,omitempty"`
	Project    string  `json:"project"`
}

type ConversationMember struct {
	ConversationID string  `json:"conversation_id"`
	AgentName      string  `json:"agent_name"`
	JoinedAt       string  `json:"joined_at"`
	LeftAt         *string `json:"left_at,omitempty"`
}

type ConversationSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	MemberCount int    `json:"member_count"`
	UnreadCount int    `json:"unread_count"`
	Project     string `json:"project"`
}
