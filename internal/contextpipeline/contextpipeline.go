// Package contextpipeline — INTEGRAÇÃO do Context Compiler (ADR-035, F6).
//
// Orquestra os 4 pacotes do Context Compiler num único ponto que o Executor
// do orchestration pode usar:
//
//	contextrouter (F4: decide L0/L1/L2)
//	  → contextcompile + budget (F2/F3: estado operacional com tokens por seção)
//	  → LLM (resposta)
//	  → actiondecode (F1: instruction packet com fail-open)
//	  → contextmetrics (F5: economia registrada no cost)
//
// ADITIVO: quando o pipeline está nil, o Executor se comporta EXATAMENTE como
// antes. A integração é opt-in via ExecutorConfig (ContextPipeline).
package contextpipeline

import (
	"time"

	"github.com/CoscaAI/cosca/internal/actiondecode"
	"github.com/CoscaAI/cosca/internal/contextcompile"
	"github.com/CoscaAI/cosca/internal/contextmetrics"
	"github.com/CoscaAI/cosca/internal/contextrouter"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/taskaffinity"
	"github.com/CoscaAI/cosca/internal/taskphase"
)

// Config configura o pipeline de contexto.
type Config struct {
	// MaxTokens é o teto do contexto compilado (F3). <= 0 = sem teto.
	MaxTokens int
	// Constraints são as restrições da tarefa (vão para a seção CONSTRAINTS).
	Constraints []string
	// Allowed/Forbidden delimitam o espaço de ação.
	Allowed   []string
	Forbidden []string
}

// Pipeline é o orquestrador do Context Compiler. Nil-safe: quando o Executor
// não o configura, nada muda.
type Pipeline struct {
	cfg Config
	// Track acumula as métricas de sessão (F5).
	Track *contextmetrics.Track
}

// New cria um pipeline de contexto. O Track é inicializado para acumular
// métricas da sessão.
func New(cfg Config) *Pipeline {
	return &Pipeline{cfg: cfg, Track: contextmetrics.NewTrack()}
}

// BuildContext decide a camada e compila o estado operacional a partir do
// PipelineData. Devolve o contexto compilado (para injetar no system prompt)
// e a camada decidida (contrato do orchestration.PipelineLevel). Sem dados de
// conhecimento/memória, o nível cai para L1 (o Kernel não decide sem evidência).
func (p *Pipeline) BuildContext(prompt string, data *orchestration.PipelineData) (string, orchestration.PipelineLevel) {
	if p == nil || data == nil {
		return "", orchestration.PipelineLevel{Name: "L1"}
	}

	hasKnowledge := data.KnowledgeResults != nil && len(data.KnowledgeResults.Results) > 0
	hasMemory := len(data.MemoryResults) > 0 || len(data.RetrievedMemories) > 0

	// ═══ NOVO (ADR-045 F4): Task-Aware Search ═══
	// Aditivo: quando data.TaskContext é nil (ou Profile nil), o fluxo é
	// idêntico ao atual — zero regressão.
	if data.TaskContext != nil && data.TaskContext.Profile != nil {
		profile := data.TaskContext.Profile
		detection := data.TaskContext.PhaseDetection

		// 1. Ajustar SearchParams pela fase (ANTES da busca downstream).
		// O contextpipeline não executa a busca — os resultados chegam prontos
		// no PipelineData — mas os parâmetros ajustados fluem para o estágio
		// de busca do executor (DESIGN-001 §5.4 passo 1).
		if detection != nil {
			data.SearchParams = taskphase.PhaseSearchParams(data.SearchParams, *detection)
		}

		// 2. AffinityRerank sobre os resultados (DEPOIS da busca, ANTES do
		// compile). Os resultados são re-ponderados por afinidade com a tarefa
		// (DESIGN-001 §5.4 passo 2).
		if data.KnowledgeResults != nil && len(data.KnowledgeResults.Results) > 0 {
			data.KnowledgeResults.Results = rerankKnowledgeResults(data.KnowledgeResults.Results, profile)
		}
	}
	// ═══ FIM NOVO ═══

	// F4: decide a camada (confiança do deliberate + disponibilidade de
	// evidência). Sem deliberação real, usamos a disponibilidade de evidência
	// como proxy conservador (L1 quando há conhecimento, L2 quando não há
	// nada — o LLM precisa do contexto completo para não inventar).
	conf := contextrouter.Confidence{Final: pipelineConfidence(data)}
	level := contextrouter.DecideLevelWithInfo(conf, hasKnowledge, hasMemory)
	deterministic := contextrouter.Emit(level, conf, hasKnowledge)

	// F2/F3: compila o estado operacional com budget por seção.
	cc := contextcompile.CompileWithBudget(contextcompile.CompileInput{
		Prompt:      prompt,
		Agent:       data.ResolvedAgent,
		Role:        data.AgentRole,
		Knowledge:   data.KnowledgeResults,
		Memory:      mergeMemory(data),
		Constraints: p.cfg.Constraints,
		Allowed:     p.cfg.Allowed,
		Forbidden:   p.cfg.Forbidden,
	}, contextcompile.TokenBudget{MaxTokens: p.cfg.MaxTokens})

	// ═══ NOVO (ADR-045 F4): Profile no CompiledContext ═══
	// Enriquecer a seção STATE com stack/target do perfil de tarefa
	// (DESIGN-001 §5.5). Nil-safe: profile nil → no-op.
	if data.TaskContext != nil && data.TaskContext.Profile != nil {
		cc.EnrichWithProfile(data.TaskContext.Profile)
	}
	// ═══ FIM NOVO ═══

	// F5: registra as estatísticas da compilação (economia vs contexto bruto).
	p.Track.Add(contextmetrics.CompileStats{
		Level:            level.String(),
		RawContextTokens: estimateRawTokens(data),
		CompiledTokens:   estimateCompiledTokens(cc),
		Deterministic:    deterministic,
		At:               time.Now(),
	})

	return cc.Text, orchestration.PipelineLevel{Name: level.String(), Deterministic: deterministic}
}

// DecodeAction interpreta a resposta do LLM como instruction packet (F1).
// Sempre fail-open: prosa é devolvida como texto, nunca quebra.
func DecodeAction(raw string) actiondecode.Packet {
	return actiondecode.Decode(raw)
}

// pipelineConfidence extrai a confiança da deliberação, se houver. Sem
// deliberação, usa 0 — o que força L1/L2 (conservador, não inventa).
func pipelineConfidence(data *orchestration.PipelineData) float64 {
	if data == nil {
		return 0
	}
	// O PipelineData não carrega a confiança da deliberação hoje; quando a F6
	// estiver conectada ao Deliberator, este valor virá da DeliberationTrace.
	// Por ora, sem deliberação real, o proxy é a presença de conhecimento:
	// conhecimento suficiente → confiança média (L1); sem → L2.
	if data.KnowledgeResults != nil && len(data.KnowledgeResults.Results) >= 3 {
		return 0.60 // reservations → L1 (LLM com contexto relevante)
	}
	return 0.0 // escalate → L2 (LLM com contexto completo)
}

// mergeMemory combina as memórias do PipelineData (duas fontes possíveis).
func mergeMemory(data *orchestration.PipelineData) []orchestration.MemoryRecord {
	var out []orchestration.MemoryRecord
	if data == nil {
		return out
	}
	out = append(out, data.MemoryResults...)
	out = append(out, data.RetrievedMemories...)
	return out
}

// estimateRawTokens estima o custo do contexto BRUTO (contrafactual honesto):
// todos os resultados de conhecimento/memória sem compilação.
func estimateRawTokens(data *orchestration.PipelineData) int {
	if data == nil {
		return 0
	}
	total := 0
	if data.KnowledgeResults != nil {
		for _, r := range data.KnowledgeResults.Results {
			total += len(r.Content) / 4
		}
	}
	for _, m := range mergeMemory(data) {
		total += len(m.Content) / 4
	}
	return total
}

// estimateCompiledTokens estima o custo do contexto compilado (o que o LLM
// realmente recebeu).
func estimateCompiledTokens(cc *contextcompile.CompiledContext) int {
	if cc == nil {
		return 0
	}
	return len(cc.Text) / 4
}

// rerankKnowledgeResults aplica o AffinityRerank (taskaffinity) sobre os
// resultados de conhecimento do PipelineData. O taskaffinity opera sobre
// search.SearchResult; o pipeline carrega orchestration.KnowledgeSearchResult
// — o adapter converte ida-e-volta preservando os campos extras (Epistemic,
// PolicyState) via lookup por ID.
//
// Nil-safe: profile nil ou results vazio → retorna results inalterados.
func rerankKnowledgeResults(results []orchestration.KnowledgeSearchResult, profile *taskaffinity.TaskProfile) []orchestration.KnowledgeSearchResult {
	if profile == nil || len(results) == 0 {
		return results
	}

	// Preserva os originais por ID para o round-trip dos campos extras.
	original := make(map[string]orchestration.KnowledgeSearchResult, len(results))
	converted := make([]search.SearchResult, len(results))
	for i, r := range results {
		original[r.ID] = r
		converted[i] = search.SearchResult{
			ID:           r.ID,
			Title:        r.Title,
			Content:      r.Content,
			Snippet:      r.Snippet,
			Score:        r.Score,
			DocumentPath: r.DocumentPath,
		}
	}

	reranked := taskaffinity.AffinityRerank(converted, profile)

	out := make([]orchestration.KnowledgeSearchResult, len(reranked))
	for i, r := range reranked {
		orig := original[r.ID]
		orig.Score = r.Score
		out[i] = orig
	}
	return out
}
