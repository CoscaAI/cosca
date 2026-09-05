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
