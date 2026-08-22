// Package config provides the configuration system for the Cosca platform.
// It supports loading configuration from multiple sources with the
// following precedence (highest to lowest):
//
//  1. Command-line flags
//  2. Environment variables (COSCA_*)
//  3. User config file   (~/.config/cosca/config.yaml)
//  4. Project config file (.cosca/config.yaml)
//  5. Default values
//
// Environment variables use the prefix COSCA_ followed by the config key
// in UPPER_SNAKE_CASE. Nested keys are separated by double underscores.
//
// Example:
//
//	COSCA_HOME=/custom/path
//	COSCA_DB__PATH=/data/cosca.db
//	COSCA_PROVIDER__MODEL=gpt-4o
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/models"
)

// =============================================================================
// Top-Level Configuration
// =============================================================================

// Config represents the complete Cosca configuration.
type Config struct {
	// Version of the config schema.
	Version string `yaml:"version" json:"version"`

	// Profile is the active configuration profile name.
	Profile string `yaml:"profile,omitempty" json:"profile,omitempty"`

	// Mode sets the runtime mode (dev, test, staging, production).
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

	// Verbose enables verbose debug output.
	Verbose bool `yaml:"verbose" json:"verbose"`

	// Paths defines all filesystem paths used by Cosca.
	Paths PathConfig `yaml:"paths" json:"paths"`

	// DB configures the SQLite database.
	DB DatabaseConfig `yaml:"db" json:"db"`

	// Provider configures LLM provider settings.
	Provider ProviderConfig `yaml:"provider" json:"provider"`

	// Embedding configures embedding model settings.
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`

	// Editor configures the built-in editor.
	Editor EditorConfig `yaml:"editor" json:"editor"`

	// Watch configures the file system watcher.
	Watch WatchConfig `yaml:"watch" json:"watch"`

	// Search configures search functionality.
	Search SearchConfig `yaml:"search" json:"search"`

	// Server configures the API and RPC servers.
	Server ServerConfig `yaml:"server" json:"server"`

	// Log configures logging.
	Log LogConfig `yaml:"log" json:"log"`

	// Cache configures caching.
	Cache CacheConfig `yaml:"cache" json:"cache"`

	// Network configures network settings.
	Network NetworkConfig `yaml:"network" json:"network"`

	// Features enables or disables optional features.
	Features FeatureConfig `yaml:"features" json:"features"`

	// Pipeline configures the pipeline.
	Pipeline PipelineConfig `yaml:"pipeline" json:"pipeline"`

	// Plugins configures the plugin system.
	Plugins PluginConfig `yaml:"plugins" json:"plugins"`

	// Performance sets resource limits and tuning parameters.
	Performance PerformanceConfig `yaml:"performance" json:"performance"`

	// ExecPolicy is the path to a YAML file defining shell command execution
	// policies (see internal/execpolicy). When set, commands executed through
	// the shell tool are evaluated against these rules before running. Empty
	// = no policy enforcement (current behavior).
	ExecPolicy string `yaml:"exec_policy,omitempty" json:"execPolicy,omitempty"`

	// Timeouts defines operation timeouts.
	Timeouts TimeoutConfig `yaml:"timeouts" json:"timeouts"`

	// UserDefined holds any extra keys not covered by the schema.
	UserDefined map[string]interface{} `yaml:",inline" json:"-"`

	// loadedFrom tracks which file (if any) the config was loaded from.
	loadedFrom string `yaml:"-" json:"-"`
	// contextWindowExplicit records whether the provider's context_window was
	// explicitly set in a config file (vs. inherited from hardcoded defaults).
	// Explicit values are honored; defaults are overridden by the models.dev
	// cache so Cosca uses real per-model context windows.
	contextWindowExplicit bool `yaml:"-" json:"-"`
}

// =============================================================================
// Sub-Configuration Types
// =============================================================================

// PathConfig defines filesystem paths.
type PathConfig struct {
	// Home is the Cosca home directory (default: ~/.config/cosca).
	Home string `yaml:"home" json:"home"`
	// Project is the project root directory.
	Project string `yaml:"project,omitempty" json:"project,omitempty"`
	// Runtime is the runtime directory for ephemeral state.
	Runtime string `yaml:"runtime" json:"runtime"`
	// Data is the data directory for persistent state.
	Data string `yaml:"data" json:"data"`
	// Cache is the cache directory.
	Cache string `yaml:"cache" json:"cache"`
	// Logs is the log output directory.
	Logs string `yaml:"logs" json:"logs"`
	// Temp is the temporary files directory.
	Temp string `yaml:"temp" json:"temp"`
	// Plugins is the plugins directory.
	Plugins string `yaml:"plugins" json:"plugins"`
	// Backups is the backups directory.
	Backups string `yaml:"backups" json:"backups"`
}

// DatabaseConfig configures the SQLite database.
type DatabaseConfig struct {
	// Path is the path to the SQLite database file.
	Path string `yaml:"path" json:"path"`
	// WALMode enables Write-Ahead Logging mode.
	WALMode bool `yaml:"wal_mode" json:"walMode"`
	// PageSize is the database page size in bytes.
	PageSize int `yaml:"page_size" json:"pageSize"`
	// CacheSizeKB is the database cache size in KB.
	CacheSizeKB int `yaml:"cache_size_kb" json:"cacheSizeKb"`
	// MaxOpenConns is the max open database connections.
	MaxOpenConns int `yaml:"max_open_conns" json:"maxOpenConns"`
	// MaxIdleConns is the max idle database connections.
	MaxIdleConns int `yaml:"max_idle_conns" json:"maxIdleConns"`
	// ConnMaxLifetime is the max connection lifetime.
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"connMaxLifetime"`
	// ConnMaxIdleTime is the max idle connection time.
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"connMaxIdleTime"`
	// EnableFTS5 enables Full-Text Search extension.
	EnableFTS5 bool `yaml:"enable_fts5" json:"enableFts5"`
	// EnableVectorExt enables vector search extension (sqlite-vec).
	EnableVectorExt bool `yaml:"enable_vector_ext" json:"enableVectorExt"`
	// BackupInterval is how often to backup the database.
	BackupInterval time.Duration `yaml:"backup_interval" json:"backupInterval"`
}

// ProviderConfig configures the LLM/embedding provider.
type ProviderConfig struct {
	// Name is the provider name (openai, anthropic, ollama, etc.).
	Name string `yaml:"name" json:"name"`
	// Model is the model identifier.
	Model string `yaml:"model" json:"model"`
	// ContextWindow is the model's context window in tokens.
	ContextWindow int64 `yaml:"context_window" json:"contextWindow"`
	// APIKey is the API key for authentication.
	// APIKey is deliberately excluded from the default JSON representation. It
	// is kept in memory for provider compatibility; Save uses an encrypted
	// legacy wire value unless APIKeyEnv is configured.
	APIKey string `yaml:"api_key" json:"-"`
	// APIKeyEnv names an environment variable containing the credential. This
	// is the preferred persistence mechanism for new configurations.
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"apiKeyEnv,omitempty"`
	// BaseURL is the base URL for the provider API.
	BaseURL string `yaml:"base_url" json:"baseUrl"`
	// MaxTokens is the max tokens for completion.
	MaxTokens int `yaml:"max_tokens" json:"maxTokens"`
	// Temperature is the sampling temperature.
	Temperature float64 `yaml:"temperature" json:"temperature"`
	// Timeout is the request timeout duration.
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	// MaxRetries is the max retry count on failure.
	MaxRetries int `yaml:"max_retries" json:"maxRetries"`
	// RateLimitPerMin is the max requests per minute.
	RateLimitPerMin int `yaml:"rate_limit_per_min" json:"rateLimitPerMin"`
}

// EmbeddingConfig configures embedding models.
type EmbeddingConfig struct {
	// Model is the embedding model name.
	Model string `yaml:"model" json:"model"`
	// Dimensions is the embedding vector dimensions.
	Dimensions int `yaml:"dimensions" json:"dimensions"`
	// BatchSize is the batch size for embedding requests.
	BatchSize int `yaml:"batch_size" json:"batchSize"`
	// Provider is the embedding provider name.
	Provider string `yaml:"provider" json:"provider"`

	// Digest pina o digest esperado do modelo (L376) — identidade é parte
	// da integridade do índice.
	Digest string `yaml:"digest,omitempty" json:"digest,omitempty"`
	// BaseURL is the base URL for the embedding provider API. When set it
	// points the provider at a custom endpoint (e.g. a local OpenAI-compatible
	// embeddings server such as llama-server) instead of the provider's default
	// remote endpoint.
	BaseURL string `yaml:"base_url" json:"baseURL"`
	// APIKey is the API key for the embedding provider API. Like the provider
	// API key it is excluded from JSON output; it is only read from the config
	// file or environment.
	APIKey string `yaml:"api_key" json:"-"`
}

// EditorConfig configures the built-in TUI editor.
type EditorConfig struct {
	// Name is the editor executable.
	Name string `yaml:"name" json:"name"`
	// Theme is the editor color theme.
	Theme string `yaml:"theme" json:"theme"`
	// FontSize is the editor font size.
	FontSize int `yaml:"font_size" json:"fontSize"`
	// TabSize is the tab width in spaces.
	TabSize int `yaml:"tab_size" json:"tabSize"`
	// LineNumbers shows line numbers.
	LineNumbers bool `yaml:"line_numbers" json:"lineNumbers"`
	// WordWrap enables word wrapping.
	WordWrap bool `yaml:"word_wrap" json:"wordWrap"`
	// AutoSave enables auto-save with the given interval.
	AutoSave bool `yaml:"auto_save" json:"autoSave"`
	// AutoSaveInterval is the auto-save interval.
	AutoSaveInterval time.Duration `yaml:"auto_save_interval" json:"autoSaveInterval"`
}

// WatchConfig configures the file system watcher.
type WatchConfig struct {
	// Enabled enables file watching.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Debounce is the debounce duration for events.
	Debounce time.Duration `yaml:"debounce" json:"debounce"`
	// Recursive watches subdirectories recursively.
	Recursive bool `yaml:"recursive" json:"recursive"`
	// ExcludePatterns are glob patterns to exclude.
	ExcludePatterns []string `yaml:"exclude_patterns" json:"excludePatterns"`
	// IncludePatterns are glob patterns to include.
	IncludePatterns []string `yaml:"include_patterns" json:"includePatterns"`
	// MaxFileSize is the max file size in bytes to watch.
	MaxFileSize int64 `yaml:"max_file_size" json:"maxFileSize"`
}

// SearchConfig configures search functionality.
type SearchConfig struct {
	// DefaultLimit is the default search result limit.
	DefaultLimit int `yaml:"default_limit" json:"defaultLimit"`
	// MaxResults is the maximum allowed results.
	MaxResults int `yaml:"max_results" json:"maxResults"`
	// MinScore is the minimum similarity score (0.0 - 1.0).
	MinScore float64 `yaml:"min_score" json:"minScore"`
	// EnableVector enables vector search.
	EnableVector bool `yaml:"enable_vector" json:"enableVector"`
	// EnableFullText enables full-text search.
	EnableFullText bool `yaml:"enable_fulltext" json:"enableFullText"`
	// EnableGraph enables graph-based search.
	EnableGraph bool `yaml:"enable_graph" json:"enableGraph"`
	// IndexInterval is the indexing interval.
	IndexInterval time.Duration `yaml:"index_interval" json:"indexInterval"`
}

// ServerConfig configures the API and RPC servers.
type ServerConfig struct {
	// Host is the bind address.
	Host string `yaml:"host" json:"host"`
	// APIPort is the HTTP API port.
	APIPort int `yaml:"api_port" json:"apiPort"`
	// MetricsPort is the metrics endpoint port.
	MetricsPort int `yaml:"metrics_port" json:"metricsPort"`
	// RPCPort is the gRPC port.
	RPCPort int `yaml:"rpc_port" json:"rpcPort"`
	// TLS enables TLS for all servers.
	TLS bool `yaml:"tls" json:"tls"`
	// TLSCert is the TLS certificate path.
	TLSCert string `yaml:"tls_cert,omitempty" json:"tlsCert,omitempty"`
	// TLSKey is the TLS key path.
	TLSKey string `yaml:"tls_key,omitempty" json:"tlsKey,omitempty"`
	// ReadTimeout is the read timeout.
	ReadTimeout time.Duration `yaml:"read_timeout" json:"readTimeout"`
	// WriteTimeout is the write timeout.
	WriteTimeout time.Duration `yaml:"write_timeout" json:"writeTimeout"`
	// MaxHeaderBytes is the max request header size.
	MaxHeaderBytes int `yaml:"max_header_bytes" json:"maxHeaderBytes"`
}

// LogConfig configures logging.
type LogConfig struct {
	// Level is the log level (debug, info, warn, error, fatal, panic).
	Level string `yaml:"level" json:"level"`
	// Format is the log format (console, json).
	Format string `yaml:"format" json:"format"`
	// Output is the log output path (empty = stderr).
	Output string `yaml:"output,omitempty" json:"output,omitempty"`
	// MaxSizeMB is the max log file size in MB before rotation.
	MaxSizeMB int `yaml:"max_size_mb" json:"maxSizeMb"`
	// MaxBackups is the max number of rotated log files.
	MaxBackups int `yaml:"max_backups" json:"maxBackups"`
	// MaxAgeDays is the max age of log files in days.
	MaxAgeDays int `yaml:"max_age_days" json:"maxAgeDays"`
	// Compress enables log file compression.
	Compress bool `yaml:"compress" json:"compress"`
	// ShowCaller shows the caller in log entries.
	ShowCaller bool `yaml:"show_caller" json:"showCaller"`
}

// CacheConfig configures caching behavior.
type CacheConfig struct {
	// Type is the cache backend type (memory, sqlite, redis, etc.).
	Type string `yaml:"type" json:"type"`
	// Size is the max number of cache entries.
	Size int `yaml:"size" json:"size"`
	// TTL is the default TTL for cache entries.
	TTL time.Duration `yaml:"ttl" json:"ttl"`
	// EnableCompression enables value compression.
	EnableCompression bool `yaml:"enable_compression" json:"enableCompression"`
}

// NetworkConfig configures network settings.
type NetworkConfig struct {
	// ProxyURL is the HTTP proxy URL.
	ProxyURL string `yaml:"proxy_url,omitempty" json:"proxyUrl,omitempty"`
	// NoProxy are hosts to exclude from proxy.
	NoProxy []string `yaml:"no_proxy,omitempty" json:"noProxy,omitempty"`
	// InsecureSkipVerify disables TLS verification (development only).
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecureSkipVerify"`
	// DialTimeout is the TCP dial timeout.
	DialTimeout time.Duration `yaml:"dial_timeout" json:"dialTimeout"`
	// KeepAlive is the TCP keep-alive interval.
	KeepAlive time.Duration `yaml:"keep_alive" json:"keepAlive"`
	// MaxIdleConns is the max idle HTTP connections.
	MaxIdleConns int `yaml:"max_idle_conns" json:"maxIdleConns"`
	// IdleConnTimeout is the idle connection timeout.
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout" json:"idleConnTimeout"`
	// TLSHandshakeTimeout is the TLS handshake timeout.
	TLSHandshakeTimeout time.Duration `yaml:"tls_handshake_timeout" json:"tlsHandshakeTimeout"`
	// ResponseHeaderTimeout is the response header timeout.
	ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout" json:"responseHeaderTimeout"`
	// ExpectContinueTimeout is the expect continue timeout.
	ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout" json:"expectContinueTimeout"`
}

// FeatureConfig enables or disables features.
type FeatureConfig struct {
	// Metrics enables metrics collection and export.
	Metrics bool `yaml:"metrics" json:"metrics"`
	// Telemetry enables anonymous usage telemetry.
	Telemetry bool `yaml:"telemetry" json:"telemetry"`
	// AutoUpdate enables automatic update checks.
	AutoUpdate bool `yaml:"auto_update" json:"autoUpdate"`
	// VectorSearch enables vector search capabilities.
	VectorSearch bool `yaml:"vector_search" json:"vectorSearch"`
	// GraphSearch enables graph-based search.
	GraphSearch bool `yaml:"graph_search" json:"graphSearch"`
	// FileWatching enables file system watcher.
	FileWatching bool `yaml:"file_watching" json:"fileWatching"`
	// PluginSystem enables the plugin system.
	PluginSystem bool `yaml:"plugin_system" json:"pluginSystem"`
	// ShellCompletion enables shell completion generation.
	ShellCompletion bool `yaml:"shell_completion" json:"shellCompletion"`
	// Experimental enables experimental features.
	Experimental bool `yaml:"experimental" json:"experimental"`
}

// PipelineConfig configures the pipeline.
type PipelineConfig struct {
	// Enabled enables the pipeline.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// AutoFix enables automatic fixes.
	AutoFix bool `yaml:"auto_fix" json:"autoFix"`
}

// PluginConfig configures the plugin system.
type PluginConfig struct {
	// Enabled enables the plugin system.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Dir is the plugins directory path.
	Dir string `yaml:"dir" json:"dir"`
	// AllowedPolicies defines plugin security policies.
	AllowedPolicies []string `yaml:"allowed_policies,omitempty" json:"allowedPolicies,omitempty"`
	// Timeout is the max execution time for plugins.
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	// MaxMemoryMB is the max memory per plugin.
	MaxMemoryMB int `yaml:"max_memory_mb" json:"maxMemoryMb"`
}

// PerformanceConfig sets resource limits and tuning.
type PerformanceConfig struct {
	// MaxMemoryMB is the max memory usage target.
	MaxMemoryMB int `yaml:"max_memory_mb" json:"maxMemoryMb"`
	// MaxOpenFiles is the max open file descriptors.
	MaxOpenFiles int `yaml:"max_open_files" json:"maxOpenFiles"`
	// MaxConcurrentOps is the max concurrent operations.
	MaxConcurrentOps int `yaml:"max_concurrent_ops" json:"maxConcurrentOps"`
	// BatchSize is the default batch processing size.
	BatchSize int `yaml:"batch_size" json:"batchSize"`
	// WorkerPoolSize is the worker pool goroutine count.
	WorkerPoolSize int `yaml:"worker_pool_size" json:"workerPoolSize"`
	// PrefetchSize is the prefetch buffer size.
	PrefetchSize int `yaml:"prefetch_size" json:"prefetchSize"`
}

// TimeoutConfig defines operation timeouts.
type TimeoutConfig struct {
	// Command is the default command execution timeout.
	Command time.Duration `yaml:"command" json:"command"`
	// Agent is the agent execution timeout.
	Agent time.Duration `yaml:"agent" json:"agent"`
	// Skill is the skill execution timeout.
	Skill time.Duration `yaml:"skill" json:"skill"`
	// Workflow is the workflow execution timeout.
	Workflow time.Duration `yaml:"workflow" json:"workflow"`
	// Request is the HTTP request timeout.
	Request time.Duration `yaml:"request" json:"request"`
	// GracefulShutdown is the graceful shutdown timeout.
	GracefulShutdown time.Duration `yaml:"graceful_shutdown" json:"gracefulShutdown"`
	// HealthCheck is the health check interval.
	HealthCheck time.Duration `yaml:"health_check" json:"healthCheck"`
}

// =============================================================================
// Configuration Loading
// =============================================================================

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	homeDir := UserHomeDir()
	coscaHome := filepath.Join(homeDir, ".config", "cosca")
	projectDir := "."

	// Try to detect project root from current directory
	if cwd, err := os.Getwd(); err == nil {
		projectDir = cwd
	}

	return &Config{
		Version: "1.0",
		Profile: "default",
		Mode:    "development",
		Paths: PathConfig{
			Home:    coscaHome,
			Project: projectDir,
			Runtime: filepath.Join(coscaHome, DefaultRuntimeDir),
			Data:    filepath.Join(coscaHome, DefaultDataDir),
			Cache:   filepath.Join(coscaHome, DefaultCacheDir),
			Logs:    filepath.Join(coscaHome, DefaultLogsDir),
			Temp:    filepath.Join(coscaHome, DefaultTempDir),
			Plugins: filepath.Join(coscaHome, DefaultPluginsDir),
			Backups: filepath.Join(coscaHome, DefaultBackupDir),
		},
		DB: DatabaseConfig{
			Path:            filepath.Join(coscaHome, "cosca.db"),
			WALMode:         true,
			PageSize:        DefaultDBPageSize,
			CacheSizeKB:     DefaultDBCacheSizeKB,
			MaxOpenConns:    DefaultMaxOpenDBConn,
			MaxIdleConns:    DefaultMaxIdleDBConn,
			ConnMaxLifetime: DefaultDBConnMaxLifetime,
			ConnMaxIdleTime: DefaultDBConnMaxIdleTime,
			EnableFTS5:      true,
			EnableVectorExt: false,
			BackupInterval:  1 * time.Hour,
		},
		Provider: ProviderConfig{
			Name:            DefaultProvider,
			Model:           DefaultProviderModel,
			ContextWindow:   DefaultProviderContextWindow,
			MaxTokens:       DefaultProviderMaxTokens,
			Temperature:     DefaultProviderTemperature,
			Timeout:         DefaultRequestTimeout,
			MaxRetries:      DefaultMaxRetries,
			RateLimitPerMin: DefaultRateLimitPerMin,
		},
		Embedding: EmbeddingConfig{
			Model:      DefaultEmbeddingModel,
			Dimensions: DefaultEmbeddingDimensions,
			BatchSize:  DefaultBatchSize,
			Provider:   DefaultProvider,
		},
		Editor: EditorConfig{
			Name:             DefaultEditor,
			Theme:            DefaultEditorTheme,
			FontSize:         DefaultEditorFontSize,
			TabSize:          4,
			LineNumbers:      true,
			WordWrap:         true,
			AutoSave:         true,
			AutoSaveInterval: 30 * time.Second,
		},
		Watch: WatchConfig{
			Enabled:         DefaultEnableWatch,
			Debounce:        DefaultWatchTimeout,
			Recursive:       true,
			ExcludePatterns: []string{".git/**", "node_modules/**", "*.log", "*.tmp"},
			MaxFileSize:     10 * 1024 * 1024, // 10 MB
		},
		Search: SearchConfig{
			DefaultLimit:   DefaultSearchResultLimit,
			MaxResults:     DefaultSearchMaxResults,
			MinScore:       0.7,
			EnableVector:   DefaultEnableVectorSearch,
			EnableFullText: true,
			EnableGraph:    DefaultEnableGraphSearch,
			IndexInterval:  DefaultIndexInterval,
		},
		Server: ServerConfig{
			Host:           DefaultHost,
			APIPort:        DefaultAPIPort,
			MetricsPort:    DefaultMetricsPort,
			RPCPort:        DefaultRPCPort,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MB
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "console",
			MaxSizeMB:  DefaultLogMaxSizeMB,
			MaxBackups: DefaultLogMaxBackups,
			MaxAgeDays: DefaultLogMaxAgeDays,
			Compress:   true,
			ShowCaller: false,
		},
		Cache: CacheConfig{
			Type:              "memory",
			Size:              DefaultCacheSize,
			TTL:               DefaultCacheTTL,
			EnableCompression: false,
		},
		Network: NetworkConfig{
			DialTimeout:           10 * time.Second,
			KeepAlive:             30 * time.Second,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Features: FeatureConfig{
			Metrics:         DefaultEnableMetrics,
			Telemetry:       DefaultEnableTelemetry,
			AutoUpdate:      DefaultEnableAutoUpdate,
			VectorSearch:    DefaultEnableVectorSearch,
			GraphSearch:     DefaultEnableGraphSearch,
			FileWatching:    DefaultEnableWatch,
			PluginSystem:    DefaultEnablePluginSystem,
			ShellCompletion: true,
			Experimental:    false,
		},
		Plugins: PluginConfig{
			Enabled:         DefaultEnablePluginSystem,
			Dir:             filepath.Join(coscaHome, DefaultPluginsDir),
			AllowedPolicies: []string{"network", "filesystem", "exec"},
			Timeout:         30 * time.Second,
			MaxMemoryMB:     128,
		},
		Performance: PerformanceConfig{
			MaxMemoryMB:      DefaultMaxMemoryMB,
			MaxOpenFiles:     DefaultMaxOpenFiles,
			MaxConcurrentOps: DefaultMaxConcurrentOps,
			BatchSize:        DefaultBatchSize,
			WorkerPoolSize:   runtime.NumCPU(),
			PrefetchSize:     1024,
		},
		Timeouts: TimeoutConfig{
			Command:          DefaultCommandTimeout,
			Agent:            DefaultAgentTimeout,
			Skill:            DefaultSkillTimeout,
			Workflow:         DefaultWorkflowTimeout,
			Request:          DefaultRequestTimeout,
			GracefulShutdown: DefaultGracefulShutdown,
			HealthCheck:      DefaultHealthCheckInterval,
		},
	}
}

// Load loads configuration from multiple sources.
// Precedence: defaults < project config < user config < env vars < flags.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// 1. User-level config first (~/.config/cosca/config.yaml) — base layer
	userCfgPath := filepath.Join(cfg.Paths.Home, "config.yaml")
	if _, err := os.Stat(userCfgPath); err == nil {
		if err := cfg.loadFromFile(userCfgPath); err != nil {
			log.Warn().Err(err).Str("path", userCfgPath).
				Msg("failed to load user config")
		} else {
			cfg.loadedFrom = userCfgPath
			log.Debug().Str("path", userCfgPath).Msg("loaded user config")
		}
	}

	// 2. Project-level config on top (.cosca/config.yaml) — overrides user
	//    O project config tem precedencia. Valores definidos no projeto
	//    sobrescrevem os do user config (ex: provider.model, provider.base_url).
	projectCfgPath := filepath.Join(cfg.Paths.Project, DefaultCoscaProjectDir, "config.yaml")
	if _, err := os.Stat(projectCfgPath); err == nil {
		if err := cfg.loadFromFile(projectCfgPath); err != nil {
			log.Warn().Err(err).Str("path", projectCfgPath).
				Msg("failed to load project config")
		} else {
			cfg.loadedFrom = projectCfgPath
			log.Debug().Str("path", projectCfgPath).Msg("loaded project config")
		}
	}

	// 3. Apply environment variables (override config file values)
	cfg.loadFromEnv()

	// 4. Override the hardcoded default context window with the real
	//    per-model value from the models.dev cache when the model is known.
	//    This is an enhancement: an absent/stale cache keeps current behavior.
	cfg.applyModelsContextWindow()

	// 5. Validate
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("configuration validation: %w", err)
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a specific YAML file.
func LoadFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	if err := cfg.loadFromFile(path); err != nil {
		return nil, err
	}
	cfg.loadedFrom = path
	cfg.applyModelsContextWindow()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyModelsContextWindow overrides the provider context window with the real
// per-model value from the models.dev cache when the model is known. It never
// fails: an absent or stale cache, an unknown model, or a broken file simply
// leaves the current (hardcoded) value untouched. Explicitly configured
// context_window values in a config file are always honored.
func (c *Config) applyModelsContextWindow() {
	if c.contextWindowExplicit {
		return
	}
	if c.Provider.Model == "" {
		return
	}
	cachePath := models.CachePathIn(c.Paths.Home)
	cl := models.ContextWindowFor(cachePath, c.Provider.Model)
	if cl <= 0 {
		return
	}
	c.Provider.ContextWindow = cl
}

// =============================================================================
// File I/O
// =============================================================================

// loadFromFile reads a YAML file and merges it into the config.
func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	// Detect whether the provider context_window was explicitly set in this
	// file. Explicit values must not be overridden by the models cache;
	// hardcoded defaults and zeros may be.
	var probe struct {
		Provider struct {
			ContextWindow *int64 `yaml:"context_window"`
		} `yaml:"provider"`
	}
	if err := yaml.Unmarshal(data, &probe); err == nil && probe.Provider.ContextWindow != nil {
		c.contextWindowExplicit = *probe.Provider.ContextWindow != 0
	}
	// APIKey is intentionally not obtained through generic JSON output, but
	// YAML loading remains compatible with old files. Keep this migration
	// boundary explicit and never include the value in diagnostics or errors.
	var wire struct {
		Provider struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"provider"`
	}
	if err := yaml.Unmarshal(data, &wire); err == nil && wire.Provider.APIKey != "" {
		c.Provider.APIKey = wire.Provider.APIKey
	}

	// Decrypt encrypted API keys at rest (backward-compat: plaintext is left as-is).
	if decErr := c.Provider.DecryptAPIKeys(); decErr != nil {
		// Log but continue — a corrupted encrypted key is better than
		// crashing the entire config load.
		log.Warn().Err(decErr).Msg("failed to decrypt provider API key")
	}
	if c.Provider.APIKeyEnv != "" {
		if value := os.Getenv(c.Provider.APIKeyEnv); value != "" {
			c.Provider.APIKey = value
		}
	}

	return nil
}

// Save writes the configuration to a YAML file.
// API keys are encrypted at rest before writing. The in-memory config
// retains plaintext keys so callers can continue using them after Save.
func (c *Config) Save(path string) error {
	if err := validateConfigWritePath(path); err != nil {
		return err
	}
	// Never marshal a plaintext credential. External references are preferred;
	// the encrypted legacy representation is retained only for compatibility.
	wire := *c
	wire.Provider = c.Provider
	if wire.Provider.APIKeyEnv != "" {
		wire.Provider.APIKey = ""
	} else if encErr := wire.Provider.EncryptAPIKeys(); encErr != nil {
		log.Warn().Err(encErr).Msg("failed to encrypt provider API key for save")
		return fmt.Errorf("protect provider credential for save: %w", encErr)
	}

	data, err := yaml.Marshal(&wire)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink config directory")
	}
	if err := rejectSymlinkComponents(dir); err != nil {
		return err
	}
	// Config files hold provider API keys — owner-only directory (M6b).
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := rejectSymlinkComponents(dir); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure config directory: %w", err)
	}
	return atomicOwnerOnlyWrite(path, data)
}

func atomicOwnerOnlyWrite(path string, data []byte) error {
	if err := rejectSymlinkComponents(filepath.Dir(path)); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write symlink config file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect config file: %w", err)
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".config.yaml.tmp-*")
	if err != nil {
		return fmt.Errorf("create config tempfile: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return fmt.Errorf("secure config tempfile: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write config tempfile: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync config tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close config tempfile: %w", err)
	}
	// Re-check immediately before replacement. This narrows, but cannot remove,
	// the race between path inspection and rename on platforms without a
	// portable renameat2/RESOLVE_* API; callers must protect the config parent
	// from untrusted concurrent writers for complete TOCTOU protection.
	if err := validateConfigWritePath(path); err != nil {
		return err
	}
	if err := rejectSymlinkComponents(dir); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write symlink config file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect config file before rename: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename config tempfile: %w", err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open config directory for sync: %w", err)
	}
	// Directory fsync is a POSIX durability guarantee; on Windows
	// FlushFileBuffers on a directory handle returns ERROR_ACCESS_DENIED, and
	// NTFS rename semantics are already durable — skip it. The Close below
	// still runs on every platform.
	if runtime.GOOS != "windows" {
		if err := d.Sync(); err != nil {
			_ = d.Close()
			return fmt.Errorf("sync config directory: %w", err)
		}
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("close config directory: %w", err)
	}
	return nil
}

func validateConfigWritePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if filepath.Clean(absPath) == string(filepath.Separator) {
		return fmt.Errorf("refusing to save configuration at filesystem root")
	}
	dir := filepath.Dir(absPath)
	if filepath.Clean(dir) == string(filepath.Separator) {
		return fmt.Errorf("refusing to save configuration with filesystem root as parent")
	}
	return nil
}

func rejectSymlinkComponents(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve config directory: %w", err)
	}
	volume, rest := filepath.VolumeName(abs), strings.TrimPrefix(abs, filepath.VolumeName(abs))
	current := volume + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(rest, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect config path component: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink config path component")
		}
	}
	return nil
}

// SaveDefault saves the configuration to the default user config path.
func (c *Config) SaveDefault() error {
	path := filepath.Join(c.Paths.Home, "config.yaml")
	return c.Save(path)
}

// =============================================================================
// Environment Variable Loading
// =============================================================================

// loadFromEnv reads environment variables with the COSCA_ prefix.
// It uses a simple key-path mapping: COSCA_DB__PATH -> Config.DB.Path.
func (c *Config) loadFromEnv() {
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "COSCA_") {
			continue
		}
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimPrefix(parts[0], "COSCA_")
		envMap[key] = parts[1]
	}

	// Apply known env vars
	if v, ok := envMap["HOME"]; ok {
		c.Paths.Home = v
	}
	if v, ok := envMap["PROJECT"]; ok {
		c.Paths.Project = v
	}
	if v, ok := envMap["EDITOR"]; ok {
		c.Editor.Name = v
	}
	if v, ok := envMap["MODE"]; ok {
		c.Mode = v
	}
	if v, ok := envMap["VERBOSE"]; ok {
		c.Verbose = v == "true" || v == "1"
	}
	if v, ok := envMap["LOG_LEVEL"]; ok {
		c.Log.Level = v
	}
	if v, ok := envMap["PROVIDER__API_KEY"]; ok {
		c.Provider.APIKey = v
	}
	if v, ok := envMap["PROVIDER__MODEL"]; ok {
		c.Provider.Model = v
	}
	if v, ok := envMap["PROVIDER__BASE_URL"]; ok {
		c.Provider.BaseURL = v
	}
	if v, ok := envMap["EMBEDDING__BASE_URL"]; ok {
		c.Embedding.BaseURL = v
	}
	if v, ok := envMap["DB__PATH"]; ok {
		c.DB.Path = v
	}
	if v, ok := envMap["FEATURES__TELEMETRY"]; ok {
		c.Features.Telemetry = v == "true" || v == "1"
	}
	if v, ok := envMap["FEATURES__METRICS"]; ok {
		c.Features.Metrics = v == "true" || v == "1"
	}
	if v, ok := envMap["NETWORK__PROXY_URL"]; ok {
		c.Network.ProxyURL = v
	}
	if v, ok := envMap["DEV"]; ok {
		if v == "true" || v == "1" {
			c.Mode = "development"
		}
	}
}

// =============================================================================
// Validation
// =============================================================================

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	var errs []string

	// Version check
	if c.Version == "" {
		errs = append(errs, "config version is required")
	}

	// Path validation
	if c.Paths.Home == "" {
		errs = append(errs, "paths.home is required")
	}

	// Database validation
	if c.DB.MaxOpenConns < 1 {
		errs = append(errs, "db.max_open_conns must be >= 1")
	}
	if c.DB.PageSize < 512 || c.DB.PageSize > 65536 {
		errs = append(errs, "db.page_size must be between 512 and 65536")
	}

	// Network security validation
	if c.Network.InsecureSkipVerify && c.Mode != "development" {
		errs = append(errs, "network.insecure_skip_verify is only allowed in development mode")
	}

	// Provider validation
	if c.Provider.Name != "" {
		switch c.Provider.Name {
		case "openai", "anthropic", "ollama", "azure", "google", "aws-bedrock", "custom",
			"deepseek", "mistral", "groq":
			// valid
		default:
			errs = append(errs, fmt.Sprintf("unsupported provider: %s", c.Provider.Name))
		}
	}
	if c.Provider.Temperature < 0 || c.Provider.Temperature > 2 {
		errs = append(errs, "provider.temperature must be between 0 and 2")
	}
	if c.Provider.MaxTokens < 1 {
		errs = append(errs, "provider.max_tokens must be >= 1")
	}

	// Server validation
	if c.Server.APIPort < 1 || c.Server.APIPort > 65535 {
		errs = append(errs, "server.api_port must be between 1 and 65535")
	}
	if c.Server.RPCPort < 1 || c.Server.RPCPort > 65535 {
		errs = append(errs, "server.rpc_port must be between 1 and 65535")
	}
	if c.Server.APIPort == c.Server.RPCPort {
		errs = append(errs, "server.api_port and server.rpc_port must be different")
	}

	// Performance validation
	if c.Performance.MaxMemoryMB < 64 {
		errs = append(errs, "performance.max_memory_mb must be >= 64")
	}
	if c.Performance.MaxConcurrentOps < 1 {
		errs = append(errs, "performance.max_concurrent_ops must be >= 1")
	}

	// Feature validation
	if c.Features.Experimental && c.Verbose {
		c.Verbose = true
	}

	// Search validation
	if c.Search.DefaultLimit < 1 {
		errs = append(errs, "search.default_limit must be >= 1")
	}
	if c.Search.DefaultLimit > c.Search.MaxResults {
		errs = append(errs, "search.default_limit must be <= search.max_results")
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}

	return nil
}

// =============================================================================
// Validation Error
// =============================================================================

// ValidationError represents configuration validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration validation failed (%d errors):\n  - %s",
		len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// =============================================================================
// Accessors
// =============================================================================

// LoadedFrom returns the path the config was loaded from, if any.
func (c *Config) LoadedFrom() string {
	return c.loadedFrom
}

// IsSet reports whether a config key is set to a non-zero value.
// Uses a dot-separated path (e.g., "db.path").
func (c *Config) IsSet(_ string) bool {
	// This is a simplified accessor; full implementation would
	// use reflection or viper.
	return true
}

// DBPath returns the resolved database path.
func (c *Config) DBPath() string {
	if c.DB.Path != "" {
		expanded, _ := homedir.Expand(c.DB.Path)
		return expanded
	}
	return filepath.Join(c.Paths.Home, "cosca.db")
}

// RuntimeDir returns the resolved runtime directory.
func (c *Config) RuntimeDir() string {
	if c.Paths.Runtime != "" {
		expanded, _ := homedir.Expand(c.Paths.Runtime)
		return expanded
	}
	return filepath.Join(c.Paths.Home, DefaultRuntimeDir)
}

// ConfigFilePath returns the default user config file path.
func (c *Config) ConfigFilePath() string {
	return filepath.Join(c.Paths.Home, "config.yaml")
}

// =============================================================================
// Viper Bridge
// =============================================================================

// viperInstance holds the global viper instance.
var (
	viperMu     sync.Mutex
	globalViper *viper.Viper
)

// SetViper sets the global viper instance used by CLI commands.
func SetViper(v *viper.Viper) {
	viperMu.Lock()
	defer viperMu.Unlock()
	globalViper = v
}

// GetViper returns the global viper instance, creating one if needed.
func GetViper() *viper.Viper {
	viperMu.Lock()
	defer viperMu.Unlock()
	if globalViper == nil {
		globalViper = viper.New()
	}
	return globalViper
}
