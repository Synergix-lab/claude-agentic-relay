package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// --- Vault config (per-project) ---

func (d *DB) RegisterVault(project, path string) error {
	now := time.Now().UTC().Format(memoryTimeFmt)
	_, err := d.conn.Exec(`
		INSERT INTO vaults (project, path, created_at) VALUES (?, ?, ?)
		ON CONFLICT(project) DO UPDATE SET path = excluded.path`,
		project, path, now,
	)
	return err
}

func (d *DB) GetVaultConfig(project string) (*models.VaultConfig, error) {
	var cfg models.VaultConfig
	err := d.ro().QueryRow("SELECT project, path FROM vaults WHERE project = ?", project).Scan(&cfg.Project, &cfg.Path)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (d *DB) ListVaultConfigs() ([]models.VaultConfig, error) {
	rows, err := d.ro().Query("SELECT project, path FROM vaults ORDER BY project")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var configs []models.VaultConfig
	for rows.Next() {
		var cfg models.VaultConfig
		if err := rows.Scan(&cfg.Project, &cfg.Path); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// --- Vault docs ---

const vaultDocColumns = "path, project, title, owner, status, tags, content, size_bytes, updated_at, indexed_at"

func scanVaultDoc(row interface{ Scan(...any) error }) (models.VaultDoc, error) {
	var d models.VaultDoc
	err := row.Scan(&d.Path, &d.Project, &d.Title, &d.Owner, &d.Status, &d.Tags, &d.Content, &d.SizeBytes, &d.UpdatedAt, &d.IndexedAt)
	return d, err
}

func (d *DB) UpsertVaultDoc(doc *models.VaultDoc) error {
	now := time.Now().UTC().Format(memoryTimeFmt)
	doc.IndexedAt = now

	_, err := d.conn.Exec(`
		INSERT INTO vault_docs (path, project, title, owner, status, tags, content, size_bytes, updated_at, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path, project) DO UPDATE SET
			title = excluded.title,
			owner = excluded.owner,
			status = excluded.status,
			tags = excluded.tags,
			content = excluded.content,
			size_bytes = excluded.size_bytes,
			updated_at = excluded.updated_at,
			indexed_at = excluded.indexed_at`,
		doc.Path, doc.Project, doc.Title, doc.Owner, doc.Status, doc.Tags, doc.Content, doc.SizeBytes, doc.UpdatedAt, doc.IndexedAt,
	)
	return err
}

func (d *DB) DeleteVaultDoc(project, path string) error {
	_, err := d.conn.Exec("DELETE FROM vault_docs WHERE project = ? AND path = ?", project, path)
	return err
}

func (d *DB) GetVaultDoc(project, path string) (*models.VaultDoc, error) {
	// Try project-specific first, then fall back to _relay
	doc, err := scanVaultDoc(d.ro().QueryRow(
		"SELECT "+vaultDocColumns+" FROM vault_docs WHERE project IN (?, '_relay') AND path = ? ORDER BY CASE WHEN project = ? THEN 0 ELSE 1 END LIMIT 1",
		project, path, project,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get vault doc: %w", err)
	}
	return &doc, nil
}

func (d *DB) SearchVault(project, query string, tags []string, limit int) ([]models.VaultSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build FTS5 query
	q := `
		SELECT vd.path, vd.title, vd.owner, vd.tags,
			snippet(vault_docs_fts, 3, '>>>', '<<<', '...', 40) as excerpt,
			rank
		FROM vault_docs_fts
		JOIN vault_docs vd ON vd.rowid = vault_docs_fts.rowid
		WHERE vault_docs_fts MATCH ?
		AND vd.project IN (?, '_relay')`

	args := []any{escapeFTSQuery(query), project}

	if len(tags) > 0 {
		for _, tag := range tags {
			q += " AND vd.tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
	}

	q += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search vault: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []models.VaultSearchResult
	for rows.Next() {
		var r models.VaultSearchResult
		if err := rows.Scan(&r.Path, &r.Title, &r.Owner, &r.Tags, &r.Excerpt, &r.Score); err != nil {
			return nil, fmt.Errorf("scan vault search result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (d *DB) ListVaultDocs(project string, tags []string, limit int) ([]models.VaultDoc, error) {
	if limit <= 0 {
		limit = 100
	}

	q := "SELECT " + vaultDocColumns + " FROM vault_docs WHERE project IN (?, '_relay')"
	args := []any{project}

	if len(tags) > 0 {
		for _, tag := range tags {
			q += " AND tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
	}

	q += " ORDER BY path LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list vault docs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var docs []models.VaultDoc
	for rows.Next() {
		doc, err := scanVaultDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan vault doc: %w", err)
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// ListAllVaultDocs returns vault docs across all projects (metadata only, no content).
func (d *DB) ListAllVaultDocs(limit int) ([]models.VaultDoc, error) {
	if limit <= 0 {
		limit = 500
	}
	q := "SELECT " + vaultDocColumns + " FROM vault_docs ORDER BY project, path LIMIT ?"
	rows, err := d.ro().Query(q, limit)
	if err != nil {
		return nil, fmt.Errorf("list all vault docs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var docs []models.VaultDoc
	for rows.Next() {
		doc, err := scanVaultDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan vault doc: %w", err)
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// GetVaultDocsByPaths returns vault docs matching the given paths (supports glob-like prefix matching with *).
func (d *DB) GetVaultDocsByPaths(project string, patterns []string, maxTotalBytes int) ([]models.VaultDoc, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	var docs []models.VaultDoc
	totalBytes := 0

	for _, pattern := range patterns {
		var rows *sql.Rows
		var err error

		if strings.Contains(pattern, "*") {
			// Convert glob to SQL LIKE: "guides/supabase-*.md" → "guides/supabase-%.md"
			like := strings.ReplaceAll(pattern, "*", "%")
			rows, err = d.ro().Query(
				"SELECT "+vaultDocColumns+" FROM vault_docs WHERE project = ? AND path LIKE ? ORDER BY path",
				project, like,
			)
		} else {
			rows, err = d.ro().Query(
				"SELECT "+vaultDocColumns+" FROM vault_docs WHERE project = ? AND path = ?",
				project, pattern,
			)
		}

		if err != nil {
			continue
		}

		for rows.Next() {
			doc, err := scanVaultDoc(rows)
			if err != nil {
				_ = rows.Close()
				break
			}
			if maxTotalBytes > 0 && totalBytes+doc.SizeBytes > maxTotalBytes {
				_ = rows.Close()
				return docs, nil // budget exhausted
			}
			totalBytes += doc.SizeBytes
			docs = append(docs, doc)
		}
		_ = rows.Close()
	}

	return docs, nil
}

// GetVaultDocsIndex returns path + title only (no content) for matching patterns. Used for prompt injection.
func (d *DB) GetVaultDocsIndex(project string, patterns []string) ([]models.VaultDocRef, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	var refs []models.VaultDocRef
	for _, pattern := range patterns {
		var rows *sql.Rows
		var err error

		if strings.Contains(pattern, "*") {
			like := strings.ReplaceAll(pattern, "*", "%")
			rows, err = d.ro().Query(
				"SELECT path, title FROM vault_docs WHERE project = ? AND path LIKE ? ORDER BY path",
				project, like,
			)
		} else {
			rows, err = d.ro().Query(
				"SELECT path, title FROM vault_docs WHERE project = ? AND path = ?",
				project, pattern,
			)
		}
		if err != nil {
			continue
		}
		for rows.Next() {
			var ref models.VaultDocRef
			if err := rows.Scan(&ref.Path, &ref.Title); err != nil {
				break
			}
			refs = append(refs, ref)
		}
		_ = rows.Close()
	}
	return refs, nil
}

// GetVaultDocsByTags returns vault docs where tags match any of the given tags.
func (d *DB) GetVaultDocsByTags(project string, tags []string, maxTotalBytes int) ([]models.VaultDoc, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	conditions := make([]string, len(tags))
	args := []any{project}
	for i, tag := range tags {
		conditions[i] = "tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}

	q := "SELECT " + vaultDocColumns + " FROM vault_docs WHERE project = ? AND (" + strings.Join(conditions, " OR ") + ") ORDER BY size_bytes ASC"
	rows, err := d.ro().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get vault docs by tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var docs []models.VaultDoc
	totalBytes := 0
	for rows.Next() {
		doc, err := scanVaultDoc(rows)
		if err != nil {
			return nil, err
		}
		if maxTotalBytes > 0 && totalBytes+doc.SizeBytes > maxTotalBytes {
			return docs, nil
		}
		totalBytes += doc.SizeBytes
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (d *DB) GetVaultStats(project string) (int, int, error) {
	var count, totalSize int
	err := d.ro().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM vault_docs WHERE project = ?",
		project,
	).Scan(&count, &totalSize)
	return count, totalSize, err
}
