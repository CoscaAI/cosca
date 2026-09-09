# Integrations Audit Report — Cosca v1.4.0-dev

> **Auditor**: cosca-integrations (Integrations Chief)
> **Date**: 2026-07-28
> **Scope**: All third-party API integrations, LLM providers, webhooks, external services
> **Risk Registry Reference**: R17 ("Provider coverage — 6 providers faltando")

---

## 1. Executive Summary

A plataforma Cosca integra com **11 providers** de serviços externos, todos focados em LLM (chat + embeddings). Não foram encontradas integrações não-LLM (GitHub, Slack, cloud services, webhooks). A arquitetura de providers é madura: interfaces consistentes, fábricas com auto-registro via `init()`, e uma camada compartilhada (`openaicompat`) que elimina duplicação para providers compatíveis com OpenAI.

**Forças**:
- 10 providers de chat funcionais e 9 de embeddings
- Rate limiting (token bucket) implementado
- Retry com exponential backoff em todos os providers
- Transport HTTP compartilhado com connection pooling
- Secrets vault com AES-256-GCM para tokens
- Testes unitários para todos os providers (httptest + golden files)

**Gaps críticos**:
- **Sem circuit breaker** — falhas em cascata são possíveis
- **Retry inconsistente** — algumas implementações têm retry no provider, outras delegam a um "executor layer" que não existe
- **Sem métricas/monitoramento** — zero observabilidade de integrações (latência, taxa de erro, saturação de rate limit)
- **Retry sem jitter** — thundering herd em rate limits compartilhados
- **Google passa API key em query param** — exposto em logs de proxy

---

## 2. Inventory — All Integrations

### 2.1 LLM Chat Providers

| # | Provider | Status | Interface | Stream | Tools | Retry | Rate Limit | Tests |
|---|----------|--------|-----------|--------|-------|-------|------------|-------|
| 1 | **openai** | 🟢 Functional | `chat.ChatProvider` | SSE ✅ | ✅ | ✅ (manual loop) | ✅ (RPM) | 1,526 linhas |
| 2 | **anthropic** | 🟢 Functional | `chat.ChatProvider` | SSE ✅ | ✅ | ⚠️ (delegado) | ❌ | 1,492 linhas |
| 3 | **google** | 🟢 Functional | `chat.ChatProvider` | SSE ✅ | ✅ | ⚠️ (delegado) | ❌ | ✅ |
| 4 | **azure** | 🟢 Functional | `chat.ChatProvider` | SSE ✅ | ✅ | ⚠️ (delegado) | ✅ (RPM) | ✅ |
| 5 | **deepseek** | 🟢 Functional | `chat.ChatProvider` (via openaicompat) | SSE ✅ | ✅ | ⚠️ (delegado) | ❌ | ✅ |
| 6 | **groq** | 🟢 Functional | `chat.ChatProvider` (via openaicompat) | SSE ✅ | ✅ | ⚠️ (delegado) | ❌ | ✅ |
| 7 | **mistral** | 🟢 Functional | `chat.ChatProvider` (via openaicompat) | SSE ✅ | ✅ | ⚠️ (delegado) | ❌ | ✅ |
| 8 | **ollama** | 🟢 Functional | `chat.ChatProvider` | NDJSON ✅ | ❌ | ⚠️ (delegado) | ❌ | ✅ |
| 9 | **bedrock** | 🟢 Functional | `chat.ChatProvider` (Converse API) | NDJSON ✅ | ✅ | ⚠️ (delegado) | ❌ | 1,048 linhas |
| 10 | **local** | 🟢 N/A (no chat) | — | — | — | — | — | ✅ |

### 2.2 LLM Embedding Providers

| # | Provider | Status | Interface | Batch | Retry | Rate Limit | Tests |
|---|----------|--------|-----------|-------|-------|------------|-------|
| 1 | **openai** | 🟢 Functional | `embeddings.Provider` | ✅ (20/batch) | ✅ (3 retries) | ✅ (1M TPM) | ✅ |
| 2 | **anthropic** | 🔴 **Stub** | `embeddings.Provider` | — | — | — | ✅ |
| 3 | **google** | 🟢 Functional | `embeddings.Provider` | ✅ (10/batch) | ✅ (3 retries) | ✅ (1500 TPM) | ✅ |
| 4 | **azure** | 🟢 Functional | `embeddings.Provider` | ✅ (20/batch) | ✅ (3 retries) | ✅ (1M TPM) | ✅ |
| 5 | **deepseek** | 🟢 Functional | `embeddings.Provider` | ❌ (single only) | ❌ | ✅ (500 TPM) | ✅ |
| 6 | **groq** | 🟢 Functional | `embeddings.Provider` (via openaicompat) | ✅ (20/batch) | ✅ (3 retries) | ✅ (300K TPM) | ✅ |
| 7 | **mistral** | 🟢 Functional | `embeddings.Provider` (via openaicompat) | ✅ (20/batch) | ✅ (3 retries) | ✅ (500K TPM) | ✅ |
| 8 | **ollama** | 🟢 Functional | `embeddings.Provider` | ❌ (one-by-one) | ✅ (2 retries) | ❌ | ✅ |
| 9 | **bedrock** | 🟢 Functional | `embeddings.Provider` | ❌ (one-by-one) | ✅ (3 retries) | ✅ (500K TPM) | ✅ |
| 10 | **local** | 🟢 Functional | `embeddings.Provider` (TF-IDF) | ✅ (batch) | N/A (local) | N/A | ✅ |

### 2.3 Non-LLM Integrations

| Type | Status |
|------|--------|
| Webhooks | ❌ Nenhum encontrado |
| GitHub API | ❌ Nenhum encontrado |
| Slack/Discord | ❌ Nenhum encontrado |
| Cloud services (AWS SDK, GCP SDK) | ❌ Nenhum (Bedrock usa HTTP direto + SigV4) |
| OAuth providers | ❌ Nenhum |

---

## 3. R17 Analysis — "6 providers faltando"

O Risk Registry reporta 6 providers de LLM ainda não integrados. Com 10 já implementados, os providers provavelmente ausentes são:

| # | Provider | Prioridade | Justificativa |
|---|----------|-----------|---------------|
| 1 | **Cohere** | Média | Embeddings + Chat próprios; API difere de OpenAI |
| 2 | **HuggingFace** | Média | Inference API; ecossistema amplo de modelos |
| 3 | **Together AI** | Baixa | OpenAI-compatible; trivial via openaicompat (1 arquivo) |
| 4 | **Replicate** | Baixa | API REST própria para modelos open-source |
| 5 | **Fireworks AI** | Baixa | OpenAI-compatible; trivial via openaicompat |
| 6 | **xAI (Grok)** | Baixa | API própria; baixa demanda atual |

**Nota**: Together AI e Fireworks AI podem ser adicionados com ~80 linhas cada usando `openaicompat.Register()`. Cohere e HuggingFace exigem implementações dedicadas (~500 linhas cada).

---

## 4. Resilience Audit

### 4.1 Retry Logic

| Aspect | Status | Detail |
|--------|--------|--------|
| Exponential backoff | ✅ Parcial | Fórmula: `attempt² * 1s` (quadrática, sem jitter) |
| Retry centralizado | ❌ Ausente | Cada provider implementa seu próprio loop ou delega a "executor layer" inexistente |
| Non-retryable errors | ✅ Presente | `IsNonRetryable()`: 400, 401, 403, 404 |
| Max retries configurável | ✅ Presente | Padrão: 3; configurável por provider |
| Retry-After header | ❌ Ausente | Nenhum provider usa o header `Retry-After` |

**Providers com retry no próprio código** (✅ real):
- openai (embeddings): loop manual com backoff
- bedrock (embeddings): loop manual com backoff
- azure (embeddings): loop manual com backoff
- ollama (embeddings): loop manual com backoff
- openaicompat (embeddings): loop manual com backoff

**Providers que delegam retry a "executor layer"** (⚠️ inexistente):
- anthropic (chat): comentário diz "Retry is handled by the executor layer"
- google (chat): comentário diz "Retry is handled by the executor layer"
- ollama (chat): comentário diz "Retry is handled by the executor layer"
- bedrock (chat): comentário diz "Retry is handled by the executor layer"
- openaicompat (chat): comentário diz "Retry is handled by the executor layer"
- openai (chat): comentário diz "retry with exponential backoff" mas `sendChatRequest` não tem retry loop

**Risco**: 6 providers de chat não têm retry real. Se o "executor layer" não existir (não encontrei evidência no código), falhas transitórias (5xx, timeouts) não são recuperadas para esses providers.

### 4.2 Circuit Breaker

| Aspect | Status |
|--------|--------|
| Circuit breaker | ❌ **Ausente** |
| Half-open state | ❌ |
| Failure threshold | ❌ |
| Cooldown period | ❌ |

Nenhum padrão de circuit breaker foi implementado. Se um provider ficar indisponível, todas as requisições continuarão sendo enviadas e falhando, sem fast-fail.

### 4.3 Rate Limiting

| Aspect | Status |
|--------|--------|
| Token bucket | ✅ `providers.RateLimiter` e `openaicompat.rateLimiter` (duas implementações sobrepostas) |
| RPM-based | ✅ Usado em openai chat (10K RPM), azure chat (10K RPM) |
| TPM-based | ✅ Usado em openai embeddings (1M TPM), bedrock (500K TPM) |
| Rate limit headers | ❌ `x-ratelimit-*` headers não são lidos/processados |

**Duplicação**: Existem dois rate limiters idênticos: `providers.RateLimiter` e `openaicompat.rateLimiter`. O segundo deveria ser removido em favor do primeiro.

### 4.4 Timeout & Connection Management

| Aspect | Status | Detail |
|--------|--------|--------|
| HTTP client timeout | ✅ | Configurável por provider (60s–300s) |
| Connection pooling | ✅ | `SharedTransport`: 100 idle conns, 20/host |
| TLS handshake timeout | ✅ | 10s |
| Request context propagation | ✅ | `http.NewRequestWithContext` em todos |
| DNS caching | ⚠️ | Go default (não configurado explicitamente) |

### 4.5 Error Handling

| Aspect | Status |
|--------|--------|
| Erro categorizado por status code | ✅ Todos os providers |
| Erro wrapping | ✅ `fmt.Errorf("context: %w", err)` |
| Non-retryable detection | ✅ 400/401/403/404/404 |
| Truncated error bodies | ✅ `TruncateBody()` (200 chars) |
| Provider error type | ✅ `chat.NewProviderError()` |
| **Falha silenciosa no DeepSeek** | 🔴 **Bug**: Lê response body via `io.ReadAll` ANTES de verificar status code, depois tenta `json.NewDecoder(resp.Body)` em body já lido ⇾ decoder sempre falha em erro HTTP |

---

## 5. Security Audit

### 5.1 API Key Management

| Aspect | Status | Detail |
|--------|--------|--------|
| Env vars | ✅ | Todas as API keys via `os.Getenv()` |
| Secrets vault | ✅ | AES-256-GCM encrypted storage |
| No hardcoded keys | ✅ | Nenhuma key no código fonte |
| Key validation | ✅ | Providers retornam erro claro se key ausente |
| Key rotation | ⚠️ | Sem mecanismo de rotação; requer restart |

### 5.2 Auth Patterns

| Provider | Auth Method | Security |
|----------|-------------|----------|
| openai | `Authorization: Bearer <key>` | ✅ |
| anthropic | `x-api-key: <key>` | ✅ |
| google | Query param `?key=<key>` | 🔴 **Expõe key em logs de proxy/LB** |
| azure | `api-key: <key>` | ✅ |
| deepseek | `Authorization: Bearer <key>` | ✅ |
| groq | `Authorization: Bearer <key>` | ✅ |
| mistral | `Authorization: Bearer <key>` | ✅ |
| ollama | Sem auth (local) | ✅ (localhost only) |
| bedrock | AWS SigV4 | ✅ |
| local | N/A | ✅ |

**Ação recomendada**: Migrar Google Gemini de query param para header (`x-goog-api-key` já é usado no embeddings provider, mas inconsistente com o chat provider que usa query param).

### 5.3 Transport Security

| Aspect | Status |
|--------|--------|
| TLS | ✅ HTTPS para todos os providers (exceto ollama localhost) |
| HTTP/2 | ✅ `ForceAttemptHTTP2: true` |
| Certificate validation | ✅ Default Go TLS (não desabilitado) |
| Request body limits | ✅ `io.LimitReader(resp.Body, 10MB)` |

---

## 6. Infrastructure & Architecture

### 6.1 Shared Components

| Component | Location | Status |
|-----------|----------|--------|
| `SharedTransport()` | `providers/transport.go` | ✅ Connection pooling |
| `SharedHTTPClient()` | `providers/transport.go` | ✅ Timeout-configurable |
| `RateLimiter` | `providers/ratelimit.go` | ✅ Token bucket |
| `RateLimiter` (duplicate) | `openaicompat/embeddings.go` | ⚠️ Duplicado |
| `Manager` | `providers/providers.go` | ✅ Provider listing/test/status |
| `ProviderRegistry` (interface) | `providers/providers.go` | ✅ Dynamic registration |
| `openaicompat` (shared impl) | `providers/openaicompat/` | ✅ Elimina ~80% duplicação |

### 6.2 Registration Pattern

Todos os providers usam `init()` + `chat.GetRegistry().Register()` / `embeddings.GetRegistry().Register()`:

| Provider | Chat Priority | Embedding Priority |
|----------|---------------|-------------------|
| openai | 10 | 10 |
| anthropic | 20 | 200 |
| google | 25 | 30 |
| azure | 30 | 30 |
| mistral | 45 | 40 |
| bedrock | 50 | 60 |
| groq | 55 | 50 |
| deepseek | 15 | 70 |
| ollama | 20 | 20 |
| local | — | 100 (fallback) |

---

## 7. Test Coverage

### 7.1 Provider Tests

| Provider | Chat Tests | Embedding Tests | HTTP Mock | Lines |
|----------|-----------|-----------------|-----------|-------|
| openai | ✅ (1,526 L) | ✅ | ✅ httptest | ~2,900 total |
| anthropic | ✅ (1,492 L) | ✅ | ✅ httptest | ~1,560 total |
| google | ✅ | ✅ | ✅ httptest | — |
| azure | ✅ | ✅ | ✅ httptest | — |
| deepseek | ✅ | ✅ | ✅ httptest | — |
| groq | ✅ | ✅ | ✅ httptest | — |
| mistral | ✅ | ✅ | ✅ httptest | — |
| ollama | ✅ | ✅ | ✅ httptest | — |
| bedrock | ✅ | ✅ | ✅ httptest | — |
| local | N/A | ✅ | N/A | 130 L |
| providers (root) | N/A | N/A | ✅ Manager/Transport/RateLimit | 912 L |

### 7.2 Coverage Gaps

| Gap | Impact |
|-----|--------|
| Sem testes de integração com APIs reais | Bugs de parsing de resposta só aparecem em produção |
| Sem testes de resiliência (chaos) | Circuit breaker, retry exhaustion não validados |
| Sem testes de rate limit | Comportamento sob 429 não testado |
| DeepSeek embeddings bug não detectado | O bug do body double-read passou nos testes unitários? |

---

## 8. Recommendations — Prioritized

### 🔴 Critical (this sprint)

| # | Action | Effort | Resolves |
|---|--------|--------|----------|
| **A1** | **Implementar circuit breaker** — `gobreaker` ou implementação própria com estados closed/open/half-open, failure threshold configurável, cooldown. Aplicar a todos os providers de chat + embeddings. | 2-3 dias | Gaps de resiliência |
| **A2** | **Unificar retry logic** — Criar `RetryExecutor` centralizado que todos os providers usem. Suportar backoff exponencial com jitter, `Retry-After` header, tentativas configuráveis. | 1-2 dias | Retry delegado a layer inexistente |
| **A3** | **Corrigir bug DeepSeek embeddings** — Mover `io.ReadAll` para depois da verificação de status code e usar `json.Unmarshal` em vez de `json.NewDecoder` em body já lido. | 30 min | Bug: double-read em erro HTTP |

### 🟠 High (this month)

| # | Action | Effort | Resolves |
|---|--------|--------|----------|
| **B1** | **Adicionar métricas de integração** — Latência p50/p95/p99, taxa de erro, saturação de rate limit, por provider. Exportar via Prometheus/OpenTelemetry. | 2-3 dias | Observabilidade zero |
| **B2** | **Migrar Google Gemini para header auth** — Substituir `?key=` query param por header `x-goog-api-key` (já usado no embeddings). | 1 hora | Segurança |
| **B3** | **Unificar rate limiters** — Remover `openaicompat.rateLimiter` e usar apenas `providers.RateLimiter`. | 1 hora | Código duplicado |
| **B4** | **Health check endpoints** — Endpoint `/v1/providers/{name}/health` com latência, status, últimas falhas. | 1 dia | Monitoramento |

### 🟡 Medium (plan)

| # | Action | Effort | Resolves |
|---|--------|--------|----------|
| **C1** | **Adicionar Together AI + Fireworks AI** — Providers OpenAI-compatible (trivial via openaicompat). ~160 linhas total. | 2 horas | R17 parcial |
| **C2** | **Adicionar Cohere** — Implementação dedicada para chat + embeddings. | 1-2 dias | R17 parcial |
| **C3** | **Testes de resiliência** — Chaos testing: simular 5xx, timeouts, rate limits, slow responses. | 2 dias | Coverage gaps |
| **C4** | **Jitter no backoff** — Adicionar `+ rand(0, backoff)` ou full jitter ao retry centralizado. | 30 min | Thundering herd |

### ⚪ Low (backlog)

| # | Action | Effort | Resolves |
|---|--------|--------|----------|
| **D1** | Webhooks e integrações não-LLM (GitHub, Slack) | — | Sob demanda |
| **D2** | HuggingFace + Replicate + xAI providers | — | Cobertura completa |
| **D3** | Rotação automática de API keys | — | Segurança |

---

## 9. Quick Wins (< 1 day total)

1. **Corrigir DeepSeek embeddings** (30 min) — Bug real encontrado
2. **Migrar Google para header auth** (1 hora) — Segurança
3. **Unificar rate limiters** (1 hora) — Eliminar duplicação
4. **Adicionar Together AI + Fireworks AI** (2 horas) — 2 dos 6 providers R17
5. **Adicionar jitter ao backoff** (30 min) — Resiliência

**Total**: ~5 horas para 5 melhorias de alto impacto.

---

## Appendix A — Environment Variables Reference

| Variable | Provider | Used By |
|----------|----------|---------|
| `OPENAI_API_KEY` | openai | chat + embeddings |
| `ANTHROPIC_API_KEY` | anthropic | chat |
| `GOOGLE_API_KEY` | google | chat + embeddings |
| `AZURE_OPENAI_API_KEY` | azure | chat + embeddings |
| `AZURE_OPENAI_ENDPOINT` | azure | chat + embeddings |
| `AZURE_OPENAI_DEPLOYMENT` | azure | chat + embeddings |
| `AZURE_OPENAI_API_VERSION` | azure | chat |
| `DEEPSEEK_API_KEY` | deepseek | chat + embeddings |
| `GROQ_API_KEY` | groq | chat + embeddings |
| `MISTRAL_API_KEY` | mistral | chat + embeddings |
| `OLLAMA_HOST` | ollama | chat + embeddings |
| `AWS_ACCESS_KEY_ID` | bedrock | chat + embeddings |
| `AWS_SECRET_ACCESS_KEY` | bedrock | chat + embeddings |
| `AWS_SESSION_TOKEN` | bedrock | chat |
| `AWS_REGION` | bedrock | chat + embeddings |
| `BEDROCK_REGION` | bedrock | chat (alternative) |
| `BEDROCK_MODEL` | bedrock | chat |
| `COSCA_JWT_SECRET` | secrets vault | encryption key derivation |

---

## Appendix B — Bug Registry

| # | Provider | Severity | Description |
|---|----------|----------|-------------|
| BUG-001 | deepseek (embeddings) | 🔴 High | `io.ReadAll` lê o body antes do status check; `json.NewDecoder(resp.Body)` em body já consumido sempre falha em erros HTTP |
| BUG-002 | google (chat) | 🟡 Medium | API key via query param `?key=` — exposta em logs de proxy/LB |
| BUG-003 | openaicompat | 🟢 Low | Rate limiter duplicado (`providers.RateLimiter` e `openaicompat.rateLimiter`) |
| BUG-004 | multiple (chat) | 🟠 High | Retry delegado a "executor layer" que não foi encontrada no código |
