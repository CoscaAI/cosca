// Package skills provides the skill management system for Cosca.
// Skills define specialized capabilities that agents can use.
//
// The skill format follows the Anthropic Agent Skills standard
// (https://agentskills.io/specification): a skill-name/ directory containing a
// SKILL.md with YAML frontmatter (name, description, license, compatibility,
// metadata, allowed-tools) plus optional scripts/, references/, and assets/
// subdirectories used for progressive disclosure. Legacy Cosca skills — flat
// .md files with descriptive names and a blockquoted status line — remain
// fully supported for backward compatibility.
package skills

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/CoscaAI/cosca/internal/embed"
	"gopkg.in/yaml.v3"
)

// Skill represents a skill that an agent can use.
type Skill struct {
	Name         string `json:"name" yaml:"name"`
	Description  string `json:"description" yaml:"description"`
	Version      string `json:"version" yaml:"version"`
	Category     string `json:"category" yaml:"category"`
	Instructions string `json:"instructions" yaml:"instructions"`
	Tools        []Tool `json:"tools" yaml:"tools"`
	Source       string `json:"source,omitempty" yaml:"source,omitempty"`

	// Agent Skills standard frontmatter fields.
	License       string            `json:"license,omitempty" yaml:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	AllowedTools  string            `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`

	// Progressive disclosure: resource files discovered in the skill directory,
	// loaded on demand via GetResource.
	Resources []SkillResource `json:"resources,omitempty" yaml:"resources,omitempty"`

	// Dir is the skill's directory relative to the skills root ("/" separated).
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
	// Embedded reports whether the skill was loaded from the compiled-in
	// Cosca framework (vs. the local .cosca directory).
	Embedded bool `json:"embedded,omitempty" yaml:"embedded,omitempty"`
	// Standard reports whether the skill uses the standard skill-name/SKILL.md
	// layout defined by the Agent Skills standard.
	Standard bool `json:"standard,omitempty" yaml:"standard,omitempty"`

	// fromFrontmatter reports whether the skill was parsed from a YAML
	// frontmatter block (used by the legacy migration tool). Not serialized.
	fromFrontmatter bool
}

// SkillResource is a file discovered under a skill's scripts/, references/,
// or assets/ directory, available for on-demand loading (progressive
// disclosure). Path is relative to the skill's directory.
type SkillResource struct {
	Path string `json:"path" yaml:"path"`
	Kind string `json:"kind" yaml:"kind"` // script | reference | asset
}

// Resource kind constants, matching the Agent Skills standard directory names.
const (
	ResourceKindScript    = "script"
	ResourceKindReference = "reference"
	ResourceKindAsset     = "asset"
)

// resourceKindDirs maps each Agent Skills resource kind to its directory name.
var resourceKindDirs = []struct {
	dir  string
	kind string
}{
	{dir: "scripts", kind: ResourceKindScript},
	{dir: "references", kind: ResourceKindReference},
	{dir: "assets", kind: ResourceKindAsset},
}

// Tool represents a tool within a skill.
type Tool struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// Manager manages skills in the Cosca system.
type Manager struct {
	mu       sync.RWMutex
	skills   map[string]*Skill
	coscaDir string

	// plugins holds installed Agent Plugins (v1.0.0) keyed by slug name.
	// The registry is in-memory: plugin manifests and mcp.json are persisted
	// to coscaDir/plugins/<name>/, and the skills they carry are installed
	// into coscaDir/skills through the regular skill pipeline.
	plugins map[string]*Plugin

	// Warnings holds non-fatal Agent Skills spec validation warnings keyed by
	// skill name, collected during load/Add. Legacy Cosca skills that do not
	// yet conform to the standard produce warnings here without failing.
	Warnings map[string][]string

	// AllowRemoteSources opts into fetching skill sources over HTTP(S).
	//
	// SECURITY: remote sources are REJECTED by default (fail-closed) to
	// prevent SSRF (e.g. cloud metadata at 169.254.169.254, internal
	// network scanning, or file:// style abuse). This is an intentional
	// decision — the Don can re-enable remote installs by setting this
	// field to true (e.g. via COSCA_SKILLS_ALLOW_REMOTE_SOURCES=true in
	// the serve command), and even then fetches are hardened with a 5s
	// timeout, a 3-redirect cap, and a 10 MiB body limit.
	AllowRemoteSources bool
}

// NewManager creates a skill manager pre-loaded with Cosca framework skills.
// It loads from:
//  1. Embedded Cosca framework (compiled into the binary)
//  2. Optional local .cosca directory (overrides)
func NewManager(coscaDir string) *Manager {
	m := &Manager{
		skills:   make(map[string]*Skill),
		plugins:  make(map[string]*Plugin),
		coscaDir: coscaDir,
		Warnings: make(map[string][]string),
	}
	// Always load from embedded Cosca framework first
	m.loadFromEmbed()
	// Then override with local .cosca if it exists
	if coscaDir != "" {
		m.loadFromDir(coscaDir)
		m.loadPluginsFromDir(coscaDir)
	}
	return m
}

// Add adds a skill to the manager. Spec validation warnings are collected
// (non-fatal) in the manager's Warnings map.
func (m *Manager) Add(skill Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLocked(&skill, "")
}

// addLocked stores a skill and computes its non-fatal spec validation
// warnings. dirName is the parent directory name for standard-layout skills.
func (m *Manager) addLocked(skill *Skill, dirName string) {
	m.skills[skill.Name] = skill
	if m.Warnings == nil {
		m.Warnings = make(map[string][]string)
	}
	violations := ValidateSkill(skill, dirName)
	if len(violations) > 0 {
		m.Warnings[skill.Name] = violations
	} else {
		delete(m.Warnings, skill.Name)
	}
}

// List returns all available skills.
func (m *Manager) List() []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Skill, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, *s)
	}
	return result
}

// Remove removes a locally installed skill from the manager and, when it has
// a local source inside coscaDir/skills, deletes its files from disk.
// Embedded framework skills are never removed — they live in the binary.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	if s.Embedded {
		return fmt.Errorf("skill %q is embedded in the Cosca framework and cannot be removed", name)
	}
	delete(m.skills, name)
	delete(m.Warnings, name)

	// Delete the skill directory (standard layout) or file (legacy layout)
	// from coscaDir/skills when present.
	if m.coscaDir != "" && s.Dir != "" {
		skillsPath := filepath.Join(m.coscaDir, "skills")
		target := filepath.Join(skillsPath, filepath.FromSlash(s.Dir))
		resolved, err := resolveWithinRoot(skillsPath, target)
		if err == nil {
			if fi, statErr := os.Stat(resolved); statErr == nil {
				if fi.IsDir() {
					_ = os.RemoveAll(resolved)
				} else {
					_ = os.Remove(resolved)
				}
			}
		}
	}
	return nil
}

// Get returns a specific skill by name.
func (m *Manager) Get(name string) (*Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Try exact match first
	if s, ok := m.skills[name]; ok {
		return s, nil
	}
	// Try case-insensitive match
	lower := strings.ToLower(name)
	for _, s := range m.skills {
		if strings.ToLower(s.Name) == lower {
			return s, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

// Search searches for skills by query string.
func (m *Manager) Search(query string) ([]Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q := strings.ToLower(query)
	var result []Skill
	for _, s := range m.skills {
		if containsIgnoreCase(s.Name, q) || containsIgnoreCase(s.Description, q) || containsIgnoreCase(s.Category, q) {
			result = append(result, *s)
		}
	}
	return result, nil
}

// GetInstructions returns the skill's instructions (the SKILL.md body). This
// is the progressive-disclosure activation layer: name/description metadata is
// loaded at startup for all skills, while the body is retrieved on demand when
// a skill is activated.
func (m *Manager) GetInstructions(name string) (string, error) {
	s, err := m.Get(name)
	if err != nil {
		return "", err
	}
	return s.Instructions, nil
}

// GetResource loads a skill resource file on demand (progressive disclosure).
// relPath is relative to the skill's directory, e.g. "scripts/tool.sh" or
// "references/guide.md". It reads from the embedded filesystem for framework
// skills and from disk for locally installed skills. Path traversal is
// rejected.
func (m *Manager) GetResource(name, relPath string) ([]byte, error) {
	s, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	if !validResourcePath(relPath) {
		return nil, fmt.Errorf("invalid resource path %q for skill %q", relPath, name)
	}
	if s.Embedded {
		full := path.Join("skills", s.Dir, relPath)
		data, err := embed.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("resource %q for skill %q: %w", relPath, name, err)
		}
		return data, nil
	}
	if m.coscaDir == "" {
		return nil, fmt.Errorf("skill %q has no local source to read resources from", name)
	}
	skillsPath := filepath.Join(m.coscaDir, "skills")
	full := filepath.Join(skillsPath, s.Dir, filepath.FromSlash(relPath))
	resolved, err := resolveWithinRoot(skillsPath, full)
	if err != nil {
		return nil, fmt.Errorf("resource %q for skill %q: %w", relPath, name, err)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("resource %q for skill %q: %w", relPath, name, err)
	}
	return data, nil
}

// validResourcePath rejects paths that could escape a skill directory.
func validResourcePath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return false
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return true
}

// Install installs a skill from a source (file path, standard skill directory,
// or URL). The skill content is parsed, stored in-memory, and persisted to
// coscaDir/skills when coscaDir is configured.
//
// Sources in the standard Agent Skills layout are supported:
//   - a directory containing SKILL.md (a single skill), or
//   - a directory containing one or more skill-name/SKILL.md subdirectories
//     (e.g. the skills/ folder of an Anthropic ecosystem repo).
//
// SECURITY hardening:
//   - name is validated as a slug (^[a-z0-9-]+$) BEFORE any filesystem
//     join, so path traversal via "../" or ".md" tricks is impossible.
//   - remote (http/https) sources are rejected unless AllowRemoteSources
//     is explicitly enabled (SSRF prevention — fail-closed).
//   - local sources must resolve inside coscaDir/skills, and the final
//     write target is verified to stay inside that directory
//     (EvalSymlinks + filepath.Rel containment, same pattern as
//     internal/orchestration/tool_exec.go).
//   - symlinks inside skill directories are rejected during copy.
func (m *Manager) Install(name string, source string) (*Skill, error) {
	if source == "" {
		return nil, fmt.Errorf("source is required for installation")
	}
	if err := validateSkillName(name); err != nil {
		return nil, err
	}

	// Persist to coscaDir/skills if configured. Create the directory up
	// front so the containment checks below can resolve it (symlinks
	// included) and so the source is validated against the real skills dir.
	var skillsPath string
	if m.coscaDir != "" {
		skillsPath = filepath.Join(m.coscaDir, "skills")
		if err := os.MkdirAll(skillsPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create skills directory: %w", err)
		}
	}

	// Standard Agent Skills layout: source is a directory.
	if fi, err := os.Stat(expandHome(source)); err == nil && fi.IsDir() {
		return m.installFromDir(name, source, skillsPath)
	}

	content, err := m.fetchSkillSource(source, skillsPath)
	if err != nil {
		return nil, err
	}

	skill := parseSkillFromMarkdown(content, name+".md")
	// Use the explicitly provided name — it represents the user's intent.
	skill.Name = name

	if skillsPath != "" {
		targetFile := filepath.Join(skillsPath, name+".md")
		// Defense in depth: even with a validated slug, verify the final
		// write target stays inside the skills directory. This also
		// protects against a coscaDir that is (or contains) a symlink
		// pointing elsewhere on disk.
		target, err := resolveWithinRoot(skillsPath, targetFile)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to persist skill: %w", err)
		}
		skill.Source = target
	}

	m.mu.Lock()
	m.addLocked(skill, "")
	m.mu.Unlock()

	return skill, nil
}

// installFromDir installs skills from a directory in the standard Agent Skills
// layout: either a single skill directory (SKILL.md at its root) or a folder
// of skill-name/SKILL.md subdirectories (a skills repo). Every discovered
// skill is copied into skillsPath and registered with the manager.
func (m *Manager) installFromDir(name, source, skillsPath string) (*Skill, error) {
	src := expandHome(source)
	if skillsPath != "" {
		resolved, err := resolveWithinRoot(skillsPath, src)
		if err != nil {
			return nil, err
		}
		src = resolved
	}

	type candidate struct {
		dir      string // absolute source dir for the skill
		skillDir string // target subdir name under skillsPath
	}
	var candidates []candidate
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err == nil {
		// Single skill directory.
		candidates = append(candidates, candidate{dir: src, skillDir: name})
	} else {
		// Skills repo: one or more skill-name/SKILL.md subdirectories.
		entries, err := os.ReadDir(src)
		if err != nil {
			return nil, fmt.Errorf("failed to read skill source directory: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			child := e.Name()
			if err := validateSkillName(child); err != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(src, child, "SKILL.md")); err != nil {
				continue
			}
			candidates = append(candidates, candidate{dir: filepath.Join(src, child), skillDir: child})
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("source directory %q does not contain a SKILL.md (expected a skill-name/SKILL.md layout)", source)
		}
	}

	var installed *Skill
	for _, c := range candidates {
		skill, err := m.installStandardSkill(c.dir, c.skillDir, skillsPath)
		if err != nil {
			return nil, err
		}
		if c.skillDir == name || installed == nil {
			installed = skill
		}
	}
	return installed, nil
}

// installStandardSkill copies a standard Agent Skills directory (SKILL.md plus
// optional scripts/references/assets) into skillsPath/<skillDir>/ and
// registers the parsed skill. Resources are discovered for progressive
// disclosure.
func (m *Manager) installStandardSkill(srcDir, skillDir, skillsPath string) (*Skill, error) {
	content, err := os.ReadFile(filepath.Join(srcDir, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}
	skill := parseSkillFromMarkdown(string(content), "SKILL.md")
	if skill.Name == "" || validateSkillName(skill.Name) != nil {
		skill.Name = skillDir
	}

	if skillsPath != "" {
		targetDir := filepath.Join(skillsPath, skillDir)
		target, err := resolveWithinRoot(skillsPath, targetDir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(target, 0755); err != nil {
			return nil, fmt.Errorf("failed to create skill directory: %w", err)
		}
		if err := copyDir(srcDir, target); err != nil {
			return nil, fmt.Errorf("failed to copy skill directory: %w", err)
		}
		skill.Source = target
	}

	skill.Dir = skillDir
	skill.Standard = true
	if skillsPath != "" {
		skill.Resources = discoverResourcesFromFS(os.DirFS(skillsPath), skillDir)
	} else {
		skill.Resources = discoverResourcesFromFS(os.DirFS(srcDir), ".")
	}

	m.mu.Lock()
	m.addLocked(skill, skillDir)
	m.mu.Unlock()
	return skill, nil
}

// copyDir copies the directory tree src to dst, rejecting symlinks (they could
// point outside the skills directory). Regular files and directories only.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in skill directories: %s", p)
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
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

// expandHome expands a leading ~ in source paths.
func expandHome(source string) string {
	if strings.HasPrefix(source, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, source[1:])
		}
	}
	return source
}

// fetchSkillSource reads skill content from a local file path or, when
// explicitly enabled, an HTTP(S) URL.
//
// Fail-closed policy:
//   - http/https sources are rejected unless m.AllowRemoteSources is true.
//   - local sources must resolve inside skillsPath when the manager
//     persists to a skills directory (skillsPath != "").
func (m *Manager) fetchSkillSource(source, skillsPath string) (string, error) {
	// URL source — opt-in only (SSRF mitigation).
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		if !m.AllowRemoteSources {
			return "", fmt.Errorf("remote skill sources are disabled for security: %s", source)
		}
		return fetchRemoteSkillSource(source)
	}

	// File path: expand ~ if present.
	expanded := expandHome(source)

	// Containment: when the manager persists to a skills directory, the
	// source must live inside that directory (rejects /etc/passwd and any
	// other path traversal / arbitrary file read).
	if skillsPath != "" {
		resolved, err := resolveWithinRoot(skillsPath, expanded)
		if err != nil {
			return "", err
		}
		expanded = resolved
	}

	data, err := os.ReadFile(expanded)
	if err != nil {
		return "", fmt.Errorf("failed to read skill file %s: %w", expanded, err)
	}
	return string(data), nil
}

// fetchRemoteSkillSource downloads skill content over HTTP(S) with a
// hardened client: 5s timeout, at most 3 redirects, and a 10 MiB body cap.
// This path is only reachable when AllowRemoteSources is explicitly set.
func fetchRemoteSkillSource(source string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}
	resp, err := client.Get(source)
	if err != nil {
		return "", fmt.Errorf("failed to download skill from %s: %w", source, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download skill: HTTP %d", resp.StatusCode)
	}
	const maxRemoteSkillBytes = 10 << 20 // 10 MiB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteSkillBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read skill data: %w", err)
	}
	if len(data) > maxRemoteSkillBytes {
		return "", fmt.Errorf("skill source exceeds %d bytes", maxRemoteSkillBytes)
	}
	return string(data), nil
}

// skillNameRe matches valid skill names: lowercase letters, digits, and
// hyphens only. Slashes, dots, spaces, and uppercase are rejected so a name
// can never escape the skills directory or collide with reserved files.
var skillNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// validateSkillName rejects names that could escape the skills directory or
// otherwise break out of the slug convention. Applied BEFORE any
// filepath.Join so path traversal is impossible.
func validateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	if !skillNameRe.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: must match ^[a-z0-9-]+$ (lowercase letters, digits, and hyphens only)", name)
	}
	return nil
}

// resolveWithinRoot cleans, absolutizes, and symlink-resolves child, then
// ensures it is equal to or inside root. It mirrors the sandbox pattern used
// in internal/orchestration/tool_exec.go (EvalSymlinks + filepath.Rel).
// Returns the cleaned absolute child path on success.
func resolveWithinRoot(root, child string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve skills directory: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		// Root does not exist yet (e.g. first install); fall back to the
		// cleaned absolute path.
		rootResolved = rootAbs
	}

	childAbs, err := filepath.Abs(child)
	if err != nil {
		return "", fmt.Errorf("resolve skill source: %w", err)
	}
	cleaned := filepath.Clean(childAbs)

	// Resolve symlinks when the file exists; otherwise validate the
	// cleaned path against the resolved root.
	childResolved := cleaned
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		childResolved = resolved
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve skill source: %w", err)
	}

	if !isPathWithin(rootResolved, childResolved) {
		return "", fmt.Errorf("skill source %q is outside the skills directory", child)
	}
	return cleaned, nil
}

// isPathWithin returns true if child is equal to or a descendant of parent.
func isPathWithin(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	// If rel starts with "..", child is outside parent.
	return !strings.HasPrefix(rel, "..")
}

// loadFromEmbed loads all skills from the embedded Cosca framework.
func (m *Manager) loadFromEmbed() {
	files, err := embed.ListFilesRecursive("skills")
	if err != nil {
		return
	}
	skillsFS, subErr := fs.Sub(embed.CoscaAssets, path.Join(embed.CoscaRoot, "skills"))
	for _, f := range files {
		if strings.EqualFold(filepath.Base(f), "SKILLS_CATALOG.md") {
			continue
		}
		content, err := embed.ReadString(f)
		if err != nil {
			continue
		}
		skill := parseSkillFromMarkdown(content, filepath.Base(f))
		if skill.Name == "" {
			continue
		}
		relDir := path.Dir(f)
		dirName := ""
		if strings.EqualFold(path.Base(f), "SKILL.md") {
			// Standard skill-name/SKILL.md layout: discover resources.
			dirName = path.Base(relDir)
			if subErr == nil {
				skill.Resources = discoverResourcesFromFS(skillsFS, relDir)
			}
			skill.Standard = true
		}
		skill.Dir = relDir
		skill.Embedded = true
		m.mu.Lock()
		m.addLocked(skill, dirName)
		m.mu.Unlock()
	}
}

// loadFromDir scans a directory for skills markdown files (local overrides).
func (m *Manager) loadFromDir(dir string) {
	skillsDir := filepath.Join(dir, "skills")
	info, err := os.Stat(skillsDir)
	if err != nil || !info.IsDir() {
		return
	}
	root := os.DirFS(skillsDir)

	_ = filepath.Walk(skillsDir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") || strings.EqualFold(info.Name(), "SKILLS_CATALOG.md") {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		skill := parseSkillFromMarkdown(string(data), info.Name())
		if skill.Name == "" {
			return nil
		}
		rel, relErr := filepath.Rel(skillsDir, p)
		if relErr != nil {
			return nil
		}
		relDir := filepath.Dir(rel)
		dirName := ""
		if strings.EqualFold(info.Name(), "SKILL.md") {
			// Standard skill-name/SKILL.md layout: discover resources.
			dirName = filepath.Base(relDir)
			skill.Resources = discoverResourcesFromFS(root, filepath.ToSlash(relDir))
			skill.Standard = true
		}
		skill.Dir = filepath.ToSlash(relDir)
		m.mu.Lock()
		m.addLocked(skill, dirName)
		m.mu.Unlock()
		return nil
	})
}

// discoverResourcesFromFS populates resources from the standard Agent Skills
// resource subdirectories (scripts/, references/, assets/) located under
// relDir within fsys. Paths are relative to the skill directory and use '/'
// separators. Files are discovered one level deep, per the standard.
func discoverResourcesFromFS(fsys fs.FS, relDir string) []SkillResource {
	var resources []SkillResource
	for _, rd := range resourceKindDirs {
		sub := path.Join(relDir, rd.dir)
		entries, err := fs.ReadDir(fsys, sub)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			resources = append(resources, SkillResource{
				Path: path.Join(rd.dir, e.Name()),
				Kind: rd.kind,
			})
		}
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Path < resources[j].Path })
	return resources
}

// parseSkillFromMarkdown extracts skill metadata from markdown content.
// It handles three formats:
//  1. YAML frontmatter (--- blocks) per the Agent Skills standard, with a
//     markdown body as the skill's instructions.
//  2. Cosca embedded format: blockquoted status line, headings, and tables.
//  3. Plain headings with descriptive text (legacy).
func parseSkillFromMarkdown(content, filename string) *Skill {
	skill := &Skill{
		Version: "1.0",
		Source:  filename,
	}

	// Compute a fallback name from the filename (used only as a last resort).
	fallbackName := strings.TrimSuffix(filename, ".md")
	fallbackName = strings.ReplaceAll(fallbackName, "_", " ")
	fallbackName = strings.ReplaceAll(fallbackName, "-", " ")

	// Normalize blockquoted content: strip "> " prefix from every line
	// when the file uses the Cosca embedded blockquote style.
	normalized := normalizeBlockquotes(content)
	lines := strings.Split(normalized, "\n")

	inFrontmatter := false
	frontmatterLines := []string{}
	inTable := false
	inDescription := false
	var instructionsBuilder strings.Builder
	var bodyBuilder strings.Builder
	capturedDescription := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// YAML frontmatter ---
		if trimmed == "---" && !inFrontmatter && i == 0 {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			inFrontmatter = false
			parseFrontmatter(frontmatterLines, skill)
			continue
		}
		if inFrontmatter {
			// Keep raw lines so YAML indentation (e.g. nested metadata
			// maps) is preserved.
			frontmatterLines = append(frontmatterLines, line)
			continue
		}

		// For standard skills parsed from frontmatter, the raw markdown body
		// IS the instructions (progressive-disclosure activation layer). Keep
		// it verbatim, headings included.
		if skill.fromFrontmatter {
			bodyBuilder.WriteString(line)
			bodyBuilder.WriteString("\n")
		}

		// Parse Cosca status line: **Version**: X | **Status**: Y | **Owner**: Z
		if strings.Contains(trimmed, "**Version**:") && strings.Contains(trimmed, "**Status**:") {
			parseStatusLine(trimmed, skill)
			continue
		}

		// Extract name from "# SKILL NAME" heading — only if not set by frontmatter.
		if skill.Name == "" && strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "##") {
			parts := strings.SplitN(trimmed, " ", 2)
			if len(parts) == 2 {
				skill.Name = parts[1]
			}
			continue
		}

		// Detect "## Description" section.
		if strings.EqualFold(trimmed, "## description") {
			inDescription = true
			continue
		}

		// End of description section when we hit another heading.
		if inDescription && strings.HasPrefix(trimmed, "##") {
			inDescription = false
			if capturedDescription {
				continue
			}
		}

		// Capture description text.
		if inDescription && trimmed != "" && !capturedDescription {
			skill.Description = trimmed
			capturedDescription = true
			continue
		}
		// If within description and we see an empty line, end it.
		if inDescription && trimmed == "" {
			inDescription = false
			continue
		}

		// Legacy: extract description from first non-heading, non-empty line
		// after the h1 if we haven't captured one yet.
		if skill.Description == "" && trimmed != "" &&
			!strings.HasPrefix(trimmed, "#") &&
			!strings.HasPrefix(trimmed, "---") &&
			!strings.Contains(trimmed, "**Version**:") {
			skill.Description = trimmed
		}

		// Parse pipe tables for tools and inputs/outputs.
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			parts := strings.Split(trimmed, "|")
			var cols []string
			for _, p := range parts {
				col := strings.TrimSpace(p)
				if col != "" {
					cols = append(cols, col)
				}
			}
			if len(cols) >= 2 && !inTable && !strings.Contains(trimmed, "---") {
				first := strings.ToLower(cols[0])
				if first == "name" || first == "tool" || first == "input" || first == "output" {
					inTable = true
				} else {
					tool := Tool{Name: cols[0], Description: cols[1]}
					skill.Tools = append(skill.Tools, tool)
				}
			}
		} else {
			inTable = false
		}

		// Collect non-empty, non-metadata lines as instructions.
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "#") &&
			!strings.HasPrefix(trimmed, ">") &&
			!strings.HasPrefix(trimmed, "|") &&
			!strings.HasPrefix(trimmed, "---") &&
			!strings.HasPrefix(trimmed, "- [") &&
			!strings.Contains(trimmed, "**Version**:") {
			if instructionsBuilder.Len() > 0 {
				instructionsBuilder.WriteString("\n")
			}
			instructionsBuilder.WriteString(trimmed)
		}
	}

	if skill.fromFrontmatter {
		skill.Instructions = strings.TrimSpace(bodyBuilder.String())
	} else {
		skill.Instructions = strings.TrimSpace(instructionsBuilder.String())
	}

	// Use filename-derived name as a last resort.
	if skill.Name == "" {
		skill.Name = fallbackName
	}

	return skill
}

// parseFrontmatter populates skill fields from a YAML frontmatter block. It
// parses the block as a YAML document so nested metadata maps and quoted
// values are handled correctly while scalar values (e.g. version: 2.0) are
// preserved verbatim. When the block is not valid YAML (legacy files), it
// falls back to a line-based parser.
func parseFrontmatter(lines []string, skill *Skill) {
	skill.fromFrontmatter = true
	raw := strings.Join(lines, "\n")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err == nil && len(doc.Content) > 0 {
		node := doc.Content[0]
		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				key := node.Content[i]
				val := node.Content[i+1]
				if key.Kind != yaml.ScalarNode {
					continue
				}
				switch key.Value {
				case "name":
					skill.Name = yamlScalar(val)
				case "description":
					skill.Description = yamlScalar(val)
				case "version":
					skill.Version = yamlScalar(val)
				case "category":
					skill.Category = yamlScalar(val)
				case "license":
					skill.License = yamlScalar(val)
				case "compatibility":
					skill.Compatibility = yamlScalar(val)
				case "allowed-tools":
					skill.AllowedTools = yamlScalar(val)
				case "metadata":
					if val.Kind == yaml.MappingNode {
						skill.Metadata = parseMetadataNode(val)
					}
				}
			}
			return
		}
	}
	parseFrontmatterLines(lines, skill)
}

// yamlScalar returns the verbatim scalar value of a YAML node, or "" for
// non-scalar nodes.
func yamlScalar(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return ""
}

// parseMetadataNode extracts a string→string map from a YAML mapping node.
func parseMetadataNode(node *yaml.Node) map[string]string {
	m := make(map[string]string)
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			continue
		}
		if v.Kind == yaml.ScalarNode {
			m[k.Value] = v.Value
		}
	}
	return m
}

// parseFrontmatterLines is the line-based fallback for frontmatter blocks that
// are not valid YAML. It handles the Agent Skills scalar fields plus nested
// "metadata:" key/value pairs indented by two spaces.
func parseFrontmatterLines(lines []string, skill *Skill) {
	inMetadata := false
	for _, rawLine := range lines {
		fl := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(fl, "name:"):
			skill.Name = frontmatterValue(fl, "name:")
		case strings.HasPrefix(fl, "description:"):
			skill.Description = frontmatterValue(fl, "description:")
		case strings.HasPrefix(fl, "version:"):
			skill.Version = frontmatterValue(fl, "version:")
		case strings.HasPrefix(fl, "category:"):
			skill.Category = frontmatterValue(fl, "category:")
		case strings.HasPrefix(fl, "license:"):
			skill.License = frontmatterValue(fl, "license:")
		case strings.HasPrefix(fl, "compatibility:"):
			skill.Compatibility = frontmatterValue(fl, "compatibility:")
		case strings.HasPrefix(fl, "allowed-tools:"):
			skill.AllowedTools = frontmatterValue(fl, "allowed-tools:")
		case strings.HasPrefix(fl, "metadata:"):
			inMetadata = true
			if skill.Metadata == nil {
				skill.Metadata = make(map[string]string)
			}
			if rest := strings.TrimSpace(strings.TrimPrefix(fl, "metadata:")); rest != "" && rest != "{}" {
				parseInlineMap(rest, skill.Metadata)
			}
		default:
			if inMetadata && fl != "" && strings.Contains(fl, ":") {
				indented := strings.HasPrefix(rawLine, "  ") || strings.HasPrefix(rawLine, "\t")
				if indented {
					if idx := strings.Index(fl, ":"); idx > 0 {
						skill.Metadata[strings.TrimSpace(fl[:idx])] = stripQuotes(strings.TrimSpace(fl[idx+1:]))
					}
				} else if !strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t") {
					inMetadata = false
				}
			}
		}
	}
}

// frontmatterValue extracts the value of a "key: value" frontmatter line.
func frontmatterValue(line, prefix string) string {
	return stripQuotes(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
}

// parseInlineMap parses a simple YAML flow map like `{a: b, c: d}`.
func parseInlineMap(s string, into map[string]string) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "{")
	s = strings.TrimSuffix(strings.TrimSpace(s), "}")
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if idx := strings.Index(pair, ":"); idx > 0 {
			into[strings.TrimSpace(pair[:idx])] = stripQuotes(strings.TrimSpace(pair[idx+1:]))
		}
	}
}

// stripQuotes removes a single matching pair of surrounding quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// normalizeBlockquotes strips the "> " prefix from each line when the content
// uses the Cosca embedded blockquote style (majority of lines start with "> ").
// Lines that are only ">" (empty blockquote) become empty strings.
func normalizeBlockquotes(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Count non-empty lines and how many start with "> ".
	nonEmpty := 0
	blockquoted := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		nonEmpty++
		if strings.HasPrefix(t, "> ") || strings.HasPrefix(t, ">#") || strings.HasPrefix(t, ">|") || t == ">" {
			blockquoted++
		}
	}

	// Only normalize if more than half of non-empty lines are blockquoted.
	if nonEmpty == 0 || float64(blockquoted)/float64(nonEmpty) < 0.5 {
		return content
	}

	var result strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " >\t")
		// Preserve original line if it was entirely blank (just ">").
		if strings.TrimSpace(line) == ">" {
			trimmed = ""
		}
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(trimmed)
	}
	return result.String()
}

// parseStatusLine extracts version, status, and category from an Cosca status line.
// Format: **Version**: 1.0.0 | **Status**: active | **Owner**: Some Chief
func parseStatusLine(line string, skill *Skill) {
	pairs := strings.Split(line, "|")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if strings.HasPrefix(pair, "**Version**:") {
			skill.Version = strings.TrimSpace(strings.TrimPrefix(pair, "**Version**:"))
		} else if strings.HasPrefix(pair, "**Status**:") {
			// Store status as a pseudo-category or ignore for now.
		} else if strings.HasPrefix(pair, "**Owner**:") {
			skill.Category = strings.TrimSpace(strings.TrimPrefix(pair, "**Owner**:"))
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, substr)
}

// ValidateSkill checks a parsed Skill against the Agent Skills standard naming
// and frontmatter rules. dirName is the name of the parent directory when the
// skill uses the standard skill-name/SKILL.md layout ("" otherwise). It returns
// a list of violations; an empty list means the skill is conformant.
//
// Rules enforced:
//   - name: required, 1-64 chars, lowercase alphanumeric + hyphens, no
//     leading/trailing/double hyphens, must match the parent directory name
//   - description: required, 1-1024 chars
//   - compatibility: at most 500 chars when set
func ValidateSkill(s *Skill, dirName string) []string {
	var violations []string

	if s.Name == "" {
		violations = append(violations, "name is required")
	} else {
		if n := utf8.RuneCountInString(s.Name); n > 64 {
			violations = append(violations, fmt.Sprintf("name exceeds 64 characters (%d)", n))
		}
		if !skillNameRe.MatchString(s.Name) {
			violations = append(violations, fmt.Sprintf("name %q must be lowercase alphanumeric with hyphens only", s.Name))
		} else {
			if strings.HasPrefix(s.Name, "-") || strings.HasSuffix(s.Name, "-") {
				violations = append(violations, fmt.Sprintf("name %q must not start or end with a hyphen", s.Name))
			}
			if strings.Contains(s.Name, "--") {
				violations = append(violations, fmt.Sprintf("name %q must not contain consecutive hyphens", s.Name))
			}
		}
		if dirName != "" && s.Name != dirName {
			violations = append(violations, fmt.Sprintf("name %q does not match parent directory name %q", s.Name, dirName))
		}
	}

	if s.Description == "" {
		violations = append(violations, "description is required")
	} else if n := utf8.RuneCountInString(s.Description); n > 1024 {
		violations = append(violations, fmt.Sprintf("description exceeds 1024 characters (%d)", n))
	}

	if s.Compatibility != "" && utf8.RuneCountInString(s.Compatibility) > 500 {
		violations = append(violations, fmt.Sprintf("compatibility exceeds 500 characters (%d)", utf8.RuneCountInString(s.Compatibility)))
	}

	return violations
}

// ValidationResult reports Agent Skills spec validation for a single skill.
type ValidationResult struct {
	Name       string   `json:"name"`
	Dir        string   `json:"dir,omitempty"`
	Source     string   `json:"source,omitempty"`
	Standard   bool     `json:"standard"`
	Violations []string `json:"violations,omitempty"`
	Valid      bool     `json:"valid"`
}

// ValidateAll validates every loaded skill against the Agent Skills standard.
// Results are sorted by name. Legacy Cosca skills commonly produce warnings
// (uppercase/spacey names, missing frontmatter) — these are reported, not
// fatal.
func (m *Manager) ValidateAll() []ValidationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]ValidationResult, 0, len(m.skills))
	for _, s := range m.skills {
		dirName := ""
		if s.Standard {
			dirName = filepath.Base(s.Dir)
		}
		violations := ValidateSkill(s, dirName)
		results = append(results, ValidationResult{
			Name:       s.Name,
			Dir:        s.Dir,
			Source:     s.Source,
			Standard:   s.Standard,
			Violations: violations,
			Valid:      len(violations) == 0,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

// MigrationResult describes the outcome of the Agent Skills migration for a
// single legacy skill file.
type MigrationResult struct {
	File        string `json:"file"`
	Source      string `json:"source"` // "embed" | "local"
	Name        string `json:"name"`
	Frontmatter string `json:"frontmatter,omitempty"`
	Action      string `json:"action"` // skip | would-add | written | embedded-readonly
}

// MigrateLegacy computes the frontmatter block that makes each legacy skill
// conformant with the Agent Skills standard. When write is true and the skill
// has a local source, the frontmatter is prepended to the file. Embedded
// framework skills are always reported read-only (they live in the compiled
// binary). Skills that already carry spec frontmatter are skipped.
func (m *Manager) MigrateLegacy(write bool) ([]MigrationResult, error) {
	var results []MigrationResult

	files, err := embed.ListFilesRecursive("skills")
	if err == nil {
		for _, f := range files {
			if strings.EqualFold(filepath.Base(f), "SKILLS_CATALOG.md") {
				continue
			}
			content, err := embed.ReadString(f)
			if err != nil {
				continue
			}
			skill := parseSkillFromMarkdown(content, filepath.Base(f))
			res := buildMigrationResult(skill, f, "embed")
			if res.Action == "would-add" {
				res.Action = "embedded-readonly"
			}
			results = append(results, res)
		}
	}

	if m.coscaDir != "" {
		skillsDir := filepath.Join(m.coscaDir, "skills")
		_ = filepath.Walk(skillsDir, func(p string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") ||
				strings.EqualFold(info.Name(), "SKILLS_CATALOG.md") {
				return nil
			}
			rel, err := filepath.Rel(skillsDir, p)
			if err != nil {
				return nil
			}
			if hasHiddenSegment(rel) {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			skill := parseSkillFromMarkdown(string(data), info.Name())
			res := buildMigrationResult(skill, rel, "local")
			if res.Action == "would-add" && write {
				block := res.Frontmatter
				if err := os.WriteFile(p, []byte(block+"\n"+string(data)), 0644); err == nil {
					res.Action = "written"
				}
			}
			results = append(results, res)
			return nil
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// buildMigrationResult generates the frontmatter block for a legacy skill, or
// reports that the skill is already conformant.
func buildMigrationResult(skill *Skill, file, source string) MigrationResult {
	res := MigrationResult{File: file, Source: source, Name: skill.Name}
	if skill.fromFrontmatter && skill.Name != "" && skill.Description != "" {
		res.Action = "skip"
		return res
	}

	name := slugifyName(skill.Name)
	if name == "" {
		name = slugifyName(strings.TrimSuffix(filepath.Base(file), ".md"))
	}
	if name == "" {
		name = "skill"
	}
	if utf8.RuneCountInString(name) > 64 {
		name = string([]rune(name)[:64])
	}

	desc := strings.TrimSpace(skill.Description)
	if desc == "" {
		desc = "Cosca legacy skill migrated from " + file
	}
	if utf8.RuneCountInString(desc) > 1024 {
		desc = string([]rune(desc)[:1024])
	}

	res.Name = name
	res.Frontmatter = fmt.Sprintf("---\nname: %s\ndescription: %s\n---", name, desc)
	res.Action = "would-add"
	return res
}

// slugifyName converts an arbitrary skill name into a spec-conformant slug:
// lowercase, alphanumeric + hyphens only, no leading/trailing/double hyphens.
func slugifyName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// hasHiddenSegment reports whether any path segment starts with "." (e.g.
// .archive, .usage.json), used to skip hidden files during migration.
func hasHiddenSegment(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
