//
// Tests for the `cosca budget` command tree (internal/cli/budget.go).
//
// Covers:
//   - Registration of `budget` in the root command
//   - Command properties + subcommands (default, check)
//   - `budget default` output (Tokens: 8k, Tempo: 20s, Custo: $0.05)
//   - `budget check` dentro do orçamento (fits)
//   - `budget check` acima do orçamento por tokens, tempo e custo
//   - JSON output (formatter injection pattern)
//
// Read-only/diagnostic: nothing is written to disk.
//

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Registration — `budget` no root
// =============================================================================

func TestBudgetCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "budget" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("budget subcommand not registered in root command")
	}
}

func TestBudgetCommand_Properties(t *testing.T) {
	cmd := NewBudgetCommand()
	if cmd == nil {
		t.Fatal("NewBudgetCommand returned nil")
	}
	if cmd.Use != "budget" {
		t.Errorf("expected Use='budget', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"default", "check"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing budget subcommand: %s", name)
		}
	}
}

// runBudgetCommand executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runBudgetCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// =============================================================================
// `budget default`
// =============================================================================

func TestBudgetDefault_Output(t *testing.T) {
	cmd := NewBudgetDefaultCommand()
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget default: %v", err)
	}
	for _, want := range []string{"Budget cognitivo (default)", "8000", "20s", "$0.05"} {
		if !strings.Contains(out, want) {
			t.Errorf("budget default output missing %q: %q", want, out)
		}
	}
}

func TestBudgetDefault_JSON(t *testing.T) {
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	cmd := NewBudgetDefaultCommand()
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget default --json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["max_tokens"] != float64(8000) {
		t.Errorf("max_tokens = %v, want 8000", parsed["max_tokens"])
	}
	if parsed["max_cost"] != 0.05 {
		t.Errorf("max_cost = %v, want 0.05", parsed["max_cost"])
	}
}

// =============================================================================
// `budget check` — dentro do orçamento
// =============================================================================

func TestBudgetCheck_WithinBudget(t *testing.T) {
	cmd := NewBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--tokens", "500", "--time", "1.2s"})
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget check: %v", err)
	}
	if !strings.Contains(out, "Dentro do orçamento") {
		t.Errorf("expected 'Dentro do orçamento', got: %q", out)
	}
	if strings.Contains(out, "Acima do orçamento") {
		t.Errorf("should not report over budget: %q", out)
	}
	for _, want := range []string{"500 / 8000", "1.2s / 20s"} {
		if !strings.Contains(out, want) {
			t.Errorf("budget check output missing %q: %q", want, out)
		}
	}
}

// =============================================================================
// `budget check` — acima do orçamento
// =============================================================================

func TestBudgetCheck_OverTokens(t *testing.T) {
	cmd := NewBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--tokens", "9000", "--time", "1.2s"})
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget check: %v", err)
	}
	if !strings.Contains(out, "Acima do orçamento") {
		t.Errorf("expected 'Acima do orçamento', got: %q", out)
	}
	if !strings.Contains(out, "tokens") {
		t.Errorf("over dimension 'tokens' should be listed: %q", out)
	}
	if strings.Contains(out, "Dentro do orçamento") {
		t.Errorf("should not report within budget: %q", out)
	}
}

func TestBudgetCheck_OverTime(t *testing.T) {
	cmd := NewBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--tokens", "100", "--time", "30s"})
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget check: %v", err)
	}
	if !strings.Contains(out, "Acima do orçamento") {
		t.Errorf("expected 'Acima do orçamento', got: %q", out)
	}
	if !strings.Contains(out, "tempo") {
		t.Errorf("over dimension 'tempo' should be listed: %q", out)
	}
}

func TestBudgetCheck_OverCost(t *testing.T) {
	cmd := NewBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--tokens", "100", "--time", "1.2s", "--cost", "0.99"})
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget check: %v", err)
	}
	if !strings.Contains(out, "Acima do orçamento") {
		t.Errorf("expected 'Acima do orçamento', got: %q", out)
	}
	if !strings.Contains(out, "custo") {
		t.Errorf("over dimension 'custo' should be listed: %q", out)
	}
}

// =============================================================================
// `budget check` — JSON
// =============================================================================

func TestBudgetCheck_JSON(t *testing.T) {
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	cmd := NewBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--tokens", "9000", "--time", "1.2s"})
	out, err := runBudgetCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("budget check --json: %v", err)
	}
	var parsed struct {
		Within bool     `json:"within"`
		Tokens int      `json:"tokens"`
		OverBy []string `json:"over_by"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed.Within {
		t.Error("within should be false for an over-budget check")
	}
	if parsed.Tokens != 9000 {
		t.Errorf("tokens = %d, want 9000", parsed.Tokens)
	}
	if len(parsed.OverBy) != 1 || parsed.OverBy[0] != "tokens" {
		t.Errorf("over_by = %v, want [tokens]", parsed.OverBy)
	}
}

// compile-time guard: os argumentos dos comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewBudgetCommand()
	_ *cobra.Command = NewBudgetDefaultCommand()
	_ *cobra.Command = NewBudgetCheckCommand()
)
