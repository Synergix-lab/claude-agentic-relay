package spawn

import (
	"sync"
	"time"
)

// CycleMetric records one cycle execution.
type CycleMetric struct {
	Agent               string        `json:"agent"`
	Project             string        `json:"project"`
	Cycle               string        `json:"cycle"`
	Duration            time.Duration `json:"duration_ms"`
	Success             bool          `json:"success"`
	ExitCode            int           `json:"exit_code"`
	Timestamp           time.Time     `json:"timestamp"`
	Error               string        `json:"error,omitempty"`
	InputTokens         int64         `json:"input_tokens"`
	OutputTokens        int64         `json:"output_tokens"`
	CacheReadTokens     int64         `json:"cache_read_tokens"`
	CacheCreationTokens int64         `json:"cache_creation_tokens"`
}

// MetricsCollector stores cycle metrics in memory (ring buffer).
type MetricsCollector struct {
	mu      sync.RWMutex
	history []CycleMetric
	maxSize int
}

// NewMetricsCollector creates a collector with a max history size.
func NewMetricsCollector(maxSize int) *MetricsCollector {
	return &MetricsCollector{
		history: make([]CycleMetric, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds a cycle metric.
func (mc *MetricsCollector) Record(m CycleMetric) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if len(mc.history) >= mc.maxSize {
		mc.history = mc.history[1:]
	}
	mc.history = append(mc.history, m)
}

// History returns the last N raw metrics.
func (mc *MetricsCollector) History(n int) []CycleMetric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if n > len(mc.history) {
		n = len(mc.history)
	}
	result := make([]CycleMetric, n)
	copy(result, mc.history[len(mc.history)-n:])
	return result
}
