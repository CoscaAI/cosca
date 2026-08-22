// Package skills — public catalog search.
//
// The Google Agent Skills ecosystem (agentskills.io) catalogs skills on
// skills.sh, but that API requires a Vercel OIDC Bearer token (401 without it),
// so it is not a viable unauthenticated channel. Our public search channel is
// the GitHub Search API (repositories with the topic:skills qualifier), which
// works unauthenticated at 10 requests/minute:
//
//	GET https://api.github.com/search/repositories?q=<query> topic:skills&per_page=<n>
//
// SearchCatalog is fail-closed exactly like the GitHub install channel: it
// requires AllowRemote so a bare manager can never trigger a surprise network
// fetch. The HTTP client mirrors install_remote.go's hardened posture (10s
// timeout, 3-redirect cap) and always sends a User-Agent (GitHub returns 403
// without one).
package skills

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// catalogAPIBase is the GitHub REST API base URL used by SearchCatalog. It is
// a package-level variable so tests can point it at an httptest.Server.
var catalogAPIBase = "https://api.github.com"

// CatalogSkill is a skill discovered in the public catalog (GitHub repository
// search, topic:skills). The JSON tags mirror the skills.sh V1Skill shape,
// which is the ecosystem's common interchange format.
type CatalogSkill struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Installs    int    `json:"installs"`
	URL         string `json:"url"`
}

// CatalogSearchResult is the outcome of a catalog search.
type CatalogSearchResult struct {
	Query  string         `json:"query"`
	Count  int            `json:"count"`
	Skills []CatalogSkill `json:"skills"`
}

// CatalogSearchOptions controls a catalog search.
type CatalogSearchOptions struct {
	// Owner restricts the search to a GitHub org or user (org:<owner>).
	Owner string
	// Limit caps the number of results returned (default 10, hard cap 50).
	Limit int
	// MinStars only returns repositories with at least this many stars.
	MinStars int
	// AllowRemote opts into the network fetch (fail-closed otherwise).
	AllowRemote bool
}

// catalogLimitDefault is the default number of catalog results per search.
const catalogLimitDefault = 10

// catalogLimitCap is the hard ceiling for catalog results per search.
const catalogLimitCap = 50

// sanitizeCatalogQuery strips shell metacharacters and other syntax that could
// be interpreted as extra search qualifiers or a shell payload, keeping the
// query a plain-text term. URL-encoding alone would defeat shell injection;
// stripping keeps the built query deterministic and the qualifiers unambiguous.
func sanitizeCatalogQuery(q string) string {
	var b strings.Builder
	for _, r := range q {
		switch r {
		case ';', '|', '&', '$', '`', '>', '<', '\'', '"', '(', ')', '\n', '\r', '\t':
			continue
		default:
			b.WriteRune(r)
		}
	}
	// Collapse whitespace runs so a stripped metacharacter never leaves a
	// double space in the query.
	return strings.Join(strings.Fields(b.String()), " ")
}

// buildGitHubQuery renders the GitHub Search API "q" parameter for a catalog
// search: the sanitized query plus the in:readme and topic:skills qualifiers,
// an optional org:<owner> qualifier, and an optional stars:>N floor.
//
// in:readme matters: GitHub repository search does NOT index a repo's skill
// contents by default, so a bare term like "gke" would miss google/skills
// (whose README lists the GKE skills). Combining the term with in:readme and
// topic:skills keeps the results scoped to skill repositories while matching
// the descriptions that live in their READMEs.
func buildGitHubQuery(query string, opts CatalogSearchOptions) string {
	var b strings.Builder
	b.WriteString(sanitizeCatalogQuery(query))
	b.WriteString(" in:readme topic:skills")
	if opts.Owner != "" {
		b.WriteString(" org:")
		b.WriteString(opts.Owner)
	}
	if opts.MinStars > 0 {
		b.WriteString(" stars:>")
		b.WriteString(strconv.Itoa(opts.MinStars))
	}
	return b.String()
}

// SearchCatalog queries the GitHub Search API (repositories with the
// topic:skills qualifier) for skills matching query.
//
// SECURITY: the fetch is fail-closed — opts.AllowRemote must be true, mirroring
// InstallFromGitHub. The HTTP client is hardened (10s timeout, 3-redirect cap)
// and always sends a User-Agent header, which the GitHub API requires.
func (m *Manager) SearchCatalog(query string, opts CatalogSearchOptions) (*CatalogSearchResult, error) {
	if !opts.AllowRemote {
		return nil, fmt.Errorf("catalog search requires --allow-remote (network fetch) to query the GitHub Search API")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("catalog search query is required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = catalogLimitDefault
	}
	if limit > catalogLimitCap {
		limit = catalogLimitCap
	}

	params := url.Values{}
	params.Set("q", buildGitHubQuery(query, opts))
	params.Set("per_page", strconv.Itoa(limit))

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, catalogAPIBase+"/search/repositories?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog search request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "cosca")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("catalog search rate-limited by the GitHub Search API (10 req/min unauthenticated): HTTP %d — wait a minute or narrow the search with --owner <owner>", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog search failed: HTTP %d", resp.StatusCode)
	}

	var body struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("parse catalog search response: %w", err)
	}

	result := &CatalogSearchResult{
		Query:  query,
		Count:  body.TotalCount,
		Skills: make([]CatalogSkill, 0, len(body.Items)),
	}
	for _, it := range body.Items {
		result.Skills = append(result.Skills, CatalogSkill{
			ID:          it.FullName,
			Slug:        repoSlug(it.FullName),
			Name:        it.FullName,
			Source:      it.FullName,
			Description: it.Description,
			Installs:    it.Stars,
			URL:         it.HTMLURL,
		})
	}
	return result, nil
}

// repoSlug returns the repository name (the part after the last "/") from a
// full "owner/repo" name.
func repoSlug(fullName string) string {
	if i := strings.LastIndex(fullName, "/"); i >= 0 {
		return fullName[i+1:]
	}
	return fullName
}
