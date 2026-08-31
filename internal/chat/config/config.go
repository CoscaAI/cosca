// Package config provides typed configuration loading for the Cosca Chat CLI.
// It reads .cosca/config.yaml and exposes the configuration through a Go struct
// with support for environment variable expansion, sensible defaults, and
// provider resolution.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	mainconfig "github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/permission"
)

// Config represents the .cosca/config.yaml structure.
type Config struct {
	Version  string       `yaml:"version"`
	Provider ProviderConf `yaml:"provider"`
	Sandbox  SandboxConf  `yaml:"sandbox"`
	Session  SessionConf  `yaml:"session"`
	MCP      MCPConf      `yaml:"mcp"`

	// ExecPolicy is the path to a YAML file defining shell command execution
	// policies (see internal/execpolicy). When set, commands executed through
	// the shell tool are evaluated against these rules before running. Empty
	// = no policy enforcement (current behavior).
	ExecPolicy string `yaml:"exec_policy" json:"execPolicy"`

	// Permissions is the allow/ask/deny ruleset that gates tool execution
	// (see internal/permission). Empty = no enforcement (fail-open, current
	// behavior). Supports string actions and per-pattern maps. Read from the
	// same `.cosca/config.yaml` as the main cosca config.
	Permissions map[string]interface{} `yaml:"permissions,omitempty" json:"permissions,omitempty"`
}

// MCPConf holds MCP server connection configurations for external tools.
type MCPConf struct {
	Servers []MCPServerConf `yaml:"servers"`
}

// MCPServerConf defines how to connect to an external MCP server.
// Either Command (stdio transport) or URL (HTTP transport) must be set.
type MCPServerConf struct {
	// Name is a human-readable identifier for this server (used in logs).
	Name string `yaml:"name"`

	// Command is the executable path for stdio transport.
	Command string `yaml:"command"`

	// Args are the command-line arguments for stdio transport.
	Args []string `yaml:"args"`

	// URL is the HTTP endpoint for HTTP transport (e.g. "http://localhost:8370/mcp").
	URL string `yaml:"url"`

	// Env provides additional environment variables for the subprocess.
	Env map[string]string `yaml:"env"`
}

// ProviderConf holds all provider configurations and the primary selection.
type ProviderConf struct {
	Primary   string       `yaml:"primary"`
	DeepSeek  ProviderItem `yaml:"deepseek"`
	OpenAI    ProviderItem `yaml:"openai"`
	Anthropic ProviderItem `yaml:"anthropic"`
	Ollama    OllamaItem   `yaml:"ollama"`
}

// ProviderItem represents a provider that authenticates with an API key.
type ProviderItem struct {
	APIKey string `yaml:"api_key"`
	// APIKeyEnv is the preferred, non-secret way to persist provider auth.
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	// ContextWindow is the model's context window in tokens (e.g. 200000).
	ContextWindow int `yaml:"context_window"`
}

// OllamaItem represents the Ollama provider configuration (no API key).
type OllamaItem struct {
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
}

// SandboxConf controls the sandbox isolation mode for command execution.
type SandboxConf struct {
	Mode string `yaml:"mode"` // read-only, workspace, full
}

// SessionConf controls session-level behavior.
type SessionConf struct {
	AutoSave bool `yaml:"auto_save"`
	MaxTurns int  `yaml:"max_turns"`
}

// DefaultConfig returns a Config with sensible defaults matching
// the scaffold/config.yaml template.
func DefaultConfig() Config {
	return Config{
		Version: "1",
		Provider: ProviderConf{
			Primary: "deepseek",
			DeepSeek: ProviderItem{
				Model:         "deepseek-v4-flash",
				ContextWindow: 200000,
			},
		},
		Sandbox: SandboxConf{
			Mode: "workspace",
		},
		Session: SessionConf{
			AutoSave: true,
			MaxTurns: 100,
		},
		MCP: MCPConf{
			Servers: nil, // no external MCP servers by default
		},
	}
}

// Load reads and parses .cosca/config.yaml from the given root directory.
// Returns DefaultConfig() if the file does not exist (not an error).
// Supports ${VAR} and $VAR environment variable expansion in config values.
// Returns an error only on parse failure.
//
// After loading the project config, the shared global config
// (~/.config/cosca/config.yaml — the same file used by the `cosca` binary)
// is overlaid so both CLIs share the same provider, API key, model and
// context window. Project values win over global values for model/primary;
// the API key from the global config takes precedence (it is the single
// source of truth for credentials).
func Load(rootDir string) (*Config, error) {
	path := filepath.Join(rootDir, ".cosca", "config.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := cfg.mergeGlobal(); err != nil {
				log.Warn().Err(err).Msg("failed to merge global config")
			}
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables in the raw YAML content so that values
	// like ${OPENAI_API_KEY} or $OPENAI_API_KEY are resolved before parsing.
	expanded := os.Expand(string(data), os.Getenv)

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	resolveProviderEnv(&cfg)

	// Overlay the shared global config (same file as the `cosca` binary).
	if err := cfg.mergeGlobal(); err != nil {
		log.Warn().Err(err).Msg("failed to merge global config")
	}

	return &cfg, nil
}

// GlobalProvider is the shared provider section read from the global cosca
// config (~/.config/cosca/config.yaml), decrypted and resolved.
type GlobalProvider struct {
	Name          string
	Model         string
	APIKey        string
	BaseURL       string
	ContextWindow int64
}

// globalConfigPath returns the path to the global cosca config shared by the
// binário único `cosca` (chat/exec/mcp) (~/.config/cosca/config.yaml).
// COSCA_GLOBAL_CONFIG overrides the location (used by tests and power users).
func globalConfigPath() string {
	if p := os.Getenv("COSCA_GLOBAL_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "cosca", "config.yaml")
	}
	return filepath.Join(dir, "cosca", "config.yaml")
}

// LoadGlobal reads the shared global config using the main cosca schema
// (internal/config) so the API key is decrypted automatically. Returns nil
// if the global config does not exist.
func LoadGlobal() (*GlobalProvider, error) {
	path := globalConfigPath()
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	mainCfg, err := mainconfig.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load global config %s: %w", path, err)
	}
	return &GlobalProvider{
		Name:          mainCfg.Provider.Name,
		Model:         mainCfg.Provider.Model,
		APIKey:        mainCfg.Provider.APIKey,
		BaseURL:       mainCfg.Provider.BaseURL,
		ContextWindow: mainCfg.Provider.ContextWindow,
	}, nil
}

// mergeGlobal overlays the shared global provider config into this chat config.
// The API key from the global config always wins (single source of truth for
// credentials). Project-level model/primary override global values when set.
func (c *Config) mergeGlobal() error {
	g, err := LoadGlobal()
	if err != nil || g == nil {
		return err
	}

	if c.Provider.Primary == "" && g.Name != "" {
		c.Provider.Primary = g.Name
	}
	if g.Name == "" {
		return nil
	}

	switch g.Name {
	case "deepseek":
		if c.Provider.DeepSeek.APIKey == "" {
			c.Provider.DeepSeek.APIKey = g.APIKey
		}
		if c.Provider.DeepSeek.Model == "" {
			c.Provider.DeepSeek.Model = g.Model
		}
		if c.Provider.DeepSeek.ContextWindow == 0 && g.ContextWindow > 0 {
			c.Provider.DeepSeek.ContextWindow = int(g.ContextWindow)
		}
		if c.Provider.DeepSeek.BaseURL == "" {
			c.Provider.DeepSeek.BaseURL = g.BaseURL
		}
	case "openai":
		if c.Provider.OpenAI.APIKey == "" {
			c.Provider.OpenAI.APIKey = g.APIKey
		}
		if c.Provider.OpenAI.Model == "" {
			c.Provider.OpenAI.Model = g.Model
		}
		if c.Provider.OpenAI.BaseURL == "" {
			c.Provider.OpenAI.BaseURL = g.BaseURL
		}
	case "anthropic":
		if c.Provider.Anthropic.APIKey == "" {
			c.Provider.Anthropic.APIKey = g.APIKey
		}
		if c.Provider.Anthropic.Model == "" {
			c.Provider.Anthropic.Model = g.Model
		}
		if c.Provider.Anthropic.BaseURL == "" {
			c.Provider.Anthropic.BaseURL = g.BaseURL
		}
	}
	return nil
}

// Save writes the config to .cosca/config.yaml relative to rootDir.
// It creates the .cosca/ directory if it does not exist.
// If cfg is nil, the operation is a no-op (existing file is preserved).
func Save(rootDir string, cfg *Config) error {
	if cfg == nil {
		return nil
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("resolve config root: %w", err)
	}
	// filepath.Dir(rootAbs) == rootAbs detecta a raiz do filesystem de forma
	// portátil: "/" no Unix e "C:\" (drive root) no Windows. Comparar com
	// string(filepath.Separator) falharia no Windows, onde filepath.Abs("\\")
	// resolve para "C:\" e Clean("C:\") != "\".
	if filepath.Dir(rootAbs) == rootAbs {
		return fmt.Errorf("refusing filesystem root as config root")
	}
	rootInfo, err := os.Stat(rootAbs)
	if err != nil || !rootInfo.IsDir() {
		return fmt.Errorf("invalid config root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil || resolvedRoot != rootAbs || filepath.Dir(resolvedRoot) == resolvedRoot {
		return fmt.Errorf("refusing unsafe config root")
	}
	dir := filepath.Join(rootAbs, ".cosca")
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink config directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := rejectSymlinkComponents(dir); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure config directory: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")

	// Project config must never receive a plaintext API key. Preserve provider
	// selection and model settings, and write only an environment reference.
	wire := *cfg
	wire.Provider = cfg.Provider
	secureProviderItem(&wire.Provider.DeepSeek, "DEEPSEEK_API_KEY")
	secureProviderItem(&wire.Provider.OpenAI, "OPENAI_API_KEY")
	secureProviderItem(&wire.Provider.Anthropic, "ANTHROPIC_API_KEY")
	data, err := yaml.Marshal(&wire)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write symlink config file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect config file: %w", err)
	}
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
	// Revalidate immediately before replacement. This reduces the TOCTOU
	// window, but os.Rename has no portable no-follow/FD-relative variant in
	// the standard library; a hostile concurrent replacement of the parent can
	// still race this check. The config root must therefore not be attacker-
	// writable.
	if filepath.Dir(rootAbs) == rootAbs {
		return fmt.Errorf("refusing filesystem root as config root")
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

func rejectSymlinkComponents(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
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

func secureProviderItem(item *ProviderItem, defaultEnv string) {
	if item.APIKey == "" {
		return
	}
	if item.APIKeyEnv == "" {
		item.APIKeyEnv = defaultEnv
	}
	item.APIKey = ""
}

func resolveProviderEnv(cfg *Config) {
	resolve := func(item *ProviderItem, defaultEnv string) {
		envName := item.APIKeyEnv
		if envName == "" {
			envName = defaultEnv
		}
		if value := os.Getenv(envName); value != "" {
			item.APIKey = value
		}
	}
	resolve(&cfg.Provider.DeepSeek, "DEEPSEEK_API_KEY")
	resolve(&cfg.Provider.OpenAI, "OPENAI_API_KEY")
	resolve(&cfg.Provider.Anthropic, "ANTHROPIC_API_KEY")
}

// SetAndSave sets a config value identified by a dot-separated key path and
// persists the config to disk. Supported keys:
//
//	providers.deepseek.api_key
//	providers.deepseek.model
//	providers.openai.api_key
//	providers.openai.model
//	providers.openai.base_url
//	providers.anthropic.api_key
//	providers.anthropic.model
//	providers.anthropic.base_url
//	providers.ollama.model
//	providers.ollama.base_url
//	providers.primary
//	sandbox.mode
//	session.max_turns
//	session.auto_save
//
// // Returns an error for unknown keys or if the config cannot be saved.
func SetAndSave(rootDir string, key, value string) error {
	cfg, err := Load(rootDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.setByKey(key, value); err != nil {
		return err
	}

	return Save(rootDir, cfg)
}

// setByKey sets a config field by a dot-separated key path.
func (c *Config) setByKey(key, value string) error {
	switch key {
	case "providers.primary":
		c.Provider.Primary = value

	case "providers.deepseek.api_key":
		c.Provider.DeepSeek.APIKey = value
	case "providers.deepseek.model":
		c.Provider.DeepSeek.Model = value
	case "providers.deepseek.context_window":
		n, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("providers.deepseek.context_window requires an integer: %w", err)
		}
		c.Provider.DeepSeek.ContextWindow = n

	case "providers.openai.api_key":
		c.Provider.OpenAI.APIKey = value
	case "providers.openai.model":
		c.Provider.OpenAI.Model = value
	case "providers.openai.base_url":
		c.Provider.OpenAI.BaseURL = value

	case "providers.anthropic.api_key":
		c.Provider.Anthropic.APIKey = value
	case "providers.anthropic.model":
		c.Provider.Anthropic.Model = value
	case "providers.anthropic.base_url":
		c.Provider.Anthropic.BaseURL = value

	case "providers.ollama.model":
		c.Provider.Ollama.Model = value
	case "providers.ollama.base_url":
		c.Provider.Ollama.BaseURL = value

	case "sandbox.mode":
		c.Sandbox.Mode = value

	case "session.max_turns":
		n, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("session.max_turns requires an integer: %w", err)
		}
		c.Session.MaxTurns = n

	case "session.auto_save":
		switch value {
		case "true", "1", "yes":
			c.Session.AutoSave = true
		case "false", "0", "no":
			c.Session.AutoSave = false
		default:
			return fmt.Errorf("session.auto_save requires a boolean (true/false)")
		}

	case "exec_policy":
		c.ExecPolicy = value

	default:
		return fmt.Errorf("unknown config key: %q", key)
	}
	return nil
}

// GetValue loads the config and retrieves a single value by dot-separated key.
// Returns an error for unknown keys or if the config cannot be loaded.
func GetValue(rootDir, key string) (string, error) {
	cfg, err := Load(rootDir)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.getByKey(key)
}

// getByKey retrieves a config field by a dot-separated key path.
func (c *Config) getByKey(key string) (string, error) {
	switch key {
	case "providers.primary":
		return c.Provider.Primary, nil

	case "providers.deepseek.api_key":
		return credentialStatus(c.Provider.DeepSeek), nil
	case "providers.deepseek.model":
		return c.Provider.DeepSeek.Model, nil
	case "providers.deepseek.context_window":
		return fmt.Sprintf("%d", c.Provider.DeepSeek.ContextWindow), nil

	case "providers.openai.api_key":
		return credentialStatus(c.Provider.OpenAI), nil
	case "providers.openai.model":
		return c.Provider.OpenAI.Model, nil
	case "providers.openai.base_url":
		return c.Provider.OpenAI.BaseURL, nil

	case "providers.anthropic.api_key":
		return credentialStatus(c.Provider.Anthropic), nil
	case "providers.anthropic.model":
		return c.Provider.Anthropic.Model, nil
	case "providers.anthropic.base_url":
		return c.Provider.Anthropic.BaseURL, nil

	case "providers.ollama.model":
		return c.Provider.Ollama.Model, nil
	case "providers.ollama.base_url":
		return c.Provider.Ollama.BaseURL, nil

	case "sandbox.mode":
		return c.Sandbox.Mode, nil

	case "session.max_turns":
		return fmt.Sprintf("%d", c.Session.MaxTurns), nil
	case "session.auto_save":
		return fmt.Sprintf("%t", c.Session.AutoSave), nil

	case "exec_policy":
		return c.ExecPolicy, nil

	default:
		return "", fmt.Errorf("unknown config key: %q", key)
	}
}

func credentialStatus(item ProviderItem) string {
	if item.APIKey != "" || item.APIKeyEnv != "" {
		return "(configured; value redacted)"
	}
	return "(not set)"
}

// parseInt is a small helper to convert a string to an int.
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// ResolveProviderConfig returns the configuration for the named provider.
//
//   - If name is "primary" or empty, it resolves to the configured primary
//     provider (cfg.Provider.Primary).
//   - Otherwise it looks up the provider by the given name directly.
//
// Returns nil if the named provider is not found or is of an incompatible type
// (e.g. Ollama, which has a different configuration shape without api_key).
func (c *Config) ResolveProviderConfig(name string) *ProviderItem {
	if name == "primary" || name == "" {
		name = c.Provider.Primary
	}

	switch name {
	case "deepseek":
		return &c.Provider.DeepSeek
	case "openai":
		return &c.Provider.OpenAI
	case "anthropic":
		return &c.Provider.Anthropic
	default:
		return nil
	}
}

// PermissionRuleset converts the configured permissions block (from `.cosca/
// config.yaml`) into a permission.Ruleset. It returns nil when no permissions
// are configured, so callers treat "not configured" as fail-open (current
// behavior) and enforce only when an explicit allow/ask/deny policy exists.
func (c *Config) PermissionRuleset() permission.Ruleset {
	if len(c.Permissions) == 0 {
		return nil
	}
	return permission.FromConfig(c.Permissions)
}

// StoreAPIKey persiste a API key do provider no config global do Cosca
// (~/.config/cosca/config.yaml), criptografada em repouso (AES-256-GCM via
// internal/config.Save). É usado pelo seletor de modelo da TUI quando o Don
// conecta um provider pela primeira vez.
func StoreAPIKey(name, key string) error {
	if key == "" {
		return fmt.Errorf("empty API key")
	}

	path := globalConfigPath()

	// Carrega o config global existente (ou cria um novo padrão).
	var mainCfg *mainconfig.Config
	if _, err := os.Stat(path); err == nil {
		loaded, err := mainconfig.LoadFromFile(path)
		if err != nil {
			return fmt.Errorf("load global config: %w", err)
		}
		mainCfg = loaded
	} else {
		mainCfg = mainconfig.DefaultConfig()
	}

	// Aplica a chave ao provider correto. O config global guarda um provider
	// ativo por vez (bloco provider:), então a chave vai para esse bloco.
	mainCfg.Provider.Name = name
	mainCfg.Provider.APIKey = key
	switch name {
	case "deepseek":
		if mainCfg.Provider.Model == "" {
			mainCfg.Provider.Model = "deepseek-v4-flash"
		}
	case "openai":
		if mainCfg.Provider.Model == "" {
			mainCfg.Provider.Model = "gpt-4o"
		}
	case "anthropic":
		if mainCfg.Provider.Model == "" {
			mainCfg.Provider.Model = "claude-sonnet-4-20250514"
		}
	}

	// Salva — o Save do internal/config criptografa a chave automaticamente.
	return mainCfg.Save(path)
}
