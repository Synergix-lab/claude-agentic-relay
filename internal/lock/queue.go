package lock

import (
	"log/slog"
	"sync"
)

// PriorityQueue manages pending cycles per agent.
// Only one cycle can be queued per agent (latest wins).
type PriorityQueue struct {
	mu      sync.Mutex
	pending map[string]string // agent -> pending cycle name
	logger  *slog.Logger
}

// NewPriorityQueue creates a new priority queue.
func NewPriorityQueue(logger *slog.Logger) *PriorityQueue {
	return &PriorityQueue{
		pending: make(map[string]string),
		logger:  logger,
	}
}

// Enqueue adds a cycle to the queue. Replaces any existing pending cycle.
func (q *PriorityQueue) Enqueue(agent string, cycle string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if existing, ok := q.pending[agent]; ok {
		q.logger.Info("cycle replaced in queue",
			"agent", agent,
			"new", cycle,
			"replaced", existing,
		)
	}
	q.pending[agent] = cycle
	q.logger.Info("cycle queued", "agent", agent, "cycle", cycle)
}

// Dequeue returns and removes the next pending cycle for the agent.
func (q *PriorityQueue) Dequeue(agent string) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cycle, ok := q.pending[agent]
	if ok {
		delete(q.pending, agent)
	}
	return cycle, ok
}

// Clear removes all pending cycles for an agent.
func (q *PriorityQueue) Clear(agent string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.pending, agent)
}
