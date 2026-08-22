//
// Unit tests for doctor_fix.go — SAFE vs REQUIRES_SUDO classification and
// `cosca doctor --fix` behavior.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Classification: SAFE vs REQUIRES_SUDO
// =============================================================================

func TestClassifyFix_SudoFixIsNotSafe(t *testing.T) {
	fix := classifyFix("sudo apt-get install rocm")
	if fix.Safe {
		t.Error("sudo fix must be classified REQUIRES_SUDO (never auto-executed)")
	}
}

func TestClassifyFix_DriverKernelFirewallAreNotSafe(t *testing.T) {
	for _, fix := range []string{
		"Install NVIDIA driver (requires sudo)",
		"Update kernel to fix boot issue",
		"Configure firewall to allow gRPC",
	} {
		if classifyFix(fix).Safe {
			t.Errorf("fix %q must be REQUIRES_SUDO", fix)
		}
	}
}

func TestClassifyFix_CreateDirectoryIsSafe(t *testing.T) {
	fix := classifyFix("Create directory /tmp/x/.cosca/memory")
	if !fix.Safe {
		t.Error("create-directory fix must be SAFE")
	}
}

// =============================================================================
// applySafeFix: creating a missing directory is executed
// =============================================================================

func TestApplySafeFix_CreatesMissingDirectory(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	target := filepath.Join(coscaDir, "memory")

	action, applied, err := applySafeFix(coscaDir, "Create directory "+target)
	if err != nil {
		t.Fatalf("applySafeFix error: %v", err)
	}
	if !applied {
		t.Fatal("expected the directory fix to be applied")
	}
	if action == "" {
		t.Error("expected a non-empty action description")
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Errorf("directory was not created: %v", err)
	}
}

func TestApplySafeFix_RejectsDirectoryOutsideCosca(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	outside := filepath.Join(t.TempDir(), "elsewhere")

	_, applied, err := applySafeFix(coscaDir, "Create directory "+outside)
	if err == nil {
		t.Error("expected an error for a directory outside .cosca")
	}
	if applied {
		t.Error("fix must not be applied outside .cosca")
	}
}

// =============================================================================
// applyDoctorFixes: classification drives execution
// =============================================================================

func TestApplyDoctorFixes_SudoFixListedNotExecuted(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)

	executed := applyDoctorFixes(f, coscaDir, []string{
		"sudo apt-get install rocm",
		"Install NVIDIA driver (requires sudo)",
	})
	if executed != 0 {
		t.Errorf("expected 0 fixes executed, got %d", executed)
	}
	out := buf.String()
	if !strings.Contains(out, "requer o Don (sudo)") {
		t.Errorf("expected sudo fixes listed for the Don, got:\n%s", out)
	}
	if strings.Contains(out, "corrigido:") {
		t.Errorf("no fix should be marked as corrected, got:\n%s", out)
	}
}

func TestApplyDoctorFixes_CreateDirectoryExecuted(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	target := filepath.Join(coscaDir, "cache")
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)

	executed := applyDoctorFixes(f, coscaDir, []string{"Create directory " + target})
	if executed != 1 {
		t.Errorf("expected 1 fix executed, got %d", executed)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("directory not created: %v", err)
	}
	if !strings.Contains(buf.String(), "✓ corrigido") {
		t.Errorf("expected ✓ corrigido output, got:\n%s", buf.String())
	}
}

// =============================================================================
// runDoctorFix: --fix creates a missing .cosca/memory dir (temp tree)
// =============================================================================

func TestRunDoctorFix_CreatesMissingMemoryDir(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)

	if err := runDoctorFix(f, coscaDir, ""); err != nil {
		t.Fatalf("runDoctorFix error: %v", err)
	}
	if info, err := os.Stat(filepath.Join(coscaDir, "memory")); err != nil || !info.IsDir() {
		t.Errorf("memory dir was not created by --fix: %v", err)
	}
	if !strings.Contains(buf.String(), "✓ corrigido") {
		t.Errorf("expected ✓ corrigido output, got:\n%s", buf.String())
	}
}

func TestRunDoctorFix_IdempotentAfterSuccess(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")

	var buf1 bytes.Buffer
	f1 := NewOutputFormatter(&buf1, OutputFormatText, false, false, true)
	if err := runDoctorFix(f1, coscaDir, ""); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "memory")); err != nil {
		t.Fatalf("memory dir missing after first run: %v", err)
	}

	var buf2 bytes.Buffer
	f2 := NewOutputFormatter(&buf2, OutputFormatText, false, false, true)
	if err := runDoctorFix(f2, coscaDir, ""); err != nil {
		t.Fatalf("second run error: %v", err)
	}
	if !strings.Contains(buf2.String(), "nada a corrigir") {
		t.Errorf("second run should report nothing to fix, got:\n%s", buf2.String())
	}
}

func TestRunDoctorFix_OnlySudoFixes_ExecutesNothing(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		t.Fatalf("mkdir .cosca: %v", err)
	}
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)

	executed := applyDoctorFixes(f, coscaDir, []string{
		"sudo apt-get install rocm-opencl-runtime",
		"Install NVIDIA driver (requires sudo)",
		"Update kernel to latest",
	})
	if executed != 0 {
		t.Errorf("expected nothing executed, got %d", executed)
	}
	if !strings.Contains(buf.String(), "requer o Don (sudo)") {
		t.Errorf("expected commands listed for the Don, got:\n%s", buf.String())
	}
	// Nothing inside .cosca should have been created.
	if _, err := os.Stat(filepath.Join(coscaDir, "memory")); !os.IsNotExist(err) {
		t.Error("nothing should be created when only sudo fixes exist")
	}
}

// =============================================================================
// Without --fix: behavior unchanged (no fixes executed)
// =============================================================================

func TestRunDoctorFull_WithoutFix_NoFixesExecuted(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)

	if err := runDoctorFull(nil, f, coscaDir); err != nil {
		t.Fatalf("runDoctorFull error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "✓ corrigido") {
		t.Errorf("without --fix no fix should be executed, got:\n%s", out)
	}
	if strings.Contains(out, "requer o Don (sudo)") {
		t.Errorf("without --fix no fix list should be printed, got:\n%s", out)
	}
}

func TestDoctorCommand_FixFlag_DefaultFalse(t *testing.T) {
	cmd := NewDoctorCommand()
	flag := cmd.PersistentFlags().Lookup("fix")
	if flag == nil {
		t.Fatal("expected --fix flag on doctor command")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected --fix default false, got %q", flag.DefValue)
	}
}

// =============================================================================
// Formatter injection pattern (command-level, --fix via flag)
// =============================================================================

func TestDoctorCommand_FixFlag_CreatesMissingMemoryDir(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewDoctorCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	if err := cmd.PersistentFlags().Set("fix", "true"); err != nil {
		t.Fatalf("set --fix: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca", "memory")); err != nil {
		t.Errorf("memory dir not created via doctor --fix: %v", err)
	}
	if !strings.Contains(buf.String(), "✓ corrigido") {
		t.Errorf("expected ✓ corrigido in formatter output, got:\n%s", buf.String())
	}
}

func TestDoctorCommand_WithoutFix_NoExecution(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewDoctorCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if strings.Contains(buf.String(), "✓ corrigido") {
		t.Errorf("without --fix nothing should be executed, got:\n%s", buf.String())
	}
}
