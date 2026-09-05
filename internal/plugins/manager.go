//
// Plugin Manager: lifecycle management, installation, updates, and dependency resolution.

package plugins

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// Manager
// =============================================================================

// Manager handles the lifecycle of all plugins: installation, updates,
// dependency resolution, enable/disable, and enumeration.
type Manager struct {
	mu sync.RWMutex

	// plugins maps plugin ID to plugin instance.
	plugins map[string]pluginEntry

	// loader is used to load plugins from disk.
	loader *Loader

	// pluginsDir is the base directory for installed plugins.
	pluginsDir string

	// registryURL is the URL of the plugin registry.
	registryURL string

	// httpClient is used for registry communication.
	httpClient *http.Client

	// config holds the plugin manager configuration.
	config ManagerConfig
}

// ManagerConfig configures the plugin manager.
type ManagerConfig struct {
	// Dir is the plugins directory path.
	Dir string
	// RegistryURL is the plugin registry URL.
	RegistryURL string
	// AllowInstallFrom specifies allowed installation sources (local, git, registry).
	AllowInstallFrom []string
	// MaxPluginSize is the maximum plugin archive size in bytes.
	MaxPluginSize int64
	// VerifyChecksum enables SHA-256 checksum verification on install.
	VerifyChecksum bool
}

// pluginEntry holds a loaded plugin along with its metadata.
type pluginEntry struct {
	plugin    Plugin
	info      PluginInfo
	startedAt time.Time
}

// DefaultManagerConfig returns a default configuration for the plugin manager.
func DefaultManagerConfig(baseDir string) ManagerConfig {
	return ManagerConfig{
		Dir:              filepath.Join(baseDir, "plugins"),
		RegistryURL:      "https://plugins.cosca.dev/v1",
		AllowInstallFrom: []string{"local", "git", "registry"},
		MaxPluginSize:    50 * 1024 * 1024, // 50 MB
		VerifyChecksum:   true,
	}
}

// NewManager creates a new plugin manager.
func NewManager(config ManagerConfig, loader *Loader) *Manager {
	if loader == nil {
		loader = NewLoader(LoaderConfig{
			PluginsDir: config.Dir,
		})
	}

	return &Manager{
		plugins:     make(map[string]pluginEntry),
		loader:      loader,
		pluginsDir:  config.Dir,
		registryURL: config.RegistryURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
		config: config,
	}
}

// =============================================================================
// Installation
// =============================================================================

// Install installs a plugin from the given source.
// Supported sources:
//   - Local path: "file:///path/to/plugin.tar.gz" or "/path/to/plugin.tar.gz"
//   - Git URL:    "https://github.com/user/plugin.git"
//   - Registry:   "registry://plugin-id" or just "plugin-id"
func (m *Manager) Install(src string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse source
	sourceType, pluginID, err := m.parseSource(src)
	if err != nil {
		return fmt.Errorf("install: parse source: %w", err)
	}

	// Check if already installed
	if _, exists := m.plugins[pluginID]; exists {
		return fmt.Errorf("install %q: %w", pluginID, ErrPluginAlreadyInstalled)
	}

	// Ensure plugins directory exists
	if err := os.MkdirAll(m.pluginsDir, 0o755); err != nil {
		return fmt.Errorf("install: create plugins dir: %w", err)
	}

	log.Debug().Str("source", src).Str("type", sourceType).Str("plugin", pluginID).
		Msg("installing plugin")

	switch sourceType {
	case "local":
		if err := m.installFromLocal(src, pluginID); err != nil {
			return fmt.Errorf("install from local: %w", err)
		}
	case "git":
		if err := m.installFromGit(src, pluginID); err != nil {
			return fmt.Errorf("install from git: %w", err)
		}
	case "registry":
		if err := m.installFromRegistry(pluginID); err != nil {
			return fmt.Errorf("install from registry: %w", err)
		}
	default:
		return fmt.Errorf("unsupported install source: %s", sourceType)
	}

	// Load the newly installed plugin
	pluginDir := filepath.Join(m.pluginsDir, pluginID)
	plugin, err := m.loader.LoadPlugin(pluginDir)
	if err != nil {
		return fmt.Errorf("install: load plugin: %w", err)
	}

	m.plugins[pluginID] = pluginEntry{
		plugin: plugin,
		info: PluginInfo{
			Manifest: PluginManifest{
				ID:          plugin.ID(),
				Name:        plugin.Name(),
				Version:     plugin.Version(),
				Description: plugin.Description(),
				Author:      plugin.Author(),
			},
			State:       PluginStateInstalled,
			Enabled:     true,
			InstalledAt: time.Now(),
		},
	}

	log.Info().Str("plugin", pluginID).Msg("plugin installed successfully")
	return nil
}

// installFromLocal installs a plugin from a local file path.
func (m *Manager) installFromLocal(src, pluginID string) error {
	srcPath := strings.TrimPrefix(src, "file://")

	// Determine if source is a directory or archive
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("access source: %w", err)
	}

	destDir := filepath.Join(m.pluginsDir, pluginID)

	if info.IsDir() {
		// Copy directory
		return m.copyDirectory(srcPath, destDir)
	}

	// Extract archive
	return m.extractArchive(srcPath, destDir)
}

// installFromGit installs a plugin from a Git repository.
func (m *Manager) installFromGit(src, pluginID string) error {
	destDir := filepath.Join(m.pluginsDir, pluginID)

	// Use git clone to download the repository
	// This is a simplified implementation; production would use go-git
	// or exec git clone.
	args := []string{"clone", "--depth", "1", src, destDir}
	if err := m.execCommand("git", args...); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Remove .git directory to save space
	safe.RemoveAll(filepath.Join(destDir, ".git"))

	return nil
}

// installFromRegistry downloads and installs a plugin from the registry.
func (m *Manager) installFromRegistry(pluginID string) error {
	// Fetch manifest from registry
	manifestURL := fmt.Sprintf("%s/plugins/%s/manifest", m.registryURL, pluginID)
	resp, err := m.httpClient.Get(manifestURL)
	if err != nil {
		return fmt.Errorf("fetch manifest: %w", err)
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry: manifest fetch returned %s", resp.Status)
	}

	var manifest PluginManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	// Download the plugin archive
	downloadURL := fmt.Sprintf("%s/plugins/%s/%s/download", m.registryURL, pluginID, manifest.Version)
	dlResp, err := m.httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download plugin: %w", err)
	}
	defer safe.Close(dlResp.Body)

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry: download returned %s", dlResp.Status)
	}

	destDir := filepath.Join(m.pluginsDir, pluginID)
	tmpFile, err := os.CreateTemp("", pluginID+"-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		safe.Remove(tmpFile.Name())
		return fmt.Errorf("chmod temp file: %w", err)
	}

	// Write download to temp file, computing SHA-256 checksum if configured
	var computeHash hash.Hash
	if m.config.VerifyChecksum && manifest.Checksum != "" {
		computeHash = sha256.New()
	}
	multiWriter := io.MultiWriter(tmpFile)
	if computeHash != nil {
		multiWriter = io.MultiWriter(tmpFile, computeHash)
	}
	limited := io.LimitReader(dlResp.Body, m.config.MaxPluginSize)
	if _, err := io.Copy(multiWriter, limited); err != nil {
		_ = tmpFile.Close()
		safe.Remove(tmpFile.Name())
		return fmt.Errorf("download write: %w", err)
	}
	_ = tmpFile.Close()
	defer safe.Remove(tmpFile.Name())

	// Verify checksum
	if m.config.VerifyChecksum && manifest.Checksum != "" && computeHash != nil {
		computedSum := fmt.Sprintf("sha256-%x", computeHash.Sum(nil))
		if computedSum != manifest.Checksum {
			return fmt.Errorf("checksum mismatch: got %s, want %s", computedSum, manifest.Checksum)
		}
	}

	// Extract archive
	if err := m.extractArchive(tmpFile.Name(), destDir); err != nil {
		return fmt.Errorf("extract plugin: %w", err)
	}

	// Write manifest
	manifestPath := filepath.Join(destDir, "manifest.json")
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// =============================================================================
// Uninstallation
// =============================================================================

// Uninstall removes a plugin and its data.
func (m *Manager) Uninstall(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("uninstall %q: %w", id, ErrPluginNotFound)
	}

	// Stop the plugin if it's running
	if entry.info.State == PluginStateStarted {
		if err := entry.plugin.Stop(); err != nil {
			log.Warn().Err(err).Str("plugin", id).Msg("plugin stop during uninstall failed")
		}
	}

	// Remove plugin directory
	pluginDir := filepath.Join(m.pluginsDir, id)
	if err := os.RemoveAll(pluginDir); err != nil {
		log.Warn().Err(err).Str("plugin", id).Msg("failed to remove plugin directory")
	}

	delete(m.plugins, id)
	log.Info().Str("plugin", id).Msg("plugin uninstalled")
	return nil
}

// =============================================================================
// Updates
// =============================================================================

// Update checks for and applies plugin updates.
func (m *Manager) Update(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("update %q: %w", id, ErrPluginNotFound)
	}

	// Check registry for newer version
	manifestURL := fmt.Sprintf("%s/plugins/%s/manifest", m.registryURL, id)
	resp, err := m.httpClient.Get(manifestURL)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update check: registry returned %s", resp.Status)
	}

	var remoteManifest PluginManifest
	if err := json.NewDecoder(resp.Body).Decode(&remoteManifest); err != nil {
		return fmt.Errorf("decode remote manifest: %w", err)
	}

	// Compare versions (simple string comparison — production should use semver)
	currentVer := entry.info.Manifest.Version
	remoteVer := remoteManifest.Version
	if remoteVer <= currentVer {
		log.Info().Str("plugin", id).
			Str("current", currentVer).
			Str("remote", remoteVer).
			Msg("plugin is up to date")
		return nil
	}

	// Stop plugin if running
	if entry.info.State == PluginStateStarted {
		if err := entry.plugin.Stop(); err != nil {
			return fmt.Errorf("stop plugin for update: %w", err)
		}
	}

	// Download and replace
	pluginDir := filepath.Join(m.pluginsDir, id)

	// Create backup
	backupDir := pluginDir + ".backup"
	if err := os.Rename(pluginDir, backupDir); err != nil {
		return fmt.Errorf("backup plugin: %w", err)
	}
	defer func() { _ = os.RemoveAll(backupDir) }()

	// Reinstall from registry
	if err := m.installFromRegistry(id); err != nil {
		// Restore from backup
		_ = os.RemoveAll(pluginDir)
		_ = os.Rename(backupDir, pluginDir)
		return fmt.Errorf("update install: %w", err)
	}

	// Reload plugin
	loadedPlugin, err := m.loader.LoadPlugin(pluginDir)
	if err != nil {
		return fmt.Errorf("update: reload plugin: %w", err)
	}

	m.plugins[id] = pluginEntry{
		plugin: loadedPlugin,
		info: PluginInfo{
			Manifest:    remoteManifest,
			State:       PluginStateInstalled,
			Enabled:     entry.info.Enabled,
			InstalledAt: entry.info.InstalledAt,
			UpdatedAt:   time.Now(),
		},
	}

	log.Info().Str("plugin", id).
		Str("from", currentVer).
		Str("to", remoteVer).
		Msg("plugin updated")
	return nil
}

// =============================================================================
// Enable / Disable
// =============================================================================

// Enable enables a plugin for loading and execution.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("enable %q: %w", id, ErrPluginNotFound)
	}

	entry.info.Enabled = true
	m.plugins[id] = entry
	log.Info().Str("plugin", id).Msg("plugin enabled")
	return nil
}

// Disable disables a plugin without uninstalling it.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("disable %q: %w", id, ErrPluginNotFound)
	}

	// Stop if running
	if entry.info.State == PluginStateStarted {
		if err := entry.plugin.Stop(); err != nil {
			log.Warn().Err(err).Str("plugin", id).Msg("stop during disable")
		}
		entry.info.State = PluginStateStopped
	}

	entry.info.Enabled = false
	m.plugins[id] = entry
	log.Info().Str("plugin", id).Msg("plugin disabled")
	return nil
}

// =============================================================================
// Query
// =============================================================================

// List returns information about all installed plugins.
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PluginInfo, 0, len(m.plugins))
	for _, entry := range m.plugins {
		result = append(result, entry.info)
	}

	// Sort by name for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Manifest.ID < result[j].Manifest.ID
	})

	return result
}

// Get returns information about a specific plugin.
func (m *Manager) Get(id string) (PluginInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.plugins[id]
	if !exists {
		return PluginInfo{}, fmt.Errorf("get %q: %w", id, ErrPluginNotFound)
	}
	return entry.info, nil
}

// GetPlugin returns the real Plugin instance by ID.
func (m *Manager) GetPlugin(id string) (Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, exists := m.plugins[id]
	if !exists {
		return nil, fmt.Errorf("get plugin %q: %w", id, ErrPluginNotFound)
	}
	if entry.plugin == nil {
		return nil, fmt.Errorf("plugin %q not loaded", id)
	}
	return entry.plugin, nil
}

// =============================================================================
// Dependency Resolution
// =============================================================================

// ResolveDependencies orders plugins by their dependencies using
// topological sorting, ensuring dependent plugins come after their
// dependencies.
func (m *Manager) ResolveDependencies() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build dependency graph
	graph := make(map[string][]string) // pluginID -> dependencies
	for id, entry := range m.plugins {
		deps := make([]string, 0, len(entry.info.Manifest.Dependencies))
		for _, dep := range entry.info.Manifest.Dependencies {
			deps = append(deps, dep.PluginID)
		}
		graph[id] = deps
	}

	// Topological sort (Kahn's algorithm)
	order, err := topologicalSort(graph)
	if err != nil {
		return fmt.Errorf("dependency resolution: %w", err)
	}

	// Rebuild plugins map in dependency order
	ordered := make(map[string]pluginEntry, len(m.plugins))
	for _, id := range order {
		if entry, ok := m.plugins[id]; ok {
			ordered[id] = entry
		}
	}

	m.plugins = ordered
	log.Debug().Int("count", len(order)).Msg("dependencies resolved")
	return nil
}

// ValidateDependencies checks that all plugin dependencies are satisfied.
func (m *Manager) ValidateDependencies() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, entry := range m.plugins {
		for _, dep := range entry.info.Manifest.Dependencies {
			depEntry, exists := m.plugins[dep.PluginID]
			if !exists {
				if !dep.Optional {
					return &ErrDependencyMissing{
						PluginID:   id,
						Dependency: dep,
					}
				}
				log.Warn().Str("plugin", id).
					Str("missing_dep", dep.PluginID).
					Msg("optional dependency not installed")
				continue
			}

			if !depEntry.info.Enabled {
				if !dep.Optional {
					return &ErrDependencyMissing{
						PluginID:   id,
						Dependency: dep,
					}
				}
			}

			// Version check (simplified — production should use semver comparison)
			if dep.Version != "" && depEntry.info.Manifest.Version != dep.Version {
				if !dep.Optional {
					return fmt.Errorf("plugin %q: dependency %q version mismatch: want %s, have %s",
						id, dep.PluginID, dep.Version, depEntry.info.Manifest.Version)
				}
			}
		}
	}

	return nil
}

// =============================================================================
// Internal Helpers
// =============================================================================

// parseSource determines the source type from a source string.
func (m *Manager) parseSource(src string) (sourceType, pluginID string, err error) {
	if src == "" {
		return "", "", fmt.Errorf("empty source")
	}

	// Check for explicit scheme
	if strings.HasPrefix(src, "file://") {
		return "local", m.extractPluginIDFromPath(strings.TrimPrefix(src, "file://")), nil
	}
	if strings.HasPrefix(src, "registry://") {
		return "registry", strings.TrimPrefix(src, "registry://"), nil
	}
	if strings.HasPrefix(src, "git://") || strings.HasPrefix(src, "https://") && strings.HasSuffix(src, ".git") {
		return "git", m.extractPluginIDFromGitURL(src), nil
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		// Could be git URL or registry URL
		if strings.Contains(src, ".git") {
			return "git", m.extractPluginIDFromGitURL(src), nil
		}
		// Assume it's a local/remote archive
		return "local", m.extractPluginIDFromPath(src), nil
	}

	// Check if it's a local path
	if info, statErr := os.Stat(src); statErr == nil {
		_ = info // file/dir exists
		return "local", m.extractPluginIDFromPath(src), nil
	}

	// Default to registry lookup
	return "registry", src, nil
}

// extractPluginIDFromPath extracts a plugin ID from a file path.
func (m *Manager) extractPluginIDFromPath(path string) string {
	base := filepath.Base(path)
	// Remove common extensions
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".tgz")
	base = strings.TrimSuffix(base, ".zip")
	base = strings.TrimSuffix(base, ".so")
	base = strings.TrimSuffix(base, ".wasm")
	return base
}

// extractPluginIDFromGitURL extracts a plugin ID from a git URL.
func (m *Manager) extractPluginIDFromGitURL(gitURL string) string {
	// Extract repo name: https://github.com/user/repo.git -> repo
	u, err := url.Parse(gitURL)
	if err != nil {
		return filepath.Base(gitURL)
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, ".git"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractArchive extracts a tar.gz archive to the specified directory.
func (m *Manager) extractArchive(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer safe.Close(f)

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer safe.Close(gzr)

	tr := tar.NewReader(gzr)

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Sanitize path
		target := filepath.Join(dest, filepath.Clean(header.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("archive path traversal detected: %s", header.Name)
		}

		// Sanitize permissions: strip setuid/setgid/sticky bits
		fileMode := os.FileMode(header.Mode) & 0o777

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fileMode); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close file: %w", err)
			}
		case tar.TypeSymlink:
			// Reject absolute link targets. os.Symlink writes the Linkname
			// verbatim, so an absolute target would point outside the
			// destination even though the old prefix check passed
			// (filepath.Join treats an absolute path as relative).
			if filepath.IsAbs(header.Linkname) {
				return fmt.Errorf("symlink target %q must be relative", header.Linkname)
			}
			// A relative symlink resolves from the directory of the link
			// itself, not from dest. Validate the resolved target stays
			// inside the destination.
			linkResolved := filepath.Join(filepath.Dir(target), filepath.Clean(header.Linkname))
			targetDir := filepath.Clean(dest)
			if linkResolved != targetDir &&
				!strings.HasPrefix(linkResolved, targetDir+string(os.PathSeparator)) {
				return fmt.Errorf("symlink target %q escapes destination", header.Linkname)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink: %w", err)
			}
		case tar.TypeLink:
			// Hardlink targets must resolve inside the destination too —
			// a Linkname like "../../etc/passwd" would otherwise link a
			// host file into the plugin directory.
			linkTarget := filepath.Join(filepath.Dir(target), filepath.Clean(header.Linkname))
			targetDir := filepath.Clean(dest)
			if linkTarget != targetDir &&
				!strings.HasPrefix(linkTarget, targetDir+string(os.PathSeparator)) {
				return fmt.Errorf("hardlink target %q escapes destination", header.Linkname)
			}
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("create hardlink: %w", err)
			}
		}
	}

	return nil
}

// copyDirectory recursively copies a directory.
func (m *Manager) copyDirectory(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// execCommand executes an external command with stdout/stderr passthrough.
func (m *Manager) execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s: %w", name, err)
	}
	return nil
}

// =============================================================================
// Topological Sort
// =============================================================================

// topologicalSort performs a topological sort on a dependency graph
// using Kahn's algorithm.
func topologicalSort(graph map[string][]string) ([]string, error) {
	inDegree := make(map[string]int)
	for node := range graph {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep]++
		}
	}

	// Queue nodes with no dependencies
	queue := make([]string, 0)
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]string, 0, len(graph))
	visited := make(map[string]bool)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if visited[node] {
			continue
		}
		visited[node] = true
		result = append(result, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(graph) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// =============================================================================
// Load Manifest Helpers
// =============================================================================

// loadManifestFromDir loads a plugin manifest from a directory.
// It looks for manifest.yaml, manifest.yml, or manifest.json.
func loadManifestFromDir(dir string) (*PluginManifest, error) {
	var manifest *PluginManifest

	// Try YAML first
	paths := []string{
		filepath.Join(dir, "manifest.yaml"),
		filepath.Join(dir, "manifest.yml"),
		filepath.Join(dir, "manifest.json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		manifest = &PluginManifest{}
		switch filepath.Ext(path) {
		case ".yaml", ".yml":
			if err := yaml.Unmarshal(data, manifest); err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
		case ".json":
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
		}
		break
	}

	if manifest == nil {
		return nil, fmt.Errorf("no manifest file found in %s", dir)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return manifest, nil
}
