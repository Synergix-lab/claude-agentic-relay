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
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dbPath := filepath.Join(dbDir, "relay.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	conn.SetMaxOpenConns(1)

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

func migrate(conn *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT '',
		description   TEXT NOT NULL DEFAULT '',
		registered_at TEXT NOT NULL,
		last_seen     TEXT NOT NULL,
		project       TEXT NOT NULL DEFAULT 'default',
		reports_to    TEXT
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

	// Add conversation_id column to messages if it doesn't exist (migration).
	alterSchema := `
	ALTER TABLE messages ADD COLUMN conversation_id TEXT;
	`
	alterIndex := `
	CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
	`

	if _, err := conn.Exec(schema); err != nil {
		return err
	}

	// ALTER TABLE may fail if column already exists — that's fine.
	conn.Exec(alterSchema)

	if _, err := conn.Exec(alterIndex); err != nil {
		return err
	}

	// Project isolation migration (idempotent — ALTER fails if column exists).
	conn.Exec(`ALTER TABLE agents ADD COLUMN project TEXT NOT NULL DEFAULT 'default'`)

	// Hierarchy migration (idempotent — ALTER fails if column exists).
	conn.Exec(`ALTER TABLE agents ADD COLUMN reports_to TEXT`)
	conn.Exec(`ALTER TABLE messages ADD COLUMN project TEXT NOT NULL DEFAULT 'default'`)
	conn.Exec(`ALTER TABLE conversations ADD COLUMN project TEXT NOT NULL DEFAULT 'default'`)

	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_conversations_project ON conversations(project)`)
	conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_name ON agents(project, name)`)

	// Remove the old global UNIQUE constraint on agents.name (existing DBs only).
	// SQLite can't drop inline constraints, so we rebuild the table.
	migrateDropGlobalUnique(conn)

	return nil
}

// migrateDropGlobalUnique removes the old UNIQUE constraint on agents.name
// that was created in early versions. Only runs if the constraint still exists.
func migrateDropGlobalUnique(conn *sql.DB) {
	// Check if the old sqlite_autoindex for UNIQUE(name) exists.
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='sqlite_autoindex_agents_1'`).Scan(&count)
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
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			role          TEXT NOT NULL DEFAULT '',
			description   TEXT NOT NULL DEFAULT '',
			registered_at TEXT NOT NULL,
			last_seen     TEXT NOT NULL,
			project       TEXT NOT NULL DEFAULT 'default',
			reports_to    TEXT
		)`,
		`INSERT INTO agents_new SELECT id, name, role, description, registered_at, last_seen, project, reports_to FROM agents`,
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
