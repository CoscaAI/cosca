package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/plugins"
)

// Plugins Adapter
// =============================================================================

// pluginManagerAdapter wraps plugins.Manager to match CLI expected API.
type pluginManagerAdapter struct {
	inner      *plugins.Manager
	pluginsDir string
}

// newPluginManagerAdapter creates a plugin manager adapter.
// CLI uses plugins.NewManager(path) but real API needs (config, loader).
func newPluginManagerAdapter(pluginDir string) *pluginManagerAdapter {
	cfg := plugins.DefaultManagerConfig(filepath.Dir(pluginDir))
	cfg.Dir = pluginDir
	mgr := plugins.NewManager(cfg, nil)
	return &pluginManagerAdapter{inner: mgr, pluginsDir: pluginDir}
}

// Install installs a plugin. CLI expects (name, source string) but real API takes just source.
func (a *pluginManagerAdapter) Install(name, source string) (PluginInfoEx, error) {
	if a.inner == nil {
		return PluginInfoEx{}, fmt.Errorf("plugin manager not available")
	}
	src := source
	if src == "" {
		src = name
	}
	if err := a.inner.Install(src); err != nil {
		return PluginInfoEx{}, err
	}
	// Get the plugin info after install
	info, err := a.inner.Get(name)
	if err != nil {
		return PluginInfoEx{}, err
	}
	return toPluginInfoEx(info), nil
}

// Uninstall removes a plugin.
func (a *pluginManagerAdapter) Uninstall(name string) error {
	if a.inner == nil {
		return fmt.Errorf("plugin manager not available")
	}
	return a.inner.Uninstall(name)
}

// List returns all installed plugins.
func (a *pluginManagerAdapter) List() []PluginInfoEx {
	if a.inner == nil {
		return nil
	}
	list := a.inner.List()
	result := make([]PluginInfoEx, len(list))
	for i, p := range list {
		result[i] = toPluginInfoEx(p)
	}
	return result
}

// Update updates a plugin. CLI expects (updated plugin, error).
func (a *pluginManagerAdapter) Update(name string) (PluginInfoEx, error) {
	if a.inner == nil {
		return PluginInfoEx{}, fmt.Errorf("plugin manager not available")
	}
	if err := a.inner.Update(name); err != nil {
		return PluginInfoEx{}, err
	}
	info, err := a.inner.Get(name)
	if err != nil {
		return PluginInfoEx{}, err
	}
	return toPluginInfoEx(info), nil
}

// Search searches for plugins. Placeholder.
func (a *pluginManagerAdapter) Search(_ string) ([]PluginSearchResult, error) {
	return nil, nil
}

// Info returns plugin info. CLI expects (info, error).
func (a *pluginManagerAdapter) Info(name string) (PluginInfoEx, error) {
	if a.inner == nil {
		return PluginInfoEx{}, fmt.Errorf("plugin manager not available")
	}
	info, err := a.inner.Get(name)
	if err != nil {
		return PluginInfoEx{}, err
	}
	return toPluginInfoEx(info), nil
}

// Enable enables a plugin.
func (a *pluginManagerAdapter) Enable(name string) error {
	if a.inner == nil {
		return fmt.Errorf("plugin manager not available")
	}
	return a.inner.Enable(name)
}

// Disable disables a plugin.
func (a *pluginManagerAdapter) Disable(name string) error {
	if a.inner == nil {
		return fmt.Errorf("plugin manager not available")
	}
	return a.inner.Disable(name)
}

// Scan scans the plugins directory. Placeholder for install flow.
func (a *pluginManagerAdapter) Scan() {
	// no-op
}

// PluginInfoEx mirrors PluginInfo for CLI command consumption.
type PluginInfoEx struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	License      string    `json:"license"`
	Homepage     string    `json:"homepage"`
	Enabled      bool      `json:"enabled"`
	InstalledAt  time.Time `json:"installed_at"`
	Dependencies []string  `json:"dependencies,omitempty"`
	Downloads    int       `json:"downloads,omitempty"`
}

// PluginSearchResult holds plugin search results.
type PluginSearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Downloads   int    `json:"downloads"`
}

func toPluginInfoEx(p plugins.PluginInfo) PluginInfoEx {
	return PluginInfoEx{
		Name:        p.Manifest.Name,
		Version:     p.Manifest.Version,
		Description: p.Manifest.Description,
		Author:      p.Manifest.Author,
		Enabled:     p.Enabled,
		InstalledAt: p.InstalledAt,
	}
}

// =============================================================================
