package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestIntelligenceCommand_Tree verifica que a árvore de comandos existe.
func TestIntelligenceCommand_Tree(t *testing.T) {
	cmd := NewIntelligenceCommand()
	if cmd == nil {
		t.Fatal("NewIntelligenceCommand returned nil")
	}
	if cmd.Use != "intelligence" {
		t.Errorf("expected Use 'intelligence', got %q", cmd.Use)
	}

	// subcomandos obrigatórios
	want := []string{"plan", "conflicts", "gate"}
	for _, sub := range want {
		if findSub(cmd, sub) == nil {
			t.Errorf("expected subcommand %q", sub)
		}
	}
}

// TestIntelligenceCommand_RunsInShadow verifica que o comando em si não tem
// flags de escrita (shadow-first: o CLI nunca aplica edição).
func TestIntelligenceCommand_NoWriteFlags(t *testing.T) {
	cmd := NewIntelligenceCommand()
	flags := cmd.Flags()
	if flags == nil {
		t.Fatal("expected flags")
	}
	// o root do intelligence não deve ter flags de aplicação
	plan := findSub(cmd, "plan")
	if plan == nil {
		t.Fatal("plan not found")
	}
	if isWriteCommand(plan) {
		t.Error("plan should be a read-only/shadow command")
	}
}

func findSub(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func isWriteCommand(cmd *cobra.Command) bool {
	// heurística: comandos de escrita costumam ter flags de recurso/aplicação
	// aqui nenhum subcomando do intelligence deve alterar o conhecimento.
	if cmd == nil {
		return true
	}
	for _, f := range []string{"resource", "content", "new", "apply", "write"} {
		if cmd.Flag(f) != nil {
			return true
		}
	}
	return false
}

// TestIntelligenceGate_Flags verifica as flags do subcomando gate.
func TestIntelligenceGate_Flags(t *testing.T) {
	cmd := NewIntelligenceCommand()
	gate := findSub(cmd, "gate")
	if gate == nil {
		t.Fatal("gate not found")
	}
	for _, f := range []string{"resource", "evidence", "don", "shadow", "content"} {
		if gate.Flag(f) == nil {
			t.Errorf("expected gate flag %q", f)
		}
	}
}

// evita import não usado em builds com strings
var _ = strings.TrimSpace
