# PROVIDER INTERFACE — AI Model Provider Abstraction

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-12

## PURPOSE
This document defines the standardized interface for AI model providers in the Cosca ecosystem. It enables:
- **Provider agnosticism** — Swap AI providers without changing agent logic
- **Provider failover** — Automatic fallback when primary provider fails
- **Cost optimization** — Route tasks to cheapest capable provider
- **Multi-model orchestration** — Use different providers for different task types

## ARCHITECTURE

```
┌──────────────────────────────────────────────────┐
│                 Cosca AGENT LAYER                   │
│  (Chiefs, Specialists, Engines)                   │
│  Agents request: "complete this task"             │
│  Agents do NOT know which provider is used        │
└──────────────────────┬───────────────────────────┘
                       │
              ┌────────▼────────┐
              │ PROVIDER ROUTER │
              │ (selects best   │
              │  provider per   │
              │  task type)     │
              └────────┬────────┘
                       │
     ┌─────────────────┼─────────────────┐
     │                 │                 │
┌────▼─────┐   ┌──────▼──────┐   ┌─────▼──────┐
│ PRIMARY  │   │ SECONDARY   │   │ FALLBACK   │
│ Provider │   │ Provider    │   │ Provider   │
│ (OpenAI) │   │(Anthropic)  │   │ (Local LLM)│
└──────────┘   └─────────────┘   └────────────┘
```

## 1. PROVIDER REGISTRY

### 1.1 Provider Configuration

```yaml
providers:
  - id: openai-gpt4
    name: OpenAI GPT-4o
    type: cloud
    priority: 1                    # Lower = higher priority
    models:
      - gpt-4o
      - gpt-4o-mini
    capabilities:
      - code_generation
      - code_review
      - architecture_design
      - creative_writing
      - analysis
    cost_per_1k_tokens:
      input: 0.005
      output: 0.015
    rate_limit:
      requests_per_minute: 500
      tokens_per_minute: 100000
    timeout_ms: 240000
    retry:
      max_attempts: 3
      backoff_ms: 5000

  - id: anthropic-claude
    name: Anthropic Claude 3.5 Sonnet
    type: cloud
    priority: 2
    models:
      - claude-3-5-sonnet
      - claude-3-haiku
    capabilities:
      - code_generation
      - code_review
      - architecture_design
      - analysis
      - long_context
    cost_per_1k_tokens:
      input: 0.003
      output: 0.015
    rate_limit:
      requests_per_minute: 200
      tokens_per_minute: 80000
    timeout_ms: 240000
    retry:
      max_attempts: 3
      backoff_ms: 5000

  - id: local-llama
    name: Local Llama 3
    type: local
    priority: 10
    models:
      - llama-3-70b
    capabilities:
      - code_generation
      - analysis
      - simple_tasks
    cost_per_1k_tokens:
      input: 0.0
      output: 0.0
    rate_limit:
      requests_per_minute: 10
      tokens_per_minute: 20000
    timeout_ms: 300000
    endpoint: http://localhost:8080/v1
    retry:
      max_attempts: 1
      backoff_ms: 10000
```

### 1.2 Provider Capabilities Matrix

| Capability | GPT-4o | Claude 3.5 | Llama 3 | DeepSeek | Gemini |
|-----------|--------|------------|---------|----------|--------|
| code_generation | ✅ | ✅ | ✅ | ✅ | ✅ |
| code_review | ✅ | ✅ | ⚠️ | ✅ | ⚠️ |
| architecture_design | ✅ | ✅ | ❌ | ✅ | ⚠️ |
| creative_writing | ✅ | ✅ | ⚠️ | ⚠️ | ✅ |
| analysis | ✅ | ✅ | ✅ | ✅ | ✅ |
| long_context | ⚠️ | ✅ | ❌ | ✅ | ✅ |
| security_audit | ✅ | ✅ | ❌ | ✅ | ❌ |
| test_generation | ✅ | ✅ | ✅ | ✅ | ✅ |
| documentation | ✅ | ✅ | ✅ | ⚠️ | ✅ |

## 2. TASK ROUTING RULES

### 2.1 Task Type → Provider Selection

| Task Type | Primary | Secondary | Fallback | Temperature |
|-----------|---------|-----------|----------|-------------|
| Strategic (CEO) | GPT-4o | Claude 3.5 | — | 0.7 |
| Planning (CTO, Product) | GPT-4o | Claude 3.5 | — | 0.5 |
| Architecture | Claude 3.5 | GPT-4o | — | 0.5 |
| Code Generation | Claude 3.5 | GPT-4o | Llama 3 | 0.3 |
| Code Review | GPT-4o | Claude 3.5 | — | 0.2 |
| Security Audit | GPT-4o | Claude 3.5 | — | 0.1 |
| Testing | Claude 3.5 | GPT-4o | Llama 3 | 0.2 |
| Documentation | GPT-4o | Claude 3.5 | Llama 3 | 0.3 |
| Simple Tasks | Llama 3 | GPT-4o-mini | Claude Haiku | 0.1 |
| Creative | GPT-4o | Claude 3.5 | — | 0.7 |

### 2.2 Failover Logic

```
1. Attempt PRIMARY provider
   ├── Success → return result
   └── Failure (timeout, rate limit, error)
       2. Attempt SECONDARY provider
          ├── Success → return result, log failover event
          └── Failure
              3. Attempt FALLBACK provider
                 ├── Success → return result, log double-failover event
                 └── Failure → escalate to Kernel
```

### 2.3 Circuit Breaker

```
State: CLOSED → (failures > 5 in 60s) → OPEN (reject all for 30s)
State: OPEN → (30s elapsed) → HALF_OPEN (allow 1 probe request)
State: HALF_OPEN → (success) → CLOSED | (failure) → OPEN
```

## 3. PROVIDER API INTERFACE

### 3.1 Request Format (Provider-Agnostic)

```json
{
  "provider_id": "openai-gpt4",
  "model": "gpt-4o",
  "messages": [
    { "role": "system", "content": "You are the Cosca Architecture Chief..." },
    { "role": "user", "content": "Design a microservice architecture for..." }
  ],
  "temperature": 0.5,
  "max_tokens": 4096,
  "tools": ["read_file", "write_file"],
  "metadata": {
    "agent": "cosca-architecture",
    "department": "architecture",
    "workflow_id": "wf-001",
    "task_id": "task-042"
  }
}
```

### 3.2 Response Format (Provider-Agnostic)

```json
{
  "provider_id": "openai-gpt4",
  "model": "gpt-4o",
  "content": "Based on the requirements...",
  "tool_calls": [
    { "tool": "write_file", "arguments": { "path": "...", "content": "..." } }
  ],
  "usage": {
    "prompt_tokens": 1500,
    "completion_tokens": 800,
    "total_tokens": 2300,
    "cost_usd": 0.0195
  },
  "duration_ms": 4500,
  "finish_reason": "stop"
}
```

### 3.3 Error Response

```json
{
  "provider_id": "openai-gpt4",
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Retry after 30s.",
    "retry_after_ms": 30000,
    "failover_triggered": true,
    "failover_provider": "anthropic-claude"
  }
}
```

## 4. COST TRACKING

### 4.1 Cost Event

```json
{
  "timestamp": "2026-07-12T10:30:00Z",
  "provider": "openai-gpt4",
  "model": "gpt-4o",
  "agent": "cosca-architecture",
  "task_type": "architecture_design",
  "tokens": { "prompt": 1500, "completion": 800 },
  "cost_usd": 0.0195,
  "session_id": "sess-001",
  "workflow_id": "wf-001"
}
```

### 4.2 Cost Optimization Rules

| Rule | Action |
|------|--------|
| Simple task (< 100 LOC change) | Route to cheapest capable provider |
| Repeated task (same pattern) | Use cached result if < 1h old |
| Long context task (> 10k tokens) | Prefer Claude (lower input cost) |
| Batch processing | Aggregate into single request when possible |
| Idempotent retries | Reuse previous result on provider failover |

## 5. PROVIDER HEALTH MONITORING

| Metric | Threshold | Action |
|--------|-----------|--------|
| Error rate | > 5% in 5 min | Trigger failover |
| Latency p95 | > 30s | Log warning, consider secondary |
| Rate limit hits | > 10 in 1 min | Reduce concurrency |
| Cost per session | > $5.00 | Alert CTO |
| Circuit breaker opens | Any | Alert Monitoring Chief |

## 6. EXTENDING WITH NEW PROVIDERS

### 6.1 To Add a New Provider:

1. Implement the provider adapter interface:
   - `complete(request) → response`
   - `health_check() → status`
   - `get_models() → model_list`
   - `get_cost(model, tokens) → cost_usd`
2. Register in provider configuration YAML
3. Add to capability matrix
4. Configure routing rules
5. Test failover chain

### 6.2 Provider Adapter Template

```python
class ProviderAdapter:
    provider_id: str
    models: list[str]
    capabilities: list[str]
    
    async def complete(self, request: ProviderRequest) -> ProviderResponse: ...
    async def health_check(self) -> HealthStatus: ...
    def get_models(self) -> list[ModelInfo]: ...
    def get_cost(self, model: str, prompt_tokens: int, completion_tokens: int) -> float: ...
```

## RELATED
- [KERNEL.md](KERNEL.md) — Orchestration entry point
- [RUNTIME_CONTRACT.md](RUNTIME_CONTRACT.md) — Runtime interface that consumes providers
- [departments/ai/SKILL.md](departments/ai/SKILL.md) — AI Chief (provider selection strategy)
- [departments/monitoring/SKILL.md](departments/monitoring/SKILL.md) — Provider health monitoring
- [engines/observability/SKILL.md](engines/observability/SKILL.md) — Cost and latency metrics

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial provider interface — abstraction, failover, cost tracking |
