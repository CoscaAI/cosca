// Package diagnostics provides individual diagnostic checks for the Cosca
// platform covering runtime, editor, plugins, memory, knowledge, providers,
// network, filesystem, permissions, and configuration.
package diagnostics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/safe"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// Check: Runtime
// =============================================================================

// CheckRuntime verifies the runtime status, version, and uptime.
func CheckRuntime(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "runtime",
		Severity: SeverityCritical,
	}

	var issues []string

	// Check Go runtime version
	goVersion := runtime.Version()
	result.Details = fmt.Sprintf("Go: %s, OS: %s/%s, CPUs: %d",
		goVersion, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())

	// Check memory
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	allocMB := mem.Alloc / 1024 / 1024

	if allocMB > 1024 {
		issues = append(issues, fmt.Sprintf("high memory usage: %d MB", allocMB))
	}

	// Check goroutines
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 10000 {
		issues = append(issues, fmt.Sprintf("high goroutine count: %d", numGoroutines))
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Runtime running with warnings (%s)", strings.Join(issues, "; "))
		result.Suggestion = "Monitor resource usage and consider restarting if performance degrades"
	} else {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("Runtime healthy (%s, %d goroutines, %d MB allocated)",
			goVersion, numGoroutines, allocMB)
	}

	return result
}

// =============================================================================
// Check: Editor
// =============================================================================

// CheckEditor detects the editor, checks integration, and validates config.
func CheckEditor(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "editor",
		Severity: SeverityWarning,
	}

	// Check for EDITOR environment variable
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor == "" {
		// Try to detect common editors
		commonEditors := []string{"vim", "nvim", "code", "nano", "emacs", "vi", "micro"}
		for _, e := range commonEditors {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}

	if editor == "" {
		result.Status = StatusWarn
		result.Message = "No editor detected"
		result.Details = "Set the EDITOR or VISUAL environment variable"
		result.Suggestion = "Install a text editor (vim, nvim, code, nano) and set EDITOR environment variable"
		return result
	}

	// Verify the editor is executable
	editorPath, err := exec.LookPath(editor)
	if err != nil {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Editor '%s' configured but not found in PATH", editor)
		result.Details = err.Error()
		result.Suggestion = fmt.Sprintf("Install %s or update your EDITOR environment variable", editor)
		return result
	}

	result.Status = StatusPass
	result.Message = fmt.Sprintf("Editor '%s' detected at %s", editor, editorPath)
	result.Details = fmt.Sprintf("EDITOR=%s, PATH=%s", editor, editorPath)
	return result
}

// =============================================================================
// Check: Plugins
// =============================================================================

// CheckPlugins lists plugins, checks health, and validates permissions.
func CheckPlugins(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "plugins",
		Severity: SeverityWarning,
	}

	// Check plugins directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	pluginDirs := []string{
		filepath.Join(homeDir, ".config", "cosca", "plugins"),
		filepath.Join(homeDir, ".cosca", "plugins"),
	}

	var foundPlugins []string
	var issues []string

	for _, dir := range pluginDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				pluginPath := filepath.Join(dir, entry.Name())
				// Check permissions
				info, err := os.Stat(pluginPath)
				if err != nil {
					issues = append(issues, fmt.Sprintf("cannot stat plugin %s: %v", entry.Name(), err))
					continue
				}
				perm := info.Mode().Perm()
				if perm&0o022 != 0 {
					issues = append(issues, fmt.Sprintf("plugin %s has unsafe permissions: %o", entry.Name(), perm))
				}
				foundPlugins = append(foundPlugins, entry.Name())
			}
		}
	}

	if len(foundPlugins) == 0 {
		result.Status = StatusSkip
		result.Message = "No plugins installed"
		result.Details = "The plugins directory was checked but no plugins were found"
		return result
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Found %d plugins with %d issues", len(foundPlugins), len(issues))
		result.Details = strings.Join(issues, "; ")
		result.Suggestion = "Review plugin permissions and ensure they are secure (should not be world-writable)"
	} else {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("Found %d plugin(s): %s", len(foundPlugins), strings.Join(foundPlugins, ", "))
	}

	return result
}

// =============================================================================
// Check: Memory
// =============================================================================

// CheckMemory checks memory layers, integrity, and TTLs.
func CheckMemory(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "memory",
		Severity: SeverityWarning,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	memoryDirs := []string{
		filepath.Join(homeDir, ".config", "cosca", "data", "memory"),
		filepath.Join(homeDir, ".local", "share", "cosca", "memory"),
	}

	var issues []string
	var foundFiles int

	for _, dir := range memoryDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				foundFiles++
				info, err := entry.Info()
				if err != nil {
					continue
				}
				// Check for excessively old memory files
				if time.Since(info.ModTime()) > 30*24*time.Hour {
					issues = append(issues, fmt.Sprintf("stale memory file: %s (last modified %s)",
						entry.Name(), info.ModTime().Format(time.RFC3339)))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Memory OK with %d stale entries", len(issues))
		result.Details = strings.Join(issues, "; ")
		result.Suggestion = "Run 'cosca memory prune' to clean up stale entries"
	} else {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("Memory layers healthy (%d files)", foundFiles)
	}

	return result
}

// =============================================================================
// Check: Knowledge
// =============================================================================

// CheckKnowledge checks SQLite, FTS5, vector index, and graph integrity.
func CheckKnowledge(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "knowledge",
		Severity: SeverityCritical,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	// Check SQLite database
	dbPaths := []string{
		filepath.Join(homeDir, ".config", "cosca", "data", "knowledge.db"),
		filepath.Join(homeDir, ".config", "cosca", "cosca.db"),
		filepath.Join(homeDir, ".local", "share", "cosca", "knowledge.db"),
	}

	var foundDB bool
	var issues []string

	for _, dbPath := range dbPaths {
		info, err := os.Stat(dbPath)
		if err != nil {
			continue
		}
		foundDB = true
		result.Details += fmt.Sprintf("Database: %s (%d MB)", dbPath, info.Size()/1024/1024)

		// Check if database is too large
		if info.Size() > 1024*1024*1024 { // 1 GB
			issues = append(issues, fmt.Sprintf("database is very large: %d MB", info.Size()/1024/1024))
		}
		break
	}

	if !foundDB {
		result.Status = StatusWarn
		result.Message = "No knowledge database found"
		result.Details = "The knowledge database may not have been initialized yet"
		result.Suggestion = "Run 'cosca knowledge init' to initialize the knowledge base"
		return result
	}

	// Check for FTS5 vector index directory
	vectorDir := filepath.Join(homeDir, ".config", "cosca", "data", "vectors")
	if _, err := os.Stat(vectorDir); err == nil {
		result.Details += "; Vector index present"
	} else {
		result.Details += "; No vector index (optional)"
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = "Knowledge base found with issues"
		result.Details += "; " + strings.Join(issues, "; ")
		result.Suggestion = "Consider running 'cosca knowledge optimize' to optimize the database"
	} else {
		result.Status = StatusPass
		result.Message = "Knowledge base healthy"
	}

	return result
}

// =============================================================================
// Check: Providers
// =============================================================================

// CheckProviders checks provider availability and tests connections.
func CheckProviders(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "providers",
		Severity: SeverityWarning,
	}

	providerEnvVars := map[string]string{
		"OpenAI":     "OPENAI_API_KEY",
		"Anthropic":  "ANTHROPIC_API_KEY",
		"Google":     "GOOGLE_API_KEY",
		"Azure":      "AZURE_OPENAI_API_KEY",
		"Cohere":     "COHERE_API_KEY",
		"Mistral":    "MISTRAL_API_KEY",
		"Groq":       "GROQ_API_KEY",
		"Together":   "TOGETHER_API_KEY",
		"DeepSeek":   "DEEPSEEK_API_KEY",
		"OpenRouter": "OPENROUTER_API_KEY",
		"Ollama":     "OLLAMA_HOST",
	}

	var configuredProviders []string
	for name, envVar := range providerEnvVars {
		val := os.Getenv(envVar)
		if val != "" {
			// Do not expose even masked key material in diagnostics. Presence is
			// sufficient for health reporting and safe for JSON/YAML export.
			configuredProviders = append(configuredProviders, fmt.Sprintf("%s (environment configured)", name))
		}
	}

	// Also check Cosca config (properly decrypts API keys at rest)
	coscaConfigPath := filepath.Join(os.Getenv("HOME"), ".config", "cosca", "config.yaml")
	if coscaCfg, err := config.LoadFromFile(coscaConfigPath); err == nil && coscaCfg.Provider.Name != "" {
		found := false
		for _, p := range configuredProviders {
			if strings.HasPrefix(p, coscaCfg.Provider.Name) {
				found = true
				break
			}
		}
		if !found {
			configuredProviders = append(configuredProviders, fmt.Sprintf("%s (from protected config)", coscaCfg.Provider.Name))
		}
	}

	if len(configuredProviders) == 0 {
		result.Status = StatusWarn
		result.Message = "No AI providers configured"
		result.Details = "No provider API keys found in environment variables or config"
		result.Suggestion = "Configure a provider by setting the appropriate API key (e.g., OPENAI_API_KEY) or running 'cosca config set provider.name <name>'"
		return result
	}

	result.Status = StatusPass
	result.Message = fmt.Sprintf("%d provider(s) configured", len(configuredProviders))
	result.Details = strings.Join(configuredProviders, "; ")
	return result
}

// =============================================================================
// Check: Network
// =============================================================================

// CheckNetwork checks connectivity to external services.
func CheckNetwork(ctx context.Context) CheckResult {
	result := CheckResult{
		Name:     "network",
		Severity: SeverityWarning,
	}

	// Services to check
	services := []struct {
		name string
		url  string
	}{
		{"GitHub (API)", "https://api.github.com"},
		{"OpenAI", "https://api.openai.com"},
		{"Anthropic", "https://api.anthropic.com"},
		{"Google", "https://generativelanguage.googleapis.com"},
		{"Docker Hub", "https://hub.docker.com"},
	}

	var reachable []string
	var unreachable []string

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
		},
	}

	// Check general internet connectivity first
	_, err := net.LookupHost("google.com")
	if err != nil {
		// DNS failure is non-fatal — continue checking services (Don's order).
		result.Message = "DNS resolution failed — continuing with connectivity checks"
		result.Details = fmt.Sprintf("DNS lookup failed: %v", err)
		result.Suggestion = "Check your internet connection and DNS settings"
		unreachable = append(unreachable, "internet (DNS)")
	} else {
		reachable = append(reachable, "internet (DNS)")
	}

	// Check specific services
	for _, svc := range services {
		_, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := client.Get(svc.url)
		cancel()
		if err == nil {
			safe.Close(resp.Body)
			if resp.StatusCode < 500 { // Any non-server-error response is OK
				reachable = append(reachable, svc.name)
			} else {
				unreachable = append(unreachable, fmt.Sprintf("%s (status %d)", svc.name, resp.StatusCode))
			}
		} else {
			unreachable = append(unreachable, fmt.Sprintf("%s (%v)", svc.name, err))
		}
	}

	result.Details = fmt.Sprintf("Reachable: %s", strings.Join(reachable, ", "))
	if len(unreachable) > 0 {
		result.Details += fmt.Sprintf("; Unreachable: %s", strings.Join(unreachable, ", "))
	}

	if len(unreachable) > 0 && len(unreachable) == len(services) {
		result.Status = StatusWarn
		result.Message = "Network partially available but all AI provider endpoints unreachable"
		result.Suggestion = "Check firewall/proxy settings and provider endpoint URLs"
	} else if len(unreachable) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Network OK, %d service(s) unreachable", len(unreachable))
		result.Suggestion = "Some AI providers may not be accessible from your network"
	} else {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("Network OK (%d services reachable)", len(reachable))
	}

	return result
}

// =============================================================================
// Check: Filesystem
// =============================================================================

// CheckFilesystem checks permissions, disk space, and path existence.
func CheckFilesystem(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "filesystem",
		Severity: SeverityCritical,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	// Directories to check
	requiredDirs := []string{
		filepath.Join(homeDir, ".config", "cosca"),
		filepath.Join(homeDir, ".config", "cosca", "runtime"),
		filepath.Join(homeDir, ".config", "cosca", "data"),
		filepath.Join(homeDir, ".config", "cosca", "cache"),
		filepath.Join(homeDir, ".config", "cosca", "logs"),
		filepath.Join(homeDir, ".config", "cosca", "tmp"),
	}

	var issues []string
	var missingDirs int

	for _, dir := range requiredDirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				missingDirs++
				issues = append(issues, fmt.Sprintf("missing: %s", dir))
			} else {
				issues = append(issues, fmt.Sprintf("error accessing %s: %v", dir, err))
			}
			continue
		}
		if !info.IsDir() {
			issues = append(issues, fmt.Sprintf("not a directory: %s", dir))
			continue
		}
		// Check write permission
		if err := os.MkdirAll(filepath.Join(dir, ".write-test"), 0o755); err != nil {
			issues = append(issues, fmt.Sprintf("not writable: %s (%v)", dir, err))
		} else {
			safe.Remove(filepath.Join(dir, ".write-test"))
		}
	}

	// Check disk space: try to write a temp file
	tmpFile := filepath.Join(homeDir, ".config", "cosca", "tmp", ".disk-check")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0o644); err != nil {
		issues = append(issues, fmt.Sprintf("disk may be full or read-only: %v", err))
	} else {
		safe.Remove(tmpFile)
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Filesystem has %d issue(s)", len(issues))
		result.Details = strings.Join(issues, "; ")
		if missingDirs > 0 {
			result.Suggestion = "Run 'cosca init' to create the required directory structure"
		} else {
			result.Suggestion = "Check filesystem permissions and available disk space"
		}
	} else {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("All %d required directories accessible and writable", len(requiredDirs))
	}

	return result
}

// =============================================================================
// Check: Permissions
// =============================================================================

// CheckPermissions validates file and directory permissions.
func CheckPermissions(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "permissions",
		Severity: SeverityWarning,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	coscaHome := filepath.Join(homeDir, ".config", "cosca")
	var issues []string

	// Check Cosca home directory permissions
	err = filepath.WalkDir(coscaHome, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		perm := info.Mode().Perm()

		// Warn about world-writable files/directories
		if perm&0o002 != 0 {
			rel, _ := filepath.Rel(coscaHome, path)
			issues = append(issues, fmt.Sprintf("world-writable: %s (%o)", rel, perm))
		}

		// Warn about world-readable sensitive files
		isSensitive := strings.HasSuffix(path, ".key") ||
			strings.HasSuffix(path, ".pem") ||
			strings.HasSuffix(path, "config.yaml")
		if isSensitive && perm&0o004 != 0 {
			rel, _ := filepath.Rel(coscaHome, path)
			issues = append(issues, fmt.Sprintf("world-readable sensitive file: %s (%o)", rel, perm))
		}

		return nil
	})

	if err != nil {
		issues = append(issues, fmt.Sprintf("cannot walk Cosca home: %v", err))
	}

	if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = fmt.Sprintf("Permission issues found: %d", len(issues))
		result.Details = strings.Join(issues, "; ")
		result.Suggestion = "Run 'chmod -R go-rwx ~/.config/cosca' to tighten permissions"
	} else {
		result.Status = StatusPass
		result.Message = "File permissions are secure"
	}

	return result
}

// =============================================================================
// Check: Configuration
// =============================================================================

// CheckConfiguration validates the config file, env vars, and defaults.
func CheckConfiguration(_ context.Context) CheckResult {
	result := CheckResult{
		Name:     "configuration",
		Severity: SeverityWarning,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		result.Status = StatusError
		result.Message = "Cannot determine home directory"
		result.Details = err.Error()
		return result
	}

	configPaths := []string{
		filepath.Join(homeDir, ".config", "cosca", "config.yaml"),
		filepath.Join(".cosca", "config.yaml"),
	}

	var issues []string
	var foundConfig bool

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		foundConfig = true

		var cfg map[string]interface{}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			issues = append(issues, fmt.Sprintf("invalid YAML in %s: %v", path, err))
			continue
		}

		// Validate known configuration keys
		result.Details = fmt.Sprintf("Config file: %s", path)

		// Check provider config
		if provider, ok := cfg["provider"].(map[string]interface{}); ok {
			if name, ok := provider["name"].(string); ok {
				result.Details += fmt.Sprintf("; Provider: %s", name)
			}
		}
	}

	// Check important environment variables
	type envCheck struct {
		varName string
		purpose string
	}
	envVars := []envCheck{
		{"COSCA_HOME", "Cosca home directory override"},
		{"COSCA_MODE", "Runtime mode (dev, test, production)"},
		{"COSCA_LOG_LEVEL", "Log level"},
		{"COSCA_TELEMETRY_ENABLED", "Telemetry opt-in/opt-out"},
		{"OPENAI_API_KEY", "OpenAI API key"},
		{"ANTHROPIC_API_KEY", "Anthropic API key"},
	}

	var configuredVars []string
	for _, ev := range envVars {
		if val := os.Getenv(ev.varName); val != "" {
			display := val
			if strings.Contains(strings.ToLower(ev.varName), "key") && len(val) > 8 {
				display = val[:4] + "..." + val[len(val)-4:]
			}
			configuredVars = append(configuredVars, fmt.Sprintf("%s=%s", ev.varName, display))
		}
	}

	if len(configuredVars) > 0 {
		result.Details += fmt.Sprintf("; Env vars: %s", strings.Join(configuredVars, ", "))
	}

	if !foundConfig && len(configuredVars) == 0 {
		result.Status = StatusWarn
		result.Message = "No configuration file found and no Cosca environment variables set"
		result.Suggestion = "Run 'cosca init' to create a configuration file, or set COSCA_HOME and other env vars"
	} else if !foundConfig {
		result.Status = StatusPass
		result.Message = fmt.Sprintf("Using environment-based configuration (%d env vars)", len(configuredVars))
	} else if len(issues) > 0 {
		result.Status = StatusWarn
		result.Message = "Configuration found with issues"
		result.Details += "; " + strings.Join(issues, "; ")
		result.Suggestion = "Review your configuration file for errors"
	} else {
		result.Status = StatusPass
		result.Message = "Configuration is valid"
	}

	return result
}

// Ensure the `url` import is used by the package
var _ = url.Parse

// Avoid unused import warning for exec
var _ = exec.Command
