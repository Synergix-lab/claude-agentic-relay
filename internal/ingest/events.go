package ingest

import "time"

type EventType string

const (
	EventToolStart  EventType = "tool_start"
	EventToolEnd    EventType = "tool_end"
	EventStop       EventType = "stop"
	EventAgentSpawn EventType = "agent_spawn"
	EventAgentExit  EventType = "agent_exit"
	EventIdle       EventType = "idle"
	EventWaiting    EventType = "waiting"
)

type Activity string

const (
	ActivityTyping   Activity = "typing"
	ActivityReading  Activity = "reading"
	ActivityTerminal Activity = "terminal"
	ActivityBrowsing Activity = "browsing"
	ActivityThinking Activity = "thinking"
	ActivityIdle     Activity = "idle"
	ActivityWaiting  Activity = "waiting"
)

type AgentEvent struct {
	Type      EventType `json:"type"`
	SessionID string    `json:"session_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	File      string    `json:"file,omitempty"`
	Activity  Activity  `json:"activity"`
	Timestamp time.Time `json:"ts"`
}
