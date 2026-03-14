package spawn

import (
	"log/slog"
	"time"

	"agent-relay/internal/db"
)

// StartReaper runs a background goroutine that detects ghost spawns
// (DB says "running" but process is gone) and marks them as dead.
func StartReaper(database *db.DB, mgr *Manager, done <-chan struct{}, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				reapGhosts(database, mgr, logger)
			}
		}
	}()
}

func reapGhosts(database *db.DB, mgr *Manager, logger *slog.Logger) {
	// Find all children marked as "running" in DB
	children := database.ListRunningChildren()
	if len(children) == 0 {
		return
	}

	// Get the set of children tracked in-memory (= truly alive)
	mgr.mu.RLock()
	activeIDs := make(map[string]bool, len(mgr.children))
	for id := range mgr.children {
		activeIDs[id] = true
	}
	mgr.mu.RUnlock()

	for _, c := range children {
		id, _ := c["id"].(string)
		if id == "" {
			continue
		}

		// If the child is in the manager's map, it's still alive
		if activeIDs[id] {
			continue
		}

		// Ghost detected — DB says running but manager doesn't know about it.
		// This happens when relay restarts while children were running,
		// or when a process disappears (OOM, SIGKILL, machine reboot).
		startedAt, _ := c["started_at"].(string)
		profile, _ := c["profile"].(string)
		logger.Warn("ghost spawn detected — marking dead",
			"child_id", id,
			"profile", profile,
			"started_at", startedAt,
		)
		database.UpdateSpawnChild(id, "dead", -1, "ghost: process not tracked (relay restart or crash)")
	}
}
