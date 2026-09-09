# cosca-review - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-review — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Code Review Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | PR code review |
| **Technique** | 6-gate review: Security, Correctness, Architecture, Performance, Style, Testing |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #code-review #pr #gates #checklist |
| **Related** | SOLID, Clean Architecture, Go idioms |
| **Learned** | Go-specific: check for defer usage, context propagation, goroutine leaks. TypeScript: check React state management, accessibility. |
| **Next** | Level 2: Add automated lint integration (golangci-lint output parsing) |

## Experience Learnings

### 2026-07-28 — Onda 2 Review: CI Pipeline + Integration Tests
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | Revisão de CI pipeline e testes de integração (Onda 2) |
| **Technique** | Cross-reference com quality-gates.md; execução de testes com -race; verificação de flakiness via loop; contagem de transições vs validação de cobertura |
| **Level** | 2 |
| **Outcome** | approved_with_conditions |
| **Tags** | #ci-pipeline #integration-tests #quality-gates #bug-reproduction #flaky-tests #race-conditions |
| **Related** | SOLID, Go -race detector, GitHub Actions, AAA pattern, state machines |
| **Learned** | 1) CI pipelines devem ser testados quanto ao enforcement real dos gates — `continue-on-error: true` torna um gate inútil. 2) Testes de bug que usam t.Logf em vez de t.Errorf permitem CI verde enquanto rastreiam bugs — padrão inteligente. 3) Flakiness detection: loop de 10+ execuções é necessário; 1 falha já confirma. 4) Coverage total pode ser arrastada para baixo por pacotes com 0% coverage (no test files). 5) GitHub Actions: `secrets: inherit` em reusable workflows passa privilégios desnecessários. |
| **Confidence** | 0.45 — primary domain (code review) |
| **Next** | Level 3: Integrar gosec + govulncheck outputs no review automatizado. Adicionar diff-coverage check para código alterado. |

### 2026-07-28 — Pattern: Fire-and-Forget Event Before State Readiness
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | Pattern discovery during Onda 2 review |
| **Technique** | Root cause analysis of EventStartupComplete premature emission |
| **Level** | 2 |
| **Outcome** | discovered |
| **Tags** | #antipattern #event-timing #lifecycle #state-machine |
| **Related** | BUG-U02, EventStartupComplete, runtime.go:333 |
| **Learned** | Emitir eventos de "completion" ANTES do passo de lifecycle correspondente ser executado é um antipattern crítico. A ordem correta é: executar o lifecycle step → fazer a transição de estado → emitir o evento. A ordem invertida faz com que subscribers operem com estado inconsistente (nil subsystems, estado de init quando deveria ser running). |
| **Next** | Audit de todos os eventos do runtime para verificar timing correto. |

### 2026-07-28 — Pattern: State Machine Definition Without Contract Enforcement
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | Pattern discovery during Onda 2 review |
| **Technique** | Root cause analysis of Restart() failure |
| **Level** | 2 |
| **Outcome** | discovered |
| **Tags** | #antipattern #state-machine #transition-gap #contract-violation |
| **Related** | BUG-U01, Restart(), validTransitions, state.go:106 |
| **Learned** | Definir um mapa de transições válidas (validTransitions) não garante que o código de produção as execute. A transição Stopped→Uninitialized existe no mapa (state.go:106) mas Restart() chama Stop()→Start() sem nunca executar essa transição. O state machine é "declarative" (mapa de regras) mas o uso é "imperative" (chamadas manuais) — o gap entre definição e enforcement é onde bugs nascem. Solução: wrapper methods que encapsulam sequências de transições (ex: `RestartSequence()`) em vez de deixar cada caller compor as transições manualmente. |
| **Next** | Propor TransitionSequence pattern como ADR. |

### 2026-08-22 — Review: DPAPI machine-bound key (Opção B)
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | Revisão crítica de arquitetura+segurança do mecanismo de assinatura machine-bound via DPAPI |
| **Technique** | Gate por severidade (CRÍTICO/ALTO/MÉDIO/BAIXO) + verificação empírica (go build/vet/test -race) + triangulação de ameaça (agente=mesmo usuário/máquina satisfaz DPAPI) + cross-ref dos "Next" em learnings de outras squads |
| **Level** | 3 |
| **Outcome** | approved_with_conditions |
| **Tags** | #dpapi #ed25519 #machine-bound #identity-gate #crypto #code-review #threat-model #ptty |
| **Related** | #kernel-key #M1 #M3 #M4 #M5 #M6 #M7 #passphrase-removal #opte-B |
| **Learned** | 1) DPAPI correct: go vet+build limpos confirmam `unsafe.Slice(Data, Size)` com `uint32` (IntegerType), `LocalFree` é o free correto e o bloco é copiado ANTES do defer (sem UAF); escopo CurrentUser + UI_FORBIDDEN corretos. 2) A MÉTRICA crítica: o fator "máquina" (DPAPI) é satisfeito por QUALQUER processo do mesmo usuário na mesma máquina — logo NÃO distingue Don de subagente; quem distingue é só o TTY. Descobri que `--rekey`/`--init` NÃO gateiam o Don (só máquina) → rotação de identidade sem consentimento = ALTO (é o achado nº 1). 3) M3 bloqueia pipes (IsTerminal=false) mas NÃO ptys (script/expect) — a fraqueza do consent-to-content. 4) TOCTOU consentChallenge→Sign (double scan). 5) docs embed (CLI_PROTOCOL/MANUAL_DO_DON/DON_PROTOCOL) e comment em memory_register.go:64-65 continuam citando `--passphrase-stdin`/`COSCA_KERNEL_PASSPHRASE`/3-fatores. 6) fallback dpapi_other é AES-GCM com KDF de 1000-iter SHA-256 + machine-id público → binding real é o 0600/local, não cripto. 7) `git -C root push` sem scope + `opendev_user` hardcoded no askpass. |
| **Next** | (1) Verificar se o Don aprovou gatear `--rekey`/`--init`; (2) medir se 8-hex é suficiente ou precisa 16; (3) check para ver se subagente consegue exec `cosca-check` dentro da jaula (é o que fecha o ALTO). |

### 2026-08-29 — Review: Camada 2 "Exploração Cirúrgica" do cérebro 3D (brainweb)
| Field | Value |
|-------|-------|
| **Agent** | cosca-review |
| **Task** | Review de segurança da "Camada 2 — Exploração Cirúrgica" (pick por clique → painel lateral) em internal/brainweb/web/app.js + style.css |
| **Technique** | Gate 0 (SCM truth): ANTES de revisar um diff, PROVAR que ele existe no repo. `git status --short`, `git diff --stat (worktree)`, `git diff -- <file>`, `git log -- <dir>`, `git grep` e inspeção cruzada de index.html para achar o container do painel. Só então revisar o código. |
| **Level** | 3 |
| **Outcome** | inconclusive — obra ausente do repo |
| **Tags** | #brainweb #threejs #picking #diff-verification #gate0 #scm-truth #review-blocker #read-only #go-embed |
| **Related** | #security-checklist #esc() #xss #read-only #go-embed #WebFS |
| **Learned** | 1) A primeira verificação de QUALQUER review deve ser "o diff a revisar existe?", nunca revisar direto o arquivo. Aqui a premissa do Don dizia que o Frontend acabara de implementar a Camada 2, mas o worktree só tinha `.cosca/trace.db` modificado; o último commit que tocou brainweb (e446ec8) alterou só handler.go (rota /brain/perception); `git grep` não achou picking/painel; index.html não tem container de painel lateral; app.js não tem handler de clique nem `raycaster.setFromCamera` para agentes. 2) `go build ./...` (EXIT=0) é parte do gate, mas build verde NÃO prova que a feature existe (stack estática go:embed + MCP/brainweb). 3) Veredito seguro quando o artefato está ausente: NÃO APROVADO (não se aprova o que não está no repositório) — nunca inventar achados de segurança sobre código inexistente. 4) Scope-check limpo via worktree diff não implica a feature presente — o contrário (escopo limpo aponta que NADA foi entregue). |
| **Confidence** | 0.7 — primary domain (code review), gate-0 eleva a robustez |
| **Next** | (1) Ao receber review de feature, rodar `git status --porcelain`, `git diff --stat` e `git log -5 -- <dir>` ANTES de abrir o arquivo; (2) ampliar gate-0 para comparar com base branch (origin/main) quando o trabalho vier via branch; (3) usar `git log -p --follow -- <file>` para rastrear onde a feature entrou de fato. |

