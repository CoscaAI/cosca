package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CoscaAI/cosca/internal/asset"
)

// TestAssetRegistry_AddListInfo exercita o fluxo completo via Registry
// (camada core) + resolução de root.
func TestAssetRegistry_AddListInfo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cosca"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := asset.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(root, "logo.png")
	if err := os.WriteFile(src, []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := r.AddFile(src, asset.TypeImage, "design/logo.png")
	if err != nil {
		t.Fatal(err)
	}

	// Dedup: mesma imagem adicionada de novo não duplica.
	a2, err := r.AddFile(src, asset.TypeImage, "outra-fonte.png")
	if err != nil {
		t.Fatal(err)
	}
	if a2.ID != a.ID {
		t.Fatalf("dedup failed: %q != %q", a2.ID, a.ID)
	}
	if r.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r.Count())
	}

	// List filtra por tipo.
	all := r.List()
	if len(all) != 1 || !all[0].IsType(asset.TypeImage) {
		t.Fatalf("list mismatch: %+v", all)
	}

	// Info por prefixo curto.
	resolved, err := resolveAssetID(r, a.ID[:12])
	if err != nil {
		t.Fatalf("resolveAssetID prefix: %v", err)
	}
	if resolved != a.ID {
		t.Fatalf("resolved = %q, want %q", resolved, a.ID)
	}
}

func TestFindProjectRoot_WalksUp(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b", ".cosca"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findProjectRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "a", "b")
	if got != want {
		t.Fatalf("findProjectRoot = %q, want %q", got, want)
	}
}

func TestFindProjectRoot_NoRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("walk-up encontra o .cosca do home do usuário (C:\\Users\\<user>\\.cosca) — condição de ambiente, não de código")
	}
	dir := t.TempDir()
	if _, err := findProjectRoot(dir); err == nil {
		t.Fatal("expected error when no .cosca/ found")
	}
}

func TestResolveAssetID_Ambiguous(t *testing.T) {
	root := t.TempDir()
	r, err := asset.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Dois assets com conteúdo diferente — prefixes provavelmente únicos.
	a1, err := r.Add([]byte("conteudo um asset aqui"), asset.TypeText, "")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := r.Add([]byte("conteudo dois bem diferente"), asset.TypeText, "")
	if err != nil {
		t.Fatal(err)
	}

	// Prefixo inexistente.
	if _, err := resolveAssetID(r, "000000000000"); err == nil {
		t.Fatal("expected error for missing prefix")
	}
	// Prefixo curto demais.
	if _, err := resolveAssetID(r, "abc"); err == nil {
		t.Fatal("expected error for short prefix")
	}
	// IDs completos resolvem.
	if id, err := resolveAssetID(r, a1.ID); err != nil || id != a1.ID {
		t.Fatalf("full id resolve failed: id=%q err=%v", id, err)
	}
	if id, err := resolveAssetID(r, a2.ID); err != nil || id != a2.ID {
		t.Fatalf("full id resolve failed: id=%q err=%v", id, err)
	}
}
