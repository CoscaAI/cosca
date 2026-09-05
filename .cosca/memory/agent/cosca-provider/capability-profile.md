# cosca-provider — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2 (first real audit completed — Onda 5)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Provider integration audit & analysis | 0.85 | 1 | success | ↑ |
| Provider architecture design (routing, failover) | 0.70 | 1 | success | ↑ |
| Rate limiting & transport optimization | 0.78 | 1 | success | ↑ |
| Circuit breaker implementation | 0.60 | 0 | — | → |
| Multi-provider failover implementation | 0.55 | 0 | — | → |
| Provider cost tracking & optimization | 0.45 | 0 | — | → |

## Strengths
- Deep knowledge of all 10 provider implementations in the Cosca codebase (Go)
- Identified the openaicompat shared layer pattern and its effectiveness at DRY
- Ability to analyze retry, rate-limit, and transport patterns across providers
- Can identify duplication, inconsistency, and gaps between architecture docs and code
- Understands the Provider Registry pattern used for both chat and embeddings

## Weaknesses
- No hands-on implementation of circuit breaker or multi-provider failover yet
- Cost tracking domain is entirely theoretical (no code exists)
- No experience with provider health monitoring or continuous observability
- Never implemented a Provider Router (cost/latency/capability-based selection)

## Preferred Strategies
- Start every task by auditing existing code against architecture docs (PROVIDER_INTERFACE.md)
- Identify code duplication and push shared abstractions (like openaicompat but for retry/CB)
- Prioritize reliability (circuit breaker, failover) before optimization (cost routing)
- Use token-bucket rate limiting consistently across all provider implementations
- Delegate embedding model training to AI Chief, infrastructure to Infrastructure Chief

## Known Failure Modes
- Provider implementations diverge in error handling (some have retry, some don't) — need standardization
- openaicompat ChatProvider lacks rate limiting (gap vs OpenAI/Azure which have it)
- Bedrock embedBatch makes N individual HTTP requests — inefficient batching
- Architecture doc describes features (CB, failover, cost tracking) not yet implemented in code

## Evolution Goal
Reach Level 3:
"Implement circuit breaker and multi-provider failover in the executor layer, achieving sub-500ms failover as specified in PROVIDER_INTERFACE.md"
