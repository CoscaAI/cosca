package search

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fake engine ────────────────────────────────────────────────────────────

// fakeLayeredEngine implements LayeredSearcher with a pluggable Search func.
type fakeLayeredEngine struct {
	searchFunc func(ctx context.Context, params SearchParams) (*SearchResults, error)
}

func (f *fakeLayeredEngine) Search(ctx context.Context, params SearchParams) (*SearchResults, error) {
	if f.searchFunc != nil {
		return f.searchFunc(ctx, params)
	}
	return &SearchResults{}, nil
}

func (f *fakeLayeredEngine) Query(ctx context.Context, _ string) (*SearchResults, error) {
	return &SearchResults{}, nil
}

// ftsResults builds a fake FTS hit set.
func ftsResults(idsAndText ...string) []SearchResult {
	var out []SearchResult
	for _, text := range idsAndText {
		parts := strings.SplitN(text, "|", 2)
		id := parts[0]
		content := "backup"
		if len(parts) == 2 {
			content = parts[1]
		}
		out = append(out, SearchResult{ID: id, Title: "T-" + id, Content: content, Type: ResultDocument, Source: "fts"})
	}
	return out
}

// vectorResults builds a fake vector hit set with descending similarity.
func vectorResults(n int) []SearchResult {
	var out []SearchResult
	for i := 0; i < n; i++ {
		out = append(out, SearchResult{
			ID:      "vec-" + string(rune('a'+i)),
			Score:   1.0 - float64(i)*0.05,
			Content: "semantic content " + strings.Repeat("x", i),
			Source:  "vector",
		})
	}
	return out
}

// vectorOnParams returns a Search func that returns fts for FTS calls and vec
// for vector calls (mirrors how the real engine gates phases by params).
func vectorOnParams(fts, vec []SearchResult) func(context.Context, SearchParams) (*SearchResults, error) {
	return func(_ context.Context, params SearchParams) (*SearchResults, error) {
		if params.EnableVector && !params.EnableFTS {
			return &SearchResults{Results: vec, TotalCount: len(vec)}, nil
		}
		return &SearchResults{Results: fts, TotalCount: len(fts)}, nil
	}
}

// ── Layer.String ────────────────────────────────────────────────────────────

func TestLayerString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		layer Layer
		want  string
	}{
		{LayerFTS5, "L1-fts5"},
		{LayerBM25, "L2-bm25"},
		{LayerVector, "L3-vector"},
		{LayerLLM, "L4-llm"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.layer.String())
		})
	}
}

func TestLayerString_Unknown(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "L5-unknown", Layer(4).String())
	assert.Equal(t, "L0-unknown", Layer(-1).String())
}

// ── Defaults ────────────────────────────────────────────────────────────────

func TestDefaultLayeredConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultLayeredConfig()
	assert.Equal(t, 0, cfg.MinFTSCount, "0 = nunca parar em L1; sempre alcançar o vetor (L3)")
	assert.Equal(t, 999.0, cfg.MinBM25Score, "999 = BM25 sozinho nunca confirma L2")
	assert.Equal(t, 8, cfg.VectorTopK)
	assert.Equal(t, 4, cfg.MaxLayers)
	assert.False(t, cfg.AllowLLM, "LLM deve ser opt-in (default FALSE)")
}

func TestNormalizeLayeredConfig_ZeroFallsBackToDefaults(t *testing.T) {
	t.Parallel()
	cfg := normalizeLayeredConfig(LayeredConfig{})
	assert.Equal(t, 0, cfg.MinFTSCount, "0 = nunca parar em L1; sempre alcançar o vetor (L3)")
	assert.Equal(t, 999.0, cfg.MinBM25Score, "999 = BM25 sozinho nunca confirma L2")
	assert.Equal(t, 8, cfg.VectorTopK)
	assert.Equal(t, 4, cfg.MaxLayers)
}

func TestNormalizeLayeredConfig_ClampsMaxLayers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, normalizeLayeredConfig(LayeredConfig{MaxLayers: 1}).MaxLayers)
	assert.Equal(t, 4, normalizeLayeredConfig(LayeredConfig{MaxLayers: 9}).MaxLayers)
	assert.Equal(t, 4, normalizeLayeredConfig(LayeredConfig{MaxLayers: 0}).MaxLayers)
}

// ── L1 — para quando FTS5 é suficiente (MinFTSCount explícito) ─────────────

func TestSearch_AlwaysReachesL3WithDefaults(t *testing.T) {
	t.Parallel()
	// Com os defaults atuais (MinFTSCount=0, MinBM25Score=999) a escada NUNCA
	// para em L1/L2 — mesmo com hits FTS5 "bons" ela percorre até o vetor (L3).
	engine := &fakeLayeredEngine{
		searchFunc: vectorOnParams(
			ftsResults("a|backup plan", "b|backup strategy", "c|backup restore"),
			vectorResults(3),
		),
	}
	ls := NewLayeredSearch(engine, DefaultLayeredConfig())

	res, err := ls.Search(context.Background(), "backup")
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, LayerVector, res.Layer, "defaults forçam a busca até L3 (vetor)")
	assert.True(t, res.StoppedEarly, "parou antes do LLM (0 tokens de IA)")
	assert.False(t, res.Stats.LLMCalled)
	assert.Equal(t, 3, res.Stats.FTS5Hits)
	assert.Equal(t, 3, res.Stats.BM25Hits)
	assert.Equal(t, 3, res.Stats.VectorHits)
	assert.Contains(t, res.Reason, "raciocínio não habilitado")
	// Vector hits REFINE, never replace: FTS (a,b,c) + vector (vec-a,b,c)
	// are merged and re-ranked, so FTS-only tables stay visible.
	assert.Len(t, res.Results, 6)
	assert.Equal(t, "a", res.Results[0].ID, "FTS hit com score mais alto fica no topo")
}

func TestSearch_StopsAtL1WhenFTS5GoodEnough(t *testing.T) {
	t.Parallel()
	// 3+ hits todos contendo o termo "backup" → L1 responde sem IA.
	// (MinFTSCount explícito: com o default 0, L1 nunca para.)
	engine := &fakeLayeredEngine{
		searchFunc: func(_ context.Context, _ SearchParams) (*SearchResults, error) {
			return &SearchResults{Results: ftsResults("a|backup plan", "b|backup strategy", "c|backup restore")}, nil
		},
	}
	cfg := DefaultLayeredConfig()
	cfg.MinFTSCount = 3
	ls := NewLayeredSearch(engine, cfg)

	res, err := ls.Search(context.Background(), "backup")
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, LayerFTS5, res.Layer)
	assert.True(t, res.StoppedEarly, "L1 parou antes de qualquer IA")
	assert.False(t, res.Stats.LLMCalled)
	assert.Equal(t, 3, res.Stats.FTS5Hits)
	assert.Contains(t, res.Reason, "FTS5 suficiente")
	assert.Len(t, res.Results, 3)
}

func TestSearch_L1DoesNotStopBelowMinFTSCount(t *testing.T) {
	t.Parallel()
	// Apenas 2 hits (MinFTSCount=3) → L1 não para; BM25 deve confirmar (L2).
	// (Limiares explícitos: com os defaults, a busca iria até L3.)
	engine := &fakeLayeredEngine{
		searchFunc: func(_ context.Context, _ SearchParams) (*SearchResults, error) {
			return &SearchResults{Results: ftsResults("a|backup plan", "b|backup strategy")}, nil
		},
	}
	cfg := DefaultLayeredConfig()
	cfg.MinFTSCount = 3
	cfg.MinBM25Score = 0.35
	ls := NewLayeredSearch(engine, cfg)

	res, err := ls.Search(context.Background(), "backup")
	require.NoError(t, err)

	assert.Equal(t, LayerBM25, res.Layer)
	assert.True(t, res.StoppedEarly)
	assert.False(t, res.Stats.LLMCalled)
	assert.Equal(t, 2, res.Stats.FTS5Hits)
	assert.Equal(t, 2, res.Stats.BM25Hits)
	assert.GreaterOrEqual(t, res.Results[0].Score, 0.35, "melhor candidato BM25 deve superar o limiar")
}

// ── L2 — BM25 confirma (MinBM25Score explícito) ─────────────────────────────

func TestSearch_StopsAtL2WhenBM25Confirms(t *testing.T) {
	t.Parallel()
	// 2 hits reais contendo "backup" → L2 confirma e para antes de vector/LLM.
	// (MinBM25Score explícito: com o default 999, L2 nunca para.)
	engine := &fakeLayeredEngine{
		searchFunc: func(_ context.Context, _ SearchParams) (*SearchResults, error) {
			return &SearchResults{Results: ftsResults("a|backup automation", "b|backup policy")}, nil
		},
	}
	cfg := DefaultLayeredConfig()
	cfg.MinFTSCount = 3
	cfg.MinBM25Score = 0.35
	ls := NewLayeredSearch(engine, cfg)

	res, err := ls.Search(context.Background(), "backup")
	require.NoError(t, err)

	assert.Equal(t, LayerBM25, res.Layer)
	assert.True(t, res.StoppedEarly)
	assert.False(t, res.Stats.LLMCalled)
	assert.Zero(t, res.Stats.VectorHits, "L3 nunca deve rodar quando L2 para")
}

// ── L3 — vector disponível vs indisponível ──────────────────────────────────

func TestSearch_FallsToL3WhenVectorAvailable(t *testing.T) {
	t.Parallel()
	// Sem hits FTS → L1/L2 insuficientes. Vector disponível → refina e segue.
	engine := &fakeLayeredEngine{
		searchFunc: vectorOnParams(nil, vectorResults(10)),
	}
	cfg := DefaultLayeredConfig()
	cfg.AllowLLM = true
	ls := NewLayeredSearch(engine, cfg)

	res, err := ls.Search(context.Background(), "termo sem match exato")
	require.NoError(t, err)

	assert.Zero(t, res.Stats.FTS5Hits)
	assert.Zero(t, res.Stats.BM25Hits)
	assert.Equal(t, 8, res.Stats.VectorHits, "L3 mantém no máximo VectorTopK=8")
	assert.Equal(t, LayerLLM, res.Layer, "com vector + AllowLLM, segue para L4")
	assert.False(t, res.StoppedEarly)
	assert.True(t, res.Stats.LLMCalled)
	assert.Len(t, res.Results, 8)
	assert.Equal(t, "vector", res.Results[0].Source)
}

func TestSearch_SkipsL3WhenVectorUnavailable(t *testing.T) {
	t.Parallel()
	// Sem hits FTS e sem vetores (provedor de embeddings indisponível) →
	// L3 pula com graça (VectorHits=0) e continua para L4.
	engine := &fakeLayeredEngine{
		searchFunc: vectorOnParams(nil, nil),
	}
	cfg := DefaultLayeredConfig()
	cfg.AllowLLM = true
	ls := NewLayeredSearch(engine, cfg)

	res, err := ls.Search(context.Background(), "query qualquer")
	require.NoError(t, err)

	assert.Zero(t, res.Stats.VectorHits, "provedor indisponível → 0 vetores, skip gracioso")
	assert.Equal(t, LayerLLM, res.Layer)
	assert.True(t, res.Stats.LLMCalled)
	assert.Empty(t, res.Results)
}

// ── L4 — apenas com AllowLLM ────────────────────────────────────────────────

func TestSearch_L4OnlyWhenAllowLLM(t *testing.T) {
	t.Parallel()
	// Mesmo cenário (sem hits determinísticos): sem AllowLLM para antes do LLM;
	// com AllowLLM marca LLMCalled e StoppedEarly=false.
	engine := &fakeLayeredEngine{searchFunc: vectorOnParams(nil, nil)}

	t.Run("AllowLLM=false para em L3, 0 tokens IA", func(t *testing.T) {
		ls := NewLayeredSearch(engine, DefaultLayeredConfig())
		res, err := ls.Search(context.Background(), "nada")
		require.NoError(t, err)

		assert.Equal(t, LayerVector, res.Layer)
		assert.True(t, res.StoppedEarly)
		assert.False(t, res.Stats.LLMCalled)
		assert.Contains(t, res.Reason, "raciocínio não habilitado")
	})

	t.Run("AllowLLM=true alcança L4", func(t *testing.T) {
		cfg := DefaultLayeredConfig()
		cfg.AllowLLM = true
		ls := NewLayeredSearch(engine, cfg)
		res, err := ls.Search(context.Background(), "nada")
		require.NoError(t, err)

		assert.Equal(t, LayerLLM, res.Layer)
		assert.False(t, res.StoppedEarly, "chegou ao LLM → não parou cedo")
		assert.True(t, res.Stats.LLMCalled)
		assert.Contains(t, res.Reason, "raciocínio necessário")
	})
}

func TestSearch_L4HookInvokedOnlyWhenAllowed(t *testing.T) {
	t.Parallel()
	engine := &fakeLayeredEngine{searchFunc: vectorOnParams(nil, nil)}

	hookCalls := 0
	ls := NewLayeredSearch(engine, DefaultLayeredConfig())
	ls.SetLLMHook(func(_ context.Context, query string, candidates []SearchResult) error {
		hookCalls++
		assert.Equal(t, "nada", query)
		assert.Empty(t, candidates)
		return nil
	})

	// AllowLLM=false → hook nunca invocado.
	_, err := ls.Search(context.Background(), "nada")
	require.NoError(t, err)
	assert.Zero(t, hookCalls, "sem AllowLLM o hook de L4 não deve rodar")

	// AllowLLM=true → hook invocado uma vez.
	cfg := DefaultLayeredConfig()
	cfg.AllowLLM = true
	ls2 := NewLayeredSearch(engine, cfg)
	ls2.SetLLMHook(func(_ context.Context, _ string, _ []SearchResult) error {
		hookCalls++
		return nil
	})
	_, err = ls2.Search(context.Background(), "nada")
	require.NoError(t, err)
	assert.Equal(t, 1, hookCalls)
}

// ── MaxLayers — teto rígido ─────────────────────────────────────────────────

func TestSearch_MaxLayersCaps(t *testing.T) {
	t.Parallel()
	// 1 hit não on-topic → nem L1 nem L2 confirmam; MaxLayers decide o teto.
	engine := &fakeLayeredEngine{
		searchFunc: vectorOnParams(
			ftsResults("a|conteúdo sem relação nenhuma"),
			vectorResults(3),
		),
	}

	t.Run("MaxLayers=1 para em L1", func(t *testing.T) {
		cfg := DefaultLayeredConfig()
		cfg.MaxLayers = 1
		res, err := NewLayeredSearch(engine, cfg).Search(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, LayerFTS5, res.Layer)
		assert.True(t, res.StoppedEarly)
		assert.Zero(t, res.Stats.BM25Hits)
		assert.Zero(t, res.Stats.VectorHits)
	})

	t.Run("MaxLayers=2 para em L2", func(t *testing.T) {
		cfg := DefaultLayeredConfig()
		cfg.MaxLayers = 2
		res, err := NewLayeredSearch(engine, cfg).Search(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, LayerBM25, res.Layer)
		assert.True(t, res.StoppedEarly)
		assert.Zero(t, res.Stats.VectorHits, "L3 não roda com MaxLayers=2")
	})

	t.Run("MaxLayers=3 para em L3 mesmo com AllowLLM", func(t *testing.T) {
		cfg := DefaultLayeredConfig()
		cfg.MaxLayers = 3
		cfg.AllowLLM = true
		res, err := NewLayeredSearch(engine, cfg).Search(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, LayerVector, res.Layer)
		assert.True(t, res.StoppedEarly)
		assert.False(t, res.Stats.LLMCalled, "MaxLayers=3 nunca alcança L4")
		// Merge: 1 FTS + 3 vector = 4 candidatos (vector refine, não substitui).
		assert.Len(t, res.Results, 4)
	})
}

// ── Erros e pré-condições ───────────────────────────────────────────────────

func TestSearch_EmptyQueryErrors(t *testing.T) {
	t.Parallel()
	ls := NewLayeredSearch(&fakeLayeredEngine{}, DefaultLayeredConfig())
	_, err := ls.Search(context.Background(), "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestSearch_NilEngineErrors(t *testing.T) {
	t.Parallel()
	ls := NewLayeredSearch(nil, DefaultLayeredConfig())
	_, err := ls.Search(context.Background(), "backup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine not configured")
}

// ── Heurísticas documentadas ────────────────────────────────────────────────

func TestFTS5GoodEnough(t *testing.T) {
	t.Parallel()
	query := "backup strategy"

	t.Run("hits insuficientes", func(t *testing.T) {
		assert.False(t, fts5GoodEnough(ftsResults("a|backup strategy"), query, 3))
	})
	t.Run("todos os top hits contêm todos os termos", func(t *testing.T) {
		hits := ftsResults("a|backup strategy", "b|backup strategy", "c|backup strategy")
		assert.True(t, fts5GoodEnough(hits, query, 3))
	})
	t.Run("top hit sem termo parcial", func(t *testing.T) {
		hits := ftsResults("a|apenas backup", "b|backup strategy", "c|backup strategy")
		assert.False(t, fts5GoodEnough(hits, query, 3))
	})
	t.Run("case-insensitive", func(t *testing.T) {
		hits := []SearchResult{{ID: "a", Title: "BACKUP", Content: "STRATEGY plan"}}
		assert.True(t, fts5GoodEnough(hits, "backup strategy", 1))
	})
}

func TestBM25Threshold(t *testing.T) {
	t.Parallel()
	// Documento com o termo uma vez (comprimento médio) → score ≈ 1.0 >> 0.35.
	onTopic := bm25("backup automation for nightly jobs", "backup")
	assert.Greater(t, onTopic, 0.35, "hit genuíno deve passar o limiar de L2")

	// Documento sem qualquer termo → 0, fica abaixo do limiar.
	offTopic := bm25("completely unrelated content here", "backup")
	assert.Less(t, offTopic, 0.35)

	assert.Zero(t, bm25("", "backup"))
	assert.Zero(t, bm25("conteúdo", ""))
}

// ── Hybrid-first: L3 bounded pelos candidatos lexicais ─────────────────────

func TestLayeredSearch_PassesCandidatesToVectorLayer(t *testing.T) {
	engine := &fakeLayeredEngine{}
	var gotVec SearchParams
	engine.searchFunc = func(_ context.Context, params SearchParams) (*SearchResults, error) {
		if params.EnableVector && !params.EnableFTS {
			gotVec = params
			return &SearchResults{Results: vectorResults(2), TotalCount: 2}, nil
		}
		return &SearchResults{
			Results: ftsResults("chunks_fts_1|kernel runtime", "chunks_fts_2|kernel boot"),
			TotalCount: 2,
		}, nil
	}

	ls := NewLayeredSearch(engine, LayeredConfig{VectorTopK: 3})
	res, err := ls.Search(context.Background(), "kernel")
	require.NoError(t, err)
	require.NotNil(t, res)

	// A L3 recebe os IDs FTS dos candidatos BM25 + o pool de recência.
	require.Len(t, gotVec.CandidateIDs, 2, "L3 deve receber os candidatos lexicais")
	assert.ElementsMatch(t, []string{"chunks_fts_1", "chunks_fts_2"}, gotVec.CandidateIDs)
	assert.Equal(t, 250, gotVec.CandidatePool, "pool de recência padrão")
	assert.Equal(t, 3, gotVec.Limit)
}

func TestLayeredSearch_NoCandidatesReachL3(t *testing.T) {
	engine := &fakeLayeredEngine{}
	var gotVec SearchParams
	engine.searchFunc = func(_ context.Context, params SearchParams) (*SearchResults, error) {
		if params.EnableVector && !params.EnableFTS {
			gotVec = params
			return &SearchResults{Results: vectorResults(2), TotalCount: 2}, nil
		}
		// Sem hits FTS → sem candidatos lexicais → L3 sem CandidateIDs.
		return &SearchResults{Results: nil, TotalCount: 0}, nil
	}

	ls := NewLayeredSearch(engine, LayeredConfig{VectorTopK: 3})
	res, err := ls.Search(context.Background(), "algo-bem-semantico")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Nil(t, gotVec.CandidateIDs, "sem candidatos lexicais, L3 deve cair no brute-force")
}
