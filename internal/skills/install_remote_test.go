package skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ref   string
		owner string
		repo  string
		path  string
		refS  string
		ok    bool
	}{
		{ref: "google/skills", owner: "google", repo: "skills", ok: true},
		{ref: "google/skills/skills/cloud/gke-basics", owner: "google", repo: "skills", path: "skills/cloud/gke-basics", ok: true},
		{ref: "google/skills@main", owner: "google", repo: "skills", refS: "main", ok: true},
		{ref: "google/skills/skills/cloud@v1.2", owner: "google", repo: "skills", path: "skills/cloud", refS: "v1.2", ok: true},
		{ref: "owner/repo-name", owner: "owner", repo: "repo-name", ok: true},
		{ref: "", ok: false},
		{ref: "justowner", ok: false},
		{ref: "a/b/c/d/e", owner: "a", repo: "b", path: "c/d/e", ok: true},
	}

	for _, c := range cases {
		g, err := parseGitHubRef(c.ref)
		if c.ok {
			if err != nil {
				t.Errorf("parseGitHubRef(%q) unexpected error: %v", c.ref, err)
				continue
			}
			if g.Owner != c.owner || g.Repo != c.repo || g.Path != c.path || g.Ref != c.refS {
				t.Errorf("parseGitHubRef(%q) = {%s %s %q %q}, want {%s %s %q %q}",
					c.ref, g.Owner, g.Repo, g.Path, g.Ref, c.owner, c.repo, c.path, c.refS)
			}
		} else if err == nil {
			t.Errorf("parseGitHubRef(%q) expected error, got nil", c.ref)
		}
	}
}

// makeTarGz builds an in-memory gzipped tar with the given entries.
// Each entry is {name, content} or {name, ""} with a trailing slash for dirs.
func makeTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		if strings.HasSuffix(name, "/") {
			if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0755}); err != nil {
				t.Fatalf("write dir header: %v", err)
			}
			continue
		}
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	t.Parallel()

	data := makeTarGz(t, map[string]string{
		"skills-main/":            "",
		"skills-main/SKILL.md":    "# hello",
		"skills-main/scripts/":    "",
		"skills-main/scripts/run": "#!/bin/sh\n",
	})
	dst := t.TempDir()
	if err := extractTarGz(bytes.NewReader(data), 1<<20, dst); err != nil {
		t.Fatalf("extractTarGz error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "skills-main", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "skills-main", "scripts", "run")); err != nil {
		t.Errorf("script not extracted: %v", err)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	t.Parallel()

	data := makeTarGz(t, map[string]string{
		"skills-main/":              "",
		"../escape.md":              "# pwned",
		"skills-main/ok/SKILL.md":   "# ok",
	})
	dst := t.TempDir()
	if err := extractTarGz(bytes.NewReader(data), 1<<20, dst); err == nil {
		t.Fatal("expected error for path traversal entry, got nil")
	}
	if _, err := os.Stat(filepath.Join(dst, "escape.md")); !os.IsNotExist(err) {
		t.Errorf("traversal file should not exist")
	}
}

func TestExtractTarGzRejectsSymlink(t *testing.T) {
	t.Parallel()

	// Build a tar.gz containing a regular file plus a symlink entry
	// (makeTarGz only writes regular files, so build it manually).
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "skills-main/SKILL.md", Mode: 0644, Size: 5, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("# ok"))
	_ = tw.WriteHeader(&tar.Header{Name: "skills-main/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
	_ = tw.Close()
	_ = gz.Close()

	dst := t.TempDir()
	if err := extractTarGz(&buf, 1<<20, dst); err == nil {
		t.Fatal("expected error for symlink entry, got nil")
	}
}

func TestFindSkillDirsRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// google/skills layout: nested category tree.
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/cloud/gke-basics/SKILL.md", "---\nname: gke-basics\ndescription: GKE basics\n---")
	write("skills/cloud/bigquery-basics/SKILL.md", "---\nname: bigquery-basics\ndescription: BigQuery basics\n---")
	write("skills/ai/gemini-api/SKILL.md", "---\nname: gemini-api\ndescription: Gemini API\n---")
	write("skills/ads/README.md", "not a skill") // ignored
	write("README.md", "repo readme")            // ignored

	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		t.Fatalf("findSkillDirsRecursive error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidates = %d, want 3", len(candidates))
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	names := map[string]bool{}
	for _, c := range candidates {
		names[c.skillDir] = true
	}
	for _, want := range []string{"gke-basics", "bigquery-basics", "gemini-api"} {
		if !names[want] {
			t.Errorf("missing candidate %q in %v", want, names)
		}
	}
}

func TestFindSkillDirsSkipsInvalidSlugs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	p := filepath.Join(root, "skills", "Bad Skill Name", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("# x"), 0644); err != nil {
		t.Fatal(err)
	}

	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		t.Fatalf("findSkillDirsRecursive error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for invalid slug, got none")
	}
}

func TestInstallFromGitHubRequiresAllowRemote(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, _, err := m.InstallFromGitHub("google/skills", false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

// TestInstallStandardSkillLayout exercises the full install pipeline with a
// local directory mirroring the google/skills nested layout.
func TestInstallStandardSkillLayout(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	src := t.TempDir()
	skillDir := filepath.Join(src, "skills", "cloud", "gke-basics")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	skillMD := `---
name: gke-basics
description: GKE basics skill for Kubernetes on Google Cloud
license: Apache-2.0
compatibility: google-cloud
---

# GKE Basics

How to manage GKE clusters.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "networking.md"), []byte("# Networking\n"), 0644); err != nil {
		t.Fatal(err)
	}

	installed, err := m.installStandardSkill(skillDir, "gke-basics", filepath.Join(coscaDir, "skills"))
	if err != nil {
		t.Fatalf("installStandardSkill error: %v", err)
	}
	if installed.Name != "gke-basics" {
		t.Errorf("Name = %q, want gke-basics", installed.Name)
	}
	if installed.License != "Apache-2.0" {
		t.Errorf("License = %q, want Apache-2.0", installed.License)
	}
	if !installed.Standard {
		t.Error("skill should be flagged Standard")
	}
	if len(installed.Resources) != 1 {
		t.Fatalf("Resources = %d, want 1 (references/networking.md)", len(installed.Resources))
	}

	// Resource must be readable via progressive disclosure.
	data, err := m.GetResource("gke-basics", "references/networking.md")
	if err != nil {
		t.Fatalf("GetResource error: %v", err)
	}
	if !strings.Contains(string(data), "Networking") {
		t.Errorf("resource content unexpected: %q", string(data))
	}

	// Persisted on disk.
	persisted := filepath.Join(coscaDir, "skills", "gke-basics", "SKILL.md")
	if _, err := os.Stat(persisted); err != nil {
		t.Errorf("skill not persisted on disk: %v", err)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	// Install a standard skill first.
	skillDir := filepath.Join(t.TempDir(), "gke-basics")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: gke-basics\ndescription: GKE basics\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.installStandardSkill(skillDir, "gke-basics", filepath.Join(coscaDir, "skills")); err != nil {
		t.Fatalf("install error: %v", err)
	}
	if _, err := m.Get("gke-basics"); err != nil {
		t.Fatalf("skill should exist after install: %v", err)
	}

	if err := m.Remove("gke-basics"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}
	if _, err := m.Get("gke-basics"); err == nil {
		t.Error("skill should be gone after remove")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "gke-basics")); !os.IsNotExist(err) {
		t.Errorf("skill directory should be removed from disk")
	}

	// Embedded skills cannot be removed.
	embedded := &Manager{skills: map[string]*Skill{"core": {Name: "core", Embedded: true}}}
	if err := embedded.Remove("core"); err == nil {
		t.Error("expected error removing embedded skill")
	}
}

func TestInstallFromDirSingleSkill(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	// Source must live inside the manager's skills directory (containment).
	skillsDir := filepath.Join(coscaDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(skillsDir, "cloud-run-basics")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: cloud-run-basics\ndescription: Deploy containers on Cloud Run\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	skill, err := m.Install("cloud-run-basics", src)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if skill.Name != "cloud-run-basics" {
		t.Errorf("Name = %q", skill.Name)
	}
	if !skill.Standard {
		t.Error("skill should be Standard layout")
	}
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "cloud-run-basics", "SKILL.md")); err != nil {
		t.Errorf("skill not persisted: %v", err)
	}
}

func TestInstallFromDirSkillsRepo(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	// Repo layout: multiple skill-name/SKILL.md subdirectories, inside the
	// manager's skills directory (containment).
	skillsDir := filepath.Join(coscaDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(skillsDir, "gke-repo")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ name, desc string }{
		{"gke-networking", "GKE networking"},
		{"gke-storage", "GKE storage"},
	} {
		dir := filepath.Join(src, pair.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: " + pair.name + "\ndescription: " + pair.desc + "\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Directory without SKILL.md at root → treat as skills repo.
	first, err := m.Install("gke-networking", src)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if first.Name != "gke-networking" {
		t.Errorf("first.Name = %q", first.Name)
	}
	if _, err := m.Get("gke-storage"); err != nil {
		t.Errorf("gke-storage should also be installed: %v", err)
	}
	// Both persisted.
	for _, n := range []string{"gke-networking", "gke-storage"} {
		if _, err := os.Stat(filepath.Join(coscaDir, "skills", n, "SKILL.md")); err != nil {
			t.Errorf("%s not persisted: %v", n, err)
		}
	}
}

func TestListRepoSkillsRequiresAllowRemote(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, _, err := m.ListRepoSkills("google/skills", false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

func TestInstallRepoSkillsRequiresAllowRemote(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, _, err := m.InstallRepoSkills("google/skills", []int{1}, false)
	if err == nil {
		t.Fatal("expected error when allowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote flag, got: %v", err)
	}
}

func TestInstallRepoSkillsBadRef(t *testing.T) {
	t.Parallel()

	m := &Manager{skills: make(map[string]*Skill), coscaDir: t.TempDir()}
	_, _, err := m.InstallRepoSkills("not-a-ref", nil, true)
	if err == nil {
		t.Fatal("expected error for invalid github reference, got nil")
	}
}

// writeSkillRepo scaffolds a local skills repo (skills repo layout: one or
// more skill-name/SKILL.md subdirectories) inside dir and returns its path.
func writeSkillRepo(t *testing.T, skills ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range skills {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: " + name + "\ndescription: Description of " + name + "\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestListRepoSkillsLocal(t *testing.T) {
	t.Parallel()

	root := writeSkillRepo(t, "alpha", "beta", "gamma")

	repoSkills, warnings, err := ListRepoSkillsLocal(root)
	if err != nil {
		t.Fatalf("ListRepoSkillsLocal error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(repoSkills) != 3 {
		t.Fatalf("repo skills = %d, want 3", len(repoSkills))
	}

	// 1-based sequential indexes with the skills' descriptions.
	seen := make(map[int]string)
	for _, s := range repoSkills {
		if s.Index < 1 || s.Index > 3 {
			t.Errorf("Index %d out of range 1-3", s.Index)
		}
		if s.Name != s.Dir {
			t.Errorf("Name %q != Dir %q", s.Name, s.Dir)
		}
		seen[s.Index] = s.Name
	}
	for i := 1; i <= 3; i++ {
		if seen[i] == "" {
			t.Errorf("missing skill at index %d (got %v)", i, seen)
		}
	}
	// Every description was parsed from SKILL.md.
	for _, s := range repoSkills {
		want := "Description of " + s.Name
		if s.Description != want {
			t.Errorf("skill %q description = %q, want %q", s.Name, s.Description, want)
		}
	}
}

func TestListRepoSkillsLocalNoSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("nothing"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ListRepoSkillsLocal(root); err == nil {
		t.Fatal("expected error for repo without SKILL.md, got nil")
	}
}

// TestInstallRepoCandidatesSelective installs only the selected indexes from a
// candidate set (the shared helper behind InstallRepoSkills), so the selection
// logic is exercised without a network.
func TestInstallRepoCandidatesSelective(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	root := writeSkillRepo(t, "alpha", "beta", "gamma")
	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		t.Fatalf("findSkillDirsRecursive error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidates = %d, want 3", len(candidates))
	}

	installed, retWarnings, err := m.installRepoCandidates(candidates, []int{1, 2}, warnings)
	if err != nil {
		t.Fatalf("installRepoCandidates error: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %d, want 2", len(installed))
	}
	names := map[string]bool{}
	for _, s := range installed {
		names[s.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected alpha and beta installed, got %v", names)
	}
	if names["gamma"] {
		t.Error("gamma should not be installed (index 3 not selected)")
	}
	if len(retWarnings) != 0 {
		t.Errorf("unexpected warnings: %v", retWarnings)
	}
	// Only the selected skills persist on disk.
	if _, err := os.Stat(filepath.Join(coscaDir, "skills", "gamma")); !os.IsNotExist(err) {
		t.Errorf("gamma should not be persisted on disk")
	}
	for _, n := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(coscaDir, "skills", n, "SKILL.md")); err != nil {
			t.Errorf("%s not persisted: %v", n, err)
		}
	}
}

// TestInstallRepoCandidatesEmptyIndexes installs ALL candidates, matching
// InstallFromGitHub's behavior.
func TestInstallRepoCandidatesEmptyIndexes(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	root := writeSkillRepo(t, "alpha", "beta")
	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		t.Fatalf("findSkillDirsRecursive error: %v", err)
	}

	installed, _, err := m.installRepoCandidates(candidates, nil, warnings)
	if err != nil {
		t.Fatalf("installRepoCandidates error: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %d, want 2", len(installed))
	}
	for _, n := range []string{"alpha", "beta"} {
		if _, err := m.Get(n); err != nil {
			t.Errorf("skill %q not registered: %v", n, err)
		}
	}
}

// TestInstallRepoCandidatesOutOfRange reports a clear error with the valid
// range and installs nothing.
func TestInstallRepoCandidatesOutOfRange(t *testing.T) {
	t.Parallel()

	coscaDir := t.TempDir()
	m := &Manager{skills: make(map[string]*Skill), coscaDir: coscaDir}

	root := writeSkillRepo(t, "alpha", "beta")
	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		t.Fatalf("findSkillDirsRecursive error: %v", err)
	}

	installed, _, err := m.installRepoCandidates(candidates, []int{7}, warnings)
	if err == nil {
		t.Fatal("expected out-of-range error, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") || !strings.Contains(err.Error(), "1-2") {
		t.Errorf("error should mention the valid range, got: %v", err)
	}
	if len(installed) != 0 {
		t.Errorf("no skill should be installed on out-of-range index, got %d", len(installed))
	}
	if _, err := m.Get("alpha"); err == nil {
		t.Error("alpha should not be registered after out-of-range error")
	}
}
