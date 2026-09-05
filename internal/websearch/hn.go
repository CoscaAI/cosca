package websearch

import (
	"context"
	"encoding/json"
	"fmt"
)

// HNProvider busca no Hacker News via Algolia API (gratuita, sem chave).
//
// O Hacker News é a terceira camada de investigação: "o que a comunidade tech
// está discutindo". Útil para validar se um problema/tendência é real (se devs
// reclamam, há substância) e para achar problemas concretos que pessoas
// enfrentam. A Algolia Search API do HN é aberta e não exige chave.
type HNProvider struct{}

// NewHNProvider cria o provedor de Hacker News.
func NewHNProvider() *HNProvider { return &HNProvider{} }

// Name implementa Provider.
func (p *HNProvider) Name() string { return "hn" }

// hnSearchResponse é a resposta da Algolia API do HN.
type hnSearchResponse struct {
	Hits []struct {
		Title    string `json:"title"`
		URL      string `json:"url"`
		StoryURL string `json:"story_url"`
		Points   int    `json:"points"`
	} `json:"hits"`
	NbHits int `json:"nbHits"`
}

// Search consulta a Algolia API do HN. Ordena por relevância (padrão da API) e
// inclui os pontos (karma) como proxy de confiança da comunidade.
func (p *HNProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	raw := fmt.Sprintf(
		"https://hn.algolia.com/api/v1/search?query=%s&hitsPerPage=%d&tags=story",
		encodeQuery(query), limit,
	)
	body, err := get(ctx, raw)
	if err != nil {
		return nil, err
	}
	var resp hnSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("hn parse json: %w", err)
	}
	results := make([]Result, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		title := h.Title
		if title == "" {
			title = h.StoryURL
		}
		link := h.URL
		if link == "" {
			link = h.StoryURL
		}
		results = append(results, Result{
			Title:   trimSnippet(title, 200),
			Snippet: fmt.Sprintf("(%d points)", h.Points),
			Link:    link,
			Source:  "hackernews",
		})
	}
	return results, nil
}
