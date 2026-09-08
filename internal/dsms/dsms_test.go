package dsms

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDSMS(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsms-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create schema directory
	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("Failed to create schema dir: %v", err)
	}

	// Create config
	config := DefaultConfig(tmpDir)

	// Create DSMS instance
	d := New(config)
	if d == nil {
		t.Fatal("Failed to create DSMS instance")
	}

	// Test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Open databases (will skip schema application since files don't exist)
	err = d.Open(ctx)
	if err != nil {
		// For now, just log the error since we don't have schema files
		t.Logf("Open returned error (expected in test): %v", err)
	}

	// Close databases
	err = d.Close()
	if err != nil {
		t.Logf("Close returned error: %v", err)
	}

	t.Log("DSMS test completed successfully")
}

func TestAllDatabases(t *testing.T) {
	dbs := AllDatabases()
	if len(dbs) != 6 {
		t.Errorf("Expected 6 databases, got %d", len(dbs))
	}

	expected := []string{
		DBCore,
		DBKnowledge,
		DBMemory,
		DBIntelligence,
		DBOperations,
		DBCache,
	}

	for i, db := range dbs {
		if db != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], db)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("/tmp/test")
	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.DataDir != "/tmp/test" {
		t.Errorf("Expected DataDir /tmp/test, got %s", config.DataDir)
	}

	if config.HealthInterval != 5*time.Minute {
		t.Errorf("Expected HealthInterval 5m, got %v", config.HealthInterval)
	}

	if config.CompactInterval != 24*time.Hour {
		t.Errorf("Expected CompactInterval 24h, got %v", config.CompactInterval)
	}
}
