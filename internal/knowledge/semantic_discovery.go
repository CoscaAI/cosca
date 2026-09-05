package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/CoscaAI/cosca/internal/search"
)

// FASE 5.4 — descobrimento semântico ENTRE agentes (cross-agent discovery).
//
// A vantagem competitiva da família: achar conhecimento por SIGNIFICADO através
// de agentes diferentes — não por nome de agente/caminho. Usa o vector search
// (meaning-first, agora com cobertura completa do corpus após a FASE 5.3) e
// anota cada resultado com o agente dono (derivado do path da memória
// .../memory/agent/<nome>/...). Um único console semântico que atravessa a
// família.
type SemanticHit struct {
	DocumentID string  `json:"document_id"`
	Path       string  `json:"path"`
	Agent      string  `json:"agent,omitempty"`
	Title      string  `json:"title,omitempty"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score"`
}

type SemanticDiscoveryResult struct {
	Query          string           `json:"query"`
	Limit          int              `json:"limit"`
	DistinctAgents int              `json:"distinct_agents"`
	Hits           []SemanticHit    `json:"hits"`
	ByAgent        map[string]int   `json:"by_agent"`
}

// SemanticDiscovery busca por significado em TODO o corpus (sem confinar a um
// agente) e retorna os resultados anotados com o agente dono, provando a
// recuperação entre agentes. É o modo "consulta a família inteira por sentido".
func (e *Engine) SemanticDiscovery(ctx context.Context, query string, limit int) (*SemanticDiscoveryResult, error) {
	e.mu.RLock()
	searchEngine := e.search
	e.mu.RUnlock()
	if searchEngine == nil {
		return nil, fmt.Errorf("search engine unavailable (initialized?)")
	}
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if limit <= 0 {
		limit = 20
	}

	params := search.DefaultSearchParams()
	params.Query = query
	params.Limit = limit
	params.EnableFTS = false // meaning-first: vetor lidera
	params.EnableVector = true
	params.EnableGraph = false

	res, err := searchEngine.Search(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("semantic discovery search: %w", err)
	}

	out := &SemanticDiscoveryResult{
		Query:   query,
		Limit:   limit,
		ByAgent: map[string]int{},
	}
	if res == nil {
		return out, nil
	}
	seen := map[string]bool{}
	for _, r := range res.Results {
		agent := agentOf(r.DocumentPath)
		key := r.DocumentID
		if key == "" {
			key = r.DocumentPath
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out.Hits = append(out.Hits, SemanticHit{
			DocumentID: r.DocumentID,
			Path:       r.DocumentPath,
			Agent:      agent,
			Title:      r.Title,
			Snippet:    r.Snippet,
			Score:      r.Score,
		})
		out.ByAgent[agent]++
	}
	out.DistinctAgents = len(out.ByAgent)

	// Ordena por score desc (significado) — estável.
	sort.SliceStable(out.Hits, func(i, j int) bool {
		return out.Hits[i].Score > out.Hits[j].Score
	})
	return out, nil
}

// agentOf deriva o agente dono de um path de memória (.../memory/agent/<nome>/...).
// Vazio quando a origem não é de agente (docs/, embed/knowledge, etc.).
func agentOf(path string) string {
	parts := strings.Split(slashPath(path), "/memory/agent/")
	if len(parts) < 2 {
		return ""
	}
	rest := strings.TrimPrefix(parts[1], "/")
	if i := strings.Index(rest, "/"); i > 0 {
		return rest[:i]
	}
	return rest
}
