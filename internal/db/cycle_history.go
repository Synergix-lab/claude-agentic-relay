package db

import "time"

// PurgeCycleHistory removes cycle_history rows older than the given duration.
func (d *DB) PurgeCycleHistory(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format("2006-01-02 15:04:05")
	res, err := d.conn.Exec("DELETE FROM cycle_history WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RecordCycleHistory persists a cycle execution metric.
func (d *DB) RecordCycleHistory(agentName, project, cycleName string, durationMs int64, success bool, exitCode int, errMsg string, inputTokens, outputTokens, cacheRead, cacheCreation int64) {
	successInt := 0
	if success {
		successInt = 1
	}
	_, _ = d.conn.Exec(`INSERT INTO cycle_history (agent_name, project, cycle_name, duration_ms, success, exit_code, error, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentName, project, cycleName, durationMs, successInt, exitCode, errMsg, inputTokens, outputTokens, cacheRead, cacheCreation)
}

// GetCycleHistory returns recent cycle history for an agent.
func (d *DB) GetCycleHistory(project, agentName string, limit int) []map[string]any {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT agent_name, project, cycle_name, duration_ms, success, exit_code, error, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, created_at
		FROM cycle_history WHERE project = ?`
	args := []any{project}

	if agentName != "" {
		query += " AND agent_name = ?"
		args = append(args, agentName)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var agent, proj, cycle, errMsg, createdAt string
		var durationMs, inputTokens, outputTokens, cacheRead, cacheCreation int64
		var success, exitCode int
		if err := rows.Scan(&agent, &proj, &cycle, &durationMs, &success, &exitCode, &errMsg, &inputTokens, &outputTokens, &cacheRead, &cacheCreation, &createdAt); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"agent_name":            agent,
			"project":               proj,
			"cycle_name":            cycle,
			"duration_ms":           durationMs,
			"success":               success == 1,
			"exit_code":             exitCode,
			"error":                 errMsg,
			"input_tokens":          inputTokens,
			"output_tokens":         outputTokens,
			"cache_read_tokens":     cacheRead,
			"cache_creation_tokens": cacheCreation,
			"created_at":            createdAt,
		})
	}
	return result
}
