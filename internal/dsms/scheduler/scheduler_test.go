package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosca/internal/dsms"
)

// TestSchedulerShortRun verifies the scheduler starts and runs briefly.
func TestSchedulerShortRun(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsms-scheduler-*")
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
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("Failed to copy schema %s: %v", schema, err)
		}
	}

	// Create DSMS instance
	config := dsms.DefaultConfig(tmpDir)
	d := dsms.New(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Open databases
	if err := d.Open(ctx); err != nil {
		t.Fatalf("Failed to open databases: %v", err)
	}
	defer d.Close()

	// Create scheduler with fast intervals for testing
	schedConfig := &Config{
		HealthInterval:  50 * time.Millisecond,
		CompactInterval: 200 * time.Millisecond,
		ArchiveInterval: 300 * time.Millisecond,
		MetricsInterval: 50 * time.Millisecond,
		RepairEnabled:   true,
	}

	s := NewScheduler(d, schedConfig)
	if s == nil {
		t.Fatal("Failed to create scheduler")
	}

	// Start scheduler
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(300 * time.Millisecond)

	// Verify it's running
	if !s.IsRunning() {
		t.Error("Scheduler should be running")
	}

	// Stop scheduler
	s.Stop()

	// Wait for graceful shutdown
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Scheduler stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Scheduler did not stop in time")
	}

	t.Log("✓ Scheduler started and stopped successfully")
}

// TestSchedulerStatus verifies status reporting.
func TestSchedulerStatus(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsms-status-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create DSMS instance
	config := dsms.DefaultConfig(tmpDir)
	d := dsms.New(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Open databases
	if err := d.Open(ctx); err != nil {
		t.Fatalf("Failed to open databases: %v", err)
	}
	defer d.Close()

	// Create scheduler
	s := NewScheduler(d, nil)
	if s == nil {
		t.Fatal("Failed to create scheduler")
	}

	// Check status (not running yet)
	status := s.Status()
	if status == nil {
		t.Fatal("Status returned nil")
	}

	if status.Running {
		t.Error("Scheduler should not be running yet")
	}

	t.Log("✓ Scheduler status check passed")
}
