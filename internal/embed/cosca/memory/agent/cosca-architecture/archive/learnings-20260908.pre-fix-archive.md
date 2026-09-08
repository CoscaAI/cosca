# cosca-architecture - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-architecture — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | Initial capability establishment |
| **Technique** | Standard architecture patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #architecture #baseline #initialization |
| **Related** | See internal/embed/cosca/memory/codebase/overview.md, internal/embed/cosca/memory/pattern/ |
| **Learned** | Project established. Core architecture patterns documented. Ready for level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — gRPC Server Planning
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture + cosca-backend |
| **Task** | Architectural planning for gRPC server (3 services, 12 RPCs) |
| **Technique** | Full-stack analysis: proto files → engine interfaces → handler patterns → startup lifecycle. DRY via engine-sharing. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #grpc #architecture #planning #adr #protobuf #server |
| **Related** | `internal/embed/cosca/memory/architecture/grpc-server-plan.md`, `api/rest/server.go`, `internal/runtime/runtime.go`, `internal/cli/serve.go` |
| **Learned** | 1) Proto files at `proto/aos/v1/` with go_package `api/grpc/pb` — mismatch requires move. 2) REST handlers call engines directly (no service layer) — gRPC follows same pattern. 3) `memory.MemoryEngine.Delete()` exists and is functional (not placeholder). 4) Start gRPC in `internal/cli/serve.go` alongside REST/metrics servers with shared engine instances. 5) 12 RPCs confirmed (4 Knowledge + 6 Memory + 2 Runtime), not 15 as initially stated. 6) gRPC deps already in go.mod (grpc v1.64.0, protobuf v1.33.0). 7) All RPCs are unary — no streaming needed. 8) Server reflection should be opt-in flag for production safety. 9) Graceful shutdown: gRPC first (GracefulStop), then REST, then engines Close. 10) Mapping layer (pb ↔ domain) in separate files keeps service implementations clean. |
| **Next** | Execute Phase 0: move .pb.go to api/grpc/pb/, verify compilation. Then Phase 1: RuntimeService as template. |

### 2026-07-28 — Streaming Architecture Planning
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture + cosca-backend |
| **Task** | Discovery & planning for WebSocket/SSE streaming in Cosca serve.go |
| **Technique** | Full-stack discovery: go.mod deps → grep for stream/websocket/sse → chat/types.go interface → handler/run.go SSE impl → provider ChatStream → knowledge Sync → workflow Run → frontend hooks. Decision matrix SSE vs WS. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #streaming #sse #websocket #architecture #planning #adr |
| **Related** | `internal/embed/cosca/memory/architecture/streaming-plan.md`, `api/rest/handler/run.go`, `internal/chat/types.go`, `api/middleware/security.go`, `internal/knowledge/knowledge.go`, `internal/workflows/workflows.go` |
| **Learned** | 1) SSE already mature for chat — `POST /v1/run/stream` uses `sendSSE()` with `text/event-stream` + `http.Flusher`. 2) Zero WebSocket code or deps — no gorilla, nhooyr, or gobwas. 3) All 10+ providers implement `ChatStream` via `chat.ChatStream` interface. 4) Knowledge `Sync()` and Workflow `Run()` are fully sync — no progress callbacks exist yet. 5) CSP already has `ws://` in connect-src (line 46 of security.go). 6) Frontend `use-provider-stream.ts` is a placeholder waiting for backend wiring. 7) `sendSSE()` is trapped in handler/run.go — needs extraction to shared package. 8) Auth middleware uses JWT via cookie/Bearer header — WebSocket needs query param pattern. 9) Recommended lib: `nhooyr.io/websocket` (context-aware, Go std interfaces, pure Go). 10) 4 SSE + 1 WS endpoints planned. Total effort 40-55h across 5 phases. |
| **Next** | Phase 0: Extract SSE utilities to `api/stream/sse.go`. Phase 1: Add progress callbacks to knowledge Sync. |

### 2026-08-23 — Mega Brain: deliberação evidência-gated + plan-only
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | ADR-011 — adotar padrões-ouro (A1-A7, D1-D6) do Mega Brain no Cosca, sem implementar |
| **Technique** | Mapa gap: o Cosca TEM RAG/critic/gates/DAG/Ciclo-de-Decisão, mas é 1-agente auto-avaliação (passo 8), confiança intuitiva, sem convergência calculada, sem plan-only nem gate 3-estados. Solução: `internal/deliberate` determinístico (Convergence/Confidence/Emit/VoteCross/ValidateSpec) para P0/P1 + Plan-only no `Engine` reusando `internal/gate` e `internal/workflow`. Régua P9: modelo propõe, sistema decide. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #mega-brain #deliberation #plan-only #adr #convergence #confidence #cpu #over-engineering |
| **Related** | `docs/adr/ADR-011-mega-brain-deliberation-and-plan-only.md`, `.cosca/fallback/knowledge/patterns/mega-brain-patterns.md`, `internal/orchestration/orchestrator.go`, `internal/orchestration/router.go`, `internal/gate/gate.go`, `internal/workflow/workflow.go`, `internal/confidence/tracker.go`, `internal/embed/cosca/{CONSTITUTION,QUALITY_GATES,DECISION_PROTOCOL,CONFIDENCE_MODEL}.md` |
| **Learned** | 1) Não existe `internal/review` (cosca-review/qa são agentes-texto) — deliberação deve viver num pacote determinístico `internal/deliberate`. 2) `Engine.Execute` roda inline (sem plan-only); a guarda já está em `internal/gate.GateStore` (plan→approving→approved→executed) — estender p/ 3-estados, não recriar. 3) `internal/workflow` já é DAG typed-routing com JoinNode paralelo + ErrCycle — usar como substrato, só somar CPM/FMEA. 4) Reuso é a régua: NÃO tocar `internal/embed/cosca/*` (P8). 5) Over-engineering honesto: batalha adversarial completa (A5), arquétipos D1-D10 (A6), D7 token-fencing, e todo o bloco C (RAG) ficam de fora — RAG merece ADR próprio. 6) Fatia 1 = plan-only + gate 3-estados (bloco 2) e `internal/deliberate` só P0/P1 (bloco 1). |
| **Next** | Se aprovado: fatiar 1 dos 2 blocos; considerar ADR-012 para RAG "zero achismo" (C2/C4/C5/C6). |

### 2026-08-24 — ADR-012: Arquitetura de 2 Zonas (Cofre air-gap + Kernel internet)
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | ADR-012 — formalizar o desenho de 2 zonas (Cofre=IA local+Oracle puro sem internet; Kernel=internet) que só existia fragmentado no cognitive-state.md |
| **Technique** | Mapa gap honesto: o Oráculo JÁ existe e está conectado (internal/oracle oracle.go/search.go + oracleGate no api/rest/handler/run.go, REJECT→HTTP400); o data-vault é AES-256-GCM (internal/secrets/vault.go); o jail é fail-closed (pkg/cosca/jail.go) mas Linux-only. Gap real = air-gap de rede, IA local compreendendo (nunca decidindo), e envelope íntegro. Solução: definir zonas + princípio de rede + fluxo Kernel→Oráculo→DECISÃO com a regra "o determinístico decide, a IA compreende" — a saída da IA local é INPUT, nunca VEREDITO (proibido REJECT→ACCEPT). |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #2-zonas #cofre #kernel #air-gap #oracle #adr #seguranca #wsl2 #bwrap #fail-closed |
| **Related** | `docs/adr/ADR-012-2-zone-cofre-kernel.md`, `internal/embed/cosca/memory/context/cognitive-state.md`, `internal/embed/cosca/{ORACLE_PROTOCOL,MODEL_PROTOCOL,SECURITY_PROTOCOL}.md`, `internal/oracle/{oracle,search}.go`, `internal/secrets/vault.go`, `pkg/cosca/{jail.go,jail_windows.go}`, `deploy/README-WINDOWS.md` |
| **Learned** | 1) **Desambiguação crítica**: há DOIS "Cofre" — SECURITY_PROTOCOL (data-vault: knowledge.db/secrets.db/chain) vs a ZONA Cofre (runtime isolado que CONTAINS o data-vault + IA local + Oracle). O ADR deve explicitar isso ou o security chief confunde os dois. 2) O Oráculo de fronteira (internal/oracle) é CONCEITUALMENTE distinto do Oráculo de medição (internal/evals/ORACLE_SPEC.md — secret_verify caixa-preta) — não misturar. 3) O `oracleGate` no RunHandler JÁ é o ingress point da fronteira; o ADR só formaliza. 4) **Air-gap no Windows/WSL2 é best-effort**: WSL2 default NAT tem internet; sem derrubar rota default + resolv.conf, o "sem internet" é meia-verdade. Bwrap é Linux-only e não está instalado. 5) Regra de ouro do loop: determinístico decide; IA local só compreende (Comprehend devolve insights, nunca Decision) — senão vira "jaula de LLM". 6) Número do ADR: há DOIS ADR-011 (duplicata), próximo é 012. 7) Não existe INDEX de ADRs (só o INDEX.md da memória de contexto) — nada a atualizar. |
| **Next** | Quando WSL2-distro+bwrap estiverem de pé (ou AppContainer/Low-IL nativo validado), implementar a fatia 1: `internal/oracle/ai_adapter.go` (Comprehend sem Decision), envelope assinado Kernel→Oráculo, `cosca cofre netcheck --strict` (falha se houver egresso). |

### 2026-08-24 — ADR-013: Bancos de Dados Modulares (Core imutável como âncora + módulos coesos + índice agregador)
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | ADR-013 — formalizar a visão do Don de bancos modulares: memória da família imutável como base de comparação, bancos por módulo idempotentes, índice agregador vetorial para busca semântica unificada |
| **Technique** | Mapa gap HONESTO + inspeção real do banco (somente leitura via Python sqlite3, sem tocar o arquivo) + tese do Don ("ler cada módulo = entender de uma vez"). Fatos medidos: `knowledge.db`=257,6MB, 38 tabelas, documents 2383/chunks 38742/entities 36539/relationships 32535/vectors 28888/users 1, `user_version=0` apesar de 6 migrações em `migration_history`. Solução: Core=NÃO é SQLite reescritível (chain+blocks+embed já são arquivos append-only assinados); projects/graph/vector/core split; `internal/modlink` (Ref por ID+hash, resolução app-layer); `internal/vectoragg` (read-model via ATTACH read-only). |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #bancos-modulares #modulos #imutavel #ancora #oracle #indice-agregador #adr #sqlite #busca-semantica #tese #apreensao-holistica |
| **Related** | `docs/adr/ADR-013-modular-knowledge-databases.md`, `docs/adr/ADR-012-2-zone-cofre-kernel.md`, `docs/adr/ADR-002-knowledge-engine.md`, `internal/sqlite/{schema,migrations}.go`, `internal/integrity/*` (chain), `internal/oracle/{oracle,search}.go`, `internal/memory/store.go`, `.cosca/knowledge.db` (estado atual) |
| **Learned** | 1) **Desambiguação do mapa**: o "banco único" é verdade SÓ para o conhecimento — o runtime JÁ é multi-banco (audit/auth_tokens/department/memory-index/secrets/trace são .db separados). O outlier é o `knowledge.db` (257MB). 2) **A memória imutável da família NÃO está em SQLite** — family_chain.dat + blocks/*.md + embed já são append-only/read-only assinados; forçá-los para um SQLite reescritível DESFAZERIA a imutabilidade. 3) **Honestidade técnica obrigatória**: SQLite NÃO tem trigger nem FK cross-database; os links são na camada de aplicação (Ref{Module,ID,Hash} + Resolve + detecção de Drift). `ATTACH DATABASE` read-only serve à LEITURA agregada (JOIN cross-db), mas NÃO é link semântico nem constraint. 4) **Agregador é read-model derivado** (reconstruível, nunca fonte primária), para centralizar o espaço vetorial comparável — sem ele, buscar em N índices vetoriais fragmenta e reintroduz ruído. 5) Migração de 257MB é a etapa mais perigosa: fazer offline com backup, split de leitura (espelho via ATTACH) antes de qualquer escrita, jamais com serve rodando. 6) Número do ADR: dado que há dois ADR-011, o próximo é 013 — confirmado que ADR-013 não existia. |
| **Next** | Se aprovado: fatia 1 = `internal/modlink` (Ref+Resolve+Drift) + `cosca index rebuild --verify` (agregador idempotente) + `core.db` query_only. Fatia 2+ = migração destrutiva dos 257MB com backup e validação, e NUNCA tocar a política de versionamento do git sem aprovação do Don (porque hoje é snapshot consistente versionado por ordem expressa). |

### 2026-08-27 — ADR-015: F1 — elevar as primitivas neutras de control plane do cosca-trader ao Root
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | F1 da elevação — copiar/adaptar `internal/{task,orchestrator,decision}` do cosca-trader para o Cosca Root como infra neutra (stdlib only), frente `Domain Adapter → Generic Control Plane`, sem quebrar o Root |
| **Technique** | Mapa de reuso honesto ANTES de criar: ler `internal/pipeline`, `internal/durable`, `internal/orchestration`, `internal/aitask`, `internal/audit`. Invariante verificada na prática com `go list -deps`. Reuso real DELEGADO à borda (adapter concreto), não à primitiva. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #f1-elevacao #task-continuation-loop #primitivas-neutras #stdlib-only #adr-015 #reuso #boundary #adapter #go-list-deps |
| **Related** | `docs/adr/ADR-015-task-continuation-loop.md`, `internal/task/{task,events,repository}.go`, `internal/orchestrator/orchestrator.go`, `internal/decision/decision.go`, campo de prova `C:\Users\Henrique\Documents\projects\cosca-trader\internal\{task,orchestrator,decision}` |
| **Learned** | 1) **Não colidem**: `internal/task` (livre), `internal/orchestrator` (livre — `orchestration` é outro), `internal/decision` (livre — `audit.DecisionStore` é OUTRO conceito: explicabilidade SQLite D-xxxx vs memória event-sourced latest-winner). Criei pacotes novos, sem duplicar conceito. 2) **Não reusar durable/pipeline DENTRO da primitiva**: `durable` importa `internal/sqlite` → `modernc.org/sqlite` (dep externa) que derrubaria a invariante `go list -deps = stdlib only`; `pipeline` é neutro mas não puro (Plan/TaskNode/TaskResult). Reuso delegado à borda (TaskRepository/DecisionStore concretos, F2/F3). 3) **Trader x Root — o que quebrava a pureza**: `orchestrator` importava `github.com/google/uuid` (troquei por `crypto/rand`→hex + fallback timestamp+counter) e `internal/event` (não existe no Root). Movi o envelope mínimo `task.Event` + constantes `task.EventXxx`/`task.SeverityXxx` para DENTRO de `internal/task`; NÃO criei `internal/event` de propósito (o adapter mapeia p/ o transporte real). Renomeei env `COSCA_TRADER_TASK_MAX_CONTINUE`→`COSCA_TASK_MAX_CONTINUE`. 4) **Invariante confirmada**: `go list -deps` = só stdlib + `internal/task` (p/ orchestrator); 0 domínio, 0 externa. 5) **`go build ./...` verde; testes das 3 primitivas verdes** (adaptados do campo de prova: task, orchestrator, orchestrator_f2, decision). 6) **Nag**: `internal/integrity` FALHA por razão AMBIENTAL (Ed25519 machine-bound, "cannot load private key (only the kernel on this machine can sign)" → IDENTIDADE-BREACH). Provei que é pré-existente: movendo os 3 pacotes para fora da árvore, o `internal/integrity` segue falhando. NÃO é causada pela F1 (packages isolados, zero relação). |
| **Next** | F2: implementações concretas de `TaskRepository`/`DecisionStore` reusando `internal/durable`/`pipeline` (borda) + ligar `MaxContinue` ao Don-escalate real. F3: compor `TaskOrchestrator` no Kernel (workers/edições/provedores) + transporte real de eventos. F4: `WithObjectiveSatisfied` real + integração `internal/deliberate`/`internal/gate`. F5: observabilidade do loop + cross-trace com `internal/audit`. |


> [!NOTE - Decisao do Don 2026-09-08] Arquivo reduzido para conter custo de tokens. Historico em archive\learnings-20260908.archive.md (grep, nunca leitura integral).

> As entradas recentes abaixo sao as que estavam no topo; historico mais antigo foi arquivado.

