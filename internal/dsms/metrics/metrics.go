package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"cosca/internal/dsms"
)

// ============================================================
// METRICS MODULE
// ============================================================

// Collector collects database metrics.
type Collector struct {
	dsms   *dsms.DSMS
	config *Config
	cache  *MetricsCache
	mu     sync.RWMutex
}

// Config holds metrics configuration.
type Config struct {
	CollectionInterval time.Duration
	RetentionPeriod    time.Duration
	EnablePrometheus   bool
}

// DefaultConfig returns default metrics config.
func DefaultConfig() *Config {
	return &Config{
		CollectionInterval: 5 * time.Minute,
		RetentionPeriod:    24 * time.Hour,
		EnablePrometheus:   false,
	}
}

// MetricsCache caches metrics for quick access.
type MetricsCache struct {
	mu      sync.RWMutex
	metrics map[string]*DatabaseMetrics
	updated time.Time
}

// NewMetricsCache creates a new metrics cache.
func NewMetricsCache() *MetricsCache {
	return &MetricsCache{
		metrics: make(map[string]*DatabaseMetrics),
	}
}

// Get retrieves metrics from cache.
func (c *MetricsCache) Get(dbName string) (*DatabaseMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m, ok := c.metrics[dbName]
	return m, ok
}

// Set stores metrics in cache.
func (c *MetricsCache) Set(dbName string, m *DatabaseMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics[dbName] = m
	c.updated = time.Now()
}

// ============================================================
// METRIC TYPES
// ============================================================

// DatabaseMetrics represents metrics for a single database.
type DatabaseMetrics struct {
	Database      string            `json:"database"`
	SizeBytes     int64             `json:"size_bytes"`
	WALSizeBytes  int64             `json:"wal_size_bytes"`
	TableCount    int               `json:"table_count"`
	IndexCount    int               `json:"index_count"`
	RecordCounts  map[string]int    `json:"record_counts"`
	Fragmentation float64           `json:"fragmentation"`
	HealthScore   float64           `json:"health_score"`
	LastCompacted time.Time         `json:"last_compacted"`
	CollectedAt   time.Time         `json:"collected_at"`
}

// SystemMetrics represents overall system metrics.
type SystemMetrics struct {
	TotalSizeBytes   int64                      `json:"total_size_bytes"`
	TotalTables      int                        `json:"total_tables"`
	TotalIndexes     int                        `json:"total_indexes"`
	TotalRecords     int                        `json:"total_records"`
	DatabaseCount    int                        `json:"database_count"`
	AvgHealthScore   float64                    `json:"avg_health_score"`
	Databases        map[string]*DatabaseMetrics `json:"databases"`
	CollectedAt      time.Time                  `json:"collected_at"`
}

// CacheMetrics represents cache-specific metrics.
type CacheMetrics struct {
	TotalEntries    int     `json:"total_entries"`
	ValidEntries    int     `json:"valid_entries"`
	ExpiredEntries  int     `json:"expired_entries"`
	SizeBytes       int64   `json:"size_bytes"`
	HitRate         float64 `json:"hit_rate"`
	AvgHitsPerEntry float64 `json:"avg_hits_per_entry"`
}

// ============================================================
// CONSTRUCTOR
// ============================================================

// NewCollector creates a new metrics collector.
func NewCollector(d *dsms.DSMS, config *Config) *Collector {
	if config == nil {
		config = DefaultConfig()
	}
	return &Collector{
		dsms:   d,
		config: config,
		cache:  NewMetricsCache(),
	}
}

// ============================================================
// COLLECTION
// ============================================================

// CollectAll collects metrics from all databases.
func (c *Collector) CollectAll(ctx context.Context) (*SystemMetrics, error) {
	system := &SystemMetrics{
		Databases:   make(map[string]*DatabaseMetrics),
		CollectedAt: time.Now(),
	}

	for _, dbName := range dsms.AllDatabases() {
		metrics, err := c.Collect(ctx, dbName)
		if err != nil {
			return nil, fmt.Errorf("collect %s: %w", dbName, err)
		}

		system.Databases[dbName] = metrics
		system.TotalSizeBytes += metrics.SizeBytes
		system.TotalTables += metrics.TableCount
		system.TotalIndexes += metrics.IndexCount
		
		for _, count := range metrics.RecordCounts {
			system.TotalRecords += count
		}
	}

	system.DatabaseCount = len(system.Databases)
	
	// Calculate average health score
	totalHealth := 0.0
	for _, m := range system.Databases {
		totalHealth += m.HealthScore
	}
	if system.DatabaseCount > 0 {
		system.AvgHealthScore = totalHealth / float64(system.DatabaseCount)
	}

	return system, nil
}

// Collect collects metrics from a single database.
func (c *Collector) Collect(ctx context.Context, dbName string) (*DatabaseMetrics, error) {
	db, err := c.dsms.DB(dbName)
	if err != nil {
		return nil, err
	}

	metrics := &DatabaseMetrics{
		Database:      dbName,
		RecordCounts:  make(map[string]int),
		CollectedAt:   time.Now(),
	}

	// 1. Get size
	if err := c.getSize(ctx, db, metrics); err != nil {
		return nil, err
	}

	// 2. Get WAL size
	if err := c.getWALSize(ctx, db, metrics); err != nil {
		// Non-critical, continue
	}

	// 3. Get table count
	if err := c.getTableCount(ctx, db, metrics); err != nil {
		return nil, err
	}

	// 4. Get index count
	if err := c.getIndexCount(ctx, db, metrics); err != nil {
		return nil, err
	}

	// 5. Get record counts
	if err := c.getRecordCounts(ctx, db, metrics); err != nil {
		// Non-critical, continue
	}

	// 6. Calculate health score
	metrics.HealthScore = c.calculateHealthScore(metrics)

	// Cache metrics
	c.cache.Set(dbName, metrics)

	return metrics, nil
}

// ============================================================
// METRIC COLLECTION HELPERS
// ============================================================

func (c *Collector) getSize(ctx context.Context, db *sql.DB, metrics *DatabaseMetrics) error {
	var pageCount, pageSize int64

	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return err
	}

	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}

	metrics.SizeBytes = pageCount * pageSize
	return nil
}

func (c *Collector) getWALSize(ctx context.Context, db *sql.DB, metrics *DatabaseMetrics) error {
	// WAL size is harder to get programmatically
	// For now, set to 0
	metrics.WALSizeBytes = 0
	return nil
}

func (c *Collector) getTableCount(ctx context.Context, db *sql.DB, metrics *DatabaseMetrics) error {
	return db.QueryRowContext(ctx, 
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&metrics.TableCount)
}

func (c *Collector) getIndexCount(ctx context.Context, db *sql.DB, metrics *DatabaseMetrics) error {
	return db.QueryRowContext(ctx, 
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index'").Scan(&metrics.IndexCount)
}

func (c *Collector) getRecordCounts(ctx context.Context, db *sql.DB, metrics *DatabaseMetrics) error {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		var count int
		err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
		if err != nil {
			continue
		}

		metrics.RecordCounts[tableName] = count
	}

	return nil
}

// ============================================================
// HEALTH SCORE
// ============================================================

func (c *Collector) calculateHealthScore(metrics *DatabaseMetrics) float64 {
	score := 100.0

	// Deduct for fragmentation
	if metrics.Fragmentation > 20 {
		score -= 20
	} else if metrics.Fragmentation > 10 {
		score -= 10
	}

	// Deduct for WAL size
	if metrics.WALSizeBytes > 100*1024*1024 { // 100MB
		score -= 15
	} else if metrics.WALSizeBytes > 50*1024*1024 { // 50MB
		score -= 5
	}

	// Deduct for size (relative to expected)
	// This is a simplified heuristic
	if metrics.SizeBytes > 1*1024*1024*1024 { // 1GB
		score -= 10
	}

	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// ============================================================
// CACHE METRICS
// ============================================================

// GetCacheMetrics retrieves cache metrics.
func (c *Collector) GetCacheMetrics(ctx context.Context) (*CacheMetrics, error) {
	db, err := c.dsms.DB(dsms.DBCache)
	if err != nil {
		return nil, err
	}

	cache := &CacheMetrics{}

	// Get total entries
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT query_hash FROM search_cache
			UNION ALL
			SELECT prompt_hash FROM llm_cache
			UNION ALL
			SELECT key FROM computed_cache
			UNION ALL
			SELECT text_hash FROM embedding_cache
		)
	`).Scan(&cache.TotalEntries)
	if err != nil {
		return nil, err
	}

	// Get valid entries
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT query_hash FROM search_cache WHERE expires_at > datetime('now')
			UNION ALL
			SELECT prompt_hash FROM llm_cache WHERE expires_at > datetime('now')
			UNION ALL
			SELECT key FROM computed_cache WHERE expires_at > datetime('now')
			UNION ALL
			SELECT text_hash FROM embedding_cache WHERE expires_at > datetime('now')
		)
	`).Scan(&cache.ValidEntries)
	if err != nil {
		return nil, err
	}

	cache.ExpiredEntries = cache.TotalEntries - cache.ValidEntries

	// Get hit rate (simplified)
	cache.HitRate = 0.0
	cache.AvgHitsPerEntry = 0.0

	return cache, nil
}

// ============================================================
// REPORT
// ============================================================

// Report generates a metrics report.
func Report(system *SystemMetrics) string {
	report := "=== DSMS Metrics Report ===\n\n"

	report += fmt.Sprintf("Total Size: %d bytes (%.2f MB)\n", 
		system.TotalSizeBytes, float64(system.TotalSizeBytes)/1024/1024)
	report += fmt.Sprintf("Databases: %d\n", system.DatabaseCount)
	report += fmt.Sprintf("Tables: %d\n", system.TotalTables)
	report += fmt.Sprintf("Indexes: %d\n", system.TotalIndexes)
	report += fmt.Sprintf("Records: %d\n", system.TotalRecords)
	report += fmt.Sprintf("Avg Health Score: %.2f\n\n", system.AvgHealthScore)

	for dbName, m := range system.Databases {
		statusIcon := "✓"
		if m.HealthScore < 80 {
			statusIcon = "!"
		}
		if m.HealthScore < 60 {
			statusIcon = "✗"
		}

		report += fmt.Sprintf("[%s] %s: %.2f MB, %d tables, %d records\n",
			statusIcon, dbName, float64(m.SizeBytes)/1024/1024,
			m.TableCount, len(m.RecordCounts))
	}

	report += fmt.Sprintf("\nCollected at: %s\n", system.CollectedAt.Format(time.RFC3339))

	return report
}
