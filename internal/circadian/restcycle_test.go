package circadian

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/knowledge"

	_ "modernc.org/sqlite"
)

// TestDedupeDocuments verifies that duplicate rows (same path + hash) are
// removed keeping the first row of each group.
func TestDedupeDocuments(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "dedupe.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	// documents table without UNIQUE(path) so duplicates are representable.
	if _, err := db.Exec(`CREATE TABLE documents (
		id    TEXT PRIMARY KEY,
		path  TEXT NOT NULL,
		hash  TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	inserts := []struct{ id, path, hash string }{
		{"id-1", "/docs/a.md", "hash-1"},
		{"id-2", "/docs/a.md", "hash-1"}, // duplicate of id-1
		{"id-3", "/docs/b.md", "hash-2"}, // unique
		{"id-4", "/docs/c.md", "hash-3"},
		{"id-5", "/docs/c.md", "hash-3"}, // duplicate of id-4
	}
	for _, row := range inserts {
		if _, err := db.Exec(`INSERT INTO documents (id, path, hash) VALUES (?, ?, ?)`,
			row.id, row.path, row.hash); err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	removed, err := dedupeDocuments(db)
	if err != nil {
		t.Fatalf("dedupeDocuments: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 duplicates removed, got %d", removed)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 remaining rows, got %d", count)
	}

	// The kept row must be the first inserted (MIN(id) → id-1).
	var keptID string
	if err := db.QueryRow("SELECT id FROM documents WHERE path = ?", "/docs/a.md").Scan(&keptID); err != nil {
		t.Fatalf("query kept row: %v", err)
	}
	if keptID != "id-1" {
		t.Errorf("expected kept row id-1, got %s", keptID)
	}
}

// TestRunWisdomDecay verifies that learnings with old timestamps are
// deprecated (moved to audit/expired-entries.md) while fresh ones are kept.
func TestRunWisdomDecay(t *testing.T) {
	dir := t.TempDir()
	memoryAgentDir := filepath.Join(dir, "memory", "agent")
	agentDir := filepath.Join(memoryAgentDir, "cosca-test")
	auditDir := filepath.Join(dir, "memory", "audit")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}

	old := time.Now().AddDate(0, 0, -400).Format("2006-01-02")  // 400 days → 0.2 → deprecated
	recent := time.Now().AddDate(0, 0, -5).Format("2006-01-02") // 5 days → 1.0 → kept

	content := fmt.Sprintf(`# cosca-test — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## L01 | %s | Ancient Learning | Level 2

| Field | Value |
|-------|-------|
| **Learned** | old fact that is no longer true |

## L02 | %s | Recent Learning | Level 3

| Field | Value |
|-------|-------|
| **Learned** | fresh fact |
`, old, recent)

	learningsPath := filepath.Join(agentDir, "learnings.md")
	if err := os.WriteFile(learningsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write learnings: %v", err)
	}

	deprecated, err := runWisdomDecay(memoryAgentDir, auditDir)
	if err != nil {
		t.Fatalf("runWisdomDecay: %v", err)
	}
	if deprecated != 1 {
		t.Errorf("expected 1 deprecated entry, got %d", deprecated)
	}

	// The expired entry must have been appended to the audit file.
	expiredPath := filepath.Join(auditDir, "expired-entries.md")
	data, err := os.ReadFile(expiredPath)
	if err != nil {
		t.Fatalf("read expired entries: %v", err)
	}
	expired := string(data)
	if !strings.Contains(expired, "L01") {
		t.Errorf("expired-entries.md should contain the deprecated L01 entry")
	}
	if strings.Contains(expired, "L02") {
		t.Errorf("expired-entries.md should NOT contain the fresh L02 entry")
	}

	// The source file must keep only the fresh entry.
	src, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read source learnings: %v", err)
	}
	if strings.Contains(string(src), "Ancient Learning") {
		t.Errorf("source learnings.md should no longer contain the deprecated entry")
	}
	if !strings.Contains(string(src), "Recent Learning") {
		t.Errorf("source learnings.md should keep the fresh entry")
	}
}

// TestRunORC_NoDir verifies that a non-existent cosca dir yields a result
// with all steps skipped and no fatal error.
func TestRunORC_NoDir(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), "does-not-exist")

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC should not fail fatally for a missing dir, got: %v", err)
	}
	if result == nil {
		t.Fatal("RunORC returned a nil result")
	}
	if len(result.Steps) != 9 {
		t.Errorf("expected 9 steps, got %d", len(result.Steps))
	}
	for _, s := range result.Steps {
		if s.Status != "skipped" {
			t.Errorf("step %s: expected status skipped, got %q (%s)", s.Name, s.Status, s.Detail)
		}
		if s.DurationMs < 0 {
			t.Errorf("step %s: negative duration %d", s.Name, s.DurationMs)
		}
	}
	if result.Duration < 0 {
		t.Errorf("expected non-negative cycle duration, got %v", result.Duration)
	}
	if result.ReportPath != "" {
		t.Errorf("expected empty report path for missing dir, got %s", result.ReportPath)
	}
	// NOTE: Duration may legitimately be 0 and EndedAt equal to StartedAt here:
	// with no cosca dir all 8 steps are skipped instantly, and on coarse clocks
	// (Windows time.Now() granularity ~0.5ms) the cycle measures 0s.
	if result.EndedAt.Before(result.StartedAt) {
		t.Errorf("expected EndedAt not before StartedAt")
	}
}

// TestRunORC_WithTempDir verifies that a minimal cosca dir runs the full
// pipeline and produces the audit report.
func TestRunORC_WithTempDir(t *testing.T) {
	coscaDir := t.TempDir()

	// Minimal structure: one knowledge category, one agent learnings file and
	// an (empty) knowledge.db.
	repoPath := filepath.Join(coscaDir, "fallback", "knowledge", "patterns")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	doc := `---
title: Test Pattern
tags: [test]
---
# Test Pattern

A test knowledge entry for the ORC pipeline.
`
	if err := os.WriteFile(filepath.Join(repoPath, "test-pattern.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write pattern: %v", err)
	}

	agentDir := filepath.Join(coscaDir, "memory", "agent", "cosca-test")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	learnings := fmt.Sprintf(`# cosca-test — Semantic Learnings

## L01 | %s | Fresh Learning | Level 2

| Field | Value |
|-------|-------|
| **Learned** | fresh fact |
`, today)
	if err := os.WriteFile(filepath.Join(agentDir, "learnings.md"), []byte(learnings), 0o644); err != nil {
		t.Fatalf("write learnings: %v", err)
	}

	// Pre-create an empty knowledge.db file so step 2 finds it.
	dbPath := filepath.Join(coscaDir, "knowledge.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		t.Fatalf("init db file: %v", err)
	}
	_ = db.Close()

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC: %v", err)
	}
	if result == nil {
		t.Fatal("RunORC returned a nil result")
	}
	if len(result.Steps) != 9 {
		t.Fatalf("expected 9 steps, got %d", len(result.Steps))
	}
	for _, s := range result.Steps {
		switch s.Status {
		case "ok", "skipped", "error":
		default:
			t.Errorf("step %s has invalid status %q", s.Name, s.Status)
		}
		if s.DurationMs < 0 {
			t.Errorf("step %s: negative duration %d", s.Name, s.DurationMs)
		}
	}

	// The report must exist and cover all steps.
	if result.ReportPath == "" {
		t.Fatal("expected report path to be set")
	}
	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatalf("report file not found: %v", err)
	}
	report, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportStr := string(report)
	for _, name := range []string{
		"compact_learnings", "update_indexes", "dedupe",
		"recalc_confidence", "ckl_promotion", "consolidate_knowledge",
		"wisdom_decay", "generate_report",
	} {
		if !strings.Contains(reportStr, name) {
			t.Errorf("report missing step %s", name)
		}
	}
	for _, marker := range []string{"# Operational Rest Cycle (ORC) Report", "## Summary", "## Steps"} {
		if !strings.Contains(reportStr, marker) {
			t.Errorf("report missing section %q", marker)
		}
	}

	// The report file name must follow the rest-cycle-{timestamp}.md pattern.
	if base := filepath.Base(result.ReportPath); !strings.HasPrefix(base, "rest-cycle-") || !strings.HasSuffix(base, ".md") {
		t.Errorf("unexpected report filename %q", base)
	}

	if result.Duration <= 0 {
		t.Errorf("expected positive cycle duration, got %v", result.Duration)
	}
	if !result.EndedAt.After(result.StartedAt) {
		t.Errorf("expected EndedAt after StartedAt")
	}
}

// TestRunORC_ContextCancelled verifies that a cancelled context ends the
// cycle early with a non-nil error instead of hanging.
func TestRunORC_ContextCancelled(t *testing.T) {
	coscaDir := t.TempDir()
	repoPath := filepath.Join(coscaDir, "fallback", "knowledge", "patterns")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "p.md"), []byte("# P\n\nx\n"), 0o644); err != nil {
		t.Fatalf("write pattern: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the cycle starts

	result, err := RunORC(ctx, coscaDir)
	if err == nil {
		t.Fatal("expected a non-nil error for a cancelled context")
	}
	if result == nil {
		t.Fatal("expected a non-nil result even on cancellation")
	}
}

// =============================================================================
// CKL integration — ckl_promotion step + CKL Status report section
// =============================================================================

// writeLawsFile writes a runtime laws.json (CKL) under <coscaDir>/knowledge
// using the PromotionEngine — the same layout `cosca knowledge law` uses.
func writeLawsFile(t *testing.T, coscaDir string, items ...*knowledge.KnowledgeItem) {
	t.Helper()
	engine := knowledge.NewPromotionEngine()
	for _, it := range items {
		if err := engine.Register(it); err != nil {
			t.Fatalf("register law: %v", err)
		}
	}
	if err := engine.Save(filepath.Join(coscaDir, "knowledge", "laws.json")); err != nil {
		t.Fatalf("save laws.json: %v", err)
	}
}

// readLawsItems reads back the CKL items persisted in
// <coscaDir>/knowledge/laws.json.
func readLawsItems(t *testing.T, coscaDir string) []*knowledge.KnowledgeItem {
	t.Helper()
	engine := knowledge.NewPromotionEngine()
	if err := engine.Load(filepath.Join(coscaDir, "knowledge", "laws.json")); err != nil {
		t.Fatalf("load laws.json: %v", err)
	}
	return engine.All()
}

// findStep returns the ORC step with the given name, or nil.
func findStep(steps []ORCStep, name string) *ORCStep {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

// TestRunORC_CKLPromotion verifies that the ckl_promotion step evaluates the
// CKL library from laws.json: 1 item with 3 evidences and confidence 0.95 is
// already learning and stays learning after the cycle.
func TestRunORC_CKLPromotion(t *testing.T) {
	coscaDir := t.TempDir()
	writeLawsFile(t, coscaDir, &knowledge.KnowledgeItem{
		ID:         "K-1",
		Title:      "Lei de teste",
		Confidence: 0.95,
		Evidence: []knowledge.Evidence{
			{ID: "ev-1", Kind: "test", Source: "test/", Description: "e1", Timestamp: time.Now()},
			{ID: "ev-2", Kind: "test", Source: "test/", Description: "e2", Timestamp: time.Now()},
			{ID: "ev-3", Kind: "test", Source: "test/", Description: "e3", Timestamp: time.Now()},
		},
	})

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC: %v", err)
	}

	step := findStep(result.Steps, "ckl_promotion")
	if step == nil {
		t.Fatal("ckl_promotion step not found in pipeline")
	}
	if step.Status != "ok" {
		t.Errorf("ckl_promotion status = %q, want ok (%s)", step.Status, step.Detail)
	}
	if !strings.Contains(step.Detail, "evaluated 1 items") {
		t.Errorf("detail should report the item count, got: %s", step.Detail)
	}
	if !strings.Contains(step.Detail, "0 promoted") {
		t.Errorf("detail should report 0 promotions (already learning), got: %s", step.Detail)
	}

	if result.CKLLaws != 1 {
		t.Errorf("CKLLaws = %d, want 1", result.CKLLaws)
	}
	if result.CKLByLevel[string(knowledge.LevelLearning)] != 1 {
		t.Errorf("CKLByLevel[learning] = %d, want 1", result.CKLByLevel[string(knowledge.LevelLearning)])
	}

	// O item continua learning (já era): 3 evidências + confiança 0.95.
	items := readLawsItems(t, coscaDir)
	if len(items) != 1 || items[0].ID != "K-1" {
		t.Fatalf("unexpected laws after cycle: %+v", items)
	}
	if items[0].Level != knowledge.LevelLearning {
		t.Errorf("item level = %q, want %q", items[0].Level, knowledge.LevelLearning)
	}
	if len(items[0].Evidence) != 3 {
		t.Errorf("item evidence count = %d, want 3", len(items[0].Evidence))
	}
}

// TestRunORC_CKLPromotion_PromotesStaleItem verifies the auto-heal +
// persist path: an item persisted at the observation floor with 3
// evidences and confidence 0.95 is auto-healed to learning on Load
// (ValidateLevels detects the regression), and the corrected level
// is persisted back to laws.json by the always-save guarantee.
func TestRunORC_CKLPromotion_PromotesStaleItem(t *testing.T) {
	coscaDir := t.TempDir()
	// Nível congelado em observation apesar dos thresholds já dizerem
	// learning — simula uma regressão (ex: law→observation após restart).
	writeLawsFile(t, coscaDir, &knowledge.KnowledgeItem{
		ID:         "K-stale",
		Title:      "Lei defasada",
		Level:      knowledge.LevelObservation,
		Confidence: 0.95,
		Evidence: []knowledge.Evidence{
			{ID: "ev-1", Kind: "test", Source: "test/", Description: "e1", Timestamp: time.Now()},
			{ID: "ev-2", Kind: "test", Source: "test/", Description: "e2", Timestamp: time.Now()},
			{ID: "ev-3", Kind: "test", Source: "test/", Description: "e3", Timestamp: time.Now()},
		},
	})

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC: %v", err)
	}

	step := findStep(result.Steps, "ckl_promotion")
	if step == nil {
		t.Fatal("ckl_promotion step not found in pipeline")
	}
	if step.Status != "ok" {
		t.Errorf("ckl_promotion status = %q, want ok (%s)", step.Status, step.Detail)
	}
	// ValidateLevels already healed the item on Load, so Reevaluate
	// sees 0 promotions (the item is already at the correct level).
	if !strings.Contains(step.Detail, "0 promoted") {
		t.Errorf("detail should report 0 promoted (auto-healed on load), got: %s", step.Detail)
	}

	// O item foi auto-curado no Load e persistido pelo always-save.
	items := readLawsItems(t, coscaDir)
	if len(items) != 1 || items[0].ID != "K-stale" {
		t.Fatalf("unexpected laws after cycle: %+v", items)
	}
	if items[0].Level != knowledge.LevelLearning {
		t.Errorf("item level = %q, want %q (auto-healed on load + persisted)", items[0].Level, knowledge.LevelLearning)
	}
}

// TestRunORC_CKLPromotion_NoLawsFile verifies that a missing laws.json is
// skipped — never an error — and never recorded in ORCResult.Errors.
func TestRunORC_CKLPromotion_NoLawsFile(t *testing.T) {
	coscaDir := t.TempDir()

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC: %v", err)
	}

	step := findStep(result.Steps, "ckl_promotion")
	if step == nil {
		t.Fatal("ckl_promotion step not found in pipeline")
	}
	if step.Status != "skipped" {
		t.Errorf("ckl_promotion status = %q, want skipped (%s)", step.Status, step.Detail)
	}
	if !strings.Contains(step.Detail, "laws.json") {
		t.Errorf("detail should mention laws.json, got: %s", step.Detail)
	}
	for _, e := range result.Errors {
		if strings.Contains(e, "ckl_promotion") {
			t.Errorf("missing laws.json must not record errors, got: %s", e)
		}
	}
}

// TestRunORC_Report_IncludesCKL verifies that the ORC markdown report
// contains the "## CKL Status" section with total laws, per-level counts and
// average confidence.
func TestRunORC_Report_IncludesCKL(t *testing.T) {
	coscaDir := t.TempDir()
	writeLawsFile(t, coscaDir, &knowledge.KnowledgeItem{
		ID:         "K-1",
		Title:      "Lei de teste",
		Confidence: 0.95,
		Evidence: []knowledge.Evidence{
			{ID: "ev-1", Kind: "test", Source: "test/", Description: "e1", Timestamp: time.Now()},
			{ID: "ev-2", Kind: "test", Source: "test/", Description: "e2", Timestamp: time.Now()},
			{ID: "ev-3", Kind: "test", Source: "test/", Description: "e3", Timestamp: time.Now()},
		},
	})

	result, err := RunORC(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("RunORC: %v", err)
	}
	if result.ReportPath == "" {
		t.Fatal("expected a report path")
	}
	data, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	report := string(data)
	for _, want := range []string{"## CKL Status", "Total Laws", "learning", "Average Confidence"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
}
