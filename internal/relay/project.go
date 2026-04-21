package relay

import (
	"fmt"

	"agent-relay/internal/models"
)

// This file implements the paper's Def. 7 (Budget Projection):
//   M_boot = top-k(M_db, U, B_max), |M_boot| ≤ B_max
//
// Raw entities (Task, Goal, vault doc Content) are projected into compact
// summaries bounded by a byte budget before being injected into session_context
// or spawn prompts. The agent can always pay-for-what-you-use by calling
// get_task / get_goal / get_vault_doc for full content.

// taskDescPreview is the byte ceiling for a single task's description preview
// in a projection. Full descriptions are fetched via get_task(id) on demand.
const taskDescPreview = 200

// goalDescPreview bounds the goal description surfaced in goal_context.
const goalDescPreview = 200

// goalAncestryCap limits the ancestry depth in projected goal_context.
const goalAncestryCap = 3

// goalContextCap limits the number of unique goals surfaced in session_context.
const goalContextCap = 10

// vaultDocHeadBytes is kept verbatim when a vault doc is tail-truncated so the
// reader still sees the title / frontmatter.
const vaultDocHeadBytes = 200

// TaskSummary is the projected form of models.Task injected into session_context.
// Heavy fields (description, result, blocked_reason) are dropped or truncated;
// the full task is reachable via get_task(id).
type TaskSummary struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Priority       string  `json:"priority"`
	Status         string  `json:"status"`
	ProfileSlug    string  `json:"profile_slug,omitempty"`
	AssignedTo     *string `json:"assigned_to,omitempty"`
	DispatchedBy   string  `json:"dispatched_by,omitempty"`
	GoalID         *string `json:"goal_id,omitempty"`
	BoardID        *string `json:"board_id,omitempty"`
	DispatchedAt   string  `json:"dispatched_at,omitempty"`
	DescPreview    string  `json:"desc_preview,omitempty"`
	DescTruncated  bool    `json:"desc_truncated,omitempty"`
}

// priorityRank returns the integer rank used for stable priority ordering.
// Lower = more important (P0=0). Unknown values sort last.
func priorityRank(p string) int {
	switch p {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	}
	return 4
}

// summarizeTask converts a Task into a TaskSummary with a bounded description preview.
func summarizeTask(t models.Task) TaskSummary {
	s := TaskSummary{
		ID:           t.ID,
		Title:        t.Title,
		Priority:     t.Priority,
		Status:       t.Status,
		ProfileSlug:  t.ProfileSlug,
		AssignedTo:   t.AssignedTo,
		DispatchedBy: t.DispatchedBy,
		GoalID:       t.GoalID,
		BoardID:      t.BoardID,
		DispatchedAt: t.DispatchedAt,
	}
	if t.Description != "" {
		if len(t.Description) > taskDescPreview {
			s.DescPreview = t.Description[:taskDescPreview]
			s.DescTruncated = true
		} else {
			s.DescPreview = t.Description
		}
	}
	return s
}

// taskSummaryBytes estimates the serialized size of a TaskSummary (for budget accounting).
func taskSummaryBytes(s TaskSummary) int {
	n := len(s.ID) + len(s.Title) + len(s.Priority) + len(s.Status) +
		len(s.ProfileSlug) + len(s.DispatchedBy) + len(s.DispatchedAt) + len(s.DescPreview)
	if s.AssignedTo != nil {
		n += len(*s.AssignedTo)
	}
	if s.GoalID != nil {
		n += len(*s.GoalID)
	}
	if s.BoardID != nil {
		n += len(*s.BoardID)
	}
	// JSON structural overhead (quotes, commas, field names)
	n += 160
	return n
}

// projectTasks applies Def. 7 to a list of tasks: sort by priority (P0 first),
// summarize each, then greedily select until maxBytes is reached.
// P0 tasks always fit (they're surfaced even if the budget is blown — mirrors
// applyBudget's P0-bypass rule in internal/relay/budget.go).
func projectTasks(tasks []models.Task, maxBytes int) []TaskSummary {
	if len(tasks) == 0 {
		return []TaskSummary{}
	}

	// Stable sort: P0 first, then by dispatched_at DESC within priority.
	// insertion sort keeps it obvious and O(n²) is fine at our scale (≤100 items).
	sorted := make([]models.Task, len(tasks))
	copy(sorted, tasks)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, b := sorted[j-1], sorted[j]
			if priorityRank(a.Priority) < priorityRank(b.Priority) {
				break
			}
			if priorityRank(a.Priority) == priorityRank(b.Priority) && a.DispatchedAt >= b.DispatchedAt {
				break
			}
			sorted[j-1], sorted[j] = b, a
		}
	}

	var out []TaskSummary
	used := 0
	for _, t := range sorted {
		s := summarizeTask(t)
		b := taskSummaryBytes(s)
		// P0 always bypasses the budget
		if t.Priority == "P0" {
			out = append(out, s)
			used += b
			continue
		}
		if maxBytes > 0 && used+b > maxBytes {
			continue
		}
		out = append(out, s)
		used += b
	}
	return out
}

// projectGoal returns a copy of the goal with its description truncated to
// goalDescPreview bytes. The full description is available via get_goal(id).
func projectGoal(g models.Goal) models.Goal {
	if len(g.Description) > goalDescPreview {
		g.Description = g.Description[:goalDescPreview] + "…"
	}
	return g
}

// projectGoalAncestry truncates each ancestor and caps chain length.
func projectGoalAncestry(chain []models.Goal) []models.Goal {
	if len(chain) > goalAncestryCap {
		// Keep the root-most ancestors (the chain is already root-first).
		chain = chain[:goalAncestryCap]
	}
	out := make([]models.Goal, len(chain))
	for i, g := range chain {
		out[i] = projectGoal(g)
	}
	return out
}

// projectVaultDoc applies head+tail truncation with a human-readable marker when
// content exceeds maxBytes. Returns the original content untouched if it fits.
func projectVaultDoc(content, path string, maxBytes int) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	truncatedKB := (len(content) - maxBytes) / 1024
	marker := fmt.Sprintf("\n<!-- %d KB truncated, call get_vault_doc(%q) for full content -->\n", truncatedKB, path)
	head := content[:vaultDocHeadBytes]
	tailStart := len(content) - (maxBytes - vaultDocHeadBytes - len(marker))
	if tailStart <= vaultDocHeadBytes {
		// Budget too small for head+marker+tail; return head+marker.
		return head + marker
	}
	return head + marker + content[tailStart:]
}
