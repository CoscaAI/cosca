package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPerfCommand_Registered garante que `cosca perf` está na árvore raiz.
func TestPerfCommand_Registered(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Name() == "perf" {
			return
		}
	}
	t.Error("missing root subcommand: perf")
}

// TestPerfColdStart_Registered garante que `cosca perf cold-start` existe.
func TestPerfColdStart_Registered(t *testing.T) {
	cmd := NewPerfCommand()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "cold-start" {
			return
		}
	}
	t.Error("missing perf subcommand: cold-start")
}

// TestPerfColdStart_Run mede o cold-start num repo de teste e verifica a saída.
func TestPerfColdStart_Run(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var buf bytes.Buffer
	cmd := NewPerfColdStartCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	cmd.SetContext(newContextWithFormatter(t.Context(), f))
	_ = cmd.ParseFlags([]string{"--budget", "5000"})

	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Cold-Start Contract")
	require.Contains(t, buf.String(), "Mediana")
	require.Contains(t, buf.String(), "Classe epistêmica: measured", "must declare epistemic class measured")

	// O histórico foi persistido em .cosca/performance/coldstart.jsonl
	require.FileExists(t, dir+"/.cosca/performance/coldstart.jsonl")
}
