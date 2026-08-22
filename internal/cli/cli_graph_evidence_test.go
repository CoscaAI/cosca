//
// Tests for `cosca graph evidence` and `cosca graph link` — o grafo de
// evidências (relações semânticas de evidência no knowledge graph).
//
// Covers:
//   - Registration of `evidence` and `link` in the `graph` command tree
//   - `graph evidence <entity>` empty + populated (mapa de evidências)
//   - `graph evidence <entity> --json`
//   - `graph link --from --to --type [--weight]` (valid + invalid type error)
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.
//

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
)

// =============================================================================
// Registration — `evidence` and `link` in the `graph` command tree
// =============================================================================

func TestGraphEvidenceCommand_RegisteredInGraph(t *testing.T) {
	cmd := NewGraphCommand()
	subs := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subs[sub.Name()] = true
	}
	for _, want := range []string{"evidence", "link"} {
		if !subs[want] {
			t.Errorf("missing graph subcommand: %s", want)
		}
	}
}

func TestGraphEvidenceCommand_Properties(t *testing.T) {
	cmd := NewGraphEvidenceCommand()
	if cmd == nil {
		t.Fatal("NewGraphEvidenceCommand returned nil")
	}
	if !strings.HasPrefix(cmd.Use, "evidence") {
		t.Errorf("expected Use to start with 'evidence', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short and Long")
	}
	if err := cmd.Args(cmd, nil); err == nil {
		t.Error("evidence with no args should fail (ExactArgs(1))")
	}
	if err := cmd.Args(cmd, []string{"Bubblewrap"}); err != nil {
		t.Errorf("evidence with one arg should be allowed: %v", err)
	}
}

func TestGraphLinkCommand_Properties(t *testing.T) {
	cmd := NewGraphLinkCommand()
	if cmd == nil {
		t.Fatal("NewGraphLinkCommand returned nil")
	}
	if cmd.Use != "link" {
		t.Errorf("expected Use='link', got %q", cmd.Use)
	}
	for _, f := range []string{"from", "to", "type", "weight"} {
		if flag := cmd.Flags().Lookup(f); flag == nil {
			t.Errorf("missing --%s flag", f)
		}
	}
}

// =============================================================================
// executeGraph runs a `graph` subcommand with cwd set to root.
// =============================================================================

func executeGraph(t *testing.T, root string, args ...string) (string, error) {
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
	cmd.SetArgs(append([]string{"graph"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

func fakeGraphTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/config.yaml", "cache:\n  enabled: true\n")
	return root
}

// =============================================================================
// `graph evidence` — empty
// =============================================================================

func TestGraphEvidence_Empty(t *testing.T) {
	root := fakeGraphTree(t)
	out, err := executeGraph(t, root, "evidence", "Bubblewrap")
	if err != nil {
		t.Fatalf("graph evidence (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhuma evidência registrada") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}
}

func TestGraphEvidence_EmptyJSON(t *testing.T) {
	root := fakeGraphTree(t)
	out, err := executeGraph(t, root, "evidence", "Bubblewrap", "--json")
	if err != nil {
		t.Fatalf("graph evidence --json (empty): %v", err)
	}
	var res graphEvidenceResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if res.Entity != "Bubblewrap" || len(res.Edges) != 0 {
		t.Errorf("unexpected empty JSON result: %+v", res)
	}
}

// =============================================================================
// `graph link` — valid and invalid type
// =============================================================================

func TestGraphLink_Valid(t *testing.T) {
	root := fakeGraphTree(t)
	out, err := executeGraph(t, root,
		"link", "--from", "Bubblewrap", "--to", "namespaces", "--type", "implements", "--weight", "0.8")
	if err != nil {
		t.Fatalf("graph link (valid): %v", err)
	}
	if !strings.Contains(out, "registrada no grafo de evidências") {
		t.Errorf("expected registration confirmation, got: %q", out)
	}
	if !strings.Contains(out, "implements") || !strings.Contains(out, "namespaces") {
		t.Errorf("output sem a relação registrada: %q", out)
	}
}

func TestGraphLink_InvalidType(t *testing.T) {
	root := fakeGraphTree(t)
	_, err := executeGraph(t, root,
		"link", "--from", "Bubblewrap", "--to", "Linux", "--type", "nao_existe")
	if err == nil {
		t.Fatal("graph link with invalid type should fail")
	}
	if !strings.Contains(err.Error(), "tipo de relação inválido") {
		t.Errorf("expected pt-BR invalid-type error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "depends_on") {
		t.Errorf("expected error to list valid types, got: %v", err)
	}
}

func TestGraphLink_MissingFlags(t *testing.T) {
	root := fakeGraphTree(t)
	if _, err := executeGraph(t, root, "link"); err == nil {
		t.Error("link with no flags should fail (required --from/--to/--type)")
	}
}

// =============================================================================
// `graph evidence` — populated
// =============================================================================

func TestGraphEvidence_Populated(t *testing.T) {
	root := fakeGraphTree(t)

	for _, link := range []struct{ to, rel, weight string }{
		{to: "namespaces", rel: "implements", weight: "0.8"},
		{to: "Linux", rel: "depends_on", weight: ""},
		{to: "CVE-2024-2947", rel: "affected_by", weight: ""},
		{to: "repo-A", rel: "documented_in", weight: ""},
		{to: "test-B", rel: "tested_by", weight: ""},
		{to: "CONFLICT-102", rel: "contradicted_by", weight: ""},
	} {
		args := []string{"link", "--from", "Bubblewrap", "--to", link.to, "--type", link.rel}
		if link.weight != "" {
			args = append(args, "--weight", link.weight)
		}
		if _, err := executeGraph(t, root, args...); err != nil {
			t.Fatalf("link %s: %v", link.rel, err)
		}
	}

	out, err := executeGraph(t, root, "evidence", "Bubblewrap")
	if err != nil {
		t.Fatalf("graph evidence: %v", err)
	}
	if !strings.Contains(out, "Mapa de evidências — Bubblewrap") {
		t.Errorf("output sem header do mapa de evidências: %q", out)
	}
	for _, want := range []string{
		"implements → namespaces (weight 0.8)",
		"depends_on → Linux",
		"affected_by → CVE-2024-2947",
		"documented_in → repo-A",
		"tested_by → test-B",
		"contradicted_by → CONFLICT-102",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mapa sem a relação %q: %q", want, out)
		}
	}
}

func TestGraphEvidence_PopulatedJSON(t *testing.T) {
	root := fakeGraphTree(t)

	if _, err := executeGraph(t, root,
		"link", "--from", "Bubblewrap", "--to", "CVE-2024-2947", "--type", "affected_by"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if _, err := executeGraph(t, root,
		"link", "--from", "Bubblewrap", "--to", "repo-A", "--type", "documented_in"); err != nil {
		t.Fatalf("link: %v", err)
	}

	out, err := executeGraph(t, root, "evidence", "Bubblewrap", "--json")
	if err != nil {
		t.Fatalf("graph evidence --json: %v", err)
	}
	var res graphEvidenceResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if res.Entity != "Bubblewrap" || len(res.Edges) != 2 {
		t.Errorf("unexpected JSON result: %+v", res)
	}
	types := map[string]bool{}
	for _, e := range res.Edges {
		types[e.Type] = true
	}
	if !types["affected_by"] || !types["documented_in"] {
		t.Errorf("JSON edges missing evidence relations: %+v", res.Edges)
	}
}

func TestGraphEvidence_PersistedOnDisk(t *testing.T) {
	root := fakeGraphTree(t)
	if _, err := executeGraph(t, root,
		"link", "--from", "Bubblewrap", "--to", "CONFLICT-102", "--type", "contradicted_by"); err != nil {
		t.Fatalf("link: %v", err)
	}
	storePath := filepath.Join(root, ".cosca", "graph", "evidence.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Errorf("evidence.json não criado no projeto: %v", err)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewGraphEvidenceCommand()
	_ *cobra.Command = NewGraphLinkCommand()
)
