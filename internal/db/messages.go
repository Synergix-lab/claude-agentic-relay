package db

import (
	"agent-relay/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (d *DB) InsertMessage(from, to, msgType, subject, content, metadata string, replyTo *string) (*models.Message, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	msg := &models.Message{
		ID:        uuid.New().String(),
		From:      from,
		To:        to,
		ReplyTo:   replyTo,
		Type:      msgType,
		Subject:   subject,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: now,
	}

	_, err := d.conn.Exec(
		"INSERT INTO messages (id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.From, msg.To, msg.ReplyTo, msg.Type, msg.Subject, msg.Content, msg.Metadata, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	return msg, nil
}

func (d *DB) GetInbox(agentName string, unreadOnly bool, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at
		FROM messages
		WHERE (to_agent = ? OR (to_agent = '*' AND from_agent != ?))
	`
	args := []any{agentName, agentName}

	if unreadOnly {
		query += " AND read_at IS NULL"
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	return d.queryMessages(query, args...)
}

func (d *DB) GetThread(messageID string) ([]models.Message, error) {
	// Find the root message ID
	rootID := messageID
	for {
		var replyTo *string
		err := d.conn.QueryRow("SELECT reply_to FROM messages WHERE id = ?", rootID).Scan(&replyTo)
		if err != nil {
			break
		}
		if replyTo == nil {
			break
		}
		rootID = *replyTo
	}

	// Get root + all descendants
	query := `
		WITH RECURSIVE thread AS (
			SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at
			FROM messages WHERE id = ?
			UNION ALL
			SELECT m.id, m.from_agent, m.to_agent, m.reply_to, m.type, m.subject, m.content, m.metadata, m.created_at, m.read_at
			FROM messages m
			JOIN thread t ON m.reply_to = t.id
		)
		SELECT * FROM thread ORDER BY created_at ASC
	`

	return d.queryMessages(query, rootID)
}

func (d *DB) MarkRead(messageIDs []string, agentName string) (int, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Build placeholders
	placeholders := ""
	args := []any{now}
	for i, id := range messageIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	args = append(args, agentName, agentName)

	query := fmt.Sprintf(
		"UPDATE messages SET read_at = ? WHERE id IN (%s) AND (to_agent = ? OR (to_agent = '*' AND from_agent != ?)) AND read_at IS NULL",
		placeholders,
	)

	result, err := d.conn.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("mark read: %w", err)
	}

	n, _ := result.RowsAffected()
	return int(n), nil
}

func (d *DB) GetMessage(id string) (*models.Message, error) {
	msgs, err := d.queryMessages(
		"SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at FROM messages WHERE id = ?",
		id,
	)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return &msgs[0], nil
}

func (d *DB) queryMessages(query string, args ...any) ([]models.Message, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.ReplyTo, &m.Type, &m.Subject, &m.Content, &m.Metadata, &m.CreatedAt, &m.ReadAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
