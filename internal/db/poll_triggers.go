package db

import (
	"agent-relay/internal/models"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertPollTrigger creates or updates a poll trigger by project+name.
func (d *DB) UpsertPollTrigger(project, name, url, headers, path, op, value, interval, event, meta string, cooldown int) (*models.PollTrigger, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)
	if headers == "" {
		headers = "{}"
	}
	if meta == "" {
		meta = "{}"
	}
	if cooldown <= 0 {
		cooldown = 300
	}

	// Check if exists
	var existingID string
	err := d.conn.QueryRow(`SELECT id FROM poll_triggers WHERE project = ? AND name = ?`, project, name).Scan(&existingID)
	if err == sql.ErrNoRows {
		id := uuid.New().String()
		_, err := d.conn.Exec(`INSERT INTO poll_triggers
			(id, project, name, url, headers, condition_path, condition_op, condition_value, poll_interval, fire_event, fire_meta, cooldown_seconds, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, project, name, url, headers, path, op, value, interval, event, meta, cooldown, now)
		if err != nil {
			return nil, fmt.Errorf("insert poll trigger: %w", err)
		}
		return &models.PollTrigger{
			ID: id, Project: project, Name: name, URL: url, Headers: headers,
			ConditionPath: path, ConditionOp: op, ConditionValue: value,
			PollInterval: interval, FireEvent: event, FireMeta: meta,
			Enabled: true, CooldownSeconds: cooldown, CreatedAt: now,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check poll trigger: %w", err)
	}

	// Update existing
	_, err = d.conn.Exec(`UPDATE poll_triggers SET url=?, headers=?, condition_path=?, condition_op=?, condition_value=?,
		poll_interval=?, fire_event=?, fire_meta=?, cooldown_seconds=? WHERE id=?`,
		url, headers, path, op, value, interval, event, meta, cooldown, existingID)
	if err != nil {
		return nil, fmt.Errorf("update poll trigger: %w", err)
	}

	return &models.PollTrigger{
		ID: existingID, Project: project, Name: name, URL: url, Headers: headers,
		ConditionPath: path, ConditionOp: op, ConditionValue: value,
		PollInterval: interval, FireEvent: event, FireMeta: meta,
		Enabled: true, CooldownSeconds: cooldown, CreatedAt: now,
	}, nil
}

// ListPollTriggers returns all poll triggers for a project.
func (d *DB) ListPollTriggers(project string) ([]models.PollTrigger, error) {
	rows, err := d.ro().Query(`SELECT id, project, name, url, headers, condition_path, condition_op, condition_value,
		poll_interval, fire_event, fire_meta, enabled, COALESCE(last_polled_at, ''), COALESCE(last_result, ''),
		last_matched, cooldown_seconds, created_at
		FROM poll_triggers WHERE project = ? ORDER BY name`, project)
	if err != nil {
		return nil, fmt.Errorf("list poll triggers: %w", err)
	}
	defer rows.Close()

	var result []models.PollTrigger
	for rows.Next() {
		var pt models.PollTrigger
		if err := rows.Scan(&pt.ID, &pt.Project, &pt.Name, &pt.URL, &pt.Headers, &pt.ConditionPath, &pt.ConditionOp, &pt.ConditionValue,
			&pt.PollInterval, &pt.FireEvent, &pt.FireMeta, &pt.Enabled, &pt.LastPolledAt, &pt.LastResult,
			&pt.LastMatched, &pt.CooldownSeconds, &pt.CreatedAt); err != nil {
			continue
		}
		result = append(result, pt)
	}
	return result, rows.Err()
}

// GetDuePollTriggers returns enabled poll triggers that are due for polling.
func (d *DB) GetDuePollTriggers() ([]models.PollTrigger, error) {
	rows, err := d.ro().Query(`SELECT id, project, name, url, headers, condition_path, condition_op, condition_value,
		poll_interval, fire_event, fire_meta, enabled, COALESCE(last_polled_at, ''), COALESCE(last_result, ''),
		last_matched, cooldown_seconds, created_at
		FROM poll_triggers WHERE enabled = 1 ORDER BY last_polled_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("get due poll triggers: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var result []models.PollTrigger
	for rows.Next() {
		var pt models.PollTrigger
		if err := rows.Scan(&pt.ID, &pt.Project, &pt.Name, &pt.URL, &pt.Headers, &pt.ConditionPath, &pt.ConditionOp, &pt.ConditionValue,
			&pt.PollInterval, &pt.FireEvent, &pt.FireMeta, &pt.Enabled, &pt.LastPolledAt, &pt.LastResult,
			&pt.LastMatched, &pt.CooldownSeconds, &pt.CreatedAt); err != nil {
			continue
		}

		// Check if interval has elapsed
		interval, err := time.ParseDuration(pt.PollInterval)
		if err != nil {
			continue
		}
		if pt.LastPolledAt == "" {
			result = append(result, pt)
			continue
		}
		lastPolled, err := time.Parse("2006-01-02T15:04:05Z", pt.LastPolledAt)
		if err != nil {
			result = append(result, pt)
			continue
		}
		if now.Sub(lastPolled) >= interval {
			result = append(result, pt)
		}
	}
	return result, rows.Err()
}

// UpdatePollResult updates the last poll result for a trigger.
func (d *DB) UpdatePollResult(id, result string, matched bool) error {
	now := time.Now().UTC().Format(memoryTimeFmt)
	m := 0
	if matched {
		m = 1
	}
	_, err := d.conn.Exec(`UPDATE poll_triggers SET last_polled_at=?, last_result=?, last_matched=? WHERE id=?`,
		now, result, m, id)
	return err
}

// DeletePollTrigger removes a poll trigger by ID.
func (d *DB) DeletePollTrigger(id string) error {
	_, err := d.conn.Exec(`DELETE FROM poll_triggers WHERE id = ?`, id)
	return err
}

// GetPollTrigger returns a single poll trigger by ID.
func (d *DB) GetPollTrigger(id string) (*models.PollTrigger, error) {
	var pt models.PollTrigger
	err := d.ro().QueryRow(`SELECT id, project, name, url, headers, condition_path, condition_op, condition_value,
		poll_interval, fire_event, fire_meta, enabled, COALESCE(last_polled_at, ''), COALESCE(last_result, ''),
		last_matched, cooldown_seconds, created_at
		FROM poll_triggers WHERE id = ?`, id).Scan(
		&pt.ID, &pt.Project, &pt.Name, &pt.URL, &pt.Headers, &pt.ConditionPath, &pt.ConditionOp, &pt.ConditionValue,
		&pt.PollInterval, &pt.FireEvent, &pt.FireMeta, &pt.Enabled, &pt.LastPolledAt, &pt.LastResult,
		&pt.LastMatched, &pt.CooldownSeconds, &pt.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pt, nil
}
