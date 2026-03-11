package ingest

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type hookEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	ParentID  string `json:"parent_id,omitempty"`
	Tool      string `json:"tool,omitempty"`
	File      string `json:"file,omitempty"`
	Timestamp string `json:"ts"`
}

type hooksWatcher struct {
	dir             string
	out             chan<- AgentEvent
	detector        *Detector
	sessionProvider SessionProvider
}

func newHooksWatcher(dir string, out chan<- AgentEvent, detector *Detector, sp SessionProvider) *hooksWatcher {
	return &hooksWatcher{dir: dir, out: out, detector: detector, sessionProvider: sp}
}

func (h *hooksWatcher) run(ctx context.Context) {
	// Ensure directory exists
	if err := os.MkdirAll(h.dir, 0755); err != nil {
		log.Printf("[ingest] failed to create hooks dir %s: %v", h.dir, err)
		return
	}

	// Process existing files (catch-up)
	h.processExisting()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[ingest] failed to create fsnotify watcher: %v", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(h.dir); err != nil {
		log.Printf("[ingest] failed to watch %s: %v", h.dir, err)
		return
	}

	log.Printf("[ingest] watching %s for hook events", h.dir)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				h.processFile(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[ingest] watcher error: %v", err)
		}
	}
}

func (h *hooksWatcher) processExisting() {
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		h.processFile(filepath.Join(h.dir, e.Name()))
	}
}

func (h *hooksWatcher) processFile(path string) {
	if !strings.HasSuffix(path, ".json") {
		return
	}
	// Skip .tmp files (atomic write in progress)
	if strings.HasSuffix(path, ".tmp.json") {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var he hookEvent
	if err := json.Unmarshal(data, &he); err != nil {
		log.Printf("[ingest] invalid JSON in %s: %v", filepath.Base(path), err)
		_ = os.Remove(path)
		return
	}

	// Note: we no longer filter by registered sessions here.
	// All events are ingested; the UI/API can filter by known sessions if needed.
	// This avoids the chicken-and-egg problem where agents need to register
	// before their activity can be tracked.

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339, he.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}

	evtType := EventType(he.Type)
	activity := MapToolToActivity(he.Tool)

	// Override activity for lifecycle events
	switch evtType {
	case EventAgentSpawn:
		activity = ActivityReading
	case EventAgentExit:
		activity = ActivityIdle
	case EventStop:
		activity = ActivityWaiting
	}

	evt := AgentEvent{
		Type:      evtType,
		SessionID: he.SessionID,
		ParentID:  he.ParentID,
		Tool:      he.Tool,
		File:      he.File,
		Activity:  activity,
		Timestamp: ts,
	}

	h.detector.RecordEvent(evt)

	select {
	case h.out <- evt:
	default:
		log.Printf("[ingest] event channel full, dropping event for session %s", he.SessionID)
	}

	_ = os.Remove(path)
}
