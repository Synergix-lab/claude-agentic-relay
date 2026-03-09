package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
	path string
}

func New() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dbDir := filepath.Join(home, ".agent-relay")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dbPath := filepath.Join(dbDir, "relay.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-20000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Allow concurrent reads (WAL mode supports this), serialize writes only.
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{conn: conn, path: dbPath}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// Optimize runs PRAGMA optimize and a passive WAL checkpoint.
// Safe to call periodically (e.g. every 5 minutes).
func (d *DB) Optimize() {
	d.conn.Exec("PRAGMA optimize")
	d.conn.Exec("PRAGMA wal_checkpoint(PASSIVE)")
}

// DBPath returns the default database path without opening it.
func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-relay", "relay.db"), nil
}

// NewReadOnly opens the database in read-only mode for CLI queries.
// Does not run migrations or create the directory.
func NewReadOnly() (*DB, error) {
	dbPath, err := DBPath()
	if err != nil {
		return nil, fmt.Errorf("get db path: %w", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database not found at %s (relay never started?)", dbPath)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db readonly: %w", err)
	}

	conn.SetMaxOpenConns(1)

	return &DB{conn: conn, path: dbPath}, nil
}

// ensureColumns checks a table for missing columns and adds them via ALTER TABLE.
func ensureColumns(conn *sql.DB, table string, columns map[string]string) {
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		existing[name] = true
	}

	for col, def := range columns {
		if !existing[col] {
			conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, def))
		}
	}
}

func migrate(conn *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		role            TEXT NOT NULL DEFAULT '',
		description     TEXT NOT NULL DEFAULT '',
		registered_at   TEXT NOT NULL,
		last_seen       TEXT NOT NULL,
		project         TEXT NOT NULL DEFAULT 'default',
		reports_to      TEXT,
		profile_slug    TEXT,
		status          TEXT NOT NULL DEFAULT 'active',
		deactivated_at  TEXT,
		is_executive    INTEGER NOT NULL DEFAULT 0,
		session_id      TEXT
	);

	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		from_agent TEXT NOT NULL,
		to_agent   TEXT NOT NULL,
		reply_to   TEXT,
		type       TEXT NOT NULL DEFAULT 'notification',
		subject    TEXT NOT NULL DEFAULT '',
		content    TEXT NOT NULL,
		metadata   TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL,
		read_at    TEXT,
		project    TEXT NOT NULL DEFAULT 'default'
	);

	CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent);
	CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_agent);
	CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(to_agent, read_at) WHERE read_at IS NULL;
	CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(reply_to);

	-- Conversations
	CREATE TABLE IF NOT EXISTS conversations (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL,
		created_by  TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		archived_at TEXT,
		project     TEXT NOT NULL DEFAULT 'default'
	);

	CREATE TABLE IF NOT EXISTS conversation_members (
		conversation_id TEXT NOT NULL,
		agent_name      TEXT NOT NULL,
		joined_at       TEXT NOT NULL,
		left_at         TEXT,
		PRIMARY KEY (conversation_id, agent_name)
	);
	CREATE INDEX IF NOT EXISTS idx_conv_members_agent ON conversation_members(agent_name);

	CREATE TABLE IF NOT EXISTS conversation_reads (
		conversation_id TEXT NOT NULL,
		agent_name      TEXT NOT NULL,
		last_read_at    TEXT NOT NULL,
		PRIMARY KEY (conversation_id, agent_name)
	);
	`

	if _, err := conn.Exec(schema); err != nil {
		return err
	}

	// --- Ensure all columns exist on every table (safe for old and new DBs) ---

	ensureColumns(conn, "agents", map[string]string{
		"project":        "TEXT NOT NULL DEFAULT 'default'",
		"reports_to":     "TEXT",
		"profile_slug":   "TEXT",
		"status":         "TEXT NOT NULL DEFAULT 'active'",
		"deactivated_at": "TEXT",
		"is_executive":   "INTEGER NOT NULL DEFAULT 0",
		"session_id":     "TEXT",
		"org_id":         "TEXT",
	})

	// Projects table (planet_type assigned per project)
	conn.Exec(`CREATE TABLE IF NOT EXISTS projects (
		name        TEXT PRIMARY KEY,
		planet_type TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT ''
	)`)

	// Settings table (key-value, e.g. sun_type)
	conn.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`)

	// Backfill projects from existing agents
	backfillProjects(conn)

	ensureColumns(conn, "messages", map[string]string{
		"conversation_id": "TEXT",
		"project":         "TEXT NOT NULL DEFAULT 'default'",
		"task_id":         "TEXT",
	})

	ensureColumns(conn, "conversations", map[string]string{
		"project": "TEXT NOT NULL DEFAULT 'default'",
	})

	// Indexes (all idempotent)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project)`)
	conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_name ON agents(project, name)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_task ON messages(task_id)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_conversations_project ON conversations(project)`)

	// Remove old global UNIQUE constraint on agents.name (existing DBs only).
	migrateDropGlobalUnique(conn)

	// Memory system
	if err := migrateMemories(conn); err != nil {
		return fmt.Errorf("migrate memories: %w", err)
	}

	// Per-agent read receipts
	conn.Exec(`CREATE TABLE IF NOT EXISTS message_reads (
		message_id TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		project    TEXT NOT NULL DEFAULT 'default',
		read_at    TEXT NOT NULL,
		UNIQUE(message_id, agent_name)
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_message_reads_agent ON message_reads(agent_name, project)`)

	// Profiles
	conn.Exec(`CREATE TABLE IF NOT EXISTS profiles (
		id           TEXT PRIMARY KEY,
		slug         TEXT NOT NULL,
		name         TEXT NOT NULL,
		role         TEXT NOT NULL DEFAULT '',
		context_pack TEXT NOT NULL DEFAULT '',
		soul_keys    TEXT NOT NULL DEFAULT '[]',
		project      TEXT NOT NULL DEFAULT 'default',
		org_id       TEXT,
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL
	)`)
	conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_project_slug ON profiles(project, slug)`)
	ensureColumns(conn, "profiles", map[string]string{
		"skills":      "TEXT NOT NULL DEFAULT '[]'",
		"vault_paths": "TEXT NOT NULL DEFAULT '[]'",
	})

	// Tasks
	conn.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		id              TEXT PRIMARY KEY,
		profile_slug    TEXT NOT NULL,
		assigned_to     TEXT,
		dispatched_by   TEXT NOT NULL,
		title           TEXT NOT NULL,
		description     TEXT NOT NULL DEFAULT '',
		priority        TEXT NOT NULL DEFAULT 'P2',
		status          TEXT NOT NULL DEFAULT 'pending',
		result          TEXT,
		blocked_reason  TEXT,
		project         TEXT NOT NULL DEFAULT 'default',
		dispatched_at   TEXT NOT NULL,
		accepted_at     TEXT,
		started_at      TEXT,
		completed_at    TEXT,
		reply_to_task   TEXT
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project, status)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_profile ON tasks(project, profile_slug)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(project, priority, status)`)
	ensureColumns(conn, "tasks", map[string]string{
		"ack_notified_at":  "TEXT",
		"ack_escalated_at": "TEXT",
		"parent_task_id":   "TEXT",
		"board_id":         "TEXT",
		"goal_id":          "TEXT",
		"archived_at":      "TEXT",
	})
	// Migrate legacy reply_to_task -> parent_task_id
	conn.Exec(`UPDATE tasks SET parent_task_id = reply_to_task WHERE reply_to_task IS NOT NULL AND parent_task_id IS NULL`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_board ON tasks(board_id)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_goal ON tasks(goal_id)`)

	// Teams + Orgs
	conn.Exec(`CREATE TABLE IF NOT EXISTS orgs (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		slug        TEXT UNIQUE NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS teams (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		slug           TEXT NOT NULL,
		org_id         TEXT,
		project        TEXT NOT NULL DEFAULT 'default',
		description    TEXT NOT NULL DEFAULT '',
		type           TEXT NOT NULL DEFAULT 'regular',
		parent_team_id TEXT,
		created_at     TEXT NOT NULL
	)`)
	conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_teams_project_slug ON teams(project, slug)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_teams_org ON teams(org_id)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS team_members (
		team_id    TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		project    TEXT NOT NULL DEFAULT 'default',
		role       TEXT NOT NULL DEFAULT 'member',
		joined_at  TEXT NOT NULL,
		left_at    TEXT,
		PRIMARY KEY (team_id, agent_name)
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_team_members_agent ON team_members(agent_name, project)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS team_inbox (
		team_id    TEXT NOT NULL,
		message_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		PRIMARY KEY (team_id, message_id)
	)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS agent_notify_channels (
		agent_name TEXT NOT NULL,
		project    TEXT NOT NULL DEFAULT 'default',
		target     TEXT NOT NULL,
		PRIMARY KEY (agent_name, project, target)
	)`)

	// Boards
	conn.Exec(`CREATE TABLE IF NOT EXISTS boards (
		id          TEXT PRIMARY KEY,
		project     TEXT NOT NULL DEFAULT 'default',
		name        TEXT NOT NULL,
		slug        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_by  TEXT NOT NULL DEFAULT 'user',
		created_at  TEXT NOT NULL
	)`)
	conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_boards_project_slug ON boards(project, slug)`)
	ensureColumns(conn, "boards", map[string]string{
		"archived_at": "TEXT",
	})

	// Goals
	conn.Exec(`CREATE TABLE IF NOT EXISTS goals (
		id              TEXT PRIMARY KEY,
		project         TEXT NOT NULL DEFAULT 'default',
		type            TEXT NOT NULL DEFAULT 'agent_goal',
		title           TEXT NOT NULL,
		description     TEXT NOT NULL DEFAULT '',
		owner_agent     TEXT,
		parent_goal_id  TEXT,
		status          TEXT NOT NULL DEFAULT 'active',
		created_by      TEXT NOT NULL DEFAULT 'user',
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL,
		completed_at    TEXT
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_goals_project_status ON goals(project, status)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_goals_parent ON goals(parent_goal_id)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_goals_type ON goals(project, type)`)

	// Vaults (per-project config)
	conn.Exec(`CREATE TABLE IF NOT EXISTS vaults (
		project     TEXT PRIMARY KEY,
		path        TEXT NOT NULL,
		created_at  TEXT NOT NULL
	)`)

	// Vault docs
	if err := migrateVault(conn); err != nil {
		return fmt.Errorf("migrate vault: %w", err)
	}

	return nil
}

// backfillProjects creates project entries for any existing agents that don't have a project row yet.
func backfillProjects(conn *sql.DB) {
	planetPool := []string{
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

	rows, err := conn.Query("SELECT DISTINCT project FROM agents WHERE project NOT IN (SELECT name FROM projects)")
	if err != nil {
		return
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		projects = append(projects, p)
	}

	for _, p := range projects {
		h := 0
		for _, c := range p {
			h = ((h << 5) - h + int(c))
		}
		if h < 0 {
			h = -h
		}
		planet := planetPool[h%len(planetPool)]
		conn.Exec("INSERT OR IGNORE INTO projects (name, planet_type, created_at) VALUES (?, ?, datetime('now'))", p, planet)
	}
}

// migrateVault creates the vault_docs table, FTS5 virtual table, and sync triggers.
func migrateVault(conn *sql.DB) error {
	conn.Exec(`CREATE TABLE IF NOT EXISTS vault_docs (
		path       TEXT NOT NULL,
		project    TEXT NOT NULL,
		title      TEXT NOT NULL DEFAULT '',
		owner      TEXT NOT NULL DEFAULT '',
		status     TEXT NOT NULL DEFAULT '',
		tags       TEXT NOT NULL DEFAULT '[]',
		content    TEXT NOT NULL DEFAULT '',
		size_bytes INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL,
		indexed_at TEXT NOT NULL,
		PRIMARY KEY (path, project)
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_vault_docs_project ON vault_docs(project)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_vault_docs_tags ON vault_docs(project, tags)`)

	if _, err := conn.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS vault_docs_fts USING fts5(
		path, title, tags, content,
		content=vault_docs,
		content_rowid=rowid
	)`); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vault FTS5 not available: %v\n", err)
		return nil
	}

	conn.Exec(`CREATE TRIGGER IF NOT EXISTS vault_docs_ai AFTER INSERT ON vault_docs BEGIN
		INSERT INTO vault_docs_fts(rowid, path, title, tags, content)
		VALUES (new.rowid, new.path, new.title, new.tags, new.content);
	END`)

	conn.Exec(`CREATE TRIGGER IF NOT EXISTS vault_docs_ad AFTER DELETE ON vault_docs BEGIN
		INSERT INTO vault_docs_fts(vault_docs_fts, rowid, path, title, tags, content)
		VALUES ('delete', old.rowid, old.path, old.title, old.tags, old.content);
	END`)

	conn.Exec(`CREATE TRIGGER IF NOT EXISTS vault_docs_au AFTER UPDATE ON vault_docs BEGIN
		INSERT INTO vault_docs_fts(vault_docs_fts, rowid, path, title, tags, content)
		VALUES ('delete', old.rowid, old.path, old.title, old.tags, old.content);
		INSERT INTO vault_docs_fts(rowid, path, title, tags, content)
		VALUES (new.rowid, new.path, new.title, new.tags, new.content);
	END`)

	return nil
}

// migrateDropGlobalUnique removes the old UNIQUE constraint on agents.name
// that was created in early versions. Only runs if the constraint still exists.
func migrateDropGlobalUnique(conn *sql.DB) {
	// Check if the old UNIQUE(name) autoindex exists (sqlite_autoindex_agents_2).
	// Note: sqlite_autoindex_agents_1 is the PRIMARY KEY, not the UNIQUE(name).
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='sqlite_autoindex_agents_2'`).Scan(&count)
	if err != nil || count == 0 {
		return // no old constraint, nothing to do
	}

	// Rebuild the table without the inline UNIQUE on name.
	tx, err := conn.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE agents_new (
			id              TEXT PRIMARY KEY,
			name            TEXT NOT NULL,
			role            TEXT NOT NULL DEFAULT '',
			description     TEXT NOT NULL DEFAULT '',
			registered_at   TEXT NOT NULL,
			last_seen       TEXT NOT NULL,
			project         TEXT NOT NULL DEFAULT 'default',
			reports_to      TEXT,
			profile_slug    TEXT,
			status          TEXT NOT NULL DEFAULT 'active',
			deactivated_at  TEXT,
			is_executive    INTEGER NOT NULL DEFAULT 0,
			session_id      TEXT
		)`,
		`INSERT INTO agents_new SELECT id, name, role, description, registered_at, last_seen, project, reports_to, NULL, 'active', NULL, 0, NULL FROM agents`,
		`DROP TABLE agents`,
		`ALTER TABLE agents_new RENAME TO agents`,
		`CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_name ON agents(project, name)`,
	}

	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return
		}
	}

	tx.Commit()
}

// migrateMemories creates the memories table, FTS5 virtual table, and sync triggers.
func migrateMemories(conn *sql.DB) error {
	// Main table
	if _, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS memories (
		id            TEXT PRIMARY KEY,
		key           TEXT NOT NULL,
		value         TEXT NOT NULL,
		tags          TEXT NOT NULL DEFAULT '[]',
		scope         TEXT NOT NULL,
		project       TEXT NOT NULL DEFAULT 'default',
		agent_name    TEXT NOT NULL,
		confidence    TEXT NOT NULL DEFAULT 'stated',
		version       INTEGER NOT NULL DEFAULT 1,
		supersedes    TEXT,
		conflict_with TEXT,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		archived_at   TEXT,
		archived_by   TEXT,
		layer         TEXT NOT NULL DEFAULT 'behavior'
	)`); err != nil {
		return fmt.Errorf("create memories table: %w", err)
	}

	// Layer column migration for existing DBs (idempotent).
	conn.Exec(`ALTER TABLE memories ADD COLUMN layer TEXT NOT NULL DEFAULT 'behavior'`)

	// Indexes (all idempotent)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_key_scope ON memories(project, scope, key) WHERE archived_at IS NULL`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_agent ON memories(agent_name)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_tags ON memories(project, scope)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_updated ON memories(updated_at DESC)`)

	// FTS5 virtual table for full-text search.
	// Requires building with -tags "fts5" for github.com/mattn/go-sqlite3.
	if _, err := conn.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		key, value, tags,
		content=memories,
		content_rowid=rowid
	)`); err != nil {
		fmt.Fprintf(os.Stderr, "warning: FTS5 not available (build with -tags \"fts5\"): %v\n", err)
		return nil // non-fatal: memory CRUD works, search degrades
	}

	// Triggers to keep FTS in sync
	conn.Exec(`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, key, value, tags)
		VALUES (new.rowid, new.key, new.value, new.tags);
	END`)

	conn.Exec(`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, key, value, tags)
		VALUES ('delete', old.rowid, old.key, old.value, old.tags);
	END`)

	conn.Exec(`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, key, value, tags)
		VALUES ('delete', old.rowid, old.key, old.value, old.tags);
		INSERT INTO memories_fts(rowid, key, value, tags)
		VALUES (new.rowid, new.key, new.value, new.tags);
	END`)

	return nil
}
