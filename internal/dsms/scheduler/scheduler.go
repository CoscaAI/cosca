package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cosca/internal/dsms"
	"cosca/internal/dsms/archive"
	"cosca/internal/dsms/compact"
	"cosca/internal/dsms/health"
	"cosca/internal/dsms/metrics"
)

// ============================================================
// SCHEDULER MODULE (INFINITE LOOP)
// ============================================================

// Scheduler runs DSMS tasks in an infinite loop.
type Scheduler struct {
	dsms     *dsms.DSMS
	config   *Config
	health   *health.HealthChecker
	compact  *compact.Compactor
	archive  *archive.Archiver
	metrics  *metrics.Collector
	running  bool
	cancel   context.CancelFunc
	mu       sync.Mutex
}

// Config holds scheduler configuration.
type Config struct {
	HealthInterval  time.Duration
	CompactInterval time.Duration
	ArchiveInterval time.Duration
	MetricsInterval time.Duration
	RepairEnabled   bool
}

// DefaultConfig returns default scheduler config.
func DefaultConfig() *Config {
	return &Config{
		HealthInterval:  5 * time.Minute,
		CompactInterval: 24 * time.Hour,
		ArchiveInterval: 7 * 24 * time.Hour,
		MetricsInterval: 5 * time.Minute,
		RepairEnabled:   true,
	}
}

// TaskStatus represents the status of a scheduled task.
type TaskStatus struct {
	Name      string        `json:"name"`
	Running   bool          `json:"running"`
	LastRun   time.Time     `json:"last_run"`
	NextRun   time.Time     `json:"next_run"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"error,omitempty"`
}

// SchedulerStatus represents the overall scheduler status.
type SchedulerStatus struct {
	Running    bool                      `json:"running"`
	Tasks      map[string]*TaskStatus    `json:"tasks"`
	StartedAt  time.Time                 `json:"started_at"`
	Uptime     time.Duration             `json:"uptime"`
}

// ============================================================
// CONSTRUCTOR
// ============================================================

// NewScheduler creates a new scheduler.
func NewScheduler(d *dsms.DSMS, config *Config) *Scheduler {
	if config == nil {
		config = DefaultConfig()
	}

	return &Scheduler{
		dsms:    d,
		config:  config,
		health:  health.NewHealthChecker(d),
		compact: compact.NewCompactor(d, nil),
		archive: archive.NewArchiver(d, nil),
		metrics: metrics.NewCollector(d, nil),
	}
}

// ============================================================
// LIFECYCLE
// ============================================================

// Start starts the scheduler in an infinite loop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}

	s.running = true
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	fmt.Println("[DSMS] Scheduler started")

	// Run initial health check
	s.runHealthCheck(ctx)

	// Start loops
	go s.healthLoop(ctx)
	go s.compactLoop(ctx)
	go s.archiveLoop(ctx)
	go s.metricsLoop(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	fmt.Println("[DSMS] Scheduler stopped")
	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// ============================================================
// LOOPS
// ============================================================

func (s *Scheduler) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runHealthCheck(ctx)
		}
	}
}

func (s *Scheduler) compactLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.CompactInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runCompact(ctx)
		}
	}
}

func (s *Scheduler) archiveLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.ArchiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runArchive(ctx)
		}
	}
}

func (s *Scheduler) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runMetrics(ctx)
		}
	}
}

// ============================================================
// TASK EXECUTION
// ============================================================

func (s *Scheduler) runHealthCheck(ctx context.Context) {
	start := time.Now()
	fmt.Println("[DSMS] Running health check...")

	statuses, err := s.health.CheckAll(ctx)
	if err != nil {
		fmt.Printf("[DSMS] Health check error: %v\n", err)
		return
	}

	// Check for alerts
	alerts := s.health.CheckAlerts(statuses)
	if len(alerts) > 0 {
		for _, alert := range alerts {
			fmt.Printf("[DSMS] ALERT [%s] %s: %s\n", alert.Severity, alert.Database, alert.Message)
		}
	}

	// Generate report
	report := s.health.Report(statuses)
	fmt.Print(report)

	fmt.Printf("[DSMS] Health check completed in %v\n", time.Since(start))
}

func (s *Scheduler) runCompact(ctx context.Context) {
	start := time.Now()
	fmt.Println("[DSMS] Running compaction...")

	results, err := s.compact.CompactAll(ctx)
	if err != nil {
		fmt.Printf("[DSMS] Compaction error: %v\n", err)
		return
	}

	// Generate report
	report := compact.Report(results)
	fmt.Print(report)

	fmt.Printf("[DSMS] Compaction completed in %v\n", time.Since(start))
}

func (s *Scheduler) runArchive(ctx context.Context) {
	start := time.Now()
	fmt.Println("[DSMS] Running archive...")

	results, err := s.archive.ArchiveAll(ctx)
	if err != nil {
		fmt.Printf("[DSMS] Archive error: %v\n", err)
		return
	}

	// Generate report
	report := archive.Report(results)
	fmt.Print(report)

	fmt.Printf("[DSMS] Archive completed in %v\n", time.Since(start))
}

func (s *Scheduler) runMetrics(ctx context.Context) {
	start := time.Now()
	fmt.Println("[DSMS] Collecting metrics...")

	systemMetrics, err := s.metrics.CollectAll(ctx)
	if err != nil {
		fmt.Printf("[DSMS] Metrics error: %v\n", err)
		return
	}

	// Generate report
	report := metrics.Report(systemMetrics)
	fmt.Print(report)

	fmt.Printf("[DSMS] Metrics collection completed in %v\n", time.Since(start))
}

// ============================================================
// STATUS
// ============================================================

// Status returns the scheduler status.
func (s *Scheduler) Status() *SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &SchedulerStatus{
		Running: s.running,
		Tasks:   make(map[string]*TaskStatus),
	}
}
