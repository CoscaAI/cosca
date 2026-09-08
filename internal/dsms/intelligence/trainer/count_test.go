// Package trainer provides knowledge training.
package trainer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCountKnowledgeEntries counts entries in each knowledge table.
func TestCountKnowledgeEntries(t *testing.T) {
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

	tables := []string{"knowledge_entries", "documents", "chunks", "code_blocks", "entities", "vectors", "symbols"}

	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Logf("%s: erro (%v)", table, err)
			continue
		}
		t.Logf("%s: %d", table, count)
	}
}