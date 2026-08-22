package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePluginManifest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		data    string
		want    string // expected plugin name on success
		wantErr bool
	}{
		{
			name: "valid",
			data: `{
				"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
				"name": "alloydb",
				"version": "0.2.0",
				"description": "Create and interact with AlloyDB databases",
				"author": {"name": "Google LLC", "email": "noreply@google.com"},
				"homepage": "https://example.com",
				"repository": "https://github.com/gemini-cli-extensions/alloydb",
				"license": "Apache-2.0",
				"keywords": ["alloydb", "postgresql"]
			}`,
			want: "alloydb",
		},
		{
			name: "missing optional fields",
			data: `{"name": "minimal"}`,
			want: "minimal",
		},
		{
			name:    "missing name",
			data:    `{"version": "1.0.0"}`,
			wantErr: true,
		},
		{
			name:    "non-slug name",
			data:    `{"name": "../evil"}`,
			wantErr: true,
		},
		{
			name:    "uppercase name",
			data:    `{"name": "AlloyDB"}`,
			wantErr: true,
		},
		{
			name:    "bad json",
			data:    `{not json`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		m, err := parsePluginManifest([]byte(c.data))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got nil", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if m.Name != c.want {
			t.Errorf("%s: Name = %q, want %q", c.name, m.Name, c.want)
		}
	}
}

func TestParseMCPConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "stdio valid",
			data: `{"mcpServers": {"toolbox": {"type": "stdio", "command": "toolbox", "args": ["--stdio"], "env": {"KEY": "VALUE"}}}}`,
		},
		{
			name: "streamable-http valid",
			data: `{"mcpServers": {"remote": {"type": "streamable-http", "url": "https://example.com/mcp", "headers": {"Authorization": "Bearer x"}}}}`,
		},
		{
			name: "sse valid",
			data: `{"mcpServers": {"events": {"type": "sse", "url": "https://example.com/sse"}}}`,
		},
		{
			name:    "stdio missing command",
			data:    `{"mcpServers": {"toolbox": {"type": "stdio"}}}`,
			wantErr: true,
		},
		{
			name:    "http missing url",
			data:    `{"mcpServers": {"remote": {"type": "streamable-http"}}}`,
			wantErr: true,
		},
		{
			name:    "bad type",
			data:    `{"mcpServers": {"x": {"type": "websocket", "url": "ws://..."}}}`,
			wantErr: true,
		},
		{
			name:    "empty mcpServers",
			data:    `{"mcpServers": {}}`,
			wantErr: true,
		},
		{
			name:    "bad json",
			data:    `{`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		servers, err := parseMCPConfig([]byte(c.data))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got nil", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if len(servers) == 0 {
			t.Errorf("%s: expected at least one server", c.name)
		}
	}
}

// writeTestPlugin scaffolds a plugin directory (plugin.json + skills/<name>/SKILL.md
// plus optional mcp.json) inside dir and returns its absolute path.
func writeTestPlugin(t *testing.T, dir, name, desc string, withMCP bool) string {
	t.Helper()
	root := filepath.Join(dir, name+"-plugin")
	skillsDir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	pluginJSON := fmt.Sprintf(`{
		"name": %q,
		"version": "1.2.3",
		"description": %q,
		"repository": "https://github.com/example/%s",
		"license": "Apache-2.0"
	}`, name, desc, name)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(pluginJSON), 0644); err != nil {
		t.Fatal(err)
	}
	skillMD := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\nBody.\n", name, desc)
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if withMCP {
		mcpJSON := `{"mcpServers": {"serve": {"type": "stdio", "command": "toolbox", "args": ["--stdio"]}}}`
		if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInstallPluginLocalDir(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "A greeting plugin", true)

	plugin, err := m.InstallPlugin(src, false)
	if err != nil {
		t.Fatalf("InstallPlugin error: %v", err)
	}
	if plugin.Name != "greet" {
		t.Errorf("Name = %q, want greet", plugin.Name)
	}
	if plugin.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", plugin.Version)
	}
	if len(plugin.Skills) != 1 {
		t.Fatalf("plugin skills = %d, want 1", len(plugin.Skills))
	}
	if len(plugin.MCPServers) != 1 {
		t.Errorf("plugin MCP servers = %d, want 1", len(plugin.MCPServers))
	}
	if srv, ok := plugin.MCPServers["serve"]; !ok || srv.Command != "toolbox" {
		t.Errorf("MCP server 'serve' = %+v, want stdio toolbox", srv)
	}

	// Plugin registered in the manager.
	got, err := m.GetPlugin("greet")
	if err != nil {
		t.Fatalf("GetPlugin error: %v", err)
	}
	if got.Name != "greet" {
		t.Errorf("GetPlugin Name = %q", got.Name)
	}

	// Bundled skill registered in the skill manager.
	skill, err := m.Get("greet")
	if err != nil {
		t.Fatalf("bundled skill not registered: %v", err)
	}
	if skill.Standard != true {
		t.Error("bundled skill should use the standard layout")
	}

	// Plugin dir persisted, scaffold/test files skipped.
	persisted := filepath.Join(coscaDir, "plugins", "greet")
	if _, err := os.Stat(filepath.Join(persisted, "plugin.json")); err != nil {
		t.Errorf("plugin.json not persisted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(persisted, "mcp.json")); err != nil {
		t.Errorf("mcp.json not persisted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(persisted, "skills", "greet", "SKILL.md")); err != nil {
		t.Errorf("plugin skills/ not persisted: %v", err)
	}
	// Skill persisted in coscaDir/skills.
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "greet", "SKILL.md")); err != nil {
		t.Errorf("bundled skill not persisted: %v", err)
	}
}

func TestInstallPluginSkipsScaffolding(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "greets", false)
	os.WriteFile(filepath.Join(src, "README.md"), []byte("docs"), 0644)
	os.WriteFile(filepath.Join(src, "README_zh.md"), []byte("docs"), 0644)
	os.MkdirAll(filepath.Join(src, ".git"), 0755)
	os.MkdirAll(filepath.Join(src, ".github"), 0755)
	os.MkdirAll(filepath.Join(src, "evals"), 0755)
	os.MkdirAll(filepath.Join(src, "test"), 0755)

	if _, err := m.InstallPlugin(src, false); err != nil {
		t.Fatalf("InstallPlugin error: %v", err)
	}

	persisted := filepath.Join(coscaDir, "plugins", "greet")
	for _, rel := range []string{"README.md", "README_zh.md", ".git", ".github", "evals", "test"} {
		if _, err := os.Stat(filepath.Join(persisted, rel)); !os.IsNotExist(err) {
			t.Errorf("scaffolding %q should not be persisted, stat err = %v", rel, err)
		}
	}
}

func TestInstallPluginRejectsSymlink(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "greets", false)
	if err := os.Symlink("/etc/passwd", filepath.Join(src, "skills", "greet", "escaped")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := m.InstallPlugin(src, false)
	if err == nil {
		t.Fatal("expected error for symlink in plugin tree, got nil")
	}
	if !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Errorf("error should mention symlinks, got: %v", err)
	}
}

func TestInstallPluginRejectsTraversalName(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "greets", false)
	// Poison the manifest name so it would escape the plugins directory.
	poisoned := `{"name": "../evil", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(poisoned), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := m.InstallPlugin(src, false)
	if err == nil {
		t.Fatal("expected error for traversal name, got nil")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "plugins", "evil")); !os.IsNotExist(err) {
		t.Errorf("traversal plugin must not be created")
	}
}

// TestInstallPluginClaudePluginLayout covers the real google plugin layout
// where plugin.json lives in .claude-plugin/ (or .codex-plugin/) and the
// skills/ payload sits at the plugin root.
func TestInstallPluginClaudePluginLayout(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "greet"), 0755); err != nil {
		t.Fatal(err)
	}
	pluginJSON := `{"name": "greet", "version": "1.2.3", "description": "claude-plugin layout"}`
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(pluginJSON), 0644); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: greet\ndescription: greets\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, "skills", "greet", "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	plugin, err := m.InstallPlugin(root, false)
	if err != nil {
		t.Fatalf("InstallPlugin error: %v", err)
	}
	if plugin.Name != "greet" {
		t.Errorf("Name = %q, want greet", plugin.Name)
	}
	if len(plugin.Skills) != 1 {
		t.Fatalf("plugin skills = %d, want 1", len(plugin.Skills))
	}
	// Skills payload persisted in both coscaDir/plugins and coscaDir/skills.
	if _, err := os.Stat(filepath.Join(coscaDir, "plugins", "greet", "skills", "greet", "SKILL.md")); err != nil {
		t.Errorf("plugin skills/ not persisted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "greet", "SKILL.md")); err != nil {
		t.Errorf("bundled skill not persisted: %v", err)
	}
}

func TestInstallPluginLocalNoPluginJSON(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, err := m.InstallPlugin(t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error for directory without plugin.json, got nil")
	}
}

func TestInstallPluginRemoteFailClosed(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, err := m.InstallPlugin("gemini-cli-extensions/alloydb", false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

func TestListPluginsSortedAndGetCaseInsensitive(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}

	srcA := writeTestPlugin(t, t.TempDir(), "beta", "beta plugin", false)
	srcC := writeTestPlugin(t, t.TempDir(), "alpha", "alpha plugin", false)
	for _, src := range []string{srcA, srcC} {
		if _, err := m.InstallPlugin(src, false); err != nil {
			t.Fatalf("InstallPlugin error: %v", err)
		}
	}

	plugins := m.ListPlugins()
	if len(plugins) != 2 {
		t.Fatalf("ListPlugins = %d, want 2", len(plugins))
	}
	if plugins[0].Name != "alpha" || plugins[1].Name != "beta" {
		t.Errorf("plugins not sorted by name: %v", []string{plugins[0].Name, plugins[1].Name})
	}

	if _, err := m.GetPlugin("ALPHA"); err != nil {
		t.Errorf("GetPlugin case-insensitive error: %v", err)
	}
	if _, err := m.GetPlugin("nope"); err == nil {
		t.Error("GetPlugin should error for unknown plugin")
	}
}

// TestPluginReloadFromDisk simulates a fresh process: a new Manager over the
// same coscaDir must rediscover installed plugins (and their skills) from
// disk, mirroring loadFromDir for skills.
func TestPluginReloadFromDisk(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "greets", true)
	if _, err := m.InstallPlugin(src, false); err != nil {
		t.Fatalf("InstallPlugin error: %v", err)
	}

	// New manager instance — must reload the plugin from coscaDir/plugins.
	reloaded := NewManager(coscaDir)
	plugin, err := reloaded.GetPlugin("greet")
	if err != nil {
		t.Fatalf("reloaded plugin not found: %v", err)
	}
	if plugin.Version != "1.2.3" {
		t.Errorf("reloaded Version = %q, want 1.2.3", plugin.Version)
	}
	if len(plugin.Skills) != 1 {
		t.Errorf("reloaded Skills = %d, want 1", len(plugin.Skills))
	}
	if len(plugin.MCPServers) != 1 {
		t.Errorf("reloaded MCPServers = %d, want 1", len(plugin.MCPServers))
	}
	// The bundled skill is loaded from coscaDir/skills too.
	if _, err := reloaded.Get("greet"); err != nil {
		t.Errorf("reloaded bundled skill not found: %v", err)
	}
	// Remove through the reloaded manager works end to end.
	if err := reloaded.RemovePlugin("greet"); err != nil {
		t.Fatalf("reloaded RemovePlugin error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "plugins", "greet")); !os.IsNotExist(err) {
		t.Errorf("plugin directory should be removed")
	}
}

func TestRemovePlugin(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	src := writeTestPlugin(t, t.TempDir(), "greet", "greets", true)
	if _, err := m.InstallPlugin(src, false); err != nil {
		t.Fatalf("InstallPlugin error: %v", err)
	}
	if _, err := m.Get("greet"); err != nil {
		t.Fatalf("skill should exist after install: %v", err)
	}

	if err := m.RemovePlugin("greet"); err != nil {
		t.Fatalf("RemovePlugin error: %v", err)
	}
	if _, err := m.GetPlugin("greet"); err == nil {
		t.Error("plugin should be gone after remove")
	}
	if _, err := m.Get("greet"); err == nil {
		t.Error("bundled skill should be unregistered after remove")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "plugins", "greet")); !os.IsNotExist(err) {
		t.Errorf("plugin directory should be removed from disk")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "greet")); !os.IsNotExist(err) {
		t.Errorf("bundled skill directory should be removed from disk")
	}

	// Removing a nonexistent plugin errors.
	if err := m.RemovePlugin("nope"); err == nil {
		t.Error("expected error removing nonexistent plugin")
	}
}
