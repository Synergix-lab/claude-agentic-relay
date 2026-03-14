package spawn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

func parseTTLDuration(s string) time.Duration {
	if s == "" {
		return 10 * time.Minute
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

// PTYManager tracks interactive PTY sessions (separate from headless children).
type PTYManager struct {
	mu       sync.RWMutex
	sessions map[string]*PTYSession // sessionID -> PTYSession
	executor *Executor
}

// NewPTYManager creates a PTY session manager.
func NewPTYManager(executor *Executor) *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
		executor: executor,
	}
}

// Spawn creates a new interactive PTY session.
func (pm *PTYManager) Spawn(prompt, allowedTools, ttl, workDir string) (string, *PTYSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.sessions) >= 5 {
		return "", nil, fmt.Errorf("max 5 concurrent PTY sessions")
	}

	params := SpawnParams{
		Prompt:       prompt,
		TTL:          parseTTLDuration(ttl),
		AllowedTools: allowedTools,
		Streaming:    true,
	}

	sess, err := pm.executor.StartPTY(context.Background(), params, workDir)
	if err != nil {
		return "", nil, fmt.Errorf("start PTY: %w", err)
	}

	id := uuid.New().String()
	sess.ID = id
	pm.sessions[id] = sess

	// Auto-cleanup when process exits
	go func() {
		_ = sess.Cmd.Wait()
		pm.mu.Lock()
		delete(pm.sessions, id)
		pm.mu.Unlock()
	}()

	return id, sess, nil
}

// Get returns a PTY session by ID.
func (pm *PTYManager) Get(id string) *PTYSession {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.sessions[id]
}

// Kill terminates a PTY session.
func (pm *PTYManager) Kill(id string) error {
	pm.mu.Lock()
	sess, ok := pm.sessions[id]
	if !ok {
		pm.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(pm.sessions, id)
	pm.mu.Unlock()

	sess.Close()
	return nil
}

// List returns all active session IDs.
func (pm *PTYManager) List() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	ids := make([]string, 0, len(pm.sessions))
	for id := range pm.sessions {
		ids = append(ids, id)
	}
	return ids
}
