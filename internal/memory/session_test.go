package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultSessionSummary(t *testing.T) {
	t.Parallel()
	s := DefaultSessionSummary()

	today := time.Now().Format("2006-01-02")
	if s.Date != today {
		t.Errorf("Date = %q, want %q", s.Date, today)
	}
	if s.ShutdownReason != "signal" {
		t.Errorf("ShutdownReason = %q, want %q", s.ShutdownReason, "signal")
	}
}

func TestRecordSession(t *testing.T) {
	t.Parallel()
	coscaDir := t.TempDir()

	summary := SessionSummary{
		Date:           "2026-07-27",
		Commits:        5,
		FilesChanged:   12,
		Deliverables:   []string{"feature-a", "bugfix-b"},
		ScoreBefore:    70,
		ScoreAfter:     85,
		UptimeSeconds:  3600,
		ShutdownReason: "signal",
	}

	err := RecordSession(coscaDir, summary)
	if err != nil {
		t.Fatalf("RecordSession error: %v", err)
	}

	// Verify file exists
	sessionDir := filepath.Join(coscaDir, "memory", "session")
	expectedFile := filepath.Join(sessionDir, "2026-07-27-session-auto.md")

	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	content := string(data)

	// Check YAML frontmatter
	if !strings.HasPrefix(content, "---") {
		t.Fatal("file should start with YAML frontmatter")
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		t.Fatal("invalid frontmatter format")
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		t.Fatalf("parsing frontmatter: %v", err)
	}

	if fm["type"] != string(MemoryTypeSession) {
		t.Errorf("type = %v, want %q", fm["type"], MemoryTypeSession)
	}
	if fm["layer"] != string(LayerSession) {
		t.Errorf("layer = %v, want %q", fm["layer"], LayerSession)
	}
	if fm["commits"] != 5 {
		t.Errorf("commits = %v, want 5", fm["commits"])
	}
	if fm["files_changed"] != 12 {
		t.Errorf("files_changed = %v, want 12", fm["files_changed"])
	}
	if fm["score_before"] != 70 {
		t.Errorf("score_before = %v, want 70", fm["score_before"])
	}
	if fm["score_after"] != 85 {
		t.Errorf("score_after = %v, want 85", fm["score_after"])
	}
	if v, ok := fm["uptime_seconds"].(int); !ok || v != 3600 {
		t.Errorf("uptime_seconds = %v (%T), want 3600", fm["uptime_seconds"], fm["uptime_seconds"])
	}
	if fm["shutdown_reason"] != "signal" {
		t.Errorf("shutdown_reason = %v, want signal", fm["shutdown_reason"])
	}

	// Check body content
	body := parts[1]
	if !strings.Contains(body, "Session Auto-Record") {
		t.Error("body should contain title")
	}
	if !strings.Contains(body, "Commits: **5**") {
		t.Error("body should contain commits count")
	}
	if !strings.Contains(body, "Files changed: **12**") {
		t.Error("body should contain files changed")
	}
	if !strings.Contains(body, "## Deliverables") {
		t.Error("body should contain deliverables section")
	}
	if !strings.Contains(body, "feature-a") {
		t.Error("body should contain first deliverable")
	}
	if !strings.Contains(body, "bugfix-b") {
		t.Error("body should contain second deliverable")
	}
	if !strings.Contains(body, "+15") {
		t.Error("body should contain score delta (+15)")
	}
	if !strings.Contains(body, "Auto-generated") {
		t.Error("body should contain footer")
	}
}

func TestRecordSession_EmptyDir(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date: "2026-07-27",
	}
	err := RecordSession("", summary)
	if err != nil {
		t.Errorf("RecordSession with empty dir should return nil, got %v", err)
	}
}

func TestRecordSession_EmptyDate(t *testing.T) {
	t.Parallel()
	coscaDir := t.TempDir()

	summary := SessionSummary{
		Date: "", // Empty — should fall back to today
	}

	err := RecordSession(coscaDir, summary)
	if err != nil {
		t.Fatalf("RecordSession error: %v", err)
	}

	// File should use today's date
	today := time.Now().Format("2006-01-02")
	expectedFile := filepath.Join(coscaDir, "memory", "session", today+"-session-auto.md")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist (date fallback failed)", expectedFile)
	}
}

func TestBuildSessionBody_Full(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date:           "2026-07-27",
		Commits:        10,
		FilesChanged:   25,
		ScoreBefore:    60,
		ScoreAfter:     90,
		UptimeSeconds:  7200,
		ShutdownReason: "interrupt",
		Deliverables:   []string{"module-a", "module-b", "module-c"},
	}

	body := buildSessionBody(summary)

	if !strings.Contains(body, "# Session Auto-Record") {
		t.Error("missing title")
	}
	if !strings.Contains(body, "**Date**: 2026-07-27") {
		t.Error("missing date")
	}
	if !strings.Contains(body, "**Uptime**: 120 minutes") {
		t.Error("missing uptime in minutes")
	}
	if !strings.Contains(body, "**Shutdown**: interrupt") {
		t.Error("missing shutdown reason")
	}
	if !strings.Contains(body, "Commits: **10**") {
		t.Error("missing commits")
	}
	if !strings.Contains(body, "Files changed: **25**") {
		t.Error("missing files changed")
	}
	if !strings.Contains(body, "60 → 90 (+30)") {
		t.Error("missing score delta +30")
	}
	if !strings.Contains(body, "module-a") {
		t.Error("missing deliverable module-a")
	}
	if !strings.Contains(body, "module-c") {
		t.Error("missing deliverable module-c")
	}
	if !strings.Contains(body, "Auto-generated by cosca serve") {
		t.Error("missing footer")
	}
}

func TestBuildSessionBody_NegativeDelta(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date:           "2026-07-27",
		ScoreBefore:    80,
		ScoreAfter:     75,
		ShutdownReason: "error",
	}

	body := buildSessionBody(summary)

	if !strings.Contains(body, "80 → 75 (-5)") {
		t.Errorf("expected negative delta '-5', got body: %s", body)
	}
}

func TestBuildSessionBody_ZeroDelta(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date:        "2026-07-27",
		ScoreBefore: 80,
		ScoreAfter:  80,
	}

	body := buildSessionBody(summary)

	// When both are > 0, the line is shown but delta is 0 → "(+0)"
	if !strings.Contains(body, "80 → 80") {
		t.Error("should show scores even with zero delta")
	}
}

func TestBuildSessionBody_Minimal(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date:           "2026-07-27",
		ShutdownReason: "signal",
	}

	body := buildSessionBody(summary)

	if !strings.Contains(body, "# Session Auto-Record") {
		t.Error("missing title")
	}
	if !strings.Contains(body, "## Summary") {
		t.Error("missing summary section")
	}
	// Should NOT contain commits/files since they are 0
	if strings.Contains(body, "Commits: **") {
		t.Error("should not contain commits when 0")
	}
	if strings.Contains(body, "Files changed: **") {
		t.Error("should not contain files changed when 0")
	}
	if strings.Contains(body, "## Deliverables") {
		t.Error("should not contain deliverables when empty")
	}
	if !strings.Contains(body, "Auto-generated") {
		t.Error("missing footer")
	}
}

func TestBuildSessionBody_NoUptime(t *testing.T) {
	t.Parallel()
	summary := SessionSummary{
		Date:           "2026-07-27",
		UptimeSeconds:  0,
		ShutdownReason: "signal",
	}

	body := buildSessionBody(summary)

	if strings.Contains(body, "**Uptime**") {
		t.Error("should not show uptime when 0")
	}
}
