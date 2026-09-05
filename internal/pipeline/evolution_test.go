package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPostTaskHook(t *testing.T) {
	hook := NewPostTaskHook("/tmp/test-memory")
	if hook == nil {
		t.Fatal("NewPostTaskHook returned nil")
	}
	if hook.MemoryDir() != "/tmp/test-memory" {
		t.Errorf("MemoryDir = %q, want %q", hook.MemoryDir(), "/tmp/test-memory")
	}
}

func TestRecordTaskAppendsLearning(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	record := EvolutionRecord{
		TaskID:    "TASK-001",
		AgentName: "cosca-test",
		TaskType:  "security audit",
		Outcome:   "success",
		Technique: "XSS detection via CSP analysis",
		Level:     3,
		Learned:   []string{"CSP headers block inline scripts", "nonce-based CSP is more secure"},
		Pattern:   "csp-header-analysis",
	}

	err := hook.RecordTask(context.Background(), record)
	if err != nil {
		t.Fatalf("RecordTask: %v", err)
	}

	learningsPath := filepath.Join(dir, "cosca-test", "learnings.md")
	data, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read learnings.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "XSS detection via CSP analysis") {
		t.Error("learnings.md should contain the technique name")
	}
	if !strings.Contains(content, "cosca-test") {
		t.Error("learnings.md should contain the agent name")
	}
	if !strings.Contains(content, "security audit") {
		t.Error("learnings.md should contain the task type")
	}
	if !strings.Contains(content, "#success") {
		t.Error("learnings.md should contain the #success tag")
	}
	if !strings.Contains(content, "#level-3") {
		t.Error("learnings.md should contain #level-3 tag")
	}
	if !strings.Contains(content, "CSP headers block inline scripts") {
		t.Error("learnings.md should contain the learned items")
	}

	patternsPath := filepath.Join(dir, "cosca-test", "patterns.md")
	patData, err := os.ReadFile(patternsPath)
	if err != nil {
		t.Fatalf("read patterns.md: %v", err)
	}
	if !strings.Contains(string(patData), "csp-header-analysis") {
		t.Error("patterns.md should contain the extracted pattern")
	}
}

func TestRecordTaskWithoutPattern(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	record := EvolutionRecord{
		TaskID:    "TASK-002",
		AgentName: "cosca-test",
		TaskType:  "refactoring",
		Outcome:   "partial",
		Technique: "Extract method refactoring",
		Level:     2,
		Learned:   []string{"Smaller methods are easier to test"},
		Pattern:   "",
	}

	err := hook.RecordTask(context.Background(), record)
	if err != nil {
		t.Fatalf("RecordTask: %v", err)
	}

	learningsPath := filepath.Join(dir, "cosca-test", "learnings.md")
	data, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read learnings.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "#partial") {
		t.Error("learnings.md should contain #partial tag")
	}

	// Patterns file should NOT be created when Pattern is empty.
	patternsPath := filepath.Join(dir, "cosca-test", "patterns.md")
	if _, err := os.Stat(patternsPath); err == nil {
		t.Error("patterns.md should not exist when no pattern was extracted")
	}
}

func TestRecordTaskContextCancellation(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record := EvolutionRecord{
		TaskID:    "TASK-003",
		AgentName: "cosca-test",
		Outcome:   "success",
		Technique: "Test",
	}

	err := hook.RecordTask(ctx, record)
	if err == nil {
		t.Error("RecordTask should return error on cancelled context")
	}
}

func TestUpdateCapabilityCreatesProfileUpdate(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	err := hook.UpdateCapability(context.Background(), "cosca-test", "success")
	if err != nil {
		t.Fatalf("UpdateCapability: %v", err)
	}

	profilePath := filepath.Join(dir, "cosca-test", "capability-profile.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read capability-profile.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Post-task capability update") {
		t.Error("capability-profile.md should contain capability update header")
	}
	if !strings.Contains(content, "stage 8") {
		t.Error("capability-profile.md should reference stage 8")
	}
	if !strings.Contains(content, "+0.05") {
		t.Error("success outcome should show +0.05 confidence delta")
	}
}

func TestUpdateCapabilityFailureOutcome(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	err := hook.UpdateCapability(context.Background(), "cosca-test", "failed")
	if err != nil {
		t.Fatalf("UpdateCapability: %v", err)
	}

	profilePath := filepath.Join(dir, "cosca-test", "capability-profile.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read capability-profile.md: %v", err)
	}

	if !strings.Contains(string(data), "-0.10") {
		t.Error("failure outcome should show -0.10 confidence delta")
	}
}

func TestUpdateCapabilityContextCancellation(t *testing.T) {
	dir := t.TempDir()
	hook := NewPostTaskHook(dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := hook.UpdateCapability(ctx, "cosca-test", "success")
	if err == nil {
		t.Error("UpdateCapability should return error on cancelled context")
	}
}

func TestOutcomeConfidence(t *testing.T) {
	hook := NewPostTaskHook("/tmp")

	tests := []struct {
		outcome string
		want    float64
	}{
		{"success", 0.85},
		{"partial", 0.65},
		{"failed", 0.40},
		{"unknown", 0.70},
	}

	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			got := hook.outcomeConfidence(tt.outcome)
			if got != tt.want {
				t.Errorf("outcomeConfidence(%q) = %.2f, want %.2f", tt.outcome, got, tt.want)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"XSS detection", "XSS detection"},
		{"multi-line\nname", "multi-line name"},
		{"pipe|test", "pipe/test"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
