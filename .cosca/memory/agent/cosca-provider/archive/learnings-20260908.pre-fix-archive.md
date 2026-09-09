# cosca-provider - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-provider — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Initial capability establishment |
| **Technique** | Standard provider patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #provider #baseline |
| **Learned** | Ready for Level 2 techniques. |
| **Next** | Identify first advanced technique to master |

## Audit Learnings (Onda 5 — 2026-07-28)

### 2026-07-28 — Full Provider Layer Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Comprehensive audit of all 10 LLM provider integrations |
| **Technique** | Systematic codebase audit: read every provider implementation, compare against architecture docs, identify gaps |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #provider #audit #10-providers #gap-analysis #architecture |
| **Related** | PROVIDER_INTERFACE.md, executor.go, openaicompat layer |
| **Learned** | 1) Cosca has 10 providers (OpenAI, Anthropic, Google, Azure, DeepSeek, Ollama, Groq, Mistral, Bedrock, Local/TF-IDF). 2) The openaicompat shared layer eliminates ~80% code duplication across OpenAI-compatible providers (DeepSeek, Groq, Mistral). 3) Retry with exponential backoff exists at the executor level but is single-provider only. 4) Token-bucket rate limiter is shared but inconsistently applied. 5) PROVIDER_INTERFACE.md describes circuit breaker, multi-provider failover, and cost tracking — none are implemented in code. 6) The Provider Manager (providers.go) has a catalog-based system with registry enrichment. 7) API key encryption at rest exists in config/crypto.go. 8) Transport.go provides shared HTTP/2 connection pooling. |
| **Next** | Implement circuit breaker pattern in executor; add multi-provider failover routing; standardize retry across all providers |

### 2026-07-28 — Inconsistency: Rate Limiting Gap in openaicompat Chat
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Compare rate limiting implementations across providers |
| **Technique** | Cross-provider pattern analysis |
| **Level** | 2 |
| **Outcome** | partial |
| **Tags** | #rate-limit #gap #openaicompat #inconsistency |
| **Related** | openaicompat/chat.go:86-90, openai/chat.go:87-89, azure/chat.go:96-100 |
| **Learned** | openaicompat.ChatProvider (used by DeepSeek, Groq, Mistral chat) has NO rateLimiter field. OpenAI and Azure ChatProviders have it. This means 3 providers operate without rate limiting. The fix is to add a RateLimiter to openaicompat.ChatProvider. |
| **Next** | Add RateLimiter to openaicompat ChatProvider; set sensible default RPM per provider config |

### 2026-07-28 — Architecture-Code Gap: Missing Circuit Breaker
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Verify circuit breaker implementation against PROVIDER_INTERFACE.md §2.3 |
| **Technique** | Document-driven verification |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #circuit-breaker #gap #architecture #reliability |
| **Related** | PROVIDER_INTERFACE.md §2.3, executor.go |
| **Learned** | PROVIDER_INTERFACE.md §2.3 defines a 3-state circuit breaker (CLOSED → OPEN → HALF_OPEN) with thresholds (5 failures in 60s → OPEN for 30s). Zero implementation exists in code. The executor has retry logic but no state tracking, no failure counting, and no circuit opening. This is the single biggest reliability gap. |
| **Next** | Implement circuit breaker at the executor level; integrate with the existing retry logic; add metrics for CB state transitions |

### 2026-07-28 — Architecture-Code Gap: Missing Provider Failover
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Verify multi-provider failover against PROVIDER_INTERFACE.md §2.2 |
| **Technique** | Document-driven verification |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #failover #gap #architecture #multi-provider |
| **Related** | PROVIDER_INTERFACE.md §2.2, orchestrator.go |
| **Learned** | PROVIDER_INTERFACE.md §2.2 defines a 3-tier failover chain (Primary → Secondary → Fallback). The orchestrator/executor currently binds to a SINGLE ChatProvider. There is no provider chain, no failover logic, no fallback provider selection. Each agent has one provider. |
| **Next** | Design ProviderChain abstraction; implement multi-provider executor that tries providers in priority order; integrate with circuit breaker state |

### 2026-07-28 — Inefficiency: Bedrock Single-Text Embedding
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Audit Bedrock embedding batching |
| **Technique** | Code review of batch processing loops |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #bedrock #performance #batching #inefficiency |
| **Related** | bedrock/bedrock.go:186-198 |
| **Learned** | bedrock.embedBatch iterates texts one-by-one (N HTTP requests for N texts) despite AWS Titan supporting batch embeddings. For a batch of 20 texts, this makes 20 SigV4-signed HTTP requests instead of 1. Other providers batch properly. |
| **Next** | Investigate Titan Embeddings v2 batch API; refactor to use single request with multiple inputs |

### 2026-07-28 — Code Duplication: Error Handling Utilities
| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | Identify duplicated utility functions across providers |
| **Technique** | Cross-file grep for isNonRetryable, truncateBody, estimateTokens, isBadRequest |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #duplication #dry #refactoring |
| **Related** | common.go, anthropic/chat.go, google/google.go, deepseek/deepseek.go, ollama/ollama.go |
| **Learned** | Several utility functions are duplicated: isNonRetryable (common.go + anthropic/chat.go), truncateBody (common.go + anthropic/chat.go + google/google.go), estimateTokens (common.go + google/google.go + ollama/ollama.go + deepseek/deepseek.go). common.go already exports TruncateString, TruncateBody, IsNonRetryable, EstimateTokens — the duplicates should be removed. |
| **Next** | Remove duplicated functions; ensure all providers import from providers/common.go |

### 2026-08-31 �?" Fix: Scaffold do client project agora herda provider/modelo (cosca project new)

| Field | Value |
|-------|-------|
| **Agent** | cosca-provider |
| **Task** | BUG: `cosca project new` gerava `.cosca/config.yml` sem provider/model; `cosca terminal --task` caia em fallback do ModelRouter dentro do client project. Causa raiz: template de scaffold (embed) não declara `provider`. |
| **Technique** | Injetar bloco `provider:` no HANDLER (project.go), lendo o template via embed e enriquecendo o conteúdo escrito (append/replace em nível raiz). NUNCA tocar em `internal/embed/**`. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #provider #scaffold #config-injection #client-project #project-new #yaml-injection #ModelRouter-fallback |
| **Related** | internal/cli/project.go (scaffoldProject, injectProviderBlock, renderProviderBlock, loadInheritedProvider, hasTopLevelKey, replaceTopLevelSection), internal/config/config.go (Config.Provider / config.Load), internal/embed/cosca/templates/scaffold/config.yml |
| **Learned** | 1) O template de scaffold do client project NÃO tem `provider`; toda injeção deve ser no handler `scaffoldProject` — o embed é intocável. 2) A herança deve vir de `config.Load()` (config do processo: defaults < project config < user config < env), retornando `cfg.Provider`. 3) Injeção via string-append (não re-marshal do YAML inteiro) preserva os comentários/placeholder do template e o `name: "acme-app"` já substituído — re-marshal quebraria o teste que checa `name: "acme-app"`. 4) `renderProviderBlock` NUNCA grava `api_key` em texto puro — só `api_key_env`. 5) Precedência: o `.cosca/config.yaml` do workspace pode sobrescrever `base_url: ""`, então o bloco herdado omite base_url quando vazio — seguro, pois o provider Ollama default é `http://localhost:11434` (ollama.go:98). |
| **Next** | Validar injetar também `embedding` quando um embed model estiver configurado no global; considerar herdar base_url do user-global quando o processo tiver vazio. |


