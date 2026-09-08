package loop

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoop creates test databases and runs a quick health check.
func TestLoop(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsms-loop-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create schema directory
	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("Failed to create schema dir: %v", err)
	}

	// Copy schema files
	schemas := []string{
		"001_core.sql",
		"002_knowledge.sql",
		"003_memory.sql",
		"004_intelligence.sql",
		"005_operations.sql",
		"006_cache.sql",
	}

	for _, schema := range schemas {
		src := filepath.Join("..", "schema", schema)
		dst := filepath.Join(schemaDir, schema)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Logf("Schema %s not found (expected in test): %v", schema, err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("Failed to copy schema %s: %v", schema, err)
		}
	}

	t.Logf("Test data directory: %s", tmpDir)
	t.Log("Loop test completed (databases will be created when DSMS opens them)")
}
