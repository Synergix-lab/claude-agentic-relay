package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-relay/internal/models"
)

// StartPoller runs a background goroutine that polls URLs and fires triggers.
func (h *Handlers) StartPoller(done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				h.pollAll()
			}
		}
	}()
}

func (h *Handlers) pollAll() {
	triggers, err := h.db.GetDuePollTriggers()
	if err != nil {
		log.Printf("[poller] error fetching due triggers: %v", err)
		return
	}
	for _, pt := range triggers {
		matched, value, err := h.pollOnce(pt)
		if err != nil {
			log.Printf("[poller] %s/%s error: %v", pt.Project, pt.Name, err)
			_ = h.db.UpdatePollResult(pt.ID, fmt.Sprintf("error: %v", err), false)
			continue
		}
		_ = h.db.UpdatePollResult(pt.ID, value, matched)

		if matched {
			// Check cooldown
			if pt.LastPolledAt != "" && pt.LastMatched {
				if lastPolled, err := time.Parse("2006-01-02T15:04:05Z", pt.LastPolledAt); err == nil {
					cooldown := time.Duration(pt.CooldownSeconds) * time.Second
					if time.Since(lastPolled) < cooldown {
						continue
					}
				}
			}

			// Parse fire_meta
			meta := map[string]string{}
			if pt.FireMeta != "" && pt.FireMeta != "{}" {
				_ = json.Unmarshal([]byte(pt.FireMeta), &meta)
			}
			meta["poll_trigger"] = pt.Name
			meta["matched_value"] = value

			go h.fireTriggers(pt.Project, pt.FireEvent, meta)
			log.Printf("[poller] %s/%s matched → firing %s", pt.Project, pt.Name, pt.FireEvent)
		}
	}
}

// pollOnce fetches the URL and evaluates the condition.
func (h *Handlers) pollOnce(pt models.PollTrigger) (matched bool, value string, err error) {
	req, err := http.NewRequest("GET", pt.URL, nil)
	if err != nil {
		return false, "", fmt.Errorf("create request: %w", err)
	}

	// Apply headers
	if pt.Headers != "" && pt.Headers != "{}" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(pt.Headers), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return false, "", fmt.Errorf("read body: %w", err)
	}

	value, err = extractJSONPath(body, pt.ConditionPath)
	if err != nil {
		return false, "", fmt.Errorf("extract path %q: %w", pt.ConditionPath, err)
	}

	matched = evaluateCondition(value, pt.ConditionOp, pt.ConditionValue)
	return matched, value, nil
}

// PollOnceByID fetches and evaluates a single poll trigger (for test endpoint).
func (h *Handlers) PollOnceByID(id string) (matched bool, value string, err error) {
	pt, err := h.db.GetPollTrigger(id)
	if err != nil || pt == nil {
		return false, "", fmt.Errorf("poll trigger not found")
	}
	return h.pollOnce(*pt)
}

// evaluateCondition compares a value against an expected value with the given operator.
func evaluateCondition(value, op, expected string) bool {
	switch op {
	case "eq":
		return value == expected
	case "neq":
		return value != expected
	case "contains":
		return strings.Contains(value, expected)
	case "gt":
		v, errV := strconv.ParseFloat(value, 64)
		e, errE := strconv.ParseFloat(expected, 64)
		if errV != nil || errE != nil {
			return value > expected // string comparison fallback
		}
		return v > e
	case "lt":
		v, errV := strconv.ParseFloat(value, 64)
		e, errE := strconv.ParseFloat(expected, 64)
		if errV != nil || errE != nil {
			return value < expected
		}
		return v < e
	default:
		return value == expected
	}
}

// extractJSONPath extracts a value from JSON using dot-notation (e.g., "workflow_runs.0.conclusion").
func extractJSONPath(body []byte, path string) (string, error) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parse JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return "", fmt.Errorf("key %q not found", part)
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return "", fmt.Errorf("invalid array index %q", part)
			}
			current = v[idx]
		default:
			return "", fmt.Errorf("cannot traverse %T with key %q", current, part)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "null", nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}
