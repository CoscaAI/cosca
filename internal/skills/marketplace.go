// Package skills — plugin marketplaces.
//
// A marketplace is a Claude Code / Codex style manifest describing installable
// Agent Plugins by their GitHub source. The canonical example is google/skills
// .claude-plugin/marketplace.json:
//
//	{
//	  "name": "google-plugins",
//	  "metadata": {"description": "..."},
//	  "plugins": [
//	    {"name": "alloydb", "source": {"source": "github", "repo": "gemini-cli-extensions/alloydb", "ref": "0.2.0"}, "description": "..."},
//	    {"name": "db-context-engineering", "source": {"source": "git-subdir", "url": "GoogleCloudPlatform/db-context-enrichment", "path": "plugin", "ref": "v0.6.0"}, "description": "..."}
//	  ]
//	}
//
// The marketplace.json may live at <repo>/.claude-plugin/marketplace.json,
// <repo>/.codex-plugin/marketplace.json, or <repo>/marketplace.json — these
// three locations are searched in order. Listing a marketplace only fetches
// and parses the manifest; nothing is installed until the user asks for a
// specific plugin (or "all").
//
// SECURITY: remote marketplace fetches are fail-closed exactly like skill and
// plugin installs — ListMarketplace and InstallPluginFromMarketplace require
// allowRemote for the GitHub download.
package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marketplace is a parsed marketplace.json manifest. Description is read from
// the nested metadata.description field of the real format.
type Marketplace struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Plugins     []MarketplacePlugin `json:"plugins"`
}

// MarketplacePlugin is a single plugin entry of a marketplace manifest.
type MarketplacePlugin struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Source      MarketplaceSource `json:"source"`
}

// MarketplaceSource is the source block of a marketplace plugin entry. For
// "github" sources Repo holds "owner/repo"; for "git-subdir" sources URL holds
// "owner/repo" and Path the subdirectory inside it.
type MarketplaceSource struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
	Ref    string `json:"ref"`
	URL    string `json:"url"`
	Path   string `json:"path"`
}

// parseMarketplace parses and validates a marketplace.json manifest. Only the
// name is required; a missing or empty plugins list is tolerated.
func parseMarketplace(data []byte) (*Marketplace, error) {
	var raw struct {
		Name     string `json:"name"`
		Metadata struct {
			Description string `json:"description"`
		} `json:"metadata"`
		Plugins []MarketplacePlugin `json:"plugins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse marketplace: %w", err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("marketplace name is required")
	}
	mp := &Marketplace{
		Name:        raw.Name,
		Description: raw.Metadata.Description,
		Plugins:     raw.Plugins,
	}
	if mp.Plugins == nil {
		mp.Plugins = []MarketplacePlugin{}
	}
	return mp, nil
}

// marketplaceJSONLocations lists where a marketplace manifest can live within
// a repository, in priority order.
var marketplaceJSONLocations = []string{
	".claude-plugin/marketplace.json",
	".codex-plugin/marketplace.json",
	"marketplace.json",
}

// loadMarketplaceFromDir locates and parses a marketplace.json inside dir,
// searching the three standard locations in order. Shared by ListMarketplace
// and tests so the lookup logic is exercised without a network.
func loadMarketplaceFromDir(dir string) (*Marketplace, error) {
	for _, rel := range marketplaceJSONLocations {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		mp, err := parseMarketplace(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		return mp, nil
	}
	return nil, fmt.Errorf("no marketplace.json found in %s (searched .claude-plugin/marketplace.json, .codex-plugin/marketplace.json, marketplace.json)", dir)
}

// ListMarketplace fetches a GitHub reference ("owner/repo[/path][@ref]"),
// locates its marketplace.json, and returns the parsed manifest WITHOUT
// installing anything. allowRemote must be true for the network fetch
// (fail-closed).
func (m *Manager) ListMarketplace(ref string, allowRemote bool) (*Marketplace, error) {
	if !allowRemote {
		return nil, fmt.Errorf("remote marketplace lookup is disabled: pass --allow-remote to enable it (e.g. cosca skill marketplace list %s --allow-remote)", ref)
	}
	parsed, err := parseGitHubRef(ref)
	if err != nil {
		return nil, err
	}

	dir, cleanup, err := downloadAndExtractRepo(parsed)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	root, err := singleTopLevelDir(dir)
	if err != nil {
		return nil, err
	}
	if parsed.Path != "" {
		root = filepath.Join(root, filepath.FromSlash(parsed.Path))
		if _, err := os.Stat(root); err != nil {
			return nil, fmt.Errorf("path %q not found in %s/%s: %w", parsed.Path, parsed.Owner, parsed.Repo, err)
		}
	}
	return loadMarketplaceFromDir(root)
}

// InstallPluginFromMarketplace lists the marketplace at marketplaceRef, finds
// pluginName (exact or case-insensitive), and installs it. When pluginName is
// "all" every plugin in the marketplace is installed: the first installed
// plugin is returned and the others are reported as warnings.
func (m *Manager) InstallPluginFromMarketplace(marketplaceRef, pluginName string, allowRemote bool) (*Plugin, []string, error) {
	mp, err := m.ListMarketplace(marketplaceRef, allowRemote)
	if err != nil {
		return nil, nil, err
	}
	return m.installPluginFromMarketplace(mp, pluginName, allowRemote)
}

// installPluginFromMarketplace installs plugins described by a parsed
// Marketplace. Factored out so tests can drive the full install path with a
// constructed Marketplace instead of a network fetch.
func (m *Manager) installPluginFromMarketplace(mp *Marketplace, pluginName string, allowRemote bool) (*Plugin, []string, error) {
	if pluginName == "" {
		return nil, nil, fmt.Errorf("plugin name is required")
	}

	var targets []MarketplacePlugin
	if pluginName == "all" {
		targets = mp.Plugins
	} else {
		p, err := findMarketplacePlugin(mp, pluginName)
		if err != nil {
			return nil, nil, err
		}
		targets = []MarketplacePlugin{*p}
	}

	var warnings []string
	var first *Plugin
	for i, p := range targets {
		installed, err := m.installMarketplacePlugin(p, allowRemote)
		if err != nil {
			return first, warnings, fmt.Errorf("install plugin %q: %w", p.Name, err)
		}
		if i == 0 {
			first = installed
		} else {
			warnings = append(warnings, fmt.Sprintf("also installed %q", installed.Name))
		}
	}
	return first, warnings, nil
}

// findMarketplacePlugin locates a plugin by name (exact match, then
// case-insensitive) in the marketplace manifest.
func findMarketplacePlugin(mp *Marketplace, name string) (*MarketplacePlugin, error) {
	for i := range mp.Plugins {
		if mp.Plugins[i].Name == name {
			return &mp.Plugins[i], nil
		}
	}
	lower := strings.ToLower(name)
	for i := range mp.Plugins {
		if strings.ToLower(mp.Plugins[i].Name) == lower {
			return &mp.Plugins[i], nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in marketplace %q", name, mp.Name)
}

// installMarketplacePlugin installs a single marketplace plugin by resolving
// its source block into an InstallPlugin GitHub reference:
//
//	github     → repo[/path][@ref]
//	git-subdir → url[/path][@ref]
//
// The resulting reference may point at a local directory (os.Stat check inside
// InstallPlugin), which is how tests exercise this path without a network.
func (m *Manager) installMarketplacePlugin(p MarketplacePlugin, allowRemote bool) (*Plugin, error) {
	base := ""
	switch p.Source.Source {
	case "github":
		base = p.Source.Repo
	case "git-subdir":
		base = p.Source.URL
	default:
		return nil, fmt.Errorf("unsupported marketplace source %q for plugin %q (expected github or git-subdir)", p.Source.Source, p.Name)
	}
	if base == "" {
		return nil, fmt.Errorf("plugin %q has no repository source", p.Name)
	}
	ref := base
	if p.Source.Path != "" {
		ref += "/" + strings.Trim(p.Source.Path, "/")
	}
	if p.Source.Ref != "" {
		ref += "@" + p.Source.Ref
	}
	return m.InstallPlugin(ref, allowRemote)
}
