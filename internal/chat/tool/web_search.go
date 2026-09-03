package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/websearch"
)

// WebSearchTool busca informação na web via fontes públicas legítimas.
//
// Antigamente era um stub dependente de SERPER_API_KEY (pago, não implementado).
// Agora roteia pelos provedores gratuitos do pacote internal/websearch — sem chave,
// sem custo, sem ser barrado. É a ponte entre um agente e a investigação de
// realidade: notícia, contexto (wiki), debate (HN), problemas de software (GitHub).
type WebSearchTool struct {
	// provider é o provedor de busca usado. Default: news (notícia).
	provider websearch.Provider
}

// NewWebSearchTool cria a tool de busca web com o provedor dado.
//
// providerName ("news", "wiki", "hn", "github", "github-issues") seleciona a
// fonte. Vazio/desconhecido cai em "news".
func NewWebSearchTool(providerName string) *WebSearchTool {
	return &WebSearchTool{provider: websearchProviderFor(providerName)}
}

// websearchProviderFor resolve o nome do provedor para a implementação.
func websearchProviderFor(name string) websearch.Provider {
	switch strings.ToLower(name) {
	case "wiki":
		return websearch.NewWikiProvider()
	case "hn":
		return websearch.NewHNProvider()
	case "github", "github-repos":
		return websearch.NewGitHubProvider("repos")
	case "github-issues":
		return websearch.NewGitHubProvider("issues")
	case "github-code":
		return websearch.NewGitHubProvider("codes")
	default: // "news" ou qualquer outro
		return websearch.NewNewsProvider()
	}
}

// Name retorna o identificador da tool.
func (t *WebSearchTool) Name() string { return "web_search" }

// Description descreve o que a tool faz.
func (t *WebSearchTool) Description() string {
	return "Search the web using public free sources (news RSS, Wikipedia, Hacker News, GitHub). Input: search query. Output: titled results with URL + snippet."
}

// IsAvailable retorna true — a busca via fontes públicas não exige chave.
// Um provider sempre está disponível; apenas a fonte alvo pode estar bloqueada
// (nesse caso Search devolve erro claro, não resultado fabricado).
func (t *WebSearchTool) IsAvailable() bool { return true }

// Execute realiza a busca e formata os resultados como texto.
func (t *WebSearchTool) Execute(ctx context.Context, input string) (string, error) {
	// Input é a query; o provider já está fixado na construção da tool.
	results, err := t.provider.Search(ctx, input, 5)
	if err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("web %s — %q\n\n", t.provider.Name(), input))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.Snippet, r.Link))
	}
	return b.String(), nil
}

// SearchResult is a single web search result.
type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
}
