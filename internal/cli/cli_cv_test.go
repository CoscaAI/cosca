//
// Tests for the `cosca cv` command tree (internal/cli/cv.go).
//
// Covers:
//   - Registration of `cv` (and its 4 subcommands) in the root command
//   - Command properties (Use/Short/Long) and arg constraints
//   - `cv snapshot` output (table) + JSON mode against a fake .cosca tree
//   - `cv list` output
//   - `cv verify` (clean + divergent) output
//   - `cv rollback` (guide printed, nothing overwritten, exit success)
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory. Each test that changes the cwd does so in a
// deferred-restore way and is serialized by the absence of t.Parallel().

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Registration — `cv` in the root command
// =============================================================================

func TestCVCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "cv" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cv subcommand not registered in root command")
	}
}

func TestCVCommand_Properties(t *testing.T) {
	cmd := NewCVCommand()
	if cmd == nil {
		t.Fatal("NewCVCommand returned nil")
	}
	if cmd.Use != "cv" {
		t.Errorf("expected Use='cv', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"snapshot", "list", "verify", "rollback"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing cv subcommand: %s", name)
		}
	}
}

func TestCVCommand_VerifyAndRollback_AcceptOptionalVersion(t *testing.T) {
	verify := NewCVVerifyCommand()
	if verify.Args == nil {
		t.Error("verify should define Args")
	}
	if err := verify.Args(verify, nil); err != nil {
		t.Errorf("verify with no args should be allowed: %v", err)
	}
	if err := verify.Args(verify, []string{"CV-0001", "CV-0002"}); err == nil {
		t.Error("verify with two args should fail")
	}

	rollback := NewCVRollbackCommand()
	if err := rollback.Args(rollback, []string{"CV-0001"}); err != nil {
		t.Errorf("rollback with one arg should be allowed: %v", err)
	}
	if err := rollback.Args(rollback, []string{"CV-0001", "CV-0002"}); err == nil {
		t.Error("rollback with two args should fail")
	}
}

// =============================================================================
// executeCV runs a `cv` subcommand with cwd set to root and returns output.
// =============================================================================

func executeCV(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()

	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"cv"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

// fakeCVTree builds a fake .cosca tree for CLI tests.
func fakeCVTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/framework/KERNEL.md", "# Kernel\n")
	writeCLIFile(t, root, ".cosca/framework/CONSTITUTION.md", "# Constitution\n")
	writeCLIFile(t, root, ".cosca/framework/Cognitive_State_Specification.md", "# Cognitive State\n")
	writeCLIFile(t, root, ".cosca/framework/shared/PROJECT_CONTEXT.md", "# Context\n")
	writeCLIFile(t, root, ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md", "# Evolution\n")
	writeCLIFile(t, root, ".cosca/framework/knowledge/INDEX.md", "# Index\n")
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1}`)
	writeCLIFile(t, root, ".cosca/memory/project/note.md", "# Note\n")
	writeCLIFile(t, root, ".cosca/framework/agents/cosca-kernel/PROMPT.md", "# Prompt\n")
	writeCLIFile(t, root, ".cosca/framework/skills/ai/SKILL.md", "# Skill\n")
	writeCLIFile(t, root, ".cosca/config.yaml", "cache:\n  enabled: true\n")
	return root
}

// writeCLIFile writes a file creating parents.
func writeCLIFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// =============================================================================
// `cv snapshot`
// =============================================================================

func TestCVSnapshot_TextOutput(t *testing.T) {
	root := fakeCVTree(t)
	out, err := executeCV(t, root, "snapshot")
	if err != nil {
		t.Fatalf("cv snapshot: %v", err)
	}
	if !strings.Contains(out, "CV-0001") {
		t.Errorf("output missing CV version: %q", out)
	}
	for _, comp := range []string{"kernel-docs", "knowledge", "laws", "memory", "agents", "skills", "config"} {
		if !strings.Contains(out, comp) {
			t.Errorf("output missing component %q", comp)
		}
	}
	if !strings.Contains(out, "Manifest") {
		t.Errorf("output missing manifest path: %q", out)
	}
	// The manifest must exist on disk.
	if _, err := os.Stat(filepath.Join(root, ".cosca", "cv", "CV-0001.json")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
}

func TestCVSnapshot_JSONOutput(t *testing.T) {
	root := fakeCVTree(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"cv", "snapshot", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cv snapshot --json: %v", err)
	}
	if !strings.Contains(buf.String(), `"version": "CV-0001"`) {
		t.Errorf("expected JSON manifest output, got: %q", buf.String())
	}
}

// =============================================================================
// `cv list`
// =============================================================================

func TestCVList_EmptyAndPopulated(t *testing.T) {
	root := fakeCVTree(t)

	// Empty cv dir.
	out, err := executeCV(t, root, "list")
	if err != nil {
		t.Fatalf("cv list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhum CV") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}

	// Populate.
	if _, err := executeCV(t, root, "snapshot"); err != nil {
		t.Fatal(err)
	}
	out, err = executeCV(t, root, "list")
	if err != nil {
		t.Fatalf("cv list: %v", err)
	}
	if !strings.Contains(out, "CV-0001") {
		t.Errorf("list missing CV-0001: %q", out)
	}
	if !strings.Contains(out, "Manifest") {
		t.Errorf("list missing Manifest header: %q", out)
	}
}

// =============================================================================
// `cv verify`
// =============================================================================

func TestCVVerify_CleanAndDivergent(t *testing.T) {
	root := fakeCVTree(t)
	if _, err := executeCV(t, root, "snapshot"); err != nil {
		t.Fatal(err)
	}

	// Clean verify.
	out, err := executeCV(t, root, "verify")
	if err != nil {
		t.Fatalf("cv verify (clean): %v", err)
	}
	if !strings.Contains(out, "confere") {
		t.Errorf("expected clean verification message, got: %q", out)
	}

	// Divergent verify after modification.
	writeCLIFile(t, root, ".cosca/memory/project/note.md", "# Note — ALTERADO\n")
	out, err = executeCV(t, root, "verify", "CV-0001")
	if err != nil {
		t.Fatalf("cv verify (divergent): %v", err)
	}
	if !strings.Contains(out, "NÃO confere") {
		t.Errorf("expected divergent message, got: %q", out)
	}
	if !strings.Contains(out, "memory") || !strings.Contains(out, "note.md") {
		t.Errorf("expected changed memory component/file, got: %q", out)
	}
}

// =============================================================================
// `cv rollback` — diagnosis + guide, NEVER auto-overwrites
// =============================================================================

func TestCVRollback_PrintsGuideAndDoesNotOverwrite(t *testing.T) {
	root := fakeCVTree(t)
	if _, err := executeCV(t, root, "snapshot"); err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, root, ".cosca/framework/shared/PROJECT_CONTEXT.md", "# Context — QUEBRADO\n")
	target := filepath.Join(root, ".cosca", "fallback", "shared", "PROJECT_CONTEXT.md")
	before, _ := os.ReadFile(target)

	out, err := executeCV(t, root, "rollback", "CV-0001")
	if err != nil {
		t.Fatalf("cv rollback should exit success (diagnóstico + guia): %v", err)
	}

	// The broken file must NOT have been touched.
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatal("cv rollback must NEVER overwrite files")
	}

	// The guide must mention git restore and the diagnosis.
	if !strings.Contains(out, "ROLLBACK") || !strings.Contains(out, "diagnóstico") {
		t.Errorf("expected diagnosis header, got: %q", out)
	}
	if !strings.Contains(out, "git revert") && !strings.Contains(out, "git checkout") {
		t.Errorf("expected git restore instructions, got: %q", out)
	}
	if !strings.Contains(out, "Marcador do estado atual") {
		t.Errorf("expected marker snapshot mention, got: %q", out)
	}

	// A marker snapshot of the current state must exist on disk.
	marker, err := os.Stat(filepath.Join(root, ".cosca", "cv", "CV-0002.json"))
	if err != nil {
		t.Errorf("expected marker snapshot CV-0002: %v", err)
	}
	if marker.Size() == 0 {
		t.Error("marker snapshot must not be empty")
	}
}

// =============================================================================
// `cv` with no subcommand → help
// =============================================================================

func TestCVCommand_NoSubcommand_ShowsHelp(t *testing.T) {
	root := fakeCVTree(t)
	out, err := executeCV(t, root)
	if err != nil {
		t.Fatalf("cv with no subcommand should show help without error: %v", err)
	}
	if !strings.Contains(out, "snapshot") || !strings.Contains(out, "rollback") {
		t.Errorf("expected help listing subcommands, got: %q", out)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewCVCommand()
	_ *cobra.Command = NewCVSnapshotCommand()
	_ *cobra.Command = NewCVListCommand()
	_ *cobra.Command = NewCVVerifyCommand()
	_ *cobra.Command = NewCVRollbackCommand()
)
