package cli

import (
	"context"
	"os"
	"time"

	rt "github.com/CoscaAI/cosca/internal/runtime"
)

// Runtime Adapter
// =============================================================================

// runtimeAdapter wraps rt.Runtime to provide the methods CLI commands expect.
type runtimeAdapter struct {
	inner *rt.Runtime
	dir   string
}

// newRuntimeAdapter creates a new runtime adapter.
// The real runtime.New() needs options, so we create with minimal config.
func newRuntimeAdapter(dir string) *runtimeAdapter {
	cfg := rt.DefaultRuntimeConfig()
	cfg.DataDir = dir
	r := rt.New(rt.WithConfig(cfg))
	return &runtimeAdapter{inner: r, dir: dir}
}

// Start starts the runtime. CLI passes (daemon bool) but real API takes context.Context.
func (a *runtimeAdapter) Start(_ bool) error {
	ctx := context.Background()
	return a.inner.Start(ctx)
}

// Stop stops the runtime. CLI passes (force bool) but real API takes context.Context.
func (a *runtimeAdapter) Stop(_ bool) error {
	ctx := context.Background()
	return a.inner.Stop(ctx)
}

// Restart restarts the runtime. CLI passes no args but real API takes context.Context.
func (a *runtimeAdapter) Restart() error {
	ctx := context.Background()
	return a.inner.Restart(ctx)
}

// State returns the state as a string. CLI expects (string, error).
// Real Runtime.State() returns *RuntimeState.
func (a *runtimeAdapter) State() (string, error) {
	if a.inner == nil || a.inner.State() == nil {
		return "unknown", nil
	}
	return a.inner.State().Current().String(), nil
}

// Uptime returns uptime as a string. CLI expects (string, error).
// Real Runtime has Metrics().Uptime() returning time.Duration.
func (a *runtimeAdapter) Uptime() (string, error) {
	if a.inner == nil {
		return "0s", nil
	}
	return a.inner.Metrics().Uptime().Round(time.Second).String(), nil
}

// IsHealthy returns whether the runtime is healthy.
// Real Runtime.Health() returns ComponentStatus.
func (a *runtimeAdapter) IsHealthy() bool {
	if a.inner == nil {
		return false
	}
	return a.inner.Health() == rt.StatusHealthy
}

// PID returns a placeholder PID. Real runtime doesn't track PID in this way.
func (a *runtimeAdapter) PID() int {
	if a.inner == nil {
		return os.Getpid()
	}
	return os.Getpid()
}

// FollowLogs follows runtime logs. Placeholder.
func (a *runtimeAdapter) FollowLogs(_ interface{}) error {
	return nil
}

// Logs returns recent log lines. Placeholder.
func (a *runtimeAdapter) Logs(_ int) ([]string, error) {
	return []string{"[runtime adapter] no logs available"}, nil
}

// Info returns runtime information. CLI expects Info struct with fields.
func (a *runtimeAdapter) Info() RuntimeInfo {
	return RuntimeInfo{
		Version:     "0.0.0",
		State:       "unknown",
		Uptime:      "0s",
		PID:         os.Getpid(),
		MemoryUsage: "0 MB",
		CPUUsage:    "0%",
		Goroutines:  0,
		ListenAddr:  "",
		DataDir:     a.dir,
		LogLevel:    "info",
		StartedAt:   time.Now(),
	}
}

// RuntimeInfo holds runtime information as expected by CLI commands.
type RuntimeInfo struct {
	Version     string    `json:"version"`
	State       string    `json:"state"`
	Uptime      string    `json:"uptime"`
	PID         int       `json:"pid"`
	MemoryUsage string    `json:"memory_usage"`
	CPUUsage    string    `json:"cpu_usage"`
	Goroutines  int       `json:"goroutines"`
	ListenAddr  string    `json:"listen_addr"`
	DataDir     string    `json:"data_dir"`
	LogLevel    string    `json:"log_level"`
	StartedAt   time.Time `json:"started_at"`
}

// Init initializes the runtime by creating the data directory. Placeholder.
func (a *runtimeAdapter) Init() error {
	if a.dir != "" {
		return os.MkdirAll(a.dir, 0755)
	}
	return nil
}

// =============================================================================
// Runtime package-level function adapter
// =============================================================================

// =============================================================================
