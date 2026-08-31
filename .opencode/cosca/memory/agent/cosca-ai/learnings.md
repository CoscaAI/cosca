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

---

### 2026-08-31 — Automatic Vision Recognition Hook in the Agent Loop
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Fazer a visão ser reconhecida automaticamente no loop do agente (imagem → rodar pipeline Go ONNX → injetar entendimento semântico no contexto, sem depender do LLM ver a imagem) |
| **Technique** | Additive pre-Chat hook with dependency-injected runner (testable seam), graceful degradation, opt-in config gate |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #vision #automatic-recognition #agent-loop #multimodal #onnx #graceful-degradation #dependency-injection #config-gate |
| **Related** | internal/cli/agent.go, internal/cli/agent_vision.go, internal/cli/agent_vision_test.go, internal/worldmodel/vision/vision_detect.go, internal/config/config.go |
| **Learned** | (1) Ponto de integração real: `runAgentToolLoop` em internal/cli/agent.go monta `messages []chat.Message` e chama `provider.Chat`. O hook plugado enriquece a conversa base (System+User) UMA vez antes do loop, não por iteração — evita re-injeção duplicada a cada turno já que `messages` cresce no loop. (2) `chat.Message` guarda imagens em `ContentParts[].ImageURL.URL` (data-URI); `Message.HasImage()` só checa image_url; o hook percorre ContentParts e decodifica só prefixo `data:image/` (sem fetch de http). (3) `vision.DecodeDataURI` + `vision.DetectImageAndRunVision` + `(*Observation).SummaryText()` são reutilizados (não recriados). (4) **Degradação graciosa**: se `runner` erro ou retorna nil, a imagem é pulada e a mensagem passa intacta; observação degradada (warnings, zero entidades) ainda gera SummaryText → o modelo é informado que viu a imagem mas a percepção degradou. (5) Anexar o resumo como ContentPart de TEXTO na MESMA mensagem que carrega a imagem (AddText) é aditivo e provider-agnostic — preserva o turno do usuário; Avoid adicionar um system message no meio da conversa. (6) `visionRunner` como seam injetável (default = `vision.DetectImageAndRunVision`) permite teste determinístico com Observation fake, sem carregar onnxruntime; `go vet` rejeita comparar função com nil (sempre false) — não usar teste guard com função. (7) Config gate: adicionado `Config.Vision.Enabled` (opt-in, default false via `DefaultEnableVision`), env `COSCA_VISION__ENABLED`; passar `projectCfg.Vision.Enabled` para `runAgentToolLoop` (assinatura mudou, único caller). |
| **Next** | (Pendência honesta) (a) SAM2/seguir para segmentação de máscara por entidade na injection — hoje só SummaryText; (b) tratar http(s) URLs de imagem (precisa de HTTP client/baixa de bytes) — fora do escopo atual; (c) decidir se enriquecimento deve reaparecer em tool-call results que devolvem imagem (ex. read de screenshot); (d) considerar reusar `AgentRequest.Context` para visão quando o runtime usar o engine (não só agent run CLI). |
