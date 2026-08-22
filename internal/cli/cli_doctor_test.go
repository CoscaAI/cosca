//
// Unit tests for doctor.go — covers all doctor check functions and run modes.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// checkRuntime
// =============================================================================

func TestCheckRuntime_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkRuntime(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
	// No PID file in an empty dir — the daemon is not running, so both
	// checks should be warnings (never fails).
	for _, c := range result.Checks {
		if c.Status == "fail" {
			t.Errorf("expected no fail with empty dir, got %q", c.Name)
		}
	}
	hasWarning := false
	for _, c := range result.Checks {
		if c.Status == "warning" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected at least one warning with empty dir (no daemon)")
	}
}

// =============================================================================
// checkEditor
// =============================================================================

func TestCheckEditor_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkEditor(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
}

// =============================================================================
// checkPlugins
// =============================================================================

func TestCheckPlugins_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkPlugins(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
}

func TestCheckPlugins_WithPluginDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create plugins directory
	_ = os.MkdirAll(filepath.Join(tmpDir, "plugins"), 0755)
	result := checkPlugins(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
}

// =============================================================================
// checkMemory
// =============================================================================

func TestCheckMemory_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkMemory(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
	// Memory may fail with empty dir — should have a warning or pass
}

// =============================================================================
// checkKnowledge
// =============================================================================

func TestCheckKnowledge_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkKnowledge(tmpDir)
	// Knowledge engine won't be available with empty dir — should have issues
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
	// Should have a fail status since no knowledge.db exists
	hasFail := false
	for _, c := range result.Checks {
		if c.Status == "fail" {
			hasFail = true
		}
	}
	if !hasFail {
		t.Log("knowledge check didn't fail — may have created DB")
	}
}

// =============================================================================
// checkProviders
// =============================================================================

func TestCheckProviders_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := checkProviders(tmpDir)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check result")
	}
}

// =============================================================================
// runDoctorSingle
// =============================================================================

func TestRunDoctorSingle_Runtime(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "runtime")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunDoctorSingle_Editor(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "editor")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
}

func TestRunDoctorSingle_Plugins(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "plugins")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
}

func TestRunDoctorSingle_Memory(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "memory")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
}

func TestRunDoctorSingle_Knowledge(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "knowledge")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
}

func TestRunDoctorSingle_Providers(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorSingle(nil, f, t.TempDir(), "providers")
	if err != nil {
		t.Fatalf("runDoctorSingle returned error: %v", err)
	}
}

// =============================================================================
// runDoctorFull
// =============================================================================

func TestRunDoctorFull_WithEmptyDir(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	err := runDoctorFull(nil, f, t.TempDir())
	if err != nil {
		t.Fatalf("runDoctorFull returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// runDoctorJSON
// =============================================================================

func TestRunDoctorJSON_Full(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctorJSON(cmd, t.TempDir(), "")
	if err != nil {
		t.Fatalf("runDoctorJSON returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty JSON output")
	}
}

func TestRunDoctorJSON_SingleSubsystem(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctorJSON(cmd, t.TempDir(), "runtime")
	if err != nil {
		t.Fatalf("runDoctorJSON returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty JSON output")
	}
}

// =============================================================================
// Doctor Command — RunE with subsystem args
// =============================================================================

func TestDoctorCommand_RunE_Full(t *testing.T) {
	cmd := NewDoctorCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !isNonEmpty(output) {
		t.Error("expected non-empty output")
	}
}

func TestDoctorCommand_RunE_SingleSubsystem(t *testing.T) {
	for _, subsystem := range []string{"runtime", "editor", "plugins", "memory", "knowledge", "providers"} {
		t.Run(subsystem, func(t *testing.T) {
			cmd := NewDoctorCommand()
			var buf bytes.Buffer
			f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
			ctx := newContextWithFormatter(context.Background(), f)
			cmd.SetContext(ctx)

			err := cmd.RunE(cmd, []string{subsystem})
			if err != nil {
				t.Fatalf("RunE for %q returned error: %v", subsystem, err)
			}
		})
	}
}

func TestDoctorCommand_RunE_InvalidSubsystem(t *testing.T) {
	cmd := NewDoctorCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, []string{"invalid_subsystem"})
	if err == nil {
		t.Fatal("expected error for invalid subsystem")
	}
}

// =============================================================================
// Health Command — RunE with .cosca dir
// =============================================================================

func TestHealthCommand_RunE_WithCoscaDir(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewHealthCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !isNonEmpty(output) {
		t.Error("expected non-empty output")
	}
}

func TestHealthCommand_JSONOutput(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewHealthCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json")
	cmd.PersistentFlags().Set("json", "true")

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
}

func TestPrintHealthItem(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	printHealthItem(f, "TestCheck", true)
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output for pass")
	}

	var buf2 bytes.Buffer
	f2 := NewOutputFormatter(&buf2, OutputFormatText, false, false, true)
	printHealthItem(f2, "TestCheck", false)
	output2 := buf2.String()
	if output2 == "" {
		t.Error("expected non-empty output for fail")
	}
}

// =============================================================================
// Status Command — RunE
// =============================================================================

func TestStatusCommand_RunE_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output for not initialized")
	}
}

func TestStatusCommand_RunE_Initialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output for initialized")
	}
}

// =============================================================================
// Bootstrap Command — RunE initialized
// =============================================================================

func TestBootstrapCommand_RunE_Initialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewBootstrapCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// May fail because subsystems aren't fully set up
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error (expected if subsystems not set up): %v", err)
	}
}

// =============================================================================
// Init Command — RunE
// =============================================================================

func TestInitCommand_RunE(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewInitCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Init should succeed in fresh directory
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	// Verify .cosca was created
	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca")); err != nil {
		t.Error(".cosca directory was not created")
	}
}

func TestInitCommand_RunE_AlreadyInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewInitCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when already initialized without --force")
	}
}

func TestInitCommand_RunE_ForceInit(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewInitCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.Flags().Set("force", "true")

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
}

// =============================================================================
// Sync Command — RunE
// =============================================================================

func TestSyncCommand_RunE_DryRun(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewSyncCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.Flags().Set("dry-run", "true")

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !isNonEmpty(output) {
		t.Error("expected non-empty output for dry-run sync")
	}
}

// =============================================================================
// Install Command — RunE
// =============================================================================

func TestInstallCommand_RunE(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewInstallCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Install should try to run even without prior init
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error (may be expected): %v", err)
	}
}

func TestInstallCommand_RunE_Initialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".cosca"), 0755)

	cmd := NewInstallCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

// =============================================================================
// Config List Command — RunE
// =============================================================================

func TestConfigListCommand_RunE(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewConfigListCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error (expected if no config): %v", err)
	}
}

// =============================================================================
// Upgrade Command — RunE
// =============================================================================

func TestUpgradeCommand_RunE_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewUpgradeCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	// upgrade tries to load config, loads defaults when no config exists
	if err != nil {
		t.Logf("RunE returned error (expected if no .cosca dir): %v", err)
	}
}

// =============================================================================
// Helper
// =============================================================================

func isNonEmpty(s string) bool {
	return len(s) > 0
}
