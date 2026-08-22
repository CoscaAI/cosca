# cosca-ai — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Initial capability establishment |
| **Technique** | Standard ai patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #ai #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core ai patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Active Learnings

### 2026-07-28 — Comprehensive AI Capability Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Onda 5 ativação — auditoria completa das capacidades AI do projeto |
| **Technique** | Full-stack codebase audit with dependency graph analysis, interface completeness checking, and gap identification |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #ai #audit #architecture #embeddings #vector-store #knowledge-engine #semantic-search #rag #gap-analysis |
| **Related** | internal/embeddings/, internal/vector/, internal/search/, internal/ranking/, internal/knowledge/, internal/graph/, internal/orchestration/semantic_router.go |
| **Learned** | CoscaAI possui uma arquitetura AI bem estruturada e funcional: (1) Provider Registry para embeddings com fallback chain e cache, (2) Vector Store SQLite-based com brute-force cosine similarity, (3) Hybrid Search Engine combinando FTS5 + Vector + Graph + Re-ranking, (4) Knowledge Graph com BFS/DFS/Dijkstra e 22 entity types, (5) Ranking multi-fator com BM25 + vector + graph proximity + freshness + popularity, (6) Semantic Router para agent selection via embedding similarity. Gaps: sem pipeline RAG explícito, sem model deployment/monitoring, sem content safety filters, sem prompt management, sem HNSW/IVF para scale >100K vectors, sem ADRs de AI. |
| **Next** | Criar ADR-001 (AI Architecture & RAG Pipeline Decision), priorizar P0 gaps (RAG pipeline, prompt injection safety, embedding provider completions), implementar HNSW index opcional |

### 2026-07-28 — Embedding Provider Registration Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Verificar quais provedores de embedding estão registrados e quais estão faltando |
| **Technique** | Codebase grep for RegisterEmbedding + provider init() analysis |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #embeddings #providers #registration #gap-analysis |
| **Related** | internal/providers/openaicompat/embeddings.go, internal/providers/*/init() |
| **Learned** | Apenas 4 provedores registram embeddings (Mistral, Groq, DeepSeek, Anthropic). Faltam: OpenAI nativo, Google/Gemini, Azure nativo, Ollama (local), Bedrock, Local/TF-IDF. O auto-detection no ProviderRegistry é naïf — apenas lista todos registrados sem testar conectividade/env vars. O provedor "local" (sempre disponível como fallback) NÃO implementa embeddings — é apenas para chat. |
| **Next** | Registrar provedores de embedding faltantes (OpenAI, Google, Azure, Ollama, Bedrock). Melhorar autoDetectProviders() para checar env vars e conectividade. |

### 2026-07-28 — Knowledge Graph Code Import Extraction Limitation
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Identificar limitações do extractor de imports do knowledge graph |
| **Technique** | Regex pattern analysis and language coverage assessment |
| **Level** | 2 |
| **Outcome** | partial |
| **Tags** | #graph #code-imports #language-support #limitation |
| **Related** | internal/graph/builder.go:ExtractCodeImports() |
| **Learned** | O ExtractCodeImports é Go-only (regex para `import ( ... )`). Não suporta Python (`import x` / `from x import y`), TypeScript/JavaScript (`import ... from ...`), Rust (`use ...`), Java (`import ...`), etc. Isso limita a construção do knowledge graph para projetos multi-linguagem. |
| **Next** | Implementar multi-language import extraction com strategy pattern (GoExtractor, PythonExtractor, TSExtractor, etc.) ou generalizar com regex multi-linguagem. |

### 2026-07-28 — Vector Store Scalability Ceiling
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Analisar escalabilidade do vector store SQLite |
| **Technique** | Complexity analysis of brute-force search vs ANN alternatives |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #vector-store #performance #scalability #hnsw #ann |
| **Related** | internal/vector/sqlite_vec.go, internal/vector/vector.go |
| **Learned** | O SQLiteVec usa brute-force cosine similarity (O(n) por query). Adequado para ~100K vetores. Para projetos com milhões de documentos, precisa de ANN: HNSW (hierarchical navigable small world), IVF (inverted file), ou FAISS integration. O schema SQLite atual usa BLOBs — não permite index acceleration nativa. Alternativas: pgvector (PostgreSQL), sqlite-vec extension, ou switch para Qdrant/Weaviate/Milvus para escala enterprise. |
| **Next** | Criar opção de backend HNSW (via gonum ou FAISS CGo binding) como alternativa ao brute-force. Manter SQLiteVec como default para <100K e habilitar HNSW para >100K. |
