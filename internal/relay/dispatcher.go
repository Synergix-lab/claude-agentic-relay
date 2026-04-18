package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"agent-relay/internal/db"
)

// canonicalEvent returns the canonical dot-notation form of an event name.
// Both dot ("task.dispatched") and legacy underscore ("task_pending") forms
// are accepted on input; internally we always emit dot notation.
// This keeps triggers registered with either form working after the switch.
var eventAliases = map[string]string{
	// underscore (historical handler-side names) → canonical dot
	"task_pending":     "task.dispatched",
	"task_completed":   "task.completed",
	"task_blocked":     "task.blocked",
	"task_resumed":     "task.resumed",
	"message_received": "message.received",
	"signal:interrupt": "signal.interrupt",
	"signal:alert":     "signal.alert",
}

func canonicalEvent(event string) string {
	if c, ok := eventAliases[event]; ok {
		return c
	}
	return event
}

// eventMatchSet returns all event-name variants that should match the canonical form.
// Triggers registered with either dot or underscore notation fire for the same event.
func eventMatchSet(canonical string) []string {
	set := []string{canonical}
	for alias, c := range eventAliases {
		if c == canonical && alias != canonical {
			set = append(set, alias)
		}
	}
	return set
}

// fireTriggers queries matching triggers for the given project/event and spawns children.
// Also fires any workflow DAGs that have matching trigger:event nodes.
// Always called as a goroutine: go h.fireTriggers(project, event, meta)
func (h *Handlers) fireTriggers(project, event string, meta map[string]string) {
	event = canonicalEvent(event)

	// Fire workflow DAGs that match this event
	if h.wfEngine != nil {
		h.wfEngine.FireWorkflows(project, event, meta)
	}

	// Accept triggers registered under any alias for this canonical event
	var triggers []db.Trigger
	for _, name := range eventMatchSet(event) {
		triggers = append(triggers, h.db.ListTriggers(project, name)...)
	}
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
				if remaining := cooldown - time.Since(lastFired); remaining > 0 {
					msg := fmt.Sprintf("cooldown (%s remaining)", remaining.Truncate(time.Second))
					log.Printf("[dispatcher] trigger %s skipped — %s", t.ID, msg)
					h.db.RecordTriggerFire(t.ID, project, event, "", fmt.Errorf("%s", msg))
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
	event = canonicalEvent(event)

	var triggers []db.Trigger
	for _, name := range eventMatchSet(event) {
		triggers = append(triggers, h.db.ListTriggers(project, name)...)
	}
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
