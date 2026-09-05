package skills

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildGitHubQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		query string
		opts  CatalogSearchOptions
		want  string
	}{
		{
			name:  "plain query plus topic qualifier",
			query: "gke",
			want:  "gke in:readme topic:skills",
		},
		{
			name:  "org qualifier",
			query: "gke",
			opts:  CatalogSearchOptions{Owner: "google"},
			want:  "gke in:readme topic:skills org:google",
		},
		{
			name:  "min stars",
			query: "gke",
			opts:  CatalogSearchOptions{MinStars: 100},
			want:  "gke in:readme topic:skills stars:>100",
		},
		{
			name:  "org plus min stars",
			query: "gke",
			opts:  CatalogSearchOptions{Owner: "google", MinStars: 50},
			want:  "gke in:readme topic:skills org:google stars:>50",
		},
		{
			name:  "shell injection stripped",
			query: "; rm -rf /;",
			want:  "rm -rf / in:readme topic:skills",
		},
		{
			name:  "injection with pipes and quotes",
			query: "gke | cat /etc/passwd",
			want:  "gke cat /etc/passwd in:readme topic:skills",
		},
		{
			name:  "empty query still carries qualifier",
			query: "",
			want:  " in:readme topic:skills",
		},
	}

	for _, c := range cases {
		got := buildGitHubQuery(c.query, c.opts)
		if got != c.want {
			t.Errorf("%s: buildGitHubQuery(%q, %+v) = %q, want %q", c.name, c.query, c.opts, got, c.want)
		}
	}
}

func TestSanitizeCatalogQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "; rm -rf /", want: "rm -rf /"},
		{in: "gke; drop table skills;", want: "gke drop table skills"},
		{in: "a&b|c$d`e", want: "abcde"},
		{in: "stars:>10", want: "stars:10"},
		{in: "  padded  ", want: "padded"},
		{in: "org:google", want: "org:google"},
	}
	for _, c := range cases {
		if got := sanitizeCatalogQuery(c.in); got != c.want {
			t.Errorf("sanitizeCatalogQuery(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRepoSlug(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{in: "google/skills", want: "skills"},
		{in: "gemini-cli-extensions/alloydb", want: "alloydb"},
		{in: "single", want: "single"},
		{in: "", want: ""},
	}
	for _, c := range cases {
		if got := repoSlug(c.in); got != c.want {
			t.Errorf("repoSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSearchCatalogRequiresAllowRemote(t *testing.T) {
	t.Parallel()

	m := &Manager{coscaDir: t.TempDir()}
	_, err := m.SearchCatalog("gke", CatalogSearchOptions{})
	if err == nil {
		t.Fatal("expected error when AllowRemote is false, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-remote") {
		t.Errorf("error should mention --allow-remote, got: %v", err)
	}
}

func TestSearchCatalogEmptyQuery(t *testing.T) {
	t.Parallel()

	m := &Manager{coscaDir: t.TempDir()}
	if _, err := m.SearchCatalog("   ", CatalogSearchOptions{AllowRemote: true}); err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

// githubSearchFixture mirrors the GitHub Search API repositories response for
// the topic:skills qualifier.
const githubSearchFixture = `{
	"total_count": 1234,
	"incomplete_results": false,
	"items": [
		{
			"id": 1,
			"full_name": "google/skills",
			"html_url": "https://github.com/google/skills",
			"description": "Google's collection of Agent Skills",
			"stargazers_count": 9100,
			"updated_at": "2026-08-01T00:00:00Z"
		},
		{
			"id": 2,
			"full_name": "anthropics/skills",
			"html_url": "https://github.com/anthropics/skills",
			"description": "Anthropic's Agent Skills collection",
			"stargazers_count": 5200,
			"updated_at": "2026-07-20T00:00:00Z"
		}
	]
}`

// githubSearchHandler builds an httptest server that records the received
// query and returns the fixture (or a configurable status code).
func githubSearchHandler(t *testing.T, fixture string, status int) (*httptest.Server, *string, *string) {
	t.Helper()
	var gotQuery, gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotRawQuery = r.URL.RawQuery
		if status != http.StatusOK {
			http.Error(w, fixture, status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	return srv, &gotQuery, &gotRawQuery
}

// withCatalogAPIBase repoints catalogAPIBase at an httptest server for the
// duration of the test. It MUST NOT run in parallel with other tests that
// touch catalogAPIBase (package-level state).
func withCatalogAPIBase(t *testing.T, base string) {
	t.Helper()
	old := catalogAPIBase
	catalogAPIBase = base
	t.Cleanup(func() { catalogAPIBase = old })
}

func TestSearchCatalogParsesResponse(t *testing.T) {
	srv, gotQuery, _ := githubSearchHandler(t, githubSearchFixture, http.StatusOK)
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}
	result, err := m.SearchCatalog("gke", CatalogSearchOptions{AllowRemote: true})
	if err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}

	// The received query must carry the topic qualifier (URL-decoded) and no
	// stray shell metacharacters.
	if *gotQuery != "gke in:readme topic:skills" {
		t.Errorf("received q = %q, want %q", *gotQuery, "gke in:readme topic:skills")
	}

	if result.Query != "gke" {
		t.Errorf("Query = %q, want gke", result.Query)
	}
	if result.Count != 1234 {
		t.Errorf("Count = %d, want 1234", result.Count)
	}
	if len(result.Skills) != 2 {
		t.Fatalf("Skills = %d, want 2", len(result.Skills))
	}

	first := result.Skills[0]
	if first.ID != "google/skills" || first.Slug != "skills" || first.Name != "google/skills" {
		t.Errorf("first catalog skill identity = %+v", first)
	}
	if first.Description != "Google's collection of Agent Skills" {
		t.Errorf("first.Description = %q", first.Description)
	}
	if first.Installs != 9100 {
		t.Errorf("first.Installs = %d, want 9100", first.Installs)
	}
	if first.URL != "https://github.com/google/skills" {
		t.Errorf("first.URL = %q", first.URL)
	}
}

func TestSearchCatalogSanitizesAndEncodesQuery(t *testing.T) {
	srv, gotQuery, gotRawQuery := githubSearchHandler(t, githubSearchFixture, http.StatusOK)
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}
	_, err := m.SearchCatalog("; rm -rf /", CatalogSearchOptions{AllowRemote: true})
	if err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}
	// The decoded query has the shell metacharacter stripped and no leftover
	// payload separator; the raw query carries no literal spaces or semicolons
	// (fully URL-encoded).
	if *gotQuery != "rm -rf / in:readme topic:skills" {
		t.Errorf("decoded q = %q, want sanitized %q", *gotQuery, "rm -rf / in:readme topic:skills")
	}
	if strings.Contains(*gotRawQuery, ";") || strings.Contains(*gotRawQuery, " ") {
		t.Errorf("raw query must be fully encoded, got: %q", *gotRawQuery)
	}
}

func TestSearchCatalogOwnerQualifier(t *testing.T) {
	srv, gotQuery, _ := githubSearchHandler(t, githubSearchFixture, http.StatusOK)
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}
	if _, err := m.SearchCatalog("gke", CatalogSearchOptions{Owner: "google", AllowRemote: true}); err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}
	if *gotQuery != "gke in:readme topic:skills org:google" {
		t.Errorf("received q = %q, want org qualifier", *gotQuery)
	}
}

func TestSearchCatalogLimitApplied(t *testing.T) {
	var gotPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_count": 0, "items": []}`))
	}))
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}

	// Default limit when unset.
	if _, err := m.SearchCatalog("gke", CatalogSearchOptions{AllowRemote: true}); err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}
	if gotPerPage != "10" {
		t.Errorf("default per_page = %q, want 10", gotPerPage)
	}

	// Cap at 50.
	if _, err := m.SearchCatalog("gke", CatalogSearchOptions{Limit: 500, AllowRemote: true}); err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}
	if gotPerPage != "50" {
		t.Errorf("capped per_page = %q, want 50", gotPerPage)
	}

	// Explicit limit honored.
	if _, err := m.SearchCatalog("gke", CatalogSearchOptions{Limit: 3, AllowRemote: true}); err != nil {
		t.Fatalf("SearchCatalog error: %v", err)
	}
	if gotPerPage != "3" {
		t.Errorf("explicit per_page = %q, want 3", gotPerPage)
	}
}

func TestSearchCatalogRateLimit(t *testing.T) {
	srv, _, _ := githubSearchHandler(t, `{"message": "API rate limit exceeded"}`, http.StatusForbidden)
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}
	_, err := m.SearchCatalog("gke", CatalogSearchOptions{AllowRemote: true})
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "10 req/min") {
		t.Errorf("rate-limit error should mention the 10/min limit, got: %v", err)
	}
	if !strings.Contains(msg, "--owner") {
		t.Errorf("rate-limit error should suggest --owner, got: %v", err)
	}
}

func TestSearchCatalogMalformedResponse(t *testing.T) {
	srv, _, _ := githubSearchHandler(t, `{not json`, http.StatusOK)
	defer srv.Close()
	withCatalogAPIBase(t, srv.URL)

	m := &Manager{coscaDir: t.TempDir()}
	if _, err := m.SearchCatalog("gke", CatalogSearchOptions{AllowRemote: true}); err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
}

// TestCatalogSkillJSONShape asserts the CatalogSkill JSON tags mirror the
// skills.sh V1Skill shape so catalog consumers can rely on the interchange
// format.
func TestCatalogSkillJSONShape(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(CatalogSkill{
		ID:          "google/skills",
		Slug:        "skills",
		Name:        "google/skills",
		Source:      "google/skills",
		Description: "desc",
		Installs:    9100,
		URL:         "https://github.com/google/skills",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"id", "slug", "name", "source", "description", "installs", "url"} {
		if _, ok := m[key]; !ok {
			t.Errorf("CatalogSkill JSON missing key %q in %s", key, string(b))
		}
	}
}
