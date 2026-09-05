package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontmatterParsing(t *testing.T) {
	skill := &Skill{}
	lines := []string{
		"name: test-skill",
		`description: "A skill with spaces"`,
		"version: 1.2.0",
		"category: coding",
		`license: MIT`,
		"allowed-tools: [read, write]",
		"metadata: {author: \"kernel\", priority: high}",
		"  verified: true",
	}
	parseFrontmatterLines(lines, skill)
	if skill.Name != "test-skill" {
		t.Fatalf("name = %q", skill.Name)
	}
	if skill.Description != "A skill with spaces" {
		t.Fatalf("desc = %q", skill.Description)
	}
	if skill.Version != "1.2.0" || skill.Category != "coding" {
		t.Fatalf("ver/cat = %q/%q", skill.Version, skill.Category)
	}
	if skill.Metadata["author"] != "kernel" || skill.Metadata["priority"] != "high" {
		t.Fatalf("metadata = %v", skill.Metadata)
	}
	if skill.Metadata["verified"] != "true" {
		t.Fatalf("metadata indented = %v", skill.Metadata)
	}
}

func TestFrontmatterHelpers(t *testing.T) {
	if got := frontmatterValue("name: x", "name:"); got != "x" {
		t.Fatalf("frontmatterValue = %q", got)
	}
	if got := frontmatterValue(`name: "x"`, "name:"); got != "x" {
		t.Fatalf("frontmatterValue quoted = %q", got)
	}
	if got := stripQuotes(`"hello"`); got != "hello" {
		t.Fatalf("stripQuotes = %q", got)
	}
	if got := stripQuotes("plain"); got != "plain" {
		t.Fatalf("stripQuotes plain = %q", got)
	}
	m := map[string]string{}
	parseInlineMap(`{a: "1", b: two}`, m)
	if m["a"] != "1" || m["b"] != "two" {
		t.Fatalf("parseInlineMap = %v", m)
	}
}

func TestSkillsDirAndTopLevel(t *testing.T) {
	// SkillsDir do UsageStore.
	us, err := NewUsageStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}
	if us.SkillsDir() == "" {
		t.Fatal("SkillsDir vazio")
	}
}

func TestTopLevelNamesAndPluginRoot(t *testing.T) {
	// topLevelNames: lista de primeiro nível (join por ", ").
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "a"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "b"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644)
	names := topLevelNames(dir)
	if names != "a, b, f.txt" {
		t.Fatalf("topLevelNames = %q", names)
	}
	if got := topLevelNames(filepath.Join(dir, "missing")); got != "<unreadable>" {
		t.Fatalf("topLevelNames(missing) = %q", got)
	}
	if got := topLevelNames(t.TempDir()); got != "<empty>" {
		t.Fatalf("topLevelNames(empty) = %q", got)
	}
	// locatePluginRoot: precisa de plugin.json no repo.
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a", "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"p","version":"1.0.0"}`), 0o644)
	if got, err := locatePluginRoot(root, "a/sub"); err != nil || !strings.Contains(got, "a") {
		t.Fatalf("locatePluginRoot = %q, %v", got, err)
	}
}

func TestSingleTopLevelDir(t *testing.T) {
	// Dir com 1 subdir → retorna o subdir.
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "only"), 0o755)
	got, err := singleTopLevelDir(dir)
	if err != nil || filepath.Base(got) != "only" {
		t.Fatalf("singleTopLevelDir = %q, %v", got, err)
	}
	// Dir vazio → erro.
	empty := t.TempDir()
	if _, err := singleTopLevelDir(empty); err == nil {
		t.Fatal("singleTopLevelDir(empty) deve dar erro")
	}
	// pathOrEmpty: "" → ""; senão "/"+p (o prefixo é adicionado).
	if got := pathOrEmpty(""); got != "" {
		t.Fatalf("pathOrEmpty(\"\") = %q", got)
	}
	if got := pathOrEmpty("plugins/x"); got != "/plugins/x" {
		t.Fatalf("pathOrEmpty = %q", got)
	}
	_ = strings.ToUpper
}
