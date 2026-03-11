package db

import (
	"agent-relay/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateDeliveries creates delivery records for a message to the specified recipients.
func (d *DB) CreateDeliveries(messageID, project string, recipients []string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	for i, agent := range recipients {
		_, err := d.conn.Exec(
			"INSERT INTO deliveries (id, message_id, to_agent, state, sequence_number, created_at, project) VALUES (?, ?, ?, 'queued', ?, ?, ?)",
			uuid.New().String(), messageID, agent, i, now, project,
		)
		if err != nil {
			return fmt.Errorf("create delivery for %s: %w", agent, err)
		}
	}
	return nil
}

// GetInboxViaDeliveries returns messages for an agent using the deliveries table.
// It marks returned deliveries as 'surfaced'.
func (d *DB) GetInboxViaDeliveries(project, agentName string, unreadOnly bool, limit int, filters ...InboxFilter) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	var f InboxFilter
	if len(filters) > 0 {
		f = filters[0]
	}

	query := `
		SELECT m.id, m.from_agent, m.to_agent, m.reply_to, m.type, m.subject, m.content, m.metadata,
		       m.created_at, m.read_at, m.conversation_id, m.project, m.task_id, m.priority, m.ttl_seconds, m.expired_at,
		       d.id, d.state
		FROM deliveries d
		JOIN messages m ON d.message_id = m.id
		WHERE d.project = ? AND d.to_agent = ?
		  AND d.state IN ('queued', 'surfaced')
		  AND m.expired_at IS NULL
	`
	args := []any{project, agentName}

	if unreadOnly {
		query += " AND d.state = 'queued'"
	}
	if f.MinPriority != "" {
		query += " AND m.priority <= ?"
		args = append(args, f.MinPriority)
	}
	if f.From != "" {
		query += " AND m.from_agent = ?"
		args = append(args, f.From)
	}
	if f.Since != "" {
		query += " AND m.created_at >= ?"
		args = append(args, f.Since)
	}
	if f.ExcludeBroadcasts {
		query += " AND m.to_agent != '*'"
	}

	query += " ORDER BY m.priority ASC, m.created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get inbox via deliveries: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	var deliveryIDs []string
	for rows.Next() {
		var m models.Message
		var deliveryID, deliveryState string
		if err := rows.Scan(
			&m.ID, &m.From, &m.To, &m.ReplyTo, &m.Type, &m.Subject, &m.Content, &m.Metadata,
			&m.CreatedAt, &m.ReadAt, &m.ConversationID, &m.Project, &m.TaskID, &m.Priority, &m.TTLSeconds, &m.ExpiredAt,
			&deliveryID, &deliveryState,
		); err != nil {
			return nil, fmt.Errorf("scan delivery message: %w", err)
		}
		m.DeliveryID = &deliveryID
		m.DeliveryState = &deliveryState
		messages = append(messages, m)
		if deliveryState == "queued" {
			deliveryIDs = append(deliveryIDs, deliveryID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Mark queued deliveries as surfaced
	if len(deliveryIDs) > 0 {
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
		for _, id := range deliveryIDs {
			d.conn.Exec("UPDATE deliveries SET state = 'surfaced', surfaced_at = ? WHERE id = ? AND state = 'queued'", now, id)
		}
	}

	return messages, nil
}

// AcknowledgeDelivery marks a delivery as acknowledged.
func (d *DB) AcknowledgeDelivery(deliveryID string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	_, err := d.conn.Exec(
		"UPDATE deliveries SET state = 'acknowledged', acknowledged_at = ? WHERE id = ? AND state IN ('queued', 'surfaced')",
		now, deliveryID,
	)
	return err
}

// AcknowledgeDeliveryByMessage finds a delivery by message_id + agent and acknowledges it.
// Used for backward compat with mark_read.
func (d *DB) AcknowledgeDeliveryByMessage(messageID, agentName, project string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	_, err := d.conn.Exec(
		"UPDATE deliveries SET state = 'acknowledged', acknowledged_at = ? WHERE message_id = ? AND to_agent = ? AND project = ? AND state IN ('queued', 'surfaced')",
		now, messageID, agentName, project,
	)
	return err
}

// ExpireDeliveries marks deliveries for expired messages.
func (d *DB) ExpireDeliveries() (int, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	result, err := d.conn.Exec(
		`UPDATE deliveries SET state = 'expired', expired_at = ?
		 WHERE state IN ('queued', 'surfaced')
		   AND message_id IN (SELECT id FROM messages WHERE expired_at IS NOT NULL)`,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("expire deliveries: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// HasDeliveries returns true if the deliveries table has any rows.
func (d *DB) HasDeliveries() bool {
	var count int
	d.ro().QueryRow("SELECT COUNT(*) FROM deliveries LIMIT 1").Scan(&count)
	return count > 0
}

// ResolveRecipients determines the actual recipient agents for a message.
func (d *DB) ResolveRecipients(project, to, from string, conversationID *string) ([]string, error) {
	if conversationID != nil {
		// Conversation: all members except sender
		members, err := d.GetConversationMembers(*conversationID)
		if err != nil {
			return nil, err
		}
		var recipients []string
		for _, m := range members {
			if m.AgentName != from {
				recipients = append(recipients, m.AgentName)
			}
		}
		return recipients, nil
	}

	if to == "*" {
		// Broadcast: all active agents in project except sender
		agents, err := d.ListAgents(project)
		if err != nil {
			return nil, err
		}
		var recipients []string
		for _, a := range agents {
			if a.Name != from {
				recipients = append(recipients, a.Name)
			}
		}
		return recipients, nil
	}

	// Direct: single recipient
	return []string{to}, nil
}
