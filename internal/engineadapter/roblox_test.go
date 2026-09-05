package engineadapter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/gameengine"
	"github.com/CoscaAI/cosca/internal/worldspec"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestRoblox_Materialize(t *testing.T) {
	spec := worldspec.New(42, "cidade-costeira", "open_world")
	spec.Provenance = worldspec.Provenance{Source: "llm_proposal", Class: "generated", Tool: "cosca-planner@1", Seed: 42}
	spec.Entities = append(spec.Entities,
		gameengine.Entity{ID: "porto", Name: "porto", Components: []gameengine.Component{{Type: "building"}}},
		gameengine.Entity{ID: "avenida", Name: "avenida", Components: []gameengine.Component{{Type: "road"}}},
	)

	dir := filepath.Join(t.TempDir(), "out")
	adapter := NewRobloxAdapter(dir)
	handle, err := adapter.Materialize(context.Background(), *spec)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if handle.Target != "roblox" {
		t.Fatalf("target: got %s", handle.Target)
	}
	// default.project.json deve ser JSON válido.
	data, err := os.ReadFile(filepath.Join(dir, "default.project.json"))
	if err != nil {
		t.Fatalf("projeto nao gerado: %v", err)
	}
	var proj map[string]any
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("default.project.json inválido: %v", err)
	}
	if proj["name"] != "cidade-costeira" {
		t.Fatalf("name: got %v", proj["name"])
	}

	// O Luau reflete o WorldSpec (checa linhas-chave).
	wl, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash("src/ReplicatedStorage/Packages/WorldSpec.luau")))
	if err != nil {
		t.Fatalf("worldspec.luau: %v", err)
	}
	s := string(wl)
	for _, want := range []string{`WorldSpec.WorldType = "open_world"`, "WorldSpec.Seed = 42", "WorldSpec.EntityCount = 2", "return WorldSpec"} {
		if !contains(s, want) {
			t.Fatalf("worldspec.luau sem %q:\n%s", want, s)
		}
	}

	if handle.WorldHash == "" {
		t.Fatal("world hash vazio")
	}
	// O handle deve listar todos os arquivos escritos.
	if len(handle.Files) < 8 {
		t.Fatalf("esperava >=8 arquivos, got %d", len(handle.Files))
	}
}
