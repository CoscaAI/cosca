package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
)

// newTestKnowledgeEngine cria um knowledge.Engine inicializado com um banco SQLite
// temporário (espelha o helper do handler REST). Embeddings "auto" degradam com
// graça sem rede — FTS cobre; banco vazio → busca retorna 0 resultados de forma
// determinística.
func newTestKnowledgeEngineAdapter(t *testing.T) *knowledge.Engine {
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
	engine, err := knowledge.New(cfg)
	if err != nil {
		t.Fatalf("failed to create knowledge engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if err := engine.Init(); err != nil {
		t.Fatalf("failed to initialize knowledge engine: %v", err)
	}
	return engine
}

// TestKnowledgeAdapter_NoRoute_Modular: em modo modular, uma query sem rota
// devolve KnowledgeSearchResults{NoRoute:true} com knowledge vazio — NUNCA
// chama o engine.Search (o adapter é criado com engine nil para provar que a
// curto-circuita antes de tocar no engine).
func TestKnowledgeAdapter_NoRoute_Modular(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())

	// engine nil: se o adapter chegasse a chamar engine.Search, seria panic/NPE.
	// NoRoute impede isso — o retorno é vazio + sinal, sem full-scan.
	adapter := NewKnowledgeAdapter(nil).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), KnowledgeSearchParams{
		Query: "ouvir musica eletronica",
		Limit: 3,
	})
	requireEngineNoError(t, err)
	requireEngine(t, res != nil, "results must be non-nil")
	if !res.NoRoute {
		t.Fatalf("expected NoRoute=true for unrouted query, got %+v", res)
	}
	if len(res.Results) != 0 || res.TotalCount != 0 {
		t.Fatalf("expected empty knowledge on NoRoute, got %+v", res)
	}
}

// TestKnowledgeAdapter_ValidRoute_NotNoRoute: em modo modular, uma query com
// rota conhecida NÃO é NoRoute — o adapter avança para o engine.Search com o
// escopo confinado (banco vazio → 0 resultados, mas sinal NoRoute=false).
func TestKnowledgeAdapter_ValidRoute_NotNoRoute(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(newTestKnowledgeEngineAdapter(t)).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), KnowledgeSearchParams{
		Query: "decisão arquitetural do banco",
		Limit: 3,
	})
	requireEngineNoError(t, err)
	requireEngine(t, res != nil, "results must be non-nil")
	if res.NoRoute {
		t.Fatalf("expected a routed (non-NoRoute) result for query with a known route, got %+v", res)
	}
}

// TestKnowledgeAdapter_NoRoute_IsEmptyKnowledge: o contrato de "knowledge vazio"
// no retorno NoRoute — Results vazio e TotalCount 0, preservando o Query.
func TestKnowledgeAdapter_NoRoute_IsEmptyKnowledge(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(nil).WithScope(resolver, search.ModeModular)

	res, err := adapter.Search(context.Background(), KnowledgeSearchParams{Query: "xyz sem rota", Limit: 3})
	requireEngineNoError(t, err)
	requireEngine(t, res != nil, "results must be non-nil")
	if res.NoRoute != true {
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
	sp := toSearchParams(KnowledgeSearchParams{
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

// TestKnowledgeAdapter_LegacyMode_NoScope: em modo legacy (default) o adapter
// não roteia — uma query sem rota NÃO vira NoRoute (busca atual intacta), mesmo
// com um resolver configurado. Legacy nunca suprime a busca.
func TestKnowledgeAdapter_LegacyMode_NoNoRoute(t *testing.T) {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	adapter := NewKnowledgeAdapter(newTestKnowledgeEngineAdapter(t)).WithScope(resolver, search.ModeLegacy)

	res, err := adapter.Search(context.Background(), KnowledgeSearchParams{Query: "xyz sem rota", Limit: 3})
	requireEngineNoError(t, err)
	requireEngine(t, res != nil, "results must be non-nil")
	if res.NoRoute {
		t.Fatalf("legacy must not produce NoRoute, got %+v", res)
	}
}

func requireEngine(t *testing.T, cond bool, msg string) {
	t.Helper()
	if !cond {
		t.Fatal(msg)
	}
}

func requireEngineNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
