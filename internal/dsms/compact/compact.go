package compact

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cosca/internal/dsms"
)

// ============================================================
// COMPACT MODULE
// ============================================================

// Compactor performs database compaction and optimization.
type Compactor struct {
	dsms    *dsms.DSMS
	config  *Config
}

// Config holds compaction configuration.
type Config struct {
	FragmentationThreshold float64       // Trigger compaction above this %
	VacuumThreshold        int64         // Min size in bytes to vacuum
	ReindexThreshold       float64       // Trigger reindex above this %
	MaxWALSize             int64         // Max WAL size before checkpoint
}

// DefaultConfig returns default compaction config.
func DefaultConfig() *Config {
	return &Config{
		FragmentationThreshold: 20.0,
		VacuumThreshold:        1024 * 1024, // 1MB
		ReindexThreshold:       30.0,
		MaxWALSize:             100 * 1024 * 1024, // 100MB
	}
}

// CompactResult represents the result of a compaction.
type CompactResult struct {
	Database        string        `json:"database"`
	Vacuumed        bool          `json:"vacuumed"`
	Reindexed       bool          `json:"reindexed"`
	Analyzed        bool          `json:"analyzed"`
	WALCheckpoint   bool          `json:"wal_checkpoint"`
	SizeBefore      int64         `json:"size_before"`
	SizeAfter       int64         `json:"size_after"`
	SizeReduction   int64         `json:"size_reduction"`
	Duration        time.Duration `json:"duration"`
}

// NewCompactor creates a new compactor.
func NewCompactor(d *dsms.DSMS, config *Config) *Compactor {
	if config == nil {
		config = DefaultConfig()
	}
	return &Compactor{
		dsms:   d,
		config: config,
	}
}

// CompactAll compacts all databases.
func (c *Compactor) CompactAll(ctx context.Context) ([]*CompactResult, error) {
	var results []*CompactResult

	for _, dbName := range dsms.AllDatabases() {
		result, err := c.Compact(ctx, dbName)
		if err != nil {
			return nil, fmt.Errorf("compact %s: %w", dbName, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// Compact compacts a single database.
func (c *Compactor) Compact(ctx context.Context, dbName string) (*CompactResult, error) {
	start := time.Now()

	db, err := c.dsms.DB(dbName)
	if err != nil {
		return nil, err
	}

	result := &CompactResult{
		Database: dbName,
	}

	// Get size before
	result.SizeBefore, _ = c.getSize(ctx, db)

	// 1. VACUUM if fragmentation is high
	shouldVacuum, err := c.shouldVacuum(ctx, db)
	if err == nil && shouldVacuum {
		if err := c.vacuum(ctx, db); err != nil {
			// Log error but continue
			fmt.Printf("vacuum %s: %v\n", dbName, err)
		} else {
			result.Vacuumed = true
		}
	}

	// 2. REINDEX if needed
	shouldReindex, err := c.shouldReindex(ctx, db)
	if err == nil && shouldReindex {
		if err := c.reindex(ctx, db); err != nil {
			fmt.Printf("reindex %s: %v\n", dbName, err)
		} else {
			result.Reindexed = true
		}
	}

	// 3. ANALYZE to update statistics
	if err := c.analyze(ctx, db); err != nil {
		fmt.Printf("analyze %s: %v\n", dbName, err)
	} else {
		result.Analyzed = true
	}

	// 4. WAL checkpoint
	shouldCheckpoint, err := c.shouldCheckpoint(ctx, db)
	if err == nil && shouldCheckpoint {
		if err := c.checkpoint(ctx, db); err != nil {
			fmt.Printf("checkpoint %s: %v\n", dbName, err)
		} else {
			result.WALCheckpoint = true
		}
	}

	// Get size after
	result.SizeAfter, _ = c.getSize(ctx, db)
	result.SizeReduction = result.SizeBefore - result.SizeAfter
	result.Duration = time.Since(start)

	return result, nil
}

// ============================================================
// CHECKS
// ============================================================

func (c *Compactor) shouldVacuum(ctx context.Context, db *sql.DB) (bool, error) {
	var pageCount, pageSize int64

	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return false, err
	}

	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return false, err
	}

	totalSize := pageCount * pageSize
	if totalSize < c.config.VacuumThreshold {
		return false, nil
	}

	// Check free pages
	var freePages int64
	err := db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freePages)
	if err != nil {
		return false, err
	}

	fragmentation := float64(freePages) / float64(pageCount) * 100
	return fragmentation > c.config.FragmentationThreshold, nil
}

func (c *Compactor) shouldReindex(ctx context.Context, db *sql.DB) (bool, error) {
	// Check if any index is corrupted or needs rebuild
	var result string
	err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return false, err
	}

	// If integrity check passes, no need to reindex
	if result == "ok" {
		return false, nil
	}

	// If there are index issues, reindex
	return result != "ok", nil
}

func (c *Compactor) shouldCheckpoint(ctx context.Context, db *sql.DB) (bool, error) {
	// WAL checkpoint is needed if WAL is large
	// For now, always checkpoint (safe operation)
	return true, nil
}

// ============================================================
// OPERATIONS
// ============================================================

func (c *Compactor) vacuum(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "VACUUM")
	return err
}

func (c *Compactor) reindex(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "REINDEX")
	return err
}

func (c *Compactor) analyze(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "ANALYZE")
	return err
}

func (c *Compactor) checkpoint(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func (c *Compactor) getSize(ctx context.Context, db *sql.DB) (int64, error) {
	var pageCount, pageSize int64

	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, err
	}

	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, err
	}

	return pageCount * pageSize, nil
}

// ============================================================
// REPORT
// ============================================================

// Report generates a compaction report.
func Report(results []*CompactResult) string {
	report := "=== DSMS Compaction Report ===\n\n"

	totalSaved := int64(0)
	for _, result := range results {
		totalSaved += result.SizeReduction

		vacuumIcon := " "
		if result.Vacuumed {
			vacuumIcon = "V"
		}

		reindexIcon := " "
		if result.Reindexed {
			reindexIcon = "R"
		}

		analyzeIcon := " "
		if result.Analyzed {
			analyzeIcon = "A"
		}

		checkpointIcon := " "
		if result.WALCheckpoint {
			checkpointIcon = "C"
		}

		report += fmt.Sprintf("[%s%s%s%s] %s: %d → %d bytes (saved %d)\n",
			vacuumIcon, reindexIcon, analyzeIcon, checkpointIcon,
			result.Database, result.SizeBefore, result.SizeAfter, result.SizeReduction)
	}

	report += fmt.Sprintf("\nTotal saved: %d bytes\n", totalSaved)

	return report
}
