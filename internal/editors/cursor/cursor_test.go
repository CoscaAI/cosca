package cursor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestSetupCreatesKernelRule(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	rulePath := filepath.Join(proj, ".cursor", "rules", CoscaKernelRuleFileName)
	data, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("kernel rule not created: %v", err)
	}

	// Tests run in a temp dir (third-party project), so verify the
	// self-contained third-party content.
	content := string(data)
	if !strings.Contains(content, "Cosca Kernel") {
		t.Errorf("kernel rule must reference Cosca Kernel, got: %s", content)
	}
	if !strings.Contains(content, "consigliere") {
		t.Errorf("kernel rule must mention the consigliere persona, got: %s", content)
	}
	if !strings.Contains(content, "knowledge.db") {
		t.Errorf("kernel rule must reference knowledge.db, got: %s", content)
	}
	if !strings.Contains(content, "memory/") {
		t.Errorf("kernel rule must reference .cosca/memory/, got: %s", content)
	}

	// The classic .cursorrules must still be created.
	cursorRulesPath := filepath.Join(proj, ".cursorrules")
	if _, err := os.Stat(cursorRulesPath); err != nil {
		t.Errorf(".cursorrules not created: %v", err)
	}
}

func TestSetupKernelRuleIdempotent(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("second Setup: %v", err)
	}

	rulePath := filepath.Join(proj, ".cursor", "rules", CoscaKernelRuleFileName)
	data, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("kernel rule missing after second Setup: %v", err)
	}
	if count := strings.Count(string(data), "description: Cosca Kernel boot"); count != 1 {
		t.Errorf("kernel rule duplicated: expected 1 header, got %d", count)
	}
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v == "" {
		t.Error("Version should not be empty")
	}
}

func TestInfo(t *testing.T) {
	a := NewAdapter()
	info, err := a.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name == "" {
		t.Error("Info.Name should not be empty")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := NewAdapter()
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })
	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	_ = found
}
