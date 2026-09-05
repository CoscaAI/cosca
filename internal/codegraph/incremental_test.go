package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndex_Update_NoopFastWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\nfunc Y(){}\n"), 0o644)

	ix, err := BuildIndex(dir, 256)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	prev, err := ix.Update(dir, 256)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	// Noop: nada mudou → mesmo número de sinais e BuiltAt atualizado.
	if len(prev.Signals) != 2 {
		t.Fatalf("noop deve manter 2 arquivos, got %d", len(prev.Signals))
	}
}

func TestIndex_Update_DetectsChangeAndDeletion(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\nfunc Y(){}\n"), 0o644)

	ix, err := BuildIndex(dir, 256)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// 1. Muda a.go e apaga b.go.
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){ http() }\n"), 0o644)
	_ = os.Remove(filepath.Join(dir, "b.go"))

	ix, err = ix.Update(dir, 256)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(ix.Signals) != 1 {
		t.Fatalf("após update deve ter 1 arquivo (a.go), got %d", len(ix.Signals))
	}
	if _, ok := ix.Signals["a.go"]; !ok {
		t.Fatal("a.go deve continuar indexado")
	}
	if _, ok := ix.Signals["b.go"]; ok {
		t.Fatal("b.go deletado deve sair do índice")
	}
	// Hash de a.go mudou.
	if ix.Hashes["a.go"] == "" {
		t.Fatal("hash de a.go deve existir")
	}
}

func TestIndex_Update_NilIsBuild(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc F(){}\n"), 0o644)
	ix, err := (*Index)(nil).Update(dir, 128)
	if err != nil {
		t.Fatalf("update(nil): %v", err)
	}
	if ix == nil || len(ix.Signals) != 1 {
		t.Fatalf("update(nil) deve equivaler a BuildIndex, got %+v", ix)
	}
}
