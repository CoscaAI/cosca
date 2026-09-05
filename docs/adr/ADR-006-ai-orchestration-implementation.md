# ADR-006: AI Orchestration Engine — Implementation Architecture

> **Status:** Accepted | **Owner:** Architecture Chief | **Last Updated:** 2026-07-24

## Context

ADR-005 defined the high-level pipeline-based orchestration blueprint with four stages (Context → Router → Execute → Memory), core interfaces (`Orchestrator`, `Router`, `Pipeline`), and a directory structure. However, ADR-005 left six open implementation questions:

1. Where should the LLM Chat Provider interface and registry live?
2. How should chat providers and embedding providers coexist in the registry pattern?
3. How do we prevent tight coupling between the orchestration engine and internal subsystems?
4. How should pipeline state flow through stages — mutable or immutable?
5. How should pipelines handle errors gracefully without aborting the entire request?
6. How should streaming execution work alongside synchronous execution?

All infrastructure subsystems were already production-ready:
- `internal/knowledge/` — Hybrid FTS5 + vector search
- `internal/memory/` — 5-layer memory engine with SQLite FTS5
- `internal/agents/` — Agent manager parsing SKILL.md
- `internal/skills/` — Skill manager with embedded + local loading
- `internal/embeddings/` — 10 embedding providers with registry

This ADR documents the implementation decisions that resolved these questions and produced the working AI Orchestration Engine.

---

## Decisions

### Decision 1: Chat Provider Interface Package Location

**Chosen:** `internal/chat/` (dedicated package)

**Alternatives considered:**

| Location | Pros | Cons |
|----------|------|------|
| `internal/providers/` (rejected; co-located with providers) | Co-located with providers | Blurs interface ownership; providers package becomes a dumping ground |
| `internal/orchestration/` | Close to consumer | Orchestration becomes too large; chat types not orchestration-specific |
| **`internal/chat/` (chosen)** | Clean separation, dedicated package | One more import for consumers |

**Why:** The `chat` package is a standalone domain package — it defines the universal LLM chat contract that both provider implementations and the orchestration engine consume. It is not a sub-package of providers or orchestration; it is their shared dependency. The package contains:

```go
// internal/chat/types.go
type ChatProvider interface {
    Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error)
    ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (ChatStream, error)
    Model() string
    Name() string
    Close() error
}
```

The `ChatStream` interface abstracts over provider-specific streaming protocols (SSE, NDJSON, Gemini SSE):

```go
type ChatStream interface {
    Recv() (*ChatStreamChunk, error)
    Close() error
}
```

---

### Decision 2: Separate ChatRegistry from Embedding Registry

**Chosen:** Independent registries with analogous patterns but separate singletons

**Why:** Chat providers and embedding providers have fundamentally different:
- **Interfaces**: `ChatProvider` (Chat, ChatStream, Model, Name, Close) vs `embeddings.Provider` (GenerateEmbedding, GenerateEmbeddings, Model, Name, Dimensions, Close)
- **Selection logic**: Chat uses primary + ordered fallback chain; embeddings use model → dimensions → compatibility matching
- **Stats tracking**: Chat tracks requests/tokens/errors per call; embeddings tracks tokens per batch
- **Fallback behavior**: ChatRegistry tries primary then each fallback on error; embedding registry selects based on model availability

```go
// internal/chat/registry.go
type ChatRegistry struct {
    mu         sync.RWMutex
    providers  []registeredChatProvider
    selected   ChatProvider
    fallbacks  []ChatProvider
    stats      *ChatStats
    autoDetect bool
}
```

The `ChatRegistry` itself implements `ChatProvider`, acting as a composite that transparently handles fallback — consumers don't need to know the fallback logic:

```go
func (r *ChatRegistry) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
    // Try primary
    response, err := primary.Chat(ctx, messages, opts)
    if err == nil { return response, nil }

    // Try each fallback
    for _, fb := range fallbacks {
        response, err = fb.Chat(ctx, messages, opts)
        if err == nil { return response, nil }
    }
    return nil, fmt.Errorf("all chat providers failed")
}
```

---

### Decision 3: Co-located Provider Implementations

**Chosen:** `chat.go` lives alongside `*.go` (embedding provider) in `internal/providers/<name>/`

**Why:** Each provider package (`internal/providers/openai/`, `internal/providers/anthropic/`, etc.) implements both `embeddings.Provider` and `chat.ChatProvider` as separate files within the same package:

```
internal/providers/openai/
├── openai.go        # embeddings.Provider implementation (New, GenerateEmbedding)
├── chat.go          # chat.ChatProvider implementation (NewChat, Chat, ChatStream)
└── chat_test.go     # unit tests for chat provider

internal/providers/anthropic/
├── anthropic.go     # embeddings.Provider (stub — Anthropic has no embedding API)
└── chat.go          # chat.ChatProvider (full implementation)
```

Each package registers both into their respective global registries via separate `init()` functions. This co-location means a single import (`_ "github.com/.../providers/openai"`) registers both the embedding and chat providers automatically.

**Provider priority table** (lower = preferred):

| Provider | Chat Priority | Default Model | Streaming Protocol |
|----------|:---:|---------------|-------------------|
| OpenAI | 10 | gpt-4o | SSE (OpenAI) |
| DeepSeek | 15 | deepseek-chat | SSE (OpenAI-compatible) |
| Anthropic | 20 | claude-sonnet-4-20250514 | SSE (Anthropic) |
| Ollama | 20 | llama3 | NDJSON |
| Google | 25 | gemini-2.5-flash | SSE (Gemini) |
| Mistral | 45 | mistral-large-latest | SSE (OpenAI-compatible) |
| Groq | 55 | llama-3.1-8b-instant | SSE (OpenAI-compatible) |

---

### Decision 4: Port Interfaces in Orchestration Package

**Chosen:** Consumer-defined interfaces at `internal/orchestration/ports.go` with adapters in `internal/adapter/`

**Why:** Following the Go idiom of "consumer defines the interface," the orchestration package declares all subsystem interfaces it depends on. This applies the Dependency Inversion Principle: the orchestration engine owns the contract; subsystems conform through adapters.

```go
// internal/orchestration/ports.go
type KnowledgeSearcher interface {
    Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error)
}
type MemoryRetriever interface {
    Search(ctx context.Context, query string, opts MemorySearchOptions) ([]MemoryRecord, error)
    Retrieve(ctx context.Context, id, layer string) (*MemoryRecord, error)
}
type MemoryStorer interface {
    Store(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
}
type AgentResolver interface {
    Get(name string) (*AgentInfo, error)
    Search(query string) ([]AgentInfo, error)
}
type SkillResolver interface {
    Get(name string) (*SkillInfo, error)
    Search(query string) ([]SkillInfo, error)
    List() ([]SkillInfo, error)
}
```

Adaptation costs are low — each adapter is ~100-150 lines of type-mapping code:

```go
// internal/adapter/memory.go
type MemoryAdapter struct { engine *memory.MemoryEngine }
// Satisfies both MemoryRetriever and MemoryStorer
var _ orchestration.MemoryRetriever = (*MemoryAdapter)(nil)
var _ orchestration.MemoryStorer = (*MemoryAdapter)(nil)
```

**Alternative rejected:** Having subsystems export interfaces. This inverts ownership — subsystems would need to know about orchestration requirements, making them rigid.

---

### Decision 5: Immutable PipelineContext

**Chosen:** Builder pattern with copy-on-write semantics

**Why:** The `PipelineContext` is passed by value and each mutation method returns a new copy. This guarantees that no stage can accidentally modify state used by an earlier or concurrent stage.

```go
// internal/orchestration/types.go
type PipelineContext struct {
    RequestID   string
    Prompt      string
    ContextData map[string]interface{}
    Stage       string
    Inputs      map[string]interface{}
}

func (pc PipelineContext) WithContextData(key string, value interface{}) PipelineContext {
    pc.ContextData = copyStringMap(pc.ContextData)  // shallow copy
    pc.ContextData[key] = value
    return pc                                   // return by value
}

func (pc PipelineContext) WithStage(name string) PipelineContext { ... }
func (pc PipelineContext) WithPrompt(prompt string) PipelineContext { ... }
func (pc PipelineContext) WithInput(key string, value interface{}) PipelineContext { ... }
```

The `copyStringMap` helper ensures maps are never shared between context copies:

```go
func copyStringMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src { dst[k] = v }
    return dst
}
```

**Alternative rejected:** Mutable context with mutexes. Adds locking overhead, complicates concurrent stages, and makes testing harder.

---

### Decision 6: Sequential Pipeline with Explicit Parallelism

**Chosen:** Steps are sequential by default; `Parallel: true` in `PipelineDefinition` activates concurrent execution

**Why:** Sequential execution is predictable, debuggable, and the most common use case. Parallelism is opt-in and explicit — callers must set `Parallel: true` or use `ExecuteParallel` directly.

```go
type PipelineDefinition struct {
    Name     string
    Steps    []PipelineStep
    Parallel bool       // false → sequential; true → fan-out
}

type PipelineStep struct {
    Name      string
    Skill     string
    Input     string          // key in ContextData for input
    Output    string          // key for storing result
    Timeout   time.Duration   // per-step deadline
    Condition string          // key in ContextData; skip if falsy
    OnError   string          // "skip", "warn", "fail" (default)
}
```

Parallel execution uses goroutines with buffered channels, merging results sequentially after all complete:

```go
func (p *Pipeline) ExecuteParallel(ctx context.Context, pc PipelineContext, steps []PipelineStep) (PipelineContext, error) {
    results := make(chan stepResult, len(steps))
    for i, step := range steps {
        go func(idx int, s PipelineStep) {
            stepPC, _ := p.ExecuteStep(ctx, pc, s) // each goroutine uses a copy
            results <- stepResult{index: idx, pc: stepPC}
        }(i, step)
    }
    // Merge context data from all successful steps
    for res := range results {
        for k, v := range res.pc.ContextData {
            pc = pc.WithContextData(k, v)
        }
    }
}
```

---

### Decision 7: Graceful Degradation per Stage

**Chosen:** Each pipeline stage handles its own errors independently

**Why:** The orchestrator's `Execute()` method treats every stage except the executor as best-effort. Knowledge search failure, memory retrieval failure, and routing failure produce log warnings but do not abort the pipeline:

```go
// internal/orchestration/orchestrator.go
func (e *Engine) Execute(ctx context.Context, req *Request) (*Result, error) {
    // Context Builder: non-fatal
    if e.contextBuilder != nil {
        pc, err = e.contextBuilder.Build(ctx, pc)
        if err != nil {
            logger.Warn().Err(err).Msg("context builder failed, continuing")
        }
    }
    // Router: non-fatal
    if e.router != nil {
        pc, err = e.router.Route(ctx, pc)
        if err != nil {
            logger.Warn().Err(err).Msg("router failed, continuing")
        }
    }
    // Executor: FATAL — this is the core of the pipeline
    if e.executor != nil {
        pc, err = e.executor.Execute(ctx, pc)
        if err != nil {
            return nil, fmt.Errorf("orchestration: executor failed: %w", err)
        }
    }
    // Pipeline (skills): non-fatal
    if e.pipeline != nil {
        pc, err = e.pipeline.Execute(ctx, pc, e.config.Pipeline)
        if err != nil {
            logger.Warn().Err(err).Msg("pipeline execution failed, continuing")
        }
    }
    // MAG storage: non-fatal
    if e.config.EnableMAG && e.mag != nil {
        stored, err := e.mag.StoreResult(ctx, pc, result)
        if err != nil {
            logger.Warn().Err(err).Msg("MAG store result failed")
        }
    }
}
```

**Per-step error policy** via `OnError` field:

| OnError | Behavior |
|---------|----------|
| `"skip"` | Silently skip; continue to next step |
| `"warn"` | Log warning; continue to next step |
| `"fail"` (default) | Return error immediately |

---

### Decision 8: Streaming Architecture

**Chosen:** `ExecuteStream()` with buffered channel of `StreamEvent`, non-blocking emit with timeout

**Why:** The streaming pipeline runs pre-execution stages synchronously (MAG, context builder, router), then hands off to the executor's `readStream` background goroutine. The goroutine reads from `ChatStream.Recv()` and forwards chunks through a buffered channel. The channel is closed when the underlying stream ends.

Event types:

```go
type StreamEventType string
const (
    StreamEventProgress        StreamEventType = "progress"
    StreamEventChunk           StreamEventType = "chunk"
    StreamEventStageTransition StreamEventType = "stage_transition"
    StreamEventError           StreamEventType = "error"
)

type StreamEvent struct {
    Type     StreamEventType
    Content  string
    Metadata map[string]interface{}
}
```

Non-blocking emit prevents the streaming goroutine from deadlocking if the consumer is slow:

```go
func (e *Executor) emitEvent(eventCh chan<- StreamEvent, ev StreamEvent) {
    timer := time.NewTimer(100 * time.Millisecond)
    defer timer.Stop()
    select {
    case eventCh <- ev:
    case <-timer.C:  // drop event if consumer is not reading
    }
}
```

The orchestrator's `runStreamingPipeline` employs a similar 500ms timeout emit for pre-execution events.

**Streaming flow:**

```
Client calls ExecuteStream(ctx, req)
  → returns <-chan StreamEvent immediately
  → background goroutine runs:
     [MAG AugmentContext] → emit StreamEventProgress
     [ContextBuilder.Build] → (no emit)
     [Router.Route] → emit StreamEventProgress with agent name
     [Executor.ExecuteStream] → spawns readStream goroutine
       → readStream emits StreamEventChunk for each token
       → emits StreamEventStageTransition on "[STAGE: name]" markers
       → emits StreamEventProgress on completion
       → closes eventCh
```

---

### Decision 9: Memory-Augmented Generation (MAG)

**Chosen:** Augment → Execute → Store cycle with automatic memory persistence

**Why:** The MAG module wraps the pipeline in a memory lifecycle:

```go
type MAG struct {
    retriever MemoryRetriever
    storer    MemoryStorer
    config    MAGConfig
}

type MAGConfig struct {
    AutoStore          bool          // default: true
    AutoRetrieve       bool          // default: true
    MemoryLayer        string        // default: "session"
    MemoryType         string        // default: "decision"
    MaxRetrieveResults int           // default: 10
    StoreTTL           time.Duration // default: 24h
    MinPriority        int           // default: 5
}
```

Three lifecycle hooks:

1. **Pre-execution** (`AugmentContext`): Searches memory for records relevant to the prompt, stores a formatted summary in `ContextData["memory_context"]` and raw records in `ContextData["retrieved_memories"]`
2. **During execution** (`RecordDecision`): Stores intermediate decisions with pipeline stage annotations — useful for multi-step agent workflows
3. **Post-execution** (`StoreResult`): Formats the complete result (request, agent, response, skills, duration) as structured Markdown and persists it. Priority increases with the number of skills used

All MAG operations are non-fatal — log warnings on failure, continue pipeline:

```go
stored, storeErr := e.mag.StoreResult(ctx, resultPc, result)
if storeErr != nil {
    log.Warn().Err(storeErr).Msg("MAG: result storage failed, but pipeline succeeded")
}
```

---

### Decision 10: Provider Architecture

**Chosen:** Factory pattern with auto-registration via `init()`, priority-based selection

**Why:** Every provider package registers itself into the global `ChatRegistry` singleton at import time:

```go
// internal/providers/openai/chat.go
func init() {
    registry := chat.GetRegistry()
    name, factory := ChatFactory()
    registry.Register(name, factory, "OpenAI Chat Completions API", 10)
}

func ChatFactory() (string, chat.ChatProviderFactory) {
    return "openai", func(ctx context.Context, cfg map[string]interface{}) (chat.ChatProvider, error) {
        config := DefaultChatConfig()
        if cfg != nil {
            if v, ok := cfg["model"].(string); ok && v != "" {
                config.Model = v
            }
            // ... other overrides
        }
        return NewChat(config)
    }
}
```

The registry supports:
- **Auto-detection**: `ChatRegistryConfig{AutoDetect: true}` iterates registered providers by priority
- **Explicit primary**: `ChatRegistryConfig{Primary: "openai"}` forces a specific provider
- **Fallback chain**: `ChatRegistryConfig{Fallbacks: []string{"anthropic", "ollama"}}` defines ordered fallbacks
- **Priority ordering**: Lower numeric priority = higher preference (10 = highest, 100 = lowest)

**Provider stats tracking** via atomic counters:

```go
type ChatStats struct {
    TotalRequests int64
    TotalTokens   int64
    Errors        int64
}
```

---

### Decision 11: CLI Integration

**Chosen:** `cosca run` command with `--stream`, `--agent`, `--provider` flags

The orchestrator exposes two execution modes that map directly to CLI commands:

```
cosca run "implement authentication flow"
  → synchronous Execute(): returns Result with full response, agent, skills, memory ID

cosca run --stream "implement authentication flow"
  → streaming ExecuteStream(): emits progress events, content chunks in real-time

cosca run --agent "Backend Chief" "build an API"
  → explicit agent override in ContextData["agent"], router skips keyword matching

cosca run --provider "anthropic" "design architecture"
  → ChatRegistryConfig{Primary: "anthropic"}, forces specific LLM provider

cosca pipeline run --file workflow.yaml
  → loads PipelineDefinition, passes to engine.Pipeline.Execute()
```

---

## Rationale (Cross-Cutting)

### Immutability over Mutability
Pipeline stages are sequential but may become concurrent. Immutable context guarantees no data races without mutexes. The shallow-copy approach keeps overhead minimal — only the maps that change are duplicated.

### Consumer-Defined Interfaces over Exported Interfaces
Go idiom: declare interfaces where you consume them. The orchestration package owns the contract; subsystems can evolve independently as long as adapters are maintained.

### Best-Effort over Strict Fail-Fast
The architecture treats the LLM call as the critical path. All other stages (knowledge search, memory retrieval, routing, MAG storage) are enrichment — losing them degrades quality but doesn't break the user experience.

### Channels over Callbacks for Streaming
Go channels are the idiomatic way to stream data. They compose naturally with `select`, support buffering, and cleanly signal completion via `close()`. The non-blocking emit pattern prevents deadlocks without requiring complex goroutine lifecycle management.

---

## Package Structure (Implemented)

```
internal/
├── chat/                              # LLM Chat Provider contract & registry
│   ├── types.go                       # ChatProvider, ChatStream, Message, ToolCall, ChatOptions, etc.
│   └── registry.go                    # ChatRegistry (global singleton, factory, fallback, stats)
│
├── orchestration/                     # AI Orchestration Engine
│   ├── types.go                       # Request, Result, PipelineContext, StreamEvent, Orchestrator interface
│   ├── ports.go                       # Consumer-defined ports: KnowledgeSearcher, MemoryRetriever/Storer, AgentResolver, SkillResolver
│   ├── orchestrator.go                # Engine struct, NewEngine(), Execute(), ExecuteStream(), runStreamingPipeline()
│   ├── context.go                     # ContextBuilder: Build(), BuildKnowledge(), BuildMemory(), augmentPrompt()
│   ├── router.go                      # Router: keywordAgentMap, keywordSkillMap, Route(), matchKeywords(), deriveSkills()
│   ├── executor.go                    # Executor: system prompt builder, chatWithRetry(), isTransientError(), tool derivation, readStream()
│   ├── pipeline.go                    # Pipeline: executeSequential(), ExecuteParallel(), ExecuteStep(), builtin processors, Service wrapper
│   ├── mag.go                         # MAG: AugmentContext(), RecordDecision(), StoreResult(), WrapExecute(), WrapExecuteStream()
│   ├── pipeline_test.go              # 1100 lines of unit tests (sequential, parallel, conditions, timeout, builtins)
│   ├── executor_test.go              # 1150 lines of unit tests (mock chat provider, retry, streaming, tools, prompts)
│   └── router_test.go                # Router tests
│
├── adapter/                           # Adapts internal subsystems → orchestration ports
│   ├── memory.go                      # MemoryAdapter: MemoryRetriever + MemoryStorer (maps to memory.MemoryEngine)
│   └── knowledge.go                   # KnowledgeAdapter: KnowledgeSearcher (maps to knowledge.Engine)
│
└── providers/                         # LLM + Embedding provider implementations
    ├── providers.go                   # Manager placeholder (existing)
    ├── openai/
    │   ├── openai.go                  # Embeddings provider (New, GenerateEmbedding, Factory, init)
    │   ├── chat.go                    # Chat provider (NewChat, Chat, ChatStream, SSE stream, ChatFactory, init)
    │   └── chat_test.go
    ├── anthropic/
    │   ├── anthropic.go               # Embeddings provider (stub — no API)
    │   └── chat.go                    # Chat provider (full Messages API with SSE, tool_use, content blocks)
    ├── deepseek/
    │   ├── deepseek.go                # Embeddings provider
    │   └── chat.go                    # Chat provider (OpenAI-compatible SSE)
    ├── groq/
    │   ├── groq.go                    # Embeddings provider
    │   └── chat.go                    # Chat provider (OpenAI-compatible SSE)
    ├── mistral/
    │   ├── mistral.go                 # Embeddings provider
    │   └── chat.go                    # Chat provider (OpenAI-compatible SSE)
    ├── google/
    │   ├── google.go                  # Embeddings provider
    │   └── chat.go                    # Chat provider (Gemini generateContent + streamGenerateContent)
    ├── ollama/
    │   ├── ollama.go                  # Embeddings provider
    │   └── chat.go                    # Chat provider (NDJSON streaming, delta computation)
    ├── azure/
    │   └── azure.go                   # Embeddings provider only (no chat implementation)
    ├── bedrock/
    │   └── bedrock.go                 # Embeddings provider (AWS SigV4 signing)
    └── local/
        └── local.go                   # Zero-dependency TF-IDF embeddings (fallback)
```

---

## Dependency Graph

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI (cosca run)                           │
│  Imports: orchestration, chat, adapter, providers           │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  orchestration.Engine                         │
│  Dependencies (ports):                                       │
│    KnowledgeSearcher  ← adapter.KnowledgeAdapter             │
│    MemoryRetriever    ← adapter.MemoryAdapter                │
│    MemoryStorer       ← adapter.MemoryAdapter                │
│    AgentResolver      ← agents.Manager (direct)              │
│    SkillResolver      ← skills.Manager (direct)              │
│    ChatProvider       ← chat.ChatRegistry (satisfies iface)  │
│    Embedder           ← embeddings.ProviderRegistry          │
└───────────────────────────┬─────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────────┐
│  chat        │  │  adapter     │  │  providers/*     │
│  (interface  │  │  (glue code) │  │  (impl)          │
│   + registry)│  └──────┬───────┘  └────────┬─────────┘
└──────────────┘         │                   │
                         ▼                   │ import side-effects
              ┌──────────────────┐           │ for init()
              │  knowledge        │           │
              │  memory           │◄──────────┘
              │  agents           │
              │  skills           │
              │  embeddings       │
              └──────────────────┘
```

---

## Consequences

### Positive

- **Complete LLM provider abstraction**: 9 chat providers (OpenAI, Anthropic, DeepSeek, Groq, Mistral, Google, Ollama) with 7 supporting streaming — all behind a single `ChatProvider` interface
- **Reusable pipeline architecture**: The `Pipeline` with `SkillProcessor` registration, sequential/parallel/conditional execution, per-step timeouts, and OnError policies can execute any workflow
- **Full streaming support**: `ExecuteStream()` with `StreamEvent` types (progress, chunk, stage_transition, error) supports real-time CLI output and future API streaming
- **Automatic memory persistence**: MAG's Augment → Execute → Store cycle captures every orchestration result as a searchable, prioritized memory record
- **Graceful degradation**: Knowledge search, memory retrieval, routing errors are non-fatal — the system works even when enrichment subsystems are unavailable
- **Testable design**: All dependencies injected via constructor; tests use mock implementations of port interfaces (see `pipeline_test.go` — 1100 lines, `executor_test.go` — 1150 lines)

### Negative

- **AWS Bedrock requires complex SigV4 signing**: The Bedrock embedding provider implements a full AWS Signature V4 signer (~200 lines) that must be maintained
- **Azure requires separate API version management**: The Azure embedding provider must track API version compatibility independently of the rest of the provider layer
- **Pipeline adds overhead for simple requests**: A single `Chat()` call goes through ContextBuilder, Router, Executor, MAG — unnecessary for direct LLM queries that don't need enrichment
- **Provider auto-registration via `init()`**: Import side-effects mean dead code elimination cannot remove unused providers; all registered factories are always available

### Neutral

- **Local-only operation**: The `local` TF-IDF provider enables zero-dependency embedding but produces lower quality vectors than API-based providers
- **Streaming protocols differ**: OpenAI/Anthropic use SSE, Ollama uses NDJSON with cumulative content (requires delta computation), Gemini uses a custom SSE format — the `ChatStream` interface abstracts this but each provider must implement mapping logic
- **Router is keyword-based**: Keyword matching is fast and deterministic but requires manual maintenance of the `keywordAgentMap` and `keywordSkillMap` tables as new agents and skills are added

---

## Related

- [ADR-001: Cosca Enterprise Architecture](ADR-001-cosca-cli-architecture.md)
- [ADR-005: AI Orchestration Engine (blueprint)](ADR-005-ai-orchestration.md)
- `internal/chat/types.go` — ChatProvider interface definition
- `internal/chat/registry.go` — ChatRegistry with fallback chain
- `internal/orchestration/orchestrator.go` — Engine implementation
- `internal/orchestration/executor.go` — LLM executor with retry logic
- `internal/orchestration/pipeline.go` — Skill pipeline execution
- `internal/orchestration/mag.go` — Memory-Augmented Generation
- `internal/orchestration/ports.go` — Consumer-defined port interfaces
- `internal/adapter/memory.go` — Memory engine adapter
- `internal/adapter/knowledge.go` — Knowledge engine adapter
