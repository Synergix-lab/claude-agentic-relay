package db

import (
	"agent-relay/internal/models"
	"fmt"
)

// ListAllConversations returns all non-archived conversations with member names for a project.
func (d *DB) ListAllConversations(project string) ([]models.ConversationWithMembers, error) {
	query := `
		SELECT c.id, c.title, c.created_by, c.created_at,
			(SELECT COUNT(*) FROM messages WHERE conversation_id = c.id) AS message_count
		FROM conversations c
		WHERE c.archived_at IS NULL AND c.project = ?
		ORDER BY c.created_at DESC
	`

	rows, err := d.conn.Query(query, project)
	if err != nil {
		return nil, fmt.Errorf("list all conversations: %w", err)
	}
	defer rows.Close()

	var convs []models.ConversationWithMembers
	for rows.Next() {
		var c models.ConversationWithMembers
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedBy, &c.CreatedAt, &c.MessageCount); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.Project = project
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch members for each conversation
	for i := range convs {
		members, err := d.GetConversationMembers(convs[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get members for %s: %w", convs[i].ID, err)
		}
		for _, m := range members {
			convs[i].Members = append(convs[i].Members, m.AgentName)
		}
	}

	return convs, nil
}

// GetAllRecentMessages returns the most recent messages across all conversations for a project.
func (d *DB) GetAllRecentMessages(project string, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 200
	}

	query := `
		SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project
		FROM messages
		WHERE project = ?
		ORDER BY created_at ASC
		LIMIT ?
	`
	return d.queryMessages(query, project, limit)
}

// GetMessagesSince returns all messages created after the given RFC3339 timestamp for a project.
func (d *DB) GetMessagesSince(project, since string, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, from_agent, to_agent, reply_to, type, subject, content, metadata, created_at, read_at, conversation_id, project
		FROM messages
		WHERE project = ? AND created_at > ?
		ORDER BY created_at ASC
		LIMIT ?
	`
	return d.queryMessages(query, project, since, limit)
}

// ListProjects returns all distinct project names across agents, messages, and conversations.
func (d *DB) ListProjects() ([]string, error) {
	rows, err := d.conn.Query(`
		SELECT DISTINCT project FROM (
			SELECT project FROM agents
			UNION
			SELECT project FROM messages
			UNION
			SELECT project FROM conversations
		) ORDER BY project
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
