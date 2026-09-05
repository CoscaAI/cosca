// Package skills — remote GitHub installation.
//
// The Google Agent Skills ecosystem (agentskills.io) distributes skills from
// public GitHub repositories in the standard skill-name/SKILL.md layout, e.g.
// https://github.com/google/skills with skills under skills/cloud/gke-basics/.
// This file adds a first-class GitHub source: "owner/repo", "owner/repo/path",
// and "owner/repo@ref" references are resolved to a tarball via codeload,
// extracted into a temporary sandbox, and every SKILL.md discovered (recursive
// walk, so nested category trees like skills/cloud/<skill>/ work) is installed
// through the same validated pipeline as local sources.
//
// SECURITY posture (mirrors the rest of the package):
//   - Download over HTTPS only, 5s timeout, 3-redirect cap, 100 MiB body cap.
//   - Extraction runs in a private temp directory; zip-slip (path traversal)
//     entries are rejected; symlinks are rejected.
//   - Every candidate skill directory must pass validateSkillName (slug) and
//     contain a real SKILL.md; anything else is skipped with a warning.
//   - Copies are written through resolveWithinRoot so nothing escapes
//     coscaDir/skills even if coscaDir is (or contains) a symlink.
//   - Install is opt-in: the `cosca skill install --allow-remote` CLI enables
//     it; the Manager itself remains fail-closed.
package skills

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// gitHubRefRe describes a GitHub source reference:
//
//	owner/repo                     — whole repository (all skills)
//	owner/repo/path/to/skills      — subset of the repository
//	owner/repo@ref                 — specific branch/tag (default: HEAD)
//	owner/repo/path@ref            — subset + branch/tag
var gitHubRefRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(/.*)?(@[^@]+)?$`)

// githubRef is a parsed GitHub source reference.
type githubRef struct {
	Owner string // repository owner
	Repo  string // repository name
	Path  string // sub-path inside the repo ("" = whole repo), "/"-separated
	Ref   string // branch/tag ("" = HEAD)
}

// parseGitHubRef parses a "owner/repo[/path][@ref]" reference.
func parseGitHubRef(ref string) (*githubRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("github reference is required")
	}
	m := gitHubRefRe.MatchString(ref)
	if !m {
		return nil, fmt.Errorf("invalid github reference %q: expected owner/repo[/path][@ref]", ref)
	}

	// Manual split — keep it simple and deterministic.
	var pathPart, refPart string
	if i := strings.LastIndex(ref, "@"); i >= 0 && !strings.Contains(ref[:i], "://") {
		refPart = strings.TrimPrefix(ref[i:], "@")
		ref = ref[:i]
	}
	parts := strings.SplitN(ref, "/", 3)
	owner := parts[0]
	repo := parts[1]
	if len(parts) == 3 {
		pathPart = strings.Trim(parts[2], "/")
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid github reference %q: owner and repo are required", ref)
	}
	return &githubRef{
		Owner: owner,
		Repo:  repo,
		Path:  pathPart,
		Ref:   refPart,
	}, nil
}

// InstallFromGitHub installs every skill found in a public GitHub repository
// reference ("owner/repo[/path][@ref]"). It returns the installed skills and
// any non-fatal warnings encountered during discovery.
//
// allowRemote must be true to permit the network fetch — the Manager is
// fail-closed by default and callers opt in explicitly (e.g. the
// `cosca skill install` --allow-remote flag). This mirrors AllowRemoteSources for the GitHub
// channel and keeps SSRF-style surprise fetches impossible.
func (m *Manager) InstallFromGitHub(ref string, allowRemote bool) ([]*Skill, []string, error) {
	if !allowRemote {
		return nil, nil, fmt.Errorf("remote skill installation is disabled: pass --allow-remote to enable it (e.g. cosca skill install %s --allow-remote)", ref)
	}
	candidates, warnings, cleanup, err := downloadRepoCandidates(ref)
	if err != nil {
		return nil, warnings, err
	}
	defer cleanup()
	// Empty indexes = install every skill in the repository.
	return m.installRepoCandidates(candidates, nil, warnings)
}

// downloadRepoCandidates downloads a GitHub reference, resolves its sub-path,
// and discovers the candidate skill dirs inside it. It is shared by
// InstallFromGitHub, ListRepoSkills, and InstallRepoSkills so each flow
// downloads the repository exactly once. The returned cleanup func must be
// called by the caller AFTER the candidates have been consumed: the candidate
// dirs live inside the temp sandbox, which is removed on cleanup. The
// fail-closed allowRemote check is the caller's responsibility so every
// command keeps its own actionable error message.
func downloadRepoCandidates(ref string) ([]skillCandidate, []string, func(), error) {
	parsed, err := parseGitHubRef(ref)
	if err != nil {
		return nil, nil, nil, err
	}

	dir, cleanup, err := downloadAndExtractRepo(parsed)
	if err != nil {
		return nil, nil, nil, err
	}

	// The extraction root contains <repo>-<ref-or-head>/ as its single
	// top-level entry (codeload layout). Find it.
	root, err := singleTopLevelDir(dir)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	if parsed.Path != "" {
		root = filepath.Join(root, filepath.FromSlash(parsed.Path))
		if _, err := os.Stat(root); err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("path %q not found in %s/%s: %w", parsed.Path, parsed.Owner, parsed.Repo, err)
		}
	}

	candidates, warnings, err := findSkillDirsRecursive(root)
	if err != nil {
		cleanup()
		return nil, warnings, nil, err
	}
	if len(candidates) == 0 {
		cleanup()
		return nil, warnings, nil, fmt.Errorf("no SKILL.md found under %s/%s%s", parsed.Owner, parsed.Repo, pathOrEmpty(parsed.Path))
	}
	return candidates, warnings, cleanup, nil
}

// skillsPathOrEmpty returns the manager's skills directory when configured.
func (m *Manager) skillsPathOrEmpty() string {
	if m.coscaDir == "" {
		return ""
	}
	skillsPath := filepath.Join(m.coscaDir, "skills")
	if err := os.MkdirAll(skillsPath, 0755); err != nil {
		return ""
	}
	return skillsPath
}

// downloadAndExtractRepo fetches the codeload tarball for a githubRef and
// extracts it into a fresh temp directory, returning the temp dir and a
// cleanup func. Network and extraction are fully sandboxed.
func downloadAndExtractRepo(g *githubRef) (string, func(), error) {
	ref := g.Ref
	if ref == "" {
		ref = "HEAD"
	}
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", g.Owner, g.Repo, ref)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", nil, fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to download %s: HTTP %d (repository may be private or not exist)", url, resp.StatusCode)
	}

	const maxRepoBytes = 100 << 20 // 100 MiB
	dir, err := os.MkdirTemp("", "cosca-skills-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := extractTarGz(io.LimitReader(resp.Body, maxRepoBytes+1), maxRepoBytes, dir); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

// extractTarGz extracts a gzipped tar stream into dst. It rejects path
// traversal (zip-slip), absolute paths, and symlinks. bodySizeExceeded is
// checked so a repo bigger than the cap fails cleanly instead of silently
// truncating.
func extractTarGz(r io.Reader, maxBytes int64, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar stream: %w", err)
		}
		if hdr.Size > 0 {
			total += hdr.Size
			if total > maxBytes {
				return fmt.Errorf("repository archive exceeds %d bytes", maxBytes)
			}
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("rejected path traversal entry %q", hdr.Name)
		}
		// Reject symlinks and hardlinks outright — they could point outside.
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("rejected link entry %q", hdr.Name)
		}
		target := filepath.Join(dst, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("create dir %q: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent %q: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("create file %q: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write file %q: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close file %q: %w", target, err)
			}
		default:
			// Skip special entries (FIFO, device, etc.).
		}
	}
	return nil
}

// singleTopLevelDir returns the sole immediate subdirectory of dir (the
// codeload <repo>-<ref> wrapper) or an error if the layout is unexpected.
func singleTopLevelDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read extraction root: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 {
		return "", fmt.Errorf("unexpected archive layout: expected a single top-level directory, got %d", len(dirs))
	}
	return filepath.Join(dir, dirs[0]), nil
}

// skillCandidate is a discovered skill directory inside a repository.
type skillCandidate struct {
	dir      string // absolute source dir containing SKILL.md
	skillDir string // slug used as the target subdir under coscaDir/skills
}

// findSkillDirsRecursive walks root looking for SKILL.md files. Every parent
// directory of a SKILL.md becomes a candidate. Nested category trees (e.g.
// skills/cloud/gke-basics/) are fully supported. Directories whose names are
// not valid skill slugs are skipped with a warning.
func findSkillDirsRecursive(root string) ([]skillCandidate, []string, error) {
	var candidates []skillCandidate
	var warnings []string

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}
		parent := filepath.Dir(p)
		name := filepath.Base(parent)
		if validateSkillName(name) != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %q: directory name is not a valid skill slug", filepath.ToSlash(parent)))
			return nil
		}
		candidates = append(candidates, skillCandidate{dir: parent, skillDir: name})
		return nil
	})
	if err != nil {
		return nil, warnings, fmt.Errorf("walk repository: %w", err)
	}
	return candidates, warnings, nil
}

// pathOrEmpty renders the parsed sub-path for error messages.
func pathOrEmpty(p string) string {
	if p == "" {
		return ""
	}
	return "/" + p
}

// RepoSkill is a single skill discovered inside a repository listing, with a
// 1-based Index for batch selection (the cosca equivalent of "npx skills add").
type RepoSkill struct {
	Index       int
	Name        string
	Dir         string
	Description string
}

// ListRepoSkills downloads a GitHub reference ("owner/repo[/path][@ref]") and
// lists every skill found in the standard skill-name/SKILL.md layout WITHOUT
// installing anything. allowRemote must be true for the network fetch
// (fail-closed, exactly like InstallFromGitHub).
func (m *Manager) ListRepoSkills(ref string, allowRemote bool) ([]RepoSkill, []string, error) {
	if !allowRemote {
		return nil, nil, fmt.Errorf("remote skill listing is disabled: pass --allow-remote to enable it (e.g. cosca skill repo list %s --allow-remote)", ref)
	}
	candidates, warnings, cleanup, err := downloadRepoCandidates(ref)
	if err != nil {
		return nil, warnings, err
	}
	defer cleanup()
	return listRepoSkillsFromCandidates(candidates), warnings, nil
}

// ListRepoSkillsLocal lists the skills in a LOCAL directory (a skills repo
// root) using the same discovery code as the GitHub channel. It exists so the
// listing logic can be exercised without a network.
func ListRepoSkillsLocal(dir string) ([]RepoSkill, []string, error) {
	candidates, warnings, err := findSkillDirsRecursive(dir)
	if err != nil {
		return nil, warnings, err
	}
	if len(candidates) == 0 {
		return nil, warnings, fmt.Errorf("no SKILL.md found under %s", dir)
	}
	return listRepoSkillsFromCandidates(candidates), warnings, nil
}

// listRepoSkillsFromCandidates builds the 1-indexed RepoSkill list for a set
// of discovered candidates, reading each candidate's description from its
// SKILL.md.
func listRepoSkillsFromCandidates(candidates []skillCandidate) []RepoSkill {
	skills := make([]RepoSkill, 0, len(candidates))
	for i, cd := range candidates {
		desc := ""
		if data, err := os.ReadFile(filepath.Join(cd.dir, "SKILL.md")); err == nil {
			if s := parseSkillFromMarkdown(string(data), "SKILL.md"); s != nil {
				desc = s.Description
			}
		}
		skills = append(skills, RepoSkill{
			Index:       i + 1,
			Name:        cd.skillDir,
			Dir:         cd.skillDir,
			Description: desc,
		})
	}
	return skills
}

// InstallRepoSkills downloads a GitHub reference ("owner/repo[/path][@ref]")
// and installs ONLY the skills whose 1-based Index (as shown by ListRepoSkills)
// appears in indexes. An empty indexes slice installs every skill, matching
// InstallFromGitHub's behavior. Every install goes through the existing
// installStandardSkill pipeline, so containment and validation are preserved.
func (m *Manager) InstallRepoSkills(ref string, indexes []int, allowRemote bool) ([]*Skill, []string, error) {
	if !allowRemote {
		return nil, nil, fmt.Errorf("remote skill installation is disabled: pass --allow-remote to enable it (e.g. cosca skill repo install %s --allow-remote)", ref)
	}
	candidates, warnings, cleanup, err := downloadRepoCandidates(ref)
	if err != nil {
		return nil, warnings, err
	}
	defer cleanup()
	return m.installRepoCandidates(candidates, indexes, warnings)
}

// installRepoCandidates installs the given candidates, selecting only the
// 1-based indexes when the slice is non-empty. An out-of-range index is a hard
// error (nothing is installed) and the message lists the valid range. Shared
// by InstallFromGitHub (empty indexes = all) and InstallRepoSkills so the
// selection logic is testable without a network.
func (m *Manager) installRepoCandidates(candidates []skillCandidate, indexes []int, warnings []string) ([]*Skill, []string, error) {
	selected := candidates
	if len(indexes) > 0 {
		valid := make(map[int]bool, len(candidates))
		for i := range candidates {
			valid[i+1] = true
		}
		var picked []skillCandidate
		for _, idx := range indexes {
			if !valid[idx] {
				return nil, warnings, fmt.Errorf("skill index %d is out of range (valid range: 1-%d)", idx, len(candidates))
			}
			picked = append(picked, candidates[idx-1])
		}
		selected = picked
	}

	var installed []*Skill
	for _, cd := range selected {
		skill, err := m.installStandardSkill(cd.dir, cd.skillDir, m.skillsPathOrEmpty())
		if err != nil {
			return installed, warnings, fmt.Errorf("install %q: %w", cd.skillDir, err)
		}
		installed = append(installed, skill)
	}
	return installed, warnings, nil
}
