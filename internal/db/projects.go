package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"math/rand"
	"time"
)

// Planet pool: category/variant pairs (48x48, 60 frames each).
var planetPool = []string{
	"barren/1", "barren/2", "barren/3", "barren/4",
	"desert/1", "desert/2",
	"forest/1", "forest/2",
	"gas_giant/1", "gas_giant/2", "gas_giant/3", "gas_giant/4",
	"ice/1",
	"lava/1", "lava/2", "lava/3",
	"ocean/1",
	"terran/1", "terran/2",
	"tundra/1", "tundra/2",
}

func randomPlanet() string {
	return planetPool[rand.Intn(len(planetPool))]
}

// EnsureProject creates a project entry if it doesn't exist, assigning a random planet.
func (d *DB) EnsureProject(name string) {
	now := time.Now().UTC().Format(time.RFC3339)
	d.conn.Exec(
		"INSERT OR IGNORE INTO projects (name, planet_type, created_at) VALUES (?, ?, ?)",
		name, randomPlanet(), now,
	)
}

// GetProject returns a project by name.
func (d *DB) GetProject(name string) (*models.Project, error) {
	var p models.Project
	err := d.conn.QueryRow("SELECT name, planet_type, created_at FROM projects WHERE name = ?", name).Scan(&p.Name, &p.PlanetType, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProjectPlanetType changes a project's planet_type.
func (d *DB) UpdateProjectPlanetType(name, planetType string) error {
	_, err := d.conn.Exec("UPDATE projects SET planet_type = ? WHERE name = ?", planetType, name)
	return err
}

// GetSetting returns a setting value by key.
func (d *DB) GetSetting(key string) string {
	var val string
	d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	return val
}

// SetSetting upserts a setting.
func (d *DB) SetSetting(key, value string) {
	d.conn.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?", key, value, value)
}

// DeleteProject removes a project and all its associated data (cascade delete).
func (d *DB) DeleteProject(name string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete all related data
	tables := []string{
		"notify_channels", "team_members", "teams", "orgs",
		"goals", "boards", "vault_docs", "vaults",
		"message_reads", "memories", "profiles",
		"tasks", "conversations", "messages", "agents",
	}
	for _, t := range tables {
		if _, err := tx.Exec("DELETE FROM "+t+" WHERE project = ?", name); err != nil {
			return fmt.Errorf("delete from %s: %w", t, err)
		}
	}

	// Delete the project itself
	res, err := tx.Exec("DELETE FROM projects WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %q not found", name)
	}

	return tx.Commit()
}

// ListProjectsWithInfo returns all projects with their planet_type and stats.
func (d *DB) ListProjectsWithInfo() ([]models.ProjectInfo, error) {
	rows, err := d.conn.Query(`
		SELECT p.name, p.planet_type, p.created_at,
			COALESCE(ac.agent_count, 0),
			COALESCE(ac.online_count, 0),
			COALESCE(tc.total_tasks, 0),
			COALESCE(tc.active_tasks, 0),
			COALESCE(tc.done_tasks, 0)
		FROM projects p
		LEFT JOIN (
			SELECT project, COUNT(*) as agent_count,
				SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END) as online_count
			FROM agents WHERE status IN ('active', 'sleeping', 'inactive')
			GROUP BY project
		) ac ON ac.project = p.name
		LEFT JOIN (
			SELECT project, COUNT(*) as total_tasks,
				SUM(CASE WHEN status IN ('accepted', 'in-progress') THEN 1 ELSE 0 END) as active_tasks,
				SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END) as done_tasks
			FROM tasks GROUP BY project
		) tc ON tc.project = p.name
		ORDER BY p.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.ProjectInfo
	for rows.Next() {
		var p models.ProjectInfo
		if err := rows.Scan(&p.Name, &p.PlanetType, &p.CreatedAt, &p.AgentCount, &p.OnlineCount, &p.TotalTasks, &p.ActiveTasks, &p.DoneTasks); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
