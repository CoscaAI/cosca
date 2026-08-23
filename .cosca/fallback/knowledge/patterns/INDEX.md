# Patterns Knowledge Domain

> **Category**: Patterns | **Version**: 2.1.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-08-09

## Purpose

Architecture, design, code, testing, and security patterns — cross-project reusable knowledge. Patterns are loaded by the Memory Engine during context initialization and referenced by the Planning Engine, Evolution Engine, and all Department Chiefs.

## Categories

### Architecture Patterns (3)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`architecture-patterns.md`](architecture-patterns.md) | General Architecture Patterns | 0.90 | 2026-07-12 |
| [`pattern-enterprise-gaps.md`](pattern-enterprise-gaps.md) | Enterprise Architecture Gaps in AI Orchestration | 0.85 | 2026-07-12 |
| [`enterprise-prompt-governance.md`](enterprise-prompt-governance.md) | Enterprise Prompt Governance Pattern | 0.95 | 2026-07-12 |

### Design Patterns (3)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`api-patterns.md`](api-patterns.md) | API Design Patterns | 0.90 | 2026-07-12 |
| [`testing-patterns.md`](testing-patterns.md) | Testing Patterns | 0.90 | 2026-07-12 |
| [`cosca-product-pattern.md`](cosca-product-pattern.md) | Cosca Product Pattern — uma ferramenta = um comando (PADRÃO OFICIAL) | 0.96 | 2026-08-12 |

### Go-Specific Patterns (3)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`go-provider-pattern.md`](go-provider-pattern.md) | Go Provider Pattern | 0.90 | 2026-07-12 |
| [`go-plugin-wasm-pattern.md`](go-plugin-wasm-pattern.md) | Go Plugin WASM Pattern | 0.90 | 2026-07-12 |
| [`go-editor-adapter-pattern.md`](go-editor-adapter-pattern.md) | Go Editor Adapter Pattern | 0.90 | 2026-07-12 |

### Integration Patterns (2)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`comfyui-blueprints.md`](comfyui-blueprints.md) | ComfyUI Blueprints (node-based diffusion, 89 blueprints) | 0.93 | 2026-08-02 |
| [`tensorart-openapi-integration.md`](tensorart-openapi-integration.md) | TensorArt OpenAPI Integration (image/video gen, presigned upload, polling, ComfyUI nodes) | 0.85 | 2026-08-08 |

### AI/Generation Patterns (1)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`diffusers-pipeline-architecture.md`](diffusers-pipeline-architecture.md) | Diffusers Pipeline Architecture (text-to-image, LoRA, otimização VRAM, produção) | 0.88 | 2026-08-08 |

### Enterprise Platform Patterns (10)

| File | Pattern Name | Confidence | Date |
|------|-------------|:----------:|------|
| [`civitai-enterprise-platform-patterns.md`](civitai-enterprise-platform-patterns.md) | CivitAI Enterprise Platform Patterns (15 patterns: hub-and-spoke auth, outbox CDC, tRPC guards, bitwise flags, fail-open, event engine, virtual currency, analytics) | 0.92 | 2026-08-09 |
| [`facefusion-pipeline-patterns.md`](facefusion-pipeline-patterns.md) | FaceFusion Pipeline & Job System Patterns (10 patterns: pipe-and-filter processors, file-based job queue, step/key split, dual-context state, ONNX multi-GPU, FFmpeg builder, 3-tier memory, error codes, CRC32 integrity, modular argparse) | 0.93 | 2026-08-09 |
| [`backstage-plugin-platform-patterns.md`](backstage-plugin-platform-patterns.md) | Backstage Plugin Platform Patterns (15 patterns: Plugin-Module-ExtensionPoint triad, ServiceRef DI container, Extension Tree model, Blueprint factory, Catalog processing pipeline, Scaffolder template engine+actions, Search engine abstraction, Permission RBAC/ABAC conditional, Event system tri-modal, Test harness multi-DB, Config schema compilation, Entity model Kubernetes-style, Notification processor pipeline, Module federation, Opaque type discriminators) | 0.93 | 2026-08-09 |
| [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) | Temporal Workflow Engine Patterns (12 patterns: Event sourcing+replay, Staged execution decision-event-task, Task queue durable buffer, Optimistic concurrency conditional updates, Shard partitioning RangeID fencing, Checksum nondeterminism detection, Hierarchical state machine HSM, fx dependency injection, Dynamic configuration, Durable timers, Multi-tenancy namespace replication, Callback+Nexus components) | 0.95 | 2026-08-09 |
| [`argocd-gitops-reconciliation-patterns.md`](argocd-gitops-reconciliation-patterns.md) | Argo CD GitOps & Reconciliation Patterns (12 patterns: Double-loop reconciliation, Three-tier diff, Phased+waved sync, Event-driven+polling hybrid, Health assessment built-in+Lua, Generator template generation, Progressive sync state machine, Resource ownership stamping, Notification trigger/template/service, Cluster-per-shard caching, Repo server separated, Project multi-tenancy Casbin) | 0.94 | 2026-08-09 |
| [`kubernetes-temporal-ecosystem-patterns.md`](kubernetes-temporal-ecosystem-patterns.md) | Kubernetes + Temporal Ecosystem Patterns (15 patterns: Contract-first+code generation, SharedInformer+Lister+Workqueue trinity, Spec/Status split+reconciliation, Interface segregation by capability, Two-phase admission, Scheduler plugin extension points, Optimistic concurrency ResourceVersion, Watch+Bookmark, CRD+conversion webhooks, Typed errors StatusError, Determinism enforcement, Test frameworks time control, Multi-dimensional autoscaling, Leader election lease, AI workflows as durable workflows) | 0.95 | 2026-08-09 |
| [`infrastructure-observability-patterns.md`](infrastructure-observability-patterns.md) | Infrastructure & Observability Patterns (15 patterns: OTel Span+W3C+SDK+Collector, OpenFGA ReBAC DSL+Check+Usersets+ABAC, Dapr Sidecar+Building Blocks+Components+Resiliency, OpenHands EventLoop+ToolContract+Condensation) | 0.92 | 2026-08-09 |
| [`vscode-go-extension-patterns.md`](vscode-go-extension-patterns.md) | vscode-go Extension Patterns (LSP lifecycle orchestrator, circuit breaker, provider swap, helper process, 3-mode IPC, versioned tool catalog, dlv-dap proxy, dual-parser test runner, CommandFactory, batch telemetry) | 0.93 | 2026-08-13 |
| [`kubernetes-core-patterns.md`](kubernetes-core-patterns.md) | Kubernetes Core Patterns (17 patterns: informer/DeltaFIFO, keyed workqueue, reconcile loop, lister/indexer, parallelizer, scheduler extension points, two-phase cycle, typed status chain, priority queue+backoff, CycleState, plugin registry, Scheme/GVK, hub-and-spoke conversion, declarative validation, admission chain, storage+resourceVersion, watch+bookmark) | 0.95 | 2026-08-13 |
| [`hermes-agent-patterns.md`](hermes-agent-patterns.md) | Hermes Agent Patterns (self-evolution DSPy+GEPA, skill-as-module, benchmarks-as-gates, guardrails, session-DB mining, memory nudges, auto-triage, trajectory compression, SessionDB FTS5) | 0.93 | 2026-08-13 |
| [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) | DeepSeek Harness Patterns (28 patterns: plugin-composition Cordis, sandbox landlock self-restrict, probed runner chain full/partial, process-tree escalation, skill registry multi-layer, catalog digest, subagent capabilities+delegation lock, approval waterfall fail-closed, CredentialRef, durable session append-only+surface, compaction bracket, whole-value projection, spill policy, typed wire contract ACP/MCP, generate-and-diff gates) | 0.90 | 2026-08-22 |
| [`openai-agents-sdk-patterns.md`](openai-agents-sdk-patterns.md) | OpenAI Agents SDK Patterns (7 patterns: agent declarativo+clone, handoff de 1ª classe+history nest, Runner loop+RunState, RunContextWrapper+ledger approval, function_tool+agent.as_tool, guardrails tripwire paralelos, tracing spans+gather_with_cancel) | 0.90 | 2026-08-22 |
| [`openai-symphony-patterns.md`](openai-symphony-patterns.md) | OpenAI Symphony Patterns (7 patterns: WORKFLOW.md política versionada, orquestrador de claim, runs isoladas workspace+path-safety, worker sessão contínua+re-checagem, retry 2 níveis, concorrência 3 níveis, tracker kernel de leitura+segredos host) | 0.88 | 2026-08-22 |
| [`vercel-ai-sdk-patterns.md`](vercel-ai-sdk-patterns.md) | Vercel AI SDK Patterns (7 patterns: provider factory+spec versionada, Output tipado+parsePartialJson, tool union+deferred, multi-step tool loop+stopWhen, streaming 3 camadas+finish-reason, capacity-aware batching de embedding) | 0.88 | 2026-08-22 |
| [`aws-agent-toolkit-patterns.md`](aws-agent-toolkit-patterns.md) | AWS Agent Toolkit Patterns (7 patterns: gate em código não em instrução, credencial na borda via Gateway, least-privilege determinístico+confused deputy, blast radius separado+alerta, auth dual SigV4/JWT, sessão/quota, progressive disclosure+observabilidade) | 0.88 | 2026-08-22 |
| [`n8n-workflow-patterns.md`](n8n-workflow-patterns.md) | n8n Workflow Engine Patterns (7 patterns: índice bidirecional origem+destino, stack+waitingExecution barrier, pairedItem lineage, WorkflowDataProxy lazy+isolate, credenciais envelope-key, queue+leader election+recovery, runExecutionData versionado+hooks) | 0.90 | 2026-08-22 |
| [`hermes-self-evolution-patterns.md`](hermes-self-evolution-patterns.md) | Hermes Self-Evolution Patterns (7 patterns: texto-que-vira-genoma, GEPA reflexivo guiado por feedback, benchmarks como GATES, corral de restrições+anti-bloat, eval 4 fontes+holdout, rubrica+parsing robusto, auto-triage+deploy via PR) | 0.93 | 2026-08-22 |
| [`uber-cadence-patterns.md`](uber-cadence-patterns.md) | Uber Cadence Patterns (7 patterns: decisor+event-sourcing, replay determinístico, matching persistência+canais, sticky tasklists, partições em árvore+forwarding, activity vs workflow+heartbeats, history sharding+NDC-AP) | 0.90 | 2026-08-22 |
| [`kubernetes-org-patterns.md`](kubernetes-org-patterns.md) | Kubernetes Org Patterns (27 patterns: CRI sandbox gRPC plugável+feature handshake, sandbox lifecycle idempotente, spec declarativa de isolamento, ExecSync bounded/assert stream, VPA decaying histogram+estimador em banda+anti-thrash, CA simulação de capacidade+eviction PDB-aware, KEP estados+graduação, OWNERS 2 fases, SIG/WG charter+OARP, escada de contribuidor, triage lifecycle, RFC2119+super-majority, kube-state-metrics state->metrics+cardinalidade allowlist+health one-hot+sharding) | 0.90 | 2026-08-22 |
| [`google-agent-patterns.md`](google-agent-patterns.md) | Google Agent Patterns (14 patterns: agent como estrutura de dados+clone, transfer enum-restrito, agent loop por processors, event-sourcing+rewind, InvocationContext resumável, plugins hooks lifecycle, workflow trigger-buffer+scheduler; skills frontmatter mínimo+Use/Don't-use, progressive disclosure, disambiguation, composição+fallback, guardrails em content, anti-alucinação por MCP, ecossistema instalável) | 0.88 | 2026-08-22 |
| [`anthropics-skills-patterns.md`](anthropics-skills-patterns.md) | Anthropic/Claude Patterns (21 patterns: hooks contrato eventos+exit-code, permissões 3 estados+managed-settings, memória sessão lock+TTL, revisão git-baseline, feedback async rewake, loop auto-referencial pela promise, trust-model anti-prompt-injection; skills frontmatter validável, progressive disclosure 3 níveis, roteador por domínio, descrição anti-undertrigger, scripts caixa-preta, marketplace .skill, meta-loop A/B; SDK query/ClaudeSDKClient, full-duplex multiplexado, can_use_tool, hooks tipados, tools in-process MCP, SessionStore protocol, batcher mirror) | 0.90 | 2026-08-22 |
| [`ruflo-patterns.md`](ruflo-patterns.md) | Ruflo Patterns (35 patterns: swarm topology+anti-drift, closed-loop learned routing, weighted semantic matching, DAG scheduling+rollback, graceful failure contract, cache-aware loop heartbeat, capability brain data-only+5 fatos, memória imutável invalida-não-sobrescreve, SmartRetrieval 5 fases (RRF/MMR/recency), hybrid search 3 braços+signals, reforço de confiança boost/decay/EWC, pattern mining EMA/pruning, registry degradável, ponte legível↔vetorial+escopo triplo, federação A2A card+signed manifest+challenge handshake+JCS envelope+PII transform+trust ladder+cicruit breaker, meta-harness degradável+hooks membrana+skill-as-tool pin-version+self-introspection+envelope uniforme+mint/evolve/flywheel, baseline monotônico+witness manifest+drift catalog+contrato plugin) | 0.90 | 2026-08-23 |
| [`mega-brain-patterns.md`](mega-brain-patterns.md) | Mega Brain Patterns (31 patterns: conclave deliberação multi-agente — separação domínio/meta-cognição, zero-achismo evidência rastreável, convergência calculada+circuit breaker, confiança aritmética+thresholds, votação cruzada sem auto-voto+juiz-relay, arquétipos adversarial+FMEA RPN, síntese SPEC acionável; DNA cognitivo L1-L10+discriminadores, peso+genealogia, pipeline MCE checkpoints, cascata de granularidade, append-only idempotente, agente como projeção do DNA; RAG grounded — RRF+cosine, invariante espaço único embedding+quarentena, fail-open, cascata fidelidade self-RAG→HHEM→block, atribuição por claim, gabarito congelado+gate CI, hyperedge+atomic_facts+saúde; orquestração multi-squad — plan-only separado, registry+capability cache, roteamento por intenção+elicitation, DAG architect+CPM+FMEA, maturidade 0→1→10→100, quality gate 3 estados+veto, executor DAG paralelo+token-fencing+FSM resume) | 0.90 | 2026-08-23 |
| [`ai-products-patterns.md`](ai-products-patterns.md) | AI Product Patterns (34 produtos/empresas de IA: Mistral open-weight/MoE, Grok real-time, Pi persona; Midjourney/Ideogram/Recraft/Leonardo/Magnific imagem+design+vector+upscale, Runway/Luma vídeo+3D, Synthasia/HeyGen avatar+lipsync, Suno música, Imgs curadoria; Copy.ai/Jasper/MarketMuse/QuillBot/Rephrase/Speechify escrita; Fireflies/Krisp/Vidyo meeting intelligence; Gamma/SlidesAI/Uizard/Looka/Durable/Taskade produtividade; DoNotPay/AdCreative/InVideo/OpusClub consumer; Claude Artifacts artefato vivo) | 0.70 | 2026-08-23 |
| [`generative-media-patterns.md`](generative-media-patterns.md) | Generative Media Patterns (30 patterns: PCG Sceelix — dataflow graph engine, schema reflexivo, atributos+gramática, shape grammar BoxScope, terreno camadas+Perlin multi-oitava, seed/cache determinismo, componentes serializáveis; 3D Gaussian Splatting — gaussianas 3D, ativação inversa, SH view-dependent, SfM→semeadura, densificação adaptativa, rasterização diferenciável, LR por-propriedade; imagem ComfyUI — node graph declarativo, execução+ cache assinatura, ModelPatcher, LoRA patch, condicionamento/CFG, sampler/denoise, API assíncrona; áudio AudioCraft — codec discreto→LM, interleaving codebooks, fusão condicionamento, CFG empacotado, janelas+KV-cache, pós-processamento, BaseGenModel) | 0.90 | 2026-08-23 |
| [`unreal-integration-patterns.md`](unreal-integration-patterns.md) | Unreal Engine Integration Patterns (22 patterns: PCG grafo espacial data-driven+seed, PCGData átomo de mundo, regras reconfiguráveis, runtime vs cook; World Partition células+streaming+Data Layers; Gameplay Framework GameMode/GameState/Controller+Pawn/Character+Actor Component; AI Framework AIController+Behavior Tree+Blackboard, Perception, GAS abilities/attributes/effects, animação/Control Rig; Data Assets+Blueprint/C+++UInterface; ponte Cosca↔Unreal — agente como ACoscaAgentPawn (corpo) dirigido pela mente Cosca, loop percebe→decide→age→estado, WebSocket bridge) | 0.78 | 2026-08-23 |
| [`mining-map-world-vivo.md`](mining-map-world-vivo.md) | Mapa de Mineração do Mundo Vivo — 10 camadas tecnológicas (Procedural World, Character/3D, Game AI, Agent Memory, Vision, Spatial AI, VFX, Audio, Destruction, Simulation), template REPO→UTILITY por projeto, top projetos por camada com license+stars, gaps identificados, ordem de mineração recomendada, prioridade: projetos que resolveram "IA→operacional" (professor) | 0.82 | 2026-08-23 |

## Statistics

| Metric | Value |
|--------|-------|
| Total patterns | 39 |
| Architecture patterns | 3 |
| Design patterns | 3 |
| Go-specific patterns | 3 |
| Integration patterns | 2 |
| AI/Generation patterns | 1 |
| Enterprise Platform patterns | 19 |
| Agent patterns | 4 |
| Confidence range | 0.85 – 0.96 |
| Average confidence | ~0.92 |

## Confidence & Usage Tracking

Each pattern carries a **confidence score** (0.0–1.0) representing how well-validated the pattern is through cross-agent consensus and real-world application:

| Score Range | Interpretation | Action |
|-------------|---------------|--------|
| 0.90–1.00 | High confidence — validated across multiple agents | Use as canonical reference |
| 0.80–0.89 | Medium-high — validated by 1–2 agents | Use with awareness, seek cross-validation |
| 0.70–0.79 | Medium — single-agent insight | Corroborate before applying broadly |
| <0.70 | Low — experimental or speculative | Treat as proposal, not pattern |

## Structure

Each pattern follows the canonical format:
- **Intent** — What problem does this solve?
- **Context** — When and where should this be applied?
- **Solution** — The pattern description and implementation guidance
- **Consequences** — Trade-offs, benefits, and drawbacks
- **Known Uses** — Where has this been successfully applied?
- **Related Patterns** — Complementary or alternative patterns

## Usage

Patterns are loaded by the **Memory Engine** during context initialization and referenced by:
- **Planning Engine** — Task decomposition and agent assignment
- **Evolution Engine** — Codebase improvement suggestions
- **Department Chiefs** — Domain-specific decision-making
- **Semantic Memory Chief** — Cross-agent knowledge discovery

---

*Previously v1.0.0 — upgraded to v2.0.0 with full pattern inventory, confidence tracking, and category breakdown.*
