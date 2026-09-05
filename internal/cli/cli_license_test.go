//
// Tests for the `cosca license` command tree (internal/cli/license.go).
//
// Covers:
//   - Registration of `license` (and its subcommands show/verify) in root
//   - `license show` — prints the LICENSE text (cwd), preferring
//     .cosca/framework/LICENSE when present
//   - `license verify` — factor table with Authenticated: sim/não for an
//     authenticated and a non-authenticated project (temp dirs)
//   - `license verify --json` — machine-readable AuthStatus
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

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

	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
	"github.com/CoscaAI/cosca/internal/license"
)

// =============================================================================
// Registration
// =============================================================================

func TestLicenseCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "license" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("license subcommand not registered in root command")
	}
}

func TestLicenseCommand_Properties(t *testing.T) {
	cmd := NewLicenseCommand()
	if cmd == nil {
		t.Fatal("NewLicenseCommand returned nil")
	}
	if cmd.Use != "license" {
		t.Errorf("expected Use='license', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"show", "verify"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing license subcommand: %s", name)
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

// runLicenseCmd finds a command under the root and runs its RunE, capturing
// both the formatter output and the cmd output into the returned string.
func runLicenseCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	globalFlags = GlobalFlags{} // avoid cross-test contamination

	root := NewRootCommand()
	cmd, _, err := root.Find(args)
	if err != nil {
		t.Fatalf("Find(%v) error: %v", args, err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err = cmd.RunE(cmd, nil)
	return buf.String(), err
}

// writeGoMod writes a go.mod into dir.
func writeGoMod(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

// seedAuthenticatedProject creates a fully authenticated project in dir:
// correct go.mod + .cosca/manifest.yaml (via WriteManifest).
func seedAuthenticatedProject(t *testing.T, dir string) {
	t.Helper()
	writeGoMod(t, dir, "module github.com/CoscaAI/cosca\n")
	if err := embedcosca.WriteManifest(filepath.Join(dir, ".cosca")); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
}

// =============================================================================
// `license show`
// =============================================================================

func TestLicenseShow(t *testing.T) {
	dir := chdirTemp(t)
	licenseText := "# COSCA — LICENÇA DE DIREITOS AUTORAIS\n" +
		"Copyright © 2026 CoscaAI. Todos os direitos reservados.\n" +
		"Repositório oficial: https://github.com/CoscaAI\n"
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(licenseText), 0o644); err != nil {
		t.Fatalf("write LICENSE: %v", err)
	}

	output, err := runLicenseCmd(t, "license", "show")
	if err != nil {
		t.Fatalf("license show returned error: %v", err)
	}
	if !strings.Contains(output, "CoscaAI") {
		t.Errorf("expected output to contain 'CoscaAI', got:\n%s", output)
	}
	if !strings.Contains(output, "https://github.com/CoscaAI") {
		t.Errorf("expected output to contain the official repository, got:\n%s", output)
	}
}

func TestLicenseShow_FallbackPreferred(t *testing.T) {
	dir := chdirTemp(t)

	// Root LICENSE (would be used only if the fallback copy is absent).
	rootText := "ROOT LICENSE — CoscaAI root\n"
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(rootText), 0o644); err != nil {
		t.Fatalf("write LICENSE: %v", err)
	}

	// Synced framework copy in .cosca/framework/ — must be preferred.
	frameworkDir := filepath.Join(dir, ".cosca", "framework")
	if err := os.MkdirAll(frameworkDir, 0o755); err != nil {
		t.Fatalf("mkdir framework: %v", err)
	}
	fallbackText := "FALLBACK LICENSE — Copyright © 2026 CoscaAI. All rights reserved.\n"
	if err := os.WriteFile(filepath.Join(frameworkDir, "LICENSE"), []byte(fallbackText), 0o644); err != nil {
		t.Fatalf("write framework LICENSE: %v", err)
	}

	output, err := runLicenseCmd(t, "license", "show")
	if err != nil {
		t.Fatalf("license show returned error: %v", err)
	}
	if !strings.Contains(output, "FALLBACK LICENSE") {
		t.Errorf("expected the fallback copy to be displayed, got:\n%s", output)
	}
}

func TestLicenseShow_NoLicenseFile(t *testing.T) {
	chdirTemp(t) // empty temp dir — no LICENSE anywhere

	_, err := runLicenseCmd(t, "license", "show")
	if err == nil {
		t.Fatal("expected an error when LICENSE is missing")
	}
}

func TestLicenseShow_JSON(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("CoscaAI license text\n"), 0o644); err != nil {
		t.Fatalf("write LICENSE: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"license", "show"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if err := cmd.Root().PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set json flag: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("license show --json returned error: %v", err)
	}
	var out map[string]string
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !strings.Contains(out["content"], "CoscaAI") {
		t.Errorf("json content missing 'CoscaAI': %q", out["content"])
	}
}

// =============================================================================
// `license verify`
// =============================================================================

func TestLicenseVerify_Authenticated(t *testing.T) {
	dir := chdirTemp(t)
	seedAuthenticatedProject(t, dir)

	output, err := runLicenseCmd(t, "license", "verify")
	if err != nil {
		t.Fatalf("license verify returned error: %v", err)
	}
	for _, want := range []string{
		"Chave de Segurança",
		"F1 módulo Go",
		"F2 manifesto",
		"F3 build",
		"sim",
		"✓",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("verify output missing %q; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "não") {
		t.Errorf("verified project reported 'não'; output:\n%s", output)
	}
}

func TestLicenseVerify_NotAuthenticated(t *testing.T) {
	dir := chdirTemp(t)
	// Correct go.mod but NO .cosca/manifest.yaml → F2 fails → not authenticated.
	writeGoMod(t, dir, "module github.com/CoscaAI/cosca\n")

	output, err := runLicenseCmd(t, "license", "verify")
	if err != nil {
		t.Fatalf("license verify returned error: %v", err)
	}
	if !strings.Contains(output, "não") {
		t.Errorf("expected 'não' (not authenticated), got:\n%s", output)
	}
	if !strings.Contains(output, "✗") {
		t.Errorf("expected a failed factor (✗), got:\n%s", output)
	}
	if strings.Contains(output, "sim") {
		t.Errorf("non-authenticated project reported 'sim'; output:\n%s", output)
	}
}

func TestLicenseVerify_ForeignModule(t *testing.T) {
	dir := chdirTemp(t)
	// Foreign module: F1 fails even with a forged manifest.
	writeGoMod(t, dir, "module example.com/not-cosca\n")
	if err := embedcosca.WriteManifest(filepath.Join(dir, ".cosca")); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	output, err := runLicenseCmd(t, "license", "verify")
	if err != nil {
		t.Fatalf("license verify returned error: %v", err)
	}
	if !strings.Contains(output, "não") {
		t.Errorf("expected 'não' for a foreign module, got:\n%s", output)
	}
}

func TestLicenseVerify_JSON(t *testing.T) {
	dir := chdirTemp(t)
	seedAuthenticatedProject(t, dir)

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"license", "verify"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if err := cmd.Root().PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set json flag: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("license verify --json returned error: %v", err)
	}
	var status license.AuthStatus
	if err := json.Unmarshal(buf.Bytes(), &status); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !status.Authenticated {
		t.Fatalf("expected authenticated=true in JSON, got false (detail: %s)", status.Detail)
	}
	if !status.Factors[license.FactorModule] || !status.Factors[license.FactorManifest] || !status.Factors[license.FactorBuild] {
		t.Errorf("expected all 3 factors true, got %v", status.Factors)
	}
	if status.ModulePath != "github.com/CoscaAI/cosca" {
		t.Errorf("ModulePath = %q, want %q", status.ModulePath, "github.com/CoscaAI/cosca")
	}
}

// =============================================================================
// `license` (no subcommand) behaves as `license show`
// =============================================================================

func TestLicense_Bare_RunsShow(t *testing.T) {
	dir := chdirTemp(t)
	licenseText := "# COSCA — LICENÇA\nCopyright © 2026 CoscaAI. Todos os direitos reservados.\n"
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(licenseText), 0o644); err != nil {
		t.Fatalf("write LICENSE: %v", err)
	}

	output, err := runLicenseCmd(t, "license")
	if err != nil {
		t.Fatalf("bare 'license' returned error: %v", err)
	}
	if !strings.Contains(output, "CoscaAI") {
		t.Errorf("expected bare 'license' to show the license text, got:\n%s", output)
	}
}

// =============================================================================
// Subcommand structure
// =============================================================================

func TestLicenseSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"show", newLicenseShowCommand, "show"},
		{"verify", newLicenseVerifyCommand, "verify"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.factory()
			if cmd == nil {
				t.Fatalf("factory for %q returned nil", tt.name)
			}
			if cmd.Use != tt.use {
				t.Errorf("expected Use=%q, got %q", tt.use, cmd.Use)
			}
			if cmd.Short == "" {
				t.Errorf("expected non-empty Short description for %s", tt.name)
			}
			if cmd.RunE == nil {
				t.Errorf("expected RunE to be set for %s", tt.name)
			}
		})
	}
}
