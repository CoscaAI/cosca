//
// Tests for the `cosca machine` command tree (internal/cli/machine.go).
//
// Covers:
//   - Registration of `machine` (probe/profile/diff) in the root command
//   - Command properties (Use/Short) and arg constraints
//   - `machine probe` text output contains CPU/Memória/GPU/Capabilities/Hash
//   - `machine probe --json` machine-readable output
//   - `machine profile` first run saves + hash; second run "inalterado"
//   - `machine diff` without a profile → error; after profile → no changes
//   - Unknown subcommand rejected
//
// Tests that chdir() must NOT run in parallel — they mutate the process
// working directory. Each test restores the cwd via deferred os.Chdir.
//
// Uses the newContextWithFormatter + PersistentPreRunE = nil pattern so the
// output goes to a buffer (mirroring cli_hardware_test.go / cli_cv_test.go).

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

func TestMachineCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "machine" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("machine subcommand not registered in root command")
	}
}

func TestMachineCommand_Properties(t *testing.T) {
	cmd := NewMachineCommand()
	if cmd == nil {
		t.Fatal("NewMachineCommand returned nil")
	}
	if cmd.Use != "machine" {
		t.Errorf("expected Use='machine', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	expected := []string{"probe", "profile", "diff", "capability"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing machine subcommand: %s", name)
		}
	}
}

func TestMachineSubcommands_ArgsNoArgs(t *testing.T) {
	for name, cmd := range map[string]*cobra.Command{
		"probe":   NewMachineProbeCommand(),
		"profile": NewMachineProfileCommand(),
		"diff":    NewMachineDiffCommand(),
	} {
		if err := cmd.Args(cmd, []string{"extra"}); err == nil {
			t.Errorf("%s with args should fail", name)
		}
		if err := cmd.Args(cmd, nil); err != nil {
			t.Errorf("%s with no args should be allowed: %v", name, err)
		}
	}
}

// executeMachine runs a `machine` subcommand from `root` with output to a buffer.
func executeMachine(t *testing.T, root string, args ...string) (string, error) {
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
	cmd.SetArgs(append([]string{"machine"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

func TestMachineProbe_TextOutputContainsSections(t *testing.T) {
	out, err := executeMachine(t, t.TempDir(), "probe")
	if err != nil {
		t.Fatalf("machine probe: %v", err)
	}
	for _, want := range []string{"CPU", "Memória", "GPU", "Capabilities"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q section: %q", want, out)
		}
	}
	for _, want := range []string{"Cores", "Total", "Vendor", "Lista", "Hash"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q key: %q", want, out)
		}
	}
}

func TestMachineProbe_JSONOutput(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"machine", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("machine probe --json: %v", err)
	}
	for _, want := range []string{`"cpu"`, `"memory"`, `"gpu"`, `"capabilities"`, `"hash"`, `"probed_at"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestMachineProfile_FirstRunSavesSecondRunUnchanged(t *testing.T) {
	root := t.TempDir()

	// Primeira execução: salva o perfil e imprime o hash.
	out, err := executeMachine(t, root, "profile")
	if err != nil {
		t.Fatalf("machine profile (first): %v", err)
	}
	if !strings.Contains(out, "Perfil de capacidade salvo") {
		t.Errorf("expected save message, got: %q", out)
	}
	if !strings.Contains(out, "Hash") {
		t.Errorf("expected hash in output, got: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".cosca", "machine", "profile.json")); err != nil {
		t.Errorf("profile.json not written: %v", err)
	}

	// Segunda execução (mesmo hardware): perfil inalterado.
	out, err = executeMachine(t, root, "profile")
	if err != nil {
		t.Fatalf("machine profile (second): %v", err)
	}
	if !strings.Contains(out, "Perfil inalterado") {
		t.Errorf("expected unchanged message, got: %q", out)
	}
}

func TestMachineProfile_JSONOutputFirstRun(t *testing.T) {
	root := t.TempDir()
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
	cmd.SetArgs([]string{"machine", "profile", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("machine profile --json: %v", err)
	}
	for _, want := range []string{`"cpu"`, `"capabilities"`, `"hash"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestMachineDiff_WithoutProfileReturnsError(t *testing.T) {
	_, err := executeMachine(t, t.TempDir(), "diff")
	if err == nil {
		t.Fatal("expected error when no profile is saved")
	}
	if !strings.Contains(err.Error(), "nenhum perfil salvo") {
		t.Errorf("expected missing-profile message, got: %v", err)
	}
}

func TestMachineDiff_AfterProfileNoChanges(t *testing.T) {
	root := t.TempDir()
	if _, err := executeMachine(t, root, "profile"); err != nil {
		t.Fatalf("machine profile: %v", err)
	}
	out, err := executeMachine(t, root, "diff")
	if err != nil {
		t.Fatalf("machine diff: %v", err)
	}
	if !strings.Contains(out, "Sem mudanças") {
		t.Errorf("expected no-changes message, got: %q", out)
	}
}

func TestMachineCommand_UnknownSubcommand_ReturnsError(t *testing.T) {
	out, err := executeMachine(t, t.TempDir(), "bogus")
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
	if strings.Contains(out, "Capabilities") {
		t.Error("probe should not run for unknown subcommand")
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewMachineCommand()
	_ *cobra.Command = NewMachineProbeCommand()
	_ *cobra.Command = NewMachineProfileCommand()
	_ *cobra.Command = NewMachineDiffCommand()
)
