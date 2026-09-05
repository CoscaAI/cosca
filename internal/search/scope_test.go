package search

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Helpers ────────────────────────────────────────────────────────────────

// graphEngineMounta o Engine de teste com apenas a fase de grafo habilitada
// (Type=="" para os nós aparecerem via FilterNodes("")), sem FTS/vector/ranker.
func graphEngineNodes(t *testing.T, nodes ...*graph.Node) *Engine {
	t.Helper()
	g := graph.New()
	for _, n := range nodes {
		require.NoError(t, g.AddNode(n))
	}
	return NewEngine(nil, nil, g, nil, nil)
}

// idList devolve a lista ordenada de IDs de um resultado (para comparação
// determinística de ordem).
func idList(results []SearchResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.ID
	}
	return out
}

// ── Mecânica do filtro: pathHasSegment / moduleMatches / confineToScope ─────

func TestPathHasSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		path   string
		module string
		want   bool
	}{
		{"unix path memoria", ".cosca/fallback/memory/agent/notes.md", "memory", true},
		{"case-insensitive", ".cosca/fallback/Memory/notes.md", "memory", true},
		{"windows separator", `.cosca\fallback\memory\notes.md`, "memory", true},
		{"cosca embed", "internal/embed/cosca/x.md", "cosca", true},
		{"unreal present", "content/unreal/actors.md", "unreal", true},
		{"module ausente", "internal/embed/cosca/docs.md", "memory", false},
		{"segmento inteiro nao substring", "a/memory.md", "memory", false},
		{"path vazio", "", "memory", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pathHasSegment(tt.path, tt.module))
		})
	}
}

func TestModuleMatches(t *testing.T) {
	t.Parallel()
	// Sinal primário: path do documento.
	assert.True(t, moduleMatches("memory", SearchResult{DocumentPath: ".cosca/fallback/memory/x.md"}))
	// Sinal secundário: entity_type.
	assert.True(t, moduleMatches("skill", SearchResult{EntityType: "skill"}))
	// Sem sinal de domínio → não mapeia (descartado do espaço roteado).
	assert.False(t, moduleMatches("unreal", SearchResult{ID: "no-signal", Content: "x"}))
	// Módulo vazio nunca casa.
	assert.False(t, moduleMatches("", SearchResult{DocumentPath: "memory/x.md"}))
}

func TestConfineToScope(t *testing.T) {
	t.Parallel()
	inputs := []SearchResult{
		{ID: "m1", DocumentPath: ".cosca/fallback/memory/notes.md"},
		{ID: "m2", DocumentPath: `/cosca\memory\x.md`},
		{ID: "u1", DocumentPath: "content/unreal/actors.md"},
		{ID: "no-signal", Content: "sem path nem entity_type"},
	}

	t.Run("escopo memory restringe a memory", func(t *testing.T) {
		got := confineToScope(inputs, []string{"memory"})
		assert.Equal(t, []string{"m1", "m2"}, idList(got))
	})

	t.Run("modules vazio devolve tudo (retrocompativel)", func(t *testing.T) {
		got := confineToScope(inputs, nil)
		assert.Equal(t, 4, len(got))
	})

	t.Run("modulo conceitual sem conteudo retorna vazio", func(t *testing.T) {
		got := confineToScope(inputs, []string{"vegetation", "world"})
		assert.Empty(t, got)
	})
}

// ── Engine: Scope aplicado (mecânica) via path — modo confinado ────────────

// TestSearch_ScopeRestrictedToMemory monta dois hits (um de memory, um de
// unreal) que casam com a mesma query, e confirma que o escopo {memory} mantém
// apenas o hit cujo path contém o segmento "memory" — sem "pesquisar tudo".
func TestSearch_ScopeRestrictedToMemory(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "mem", Type: "", Name: "Memory Record", Path: ".cosca/fallback/memory/notes.md"},
		&graph.Node{ID: "unr", Type: "", Name: "Unreal Record", Path: "content/unreal/actors.md"},
	)

	scope := &modlink.SearchScope{Modules: []string{"memory"}}
	params := SearchParams{
		Query:        "record",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		EnableFacets: false,
		Scope:        scope,
	}

	res, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, res.TotalCount)
	assert.Len(t, res.Results, 1)
	assert.Equal(t, "mem", res.Results[0].ID, "apenas o hit de memory sobrevive")
	assert.Equal(t, ".cosca/fallback/memory/notes.md", res.Results[0].DocumentPath)
}

// TestSearch_ScopeRestrictedToUnreal é o espelho: escopo {unreal} mantém o hit
// de unreal e descarta o de memory.
func TestSearch_ScopeRestrictedToUnreal(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "mem", Type: "", Name: "Memory Record", Path: ".cosca/fallback/memory/notes.md"},
		&graph.Node{ID: "unr", Type: "", Name: "Unreal Record", Path: "content/unreal/actors.md"},
	)

	scope := &modlink.SearchScope{Modules: []string{"unreal"}}
	params := SearchParams{
		Query:        "record",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Scope:        scope,
	}

	res, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, res.TotalCount)
	assert.Len(t, res.Results, 1)
	assert.Equal(t, "unr", res.Results[0].ID)
}

// ── Engine: Scope não encontrado — sem fallback "pesquisar tudo" ────────────

// TestSearch_ScopeModuleNoContent prova que um escopo com um módulo conceitual
// que hoje não tem conteúdo indexado (vegetation/world/unreal ficam vazios a
// menos que haja path/entity explicitamente) retorna VAZIO — e não recai na
// busca ilimitada — mesmo havendo hits genéricos para a query.
func TestSearch_ScopeModuleNoContent(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "mem", Type: "", Name: "Memory Record", Path: ".cosca/fallback/memory/notes.md"},
		&graph.Node{ID: "unr", Type: "", Name: "Unreal Record", Path: "content/unreal/actors.md"},
	)

	// (1) Escopo que nenhum path/entity mapeia → vazio, SEM buscar tudo.
	scoped := SearchParams{
		Query:        "record",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Scope:        &modlink.SearchScope{Modules: []string{"vegetation", "world"}},
	}
	res, err := engine.Search(context.Background(), scoped)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 0, res.TotalCount, "escopo sem conteúdo não deve recair em 'pesquisar tudo'")
	assert.Empty(t, res.Results)

	// (2) A MESMA query sem escopo retorna os 2 hits — prova que era o escopo
	// que confinava, não um erro de busca.
	unscoped := scoped
	unscoped.Scope = nil
	res2, err := engine.Search(context.Background(), unscoped)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, 2, res2.TotalCount)
	assert.Len(t, res2.Results, 2)
}

// ── Engine: Retrocompatibilidade (Scope nil = comportamento atual) ──────────

func TestSearch_NilScopeKeepsCurrentBehavior(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "mem", Type: "", Name: "Memory Record", Path: ".cosca/fallback/memory/notes.md"},
		&graph.Node{ID: "unr", Type: "", Name: "Unreal Record", Path: "content/unreal/actors.md"},
	)

	params := SearchParams{
		Query:        "record",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		// Scope nil → busca ilimitada (comportamento atual).
	}

	res, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 2, res.TotalCount)
	assert.Len(t, res.Results, 2)
}

// ── Engine: Determinístico (mesma query + mesmo escopo → mesma ordem) ───────

func TestSearch_ScopeDeterministic(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "a", Type: "", Name: "Memory Alpha", Path: ".cosca/fallback/memory/alpha.md"},
		&graph.Node{ID: "b", Type: "", Name: "Memory Beta", Path: ".cosca/fallback/memory/beta.md"},
		// nó "connector" dá boost de centralidade ao nó "a", fixando a ordem
		// (a > b por score), para a comparação de ordem ser não-trivial.
		&graph.Node{ID: "conn", Type: "", Name: "Connector"},
	)
	require.NoError(t, engine.graph.AddEdge(&graph.Edge{Source: "a", Target: "conn", Type: graph.RelRelatedTo}))

	params := SearchParams{
		Query:        "memory",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Scope:        &modlink.SearchScope{Modules: []string{"memory"}},
	}

	first, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Len(t, first.Results, 2)

	for i := 0; i < 3; i++ {
		next, err := engine.Search(context.Background(), params)
		require.NoError(t, err)
		require.NotNil(t, next)
		assert.Equal(t, idList(first.Results), idList(next.Results),
			"mesma query + mesmo escopo devem produzir a mesma ordem")
	}
	// A ordem é a determinística por score (a tem boost de centralidade).
	assert.Equal(t, "a", first.Results[0].ID)
	assert.Equal(t, "b", first.Results[1].ID)
}

// ── Engine: sinal por entity_type (vetor) ───────────────────────────────────

// TestSearch_ScopeEntityTypeSignal prova o sinal secundário de domínio: um hit
// de entidade vetorial com entity_type que casa com o módulo sobrevive ao
// escopo; os demais são descartados.
func TestSearch_ScopeEntityTypeSignal(t *testing.T) {
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{ID: "skill-1", Score: 0.9, Content: "skill content", EntityID: "e1", Metadata: map[string]string{"entity_type": "skill"}},
				{ID: "agent-1", Score: 0.8, Content: "agent content", EntityID: "e2", Metadata: map[string]string{"entity_type": "agent"}},
			}, nil
		},
	}
	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)

	params := SearchParams{
		Query:        "coisa",
		Limit:        20,
		EnableVector: true,
		EnableFTS:    false,
		Scope:        &modlink.SearchScope{Modules: []string{"skill"}},
	}

	res, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, res.TotalCount)
	assert.Len(t, res.Results, 1)
	assert.Equal(t, "skill-1", res.Results[0].ID)
	assert.Equal(t, "skill", res.Results[0].EntityType)
}

// ── Integração modlink → search (query → ResolveRoute → Scope → Search) ─────

func TestApplyScope_InjectsScopeAndPreservesNoRoute(t *testing.T) {
	resolver := modlink.NewResolver([]modlink.Route{
		{Trigger: "memoria da familia", Module: "memory", Capability: "memory.search", Priority: 10},
		{Trigger: "arvore no terreno", Module: "vegetation", Capability: "vegetation.generate_tree", Priority: 20},
	})

	t.Run("rota conhecida injeta modules", func(t *testing.T) {
		params, scope := ApplyScope(resolver, "memoria da familia", SearchParams{Query: "memoria da familia"})
		assert.NotNil(t, params.Scope)
		assert.Equal(t, []string{"memory"}, scope.Modules)
		assert.Equal(t, []string{"memory"}, params.Scope.Modules)
	})

	t.Run("rota desconhecida (NoRoute) deixa Modules vazio", func(t *testing.T) {
		params, scope := ApplyScope(resolver, "musica eletronica", SearchParams{Query: "musica eletronica"})
		require.NotNil(t, scope)
		assert.True(t, scope.NoRoute)
		assert.Empty(t, scope.Modules)
		// Modules vazio → Engine.Search fica ilimitado (retrocompatível).
		assert.Empty(t, params.Scope.Modules)
	})
}

// TestSearchWithRoute_EndToEnd percorre o fluxo completo: query → ResolveRoute →
// SearchScope → SearchParams.Scope → Engine.Search(scope). O roteador envia
// "memoria" ao módulo "memory" e a busca confinada retorna só o hit de memory.
func TestSearchWithRoute_EndToEnd(t *testing.T) {
	resolver := modlink.NewResolver([]modlink.Route{
		{Trigger: "memoria", Module: "memory", Capability: "memory.search", Priority: 10},
	})
	engine := graphEngineNodes(t,
		&graph.Node{ID: "mem", Type: "", Name: "Memoria da Familia", Path: ".cosca/fallback/memory/family.md"},
		&graph.Node{ID: "unr", Type: "", Name: "Memoria Unreal", Path: "content/unreal/actors.md"},
	)

	res, err := SearchWithRoute(context.Background(), engine, resolver, "memoria da familia", SearchParams{
		Query:        "memoria",
		Limit:        20,
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	// O hit "Memoria Unreal" (path "unreal", sem segmento "memory") é descartado.
	assert.Equal(t, 1, res.TotalCount)
	assert.Len(t, res.Results, 1)
	assert.Equal(t, "mem", res.Results[0].ID)
}

// TestScopeModuleMatchesDeterministic reflete a honestidade do ADR: a extração
// de módulo é uma função pura do path/entity — mesma entrada, mesma saída.
func TestScopeModuleMatchesDeterministic(t *testing.T) {
	t.Parallel()
	n := 10
	first := make([]bool, n)
	for i := 0; i < n; i++ {
		first[i] = moduleMatches("memory", SearchResult{DocumentPath: ".cosca/fallback/memory/x.md"})
	}
	// Todas as iterações produzem o mesmo resultado determinístico.
	for _, v := range first {
		assert.True(t, v)
	}
	// E a ordem de avaliação não altera o resultado (função pura).
	got := moduleMatches("memory", SearchResult{DocumentPath: "content/unreal/y.md"})
	assert.False(t, got)
}

// ── ANTI-REGRESSÃO: contrato `vector result → DocumentPath → scope` ─────────
//
// Este teste protege o bug corrigido na auditoria do instrumento (recall=0):
// o vectorResults NÃO preenchia SearchResult.DocumentPath, e o confineToScope
// (que chaveia por DocumentPath) descartava TODO o resultado vetorial no caminho
// roteado. Ele falha se vectorResults voltar a produzir um SearchResult com o
// DocumentPath vazio para um documento conhecido — exatamente o que causava o
// recall=0 mesmo com o GT no candidate set e a produção achando o top-1.
//
// Contrato verificado (comportamento observável, não inspeção interna):
//   - o vetor pertence a um documento de um módulo conhecido (memory);
//   - o resultado vetorial (via vectorResults) carrega o DocumentPath;
//   - o moduleMatches/scope reconhece o módulo;
//   - o confineToScope NÃO descarta o resultado.
func TestVectorResults_PropagatesDocumentPath_ScopeKeepsResult(t *testing.T) {
	// 1. DB em memória/temp com a tabela `documents` (id, path) — a fonte
	//    canônica do DocumentPath que o FTSClient.DocumentPaths resolve.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "knowledge.db")
	ftsDB, err := sqlite.Open(sqlite.Config{Path: dbPath, AutoMigrate: false})
	require.NoError(t, err)
	defer ftsDB.Close()
	_, err = ftsDB.Exec(`CREATE TABLE IF NOT EXISTS documents (id TEXT PRIMARY KEY, path TEXT)`)
	require.NoError(t, err)
	_, err = ftsDB.Exec(`INSERT INTO documents (id, path) VALUES (?, ?)`,
		"doc-mem-1", ".cosca/fallback/memory/notes.md")
	require.NoError(t, err)
	ftsClient := sqlite.NewFTSClient(ftsDB)

	// 2. mockVectorStore retorna um resultado VETORIAL genuíno com DocumentID
	//    (NÃO EntityID — o caminho que quebrava: sem DocumentPath o scope descarta).
	docID := "doc-mem-1"
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{ID: "chunk-mem-1", Score: 0.9, DocumentID: docID, Content: "memoria notes"},
			}, nil
		},
	}
	engine := NewEngine(ftsClient, vs, nil, nil, stubEmbedFunc)

	// 3. Scope roteado {memory}; a busca vetorial deve manter o hit de memory.
	params := SearchParams{
		Query:        "memoria",
		Limit:        20,
		EnableVector: true,
		EnableFTS:    false,
		EnableGraph:  false,
		Scope:        &modlink.SearchScope{Modules: []string{"memory"}},
	}

	res, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, res)

	// CONTRATO: o resultado não é descartado (se DocumentPath ficasse vazio,
	// moduleMatches=false e confineToScope o removeria → TotalCount=0).
	assert.Equal(t, 1, res.TotalCount,
		"contrato: vetor de um documento de 'memory' deve sobreviver ao scope")
	require.Len(t, res.Results, 1)
	assert.Equal(t, "chunk-mem-1", res.Results[0].ID)
	// O DocumentPath foi propagado (a fonte da correção do bug).
	assert.Equal(t, ".cosca/fallback/memory/notes.md", res.Results[0].DocumentPath,
		"contrato: vectorResults deve preencher DocumentPath (nao pode voltar a vazio)")
}

// ── FASE 1 routing/scope — SearchWithRoute / SearchWithRouteForced ────────────

// adrResolver é um resolver mínimo com apenas a rota real de ADR (trigger
// nominal do registry DefaultRoutes), para os testes de routing/scope.
func adrResolver() *modlink.Resolver {
	return modlink.NewResolver([]modlink.Route{
		{Trigger: "decisão arquitetural", Module: "adr", Capability: "adr.record", Priority: 1},
	})
}

// TestSearchWithRoute_ConfinesToADROnly: uma rota VÁLIDA (query → adr) faz a
// busca confinar ao módulo "adr" — o hit de outro módulo é DESCARTADO.
func TestSearchWithRoute_ConfinesToADROnly(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "adr-1", Type: "", Name: "Decisão arquitetural do banco de dados", Path: ".cosca/fallback/adr/0001-banco.md"},
		&graph.Node{ID: "docs-1", Type: "", Name: "Guia de decisão arquitetural", Path: ".cosca/fallback/docs/guia.md"},
	)

	res, err := SearchWithRoute(context.Background(), engine, adrResolver(), "decisão arquitetural do banco", SearchParams{
		Query:        "decisão arquitetural",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	// Só o hit cujo path tem o segmento "adr" sobrevive.
	assert.Equal(t, 1, res.TotalCount)
	require.Len(t, res.Results, 1)
	assert.Equal(t, "adr-1", res.Results[0].ID)
}

// TestSearchWithRoute_NoRoute_NeverFullScan: uma consulta SEM rota em modo
// modular devolve 0 resultados + NoRoute=true, e NUNCA chama engine.Search com
// Scope nil. Provamos o "não full-scan" comportamentalmente: a MESMA query sem
// escopo (legacy/full-scan) retornaria os hits amplos — que não aparecem.
func TestSearchWithRoute_NoRoute_NeverFullScan(t *testing.T) {
	// Resolver com apenas a rota de ADR: "musica" não casa → NoRoute.
	resolver := adrResolver()

	// Dois hits amplos que casam com a query "musica" (se houvesse full-scan
	// retornariam ambos).
	engine := graphEngineNodes(t,
		&graph.Node{ID: "m1", Type: "", Name: "Musica do metal", Path: ".cosca/fallback/memory/metal.md"},
		&graph.Node{ID: "m2", Type: "", Name: "Musica classica", Path: ".cosca/fallback/docs/classica.md"},
	)

	// (1) A rota desconhecida é explícita: NoRoute=true, sem inventar módulos.
	scope := resolver.Resolve("musica")
	assert.True(t, scope.NoRoute, "esperado NoRoute=true para query sem rota")
	assert.Empty(t, scope.Modules)

	// (2) O caminho MODULAR NUNCA faz full-scan: 0 resultados, sem consultar o engine.
	res, err := SearchWithRouteModular(context.Background(), engine, resolver, "musica", SearchParams{
		Query:        "musica",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 0, res.TotalCount,
		"NoRoute não deve recair no full-scan silencioso (0 resultados)")
	assert.Empty(t, res.Results)

	// (3) Prova que os hits EXISTEM: um engine.Search sem Scope (legacy) os
	// retornaria — então o 0 do passo (2) só pode ter vindo do short-circuit,
	// não de uma busca que achou nada.
	unscoped := SearchParams{Query: "musica", EnableGraph: true, EnableFTS: false, EnableVector: false, Limit: 20}
	full, err := engine.Search(context.Background(), unscoped)
	require.NoError(t, err)
	assert.Equal(t, 2, full.TotalCount,
		"a query produziu hits no full-scan — logo o 0 anterior veio do short-circuit, não do engine")
}

// TestSearchWithRoute_LegacyIntact confirma a compatibilidade do modo LEGACY:
// os constantes de modo são os nomes esperados e a busca sem escopo (nil) é
// ilimitada — o comportamento atual permanece intacto.
func TestSearchWithRoute_LegacyIntact(t *testing.T) {
	assert.Equal(t, "legacy", ModeLegacy)
	assert.Equal(t, "modular", ModeModular)

	engine := graphEngineNodes(t,
		&graph.Node{ID: "m1", Type: "", Name: "Musica do metal", Path: ".cosca/fallback/memory/metal.md"},
		&graph.Node{ID: "d1", Type: "", Name: "Musica classica", Path: ".cosca/fallback/docs/classica.md"},
	)

	// Legacy = busca atual: Scope nil → ilimitada (retorna ambos os hits),
	// mesmo que em modo modular "musica" fosse NoRoute.
	res, err := engine.Search(context.Background(), SearchParams{
		Query:        "musica",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
}

// TestResolveForcedScope_Incompatible_NoRoute: --scope=M é uma restrição
// ADICIONAL. Se M não está entre os módulos roteados (query incompatível com o
// escopo forçado), o resultado é NoRoute explícito — nunca amplia.
func TestResolveForcedScope_Incompatible_NoRoute(t *testing.T) {
	resolver := adrResolver()

	// Compatível: query roteia para [adr] e --scope=adr é aceito → confina a adr.
	compat := ResolveForcedScope(resolver, "decisão arquitetural do banco", "adr")
	assert.False(t, compat.NoRoute)
	assert.Equal(t, []string{"adr"}, compat.Modules)

	// Incompatível: query roteia para [adr], mas --scope=knowledge não está no
	// espaço roteado → NoRoute explícito (0 resultados, nunca amplia).
	incompat := ResolveForcedScope(resolver, "decisão arquitetural do banco", "knowledge")
	assert.True(t, incompat.NoRoute)
	assert.Empty(t, incompat.Modules)

	// Sem escopo forçado: devolve exatamente o espaço roteado.
	noforce := ResolveForcedScope(resolver, "decisão arquitetural do banco", "")
	assert.False(t, noforce.NoRoute)
	assert.Equal(t, []string{"adr"}, noforce.Modules)
}

// TestSearchWithRouteForced_Incompatible_ReturnsZero: no caminho de uma chamada
// só, um escopo forçado incompatível devolve 0 resultados sem consultar o engine
// (o roteador já disse NoRoute). A mesma query sem escopo forçado retorna o hit.
func TestSearchWithRouteForced_Incompatible_ReturnsZero(t *testing.T) {
	engine := graphEngineNodes(t,
		&graph.Node{ID: "adr-1", Type: "", Name: "Decisão arquitetural do banco", Path: ".cosca/fallback/adr/0001.md"},
	)

	forced, err := SearchWithRouteForcedModular(context.Background(), engine, adrResolver(), "decisão arquitetural do banco", "knowledge", SearchParams{
		Query:        "decisão arquitetural",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, forced.TotalCount, "escopo forçado incompatível → 0, nunca amplia")

	routed, err := SearchWithRouteForcedModular(context.Background(), engine, adrResolver(), "decisão arquitetural do banco", "", SearchParams{
		Query:        "decisão arquitetural",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, routed.TotalCount, "sem escopo for�ado a rota confina e acha o hit")
	assert.Equal(t, "adr-1", routed.Results[0].ID)
}

// ── FASE 4 — confinamento por classe epistêmica (fail-closed) ────────────────

func TestConfineEpistemic_KeepsOnlyAllowed(t *testing.T) {
	t.Parallel()
	results := []SearchResult{
		{ID: "a", Metadata: map[string]string{"epistemic": "MEASURED"}},
		{ID: "b", Metadata: map[string]string{"epistemic": "FACT"}},
		{ID: "c", Metadata: map[string]string{"epistemic": "INFERRED"}},
	}

	// Só medidos sobrevivem.
	out := confineEpistemic(results, []string{"MEASURED"})
	assert.Len(t, out, 1)
	assert.Equal(t, "a", out[0].ID)

	// FACT + MEASURED.
	out = confineEpistemic(results, []string{"FACT", "MEASURED"})
	assert.Len(t, out, 2)

	// Vazio = sem filtro (todos).
	assert.Len(t, confineEpistemic(results, nil), 3)
}

func TestConfineEpistemic_FailClosed_UnknownDropped(t *testing.T) {
	t.Parallel()
	results := []SearchResult{
		{ID: "has-ep", Metadata: map[string]string{"epistemic": "FACT"}},
		{ID: "no-ep", Metadata: map[string]string{}}, // SEM classe epistêmica
	}

	// FAIL-CLOSED: um resultado sem classe é DESCARTADO quando há filtro —
	// nunca incluído por engano.
	out := confineEpistemic(results, []string{"FACT"})
	assert.Len(t, out, 1)
	assert.Equal(t, "has-ep", out[0].ID)
}

func TestSearch_EpistemicFilter(t *testing.T) {
	t.Parallel()
	engine := graphEngineNodes(t,
		&graph.Node{ID: "adr-1", Type: "", Name: "Decisão arquitetural do banco", Path: ".cosca/fallback/adr/0001.md"},
	)

	// Filtro por epistemic sobre resultados que carregam a classe no Metadata.
	// (FTS/vector nem sempre populam Metadata["epistemic"] ainda — o filtro é
	// fail-closed e descarta o que não tem a classe, provado acima.)
	res, err := engine.Search(context.Background(), SearchParams{
		Query:        "decisão arquitetural",
		EnableGraph:  true,
		EnableFTS:    false,
		EnableVector: false,
		Limit:        20,
		Epistemic:    []string{"MEASURED"},
	})
	require.NoError(t, err)
	// Como o nó do grafo não carrega Metadata["epistemic"], o filtro fail-closed
	// descarta o resultado (não retorna por engano).
	assert.Equal(t, 0, res.TotalCount,
		"sem classe epistêmica, o filtro fail-closed NÃO deve incluir por engano")
}
