package discovery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// DiscoveryReport is the complete result of a full workspace discovery.
//
//nolint:revive // Stutter name preserved for API compatibility — used as discovery.DiscoveryReport externally.
type DiscoveryReport struct {
	Project      *ProjectInfo     `json:"project,omitempty" yaml:"project,omitempty"`
	Workspace    *WorkspaceInfo   `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Editor       *EditorInfo      `json:"editor,omitempty" yaml:"editor,omitempty"`
	Cosca        *CoscaInfo       `json:"cosca,omitempty" yaml:"cosca,omitempty"`
	Providers    []ProviderInfo   `json:"providers,omitempty" yaml:"providers,omitempty"`
	Plugins      []PluginInfo     `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Runtime      *RuntimeInfo     `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Environment  *EnvironmentInfo `json:"environment,omitempty" yaml:"environment,omitempty"`
	DiscoveredAt time.Time        `json:"discovered_at" yaml:"discovered_at"`
	Duration     time.Duration    `json:"duration" yaml:"duration"`
}

// WorkspaceInfo describes the workspace structure.
type WorkspaceInfo struct {
	Root        string   `json:"root" yaml:"root"`
	IsGitRepo   bool     `json:"is_git_repo" yaml:"is_git_repo"`
	IsMonorepo  bool     `json:"is_monorepo" yaml:"is_monorepo"`
	GitRemote   string   `json:"git_remote,omitempty" yaml:"git_remote,omitempty"`
	GitBranch   string   `json:"git_branch,omitempty" yaml:"git_branch,omitempty"`
	GitCommit   string   `json:"git_commit,omitempty" yaml:"git_commit,omitempty"`
	SubProjects []string `json:"sub_projects,omitempty" yaml:"sub_projects,omitempty"`
}

// ProviderInfo describes an AI provider configuration.
type ProviderInfo struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type" yaml:"type"`
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Model    string `json:"model,omitempty" yaml:"model,omitempty"`
}

// PluginInfo describes an installed plugin.
type PluginInfo struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Path    string `json:"path" yaml:"path"`
}

// RuntimeInfo describes the runtime configuration.
type RuntimeInfo struct {
	Mode        string `json:"mode" yaml:"mode"` // "server", "cli", "daemon"
	LogLevel    string `json:"log_level" yaml:"log_level"`
	ConfigFile  string `json:"config_file,omitempty" yaml:"config_file,omitempty"`
	CacheDir    string `json:"cache_dir,omitempty" yaml:"cache_dir,omitempty"`
	DataDir     string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	MaxMemory   int64  `json:"max_memory,omitempty" yaml:"max_memory,omitempty"`
	Concurrency int    `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
}

// Engine performs workspace and environment discovery.
type Engine struct {
	mu       sync.RWMutex
	logger   zerolog.Logger
	workDir  string
	cache    *DiscoveryReport
	cacheTTL time.Duration
}

// Option configures the discovery engine.
type Option func(*Engine)

// WithLogger sets the logger for the engine.
func WithLogger(logger zerolog.Logger) Option {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithWorkDir sets the working directory for discovery.
func WithWorkDir(dir string) Option {
	return func(e *Engine) {
		e.workDir = dir
	}
}

// WithCacheTTL sets the cache TTL for discovery results.
func WithCacheTTL(ttl time.Duration) Option {
	return func(e *Engine) {
		e.cacheTTL = ttl
	}
}

// NewEngine creates a new discovery engine.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		logger:   zerolog.Nop(),
		workDir:  ".",
		cacheTTL: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// DiscoverAll performs full discovery and returns the report.
func (e *Engine) DiscoverAll(ctx context.Context) (*DiscoveryReport, error) {
	start := time.Now()
	e.logger.Info().Msg("starting full workspace discovery")

	report := &DiscoveryReport{
		DiscoveredAt: time.Now(),
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	// I/O-bound file scanning — keep goroutines
	wg.Add(3)
	go func() {
		defer wg.Done()
		p, err := e.DiscoverProject(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Msg("project discovery failed")
			errCh <- fmt.Errorf("project: %w", err)
			return
		}
		report.Project = p
	}()
	go func() {
		defer wg.Done()
		w, err := e.DiscoverWorkspace(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Msg("workspace discovery failed")
			errCh <- fmt.Errorf("workspace: %w", err)
			return
		}
		report.Workspace = w
	}()
	go func() {
		defer wg.Done()
		plugins, err := e.DiscoverPlugins(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Msg("plugin discovery failed")
			errCh <- fmt.Errorf("plugins: %w", err)
			return
		}
		report.Plugins = plugins
	}()

	// Sub-millisecond operations — sequential
	if ed, err := e.DiscoverEditor(ctx); err != nil {
		e.logger.Warn().Err(err).Msg("editor discovery failed")
	} else {
		report.Editor = ed
	}
	if a, err := e.DiscoverCoscaGlobal(ctx); err != nil {
		e.logger.Warn().Err(err).Msg("Cosca discovery failed")
	} else {
		report.Cosca = a
	}
	if providers, err := e.DiscoverProviders(ctx); err != nil {
		e.logger.Warn().Err(err).Msg("provider discovery failed")
	} else {
		report.Providers = providers
	}
	if rt, err := e.DiscoverRuntime(ctx); err != nil {
		e.logger.Warn().Err(err).Msg("runtime discovery failed")
	} else {
		report.Runtime = rt
	}
	if env, err := e.DiscoverEnvironment(ctx); err != nil {
		e.logger.Warn().Err(err).Msg("environment discovery failed")
	} else {
		report.Environment = env
	}

	wg.Wait()
	close(errCh)

	// Collect non-critical errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	report.Duration = time.Since(start)

	if len(errors) > 0 {
		e.logger.Warn().Int("errors", len(errors)).Msg("discovery completed with errors")
	}

	e.mu.Lock()
	e.cache = report
	e.mu.Unlock()

	e.logger.Info().
		Dur("duration", report.Duration).
		Bool("has_project", report.Project != nil).
		Bool("has_editor", report.Editor != nil).
		Bool("has_cosca", report.Cosca != nil).
		Msg("discovery completed")

	return report, nil
}

// DiscoverProject detects the project type, language, framework, database, etc.
func (e *Engine) DiscoverProject(ctx context.Context) (*ProjectInfo, error) {
	return DetectProject(ctx, e.workDir, e.logger)
}

// DiscoverWorkspace detects workspace structure.
func (e *Engine) DiscoverWorkspace(ctx context.Context) (*WorkspaceInfo, error) {
	return DetectWorkspace(ctx, e.workDir, e.logger)
}

// DiscoverEditor detects the running editor/environment.
func (e *Engine) DiscoverEditor(ctx context.Context) (*EditorInfo, error) {
	return DetectEditor(ctx, e.logger)
}

// DiscoverCoscaGlobal finds the Cosca global installation.
func (e *Engine) DiscoverCoscaGlobal(ctx context.Context) (*CoscaInfo, error) {
	return DetectCosca(ctx, e.logger)
}

// DiscoverProviders detects available AI providers.
func (e *Engine) DiscoverProviders(ctx context.Context) ([]ProviderInfo, error) {
	return DetectProviders(ctx, e.logger)
}

// DiscoverPlugins scans for installed plugins.
func (e *Engine) DiscoverPlugins(ctx context.Context) ([]PluginInfo, error) {
	return DetectPlugins(ctx, e.logger)
}

// DiscoverRuntime detects the runtime configuration.
func (e *Engine) DiscoverRuntime(ctx context.Context) (*RuntimeInfo, error) {
	return DetectRuntime(ctx, e.logger)
}

// DiscoverEnvironment detects OS, shell, terminal, and system info.
func (e *Engine) DiscoverEnvironment(ctx context.Context) (*EnvironmentInfo, error) {
	return DetectEnvironment(ctx, e.logger)
}

// GetCachedReport returns the cached discovery report if still valid.
func (e *Engine) GetCachedReport() (*DiscoveryReport, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.cache == nil {
		return nil, false
	}
	if time.Since(e.cache.DiscoveredAt) > e.cacheTTL {
		return nil, false
	}
	return e.cache, true
}

// InvalidateCache clears the cached discovery report.
func (e *Engine) InvalidateCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = nil
}

// DetectWorkspace detects workspace structure.
func DetectWorkspace(ctx context.Context, workDir string, logger zerolog.Logger) (*WorkspaceInfo, error) {
	info := &WorkspaceInfo{
		Root: workDir,
	}

	// Check if .git exists
	gitPath := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		info.IsGitRepo = true
		// Try to get remote
		if remote, err := exec.Command("git", "-C", workDir, "remote", "get-url", "origin").Output(); err == nil {
			info.GitRemote = strings.TrimSpace(string(remote))
		}
		// Try to get branch
		if branch, err := exec.Command("git", "-C", workDir, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			info.GitBranch = strings.TrimSpace(string(branch))
		}
		// Try to get commit
		if commit, err := exec.Command("git", "-C", workDir, "rev-parse", "HEAD").Output(); err == nil {
			info.GitCommit = strings.TrimSpace(string(commit))
		}
	}

	// Detect monorepo
	info.IsMonorepo = detectMonorepo(ctx, workDir)

	// Find sub-projects
	info.SubProjects = findSubProjects(ctx, workDir)

	logger.Debug().
		Bool("is_git", info.IsGitRepo).
		Bool("is_monorepo", info.IsMonorepo).
		Str("branch", info.GitBranch).
		Msg("workspace discovery complete")

	return info, nil
}

// DetectProviders detects available AI providers from config.
func DetectProviders(_ context.Context, logger zerolog.Logger) ([]ProviderInfo, error) {
	var providers []ProviderInfo

	// Check common provider environment variables
	providerEnvVars := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"google":     "GOOGLE_API_KEY",
		"azure":      "AZURE_OPENAI_API_KEY",
		"cohere":     "COHERE_API_KEY",
		"mistral":    "MISTRAL_API_KEY",
		"groq":       "GROQ_API_KEY",
		"together":   "TOGETHER_API_KEY",
		"deepseek":   "DEEPSEEK_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	for name, envVar := range providerEnvVars {
		val := os.Getenv(envVar)
		if val != "" {
			providers = append(providers, ProviderInfo{
				Name:    name,
				Type:    name,
				Enabled: true,
			})
		}
	}

	// Also check Cosca config for providers
	homeDir, err := os.UserHomeDir()
	if err == nil {
		coscaConfig := filepath.Join(homeDir, ".config", "opencode", "cosca", "cosca.config.yaml")
		if data, err := os.ReadFile(coscaConfig); err == nil {
			var cfg struct {
				Providers []ProviderInfo `yaml:"providers"`
			}
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				providers = append(providers, cfg.Providers...)
			}
		}
	}

	logger.Debug().Int("count", len(providers)).Msg("provider discovery complete")
	return providers, nil
}

// DetectPlugins scans for installed plugins.
func DetectPlugins(_ context.Context, logger zerolog.Logger) ([]PluginInfo, error) {
	var plugins []PluginInfo

	// Look for plugins directory in Cosca installation
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return plugins, err
	}

	pluginDirs := []string{
		filepath.Join(homeDir, ".config", "opencode", "cosca", "plugins"),
		filepath.Join(homeDir, ".cosca", "plugins"),
	}

	for _, dir := range pluginDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				plugin := PluginInfo{
					Name:    entry.Name(),
					Enabled: true,
					Path:    filepath.Join(dir, entry.Name()),
				}
				// Try to read version from plugin metadata
				metaFile := filepath.Join(dir, entry.Name(), "plugin.yaml")
				if data, err := os.ReadFile(metaFile); err == nil {
					var meta struct {
						Version string `yaml:"version"`
					}
					if err := yaml.Unmarshal(data, &meta); err == nil {
						plugin.Version = meta.Version
					}
				}
				plugins = append(plugins, plugin)
			}
		}
	}

	logger.Debug().Int("count", len(plugins)).Msg("plugin discovery complete")
	return plugins, nil
}

// DetectRuntime detects runtime configuration.
func DetectRuntime(_ context.Context, logger zerolog.Logger) (*RuntimeInfo, error) {
	info := &RuntimeInfo{
		Mode:     "cli",
		LogLevel: "info",
	}

	if mode := os.Getenv("COSCA_MODE"); mode != "" {
		info.Mode = mode
	}
	if level := os.Getenv("COSCA_LOG_LEVEL"); level != "" {
		info.LogLevel = level
	}
	if cfg := os.Getenv("COSCA_CONFIG_FILE"); cfg != "" {
		info.ConfigFile = cfg
	}
	if cache := os.Getenv("COSCA_CACHE_DIR"); cache != "" {
		info.CacheDir = cache
	}
	if data := os.Getenv("COSCA_DATA_DIR"); data != "" {
		info.DataDir = data
	}

	// Set defaults based on XDG or home
	homeDir, err := os.UserHomeDir()
	if err == nil {
		if info.CacheDir == "" {
			info.CacheDir = filepath.Join(homeDir, ".cache", "cosca")
		}
		if info.DataDir == "" {
			info.DataDir = filepath.Join(homeDir, ".local", "share", "cosca")
		}
	}

	logger.Debug().Str("mode", info.Mode).Str("log_level", info.LogLevel).Msg("runtime discovery complete")
	return info, nil
}

// detectMonorepo checks if the workspace is a monorepo.
func detectMonorepo(_ context.Context, workDir string) bool {
	moduleFiles := 0
	patterns := []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "build.gradle", "CMakeLists.txt"}

	for _, pattern := range patterns {
		path := filepath.Join(workDir, pattern)
		if _, err := os.Stat(path); err == nil {
			moduleFiles++
		}
	}

	// Count subdirectories with their own module files
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		for _, pattern := range patterns {
			subPath := filepath.Join(workDir, entry.Name(), pattern)
			if _, err := os.Stat(subPath); err == nil {
				moduleFiles++
				break
			}
		}
	}

	return moduleFiles > 1
}

// findSubProjects finds sub-projects in the workspace.
func findSubProjects(_ context.Context, workDir string) []string {
	var projects []string
	patterns := []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml"}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		for _, pattern := range patterns {
			subPath := filepath.Join(workDir, entry.Name(), pattern)
			if _, err := os.Stat(subPath); err == nil {
				projects = append(projects, entry.Name())
				break
			}
		}
	}

	return projects
}
