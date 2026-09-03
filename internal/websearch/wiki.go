package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// WikiProvider busca na Wikipedia via API pública (MediaWiki action=query).
//
// A Wikipedia é a segunda camada de investigação: contexto/verificação de fato.
// É uma API legítima, gratuita, sem chave, com anti-bot amigável (basta um
// User-Agent definido). Suporta busca em qualquer idioma (lang default pt).
type WikiProvider struct {
	// lang é o idioma da Wikipedia (ex: "pt", "en"). Vazio = "pt".
	lang string
}

// NewWikiProvider cria o provedor de Wikipedia com idioma pt (padrão).
func NewWikiProvider() *WikiProvider {
	return &WikiProvider{lang: "pt"}
}

// WithLang define o idioma da Wikipedia.
func (p *WikiProvider) WithLang(lang string) *WikiProvider {
	if lang != "" {
		p.lang = lang
	}
	return p
}

// Name implementa Provider.
func (p *WikiProvider) Name() string { return "wiki" }

// wikiSearchResponse é a resposta da API de busca da MediaWiki.
type wikiSearchResponse struct {
	Query struct {
		Search []struct {
			Title  string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"search"`
		SearchInfo struct {
			TotalHits int `json:"totalhits"`
		} `json:"searchinfo"`
	} `json:"query"`
}

// Search consulta a API de busca da Wikipedia e devolve os títulos+trechos.
func (p *WikiProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	lang := p.lang
	if lang == "" {
		lang = "pt"
	}
	raw := fmt.Sprintf(
		"https://%s.wikipedia.org/w/api.php?action=query&list=search&format=json&srsearch=%s&srlimit=%d",
		lang, encodeQuery(query), limit,
	)
	body, err := get(ctx, raw)
	if err != nil {
		return nil, err
	}
	var resp wikiSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("wiki parse json: %w", err)
	}
	results := make([]Result, 0, len(resp.Query.Search))
	for _, s := range resp.Query.Search {
		results = append(results, Result{
			Title:   trimSnippet(s.Title, 200),
			Snippet: trimSnippet(stripHTML(s.Snippet), 300),
			// Link canônico: /wiki/{título com espaço→underscore}.
			Link:   fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, strings.ReplaceAll(s.Title, " ", "_")),
			Source: "wikipedia:" + lang,
		})
	}
	return results, nil
}
