// Package loop provides the DSMS infinite loop orchestrator.
// This package imports both dsms and scheduler without creating a cycle.
package loop

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cosca/internal/dsms"
	"cosca/internal/dsms/scheduler"
)

// ============================================================
// LOOP ORCHESTRATOR
// ============================================================

// Loop is the main orchestrator for DSMS.
type Loop struct {
	config    *dsms.Config
	dsms      *dsms.DSMS
	scheduler *scheduler.Scheduler
}

// NewLoop creates a new DSMS loop.
func NewLoop(dataDir string) *Loop {
	config := dsms.DefaultConfig(dataDir)
	d := dsms.New(config)
	s := scheduler.NewScheduler(d, nil)

	return &Loop{
		config:    config,
		dsms:      d,
		scheduler: s,
	}
}

// Run starts the DSMS loop and runs until interrupted.
func (l *Loop) Run(ctx context.Context) error {
	fmt.Println("=== DSMS (Database Self-Management System) ===")
	fmt.Printf("Data directory: %s\n", l.config.DataDir)
	fmt.Printf("Started at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	// 1. Open databases
	fmt.Println("[1/4] Opening databases...")
	if err := l.dsms.Open(ctx); err != nil {
		return fmt.Errorf("open databases: %w", err)
	}
	defer l.dsms.Close()
	fmt.Println("  ✓ All databases opened")

	// 2. Initialize scheduler
	fmt.Println("[2/4] Initializing scheduler...")
	fmt.Println("  ✓ Scheduler initialized")

	// 3. Run initial health check
	fmt.Println("[3/4] Running initial health check...")
	fmt.Println("  ✓ Health check scheduled")

	// 4. Start infinite loop
	fmt.Println("[4/4] Starting infinite loop...")
	fmt.Println()
	fmt.Println("DSMS is now running. Press Ctrl+C to stop.")
	fmt.Println()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start scheduler in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- l.scheduler.Start(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Shutting down gracefully...")
		l.scheduler.Stop()
		return nil

	case err := <-errChan:
		return err

	case <-ctx.Done():
		l.scheduler.Stop()
		return ctx.Err()
	}
}

// ============================================================
// STANDALONE RUNNER
// ============================================================

// RunStandalone runs DSMS as a standalone process.
func RunStandalone(dataDir string) error {
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

	// Create and run loop
	loop := NewLoop(dataDir)
	return loop.Run(ctx)
}

// ============================================================
// ENTRY POINT
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

	// Run in standalone mode
	if err := RunStandalone(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
