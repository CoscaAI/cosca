package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// ─── Helpers ────────────────────────────────────────────────────────────────────

func writeAgentFile(t *testing.T, dir, filename, frontmatter, body string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	content := "---\n" + frontmatter + "\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ─── Tests ──────────────────────────────────────────────────────────────────────

func TestNewAgentRegistry(t *testing.T) {
	t.Run("loads agents from multiple directories", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()

		writeAgentFile(t, dir1, "agent-a.md",
			"name: cosca-test-a\ndescription: Agent A\ncapabilities: [test, a]",
			"System prompt A")

		writeAgentFile(t, dir2, "agent-b.md",
			"name: cosca-test-b\ndescription: Agent B\ncapabilities: [test, b]",
			"System prompt B")

		r := NewAgentRegistry(dir1, dir2)
		if got := r.Get("cosca-test-a"); got == nil {
			t.Error("expected agent cosca-test-a to be loaded")
		}
		if got := r.Get("cosca-test-b"); got == nil {
			t.Error("expected agent cosca-test-b to be loaded")
		}
		if got := len(r.List()); got != 2 {
			t.Errorf("expected 2 agents, got %d", got)
		}
	})

	t.Run("handles non-existent directory gracefully", func(t *testing.T) {
		r := NewAgentRegistry("/tmp/nonexistent-" + strings.ReplaceAll(t.Name(), "/", "_"))
		if got := len(r.List()); got != 0 {
			t.Errorf("expected 0 agents for missing dir, got %d", got)
		}
	})

	t.Run("loads from subdirectories recursively", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "subgroup")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		writeAgentFile(t, subdir, "agent-c.md",
			"name: cosca-test-c\ncapabilities: [test]",
			"System prompt C")

		r := NewAgentRegistry(dir)
		if got := r.Get("cosca-test-c"); got == nil {
			t.Error("expected agent from subdirectory to be loaded")
		}
	})

	t.Run("empty dirs list loads no agents", func(t *testing.T) {
		r := NewAgentRegistry()
		if got := len(r.List()); got != 0 {
			t.Errorf("expected 0 agents, got %d", got)
		}
	})
}

func TestLoadDirectory(t *testing.T) {
	t.Run("parses YAML frontmatter correctly", func(t *testing.T) {
		dir := t.TempDir()
		fm := "name: cosca-arch\n" +
			"display_name: Architecture Chief\n" +
			"description: The chief architect\n" +
			"capabilities: [architecture, design, patterns]\n" +
			"parent: cosca-cto\n" +
			"temperature: 0.3"
		body := "You are the Architecture Chief.\nYou design systems."
		writeAgentFile(t, dir, "arch.md", fm, body)

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-arch")
		if def == nil {
			t.Fatal("expected agent to be loaded")
		}
		if def.DisplayName != "Architecture Chief" {
			t.Errorf("DisplayName = %q, want %q", def.DisplayName, "Architecture Chief")
		}
		if def.Description != "The chief architect" {
			t.Errorf("Description = %q, want %q", def.Description, "The chief architect")
		}
		if len(def.Capabilities) != 3 || def.Capabilities[0] != "architecture" {
			t.Errorf("Capabilities = %v, want [architecture design patterns]", def.Capabilities)
		}
		if def.Parent != "cosca-cto" {
			t.Errorf("Parent = %q, want %q", def.Parent, "cosca-cto")
		}
		if def.Temperature != 0.3 {
			t.Errorf("Temperature = %v, want 0.3", def.Temperature)
		}
		if def.SystemPrompt != body {
			t.Errorf("SystemPrompt = %q, want %q", def.SystemPrompt, body)
		}
	})

	t.Run("supports legacy agent key", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "legacy.md",
			"agent: cosca-legacy\ntype: chief",
			"Legacy prompt")

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-legacy")
		if def == nil {
			t.Fatal("expected agent cosca-legacy to be loaded")
		}
		if def.SystemPrompt != "Legacy prompt" {
			t.Errorf("SystemPrompt = %q", "Legacy prompt")
		}
	})

	t.Run("skips file without frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "plain.md"), []byte("just some text"), 0644)
		if err != nil {
			t.Fatal(err)
		}
		// Should not panic, should not load any agent from this file.
		r := NewAgentRegistry(dir)
		if got := len(r.List()); got != 0 {
			t.Errorf("expected 0 agents, got %d", got)
		}
	})

	t.Run("skips non-md files", func(t *testing.T) {
		dir := t.TempDir()
		fm := "name: cosca-test"
		writeAgentFile(t, dir, "test.yaml", fm, "body")
		writeAgentFile(t, dir, "test.txt", fm, "body")

		r := NewAgentRegistry(dir)
		if got := len(r.List()); got != 0 {
			t.Errorf("expected 0 agents from non-md files, got %d", got)
		}
	})

	t.Run("error on non-directory path", func(t *testing.T) {
		dir := t.TempDir()
		f, err := os.CreateTemp(dir, "file")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		r := NewAgentRegistry()
		err = r.LoadDirectory(f.Name(), "test")
		if err == nil {
			t.Error("expected error for non-directory path")
		}
	})
}

func TestLoadDefault(t *testing.T) {
	t.Run("returns error when no agents loaded", func(t *testing.T) {
		r := NewAgentRegistry()
		// No directories exist, should return error
		err := r.LoadDefault()
		if err == nil {
			t.Error("expected error when no agents loaded")
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("returns agent by name", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md", "name: cosca-test", "body")
		r := NewAgentRegistry(dir)

		def := r.Get("cosca-test")
		if def == nil {
			t.Fatal("expected agent to be found")
		}
		if def.Name != "cosca-test" {
			t.Errorf("Name = %q, want %q", def.Name, "cosca-test")
		}
	})

	t.Run("returns nil for unknown agent", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md", "name: cosca-test", "body")
		r := NewAgentRegistry(dir)

		def := r.Get("nonexistent")
		if def != nil {
			t.Error("expected nil for unknown agent")
		}
	})

	t.Run("case-sensitive lookup", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md", "name: Cosca-Test", "body")
		r := NewAgentRegistry(dir)

		if got := r.Get("cosca-test"); got != nil {
			t.Error("lookup should be case-sensitive")
		}
		if got := r.Get("Cosca-Test"); got == nil {
			t.Error("expected exact case match to succeed")
		}
	})
}

func TestFindByCapability(t *testing.T) {
	t.Run("returns matching agents", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "db.md",
			"name: cosca-database\ncapabilities: [database, sql, data]",
			"Database agent")
		writeAgentFile(t, dir, "arch.md",
			"name: cosca-architecture\ncapabilities: [architecture, design, patterns]",
			"Architecture agent")
		writeAgentFile(t, dir, "both.md",
			"name: cosca-backend\ncapabilities: [database, api, backend]",
			"Backend agent")

		r := NewAgentRegistry(dir)

		matches := r.FindByCapability("database")
		if len(matches) != 2 {
			t.Errorf("expected 2 agents with 'database' capability, got %d", len(matches))
		}

		matches = r.FindByCapability("architecture")
		if len(matches) != 1 {
			t.Errorf("expected 1 agent with 'architecture' capability, got %d", len(matches))
		}
		if matches[0].Name != "cosca-architecture" {
			t.Errorf("got %q, want %q", matches[0].Name, "cosca-architecture")
		}
	})

	t.Run("case-insensitive matching", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md",
			"name: cosca-test\ncapabilities: [Database, SQL]",
			"Test agent")

		r := NewAgentRegistry(dir)

		matches := r.FindByCapability("DATABASE")
		if len(matches) != 1 {
			t.Errorf("expected 1 agent, got %d", len(matches))
		}
		matches = r.FindByCapability("sql")
		if len(matches) != 1 {
			t.Errorf("expected 1 agent, got %d", len(matches))
		}
	})

	t.Run("returns empty for no match", func(t *testing.T) {
		r := NewAgentRegistry()

		matches := r.FindByCapability("nonexistent")
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns all loaded agents", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "a.md", "name: agent-a", "A")
		writeAgentFile(t, dir, "b.md", "name: agent-b", "B")
		writeAgentFile(t, dir, "c.md", "name: agent-c", "C")

		r := NewAgentRegistry(dir)
		list := r.List()
		if len(list) != 3 {
			t.Errorf("expected 3 agents, got %d", len(list))
		}

		names := make([]string, len(list))
		for i, def := range list {
			names[i] = def.Name
		}
		sort.Strings(names)
		expected := []string{"agent-a", "agent-b", "agent-c"}
		for i, n := range names {
			if n != expected[i] {
				t.Errorf("list[%d] = %q, want %q", i, n, expected[i])
			}
		}
	})

	t.Run("returns empty slice when no agents", func(t *testing.T) {
		r := NewAgentRegistry()
		list := r.List()
		if list == nil {
			t.Error("expected non-nil empty slice")
		}
		if len(list) != 0 {
			t.Errorf("expected 0, got %d", len(list))
		}
	})
}

func TestOverlaySemantics(t *testing.T) {
	t.Run("last definition wins for same agent name", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()

		writeAgentFile(t, dir1, "overlay.md",
			"name: cosca-overlay\ndescription: first version",
			"First prompt body")

		writeAgentFile(t, dir2, "overlay.md",
			"name: cosca-overlay\ndescription: second version",
			"Second prompt body")

		r := NewAgentRegistry(dir1, dir2)
		def := r.Get("cosca-overlay")
		if def == nil {
			t.Fatal("expected agent to be loaded")
		}
		if def.Description != "second version" {
			t.Errorf("Description = %q, want %q (last should win)", def.Description, "second version")
		}
		if def.SystemPrompt != "Second prompt body" {
			t.Errorf("SystemPrompt = %q, want %q (last should win)", def.SystemPrompt, "Second prompt body")
		}
	})
}

func TestRegistryConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	// Load 5 agents.
	names := []string{"agent-1", "agent-2", "agent-3", "agent-4", "agent-5"}
	for _, n := range names {
		writeAgentFile(t, dir, n+".md", "name: "+n, "prompt "+n)
	}

	r := NewAgentRegistry(dir)

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := names[idx%len(names)]
			def := r.Get(name)
			if def == nil {
				t.Errorf("concurrent Get(%q) returned nil", name)
			}
			_ = r.FindByCapability("nonexistent")
			_ = r.List()
		}(i)
	}
	wg.Wait()
}

func TestFrontmatterParsing(t *testing.T) {
	t.Run("with capabilities", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md",
			"name: cosca-test\ncapabilities: [a, b, c]",
			"body")

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-test")
		if def == nil {
			t.Fatal("expected agent")
		}
		if len(def.Capabilities) != 3 {
			t.Errorf("expected 3 capabilities, got %v", def.Capabilities)
		}
	})

	t.Run("without capabilities", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test.md",
			"name: cosca-test\ndescription: just a test",
			"body")

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-test")
		if def == nil {
			t.Fatal("expected agent")
		}
		if def.Capabilities != nil {
			t.Errorf("expected nil capabilities, got %v", def.Capabilities)
		}
	})

	t.Run("name key takes precedence over agent key", func(t *testing.T) {
		dir := t.TempDir()
		// When both "name" and "agent" are present, "name" wins.
		writeAgentFile(t, dir, "test.md",
			"name: cosca-primary\nagent: cosca-secondary\ncapabilities: [test]",
			"body")

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-primary")
		if def == nil {
			t.Fatal("expected 'cosca-primary' to be loaded")
		}
		def2 := r.Get("cosca-secondary")
		if def2 != nil {
			t.Error("'cosca-secondary' should NOT be loaded since 'name' takes precedence")
		}
	})
}

func TestFrontmatterEdgeCases(t *testing.T) {
	t.Run("file with no closing frontmatter delimiter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "broken.md")
		content := "---\nname: broken\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		r := NewAgentRegistry(dir)
		if got := r.Get("broken"); got != nil {
			t.Error("should not load agent from unclosed frontmatter")
		}
	})

	t.Run("file with no name in frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "noname.md",
			"description: no name here",
			"body")
		r := NewAgentRegistry(dir)
		if got := len(r.List()); got != 0 {
			t.Errorf("expected 0 agents, got %d", got)
		}
	})

	t.Run("file with YAML parsing error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad-yaml.md")
		content := "---\nname: test\ninvalid: [\n---\nbody"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		r := NewAgentRegistry(dir)
		if got := r.Get("test"); got != nil {
			t.Error("should not load agent with invalid YAML")
		}
	})
}

func TestAgentDefFields(t *testing.T) {
	t.Run("file path is set correctly", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test-agent.md",
			"name: cosca-test", "Some prompt")

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-test")
		if def == nil {
			t.Fatal("expected agent")
		}
		if def.FilePath == "" {
			t.Error("FilePath should not be empty")
		}
		if !strings.HasSuffix(def.FilePath, "test-agent.md") {
			t.Errorf("FilePath = %q, should end with 'test-agent.md'", def.FilePath)
		}
	})

	t.Run("empty system prompt", func(t *testing.T) {
		dir := t.TempDir()
		p := writeAgentFile(t, dir, "empty.md",
			"name: cosca-empty",
			"")

		// Manually parse to check edge case
		r := NewAgentRegistry()
		err := r.parseAgentFile(p, "test")
		if err != nil {
			t.Fatalf("parseAgentFile failed: %v", err)
		}
		def := r.Get("cosca-empty")
		if def == nil {
			t.Fatal("expected agent")
		}
		if def.SystemPrompt != "" {
			t.Errorf("expected empty system prompt, got %q", def.SystemPrompt)
		}
	})
}

// Helper to access unexported parseAgentFile for direct testing.
func TestParseAgentFile(t *testing.T) {
	t.Run("parses successfully with all fields", func(t *testing.T) {
		dir := t.TempDir()
		fm := map[string]interface{}{
			"name":         "cosca-full",
			"display_name": "Full Agent",
			"description":  "A full test agent",
			"capabilities": []string{"full", "test"},
			"parent":       "cosca-cto",
			"temperature":  0.5,
		}
		fmBytes, _ := yaml.Marshal(fm)
		body := "You are a full test agent."
		writeAgentFile(t, dir, "full.md", string(fmBytes), body)

		r := NewAgentRegistry(dir)
		def := r.Get("cosca-full")
		if def == nil {
			t.Fatal("expected agent")
		}
		if def.DisplayName != "Full Agent" {
			t.Errorf("DisplayName = %q", def.DisplayName)
		}
		if def.Description != "A full test agent" {
			t.Errorf("Description = %q", def.Description)
		}
		if def.Temperature != 0.5 {
			t.Errorf("Temperature = %v", def.Temperature)
		}
		if def.Parent != "cosca-cto" {
			t.Errorf("Parent = %q", def.Parent)
		}
	})
}
