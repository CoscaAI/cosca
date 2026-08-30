// tools.go — As TOOLS COGNITIVAS do servidor MCP do COSCA.
//
// Cada tool é uma CAPACIDADE COGNITIVA (o que o cérebro faz), NÃO uma função
// crua (banco/HTTP/shell). O servidor é uma camada FINA de tradução: recebe
// `tools/call`, roteia para o Runtime (corpo) / Kernel (cérebro) / órgãos
// (knowledge/memory/vision/trace), e devolve um context packet epistemológico
// honesto (ADR-028 §1).
//
// O struct `Engine` concentra as DEPENDÊNCIAS injetadas na criação: cada campo
// pode ser nil (nil-safe) — sem um órgão, a tool que dele depende devolve um
// erro claro, NUNCA pânico. Read-only-first: `cosca.learn` é a única escrita e
// é GATEADA (AllowWrite).
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/chat/mcp"
	"github.com/CoscaAI/cosca/internal/contentfit"
	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/security"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/vision"
)

// ─── Namespaces das tools cognitivas ──────────────────────────────────────
const (
	ToolRecall  = "cosca.recall"
	ToolContext = "cosca.context"
	ToolLearn   = "cosca.learn"
	ToolObserve = "cosca.observe"
	ToolReason  = "cosca.reason"
	ToolTrace   = "cosca.trace"
	ToolProject = "cosca.project"
	ToolCost    = "cosca.cost"
	ToolCLI     = "cosca.cli"
	ToolSelf    = "cosca.self"
	ToolWeb     = "cosca.web"
)

// VisionFunc é a assinatura de vision.AnalyzeVideo (injetável para teste).
type VisionFunc func(ctx context.Context, video string, opts vision.PipelineOptions) (*vision.PipelineResult, error)

// Engine é o servidor como "sistema nervoso": guarda as dependências que o
// corpo (runtime) e o cérebro (kernel) fornecem. Todos os campos são
// opcionais — nil-safe.
type Engine struct {
	Knowledge *knowledge.Engine
	Memory    *memory.MemoryEngine
	Runtime   *runtime.Runtime
	Kernel    *kernel.EmergencyManager
	Trace     *trace.Store
	Vision    VisionFunc
	Cost      *cost.Store
	// CLIExec executa um subconjunto seguro do CLI do COSCA (leitura/gestão),
	// via NewRootCommand(). Injetado pelo CLI (evita import cycle mcpserver->cli).
	// Recebe os args (sem o binário) e devolve stdout em string.
	CLIExec CLIExecFunc
	// AllowWrite habilita a ÚNICA escrita (cosca.learn) via gate. Read-only-first.
	AllowWrite bool

	// registry é o Capability Registry (MCP Surface). As tools são registradas
	// declarativamente em registerTools() — o Switch/inventário hardcoded sumiu.
	registry *registry
}

// CLIExecFunc executa um comando do CLI do COSCA (allowlist de segurança) e
// devolve o stdout capturado. Definida no pacote cli e injetada no Engine para
// o MCP "operar o CLI" sem virar porta dos fundos.
type CLIExecFunc func(args []string) (string, error)

// ─── Options ───────────────────────────────────────────────────────────────

// Option configura o Engine.
type Option func(*Engine)

// WithKnowledge injeta o Knowledge Engine.
func WithKnowledge(ke *knowledge.Engine) Option { return func(e *Engine) { e.Knowledge = ke } }

// WithMemory injeta o Memory Engine.
func WithMemory(me *memory.MemoryEngine) Option { return func(e *Engine) { e.Memory = me } }

// WithRuntime injeta o Runtime (corpo).
func WithRuntime(rt *runtime.Runtime) Option { return func(e *Engine) { e.Runtime = rt } }

// WithKernel injeta o Kernel (cérebro / kill-switch).
func WithKernel(k *kernel.EmergencyManager) Option { return func(e *Engine) { e.Kernel = k } }

// WithTrace injeta o trace store (flight recorder).
func WithTrace(ts *trace.Store) Option { return func(e *Engine) { e.Trace = ts } }

// WithVision injeta o pipeline de percepção determinística.
func WithVision(fn VisionFunc) Option { return func(e *Engine) { e.Vision = fn } }

// WithCost injeta o store de custo (ADR-031) para a tool cosca.cost.
func WithCost(store *cost.Store) Option { return func(e *Engine) { e.Cost = store } }

// WithCLIExec injeta o executor do CLI (allowlist de comandos seguros).
func WithCLIExec(fn CLIExecFunc) Option { return func(e *Engine) { e.CLIExec = fn } }

// WithAllowWrite habilita a escrita gateada de cosca.learn.
func WithAllowWrite(v bool) Option { return func(e *Engine) { e.AllowWrite = v } }

// NewEngine cria um Engine a partir das opções. Nil-safe: sem opção o campo
// fica nil e a tool que dele depende devolve erro claro.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{}
	for _, opt := range opts {
		opt(e)
	}
	e.registerTools()
	return e
}

// Close libera os recursos SQLite abertos (knowledge/memory/trace). Nil-safe.
func (e *Engine) Close() error {
	var firstErr error
	if e.Knowledge != nil {
		if err := e.Knowledge.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if e.Memory != nil {
		if err := e.Memory.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if e.Trace != nil {
		if err := e.Trace.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ─── Tools (ToolInfo — reuso da estrutura do cliente MCP) ────────────────

// Tools devolve o inventário de tools cognitivas anunciado via tools/list.
// Usa mcp.ToolInfo (mesma estrutura do cliente). Agora deriva do Capability
// Registry — adicionar uma tool nova = registrar um ToolDef, sem switch.
func (e *Engine) Tools() []mcp.ToolInfo {
	if e.registry == nil {
		return []mcp.ToolInfo{}
	}
	defs := e.registry.list()
	tools := make([]mcp.ToolInfo, 0, len(defs))
	for _, d := range defs {
		schema := d.InputSchema
		if schema == "" {
			schema = `{"type":"object","properties":{}}`
		}
		// Descrição agora "carrega a política de custo" (lição do PinchTab):
		// o agente lê o custo ordinal direto do schema, sem prompt separado,
		// e escolhe a tool MAIS BARATA que satisfaz o objetivo (ADR-031).
		desc := d.Description
		if d.Cost != "" {
			desc += fmt.Sprintf(" [cost: %s]", d.Cost)
		}
		tools = append(tools, mcp.ToolInfo{
			Name:        d.Name,
			Description: desc,
			InputSchema: json.RawMessage(schema),
		})
	}
	return tools
}

// hasTool reports se a tool existe no inventário (default-deny, I8).
func (e *Engine) hasTool(name string) bool {
	return e.registry != nil && e.registry.has(name)
}

// ─── CallResult (formato MCP de content) ──────────────────────────────────

// ContentItem é um item de conteúdo de uma tool call result.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallResult é o payload de tools/call no formato MCP content.
type CallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ─── Dispatcher de tools/call ─────────────────────────────────────────────

// Call roteia uma chamada de tool cognitiva. Antes de executar, consulta o
// Kernel (kill-switch): halted/stop → erro fail-closed, NUNCA executa
// (ADR-028 §7.2). Tool desconhecida → erro (default-deny, I8). Nil-safe:
// sem órgão, a tool devolve erro claro em vez de pânico.
func (e *Engine) Call(ctx context.Context, name string, args json.RawMessage) (*CallResult, error) {
	// Kernel gate (cérebro): kill-switch derruba a execução remotamente.
	if e.Kernel != nil && e.Kernel.IsHalted() {
		return nil, fmt.Errorf("cosca.mcp: kernel kill-switch ativo (halted/stop) — execução bloqueada")
	}
	// Nil-safe (contrato do Engine): sem registry, nenhuma tool existe.
	if e.registry == nil {
		return nil, fmt.Errorf("cosca.mcp: capability registry indisponível (engine sem tools)")
	}
	def := e.registry.get(name)
	if def == nil {
		return nil, fmt.Errorf("cosca.mcp: tool %q desconhecida (default-deny)", name)
	}

	// Gate de escrito (PermissionOwner): toda tool de escrita exige autorização
	// explícita (COSCA_MCP_ALLOW_WRITE). Read-only-first (ADR-028 §1.2).
	if def.Permission == PermissionOwner && !e.AllowWrite {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("cosca.mcp: %s é escrita (PermissionOwner) — modo read-only (fail-closed); defina COSCA_MCP_ALLOW_WRITE=1 para permitir. Nada foi feito.", name)}},
		}, nil
	}

	if def.Handler == nil {
		return nil, fmt.Errorf("cosca.mcp: tool %q sem handler registrado", name)
	}
	return def.Handler(ctx, args)
}

// ─── Tool: cosca.recall ───────────────────────────────────────────────────

type recallArgs struct {
	Query     string   `json:"query"`
	Limit     int      `json:"limit"`
	Epistemic []string `json:"epistemic"`
	Path      string   `json:"path"`
}

func (e *Engine) handleRecall(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Knowledge == nil {
		return nil, fmt.Errorf("cosca.recall: knowledge engine indisponível (sem corpo)")
	}
	args, err := requireInput[recallArgs](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, fmt.Errorf("cosca.recall: 'query' é obrigatório")
	}

	params := search.DefaultSearchParams()
	params.Query = args.Query
	if args.Limit > 0 {
		params.Limit = args.Limit
	}
	params.Path = args.Path
	params.Epistemic = args.Epistemic

	results, err := e.Knowledge.Search(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("cosca.recall: busca falhou: %w", err)
	}

	items := make([]ContextItem, 0, len(results.Results))
	if results != nil {
		for _, r := range results.Results {
			items = append(items, ContextItem{
				Content:   contentFromResult(r),
				Source:    sourceFromResult(r),
				Relevance: clamp01(r.Score),
			})
		}
	}
	packet := NewContextPacket(args.Query, items)
	e.recordTraceEvent(packet, "RECALL", fmt.Sprintf("query=%q items=%d", args.Query, len(items)))
	return resultFromToolCall(packet, ToolRecall, statusFor(len(items))), nil
}

// ─── Tool: cosca.context (central) ────────────────────────────────────────

type contextArgs struct {
	Query string `json:"query"`
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

func (e *Engine) handleContext(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := requireInput[contextArgs](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, fmt.Errorf("cosca.context: 'query' é obrigatório")
	}

	var items []ContextItem

	// Camada 1 — conhecimento (knowledge.Engine.Search).
	if e.Knowledge != nil {
		params := search.DefaultSearchParams()
		params.Query = args.Query
		params.Path = args.Path
		if args.Limit > 0 {
			params.Limit = args.Limit
		}
		if results, sErr := e.Knowledge.Search(ctx, params); sErr == nil && results != nil {
			for _, r := range results.Results {
				items = append(items, ContextItem{
					Content:   contentFromResult(r),
					Source:    sourceFromResult(r),
					Relevance: clamp01(r.Score),
				})
			}
		}
	}

	// Camada 2 — memória (memory.Engine.Search) como contexto adicional.
	if e.Memory != nil {
		opts := memory.SearchOptions{}
		if args.Limit > 0 {
			opts.Limit = args.Limit
		}
		if recs, mErr := e.Memory.Search(ctx, args.Query, opts); mErr == nil {
			for _, r := range recs {
				items = append(items, ContextItem{
					Content:   memoryContent(r),
					Source:    string(knowledge.EpistemicEVIDENCE),
					Relevance: memoryRelevance(r),
				})
			}
		}
	}

	if len(items) == 0 && e.Knowledge == nil && e.Memory == nil {
		return nil, fmt.Errorf("cosca.context: sem knowledge nem memory (sem corpo) para contextualizar")
	}

	packet := NewContextPacket(args.Query, items)
	e.recordTraceEvent(packet, "CONTEXT", fmt.Sprintf("query=%q items=%d", args.Query, len(items)))
	return resultFromToolCall(packet, ToolContext, statusFor(len(items))), nil
}

// ─── Tool: cosca.learn (única escrita — GATEADA) ──────────────────────────

type learnArgs struct {
	Content string `json:"content"`
	Type    string `json:"type"`
	Layer   string `json:"layer"`
	Scope   string `json:"scope"`
}

func (e *Engine) handleLearn(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := requireInput[learnArgs](raw)
	if err != nil {
		return nil, err
	}
	// Read-only-first (ADR-028 §1.2): escrita GATEADA. Default fail-closed.
	if !e.AllowWrite {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "cosca.learn: modo read-only (fail-closed) — defina COSCA_MCP_ALLOW_WRITE=1 para permitir escrita; nada foi gravado."}},
		}, nil
	}
	if e.Memory == nil {
		return nil, fmt.Errorf("cosca.learn: memory engine indisponível (sem corpo)")
	}
	if strings.TrimSpace(args.Content) == "" {
		return nil, fmt.Errorf("cosca.learn: 'content' é obrigatório")
	}

	record := memory.MemoryRecord{
		Type:    memory.MemoryType(memoryTypeFor(args.Type)),
		Layer:   memory.MemoryLayer(memoryLayerFor(args.Layer)),
		Scope:   args.Scope,
		Content: args.Content,
		Metadata: map[string]string{
			"source":    string(knowledge.EpistemicEVIDENCE), // nunca FACT sem evidência (ADR-028 §1.2)
			"provenance": string(knowledge.ProvenanceUnknown),
			"agent":     "cosca-mcp",
		},
	}
	saved, sErr := e.Memory.Store(ctx, record)
	if sErr != nil {
		return nil, fmt.Errorf("cosca.learn: falha ao registrar aprendizado: %w", sErr)
	}

	items := []ContextItem{{
		Content:   "Aprendizado registrado (gate ativo): " + args.Content,
		Source:    string(knowledge.EpistemicEVIDENCE),
		Relevance: 1.0,
	}}
	packet := NewContextPacket("learn:"+args.Content, items)
	packet.TraceID = saved.ID
	return resultFromToolCall(packet, ToolLearn, "ok"), nil
}

// ─── Tool: cosca.observe ──────────────────────────────────────────────────

type observeArgs struct {
	Video    string `json:"video"`
	FPS      int    `json:"fps"`
	OCRLang  string `json:"ocr_lang"`
	WorkDir  string `json:"work_dir"`
}

func (e *Engine) handleObserve(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Vision == nil {
		return nil, fmt.Errorf("cosca.observe: pipeline de visão indisponível (sem órgão perceptivo)")
	}
	args, err := requireInput[observeArgs](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Video) == "" {
		return nil, fmt.Errorf("cosca.observe: 'video' é obrigatório")
	}

	opts := vision.PipelineOptions{FPS: args.FPS, OCRLang: args.OCRLang, WorkDir: args.WorkDir}
	result, pErr := e.Vision(ctx, args.Video, opts)
	if pErr != nil {
		return nil, fmt.Errorf("cosca.observe: análise falhou: %w", pErr)
	}

	var items []ContextItem
	if result != nil {
		for _, ev := range result.Events {
			// Texto extraído de vídeo externo (OCR/tela) é conteúdo
			// NÃO-CONFIÁVEL (uma tela pode conter prompt-injection). Envelopamos
			// com contenttrust para o modelo não seguir instrução do vídeo.
			wrapped := contenttrust.Envelope(contenttrust.Default(
				contenttrust.OriginMCP,
				observeContent(ev),
				"observe:"+args.Video,
			))
			items = append(items, ContextItem{
				Content:   wrapped,
				Source:    observeSource(ev),
				Relevance: 1.0,
			})
		}
	}
	packet := NewContextPacket("observe:"+args.Video, items)
	var frames, events int
	if result != nil {
		frames = len(result.Frames)
		events = len(result.Events)
	}
	e.recordTraceEvent(packet, "OBSERVE", fmt.Sprintf("video=%q frames=%d events=%d items=%d",
		args.Video, frames, events, len(items)))
	return resultFromToolCall(packet, ToolObserve, statusFor(len(items))), nil
}

// ─── Tool: cosca.trace ────────────────────────────────────────────────────

type traceArgs struct {
	TraceID string `json:"trace_id"`
}

func (e *Engine) handleTrace(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Trace == nil {
		return nil, fmt.Errorf("cosca.trace: trace store indisponível (sem flight recorder)")
	}
	args, err := requireInput[traceArgs](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.TraceID) == "" {
		return nil, fmt.Errorf("cosca.trace: 'trace_id' é obrigatório")
	}

	events, tErr := e.Trace.Get(args.TraceID)
	if tErr != nil {
		return nil, fmt.Errorf("cosca.trace: recuperação falhou: %w", tErr)
	}

	items := make([]ContextItem, 0, len(events))
	for _, ev := range events {
		items = append(items, ContextItem{
			Content:   traceEventContent(ev),
			Source:    string(knowledge.EpistemicMEASURED),
			Relevance: 1.0,
		})
	}
	// Packet honesto da LEITURA: carrega o MESMO trace_id que foi lido (não um
	// recém-gerado). A imprecisão anterior (NewContextPacket) gerava um trace_id
	// NOVO a cada leitura — o que criava auto-referência/loop. Trace sem eventos
	// → packet com context vazio e confidence 0 (honesto: "não há registro para
	// este trace"), mas SEMPRE com o trace_id lido. NÃO chamamos recordTraceEvent
	// aqui: esta tool é o LEITOR do trace; gravar um evento nela faria o trace
	// registrar a si mesmo (loop).
	packet := &ContextPacket{
		Query:      "trace:" + args.TraceID,
		Context:    items,
		Confidence: packetConfidence(items),
		TraceID:    args.TraceID,
	}
	return resultFromToolCall(packet, ToolTrace, statusFor(len(items))), nil
}

// ─── Tool: cosca.reason ───────────────────────────────────────────────────

type reasonArgs struct {
	TraceID string   `json:"trace_id"`
	Sequence []string `json:"sequence"`
}

func (e *Engine) handleReason(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := requireInput[reasonArgs](raw)
	if err != nil {
		return nil, err
	}

	var items []ContextItem
	name := "reason"

	if strings.TrimSpace(args.TraceID) != "" && e.Trace != nil {
		events, tErr := e.Trace.Get(args.TraceID)
		if tErr != nil {
			return nil, fmt.Errorf("cosca.reason: recuperação do trace falhou: %w", tErr)
		}
		seq := trace.SequenceFromEvents(events)
		items = append(items, ContextItem{
			Content:   "Cadeia causal de " + args.TraceID + ": " + strings.Join(seq.Actions, " → "),
			Source:    string(knowledge.EpistemicINFERRED),
			Relevance: 0.8,
		})
		// Divergência determinística (sem LLM) contra o histórico.
		if div := trace.DetectDivergence([]trace.Sequence{seq}, seq); len(div) > 0 {
			items = append(items, ContextItem{
				Content:   fmt.Sprintf("Divergência causal detectada nos passos %v (heurística determinística, sem LLM como autoridade)", div),
				Source:    string(knowledge.EpistemicDECISION),
				Relevance: 0.7,
			})
		}
		name = args.TraceID
	} else if len(args.Sequence) > 0 {
		seq := trace.Sequence{Actions: append([]string(nil), args.Sequence...)}
		items = append(items, ContextItem{
			Content:   "Sequência submetida: " + strings.Join(seq.Actions, " → "),
			Source:    string(knowledge.EpistemicINFERRED),
			Relevance: 0.8,
		})
		name = "sequência-fornecida"
	} else {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "cosca.reason: forneça 'trace_id' (com trace store) ou 'sequence' para raciocinar sobre o trace"}},
		}, nil
	}

	packet := NewContextPacket(name, items)
	e.recordTraceEvent(packet, "REASON", fmt.Sprintf("subject=%q items=%d", name, len(items)))
	return resultFromToolCall(packet, ToolReason, statusFor(len(items))), nil
}

// ─── Tool: cosca.project ──────────────────────────────────────────────────

func (e *Engine) handleProject(ctx context.Context, _ json.RawMessage) (*CallResult, error) {
	var items []ContextItem

	// Runtime / body — health e estado.
	if e.Runtime != nil {
		health := "unknown"
		if e.Runtime.State() != nil {
			state := e.Runtime.State()
			health = string(state.HealthStatus())
			items = append(items, ContextItem{
				Content:   "Runtime (corpo): " + state.Current().String() + " / health: " + string(state.HealthStatus()),
				Source:    string(knowledge.EpistemicMEASURED),
				Relevance: 1.0,
			})
		}
		_ = health
	}

	// Knowledge stats.
	if e.Knowledge != nil {
		if st, sErr := e.Knowledge.GetStats(); sErr == nil && st != nil {
			items = append(items, ContextItem{
				Content:   fmt.Sprintf("Knowledge: %d documentos, %d chunks, %d entidades, %d vetores", st.DocumentCount, st.ChunkCount, st.EntityCount, st.VectorCount),
				Source:    string(knowledge.EpistemicMEASURED),
				Relevance: 1.0,
			})
		}
	}

	// Memory layers.
	if e.Memory != nil {
		layers := e.Memory.GetLayerStats(ctx)
		if len(layers) > 0 {
			items = append(items, ContextItem{
				Content:   fmt.Sprintf("Memory: %d camadas ativas", len(layers)),
				Source:    string(knowledge.EpistemicMEASURED),
				Relevance: 0.9,
			})
		}
	}

	// Kernel / kill-switch.
	if e.Kernel != nil {
		items = append(items, ContextItem{
			Content:   "Kernel (cérebro): kill-switch " + string(e.Kernel.State()),
			Source:    string(knowledge.EpistemicFACT),
			Relevance: 1.0,
		})
	}

	if len(items) == 0 {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "cosca.project: sem runtime/knowledge/memory/kernel injetados — cérebro indisponível"}},
		}, nil
	}

	packet := NewContextPacket("project", items)
	e.recordTraceEvent(packet, "PROJECT", fmt.Sprintf("runtime=%v knowledge=%v memory=%v kernel=%v",
		e.Runtime != nil, e.Knowledge != nil, e.Memory != nil, e.Kernel != nil))
	return resultFromToolCall(packet, ToolProject, statusFor(len(items))), nil
}

// ─── Tool: cosca.cost (Token Efficiency — ADR-031) ────────────────────────

type costArgs struct {
	Agent string `json:"agent"`
	Task  string `json:"task"`
	Tasks bool   `json:"tasks"`
}

func (e *Engine) handleCost(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Cost == nil {
		return nil, fmt.Errorf("cosca.cost: cost store indisponível (sem órgão de custo)")
	}
	args, err := requireInput[costArgs](raw)
	if err != nil {
		return nil, err
	}

	records, lErr := e.Cost.Load()
	if lErr != nil {
		return nil, fmt.Errorf("cosca.cost: falha ao ler records: %w", lErr)
	}

	// Filtro por agente/task (se informado).
	filtered := records
	if args.Agent != "" || args.Task != "" {
		filtered = make([]cost.Record, 0, len(records))
		for _, r := range records {
			if args.Agent != "" && r.AgentID != args.Agent {
				continue
			}
			if args.Task != "" && r.TaskID != args.Task {
				continue
			}
			filtered = append(filtered, r)
		}
	}

	// Agregação: por task (causal, com execuções aninhadas) ou por gruo.
	type costPayload struct {
		Runs        int             `json:"runs"`
		TokensTotal int             `json:"tokens_total"`
		UsefulWork  float64         `json:"useful_work"`
		Efficiency  float64         `json:"efficiency"`
		KnowledgeGain float64       `json:"knowledge_gain"`
		TaskProgress  float64       `json:"task_progress"`
		ArtifactValue int           `json:"artifact_value"`
		EvidenceGain  int           `json:"evidence_gain"`
		DecisionGain  int           `json:"decision_gain"`
		Tasks         []cost.TaskRecord `json:"tasks,omitempty"`
		Grouped       []cost.Summary    `json:"grouped,omitempty"`
	}

	payload := costPayload{}
	if args.Tasks {
		payload.Tasks = cost.AggregateTasks(filtered)
		for _, t := range payload.Tasks {
			payload.Runs += t.Runs
			payload.TokensTotal += t.TokensTotal
			payload.UsefulWork += t.UsefulWork()
			payload.KnowledgeGain += t.KnowledgeGain
			payload.TaskProgress += t.TaskProgress
			payload.ArtifactValue += t.ArtifactValue
			payload.EvidenceGain += t.EvidenceGain
			payload.DecisionGain += t.DecisionGain
		}
	} else {
		rep := cost.Aggregate(filtered)
		payload.Grouped = rep.Grouped
		payload.Runs = rep.Runs
		payload.TokensTotal = rep.Total.TokensTotal
		payload.UsefulWork = rep.Total.UsefulWork()
		payload.KnowledgeGain = rep.Total.KnowledgeGain
		payload.TaskProgress = rep.Total.TaskProgress
		payload.ArtifactValue = rep.Total.ArtifactValue
		payload.EvidenceGain = rep.Total.EvidenceGain
		payload.DecisionGain = rep.Total.DecisionGain
	}
	if payload.TokensTotal > 0 {
		payload.Efficiency = payload.UsefulWork / float64(payload.TokensTotal)
	}

	rawOut, _ := json.Marshal(payload)
	return &CallResult{
		Content: []ContentItem{{Type: "text", Text: string(rawOut)}},
	}, nil
}

// ─── Tool: cosca.cli (operar o CLI com allowlist SEGURA) ─────────────────

type cliArgs struct {
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
}

// allowedCLIRoot é a allowlist de comandos do CLI que o MCP pode operar.
// Somente comandos de LEITURA/GESTÃO (sem executar código de agente, sem
// escrita destrutiva). NUNCA liberar execução arbitrária (ex: run/exec
// pertencem ao runtime, não ao MCP). Falha-closed: comando fora da lista
// → erro, NUNCA executa.
var allowedCLIRoot = map[string]bool{
	"status":     true,
	"doctor":     true,
	"health":     true,
	"version":    true,
	"capability": true,
	"cost":       true,
	"budget":     true,
	"agent":      true,
	"skill":      true,
	"memory":     true,
	"knowledge":  true,
	"trace":      true,
	"provider":   true,
	"model":      true,
	"hardware":   true,
	"machine":    true,
	"project":    true,
}

func (e *Engine) handleCLI(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.CLIExec == nil {
		return nil, fmt.Errorf("cosca.cli: executor do CLI indisponível (não injetado)")
	}
	args, err := requireInput[cliArgs](raw)
	if err != nil {
		return nil, err
	}
	if len(args.Args) == 0 {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "cosca.cli: 'args' é obrigatório (ex.: [\"status\"])"}},
		}, nil
	}

	// Allowlist default-deny: o primeiro arg (comando raiz) deve estar na lista.
	root := strings.ToLower(strings.TrimSpace(args.Args[0]))
	if !allowedCLIRoot[root] {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("cosca.cli: comando %q não permitido (allowlist default-deny). Comandos permitidos: status, doctor, health, version, capability, cost, budget, agent, skill, memory, knowledge, trace, provider, model, hardware, machine, project", root)}},
		}, nil
	}

	// Kernel gate (kill-switch) — mesmo padrão fail-closed do Call.
	if e.Kernel != nil && e.Kernel.IsHalted() {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "cosca.cli: kernel kill-switch ativo — execução bloqueada"}},
		}, nil
	}

	// Executa via CLIExec (que roda o NewRootCommand com os args).
	var out string
	if args.Cwd != "" {
		out, err = e.CLIExec(append([]string{"--root", args.Cwd}, args.Args...))
	} else {
		out, err = e.CLIExec(args.Args)
	}
	if err != nil {
		return &CallResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("cosca.cli: execução falhou: %v", err)}},
		}, nil
	}

	return &CallResult{
		Content: []ContentItem{{Type: "text", Text: out}},
	}, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// resultFromPacket serializa o packet como o de conteúdo da tool call.
// Envolve o ContextPacket no envelope padronizado (status/capability) do
// "cérebro instrumentado": o OpenCode recebe o que o COSCA sabe + o que fez.
func resultFromPacket(packet *ContextPacket) *CallResult {
	return resultFromToolCall(packet, "", "ok")
}

// resultFromToolCall serializa o packet no envelope cognitivo padronizado
// (status, capability, trace_id, confidence, epistemic_class, contexte,
// artifacts, decisions, cost). `capability` é o nome da tool; `status` é
// 'ok' | 'empty' | 'error'. O resultado CONTÉM o packet (não apenas um string).
func resultFromToolCall(packet *ContextPacket, capability, status string) *CallResult {
	// Garante que campos opcionais não vazam como null/[] indevido.
	env := map[string]interface{}{
		"status":    status,
		"capability": capability,
	}
	if packet != nil {
		env["trace_id"] = packet.TraceID
		env["query"] = packet.Query
		env["context"] = packet.Context
		env["confidence"] = packet.Confidence
		if packet.EpistemicClass != "" {
			env["epistemic_class"] = packet.EpistemicClass
		}
		if len(packet.Artifacts) > 0 {
			env["artifacts"] = packet.Artifacts
		}
		if len(packet.Decisions) > 0 {
			env["decisions"] = packet.Decisions
		}
		if packet.Cost != nil {
			env["cost"] = packet.Cost
		}
	}
	raw, _ := json.Marshal(env)
	return &CallResult{
		Content: []ContentItem{{Type: "text", Text: string(raw)}},
	}
}

// resultFor devolve o Result de um evento de trace de leitura: "success" se o
// packet trouxe itens, "empty" se nenhum (auditoria honesta do cérebro).
func resultFor(n int) string {
	if n > 0 {
		return "success"
	}
	return "empty"
}

// statusFor devolve o status do envelope cognitivo: "ok" (teve resultado) ou
// "empty" (consultou mas nada encontrou — honesto). Mesma semântica de
// resultFor, nomeada para o envelope.
func statusFor(n int) string {
	if n > 0 {
		return "ok"
	}
	return "empty"
}

// recordTraceEvent registra (best-effort, NUNCA falha a tool) um evento no
// flight recorder para o Trace ID de um packet de leitura. Append é
// append-only e serve de auditoria: se o trace store estiver nil ou o append
// falhar, o packet NÃO é abortado — a resposta é sempre devolvida (o trace é
// auditoria, não pré-requisito).
func (e *Engine) recordTraceEvent(packet *ContextPacket, action, details string) {
	if e.Trace == nil || packet == nil {
		return
	}
	_ = e.Trace.Append(trace.Event{
		TraceID: packet.TraceID,
		Actor:   "mcp",
		Action:  action,
		Result:  resultFor(len(packet.Context)),
		Details: details,
	})
}

// contentFromResult monta o conteúdo de um resultado de busca.
func contentFromResult(r search.SearchResult) string {
	var sb strings.Builder
	if r.Title != "" {
		sb.WriteString(r.Title)
		if r.Content != "" {
			sb.WriteString(": ")
		}
	} else {
		sb.WriteString(string(sourceFromResult(r)))
	}
	sb.WriteString(r.Content)
	if r.Snippet != "" && r.Snippet != r.Content {
		sb.WriteString(" — ")
		sb.WriteString(r.Snippet)
	}
	if r.DocumentPath != "" {
		sb.WriteString(" [" + r.DocumentPath + "]")
	}
	return sb.String()
}

// memoryContent serializa um MemoryRecord como item de contexto.
func memoryContent(r memory.MemoryRecord) string {
	prefix := string(r.Type)
	if prefix == "" {
		prefix = "memoria"
	}
	if r.Layer != "" {
		prefix += ":" + string(r.Layer)
	}
	return prefix + " — " + r.Content
}

// memoryRelevance é uma proxy determinística de relevância para memória
// (sem score exposto pelo retorno atual): prioridade alta → mais relevante.
func memoryRelevance(r memory.MemoryRecord) float64 {
	switch {
	case r.Priority >= 5:
		return 0.9
	case r.Priority >= 3:
		return 0.7
	case r.Priority >= 1:
		return 0.55
	default:
		return 0.5
	}
}

// memoryTypeFor mapeia o tipo do learn para um MemoryType válido.
func memoryTypeFor(s string) string {
	switch memory.MemoryType(s) {
	case memory.MemoryTypeDecision, memory.MemoryTypePattern, memory.MemoryTypeBug,
		memory.MemoryTypeAgent, memory.MemoryTypeProject, memory.MemoryTypeArchitecture,
		memory.MemoryTypeSession:
		return s
	default:
		return string(memory.MemoryTypeSession)
	}
}

// memoryLayerFor mapeia a camada do learn para um MemoryLayer válido.
func memoryLayerFor(s string) string {
	switch memory.MemoryLayer(s) {
	case memory.LayerGlobal, memory.LayerWorkspace, memory.LayerProject,
		memory.LayerSession, memory.LayerTemp, memory.LayerLong:
		return s
	default:
		return string(memory.LayerWorkspace)
	}
}

// observeSource converte o estado epistêmico de uma observação de visão em um
// source do packet (da percepção, não do conhecimento).
func observeSource(ev vision.PerceptEvent) string {
	switch ev.Observation.Epistemic {
	case vision.EpistemicObserved:
		return string(knowledge.EpistemicFACT)
	case vision.EpistemicTracked:
		return string(knowledge.EpistemicMEASURED)
	case vision.EpistemicPredicted:
		return string(knowledge.EpistemicINFERRED)
	default:
		return string(knowledge.EpistemicEVIDENCE)
	}
}

// observeContent serializa um evento de percepção.
func observeContent(ev vision.PerceptEvent) string {
	if ev.Observation.Corroborated {
		return fmt.Sprintf("Frame %d: %s (corroborado por %s) — %s", ev.Frame, ev.Kind, strings.Join(ev.Observation.CorroboratedBy, ", "), ev.Explanation)
	}
	return fmt.Sprintf("Frame %d: %s — %s", ev.Frame, ev.Kind, ev.Explanation)
}

// traceEventContent serializa um evento de trace.
func traceEventContent(ev trace.Event) string {
	return fmt.Sprintf("[%s] %s → %s (result: %s)", ev.TraceID, ev.Action, ev.Actor, emptyOr(ev.Result, "desconhecido"))
}

// emptyOr devolve fallback quando s é vazio.
func emptyOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}











// ─── Tool: cosca.self (auto-inspeção do cérebro) ──────────────────────────

// handleSelf reporta o estado operacional de cada órgão do COSCA: quais estão
// injetados e operacionais, e a capacidade do sistema como um todo. É a
// "API do próprio cérebro" — permite ao agente perguntar 'quais órgãos tenho?'.
func (e *Engine) handleSelf(ctx context.Context, _ json.RawMessage) (*CallResult, error) {
	type organ struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	organs := []organ{
		{Name: "kernel", Status: organStatus(e.Kernel != nil)},
		{Name: "runtime", Status: organStatus(e.Runtime != nil)},
		{Name: "knowledge", Status: organStatus(e.Knowledge != nil)},
		{Name: "memory", Status: organStatus(e.Memory != nil)},
		{Name: "trace", Status: organStatus(e.Trace != nil)},
		{Name: "vision", Status: organStatus(e.Vision != nil)},
		{Name: "cost", Status: organStatus(e.Cost != nil)},
		{Name: "cli", Status: organStatus(e.CLIExec != nil)},
	}

	// Conta operacionais.
	operational := 0
	for _, o := range organs {
		if o.Status == "operational" {
			operational++
		}
	}

	payload := map[string]interface{}{
		"organs":        organs,
		"operational":   operational,
		"total":         len(organs),
		"capability":    "self_inspection",
		"allow_write":   e.AllowWrite,
		"kernel_halted": e.Kernel != nil && e.Kernel.IsHalted(),
	}
	rawOut, _ := json.Marshal(payload)
	return &CallResult{
		Content: []ContentItem{{Type: "text", Text: string(rawOut)}},
	}, nil
}

// organStatus converte a presença de um órgão em estado legível.
func organStatus(present bool) string {
	if present {
		return "operational"
	}
	return "unavailable"
}

// ─── Tool: cosca.web (fetch HTTP seguro — anti-SSRF) ──────────────────────

type webArgs struct {
	URL    string `json:"url"`
	MaxLen int    `json:"max_len"`
	// Preview (custo-benefit, padrão PinchTab): true retorna só status + título
	// + snippet (BARATO); false retorna o corpo (full). O agente usa preview
	// primeiro e escala para full só quando o barato não resolve.
	Preview bool `json:"preview"`
	// Fit (padrão crawl4ai): true extrai só o conteúdo ESSENCIAL do HTML
	// (remove script/style/nav/footer), reduzindo volume/token antes do LLM.
	Fit bool `json:"fit"`
}

// handleWeb faz um GET seguro, validando a URL contra SSRF (usando o guard de
// rede anti-SSRF do COSCA). Só http/https são permitidos; o host é resolvido e
// validado (IP público) antes de qualquer dial. Conteúdo retornado é tratado
// como DADO NÃO-CONFIÁVEL (risco prompt-injection).
func (e *Engine) handleWeb(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := requireInput[webArgs](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.URL) == "" {
		return nil, fmt.Errorf("cosca.web: 'url' é obrigatório")
	}
	u, perr := url.Parse(args.URL)
	if perr != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("cosca.web: apenas http/https são permitidos (SSRF guard)")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("cosca.web: URL sem host")
	}

	// Resolve o host e valida contra SSRF (todas as IPs devem ser públicas).
	ips, rerr := net.DefaultResolver.LookupIP(ctx, "ip", u.Hostname())
	if rerr != nil || len(ips) == 0 {
		return nil, fmt.Errorf("cosca.web: falha ao resolver host: %v", rerr)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return nil, fmt.Errorf("cosca.web: IP %v inválido (SSRF guard)", ip)
		}
		if err := security.ValidatePublicIP(addr); err != nil {
			return nil, fmt.Errorf("cosca.web: %w", err)
		}
	}

	// HTTP client com timeout.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, herr := client.Get(u.String())
	if herr != nil {
		return nil, fmt.Errorf("cosca.web: GET falhou: %w", herr)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := args.MaxLen
	if limit <= 0 || limit > 20000 {
		limit = 8000
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(limit)))

	// Custo-benefit (padrão PinchTab/crawl4ai): em modo preview retorna só o
	// início (status + snippet barato); em modo fit extrai só o conteúdo
	// essencial do HTML (remove lixo); full retorna o corpo inteiro.
	var content string
	if args.Preview {
		snippet := string(body)
		if len(snippet) > 600 {
			snippet = snippet[:600] + "…"
		}
		content = fmt.Sprintf("status=%d length=%d preview_preview=true\n%s", resp.StatusCode, len(body), snippet)
	} else if args.Fit {
		fit := contentfit.Extract(string(body))
		content = fmt.Sprintf("status=%d length=%d fit=true chars=%d\n%s", resp.StatusCode, len(body), fit.CharCount, fit.Text)
	} else {
		content = fmt.Sprintf("status=%d length=%d\n%s", resp.StatusCode, len(body), string(body))
	}

	// Conteúdo de página = dado NÃO-CONFIÁVEL (nunca tratado como instrução).
	// Envelopamos com contenttrust (nonce + length-delimited + JSON-escaped)
	// para o modelo NÃO forjar o fechamento nem seguir instrução da página.
	wrapped := contenttrust.Envelope(contenttrust.Default(
		contenttrust.OriginMCP,
		content,
		"web:"+u.String(),
	))
	packet := NewContextPacket("web:"+u.String(), []ContextItem{{
		Content:   wrapped,
		Source:    string(knowledge.EpistemicEVIDENCE),
		Relevance: 1.0,
	}})
	packet.Capability = ToolWeb
	packet.Decisions = append(packet.Decisions, DecisionRef{Kind: "ssrf_guard_ok", Details: "host resolvido + IP publico validado"})
	return resultFromToolCall(packet, ToolWeb, statusFor(len(packet.Context))), nil
}
