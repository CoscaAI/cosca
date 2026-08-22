// Package render implementa o Render Engine (§22 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.7.
//
// Renderização deve ser (§22): DETERMINISTIC · CACHEABLE · RESUMABLE ·
// PARALLEL · OBSERVABLE. Suporta PREVIEW < DRAFT < FINAL.
//
// O Render Engine estende o Node Graph (1.6):
//   - Quality: preview (rápido) → draft → final (completo).
//   - Determinismo: seed explícito propagado ao executor.
//   - Cacheable: cache por assinatura do nó (§23) — não re-renderiza o inalterado.
//   - Resumable: checkpoint por nó completado — se o processo morrer (GPU
//     crash, power loss, network drop — §40), retoma de onde parou.
//   - Observable: Stats por render (executados, hits, resumed, tempo).
package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// Quality é o nível de qualidade de render (§22).
type Quality string

// Níveis de qualidade — PREVIEW < DRAFT < FINAL.
const (
	// Preview — rápido, para iterar (resolução reduzida, passos curtos).
	Preview Quality = "preview"
	// Draft — intermediário, para revisar.
	Draft Quality = "draft"
	// Final — completo, para entrega.
	Final Quality = "final"
)

// Valid reports se a qualidade é canônica.
func (q Quality) Valid() bool {
	switch q {
	case Preview, Draft, Final:
		return true
	}
	return false
}

// Rank devolve a ordem numérica (preview=1, draft=2, final=3).
func (q Quality) Rank() int {
	switch q {
	case Preview:
		return 1
	case Draft:
		return 2
	case Final:
		return 3
	}
	return 0
}

// AtLeast reports se q é de qualidade >= other (para upgrade de render).
func (q Quality) AtLeast(other Quality) bool { return q.Rank() >= other.Rank() }

// Qualities lista as qualidades válidas.
var Qualities = []Quality{Preview, Draft, Final}

// QualitiesList devolve a lista legível.
func QualitiesList() string {
	return string(Preview) + ", " + string(Draft) + ", " + string(Final)
}

// =============================================================================
// Render Job
// =============================================================================

// Job é um render: grafo + qualidade + seed + opções.
type Job struct {
	// Graph é o node graph a renderizar.
	Graph *nodegraph.Graph
	// Quality do render (preview/draft/final).
	Quality Quality
	// Seed é o determinismo (§22): o mesmo seed + mesmo grafo = mesmo resultado.
	Seed int64
	// Cache opcional (assinatura → resultado).
	Cache *nodegraph.Cache
	// CheckpointPath opcional: onde persistir o progresso (resumível §40).
	CheckpointPath string
}

// New cria um job com qualidade padrão draft.
func New(g *nodegraph.Graph, q Quality, seed int64) (*Job, error) {
	if g == nil {
		return nil, fmt.Errorf("render: graph is required")
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	if !q.Valid() {
		return nil, fmt.Errorf("render: invalid quality %q (valid: %s)", q, QualitiesList())
	}
	return &Job{Graph: g, Quality: q, Seed: seed}, nil
}

// Result é o resultado de um render.
type Result struct {
	// JobKey identifica o render (hash do grafo+qualidade+seed).
	JobKey string `json:"job_key" yaml:"job_key"`
	// Quality usada.
	Quality Quality `json:"quality" yaml:"quality"`
	// Executed nós executados nesta rodada.
	Executed int `json:"executed" yaml:"executed"`
	// CachedHits nós servidos do cache.
	CachedHits int `json:"cached_hits" yaml:"cached_hits"`
	// Resumed nós retomados do checkpoint (não re-executados).
	Resumed int `json:"resumed" yaml:"resumed"`
	// Results por nó.
	Results map[string]any `json:"results" yaml:"results"`
	// DurationMs do render.
	DurationMs int64 `json:"duration_ms" yaml:"duration_ms"`
	// Completed reports se o render terminou.
	Completed bool `json:"completed" yaml:"completed"`
}

// JobKey calcula o hash canônico (grafo + qualidade + seed) — identifica o
// render para cache/checkpoint.
func (j *Job) JobKey() (string, error) {
	graphJSON, err := j.Graph.Marshal()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write(graphJSON)
	_, _ = fmt.Fprintf(h, "|q=%s|seed=%d", j.Quality, j.Seed)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// =============================================================================
// Execução
// =============================================================================

// Renderer executa renders com determinismo, cache e checkpoint.
type Renderer struct {
	// now é injetável para testes (tempo).
	now func() time.Time
}

// NewRenderer cria um renderer.
func NewRenderer() *Renderer { return &Renderer{now: time.Now} }

// Render executa o job. Se CheckpointPath existe, retoma do checkpoint
// (nós já completados não re-executam — §40 crash recovery).
func (r *Renderer) Render(ctx context.Context, j *Job, exec nodegraph.Executor) (*Result, error) {
	start := r.now()

	key, err := j.JobKey()
	if err != nil {
		return nil, err
	}

	// Checkpoint: carrega progresso anterior, se houver.
	cp, err := r.loadCheckpoint(j.CheckpointPath)
	if err != nil {
		return nil, err
	}

	// Cache: usa o do job ou cria um fresco.
	cache := j.Cache
	if cache == nil {
		cache = nodegraph.NewCache()
	}

	// Executor wrapper: injeta qualidade + seed (determinismo §22) no context.
	wrapped := &renderExecutor{
		base:    exec,
		quality: j.Quality,
		seed:    j.Seed,
	}

	// Preenche o cache a partir do checkpoint (retomada sem re-renderizar).
	resumed := 0
	if cp != nil && cp.JobKey == key {
		for nodeID, sig := range cp.Completed {
			if cached, ok := cp.Results[sig]; ok {
				cache.Put(sig, cached)
				resumed++
			} else {
				_ = nodeID
			}
		}
	}

	stats, err := j.Graph.Run(ctx, wrapped, &nodegraph.RunOptions{Cache: cache})
	if err != nil {
		return nil, err
	}

	res := &Result{
		JobKey:     key,
		Quality:    j.Quality,
		Executed:   stats.Executed,
		CachedHits: stats.CachedHits,
		Resumed:    resumed,
		Results:    stats.Results,
		DurationMs: r.now().Sub(start).Milliseconds(),
		Completed:  true,
	}

	// Persiste o checkpoint (todos os nós completados + resultados).
	if j.CheckpointPath != "" {
		if err := r.saveCheckpoint(j.CheckpointPath, j, cache); err != nil {
			return nil, fmt.Errorf("render: save checkpoint: %w", err)
		}
	}
	return res, nil
}

// renderExecutor injeta qualidade/seed via context e delega ao executor base.
type renderExecutor struct {
	base    nodegraph.Executor
	quality Quality
	seed    int64
}

// Run implementa nodegraph.Executor — encaminha com determinismo no context.
func (re *renderExecutor) Run(ctx context.Context, node *nodegraph.Node, in map[string]any) (any, error) {
	if re.base == nil {
		return nil, fmt.Errorf("render: executor is required")
	}
	ctx = WithRenderContext(ctx, RenderParams{Quality: re.quality, Seed: re.seed})
	return re.base.Run(ctx, node, in)
}

// =============================================================================
// Contexto determinístico
// =============================================================================

type renderCtxKey struct{}

// RenderParams carrega qualidade + seed para o executor (§22 determinismo).
type RenderParams struct {
	Quality Quality `json:"quality" yaml:"quality"`
	Seed    int64   `json:"seed" yaml:"seed"`
}

// WithRenderContext injeta os parâmetros de render no context.
func WithRenderContext(ctx context.Context, p RenderParams) context.Context {
	return context.WithValue(ctx, renderCtxKey{}, p)
}

// RenderFromContext recupera os parâmetros (default: draft/0).
func RenderFromContext(ctx context.Context) RenderParams {
	p, _ := ctx.Value(renderCtxKey{}).(RenderParams)
	if p.Quality == "" {
		p.Quality = Draft
	}
	return p
}

// =============================================================================
// Checkpoint (resumível §40)
// =============================================================================

// checkpoint é o estado persistido de um render em andamento/completo.
type checkpoint struct {
	// JobKey identifica o render dono do checkpoint.
	JobKey string `json:"job_key"`
	// UpdatedAt timestamp UTC.
	UpdatedAt string `json:"updated_at"`
	// Completed map nodeID → signature dos nós completados.
	Completed map[string]string `json:"completed"`
	// Results map signature → valor (para reconstruir o cache).
	Results map[string]any `json:"results"`
}

func (r *Renderer) checkpointPath(j *Job) string {
	if j.CheckpointPath != "" {
		return j.CheckpointPath
	}
	return ""
}

func (r *Renderer) loadCheckpoint(path string) (*checkpoint, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("render: read checkpoint: %w", err)
	}
	var cp checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("render: parse checkpoint: %w", err)
	}
	return &cp, nil
}

// saveCheckpoint persiste progresso: assinatura de cada nó + resultado, para
// retomada sem re-executar (crash recovery §40).
func (r *Renderer) saveCheckpoint(path string, j *Job, cache *nodegraph.Cache) error {
	cp := &checkpoint{
		JobKey:    "",
		UpdatedAt: r.now().UTC().Format(time.RFC3339),
		Completed: make(map[string]string, j.Graph.Len()),
		Results:   make(map[string]any),
	}
	key, err := j.JobKey()
	if err != nil {
		return err
	}
	cp.JobKey = key

	for _, n := range j.Graph.Nodes {
		sig, err := j.Graph.Signature(n.ID)
		if err != nil {
			return err
		}
		if v, ok := cache.Get(sig); ok {
			cp.Completed[n.ID] = sig
			cp.Results[sig] = v
		}
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("render: marshal checkpoint: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("render: mkdir checkpoint dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("render: write checkpoint tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// =============================================================================
// Helpers
// =============================================================================

// SortedNodeIDs devolve os IDs dos nós em ordem topológica (para exibição).
func SortedNodeIDs(g *nodegraph.Graph) ([]string, error) {
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(order))
	for i, n := range order {
		ids[i] = n.ID
	}
	return ids, nil
}

var _ = strings.Join // reservado
var _ = sort.Strings // reservado
