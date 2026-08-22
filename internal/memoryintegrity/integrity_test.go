package memoryintegrity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAndVerifyDetectsChange(t *testing.T) {
	root := t.TempDir()
	files := []string{
		".cosca/framework/KERNEL.md", ".cosca/framework/CONSTITUTION.md",
		".cosca/framework/MEMORY_MODEL.md", ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md",
		".cosca/memory/LEARNING_PROTOCOL.md", ".cosca/memory/agent/example/learnings.md",
	}
	for _, name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Write(root, ""); err != nil {
		t.Fatal(err)
	}
	result, err := Verify(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changed) != 0 {
		t.Fatalf("unexpected initial changes: %v", result.Changed)
	}
	if err := os.WriteFile(filepath.Join(root, files[0]), []byte("altered"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = Verify(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changed) != 1 || result.Changed[0] != files[0] {
		t.Fatalf("want changed %q, got %+v", files[0], result.Changed)
	}
	if err := os.Remove(filepath.Join(root, files[1])); err != nil {
		t.Fatal(err)
	}
	result, err = Verify(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Missing) != 1 || result.Missing[0] != files[1] {
		t.Fatalf("want missing %q, got %+v", files[1], result.Missing)
	}
}

func TestInventoryExcludesDatabaseAndAuditManifest(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".cosca/framework/KERNEL.md", ".cosca/framework/CONSTITUTION.md", ".cosca/framework/MEMORY_MODEL.md", ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md", ".cosca/memory/LEARNING_PROTOCOL.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(path, []byte("x"), 0o644)
	}
	if err := os.MkdirAll(filepath.Join(root, ".cosca", "fallback", "memory", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, ".cosca", "fallback", "memory", "agent", "db.sqlite"), []byte("db"), 0o644)
	m, err := Inventory(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Files {
		if filepath.Ext(f.Path) != ".md" {
			t.Fatalf("unexpected non-markdown path: %s", f.Path)
		}
	}
}

func TestWriteRefusesReplacementUnlessForced(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultManifest)
	if _, err := Write(root, ""); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, ""); err == nil {
		t.Fatal("expected replacement to be refused")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("refused replacement changed the manifest")
	}
	if _, err := WriteWithForce(root, ""); err != nil {
		t.Fatalf("forced replacement failed: %v", err)
	}
}

func TestWriteUsesRestrictivePermissions(t *testing.T) {
	// 0700/0600 are POSIX permission bits; chmod is a no-op on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits (0700/0600) are not enforced on Windows")
	}
	root := t.TempDir()
	if _, err := Write(root, ""); err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(filepath.Join(root, ".cosca", "audit"))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("manifest directory mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(root, DefaultManifest))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("manifest mode = %o, want 600", got)
	}
}
