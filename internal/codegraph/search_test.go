package codegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/graph"
)

// buildTestGraph constrói um grafo com dois arquivos Go (um sobre HTTP, outro
// sobre DB) para testar a busca semântica determinística.
func buildTestGraph(t *testing.T) (*graph.Graph, string) {
	t.Helper()
	dir := t.TempDir()
	httpFile := `package api

import "fmt"
func HandleRequest(w, r) {
    fmt.Println("route", r.Path)
}
`
	dbFile := `package store

import "database/sql"
func Connect(dsn string) (*sql.DB, error) {
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

	// Consulta sobre HTTP/rota → http.go deve vir primeiro.
	hits, err := SearchSimilar(g, dir, "http route handler", 5, 512)
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
	hits, err := SearchSimilar(g, dir, "sqlite database connection", 2, 512)
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
