---
name: embedding-pipeline
description: Use when the user asks to design, build, or optimize an embedding pipeline for semantic search or RAG.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23

# EMBEDDING PIPELINE SKILL

## Description
Design, build, and optimize embedding pipelines for semantic search, RAG (Retrieval-Augmented Generation), similarity matching, and vector-based retrieval systems.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| document_source | Yes | Path or source of documents to embed |
| embedding_model | No | Model to use: `text-embedding-3-small`, `text-embedding-3-large`, `voyage-2`, `bge-large` |
| vector_db | No | Target vector database: `pgvector`, `pinecone`, `weaviate`, `qdrant`, `milvus` |
| chunking_strategy | No | `fixed-size`, `recursive`, `semantic`, `sentence` (default: recursive) |
| chunk_size | No | Token chunk size (default: 512) |
| chunk_overlap | No | Overlap between chunks (default: 64) |

## Outputs
| Output | Description |
|--------|-------------|
| Embedding pipeline | Configured and tested pipeline |
| Index configuration | Vector DB index optimized for retrieval |
| Performance benchmarks | Embedding speed, cost, quality metrics |
| Monitoring dashboards | Pipeline health and performance monitoring |

## Pipeline Components

### Document Loading
- Load from local files, S3, databases, APIs
- Support for PDF, HTML, Markdown, code, plaintext
- Metadata extraction (source, date, author, tags)
- Document deduplication by hash

### Chunking Strategy
- Fixed-size: Split by token count (simple, fast)
- Recursive: Split by separators (paragraph → sentence → word)
- Semantic: Split at topic boundaries (best quality)
- Sentence: Split at sentence boundaries (good for QA)

### Embedding Generation
- Batch processing for efficiency
- Rate limiting for API-based models
- Caching for duplicate content
- Retry with exponential backoff on failures

### Vector DB Indexing
- Index creation with appropriate distance metric
- Metadata filtering configuration
- Hybrid search setup (vector + keyword)
- Index optimization (HNSW, IVF parameters)

## Process
1. Analyze document sources and formats
2. Design chunking strategy based on content type
3. Configure embedding model and batch size
4. Set up vector DB schema and indexes
5. Implement document ingestion pipeline
6. Test embedding quality with sample queries
7. Benchmark performance (latency, throughput, cost)
8. Configure monitoring and alerting
9. Document pipeline architecture and configuration

## Success Criteria
- [ ] Pipeline ingests all document sources
- [ ] Embedding quality validated with test queries
- [ ] Vector DB indexes optimized for recall
- [ ] Performance benchmarks documented
- [ ] Cost per document tracked
- [ ] Monitoring configured for pipeline health

## Related
- [AI Chief](../../departments/ai/SKILL.md)
- [Prompt Engineering](./PROMPT_ENGINEERING.md)
- [Provider Discovery](./PROVIDER_DISCOVERY.md)
- [Database Chief](../../departments/database/SKILL.md)
- [templates/ai-platform/TEMPLATE.md](../../templates/ai-platform/TEMPLATE.md)
