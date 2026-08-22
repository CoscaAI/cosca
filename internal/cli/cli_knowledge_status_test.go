//
// Tests for `cosca knowledge status` (internal/cli/knowledge_status.go).
//
// Covers:
//   - Registration of `status` in the knowledge command (and `cosca
//     knowledge status` through the root command)
//   - Command properties (Use/Short/Long/Args) and arg constraints
//   - `status` with no args → table of all laws (ID | Título | Status |
//     Verificações), seeding the 5 real laws on first run (all SUPPORTED:
//     learning → DefaultStatus)
//   - `status <id>` → detail: status, description, last verified, counts,
//     evidence count
//   - `status <id>` unknown id → error
//   - JSON output for both list and detail
//
// Uses the formatter-injection pattern from cli_cv_test.go:
// `newContextWithFormatter` + `PersistentPreRunE = nil`.

package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// executeKnowledgeStatus runs `cosca knowledge status [args...]` with cwd set
// to root and returns the output. Same shape as executeCV.
func executeKnowledgeStatus(t *testing.T, root string, args ...string) (string, error) {
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
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, true) // noColor
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"knowledge", "status"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeStatusCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("status subcommand not registered in knowledge command")
	}
}

func TestKnowledgeStatusCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeStatusCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeStatusCommand returned nil")
	}
	if cmd.Use != "status [id]" {
		t.Errorf("expected Use='status [id]', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
	if cmd.Args == nil {
		t.Fatal("expected Args validator")
	}
	// Zero or one arg; never more.
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("status with no args should be allowed: %v", err)
	}
	if err := cmd.Args(cmd, []string{"K-01"}); err != nil {
		t.Errorf("status with one arg should be allowed: %v", err)
	}
	if err := cmd.Args(cmd, []string{"K-01", "K-02"}); err == nil {
		t.Error("status with two args should fail")
	}
}

// =============================================================================
// `status` with no args — table
// =============================================================================

func TestKnowledgeStatus_TableSeedsAndShowsAllLaws(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	out, err := executeKnowledgeStatus(t, root)
	if err != nil {
		t.Fatalf("knowledge status: %v", err)
	}

	// The seed created the 5 real laws on first run.
	for _, id := range []string{"K-01", "K-02", "K-03", "K-04", "K-05"} {
		if !strings.Contains(out, id) {
			t.Errorf("table missing seed law %s; output:\n%s", id, out)
		}
	}

	// Headers and epistemic status: all seed laws are learning → SUPPORTED.
	for _, want := range []string{"ID", "Título", "Status", "Verificações", "SUPPORTED"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q; output:\n%s", want, out)
		}
	}

	// Nenhuma contagem de verificação persistida no seed → coluna zerada.
	if !strings.Contains(out, "K-01") || !strings.Contains(out, "0  ") {
		t.Errorf("expected zero verification counts in table; output:\n%s", out)
	}
}

func TestKnowledgeStatus_TableFromExistingLaws(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1,"items":[
		{"id":"K-1","title":"Limites são a espinha","level":"law","confidence":0.99,
		 "evidence":[{"id":"ev-1","kind":"benchmark","source":"test/","description":"reduziu","timestamp":"2026-07-31T14:30:00Z"}]}
	]}`)

	out, err := executeKnowledgeStatus(t, root)
	if err != nil {
		t.Fatalf("knowledge status: %v", err)
	}
	if !strings.Contains(out, "K-1") {
		t.Errorf("table missing existing law K-1; output:\n%s", out)
	}
	if !strings.Contains(out, "KNOWN") {
		t.Errorf("law level 'law' should map to KNOWN; output:\n%s", out)
	}
}

func TestKnowledgeStatus_EmptyEngine(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1}`)

	out, err := executeKnowledgeStatus(t, root)
	if err != nil {
		t.Fatalf("knowledge status: %v", err)
	}
	if !strings.Contains(out, "Nenhuma lei registrada") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}
}

// =============================================================================
// `status <id>` — detail
// =============================================================================

func TestKnowledgeStatus_Detail(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1,"items":[
		{"id":"K-01","title":"Nunca executar como root automaticamente","level":"learning","confidence":0.95,
		 "evidence":[
		   {"id":"F001","kind":"incident","source":"F001","description":"fuga","timestamp":"2026-07-30T09:15:00Z"},
		   {"id":"red-team-A1","kind":"audit","source":"red-team-A1","description":"red team","timestamp":"2026-07-31T10:00:00Z"},
		   {"id":"TestJail_RootUnsafe","kind":"test","source":"TestJail_RootUnsafe","description":"teste","timestamp":"2026-07-31T14:30:00Z"},
		   {"id":"Onda-1-fix","kind":"test","source":"Onda-1-fix","description":"fix","timestamp":"2026-07-31T15:00:00Z"}
		 ],
		 "created_at":"2026-07-30T09:15:00Z",
		 "status":"UNCERTAIN","last_verified":"2026-07-15T12:00:00Z","verification_count":3,"contradiction_count":1
		}
	]}`)

	out, err := executeKnowledgeStatus(t, root, "K-01")
	if err != nil {
		t.Fatalf("knowledge status K-01: %v", err)
	}
	for _, want := range []string{
		"Lei K-01",
		"Status",
		"UNCERTAIN",
		"evidência insuficiente", // Description()
		"Última verificação",
		"2026-07-15", // LastVerified formatted
		"Verificações",
		"3",
		"Contradições",
		"1",
		"Evidências",
		"4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q; output:\n%s", want, out)
		}
	}
}

func TestKnowledgeStatus_DetailDefaultStatusAndNeverVerified(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1,"items":[
		{"id":"K-2","title":"Sem estado","level":"learning","confidence":0.8,
		 "evidence":[{"id":"ev-1","kind":"test","source":"test/","description":"d","timestamp":"2026-07-31T14:30:00Z"}]}
	]}`)

	out, err := executeKnowledgeStatus(t, root, "K-2")
	if err != nil {
		t.Fatalf("knowledge status K-2: %v", err)
	}
	if !strings.Contains(out, "SUPPORTED") {
		t.Errorf("missing status via DefaultStatus (learning → SUPPORTED); output:\n%s", out)
	}
	if !strings.Contains(out, "nunca") {
		t.Errorf("expected 'nunca' for unverified law; output:\n%s", out)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("expected zero verification counts; output:\n%s", out)
	}
}

func TestKnowledgeStatus_DetailNotFound(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1}`)

	if _, err := executeKnowledgeStatus(t, root, "K-999"); err == nil {
		t.Fatal("expected error for unknown law")
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestKnowledgeStatus_JSONList(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1,"items":[
		{"id":"K-1","title":"Limites são a espinha","level":"law","confidence":0.99,
		 "evidence":[{"id":"ev-1","kind":"benchmark","source":"test/","description":"reduziu","timestamp":"2026-07-31T14:30:00Z"}]}
	]}`)

	out, err := executeKnowledgeStatus(t, root, "--json")
	if err != nil {
		t.Fatalf("knowledge status --json: %v", err)
	}
	if !strings.Contains(out, `"id": "K-1"`) {
		t.Errorf("json list missing law id; output:\n%s", out)
	}
	if !strings.Contains(out, `"status": "KNOWN"`) {
		t.Errorf("json list missing status (law → KNOWN); output:\n%s", out)
	}
}

func TestKnowledgeStatus_JSONDetail(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/knowledge/laws.json", `{"version":1,"items":[
		{"id":"K-1","title":"Limites são a espinha","level":"learning","confidence":0.8,
		 "evidence":[{"id":"ev-1","kind":"benchmark","source":"test/","description":"reduziu","timestamp":"2026-07-31T14:30:00Z"}],
		 "status":"STALE","verification_count":2}
	]}`)

	out, err := executeKnowledgeStatus(t, root, "--json", "K-1")
	if err != nil {
		t.Fatalf("knowledge status --json K-1: %v", err)
	}
	for _, want := range []string{
		`"id": "K-1"`,
		`"status": "STALE"`,
		`"verification_count": 2`,
		`"status_description": "expirado — requer revalidação"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json detail missing %q; output:\n%s", want, out)
		}
	}
}

// compile-time guard: the command constructor must match the cobra contract.
var _ = NewKnowledgeStatusCommand()
