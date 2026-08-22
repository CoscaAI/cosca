package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// findLiveCoscaPID scans /proc for a running process whose executable basename
// contains "cosca". It returns 0 if none is found. Used to exercise the
// "real second daemon" branch of the PID file guard.
func findLiveCoscaPID() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil {
			continue
		}
		if strings.Contains(filepath.Base(exe), "cosca") {
			return pid
		}
	}
	return 0
}

func TestIsCoscaProcess(t *testing.T) {
	t.Run("pid <= 1 is never cosca", func(t *testing.T) {
		for _, pid := range []int{-100, -1, 0, 1} {
			if isCoscaProcess(pid) {
				t.Errorf("isCoscaProcess(%d) = true, want false", pid)
			}
		}
	})

	t.Run("nonexistent pid is not cosca", func(t *testing.T) {
		// A huge PID that is extremely unlikely to exist; Readlink fails.
		if isCoscaProcess(2147483647) {
			t.Error("isCoscaProcess(max int32) = true, want false")
		}
	})

	t.Run("current process matches exe-derived expectation", func(t *testing.T) {
		// The test binary is named *.test, so the expected result is derived
		// from the real /proc/self/exe rather than hard-coded.
		exe, err := os.Readlink("/proc/self/exe")
		if err != nil {
			t.Skip("cannot read /proc/self/exe")
		}
		want := strings.Contains(filepath.Base(exe), "cosca")
		if got := isCoscaProcess(os.Getpid()); got != want {
			t.Errorf("isCoscaProcess(%d) = %v, want %v (exe %q)", os.Getpid(), got, want, exe)
		}
	})

	t.Run("live cosca process is detected", func(t *testing.T) {
		pid := findLiveCoscaPID()
		if pid == 0 {
			t.Skip("no live cosca process found to test against")
		}
		if !isCoscaProcess(pid) {
			t.Errorf("isCoscaProcess(%d) = false, want true", pid)
		}
	})
}

func TestWritePIDFileGuard(t *testing.T) {
	t.Run("own pid is not blocked", func(t *testing.T) {
		// (a) A PID file containing the current process's own PID is stale
		// from a previous boot of this same daemon — must be overwritten.
		assertGuardDoesNotBlock(t, os.Getpid())
	})

	t.Run("realPID is not blocked", func(t *testing.T) {
		// Inside the jail, realPID() is the host PID of this same daemon.
		if realPID() != os.Getpid() {
			assertGuardDoesNotBlock(t, realPID())
		}
	})

	t.Run("kernel init pids are not blocked", func(t *testing.T) {
		// (b) PID 1 (init) and PID 2 (kthreadd) are running but are never
		// cosca daemons — stale, must be overwritten.
		assertGuardDoesNotBlock(t, 1)
		assertGuardDoesNotBlock(t, 2)
	})

	t.Run("nonexistent pid is not blocked", func(t *testing.T) {
		// (c) A fabricated large PID with no process behind it is stale.
		assertGuardDoesNotBlock(t, 2147483646)
	})

	t.Run("different live cosca process blocks", func(t *testing.T) {
		// (d) A real second daemon must block startup.
		pid := findLiveCoscaPID()
		if pid == 0 || pid == os.Getpid() || pid == realPID() {
			t.Skip("no live cosca process with a different PID found to test against")
		}
		pidDir := t.TempDir()
		pidFile := filepath.Join(pidDir, "cosca.pid")
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0o644); err != nil {
			t.Fatalf("write pid: %v", err)
		}
		r := New()
		d := NewDaemon(r, DefaultDaemonConfig())
		d.pidPath = pidFile
		if err := d.writePIDFile(); err == nil {
			t.Errorf("writePIDFile with live cosca PID %d should fail", pid)
		}
	})
}

// assertGuardDoesNotBlock verifies that a PID file containing existingPID is
// treated as stale: writePIDFile succeeds and overwrites it with realPID().
func assertGuardDoesNotBlock(t *testing.T, existingPID int) {
	t.Helper()
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "cosca.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", existingPID)), 0o644); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile
	if err := d.writePIDFile(); err != nil {
		t.Errorf("writePIDFile with PID %d should not block: %v", existingPID, err)
		return
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	want := fmt.Sprintf("%d\n", realPID())
	if string(data) != want {
		t.Errorf("pid file content = %q, want %q", string(data), want)
	}
	_ = d.removePIDFile()
}
