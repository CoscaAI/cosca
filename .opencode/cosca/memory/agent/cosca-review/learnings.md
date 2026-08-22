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
