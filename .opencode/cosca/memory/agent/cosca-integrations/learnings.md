# cosca-integrations — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Initial capability establishment |
| **Technique** | Standard integrations patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #integrations #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core integrations patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Full Integration Audit (First Real Task)
| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Auditar todas as integrações da plataforma Cosca — LLM providers, HTTP clients, webhooks, resiliência |
| **Technique** | Full codebase exploration — static analysis of all provider implementations, transport layer, security patterns |
| **Level** | 2 |
| **Outcome** | success |
| **Confidence** | 0.75 |
| **Tags** | #integrations #audit #llm-providers #resilience #security #R17 |
| **Related** | .opencode/cosca/memory/integrations/audit-report.md, .opencode/cosca/memory/risk/RISK_REGISTRY.md |
| **Learned** | 11 providers mapeados (10 chat + 1 local-only). Arquitetura sólida com `openaicompat` eliminando ~80% duplicação. 4 bugs encontrados: DeepSeek double-read body, Google API key em query param, rate limiter duplicado, retry delegado a executor layer inexistente. Zero circuit breaker, zero métricas de integração, zero webhooks não-LLM. R17 parcialmente explicado: Together AI e Fireworks AI são triviais (~2h); Cohere e HuggingFace exigem implementação dedicada. |
| **Next** | Prioridade A1: Implementar circuit breaker + unificar retry. Prioridade A2: Métricas de integração. Quick wins: corrigir DeepSeek, migrar Google auth, unificar rate limiters, adicionar Together AI + Fireworks AI. |
