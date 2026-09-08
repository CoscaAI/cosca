// Package trainer inspects the real knowledge.db schema.
package trainer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestInspectKnowledgeDB inspects the real knowledge.db schema.
func TestInspectKnowledgeDB(t *testing.T) {
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..", "..", "..")
	projectRoot, _ = filepath.Abs(projectRoot)

	dbPath := filepath.Join(projectRoot, ".cosca", "knowledge.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("knowledge.db not found")
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// List tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	t.Logf("\n=== TABELAS no knowledge.db ===\n")
	for rows.Next() {
		var name string
		rows.Scan(&name)
		t.Logf("  %s", name)
	}

	// For each table, show schema
	rows2, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}
	defer rows2.Close()

	t.Logf("\n=== SCHEMA ===\n")
	for rows2.Next() {
		var name, sql string
		rows2.Scan(&name, &sql)
		t.Logf("\n--- %s ---\n%s", name, sql)
	}
}