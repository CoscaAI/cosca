// Package dsms provides the Database Self-Management System.
// This is an experimental prototype isolated from the main system.
// Created: 2026-09-08 | ADR-046
//
// DSMS is a self-managing database system that runs in an infinite loop.
// It consolidates 40+ fragmented SQLite databases into 5 domain-driven databases
// and maintains their health through automatic compaction, archiving, and optimization.
package dsms

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// ============================================================
// CORE TYPES
// ============================================================

// DSMS is the Database Self-Management System.
type DSMS struct {
	databases map[string]*sql.DB
	config    *Config
	mu        sync.RWMutex
}

// Config holds DSMS configuration.
type Config struct {
	// Database paths
	DataDir string

	// Intervals
	HealthInterval  time.Duration
	CompactInterval time.Duration
	ArchiveInterval time.Duration
	MetricsInterval time.Duration

	// Thresholds
	FragmentationThreshold float64 // Percentage
	WALSizeThreshold       int64   // Bytes
	CacheMaxSize           int64   // Bytes

	// Retention
	MemoryRetention     time.Duration
	OperationsRetention time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig(dataDir string) *Config {
	return &Config{
		DataDir: dataDir,

		HealthInterval:  5 * time.Minute,
		CompactInterval: 24 * time.Hour,
		ArchiveInterval: 7 * 24 * time.Hour,
		MetricsInterval: 5 * time.Minute,

		FragmentationThreshold: 20.0,
		WALSizeThreshold:       100 * 1024 * 1024, // 100MB
		CacheMaxSize:           100 * 1024 * 1024,  // 100MB

		MemoryRetention:     90 * 24 * time.Hour,
		OperationsRetention: 30 * 24 * time.Hour,
	}
}

// ============================================================
// DATABASE NAMES
// ============================================================

const (
	DBCore         = "cosca-core.db"
	DBKnowledge    = "cosca-knowledge.db"
	DBMemory       = "cosca-memory.db"
	DBIntelligence = "cosca-intelligence.db"
	DBOperations   = "cosca-operations.db"
	DBCache        = "cosca-cache.db"
)

// AllDatabases returns list of all database names.
func AllDatabases() []string {
	return []string{
		DBCore,
		DBKnowledge,
		DBMemory,
		DBIntelligence,
		DBOperations,
		DBCache,
	}
}

// ============================================================
// CONSTRUCTOR
// ============================================================

// New creates a new DSMS instance.
func New(config *Config) *DSMS {
	return &DSMS{
		databases: make(map[string]*sql.DB),
		config:    config,
	}
}

// ============================================================
// LIFECYCLE
// ============================================================

// Open opens all databases.
func (d *DSMS) Open(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create data directory if not exists
	if err := os.MkdirAll(d.config.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	for _, dbName := range AllDatabases() {
		dbPath := filepath.Join(d.config.DataDir, dbName)

		// Open database
		db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
		if err != nil {
			return fmt.Errorf("open %s: %w", dbName, err)
		}

		// Apply schema
		if err := d.applySchema(db, dbName); err != nil {
			db.Close()
			return fmt.Errorf("apply schema to %s: %w", dbName, err)
		}

		d.databases[dbName] = db
	}

	return nil
}

// Close closes all databases.
func (d *DSMS) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for dbName, db := range d.databases {
		if err := db.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dbName, err)
		}
	}

	return nil
}

// ============================================================
// SCHEMA APPLICATION
// ============================================================

func (d *DSMS) applySchema(db *sql.DB, dbName string) error {
	// Map database name to schema file
	schemaMap := map[string]string{
		DBCore:         "001_core.sql",
		DBKnowledge:    "002_knowledge.sql",
		DBMemory:       "003_memory.sql",
		DBIntelligence: "004_intelligence.sql",
		DBOperations:   "005_operations.sql",
		DBCache:        "006_cache.sql",
	}

	schemaFile, ok := schemaMap[dbName]
	if !ok {
		return fmt.Errorf("unknown database: %s", dbName)
	}

	// Read schema file
	schemaPath := filepath.Join(d.config.DataDir, "schema", schemaFile)
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		// If schema file doesn't exist, skip (for prototype)
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read schema: %w", err)
	}

	// Execute schema
	if _, err := db.Exec(string(schemaBytes)); err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	return nil
}

// ============================================================
// DATABASE ACCESS
// ============================================================

// DB returns a database connection by name.
func (d *DSMS) DB(name string) (*sql.DB, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db, ok := d.databases[name]
	if !ok {
		return nil, fmt.Errorf("database not found: %s", name)
	}

	return db, nil
}

// Core returns the core database.
func (d *DSMS) Core() (*sql.DB, error) {
	return d.DB(DBCore)
}

// Knowledge returns the knowledge database.
func (d *DSMS) Knowledge() (*sql.DB, error) {
	return d.DB(DBKnowledge)
}

// Memory returns the memory database.
func (d *DSMS) Memory() (*sql.DB, error) {
	return d.DB(DBMemory)
}

// Intelligence returns the intelligence database.
func (d *DSMS) Intelligence() (*sql.DB, error) {
	return d.DB(DBIntelligence)
}

// Operations returns the operations database.
func (d *DSMS) Operations() (*sql.DB, error) {
	return d.DB(DBOperations)
}

// Cache returns the cache database.
func (d *DSMS) Cache() (*sql.DB, error) {
	return d.DB(DBCache)
}

// Config returns the DSMS configuration.
func (d *DSMS) Config() *Config {
	return d.config
}

// ============================================================
// MAIN ENTRY POINT
// ============================================================

// Main is the entry point for the DSMS binary.
func Main() {
	// Get data directory from environment or use default
	dataDir := os.Getenv("DSMS_DATA_DIR")
	if dataDir == "" {
		// Use current directory
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			os.Exit(1)
		}
		dataDir = filepath.Join(wd, "dsms-data")
	}

	// Create config
	config := DefaultConfig(dataDir)

	// Create DSMS instance
	d := New(config)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nInterrupted, shutting down...")
		cancel()
	}()

	// Open databases
	fmt.Println("=== DSMS (Database Self-Management System) ===")
	fmt.Printf("Data directory: %s\n", dataDir)
	fmt.Printf("Started at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	fmt.Println("[1/2] Opening databases...")
	if err := d.Open(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening databases: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()
	fmt.Println("  ✓ All databases opened")

	fmt.Println("[2/2] Ready for scheduler...")
	fmt.Println()
	fmt.Println("DSMS core initialized. Use scheduler package for infinite loop.")
	fmt.Println()

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Println("Shutting down...")
}
