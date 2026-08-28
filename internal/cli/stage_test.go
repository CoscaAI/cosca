package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStageCommand_Registered garante que `cosca stage` está na árvore raiz.
func TestStageCommand_Registered(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Name() == "stage" {
			return
		}
	}
	t.Error("missing root subcommand: stage")
}

// TestStageNewAndStatus_RoundTrip (projeção, determinístico I1).
func TestStageNewAndStatus_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rollPath := filepath.Join(dir, ".cosca", "evolution", "stage.jsonl")

	// stage new <projeto>
	{
		var buf bytes.Buffer
		cmd := NewStageNewCommand()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
		cmd.SetContext(newContextWithFormatter(t.Context(), f))
		_ = cmd.ParseFlags([]string{"--roll", rollPath})
		require.NoError(t, cmd.RunE(cmd, []string{"F1.5-skill-evolve"}))
		require.Contains(t, buf.String(), "iniciado em task")
	}

	// stage status
	{
		var buf bytes.Buffer
		cmd := NewStageStatusCommand()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
		cmd.SetContext(newContextWithFormatter(t.Context(), f))
		_ = cmd.ParseFlags([]string{"--roll", rollPath})
		require.NoError(t, cmd.RunE(cmd, nil))
		require.Contains(t, buf.String(), "Estágio atual")
		require.Contains(t, buf.String(), "task")
		require.Contains(t, buf.String(), "F1.5-skill-evolve")
	}
}
