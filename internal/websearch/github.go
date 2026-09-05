package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GitHubProvider busca no GitHub — repositórios e issues (problemas reais).
//
// O GitHub é a camada de investigação de SOFTWARE: "que problemas as pessoas
// enfrentam com X?" via issues, e "existe solução/ferramenta para Y?" via repos.
// A API pública é gratuita e não exige token para busca (limite ~60/h por IP;
// um token opcional COSCA_GITHUB_TOKEN eleva para 5000/h).
//
// `kind` seleciona o tipo: "repos" (default), "issues", "codes".
type GitHubProvider struct {
	token string
	kind  string
}

// NewGitHubProvider cria o provedor de GitHub, lendo COSCA_GITHUB_TOKEN do env.
func NewGitHubProvider(kind string) *GitHubProvider {
	if kind == "" {
		kind = "repos"
	}
	return &GitHubProvider{token: os.Getenv("COSCA_GITHUB_TOKEN"), kind: kind}
}

// Name implementa Provider.
func (p *GitHubProvider) Name() string { return "github:" + p.kind }

// Search consulta a GitHub API de busca para o tipo configurado.
func (p *GitHubProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}

	switch p.kind {
	case "issues":
		q := fmt.Sprintf("%s is:issue", query)
		raw := fmt.Sprintf("https://api.github.com/search/issues?q=%s&sort=comments&order=desc&per_page=%d", encodeQuery(q), limit)
		body, err := p.getJSON(ctx, raw)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Items []struct {
				Title   string `json:"title"`
				Body    string `json:"body"`
				HTMLURL string `json:"html_url"`
				RepoURL string `json:"repository_url"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("github issues parse: %w", err)
		}
		results := make([]Result, 0, len(resp.Items))
		for _, it := range resp.Items {
			repo := strings.TrimPrefix(it.RepoURL, "https://api.github.com/repos/")
			results = append(results, Result{
				Title:   trimSnippet(it.Title, 200),
				Snippet: trimSnippet(it.Body, 300),
				Link:    it.HTMLURL,
				Source:  "github:" + repo,
			})
		}
		return results, nil

	case "codes":
		raw := fmt.Sprintf("https://api.github.com/search/code?q=%s&per_page=%d", encodeQuery(query), limit)
		body, err := p.getJSON(ctx, raw)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Items []struct {
				Name    string `json:"name"`
				Path    string `json:"path"`
				HTMLURL string `json:"html_url"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("github code parse: %w", err)
		}
		results := make([]Result, 0, len(resp.Items))
		for _, it := range resp.Items {
			results = append(results, Result{
				Title:   trimSnippet(it.Name, 200),
				Snippet: trimSnippet(it.Path, 300),
				Link:    it.HTMLURL,
				Source:  "github:code",
			})
		}
		return results, nil

	default: // repos
		raw := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=%d", encodeQuery(query), limit)
		body, err := p.getJSON(ctx, raw)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Items []struct {
				FullName    string `json:"full_name"`
				Description string `json:"description"`
				HTMLURL     string `json:"html_url"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("github repos parse: %w", err)
		}
		results := make([]Result, 0, len(resp.Items))
		for _, it := range resp.Items {
			results = append(results, Result{
				Title:   trimSnippet(it.FullName, 200),
				Snippet: trimSnippet(it.Description, 300),
				Link:    it.HTMLURL,
				Source:  "github:repos",
			})
		}
		return results, nil
	}
}

// getJSON faz um GET autenticado (quando há token) e devolve o corpo JSON.
func (p *GitHubProvider) getJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github fetch: status %d (token ausente? rate limit?)", resp.StatusCode)
	}
	return readBody(resp)
}
