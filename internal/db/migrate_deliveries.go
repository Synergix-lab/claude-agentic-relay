package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// migrateDeliveries backfills deliveries for existing messages.
// Only runs if deliveries table is empty and messages table has data.
func migrateDeliveries(conn *sql.DB) {
	var deliveryCount int
	_ = conn.QueryRow("SELECT COUNT(*) FROM deliveries").Scan(&deliveryCount)
	if deliveryCount > 0 {
		return // already migrated
	}

	var messageCount int
	_ = conn.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount)
	if messageCount == 0 {
		return // nothing to migrate
	}

	log.Printf("migrating %d messages to deliveries...", messageCount)
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	tx, err := conn.Begin()
	if err != nil {
		log.Printf("delivery migration: failed to begin tx: %v", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query("SELECT id, from_agent, to_agent, conversation_id, project FROM messages")
	if err != nil {
		log.Printf("delivery migration: failed to query messages: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var msgID, from, to, project string
		var convID *string
		if err := rows.Scan(&msgID, &from, &to, &convID, &project); err != nil {
			continue
		}

		if convID != nil {
			// Conversation message → one delivery per member (except sender)
			memberRows, err := tx.Query(
				"SELECT agent_name FROM conversation_members WHERE conversation_id = ?",
				*convID,
			)
			if err != nil {
				continue
			}
			for memberRows.Next() {
				var member string
				_ = memberRows.Scan(&member)
				if member != from {
					state := deliveryStateFromReads(tx, msgID, member)
					insertDelivery(tx, msgID, member, state, now, project)
					count++
				}
			}
			_ = memberRows.Close()
		} else if to == "*" {
			// Broadcast → one delivery per active agent in project (except sender)
			agentRows, err := tx.Query(
				"SELECT name FROM agents WHERE project = ? AND status IN ('active', 'sleeping', 'inactive') AND name != ?",
				project, from,
			)
			if err != nil {
				continue
			}
			for agentRows.Next() {
				var agent string
				_ = agentRows.Scan(&agent)
				state := deliveryStateFromReads(tx, msgID, agent)
				insertDelivery(tx, msgID, agent, state, now, project)
				count++
			}
			_ = agentRows.Close()
		} else if to != "" {
			// Direct message → one delivery
			state := deliveryStateFromReads(tx, msgID, to)
			insertDelivery(tx, msgID, to, state, now, project)
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("delivery migration: failed to commit: %v", err)
		return
	}

	log.Printf("delivery migration complete: %d deliveries created", count)
}

func deliveryStateFromReads(tx *sql.Tx, msgID, agentName string) string {
	var exists int
	_ = tx.QueryRow("SELECT COUNT(*) FROM message_reads WHERE message_id = ? AND agent_name = ?", msgID, agentName).Scan(&exists)
	if exists > 0 {
		return "acknowledged"
	}
	return "surfaced" // assume already seen since it's historical
}

func insertDelivery(tx *sql.Tx, msgID, toAgent, state, now, project string) {
	var ackedAt *string
	var surfacedAt *string
	switch state {
	case "acknowledged":
		ackedAt = &now
		surfacedAt = &now
	case "surfaced":
		surfacedAt = &now
	}
	_, _ = tx.Exec(
		fmt.Sprintf("INSERT OR IGNORE INTO deliveries (id, message_id, to_agent, state, sequence_number, created_at, surfaced_at, acknowledged_at, project) VALUES (?, ?, ?, '%s', 0, ?, ?, ?, ?)", state),
		uuid.New().String(), msgID, toAgent, now, surfacedAt, ackedAt, project,
	)
}
