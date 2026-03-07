package vault

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-relay/internal/db"
	"agent-relay/internal/models"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	db      *db.DB
	mu      sync.Mutex
	watched map[string]context.CancelFunc // project → cancel
	ctx     context.Context
	cancel  context.CancelFunc
}

func New(database *db.DB) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		db:      database,
		watched: make(map[string]context.CancelFunc),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start loads all vault configs from DB and starts watching them.
func (w *Watcher) Start() {
	configs, err := w.db.ListVaultConfigs()
	if err != nil {
		log.Printf("[vault] failed to load vault configs: %v", err)
		return
	}
	for _, cfg := range configs {
		w.AddVault(cfg)
	}
}

// AddVault indexes a vault and starts watching it for changes.
// Safe to call multiple times for the same project (restarts the watcher).
func (w *Watcher) AddVault(cfg models.VaultConfig) {
	w.mu.Lock()
	// Stop existing watcher for this project if any
	if cancelFn, ok := w.watched[cfg.Project]; ok {
		cancelFn()
	}
	w.mu.Unlock()

	// Index all files
	count := w.indexAll(cfg)
	log.Printf("[vault] indexed %d docs from %s (project: %s)", count, cfg.Path, cfg.Project)

	// Start watcher in background
	vaultCtx, vaultCancel := context.WithCancel(w.ctx)
	w.mu.Lock()
	w.watched[cfg.Project] = vaultCancel
	w.mu.Unlock()

	go w.watch(vaultCtx, cfg)
}

func (w *Watcher) Stop() {
	w.cancel()
}

func (w *Watcher) indexAll(cfg models.VaultConfig) int {
	count := 0
	_ = filepath.Walk(cfg.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		// Skip hidden dirs (.obsidian, .git, etc.)
		rel, _ := filepath.Rel(cfg.Path, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			return nil
		}

		if w.indexFile(cfg, path) == nil {
			count++
		}
		return nil
	})
	return count
}

func (w *Watcher) indexFile(cfg models.VaultConfig, absPath string) error {
	rel, err := filepath.Rel(cfg.Path, absPath)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	content := string(data)
	fm := parseFrontmatter(content)
	body := stripFrontmatter(content)

	// Extract title: from frontmatter or first H1
	title := fm["title"]
	if title == "" {
		title = extractFirstH1(body)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(absPath), ".md")
	}

	// Tags as JSON array
	tagsJSON := "[]"
	if tagsRaw, ok := fm["tags"]; ok {
		tagsJSON = tagsRaw
	}

	// Updated from frontmatter or file mtime
	info, _ := os.Stat(absPath)
	updatedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if u, ok := fm["updated"]; ok && u != "" {
		updatedAt = u
	} else if info != nil {
		updatedAt = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
	}

	doc := &models.VaultDoc{
		Path:      rel,
		Project:   cfg.Project,
		Title:     title,
		Owner:     fm["owner"],
		Status:    fm["status"],
		Tags:      tagsJSON,
		Content:   body,
		SizeBytes: len(data),
		UpdatedAt: updatedAt,
	}

	return w.db.UpsertVaultDoc(doc)
}

func (w *Watcher) watch(ctx context.Context, cfg models.VaultConfig) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[vault] failed to create watcher for %s: %v", cfg.Path, err)
		return
	}
	defer watcher.Close()

	// Add all vault dirs recursively
	_ = filepath.Walk(cfg.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(cfg.Path, path)
		if rel != "." && (strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.")) {
			return filepath.SkipDir
		}
		watcher.Add(path)
		return nil
	})
	log.Printf("[vault] watching %s for changes (project: %s)", cfg.Path, cfg.Project)

	pending := map[string]time.Time{}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				pending[event.Name] = time.Now()
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				rel, _ := filepath.Rel(cfg.Path, event.Name)
				w.db.DeleteVaultDoc(cfg.Project, rel)
				log.Printf("[vault] removed %s (project: %s)", rel, cfg.Project)
			}
		case <-ticker.C:
			now := time.Now()
			for path, t := range pending {
				if now.Sub(t) < 300*time.Millisecond {
					continue
				}
				delete(pending, path)
				if err := w.indexFile(cfg, path); err == nil {
					rel, _ := filepath.Rel(cfg.Path, path)
					log.Printf("[vault] re-indexed %s (project: %s)", rel, cfg.Project)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[vault] watcher error: %v", err)
		}
	}
}

// parseFrontmatter extracts YAML frontmatter key-value pairs.
func parseFrontmatter(content string) map[string]string {
	result := map[string]string{}
	if !strings.HasPrefix(content, "---") {
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Scan() // skip first ---

	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "title", "owner", "status", "updated", "priority":
			result[key] = value
		case "tags":
			value = strings.Trim(value, "[] ")
			if value == "" {
				result["tags"] = "[]"
				continue
			}
			tags := strings.Split(value, ",")
			cleaned := make([]string, 0, len(tags))
			for _, t := range tags {
				t = strings.TrimSpace(t)
				t = strings.Trim(t, "\"'")
				if t != "" {
					cleaned = append(cleaned, t)
				}
			}
			b, _ := json.Marshal(cleaned)
			result["tags"] = string(b)
		}
	}

	return result
}

func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	idx := strings.Index(content[3:], "---")
	if idx == -1 {
		return content
	}
	return strings.TrimSpace(content[3+idx+3:])
}

func extractFirstH1(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}
