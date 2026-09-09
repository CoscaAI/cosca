# DeerFlow + Argo Workflows + Crossplane — Multi-Agent & Workflow Patterns

> **Sources**: DeerFlow (ByteDance fork), `argoproj/argo-workflows`, `crossplane/crossplane`
> **Analyzed**: 2026-08-09 — Web research + cross-agent analysis
> **Confidence**: 0.91

## Intent

Fechar os últimos gaps: multi-agent orchestration (DeerFlow), DAG execution (Argo Workflows), infrastructure abstraction (Crossplane).

## Patterns Extraídos (Top 12)

### PARTE 1: DEERFLOW — MULTI-AGENT

#### 1. Flat Orchestrator Pattern (Lead + Worker Pool)
Sem hierarquia fixa. Um Lead Agent decide quando spawnar sub-agents via tool `task()`. Workers são stateless, genéricos (`general-purpose`, `bash`), com timeout e contexto isolado. Diferente do Cosca (Kernel→CEO→CTO→Chiefs→Specialists), é task-parallel, não chain-of-command.

#### 2. Progressive Skill Loading (Pull-Based)
Skills são arquivos `SKILL.md` com frontmatter YAML. O system prompt lista paths; o agente decide quando fazer `read_file` no skill relevante. Economiza contexto vs. injeção total do catálogo.

#### 3. Isolated Sub-Agent Context
Cada sub-agent opera em contexto totalmente isolado do orquestrador. Output retornado como string. Previne poluição de contexto e permite paralelismo real.

#### 4. Middleware Chain (Cross-Cutting Concerns)
Memória, uploads, summarization, loop detection, títulos, clarifications — cada um é middleware independente composto em cadeia ordenada. ThreadData→Uploads→Summarization→Todo→Title→Memory→LoopDetection→Clarification.

### PARTE 2: ARGO WORKFLOWS — DAG EXECUTION

#### 5. Declarative Template Composition
Templates são "funções" reutilizáveis (container, script, DAG, steps, suspend, http). `WorkflowTemplate` CRD permite referenciar templates por nome entre workflows.

#### 6. DAG as Explicit Dependency Graph
`dependencies: [A, B]` — runtime computa paralelismo do grafo. Steps = list-of-lists sequencial, DAG = partial order arbitrário.

#### 7. Artifacts as Typed Data Channels
`outputs.artifacts` → `inputs.artifacts` entre steps. Binding `{{steps.X.outputs.artifacts.Y}}` cria dependência de dados implícita que complementa o DAG estrutural.

#### 8. Parameters + Conditionals + Loops
`when: "{{steps.X.outputs.result}} == success"` + `withParam` para loops dinâmicos. Branching e fan-out declarativos.

### PARTE 3: CROSSPLANE — INFRA ABSTRACTION

#### 9. XRD → Composition → XR Pipeline (DRY Pattern)
4 camadas: **XRD** (schema da API) → **Composition** (pipeline de funções que traduz spec → managed resources) → **XR** (instância) → **Claim** (namespace-scoped). Separa "o que o usuário quer" de "como implementar".

#### 10. Composition Functions as Pipeline Steps
`mode: Pipeline` com steps ordenados. Cada Function é um gRPC server. Crossplane chama `RunFunctionRequest` → `RunFunctionResponse`. Funções acumulam desired state incrementalmente — Unix-pipe model para infra.

#### 11. Managed Resource as Source of Truth
Todo recurso cloud é um CR com `spec.forProvider` como estado autoritativo. Crossplane reconcilia continuamente: drift no console → revert. `managementPolicies`: Observe/Create/Update/Delete granular.

#### 12. Provider Abstraction
Provider = pacote que instala CRDs + controller. `ProviderConfig` separa "qual provider" de "como autenticar". Multi-account via múltiplas configs. `DeploymentRuntimeConfig` customiza o pod sem modificar o package.

---

## Comparação Cosca vs DeerFlow

| Dimensão | DeerFlow | Cosca |
|----------|----------|-------|
| Hierarquia | Flat (lead + workers) | 5 níveis (Kernel→CEO→CTO→Chiefs→Specialists) |
| Agentes | 2 tipos genéricos | 55 agentes especializados |
| Skills | Pull-based (agente decide carregar) | Push-based (Kernel injeta no contexto) |
| Memória | Blob profile/knowledge | 6 arquivos/agente, FTS5, auto-evolução |
| Sandbox | Docker/K8s real | bwrap jail |
| Delegação | task() tool call | Chain of command + delegation |

## Comparação Argo Workflows vs Temporal

| Dimensão | Argo Workflows | Temporal |
|----------|---------------|----------|
| Estado | CR status + Pod phases | Event history ledger + replay |
| Recuperação | Pod retry via K8s | Replay do event log |
| Dados | Artifacts tipados (S3/Git) | ActivityResult via SDK |
| Overhead | Pod por step | Worker pool (baixo overhead) |
| Long-running | suspend template | Timers nativos + signals |

---

## Confidence

| # | Pattern | Confidence |
|---|---------|:----------:|
| 1-4 | DeerFlow (Flat orchestrator, Progressive skills, Isolated context, Middleware chain) | 0.88 |
| 5-8 | Argo Workflows (Template composition, DAG, Artifacts, Params+Conditionals) | 0.93 |
| 9-12 | Crossplane (XRD→XR pipeline, Functions, Managed Resources, Provider abstraction) | 0.91 |

**Average**: ~0.91
