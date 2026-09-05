package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// googleSkillsMarketplaceJSON mirrors the real google/skills
// .claude-plugin/marketplace.json shape.
const googleSkillsMarketplaceJSON = `{
	"name": "google-plugins",
	"owner": {"name": "Google LLC"},
	"metadata": {"version": "0.0.1", "description": "Google's collection of Agent Plugins."},
	"plugins": [
		{
			"name": "alloydb",
			"source": {"source": "github", "repo": "gemini-cli-extensions/alloydb", "ref": "0.2.0"},
			"description": "Create, connect, and interact with an AlloyDB for PostgreSQL database and data."
		},
		{
			"name": "db-context-engineering",
			"source": {"source": "git-subdir", "url": "GoogleCloudPlatform/db-context-enrichment", "path": "plugin", "ref": "v0.6.0"},
			"description": "Inspect a database and generate useful context about its schema."
		}
	]
}`

func TestParseMarketplace(t *testing.T) {
	t.Parallel()

	mp, err := parseMarketplace([]byte(googleSkillsMarketplaceJSON))
	if err != nil {
		t.Fatalf("parseMarketplace error: %v", err)
	}
	if mp.Name != "google-plugins" {
		t.Errorf("Name = %q, want google-plugins", mp.Name)
	}
	if mp.Description != "Google's collection of Agent Plugins." {
		t.Errorf("Description = %q", mp.Description)
	}
	if len(mp.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(mp.Plugins))
	}
	if mp.Plugins[0].Name != "alloydb" {
		t.Errorf("plugin[0].Name = %q", mp.Plugins[0].Name)
	}
	if mp.Plugins[0].Source.Source != "github" || mp.Plugins[0].Source.Repo != "gemini-cli-extensions/alloydb" || mp.Plugins[0].Source.Ref != "0.2.0" {
		t.Errorf("plugin[0].Source = %+v", mp.Plugins[0].Source)
	}
	if mp.Plugins[1].Source.Source != "git-subdir" || mp.Plugins[1].Source.URL != "GoogleCloudPlatform/db-context-enrichment" || mp.Plugins[1].Source.Path != "plugin" || mp.Plugins[1].Source.Ref != "v0.6.0" {
		t.Errorf("plugin[1].Source = %+v", mp.Plugins[1].Source)
	}
}

func TestParseMarketplaceMissingPluginsTolerated(t *testing.T) {
	t.Parallel()

	mp, err := parseMarketplace([]byte(`{"name": "empty-market"}`))
	if err != nil {
		t.Fatalf("parseMarketplace error: %v", err)
	}
	if mp.Name != "empty-market" {
		t.Errorf("Name = %q", mp.Name)
	}
	if mp.Plugins == nil || len(mp.Plugins) != 0 {
		t.Errorf("plugins should be an empty list, got %v", mp.Plugins)
	}
}

func TestParseMarketplaceErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data string
	}{
		{name: "missing name", data: `{"plugins": []}`},
		{name: "bad json", data: `{not json`},
	}
	for _, c := range cases {
		if _, err := parseMarketplace([]byte(c.data)); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestLoadMarketplaceFromDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "marketplace.json"), []byte(googleSkillsMarketplaceJSON), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := loadMarketplaceFromDir(dir)
	if err != nil {
		t.Fatalf("loadMarketplaceFromDir error: %v", err)
	}
	if mp.Name != "google-plugins" {
		t.Errorf("Name = %q, want google-plugins", mp.Name)
	}
	if len(mp.Plugins) != 2 {
		t.Errorf("plugins = %d, want 2", len(mp.Plugins))
	}
}

func TestLoadMarketplaceFromDirLocationPrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(rel, name string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(map[string]any{"name": name, "plugins": []any{}})
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// All three locations present: .claude-plugin must win.
	write(".claude-plugin/marketplace.json", "claude-market")
	write(".codex-plugin/marketplace.json", "codex-market")
	write("marketplace.json", "root-market")

	mp, err := loadMarketplaceFromDir(dir)
	if err != nil {
		t.Fatalf("loadMarketplaceFromDir error: %v", err)
	}
	if mp.Name != "claude-market" {
		t.Errorf("Name = %q, want claude-market (precedence)", mp.Name)
	}

	// Drop .claude-plugin: .codex-plugin must win next.
	if err := os.RemoveAll(filepath.Join(dir, ".claude-plugin")); err != nil {
		t.Fatal(err)
	}
	mp, err = loadMarketplaceFromDir(dir)
	if err != nil {
		t.Fatalf("loadMarketplaceFromDir (codex) error: %v", err)
	}
	if mp.Name != "codex-market" {
		t.Errorf("Name = %q, want codex-market", mp.Name)
	}
}

func TestLoadMarketplaceFromDirNotFound(t *testing.T) {
	t.Parallel()

	if _, err := loadMarketplaceFromDir(t.TempDir()); err == nil {
		t.Fatal("expected error for directory without marketplace.json, got nil")
	}
}

func TestListMarketplaceRequiresAllowRemote(t *testing.T) {
	t.Parallel()

	m := &Manager{coscaDir: t.TempDir()}
	_, err := m.ListMarketplace("google/skills", false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

// TestInstallPluginFromMarketplace drives the full install path with a
// Marketplace constructed directly. The plugin source points at a LOCAL
// directory (via the git-subdir URL field) so the test needs no network —
// InstallPlugin prefers local directories over GitHub references.
func TestInstallPluginFromMarketplace(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	pluginDir := writeTestPlugin(t, t.TempDir(), "greet", "greets", true)

	mp := &Marketplace{
		Name: "test-market",
		Plugins: []MarketplacePlugin{
			{Name: "greet", Description: "greets", Source: MarketplaceSource{Source: "git-subdir", URL: pluginDir}},
		},
	}

	installed, warnings, err := m.installPluginFromMarketplace(mp, "greet", true)
	if err != nil {
		t.Fatalf("installPluginFromMarketplace error: %v", err)
	}
	if installed.Name != "greet" {
		t.Errorf("installed.Name = %q, want greet", installed.Name)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings for single install: %v", warnings)
	}
	// Plugin and its bundled skill registered.
	if _, err := m.GetPlugin("greet"); err != nil {
		t.Errorf("plugin not registered: %v", err)
	}
	if _, err := m.Get("greet"); err != nil {
		t.Errorf("bundled skill not registered: %v", err)
	}
}

func TestInstallPluginFromMarketplaceCaseInsensitive(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: t.TempDir()}
	pluginDir := writeTestPlugin(t, t.TempDir(), "greet", "greets", false)
	mp := &Marketplace{
		Name: "test-market",
		Plugins: []MarketplacePlugin{
			{Name: "greet", Source: MarketplaceSource{Source: "git-subdir", URL: pluginDir}},
		},
	}
	if _, _, err := m.installPluginFromMarketplace(mp, "GREET", true); err != nil {
		t.Fatalf("installPluginFromMarketplace (case-insensitive) error: %v", err)
	}
}

func TestInstallPluginFromMarketplaceAll(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	dirA := writeTestPlugin(t, t.TempDir(), "alpha", "alpha plugin", false)
	dirB := writeTestPlugin(t, t.TempDir(), "beta", "beta plugin", false)

	mp := &Marketplace{
		Name: "test-market",
		Plugins: []MarketplacePlugin{
			{Name: "alpha", Source: MarketplaceSource{Source: "git-subdir", URL: dirA}},
			{Name: "beta", Source: MarketplaceSource{Source: "git-subdir", URL: dirB}},
		},
	}

	first, warnings, err := m.installPluginFromMarketplace(mp, "all", true)
	if err != nil {
		t.Fatalf("installPluginFromMarketplace(all) error: %v", err)
	}
	if first.Name != "alpha" {
		t.Errorf("first.Name = %q, want alpha", first.Name)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "beta") {
		t.Errorf("warnings = %v, want a warning naming the second plugin", warnings)
	}
	for _, n := range []string{"alpha", "beta"} {
		if _, err := m.GetPlugin(n); err != nil {
			t.Errorf("plugin %q not registered: %v", n, err)
		}
	}
}

func TestInstallPluginFromMarketplaceNotFound(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	mp := &Marketplace{Name: "test-market"}
	if _, _, err := m.installPluginFromMarketplace(mp, "missing", true); err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
}

func TestInstallPluginFromMarketplaceUnsupportedSource(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	mp := &Marketplace{
		Name: "test-market",
		Plugins: []MarketplacePlugin{
			{Name: "weird", Source: MarketplaceSource{Source: "binary", Repo: "a/b"}},
		},
	}
	if _, _, err := m.installPluginFromMarketplace(mp, "weird", true); err == nil {
		t.Fatal("expected error for unsupported source type, got nil")
	}
}

func TestInstallPluginFromMarketplaceFailClosed(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	mp := &Marketplace{
		Name: "test-market",
		Plugins: []MarketplacePlugin{
			{Name: "remote", Source: MarketplaceSource{Source: "github", Repo: "gemini-cli-extensions/alloydb"}},
		},
	}
	// allowRemote=false must fail even with a valid marketplace (fail-closed).
	_, _, err := m.installPluginFromMarketplace(mp, "remote", false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

// TestInstallPluginFromMarketplaceWithLocalFile exercises the marketplace
// manifest loading via loadMarketplaceFromDir and then installs from it.
func TestInstallPluginFromMarketplaceWithLocalFile(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), plugins: make(map[string]*Plugin), coscaDir: coscaDir}
	pluginDir := writeTestPlugin(t, t.TempDir(), "greet", "greets", false)

	marketDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(marketDir, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	mpJSON := `{"name": "local-market", "plugins": [
		{"name": "greet", "source": {"source": "git-subdir", "url": ` + quote(pluginDir) + `}, "description": "greets"}
	]}`
	if err := os.WriteFile(filepath.Join(marketDir, ".claude-plugin", "marketplace.json"), []byte(mpJSON), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := loadMarketplaceFromDir(marketDir)
	if err != nil {
		t.Fatalf("loadMarketplaceFromDir error: %v", err)
	}
	installed, _, err := m.installPluginFromMarketplace(mp, "greet", true)
	if err != nil {
		t.Fatalf("installPluginFromMarketplace error: %v", err)
	}
	if installed.Name != "greet" {
		t.Errorf("installed.Name = %q, want greet", installed.Name)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
