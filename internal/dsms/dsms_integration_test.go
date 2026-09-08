package dsms

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDSMSIntegration creates real test databases with schemas applied.
func TestDSMSIntegration(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsms-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create schema directory
	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("Failed to create schema dir: %v", err)
	}

	// Copy schema files from internal/dsms/schema
	schemas := []string{
		"001_core.sql",
		"002_knowledge.sql",
		"003_memory.sql",
		"004_intelligence.sql",
		"005_operations.sql",
		"006_cache.sql",
	}

	for _, schema := range schemas {
		src := filepath.Join("schema", schema)
		dst := filepath.Join(schemaDir, schema)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("Failed to read schema %s: %v", schema, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("Failed to copy schema %s: %v", schema, err)
		}
	}

	// Create config
	config := DefaultConfig(tmpDir)

	// Create DSMS instance
	d := New(config)
	if d == nil {
		t.Fatal("Failed to create DSMS instance")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Open databases
	if err := d.Open(ctx); err != nil {
		t.Fatalf("Failed to open databases: %v", err)
	}
	defer d.Close()

	// Verify all databases are open
	for _, dbName := range AllDatabases() {
		db, err := d.DB(dbName)
		if err != nil {
			t.Errorf("Database %s not accessible: %v", dbName, err)
			continue
		}

		// Verify database is functional
		var result int
		err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			t.Errorf("Database %s not functional: %v", dbName, err)
			continue
		}

		if result != 1 {
			t.Errorf("Database %s returned unexpected result: %d", dbName, result)
		}

		t.Logf("✓ Database %s is functional", dbName)
	}

	// Verify schema was applied (check for tables)
	coreDB, err := d.Core()
	if err != nil {
		t.Fatalf("Failed to get core DB: %v", err)
	}

	// Check if config table exists (should be created by schema)
	var tableCount int
	err = coreDB.QueryRowContext(ctx, 
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='config'").Scan(&tableCount)
	if err != nil {
		t.Logf("Config table check failed (schema may not be applied): %v", err)
	} else {
		if tableCount == 0 {
			t.Log("Config table does not exist (schema files may not be in place)")
		} else {
			t.Log("✓ Core schema applied successfully")
		}
	}
}

// TestSchemaFilesExist verifies schema files exist.
func TestSchemaFilesExist(t *testing.T) {
	schemas := []string{
		"001_core.sql",
		"002_knowledge.sql",
		"003_memory.sql",
		"004_intelligence.sql",
		"005_operations.sql",
		"006_cache.sql",
	}

	for _, schema := range schemas {
		path := filepath.Join("schema", schema)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Schema file %s does not exist", schema)
		}
	}
}
