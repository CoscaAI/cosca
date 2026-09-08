// Package trainer provides knowledge training.
package trainer

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestLoadChunks tests loading chunks from the real knowledge db.
func TestLoadChunks(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test chunk query directly
	query := `SELECT c.id, c.content, c.section_type, COALESCE(d.doc_type, 'general'), 0.5
		FROM chunks c
		LEFT JOIN documents d ON d.id = c.document_id
		WHERE c.content != ''
		LIMIT 100`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("chunk query failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		var content, sectionType, domain string
		var conf float64
		if err := rows.Scan(&id, &content, &sectionType, &domain, &conf); err != nil {
			t.Logf("scan error: %v", err)
			continue
		}
		count++
		if count <= 3 {
			t.Logf("chunk %s: [%s/%s] %s...", id, domain, sectionType, content[:min(60, len(content))])
		}
	}

	t.Logf("Chunks carregados: %d", count)
	if count == 0 {
		t.Error("No chunks loaded - query problem")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}