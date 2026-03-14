package spawn

import (
	"sync"
)

// LiveBuffer stores real-time output per agent during cycle execution.
type LiveBuffer struct {
	mu      sync.RWMutex
	buffers map[string]*agentOutput
}

type agentOutput struct {
	Data    []byte
	Running bool
	Cycle   string
}

// NewLiveBuffer creates a new live buffer.
func NewLiveBuffer() *LiveBuffer {
	return &LiveBuffer{
		buffers: make(map[string]*agentOutput),
	}
}

// Start marks an agent as running and clears previous output.
func (lb *LiveBuffer) Start(agent, cycle string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.buffers[agent] = &agentOutput{
		Data:    nil,
		Running: true,
		Cycle:   cycle,
	}
}

// Writer returns an io.Writer for the agent's live output.
func (lb *LiveBuffer) Writer(agent string) *AgentWriter {
	return &AgentWriter{lb: lb, agent: agent}
}

// Finish marks agent as done.
func (lb *LiveBuffer) Finish(agent string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if buf, ok := lb.buffers[agent]; ok {
		buf.Running = false
	}
}

// Get returns current output for an agent.
func (lb *LiveBuffer) Get(agent string) (data string, running bool, cycle string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	buf, ok := lb.buffers[agent]
	if !ok {
		return "", false, ""
	}
	return string(buf.Data), buf.Running, buf.Cycle
}

// AgentWriter is an io.Writer that appends to a specific agent's buffer.
type AgentWriter struct {
	lb    *LiveBuffer
	agent string
}

func (w *AgentWriter) Write(p []byte) (n int, err error) {
	w.lb.mu.Lock()
	defer w.lb.mu.Unlock()
	if buf, ok := w.lb.buffers[w.agent]; ok {
		// Cap at 64KB to avoid memory issues
		if len(buf.Data)+len(p) > 64*1024 {
			// Keep last 32KB + new data
			buf.Data = append(buf.Data[len(buf.Data)-32*1024:], p...)
		} else {
			buf.Data = append(buf.Data, p...)
		}
	}
	return len(p), nil
}
