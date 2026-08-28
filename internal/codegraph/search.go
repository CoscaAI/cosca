// SearchSimilar — busca semântica determinística sobre o code graph (F1+F2 do
// ADR-019). Ranking por codeembed.FuseSimilarity (unigram + bigram + MinHash),
// zero LLM, zero rede (I1). Compõe sobre o codegraph existente: indexa os nós
// "file" na hora da consulta (v1), reusando vetor store / int8 nos próximos.
package codegraph

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/CoscaAI/cosca/internal/codeembed"
	"github.com/CoscaAI/cosca/internal/graph"
)

// SearchHit é um resultado da busca semântica.
type SearchHit struct {
	File  string  `json:"file"`
	Path  string  `json:"path"`
	Lang  string  `json:"lang,omitempty"`
	Score float64 `json:"score"`
}

// SearchSimilar devolve os N arquivos do grafo mais similares à consulta,
// ordenados por score decrescente. Determinístico (I1). Lê o conteúdo de cada
// nó "file" (root + Path) e o rankeia. Nós sem leitura são pulados (fail-open).
func SearchSimilar(g *graph.Graph, root, query string, limit, dim int) ([]SearchHit, error) {
	if g == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	var hits []SearchHit
	for _, n := range g.GetAllNodes() {
		if n.Type != "file" || n.Path == "" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Path)))
		if err != nil {
			continue // fail-open
		}
		score := codeembed.FuseSimilarity(query, string(content), dim)
		if score <= 0 {
			continue
		}
		lang, _ := metadataString(n, "lang")
		hits = append(hits, SearchHit{
			File:  n.Name,
			Path:  n.Path,
			Lang:  lang,
			Score: score,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func metadataString(n *graph.Node, key string) (string, bool) {
	if n.Metadata == nil {
		return "", false
	}
	v, ok := n.Metadata[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
