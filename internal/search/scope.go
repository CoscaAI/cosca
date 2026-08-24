// Package search — scope.go
//
// Confinamento da busca ao espaço roteado (ADR-013 §3.2, Fatia 2).
//
// A Fatia 2 da arquitetura modular NÃO cria coluna `domain`/`module` no banco
// (isso é Fatia 3, condicionada a ter conteúdo de mundo indexado). O único
// sinal de domínio HONESTO e disponível hoje é o caminho do documento
// (`documents.path`) e, secundariamente, o `entity_type`. Este arquivo define a
// estratégia determinística de traduzir um `modlink.SearchScope` em um filtro
// sobre os resultados da busca híbrida.
//
// ESTRATÉGIA DE EXTRAÇÃO DE MÓDULO DO PATH (documentada):
//
//   - O path é normalizado para barras "/" (filepath.ToSlash) e dividido em
//     segmentos. Isso trata tanto os separadores Unix ("/") quanto Windows
//     ("\"), já que o conteúdo hoje vive em `internal/embed/cosca/...` e
//     `.cosca/fallback/memory/...`.
//   - Um resultado mapeia a um módulo quando um de seus segmentos é IGUAL
//     (case-insensitive) a um dos módulos do escopo.
//     Ex.: ".cosca/fallback/memory/..." → segmento "memory" → módulo "memory";
//          "internal/embed/cosca/..." → segmento "cosca" → módulo "cosca".
//   - Como sinal secundário, o `entity_type` do resultado — quando IGUAL a um
//     módulo do escopo — também conta (resultados de entidades cujo tipo de
//     domínio É a própria chave do módulo).
//
// LIMITAÇÃO HONESTA: os módulos conceituais de mundo (`vegetation`, `world`,
// `unreal`, `gis`) ainda NÃO têm conteúdo indexado, então nenhum path/entity
// atual mapeia a eles. Um escopo com apenas esses módulos retorna VAZIO — nunca
// "pesquisa tudo". O isolamento por path funciona; o isolamento por módulo
// conceitual de mundo passa a funcionar quando esse conteúdo existir (Fatia 3).
package search

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/modlink"
)

// scopeRouted reports whether a routed scope is present — i.e. the
// deterministic router (modlink) chose a BOUNDED space (ADR-013 §3.2). A nil
// scope and a NoRoute/empty-Modules scope are NOT routed: for those the
// full-scan fallback of the vector phase is legitimate (retrocompatível, o
// invariante do professor). The routed-scope flag is the gate that activates
// the candidate confinement (Fase B): only when it is true are the direct
// vector IDs (the vectoragg vocabulary) treated as the permitted candidates.
func scopeRouted(scope *modlink.SearchScope) bool {
	return scope != nil && !scope.NoRoute && len(scope.Modules) > 0
}

// confineToScope mantém apenas os resultados que mapeiam a um dos módulos do
// escopo. É um filtro determinístico e puro (função dos resultados + módulos) —
// a mesma entrada produz sempre a mesma saída. Quando `modules` está vazio,
// devolve os resultados intactos (busca ilimitada, retrocompatível).
func confineToScope(results []SearchResult, modules []string) []SearchResult {
	if len(modules) == 0 {
		return results
	}
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		if resultInScope(modules, r) {
			out = append(out, r)
		}
	}
	return out
}

// resultInScope reports whether `r` mapeia a pelo menos um dos módulos do
// escopo (`modules`). Resultados que não carregam sinal de domínio (path sem
// segmento de módulo e entity_type vazio) são descartados do espaço roteado.
func resultInScope(modules []string, r SearchResult) bool {
	for _, m := range modules {
		if moduleMatches(m, r) {
			return true
		}
	}
	return false
}

// moduleMatches testa um único resultado contra um único módulo usando os
// sinais honestos disponíveis hoje: segmento do path do documento e, como
// fallback, o entity_type. Ordem determinística: path primeiro, depois
// entity_type.
func moduleMatches(module string, r SearchResult) bool {
	if module == "" {
		return false
	}
	if r.DocumentPath != "" && pathHasSegment(r.DocumentPath, module) {
		return true
	}
	if r.EntityType != "" && strings.EqualFold(r.EntityType, module) {
		return true
	}
	return false
}

// pathHasSegment reporta se o path do documento contém um segmento IGUAL
// (case-insensitive) a `module`. Os separadores são normalizados para "/"
// antes da divisão, então "\\" (Windows) e "/" (Unix) são tratados igual.
func pathHasSegment(path, module string) bool {
	norm := filepath.ToSlash(path)
	for _, seg := range strings.Split(norm, "/") {
		if strings.EqualFold(seg, module) {
			return true
		}
	}
	return false
}

// ── Integração modlink → search ─────────────────────────────────────────────
//
// FLUXO DOCUMENTADO (ADR-013 §3.2):
//
//	QUERY
//	  → modlink.Resolver.Resolve(query)   (determinístico — escolhe o espaço)
//	  → *modlink.SearchScope              (módulos/capacidades/RouteID/Fingerprint)
//	  → SearchParams.Scope                (injetado por ApplyScope)
//	  → Engine.Search(ctx, params)        (a SEMÂNTICA REFINA o espaço já roteado)
//
// O roteador escolhe o espaço; a busca semântica nunca o escolhe. A busca
// apenas refina dentro dos módulos do escopo (ver confineToScope).

// ApplyScope resolve `query` através do roteador determinístico (modlink) e
// injeta o *modlink.SearchScope resultante em `params.Scope`, para o chamador
// passar a Engine.Search. Devolve os params atualizados E o escopo, para o
// chamador inspecionar a decisão de roteamento (módulos, capacidades, RouteID,
// NoRoute).
//
// Retrocompatível (invariante do professor: Scope nil = busca atual intacta):
// quando o resolver devolve um escopo NoRoute (sem módulos), `params.Scope`
// fica com Modules vazio e a busca permanece ilimitada — o mesmo comportamento
// de `Scope == nil`. Repare que um escopo NoRoute NUNCA inventa módulos: ele
// só deixa a busca em modo atual; jamais fabrica um falso espaço roteado.
func ApplyScope(resolver *modlink.Resolver, query string, params SearchParams) (SearchParams, *modlink.SearchScope) {
	scope := resolver.Resolve(query)
	params.Scope = scope
	return params, scope
}

// SearchWithRoute é o atalho de uma chamada só: resolve a rota para `query`,
// injeta o escopo e roda a busca híbrida confinada ao espaço roteado. O fluxo é
// query → ResolveRoute → SearchScope → SearchParams.Scope → Search(scope).
//
// FASE B (ADR-013 §3.2): quando a rota resolve um espaço (SearchScope com
// Modules não-vazio), `params.CandidateIDs` são interpretados como os
// candidatos de vetor PERMITIDOS do espaço roteado (o vocabulário de
// vectoragg.SearchRequest{Query, Scope, CandidateIDs, TopK}): a fase vetorial é
// confinada a eles em vez de fazer o full-scan do índice — é a delegação do
// retrieval confinado que a Fase A provou (não materializar tudo). O `Scope`
// com qual módulo(s) é o "onde"; a query original é o "o quê".
func SearchWithRoute(ctx context.Context, engine *Engine, resolver *modlink.Resolver, query string, params SearchParams) (*SearchResults, error) {
	scoped, _ := ApplyScope(resolver, query, params)
	return engine.Search(ctx, scoped)
}
