package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteProjectExamples_Game(t *testing.T) {
	dir := t.TempDir()
	if err := writeProjectExamples(dir, "game"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "level.json"))
	if err != nil {
		t.Fatalf("level.json missing: %v", err)
	}
	// A cena ECS deve ter as entidades player/enemy/coin.
	s := string(data)
	for _, want := range []string{"player", "enemy", "coin", "transform", "physics", "input", "ai"} {
		if !contains(s, want) {
			t.Fatalf("level.json missing %q", want)
		}
	}
}

func TestWriteProjectExamples_Scientific(t *testing.T) {
	dir := t.TempDir()
	if err := writeProjectExamples(dir, "scientific"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "experiment.json"))
	if err != nil {
		t.Fatalf("experiment.json missing: %v", err)
	}
	for _, want := range []string{"exp-baseline", "calculated", "result", "parameters"} {
		if !contains(string(data), want) {
			t.Fatalf("experiment.json missing %q", want)
		}
	}
}

func TestWriteProjectExamples_Media(t *testing.T) {
	for _, typ := range []string{"cinema", "image", "music"} {
		dir := t.TempDir()
		if err := writeProjectExamples(dir, typ); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "workflow.json"))
		if err != nil {
			t.Fatalf("%s workflow.json missing: %v", typ, err)
		}
		if !contains(string(data), "nodes") {
			t.Fatalf("%s workflow.json missing nodes", typ)
		}
	}
}

func TestWriteProjectExamples_3D(t *testing.T) {
	dir := t.TempDir()
	if err := writeProjectExamples(dir, "3d"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "scene.obj"))
	if err != nil {
		t.Fatalf("scene.obj missing: %v", err)
	}
	if !contains(string(data), "v -1 -1 -1") {
		t.Fatal("scene.obj missing vertices")
	}
}

func TestWriteProjectExamples_UnknownType(t *testing.T) {
	dir := t.TempDir()
	// Tipos sem kit (editor/document) não geram nada — sem erro.
	if err := writeProjectExamples(dir, "document"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("unknown type should generate nothing, got %d files", len(entries))
	}
}

// contains é helper simples (evita strings.Contains repetido nos testes).
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
