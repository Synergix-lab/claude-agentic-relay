package lock

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Lock represents a file-based lock with flock.
type Lock struct {
	Path      string
	AgentName string
	file      *os.File
}

// LockInfo stores metadata written inside the lock file.
type LockInfo struct {
	PID       int
	Timestamp time.Time
	Cycle     string
	TTL       time.Duration
}

// Manager handles lock acquisition and release.
type Manager struct {
	Dir    string
	Prefix string
	Logger *slog.Logger
}

// NewManager creates a new lock manager.
func NewManager(dir, prefix string, logger *slog.Logger) *Manager {
	return &Manager{
		Dir:    dir,
		Prefix: prefix,
		Logger: logger,
	}
}

// lockPath returns the lock file path for an agent.
func (m *Manager) lockPath(agent string) string {
	return filepath.Join(m.Dir, fmt.Sprintf("%s%s.lock", m.Prefix, agent))
}

// Acquire tries to get an exclusive lock for the agent.
// Returns the Lock on success, nil if already locked.
func (m *Manager) Acquire(agent, cycle string, ttl time.Duration) (*Lock, error) {
	path := m.lockPath(agent)

	// Check if existing lock is expired
	if info, err := m.ReadInfo(agent); err == nil {
		if time.Since(info.Timestamp) > info.TTL {
			m.Logger.Warn("stale lock detected, removing",
				"agent", agent,
				"age", time.Since(info.Timestamp),
				"ttl", info.TTL,
			)
			os.Remove(path)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	// Try non-blocking flock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, nil // Already locked
	}

	// Write lock info
	info := fmt.Sprintf("%d\n%s\n%s\n%s",
		os.Getpid(),
		time.Now().UTC().Format(time.RFC3339),
		cycle,
		ttl.String(),
	)
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.WriteString(info)
	_ = f.Sync()

	m.Logger.Info("lock acquired", "agent", agent, "cycle", cycle)

	return &Lock{
		Path:      path,
		AgentName: agent,
		file:      f,
	}, nil
}

// Release releases the lock.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.Path)
	return nil
}

// IsLocked checks if an agent currently has a lock.
func (m *Manager) IsLocked(agent string) bool {
	path := m.lockPath(agent)

	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return false
	}
	defer f.Close()

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return true // Can't acquire = someone else holds it
	}

	// We got it, release immediately
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// ReadInfo reads the lock file metadata.
func (m *Manager) ReadInfo(agent string) (*LockInfo, error) {
	path := m.lockPath(agent)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 4 {
		return nil, fmt.Errorf("invalid lock file format")
	}

	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PID in lock: %w", err)
	}

	ts, err := time.Parse(time.RFC3339, lines[1])
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in lock: %w", err)
	}

	ttl, err := time.ParseDuration(lines[3])
	if err != nil {
		return nil, fmt.Errorf("invalid TTL in lock: %w", err)
	}

	return &LockInfo{
		PID:       pid,
		Timestamp: ts,
		Cycle:     lines[2],
		TTL:       ttl,
	}, nil
}
