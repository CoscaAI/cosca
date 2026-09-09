# cosca-integrations - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-integrations — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-08-28 — Mining: ruvnet/federated-mcp (MCP federado)
| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Deep mining de `ruvnet/federated-mcp` — alega seguir spec oficial de MCP federado |
| **Technique** | Static code analysis (não README): runtime grep de métodos MCP, chain de imports, chamadas externas, wire-up de entrypoints |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence** | 0.85 |
| **Tags** | #mining #mcp #federation #borrowing #ADR-017 #external-authority |
| **Learned** | Repo é SKELETON inflado por docs (docs/ + README descrevem recursos inexistentes). Runtime NÃO implementa MCP: `schema/schema.ts|json` é cópia da spec oficial NUNCA importada; protocolo real é envelope custom `{type,content}` (core/types.ts) tratado por switch (core/server.ts só info/capabilities; edge/mcp.ts só intent-detection/meeting-info/webhook). Zero initialize/tools/list/tools/call/resources/prompts. `packages/proxy/federation.ts` é stub: registry `Map<serverId,WebSocket>`, handleMessage só loga (nenhum roteamento/agregação/descoberta/conflito de nome). `auth.ts` gera JWT mas `verifyToken` NUNCA é chamado (token em query param `ws://` plaintext, um teatro). FederationProxy só usado em 2 testes — nenhum entrypoint o importa (server.ts/deploy-mcp.ts/worker/index.ts). `apps/node` não existe (package.json aponta phantom; deps contraditórias Deno vs Node). Edge = 3 endpoints HTTP que chamam OpenAI/Fireflies com raw fetch + Bearer env key, CORS `*`, SEM policy gate/sandbox/ledger/concentimento. |
| **Next** | Conceito aproveitável p/ Cosca: MCP Registry/Router que agrega tools de N servidores, namespaced por servidor, expostos como skills dos capos; interceptar no KERNEL/Capability Adapter, aplicar mcp_policy default-deny, gravar no ledger (externo nunca autoridade). ADOTAR só o *padrão* (registry+router+conflict naming+2 canais control/data), reescrever sobre `internal/chat/mcp`. REJEITAR todo o código e a noção de confiar no que o outro servidor "diz" ser MCP — o router do Cosca deve ser ESTRITO (validar initialize+capabilities+tools/list MCP real) para rejeitar pseudo-MCP. |
| **Related** | ADR-017 Capability Borrowing, internal/chat/mcp, internal/policy/mcp_policy.go, internal/contenttrust (I8) |

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Initial capability establishment |
| **Technique** | Standard integrations patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #integrations #baseline #initialization |
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
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
| **Related** | internal/embed/cosca/memory/integrations/audit-report.md, internal/embed/cosca/memory/risk/RISK_REGISTRY.md |
| **Learned** | 11 providers mapeados (10 chat + 1 local-only). Arquitetura sólida com `openaicompat` eliminando ~80% duplicação. 4 bugs encontrados: DeepSeek double-read body, Google API key em query param, rate limiter duplicado, retry delegado a executor layer inexistente. Zero circuit breaker, zero métricas de integração, zero webhooks não-LLM. R17 parcialmente explicado: Together AI e Fireworks AI são triviais (~2h); Cohere e HuggingFace exigem implementação dedicada. |
| **Next** | Prioridade A1: Implementar circuit breaker + unificar retry. Prioridade A2: Métricas de integração. Quick wins: corrigir DeepSeek, migrar Google auth, unificar rate limiters, adicionar Together AI + Fireworks AI. |

### 2026-08-28 — Mining: awslabs/agent-toolkit-for-aws (Agent Plugins v1.0.0 + Agent Skills)
| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Deep mining de `aws/agent-toolkit-for-aws` (clone em E:\cosca-tmp) |
| **Technique** | Static code analysis (não README): schemas (`tool/schemas/1.0.0/*.schema.json`), validadores (`validate_spec.py`/`validate.py`/`sync-plugin-skills.py`), `plugin.json`/`mcp.json`/`marketplace.json`, frontmatter de SKILL.md (`allowed-tools`), hook `com.anthropic.claude-code/hooks/secret-safety.py`, skill script (`agents-pay/x402_policy.py`) |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence** | 0.9 |
| **Tags** | #mining #agent-skills #agent-plugins #allowed-tools #provenance #I8 #borrowing #capo |
| **Learned** | Formato NÃO é novo p/ Cosca: `internal/skills` JÁ fala a mesma spec. `Skill` já tem `License/Compatibility/Metadata/AllowedTools(string)/Resources(scripts|references|assets)` + `Governance`; `Plugin/PluginManifest/Marketplace/MCPServer` = Agent Plugins v1.0.0; parser de frontmatter `---` + validação de campos. O Q que o toolkit entrega de NOVO: (1) `mcp_policy.go:101` `Evaluate(tool, _ map[string]any)` IGNORA `tool_input` e denega só por nome+classe grosseira — o `secret-safety.py` da AWS denega INSPECIONANDO args (get-secret-value/commands/SDK-call shapes) — GAP real. (2) gatilho de execução host-side (`PreToolUse` deny) com fail-open documentado (timeout 5s → tool passa). (3) caveat de segurança: `allowed-tools` de skill NÃO é load-bearing — "o gate está no código, não no frontmatter" (`agents-pay/SKILL.md:518`); reforça I8 (externo nunca autoridade). (4) biblioteca de conteúdo curado (155 SKILL.md) + biblioteca canônica→bundle via `sync-plugin-skills.py`. |
| **Next** | ADOTAR padrão "deny com insight nos argumentos" (não só nome) na camada de policy. Formalizar invariante: `Skill.AllowedTools` é ADVISÓRIO; enforce=policy/contenttrust/quarantine. Skills AWS importadas → `OriginImported` + `LifecycleQuarantined` (I6) adjudicadas por gate, com proveniência do marketplace (`source.path`+`version`+`owner`, I3). REJEITAR infra AWS (launch-with-aws, di_* Deep Insights, AgentCore/Gateway/Cedar, x402/MPP) e o endpoint remoto AWS MCP. |
| **Related** | internal/skills, internal/chat/mcp/registry.go, internal/policy/mcp_policy.go, internal/contenttrust, internal/quarantine, ADR-019 F1 |

