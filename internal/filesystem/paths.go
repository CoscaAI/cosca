//
// Package filesystem provides path management for the Cosca platform.
// It resolves all important directories with proper fallbacks
// following XDG Base Directory Specification where applicable.

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Path Constants
// =============================================================================

const (
	// DefaultCoscaHome is the default Cosca home directory under the user's config.
	DefaultCoscaHome = ".config/cosca"

	// RuntimeDirName is the name of the runtime data directory.
	RuntimeDirName = "runtime"

	// DataDirName is the name of the persistent data directory.
	DataDirName = "data"

	// CacheDirName is the name of the cache directory.
	CacheDirName = "cache"

	// LogsDirName is the name of the logs directory.
	LogsDirName = "logs"

	// TempDirName is the name of the temp directory.
	TempDirName = "tmp"

	// PluginsDirName is the name of the plugins directory.
	PluginsDirName = "plugins"

	// BackupsDirName is the name of the backups directory.
	BackupsDirName = "backups"

	// ProjectDirName is the name of the Cosca project directory.
	ProjectDirName = ".cosca"

	// PidFileName is the name of the PID file for the daemon.
	PidFileName = "cosca.pid"

	// TelemetryDBName is the name of the telemetry SQLite database.
	TelemetryDBName = "telemetry.db"

	// KnowledgeDBName is the name of the knowledge SQLite database.
	KnowledgeDBName = "knowledge.db"
)

// =============================================================================
// Global State
// =============================================================================

var (
	homeDirOnce sync.Once
	homeDir     string
	homeDirErr  error
)

// getHomeDir returns the user's home directory, cached after the first call.
func getHomeDir() (string, error) {
	homeDirOnce.Do(func() {
		homeDir, homeDirErr = homedir.Dir()
	})
	return homeDir, homeDirErr
}

// =============================================================================
// Directory Resolution
// =============================================================================

// CoscaHomeDir returns the Cosca home directory (~/.config/cosca or COSCA_HOME env).
func CoscaHomeDir() string {
	if env := os.Getenv("COSCA_HOME"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	hd, err := getHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("cannot determine home directory, using current dir")
		return filepath.Join(".", DefaultCoscaHome)
	}
	return filepath.Join(hd, DefaultCoscaHome)
}

// CoscaRuntimeDir returns the runtime directory (~/.config/cosca/runtime).
func CoscaRuntimeDir() string {
	if env := os.Getenv("COSCA_RUNTIME_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), RuntimeDirName)
}

// CoscaDataDir returns the persistent data directory (~/.config/cosca/data).
func CoscaDataDir() string {
	if env := os.Getenv("COSCA_DATA_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), DataDirName)
}

// CoscaCacheDir returns the cache directory (~/.config/cosca/cache).
func CoscaCacheDir() string {
	if env := os.Getenv("COSCA_CACHE_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	// Use XDG cache if available
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "cosca")
	}
	return filepath.Join(CoscaHomeDir(), CacheDirName)
}

// CoscaLogsDir returns the logs directory (~/.config/cosca/logs).
func CoscaLogsDir() string {
	if env := os.Getenv("COSCA_LOGS_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), LogsDirName)
}

// CoscaTempDir returns the temp directory (~/.config/cosca/tmp).
func CoscaTempDir() string {
	if env := os.Getenv("COSCA_TEMP_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), TempDirName)
}

// CoscaPluginsDir returns the plugins directory (~/.config/cosca/plugins).
func CoscaPluginsDir() string {
	if env := os.Getenv("COSCA_PLUGINS_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), PluginsDirName)
}

// CoscaBackupsDir returns the backups directory (~/.config/cosca/backups).
func CoscaBackupsDir() string {
	if env := os.Getenv("COSCA_BACKUPS_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}
	return filepath.Join(CoscaHomeDir(), BackupsDirName)
}

// CoscaProjectDir returns the Cosca project directory (.cosca/ in the project root).
// It walks up from the current working directory to find the project root.
func CoscaProjectDir() string {
	if env := os.Getenv("COSCA_PROJECT_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}

	// Try to detect project root from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", ProjectDirName)
	}

	// Walk up to find .cosca directory or common project markers
	dir := cwd
	for {
		coscaDir := filepath.Join(dir, ProjectDirName)
		if info, err := os.Stat(coscaDir); err == nil && info.IsDir() {
			return coscaDir
		}
		// Check for common project root markers
		for _, marker := range []string{"go.mod", "package.json", ".git", "Cargo.toml", "pyproject.toml"} {
			markerPath := filepath.Join(dir, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return filepath.Join(dir, ProjectDirName)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a project marker
			return filepath.Join(cwd, ProjectDirName)
		}
		dir = parent
	}
}

// CoscaGlobalDir returns the Cosca installation/global directory.
// This is the directory where the Cosca binary and global resources live.
func CoscaGlobalDir() string {
	if env := os.Getenv("COSCA_GLOBAL_DIR"); env != "" {
		expanded, err := homedir.Expand(env)
		if err == nil {
			return expanded
		}
	}

	// Try common installation paths
	candidates := []string{
		"/usr/local/share/cosca",
		"/usr/share/cosca",
		"/opt/cosca",
	}

	// Also check relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append([]string{
			filepath.Join(exeDir, "..", "share", "cosca"),
			filepath.Join(exeDir, "..", "resources"),
		}, candidates...)
	}

	for _, candidate := range candidates {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			if info, err := os.Stat(resolved); err == nil && info.IsDir() {
				return resolved
			}
		}
	}

	// Fallback to Cosca home
	return filepath.Join(CoscaHomeDir(), "global")
}

// =============================================================================
// Path Composition
// =============================================================================

// CoscaPIDFile returns the path to the PID file.
func CoscaPIDFile() string {
	return filepath.Join(CoscaRuntimeDir(), PidFileName)
}

// TelemetryDBPath returns the path to the telemetry SQLite database.
func TelemetryDBPath() string {
	return filepath.Join(CoscaDataDir(), TelemetryDBName)
}

// KnowledgeDBPath returns the path to the knowledge SQLite database.
func KnowledgeDBPath() string {
	return filepath.Join(CoscaDataDir(), KnowledgeDBName)
}

// ConfigFilePath returns the path to the user configuration file.
func ConfigFilePath() string {
	return filepath.Join(CoscaHomeDir(), "config.yaml")
}

// ProjectConfigFilePath returns the path to the project configuration file.
func ProjectConfigFilePath() string {
	return filepath.Join(CoscaProjectDir(), "config.yaml")
}

// =============================================================================
// Directory Initialization
// =============================================================================

// EnsureDirectories creates all Cosca directories if they don't exist.
// Returns a map of directory names to paths that were created or already exist.
func EnsureDirectories() (map[string]string, error) {
	dirs := map[string]string{
		"home":    CoscaHomeDir(),
		"runtime": CoscaRuntimeDir(),
		"data":    CoscaDataDir(),
		"cache":   CoscaCacheDir(),
		"logs":    CoscaLogsDir(),
		"temp":    CoscaTempDir(),
		"plugins": CoscaPluginsDir(),
		"backups": CoscaBackupsDir(),
	}

	created := make(map[string]string)
	for name, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return created, fmt.Errorf("create %s directory %s: %w", name, dir, err)
		}
		created[name] = dir
		log.Debug().Str("name", name).Str("path", dir).Msg("directory ensured")
	}

	return created, nil
}

// =============================================================================
// Platform Helpers
// =============================================================================

// DataHome returns the platform-appropriate data home directory.
// On Linux: ~/.local/share, on macOS: ~/Library/Application Support.
func DataHome() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDirOrDefault(), "Library", "Application Support")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "cosca")
		}
		return filepath.Join(homeDirOrDefault(), "AppData", "Roaming", "cosca")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "cosca")
		}
		return filepath.Join(homeDirOrDefault(), ".local", "share", "cosca")
	}
}

// ConfigHome returns the platform-appropriate config home directory.
func ConfigHome() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDirOrDefault(), "Library", "Application Support", "cosca")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "cosca")
		}
		return filepath.Join(homeDirOrDefault(), "AppData", "Roaming", "cosca")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "cosca")
		}
		return CoscaHomeDir()
	}
}

// CacheHome returns the platform-appropriate cache home directory.
func CacheHome() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDirOrDefault(), "Library", "Caches", "cosca")
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "cosca", "cache")
		}
		return filepath.Join(homeDirOrDefault(), "AppData", "Local", "cosca", "cache")
	default:
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, "cosca")
		}
		return filepath.Join(homeDirOrDefault(), ".cache", "cosca")
	}
}

// homeDirOrDefault returns the home directory or a default value.
func homeDirOrDefault() string {
	hd, err := getHomeDir()
	if err != nil {
		return "."
	}
	return hd
}
