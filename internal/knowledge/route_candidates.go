package knowledge

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/modlink"
)

// FASE B (ADR-013 §3.2) — Geração de candidatos confinados a partir do escopo
// ROTEADO.
//
// O problema (verificado na auditoria 5.1): o caminho de produção chama
// `ApplyScope`/`ApplyForcedScope`, que injeta `SearchParams.Scope`, mas NUNCA
// gera `SearchParams.CandidateIDs`. Resultado: `resolveRouteCandidates`
// (internal/search/search.go) recebe candidatos vazios e a fase vetorial faz
// FULL-SCAN do índice — o "palheiro" que a Fase A eliminou.
//
// Aqui corrigimos A ORIGEM: dado um escopo roteado (módulos não-vazios),
// devolvemos os IDs de vetor PERMITIDOS daquele espaço — os vetores cujo
// documento mapeia a um dos módulos do escopo (mesma regra determinística
// `pathHasSegment` de internal/search/scope.go). Não decodifica embeddings
// (não materializa o índice): só lê id+path e filtra por segmento; a decodificação
// fica por conta do `SearchWithMetrics` confinado (decodifica só os candidatos).
//
// Sem escopo roteado (NoRoute / Modules vazio) devolve nil — o full-scan é a
// linha de base legítima desse caso (invariante do professor), nunca candidatos
// truncados.
func (e *Engine) RouteCandidateIDs(scope *modlink.SearchScope) ([]string, error) {
	if !scopeRouted(scope) {
		return nil, nil
	}

	e.mu.RLock()
	db := e.db
	initialized := e.initialized
	e.mu.RUnlock()
	if !initialized || db == nil {
		return nil, fmt.Errorf("knowledge engine not initialized")
	}

	// Predicado de path para cada módulo (segmento, case-insensitive), no mesmo
	// espírito de pathHasSegment: cobre segmento no início, no meio (exatamente
	// delimitado) e no fim do path normalizado.
	preds := make([]string, 0, len(scope.Modules))
	args := make([]any, 0, len(scope.Modules)*3)
	for _, m := range scope.Modules {
		if m == "" {
			continue
		}
		ml := strings.ToLower(m)
		pathExpr := `LOWER(REPLACE(d.path,'\','/'))`
		preds = append(preds,
			`(`+pathExpr+` LIKE '%/'||?||'/%' OR `+pathExpr+` LIKE ?||'/%' OR `+pathExpr+` LIKE '%/'||? OR `+pathExpr+` = ?)`)
		args = append(args, ml, ml, ml, ml)
	}
	if len(preds) == 0 {
		return nil, nil
	}

	rows, err := db.Query(
		`SELECT v.id, d.path FROM vectors v JOIN documents d ON v.document_id = d.id
		 WHERE (`+strings.Join(preds, " OR ")+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("route candidate ids: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	out := make([]string, 0, 64)
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		if id == "" {
			continue
		}
		ok := false
		for _, m := range scope.Modules {
			if m != "" && pathHasSegment(path, m) {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scopeRouted replica a regra de internal/search.scopeRouted (não-exportado):
// um escopo está "roteado" quando não é NoRoute e tem módulos.
func scopeRouted(scope *modlink.SearchScope) bool {
	return scope != nil && !scope.NoRoute && len(scope.Modules) > 0
}

// pathHasSegment replica a regra de internal/search.pathHasSegment (não-exportada):
// o path contém um segmento igual (case-insensitive) ao módulo.
func pathHasSegment(path, module string) bool {
	if path == "" || module == "" {
		return false
	}
	for _, seg := range strings.Split(slashPath(path), "/") {
		if strings.EqualFold(seg, module) {
			return true
		}
	}
	return false
}
