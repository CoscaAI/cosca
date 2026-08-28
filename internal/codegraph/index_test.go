package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndex_AndSearchFast(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "http.go"), []byte("package api\nimport \"net/http\"\nfunc HandleRequest(w http.ResponseWriter, r *http.Request) { http.Handle(\"/route\", r) }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "db.go"), []byte("package store\nimport \"database/sql\"\nfunc QueryDatabase(dsn, query string) (*sql.Rows, error) { return sql.Open(\"sqlite\", dsn) }\n"), 0o644)

	ix, err := BuildIndex(dir, 512)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(ix.Signals) != 2 {
		t.Fatalf("esperava 2 arquivos indexados, got %d", len(ix.Signals))
	}

	hits := ix.SearchSimilar("http handle request", 5)
	if len(hits) == 0 {
		t.Fatal("esperava resultados")
	}
	if hits[0].Path != "http.go" {
		t.Fatalf("http.go deveria ser top, got %s", hits[0].Path)
	}
}

func TestBuildIndex_RAMFirst_NoFileReRead(t *testing.T) {
	// O índice é construído uma vez; buscar de novo não deve re-ler (usa sinais
	// em memória). Aqui validamos que o índice funciona após os arquivos-fonte
	// serem REMOVIDOS (só os sinais em memória respondem).
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X() { http() }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\nfunc Y() { db() }\n"), 0o644)

	ix, err := BuildIndex(dir, 256)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Remove os fontes — o índice segue respondendo (RAM-first).
	_ = os.Remove(filepath.Join(dir, "a.go"))
	_ = os.Remove(filepath.Join(dir, "b.go"))

	hits := ix.SearchSimilar("http handler", 5)
	if len(hits) == 0 {
		t.Fatal("índice RAM-first deveria responder sem os arquivos-fonte")
	}
}

func TestIndex_AtomicPublish_FailClosed(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc F() {}\n"), 0o644)
	ix, err := BuildIndex(dir, 128)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	idxPath := filepath.Join(dir, ".cosca-code-index.json")
	if err := ix.Save(idxPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Publish atômico: o .tmp não deve sobrar (rename atômico limpa).
	if _, err := os.Stat(idxPath + ".tmp"); err == nil {
		t.Fatal("após o publish atômico o .tmp não deve existir (fail-closed)")
	}

	got, err := LoadIndex(idxPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || len(got.Signals) != 1 {
		t.Fatalf("load deve restaurar o índice (fail-closed, índice antigo íntegro): %+v", got)
	}
}
