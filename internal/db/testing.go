package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// NewTestDB creates a database at the given path for testing.
func NewTestDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := migrate(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader, err := sql.Open("sqlite3", path+"?mode=ro&_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(10)
	return &DB{conn: conn, reader: reader, path: path}, nil
}
