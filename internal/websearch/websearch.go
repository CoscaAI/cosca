// Package websearch — busca de informação NA WEB sem API key e sem custo.
//
// Doutrina (Don, 2026-09-02): ferramenta de criação de software precisa investigar
// a realidade — validar notícia (fake ou não), descobrir problemas que pessoas
// enfrentam, e raciocinar sobre a informação. Para isso a busca roda sobre
// fontes PÚBLICAS LEGÍTIMAS (RSS de agências, Wikipedia, Hacker News, GitHub),
// em vez de depender de um "Google grátis" que não existe de forma confiável
// (SearXNG → 429/403, DuckDuckGo scrape → captcha).
//
// Cada provedor implementa a interface Provider e retorna resultados NÃO-confiáveis
// (conteúdo externo) — o chamador (CLI, agente) é quem triangula e verifica.
package websearch

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Result é um item de resultado de busca. Fonte externa (NÃO-confiável): o
// consumidor não deve tratar título/conteúdo como fato, apenas como evidência.
type Result struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
	Source  string `json:"source"`
}

// Provider é a interface de um provedor de busca. Cada fonte pública legítima
// implementa Search(ctx, query, limit).
type Provider interface {
	// Name identifica o provedor (ex: "news", "wiki", "hn", "github").
	Name() string
	// Search executa a busca na fonte e devolve os resultados. Devolve erro
	// claro quando a fonte está indisponível/bloqueada — nunca silencioso.
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// defaultHTTPClient é o cliente HTTP compartilhado, com timeout curto para
// nunca travar uma busca. Reutilizado por todos os provedores.
var defaultHTTPClient = &http.Client{Timeout: 20 * time.Second}

// userAgent é enviado a todas as fontes para ser tratado como cliente legítimo.
// Muitas fontes bloqueiam UAs vazios ou de ferramentas automatizadas conhecidas.
const userAgent = "cosca-websearch/1.0 (+https://cosca.enterprise)"

// get faz um GET simples com UA e retorna o corpo. Timeout e erros de rede são
// retornados para o provedor decidir (melhor falhar claro que fingir sucesso).
func get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// 429/403/etc — a fonte está bloqueando ou com rate-limit. Retorna o
		// status para o provedor reportar honestamente.
		return nil, fmt.Errorf("fetch %s: unexpected status %d", rawURL, resp.StatusCode)
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// readBody lê o corpo da resposta com um teto de 4 MiB (evita resposta gigante
// de um endpoint de busca fora do controle). Reutilizado por todos os provedores.
func readBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// trimSnippet limita um snippet a um tamanho legível e remove quebras de linha.
func trimSnippet(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

// encodeQueryURL-encoda a query para uso em querystring.
func encodeQuery(q string) string {
	return url.QueryEscape(q)
}

// ── RSS genérico (usado pelo provedor news) ────────────────────────────────

// rssFeed é a estrutura XML mínima de um feed RSS 2.0.
type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
}

// parseRSS decodifica um feed RSS e converte os itens em Results. A descrição
// RSS costuma trazer HTML — por isso passamos por stripHTML antes de exibir.
func parseRSS(data []byte, source string, limit int) ([]Result, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}
	items := feed.Channel.Items
	if len(items) > limit {
		items = items[:limit]
	}
	results := make([]Result, 0, len(items))
	for _, it := range items {
		results = append(results, Result{
			Title:   trimSnippet(it.Title, 200),
			Snippet: trimSnippet(stripHTML(it.Description), 300),
			Link:    strings.TrimSpace(it.Link),
			Source:  source,
		})
	}
	return results, nil
}

// stripHTML remove tags HTML de um snippet RSS. Implementação leve (regex não é
// ideal para HTML completo, mas suficiente para descrições de feed).
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case inTag == false:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
