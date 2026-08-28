package codegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/graph"
)

// buildTestGraph constrói um grafo com dois arquivos Go (um sobre HTTP, outro
// sobre DB) com sobreposição LEXICAL real — é o que o embedding determinístico
// (v1) mede. Não esperamos generalização semântica (handler → handle) do modelo.
func buildTestGraph(t *testing.T) (*graph.Graph, string) {
	t.Helper()
	dir := t.TempDir()
	httpFile := `package api

import "net/http"

func HandleRequest(w http.ResponseWriter, r *http.Request) {
    http.Handle("/route", r)
}
`
	dbFile := `package store

import "database/sql"

func QueryDatabase(dsn, query string) (*sql.Rows, error) {
    return sql.Open("sqlite", dsn)
}
`
	if err := os.WriteFile(filepath.Join(dir, "http.go"), []byte(httpFile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "db.go"), []byte(dbFile), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return g, dir
}

func TestSearchSimilar_PrefersSemanticMatch(t *testing.T) {
	g, dir := buildTestGraph(t)

	// Consulta com tokens em http.go (http/handle/request) → http.go primeiro.
	hits, err := SearchSimilar(g, dir, "http handle request", 5, 512)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("esperava resultados")
	}
	if hits[0].Path != "http.go" {
		t.Fatalf("http.go deveria ser o top para query de http, got %s", hits[0].Path)
	}
}

func TestSearchSimilar_RespectsLimitAndOrders(t *testing.T) {
	g, dir := buildTestGraph(t)
	hits, err := SearchSimilar(g, dir, "sqlite database query", 2, 512)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) > 2 {
		t.Fatalf("limit respeitado: %d", len(hits))
	}
	// Para consulta de DB, db.go deve vir primeiro.
	if len(hits) > 0 && hits[0].Path != "db.go" {
		t.Fatalf("db.go deveria ser top para query de db, got %s", hits[0].Path)
	}
}
