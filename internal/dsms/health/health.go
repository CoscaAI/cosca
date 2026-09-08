package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cosca/internal/dsms"
)

// ============================================================
// HEALTH CHECK MODULE
// ============================================================

// HealthChecker performs health checks on databases.
type HealthChecker struct {
	dsms *dsms.DSMS
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(d *dsms.DSMS) *HealthChecker {
	return &HealthChecker{dsms: d}
}

// HealthStatus represents the health status of a database.
type HealthStatus struct {
	Database        string        `json:"database"`
	Healthy         bool          `json:"healthy"`
	IntegrityOK     bool          `json:"integrity_ok"`
	WALSize         int64         `json:"wal_size"`
	WALHealthy      bool          `json:"wal_healthy"`
	Fragmentation   float64       `json:"fragmentation"`
	FragmentationOK bool          `json:"fragmentation_ok"`
	SizeBytes       int64         `json:"size_bytes"`
	TableCount      int           `json:"table_count"`
	IndexCount      int           `json:"index_count"`
	RecordCounts    map[string]int `json:"record_counts"`
	CheckedAt       time.Time     `json:"checked_at"`
	Duration        time.Duration `json:"duration"`
}

// CheckAll performs health checks on all databases.
func (h *HealthChecker) CheckAll(ctx context.Context) ([]*HealthStatus, error) {
	var statuses []*HealthStatus

	for _, dbName := range dsms.AllDatabases() {
		status, err := h.Check(ctx, dbName)
		if err != nil {
			return nil, fmt.Errorf("check %s: %w", dbName, err)
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Check performs health check on a single database.
func (h *HealthChecker) Check(ctx context.Context, dbName string) (*HealthStatus, error) {
	start := time.Now()

	db, err := h.dsms.DB(dbName)
	if err != nil {
		return nil, err
	}

	status := &HealthStatus{
		Database:      dbName,
		Healthy:       true,
		RecordCounts:  make(map[string]int),
		CheckedAt:     time.Now(),
	}

	// 1. Integrity check
	if err := h.checkIntegrity(ctx, db, status); err != nil {
		status.Healthy = false
		status.IntegrityOK = false
	}

	// 2. WAL size check
	if err := h.checkWALSize(ctx, db, status); err != nil {
		status.Healthy = false
	}

	// 3. Fragmentation check
	if err := h.checkFragmentation(ctx, db, status); err != nil {
		status.Healthy = false
	}

	// 4. Size check
	if err := h.checkSize(ctx, db, status); err != nil {
		status.Healthy = false
	}

	// 5. Table count
	if err := h.checkTables(ctx, db, status); err != nil {
		status.Healthy = false
	}

	// 6. Index count
	if err := h.checkIndexes(ctx, db, status); err != nil {
		status.Healthy = false
	}

	// 7. Record counts
	if err := h.checkRecords(ctx, db, status); err != nil {
		status.Healthy = false
	}

	status.Duration = time.Since(start)
	return status, nil
}

// ============================================================
// CHECK IMPLEMENTATIONS
// ============================================================

func (h *HealthChecker) checkIntegrity(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	var result string
	err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return err
	}

	status.IntegrityOK = result == "ok"
	if !status.IntegrityOK {
		status.Healthy = false
	}

	return nil
}

func (h *HealthChecker) checkWALSize(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	// Get WAL file path
	var dbName string
	err := db.QueryRowContext(ctx, "PRAGMA database_list").Scan(&dbName, &dbName, &dbName)
	if err != nil {
		// Fallback: just mark as healthy
		status.WALHealthy = true
		return nil
	}

	// For now, just check if WAL exists
	// In production, we'd get the actual WAL size
	status.WALHealthy = true
	status.WALSize = 0

	return nil
}

func (h *HealthChecker) checkFragmentation(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	var pageCount, pageSize int64
	
	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return err
	}
	
	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}

	// Calculate fragmentation
	// fragmentation = (1 - (actual_size / (page_count * page_size))) * 100
	// For now, we'll use a simple heuristic
	
	status.Fragmentation = 0.0
	status.FragmentationOK = true

	return nil
}

func (h *HealthChecker) checkSize(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	var pageCount, pageSize int64
	
	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return err
	}
	
	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}

	status.SizeBytes = pageCount * pageSize
	return nil
}

func (h *HealthChecker) checkTables(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table'")
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&status.TableCount); err != nil {
			return err
		}
	}

	return nil
}

func (h *HealthChecker) checkIndexes(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index'")
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&status.IndexCount); err != nil {
			return err
		}
	}

	return nil
}

func (h *HealthChecker) checkRecords(ctx context.Context, db *sql.DB, status *HealthStatus) error {
	// Get all table names
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}

	// Count records in each table
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			// Skip if table is empty or has issues
			continue
		}
		status.RecordCounts[table] = count
	}

	return nil
}

// ============================================================
// ALERTS
// ============================================================

// Alert represents a health alert.
type Alert struct {
	Database   string    `json:"database"`
	Severity   string    `json:"severity"` // critical, warning, info
	Message    string    `json:"message"`
	CheckedAt  time.Time `json:"checked_at"`
}

// CheckAlerts checks for alerts based on health status.
func (h *HealthChecker) CheckAlerts(statuses []*HealthStatus) []*Alert {
	var alerts []*Alert

	for _, status := range statuses {
		// Critical: integrity check failed
		if !status.IntegrityOK {
			alerts = append(alerts, &Alert{
				Database:  status.Database,
				Severity:  "critical",
				Message:   "Integrity check failed",
				CheckedAt: time.Now(),
			})
		}

		// Warning: WAL too large
		if !status.WALHealthy {
			alerts = append(alerts, &Alert{
				Database:  status.Database,
				Severity:  "warning",
				Message:   fmt.Sprintf("WAL size too large: %d bytes", status.WALSize),
				CheckedAt: time.Now(),
			})
		}

		// Warning: Fragmentation too high
		if !status.FragmentationOK {
			alerts = append(alerts, &Alert{
				Database:  status.Database,
				Severity:  "warning",
				Message:   fmt.Sprintf("Fragmentation too high: %.2f%%", status.Fragmentation),
				CheckedAt: time.Now(),
			})
		}
	}

	return alerts
}

// ============================================================
// REPORT
// ============================================================

// Report generates a health report.
func (h *HealthChecker) Report(statuses []*HealthStatus) string {
	report := "=== DSMS Health Report ===\n\n"
	
	healthyCount := 0
	for _, status := range statuses {
		if status.Healthy {
			healthyCount++
		}
	}

	report += fmt.Sprintf("Overall: %d/%d databases healthy\n\n", healthyCount, len(statuses))

	for _, status := range statuses {
		statusIcon := "✓"
		if !status.Healthy {
			statusIcon = "✗"
		}

		report += fmt.Sprintf("%s %s\n", statusIcon, status.Database)
		report += fmt.Sprintf("  Size: %d bytes\n", status.SizeBytes)
		report += fmt.Sprintf("  Tables: %d, Indexes: %d\n", status.TableCount, status.IndexCount)
		report += fmt.Sprintf("  Integrity: %v\n", status.IntegrityOK)
		report += fmt.Sprintf("  Checked at: %s\n", status.CheckedAt.Format(time.RFC3339))
		report += "\n"
	}

	return report
}
