// Package runtime provides the background daemon for the Cosca platform.
// The daemon manages PID files, signal handling, log rotation, component
// health monitoring with automatic restart, periodic background sync, and
// automatic knowledge DB backups.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CoscaAI/cosca/pkg/cosca"

	"github.com/rs/zerolog"
)

// =============================================================================
// Daemon
// =============================================================================

// Daemon represents the Cosca background daemon process.
// It manages PID files, signal handling, log rotation, component health
// monitoring with automatic restart, and periodic background sync.
type Daemon struct {
	mu      sync.RWMutex
	logger  zerolog.Logger
	runtime *Runtime

	// PID management
	pidPath string
	pidFile *os.File

	// Log management
	logWriter     io.Writer
	logMaxSize    int64 // max bytes before rotation
	logMaxBackups int   // max rotated log files to keep

	// Component health watchdog
	watchdogInterval time.Duration
	watchdogStop     chan struct{}
	watchdogActive   bool

	// Background sync
	syncInterval time.Duration
	syncStop     chan struct{}
	syncFunc     func(context.Context) error

	// Background backup
	backupInterval  time.Duration
	backupStop      chan struct{}
	backupFunc      func(context.Context) error
	backupDir       string
	maxBackups      int

	// Circadian ORC (Operational Rest Cycle) — manutenção não-atendida
	// quando o sistema está ocioso.
	orcInterval time.Duration
	orcStop     chan struct{}
	orcFunc     func(context.Context) error
	shutdownTimeout time.Duration

	// Reload hook
	onReload func() error

	// Signal handling
	sigCh chan os.Signal

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// stopOnce guarantees Stop() is idempotent — calling it twice (or
	// calling Stop before Start) must not double-close the stop channels
	// (audit 2026-08-01: double close panicked).
	stopOnce sync.Once
}

// DaemonConfig configures the daemon.
type DaemonConfig struct {
	// PIDPath is the path to the PID file.
	PIDPath string
	// LogMaxSize is the max log file size in bytes before rotation.
	LogMaxSize int64
	// LogMaxBackups is the max number of rotated log files to keep.
	LogMaxBackups int
	// WatchdogInterval is how often to check component health.
	WatchdogInterval time.Duration
	// SyncInterval is how often to run background sync.
	SyncInterval time.Duration
	// SyncFunc is the function to call for background sync.
	SyncFunc func(context.Context) error
	// BackupInterval is how often to run the automatic knowledge DB backup.
	BackupInterval time.Duration
	// BackupFunc is the function to call for the automatic backup.
	BackupFunc func(context.Context) error
	// BackupDir is the directory holding automatic backup files (used for
	// retention pruning). When empty, retention is disabled.
	BackupDir string
	// MaxBackups is the maximum number of automatic backups to retain.
	// Zero or negative disables retention.
	MaxBackups int
	// ORCInterval is how often the Circadian ORC (Operational Rest Cycle) is
	// evaluated. When zero, the ORC loop is disabled.
	ORCInterval time.Duration
	// ORCFunc is the Circadian ORC maintenance function (unattended
	// housekeeping when the system is idle). Conecta o circadian ao daemon
	// (era órfão — só rodava via `cosca circadian watch` manual).
	ORCFunc func(context.Context) error
	// OnReload is called when SIGHUP is received.
	OnReload func() error
	// ShutdownTimeout bounds the wait for daemon goroutines after cancellation.
	ShutdownTimeout time.Duration
}

// defaultMaxBackups is the fixed number of automatic backups retained when
// MaxBackups is not configured. Kept at 3 to bound disk usage (each backup
// is ~600MB). Mirrored by knowledge.defaultSnapshotKeep.
const defaultMaxBackups = 3

const defaultDaemonShutdownTimeout = 10 * time.Second

// DefaultDaemonConfig returns a default daemon configuration.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		PIDPath:          "./tmp/cosca.pid", // Overridden in practice
		LogMaxSize:       100 * 1024 * 1024, // 100 MB
		LogMaxBackups:    5,
		WatchdogInterval: 30 * time.Second,
		SyncInterval:     5 * time.Minute,
		BackupInterval:   1 * time.Hour,
		MaxBackups:       defaultMaxBackups,
		ShutdownTimeout:  defaultDaemonShutdownTimeout,
	}
}

// NewDaemon creates a new daemon associated with the given runtime.
func NewDaemon(r *Runtime, cfg DaemonConfig) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &Daemon{
		logger:           r.logger.With().Str("component", "daemon").Logger(),
		runtime:          r,
		pidPath:          cfg.PIDPath,
		logMaxSize:       cfg.LogMaxSize,
		logMaxBackups:    cfg.LogMaxBackups,
		watchdogInterval: cfg.WatchdogInterval,
		syncInterval:     cfg.SyncInterval,
		syncFunc:         cfg.SyncFunc,
		backupInterval:   cfg.BackupInterval,
		backupFunc:       cfg.BackupFunc,
		backupDir:        cfg.BackupDir,
		maxBackups:       cfg.MaxBackups,
		orcInterval:      cfg.ORCInterval,
		orcFunc:          cfg.ORCFunc,
		shutdownTimeout:  cfg.ShutdownTimeout,
		onReload:         cfg.OnReload,
		ctx:              ctx,
		cancel:           cancel,
		sigCh:            make(chan os.Signal, 1),
		watchdogStop:     make(chan struct{}),
		syncStop:         make(chan struct{}),
		backupStop:       make(chan struct{}),
		orcStop:          make(chan struct{}),
	}
}

// =============================================================================
// Start / Stop
// =============================================================================

// Start starts the daemon process. It writes the PID file, sets up signal
// handling, starts the health watchdog, and begins the background sync loop.
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.logger.Info().Msg("daemon starting")

	// Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	d.logger.Debug().Str("pid_file", d.pidPath).Msg("pid file written")

	// Signal handling: ONLY SIGHUP for config reload.
	// SIGINT/SIGTERM are handled exclusively by Runtime.HandleSignals()
	// to avoid duplicate handlers causing race conditions (U02 deadlock).
	signal.Notify(d.sigCh, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	// Start health watchdog
	d.watchdogActive = true
	d.wg.Add(1)
	go d.watchdogLoop()

	// Start background sync
	if d.syncFunc != nil {
		d.wg.Add(1)
		go d.syncLoop()
	}

	// Start background backup
	if d.backupFunc != nil {
		d.wg.Add(1)
		go d.backupLoop()
	}

	// Start Circadian ORC (Operational Rest Cycle) — manutenção não-atendida
	// quando o sistema está ocioso. Conecta o circadian ao daemon (antes só
	// rodava via `cosca circadian watch` manual — fio solto da auditoria).
	if d.orcFunc != nil {
		d.wg.Add(1)
		go d.orcLoop()
	}

	d.logger.Info().
		Dur("watchdog_interval", d.watchdogInterval).
		Dur("sync_interval", d.syncInterval).
		Dur("backup_interval", d.backupInterval).
		Bool("orc_enabled", d.orcFunc != nil).
		Msg("daemon started")
	return nil
}

// Stop gracefully stops the daemon process.
//
// CRITICAL FIX (audit 2026-08-01): Stop() closed the stop channels directly,
// so a second call (or Stop before Start) panicked with "close of closed
// channel". The body now runs exactly once via stopOnce.
func (d *Daemon) Stop() error {
	var stopErr error
	d.stopOnce.Do(func() {
		stopErr = d.stopOnceBody()
	})
	return stopErr
}

// stopOnceBody performs the actual stop sequence. The idempotency guard
// serializes concurrent callers, while the worker wait itself stays unlocked.
func (d *Daemon) stopOnceBody() error {
	d.mu.Lock()
	d.logger.Info().Msg("daemon stopping")

	// Stop signal handling
	signal.Stop(d.sigCh)
	d.cancel()

	// Stop watchdog, sync, and backup loops
	close(d.watchdogStop)
	close(d.syncStop)
	close(d.backupStop)
	if d.orcFunc != nil {
		close(d.orcStop)
	}
	d.mu.Unlock()

	// Wait for goroutines to finish
	waitTimeout := d.shutdownTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultDaemonShutdownTimeout
	}
	if !waitForWaitGroup(&d.wg, waitTimeout) {
		d.logger.Warn().
			Dur("timeout", waitTimeout).
			Msg("daemon shutdown wait timed out; returning with goroutines still running")
	}

	// Mark daemon as inactive
	d.mu.Lock()
	d.watchdogActive = false
	d.mu.Unlock()

	// Remove PID file
	if err := d.removePIDFile(); err != nil {
		d.logger.Warn().Err(err).Msg("failed to remove pid file")
	}

	d.logger.Info().Msg("daemon stopped")
	return nil
}

// =============================================================================
// PID File Management
// =============================================================================

// realPID retorna o PID real do processo do host.
// Dentro da jaula bwrap (--unshare-pid), os.Getpid() retorna o PID do
// namespace (ex: 2), não o PID do host. COSCA_JAIL_PID carrega o PID real
// do processo pai (setado por pkg/cosca/jail.go antes do exec).
func realPID() int {
	if v := os.Getenv(cosca.EnvJailPID); v != "" {
		if pid, err := strconv.Atoi(v); err == nil && pid > 1 {
			return pid
		}
	}
	return os.Getpid()
}

// writePIDFile writes the current process ID to the PID file.
func (d *Daemon) writePIDFile() error {
	if d.pidPath == "" {
		return nil
	}

	dir := filepath.Dir(d.pidPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pid directory: %w", err)
	}

	// Check if a PID file already exists with a running process. Only a
	// different, live cosca process indicates a real second daemon. The current
	// daemon's own PID (os.Getpid() inside the jail namespace or realPID() on
	// the host) and any non-cosca process are stale from a previous boot.
	if existingPID, err := d.readExistingPID(); err == nil {
		if existingPIDBlocksStart(existingPID) {
			return fmt.Errorf("daemon already running with PID %d", existingPID)
		}
		d.logger.Warn().Int("stale_pid", existingPID).Msg("removing stale PID file")
	}

	file, err := os.OpenFile(d.pidPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}

	pid := realPID()
	if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
		_ = file.Close()
		return fmt.Errorf("write pid: %w", err)
	}

	d.pidFile = file
	d.logger.Debug().Int("pid", pid).Str("path", d.pidPath).Msg("PID file written")
	return nil
}

// removePIDFile removes the PID file.
func (d *Daemon) removePIDFile() error {
	if d.pidFile != nil {
		_ = d.pidFile.Close()
		d.pidFile = nil
	}
	if d.pidPath != "" {
		if err := os.Remove(d.pidPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove pid file: %w", err)
		}
	}
	return nil
}

// readExistingPID reads the PID from an existing PID file.
func (d *Daemon) readExistingPID() (int, error) {
	data, err := os.ReadFile(d.pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// processRunning checks if a process with the given PID is running.
// The implementation is platform-specific (see daemon_process_unix.go and
// daemon_process_windows.go): Unix uses the signal-0 probe, Windows probes
// process existence via OpenProcess (Process.Signal is a no-op there and
// os.FindProcess returns nil + error for nonexistent PIDs).
func processRunning(pid int) bool {
	return processRunningPlatform(pid)
}

// isCoscaProcess reports whether the given PID belongs to a running cosca
// binary. Inside the bwrap jail, /proc exposes the host procfs, so the
// readlink result is reliable even though os.Getpid() returns a namespace PID.
// PIDs <= 1 are kernel threads / init and are never cosca processes.
func isCoscaProcess(pid int) bool {
	if pid <= 1 {
		return false
	}
	exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return false
	}
	return strings.Contains(filepath.Base(exe), "cosca")
}

// existingPIDBlocksStart reports whether an existing PID file entry belongs to
// a different, live cosca daemon — which means a second daemon is already
// running and startup must be blocked. The current daemon's own PID
// (os.Getpid() inside the jail namespace or realPID() on the host), a dead
// process, a kernel process, or any non-cosca process are all stale entries
// from a previous boot and never block.
func existingPIDBlocksStart(existingPID int) bool {
	if existingPID == os.Getpid() || existingPID == realPID() {
		return false
	}
	if existingPID <= 1 {
		return false
	}
	return processRunning(existingPID) && isCoscaProcess(existingPID)
}

// PID returns the daemon's process ID.
func (d *Daemon) PID() int {
	return realPID()
}

// =============================================================================
// Signal Handling
// =============================================================================

// handleSignals processes SIGHUP for configuration reload.
// SIGINT/SIGTERM are handled exclusively by Runtime.HandleSignals()
// to avoid duplicate handlers and potential deadlocks.
func (d *Daemon) handleSignals() {
	defer d.wg.Done()

	for {
		select {
		case sig := <-d.sigCh:
			d.logger.Info().Str("signal", sig.String()).Msg("daemon received signal")
			switch sig {
			case syscall.SIGHUP:
				d.logger.Info().Msg("SIGHUP received, reloading configuration")
				if d.onReload != nil {
					if err := d.onReload(); err != nil {
						d.logger.Error().Err(err).Msg("reload failed")
					}
				}
				// Re-register signal handler (some systems reset on SIGHUP)
				signal.Notify(d.sigCh, syscall.SIGHUP)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// =============================================================================
// Health Watchdog
// =============================================================================

// watchdogLoop periodically checks component health and restarts failed ones.
func (d *Daemon) watchdogLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.watchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.checkAndRestart()
		case <-d.watchdogStop:
			return
		case <-d.ctx.Done():
			return
		}
	}
}

// checkAndRestart checks all components and restarts unhealthy ones.
func (d *Daemon) checkAndRestart() {
	r := d.runtime
	if r == nil {
		return
	}

	subsystems := r.getSubsystems()
	for _, sub := range subsystems {
		if sub == nil {
			continue
		}

		health := sub.Health()
		r.state.SetComponentStatus(sub.Name(), health, "")
		r.metrics.SetComponentHealth(sub.Name(), health)

		if health == StatusUnhealthy {
			d.logger.Warn().
				Str("component", sub.Name()).
				Msg("component unhealthy, attempting restart")

			r.state.IncrementRestartCount(sub.Name())

			// Attempt restart with timeout
			restartCtx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
			err := sub.Stop(restartCtx)
			if err != nil {
				d.logger.Error().Err(err).
					Str("component", sub.Name()).
					Msg("failed to stop unhealthy component")
				cancel()
				continue
			}
			cancel()

			startCtx, startCancel := context.WithTimeout(d.ctx, 30*time.Second)
			err = sub.Start(startCtx)
			startCancel()
			if err != nil {
				d.logger.Error().Err(err).
					Str("component", sub.Name()).
					Msg("failed to restart unhealthy component")
				continue
			}

			r.state.SetComponentStatus(sub.Name(), StatusHealthy, "restarted")
			r.metrics.SetComponentHealth(sub.Name(), StatusHealthy)
			r.events.Publish(d.ctx, EventSubsystemStarted, sub.Name(), "restarted after failure")

			d.logger.Info().
				Str("component", sub.Name()).
				Msg("component restarted successfully")
		}
	}
}

// =============================================================================
// Background Sync Loop
// =============================================================================

// syncLoop periodically runs the background sync function.
func (d *Daemon) syncLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.syncInterval)
	defer ticker.Stop()

	// Run an initial sync after a short delay
	initialTimer := time.NewTimer(10 * time.Second)
	defer initialTimer.Stop()

	for {
		select {
		case <-initialTimer.C:
			d.runSync()
		case <-ticker.C:
			d.runSync()
		case <-d.syncStop:
			return
		case <-d.ctx.Done():
			return
		}
	}
}

// runSync executes the periodic sync operation.
func (d *Daemon) runSync() {
	if d.syncFunc == nil {
		return
	}

	d.logger.Debug().Msg("background sync starting")

	start := time.Now()
	syncCtx, cancel := context.WithTimeout(d.ctx, d.syncInterval-10*time.Second)
	defer cancel()

	if err := d.syncFunc(syncCtx); err != nil {
		d.logger.Error().Err(err).Dur("elapsed", time.Since(start)).Msg("background sync failed")
		if d.runtime != nil {
			d.runtime.metrics.IncrementError()
		}
		return
	}

	elapsed := time.Since(start)
	d.logger.Debug().Dur("elapsed", elapsed).Msg("background sync completed")

	if d.runtime != nil {
		d.runtime.metrics.IncrementSync()
		d.runtime.metrics.RecordIndexDuration(elapsed)
	}
}

// =============================================================================
// Background Backup Loop
// =============================================================================

// AutoBackupName returns the snapshot name used for automatic backups,
// e.g. "auto-20260731-143050". The timestamp makes the name sort
// lexicographically in chronological order for retention pruning.
func AutoBackupName() string {
	return "auto-" + time.Now().Format("20060102-150405")
}

// backupLoop periodically runs the automatic knowledge DB backup.
func (d *Daemon) backupLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.backupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.runBackup()
		case <-d.backupStop:
			return
		case <-d.ctx.Done():
			return
		}
	}
}

// runBackup executes the automatic backup and prunes expired backups.
// A failure is non-fatal: it is logged and the loop keeps running.
func (d *Daemon) runBackup() {
	if d.backupFunc == nil {
		return
	}

	d.logger.Debug().Msg("background backup starting")

	start := time.Now()
	timeout := d.backupInterval - 10*time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	backupCtx, cancel := context.WithTimeout(d.ctx, timeout)
	defer cancel()

	if err := d.backupFunc(backupCtx); err != nil {
		d.logger.Error().Err(err).Dur("elapsed", time.Since(start)).Msg("background backup failed")
		if d.runtime != nil {
			d.runtime.metrics.IncrementError()
		}
		return
	}

	d.logger.Info().Dur("elapsed", time.Since(start)).Msg("background backup completed")
	d.pruneOldBackups()
}

// orcLoop periodically runs the Circadian ORC maintenance (Operational Rest
// Cycle) — housekeeping that runs when the system is idle. Conecta o circadian
// ao daemon (fio solto da auditoria: antes só rodava via `cosca circadian
// watch` manual).
func (d *Daemon) orcLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.orcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.runORC()
		case <-d.orcStop:
			return
		case <-d.ctx.Done():
			return
		}
	}
}

// runORC executes the Circadian ORC maintenance. A failure is non-fatal: it
// is logged and the loop keeps running (o ORC é housekeeping, nunca derruba o
// daemon).
func (d *Daemon) runORC() {
	if d.orcFunc == nil {
		return
	}

	d.logger.Debug().Msg("circadian ORC maintenance starting")
	start := time.Now()
	ctx, cancel := context.WithTimeout(d.ctx, d.orcInterval)
	defer cancel()

	if err := d.orcFunc(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("circadian ORC maintenance failed (non-fatal)")
		return
	}
	d.logger.Info().Dur("elapsed", time.Since(start)).Msg("circadian ORC maintenance completed")
}

// pruneOldBackups deletes automatic backup files beyond the retention limit,
// keeping only the newest maxBackups files matching the automatic backup
// prefix. Files not matching the automatic prefix are left untouched.
func (d *Daemon) pruneOldBackups() {
	if d.backupDir == "" || d.maxBackups <= 0 {
		return
	}

	matches, err := filepath.Glob(filepath.Join(d.backupDir, "auto-*.db"))
	if err != nil {
		d.logger.Warn().Err(err).Msg("failed to enumerate automatic backups")
		return
	}
	if len(matches) <= d.maxBackups {
		return
	}

	// Automatic backup names embed zero-padded timestamps, so lexicographic
	// order equals chronological order.
	sort.Strings(matches)

	for _, path := range matches[:len(matches)-d.maxBackups] {
		if err := os.Remove(path); err != nil {
			d.logger.Warn().Err(err).Str("backup", path).Msg("failed to remove old automatic backup")
			continue
		}
		d.logger.Info().Str("backup", path).Msg("removed old automatic backup")
	}
}

// =============================================================================
// Log Management
// =============================================================================

// SetLogWriter sets the log writer and manages rotation.
func (d *Daemon) SetLogWriter(w io.Writer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logWriter = w
}

// RotateLogs forces a log rotation check.
func (d *Daemon) RotateLogs() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Log rotation is delegated to the logging infrastructure
	d.logger.Info().Msg("log rotation initiated")
	return nil
}

// =============================================================================
// Status
// =============================================================================

// IsRunning returns whether the daemon is active.
func (d *Daemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.watchdogActive
}

// Status returns daemon status information.
func (d *Daemon) Status() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"pid":               os.Getpid(),
		"pid_file":          d.pidPath,
		"watchdog_active":   d.watchdogActive,
		"watchdog_interval": d.watchdogInterval.String(),
		"sync_interval":     d.syncInterval.String(),
		"backup_interval":   d.backupInterval.String(),
		"backup_dir":        d.backupDir,
		"max_backups":       d.maxBackups,
		"log_max_size":      d.logMaxSize,
		"log_max_backups":   d.logMaxBackups,
	}
}
