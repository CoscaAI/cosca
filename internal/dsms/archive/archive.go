package archive

import (
	"context"
	"fmt"
	"time"

	"cosca/internal/dsms"
)

// ============================================================
// ARCHIVE MODULE
// ============================================================

// Archiver handles data lifecycle management.
type Archiver struct {
	dsms   *dsms.DSMS
	config *Config
}

// Config holds archive configuration.
type Config struct {
	// Retention periods
	SessionsHotDays   int
	TracesHotDays     int
	FailuresHotDays   int
	DecisionsHotDays  int
	CacheTTLHours     int
	LearningsHotDays  int

	// Archive destinations
	ArchiveDatabase string
}

// DefaultConfig returns default archive config.
func DefaultConfig() *Config {
	return &Config{
		SessionsHotDays:  7,
		TracesHotDays:    30,
		FailuresHotDays:  60,
		DecisionsHotDays: 90,
		CacheTTLHours:    1,
		LearningsHotDays: 30,

		ArchiveDatabase: dsms.DBMemory,
	}
}

// ArchiveResult represents the result of an archive operation.
type ArchiveResult struct {
	Table           string        `json:"table"`
	RecordsArchived int           `json:"records_archived"`
	RecordsDeleted  int           `json:"records_deleted"`
	BytesFreed      int64         `json:"bytes_freed"`
	Duration        time.Duration `json:"duration"`
}

// NewArchiver creates a new archiver.
func NewArchiver(d *dsms.DSMS, config *Config) *Archiver {
	if config == nil {
		config = DefaultConfig()
	}
	return &Archiver{
		dsms:   d,
		config: config,
	}
}

// ArchiveAll runs archive operations on all tables.
func (a *Archiver) ArchiveAll(ctx context.Context) ([]*ArchiveResult, error) {
	var results []*ArchiveResult

	// Archive memory tables
	if result, err := a.archiveSessions(ctx); err == nil {
		results = append(results, result)
	}

	if result, err := a.archiveTraces(ctx); err == nil {
		results = append(results, result)
	}

	if result, err := a.archiveFailures(ctx); err == nil {
		results = append(results, result)
	}

	if result, err := a.archiveDecisions(ctx); err == nil {
		results = append(results, result)
	}

	if result, err := a.archiveLearnings(ctx); err == nil {
		results = append(results, result)
	}

	// Clean cache
	if result, err := a.cleanCache(ctx); err == nil {
		results = append(results, result)
	}

	return results, nil
}

// ============================================================
// ARCHIVE OPERATIONS
// ============================================================

func (a *Archiver) archiveSessions(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "sessions"}

	db, err := a.dsms.DB(dsms.DBMemory)
	if err != nil {
		return nil, err
	}

	// Move old sessions to archive
	cutoff := time.Now().AddDate(0, 0, -a.config.SessionsHotDays)

	// Create archive table if not exists
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sessions_archive AS 
		SELECT * FROM sessions WHERE 1=0
	`)
	if err != nil {
		return nil, err
	}

	// Move old sessions
	rows, err := db.QueryContext(ctx, `
		INSERT INTO sessions_archive 
		SELECT * FROM sessions 
		WHERE ended_at < ? AND status = 'ended'
		RETURNING id
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		result.RecordsArchived++
	}

	// Delete archived sessions
	deleteResult, err := db.ExecContext(ctx, `
		DELETE FROM sessions 
		WHERE ended_at < ? AND status = 'ended'
	`, cutoff)
	if err != nil {
		return nil, err
	}

	deleted, _ := deleteResult.RowsAffected()
	result.RecordsDeleted = int(deleted)
	result.Duration = time.Since(start)

	return result, nil
}

func (a *Archiver) archiveTraces(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "traces"}

	db, err := a.dsms.DB(dsms.DBMemory)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -a.config.TracesHotDays)

	// Create archive table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS traces_archive AS 
		SELECT * FROM traces WHERE 1=0
	`)
	if err != nil {
		return nil, err
	}

	// Move old traces
	_, err = db.ExecContext(ctx, `
		INSERT INTO traces_archive 
		SELECT * FROM traces 
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return nil, err
	}

	// Delete old traces
	deleteResult, err := db.ExecContext(ctx, `
		DELETE FROM traces 
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return nil, err
	}

	deleted, _ := deleteResult.RowsAffected()
	result.RecordsArchived = int(deleted)
	result.RecordsDeleted = int(deleted)
	result.Duration = time.Since(start)

	return result, nil
}

func (a *Archiver) archiveFailures(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "failures"}

	db, err := a.dsms.DB(dsms.DBMemory)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -a.config.FailuresHotDays)

	// Delete old resolved failures
	deleteResult, err := db.ExecContext(ctx, `
		DELETE FROM failures 
		WHERE created_at < ? AND resolved_at IS NOT NULL
	`, cutoff)
	if err != nil {
		return nil, err
	}

	deleted, _ := deleteResult.RowsAffected()
	result.RecordsDeleted = int(deleted)
	result.Duration = time.Since(start)

	return result, nil
}

func (a *Archiver) archiveDecisions(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "decisions"}

	db, err := a.dsms.DB(dsms.DBMemory)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -a.config.DecisionsHotDays)

	// Delete old decisions
	deleteResult, err := db.ExecContext(ctx, `
		DELETE FROM decisions 
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return nil, err
	}

	deleted, _ := deleteResult.RowsAffected()
	result.RecordsDeleted = int(deleted)
	result.Duration = time.Since(start)

	return result, nil
}

func (a *Archiver) archiveLearnings(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "learnings"}

	db, err := a.dsms.DB(dsms.DBMemory)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -a.config.LearningsHotDays)

	// Delete old low-confidence learnings
	deleteResult, err := db.ExecContext(ctx, `
		DELETE FROM learnings 
		WHERE created_at < ? AND confidence < 0.3
	`, cutoff)
	if err != nil {
		return nil, err
	}

	deleted, _ := deleteResult.RowsAffected()
	result.RecordsDeleted = int(deleted)
	result.Duration = time.Since(start)

	return result, nil
}

func (a *Archiver) cleanCache(ctx context.Context) (*ArchiveResult, error) {
	start := time.Now()
	result := &ArchiveResult{Table: "cache"}

	db, err := a.dsms.DB(dsms.DBCache)
	if err != nil {
		return nil, err
	}

	// Clean expired entries from all cache tables
	tables := []string{"search_cache", "llm_cache", "computed_cache", "embedding_cache"}

	for _, table := range tables {
		deleteResult, err := db.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s 
			WHERE expires_at < datetime('now')
		`, table))
		if err != nil {
			continue
		}

		deleted, _ := deleteResult.RowsAffected()
		result.RecordsDeleted += int(deleted)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ============================================================
// REPORT
// ============================================================

// Report generates an archive report.
func Report(results []*ArchiveResult) string {
	report := "=== DSMS Archive Report ===\n\n"

	totalArchived := 0
	totalDeleted := 0

	for _, result := range results {
		totalArchived += result.RecordsArchived
		totalDeleted += result.RecordsDeleted

		if result.RecordsArchived > 0 || result.RecordsDeleted > 0 {
			report += fmt.Sprintf("%s: archived %d, deleted %d\n",
				result.Table, result.RecordsArchived, result.RecordsDeleted)
		}
	}

	report += fmt.Sprintf("\nTotal: archived %d, deleted %d\n", totalArchived, totalDeleted)

	return report
}
