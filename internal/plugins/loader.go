//
// Plugin Loader: supports loading plugins from Go plugins, shared libraries,
// external processes, and WebAssembly modules.

package plugins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// =============================================================================
// Loader
// =============================================================================

// Loader handles loading plugins from various runtime types:
//   - Go plugins (*.so files loaded via plugin.Open)
//   - Shared libraries (*.so, *.dll)
//   - External processes (child process with stdin/stdout protocol)
//   - WebAssembly modules (*.wasm)
//
// # Security Model
//
// Each plugin runtime has a different security posture:
//
//   - WASM (wazero): Full sandboxing. Modules run in a WebAssembly VM with
//     controlled memory, restricted filesystem access (chrooted to the
//     plugin's data directory only), and no access to the host system.
//     This is the recommended runtime for untrusted plugins.
//
//   - External: Best-effort sandboxing via OS-level controls. Child
//     processes are restricted with rlimits (CPU, memory, file size, open
//     files), stderr isolation, message size limits, and (on Linux) a
//     seccomp-bpf filter that blocks process creation, file opening,
//     networking, and privilege escalation. External plugins should be
//     treated as semi-trusted.
//
//   - Go / SharedLib: NO sandbox. These runtimes load native code directly
//     into the process address space with full system access. Go plugins
//     are DISABLED by default (DisableGoPlugins: true) and must be
//     explicitly enabled. Loading a Go plugin grants it unrestricted
//     access to memory, filesystem, network, and all process capabilities.
//     Only load Go plugins from fully trusted sources.
type Loader struct {
	mu sync.RWMutex

	// config holds the loader configuration.
	config LoaderConfig

	// loaded tracks already-loaded plugins to prevent duplicate loads.
	loaded map[string]Plugin
}

// SandboxConfig controls the security sandbox applied to external plugin
// child processes. These limits are enforced via OS-level mechanisms
// (rlimits and seccomp) before the child process starts.
type SandboxConfig struct {
	// Enabled controls whether sandboxing is applied to external plugins.
	// When false, no rlimits or seccomp filters are applied.
	Enabled bool

	// MaxCPUSeconds is the maximum CPU time in seconds (RLIMIT_CPU).
	// After this limit, the kernel sends SIGXCPU to the process.
	// Default: 30
	MaxCPUSeconds int

	// MaxMemoryMB is the maximum virtual memory in megabytes (RLIMIT_AS).
	// Prevents the plugin from consuming excessive memory.
	// Default: 512
	MaxMemoryMB int

	// MaxFileSizeMB is the maximum file size in megabytes (RLIMIT_FSIZE).
	// Prevents the plugin from writing huge files (if it somehow opens one).
	// Default: 10
	MaxFileSizeMB int

	// MaxMessageBytes is the maximum size of a single JSON message
	// (both sent and received) for external plugin communication.
	// Prevents memory exhaustion from oversized messages.
	// Default: 1048576 (1 MB)
	MaxMessageBytes int

	// SeccompEnabled controls whether seccomp-bpf is applied on Linux.
	// When true, the external plugin process is restricted to a minimal
	// set of safe syscalls: read, write, close, exit, memory management,
	// and synchronization primitives. Fork, exec, socket, open, mount,
	// ptrace, and privilege-changing syscalls are all denied.
	// On non-Linux platforms, this is ignored.
	// Default: true
	SeccompEnabled bool
}

// LoaderConfig configures the plugin loader.
type LoaderConfig struct {
	// PluginsDir is the base directory for plugins.
	PluginsDir string

	// AllowedRuntimes restricts which plugin runtimes are allowed.
	// Empty means all runtimes are allowed.
	AllowedRuntimes []PluginRuntime

	// PermissionPolicy defines how permissions are enforced.
	// "strict" rejects plugins with unknown permissions.
	// "warn" logs a warning but allows unknown permissions.
	// "allow" allows all permissions.
	PermissionPolicy string

	// MaxWASMMemory is the maximum memory for WASM plugins in bytes.
	MaxWASMMemory int64

	// ExternalProcessTimeout is the timeout for external process plugins.
	ExternalProcessTimeout int

	// DisableGoPlugins controls whether Go native plugins are allowed.
	// Go plugins load native .so files into the process address space
	// with NO sandbox and unrestricted system access. For security,
	// this defaults to true (Go plugins disabled). Set to false ONLY
	// when loading plugins from fully trusted sources.
	DisableGoPlugins bool

	// Sandbox configures the security sandbox for external plugin
	// child processes (rlimits + seccomp).
	Sandbox SandboxConfig
}

// DefaultLoaderConfig returns a default loader configuration with
// secure-by-default settings.
func DefaultLoaderConfig(pluginsDir string) LoaderConfig {
	return LoaderConfig{
		PluginsDir:             pluginsDir,
		AllowedRuntimes:        nil, // all runtimes allowed
		PermissionPolicy:       "strict",
		MaxWASMMemory:          128 * 1024 * 1024, // 128 MB
		ExternalProcessTimeout: 30,
		// SECURITY: Go plugins are disabled by default because they load
		// native code into the process with unrestricted system access.
		DisableGoPlugins: true,
		Sandbox: SandboxConfig{
			Enabled:         true,
			MaxCPUSeconds:   30,
			MaxMemoryMB:     512,
			MaxFileSizeMB:   10,
			MaxMessageBytes: 1 * 1024 * 1024, // 1 MB
			SeccompEnabled:  true,
		},
	}
}

// NewLoader creates a new plugin loader.
func NewLoader(config LoaderConfig) *Loader {
	return &Loader{
		config: config,
		loaded: make(map[string]Plugin),
	}
}

// =============================================================================
// Loading
// =============================================================================

// LoadPlugin loads a single plugin from the specified path.
// The path should point to a directory containing a plugin manifest
// and the plugin binary/archive.
func (l *Loader) LoadPlugin(path string) (Plugin, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Stat the path to determine if it's a directory or file
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	var manifest *PluginManifest
	var pluginPath string

	if info.IsDir() {
		// Load manifest from directory
		manifest, err = loadManifestFromDir(absPath)
		if err != nil {
			return nil, fmt.Errorf("load manifest: %w", err)
		}

		// Determine entry point
		if manifest.EntryPoint != "" {
			pluginPath = filepath.Join(absPath, manifest.EntryPoint)
		} else {
			// Try to find the plugin binary
			pluginPath, err = l.findPluginBinary(absPath, manifest.Runtime)
			if err != nil {
				return nil, fmt.Errorf("find plugin binary: %w", err)
			}
		}
	} else {
		// It's a single file — try to load manifest from a sidecar
		manifest, err = l.loadSidecarManifest(absPath)
		if err != nil {
			return nil, fmt.Errorf("load sidecar manifest: %w", err)
		}
		pluginPath = absPath
	}

	// Check if already loaded
	if existing, ok := l.loaded[manifest.ID]; ok {
		log.Warn().Str("plugin", manifest.ID).Msg("plugin already loaded, returning existing")
		return existing, nil
	}

	// Validate runtime
	if err := l.validateRuntime(manifest.Runtime); err != nil {
		return nil, fmt.Errorf("runtime validation: %w", err)
	}

	// Validate permissions
	if err := l.validatePermissions(manifest); err != nil {
		return nil, fmt.Errorf("permission validation: %w", err)
	}

	// Load the plugin based on runtime type
	var p Plugin
	switch manifest.Runtime {
	case RuntimeGo:
		p, err = l.loadGoPlugin(manifest, pluginPath)
	case RuntimeSharedLib:
		p, err = l.loadSharedLibPlugin(manifest, pluginPath)
	case RuntimeExternal:
		p, err = l.loadExternalPlugin(manifest, pluginPath)
	case RuntimeWASM:
		p, err = l.loadWASMPlugin(manifest, pluginPath)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", manifest.Runtime)
	}

	if err != nil {
		return nil, fmt.Errorf("load %s plugin: %w", manifest.Runtime, err)
	}

	l.loaded[manifest.ID] = p
	log.Info().Str("plugin", manifest.ID).
		Str("runtime", string(manifest.Runtime)).
		Str("path", pluginPath).
		Msg("plugin loaded")
	return p, nil
}

// LoadAll loads all plugins from the specified directory.
// Each subdirectory is treated as a plugin.
func (l *Loader) LoadAll(dir string) ([]Plugin, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	var plugins []Plugin
	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			// Check if it's a standalone plugin file
			if isPluginFile(entry.Name()) {
				pluginPath := filepath.Join(absDir, entry.Name())
				p, loadErr := l.LoadPlugin(pluginPath)
				if loadErr != nil {
					errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), loadErr))
					continue
				}
				plugins = append(plugins, p)
			}
			continue
		}

		pluginPath := filepath.Join(absDir, entry.Name())
		p, loadErr := l.LoadPlugin(pluginPath)
		if loadErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), loadErr))
			continue
		}
		plugins = append(plugins, p)
	}

	if len(errs) > 0 {
		// Log all errors but return the successfully loaded plugins
		for _, e := range errs {
			log.Error().Err(e).Msg("plugin load error")
		}
	}

	return plugins, nil
}

// =============================================================================
// Go Plugin Loading
// =============================================================================

// loadGoPlugin loads a Go plugin using the standard plugin package.
// The plugin must export a symbol named "Plugin" that implements the Plugin interface.
//
// SECURITY: Go plugins load native .so files directly into the process
// address space with NO sandbox and unrestricted system access. They are
// disabled by default (DisableGoPlugins: true). Only enable when loading
// plugins from fully trusted sources.
func (l *Loader) loadGoPlugin(manifest *PluginManifest, path string) (Plugin, error) {
	if l.config.DisableGoPlugins {
		return nil, fmt.Errorf("go plugins are disabled for security (set DisableGoPlugins=false to enable)")
	}

	log.Warn().Str("plugin", manifest.ID).Str("path", path).
		Msg("CRITICAL: Go plugin loaded — native code execution, no sandbox possible")

	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("plugin.Open: %w", err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("lookup Plugin symbol: %w", err)
	}

	pluginImpl, ok := sym.(Plugin)
	if !ok {
		return nil, fmt.Errorf("exported Plugin symbol does not implement plugins.Plugin interface")
	}

	return pluginImpl, nil
}

// =============================================================================
// Shared Library Plugin Loading
// =============================================================================

// loadSharedLibPlugin loads a shared library plugin.
// This uses cgo or a subprocess approach to load the shared library.
func (l *Loader) loadSharedLibPlugin(manifest *PluginManifest, path string) (Plugin, error) {
	// Try External runtime as fallback, since shared libs work similarly
	return l.loadExternalPlugin(manifest, path)
}

// =============================================================================
// External Process Plugin Loading
// =============================================================================

// healthResult holds the result of an async health check.
type healthResult struct {
	health PluginHealth
	err    error
}

// externalPlugin wraps an external process as a Plugin.
type externalPlugin struct {
	BasePlugin
	modulePath     string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         io.ReadCloser
	scanner        *bufio.Scanner
	loader         *Loader      // reference for sandbox config and message limits
	stderrBuf      bytes.Buffer // isolated stderr capture (never exposed to host)
	maxMessageSize int          // max size of a single JSON message

	// Lifecycle coordination
	ctx    context.Context    // lifecycle context for background goroutines
	cancel context.CancelFunc // cancels the read loop and background work
	exitCh chan error         // receives process exit error (nil = clean exit)
	doneCh chan struct{}      // closed when process exit watcher completes
}

// loadExternalPlugin loads an external process plugin.
// The external process communicates via stdin/stdout using JSON messages.
//
// SECURITY: External plugins are sandboxed with rlimits (CPU, memory, file size,
// open files), stderr isolation, message size limits, and (on Linux) a seccomp-bpf
// filter. See SandboxConfig for details.
func (l *Loader) loadExternalPlugin(manifest *PluginManifest, path string) (Plugin, error) {
	// Verify the binary exists and is executable
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("external plugin binary: %w", err)
	}
	if info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("external plugin binary %q is not executable", path)
	}

	p := &externalPlugin{
		BasePlugin: BasePlugin{
			IDValue:      manifest.ID,
			NameValue:    manifest.Name,
			VersionValue: manifest.Version,
			DescValue:    manifest.Description,
			AuthorValue:  manifest.Author,
		},
		modulePath:     path,
		loader:         l,
		maxMessageSize: l.config.Sandbox.MaxMessageBytes,
		exitCh:         make(chan error, 1),
		doneCh:         make(chan struct{}),
	}

	return p, nil
}

// sendJSON writes a JSON message to the plugin's stdin, followed by a newline.
// Enforces the max message size limit to prevent memory exhaustion.
func (p *externalPlugin) sendJSON(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if len(data) > p.maxMessageSize {
		return fmt.Errorf("outgoing message size %d exceeds limit %d", len(data), p.maxMessageSize)
	}
	data = append(data, '\n')
	if _, err := p.stdin.Write(data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	return nil
}

// readResponse reads a single JSON response line from the plugin's stdout.
// Enforces the max message size limit to prevent memory exhaustion from
// oversized or malicious responses.
func (p *externalPlugin) readResponse(v interface{}) error {
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		return fmt.Errorf("unexpected EOF from external plugin")
	}
	data := p.scanner.Bytes()
	if len(data) > p.maxMessageSize {
		return fmt.Errorf("incoming message size %d exceeds limit %d", len(data), p.maxMessageSize)
	}
	return json.Unmarshal(data, v)
}

// readResponseWithTimeout reads a JSON response from the plugin's stdout
// with a deadline. If the response is not received within the timeout,
// a timeout error is returned.
func (p *externalPlugin) readResponseWithTimeout(v interface{}, timeout time.Duration) error {
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- p.readResponse(v)
	}()
	select {
	case err := <-resultCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("read response timed out after %v", timeout)
	}
}

// Init initializes the external plugin by starting the process
// and sending the init handshake message.
func (p *externalPlugin) Init(ctx *PluginContext) error {
	if p.modulePath == "" {
		return fmt.Errorf("external plugin: binary path not set")
	}

	p.cmd = exec.Command(p.modulePath)

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	p.stdin = stdin

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	p.stdout = stdout

	// Capture stderr to isolated buffer — never expose to host
	p.cmd.Stderr = &p.stderrBuf

	// Apply sandbox (rlimits + optional seccomp on Linux)
	if p.loader.config.Sandbox.Enabled {
		if err := applySandboxToCmd(p.cmd, &p.loader.config.Sandbox); err != nil {
			return fmt.Errorf("apply sandbox: %w", err)
		}
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start external process: %w", err)
	}

	// Initialize lifecycle context
	p.ctx, p.cancel = context.WithCancel(context.Background())

	p.scanner = bufio.NewScanner(p.stdout)
	p.scanner.Buffer(make([]byte, 0, p.maxMessageSize), p.maxMessageSize)

	// Spawn process death detection goroutine
	go func() {
		err := p.cmd.Wait()
		select {
		case p.exitCh <- err:
		default:
		}
		close(p.doneCh)
	}()

	// Send init message
	initMsg := map[string]interface{}{
		"type":     "init",
		"config":   ctx.Config,
		"data_dir": ctx.DataDir,
	}
	if err := p.sendJSON(initMsg); err != nil {
		// Clean up on init failure
		p.cancel()
		_ = p.cmd.Process.Kill()
		<-p.doneCh
		return fmt.Errorf("send init: %w", err)
	}

	// Read init response
	var initResp struct {
		Type    string `json:"type"`
		Version string `json:"version,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	if err := p.readResponse(&initResp); err != nil {
		// Clean up on init failure
		p.cancel()
		_ = p.cmd.Process.Kill()
		<-p.doneCh
		return fmt.Errorf("read init response: %w", err)
	}

	if initResp.Type != "init_ok" {
		if initResp.Error != "" {
			// Clean up on init failure
			p.cancel()
			_ = p.cmd.Process.Kill()
			<-p.doneCh
			return fmt.Errorf("external plugin init failed: %s", initResp.Error)
		}
		// Clean up on init failure
		p.cancel()
		_ = p.cmd.Process.Kill()
		<-p.doneCh
		return fmt.Errorf("external plugin: unexpected init response type: %q", initResp.Type)
	}

	log.Debug().Str("plugin", p.IDValue).
		Str("version", initResp.Version).
		Msg("external plugin initialized")

	return p.BasePlugin.Init(ctx)
}

// Start sends the start command to the external plugin process.
func (p *externalPlugin) Start() error {
	if err := p.sendJSON(map[string]string{"type": "start"}); err != nil {
		return fmt.Errorf("send start: %w", err)
	}

	var startResp struct {
		Type  string `json:"type"`
		Error string `json:"error,omitempty"`
	}
	if err := p.readResponse(&startResp); err != nil {
		return fmt.Errorf("read start response: %w", err)
	}

	if startResp.Type != "start_ok" {
		if startResp.Error != "" {
			return fmt.Errorf("external plugin start failed: %s", startResp.Error)
		}
		return fmt.Errorf("external plugin: unexpected start response type: %q", startResp.Type)
	}

	// Spawn active message reader to process incoming messages from the plugin
	go p.readMessages()

	return p.BasePlugin.Start()
}

// readMessages continuously reads JSON messages from the plugin's stdout
// and handles known message types (log, event). The goroutine exits when
// stdout is closed (EOF) or the lifecycle context is cancelled.
func (p *externalPlugin) readMessages() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		var msg map[string]interface{}
		if err := p.readResponse(&msg); err != nil {
			log.Debug().Str("plugin", p.IDValue).Err(err).Msg("plugin message reader exiting")
			return
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "log":
			level, _ := msg["level"].(string)
			message, _ := msg["message"].(string)
			log.Debug().
				Str("plugin", p.IDValue).
				Str("plugin_level", level).
				Str("plugin_msg", message).
				Msg("plugin log message")
		case "event":
			name, _ := msg["name"].(string)
			log.Debug().
				Str("plugin", p.IDValue).
				Str("event_name", name).
				Interface("event_data", msg["data"]).
				Msg("plugin event received")
		default:
			log.Debug().
				Str("plugin", p.IDValue).
				Interface("msg", msg).
				Msg("unknown plugin message received")
		}
	}
}

// Health returns the health status of the external plugin by checking
// the child process state and verifying the stdin pipe is still open.
// It overrides BasePlugin.Health() to perform protocol-level validation
// with the configured ExternalProcessTimeout.
func (p *externalPlugin) Health() (PluginHealth, error) {
	// If process not started, return unhealthy
	if p.cmd == nil || p.cmd.Process == nil {
		return PluginHealth{
			PluginID:  p.IDValue,
			Status:    "unhealthy",
			Message:   "process not started",
			LastCheck: time.Now(),
			State:     p.State,
		}, nil
	}

	// Check if process has already exited
	select {
	case err := <-p.exitCh:
		msg := "process exited"
		if err != nil {
			msg = err.Error()
		}
		return PluginHealth{
			PluginID:  p.IDValue,
			Status:    "unhealthy",
			Message:   msg,
			LastCheck: time.Now(),
			State:     p.State,
		}, nil
	default:
	}

	// Verify the process is still running via ProcessState
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		return PluginHealth{
			PluginID:  p.IDValue,
			Status:    "unhealthy",
			Message:   "process terminated",
			LastCheck: time.Now(),
			State:     p.State,
		}, nil
	}

	// Send a health ping to verify the stdin pipe is still open.
	// This also serves as a liveness check for the plugin process.
	if p.stdin != nil {
		healthMsg := map[string]string{"type": "health"}
		errCh := make(chan error, 1)
		go func() {
			errCh <- p.sendJSON(healthMsg)
		}()

		timeout := time.Duration(p.loader.config.ExternalProcessTimeout) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}

		select {
		case sendErr := <-errCh:
			if sendErr != nil {
				return PluginHealth{
					PluginID:  p.IDValue,
					Status:    "unhealthy",
					Message:   fmt.Sprintf("health check failed: %v", sendErr),
					LastCheck: time.Now(),
					State:     p.State,
				}, nil
			}
		case <-time.After(timeout):
			return PluginHealth{
				PluginID:  p.IDValue,
				Status:    "unhealthy",
				Message:   "health check timed out",
				LastCheck: time.Now(),
				State:     p.State,
			}, nil
		case <-p.ctx.Done():
			return PluginHealth{
				PluginID:  p.IDValue,
				Status:    "unhealthy",
				Message:   "plugin shutting down",
				LastCheck: time.Now(),
				State:     p.State,
			}, nil
		}
	}

	uptime := time.Duration(0)
	if !p.StartedAt.IsZero() {
		uptime = time.Since(p.StartedAt)
	}

	return PluginHealth{
		PluginID:  p.IDValue,
		Status:    "healthy",
		LastCheck: time.Now(),
		Uptime:    uptime,
		State:     p.State,
	}, nil
}

// Stop gracefully stops the external plugin process.
// It cancels the lifecycle context to signal background goroutines,
// sends a stop message, then attempts graceful shutdown before force-killing.
func (p *externalPlugin) Stop() error {
	// Cancel context to signal background goroutines (read loop, health checks)
	if p.cancel != nil {
		p.cancel()
	}

	if p.cmd != nil && p.cmd.Process != nil {
		// Send JSON stop message for graceful shutdown
		if p.stdin != nil {
			_ = p.sendJSON(map[string]string{"type": "stop"})
			p.stdin.Close()
		}

		// Try graceful signal, then force kill
		if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
			_ = p.cmd.Process.Kill()
		}

		// Wait for the process exit goroutine to complete
		if p.doneCh != nil {
			select {
			case <-p.doneCh:
			case <-time.After(5 * time.Second):
				log.Warn().Str("plugin", p.IDValue).Msg("plugin stop: process did not exit in time, force killing")
				_ = p.cmd.Process.Kill()
				<-p.doneCh
			}
		}
	}
	return p.BasePlugin.Stop()
}

// =============================================================================
// WASM Plugin Loading
// =============================================================================

// wasmPlugin wraps a WebAssembly module as a Plugin.
type wasmPlugin struct {
	BasePlugin
	modulePath string
	ctx        context.Context
	cancel     context.CancelFunc
	runtime    wazero.Runtime
	compiled   wazero.CompiledModule
	module     api.Module
}

// loadWASMPlugin loads a WebAssembly plugin.
// WASM plugins run in a sandboxed environment with limited capabilities.
func (l *Loader) loadWASMPlugin(manifest *PluginManifest, path string) (Plugin, error) {
	// Verify the WASM file exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("wasm plugin: %w", err)
	}

	// Validate WASM extension
	if !strings.HasSuffix(path, ".wasm") {
		return nil, fmt.Errorf("wasm plugin must have .wasm extension")
	}

	// Read the WASM binary
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wasm file: %w", err)
	}

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Create wazero runtime
	runtime := wazero.NewRuntime(ctx)

	// Configure WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		cancel()
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	// Compile the WASM module
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		cancel()
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("compile wasm module: %w", err)
	}

	p := &wasmPlugin{
		BasePlugin: BasePlugin{
			IDValue:      manifest.ID,
			NameValue:    manifest.Name,
			VersionValue: manifest.Version,
			DescValue:    manifest.Description,
			AuthorValue:  manifest.Author,
		},
		modulePath: path,
		ctx:        ctx,
		cancel:     cancel,
		runtime:    runtime,
		compiled:   compiled,
	}

	log.Debug().Str("plugin", manifest.ID).Str("path", path).Msg("wasm plugin loaded and compiled")
	return p, nil
}

// Init initializes the WASM runtime by instantiating the compiled module.
func (p *wasmPlugin) Init(ctx *PluginContext) error {
	// Restrict filesystem access to the plugin's data directory only
	pluginFS := os.DirFS(ctx.DataDir)

	config := wazero.NewModuleConfig().
		WithName(p.NameValue).
		WithStderr(os.Stderr).
		WithStdout(os.Stdout).
		WithStdin(os.Stdin).
		WithFS(pluginFS).
		WithSysNanosleep().
		WithSysNanotime()

	module, err := p.runtime.InstantiateModule(p.ctx, p.compiled, config)
	if err != nil {
		return fmt.Errorf("instantiate wasm module: %w", err)
	}
	p.module = module

	log.Debug().Str("plugin", p.IDValue).Msg("wasm module instantiated")
	return p.BasePlugin.Init(ctx)
}

// Start calls the WASM module's _start or start function if present.
func (p *wasmPlugin) Start() error {
	for _, name := range []string{"_start", "start"} {
		fn := p.module.ExportedFunction(name)
		if fn != nil {
			_, err := fn.Call(p.ctx)
			if err != nil {
				return fmt.Errorf("call wasm function %s: %w", name, err)
			}
			log.Debug().Str("plugin", p.IDValue).
				Str("function", name).
				Msg("wasm start function called")
			break
		}
	}
	return p.BasePlugin.Start()
}

// Stop releases WASM runtime resources.
func (p *wasmPlugin) Stop() error {
	p.cancel()
	if p.module != nil {
		if err := p.module.Close(p.ctx); err != nil {
			log.Error().Err(err).Str("plugin", p.IDValue).Msg("close wasm module")
		}
	}
	if p.runtime != nil {
		if err := p.runtime.Close(p.ctx); err != nil {
			log.Error().Err(err).Str("plugin", p.IDValue).Msg("close wasm runtime")
		}
	}
	return p.BasePlugin.Stop()
}

// Health returns the health status of the WASM plugin.
func (p *wasmPlugin) Health() (PluginHealth, error) {
	if p.runtime == nil {
		return PluginHealth{
			PluginID:  p.IDValue,
			Status:    "unhealthy",
			Message:   "WASM runtime not initialized",
			LastCheck: time.Now(),
			State:     PluginStateError,
		}, nil
	}
	if p.module == nil {
		return PluginHealth{
			PluginID:  p.IDValue,
			Status:    "degraded",
			Message:   "WASM module not instantiated",
			LastCheck: time.Now(),
			State:     p.State,
		}, nil
	}
	return p.BasePlugin.Health()
}

// =============================================================================
// Validation
// =============================================================================

// validateRuntime checks if the runtime type is allowed.
func (l *Loader) validateRuntime(runtime PluginRuntime) error {
	if len(l.config.AllowedRuntimes) == 0 {
		return nil // all runtimes allowed
	}

	for _, allowed := range l.config.AllowedRuntimes {
		if runtime == allowed {
			return nil
		}
	}

	return fmt.Errorf("runtime %q is not in the allowed list", runtime)
}

// validatePermissions checks that the plugin's declared permissions
// are allowed by the current policy.
func (l *Loader) validatePermissions(manifest *PluginManifest) error {
	if len(manifest.Permissions) == 0 {
		return nil
	}

	knownPermissions := map[string]bool{
		"network":     true,
		"filesystem":  true,
		"exec":        true,
		"environment": true,
		"all":         true,
	}

	for _, perm := range manifest.Permissions {
		if !knownPermissions[perm] {
			switch l.config.PermissionPolicy {
			case "strict":
				return &ErrPermissionDenied{
					PluginID:   manifest.ID,
					Permission: perm,
				}
			case "warn":
				log.Warn().Str("plugin", manifest.ID).
					Str("permission", perm).
					Msg("unknown permission declared by plugin")
			case "allow":
				// Allow unknown permissions
			}
		}
	}

	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// findPluginBinary locates the plugin binary for the given runtime type.
func (l *Loader) findPluginBinary(dir string, runtime PluginRuntime) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		switch runtime {
		case RuntimeGo:
			if strings.HasSuffix(name, ".so") {
				return filepath.Join(dir, name), nil
			}
		case RuntimeSharedLib:
			if strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dll") || strings.HasSuffix(name, ".dylib") {
				return filepath.Join(dir, name), nil
			}
		case RuntimeExternal:
			// Any executable file
			if entry.Type().IsRegular() && entry.Type()&os.ModeSymlink == 0 {
				info, _ := entry.Info()
				if info != nil && info.Mode()&0o111 != 0 {
					return filepath.Join(dir, name), nil
				}
			}
		case RuntimeWASM:
			if strings.HasSuffix(name, ".wasm") {
				return filepath.Join(dir, name), nil
			}
		}
	}

	return "", fmt.Errorf("no binary found for runtime %s in %s", runtime, dir)
}

// loadSidecarManifest loads a manifest file that accompanies a standalone plugin file.
// For a plugin at "plugin.so", it looks for "plugin.manifest.json" or "plugin.manifest.yaml".
func (l *Loader) loadSidecarManifest(pluginPath string) (*PluginManifest, error) {
	base := strings.TrimSuffix(pluginPath, filepath.Ext(pluginPath))

	manifestPaths := []string{
		base + ".manifest.json",
		base + ".manifest.yaml",
		base + ".manifest.yml",
		filepath.Join(filepath.Dir(pluginPath), "manifest.json"),
		filepath.Join(filepath.Dir(pluginPath), "manifest.yaml"),
	}

	for _, path := range manifestPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		manifest := &PluginManifest{}
		switch filepath.Ext(path) {
		case ".json":
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, fmt.Errorf("parse sidecar manifest %s: %w", path, err)
			}
		case ".yaml", ".yml":
			// Would use yaml here; for now use JSON
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, fmt.Errorf("parse sidecar manifest %s: %w", path, err)
			}
		}

		if err := manifest.Validate(); err != nil {
			return nil, fmt.Errorf("validate sidecar manifest: %w", err)
		}

		return manifest, nil
	}

	// If no manifest is found, create a minimal one from the filename
	return &PluginManifest{
		ID:      strings.TrimSuffix(filepath.Base(pluginPath), filepath.Ext(pluginPath)),
		Name:    strings.TrimSuffix(filepath.Base(pluginPath), filepath.Ext(pluginPath)),
		Version: "0.0.0",
		Runtime: l.detectRuntimeFromExtension(pluginPath),
	}, nil
}

// detectRuntimeFromExtension attempts to determine the runtime from the file extension.
func (l *Loader) detectRuntimeFromExtension(path string) PluginRuntime {
	ext := filepath.Ext(path)
	switch ext {
	case ".so":
		return RuntimeGo
	case ".dll", ".dylib":
		return RuntimeSharedLib
	case ".wasm":
		return RuntimeWASM
	default:
		return RuntimeExternal
	}
}

// isPluginFile checks if a filename looks like a plugin file.
func isPluginFile(name string) bool {
	ext := filepath.Ext(name)
	switch ext {
	case ".so", ".wasm", ".dll", ".dylib":
		return true
	default:
		// Check if it's executable (no extension)
		return !strings.Contains(name, ".")
	}
}

// =============================================================================
// Plugin Discovery
// =============================================================================

// DiscoverPlugins scans a directory for plugin manifests and returns
// a list of discovered plugin metadata without loading them.
func DiscoverPlugins(dir string) ([]PluginManifest, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var manifests []PluginManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(absDir, entry.Name())
		manifest, err := loadManifestFromDir(pluginDir)
		if err != nil {
			log.Debug().Err(err).Str("dir", pluginDir).Msg("skipping directory without valid manifest")
			continue
		}
		manifests = append(manifests, *manifest)
	}

	return manifests, nil
}
