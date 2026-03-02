package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (d *DB) RegisterAgent(name, role, description string) (*models.Agent, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Upsert: update if exists, insert if not
	var existing models.Agent
	err := d.conn.QueryRow("SELECT id, name, role, description, registered_at, last_seen FROM agents WHERE name = ?", name).
		Scan(&existing.ID, &existing.Name, &existing.Role, &existing.Description, &existing.RegisteredAt, &existing.LastSeen)

	if err == sql.ErrNoRows {
		agent := &models.Agent{
			ID:           uuid.New().String(),
			Name:         name,
			Role:         role,
			Description:  description,
			RegisteredAt: now,
			LastSeen:     now,
		}
		_, err := d.conn.Exec(
			"INSERT INTO agents (id, name, role, description, registered_at, last_seen) VALUES (?, ?, ?, ?, ?, ?)",
			agent.ID, agent.Name, agent.Role, agent.Description, agent.RegisteredAt, agent.LastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("insert agent: %w", err)
		}
		return agent, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query agent: %w", err)
	}

	// Update existing
	_, err = d.conn.Exec(
		"UPDATE agents SET role = ?, description = ?, last_seen = ? WHERE name = ?",
		role, description, now, name,
	)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	existing.Role = role
	existing.Description = description
	existing.LastSeen = now
	return &existing, nil
}

func (d *DB) TouchAgent(name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.conn.Exec("UPDATE agents SET last_seen = ? WHERE name = ?", now, name)
	return err
}

func (d *DB) ListAgents() ([]models.Agent, error) {
	rows, err := d.conn.Query("SELECT id, name, role, description, registered_at, last_seen FROM agents ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.Description, &a.RegisteredAt, &a.LastSeen); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (d *DB) GetAgent(name string) (*models.Agent, error) {
	var a models.Agent
	err := d.conn.QueryRow(
		"SELECT id, name, role, description, registered_at, last_seen FROM agents WHERE name = ?", name,
	).Scan(&a.ID, &a.Name, &a.Role, &a.Description, &a.RegisteredAt, &a.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return &a, nil
}
