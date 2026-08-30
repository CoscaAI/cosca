// registry.go — O MCP SURFACE / CAPABILITY REGISTRY do COSCA.
//
// O professor apontou (2026-08-30): o tools.go monolítico com switch + inventário
// hardcoded estava a um passo de virar um "segundo COSCA" conforme a lista
// cresce. A evolução correta é registrar as tools por CAPABILITY (domínio,
// órgão responsável, risco, permissão, handler) — para a lista poder crescer
// para dezenas SEM virar um switch gigante nem recompilar o inventário a cada
// nova tool.
//
// Cada ToolDef declara:
//   - Name:      nome canônico (ex: cosca.recall)
//   - Domain:    família cognitiva (cognition/knowledge/runtime/audit/cost/...)
//   - Organ:     órgão do COSCA responsável (knowledge/memory/trace/runtime/...)
//   - Risk:      nível de risco (read|operate|write) → gate de permissão
//   - Handler:   função de execução (nil-safe, sem pânico)
//
// O `Engine` (tools.go) vira apenas a fachada: `Tools()` itera o registry e
// `Call()` roteia por ele. Adicionar uma tool nova = registrar um ToolDef, sem
// tocar no switch nem no inventário.

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
)

// RiskLevel classifica o risco de uma tool para o gate de permissão.
type RiskLevel string

const (
	// RiskRead — apenas leitura do estado (recall, trace, status). Sem efeito.
	RiskRead RiskLevel = "read"
	// RiskOperate — opera estado de gestão (project, agent, skill). Não muda
	// o mundo de forma destrutiva.
	RiskOperate RiskLevel = "operate"
	// RiskWrite — altera estado (learn, store, halt). Requer gate explícito.
	RiskWrite RiskLevel = "write"
)

// ToolHandler é a assinatura de execução de uma tool registrada.
type ToolHandler func(ctx context.Context, args json.RawMessage) (*CallResult, error)

// ToolDef é o registro declarativo de uma tool/capability do MCP.
type ToolDef struct {
	Name        string      // nome canônico (ex: cosca.recall)
	Domain      string      // família (cognition/knowledge/runtime/audit/cost/...)
	Organ       string      // órgão responsável (knowledge/memory/trace/runtime/...)
	Risk        RiskLevel   // nível de risco (read|operate|write)
	Description string      // descrição para tools/list
	InputSchema string      // JSON schema (string) — serializado em ToolInfo
	Handler     ToolHandler // função de execução (nil-safe)
}

// registry mantém as tools registradas, indexadas por nome.
type registry struct {
	byName map[string]*ToolDef
	order  []*ToolDef
}

// newRegistry cria um registry vazio.
func newRegistry() *registry {
	return &registry{byName: make(map[string]*ToolDef)}
}

// register adiciona um ToolDef ao registry. Nome duplicado sobrescreve
// (última vitória) — assert de integridade para o desenvolvimento.
func (r *registry) register(def ToolDef) {
	if _, exists := r.byName[def.Name]; !exists {
		r.order = append(r.order, &def)
	}
	r.byName[def.Name] = &def
}

// get devolve o ToolDef pelo nome, ou nil.
func (r *registry) get(name string) *ToolDef {
	return r.byName[name]
}

// has reporta se a tool existe (default-deny).
func (r *registry) has(name string) bool {
	return r.byName[name] != nil
}

// names devolve os nomes registrados, na ordem de registro.
func (r *registry) names() []string {
	out := make([]string, 0, len(r.order))
	for _, d := range r.order {
		out = append(out, d.Name)
	}
	return out
}

// list devolve os ToolDefs, na ordem de registro.
func (r *registry) list() []*ToolDef {
	out := make([]*ToolDef, len(r.order))
	copy(out, r.order)
	return out
}

// requireInput decodifica os arguments JSON em um struct tipado. JSON inválido
// → erro claro (%w). Usado pelos handlers registrados.
func requireInput[T any](raw json.RawMessage) (T, error) {
	var args T
	if len(raw) == 0 {
		return args, nil
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return args, fmt.Errorf("cosca.mcp: argumentos inválidos: %w", err)
	}
	return args, nil
}

// registerTools popula o Capability Registry com as tools cognitivas do COSCA.
// Cada tool é uma CAPACIDADE (domínio + órgão + risco) com handler tipado.
// Adicionar uma tool nova = registrar um ToolDef aqui — sem tocar no switch nem
// no inventário do Tools().
func (e *Engine) registerTools() {
	e.registry = newRegistry()

	e.registry.register(ToolDef{
		Name:        ToolRecall,
		Domain:      "cognition",
		Organ:       "knowledge",
		Risk:        RiskRead,
		Description: "Lembrar — busca semântica híbrida no conhecimento; devolve context packet com source epistêmico, relevância, confidence e trace_id.",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"},"epistemic":{"type":"array","items":{"type":"string"}},"path":{"type":"string"}},"required":["query"]}`,
		Handler:     e.handleRecall,
	})
	e.registry.register(ToolDef{
		Name:        ToolContext,
		Domain:      "cognition",
		Organ:       "knowledge",
		Risk:        RiskRead,
		Description: "Contextualizar (tool central) — dado arquivo/projeto/query, devolve o packet do contexto relevante para o agente usar (knowledge + memory).",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`,
		Handler:     e.handleContext,
	})
	e.registry.register(ToolDef{
		Name:        ToolLearn,
		Domain:      "cognition",
		Organ:       "memory",
		Risk:        RiskWrite,
		Description: "Aprender — registrar aprendizado com proveniência. Escrita GATEADA (require COSCA_MCP_ALLOW_WRITE=1); default read-only.",
		InputSchema: `{"type":"object","properties":{"content":{"type":"string"},"type":{"type":"string"},"layer":{"type":"string"},"scope":{"type":"string"}},"required":["content"]}`,
		Handler:     e.handleLearn,
	})
	e.registry.register(ToolDef{
		Name:        ToolObserve,
		Domain:      "perception",
		Organ:       "vision",
		Risk:        RiskRead,
		Description: "Perceber — percepção determinística frame-a-frame (OCR/pixel-diff) → eventos com estado epistêmico. Sem VLM.",
		InputSchema: `{"type":"object","properties":{"video":{"type":"string"},"fps":{"type":"integer"},"ocr_lang":{"type":"string"}},"required":["video"]}`,
		Handler:     e.handleObserve,
	})
	e.registry.register(ToolDef{
		Name:        ToolReason,
		Domain:      "reasoning",
		Organ:       "trace",
		Risk:        RiskRead,
		Description: "Raciocinar — cadeia causal / replay de raciocínio sobre um trace; divergência determinística (sem LLM como autoridade).",
		InputSchema: `{"type":"object","properties":{"trace_id":{"type":"string"},"sequence":{"type":"array","items":{"type":"string"}}}}`,
		Handler:     e.handleReason,
	})
	e.registry.register(ToolDef{
		Name:        ToolTrace,
		Domain:      "audit",
		Organ:       "trace",
		Risk:        RiskRead,
		Description: "Rastrear — traces/execuções/grafo causal de uma operação (TRACE-...), do flight recorder append-only.",
		InputSchema: `{"type":"object","properties":{"trace_id":{"type":"string"}},"required":["trace_id"]}`,
		Handler:     e.handleTrace,
	})
	e.registry.register(ToolDef{
		Name:        ToolProject,
		Domain:      "runtime",
		Organ:       "runtime",
		Risk:        RiskRead,
		Description: "Orientar — estado do projeto (runtime/health, knowledge stats, memory layers) — 'quem está sendo observado?'.",
		InputSchema: `{"type":"object","properties":{}}`,
		Handler:     e.handleProject,
	})
	e.registry.register(ToolDef{
		Name:        ToolCost,
		Domain:      "cost",
		Organ:       "cost",
		Risk:        RiskRead,
		Description: "Custar — Token Efficiency (ADR-031): útil work / tokens por agente/task; agregação causal por task com execuções aninhadas (write_file+build juntos).",
		InputSchema: `{"type":"object","properties":{"agent":{"type":"string"},"task":{"type":"string"},"tasks":{"type":"boolean"}}}`,
		Handler:     e.handleCost,
	})
	e.registry.register(ToolDef{
		Name:        ToolCLI,
		Domain:      "operate",
		Organ:       "runtime",
		Risk:        RiskOperate,
		Description: "Operar o CLI do COSCA — executar um subconjunto SEGURO de comandos (leitura/status/gestão) via allowlist; NUNCA execução arbitrária. Passa pelo kernel gate.",
		InputSchema: `{"type":"object","properties":{"args":{"type":"array","items":{"type":"string"}},"cwd":{"type":"string"}},"required":["args"]}`,
		Handler:     e.handleCLI,
	})
	e.registry.register(ToolDef{
		Name:        ToolSelf,
		Domain:      "self",
		Organ:       "kernel",
		Risk:        RiskRead,
		Description: "Auto-inspecionar — estado dos órgãos do COSCA (kernel/runtime/knowledge/memory/trace/vision/cost): quais estão operacionais e a capacidade do cérebro. 'API do próprio cérebro'.",
		InputSchema: `{"type":"object","properties":{}}`,
		Handler:     e.handleSelf,
	})
}
