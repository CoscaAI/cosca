# OpenTelemetry + OpenFGA + Dapr + OpenHands — Infrastructure & Observability Patterns

> **Sources**: `open-telemetry/opentelemetry-specification`, `openfga/openfga`, `dapr/dapr`, `All-Hands-AI/OpenHands`
> **Analyzed**: 2026-08-09 — Web research + cross-agent analysis
> **Confidence**: 0.92 (validado via documentação canônica + código fonte)

## Intent

Fechar os gaps de observabilidade, autorização, abstração de infraestrutura e agent loop identificados no gap analysis. OpenTelemetry resolve tracing, OpenFGA resolve autorização, Dapr resolve abstração de infraestrutura, OpenHands resolve o agent loop.

## Patterns Extraídos (Top 15 — Síntese dos 4 Projetos)

---

### PARTE 1: OPENTELEMETRY — OBSERVABILIDADE

#### 1. Span como Log Estruturado Imutável

**Contrato exato**:
```
Span {
    Name: string                    // Nome da operação
    SpanContext: { TraceId, SpanId, TraceFlags, TraceState }
    ParentSpanId: bytes | null      // null = span raiz
    SpanKind: CLIENT|SERVER|PRODUCER|CONSUMER|INTERNAL
    StartTime/EndTime: nanoseconds
    Attributes: map[string]any      // http.method, db.system, ...
    Events: [{Name, Timestamp, Attributes}]
    Links: [{SpanContext, Attributes}]  // cross-trace links
    Status: {Code: Unset|Ok|Error, Description}
}
```

#### 2. Context Propagation via W3C Trace Context

```
traceparent: 00-{trace-id(32 hex)}-{parent-span-id(16 hex)}-{trace-flags(02)}
```
Propagação implícita via `Inject(Context, carrier)` / `Extract(Context, carrier)`. O trace_id viaja automaticamente entre processos.

#### 3. SDK Pipeline: TracerProvider → Tracer → Span → SpanProcessor → Exporter

```
TracerProvider (Sampler, IdGenerator, SpanProcessors)
  └→ Tracer (factory de spans)
       └→ Span (unidade de trabalho)
            └→ SpanProcessor (OnStart/OnEnd hooks)
                 ├→ SimpleSpanProcessor (sync)
                 └→ BatchingSpanProcessor (async, batch)
                      └→ SpanExporter (OTLP, Jaeger, Zipkin, stdout)
```

#### 4. Collector: Receivers → Processors → Exporters

Pipeline declarativo que desacopla instrumentação de backends. Tail-sampling decide quais traces exportar baseado em erros/latência.

**Por que importa para Cosca**: O Cosca já tem um sistema de trace (`internal/trace/trace.go` com TraceID, Event, CausalGraph). Mapeamento direto: Cosca TraceID ↔ OTel TraceID, Cosca Event ↔ OTel Span, Cosca CausalEdge ↔ OTel Links. Faltam: SpanProcessor pipeline, Exporter interface, W3C propagation, semantic conventions formais.

---

### PARTE 2: OPENFGA — AUTORIZAÇÃO FINE-GRAINED

#### 5. ReBAC DSL: Type → Relations → Permissions

```yaml
type department
  relations
    define member: [user]

type agent
  relations
    define belongs_to: [department]
    define holds_skill: [skill]
    define can_act: member from belongs_to

type skill
  relations
    define possessed_by: [agent]
    define can_use: can_act from possessed_by

type resource
  relations
    define require_skill: [skill]
    define can_access: can_use from require_skill
```

Isso modela: *"alice pode acessar production-db porque é membro de engineering → que tem o agente cosca-backend → que possui a skill postgresql → que é requerida por production-db"*.

#### 6. Check API: `check(user, relation, object) → {allowed: bool}`

Uma única pergunta. O motor resolve grafos de relações (4-hop chains) via BFS/DFS com caching.

#### 7. Usersets + Concentric Roles

`[org#member]` — todos os membros de uma org herdam acesso. `editor: [user] or owner` — owner implica editor implica viewer.

#### 8. ABAC Conditions (CEL Expressions)

```
condition non_expired_grant(current_time, grant_time, grant_duration) {
  current_time < grant_time + grant_duration
}
define viewer: [user with non_expired_grant]
```

**Por que importa para Cosca**: JWT binário (válido/inválido) → ReBAC com 4-hop chains (departamento→agente→skill→recurso). O DSL do OpenFGA modela exatamente nossa hierarquia organizacional.

---

### PARTE 3: DAPR — ABSTRAÇÃO DE INFRAESTRUTURA

#### 9. Sidecar Architecture: App ↔ localhost ↔ Sidecar ↔ Infra

```
┌──────┐     localhost:3500     ┌────────┐     gRPC     ┌──────────┐
│ App  │◄──────────────────────►│ daprd  │◄────────────►│ Backends │
│(qualquer│  HTTP/gRPC          │(58MB)  │              │(Redis,   │
│ lang)  │                      │        │              │ Kafka,   │
└──────┘                      └────────┘              │ Vault)   │
                                                       └──────────┘
```

App nunca importa SDK de infra. Chama `localhost` com APIs padronizadas.

#### 10. Building Block APIs (Contratos Padronizados)

| Bloco | Endpoint | Propósito |
|-------|----------|-----------|
| State | `/v1.0/state/{store}` | KV CRUD |
| Pub/Sub | `/v1.0/publish/{topic}` | Eventos |
| Invoke | `/v1.0/invoke/{app}/method/{m}` | Chamadas serviço |
| Secrets | `/v1.0/secrets/{store}/{key}` | Segredos |
| Bindings | `/v1.0/bindings/{name}` | Conectores externos |
| Actors | `/v1.0/actors/{type}/{id}` | Virtual actors |
| Lock | `/v1.0-alpha1/lock/{store}` | Lock distribuído |
| Crypto | `/v1.0-alpha1/crypto/{comp}` | Encriptação |
| LLM | `/v1.0-alpha2/conversation/{c}` | LLM routing |

#### 11. Pluggable Component Model (YAML-declared)

```yaml
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
  name: statestore
spec:
  type: state.redis
  metadata:
  - name: redisHost
    value: localhost:6379
```

Swap Redis por PostgreSQL sem tocar no código. App só conhece `statestore`.

#### 12. Declarative Resiliency (Timeout + Retry + Circuit Breaker)

Tudo declarado em YAML. Zero código no app. Políticas por target (app, componente, actor).

**Por que importa para Cosca**: Agentes nunca deveriam saber se o state store é Redis ou Postgres, se o event bus é NATS ou Kafka. Um `cosca-agent-runtime` sidecar expõe APIs padronizadas em `localhost`. Providers plugáveis via manifestos. Resiliency declarativa.

---

### PARTE 4: OPENHANDS — AGENT LOOP

#### 13. Stateless Event-Sourced Agent Loop

```
agent.step():
  1. OBSERVE: state = EventLog (immutable append-only)
  2. View = condensar eventos → LLM messages
  3. DECIDE: LLM response → ActionEvent(s)
  4. VALIDATE: SecurityAnalyzer → risco baixo/médio/alto
  5. EXECUTE: tool.execute(action) → ObservationEvent
  6. Emitir eventos → goto 1
```

Tudo é evento imutável. O agente é stateless — o estado está no EventLog.

#### 14. Tool Contract: Action → Observation (Pydantic Typed)

```python
# Action (input, schema auto-gerado)
class CmdRunAction:
    command: str
    thought: str

# Observation (output, schema auto-gerado)
class CmdOutputObservation:
    command_id: int
    content: str
    exit_code: int

# Tool = Action type + Observation type + Executor
tool = ToolDefinition(action=CmdRunAction, observation=CmdOutputObservation, executor=...)
```

Schema dos tools gerado dos tipos Pydantic, nunca escrito manualmente. Annotations (`readOnlyHint`, `destructiveHint`) alimentam SecurityAnalyzer.

#### 15. Context Condensation como Subsistema Dedicado

Rolling-window: head (N eventos recentes) + tail (M eventos antigos) + middle (sumarizado via LLM mais barato). Triggers: event count > threshold OU context window exceeded error. Pipeline de condensers encadeáveis.

**Por que importa para Cosca**: Nosso pipeline é linear (uma passada). OpenHands é um loop iterativo. O EventLog como source of truth + tool contract tipado + condensation subsystem são padrões que fortalecem nossa arquitetura de agentes.

---

## Gap Resolution: O Que Cada Projeto Fecha

| Gap | Projeto | Padrão | Status Cosca |
|-----|---------|--------|:------------:|
| 🔴 Tracing | OpenTelemetry | Span + W3C propagation + Exporter pipeline | ⚠️ Já tem TraceID/Event/CausalGraph — falta pipeline |
| 🔴 Execução durável | (Temporal L165) | Event sourcing + replay | ❌ Não implementado |
| 🟡 Autorização | OpenFGA | ReBAC 4-hop chains (dept→agent→skill→resource) | ⚠️ JWT binário → precisa ReBAC |
| 🟡 Infra abstraction | Dapr | Sidecar + Building Blocks + Pluggable components | ❌ Agentes acoplados a infra |
| 🟡 Agent loop | OpenHands | Event-sourced loop + Tool contract tipado | ⚠️ Pipeline linear → falta loop iterativo |
| 🟡 Extensibilidade | (Backstage L164) | ExtensionPoint triad + DI container | ❌ Agentes hardcoded |
| 🟢 Conhecimento | (Já robusto L163) | FTS5 + vector + graph | ✅ |
| 🟢 Memória | (Já robusto) | Auto-evolution + semantic memory | ✅ |
| 🟢 Segurança | (Já robusto L163) | bwrap jail | ✅ |

---

## Confidence Tracking

| # | Pattern | Source | Confidence |
|---|---------|--------|:----------:|
| 1 | Span as Structured Log | OpenTelemetry | 0.95 |
| 2 | W3C Context Propagation | OpenTelemetry | 0.94 |
| 3 | SDK Pipeline (TracerProvider→Exporter) | OpenTelemetry | 0.93 |
| 4 | Collector (Receivers→Processors→Exporters) | OpenTelemetry | 0.91 |
| 5 | ReBAC DSL (type→relation→permission) | OpenFGA | 0.94 |
| 6 | Check API | OpenFGA | 0.95 |
| 7 | Usersets + Concentric Roles | OpenFGA | 0.92 |
| 8 | ABAC Conditions (CEL) | OpenFGA | 0.89 |
| 9 | Sidecar Architecture | Dapr | 0.95 |
| 10 | Building Block APIs | Dapr | 0.94 |
| 11 | Pluggable Components | Dapr | 0.93 |
| 12 | Declarative Resiliency | Dapr | 0.91 |
| 13 | Event-Sourced Agent Loop | OpenHands | 0.92 |
| 14 | Typed Tool Contract | OpenHands | 0.91 |
| 15 | Context Condensation | OpenHands | 0.88 |

**Average confidence**: ~0.92

---
