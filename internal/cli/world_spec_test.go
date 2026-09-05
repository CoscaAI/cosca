package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/gameengine"
	"github.com/CoscaAI/cosca/internal/worldspec"
)

// writeSpecJSON grava um WorldSpec válido num arquivo temporário.
func writeSpecJSON(t *testing.T, name, worldType string, isolated bool) string {
	t.Helper()
	spec := worldspec.New(42, name, worldType)
	spec.Provenance = worldspec.Provenance{Source: "llm_proposal", Class: "generated", Tool: "cosca-planner@1", Seed: 42}
	spec.Entities = append(spec.Entities,
		gameengine.Entity{ID: "porto", Name: "porto", Components: []gameengine.Component{{Type: "building"}}})
	spec.Navigation.Nodes = []worldspec.NavNode{{ID: "n_porto"}, {ID: "n_centro"}}
	spec.Navigation.Edges = []worldspec.NavEdge{{From: "n_porto", To: "n_centro", Type: "road"}}
	if isolated {
		spec.Navigation.Nodes = append(spec.Navigation.Nodes, worldspec.NavNode{ID: "n_norte"})
	}
	data, err := spec.JSON()
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	p := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return p
}

func TestWorldBuildCommand(t *testing.T) {
	p := writeSpecJSON(t, "cidade-costeira", "open_world", false)
	out := filepath.Join(t.TempDir(), "out")

	cmd := NewWorldBuildCommand()
	cmd.SetContext(newContextWithFormatter(context.Background(), NewOutputFormatter(io.Discard, OutputFormatText, false, false, false)))
	cmd.SetArgs([]string{p, "--out", out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("world build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "default.project.json")); err != nil {
		t.Fatalf("projeto Rojo nao gerado: %v", err)
	}
}

func TestWorldLoopCommand(t *testing.T) {
	p := writeSpecJSON(t, "cidade", "open_world", true) // com nó isolado (n_norte)

	cmd := NewWorldLoopCommand()
	cmd.SetContext(newContextWithFormatter(context.Background(), NewOutputFormatter(io.Discard, OutputFormatText, false, false, false)))
	cmd.SetArgs([]string{p, "--max-iter", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("world loop: %v", err)
	}
}

func TestLoadSpec_FailClosed(t *testing.T) {
	// sem seed/proveniência -> validate falha (I1/I4)
	p := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(p, []byte(`{"name":"x","worldType":"open_world"}`), 0o644)
	if _, err := loadSpec(p); err == nil {
		t.Fatal("spec inválido deveria falhar (fail-closed I1/I4)")
	}
}
