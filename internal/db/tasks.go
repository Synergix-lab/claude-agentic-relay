package db

import (
	"agent-relay/internal/models"
	"agent-relay/internal/normalize"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Valid task state transitions
// "done" and "cancelled" are reachable from any state (flexible cleanup)
var validTransitions = map[string][]string{
	"pending":     {"accepted", "in-progress", "done", "cancelled"},
	"accepted":    {"in-progress", "done", "cancelled"},
	"in-progress": {"done", "blocked", "cancelled"},
	"blocked":     {"in-progress", "done", "cancelled"},
	"done":        {"cancelled"},
	"cancelled":   {},
}

const taskColumns = "id, profile_slug, assigned_to, dispatched_by, title, description, priority, status, result, blocked_reason, project, dispatched_at, accepted_at, started_at, completed_at, parent_task_id, ack_notified_at, ack_escalated_at, board_id, goal_id, archived_at"

func scanTask(row interface{ Scan(...any) error }) (models.Task, error) {
	var t models.Task
	err := row.Scan(&t.ID, &t.ProfileSlug, &t.AssignedTo, &t.DispatchedBy, &t.Title, &t.Description,
		&t.Priority, &t.Status, &t.Result, &t.BlockedReason, &t.Project,
		&t.DispatchedAt, &t.AcceptedAt, &t.StartedAt, &t.CompletedAt, &t.ParentTaskID,
		&t.AckNotifiedAt, &t.AckEscalatedAt, &t.BoardID, &t.GoalID, &t.ArchivedAt)
	return t, err
}

func (d *DB) DispatchTask(project, profileSlug, dispatchedBy, title, description, priority string, parentTaskID, boardID, goalID *string) (*models.Task, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)
	if priority == "" {
		priority = "P2"
	}

	task := &models.Task{
		ID:           uuid.New().String(),
		ProfileSlug:  profileSlug,
		DispatchedBy: dispatchedBy,
		Title:        title,
		Description:  description,
		Priority:     priority,
		Status:       "pending",
		Project:      project,
		DispatchedAt: now,
		ParentTaskID: parentTaskID,
		BoardID:      boardID,
		GoalID:       goalID,
	}

	_, err := d.conn.Exec(
		`INSERT INTO tasks (id, profile_slug, dispatched_by, title, description, priority, status, project, dispatched_at, parent_task_id, board_id, goal_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.ProfileSlug, task.DispatchedBy, task.Title, task.Description,
		task.Priority, task.Status, task.Project, task.DispatchedAt, task.ParentTaskID, task.BoardID, task.GoalID,
	)
	if err != nil {
		return nil, fmt.Errorf("dispatch task: %w", err)
	}
	return task, nil
}

func (d *DB) ResetTask(taskID, agentName, project string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "pending", nil, nil)
}

func (d *DB) ClaimTask(taskID, agentName, project string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "accepted", nil, nil)
}

func (d *DB) StartTask(taskID, agentName, project string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "in-progress", nil, nil)
}

func (d *DB) CompleteTask(taskID, agentName, project string, result *string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "done", result, nil)
}

func (d *DB) BlockTask(taskID, agentName, project string, reason *string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "blocked", nil, reason)
}

func (d *DB) CancelTask(taskID, agentName, project string, reason *string) (*models.Task, error) {
	return d.transitionTask(taskID, agentName, project, "cancelled", nil, reason)
}

func (d *DB) transitionTask(taskID, agentName, project, newStatus string, result, blockedReason *string) (*models.Task, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)

	task, err := d.GetTask(taskID, project)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// Validate transition (skip for user — admin can force any move)
	if agentName != "user" {
		allowed := validTransitions[task.Status]
		valid := false
		for _, s := range allowed {
			if s == newStatus {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid transition: %s → %s", task.Status, newStatus)
		}
	}

	// Build update
	task.Status = newStatus
	switch newStatus {
	case "pending":
		task.AssignedTo = nil
		task.AcceptedAt = nil
		task.StartedAt = nil
		task.CompletedAt = nil
		task.Result = nil
		task.BlockedReason = nil
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, assigned_to = NULL, accepted_at = NULL, started_at = NULL, completed_at = NULL, result = NULL, blocked_reason = NULL WHERE id = ? AND project = ?",
			newStatus, taskID, project,
		)
	case "accepted":
		task.AssignedTo = &agentName
		task.AcceptedAt = &now
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, assigned_to = ?, accepted_at = ? WHERE id = ? AND project = ?",
			newStatus, agentName, now, taskID, project,
		)
	case "in-progress":
		task.AssignedTo = &agentName
		task.StartedAt = &now
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, assigned_to = ?, started_at = ? WHERE id = ? AND project = ?",
			newStatus, agentName, now, taskID, project,
		)
	case "done":
		task.CompletedAt = &now
		result = normalizePtr(result)
		task.Result = result
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, result = ?, completed_at = ? WHERE id = ? AND project = ?",
			newStatus, result, now, taskID, project,
		)
	case "blocked":
		task.BlockedReason = blockedReason
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, blocked_reason = ? WHERE id = ? AND project = ?",
			newStatus, blockedReason, taskID, project,
		)
	case "cancelled":
		task.CompletedAt = &now
		task.BlockedReason = blockedReason // reuse as cancellation reason
		_, err = d.conn.Exec(
			"UPDATE tasks SET status = ?, blocked_reason = ?, completed_at = ? WHERE id = ? AND project = ?",
			newStatus, blockedReason, now, taskID, project,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}
	return task, nil
}

func (d *DB) GetTask(taskID, project string) (*models.Task, error) {
	t, err := scanTask(d.ro().QueryRow(
		"SELECT "+taskColumns+" FROM tasks WHERE id = ? AND project = ?",
		taskID, project,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

// GetTaskWithSubtasks returns a task with its subtask chain (max depth 3).
func (d *DB) GetTaskWithSubtasks(taskID, project string) (*models.Task, error) {
	task, err := d.GetTask(taskID, project)
	if err != nil || task == nil {
		return task, err
	}
	task.Subtasks, _ = d.getSubtasks(taskID, project, 0, 3)
	return task, nil
}

func (d *DB) getSubtasks(parentID, project string, depth, maxDepth int) ([]models.Task, error) {
	if depth >= maxDepth {
		return nil, nil
	}
	rows, err := d.ro().Query(
		"SELECT "+taskColumns+" FROM tasks WHERE parent_task_id = ? AND project = ? ORDER BY dispatched_at",
		parentID, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect all tasks first to close rows before recursive calls
	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Now recursively fetch subtasks (rows is closed, no deadlock)
	for i := range tasks {
		tasks[i].Subtasks, _ = d.getSubtasks(tasks[i].ID, project, depth+1, maxDepth)
	}
	return tasks, nil
}

// GetAgentTasks returns tasks assigned to or dispatched by an agent (for session_context).
func (d *DB) GetAgentTasks(project, agentName string) (assignedToMe []models.Task, dispatchedByMe []models.Task, err error) {
	// Assigned to me (active tasks) — close rows before next query
	assignedToMe, err = d.queryTasks(
		"SELECT "+taskColumns+" FROM tasks WHERE assigned_to = ? AND project = ? AND status IN ('pending','accepted','in-progress') ORDER BY CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 END",
		agentName, project,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get assigned tasks: %w", err)
	}

	// Also get pending tasks for my profile
	pending, err := d.queryTasks(
		`SELECT `+taskColumns+` FROM tasks WHERE project = ? AND status = 'pending' AND assigned_to IS NULL
		 AND profile_slug IN (SELECT profile_slug FROM agents WHERE name = ? AND project = ? AND profile_slug IS NOT NULL)
		 ORDER BY CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 END`,
		project, agentName, project,
	)
	if err == nil {
		assignedToMe = append(assignedToMe, pending...)
	}

	// Dispatched by me (not done)
	dispatchedByMe, err = d.queryTasks(
		"SELECT "+taskColumns+" FROM tasks WHERE dispatched_by = ? AND project = ? AND status != 'done' ORDER BY dispatched_at DESC",
		agentName, project,
	)
	if err != nil {
		return assignedToMe, nil, fmt.Errorf("get dispatched tasks: %w", err)
	}

	return assignedToMe, dispatchedByMe, nil
}

// queryTasks runs a query and collects all tasks, closing rows before returning.
func (d *DB) queryTasks(query string, args ...any) ([]models.Task, error) {
	rows, err := d.ro().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetUnackedTasks returns pending tasks older than minAge that haven't been notified yet.
func (d *DB) GetUnackedTasks(minAge time.Duration) ([]models.Task, error) {
	cutoff := time.Now().UTC().Add(-minAge).Format(memoryTimeFmt)
	rows, err := d.ro().Query(
		"SELECT "+taskColumns+" FROM tasks WHERE status = 'pending' AND dispatched_at < ?",
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("get unacked tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// MarkTaskAckNotified sets the ack_notified_at timestamp.
func (d *DB) MarkTaskAckNotified(taskID string) error {
	now := time.Now().UTC().Format(memoryTimeFmt)
	_, err := d.conn.Exec("UPDATE tasks SET ack_notified_at = ? WHERE id = ?", now, taskID)
	return err
}

// MarkTaskAckEscalated sets the ack_escalated_at timestamp.
func (d *DB) MarkTaskAckEscalated(taskID string) error {
	now := time.Now().UTC().Format(memoryTimeFmt)
	_, err := d.conn.Exec("UPDATE tasks SET ack_escalated_at = ? WHERE id = ?", now, taskID)
	return err
}

// GetParentChain walks up the parent_task_id chain (max depth 5).
func (d *DB) GetParentChain(taskID, project string) ([]models.Task, error) {
	var chain []models.Task
	currentID := taskID
	for i := 0; i < 5; i++ {
		var parentID *string
		err := d.ro().QueryRow("SELECT parent_task_id FROM tasks WHERE id = ? AND project = ?", currentID, project).Scan(&parentID)
		if err != nil || parentID == nil {
			break
		}
		parent, err := d.GetTask(*parentID, project)
		if err != nil || parent == nil {
			break
		}
		chain = append(chain, *parent)
		currentID = *parentID
	}
	return chain, nil
}

func (d *DB) ListTasks(project, status, profileSlug, priority, assignedTo, boardID string, limit int, includeArchived bool) ([]models.Task, error) {
	if limit <= 0 {
		limit = 50
	}

	query := "SELECT " + taskColumns + " FROM tasks WHERE project = ?"
	args := []any{project}

	if !includeArchived {
		query += " AND archived_at IS NULL"
	}

	if status == "active" {
		query += " AND status NOT IN ('done', 'cancelled')"
	} else if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if profileSlug != "" {
		query += " AND profile_slug = ?"
		args = append(args, profileSlug)
	}
	if priority != "" {
		query += " AND priority = ?"
		args = append(args, priority)
	}
	if assignedTo != "" {
		query += " AND assigned_to = ?"
		args = append(args, assignedTo)
	}
	if boardID != "" {
		query += " AND board_id = ?"
		args = append(args, boardID)
	}

	query += " ORDER BY CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 END, dispatched_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (d *DB) ListAllTasks(limit int) ([]models.Task, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.ro().Query(
		"SELECT "+taskColumns+" FROM tasks WHERE archived_at IS NULL ORDER BY CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 END, dispatched_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list all tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (d *DB) UpdateTaskFields(taskID, project string, title, description, priority, boardID, goalID *string) (*models.Task, error) {
	task, err := d.GetTask(taskID, project)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if title != nil {
		task.Title = *title
	}
	if description != nil {
		task.Description = *description
	}
	if priority != nil {
		task.Priority = *priority
	}
	if boardID != nil {
		task.BoardID = boardID
	}
	if goalID != nil {
		task.GoalID = goalID
	}

	_, err = d.conn.Exec(
		"UPDATE tasks SET title = ?, description = ?, priority = ?, board_id = ?, goal_id = ? WHERE id = ? AND project = ?",
		task.Title, task.Description, task.Priority, task.BoardID, task.GoalID, taskID, project,
	)
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	return task, nil
}

func (d *DB) DeleteTask(taskID, project string) error {
	_, err := d.conn.Exec("DELETE FROM tasks WHERE id = ? AND project = ?", taskID, project)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// FindSimilarTasks checks for existing non-done/cancelled tasks with a similar title under the same profile.
func (d *DB) FindSimilarTasks(project, profileSlug, title string) ([]models.Task, error) {
	// Use LIKE with the first 20 chars of the title for a rough match
	search := title
	if len(search) > 20 {
		search = search[:20]
	}
	return d.queryTasks(
		"SELECT "+taskColumns+" FROM tasks WHERE project = ? AND profile_slug = ? AND status NOT IN ('done','cancelled') AND title LIKE ? LIMIT 5",
		project, profileSlug, "%"+search+"%",
	)
}

// CheckSubtasksComplete checks if all subtasks of a parent task are done or cancelled.
// Returns (allComplete, total, doneCount).
func (d *DB) CheckSubtasksComplete(parentTaskID, project string) (bool, int, int) {
	var total, doneCount int
	_ = d.ro().QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE parent_task_id = ? AND project = ?",
		parentTaskID, project,
	).Scan(&total)
	if total == 0 {
		return false, 0, 0
	}
	_ = d.ro().QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE parent_task_id = ? AND project = ? AND status IN ('done','cancelled')",
		parentTaskID, project,
	).Scan(&doneCount)
	return doneCount >= total, total, doneCount
}

func (d *DB) GetTasksSince(project, since string, limit int) ([]models.Task, error) {
	if limit <= 0 {
		limit = 100
	}
	query := "SELECT " + taskColumns + " FROM tasks WHERE archived_at IS NULL AND (dispatched_at > ? OR accepted_at > ? OR started_at > ? OR completed_at > ?)"
	args := []any{since, since, since, since}
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	query += " ORDER BY dispatched_at ASC LIMIT ?"
	args = append(args, limit)

	rows, err := d.ro().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get tasks since: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ArchiveTasks soft-deletes tasks matching the given filters.
// status: "done", "cancelled", or "" for both done+cancelled. boardID: filter by board, or "" for all.
func (d *DB) ArchiveTasks(project, status, boardID string) (int64, error) {
	now := time.Now().UTC().Format(memoryTimeFmt)

	query := "UPDATE tasks SET archived_at = ? WHERE project = ? AND archived_at IS NULL"
	args := []any{now, project}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	} else {
		query += " AND status IN ('done', 'cancelled')"
	}

	if boardID != "" {
		query += " AND board_id = ?"
		args = append(args, boardID)
	}

	result, err := d.conn.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("archive tasks: %w", err)
	}
	return result.RowsAffected()
}

// ResolveTaskID resolves a short task ID prefix to a full UUID.
// Returns the full ID if exactly one match is found, or the original if it's already a full UUID.
func (d *DB) ResolveTaskID(prefix, project string) (string, error) {
	// If it looks like a full UUID (36 chars), skip prefix search
	if len(prefix) >= 36 {
		return prefix, nil
	}
	var ids []string
	rows, err := d.ro().Query("SELECT id FROM tasks WHERE id LIKE ? AND project = ?", prefix+"%", project)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return prefix, nil // let downstream report "not found"
	}
	if len(ids) > 1 {
		return "", fmt.Errorf("ambiguous task ID prefix %q (%d matches)", prefix, len(ids))
	}
	return ids[0], nil
}

func normalizePtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := normalize.JSONKeys(*s)
	return &v
}
