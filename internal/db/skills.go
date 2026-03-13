package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertSkill creates or updates a skill in the catalog.
func (d *DB) UpsertSkill(project, name, description, tags string) (*models.Skill, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)
	if tags == "" {
		tags = "[]"
	}

	var existingID string
	err := d.conn.QueryRow(`SELECT id FROM skills WHERE project = ? AND name = ?`, project, name).Scan(&existingID)
	if err == sql.ErrNoRows {
		id := uuid.New().String()
		_, err := d.conn.Exec(`INSERT INTO skills (id, project, name, description, tags, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, project, name, description, tags, now)
		if err != nil {
			return nil, fmt.Errorf("insert skill: %w", err)
		}
		return &models.Skill{ID: id, Project: project, Name: name, Description: description, Tags: tags, CreatedAt: now}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check skill: %w", err)
	}

	_, err = d.conn.Exec(`UPDATE skills SET description=?, tags=? WHERE id=?`, description, tags, existingID)
	if err != nil {
		return nil, fmt.Errorf("update skill: %w", err)
	}
	return &models.Skill{ID: existingID, Project: project, Name: name, Description: description, Tags: tags, CreatedAt: now}, nil
}

// ListSkills returns all skills for a project.
func (d *DB) ListSkills(project string) ([]models.Skill, error) {
	rows, err := d.ro().Query(`SELECT id, project, name, description, tags, created_at FROM skills WHERE project = ? ORDER BY name`, project)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var result []models.Skill
	for rows.Next() {
		var s models.Skill
		if err := rows.Scan(&s.ID, &s.Project, &s.Name, &s.Description, &s.Tags, &s.CreatedAt); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// LinkProfileSkill creates a link between a profile and a skill.
func (d *DB) LinkProfileSkill(profileID, skillID, proficiency string) error {
	if proficiency == "" {
		proficiency = "capable"
	}
	_, err := d.conn.Exec(`INSERT OR REPLACE INTO profile_skills (profile_id, skill_id, proficiency) VALUES (?, ?, ?)`,
		profileID, skillID, proficiency)
	return err
}

// FindProfilesBySkill returns profiles linked to a specific skill via the structured registry.
func (d *DB) FindProfilesBySkill(project, skillName string) ([]models.Profile, error) {
	rows, err := d.ro().Query(
		`SELECT `+profileColumns+` FROM profiles p
		 JOIN profile_skills ps ON ps.profile_id = p.id
		 JOIN skills s ON s.id = ps.skill_id
		 WHERE p.project = ? AND s.name = ?
		 ORDER BY CASE ps.proficiency WHEN 'expert' THEN 0 WHEN 'capable' THEN 1 ELSE 2 END, p.slug`,
		project, skillName,
	)
	if err != nil {
		return nil, fmt.Errorf("find profiles by skill: %w", err)
	}
	defer rows.Close()

	var profiles []models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			continue
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// FindBestProfileForSkill returns the best profile for a skill (expert first, then capable).
func (d *DB) FindBestProfileForSkill(project, skillName string) (*models.Profile, error) {
	profiles, err := d.FindProfilesBySkill(project, skillName)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		// Fallback to LIKE search on skills JSON
		fallback, err := d.FindProfilesBySkillTag(project, skillName)
		if err != nil || len(fallback) == 0 {
			return nil, nil
		}
		return &fallback[0], nil
	}
	return &profiles[0], nil
}

// GetSkillProfileLinks returns profiles linked to a skill with proficiency info.
func (d *DB) GetSkillProfileLinks(project, skillName string) ([]map[string]any, error) {
	rows, err := d.ro().Query(
		`SELECT p.slug, p.name, ps.proficiency FROM profiles p
		 JOIN profile_skills ps ON ps.profile_id = p.id
		 JOIN skills s ON s.id = ps.skill_id
		 WHERE p.project = ? AND s.name = ?
		 ORDER BY ps.proficiency, p.slug`,
		project, skillName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var slug, name, prof string
		if err := rows.Scan(&slug, &name, &prof); err != nil {
			continue
		}
		result = append(result, map[string]any{"slug": slug, "name": name, "proficiency": prof})
	}
	return result, rows.Err()
}
