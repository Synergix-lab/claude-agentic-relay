package models

// ConversationWithMembers is used by the web API to return conversations with their member list.
type ConversationWithMembers struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	CreatedBy    string   `json:"created_by"`
	CreatedAt    string   `json:"created_at"`
	Members      []string `json:"members"`
	MessageCount int      `json:"message_count"`
	Project      string   `json:"project"`
}
