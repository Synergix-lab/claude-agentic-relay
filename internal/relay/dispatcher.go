package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// fireTriggers queries matching triggers for the given project/event and spawns children.
// Also fires any workflow DAGs that have matching trigger:event nodes.
// Always called as a goroutine: go h.fireTriggers(project, event, meta)
func (h *Handlers) fireTriggers(project, event string, meta map[string]string) {
	// Fire workflow DAGs that match this event
	if h.wfEngine != nil {
		h.wfEngine.FireWorkflows(project, event, meta)
	}

	triggers := h.db.ListTriggers(project, event)
	if len(triggers) == 0 {
		return
	}

	for _, t := range triggers {
		// Evaluate match rules first (don't waste cooldown on non-matching events)
		if !matchesRules(t.MatchRules, meta) {
			continue
		}

		// Cooldown check
		if t.LastFiredAt != "" {
			if lastFired, err := time.Parse("2006-01-02T15:04:05Z", t.LastFiredAt); err == nil {
				cooldown := time.Duration(t.CooldownSeconds) * time.Second
				if time.Since(lastFired) < cooldown {
					log.Printf("[dispatcher] trigger %s skipped (cooldown %ds)", t.ID, t.CooldownSeconds)
					continue
				}
			}
		}

		// Spawn via SpawnWithContext if spawn manager is available
		var childID string
		var spawnErr error

		if h.spawnMgr != nil {
			// Derive a task_id from meta if available
			taskID := meta["task_id"]
			childID, spawnErr = h.spawnMgr.SpawnWithContext(project, t.ProfileSlug, t.Cycle, taskID)
		} else {
			spawnErr = nil
			childID = ""
			log.Printf("[dispatcher] trigger %s matched but spawn manager not available", t.ID)
		}

		// Record the fire
		h.db.RecordTriggerFire(t.ID, project, event, childID, spawnErr)

		if spawnErr != nil {
			log.Printf("[dispatcher] trigger %s spawn error: %v", t.ID, spawnErr)
		} else if childID != "" {
			log.Printf("[dispatcher] trigger %s fired → child %s (profile=%s cycle=%s)", t.ID, childID, t.ProfileSlug, t.Cycle)
		}
	}
}

// matchesRules evaluates whether the metadata matches the trigger's JSON rules.
// Rules are a flat JSON object: {"key": "value"} — all must match.
// Special prefixes: ">" for greater-than comparison on durations/numbers.
func matchesRules(rulesJSON string, meta map[string]string) bool {
	if rulesJSON == "" || rulesJSON == "{}" {
		return true
	}

	var rules map[string]string
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return true // malformed rules = match all
	}

	for key, expected := range rules {
		actual, ok := meta[key]
		if !ok {
			return false
		}
		if strings.HasPrefix(expected, ">") {
			// Duration/numeric comparison
			threshold := strings.TrimPrefix(expected, ">")
			if !compareDuration(actual, threshold) {
				return false
			}
		} else if actual != expected {
			return false
		}
	}
	return true
}

// compareDuration returns true if actual duration > threshold duration.
func compareDuration(actual, threshold string) bool {
	a, errA := time.ParseDuration(actual)
	t, errT := time.ParseDuration(threshold)
	if errA != nil || errT != nil {
		return false
	}
	return a > t
}

// flattenJSON converts top-level JSON keys to a string map (for webhook payloads).
func flattenJSON(data []byte) map[string]string {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			result[k] = val
		case float64:
			if val == float64(int64(val)) {
				result[k] = fmt.Sprintf("%d", int64(val))
			} else {
				result[k] = fmt.Sprintf("%g", val)
			}
		case bool:
			if val {
				result[k] = "true"
			} else {
				result[k] = "false"
			}
		default:
			b, _ := json.Marshal(val)
			result[k] = string(b)
		}
	}
	return result
}

// webhookResult is the response for POST /api/webhooks/:project/:event.
type webhookResult struct {
	TriggerID string `json:"trigger_id"`
	ChildID   string `json:"child_id,omitempty"`
}

type webhookSkipped struct {
	TriggerID string `json:"trigger_id"`
	Reason    string `json:"reason"`
}

// fireTriggersSync is like fireTriggers but returns results (for webhook endpoint).
func (h *Handlers) fireTriggersSync(project, event string, meta map[string]string) (fires []webhookResult, skipped []webhookSkipped) {
	triggers := h.db.ListTriggers(project, event)
	if len(triggers) == 0 {
		return
	}

	for _, t := range triggers {
		// Evaluate match rules first (don't waste cooldown on non-matching events)
		if !matchesRules(t.MatchRules, meta) {
			skipped = append(skipped, webhookSkipped{TriggerID: t.ID, Reason: "rules_mismatch"})
			continue
		}

		// Cooldown check
		if t.LastFiredAt != "" {
			if lastFired, err := time.Parse("2006-01-02T15:04:05Z", t.LastFiredAt); err == nil {
				cooldown := time.Duration(t.CooldownSeconds) * time.Second
				if time.Since(lastFired) < cooldown {
					skipped = append(skipped, webhookSkipped{TriggerID: t.ID, Reason: "cooldown"})
					continue
				}
			}
		}

		var childID string
		var spawnErr error

		if h.spawnMgr != nil {
			taskID := meta["task_id"]
			childID, spawnErr = h.spawnMgr.SpawnWithContext(project, t.ProfileSlug, t.Cycle, taskID)
		} else {
			skipped = append(skipped, webhookSkipped{TriggerID: t.ID, Reason: "spawn_unavailable"})
			h.db.RecordTriggerFire(t.ID, project, event, "", nil)
			continue
		}

		h.db.RecordTriggerFire(t.ID, project, event, childID, spawnErr)

		if spawnErr != nil {
			skipped = append(skipped, webhookSkipped{TriggerID: t.ID, Reason: spawnErr.Error()})
		} else {
			fires = append(fires, webhookResult{TriggerID: t.ID, ChildID: childID})
		}
	}
	return
}

// newTriggerFireID generates a UUID for trigger history records.
func newTriggerFireID() string {
	return uuid.New().String()
}
