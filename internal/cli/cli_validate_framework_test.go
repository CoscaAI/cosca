//
// Tests for the framework validation CLI integration:
//   - `cosca validate crossrefs|conventions|orphans` subcommands
//   - `cosca doctor framework` subsystem
//
// Follows the ExecuteContext patterns from cli_kernel_test.go. Text-mode
// tests execute the command tree directly (no root PersistentPreRun, so the
// buffer formatter injected via context is preserved); JSON tests execute
// through the real root command and assert on the buffer.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/framework"
)

// writeTestFile creates a file (and its parent directories) with the given
// content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// runCommand executes the command tree rooted at cmd (a command with no
// PersistentPreRun) with a no-color buffer formatter injected via context,
// returning the captured output and the execution error.
func runCommand(cmd *cobra.Command, args []string) (string, error) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	ctx := newContextWithFormatter(context.Background(), f)
	err := cmd.ExecuteContext(ctx)
	return buf.String(), err
}

// =============================================================================
// Registration — validate subcommands
// =============================================================================

func TestValidateCommand_FrameworkSubcommandsRegistered(t *testing.T) {
	cmd := NewValidateCommand()
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range []string{"crossrefs", "conventions", "orphans"} {
		if !registered[name] {
			t.Errorf("validate command missing subcommand: %s", name)
		}
	}
}

// =============================================================================
// `cosca validate crossrefs`
// =============================================================================

func TestValidateCrossrefs_Subcommand(t *testing.T) {
	globalFlags = GlobalFlags{}

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "docs", "a.md"), "# A\n\n[valid](b.md)\n[broken](missing.md)\n")
	writeTestFile(t, filepath.Join(tmp, "docs", "b.md"), "# B\n")

	output, err := runCommand(NewValidateCommand(), []string{"crossrefs", "--root", tmp})
	if err == nil {
		t.Fatal("expected non-zero exit when broken cross-references exist")
	}

	// The issue must reference the file containing the broken link.
	if !strings.Contains(output, "docs/a.md") {
		t.Errorf("crossrefs output missing file docs/a.md; output:\n%s", output)
	}
	// The broken link target must be reported.
	if !strings.Contains(output, "missing.md") {
		t.Errorf("crossrefs output missing broken link target; output:\n%s", output)
	}
	// Table header and summary must be present.
	for _, want := range []string{"Severity", "Summary"} {
		if !strings.Contains(output, want) {
			t.Errorf("crossrefs output missing %q; output:\n%s", want, output)
		}
	}
	// The valid link target must NOT be flagged.
	if strings.Contains(output, `"b.md"`) {
		t.Errorf("crossrefs flagged the valid link; output:\n%s", output)
	}
}

func TestValidateCrossrefs_Subcommand_NoIssues(t *testing.T) {
	globalFlags = GlobalFlags{}

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "docs", "a.md"), "# A\n\n[valid](b.md)\n")
	writeTestFile(t, filepath.Join(tmp, "docs", "b.md"), "# B\n")

	output, err := runCommand(NewValidateCommand(), []string{"crossrefs", "--root", tmp})
	if err != nil {
		t.Fatalf("crossrefs returned error for valid framework: %v", err)
	}
	if !strings.Contains(output, "No issues found") {
		t.Errorf("expected 'No issues found'; output:\n%s", output)
	}
}

// =============================================================================
// `cosca validate conventions`
// =============================================================================

func TestValidateConventions_Subcommand(t *testing.T) {
	globalFlags = GlobalFlags{}

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "departments", "beta", "SKILL.md"), `> **Status**: weird

# BETA DEPARTMENT

## PURPOSE
Purpose paragraph.
`)

	output, err := runCommand(NewValidateCommand(), []string{"conventions", "--root", tmp})
	if err == nil {
		t.Fatal("expected non-zero exit when conventions are violated")
	}

	if !strings.Contains(output, "departments/beta/SKILL.md") {
		t.Errorf("conventions output missing department file; output:\n%s", output)
	}
	for _, want := range []string{"Version", "HISTORY"} {
		if !strings.Contains(output, want) {
			t.Errorf("conventions output missing %q issue; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `cosca validate orphans`
// =============================================================================

func TestValidateOrphans_Subcommand(t *testing.T) {
	globalFlags = GlobalFlags{}

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "guide.md"), "# guide\n\nSee [reference](reference.md)\n")
	writeTestFile(t, filepath.Join(tmp, "reference.md"), "# reference\n\nBack to [guide](guide.md)\n")
	writeTestFile(t, filepath.Join(tmp, "unlinked.md"), "# unlinked\n")

	output, err := runCommand(NewValidateCommand(), []string{"orphans", "--root", tmp})
	if err == nil {
		t.Fatal("expected non-zero exit when orphan files exist")
	}

	if !strings.Contains(output, "unlinked.md") {
		t.Errorf("orphans output missing unlinked.md; output:\n%s", output)
	}
	// Referenced files must not be reported as orphans.
	if strings.Contains(output, "guide.md") || strings.Contains(output, "reference.md") {
		t.Errorf("orphans flagged referenced files; output:\n%s", output)
	}
}

// =============================================================================
// `cosca doctor framework`
// =============================================================================

func TestDoctorFramework_Subcommand(t *testing.T) {
	globalFlags = GlobalFlags{}

	tmp := t.TempDir()
	root := filepath.Join(tmp, ".cosca", "fallback")
	writeTestFile(t, filepath.Join(root, "INDEX.md"), "# Index\n")
	writeTestFile(t, filepath.Join(root, "guide.md"), "# guide\n\nAll files: reference, flow, one, PROMPT, SKILL\n")
	writeTestFile(t, filepath.Join(root, "docs", "reference.md"), "# reference\n\nBack to guide\n")
	writeTestFile(t, filepath.Join(root, "workflows", "flow.md"), "# flow\n")
	writeTestFile(t, filepath.Join(root, "skills", "one.md"), "# one\n")
	writeTestFile(t, filepath.Join(root, "departments", "alpha", "SKILL.md"), "# alpha\n")
	writeTestFile(t, filepath.Join(root, "engines", "omega", "SKILL.md"), "# omega\n")
	writeTestFile(t, filepath.Join(root, "agents", "cosca-x", "PROMPT.md"), "# PROMPT\n")

	output, err := runCommand(NewDoctorCommand(), []string{"framework", "--root", root})
	if err != nil {
		t.Fatalf("doctor framework returned error: %v", err)
	}

	for _, want := range []string{
		"Framework Diagnostics",
		"Markdown Files: 8",
		"Departments: 1",
		"Skills: 1",
		"Engines: 1",
		"Workflows: 1",
		"Agents: 1",
		"Broken Links: 0",
		"Orphans: 0",
		"Framework health: healthy",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("doctor framework output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// JSON output via the real root command (ExecuteContext pattern)
// =============================================================================

func TestValidateCrossrefs_Subcommand_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}
	defer func() { globalFlags = GlobalFlags{} }()

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "docs", "a.md"), "# A\n\n[valid](b.md)\n[broken](missing.md)\n")
	writeTestFile(t, filepath.Join(tmp, "docs", "b.md"), "# B\n")

	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"validate", "crossrefs", "--root", tmp, "--json"})

	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected non-zero exit when broken cross-references exist")
	}

	var res struct {
		Root   string            `json:"root"`
		Issues []framework.Issue `json:"issues"`
		Errors int               `json:"errors"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if res.Errors != 1 {
		t.Errorf("errors = %d, want 1", res.Errors)
	}
	found := false
	for _, iss := range res.Issues {
		if iss.File == "docs/a.md" && strings.Contains(iss.Message, "missing.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("JSON issues missing broken link; issues: %+v", res.Issues)
	}
}

func TestDoctorFramework_Subcommand_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}
	defer func() { globalFlags = GlobalFlags{} }()

	tmp := t.TempDir()
	root := filepath.Join(tmp, ".cosca", "fallback")
	writeTestFile(t, filepath.Join(root, "INDEX.md"), "# Index\n")
	writeTestFile(t, filepath.Join(root, "guide.md"), "# guide\n\nAll files: reference, flow, one, PROMPT, SKILL\n")
	writeTestFile(t, filepath.Join(root, "docs", "reference.md"), "# reference\n\nBack to guide\n")
	writeTestFile(t, filepath.Join(root, "workflows", "flow.md"), "# flow\n")
	writeTestFile(t, filepath.Join(root, "skills", "one.md"), "# one\n")
	writeTestFile(t, filepath.Join(root, "departments", "alpha", "SKILL.md"), "# alpha\n")
	writeTestFile(t, filepath.Join(root, "engines", "omega", "SKILL.md"), "# omega\n")
	writeTestFile(t, filepath.Join(root, "agents", "cosca-x", "PROMPT.md"), "# PROMPT\n")

	rc := NewRootCommand()
	var buf bytes.Buffer
	rc.SetOut(&buf)
	rc.SetErr(&buf)
	rc.SetArgs([]string{"doctor", "framework", "--root", root, "--json"})

	if err := rc.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("doctor framework --json returned error: %v", err)
	}

	var report framework.HealthReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if report.TotalMarkdownFiles != 8 {
		t.Errorf("TotalMarkdownFiles = %d, want 8", report.TotalMarkdownFiles)
	}
	for name, got := range map[string]int{
		"Departments": report.Departments,
		"Skills":      report.Skills,
		"Engines":     report.Engines,
		"Workflows":   report.Workflows,
		"Agents":      report.Agents,
	} {
		if got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
	if report.BrokenLinks != 0 || report.OrphanCount != 0 {
		t.Errorf("expected 0 broken links and 0 orphans, got %d/%d", report.BrokenLinks, report.OrphanCount)
	}
}
