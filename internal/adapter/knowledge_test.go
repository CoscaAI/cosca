package adapter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/search"
)

// TestToOrchSearchResult_Epistemic — o resultado de conhecimento carrega a
// classe epistêmica (FASE 4) na conversão canônica search→orchestration.
func TestToOrchSearchResult_Epistemic(t *testing.T) {
	t.Parallel()

	sr := &search.SearchResults{
		Results: []search.SearchResult{
			{ID: "a", Content: "medido", Metadata: map[string]string{"epistemic": "MEASURED"}},
			{ID: "b", Content: "sem classe", Metadata: map[string]string{}},
		},
	}
	out := toOrchSearchResults(sr)
	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out.Results))
	}
	if out.Results[0].Epistemic != "MEASURED" {
		t.Errorf("expected Epistemic MEASURED, got %q", out.Results[0].Epistemic)
	}
	if out.Results[1].Epistemic != "" {
		t.Errorf("expected empty Epistemic, got %q", out.Results[1].Epistemic)
	}
}


// newTestKnowledgeEngine cria um knowledge.Engine inicializado com um banco
// SQLite temporário. Embeddings "auto" degradam com graça sem rede — FTS cobre;
// banco vazio → busca retorna 0 resultados de forma determinística.
func newTestKnowledgeEngine(t *testing.T) *knowledge.Engine {
	t.Helper()
	cfg := knowledge.Config{
		DBPath:            filepath.Join(t.TempDir(), "knowledge.db"),
		RootDir:           t.TempDir(),
		AutoMigrate:       true,
		WatchEnabled:      false,
		EmbeddingProvider: "auto",
		IndexerConfig:     knowledge.DefaultConfig().IndexerConfig,
		CacheConfig:       knowledge.DefaultConfig().CacheConfig,
		RankingConfig:     knowledge.DefaultConfig().RankingConfig,
		SearchConfig:      search.DefaultSearchParams(),
	}
	eng, err := knowledge.New(cfg)
	if err != nil {
		t.Fatalf("failed to create knowledge engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	if err := eng.Init(); err != nil {
		t.Fatalf("failed to initialize knowledge engine: %v", err)
	}
	return eng
}

// TestKnowledgeAdapter_NoRoute_Modular: em modo modular, uma query sem rota
// devolve KnowledgeSearchResults{NoRoute:true} com knowledge vazio — NUNCA
// chama o engine.Search (engine nil prova que curto-circuita antes).
func TestKnowledgeAdapter_NoRoute_Modular(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())

	// engine nil: se o adapter chegasse a chamar engine.Search, seria panic/NPE.
	adapter := NewKnowledgeAdapter(nil).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), orchestration.KnowledgeSearchParams{
		Query: "ouvir musica eletronica",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("results must be non-nil")
	}
	if !res.NoRoute {
		t.Fatalf("expected NoRoute=true for unrouted query, got %+v", res)
	}
	if len(res.Results) != 0 || res.TotalCount != 0 {
		t.Fatalf("expected empty knowledge on NoRoute, got %+v", res)
	}
}

// TestKnowledgeAdapter_ValidRoute_NotNoRoute: em modo modular, uma query com
// rota conhecida NÃO é NoRoute — avança para o engine.Search com escopo
// confinado (banco vazio → 0 resultados, mas sinal NoRoute=false).
func TestKnowledgeAdapter_ValidRoute_NotNoRoute(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(newTestKnowledgeEngine(t)).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), orchestration.KnowledgeSearchParams{
		Query: "decisão arquitetural do banco",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("results must be non-nil")
	}
	if res.NoRoute {
		t.Fatalf("expected a routed (non-NoRoute) result for query with a known route, got %+v", res)
	}
}

// TestKnowledgeAdapter_NoRoute_IsEmptyKnowledge: o contrato de "knowledge vazio"
// no retorno NoRoute — Results vazio e TotalCount 0, preservando o Query.
func TestKnowledgeAdapter_NoRoute_IsEmptyKnowledge(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(nil).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), orchestration.KnowledgeSearchParams{Query: "xyz sem rota", Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("results must be non-nil")
	}
	if !res.NoRoute {
		t.Fatalf("expected NoRoute=true, got %+v", res)
	}
	if res.Query != "xyz sem rota" {
		t.Fatalf("expected Query preserved, got %q", res.Query)
	}
	if len(res.Results) != 0 {
		t.Fatalf("expected empty knowledge, got %d results", len(res.Results))
	}
}

// TestKnowledgeAdapter_Unscoped_CopiesScopeToSearchParams: o toSearchParams
// repassa o Scope de KnowledgeSearchParams para search.SearchParams — o "onde"
// do roteamento sobrevive à conversão.
func TestKnowledgeAdapter_Unscoped_CopiesScopeToSearchParams(t *testing.T) {
	s := &modlink.SearchScope{Modules: []string{"adr"}}
	sp := toSearchParams(orchestration.KnowledgeSearchParams{
		Query: "decisão arquitetural",
		Limit: 7,
		Scope: s,
	})
	if sp.Scope == nil {
		t.Fatalf("toSearchParams must copy Scope, got nil")
	}
	if len(sp.Scope.Modules) != 1 || sp.Scope.Modules[0] != "adr" {
		t.Fatalf("toSearchParams Scope = %+v, want Modules=[adr]", sp.Scope)
	}
	if sp.Query != "decisão arquitetural" || sp.Limit != 7 {
		t.Fatalf("toSearchParams lost query/limit: %+v", sp)
	}
}

// TestKnowledgeAdapter_LegacyMode_NoNoRoute: em modo legacy (default) o adapter
// não roteia — uma query sem rota NÃO vira NoRoute (busca atual intacta).
func TestKnowledgeAdapter_LegacyMode_NoNoRoute(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(newTestKnowledgeEngine(t)).WithScope(resolver, search.ModeLegacy)

	res, err := adapter.Search(context.Background(), orchestration.KnowledgeSearchParams{Query: "xyz sem rota", Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("results must be non-nil")
	}
	if res.NoRoute {
		t.Fatalf("legacy must not produce NoRoute, got %+v", res)
	}
}
