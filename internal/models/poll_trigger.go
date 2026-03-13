package models

// PollTrigger defines a URL to check periodically and fire events when conditions match.
type PollTrigger struct {
	ID              string `json:"id"`
	Project         string `json:"project"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Headers         string `json:"headers"`          // JSON: {"Authorization": "Bearer ..."}
	ConditionPath   string `json:"condition_path"`   // dot-notation: "workflow_runs.0.conclusion"
	ConditionOp     string `json:"condition_op"`     // eq, neq, contains, gt, lt
	ConditionValue  string `json:"condition_value"`  // "success"
	PollInterval    string `json:"poll_interval"`    // "2m", "5m", "1h"
	FireEvent       string `json:"fire_event"`       // event name to fire when condition met
	FireMeta        string `json:"fire_meta"`        // JSON: extra metadata
	Enabled         bool   `json:"enabled"`
	LastPolledAt    string `json:"last_polled_at,omitempty"`
	LastResult      string `json:"last_result,omitempty"`
	LastMatched     bool   `json:"last_matched"`
	CooldownSeconds int    `json:"cooldown_seconds"`
	CreatedAt       string `json:"created_at"`
}
