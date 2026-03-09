package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-20000&_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)
	if err := migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &DB{conn: conn, path: dbPath}
}

func TestConcurrentReadsAndWrite(t *testing.T) {
	d := testDB(t)

	// Seed an agent
	d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil)

	var wg sync.WaitGroup
	var errors atomic.Int32

	// 10 concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.ListAgents("default")
			if err != nil {
				errors.Add(1)
			}
		}()
	}

	// 5 concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("msg-%d", n), "{}", nil, nil)
			if err != nil {
				errors.Add(1)
			}
		}(i)
	}

	wg.Wait()
	if errors.Load() > 0 {
		t.Errorf("got %d errors during concurrent operations", errors.Load())
	}

	// Verify all writes landed
	msgs, err := d.GetAllRecentMessages("default", 100)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}
}

func TestConcurrentWriters(t *testing.T) {
	d := testDB(t)

	d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil)

	var wg sync.WaitGroup
	var errors atomic.Int32
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("write-%d", idx), "{}", nil, nil)
			if err != nil {
				errors.Add(1)
				t.Logf("write error: %v", err)
			}
		}(i)
	}

	wg.Wait()
	if errors.Load() > 0 {
		t.Errorf("got %d errors during %d concurrent writes", errors.Load(), n)
	}

	msgs, _ := d.GetAllRecentMessages("default", 100)
	if len(msgs) != n {
		t.Errorf("expected %d messages, got %d", n, len(msgs))
	}
}

func TestOptimizeNoCorruption(t *testing.T) {
	d := testDB(t)

	// Insert some data
	d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil)
	for i := 0; i < 10; i++ {
		d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("msg-%d", i), "{}", nil, nil)
	}

	// Run optimize
	d.Optimize()

	// Verify integrity
	var result string
	err := d.conn.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil {
		t.Fatalf("integrity_check failed: %v", err)
	}
	if result != "ok" {
		t.Errorf("integrity_check returned: %s", result)
	}

	// Verify data still accessible
	msgs, err := d.GetAllRecentMessages("default", 100)
	if err != nil {
		t.Fatalf("get messages after optimize: %v", err)
	}
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages after optimize, got %d", len(msgs))
	}
}

func TestHeavyLoad(t *testing.T) {
	d := testDB(t)

	d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil)

	var wg sync.WaitGroup
	var writeErrors, readErrors atomic.Int32
	writes := 50
	reads := 50

	// Concurrent writes
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", fmt.Sprintf("agent-%d", idx), "target", "notification", "test", "heavy load", "{}", nil, nil)
			if err != nil {
				writeErrors.Add(1)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < reads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.ListAgents("default")
			if err != nil {
				readErrors.Add(1)
			}
		}()
	}

	wg.Wait()

	if writeErrors.Load() > 0 {
		t.Errorf("got %d write errors during heavy load", writeErrors.Load())
	}
	if readErrors.Load() > 0 {
		t.Errorf("got %d read errors during heavy load", readErrors.Load())
	}

	msgs, _ := d.GetAllRecentMessages("default", 200)
	if len(msgs) != writes {
		t.Errorf("expected %d messages, got %d", writes, len(msgs))
	}
}
