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

	// Writer pool (matches production config)
	writer, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_cache_size=-20000&_foreign_keys=ON&_txlock=immediate")
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	// Reader pool (matches production config)
	reader, err := sql.Open("sqlite3", dbPath+"?mode=ro&_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	reader.SetMaxOpenConns(10)
	reader.SetMaxIdleConns(5)

	if err := migrate(writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	return &DB{conn: writer, reader: reader, path: dbPath}
}

func TestConcurrentReadsAndWrite(t *testing.T) {
	d := testDB(t)

	// Seed an agent
	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)

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
			_, err := d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("msg-%d", n), "{}", "P2", 3600, nil, nil)
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

	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)

	var wg sync.WaitGroup
	var errors atomic.Int32
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("write-%d", idx), "{}", "P2", 3600, nil, nil)
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
	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)
	for i := 0; i < 10; i++ {
		_, _ = d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("msg-%d", i), "{}", "P2", 3600, nil, nil)
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

func TestReaderPoolIsSeparate(t *testing.T) {
	d := testDB(t)

	// Reader and writer should be different pool instances
	if d.conn == d.reader {
		t.Fatal("reader and writer pools should be separate *sql.DB instances")
	}

	// Reader pool should allow more concurrent connections
	writerMax := d.conn.Stats().MaxOpenConnections
	readerMax := d.reader.Stats().MaxOpenConnections
	if writerMax != 1 {
		t.Errorf("writer MaxOpenConns should be 1, got %d", writerMax)
	}
	if readerMax < 5 {
		t.Errorf("reader MaxOpenConns should be >= 5, got %d", readerMax)
	}
}

func TestReadAfterWrite(t *testing.T) {
	d := testDB(t)

	// Write via writer
	_, _, _ = d.RegisterAgent("default", "bot-a", "tester", "", nil, nil, false, nil, "[]", 0)

	// Read via reader should see the write (WAL visibility)
	agents, err := d.ListAgents("default")
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "bot-a" {
		t.Errorf("expected bot-a, got %s", agents[0].Name)
	}
}

func TestReadsNeverBlockedByWrite(t *testing.T) {
	d := testDB(t)
	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)

	// Start a long write transaction on the writer
	tx, err := d.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		_, _ = tx.Exec("INSERT INTO messages (id, from_agent, to_agent, type, content, created_at, project) VALUES (?, 'bot-a', 'bot-b', 'notification', 'test', datetime('now'), 'default')", fmt.Sprintf("tx-msg-%d", i))
	}

	// While the write tx is open, reads via reader pool should succeed immediately
	var wg sync.WaitGroup
	var readErrors atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.ListAgents("default")
			if err != nil {
				readErrors.Add(1)
				t.Logf("read error during open tx: %v", err)
			}
		}()
	}
	wg.Wait()

	if readErrors.Load() > 0 {
		t.Errorf("got %d read errors while write tx was open", readErrors.Load())
	}

	_ = tx.Commit()
}

func TestWritesDontUseManyConns(t *testing.T) {
	d := testDB(t)
	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)

	// Writer pool has MaxOpenConns=1. Concurrent writes should serialize, not error.
	var wg sync.WaitGroup
	var errors atomic.Int32
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("serial-%d", idx), "{}", "P2", 3600, nil, nil)
			if err != nil {
				errors.Add(1)
				t.Logf("write error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if errors.Load() > 0 {
		t.Errorf("got %d write errors with single-conn writer pool", errors.Load())
	}

	msgs, _ := d.GetAllRecentMessages("default", 100)
	if len(msgs) != 30 {
		t.Errorf("expected 30 messages, got %d", len(msgs))
	}
}

func TestMixedReadWriteFunction(t *testing.T) {
	d := testDB(t)

	// RegisterAgent reads (check existing) then writes (insert) — tests mixed path
	agent1, isRespawn1, err := d.RegisterAgent("default", "bot-a", "tester", "", nil, nil, false, nil, "[]", 0)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	if isRespawn1 {
		t.Error("expected new agent, got respawn")
	}
	if agent1.Name != "bot-a" {
		t.Errorf("expected bot-a, got %s", agent1.Name)
	}

	// Re-register same agent — should read existing via writer, then update
	agent2, isRespawn2, err := d.RegisterAgent("default", "bot-a", "updated", "", nil, nil, false, nil, "[]", 0)
	if err != nil {
		t.Fatalf("re-register agent: %v", err)
	}
	if !isRespawn2 {
		t.Error("expected respawn, got new")
	}
	if agent2.Role != "updated" {
		t.Errorf("expected role 'updated', got %s", agent2.Role)
	}

	// Dispatch + complete task — tests transitionTask mixed read/write
	task, err := d.DispatchTask("default", "dev", "bot-a", "test task", "desc", "P2", nil, nil, nil)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	_, err = d.ClaimTask(task.ID, "bot-a", "default")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	_, err = d.StartTask(task.ID, "bot-a", "default")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	result := "done!"
	completed, err := d.CompleteTask(task.ID, "bot-a", "default", &result)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != "done" {
		t.Errorf("expected done, got %s", completed.Status)
	}
}

func TestCloseCheckpoint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	writer, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_foreign_keys=ON&_txlock=immediate")
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	writer.SetMaxOpenConns(1)
	reader, err := sql.Open("sqlite3", dbPath+"?mode=ro&_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	reader.SetMaxOpenConns(10)
	if err := migrate(writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	d := &DB{conn: writer, reader: reader, path: dbPath}

	// Insert data to create WAL entries
	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)
	for i := 0; i < 50; i++ {
		_, _ = d.InsertMessage("default", "bot-a", "bot-b", "notification", "test", fmt.Sprintf("msg-%d", i), "{}", "P2", 3600, nil, nil)
	}

	// Close should TRUNCATE checkpoint
	_ = d.Close()

	// Reopen and verify data intact
	writer2, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON")
	if err != nil {
		t.Fatalf("reopen writer: %v", err)
	}
	defer func() { _ = writer2.Close() }()
	var count int
	_ = writer2.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	if count != 50 {
		t.Errorf("expected 50 messages after close+reopen, got %d", count)
	}
}

func TestHeavyLoad(t *testing.T) {
	d := testDB(t)

	_, _, _ = d.RegisterAgent("default", "bot-a", "test", "", nil, nil, false, nil, "[]", 0)

	var wg sync.WaitGroup
	var writeErrors, readErrors atomic.Int32
	writes := 50
	reads := 50

	// Concurrent writes
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.InsertMessage("default", fmt.Sprintf("agent-%d", idx), "target", "notification", "test", "heavy load", "{}", "P2", 3600, nil, nil)
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

func TestCountUnread(t *testing.T) {
	d := testDB(t)

	_, _, _ = d.RegisterAgent("default", "sender", "test", "", nil, nil, false, nil, "[]", 0)
	_, _, _ = d.RegisterAgent("default", "receiver", "test", "", nil, nil, false, nil, "[]", 0)

	// Empty inbox → count 0
	n, err := d.CountUnread("default", "receiver")
	if err != nil {
		t.Fatalf("CountUnread on empty: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 unread on empty, got %d", n)
	}

	// Send 3 direct messages to receiver, creating deliveries
	var msgIDs []string
	for i := 0; i < 3; i++ {
		msg, err := d.InsertMessage("default", "sender", "receiver", "notification", "test", fmt.Sprintf("msg-%d", i), "{}", "P2", 3600, nil, nil)
		if err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
		if err := d.CreateDeliveries(msg.ID, "default", []string{"receiver"}); err != nil {
			t.Fatalf("CreateDeliveries: %v", err)
		}
		msgIDs = append(msgIDs, msg.ID)
	}

	n, err = d.CountUnread("default", "receiver")
	if err != nil {
		t.Fatalf("CountUnread after inserts: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 unread after inserts, got %d", n)
	}

	// Unrelated agent should still see 0
	n, _ = d.CountUnread("default", "sender")
	if n != 0 {
		t.Fatalf("expected 0 unread for sender, got %d", n)
	}

	// MarkRead one → count drops by 1
	if _, err := d.MarkRead([]string{msgIDs[0]}, "receiver", "default"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	n, _ = d.CountUnread("default", "receiver")
	if n != 2 {
		t.Fatalf("expected 2 unread after mark_read, got %d", n)
	}

	// Force-expire one of the remaining messages → count drops by 1
	if _, err := d.conn.Exec(
		"UPDATE messages SET expired_at = ? WHERE id = ?",
		"2020-01-01T00:00:00.000000Z", msgIDs[1],
	); err != nil {
		t.Fatalf("force expire: %v", err)
	}
	n, _ = d.CountUnread("default", "receiver")
	if n != 1 {
		t.Fatalf("expected 1 unread after expiring one, got %d", n)
	}

	// Sanity: CountUnread must not flip deliveries to 'surfaced' (it's read-only).
	// A subsequent GetInbox(unread_only=true) should still return the remaining
	// queued message.
	msgs, err := d.GetInbox("default", "receiver", true, 10)
	if err != nil {
		t.Fatalf("GetInbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected GetInbox to still see 1 queued message after CountUnread, got %d", len(msgs))
	}
}
