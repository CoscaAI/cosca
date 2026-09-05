package knowledge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FASE B — RouteCandidateIDs: dado um escopo roteado, devolve os IDs de vetor
// PERMITIDOS (documentos que mapeiam ao módulo do escopo), nunca o índice inteiro.

func indexScoped(t *testing.T, engine *Engine, rel, content string) {
	t.Helper()
	d := writeDoc(t, engine, rel, content)
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))
}

func TestRouteCandidateIDs_PermitsOnlyScope(t *testing.T) {
	engine := memoryOnlyEngine(t)
	indexScoped(t, engine, filepath.Join("docs", "adr", "adr-a.md"), "# ADR-A\n\nDecisão A\n")
	indexScoped(t, engine, filepath.Join("docs", "architecture", "arch-a.md"), "# Alpha\n\nArquitetura A\n")

	// Escopo roteado para "adr" → só os vetores do doc adr.
	ids, err := engine.RouteCandidateIDs(&modlink.SearchScope{Modules: []string{"adr"}})
	require.NoError(t, err)
	require.NotEmpty(t, ids, "deve haver candidatos no escopo adr")

	// Todos os IDs retornados pertencem a documentos cujo path tem segmento "adr".
	for _, id := range ids {
		var path string
		require.NoError(t, engine.db.QueryRow(`SELECT d.path FROM vectors v JOIN documents d ON v.document_id=d.id WHERE v.id=?`, id).Scan(&path))
		assert.True(t, pathHasSegment(path, "adr"), "path %s deve estar no escopo adr", path)
	}
}

func TestRouteCandidateIDs_NoRouteIsNil(t *testing.T) {
	engine := memoryOnlyEngine(t)
	// Sem escopo roteado (Modules vazio / NoRoute) → nil (full-scan baseline legítimo).
	ids, err := engine.RouteCandidateIDs(&modlink.SearchScope{NoRoute: true})
	require.NoError(t, err)
	assert.Nil(t, ids)

	ids, err = engine.RouteCandidateIDs(&modlink.SearchScope{Modules: nil})
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestRouteCandidateIDs_ScopeDoesNotLeak(t *testing.T) {
	engine := memoryOnlyEngine(t)
	indexScoped(t, engine, filepath.Join("docs", "adr", "adr-b.md"), "# ADR-B\n\nDecisão B\n")
	indexScoped(t, engine, filepath.Join("docs", "architecture", "arch-b.md"), "# Beta\n\nArquitetura B\n")

	// Escopo "adr": nenhum vetor pode pertencer a um doc architecture.
	ids, err := engine.RouteCandidateIDs(&modlink.SearchScope{Modules: []string{"adr"}})
	require.NoError(t, err)
	for _, id := range ids {
		var path string
		require.NoError(t, engine.db.QueryRow(`SELECT d.path FROM vectors v JOIN documents d ON v.document_id=d.id WHERE v.id=?`, id).Scan(&path))
		assert.False(t, pathHasSegment(path, "architecture"), "scope adr NÃO pode conter vetor de architecture")
	}
}
