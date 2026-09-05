// Package skills — Agent Plugins (v1.0.0) and marketplaces.
//
// Agent Plugins bundle one or more standard Agent Skills plus optional MCP
// server configuration under a plugin.json manifest (see the spec at
// https://agent-plugins.org/schemas/1.0.0/plugin.schema.json):
//
//	<plugin>/
//	  plugin.json          — manifest, only "name" is required
//	  mcp.json             — optional MCP server config (stdio/http/sse)
//	  skills/<skill>/SKILL.md — one or more standard Agent Skills
//
// Installing a plugin registers every bundled skill through the regular
// validated pipeline (installStandardSkill) so they become usable Agent
// Skills, copies the plugin payload into coscaDir/plugins/<name>/, and keeps
// the mcp.json config alongside for the MCP layer to consume.
//
// A marketplace (Claude Code / Codex style, e.g. google/skills
// .claude-plugin/marketplace.json) is a manifest describing plugins by their
// GitHub source; it can be listed and installed without vendoring the whole
// ecosystem.
//
// SECURITY posture (mirrors the rest of the package):
//   - The plugin name is validated as a slug BEFORE any filepath.Join, so
//     path traversal via the manifest name is impossible.
//   - Every write target is resolved through resolveWithinRoot, so nothing
//     escapes coscaDir/plugins or coscaDir/skills even if coscaDir is (or
//     contains) a symlink.
//   - Symlinks inside a plugin tree are rejected during copy.
//   - Remote (GitHub) sources are opt-in: InstallPlugin fails closed unless
//     allowRemote is set, exactly like InstallFromGitHub.
package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Plugin is an installed Agent Plugin (v1.0.0). It bundles standard Agent
// Skills plus optional MCP server configuration.
type Plugin struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Description string               `json:"description"`
	Repository  string               `json:"repository"`
	License     string               `json:"license"`
	Skills      []*Skill             `json:"skills"`
	MCPServers  map[string]MCPServer `json:"mcpServers,omitempty"`
	Source      string               `json:"source,omitempty"`
	Dir         string               `json:"dir,omitempty"`
	SkillNames  []string             `json:"skillNames,omitempty"`
}

// MCPServer is a single MCP server entry from a plugin's mcp.json. The config
// is registered alongside the plugin; the MCP client itself (internal/chat/mcp)
// is not invoked here.
type MCPServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// PluginManifest is the parsed plugin.json manifest. Per the spec only "name"
// is required; all other fields are tolerated when absent.
type PluginManifest struct {
	Schema      string       `json:"$schema"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Author      PluginAuthor `json:"author"`
	Homepage    string       `json:"homepage"`
	Repository  string       `json:"repository"`
	License     string       `json:"license"`
	Keywords    []string     `json:"keywords"`
}

// PluginAuthor is the optional author block of a plugin.json manifest.
type PluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// parsePluginManifest parses and validates a plugin.json manifest. The name
// must be present and a valid slug (reuses validateSkillName so a malicious
// or malformed name can never escape the plugins directory).
func parsePluginManifest(data []byte) (*PluginManifest, error) {
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse plugin.json: %w", err)
	}
	if err := validateSkillName(m.Name); err != nil {
		return nil, err
	}
	return &m, nil
}

// mcpServerTypes are the supported MCP transport types from the Agent Plugins
// spec. Only these are accepted — anything else is a hard error.
var mcpServerTypes = map[string]bool{
	"stdio":           true,
	"streamable-http": true,
	"sse":             true,
}

// parseMCPConfig parses a plugin's mcp.json ("mcpServers" key). It validates
// the transport type and that the required fields per type are present: stdio
// servers need a command; streamable-http and sse servers need a url. The
// "$schema" key is ignored.
func parseMCPConfig(data []byte) (map[string]MCPServer, error) {
	var raw struct {
		MCPServers map[string]MCPServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse mcp.json: %w", err)
	}
	if len(raw.MCPServers) == 0 {
		return nil, fmt.Errorf("mcp.json contains no mcpServers")
	}
	for name, srv := range raw.MCPServers {
		switch {
		case !mcpServerTypes[srv.Type]:
			return nil, fmt.Errorf("mcp server %q: invalid type %q (must be stdio, streamable-http, or sse)", name, srv.Type)
		case srv.Type == "stdio" && srv.Command == "":
			return nil, fmt.Errorf("mcp server %q: stdio servers require a command", name)
		case srv.Type != "stdio" && srv.URL == "":
			return nil, fmt.Errorf("mcp server %q: %s servers require a url", name, srv.Type)
		}
	}
	return raw.MCPServers, nil
}

// InstallPlugin installs an Agent Plugin from a LOCAL directory containing
// plugin.json OR a GitHub reference ("owner/repo[/path][@ref]"). It returns
// the installed plugin with its bundled skills registered in the Manager.
//
// Local directories may live anywhere on disk — the user points at them
// explicitly (like `docker build .`) — but every write still goes through
// resolveWithinRoot and the plugin name must be a slug. GitHub sources are
// fail-closed: they require allowRemote.
func (m *Manager) InstallPlugin(source string, allowRemote bool) (*Plugin, error) {
	if source == "" {
		return nil, fmt.Errorf("plugin source is required")
	}

	// Local directory containing plugin.json.
	expanded := expandHome(source)
	if fi, err := os.Stat(expanded); err == nil && fi.IsDir() {
		return m.installPluginFromDir(expanded, source)
	}

	// GitHub reference — opt-in only.
	if !allowRemote {
		return nil, fmt.Errorf("remote plugin installation is disabled: pass --allow-remote to enable it (e.g. cosca skill plugin install %s --allow-remote)", source)
	}
	parsed, err := parseGitHubRef(source)
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
	pluginRoot, err := locatePluginRoot(root, parsed.Path)
	if err != nil {
		return nil, err
	}
	return m.installPluginFromDir(pluginRoot, source)
}

// pluginManifestNames are the locations a plugin.json can live inside a plugin
// root. The Agent Plugins spec puts it at the root, but real google plugins
// ship it inside the extension directory (e.g. .claude-plugin/plugin.json)
// alongside the marketplace manifest — both are accepted.
var pluginManifestNames = []string{
	"plugin.json",
	filepath.Join(".claude-plugin", "plugin.json"),
	filepath.Join(".codex-plugin", "plugin.json"),
}

// findPluginManifest returns the path to the plugin.json inside dir, or ""
// when dir is not a plugin root.
func findPluginManifest(dir string) string {
	for _, rel := range pluginManifestNames {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// locatePluginRoot finds the plugin root inside an extracted repository: the
// parsed sub-path when it contains plugin.json, then the repo root, then a
// root/plugins/* plugin one level deep. On failure it reports what was found
// so the user can correct the source.
func locatePluginRoot(root, parsedPath string) (string, error) {
	check := func(p string) bool {
		return findPluginManifest(p) != ""
	}

	if parsedPath != "" {
		p := filepath.Join(root, filepath.FromSlash(parsedPath))
		if check(p) {
			return p, nil
		}
	}
	if check(root) {
		return root, nil
	}
	pluginsDir := filepath.Join(root, "plugins")
	if entries, err := os.ReadDir(pluginsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(pluginsDir, e.Name())
			if check(p) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("no plugin.json found in repository (checked root, root/plugins/*, and path %q; top-level entries: %s)",
		parsedPath, topLevelNames(root))
}

// topLevelNames lists the immediate entries of dir for error messages.
func topLevelNames(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "<unreadable>"
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "<empty>"
	}
	return strings.Join(names, ", ")
}

// installPluginFromDir installs a plugin rooted at sourceDir (the directory
// holding the plugin payload — skills/, mcp.json — with plugin.json either at
// its root or in .claude-plugin/.codex-plugin). The plugin payload is copied
// into coscaDir/plugins/<name>/, every bundled skill under skills/*/SKILL.md
// is installed through installStandardSkill, and the mcp.json config (when
// present) is stored alongside the plugin.
func (m *Manager) installPluginFromDir(sourceDir, sourceLabel string) (*Plugin, error) {
	manifestPath := findPluginManifest(sourceDir)
	if manifestPath == "" {
		return nil, fmt.Errorf("plugin.json not found in %s", sourceDir)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin.json: %w", err)
	}
	manifest, err := parsePluginManifest(data)
	if err != nil {
		return nil, err
	}
	name := manifest.Name
	if err := validateSkillName(name); err != nil {
		return nil, err
	}

	// Persist to coscaDir/plugins/<name>/. Defense in depth: the name is a
	// validated slug, and the target is still verified to stay inside the
	// plugins directory (symlinked coscaDir included).
	pluginsPath := m.PluginDir()
	target, err := resolveWithinRoot(pluginsPath, filepath.Join(pluginsPath, name))
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, fmt.Errorf("prepare plugin directory: %w", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	if err := copyPluginDir(sourceDir, target); err != nil {
		return nil, fmt.Errorf("copy plugin payload: %w", err)
	}

	// Optional MCP config: validate, persist, and keep in the returned Plugin.
	var mcpServers map[string]MCPServer
	if mcpData, err := os.ReadFile(filepath.Join(sourceDir, "mcp.json")); err == nil {
		mcpServers, err = parseMCPConfig(mcpData)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(target, "mcp.json"), mcpData, 0644); err != nil {
			return nil, fmt.Errorf("persist mcp.json: %w", err)
		}
	}

	// Install every bundled standard skill through the regular pipeline so
	// they register in the Manager and land in coscaDir/skills.
	skillsPath := m.skillsPathOrEmpty()
	var installed []*Skill
	var skillNames []string
	skillsSrc := filepath.Join(sourceDir, "skills")
	if entries, err := os.ReadDir(skillsSrc); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillName := e.Name()
			if validateSkillName(skillName) != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(skillsSrc, skillName, "SKILL.md")); err != nil {
				continue
			}
			skill, err := m.installStandardSkill(filepath.Join(skillsSrc, skillName), skillName, skillsPath)
			if err != nil {
				return nil, fmt.Errorf("install plugin skill %q: %w", skillName, err)
			}
			installed = append(installed, skill)
			skillNames = append(skillNames, skillName)
		}
	}

	plugin := &Plugin{
		Name:        name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Repository:  manifest.Repository,
		License:     manifest.License,
		Skills:      installed,
		MCPServers:  mcpServers,
		Source:      sourceLabel,
		Dir:         target,
		SkillNames:  skillNames,
	}

	m.mu.Lock()
	if m.plugins == nil {
		m.plugins = make(map[string]*Plugin)
	}
	m.plugins[name] = plugin
	m.mu.Unlock()

	return plugin, nil
}

// loadPluginsFromDir scans coscaDir/plugins for installed plugins and populates
// the in-memory registry so list/info/remove work across Manager instances
// (mirrors loadFromDir for skills). Manifests and mcp.json are read from disk;
// bundled skill names come from the persisted skills/ payload, with each skill
// matched against the already-loaded skill registry when present.
func (m *Manager) loadPluginsFromDir(dir string) {
	pluginsDir := filepath.Join(dir, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plugins == nil {
		m.plugins = make(map[string]*Plugin)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if validateSkillName(name) != nil {
			continue
		}
		pluginDir := filepath.Join(pluginsDir, name)
		manifestPath := findPluginManifest(pluginDir)
		if manifestPath == "" {
			continue
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		manifest, err := parsePluginManifest(data)
		if err != nil {
			continue
		}
		plugin := &Plugin{
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Repository:  manifest.Repository,
			License:     manifest.License,
			Dir:         pluginDir,
		}
		if mcpData, err := os.ReadFile(filepath.Join(pluginDir, "mcp.json")); err == nil {
			if servers, err := parseMCPConfig(mcpData); err == nil {
				plugin.MCPServers = servers
			}
		}
		if skillEntries, err := os.ReadDir(filepath.Join(pluginDir, "skills")); err == nil {
			for _, se := range skillEntries {
				if !se.IsDir() {
					continue
				}
				skillName := se.Name()
				if validateSkillName(skillName) != nil {
					continue
				}
				plugin.SkillNames = append(plugin.SkillNames, skillName)
				if s, ok := m.skills[skillName]; ok {
					plugin.Skills = append(plugin.Skills, s)
				}
			}
		}
		m.plugins[plugin.Name] = plugin
	}
}

// PluginDir returns the manager's plugins directory, creating it when needed.
// It mirrors skillsPathOrEmpty: "" when the manager has no coscaDir.
func (m *Manager) PluginDir() string {
	if m.coscaDir == "" {
		return ""
	}
	pluginsPath := filepath.Join(m.coscaDir, "plugins")
	if err := os.MkdirAll(pluginsPath, 0755); err != nil {
		return ""
	}
	return pluginsPath
}

// ListPlugins returns all installed plugins, sorted by name.
func (m *Manager) ListPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// GetPlugin returns a plugin by name (exact match, then case-insensitive).
func (m *Manager) GetPlugin(name string) (*Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.plugins[name]; ok {
		return p, nil
	}
	lower := strings.ToLower(name)
	for _, p := range m.plugins {
		if strings.ToLower(p.Name) == lower {
			return p, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found", name)
}

// RemovePlugin removes an installed plugin: its coscaDir/plugins/<name>/
// directory (containment-checked), its in-memory registration, and every skill
// that was installed by the plugin (tracked via SkillNames at install time).
func (m *Manager) RemovePlugin(name string) error {
	p, err := m.GetPlugin(name)
	if err != nil {
		return err
	}
	// Remove the plugin's skills first. Remove handles deletion of both the
	// in-memory entry and the coscaDir/skills files.
	for _, skillName := range p.SkillNames {
		if err := m.Remove(skillName); err != nil {
			return fmt.Errorf("remove plugin skill %q: %w", skillName, err)
		}
	}
	// Remove the plugin directory. p.Dir was written through resolveWithinRoot
	// at install; verify containment again before deleting.
	if m.coscaDir != "" && p.Dir != "" {
		pluginsPath := filepath.Join(m.coscaDir, "plugins")
		resolved, err := resolveWithinRoot(pluginsPath, p.Dir)
		if err == nil {
			if err := os.RemoveAll(resolved); err != nil {
				return fmt.Errorf("remove plugin directory: %w", err)
			}
		}
	}
	m.mu.Lock()
	delete(m.plugins, p.Name)
	m.mu.Unlock()
	return nil
}

// isSkippedPluginPath reports whether a relative plugin payload path should be
// skipped when persisting: repo scaffolding (.git, .github), README docs,
// and test/evals directories are not part of a plugin's runtime payload.
func isSkippedPluginPath(rel string, isDir bool) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		switch {
		case strings.EqualFold(seg, ".git"),
			strings.EqualFold(seg, ".github"),
			strings.EqualFold(seg, "evals"),
			strings.EqualFold(seg, "test"),
			strings.EqualFold(seg, "tests"),
			strings.EqualFold(seg, "__tests__"),
			strings.EqualFold(seg, "testdata"):
			return true
		}
	}
	if !isDir {
		lower := strings.ToLower(filepath.Base(rel))
		if strings.HasPrefix(lower, "readme") && strings.HasSuffix(lower, ".md") {
			return true
		}
	}
	return false
}

// copyPluginDir copies a plugin tree into dst, mirroring copyDir's symlink
// rejection while skipping scaffolding/test entries that are not part of the
// plugin's runtime payload.
func copyPluginDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in plugin directories: %s", p)
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel != "." && isSkippedPluginPath(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
