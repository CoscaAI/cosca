//
// Tests for the `cosca capability` command tree (internal/cli/capability.go).
//
// Covers:
//   - Registration of `capability` (and `level`) in the root command
//   - Command properties (Use/Short/Long) and arg constraints
//   - `capability level` text output (contains the level name + capabilities)
//   - `capability level` with an explicit provider configured via viper
//   - `capability level` JSON mode
//   - `capability level` with provider "none" → deterministic mode (L0)
//
// Uses the newContextWithFormatter + PersistentPreRunE = nil pattern so the
// output goes to a buffer (mirroring cli_cv_test.go).

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
)

// resetGlobalViper limpa o viper global para que cada teste comece isolado
// (sem vazamento de chaves definidas por testes anteriores no mesmo pacote).
func resetGlobalViper() {
	config.SetViper(nil)
}

func TestCapabilityCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "capability" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("capability subcommand not registered in root command")
	}
}

func TestCapabilityCommand_Properties(t *testing.T) {
	cmd := NewCapabilityCommand()
	if cmd == nil {
		t.Fatal("NewCapabilityCommand returned nil")
	}
	if cmd.Use != "capability" {
		t.Errorf("expected Use='capability', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
	if len(cmd.Commands()) != 1 || cmd.Commands()[0].Name() != "level" {
		t.Errorf("expected exactly one subcommand 'level', got %d", len(cmd.Commands()))
	}
}

func TestCapabilityLevelCommand_Properties(t *testing.T) {
	cmd := NewCapabilityLevelCommand()
	if cmd == nil {
		t.Fatal("NewCapabilityLevelCommand returned nil")
	}
	if cmd.Use != "level" {
		t.Errorf("expected Use='level', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("level with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("level with no args should be allowed: %v", err)
	}
}

// executeCapability runs a `capability` subcommand with output to a buffer.
func executeCapability(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"capability"}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func TestCapabilityLevelCommand_TextOutputContainsLevel(t *testing.T) {
	// Without a config file, the provider is unset → deterministic (L0).
	resetGlobalViper()
	out, err := executeCapability(t, "level")
	if err != nil {
		t.Fatalf("capability level: %v", err)
	}
	for _, want := range []string{"L0-determinístico", "Nível", "Capacidades"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	if !strings.Contains(out, "✓ disponível") {
		t.Errorf("expected available capability markers, got: %q", out)
	}
}

func TestCapabilityLevelCommand_ProviderFromConfig(t *testing.T) {
	resetGlobalViper()
	v := config.GetViper()
	v.Set("provider.primary", "ollama")
	v.Set("runtime.enabled", true)
	v.Set("pipeline.enabled", true)

	out, err := executeCapability(t, "level")
	if err != nil {
		t.Fatalf("capability level (ollama): %v", err)
	}
	if !strings.Contains(out, "L3-autônomo") {
		t.Errorf("expected L3-autônomo for ollama provider with runtime enabled, got: %q", out)
	}
	if !strings.Contains(out, "ollama") {
		t.Errorf("expected provider name in output, got: %q", out)
	}
}

func TestCapabilityLevelCommand_NoneProviderIsDeterministic(t *testing.T) {
	resetGlobalViper()
	v := config.GetViper()
	v.Set("provider.primary", "none")

	out, err := executeCapability(t, "level")
	if err != nil {
		t.Fatalf("capability level (none): %v", err)
	}
	if !strings.Contains(out, "L0-determinístico") {
		t.Errorf("expected L0-determinístico for none provider, got: %q", out)
	}
	if !strings.Contains(out, "modo determinístico") {
		t.Errorf("expected deterministic-mode warning, got: %q", out)
	}
}

func TestCapabilityLevelCommand_RuntimeEnabledIsAutonomous(t *testing.T) {
	resetGlobalViper()
	v := config.GetViper()
	v.Set("provider.primary", "deepseek")
	v.Set("runtime.enabled", true)
	v.Set("pipeline.enabled", true)

	out, err := executeCapability(t, "level")
	if err != nil {
		t.Fatalf("capability level (deepseek+runtime): %v", err)
	}
	if !strings.Contains(out, "L3-autônomo") {
		t.Errorf("expected L3-autônomo with runtime enabled, got: %q", out)
	}
}

func TestCapabilityLevelCommand_JSONOutput(t *testing.T) {
	resetGlobalViper()
	v := config.GetViper()
	v.Set("provider.primary", "deepseek")
	v.Set("runtime.enabled", true)
	v.Set("pipeline.enabled", true)

	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"capability", "level", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("capability level --json: %v", err)
	}
	for _, want := range []string{`"level": "L3-autônomo"`, `"provider": "deepseek"`, `"available"`, `"unavailable"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestCapabilityCommand_NoSubcommand_ShowsHelp(t *testing.T) {
	out, err := executeCapability(t)
	if err != nil {
		t.Fatalf("capability with no subcommand should show help without error: %v", err)
	}
	if !strings.Contains(out, "level") {
		t.Errorf("expected help listing subcommands, got: %q", out)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewCapabilityCommand()
	_ *cobra.Command = NewCapabilityLevelCommand()
)
