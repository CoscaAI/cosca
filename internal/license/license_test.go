//
// Tests for the license security-key verification (internal/license).
//
// Covers:
//   - Fully authenticated project (go.mod + manifest + build) → 3 fatores ✓
//   - Foreign go.mod → not authenticated (F1 fails)
//   - Correct go.mod but missing manifest → not authenticated (F2 fails)
//   - Prefix module github.com/CoscaAI/cosca-client → NOT a match (F1 fails)
//
// The manifest is created with embedcosca.WriteManifest (the canonical
// writer), never hand-copied from the project.

package license

import (
	"os"
	"path/filepath"
	"testing"

	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
)

const correctGoMod = "module github.com/CoscaAI/cosca\n"

func TestVerifyAuthenticity_Authenticated(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, correctGoMod)
	if err := embedcosca.WriteManifest(filepath.Join(dir, ".cosca")); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	status, err := VerifyAuthenticity(dir)
	if err != nil {
		t.Fatalf("VerifyAuthenticity returned error: %v", err)
	}
	if !status.Authenticated {
		t.Fatalf("expected authenticated=true, got false (detail: %s)", status.Detail)
	}
	for _, f := range []AuthFactor{FactorModule, FactorManifest, FactorBuild} {
		if !status.Factors[f] {
			t.Errorf("factor %s = false, want true", f)
		}
	}
	if status.ModulePath != embedcosca.SelfModule {
		t.Errorf("ModulePath = %q, want %q", status.ModulePath, embedcosca.SelfModule)
	}
	if status.Version == "" {
		t.Error("expected non-empty Version")
	}
}

func TestVerifyAuthenticity_NotCosca(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module example.com/other-project\n")

	status, err := VerifyAuthenticity(dir)
	if err != nil {
		t.Fatalf("VerifyAuthenticity returned error: %v", err)
	}
	if status.Authenticated {
		t.Fatal("expected authenticated=false for a foreign go.mod")
	}
	if status.Factors[FactorModule] {
		t.Error("expected F1 (module) to fail for a foreign go.mod")
	}
	if status.ModulePath != "example.com/other-project" {
		t.Errorf("ModulePath = %q, want %q", status.ModulePath, "example.com/other-project")
	}
}

func TestVerifyAuthenticity_NoManifest(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, correctGoMod)
	// No .cosca/manifest.yaml is written — F2 must fail.

	status, err := VerifyAuthenticity(dir)
	if err != nil {
		t.Fatalf("VerifyAuthenticity returned error: %v", err)
	}
	if status.Authenticated {
		t.Fatal("expected authenticated=false when the manifest is missing")
	}
	if !status.Factors[FactorModule] {
		t.Error("expected F1 (module) to pass with the correct go.mod")
	}
	if status.Factors[FactorManifest] {
		t.Error("expected F2 (manifest) to fail when the manifest is missing")
	}
	// F3 depends on the test binary, which always embeds a version +
	// commit hash — it must still be reported (the failure is F2 alone).
	if !status.Factors[FactorBuild] {
		t.Error("expected F3 (build) to pass in the test binary")
	}
}

func TestVerifyAuthenticity_ModuleExact(t *testing.T) {
	// A prefix module path must NOT match: only the exact module
	// "github.com/CoscaAI/cosca" authenticates F1.
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/CoscaAI/cosca-client\n")

	status, err := VerifyAuthenticity(dir)
	if err != nil {
		t.Fatalf("VerifyAuthenticity returned error: %v", err)
	}
	if status.Authenticated {
		t.Fatal("expected authenticated=false for a prefix module github.com/CoscaAI/cosca-client")
	}
	if status.Factors[FactorModule] {
		t.Error("expected F1 (module) to fail for github.com/CoscaAI/cosca-client")
	}
	if status.ModulePath != "github.com/CoscaAI/cosca-client" {
		t.Errorf("ModulePath = %q, want %q", status.ModulePath, "github.com/CoscaAI/cosca-client")
	}
}

func TestVerifyAuthenticity_ModuleWithCommentAndGoVersion(t *testing.T) {
	// Robustness: a real go.mod with go directive, require blocks and a
	// trailing comment on the module line must still parse to the exact
	// module path.
	dir := t.TempDir()
	writeGoMod(t, dir, `module github.com/CoscaAI/cosca // canonical module

go 1.25.0

require (
	github.com/spf13/cobra v1.8.0
)
`)
	if err := embedcosca.WriteManifest(filepath.Join(dir, ".cosca")); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	status, err := VerifyAuthenticity(dir)
	if err != nil {
		t.Fatalf("VerifyAuthenticity returned error: %v", err)
	}
	if !status.Authenticated {
		t.Fatalf("expected authenticated=true, got false (detail: %s)", status.Detail)
	}
	if status.ModulePath != "github.com/CoscaAI/cosca" {
		t.Errorf("ModulePath = %q, want %q", status.ModulePath, embedcosca.SelfModule)
	}
}

func TestVerifyAuthenticity_InvalidProjectDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	if _, err := VerifyAuthenticity(missing); err == nil {
		t.Fatal("expected error for a missing project directory")
	}

	// A regular file is not a valid project directory either.
	file := filepath.Join(dir, "some-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := VerifyAuthenticity(file); err == nil {
		t.Fatal("expected error when projectDir is not a directory")
	}
}

// writeGoMod writes a go.mod into dir.
func writeGoMod(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}
