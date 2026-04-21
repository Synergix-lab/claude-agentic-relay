package relay

import (
	"strings"
	"testing"

	"agent-relay/internal/models"
)

func TestSummarizeTask_TruncatesLongDescription(t *testing.T) {
	longDesc := strings.Repeat("x", 5000)
	s := summarizeTask(models.Task{
		ID: "abc", Title: "t", Priority: "P2", Status: "pending",
		Description: longDesc,
	})
	if !s.DescTruncated {
		t.Fatal("expected desc_truncated=true for 5KB description")
	}
	if len(s.DescPreview) != taskDescPreview {
		t.Fatalf("desc_preview len: got %d, want %d", len(s.DescPreview), taskDescPreview)
	}
}

func TestSummarizeTask_ShortDescriptionKept(t *testing.T) {
	s := summarizeTask(models.Task{
		ID: "abc", Title: "t", Priority: "P2", Status: "pending",
		Description: "short",
	})
	if s.DescTruncated {
		t.Fatal("did not expect desc_truncated for short description")
	}
	if s.DescPreview != "short" {
		t.Fatalf("desc_preview: got %q", s.DescPreview)
	}
}

func TestProjectTasks_EnforcesBudget(t *testing.T) {
	var tasks []models.Task
	for i := 0; i < 50; i++ {
		tasks = append(tasks, models.Task{
			ID:          "task-id-" + strings.Repeat("z", 4),
			Title:       "title number " + strings.Repeat("t", 20),
			Priority:    "P2",
			Status:      "pending",
			Description: strings.Repeat("d", 10000),
		})
	}

	out := projectTasks(tasks, 2000)
	used := 0
	for _, s := range out {
		used += taskSummaryBytes(s)
	}
	if used > 2500 { // small slack for overhead computation
		t.Fatalf("budget exceeded: used %d > 2000+slack", used)
	}
}

func TestProjectTasks_P0AlwaysIncluded(t *testing.T) {
	tasks := []models.Task{
		{ID: "low1", Title: "low priority", Priority: "P3", Status: "pending"},
		{ID: "crit", Title: "critical", Priority: "P0", Status: "pending"},
	}
	// Budget=0 means "no budget" per projectTasks; use a tiny non-zero budget.
	out := projectTasks(tasks, 10)
	foundP0 := false
	for _, s := range out {
		if s.ID == "crit" {
			foundP0 = true
		}
	}
	if !foundP0 {
		t.Fatal("P0 task must bypass budget")
	}
}

func TestProjectTasks_SortsByPriority(t *testing.T) {
	tasks := []models.Task{
		{ID: "p3", Title: "p3", Priority: "P3", Status: "pending", DispatchedAt: "2026-01-01"},
		{ID: "p0", Title: "p0", Priority: "P0", Status: "pending", DispatchedAt: "2026-01-02"},
		{ID: "p1", Title: "p1", Priority: "P1", Status: "pending", DispatchedAt: "2026-01-03"},
	}
	out := projectTasks(tasks, 0)
	if len(out) != 3 {
		t.Fatalf("expected 3 out, got %d", len(out))
	}
	if out[0].ID != "p0" || out[1].ID != "p1" || out[2].ID != "p3" {
		t.Fatalf("wrong priority order: %s, %s, %s", out[0].ID, out[1].ID, out[2].ID)
	}
}

func TestProjectGoal_TruncatesDescription(t *testing.T) {
	g := models.Goal{
		Title:       "g",
		Description: strings.Repeat("y", 5000),
	}
	out := projectGoal(g)
	// +1 char for trailing "…"
	if len(out.Description) > goalDescPreview+4 {
		t.Fatalf("goal desc not truncated: len=%d", len(out.Description))
	}
}

func TestProjectGoalAncestry_CapsChain(t *testing.T) {
	chain := []models.Goal{
		{ID: "g1", Title: "g1"},
		{ID: "g2", Title: "g2"},
		{ID: "g3", Title: "g3"},
		{ID: "g4", Title: "g4"},
		{ID: "g5", Title: "g5"},
	}
	out := projectGoalAncestry(chain)
	if len(out) != goalAncestryCap {
		t.Fatalf("ancestry not capped: len=%d want %d", len(out), goalAncestryCap)
	}
}

func TestProjectVaultDoc_ShortDocPassThrough(t *testing.T) {
	doc := "small content"
	out := projectVaultDoc(doc, "x.md", 1000)
	if out != doc {
		t.Fatalf("short doc should be unchanged")
	}
}

func TestProjectVaultDoc_LargeDocTruncatedWithMarker(t *testing.T) {
	doc := strings.Repeat("a", 50000)
	out := projectVaultDoc(doc, "big.md", 5000)
	if len(out) > 5200 { // small slack for marker
		t.Fatalf("truncated doc too large: len=%d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("truncation marker missing: %q", out[:200])
	}
	if !strings.Contains(out, "get_vault_doc") {
		t.Fatalf("agent recovery hint missing")
	}
}
