package db

import (
	"agent-relay/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (d *DB) InsertMessage(project, from, to, msgType, subject, content, metadata string, replyTo, conversationID *string) (*models.Message, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	msg := &models.Message{
		ID:             uuid.New().String(),
		From:           from,
		To:             to,
		ReplyTo:        replyTo,
		Type:           msgType,
		Subject:        subject,
		Content:        content,
		Metadata:       metadata,
		CreatedAt:      now,
		ConversationID: conversationID,
		Project:        project,
	}

	_, err := d.conn.Exec(
		"INSERT INTO messages (id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, conversation_id, project) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.From, msg.To, msg.ReplyTo, msg.Type, msg.Subject, msg.Content, msg.Metadata, msg.CreatedAt, msg.ConversationID, msg.Project,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	return msg, nil
}

func (d *DB) GetInbox(project, agentName string, unreadOnly bool, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project
		FROM messages
		WHERE project = ?
			AND (
				-- Legacy 1-1 and broadcast
				(conversation_id IS NULL AND (to_agent = ? OR (to_agent = '*' AND from_agent != ?)))
				-- Conversations where I'm a member
				OR (conversation_id IS NOT NULL AND conversation_id IN (
					SELECT conversation_id FROM conversation_members
					WHERE agent_name = ? AND left_at IS NULL
				) AND from_agent != ?)
			)
	`
	args := []any{project, agentName, agentName, agentName, agentName}

	if unreadOnly {
		query += ` AND (
			(conversation_id IS NULL AND read_at IS NULL)
			OR (conversation_id IS NOT NULL AND created_at > COALESCE(
				(SELECT last_read_at FROM conversation_reads
				 WHERE conversation_id = messages.conversation_id AND agent_name = ?),
				'1970-01-01T00:00:00Z'
			))
		)`
		args = append(args, agentName)
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
			SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project
			FROM messages WHERE id = ?
			UNION ALL
			SELECT m.id, m.from_agent, m.to_agent, m.reply_to, m.type, m.subject, m.content, m.metadata, m.created_at, m.read_at, m.conversation_id, m.project
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
		"SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project FROM messages WHERE id = ?",
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

// FindMessageByPrefix resolves a short ID prefix to a full message ID.
// Returns the full ID if exactly one match is found.
func (d *DB) FindMessageByPrefix(prefix string) (string, error) {
	var ids []string
	rows, err := d.conn.Query("SELECT id FROM messages WHERE id LIKE ?", prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no message found with prefix %q", prefix)
	}
	if len(ids) > 1 {
		return "", fmt.Errorf("ambiguous prefix %q (%d matches)", prefix, len(ids))
	}
	return ids[0], nil
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
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.ReplyTo, &m.Type, &m.Subject, &m.Content, &m.Metadata, &m.CreatedAt, &m.ReadAt, &m.ConversationID, &m.Project); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
