package models

type Message struct {
	ID             string  `json:"id"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	ReplyTo        *string `json:"reply_to"`
	Type           string  `json:"type"`
	Subject        string  `json:"subject"`
	Content        string  `json:"content"`
	Metadata       string  `json:"metadata"`
	CreatedAt      string  `json:"created_at"`
	ReadAt         *string `json:"read_at"`
	ConversationID *string `json:"conversation_id,omitempty"`
}
