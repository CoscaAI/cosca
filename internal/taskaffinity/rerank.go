package taskaffinity

import (
	"sort"
	"strings"

	"github.com/CoscaAI/cosca/internal/search"
)

// maxAffinityBoost é o teto do bônus de afinidade (DESIGN-001 §3.3): 15% do
// range de score. Suficiente para subir resultados do projeto sobre genéricos,
// sem dominar o ranking multi-fator (BM25/vector/graph/fresh/pop somam 1.0).
const maxAffinityBoost = 0.15

// AffinityRerank aplica a re-ponderação por afinidade sobre resultados
// JÁ re-rankeados. É um pós-processamento determinístico e puro:
// mesmas entradas → mesmos resultados.
//
// Nil-safe: profile nil ou sem Affinity → retorna results inalterados.
// O peso da re-ponderação é modulado pela Confidence do perfil.
//
// A lista é reordenada por score final (descendente, estável) e os Ranks são
// atualizados para refletir a nova ordem (DESIGN-001 §3.1: a affinity rerank
// acontece DEPOIS do ranking multi-fator e ANTES do offset/limit).
func AffinityRerank(results []search.SearchResult, profile *TaskProfile) []search.SearchResult {
	if profile == nil || !profile.HasAffinity() || profile.Confidence <= 0 {
		return results
	}
	if len(results) == 0 {
		return results
	}

	out := make([]search.SearchResult, len(results))
	copy(out, results)

	for i := range out {
		out[i].Score += computeAffinityBoost(out[i], profile)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})

	for i := range out {
		out[i].Rank = i + 1
	}

	return out
}

// computeAffinityBoost calcula o bônus de afinidade para um resultado.
// O bônus é proporcional à fração de termos de afinidade que aparecem no
// resultado, modulado pela confiança do perfil (DESIGN-001 §3.3).
//
//	boost = maxAffinityBoost × coverage × confidence   (teto 0.15)
func computeAffinityBoost(r search.SearchResult, profile *TaskProfile) float64 {
	if profile == nil || !profile.HasAffinity() || profile.Confidence <= 0 {
		return 0
	}

	// 1. Extrair tokens do resultado (normalizados)
	resultTokens := tokenizeResult(r)

	// 2. Calcular sobreposição
	affinitySet := profile.AffinitySet()
	overlap := 0
	for token := range resultTokens {
		if _, ok := affinitySet[token]; ok {
			overlap++
		}
	}

	// 3. Fração de cobertura (0.0–1.0)
	coverage := float64(overlap) / float64(len(profile.Affinity))

	// 4. Boost ponderado pela confiança (teto maxAffinityBoost)
	return maxAffinityBoost * coverage * profile.Confidence
}

// tokenizeResult extrai tokens normalizados de um SearchResult para comparação
// com os termos de afinidade. Usa os campos mais ricos: Title, Content,
// DocumentPath e EntityType (DESIGN-001 §3.4).
func tokenizeResult(r search.SearchResult) map[string]struct{} {
	text := strings.ToLower(r.Title + " " + r.Content + " " + r.DocumentPath + " " + r.EntityType)
	tokens := tokenize(text)
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}
