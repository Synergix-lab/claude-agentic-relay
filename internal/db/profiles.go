package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const profileColumns = "id, slug, name, role, context_pack, soul_keys, skills, vault_paths, allowed_tools, pool_size, COALESCE(exit_prompt, ''), project, org_id, created_at, updated_at"

func scanProfile(row interface{ Scan(...any) error }) (models.Profile, error) {
	var p models.Profile
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Role, &p.ContextPack, &p.SoulKeys, &p.Skills, &p.VaultPaths, &p.AllowedTools, &p.PoolSize, &p.ExitPrompt, &p.Project, &p.OrgID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// ProfileOption sets optional fields when registering a profile.
type ProfileOption func(*models.Profile)

// WithAllowedTools sets the allowed tools for a profile.
func WithAllowedTools(tools string) ProfileOption {
	return func(p *models.Profile) { p.AllowedTools = tools }
}

// WithPoolSize sets the max concurrent spawns for a profile.
func WithPoolSize(size int) ProfileOption {
	return func(p *models.Profile) { p.PoolSize = size }
}

// WithExitPrompt overrides the default "when done, set_memory then exit"
// boilerplate appended to the spawn prompt. Use this to chain the child's
// final actions (message teammates, dispatch follow-ups) before exit.
func WithExitPrompt(prompt string) ProfileOption {
	return func(p *models.Profile) { p.ExitPrompt = prompt }
}

func (d *DB) RegisterProfile(project, slug, name, role, contextPack, soulKeys, skills, vaultPaths string, opts ...ProfileOption) (*models.Profile, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)
	if soulKeys == "" {
		soulKeys = "[]"
	}
	if skills == "" {
		skills = "[]"
	}
	if vaultPaths == "" {
		vaultPaths = "[]"
	}

	// Upsert: update if exists
	existing, err := scanProfile(d.conn.QueryRow(
		"SELECT "+profileColumns+" FROM profiles WHERE slug = ? AND project = ?",
		slug, project,
	))

	if err == sql.ErrNoRows {
		p := &models.Profile{
			ID:           uuid.New().String(),
			Slug:         slug,
			Name:         name,
			Role:         role,
			ContextPack:  contextPack,
			SoulKeys:     soulKeys,
			Skills:       skills,
			VaultPaths:   vaultPaths,
			AllowedTools: "[]",
			PoolSize:     3,
			Project:      project,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		for _, opt := range opts {
			opt(p)
		}
		_, err := d.conn.Exec(
			"INSERT INTO profiles (id, slug, name, role, context_pack, soul_keys, skills, vault_paths, allowed_tools, pool_size, exit_prompt, project, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			p.ID, p.Slug, p.Name, p.Role, p.ContextPack, p.SoulKeys, p.Skills, p.VaultPaths, p.AllowedTools, p.PoolSize, p.ExitPrompt, p.Project, p.CreatedAt, p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert profile: %w", err)
		}
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query profile: %w", err)
	}

	// Update existing — PATCH semantics. An empty/default parameter preserves
	// the existing value instead of wiping it. Previously a caller passing
	// only (slug, name, vault_paths) would nuke context_pack, role, exit_prompt,
	// allowed_tools, and pool_size back to empty.
	if name != "" {
		existing.Name = name
	}
	if role != "" {
		existing.Role = role
	}
	if contextPack != "" {
		existing.ContextPack = contextPack
	}
	if soulKeys != "" && soulKeys != "[]" {
		existing.SoulKeys = soulKeys
	}
	if skills != "" && skills != "[]" {
		existing.Skills = skills
	}
	if vaultPaths != "" && vaultPaths != "[]" {
		existing.VaultPaths = vaultPaths
	}
	existing.UpdatedAt = now
	// opts (WithAllowedTools, WithPoolSize, WithExitPrompt) always win when present.
	for _, opt := range opts {
		opt(&existing)
	}

	_, err = d.conn.Exec(
		"UPDATE profiles SET name = ?, role = ?, context_pack = ?, soul_keys = ?, skills = ?, vault_paths = ?, allowed_tools = ?, pool_size = ?, exit_prompt = ?, updated_at = ? WHERE slug = ? AND project = ?",
		existing.Name, existing.Role, existing.ContextPack, existing.SoulKeys, existing.Skills, existing.VaultPaths, existing.AllowedTools, existing.PoolSize, existing.ExitPrompt, now, slug, project,
	)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return &existing, nil
}

func (d *DB) GetProfile(project, slug string) (*models.Profile, error) {
	p, err := scanProfile(d.ro().QueryRow(
		"SELECT "+profileColumns+" FROM profiles WHERE slug = ? AND project = ?",
		slug, project,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

func (d *DB) ListProfiles(project string) ([]models.Profile, error) {
	rows, err := d.ro().Query(
		"SELECT "+profileColumns+" FROM profiles WHERE project = ? ORDER BY slug",
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var profiles []models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (d *DB) ListAllProfiles() ([]models.Profile, error) {
	rows, err := d.ro().Query(
		"SELECT " + profileColumns + " FROM profiles ORDER BY project, slug",
	)
	if err != nil {
		return nil, fmt.Errorf("list all profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var profiles []models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// DeleteProfile removes a profile by slug and project.
func (d *DB) DeleteProfile(project, slug string) error {
	_, err := d.conn.Exec("DELETE FROM profiles WHERE slug = ? AND project = ?", slug, project)
	return err
}

// FindProfilesBySkillTag returns profiles whose skills JSON contains the given tag.
func (d *DB) FindProfilesBySkillTag(project, tag string) ([]models.Profile, error) {
	// SQLite JSON: search in the skills JSON array for objects containing the tag
	rows, err := d.ro().Query(
		`SELECT `+profileColumns+` FROM profiles
		 WHERE project = ? AND skills LIKE ?
		 ORDER BY slug`,
		project, "%"+tag+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("find profiles by skill tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var profiles []models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}
