package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryIntegrityInitRefusesExistingManifestWithoutForce(t *testing.T) {
	root := t.TempDir()
	cmd := NewMemoryIntegrityCommand()
	cmd.SetArgs([]string{"init", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd = NewMemoryIntegrityCommand()
	cmd.SetArgs([]string{"init", "--root", root})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want refusal mentioning --force, got %v", err)
	}
}

func TestMemoryIntegrityInitForceReplacesManifest(t *testing.T) {
	root := t.TempDir()
	cmd := NewMemoryIntegrityCommand()
	cmd.SetArgs([]string{"init", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".cosca", "audit", "memory-integrity-manifest.json")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd = NewMemoryIntegrityCommand()
	cmd.SetArgs([]string{"init", "--root", root, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) == "stale" {
		t.Fatalf("force did not replace manifest: err=%v", err)
	}
}
