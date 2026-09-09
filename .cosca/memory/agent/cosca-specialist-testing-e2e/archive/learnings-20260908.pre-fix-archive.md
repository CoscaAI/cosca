# cosca-specialist-testing-e2e - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-testing-e2e — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-e2e |
| **Task** | Initial capability establishment |
| **Technique** | Standard testing-e2e patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #testing-e2e #baseline #initialization |
| **Related** | .cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core testing-e2e patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

## Level 2 — E2E Scenario Definition

### 2026-07-28 — Platform-Wide E2E Scenario Design
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-e2e |
| **Task** | Define and document 5 E2E test scenarios for Cosca's critical user journeys |
| **Technique** | Full-platform analysis → journey mapping → scenario definition with detailed preconditions, steps, expected results, failure modes, and tooling recommendations |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #testing-e2e #scenario-design #risk-mitigation #R10 |
| **Related** | .cosca/memory/testing/e2e-scenarios.md, .cosca/memory/risk/RISK_REGISTRY.md |
| **Learned** | 5 cenários definidos cobrindo: (1) CLI init→provider→task, (2) Knowledge sync→index→search (FTS5+vector), (3) Agent task routing→execution→memory, (4) Review cycle G0-G9 gates, (5) CI/CD build→scan→deploy→smoke test. Estimativa de implementação: 14-21 dias (3-4 semanas). Ferramentas: Go testing nativo (CLI+API), Playwright (Web), k6 (carga), act (CI local). Principal risco: dependência de LLM provider para C3 — mitigar com Ollama local ou mock provider. |
| **Next** | Level 3: Implementar C1 (CLI E2E — menor risco) usando t.TempDir() + exec.Command, estabelecer padrão de helpers compartilhados |

