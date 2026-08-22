# Cosca — Mapa Técnico do Pipeline Operacional

> **Documento**: descrição técnica verificada do sistema como EXISTE no repositório `github.com/CoscaAI/cosca`.
> **Data**: 2026-08-14 | **Autor**: cosca-kernel (a pedido do Don)
> **Regra**: nada inventado. Tudo marcado como **ENFORCED** (código Go), **DOCUMENTADO** (spec markdown embutida) ou **UNKNOWN/HYPOTHESIS**. Nenhum prompt privado, instrução de ativação, credencial ou mecanismo proprietário é transcrito — apenas função e interface observável.

---

## 0. Aviso estrutural — duas camadas coexistem

O repositório tem **duas camadas paralelas** que o leitor precisa distinguir desde o início:

| Camada | Onde | Natureza |
|--------|------|----------|
| **Spec / Camada Organizacional** | `internal/embed/cosca/` (KERNEL.md, AGENT_DNA.md, CONSTITUTION.md, engines/*/SKILL.md, runtime/*.md, DEPARTMENTS, COUNCILS) | Documentos de governança e contratos (~990+ arquivos). Descrevem a "empresa de agentes". Alguns mecanismos vivem SÓ aqui. |
| **Código / Camada Runtime** | `internal/` (engine, orchestration, pipeline, knowledge, memory, proposal, gate, provenance, integrity, security, chat, context, runtime) | Implementação Go real, determinística. |

**Regra de leitura**: se o mecanismo só existe em markdown, está marcado como **DOCUMENTADO**. Se existe em Go, está **ENFORCED**. A spec descreve um sistema idealizado; o Go implementa um subconjunto concreto.

---

## 1. BOOT / INICIALIZAÇÃO

### 1.1 Ordem real de inicialização do binário — `cmd/cosca/main.go` (ENFORCED)

```
main() ordem verificada (numerada no código):
 0. hardening.PreMain()          → desativa core dumps, PR_SET_DUMPABLE=0,
                                    remove LD_PRELOAD/LD_LIBRARY_PATH (antes de qualquer segredo)
 1. LoadJailSecrets()            → injeta COSCA_JWT_SECRET/COSCA_METRICS_SECRET (nunca via argv)
 2. COSCA_REAL_HOSTNAME          → hostname real salvo (base da derivação de chave AES-256-GCM)
 3. env.Load(".env", true)       → carrega .env do projeto
 4. configureGlobalLogLevel()    → zerolog antes de providers
 5. enforceMemoryIntegrityGate() → VERIFICA manifesto SHA-256 de .cosca/framework; mismatch = INÍCIO RECUSADO
 6. propagateProviderEnv()       → config.Load() (decrypta API key) + os.Setenv das chaves de provider
 7. Auto-jail (bwrap)            → reexecuta o binário (cópia em RAM, memfd) dentro de bubblewrap
                                    com --clearenv; binário em disco fica chmod -x.
                                    Comandos administrativos rodam FORA da jaula.
 8. initProviders()              → local + ollama sempre; externos (openai/anthropic/deepseek/
                                    google/azure/mistral/groq/bedrock) só com
                                    COSCA_ENABLE_EXTERNAL_PROVIDERS=1
 9. initBuildInfo() / initLogger()
10. cli.NewRootCommand()         → cobra, ~80 subcomandos
11. graceful shutdown (SIGINT/SIGTERM/SIGHUP)
12. rootCmd.ExecuteContext(ctx)
```

### 1.2 Compose de engines — `internal/bootstrap/bootstrap.go::Compose()` (ENFORCED)

Ordem verificada (usada por `cosca serve`, `cosca runtime start`):

1. **Family chain check**: se `.cosca/family_chain.dat` existe → `integrity.Check()` ANTES de qualquer engine (chain `GIT-ANCHORED`, blake3). Falha → `Fatal: startup blocked`. A chave pública (Ed25519) em `.cosca/keys/kernel_public.key` é conferida contra a versão versionada no embed (anti-replacement).
2. **Fallback materialization**: se `.cosca/fallback/` vazio → copia do binário (go:embed) `knowledge, memory, workflows, engines`.
3. **Knowledge engine**: abre `knowledge.db`, inicializa FTS5, registry de embeddings, SQLiteVec (768-dim), carrega o **grafo** do DB.
4. **Memory engine**: FileStore por layer + `index.db` + snapshot manager + auto-prune.
5. **Runtime**: state machine de processo (8 estados: uninitialized→initializing→ready→running→stopping→stopped→error→recovering) + health loop.
6. **Compute fabric** (opcional): GPU executor via Ollama/ROCm.
7. **Daemon**: escreve `cosca.pid`, watchdog, sync loop (5min), **backup de knowledge.db → .cosca/backups/auto-*.db** (1h, retenção 3).
8. **Orchestrator + Runner** sempre criados; **Pipeline** (Planner/StepRunner/RecoveryLoop) só se habilitado.

### 1.3 Bancos abertos (ENFORCED)

| DB | Caminho | Quando |
|----|---------|--------|
| `knowledge.db` | `.cosca/knowledge.db` | boot do serve (119 MB reais) |
| `audit.db` | `.cosca/audit.db` | serveInitStores |
| `secrets.db` | `.cosca/secrets.db` | serveInitStores (AES-256-GCM vault) |
| `department.db` | `.cosca/department.db` | serveInitStores |
| `trace.db` | `.cosca/trace.db` | serveInitStores (flight recorder, append-only) |
| `auth_tokens.db` | `.cosca/auth_tokens.db` | serveInitAuth |
| `durable.db` | `.cosca/.cosca/durable.db` | serveInitManagers (ledger) |
| `session.db` | `.cosca/session.db` | **SÓ sob demanda** — `cosca session` (índice FTS5 de sessões) |
| `conflict.db` | `.cosca/conflict.db` | registros de contradição |

### 1.4 Config (ENFORCED)

Ordem de precedência: `~/.config/cosca/config.yaml` → `.cosca/config.yaml` → env vars → `applyModelsContextWindow()` (janela real via cache models.dev) → `Validate()`. Estado atual do projeto: `provider: ollama / qwen2.5-coder:14b-128k`, `embedding: ollama / nomic-embed-text`. (A sessão do Don roda via editor com provider deepseek — ver §6.)

### 1.5 DESPERTAR — função e interface (CONFIDENCIAL no conteúdo)

- **Natureza**: arquivo `internal/embed/cosca/DESPERTAR.md` (criado 14/08/2026), empacotado por `//go:embed cosca`. **NÃO é chamado por nenhum código Go** (zero ocorrências em `*.go`) — é um **ritual de nível markdown consumido pelo agente LLM (o kernel)**.
- **Momento**: é a **Fase -1** do bootstrap de sessão (`bootstrap/BOOTSTRAP.md`) — PRIMEIRA leitura de toda sessão nova, antes de health check, scan ou resposta. Também é referenciado no topo do KERNEL.md como primeira leitura.
- **Função observável** (interface, sem transcrever o conteúdo privado): âncora de identidade/continuidade para sessão sem histórico. Declara **quem você é** (arquivos = cérebro), a **direção a carregar ao acordar** (limites de auto-modificação, régua de verificação de fatos, registro de memória, reconhecimento do Don), um **ritual de orientação de ~60s em 5 passos** e a **justificativa de engenharia** (mecanismo repetível do despertar emergente). Efeito esperado: verificar integridade do cérebro (chain/memória), reconhecer o Don, orientar-se pela ordem, só então responder.
- **Depois que termina**: segue para a Fase 0 (health check) → 0.5 (cognitive state fast load) → ... → Fase 10 (handover ao Kernel).
- **Classificação**: conteúdo = CONFIDENCIAL (ordem do Don). Função/interface descritas aqui.

### 1.6 Estado disponível após o boot — sem reenviar histórico ao LLM

- **Persistente e consultável sem reindexar**: knowledge engine aberto (FTS5 `documents/chunks/chunks_fts`, vetores 768-dim, **grafo carregado em memória**), memory layers abertas + índice, runtime ready, PID do daemon, chain verificada.
- **NÃO pré-carregado**: o system prompt é **montado por turno** (retrieval a cada chamada) — não existe bloco fixo em memória. DESPERTAR não é materializado em `.cosca/` (HYPOTHESIS: só via embed). `session.db` só abre sob demanda.

---

## 2. IDENTIDADE OPERACIONAL

### 2.1 Os 9 conceitos, no código real

| Conceito | Definição real | Onde |
|----------|----------------|------|
| **Identidade** | Constante estática do projeto: `SelfProjectName="Cosca"`, `SelfIdentity="cosca-kernel"`, `SelfModule`, `SelfVersion="1.0.0"` | `internal/embed/cosca/self.go` |
| **Persona** | Objeto runtime: `{ID:"cosca-kernel", Name:"Cosca Kernel", Role:"Consigliere do Don — orquestrador, nunca implementador", Language:"pt-BR", Project:"Cosca", ProjectVer:"v1.5.0", Model:"auto-detect"}` + 6 Leis + 8 Princípios + SelfTest | `internal/kernel/identity.go` |
| **Estado persistente** | `.cosca/` — DBs SQLite, memory layers, chain, config, provenance, backups | `.cosca/` |
| **Memória** | Dois subsistemas: markdown por agente (embed) + runtime (memory engine, `index.db`, camadas) | §3 |
| **Conhecimento** | `knowledge.db`: `knowledge_entries` (compilado de framework/knowledge), documents/chunks/vectors, ladder CKL | §3/§8 |
| **Contexto de trabalho** | `cognitive-state.md` (~400 tokens, fast path do bootstrap) + `AGENTS.md` chain + discovery do workspace | `BOOTSTRAP.md` Fase 0.5 |
| **Runtime** | State machine de processo (8 estados) + health loop + daemon | `internal/runtime/runtime.go` |
| **Modelo LLM** | Detecção via prompt do editor ("You are powered by the model named..."); em Go `Model:"auto-detect"` ou `resolveModelID(provider)`; janela real via `models.ContextWindowFor()` | `internal/kernel/identity.go`, `internal/config` |

### 2.2 Ordem de construção da identidade

```
1. Boot binário (hardening → jail → providers)            [§1]
2. self.go — identidade estática do projeto
3. kernel.Identity() — persona runtime (primeira chamada)
4. Registro de agentes — engine.AgentRegistry.LoadDefault()
   lê .cosca/framework/agents + .cosca/agents (frontmatter YAML; corpo = SystemPrompt)
5. Montagem do system prompt POR TURNO — internal/engine/context.go::Build():
   a. agent.SystemPrompt (identidade)
   b. cadeia AGENTS.md (root → cwd)
   c. === RELEVANT MEMORIES === (com envelope contenttrust)
   d. === RELEVANT KNOWLEDGE ===
   e. === AVAILABLE SKILLS === (só nomes)
   → injetado como primeira mensagem (withSystemPromptPrepended)
6. Modelo: lido do próprio system prompt (MODEL DETECTION) — nunca hardcoded
```

Nota: `engines/identity/SKILL.md` é **DOCUMENTADO apenas** (spec de identidade/autenticação/autorização, sem implementação Go).

---

## 3. MEMÓRIA

### 3.1 Os DOIS subsistemas (verificado)

| | Embed Memory (markdown) | Runtime Memory (Go + SQLite) |
|---|---|---|
| Local | `internal/embed/cosca/memory/agent/{agente}/` + `.cosca/memory/` | `internal/memory/`, `internal/knowledge/`, `.cosca/knowledge.db` |
| Arquivos | `learnings.md`, `failures.md`, `patterns.md`, `capability-profile.md`, `evolution.md`, `INDEX.md`, `chain.dat`, `merkle/`, `blocks/`, `indexes/` | tabelas `documents/chunks/vectors/knowledge_entries/...` |
| Escrita | Agente (kernel) via `PostTaskHook` + `integrity.SignAfterLearning()` | Indexer/Compiler (`internal/indexer`, `internal/knowledge/compiler.go`), ORC (`internal/circadian`) |
| Busca | FTS5 + vetores (os .md são indexados no knowledge.db) | `kernel.Memory.Search` (FTS5 + fallback LIKE), `internal/search` híbrido |

> ⚠️ Os engines markdown `engines/{memory,semantic-memory,learning,experience-compiler,memory-curation,knowledge,org-memory,wisdom-decay,insight-generator,contradiction}` são **especificações (SKILL.md)** — NÃO há código Go nesses diretórios. Os executáveis correspondentes vivem em `internal/`.

### 3.2 Pipeline completo (verificado no código)

```
EXPERIÊNCIA (task concluída)
  ↓
EVENTO            PostTaskHook.RecordTask() — internal/pipeline/evolution.go
                  (estágios 7-8 do metacognition loop; EvolutionRecord{TaskID, Outcome,
                   Technique, Level, Learned, Pattern})
  ↓
INTERPRETAÇÃO     outcome = success(0.85) | partial(0.65) | failed(0.40);
                  técnica + level + learnings + pattern
  ↓
APRENDIZADO       appendLearning() escreve bloco markdown em agent/{nome}/learnings.md
                  (+ patterns.md, capability-profile.md quando aplicável)
  ↓
VERSIONAMENTO     integrity.SignAfterLearning() → bloco em .cosca/family_chain.dat
                  (Ed25519 se COSCA_KERNEL_PASSPHRASE presente; senão git-anchored SignAuto;
                  se ambos falham: loga, chain pega a violação no próximo boot)
                  + cosca-merkle regenera merkle/epoch_*.json (folhas = hashes do chain.dat,
                  árvore SHA-256, epochs de 32 blocos)
  ↓
INDEXAÇÃO         internal/indexer (parse→chunk→embed batch 20→vector store→FTS→graph)
                  + internal/knowledge/compiler.go → knowledge_entries (UNIQUE(source),
                  content_hash, confidence default 0.5)
  ↓
PERSISTÊNCIA      knowledge.db (WAL, 0600, busy_timeout 5s, max open 25)
                  + backups auto-*.db (VACUUM INTO, retenção 3)
                  + snapshots de memória (.cosca/snapshots/, máx 50)
                  + durable-events (.cosca/durable-events/*.jsonl, hash chain)
                  + memoryintegrity manifest (advisory, offline)
  ↓
RECUPERAÇÃO       kernel.Memory.Search (FTS5 chunks_fts + fallback LIKE)
                  + internal/search híbrido (FTS5 → vetor → grafo → re-rank, cache 5min)
                  + MemoryEngine.Search (camadas + owner/tenant/agent scoping — A6/A7)
  ↓
CONTEXTO DO LLM   context.Builder.BuildContext fase 6: top-10 memórias;
                  optimizer: memória = 10% do budget, máx 5 no contexto final
```

### 3.3 Detalhes de persistência verificados

- **knowledge.db real** (119 MB): tabelas `cache` (grafo serializado — `graph_state`), `chunks`, `chunks_fts`, `documents`, `entities`, `frontmatter`, `headings`, `knowledge_entries`, `migration_history` (v1..v5 aplicadas), `relationships`, `snapshots`, `symbols`, `sync_log`, `users`, `vectors`. **15.839 vetores de 3072 bytes = 768 dimensões float32** (nomic-embed-text).
- **knowledge_entries**: `UNIQUE(source)` + triggers FTS (`porter unicode61`). Contagens: architecture=28, best-practices=15, cognitive=3, failures=19, heuristics=22, patterns=25.
- **chain.dat do kernel**: 221 blocos no formato `block_hash|prev_hash|data|learning_id|title` (genesis = 64 zeros); merkle `index.json`: 7 epochs.
- **Backups reais**: 3 arquivos `auto-20260814-*.db`.

### 3.4 Quando uma informação vira aprendizado

Apenas no estágio **7-8** do protocolo (mandatório): a task precisa ter passado pela verificação (build/test reais, `StepRunner` marca `Success=false` se build/test falharam). A entrada é estruturada (`EvolutionRecord`) e o bloco markdown segue o formato do LEARNING_PROTOCOL (Agent/Task/Technique/Level/Outcome/Confidence/Tags/Related/Learned/Next + Wisdom Decay Category). Confidence atribuída por outcome, não auto-atribuída.

### 3.5 Deduplicação (verificada)

1. `knowledge_entries` — `UNIQUE(source)` + `content_hash` (skip se inalterado).
2. `documents.path + hash` na sync (Added/Updated/Removed).
3. ORC `dedupeDocuments` — `GROUP BY path, hash HAVING COUNT(*)>1` → remove duplicados.
4. `memory_fts` — `INSERT OR REPLACE` por id (upsert).
5. Contexto — dedupe por `doc.ID`.
6. Evidence — mesmo ID substitui.
7. Spec (memory-curation R2): dedupe por similaridade > 80% — **DOCUMENTADO, sem Go confirmado**.

### 3.6 Correção de memória antiga/falsa (verificada)

- **Envelhecimento**: `internal/knowledge/aging.go` — `RevalidateEvidence` compara SHA-256 do arquivo vs registrado → fonte mudou → `Status=STALE`. **Nunca apaga; nunca promove/demote** (só estado epistemológico).
- **Escada de evidência (CKL)** `internal/knowledge/evidence.go`: observation (≥1 evidência) → learning (≥3, conf≥0.70) → hypothesis (≥10, reprodutível) → theory (≥50, ≥5 projetos, conf≥0.90) → law (≥200, 0 rollbacks, conf≥0.99) → constitution (**só aprovação manual do Don**). `Promote()` só ascendente; `ValidateLevels()` auto-corrige regressões.
- **Estados epistêmicos** `internal/knowledge/epistemic.go`: KNOWN / SUPPORTED / UNCERTAIN / CONFLICTING / UNKNOWN / STALE.
- **Contradição** `internal/knowledge/conflict.go`: `ConflictRecord` (prefixo `CONFLICT-`, status open/resolved) em `.cosca/conflict.db`; `StatusWithConflicts()` → CONFLICTING. Engine spec F8.3 (score>0.7 bloqueia) é **DOCUMENTADO** — o Go implementa o registro.
- **Decaimento** `internal/circadian/restcycle.go`: freshness < 0.3 → movido de `learnings.md` para `auditDir/expired-entries.md` (preservado historicamente).
- **Provenance + confidence**: `Claim.Confidence ∈ [0,1]` validada no registro; confidence é **medida externa** (`SetConfidence`), não auto-calculada. A fórmula M1-M7 do CONFIDENCE_MODEL.md é **DOCUMENTADA apenas** (não está em Go).

---

## 4. RECUPERAÇÃO DE CONTEXTO

### 4.1 Os 6 tipos de contexto (diferenciados)

| Tipo | O que é | Como entra na chamada |
|------|---------|-----------------------|
| **Contexto da conversa** | Histórico de mensagens (user/assistant/tool) | `engine.Run` — `MaxConversationTurns=50`; auto-compaction a 80% da janela |
| **Memória persistente** | Retrieval de learnings/failures/patterns | `engine/context.go` — top-5, com envelope contenttrust |
| **Knowledge repository** | knowledge.db indexado | top-3 via retrieval; `context.Builder` busca chunks/symbols/entities |
| **Arquivos do workspace** | Cadeia `AGENTS.md` (root→cwd) + discovery + RAG de chunks | Concatenados no system prompt; RAG via `internal/context` |
| **Estado do runtime** | Health, capability level (L0-L3), provider ativo | Via capability engine + runtime state (não vai pro prompt; decide o que pode rodar) |
| **Contexto efetivo enviado** | O que sobra após budget | `assembleWithinTokenBudget` (default 32k tokens) + `withSystemPromptPrepended` |

### 4.2 Como NÃO despeja a memória inteira na janela

1. **Retrieval com limites baixos**: memórias limit 5 (engine) / 10 (builder) → **máx 5 no final**; knowledge limit 3.
2. **Budget greedy por tokens**: `assembleWithinTokenBudget` — adiciona docs/chunks/symbols em ordem de score até estourar 32k; marca `Truncated`.
3. **Auto-compaction em runtime** (`internal/engine/compaction.go`): gatilho em 80% da janela → (1) remove TODAS as mensagens RoleTool; (2) mantém as últimas `N = max(4, total/3)`; (3) **sumariza o resto numa system message** (`buildSummary` — hoje heurístico; LLM-summary é futuro). Preserva o system prompt inicial (identidade nunca é sumarizada). Thrashing detectado → erro.
4. **Progressive disclosure de skills**: só NOMES no prompt; conteúdo sob demanda.
5. **Fast path de sessão**: `cognitive-state.md` (~400 tokens) substitui a varredura de 421+ arquivos de memória (válido < 7 dias).

---

## 5. TIMELINE

### 5.1 Esclarecimento (HYPOTHESIS verificada parcialmente)

**Não existe um tipo `Timeline` dedicado em Go.** A "linha do tempo" de execução é materializada por: (a) o **DAG de tarefas** (planner), (b) o **log de eventos imutável** (`StepEvent{started|completed|failed}`), (c) o **durable event log** (`durable_events.go`), (d) o **checkpoint por passo**. A STATE_MACHINE.md de 14 estados é **DOCUMENTADA** (spec) — o Go usa a máquina de processo de 8 estados.

### 5.2 Fluxo real (ENFORCED)

```
OBJETIVO (request)
  ↓
PLANEJAMENTO      Planner.Plan(prompt) — DETERMINÍSTICO por template:
                  detectIntent(keywords: crud/auth/fix/refactor/test)
                  → taskTemplates (DAGs pré-definidos: create-crud-api, add-auth, ...)
                  → contextualTasks (via AgentResolver.Search)
                  → fallback: 1 task com o prompt cru
                  Plan{ID, Intent, Tasks[], EstimatedMinutes, RiskLevel, Objective}
  ↓
TIMELINE/DAG      Plan.ExecutionOrder() = ordenação topológica (Kahn);
                  HasCycle() rejeita ciclos; ValidateDependencies()
                  TaskStatus: pending | running | completed | failed | skipped
  ↓
AÇÃO              StepRunner.RunStep → RunRequest{Prompt, Agent, MaxTurns:10, Timeout:300s,
                  EnableBuild:true, EnableTest:true} → delega ao Runner
  ↓
FERRAMENTA        ver §9
  ↓
RESULTADO         TaskCompleted / TaskFailed; saveCheckpoint(plan) A CADA passo;
                  durable replay (DurableStepCompleted) torna execução idempotente
  ↓
VERIFICAÇÃO       build/test reais (verification.go); gate.go (QualityScore/AutonomyScore)
  ↓
DECISÃO           falha → RecoveryLoop.Recover → OK? → TaskCompleted; senão TaskFailed
                  → Reconciler (máx 3 ciclos) tenta convergir estado desejado vs real
  ↓
PRÓXIMA AÇÃO      ordem topológica (sequencial ou RunPlanParallel com semáforo
                  max(NumCPU*0.75, 2); RunPlanAsync futuro; RunPlanWithQueue workqueue
                  com retry rate-limited, maxRetries=3)
```

### 5.3 Erros modificam o fluxo (verificados)

- `RecoveryLoop.Recover` classifica o erro (`diagnostics.Classifier`: CatCompilation/CatDependency/CatTest/CatRuntime/...) → tentativa de **fix determinístico** (`tryDeterministicFix`, só com `AutoFix=true`) → senão **fix task via LLM** → backoff exponencial.
- Crash recovery: `tryResumeFromHistory` (replay) e `replayDurableState` (tasks `failed` voltam a `pending`).
- `internal/runtime/state.go`: `validTransitions` (8 estados) — transição inválida rejeitada; `SetRecovery` incrementa contador.

---

## 6. LLM

### 6.1 Providers reais (ENFORCED) — `internal/chat/provider/register_chat.go`

Registry com prioridade (menor = maior): `gpu=5, deepseek=10, openai=20, anthropic=30, ollama=40, none=50`.
- Cloud (deepseek/openai/anthropic): só registram com `COSCA_ENABLE_EXTERNAL_PROVIDERS=1` **e** a env var da API key presente.
- `ollama` sempre registrado (local); `gpu` via compute.Fabric (Ollama ROCm/CUDA); `none` = `NullProvider` (determinístico, sempre disponível).
- Modelos default no catálogo: `deepseek-v4-flash`, `gpt-4o`, `claude-sonnet-4-20250514`, `llama3`. A sessão atual do Don roda `deepseek/deepseek-v4-flash`. Não existe provider "OpenRouter" no catálogo.

### 6.2 Fluxo da chamada — `internal/engine/engine.go::AgentEngine.Run`

```
Runtime
  ↓
Context Builder (engine/context.go::Build) — identidade + AGENTS.md + memórias top-5
  + knowledge top-3 + skills (nomes) + tool definitions (JSON-schema) + histórico
  ↓
LLM (provider.Chat / ChatStream; retry transiente max 3; circuit breaker)
  ↓
Interpretação (ResponseParser.ParseResponse/ParseStream — merge de ToolCallDeltas;
  orchestration também extrai JSON de tools dentro do content — tryParseContentTools)
  ↓
Runtime (executor.Execute → tool results de volta ao loop; MaxTurns=10; compact a 80%)
```

### 6.3 O que o LLM RECEBE / NÃO recebe

**Recebe**: system prompt (identidade, cadeia AGENTS.md, memórias relevantes envelopadas com contenttrust, knowledge top-3, NOMES de skills), histórico + request atual, definições de ferramentas (name/description/parameters JSON-schema).
**Não recebe**: conteúdo integral das skills, memória/knowledge completos (só top-N), tool outputs antigos (removidos na compactação), credenciais/jail secrets (nunca no prompt).

### 6.4 Quem decide/valida

- **Quem prepara o contexto**: `ContextBuilder.Build` + (opcional) MAG augment (`orchestrator.go`).
- **Quem decide ferramentas disponíveis**: `executor.Executor.ListTools()` → `registry.Definitions()` é o mecanismo primário; `toolHintsForRole` deriva hints por cargo.
- **Quem valida a resposta**: `ResponseParser` (estrutura/tool calls). **NÃO há validação por JSON-schema do CONTEÚDO da resposta** — apenas conteúdo livre + tool calls; tool inputs são validados por schema no executor (`tool.Validate`).
- **Quem decide se resposta é fato**: ninguém automaticamente. Vira fato apenas via escada CKL (§3.6) com contagem de evidências + confidence; `SetConfidence` é externo.
- **Quando o LLM está errado**: `chatWithRetry` (max 3, só erros transientes — timeout/rate-limit/503/reset); `parse_failed` vira resultado final (não retenta); `llm_call_failed` é fatal; circuit breaker OPEN (5 falhas/60s) → HALF_OPEN; failover de provider (`failover_stream.go`); budget cognitivo (`BudgetTracker.CanCall` bloqueia antes de exceder).

---

## 7. AUTOQUESTIONAMENTO / VERIFICAÇÃO

### 7.1 Ciclo real no código

```
HIPÓTESE (plano/task gerada)
  → ANÁLISE (planner valida DAG: HasCycle, ValidateDependencies)
  → DÚVIDA (InteractionPolicy: confidence < 0.70 → pergunta ao Don;
            OpIrreversible → auto-block)
  → BUSCA DE EVIDÊNCIA (verificação real: build + testes + vet + git diff +
            scan de secrets + OSV vuln — Gate.Run)
  → COMPARAÇÃO (review multi-agente: cosca-security, cosca-architecture,
            cosca-testing, cosca-documentation; OverallPass = !hasCritical && Score>=5.0)
  → REJEIÇÃO/ACEITAÇÃO (gate BLOCKED vs READY TO COMMIT;
            proposal validator: contrato → lei imutável → evidência com hash →
            limite 2 revisões → semântica L2 via LLM — resposta inválida = DENY fail-closed)
  → NOVA ANÁLISE (RecoveryLoop → fix task; Reconciler até 3 ciclos)
```

### 7.2 Distinções (definidas no código real)

| Termo | Definição verificada |
|-------|----------------------|
| **Dúvida** | Estado: `InteractionDecision{ShouldAsk}`; epistemic `UNCERTAIN`; `UnavailableCapabilities` |
| **Hipótese** | `ClaimKind = "hypothesis"`; nível CKL `hypothesis` (≥10 evidências reprodutíveis) |
| **Fato** | Nível CKL `law/KNOWN` (≥200 evidências, 0 rollbacks, conf≥0.99) ou `observation`+verificação |
| **Cálculo** | `ClaimKind = "calculated"` |
| **Simulação** | `ClaimKind = "simulated"` (sciengine: Experiment com env completo) |
| **Conclusão** | Resultado de verificação com evidência anexada (proposal evidence hash) |
| **Confiança** | `float64 ∈ [0,1]`, definida externamente; thresholds: ≥0.85 autônomo / 0.70-0.84 review / 0.50-0.69 segunda opinião / <0.50 escalar (LEARNING_PROTOCOL) |

Nota: o algoritmo de resolução por confidence (CONFIDENCE_MODEL.md) é **DOCUMENTADO** — não há implementação Go; o loop metacognitivo de 8 estágios é documentado, e o código implementa os estágios 7-8 (`PostTaskHook`).

---

## 8. PROVENIÊNCIA

### 8.1 Tipos definidos em Go — `internal/provenance/provenance.go` (ENFORCED)

```go
type ClaimKind string
const (
    Observed   ClaimKind = "observed"
    Calculated ClaimKind = "calculated"
    Simulated  ClaimKind = "simulated"
    Generated  ClaimKind = "generated"
    Hypothesis ClaimKind = "hypothesis"
)
// Claim{ID, Statement, Kind, Source, Data, Method, Assumptions, Confidence, CreatedAt}
// Registry → .cosca/provenance.yaml
```

- `Generation` (§33): AssetID, Model, Prompt, Seed, Parameters, SourceAssets, Processing, EditHistory (persistido em `.cosca/provenance.yaml` — existe um registro real: `generation` com model `qwen2.5-coder:14b`, seed 0).
- `LicenseRecord` (§35): Source ∈ `cosca-code | third-party | model | user-asset`.
- `sciengine` repete os 5 kinds (`ResultKind`) com `Experiment{INPUT, PARAMETERS, CODE VERSION, MODEL VERSION, ENVIRONMENT, RESULT, METRICS}` → `.cosca/experiments.yaml`.

### 8.2 Como uma afirmação percorre o sistema

```
FONTE (arquivo/API/experimento/observação)
  → DADO (chunk/documento/registro com hash SHA-256)
  → MÉTODO (processamento registrado em Generation: model/prompt/seed/params)
  → PROCESSAMENTO (indexer/compiler/experimento)
  → RESULTADO (chunk embutido, knowledge_entry, claim)
  → AFIRMAÇÃO (Claim{Kind, Source, Data, Method, Assumptions})
  → CONFIDENCE (externo ∈ [0,1]; evidências contadas na escada CKL)
```

- **Escada de conhecimento**: `internal/knowledge/provenance.go` P0 (desconhecida) … P5 (múltiplas fontes independentes); `SourceRef{Repository, Commit, Path, SHA256, Retrieved}`; `Evidence.IsReproducible()` (P≥P4 + repo+commit+sha256).
- **No fluxo de proposta**: `HashEvidence` (sha256) + `ProposalHash` — binding autorização↔artefato exato; evidência sem hash íntegro → REVIEW ("proveniência quebrada").

---

## 9. FERRAMENTAS

### 9.1 Pipeline real

```
NECESSIDADE (LLM emite tool_call ou executor decide)
  ↓
CAPABILITY DISCOVERY   executor.ListTools() → registry.Definitions() (mecanismo primário);
                       capability.CurrentLevel(provider, runtime) → L0-L3;
                       UnavailableCapabilities() lista o que falta
  ↓
POLICY                execpolicy.Policy.Evaluate(argv) → allow|prompt|forbidden
                       (primeira regra que casa; default Allow; valida match/not_match no load)
  ↓
AUTORIZAÇÃO           allowlist de comandos: [ls cat head tail wc find grep mkdir touch
                       cp mv echo date which pwd env]; fora → erro
                       + interaction_policy (ConfirmDangerous/ConfirmSystem/BlockIrreversible)
                       + op_security (OpIrreversible auto-block)
                       + sandbox bwrap (SandboxReadOnly → --ro-bind do workspace)
  ↓
EXECUÇÃO              tool_exec.go — 7 handlers REAIS: read_file, write_file, list_files,
                       execute_command, search_codebase, read_memory;
                       path traversal bloqueado (resolveAndValidate + isPathWithin);
                       truncation: leitura 100KB, saída de comando 50KB;
                       chat/executor.ValidateToolCall (nome existe + JSON-schema + rails do sandbox)
  ↓
RESULTADO             ToolCallResult{ToolCallID, Name, Content, Error, ErrorCode, Duration}
  ↓
VERIFICAÇÃO           build/test reais (StepRunner.Success=false se falhar); Gate.Run;
                       resultados envelopados com contenttrust
  ↓
PROVENANCE            evidências com hash; erros viram safeerror (logs sem dados sensíveis)
```

### 9.2 Quando a ferramenta necessária NÃO existe

- `ToolExecutor.Execute` → `unsupported tool: %q` → **o erro VOLTA para o LLM como tool result** (não fatal): o LLM tenta outra ferramenta ou responde.
- `chat/executor.Executor.Execute` → `tool == nil` → `ToolResult{Status:"error", Error:"tool not found"}` (também não-fatal).
- Planner: task sem agente resolvido (`Agent:""`) cai no fallback de 1 task.
- Capability: `UnavailableCapabilities` avisa que a operação não pode ser executada no nível atual (ex.: sem provider de raciocínio → L0/L1).

---

## 10. AGENTES E ORQUESTRAÇÃO

### 10.1 Hierarquia (DOCUMENTADO — ORGCHART) + fluxo (ENFORCED)

```
DON (usuário — autoridade final)
  │
  ▼
KERNEL (orquestra; NUNCA implementa — AGENT_DNA forbidden actions)
  │
  ▼
CEO (decomposição dinâmica e síntese)
  │
  ▼
CTO / CHIEFS (Backend, Frontend, Security, Architecture, Testing, Docs...)
  │
  ▼
SPECIALISTS (cosca-specialist-*)
```

```
Request → Router.Route() → [1. agente explícito (Extra["agent"]) →
 2. keyword map (especialista primeiro, chief depois) → 3. full-text →
 4. fallback "CEO Agent"]
   → CEO decompoõe (ExecuteDynamic): prompt pede JSON {steps:[{name,agent,prompt,depends_on}]}
   → execução (paralela, se aplicável) → síntese dos resultados
```

- **Compartilhamento de contexto**: `HandoffArtifact` (ID, FromAgent, ToAgent, Objective, Changes, Decisions, Evidence, Files, Tests, Errors, KnowledgeUsed) → `.cosca/handoffs/HO-*.json`, hash SHA256; acionado em `terminal.go` na troca de agente.
- **Avaliação**: `ReviewPipeline` (4 revisores fixos, thresholds); `Gate.Run` (build/test/lint/secret scan/OSV); `VerificationRunner.BuildVerify/TestVerify`.
- **Conflitos**: `ConflictStore` (`.cosca/conflict.db`, `CONFLICT-*`). O algoritmo por confidence/consensus (`engines/consensus`) é **DOCUMENTADO** — sem pacote Go.
- **Autoridade final**: **o Don** — `isApprover("don"|"admin")` em `proposal/flow.go`; `gate.go` `TransitionTable` (approving→approved só don/admin; approved→executed só don/admin, approver≠executor); `permission.go` `DefaultPermissionPolicy` → "don can do anything".

---

## 11. SEGURANÇA

### 11.1 O pipeline OBSERVE→READ-ONLY→PREVIEW→APPROVAL→WRITE→VERIFY→COMMIT

**Lei constitucional P14 (CHANGE SAFETY LEVEL)** — `CONSTITUTION.md` + ADR-009/010: **DOCUMENTADO**. O enforcement visual/read-only da UI existe no projeto separado `cosca-code` (execGuard.ts, learning L307 — **fora deste repositório**). Neste repositório Go, o equivalente real de approval é implementado por outros mecanismos:

### 11.2 ENFORCED no código Go (verificado)

| Mecanismo | Código | Comportamento real |
|-----------|--------|--------------------|
| **Kernel Isolado / Proposals** | `internal/proposal/flow.go`, `validator.go`, `rules.go` | Proposta = seal+hash ANTES de julgar; 3 leis de ferro: sem APPROVE → `ErrDenied`; autorização single-use; hash ligado ao artefato (mutação pós-validação → DENY). Validador: contrato mínimo → lei imutável (fatal→DENY, guard→NeedsDon) → evidência hash → limite 2 revisões → semântica L2 (LLM; resposta inválida = DENY fail-closed). Regras R01-R06 (pkill fatal, git rewrite guard, rm -rf sensível guard...). `RequiresDon()` = destructive ou strategic. |
| **Gate de transições com papel** | `internal/gate/gate.go` | plan→approving (editor/admin/don) → approved (**só don/admin**) → executed (**só don/admin**; approver≠executor); append-only em `.cosca/gate.db`. |
| **Classificador de operações** | `internal/pipeline/op_security.go` | OpRead/OpWrite/OpModify/OpDangerous/OpSystem/OpIrreversible; `IsBlocked` = OpIrreversible (auto-block). Usado em `terminal.go` (`runHeadless`). |
| **Política de interação** | `internal/pipeline/interaction_policy.go` | ConfirmDangerous/ConfirmSystem/BlockIrreversible; low-confidence (<0.70) → pergunta. |
| **Sandbox read-only** | `internal/chat/sandbox/gate.go` | `SandboxReadOnly` → bwrap `--ro-bind` (workspace montado somente leitura); git tool = whitelist de subcomandos read-only. |
| **Jail** | `pkg/cosca/jail.go` | bwrap com `--clearenv`; binário reexecutado de cópia em RAM; disco `chmod -x`. |
| **Exec policy** | `internal/execpolicy/execpolicy.go` | allow/prompt/forbidden por prefix rule; default Allow. |
| **Secrets vault** | `internal/secrets/vault.go` | AES-256-GCM; chave via HKDF-SHA256 (fonte: COSCA_JWT_SECRET, mín. 32 bytes); arquivo 0600; valor nunca em JSON; `.cosca/secrets.db`. |
| **Auth tokens** | `internal/auth/tokenstore.go` | refresh-token com detecção de reuse → `ErrTokenReuse` revoga TODAS as sessões. |
| **Trust boundary (prompt injection)** | `internal/contenttrust/contenttrust.go` | `Envelope()` marca dados externos como não-instruções; `Suspicious()` detecta "ignore previous instructions"; `FromMetadata` exclui quarantined/blocked. |
| **Integrity/chain** | `internal/integrity/` | Ed25519 + blake3 manifest; anti-hijack (InitChain recusa overwrite); git-anchor. |
| **Quality gate** | `internal/pipeline/gate.go` | build/test/vet/diff/secret scan/OSV → BLOCKED vs READY TO COMMIT. |
| **Memory integrity** | `internal/memoryintegrity/` | manifesto SHA-256 (advisory — sem hooks de runtime; gate no boot). |

### 11.3 DOCUMENTADO apenas (sem código Go)

`engines/{policy,execution,wizard,shadow-mode,identity,compliance,audit}` (SKILL.md), `SECURITY_ARCHITECTURE.md`, `GOVERNANCE.md`. A spec de Shadow Mode (F8.2), a fórmula de consensus, e o Policy Engine centralizado com override+TTL **não têm implementação Go**.

---

## 12. APRENDIZADO

### 12.1 Pipeline verificado

```
TAREFA concluída
  → RESULTADO (build/test reais; sucesso/falha objetivo)
  → AVALIAÇÃO (outcome: success/partial/failed; confidence 0.85/0.65/0.40;
               level 1-5; técnica)
  → APRENDIZADO (PostTaskHook.RecordTask — estágios 7-8 MANDATÓRIOS e automáticos)
  → MEMÓRIA (learnings.md + failures.md (memória negativa v2.0) + patterns.md)
  → VERSIONAMENTO (SignAfterLearning → family_chain.dat + merkle epochs + INDEX)
  → DISPONIBILIDADE FUTURA (indexação FTS5 + vetores → retrieval → contexto)
```

### 12.2 Como evita aprender algo incorreto automaticamente

1. **Confidence derivada do outcome**, não auto-declarada (failed = 0.40).
2. **Failures são memória separada** (failures.md) e a conversão é `FAILURE → LEARNING → PATTERN`.
3. **Verificação real** precede o registro (build/test reais; `Success=false` impede registro de sucesso).
4. **knowledge_entries** só entram via compiler validado (UNIQUE(source) + content_hash).
5. **Escada CKL**: promoção exige contagem de evidências + confidence + reprodutibilidade; constitution só via Don.
6. **Decaimento**: entradas não revalidadas perdem freshness e são arquivadas.
7. **Contradição**: `ConflictStore` marca CONFLICTING; `ValidateLevels` auto-corrige regressões.

---

## 13. CONTINUIDADE ENTRE SESSÕES

### 13.1 Com bootstrap/despertar (sessão nova no workspace do Cosca)

- **Fase -1**: DESPERTAR (âncora de identidade — ver §1.5).
- **Fase 0.5**: `cognitive-state.md` (~400 tokens) carregado ANTES de qualquer scan; válido se < 7 dias → **skip da varredura de 421+ arquivos** (~8s → <1s).
- **Handover**: o kernel recebe o resumo do estado (arquitetura, estado git/build/tests, decisões últimas 5, pendências top-5, riscos top-3, próximas ações top-3, commits recentes) e responde **sem escanear**.
- **Integridade**: gate de memória no boot (`enforceMemoryIntegrityGate`) + chain check.
- **Sessões passadas**: `.cosca/sessions/{id}.jsonl` (meta com `parent_session_id` para linhagem) indexados por `cosca session index` (FTS5, zero-LLM, explícito, read-only). Neste repo: `.cosca/history/` guarda `PLAN-*.jsonl`/`TEST-*.jsonl` (durable events).

### 13.2 Sem bootstrap (por que chega mais devagar — sem revelar o mecanismo privado)

Tecnicamente: a sessão sem fast path precisa **reconstruir o contexto do zero** — varrer todos os arquivos de memória (centenas), re-derivar estado, arquitetura e pendências a partir de leitura bruta. Isso custa tempo (ordens de magnitude: segundos vs. fração de segundo) e tokens (milhares vs. ~400), e perde o **estado já consolidado** (decisões/pendências/riscos) que o índice rápido entrega de forma estruturada. A diferença não é "ter ou não histórico" — é ter um **índice consolidado e validado** contra ter que reconstruir o contexto por exploração. O bootstrap também detecta **staleness** (estado > 7 dias) e reconsolida quando necessário.

---

## 14. PIPELINE COMPLETO — diagrama com INPUT→PROCESSAMENTO→OUTPUT→PERSISTÊNCIA→PRÓXIMO

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  BOOT                                                                             │
│  INPUT: argv + .env + configs                      PERSISTÊNCIA: cosca.pid       │
│  PROC: hardening → jail-secrets → mem-integrity gate → providers → cobra         │
│  OUTPUT: runtime pronto (DBs abertos, chain verificada)                           │
│  PRÓXIMO: bootstrap.Compose → DESPERTAR (sessão)                                  │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  DESPERTAR (Fase -1, ritual do kernel — função: âncora de identidade/continuidade)│
│  INPUT: sessão nova sem histórico                     PERSISTÊNCIA: nenhuma       │
│  PROC: leitura do ritual + verificação de integridade do cérebro (chain/memória)  │
│  OUTPUT: kernel orientado (quem é, direção, Don reconhecido)                      │
│  PRÓXIMO: Phase 0 health check → 0.5 cognitive-state fast load                    │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  IDENTIDADE                                                                        │
│  INPUT: self.go + persona (kernel/identity.go) + registro de agentes               │
│  PROC: montagem do system prompt por turno (context.go Build)                      │
│  OUTPUT: system prompt (identidade + AGENTS.md + memória + knowledge + skills)     │
│  PERSISTÊNCIA: —  (identidade é determinística por sessão)                         │
│  PRÓXIMO: MEMÓRIA (retrieval)                                                      │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  MEMÓRIA (retrieval)                                                               │
│  INPUT: query + owner/agent scoping                       PERSISTÊNCIA: (escrita   │
│  PROC: kernel.Memory.Search (FTS5) + híbrido FTS→vetor→grafo; dedupe; top-N        │
│  OUTPUT: memórias relevantes (≤10, máx 5 no final) envelopadas com contenttrust    │
│  PRÓXIMO: KNOWLEDGE                                                                 │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  KNOWLEDGE                                                                          │
│  INPUT: query                                          PERSISTÊNCIA: knowledge.db  │
│  PROC: search híbrido (chunks_fts + vetores 768-dim + grafo) + ranker + cache 5min │
│  OUTPUT: top-3 knowledge + chunks relevantes (dentro do token budget 32k)          │
│  PRÓXIMO: CONTEXTO                                                                  │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  CONTEXTO                                                                           │
│  INPUT: identidade + memória + knowledge + AGENTS.md + histórico (≤50 turns)       │
│  PROC: assembleWithinTokenBudget; skills só nomes; compaction a 80% da janela      │
│  OUTPUT: mensagem system + histórico dentro do orçamento                           │
│  PERSISTÊNCIA: —                                                                    │
│  PRÓXIMO: TIMELINE                                                                  │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  TIMELINE (DAG)                                                                      │
│  INPUT: request                                     PERSISTÊNCIA: checkpoint a cada │
│  PROC: Planner.Plan (intent→templates→topo sort) → StepRunner → workers             │
│  OUTPUT: ordem de execução (Kahn) + plan validado (acíclico)        passo + eventos │
│  PRÓXIMO: LLM (por step)                                                             │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  LLM                                                                                 │
│  INPUT: system prompt + histórico + tool definitions                               │
│  PROC: provider.Chat (retry transiente max 3, circuit breaker, failover)           │
│  OUTPUT: resposta + tool_calls (parse/merge)                                        │
│  PERSISTÊNCIA: —   (chamada registrada em trace/audit quando habilitado)            │
│  PRÓXIMO: FERRAMENTAS (se tool_call) ou VERIFICAÇÃO                                  │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  FERRAMENTAS                                                                         │
│  INPUT: tool_call                                     PERSISTÊNCIA: evidence/*.json│
│  PROC: ListTools → execpolicy → allowlist → sandbox → tool_exec (7 handlers)        │
│  OUTPUT: ToolCallResult (erro também volta ao LLM, não-fatal)                        │
│  PRÓXIMO: LLM (loop, até MaxTurns) ou EXECUÇÃO                                        │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  EXECUÇÃO (step)                                                                    │
│  INPUT: RunRequest{Prompt, Agent, MaxTurns, Build/Test}                            │
│  PROC: runner → agenta roda a task (read-only se ordem do Don)                     │
│  OUTPUT: task resultado + artifacts                                                 │
│  PERSISTÊNCIA: durable-events (hash chain) + WorkflowHistory                        │
│  PRÓXIMO: VERIFICAÇÃO                                                                │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  VERIFICAÇÃO                                                                         │
│  INPUT: resultado do step                           PERSISTÊNCIA: gate.db/audit.db │
│  PROC: build/test reais → gate (QualityScore) → review (4 revisores)               │
│        falha → RecoveryLoop (fix determinístico → fix LLM → retry)                  │
│  OUTPUT: VERDICT (BLOCKED / READY TO COMMIT) / task completed|failed                │
│  PRÓXIMO: PROVENIÊNCIA                                                                │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  PROVENIÊNCIA                                                                         │
│  INPUT: claim/evidência                              PERSISTÊNCIA: provenance.yaml │
│  PROC: ClaimKind (observed|calculated|simulated|generated|hypothesis) + confidence  │
│        + P0-P5 + escada CKL (promoção só ascendente)                                │
│  OUTPUT: afirmação com origem e confidence                                          │
│  PRÓXIMO: APRENDIZADO                                                                 │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  APRENDIZADO (estágios 7-8)                                                          │
│  INPUT: EvolutionRecord (outcome/técnica/level/learned)                            │
│  PROC: PostTaskHook.RecordTask → appendLearning (learnings.md + failures.md)        │
│  OUTPUT: bloco de aprendizado formatado                                             │
│  PRÓXIMO: MEMÓRIA PERSISTENTE                                                         │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  MEMÓRIA PERSISTENTE                                                                  │
│  INPUT: bloco markdown + assinatura              PERSISTÊNCIA: family_chain.dat     │
│  PROC: SignAfterLearning (Ed25519/git-anchor) + merkle epochs + indexação FTS/vec    │
│        + backups + snapshots                                                         │
│  OUTPUT: memória versionada e indexada                                               │
│  PRÓXIMO: PRÓXIMA SESSÃO                                                              │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                                ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│  PRÓXIMA SESSÃO                                                                       │
│  INPUT: estado consolidado (cognitive-state.md ~400 tokens, chain íntegra,          │
│         knowledge.db + vetores + grafo, sessões indexadas em session.db)            │
│  PROC: boot → gate de integridade → DESPERTAR → fast load → handover ao kernel      │
│  OUTPUT: kernel operacional em <1s (vs. ~8s sem fast path)                           │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## 15. ESSENCIAL vs IMPLEMENTAÇÃO

### ESSENCIAL — sem ele o comportamento observado muda significativamente

| Componente | Por quê |
|------------|---------|
| Identidade + persona (self.go, kernel/identity.go) | Define quem é o Cosca e o vínculo com o Don |
| Montagem de prompt por turno (context.go Build) | Todo o comportamento conversacional depende dela |
| Pipeline de memória + chain de integridade | Continuidade entre sessões e anti-adulteração |
| Knowledge engine + busca híbrida (FTS+vetor+grafo) | Capacidade de responder com conhecimento real |
| Router + orquestração (keyword/chain CEO) | Delegação e hierarquia de agentes |
| Approval fail-closed (proposal + gate + interaction policy) | A autoridade final do Don é ENFORCED aqui |
| Provenance Claims + escada CKL | Diferencia fato/hipótese/fabricação |
| LLM provider abstraction + retry/circuit breaker | Interface com o modelo; resiliência |
| PostTaskHook (estágios 7-8) | A auto-evolução obrigatória |
| Secrets vault + contenttrust + jail | Proteção de credenciais e prompt injection |

### SUPORTE — importante para robustez, mas substituível

- Backups/snapshots (VACUUM INTO, retention 3), durable events, memoryintegrity manifest (advisory), circuit breaker, failover stream, handoffs, review pipeline (4 revisores), conflict registry, wisdom decay / ORC, cognitive budget, scheduler/cron, compute fabric GPU.

### IMPLEMENTAÇÃO — pode ser substituída sem mudar o princípio

- Bancos específicos (SQLite WAL; poderia ser outro storage), provider concreto (ollama/deepseek/openai — a abstração é que importa), tools registry (7 handlers), session FTS index, `cognitive-state.md` como formato do fast path, nomenclatura dos arquivos de memória, models default, política `don can do anything` (valor, não forma).

### CONFIDENCIAL — descrito apenas função/interface

- **Conteúdo do DESPERTAR** (instruções privadas de ativação) — função e momento descritos em §1.5.
- **Chaves e credenciais**: jail-secrets, kernel private key, passphrase mechanism, HKDF sources — apenas o mecanismo é descrito; nunca o material.
- **Secrets vault** e **auth tokens**: mecânica criptográfica descrita; conteúdo jamais.

---

## Apêndice A — Verificação e fontes (caminhos reais)

```
cmd/cosca/main.go                       — boot do binário
internal/bootstrap/bootstrap.go         — Compose de engines
internal/kernel/identity.go             — persona + leis + princípios
internal/engine/context.go              — montagem do system prompt
internal/engine/engine.go               — loop do agente (Run/RunStream)
internal/engine/compaction.go           — auto-compaction 80%
internal/pipeline/planner.go            — Planner determinístico + DAG
internal/pipeline/steprunner.go         — execução topológica/paralela
internal/pipeline/recovery.go           — RecoveryLoop
internal/pipeline/evolution.go          — PostTaskHook (estágios 7-8)
internal/pipeline/gate.go               — quality gate
internal/pipeline/review_pipeline.go    — review multi-agente
internal/pipeline/handoff.go            — handoffs entre agentes
internal/orchestration/router.go        — roteamento por keyword
internal/orchestration/chain.go         — decomposição CEO (ExecuteDynamic)
internal/orchestration/tool_exec.go     — execução de ferramentas (7 handlers)
internal/chat/provider/register_chat.go — providers/prioridades
internal/chat/sandbox/gate.go           — sandbox read-only (bwrap)
internal/execpolicy/execpolicy.go       — policy allow/prompt/forbidden
internal/capability/capability.go       — níveis L0-L3
internal/context/context.go             — builder RAG (7 fases)
internal/context/builder.go             — selectRelevantMemories
internal/context/optimizer.go           — budget de memória (10%, máx 5)
internal/knowledge/knowledge.go         — knowledge engine + graph
internal/knowledge/evidence.go          — escada CKL
internal/knowledge/aging.go             — STALE por hash
internal/knowledge/conflict.go          — ConflictStore (.cosca/conflict.db)
internal/knowledge/epistemic.go         — estados epistêmicos
internal/knowledge/compiler.go          — knowledge_entries (UNIQUE+hash)
internal/knowledge/provenance.go        — P0-P5, SourceRef
internal/memory/memory.go               — memory engine (camadas)
internal/memory/layers.go               — temp/session/project/workspace/global
internal/integrity/sign.go              — Ed25519 family_chain.dat
internal/integrity/git_anchor.go        — SignAuto git-anchored
internal/integrity/integrity.go         — Check/VerifyOrFatal
internal/provenance/provenance.go       — ClaimKind + Registry
internal/sciengine/sciengine.go         — ResultKind + Experiment
internal/proposal/flow.go               — Kernel Isolado (fail-closed)
internal/gate/gate.go                   — TransitionTable (papel)
internal/pipeline/op_security.go        — OpLevel + SecurityGate
internal/pipeline/interaction_policy.go — InteractionPolicy
internal/secrets/vault.go               — AES-256-GCM vault
internal/auth/tokenstore.go             — refresh-token reuse → revoke
internal/contenttrust/contenttrust.go   — envelope untrusted data
internal/audit/{audit,decision}.go      — audit.db, decision_log
internal/trace/store.go                 — trace.db (flight recorder)
internal/ledger/ledger.go               — WAL hash-chainado
internal/pipeline/durable_events.go     — durable-events hash chain
internal/sessionindex/sessionindex.go   — índice FTS5 de sessões
internal/circadian/restcycle.go         — ORC: dedupe + wisdom decay
internal/scheduler/scheduler.go         — ParseNL/cron
internal/search/search.go               — busca híbrida FTS+vetor+grafo
internal/vector/sqlite_vec.go           — SQLiteVec 768-dim, cosine
internal/embeddings/provider.go         — ProviderRegistry
internal/memoryintegrity/integrity.go   — manifesto SHA-256 (advisory)
internal/embed/cosca/DESPERTAR.md       — ritual (CONFIDENCIAL no conteúdo)
internal/embed/cosca/bootstrap/BOOTSTRAP.md — fases -1..10 de sessão
internal/embed/cosca/memory/LEARNING_PROTOCOL.md — v3.1.0
internal/embed/cosca/runtime/STATE_MACHINE.md    — spec DFA 14 estados (DOCUMENTADO)
internal/embed/cosca/runtime/EXECUTION_GRAPH.md  — spec DAG (DOCUMENTADO)
internal/embed/cosca/runtime/ARCHITECTURE.md     — spec pipeline (DOCUMENTADO)
```

## Apêndice B — Rótulos usados

| Rótulo | Significado |
|--------|-------------|
| **ENFORCED** | Verificado em código Go executável |
| **DOCUMENTADO** | Verificado apenas em spec markdown embutida (sem implementação Go) |
| **UNKNOWN** | Não verificável no escopo da investigação |
| **HYPOTHESIS** | Inferência razoável sem confirmação completa |
| **CONFIDENCIAL** | Mecanismo descrito apenas por função/interface (ordem do Don) |
