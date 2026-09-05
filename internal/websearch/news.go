package websearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// NewsProvider busca notícias via RSS de agências/fontes legítimas.
//
// O caso de uso é investigação: "saiu uma notícia, é verdade ou fake?". A fonte
// primária (o que a agência publicou) é a primeira camada de verificação — antes
// de qualquer busca genérica. RSS é gratuito, sem chave, e não bloqueia um
// cliente legítimo com User-Agent definido.
//
// As fontes são configuráveis (feeds por site), mas vem um conjunto default que
// cobre agências de credibilidade. Se uma fonte falhar (site bloqueou ou mudou o
// feed), ela é omitida — as restantes continuam — mas o provedor reporta o que
// conseguiu em vez de fingir sucesso.
type NewsProvider struct {
	// feeds mapeia site -> URL do feed RSS.
	feeds map[string]string
}

// NewNewsProvider cria o provedor de notícias com as fontes default.
func NewNewsProvider() *NewsProvider {
	return &NewsProvider{feeds: defaultNewsFeeds()}
}

// defaultNewsFeeds retorna o conjunto de feeds RSS default (agências e portais
// de credibilidade, em pt-BR e internacional). O RSS não exige chave e é o
// caminho estável para notícia de verdade.
func defaultNewsFeeds() map[string]string {
	return map[string]string{
		"G1 Brasil":     "https://g1.globo.com/rss/g1/",
		"UOL":           "https://rss.uol.com.br/feed/noticias.xml",
		"BBC World":     "https://feeds.bbci.co.uk/news/world/rss.xml",
		"BBC Tech":      "https://feeds.bbci.co.uk/news/technology/rss.xml",
		"Reuters Tech":  "https://feeds.reuters.com/reuters/technologyNews",
		"Reuters World": "https://feeds.reuters.com/reuters/worldNews",
	}
}

// Name implementa Provider.
func (p *NewsProvider) Name() string { return "news" }

// Search busca o termo em cada feed. Como RSS não tem busca de texto livre, o
// provedor: (1) lê os últimos itens de cada feed; (2) filtra por relevância
// (título/descrição contém o termo); (3) ordena por proximidade. Se o termo não
// aparece em nenhum item recente, retorna os mais recentes mesmo assim — notícia
// é sobre o que está acontecendo, e o termo pode ser a angulação do consumidor.
func (p *NewsProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	// Coleciona resultados de todas as fontes que responderem; fontes que
	// falham são ignoradas (best-effort), mas nunca fingem sucesso em conjunto.
	var all []Result
	var failed []string
	for site, feed := range p.feeds {
		body, err := get(ctx, feed)
		if err != nil {
			failed = append(failed, site)
			continue
		}
		// Lê o feed inteiro (alguns feeds têm poucos itens), depois filtra.
		feedResults, err := parseRSS(body, site, 50)
		if err != nil {
			failed = append(failed, site)
			continue
		}
		all = append(all, feedResults...)
	}

	// Se nenhuma fonte respondeu, é um estado real de falha — avisamos.
	if len(all) == 0 {
		return nil, fmt.Errorf("news: todas as fontes falharam (%s)", strings.Join(failed, ", "))
	}

	// Classifica por relevância textual ao termo (busca simples nas palavras).
	q := strings.ToLower(query)
	scored := make([]Result, len(all))
	copy(scored, all)
	sort.SliceStable(scored, func(i, j int) bool {
		return relevanceScore(scored[i], q) > relevanceScore(scored[j], q)
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// relevanceScore dá uma nota simples de quanto um resultado se alinha ao termo:
// título contém o termo vale mais que só a descrição. Usado para ordenar.
func relevanceScore(r Result, q string) int {
	score := 0
	if strings.Contains(strings.ToLower(r.Title), q) {
		score += 3
	}
	if strings.Contains(strings.ToLower(r.Snippet), q) {
		score += 1
	}
	return score
}
