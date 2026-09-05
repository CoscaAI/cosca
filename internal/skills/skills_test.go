package skills

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill)}
	if m.skills == nil {
		t.Error("skills map should be initialized")
	}
}

func TestAddAndList(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill)}
	m.Add(Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Version:     "1.0",
		Category:    "general",
	})

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Name != "test-skill" {
		t.Errorf("Name = %q", list[0].Name)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill)}
	m.Add(Skill{Name: "my-skill"})

	s, err := m.Get("my-skill")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("Name = %q", s.Name)
	}

	// Case insensitive
	s, err = m.Get("MY-SKILL")
	if err != nil {
		t.Fatalf("Get (case-insensitive) error: %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("Name = %q", s.Name)
	}

	// Not found
	_, err = m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill)}
	m.Add(Skill{Name: "code-review", Description: "Review code changes", Category: "review"})
	m.Add(Skill{Name: "deploy-app", Description: "Deploy application", Category: "deploy"})

	results, err := m.Search("review")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestInstall(t *testing.T) {
	t.Parallel()

	t.Run("empty source returns error", func(t *testing.T) {
		t.Parallel()
		m := &Manager{skills: make(map[string]*Skill)}
		_, err := m.Install("my-plugin", "")
		if err == nil {
			t.Fatal("expected error for empty source")
		}
	})

	t.Run("install from local file without coscaDir", func(t *testing.T) {
		t.Parallel()
		content := "# My Plugin\n\nDoes something useful.\n"
		tmpFile := filepath.Join(t.TempDir(), "my-plugin.md")
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		m := &Manager{skills: make(map[string]*Skill)}
		s, err := m.Install("my-plugin", tmpFile)
		if err != nil {
			t.Fatalf("Install error: %v", err)
		}
		if s.Name != "my-plugin" {
			t.Errorf("Name = %q, want %q", s.Name, "my-plugin")
		}
		if s.Description != "Does something useful." {
			t.Errorf("Description = %q, want %q", s.Description, "Does something useful.")
		}
	})

	t.Run("install with coscaDir persists file", func(t *testing.T) {
		t.Parallel()
		content := "# Persisted Skill\n\nDescription here.\n"

		coscaDir := t.TempDir()
		// Local sources must live inside the manager's skills directory
		// (containment hardening) — place the source file there.
		skillsDir := filepath.Join(coscaDir, "skills")
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			t.Fatal(err)
		}
		sourceFile := filepath.Join(skillsDir, "persisted-src.md")
		if err := os.WriteFile(sourceFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		m := NewManager(coscaDir)
		s, err := m.Install("persisted", sourceFile)
		if err != nil {
			t.Fatalf("Install error: %v", err)
		}
		if s.Name != "persisted" {
			t.Errorf("Name = %q, want %q", s.Name, "persisted")
		}

		// Verify file was persisted.
		persistedPath := filepath.Join(coscaDir, "skills", "persisted.md")
		data, err := os.ReadFile(persistedPath)
		if err != nil {
			t.Fatalf("persisted file not found: %v", err)
		}
		if string(data) != content {
			t.Errorf("persisted content mismatch")
		}

		// Verify the skill is in the manager's list.
		found := false
		for _, sk := range m.List() {
			if sk.Name == "persisted" {
				found = true
				break
			}
		}
		if !found {
			t.Error("installed skill not found in List()")
		}
	})

	t.Run("install from nonexistent file", func(t *testing.T) {
		t.Parallel()
		m := &Manager{skills: make(map[string]*Skill)}
		_, err := m.Install("ghost", "/tmp/nonexistent-skill-12345.md")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

// writeSkillSourceInRoot writes a source markdown file inside root/skills so
// the containment check passes, and returns its path.
func writeSkillSourceInRoot(t *testing.T, root, name, content string) string {
	t.Helper()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillsDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestInstall_RemoteSourceRejected verifies the SSRF fix: HTTP(S) sources
// are denied by default (fail-closed) with a clear error.
func TestInstall_RemoteSourceRejected(t *testing.T) {
	t.Parallel()

	m := NewManager(t.TempDir())
	// A classic SSRF target: cloud metadata endpoint / internal network.
	_, err := m.Install("remote-skill", "http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected error for remote http source")
	}
	if !strings.Contains(err.Error(), "remote skill sources are disabled") {
		t.Errorf("error = %q, want mention of remote sources disabled", err.Error())
	}

	_, err = m.Install("remote-skill-https", "https://example.com/skill.md")
	if err == nil {
		t.Fatal("expected error for remote https source")
	}
	if !strings.Contains(err.Error(), "remote skill sources are disabled") {
		t.Errorf("error = %q, want mention of remote sources disabled", err.Error())
	}
}

// TestInstall_RemoteSourceOptIn verifies the documented opt-in path: when
// AllowRemoteSources is explicitly set, an HTTP source is fetched with the
// hardened client instead of being denied.
func TestInstall_RemoteSourceOptIn(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# Opt-In Skill\n\nFetched remotely.\n"))
	}))
	defer srv.Close()

	coscaDir := t.TempDir()
	m := NewManager(coscaDir)
	m.AllowRemoteSources = true

	s, err := m.Install("opt-in-skill", srv.URL)
	if err != nil {
		t.Fatalf("Install with AllowRemoteSources error: %v", err)
	}
	if s.Name != "opt-in-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "opt-in-skill")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "opt-in-skill.md")); err != nil {
		t.Errorf("expected persisted skill file: %v", err)
	}
}

// TestInstall_NameTraversalRejected verifies the path-traversal-of-write
// fix: names containing "..", slashes, dots, or spaces are rejected before
// any filepath.Join, so nothing can be written outside the skills dir.
func TestInstall_NameTraversalRejected(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := NewManager(coscaDir)
	source := writeSkillSourceInRoot(t, coscaDir, "ok.md", "# Ok\n\nFine.\n")

	badNames := []string{
		"../../../tmp/pwn",
		"../escape",
		"a/b",
		"evil.md",
		"UPPER",
		"has space",
		"under_score",
		"",
	}
	for _, name := range badNames {
		_, err := m.Install(name, source)
		if err == nil {
			t.Errorf("expected error for name %q", name)
			continue
		}
		if !strings.Contains(err.Error(), "invalid skill name") && !strings.Contains(err.Error(), "required") {
			t.Errorf("name %q: error = %q, want slug validation message", name, err.Error())
		}
	}

	// Nothing may have been written outside the skills directory.
	escapePath := filepath.Join(coscaDir, "tmp", "pwn.md")
	if _, err := os.Stat(escapePath); err == nil {
		t.Error("traversal write escaped the skills directory")
	}
}

// TestInstall_LocalSourceWorks verifies the happy path: a local source file
// inside the skills directory installs and persists correctly.
func TestInstall_LocalSourceWorks(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	content := "# Local Skill\n\nInstalled from a local path.\n"
	source := writeSkillSourceInRoot(t, coscaDir, "local-src.md", content)

	m := NewManager(coscaDir)
	s, err := m.Install("local-skill", source)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if s.Name != "local-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "local-skill")
	}
	if s.Description != "Installed from a local path." {
		t.Errorf("Description = %q", s.Description)
	}

	persisted := filepath.Join(coscaDir, "skills", "local-skill.md")
	data, err := os.ReadFile(persisted)
	if err != nil {
		t.Fatalf("persisted file not found: %v", err)
	}
	if string(data) != content {
		t.Error("persisted content mismatch")
	}
}

// TestInstall_PathOutsideRejected verifies the arbitrary-file-read fix: a
// local source that resolves outside the skills directory (e.g. /etc/passwd,
// or a symlink escape) is rejected and nothing is persisted.
func TestInstall_PathOutsideRejected(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := NewManager(coscaDir)

	_, err := m.Install("evil", "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for source outside skills directory")
	}
	if !strings.Contains(err.Error(), "outside the skills directory") {
		t.Errorf("error = %q, want containment rejection message", err.Error())
	}

	// Nothing should have been persisted.
	if _, statErr := os.Stat(filepath.Join(coscaDir, "skills", "evil.md")); !os.IsNotExist(statErr) {
		t.Error("skill should not be persisted for a rejected source")
	}

	// Symlink escape: a symlink inside the skills dir pointing outside.
	skillsDir := filepath.Join(coscaDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(coscaDir, "outside-target.md")
	if err := os.WriteFile(outside, []byte("# Outside\n\nNope.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(skillsDir, "escape-link.md")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := m.Install("symlink-escape", filepath.Join(skillsDir, "escape-link.md")); err == nil {
		t.Fatal("expected error for symlink pointing outside skills directory")
	}
}

func TestParseSkillFromMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		filename string
		wantName string
	}{
		{
			name:     "simple heading",
			content:  "# My Skill\n\nDescription text",
			filename: "my-skill.md",
			wantName: "My Skill",
		},
		{
			name: "with frontmatter",
			content: `---
name: custom-name
description: A custom description
version: 2.0
category: test
---
# Content`,
			filename: "file.md",
			wantName: "custom-name",
		},
		{
			name:     "name from filename",
			content:  "Just some content",
			filename: "my_awesome_skill.md",
			wantName: "my awesome skill",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			skill := parseSkillFromMarkdown(tc.content, tc.filename)
			if skill == nil {
				t.Fatal("parseSkillFromMarkdown returned nil")
			}
			if skill.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", skill.Name, tc.wantName)
			}
		})
	}
}

func TestToolParsing(t *testing.T) {
	t.Parallel()

	content := `# My Skill
| my-tool | A useful tool |
`
	skill := parseSkillFromMarkdown(content, "skill.md")
	if len(skill.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(skill.Tools))
	}
	if skill.Tools[0].Name != "my-tool" {
		t.Errorf("Tool Name = %q", skill.Tools[0].Name)
	}
}

// =============================================================================
// Agent Skills standard support
// =============================================================================

func TestParseSkillFromMarkdown_AgentSkillsFrontmatter(t *testing.T) {
	t.Parallel()

	content := `---
name: code-review
description: Review code changes for quality, correctness, and security.
license: MIT
compatibility: linux amd64
metadata:
  subcategory: quality
  priority: high
allowed-tools: "Bash(git:*) Read"
---
# Code Review

Review the diff carefully.
`
	skill := parseSkillFromMarkdown(content, "code-review.md")
	if skill == nil {
		t.Fatal("nil skill")
	}
	if skill.Name != "code-review" {
		t.Errorf("Name = %q, want %q", skill.Name, "code-review")
	}
	if skill.License != "MIT" {
		t.Errorf("License = %q, want %q", skill.License, "MIT")
	}
	if skill.Compatibility != "linux amd64" {
		t.Errorf("Compatibility = %q, want %q", skill.Compatibility, "linux amd64")
	}
	if skill.AllowedTools != "Bash(git:*) Read" {
		t.Errorf("AllowedTools = %q, want %q", skill.AllowedTools, "Bash(git:*) Read")
	}
	if len(skill.Metadata) != 2 {
		t.Fatalf("Metadata = %v, want 2 entries", skill.Metadata)
	}
	if skill.Metadata["subcategory"] != "quality" {
		t.Errorf("Metadata[subcategory] = %q", skill.Metadata["subcategory"])
	}
	if skill.Metadata["priority"] != "high" {
		t.Errorf("Metadata[priority] = %q", skill.Metadata["priority"])
	}
	if skill.Instructions == "" {
		t.Error("Instructions should contain the body")
	}
	if !strings.Contains(skill.Instructions, "Review the diff") {
		t.Errorf("Instructions = %q, want body text", skill.Instructions)
	}
	if !skill.fromFrontmatter {
		t.Error("fromFrontmatter should be true")
	}
}

func TestParseSkillFromMarkdown_MetadataMap(t *testing.T) {
	t.Parallel()

	t.Run("block style", func(t *testing.T) {
		content := `---
name: meta-skill
description: Metadata parsing test.
metadata:
  owner: Platform Chief
  tier: gold
---
# Body
`
		skill := parseSkillFromMarkdown(content, "meta-skill.md")
		if len(skill.Metadata) != 2 {
			t.Fatalf("Metadata = %v, want 2 entries", skill.Metadata)
		}
		if skill.Metadata["owner"] != "Platform Chief" || skill.Metadata["tier"] != "gold" {
			t.Errorf("Metadata = %v", skill.Metadata)
		}
	})

	t.Run("flow style", func(t *testing.T) {
		content := `---
name: meta-flow
description: Flow metadata test.
metadata: {owner: "API Chief", tier: silver}
---
# Body
`
		skill := parseSkillFromMarkdown(content, "meta-flow.md")
		if len(skill.Metadata) != 2 {
			t.Fatalf("Metadata = %v, want 2 entries", skill.Metadata)
		}
		if skill.Metadata["owner"] != "API Chief" || skill.Metadata["tier"] != "silver" {
			t.Errorf("Metadata = %v", skill.Metadata)
		}
	})

	t.Run("no metadata leaves nil", func(t *testing.T) {
		content := `---
name: plain-skill
description: No metadata.
---
# Body
`
		skill := parseSkillFromMarkdown(content, "plain-skill.md")
		if skill.Metadata != nil {
			t.Errorf("Metadata = %v, want nil", skill.Metadata)
		}
	})
}

func TestValidateSkill(t *testing.T) {
	t.Parallel()

	if v := ValidateSkill(&Skill{Name: "code-review", Description: "Review code."}, "code-review"); len(v) != 0 {
		t.Errorf("valid skill got violations: %v", v)
	}

	cases := []struct {
		name    string
		skill   *Skill
		dirName string
		want    string // substring expected in a violation
	}{
		{name: "uppercase name", skill: &Skill{Name: "CodeReview", Description: "x"}, want: "lowercase"},
		{name: "leading hyphen", skill: &Skill{Name: "-code", Description: "x"}, want: "hyphen"},
		{name: "trailing hyphen", skill: &Skill{Name: "code-", Description: "x"}, want: "hyphen"},
		{name: "double hyphen", skill: &Skill{Name: "code--review", Description: "x"}, want: "consecutive"},
		{name: "empty description", skill: &Skill{Name: "code", Description: ""}, want: "description is required"},
		{name: "long description", skill: &Skill{Name: "code", Description: strings.Repeat("x", 1025)}, want: "1024"},
		{name: "long compatibility", skill: &Skill{Name: "code", Description: "x", Compatibility: strings.Repeat("y", 501)}, want: "500"},
		{name: "name mismatch dir", skill: &Skill{Name: "code", Description: "x"}, dirName: "other", want: "does not match"},
		{name: "empty name", skill: &Skill{Name: "", Description: "x"}, want: "name is required"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ValidateSkill(tc.skill, tc.dirName)
			if len(v) == 0 {
				t.Fatalf("expected a violation for %q", tc.name)
			}
			for _, vi := range v {
				if strings.Contains(vi, tc.want) {
					return
				}
			}
			t.Errorf("violations %v do not mention %q", v, tc.want)
		})
	}
}

func TestProgressiveDisclosure_ResourcesAndGetResource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "my-skill")
	for _, sub := range []string{"scripts", "references", "assets"} {
		if err := os.MkdirAll(filepath.Join(skillDir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	skillMD := `---
name: my-skill
description: A standard skill with resources.
---
# Body
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("# Guide"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "assets", "logo.png"), []byte("PNGDATA"), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(root)
	s, err := m.Get("my-skill")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !s.Standard {
		t.Error("Standard should be true for skill-name/SKILL.md layout")
	}
	if len(s.Resources) != 3 {
		t.Fatalf("got %d resources, want 3: %+v", len(s.Resources), s.Resources)
	}
	kinds := map[string]string{}
	for _, r := range s.Resources {
		kinds[r.Path] = r.Kind
	}
	if kinds["scripts/run.sh"] != ResourceKindScript {
		t.Errorf("scripts/run.sh kind = %q", kinds["scripts/run.sh"])
	}
	if kinds["references/guide.md"] != ResourceKindReference {
		t.Errorf("references/guide.md kind = %q", kinds["references/guide.md"])
	}
	if kinds["assets/logo.png"] != ResourceKindAsset {
		t.Errorf("assets/logo.png kind = %q", kinds["assets/logo.png"])
	}
	if len(m.Warnings["my-skill"]) != 0 {
		t.Errorf("standard skill should have no warnings: %v", m.Warnings["my-skill"])
	}

	instr, err := m.GetInstructions("my-skill")
	if err != nil {
		t.Fatalf("GetInstructions: %v", err)
	}
	if !strings.Contains(instr, "Body") {
		t.Errorf("GetInstructions = %q, want body", instr)
	}

	data, err := m.GetResource("my-skill", "scripts/run.sh")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if !strings.Contains(string(data), "echo hi") {
		t.Errorf("resource content = %q", string(data))
	}

	// Path traversal is rejected.
	if _, err := m.GetResource("my-skill", "../secret"); err == nil {
		t.Error("expected path traversal to be rejected")
	}
	if _, err := m.GetResource("my-skill", "scripts/../../etc/passwd"); err == nil {
		t.Error("expected normalized traversal to be rejected")
	}
	// Missing resource errors.
	if _, err := m.GetResource("my-skill", "scripts/nope.sh"); err == nil {
		t.Error("expected error for missing resource")
	}
}

func TestProgressiveDisclosure_LegacyCategoryDirGetsNoResources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	categoryDir := filepath.Join(root, "skills", "documentation")
	if err := os.MkdirAll(filepath.Join(categoryDir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(categoryDir, "API_DOCUMENTATION.md"), []byte("# API DOCUMENTATION SKILL\n\nGenerates docs.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(categoryDir, "scripts", "helper.sh"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(root)
	s, err := m.Get("API DOCUMENTATION SKILL")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Standard {
		t.Error("legacy flat file should not be marked standard")
	}
	if len(s.Resources) != 0 {
		t.Errorf("legacy skill must not inherit category-dir resources: %+v", s.Resources)
	}
}

func TestParseSkillFromMarkdown_LegacyStatusLine(t *testing.T) {
	t.Parallel()

	content := `> **Version**: 2.1.0 | **Status**: active | **Owner**: API Chief
>
> # API DOCUMENTATION SKILL
>
> ## Description
> Generates API docs from OpenAPI specs.
`
	skill := parseSkillFromMarkdown(content, "API_DOCUMENTATION.md")
	if skill.Name != "API DOCUMENTATION SKILL" {
		t.Errorf("Name = %q, want %q", skill.Name, "API DOCUMENTATION SKILL")
	}
	if skill.Description != "Generates API docs from OpenAPI specs." {
		t.Errorf("Description = %q", skill.Description)
	}
	if skill.Version != "2.1.0" {
		t.Errorf("Version = %q, want %q", skill.Version, "2.1.0")
	}
	if skill.Category != "API Chief" {
		t.Errorf("Category = %q, want %q", skill.Category, "API Chief")
	}
	if skill.fromFrontmatter {
		t.Error("legacy file should not be marked as parsed from frontmatter")
	}
}

func TestValidateAll_LegacyWarningsNonFatal(t *testing.T) {
	t.Parallel()

	m := NewManager("")
	results := m.ValidateAll()
	if len(results) == 0 {
		t.Fatal("embedded skills should load")
	}
	if len(m.List()) != len(results) {
		t.Errorf("List() = %d, ValidateAll() = %d", len(m.List()), len(results))
	}
	if len(m.Warnings) == 0 {
		t.Error("expected validation warnings for legacy embedded skills")
	}
	// At least one legacy skill is reported as non-conformant (warnings, not fatal).
	nonConformant := 0
	for _, r := range results {
		if !r.Valid {
			nonConformant++
		}
	}
	if nonConformant == 0 {
		t.Error("expected at least one non-conformant legacy skill")
	}
}

func TestMigrateLegacy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := `> **Version**: 1.0.0 | **Status**: active | **Owner**: Doc
> # API DOCUMENTATION SKILL
> ## Description
> Generate API docs from OpenAPI specs.
`
	if err := os.WriteFile(filepath.Join(skillsDir, "API_DOCUMENTATION.md"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(root)

	results, err := m.MigrateLegacy(true)
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	var locals []MigrationResult
	for _, r := range results {
		if r.Source == "local" {
			locals = append(locals, r)
		}
	}
	if len(locals) != 1 {
		t.Fatalf("got %d local migration results, want 1", len(locals))
	}
	r := locals[0]
	if r.Action != "written" {
		t.Errorf("action = %q, want written", r.Action)
	}
	if !strings.Contains(r.Frontmatter, "name:") || !strings.Contains(r.Frontmatter, "description:") {
		t.Errorf("frontmatter missing fields: %q", r.Frontmatter)
	}
	if !strings.Contains(r.Frontmatter, "api-documentation-skill") {
		t.Errorf("frontmatter name should be slugified: %q", r.Frontmatter)
	}

	data, err := os.ReadFile(filepath.Join(skillsDir, "API_DOCUMENTATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("file should start with frontmatter after --write")
	}
	if !strings.Contains(string(data), legacy) {
		t.Errorf("original body should be preserved after migration")
	}

	// Second pass: the file now carries frontmatter → skip.
	results2, err := m.MigrateLegacy(false)
	if err != nil {
		t.Fatalf("MigrateLegacy second pass: %v", err)
	}
	for _, r2 := range results2 {
		if r2.Source == "local" && r2.Action != "skip" {
			t.Errorf("second pass action = %q, want skip", r2.Action)
		}
	}
}

func TestInstall_StandardSkillDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	coscaDir := filepath.Join(root, "proj")
	// The source must live inside the manager's skills directory (containment).
	srcRepo := filepath.Join(coscaDir, "skills")
	skillSrc := filepath.Join(srcRepo, "web-scraper")
	if err := os.MkdirAll(filepath.Join(skillSrc, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	skillMD := `---
name: web-scraper
description: Scrape and normalize web content.
---
# Web Scraper
`
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "scripts", "fetch.py"), []byte("print('fetch')"), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(coscaDir)
	s, err := m.Install("web-scraper", skillSrc)
	if err != nil {
		t.Fatalf("Install dir: %v", err)
	}
	if s.Name != "web-scraper" {
		t.Errorf("Name = %q", s.Name)
	}
	if !s.Standard {
		t.Error("installed standard skill should be marked standard")
	}
	if len(s.Resources) != 1 || s.Resources[0].Path != "scripts/fetch.py" {
		t.Errorf("Resources = %+v", s.Resources)
	}

	// Persisted as skills/web-scraper/SKILL.md and reloadable.
	persisted := filepath.Join(coscaDir, "skills", "web-scraper", "SKILL.md")
	if _, err := os.Stat(persisted); err != nil {
		t.Fatalf("persisted SKILL.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "web-scraper", "scripts", "fetch.py")); err != nil {
		t.Fatalf("persisted script missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "web-scraper.md")); !os.IsNotExist(err) {
		t.Error("standard install must not write a flat .md file")
	}

	reloaded := NewManager(coscaDir)
	if _, err := reloaded.Get("web-scraper"); err != nil {
		t.Errorf("reloaded manager should find installed skill: %v", err)
	}
	if data, err := reloaded.GetResource("web-scraper", "scripts/fetch.py"); err != nil || !strings.Contains(string(data), "fetch") {
		t.Errorf("GetResource after reload: %v %q", err, string(data))
	}
}
