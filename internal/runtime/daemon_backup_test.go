package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Backup configuration
// =============================================================================

func TestDefaultDaemonConfig_BackupDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultDaemonConfig()
	if cfg.BackupInterval != 1*time.Hour {
		t.Errorf("BackupInterval = %v, want %v", cfg.BackupInterval, 1*time.Hour)
	}
	if cfg.MaxBackups != defaultMaxBackups {
		t.Errorf("MaxBackups = %d, want %d", cfg.MaxBackups, defaultMaxBackups)
	}
}

func TestAutoBackupName(t *testing.T) {
	t.Parallel()
	name := AutoBackupName()
	if len(name) < len("auto-20060102-150405") {
		t.Fatalf("AutoBackupName() = %q, too short", name)
	}
	if name[:5] != "auto-" {
		t.Errorf("AutoBackupName() = %q, want prefix %q", name, "auto-")
	}
	if _, err := time.Parse("20060102-150405", name[5:]); err != nil {
		t.Errorf("AutoBackupName() timestamp %q not parseable: %v", name[5:], err)
	}
}

// =============================================================================
// runBackup
// =============================================================================

func TestRunBackupNoBackupFunc(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupFunc = nil

	// Should not panic
	d.runBackup()
}

func TestRunBackupSuccess(t *testing.T) {
	r := New()
	backupCalled := make(chan struct{})
	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupFunc = func(_ context.Context) error {
		close(backupCalled)
		return nil
	}

	d.runBackup()

	select {
	case <-backupCalled:
		// Success
	case <-time.After(time.Second):
		t.Error("backup function was not called")
	}

	if r.metrics.ErrorCount() != 0 {
		t.Errorf("ErrorCount = %d, want 0 on success", r.metrics.ErrorCount())
	}
}

func TestRunBackupFailure(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupFunc = func(_ context.Context) error {
		return errors.New("backup failed")
	}

	d.runBackup()

	// Error counter should be incremented, and the loop must survive.
	if r.metrics.ErrorCount() != 1 {
		t.Errorf("ErrorCount = %d, want 1", r.metrics.ErrorCount())
	}
}

func TestRunBackupSuccessPrunesOld(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 10)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupDir = backupDir
	d.maxBackups = 3
	d.backupFunc = func(_ context.Context) error { return nil }

	d.runBackup()

	remaining := listFiles(t, backupDir, "auto-*.db")
	if len(remaining) != 3 {
		t.Fatalf("remaining auto backups = %d, want 3", len(remaining))
	}
	// Newest (lexicographically greatest) must be kept.
	if remaining[0] != "auto-00000008.db" {
		t.Errorf("kept %q, want oldest kept auto-00000008.db", remaining[0])
	}
	if remaining[len(remaining)-1] != "auto-00000010.db" {
		t.Errorf("kept %q, want newest auto-00000010.db", remaining[len(remaining)-1])
	}
}

func TestRunBackupFailureNoPrune(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 10)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupDir = backupDir
	d.maxBackups = 3
	d.backupFunc = func(_ context.Context) error {
		return errors.New("backup failed")
	}

	d.runBackup()

	if got := len(listFiles(t, backupDir, "auto-*.db")); got != 10 {
		t.Errorf("auto backups after failed backup = %d, want 10 (no pruning)", got)
	}
}

// TestDaemonStart_WithBackupFunc verifies the application wiring contract: a
// daemon registered via RegisterDaemon with a BackupFunc executes the backup
// function while the daemon runs. This mirrors how serve.go wires the
// knowledge engine snapshot to the daemon's automatic backup loop.
func TestDaemonStart_WithBackupFunc(t *testing.T) {
	r := newTestRuntime()

	backupCalled := make(chan struct{}, 1)
	cfg := DaemonConfig{
		PIDPath:          "", // Empty → skip PID file
		WatchdogInterval: time.Hour,
		BackupInterval:   10 * time.Millisecond,
		BackupFunc: func(ctx context.Context) error {
			select {
			case backupCalled <- struct{}{}:
			default:
			}
			return nil
		},
	}
	d := NewDaemon(r, cfg)
	r.RegisterDaemon(d)
	if r.Daemon() != d {
		t.Fatal("RegisterDaemon did not store the daemon")
	}

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = d.Stop() }()

	select {
	case <-backupCalled:
		// Backup loop started and invoked BackupFunc.
	case <-time.After(time.Second):
		t.Error("backup function was never called")
	}
}

// TestDaemonStart_NilBackupFuncNoop verifies that a registered daemon without
// a BackupFunc (default config, as used before wiring) runs without invoking
// any backup work — the nil-check skip in Start/runBackup must hold.
func TestDaemonStart_NilBackupFuncNoop(t *testing.T) {
	r := newTestRuntime()

	d := NewDaemon(r, DaemonConfig{
		PIDPath:          "",
		WatchdogInterval: time.Hour,
		BackupInterval:   10 * time.Millisecond,
	})
	r.RegisterDaemon(d)

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = d.Stop() }()

	// Give the backup ticker time to fire; with nil BackupFunc nothing
	// should run and the daemon must stay healthy.
	time.Sleep(30 * time.Millisecond)
	if !d.IsRunning() {
		t.Error("daemon stopped running despite nil BackupFunc")
	}
}

// =============================================================================
// backupLoop
// =============================================================================

func TestBackupLoopExecutesBackupOnTick(t *testing.T) {
	r := New()
	var calls atomic.Int32
	d := NewDaemon(r, DaemonConfig{
		BackupInterval: 10 * time.Millisecond,
		BackupFunc: func(_ context.Context) error {
			calls.Add(1)
			return nil
		},
	})

	d.wg.Add(1)
	go d.backupLoop()

	deadline := time.After(time.Second)
	for calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("backupLoop never invoked backupFunc")
		case <-time.After(time.Millisecond):
		}
	}

	close(d.backupStop)
	d.wg.Wait()
}

func TestBackupLoopErrorContinues(t *testing.T) {
	r := New()
	var calls atomic.Int32
	d := NewDaemon(r, DaemonConfig{
		BackupInterval: 10 * time.Millisecond,
		BackupFunc: func(_ context.Context) error {
			calls.Add(1)
			return errors.New("backup failed")
		},
	})

	d.wg.Add(1)
	go d.backupLoop()

	deadline := time.After(2 * time.Second)
	for calls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("backupLoop stopped after error; calls = %d", calls.Load())
		case <-time.After(time.Millisecond):
		}
	}

	close(d.backupStop)
	d.wg.Wait()

	if r.metrics.ErrorCount() == 0 {
		t.Error("ErrorCount = 0, want >= 1 after backup failures")
	}
}

func TestBackupLoopStopsOnContextCancel(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		BackupInterval: 10 * time.Millisecond,
		BackupFunc:     func(_ context.Context) error { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	d.ctx = ctx

	d.wg.Add(1)
	go d.backupLoop()

	time.Sleep(25 * time.Millisecond)

	cancel()
	d.wg.Wait()
}

// =============================================================================
// pruneOldBackups
// =============================================================================

func TestPruneOldBackupsKeepsLimit(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 9)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupDir = backupDir
	d.maxBackups = 4

	d.pruneOldBackups()

	remaining := listFiles(t, backupDir, "auto-*.db")
	if len(remaining) != 4 {
		t.Fatalf("remaining auto backups = %d, want 4", len(remaining))
	}
	for i, want := range []string{"auto-00000006.db", "auto-00000007.db", "auto-00000008.db", "auto-00000009.db"} {
		if remaining[i] != want {
			t.Errorf("kept[%d] = %q, want %q", i, remaining[i], want)
		}
	}
}

func TestPruneOldBackupsPreservesForeignFiles(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 8)
	if err := os.WriteFile(filepath.Join(backupDir, "manual-20260101.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write manual backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write notes file: %v", err)
	}

	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupDir = backupDir
	d.maxBackups = 5

	d.pruneOldBackups()

	if got := len(listFiles(t, backupDir, "auto-*.db")); got != 5 {
		t.Errorf("remaining auto backups = %d, want 5", got)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "manual-20260101.db")); err != nil {
		t.Error("manual backup should be preserved")
	}
	if _, err := os.Stat(filepath.Join(backupDir, "notes.txt")); err != nil {
		t.Error("non-backup file should be preserved")
	}
}

func TestPruneOldBackupsUnderLimitNoop(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 3)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupDir = backupDir
	d.maxBackups = 7

	d.pruneOldBackups()

	if got := len(listFiles(t, backupDir, "auto-*.db")); got != 3 {
		t.Errorf("remaining auto backups = %d, want 3 (no pruning below limit)", got)
	}
}

func TestPruneOldBackupsDisabled(t *testing.T) {
	r := New()
	backupDir := t.TempDir()
	createAutoBackupFiles(t, backupDir, 10)

	// No backup dir → retention disabled.
	d := NewDaemon(r, DefaultDaemonConfig())
	d.backupFunc = func(_ context.Context) error { return nil }
	d.runBackup()
	d.pruneOldBackups()

	// MaxBackups <= 0 → retention disabled.
	d.backupDir = backupDir
	d.maxBackups = 0
	d.pruneOldBackups()

	if got := len(listFiles(t, backupDir, "auto-*.db")); got != 10 {
		t.Errorf("remaining auto backups = %d, want 10 (retention disabled)", got)
	}
}

// =============================================================================
// Status
// =============================================================================

func TestDaemonStatusBackupFields(t *testing.T) {
	t.Parallel()
	r := New()
	d := NewDaemon(r, DaemonConfig{
		BackupInterval: 30 * time.Minute,
		BackupDir:      "/data/backups",
		MaxBackups:     7,
	})
	status := d.Status()
	if status["backup_interval"] != "30m0s" {
		t.Errorf("backup_interval = %v, want 30m0s", status["backup_interval"])
	}
	if status["backup_dir"] != "/data/backups" {
		t.Errorf("backup_dir = %v, want /data/backups", status["backup_dir"])
	}
	if status["max_backups"] != 7 {
		t.Errorf("max_backups = %v, want 7", status["max_backups"])
	}
}

// =============================================================================
// Helpers
// =============================================================================

func createAutoBackupFiles(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		path := filepath.Join(dir, fmt.Sprintf("auto-%08d.db", i))
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("create backup %s: %v", path, err)
		}
	}
}

func listFiles(t *testing.T, dir, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	for i, m := range matches {
		matches[i] = filepath.Base(m)
	}
	return matches
}
