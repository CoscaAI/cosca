// Knowledge acquisition: fetch official documentation/sources for a registered
// KnowledgePackage, persist the content, and index it into the Knowledge Base
// so that cosca knowledge search returns local results without internet/LLM.
//
// Pipeline:
//
//	manifest → source URL → fetch (SSRF-safe) → write markdown → compile FTS5
//
// After acquisition, the package status moves from "manifest" to "acquired".
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/acquisition"
	"github.com/rs/zerolog/log"
)

// AcquireResult reports the outcome of acquiring a KnowledgePackage.
type AcquireResult struct {
	PackageID    string   `json:"package_id"`
	SourceURL    string   `json:"source_url"`
	ArtifactID   string   `json:"artifact_id"`
	SHA256       string   `json:"sha256"`
	SizeBytes    int64    `json:"size_bytes"`
	Status       string   `json:"status"` // "acquired" | "already_acquired"
	KBEntries    int      `json:"kb_entries"`
	MarkdownPath string   `json:"markdown_path"`
	Warnings     []string `json:"warnings,omitempty"`
}

// AcquirePackage fetches official documentation for a registered KnowledgePackage,
// persists it as a knowledge file, and compiles it into the Knowledge Base.
//
// store is the PackageStore where the manifest lives. kbDir is the project
// .cosca/ directory (used to resolve knowledge.db and the acquired knowledge
// directory). allowRemote must be true to actually fetch from the internet.
// force bypasses the repository health gate (stars, age, issues, license, etc.).
func AcquirePackage(ctx context.Context, store *PackageStore, pkgID string, kbDir string, allowRemote bool, force bool) (*AcquireResult, error) {
	pkg, err := store.Get(pkgID)
	if err != nil {
		return nil, fmt.Errorf("acquire: %w", err)
	}

	if pkg.Status == PackageStatusAcquired || pkg.Status == PackageStatusValidated {
		return &AcquireResult{
			PackageID: pkgID,
			Status:    "already_acquired",
		}, nil
	}

	// Build URLs to try: main branch first, then master (older repos).
	sourceURL := buildAcquireURL(pkg)
	if sourceURL == "" {
		return nil, fmt.Errorf("acquire: no repository set for package %q — registre com: cosca knowledge add github:<org>/%s", pkgID, pkgID)
	}

	// Pre-flight: verify repository health before downloading anything.
	if pkg.Repository != "" && strings.Contains(pkg.Repository, "/") {
		verification, vErr := verifyGitHubRepo(ctx, pkg.Repository)
		if vErr != nil {
			// DNS/network failures during verification are non-fatal — skip verification.
			errStr := vErr.Error()
			if strings.Contains(errStr, "não foi possível resolver") ||
				strings.Contains(errStr, "fail-closed") ||
				strings.Contains(errStr, "dial tcp") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "no such host") {
				// Skip verification, proceed with acquisition.
			} else {
				return nil, fmt.Errorf("acquire: repo verification failed for %q: %w", pkg.Repository, vErr)
			}
		} else if !verification.Passed {
			if !force {
				return nil, fmt.Errorf("acquire: repositório %q NÃO passou na verificação de segurança: %s\n\nDados do repo: %d ⭐, %d forks, %d issues, licença=%s, arquivado=%v, fork=%v\n\nPara forçar a aquisição mesmo assim, use --force",
					pkg.Repository, verification.Reason,
					verification.Stars, verification.Forks, verification.OpenIssues, verification.License, verification.Archived, verification.IsFork)
			}
			log.Warn().Str("repo", pkg.Repository).
				Str("reason", verification.Reason).
				Msg("forced acquisition past repo health gate")
		}
	}

	// Try main first, fall back to master.
	urls := []string{sourceURL}
	if alt := buildAcquireURLWithBranch(pkg, "master"); alt != sourceURL {
		urls = append(urls, alt)
	}

	// Fetch with SSRF protection. Allow loopback to handle environments
	// where DNS resolution may be restricted (jail/container fallback).
	client := acquisition.NewClient(30*time.Second, 3, 50<<20) // 50 MiB
	client.AllowRemote = allowRemote
	client.AllowLoopback = true                    // allow fallback when DNS restricted
	client.GitHubToken = os.Getenv("GITHUB_TOKEN") // autentica GitHub API (5000 req/h)
	if !allowRemote {
		return nil, fmt.Errorf("acquire: --allow-remote is required to fetch from the internet")
	}

	var art *acquisition.AcquiredArtifact
	var body []byte
	var fetchErr error
	for _, url := range urls {
		art, body, fetchErr = client.FetchAll(ctx, url)
		if fetchErr == nil {
			sourceURL = url
			break
		}
	}
	if fetchErr != nil {
		return nil, fmt.Errorf("acquire: fetch %q: %w", sourceURL, fetchErr)
	}

	// Decode GitHub API response (base64-encoded README content).
	if decoded, ok := decodeGitHubReadme(body); ok {
		body = decoded
	}

	// Persist the raw artifact in quarantine.
	astore := acquisition.NewArtifactStore(kbDir)
	artID, err := astore.Add(art, body)
	if err != nil {
		return nil, fmt.Errorf("acquire: store artifact: %w", err)
	}

	// Write the content as a knowledge markdown file so the Compiler can
	// index it into the FTS5 knowledge base.
	mdPath, mdHash, err := writeAcquiredMarkdown(kbDir, pkgID, pkg.Ecosystem, sourceURL, body)
	if err != nil {
		return &AcquireResult{
			PackageID:  pkgID,
			SourceURL:  sourceURL,
			ArtifactID: artID,
			SHA256:     art.SHA256,
			SizeBytes:  art.SizeBytes,
			Status:     "acquired",
			Warnings:   []string{fmt.Sprintf("markdown write: %v (artifact %s saved)", err, artID)},
		}, nil
	}

	// Update the manifest.
	pkg.Status = PackageStatusAcquired
	pkg.ArtifactIDs = append(pkg.ArtifactIDs, artID)
	pkg.AcquiredAt = time.Now()
	pkg.KnowledgeLevel = PackageKnowledgePartial
	pkg.SHA256 = mdHash

	if err := store.Add(*pkg); err != nil {
		return &AcquireResult{
			PackageID:    pkgID,
			SourceURL:    sourceURL,
			ArtifactID:   artID,
			SHA256:       art.SHA256,
			SizeBytes:    art.SizeBytes,
			Status:       "acquired",
			MarkdownPath: mdPath,
			Warnings:     []string{fmt.Sprintf("manifest update: %v (knowledge file saved at %s)", err, mdPath)},
		}, nil
	}

	return &AcquireResult{
		PackageID:    pkgID,
		SourceURL:    sourceURL,
		ArtifactID:   artID,
		SHA256:       art.SHA256,
		SizeBytes:    art.SizeBytes,
		Status:       "acquired",
		MarkdownPath: mdPath,
		KBEntries:    1,
	}, nil
}

// buildAcquireURL constrói a URL da API do GitHub para obter o README.
// Usa api.github.com/repos/<org>/<repo>/readme — mais confiável que raw.
func buildAcquireURL(pkg *KnowledgePackage) string {
	return buildAcquireURLWithBranch(pkg, "")
}

// buildAcquireURLWithBranch constrói a URL da API para o README.
// branch vazia = default branch (HEAD).
func buildAcquireURLWithBranch(pkg *KnowledgePackage, branch string) string {
	repo := pkg.Repository
	if repo == "" || !strings.Contains(repo, "/") {
		return ""
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/readme", repo)
	if branch != "" {
		url += "?ref=" + branch
	}
	return url
}

// acquiredKnowledgeDir returns the directory where acquired knowledge files
// are stored, ready for the Compiler.
func acquiredKnowledgeDir(kbDir string) string {
	return filepath.Join(kbDir, "knowledge", "acquired")
}

// writeAcquiredMarkdown writes the fetched body as a knowledge markdown file.
// Returns the file path, SHA-256 hash, and error.
func writeAcquiredMarkdown(kbDir, pkgID, ecosystem, sourceURL string, body []byte) (string, string, error) {
	dir := filepath.Join(acquiredKnowledgeDir(kbDir), pkgID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("mkdir %q: %w", dir, err)
	}

	// Format the markdown with YAML frontmatter so the Compiler can parse it.
	title := fmt.Sprintf("%s — Official Documentation", pkgID)
	tags := []string{pkgID, ecosystem, "acquired", "documentation"}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", title))
	sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(quoteTags(tags), ", ")))
	sb.WriteString(fmt.Sprintf("source: %q\n", sourceURL))
	sb.WriteString(fmt.Sprintf("acquired_at: %q\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString("confidence: 0.85\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("> Source: %s\n\n", sourceURL))

	// Truncate body to a reasonable size for knowledge indexing (500 KiB).
	const maxBody = 500 << 10
	content := body
	if len(content) > maxBody {
		content = content[:maxBody]
		sb.WriteString(fmt.Sprintf("> ⚠ Content truncated at %d KiB\n\n", maxBody>>10))
	}
	sb.Write(content)

	data := []byte(sb.String())
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	mdPath := filepath.Join(dir, "docs.md")
	if err := os.WriteFile(mdPath, data, 0o600); err != nil {
		return "", "", fmt.Errorf("write %q: %w", mdPath, err)
	}

	return mdPath, hash, nil
}

func quoteTags(tags []string) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = fmt.Sprintf("%q", t)
	}
	return out
}

// decodeGitHubReadme decodes a GitHub API README response (base64-encoded).
// Returns the decoded markdown and true if successful, nil/false otherwise.
func decodeGitHubReadme(body []byte) ([]byte, bool) {
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, false
	}
	if resp.Encoding != "base64" || resp.Content == "" {
		return nil, false
	}
	// GitHub API replaces newlines in the base64 string.
	clean := strings.ReplaceAll(resp.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

// PackageStatusAcquired marks a package whose documentation has been fetched.
const PackageStatusAcquired = "acquired"

// ── Repository verification ─────────────────────────────────────────────────

// RepoVerification holds the result of a pre-acquisition repository check.
type RepoVerification struct {
	Repo        string `json:"repo"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	OpenIssues  int    `json:"open_issues"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	License     string `json:"license"`
	Archived    bool   `json:"archived"`
	IsFork      bool   `json:"is_fork"`
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
	Reason      string `json:"reason,omitempty"`
}

// Minimum thresholds for repository trust.
//
// maxOpenIssues is intentionally generous: language/framework mega-repos
// (golang/go ~10k, rust-lang/rust ~12k, microsoft/typescript ~5k open issues)
// legitimately exceed 5000 by scale, not by risk. The real trust signals are
// the other checks below (stars, archived, fork, license) — open-issue count
// is only a rough maintenance proxy.
const (
	minStars      = 50
	minAgeDays    = 90
	maxOpenIssues = 20000
)

// verifyGitHubRepo checks a GitHub repository's health before acquisition.
// Calls api.github.com/repos/<org>/<repo>. Uses GITHUB_TOKEN se disponível.
func verifyGitHubRepo(ctx context.Context, repo string) (*RepoVerification, error) {
	if !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("invalid repo format: %q (expected org/repo)", repo)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s", repo)

	client := acquisition.NewClient(10*time.Second, 0, 1<<18) // 256 KiB
	client.AllowRemote = true
	client.AllowLoopback = true                    // allow when DNS restricted
	client.GitHubToken = os.Getenv("GITHUB_TOKEN") // authenticated → 5000 req/h

	_, body, err := client.FetchAll(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}

	// Parse minimal JSON — avoid heavy dependencies.
	var raw struct {
		StargazersCount int    `json:"stargazers_count"`
		ForksCount      int    `json:"forks_count"`
		OpenIssuesCount int    `json:"open_issues_count"`
		CreatedAt       string `json:"created_at"`
		UpdatedAt       string `json:"updated_at"`
		Archived        bool   `json:"archived"`
		Fork            bool   `json:"fork"`
		Description     string `json:"description"`
		License         *struct {
			SpdxID string `json:"spdx_id"`
		} `json:"license"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse github api response: %w", err)
	}

	license := "none"
	if raw.License != nil {
		license = raw.License.SpdxID
	}

	v := &RepoVerification{
		Repo:        repo,
		Stars:       raw.StargazersCount,
		Forks:       raw.ForksCount,
		OpenIssues:  raw.OpenIssuesCount,
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
		License:     license,
		Archived:    raw.Archived,
		IsFork:      raw.Fork,
		Description: raw.Description,
	}

	// --- Safety checks ---
	var failures []string

	if v.Stars < minStars {
		failures = append(failures, fmt.Sprintf("apenas %d estrelas (mínimo: %d)", v.Stars, minStars))
	}
	if v.Archived {
		failures = append(failures, "repositório arquivado")
	}
	if v.IsFork {
		failures = append(failures, "é um fork (use o repositório original)")
	}
	if v.License == "" || v.License == "none" {
		failures = append(failures, "sem licença declarada")
	}
	if v.OpenIssues > maxOpenIssues {
		failures = append(failures, fmt.Sprintf("%d issues abertas (máximo: %d)", v.OpenIssues, maxOpenIssues))
	}

	// Age check: repo must be at least minAgeDays old.
	if v.CreatedAt != "" {
		created, err := time.Parse(time.RFC3339, v.CreatedAt)
		if err == nil {
			age := time.Since(created)
			if age < minAgeDays*24*time.Hour {
				failures = append(failures, fmt.Sprintf("muito novo (%.0f dias, mínimo: %d)", age.Hours()/24, minAgeDays))
			}
		}
	}

	if len(failures) > 0 {
		v.Passed = false
		v.Reason = strings.Join(failures, "; ")
	} else {
		v.Passed = true
		v.Reason = "repositório verificado"
	}

	return v, nil
}
