package runtime

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/pkg/cosca"
)

func TestDefaultDaemonConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultDaemonConfig()
	if cfg.PIDPath != "./tmp/cosca.pid" {
		t.Errorf("PIDPath = %q, want %q", cfg.PIDPath, "./tmp/cosca.pid")
	}
	if cfg.LogMaxSize != 100*1024*1024 {
		t.Errorf("LogMaxSize = %d", cfg.LogMaxSize)
	}
	if cfg.LogMaxBackups != 5 {
		t.Errorf("LogMaxBackups = %d", cfg.LogMaxBackups)
	}
	if cfg.WatchdogInterval != 30*time.Second {
		t.Errorf("WatchdogInterval = %v", cfg.WatchdogInterval)
	}
	if cfg.SyncInterval != 5*time.Minute {
		t.Errorf("SyncInterval = %v", cfg.SyncInterval)
	}
}

func TestNewDaemon(t *testing.T) {
	t.Parallel()
	r := New()
	cfg := DefaultDaemonConfig()
	d := NewDaemon(r, cfg)
	if d == nil {
		t.Fatal("NewDaemon returned nil")
	}
	if d.runtime != r {
		t.Error("daemon runtime should match")
	}
	if d.pidPath != cfg.PIDPath {
		t.Errorf("pidPath = %q", d.pidPath)
	}
}

func TestDaemonIsRunning(t *testing.T) {
	t.Parallel()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	if d.IsRunning() {
		t.Error("New daemon should not be running")
	}
}

func TestDaemonStatus(t *testing.T) {
	t.Parallel()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	status := d.Status()
	if status == nil {
		t.Fatal("Status returned nil")
	}
	if status["pid_file"] != "./tmp/cosca.pid" {
		t.Errorf("pid_file = %v", status["pid_file"])
	}
	if status["watchdog_active"] != false {
		t.Errorf("watchdog_active = %v", status["watchdog_active"])
	}
}

func TestDaemonPID(t *testing.T) {
	t.Parallel()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	pid := d.PID()
	if pid <= 0 {
		t.Errorf("PID should be > 0, got %d", pid)
	}
}

// TestRealPID verifies that realPID resolves the host PID via COSCA_JAIL_PID
// inside the bwrap jail and falls back to os.Getpid() otherwise.
// Note: intentionally NOT parallel — t.Setenv mutates the process environment,
// and sequential tests run before parallel tests in Go's execution model.
func TestRealPID(t *testing.T) {
	t.Run("empty env falls back to os.Getpid", func(t *testing.T) {
		t.Setenv(cosca.EnvJailPID, "")
		if got := realPID(); got != os.Getpid() {
			t.Errorf("realPID() = %d, want %d (fallback to os.Getpid)", got, os.Getpid())
		}
	})

	t.Run("valid jail pid returns host pid", func(t *testing.T) {
		t.Setenv(cosca.EnvJailPID, "12345")
		if got := realPID(); got != 12345 {
			t.Errorf("realPID() = %d, want 12345", got)
		}
	})

	t.Run("pid 1 is not valid for the jail", func(t *testing.T) {
		t.Setenv(cosca.EnvJailPID, "1")
		if got := realPID(); got != os.Getpid() {
			t.Errorf("realPID() = %d, want %d (fallback to os.Getpid)", got, os.Getpid())
		}
	})

	t.Run("invalid value falls back to os.Getpid", func(t *testing.T) {
		t.Setenv(cosca.EnvJailPID, "not-a-pid")
		if got := realPID(); got != os.Getpid() {
			t.Errorf("realPID() = %d, want %d (fallback to os.Getpid)", got, os.Getpid())
		}
	})
}

func TestDaemonSetLogWriter(t *testing.T) {
	t.Parallel()
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	d := NewDaemon(r, DefaultDaemonConfig())
	if d == nil {
		t.Fatal("NewDaemon returned nil")
	}
	d.SetLogWriter(nil)
	// Should not panic — SetLogWriter accepts nil to disable log file output
}

func TestDaemonRotateLogs(t *testing.T) {
	t.Parallel()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	err := d.RotateLogs()
	if err != nil {
		t.Fatalf("RotateLogs error: %v", err)
	}
}

func TestProcessRunning(t *testing.T) {
	t.Parallel()
	// PID 0 is always invalid on all platforms
	running := processRunning(0)
	if running {
		t.Error("processRunning(0) should return false on all platforms — PID 0 is never a real process")
	}
}

func TestDaemonStopReturnsWhenWorkerIgnoresCancellation(t *testing.T) {
	r := New()
	started := make(chan struct{})
	release := make(chan struct{})
	workerDone := make(chan struct{})
	var workerDoneOnce sync.Once
	cfg := DefaultDaemonConfig()
	cfg.PIDPath = t.TempDir() + "/cosca.pid"
	cfg.ShutdownTimeout = 30 * time.Millisecond
	cfg.WatchdogInterval = time.Hour
	cfg.SyncInterval = time.Millisecond
	cfg.SyncFunc = func(context.Context) error {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		workerDoneOnce.Do(func() { close(workerDone) })
		return nil
	}
	d := NewDaemon(r, cfg)
	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("sync worker did not start")
	}

	start := time.Now()
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Stop took %v, want it to return within the shutdown wait deadline", elapsed)
	}

	close(release)
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("daemon worker did not exit after release")
	}
}
