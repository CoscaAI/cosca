package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func setupConfig(t *testing.T) types.EditorConfig {
	t.Helper()
	cfg := types.DefaultEditorConfig(t.TempDir())
	cfg.BackupExisting = true
	return cfg
}

func readCLAUDE(t *testing.T, projectDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	return string(data)
}

func TestSetupCreatesCLAUDEWithBoot(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	content := readCLAUDE(t, cfg.ProjectDir)

	// Boot block must be present with all required markers.
	// Tests run in a temp dir (third-party project), so verify the
	// self-contained third-party content.
	for _, marker := range []string{
		"## Cosca Kernel — Boot Automático",
		"knowledge.db",
		"memory/",
		"consigliere",
		"cosca kernel identity",
		"cosca run <prompt>",
	} {
		if !strings.Contains(content, marker) {
			t.Errorf("CLAUDE.md missing marker %q", marker)
		}
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #1: %v", err)
	}
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #2: %v", err)
	}

	content := readCLAUDE(t, cfg.ProjectDir)
	count := strings.Count(content, "## Cosca Kernel — Boot Automático")
	if count != 1 {
		t.Errorf("boot heading must appear exactly once, got %d", count)
	}
	if strings.Count(content, "## Cosca Integration") != 1 {
		t.Errorf("Cosca Integration heading must appear exactly once, got %d",
			strings.Count(content, "## Cosca Integration"))
	}
}

func TestSetupBackupExistingCLAUDE(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	claudePath := filepath.Join(cfg.ProjectDir, "CLAUDE.md")
	original := "# Existing project notes\n\nSome user content.\n"
	if err := os.WriteFile(claudePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	backup, err := os.ReadFile(claudePath + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup mismatch:\n got: %q\nwant: %q", backup, original)
	}

	// Original user content must be preserved in the new file too.
	content := readCLAUDE(t, cfg.ProjectDir)
	if !strings.Contains(content, "Some user content.") {
		t.Error("user content lost during setup")
	}
}

func TestSetupUpgradesLegacySection(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	claudePath := filepath.Join(cfg.ProjectDir, "CLAUDE.md")
	legacy := "# Project\n\n## Cosca Integration\n\nLegacy section without boot.\n\n## Other Section\n\nKeep me.\n"
	if err := os.WriteFile(claudePath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	content := readCLAUDE(t, cfg.ProjectDir)

	// New boot block present, legacy section replaced (not duplicated).
	if !strings.Contains(content, CoscaBootMarker) {
		t.Error("boot block missing after upgrade")
	}
	if strings.Count(content, "## Cosca Integration") != 1 {
		t.Errorf("legacy Cosca Integration section must be replaced, got %d occurrences",
			strings.Count(content, "## Cosca Integration"))
	}
	if !strings.Contains(content, "Keep me.") {
		t.Error("unrelated section was removed")
	}

	// Second run stays idempotent after upgrade.
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #2: %v", err)
	}
	content = readCLAUDE(t, cfg.ProjectDir)
	if strings.Count(content, CoscaBootMarker) != 1 {
		t.Errorf("boot heading duplicated after 2nd setup, got %d",
			strings.Count(content, CoscaBootMarker))
	}
}

func TestCoscaIntegrationSectionContainsBoot(t *testing.T) {
	t.Parallel()

	// Self variant: references framework files on disk.
	if !strings.Contains(coscaIntegrationSectionSelf, "KERNEL.md") {
		t.Error("coscaIntegrationSectionSelf must reference KERNEL.md")
	}
	if !strings.Contains(coscaIntegrationSectionSelf, "cognitive-state.md") {
		t.Error("coscaIntegrationSectionSelf must reference cognitive-state.md")
	}
	if !strings.Contains(coscaIntegrationSectionSelf, "consigliere") {
		t.Error("coscaIntegrationSectionSelf must describe the consigliere identity")
	}

	// Third-party variant: self-contained, references runtime data only.
	if !strings.Contains(coscaIntegrationSectionThirdParty, "knowledge.db") {
		t.Error("coscaIntegrationSectionThirdParty must reference knowledge.db")
	}
	if !strings.Contains(coscaIntegrationSectionThirdParty, "memory/") {
		t.Error("coscaIntegrationSectionThirdParty must reference .cosca/memory/")
	}
	if !strings.Contains(coscaIntegrationSectionThirdParty, "consigliere") {
		t.Error("coscaIntegrationSectionThirdParty must describe the consigliere identity")
	}
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "1.0" {
		t.Errorf("Version = %q, want %q", v, "1.0")
	}
}

func TestInfo(t *testing.T) {
	a := NewAdapter()
	info, err := a.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "claude" {
		t.Errorf("Name = %q", info.Name)
	}
}

func TestDetect_NotFound(t *testing.T) {
	// In a temp dir without CLAUDE.md, should return false
	a := NewAdapter()
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	os.Unsetenv("CLAUDE_CODE")
	os.Unsetenv("ANTHROPIC_API_KEY")

	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if found {
		t.Error("expected Detect=false in empty dir")
	}
}

func TestDetect_ByEnv(t *testing.T) {
	a := NewAdapter()
	os.Setenv("CLAUDE_CODE", "1")
	t.Cleanup(func() { os.Unsetenv("CLAUDE_CODE") })

	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !found {
		t.Error("expected Detect=true with CLAUDE_CODE env")
	}
}
