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
	"strings"

	"github.com/CoscaAI/cosca/internal/chat/mcp"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/search"
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
	// AllowWrite habilita a ÚNICA escrita (cosca.learn) via gate. Read-only-first.
	AllowWrite bool
}

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

// WithAllowWrite habilita a escrita gateada de cosca.learn.
func WithAllowWrite(v bool) Option { return func(e *Engine) { e.AllowWrite = v } }

// NewEngine cria um Engine a partir das opções. Nil-safe: sem opção o campo
// fica nil e a tool que depende dele devolve erro claro.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{}
	for _, opt := range opts {
		opt(e)
	}
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
// Usa mcp.ToolInfo (mesma estrutura do cliente) — simetria cliente↔servidor.
func (e *Engine) Tools() []mcp.ToolInfo {
	tools := []mcp.ToolInfo{
		{
			Name:        ToolRecall,
			Description: "Lembrar — busca semântica híbrida no conhecimento; devolve context packet com source epistêmico, relevância, confidence e trace_id.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"},"epistemic":{"type":"array","items":{"type":"string"}},"path":{"type":"string"}},"required":["query"]}`),
		},
		{
			Name:        ToolContext,
			Description: "Contextualizar (tool central) — dado arquivo/projeto/query, devolve o packet do contexto relevante para o agente usar (knowledge + memory).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`),
		},
		{
			Name:        ToolLearn,
			Description: "Aprender — registrar aprendizado com proveniência. Escrita GATEADA (require COSCA_MCP_ALLOW_WRITE=1); default read-only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"},"type":{"type":"string"},"layer":{"type":"string"},"scope":{"type":"string"}},"required":["content"]}`),
		},
		{
			Name:        ToolObserve,
			Description: "Perceber — percepção determinística frame-a-frame (OCR/pixel-diff) → eventos com estado epistêmico. Sem VLM.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"video":{"type":"string"},"fps":{"type":"integer"},"ocr_lang":{"type":"string"}},"required":["video"]}`),
		},
		{
			Name:        ToolReason,
			Description: "Raciocinar — cadeia causal / replay de raciocínio sobre um trace; divergência determinística (sem LLM como autoridade).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"trace_id":{"type":"string"},"sequence":{"type":"array","items":{"type":"string"}}}}`),
		},
		{
			Name:        ToolTrace,
			Description: "Rastrear — traces/execuções/grafo causal de uma operação (TRACE-...), do flight recorder append-only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"trace_id":{"type":"string"}},"required":["trace_id"]}`),
		},
		{
			Name:        ToolProject,
			Description: "Orientar — estado do projeto (runtime/health, knowledge stats, memory layers) — 'quem está sendo observado?'.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
	return tools
}

// hasTool reports se a tool existe no inventário (default-deny, I8).
func (e *Engine) hasTool(name string) bool {
	for _, t := range e.Tools() {
		if t.Name == name {
			return true
		}
	}
	return false
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
	if !e.hasTool(name) {
		return nil, fmt.Errorf("cosca.mcp: tool %q desconhecida (default-deny)", name)
	}

	switch name {
	case ToolRecall:
		return e.callRecall(ctx, args)
	case ToolContext:
		return e.callContext(ctx, args)
	case ToolLearn:
		return e.callLearn(ctx, args)
	case ToolObserve:
		return e.callObserve(ctx, args)
	case ToolReason:
		return e.callReason(ctx, args)
	case ToolTrace:
		return e.callTrace(ctx, args)
	case ToolProject:
		return e.callProject(ctx)
	default:
		return nil, fmt.Errorf("cosca.mcp: tool %q não implementada", name)
	}
}

// ─── Tool: cosca.recall ───────────────────────────────────────────────────

type recallArgs struct {
	Query     string   `json:"query"`
	Limit     int      `json:"limit"`
	Epistemic []string `json:"epistemic"`
	Path      string   `json:"path"`
}

func (e *Engine) callRecall(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Knowledge == nil {
		return nil, fmt.Errorf("cosca.recall: knowledge engine indisponível (sem corpo)")
	}
	args, err := parseArgs[recallArgs](raw)
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.context (central) ────────────────────────────────────────

type contextArgs struct {
	Query string `json:"query"`
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

func (e *Engine) callContext(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := parseArgs[contextArgs](raw)
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.learn (única escrita — GATEADA) ──────────────────────────

type learnArgs struct {
	Content string `json:"content"`
	Type    string `json:"type"`
	Layer   string `json:"layer"`
	Scope   string `json:"scope"`
}

func (e *Engine) callLearn(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := parseArgs[learnArgs](raw)
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.observe ──────────────────────────────────────────────────

type observeArgs struct {
	Video    string `json:"video"`
	FPS      int    `json:"fps"`
	OCRLang  string `json:"ocr_lang"`
	WorkDir  string `json:"work_dir"`
}

func (e *Engine) callObserve(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Vision == nil {
		return nil, fmt.Errorf("cosca.observe: pipeline de visão indisponível (sem órgão perceptivo)")
	}
	args, err := parseArgs[observeArgs](raw)
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
			items = append(items, ContextItem{
				Content:   observeContent(ev),
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.trace ────────────────────────────────────────────────────

type traceArgs struct {
	TraceID string `json:"trace_id"`
}

func (e *Engine) callTrace(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	if e.Trace == nil {
		return nil, fmt.Errorf("cosca.trace: trace store indisponível (sem flight recorder)")
	}
	args, err := parseArgs[traceArgs](raw)
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.reason ───────────────────────────────────────────────────

type reasonArgs struct {
	TraceID string   `json:"trace_id"`
	Sequence []string `json:"sequence"`
}

func (e *Engine) callReason(ctx context.Context, raw json.RawMessage) (*CallResult, error) {
	args, err := parseArgs[reasonArgs](raw)
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
	return resultFromPacket(packet), nil
}

// ─── Tool: cosca.project ──────────────────────────────────────────────────

func (e *Engine) callProject(ctx context.Context) (*CallResult, error) {
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
	return resultFromPacket(packet), nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// parseArgs decodifica os arguments JSON de uma tool call em um struct
// tipado, com erro claro (%w). JSON inválido → erro.
func parseArgs[T any](raw json.RawMessage) (T, error) {
	var args T
	if len(raw) == 0 {
		return args, nil
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return args, fmt.Errorf("cosca.mcp: argumentos inválidos: %w", err)
	}
	return args, nil
}

// resultFromPacket serializa o packet como o de conteúdo da tool call.
func resultFromPacket(packet *ContextPacket) *CallResult {
	raw, _ := json.Marshal(packet)
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
