//
// Package plugins provides the plugin system for the Cosca platform.
// It defines the core Plugin interface, manifest schema, context, health
// reporting, and state management for all plugin types (Go, WASM, external).
//
// The plugin system follows a strict lifecycle:
//
//	Installed → Initialized → Started → Stopped
//	    ↓           ↓           ↓           ↓
//	   Error       Error       Error       Error
//
// Plugins communicate with the host through PluginContext, which provides
// access to configuration, logging, data directories, and a runtime API.

package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// PluginState
// =============================================================================

// PluginState represents the lifecycle state of a plugin.
type PluginState int

const (
	// PluginStateInstalled indicates the plugin has been installed but not initialized.
	PluginStateInstalled PluginState = iota
	// PluginStateInitialized indicates the plugin has been initialized successfully.
	PluginStateInitialized
	// PluginStateStarted indicates the plugin is running.
	PluginStateStarted
	// PluginStateStopped indicates the plugin has been gracefully stopped.
	PluginStateStopped
	// PluginStateError indicates the plugin encountered an error.
	PluginStateError
)

// String returns a human-readable representation of the plugin state.
func (s PluginState) String() string {
	switch s {
	case PluginStateInstalled:
		return "installed"
	case PluginStateInitialized:
		return "initialized"
	case PluginStateStarted:
		return "started"
	case PluginStateStopped:
		return "stopped"
	case PluginStateError:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// =============================================================================
// PluginRuntime
// =============================================================================

// PluginRuntime defines the execution environment for a plugin.
type PluginRuntime string

const (
	// RuntimeGo indicates a Go plugin loaded via plugin.Open.
	RuntimeGo PluginRuntime = "go"
	// RuntimeWASM indicates a WebAssembly plugin.
	RuntimeWASM PluginRuntime = "wasm"
	// RuntimeExternal indicates an external process plugin (child process, gRPC, etc.).
	RuntimeExternal PluginRuntime = "external"
	// RuntimeSharedLib indicates a shared library plugin (.so/.dll).
	RuntimeSharedLib PluginRuntime = "sharedlib"
)

// =============================================================================
// PluginDependency
// =============================================================================

// PluginDependency describes a dependency on another plugin.
type PluginDependency struct {
	// PluginID is the ID of the required plugin.
	PluginID string `yaml:"plugin_id" json:"plugin_id"`
	// Version specifies the required version constraint (e.g., ">=1.0.0").
	Version string `yaml:"version" json:"version"`
	// Optional indicates whether the dependency is optional.
	Optional bool `yaml:"optional" json:"optional"`
}

// =============================================================================
// PluginManifest
// =============================================================================

// PluginManifest defines the metadata and capabilities of a plugin.
// It is typically loaded from a manifest.yaml or manifest.json file
// within the plugin directory.
type PluginManifest struct {
	// ID is the unique plugin identifier (e.g., "cosca-plugin-search").
	ID string `yaml:"id" json:"id"`
	// Name is the human-readable plugin name.
	Name string `yaml:"name" json:"name"`
	// Version is the plugin version string (semver).
	Version string `yaml:"version" json:"version"`
	// Description is a short description of the plugin's purpose.
	Description string `yaml:"description" json:"description"`
	// Author is the plugin author or organization.
	Author string `yaml:"author" json:"author"`
	// Runtime specifies the plugin execution runtime.
	Runtime PluginRuntime `yaml:"runtime" json:"runtime"`
	// Permissions lists the required permissions (e.g., "network", "filesystem", "exec").
	Permissions []string `yaml:"permissions" json:"permissions"`
	// Dependencies lists other plugins this plugin depends on.
	Dependencies []PluginDependency `yaml:"dependencies" json:"dependencies"`
	// Hooks lists the hook points this plugin registers for.
	Hooks []string `yaml:"hooks" json:"hooks"`
	// ConfigSchema defines the JSON Schema for the plugin's configuration.
	ConfigSchema map[string]interface{} `yaml:"config_schema" json:"config_schema"`
	// EntryPoint is the path to the plugin entry point (for external/sharedlib runtimes).
	EntryPoint string `yaml:"entry_point,omitempty" json:"entry_point,omitempty"`
	// Checksum is the SHA-256 checksum of the plugin archive (for integrity verification).
	Checksum string `yaml:"checksum,omitempty" json:"checksum,omitempty"`
}

// Validate checks the manifest for required fields and valid values.
func (m *PluginManifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("plugin manifest: id is required")
	}
	if m.Name == "" {
		return fmt.Errorf("plugin manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin manifest: version is required")
	}
	switch m.Runtime {
	case RuntimeGo, RuntimeWASM, RuntimeExternal, RuntimeSharedLib:
		// valid
	case "":
		return fmt.Errorf("plugin manifest: runtime is required")
	default:
		return fmt.Errorf("plugin manifest: unsupported runtime %q", m.Runtime)
	}
	// Validate permissions against known set
	for _, p := range m.Permissions {
		switch p {
		case "network", "filesystem", "exec", "environment", "all":
			// valid
		default:
			return fmt.Errorf("plugin manifest: unknown permission %q", p)
		}
	}
	return nil
}

// =============================================================================
// PluginHealth
// =============================================================================

// PluginHealth reports the health status of a plugin.
type PluginHealth struct {
	// PluginID is the ID of the plugin.
	PluginID string `json:"plugin_id"`
	// Status is the health status string (healthy, degraded, unhealthy).
	Status string `json:"status"`
	// Message provides additional context about the health status.
	Message string `json:"message,omitempty"`
	// LastCheck is the timestamp of the last health check.
	LastCheck time.Time `json:"last_check"`
	// Uptime is the duration the plugin has been running.
	Uptime time.Duration `json:"uptime"`
	// State is the current lifecycle state.
	State PluginState `json:"state"`
}

// IsHealthy returns true if the plugin is in a healthy state.
func (h *PluginHealth) IsHealthy() bool {
	return h.Status == "healthy"
}

// =============================================================================
// PluginContext
// =============================================================================

// PluginContext provides the runtime environment for a plugin.
// It is passed to the plugin during initialization and remains
// available throughout the plugin's lifecycle.
type PluginContext struct {
	mu sync.RWMutex

	// Config holds the plugin-specific configuration.
	Config map[string]interface{}

	// Logger provides structured logging for the plugin.
	Logger PluginLogger

	// DataDir is the directory where the plugin can store persistent data.
	DataDir string

	// TempDir is the directory for temporary files.
	TempDir string

	// RuntimeAPI provides minimal access to Cosca runtime capabilities.
	RuntimeAPI RuntimeAPI
}

// PluginLogger is a minimal logging interface for plugins.
type PluginLogger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// RuntimeAPI provides a minimal set of operations that plugins can perform
// through the Cosca runtime. This is intentionally limited for security.
type RuntimeAPI interface {
	// GetConfig returns a configuration value by key.
	GetConfig(key string) (interface{}, error)
	// SetConfig sets a configuration value (if allowed).
	SetConfig(key string, value interface{}) error
	// EmitEvent emits an event to the event bus.
	EmitEvent(eventType string, data interface{}) error
	// RegisterHook registers a hook handler for the given hook point.
	RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error)
	// UnregisterHook removes a previously registered hook.
	UnregisterHook(hookID string) error
}

// NewPluginContext creates a new PluginContext with the given parameters.
func NewPluginContext(config map[string]interface{}, logger PluginLogger, dataDir string, runtimeAPI RuntimeAPI) *PluginContext {
	return &PluginContext{
		Config:     config,
		Logger:     logger,
		DataDir:    dataDir,
		TempDir:    dataDir + "/tmp",
		RuntimeAPI: runtimeAPI,
	}
}

// GetConfig returns a configuration value by key, with thread-safe access.
func (c *PluginContext) GetConfig(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.Config[key]
	if !ok {
		return nil, fmt.Errorf("config key %q not found", key)
	}
	return val, nil
}

// SetConfig sets a configuration value with thread-safe access.
func (c *PluginContext) SetConfig(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Config == nil {
		c.Config = make(map[string]interface{})
	}
	c.Config[key] = value
}

// =============================================================================
// PluginInfo
// =============================================================================

// PluginInfo provides a snapshot of a plugin's metadata and current state.
type PluginInfo struct {
	// Manifest is the plugin's manifest data.
	Manifest PluginManifest `json:"manifest"`
	// State is the current lifecycle state.
	State PluginState `json:"state"`
	// Enabled indicates whether the plugin is enabled.
	Enabled bool `json:"enabled"`
	// Health is the latest health check result.
	Health PluginHealth `json:"health"`
	// InstalledAt is when the plugin was installed.
	InstalledAt time.Time `json:"installed_at"`
	// UpdatedAt is when the plugin was last updated.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// =============================================================================
// Plugin Interface
// =============================================================================

// Plugin defines the interface that all plugins must implement.
// The lifecycle is managed by the PluginManager.
type Plugin interface {
	// ID returns the unique plugin identifier.
	ID() string

	// Name returns the human-readable plugin name.
	Name() string

	// Version returns the plugin version.
	Version() string

	// Description returns a short description of the plugin.
	Description() string

	// Author returns the plugin author or organization.
	Author() string

	// Init initializes the plugin with the given context.
	// This is called before Start().
	Init(ctx *PluginContext) error

	// Start starts the plugin's main operation.
	// The plugin should be ready to serve after this returns.
	Start() error

	// Stop gracefully stops the plugin.
	// The plugin should release all resources.
	Stop() error

	// Health returns the current health status of the plugin.
	Health() (PluginHealth, error)

	// Run executes the plugin with the given string parameters
	// and returns the result as a string. This is used by the
	// tool system to invoke plugins as chat.Tool implementations.
	Run(ctx context.Context, params string) (string, error)
}

// =============================================================================
// BasePlugin provides a convenient base implementation for plugins.
// Embed this struct in your plugin to satisfy the Plugin interface
// with reasonable defaults.
// =============================================================================

// BasePlugin is a helper struct that provides default implementations
// for the Plugin interface. Plugin authors can embed this struct and
// override only the methods they need.
type BasePlugin struct {
	IDValue      string
	NameValue    string
	VersionValue string
	DescValue    string
	AuthorValue  string
	StartedAt    time.Time
	State        PluginState
}

// ID returns the plugin ID.
func (b *BasePlugin) ID() string { return b.IDValue }

// Name returns the plugin name.
func (b *BasePlugin) Name() string { return b.NameValue }

// Version returns the plugin version.
func (b *BasePlugin) Version() string { return b.VersionValue }

// Description returns the plugin description.
func (b *BasePlugin) Description() string { return b.DescValue }

// Author returns the plugin author.
func (b *BasePlugin) Author() string { return b.AuthorValue }

// Run returns a default "not implemented" error.
// Plugin implementations should override this to provide
// execution logic for the tool system.
func (b *BasePlugin) Run(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("Run not implemented")
}

// Init initializes the base plugin state.
func (b *BasePlugin) Init(_ *PluginContext) error {
	b.State = PluginStateInitialized
	return nil
}

// Start marks the plugin as started.
func (b *BasePlugin) Start() error {
	b.StartedAt = time.Now()
	b.State = PluginStateStarted
	return nil
}

// Stop marks the plugin as stopped.
func (b *BasePlugin) Stop() error {
	b.State = PluginStateStopped
	return nil
}

// Health returns a default health check result.
func (b *BasePlugin) Health() (PluginHealth, error) {
	status := "healthy"
	uptime := time.Duration(0)
	if !b.StartedAt.IsZero() {
		uptime = time.Since(b.StartedAt)
	}
	return PluginHealth{
		PluginID:  b.IDValue,
		Status:    status,
		LastCheck: time.Now(),
		Uptime:    uptime,
		State:     b.State,
	}, nil
}

// =============================================================================
// Errors
// =============================================================================

// ErrPluginNotFound is returned when a plugin is not found.
var ErrPluginNotFound = fmt.Errorf("plugin not found")

// ErrPluginNotLoaded is returned when attempting to use an unloaded plugin.
var ErrPluginNotLoaded = fmt.Errorf("plugin not loaded")

// ErrPluginAlreadyInstalled is returned when attempting to install a plugin that already exists.
var ErrPluginAlreadyInstalled = fmt.Errorf("plugin already installed")

// ErrPluginDisabled is returned when attempting to use a disabled plugin.
var ErrPluginDisabled = fmt.Errorf("plugin is disabled")

// ErrDependencyMissing is returned when a plugin dependency is not satisfied.
type ErrDependencyMissing struct {
	PluginID   string
	Dependency PluginDependency
}

func (e *ErrDependencyMissing) Error() string {
	return fmt.Sprintf("plugin %q: missing dependency %q (version %s)",
		e.PluginID, e.Dependency.PluginID, e.Dependency.Version)
}

// ErrPermissionDenied is returned when a plugin lacks required permissions.
type ErrPermissionDenied struct {
	PluginID   string
	Permission string
}

func (e *ErrPermissionDenied) Error() string {
	return fmt.Sprintf("plugin %q: permission %q denied", e.PluginID, e.Permission)
}
