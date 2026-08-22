//
// Tests for the `cosca acquisition budget` command tree
// (internal/cli/acquisition.go).
//
// Covers:
//   - Registration of `acquisition` (and `acquisition budget`) in the root
//   - Command properties + subcommands (budget → default, check)
//   - `acquisition budget default` output (Fontes: 8, Arquivos: 100, Rede: 20MB,
//     Tempo: 30s, IA: 8k)
//   - `acquisition budget check` dentro do orçamento (fits)
//   - `acquisition budget check` acima do orçamento por fontes/rede/tempo
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
// Registration — `acquisition` no root
// =============================================================================

func TestAcquisitionCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "acquisition" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("acquisition subcommand not registered in root command")
	}
}

func TestAcquisitionCommand_Properties(t *testing.T) {
	cmd := NewAcquisitionCommand()
	if cmd == nil {
		t.Fatal("NewAcquisitionCommand returned nil")
	}
	if cmd.Use != "acquisition" {
		t.Errorf("expected Use='acquisition', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}

	budget := NewAcquisitionBudgetCommand()
	if budget.Use != "budget" {
		t.Errorf("expected Use='budget', got %q", budget.Use)
	}
	expected := []string{"default", "check"}
	if len(budget.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(budget.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range budget.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing acquisition budget subcommand: %s", name)
		}
	}
}

// runAcquisitionCommand executa o RunE de um subcomando com formatter injetado
// em um buffer (padrão de injeção de formatter do CLI).
func runAcquisitionCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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
// `acquisition budget default`
// =============================================================================

func TestAcquisitionBudgetDefault_Output(t *testing.T) {
	cmd := NewAcquisitionBudgetDefaultCommand()
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget default: %v", err)
	}
	for _, want := range []string{
		"Orçamento de aquisição (default)",
		"8",
		"100",
		"20MB",
		"30s",
		"8k",
		"Não consegui validar com confiança dentro do orçamento. Preciso da sua decisão.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("acquisition budget default output missing %q: %q", want, out)
		}
	}
}

func TestAcquisitionBudgetDefault_JSON(t *testing.T) {
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	cmd := NewAcquisitionBudgetDefaultCommand()
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget default --json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["max_sources"] != float64(8) {
		t.Errorf("max_sources = %v, want 8", parsed["max_sources"])
	}
	if parsed["max_files"] != float64(100) {
		t.Errorf("max_files = %v, want 100", parsed["max_files"])
	}
	if parsed["max_network_mb"] != float64(20) {
		t.Errorf("max_network_mb = %v, want 20", parsed["max_network_mb"])
	}
	if parsed["max_ai_tokens"] != float64(8000) {
		t.Errorf("max_ai_tokens = %v, want 8000", parsed["max_ai_tokens"])
	}
}

// =============================================================================
// `acquisition budget check` — dentro do orçamento
// =============================================================================

func TestAcquisitionBudgetCheck_WithinBudget(t *testing.T) {
	cmd := NewAcquisitionBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--sources", "3", "--files", "2", "--network-mb", "1", "--time", "1.2s"})
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget check: %v", err)
	}
	if !strings.Contains(out, "Dentro do orçamento") {
		t.Errorf("expected 'Dentro do orçamento', got: %q", out)
	}
	if strings.Contains(out, "Acima do orçamento") {
		t.Errorf("should not report over budget: %q", out)
	}
	for _, want := range []string{"3 / 8", "2 / 100", "1MB / 20MB", "1.2s / 30s"} {
		if !strings.Contains(out, want) {
			t.Errorf("acquisition budget check output missing %q: %q", want, out)
		}
	}
}

// =============================================================================
// `acquisition budget check` — acima do orçamento
// =============================================================================

func TestAcquisitionBudgetCheck_OverSources(t *testing.T) {
	cmd := NewAcquisitionBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--sources", "10", "--files", "5", "--network-mb", "3", "--time", "1.2s"})
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget check: %v", err)
	}
	if !strings.Contains(out, "Acima do orçamento") {
		t.Errorf("expected 'Acima do orçamento', got: %q", out)
	}
	if !strings.Contains(out, "fontes") {
		t.Errorf("over dimension 'fontes' should be listed: %q", out)
	}
	if strings.Contains(out, "Dentro do orçamento") {
		t.Errorf("should not report within budget: %q", out)
	}
}

func TestAcquisitionBudgetCheck_OverNetworkAndTime(t *testing.T) {
	cmd := NewAcquisitionBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--sources", "1", "--network-mb", "21", "--time", "45s"})
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget check: %v", err)
	}
	if !strings.Contains(out, "Acima do orçamento") {
		t.Errorf("expected 'Acima do orçamento', got: %q", out)
	}
	if !strings.Contains(out, "rede") {
		t.Errorf("over dimension 'rede' should be listed: %q", out)
	}
	if !strings.Contains(out, "tempo") {
		t.Errorf("over dimension 'tempo' should be listed: %q", out)
	}
}

// =============================================================================
// `acquisition budget check` — JSON
// =============================================================================

func TestAcquisitionBudgetCheck_JSON(t *testing.T) {
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	cmd := NewAcquisitionBudgetCheckCommand()
	_ = cmd.ParseFlags([]string{"--sources", "10", "--files", "5", "--network-mb", "3", "--time", "1.2s"})
	out, err := runAcquisitionCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("acquisition budget check --json: %v", err)
	}
	var parsed struct {
		Within  bool     `json:"within"`
		Sources int      `json:"sources"`
		OverBy  []string `json:"over_by"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed.Within {
		t.Error("within should be false for an over-budget check")
	}
	if parsed.Sources != 10 {
		t.Errorf("sources = %d, want 10", parsed.Sources)
	}
	if len(parsed.OverBy) != 1 || parsed.OverBy[0] != "fontes" {
		t.Errorf("over_by = %v, want [fontes]", parsed.OverBy)
	}
}

// compile-time guard: os argumentos dos comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewAcquisitionCommand()
	_ *cobra.Command = NewAcquisitionBudgetCommand()
	_ *cobra.Command = NewAcquisitionBudgetDefaultCommand()
	_ *cobra.Command = NewAcquisitionBudgetCheckCommand()
)
