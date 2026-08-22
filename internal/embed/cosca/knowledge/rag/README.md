# 23 — RAG / KNOWLEDGE / RETRIEVAL

> Stack 23 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em retrieval: embeddings, indexação, busca (vetorial/híbrida), reranking e knowledge graphs — para respostas **fundamentadas**, não plausíveis.

## PIPELINE RAG
```
INGEST → CHUNK → EMBED → INDEX → RETRIEVE → RERANK → CONTEXT → GENERATE → EVALUATE
```

## PRINCÍPIOS CORE
1. **Avaliar groundedness** — toda resposta RAG deve ser verificável contra a fonte (nunca "parece certo") · UNIVERSAL
2. **Chunking por semântica, não por tamanho** — preservar unidades de significado; overlap consciente · STRONG
3. **Hybrid search > vector-only** — combinar vetorial + BM25/keyword (cada tipo pega o que o outro perde) · STRONG
4. **Rerank** melhora relevância real — small-to-large, cross-encoder quando o custo justifica · STRONG
5. **Contexto controlado** — não estourar o contexto com ruído; citação para rastreabilidade · UNIVERSAL
6. **Índice derivado = eventual consistency** — sincronizar com a fonte (change feed); reindex zero-downtime · STRONG
7. **Knowledge graph** quando a relação importa (não só similaridade) · CONTEXTUAL

## REGRAS DE DECISÃO
- **Vector DB**: pgvector (já no Postgres) como default; faiss/hnswlib para scale; weaviate/vespa para features (hybrid, metadata) — escolher por necessidade real.
- **Chunk**: por seção/cabeçalho > fixo; manter metadados (fonte, página, timestamp).
- **Avaliação**: retrieval (recall@k) + geração (groundedness, faithfulness).

## ANTI-PATTERNS
`RAG sem avaliação de groundedness` · `chunking cego (quebra significado)` · `vector-only (perde busca exata)` · `contexto estourado com ruído` · `sem rerank` · `índice dessincronizado da fonte` · `citação ausente (não rastreável)` · `embeddings sem revisão de idioma/dados`

## CHECKLIST (quality gate)
- [ ] Groundedness avaliada (cada claim → fonte)
- [ ] Chunking semântico + metadados
- [ ] Hybrid search (vetorial + keyword)
- [ ] Rerank considerado
- [ ] Contexto controlado + citações
- [ ] Sync fonte→índice + reindex testado
- [ ] Recall/faithfulness medidos

## A REGRA
RAG deve tornar a IA **verificável**: se a resposta não aponta para a fonte, não é resposta.

## REFERÊNCIAS
run-llama/llama_index · deepset-ai/haystack · infiniflow/ragflow · dify-ai/dify · facebookresearch/faiss · pgvector/pgvector · weaviate · chroma · vespa · spotify/annoy · nmslib/hnswlib
