package grpcserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
)

// spyEngine implements knowledgeEngine, counting Search calls (FASE 3.5 proof)
// and capturing the SearchParams. The non-Search methods are no-ops.
type spyEngine struct {
	calls int
	last  search.SearchParams
}

func (s *spyEngine) Search(_ context.Context, params search.SearchParams) (*search.SearchResults, error) {
	s.calls++
	s.last = params
	return &search.SearchResults{Query: params.Query}, nil
}
func (s *spyEngine) IndexDocument(context.Context, string) error        { return nil }
func (s *spyEngine) IndexDirectory(context.Context, string) error       { return nil }
func (s *spyEngine) GetStats() (*knowledge.Stats, error)                { return &knowledge.Stats{}, nil }
func (s *spyEngine) Sync(context.Context) (*knowledge.SyncResult, error) { return &knowledge.SyncResult{}, nil }

// adrResolver returns a router that only knows the "adr" route (trigger "decisão
// arquitetural"). A query about anything else (e.g. "musica") yields NoRoute.
func adrResolver() *modlink.Resolver {
	return modlink.NewResolver([]modlink.Route{
		{Trigger: "decisão arquitetural", Module: "adr", Capability: "adr.record", Priority: 1},
	})
}

func TestKnowledgeService_NoRoute_DoesNotCallEngine(t *testing.T) {
	t.Parallel()
	spy := &spyEngine{}
	srv := NewKnowledgeServiceServer(spy).WithRouting(adrResolver(), search.ModeModular)

	// "musica" não casa com nenhuma rota (só existe a "adr") → NoRoute.
	resp, err := srv.Search(context.Background(), &cospb.SearchRequest{Query: "musica", Limit: 10})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// ⭐ PROVA DA REGRA DE OURO: resposta vazia E engine.Search NÃO foi chamado.
	// Uma resposta vazia sozinha poderia mascarar um full-scan; 0 chamadas não.
	assert.Equal(t, int32(0), resp.GetTotal(), "NoRoute deve retornar 0 resultados")
	assert.Empty(t, resp.GetResults(), "NoRoute deve retornar lista vazia")
	assert.Equal(t, 0, spy.calls, "NoRoute NÃO pode chamar engine.Search (nunca full-scan silencioso)")
	assert.Nil(t, spy.last.Scope)
}

func TestKnowledgeService_ValidRoute_AppliesScope(t *testing.T) {
	t.Parallel()
	spy := &spyEngine{}
	srv := NewKnowledgeServiceServer(spy).WithRouting(adrResolver(), search.ModeModular)

	// "decisão arquitetural" roteia para o módulo "adr".
	_, err := srv.Search(context.Background(), &cospb.SearchRequest{Query: "decisão arquitetural do banco", Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, 1, spy.calls, "rota válida deve chegar ao engine.Search")
	require.NotNil(t, spy.last.Scope, "rota válida deve aplicar o Scope")
	assert.Contains(t, spy.last.Scope.Modules, "adr", "o Scope deve confinar ao módulo adr")
}

func TestKnowledgeService_Legacy_NoScope(t *testing.T) {
	t.Parallel()
	spy := &spyEngine{}

	// Modo legacy (default): routing NÃO se aplica — comportamento anterior.
	srv := NewKnowledgeServiceServer(spy).WithRouting(adrResolver(), search.ModeLegacy)
	_, err := srv.Search(context.Background(), &cospb.SearchRequest{Query: "musica", Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, 1, spy.calls, "legacy deve chamar o engine.Search normalmente")
	assert.True(t, spy.last.Scope == nil || len(spy.last.Scope.Modules) == 0,
		"legacy não deve aplicar escopo (busca ilimitada, compatível)")
}

func TestKnowledgeService_DefaultMode_Legacy(t *testing.T) {
	t.Parallel()
	spy := &spyEngine{}
	// Sem WithRouting → default legacy, resolver nil → sem routing, sem crash.
	srv := NewKnowledgeServiceServer(spy)
	_, err := srv.Search(context.Background(), &cospb.SearchRequest{Query: "decisão arquitetural", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, spy.calls)
	assert.Nil(t, spy.last.Scope)
}

func TestKnowledgeService_WithRouting_EmptyModeDefaultsLegacy(t *testing.T) {
	t.Parallel()
	spy := &spyEngine{}
	srv := NewKnowledgeServiceServer(spy).WithRouting(adrResolver(), "") // mode "" → legacy
	_, err := srv.Search(context.Background(), &cospb.SearchRequest{Query: "musica", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, spy.calls, "mode vazio deve ser tratado como legacy (sem NoRoute short-circuit)")
}
