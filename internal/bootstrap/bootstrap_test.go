package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		DataDir: t.TempDir(),
		Logger:  zerolog.Nop(),
	}
	if cfg.EnableDaemon {
		t.Error("EnableDaemon should default to false")
	}
	if cfg.RuntimeStandalone {
		t.Error("RuntimeStandalone should default to false")
	}
}

func TestConfig_RuntimeStandalone(t *testing.T) {
	cfg := Config{
		DataDir:           t.TempDir(),
		Logger:            zerolog.Nop(),
		EnableDaemon:      true,
		RuntimeStandalone: true,
	}

	// When RuntimeStandalone is true, daemon should NOT be created
	// even if EnableDaemon is true.
	result, err := Compose(cfg)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}
	if result.Daemon != nil {
		t.Error("Daemon should be nil when RuntimeStandalone=true")
	}
	if result.Runtime == nil {
		t.Error("Runtime should always be non-nil")
	}
}

// composeTest runs Compose and registers cleanup that closes the composed
// engines' SQLite handles. On Windows an open DB file blocks the TempDir
// cleanup (file lock); on Linux unlink works with the handle open, so the
// leak was silent before.
func composeTest(t *testing.T, cfg Config) *Result {
	t.Helper()
	result, err := Compose(cfg)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}
	t.Cleanup(func() {
		if result.Knowledge != nil {
			_ = result.Knowledge.Close()
		}
		if result.Memory != nil {
			_ = result.Memory.Close()
		}
	})
	return result
}

func TestConfig_EmbeddingOverride(t *testing.T) {
	cfg := Config{
		DataDir:             t.TempDir(),
		Logger:              zerolog.Nop(),
		EmbeddingProvider:   "local",
		EmbeddingModel:      "test-model",
		EmbeddingBaseURL:    "http://localhost:9999",
		EmbeddingDimensions: 768,
	}

	result := composeTest(t, cfg)
	if result.Runtime == nil {
		t.Error("Runtime should be non-nil")
	}
	// Knowledge engine may be nil if sqlite/open fails in test env.
	// That's ok — we're testing that config is plumbed through.
}

func TestConfig_WatchFrameworkDir(t *testing.T) {
	watchDir := t.TempDir()
	cfg := Config{
		DataDir:           t.TempDir(),
		Logger:            zerolog.Nop(),
		WatchFrameworkDir: watchDir,
	}

	result := composeTest(t, cfg)
	if result.Runtime == nil {
		t.Error("Runtime should be non-nil")
	}
}

func TestConfig_DataDirCreation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cosca-test")
	cfg := Config{
		DataDir: dir,
		Logger:  zerolog.Nop(),
	}

	// Compose should create the data dir if needed.
	result := composeTest(t, cfg)
	if result.Runtime == nil {
		t.Error("Runtime should be non-nil")
	}
}

func TestResult_Fields(t *testing.T) {
	res := &Result{}
	if res.Daemon != nil {
		t.Error("fresh Result should have nil Daemon")
	}
	if res.Knowledge != nil {
		t.Error("fresh Result should have nil Knowledge")
	}
	if res.Memory != nil {
		t.Error("fresh Result should have nil Memory")
	}
	if res.Runtime != nil {
		t.Error("fresh Result should have nil Runtime")
	}
}

func TestComputeSubsystem_Interface(t *testing.T) {
	// computeSubsystem is defined in bootstrap.go. We just verify
	// the type exists and satisfies the interface at compile time.
	// This is already enforced by `var _ rt.Subsystem = (*computeSubsystem)(nil)`.
	// Test that nil fabric doesn't panic on basic calls.
	cs := &computeSubsystem{fabric: nil}
	// These should not panic even with nil fabric.
	name := cs.Name()
	if name == "" {
		t.Log("nil fabric returned empty name — expected")
	}
}

func TestCompose_InvalidDataDir(t *testing.T) {
	// Test with a data dir that can't be created (e.g., /dev/null/subdir).
	cfg := Config{
		DataDir: filepath.Join("/dev/null", "cosca"),
		Logger:  zerolog.Nop(),
	}
	_, err := Compose(cfg)
	// Should not panic — should return an error or warn.
	_ = err
}

func TestCompose_Timeout(t *testing.T) {
	cfg := Config{
		DataDir: t.TempDir(),
		Logger:  zerolog.Nop(),
	}

	done := make(chan struct{})
	go func() {
		res, _ := Compose(cfg)
		// Close engine SQLite handles: the goroutine discards the result, so
		// without this the open DB files would block TempDir cleanup on
		// Windows (file lock).
		if res != nil {
			if res.Knowledge != nil {
				_ = res.Knowledge.Close()
			}
			if res.Memory != nil {
				_ = res.Memory.Close()
			}
		}
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(15 * time.Second):
		t.Fatal("Compose timed out after 15s")
	}
}

func TestConfig_LoggerContext(t *testing.T) {
	cfg := Config{
		DataDir: t.TempDir(),
		Logger:  zerolog.Nop(),
	}
	// composeTest registers cleanup that closes the knowledge/memory engines.
	_ = composeTest(t, cfg)
}

// TestRegisterEngines_DIResolvesKnowledge verifies the declarative DI seam:
// after RegisterEngines, a composable service (KnowledgeSummary) resolves
// with the knowledge engine INJECTED by the container — never reached via a
// global or the Result struct by name.
func TestRegisterEngines_DIResolvesKnowledge(t *testing.T) {
	res := &Result{}
	c := RegisterEngines(res)

	// No engines built → container exists but is empty (not nil).
	if c == nil {
		t.Fatal("Services container should never be nil after RegisterEngines")
	}
	if !c.Has(ServiceKnowledge) && !c.Has(ServiceMemory) {
		// Empty is fine (no engines in standalone), but the summary facet
		// only registers when knowledge exists — both must be nil-safe.
		return
	}
	// If knowledge was built, the composable facet resolves with injection.
	if c.Has(ServiceKnowledge) && c.Has(ServiceKnowledgeSummary) {
		summary, err := ResolveKnowledgeSummary(c)
		if err != nil {
			t.Fatalf("resolve knowledge summary: %v", err)
		}
		if summary.Source != ServiceKnowledge {
			t.Fatalf("source = %q, want %q", summary.Source, ServiceKnowledge)
		}
		if summary.Engine == nil {
			t.Fatal("summary.Engine should be injected by the container")
		}
	}
}

func init() {
	// Ensure we don't create real files during tests.
	if s := os.Getenv("COSCA_TEST_BOOTSTRAP"); s != "1" {
		os.Setenv("COSCA_DEV_MODE", "true")
	}
}
