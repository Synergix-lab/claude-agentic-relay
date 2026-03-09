package db

import (
	"agent-relay/internal/models"
	"agent-relay/internal/normalize"
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
		Content:        normalize.JSONKeys(content),
		Metadata:       normalize.JSONKeys(metadata),
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
		SELECT m.id, m.from_agent, m.to_agent, m.reply_to, m.type, m.subject, m.content, m.metadata, m.created_at, m.read_at, m.conversation_id, m.project, m.task_id
		FROM messages m
		WHERE m.project = ?
			AND (
				-- Legacy 1-1 and broadcast
				(m.conversation_id IS NULL AND (m.to_agent = ? OR (m.to_agent = '*' AND m.from_agent != ?)))
				-- Conversations where I'm a member
				OR (m.conversation_id IS NOT NULL AND m.conversation_id IN (
					SELECT conversation_id FROM conversation_members
					WHERE agent_name = ? AND left_at IS NULL
				) AND m.from_agent != ?)
			)
	`
	args := []any{project, agentName, agentName, agentName, agentName}

	if unreadOnly {
		query += ` AND NOT EXISTS (
			SELECT 1 FROM message_reads mr WHERE mr.message_id = m.id AND mr.agent_name = ?
		)`
		args = append(args, agentName)
	}

	query += " ORDER BY m.created_at DESC LIMIT ?"
	args = append(args, limit)

	return d.queryMessages(query, args...)
}

func (d *DB) GetThread(messageID string) ([]models.Message, error) {
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

	query := `
		WITH RECURSIVE thread AS (
			SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project, task_id
			FROM messages WHERE id = ?
			UNION ALL
			SELECT m.id, m.from_agent, m.to_agent, m.reply_to, m.type, m.subject, m.content, m.metadata, m.created_at, m.read_at, m.conversation_id, m.project, m.task_id
			FROM messages m
			JOIN thread t ON m.reply_to = t.id
		)
		SELECT * FROM thread ORDER BY created_at ASC
	`

	return d.queryMessages(query, rootID)
}

func (d *DB) MarkRead(messageIDs []string, agentName, project string) (int, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	for _, id := range messageIDs {
		result, err := d.conn.Exec(
			"INSERT OR IGNORE INTO message_reads (message_id, agent_name, project, read_at) VALUES (?, ?, ?, ?)",
			id, agentName, project, now,
		)
		if err != nil {
			return count, fmt.Errorf("mark read: %w", err)
		}
		n, _ := result.RowsAffected()
		count += int(n)
	}

	// Also update conversation_reads for any conversation messages
	convPlaceholders := ""
	convArgs := make([]any, 0, len(messageIDs))
	for i, id := range messageIDs {
		if i > 0 {
			convPlaceholders += ","
		}
		convPlaceholders += "?"
		convArgs = append(convArgs, id)
	}
	convRows, err := d.conn.Query(
		fmt.Sprintf("SELECT DISTINCT conversation_id FROM messages WHERE id IN (%s) AND conversation_id IS NOT NULL", convPlaceholders),
		convArgs...,
	)
	if err == nil {
		var convIDs []string
		for convRows.Next() {
			var convID string
			if err := convRows.Scan(&convID); err == nil {
				convIDs = append(convIDs, convID)
			}
		}
		convRows.Close()
		for _, convID := range convIDs {
			_ = d.MarkConversationRead(convID, agentName)
		}
	}

	return count, nil
}

func (d *DB) GetMessage(id string) (*models.Message, error) {
	msgs, err := d.queryMessages(
		"SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project, task_id FROM messages WHERE id = ?",
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
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.ReplyTo, &m.Type, &m.Subject, &m.Content, &m.Metadata, &m.CreatedAt, &m.ReadAt, &m.ConversationID, &m.Project, &m.TaskID); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
