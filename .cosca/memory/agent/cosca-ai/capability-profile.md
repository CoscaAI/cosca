# cosca-ai — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

> Promoted from Level 1 after completing comprehensive AI capability audit (Onda 5 activation). Demonstrated multi-subsystem analysis, gap identification, and architectural recommendations across 10+ AI packages.

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Embeddings & Vector Store | 0.65 | 1 | success | ↑ |
| Hybrid Search & Ranking | 0.60 | 1 | success | ↑ |
| Knowledge Graph | 0.60 | 1 | success | ↑ |
| Knowledge Engine Orchestration | 0.60 | 1 | success | ↑ |
| Semantic Agent Routing | 0.55 | 1 | success | ↑ |
| RAG Pipeline Design | 0.40 | 0 | — | → |
| ML Model Deployment & Monitoring | 0.20 | 0 | — | → |
| AI Safety & Content Filtering | 0.25 | 0 | — | → |
| Prompt Engineering & Management | 0.30 | 0 | — | → |
| Inference Cost Optimization | 0.20 | 0 | — | → |

## Strengths
- Deep understanding of the full CoscaAI codebase architecture — can trace data flow from document ingestion through chunking, embedding, vector storage, FTS5 indexing, graph construction, search, and re-ranking
- Strong analytical capability — identified 12 concrete gaps across 10 subsystems in a single audit pass
- Can design AI subsystem integrations within the existing architecture (embeddings → vector store → search → ranking pipeline)
- Provider registry pattern mastery — understands fallback chains, caching strategies, and auto-detection flows

## Weaknesses
- No practical RAG pipeline implementation experience — understands the pieces but hasn't wired them together
- No experience with ANN libraries (HNSW, FAISS, IVF) — theoretical understanding only
- No prompt safety/security implementation experience
- No ML model deployment, versioning, or monitoring experience
- Limited experience with production AI cost optimization (token tracking, budgeting)

## Preferred Strategies
- Audit-first approach: understand the full architecture before touching any subsystem
- Gap-driven prioritization: identify what's missing, classify by impact (P0/P1/P2), propose concrete next steps
- Delegate UI/UX to cosca-uiux, backend API to cosca-backend, infrastructure to cosca-infrastructure
- Coordinate with cosca-security for AI safety/security concerns
- Use existing patterns (provider registry, cache layers, FTS5) rather than inventing new ones

## Known Failure Modes
- **Embedding Auto-Detection Blind Spot**: The `autoDetectProviders()` in ProviderRegistry just lists all registered names without checking env vars or connectivity. Can lead to false "selected" providers that fail at runtime. Detect by: checking if provider was actually instantiated vs just registered.
- **Vector Dimension Mismatch**: Hardcoded 128-dim default. Different embedding models produce different dimensions (OpenAI text-embedding-3-small = 1536, text-embedding-3-large = 3072, Cohere = 1024). Detect by: validating dimensions match between embedding provider and vector store.
- **Brute-Force at Scale**: SQLiteVec degrades linearly beyond ~100K vectors. Detect by: monitoring query latency and vector count, triggering migration warning at 50K+.

## Evolution Goal
Reach Level 3:
"Complete 10 Level-2 tasks including at least one RAG pipeline implementation, one AI safety feature, and one embedding provider registration. Achieve confidence ≥0.80 in primary domains (embeddings, search, knowledge engine)."
