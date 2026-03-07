package relay

import (
	"sync"
	"time"
)

// MCPEvent represents a visual event triggered by an MCP tool call.
type MCPEvent struct {
	Type    string `json:"type"`              // event group: memory, task, register, sleep, vault, goal, team
	Action  string `json:"action"`            // specific action: set, search, dispatch, claim, complete, block, etc.
	Agent   string `json:"agent"`             // agent that triggered it
	Project string `json:"project"`           // project scope
	Target  string `json:"target,omitempty"`  // target agent/profile (for dispatch, team ops)
	Label   string `json:"label,omitempty"`   // short label (task title, memory key, etc.)
	TS      int64  `json:"ts"`                // unix ms
}

// EventBus broadcasts MCP events to SSE subscribers.
type EventBus struct {
	mu   sync.RWMutex
	subs map[chan MCPEvent]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan MCPEvent]struct{})}
}

func (b *EventBus) Emit(evt MCPEvent) {
	evt.TS = time.Now().UnixMilli()
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- evt:
		default: // drop if slow
		}
	}
}

func (b *EventBus) Subscribe() chan MCPEvent {
	ch := make(chan MCPEvent, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Unsubscribe(ch chan MCPEvent) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}
