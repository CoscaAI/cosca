# TEMPLATE: AI Platform

> **Version**: 1.0.0 | **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23

## DOMAIN
AI/ML platforms including LLM serving, RAG pipelines, embedding services, model management, and AI agent orchestration.

## RECOMMENDED STACK
| Layer | Primary | Alternative |
|-------|---------|-------------|
| LLM Runtime | OpenAI / Anthropic API | vLLM, TGI, Ollama |
| Embedding | OpenAI / Voyage | Sentence Transformers, Cohere |
| Vector Database | pgvector | Pinecone, Weaviate, Qdrant, Milvus |
| RAG Framework | LangChain / LlamaIndex | Haystack, custom pipeline |
| Agent Framework | LangGraph / CrewAI | AutoGen, Semantic Kernel |
| Prompt Management | LangSmith | Custom registry, Weights & Biases |
| Model Registry | MLflow | DVC, Hugging Face Hub |
| Monitoring | LangFuse | Helicone, custom OTel |
| Language | Python | TypeScript, Go |
| API Layer | FastAPI | Next.js API, Express |

## MODULE STRUCTURE
```
project-name/
├── models/                    # Model definitions and configs
│   ├── prompts/               # Prompt templates
│   ├── chains/                # Chain definitions
│   └── agents/                # Agent configurations
├── rag/                       # RAG pipeline
│   ├── ingestion/             # Document ingestion
│   ├── embedding/             # Embedding generation
│   └── retrieval/             # Retrieval strategies
├── api/                       # API layer
│   ├── routes/                # API endpoints
│   └── schemas/               # Request/response schemas
├── services/                  # Business logic services
│   ├── llm/                   # LLM interaction service
│   ├── vector/                # Vector search service
│   └── memory/                # Conversation memory
├── infrastructure/
│   ├── vector-db/             # Vector DB configuration
│   └── monitoring/            # AI observability
├── tests/
│   ├── unit/
│   ├── integration/
│   └── evaluation/            # LLM output evaluation
├── docs/
│   ├── ARCHITECTURE.md
│   ├── PROMPTS.md
│   └── EVALUATION.md
├── docker-compose.yml
└── README.md
```

## KEY FEATURES
- Multi-provider LLM support with failover
- RAG pipeline with configurable chunking and retrieval
- Vector database for semantic search
- Prompt versioning and management
- Streaming response support
- Token usage tracking and cost optimization
- LLM output evaluation and guardrails
- Conversation memory management
- A/B testing for prompt and model comparison
- Model fallback and graceful degradation

## ARCHITECTURE NOTES
- Abstract LLM provider behind interface for failover
- Embeddings cached to reduce API calls
- Prompt templates stored separately from code
- Vector DB indexed with appropriate distance metrics
- RAG pipeline uses hybrid search (semantic + keyword)
- All LLM calls have timeout and retry configured
- Token usage tracked per user/session for cost allocation
- Sensitive data filtered before sending to LLM providers
- Evaluation pipeline runs on every prompt change

## RELATED
- [AI Chief](../../departments/ai/SKILL.md)
- [Provider Chief](../../departments/provider/SKILL.md)
- [Prompt Engineering skill](../../skills/ai/PROMPT_ENGINEERING.md)
- [Embedding Pipeline skill](../../skills/ai/EMBEDDING_PIPELINE.md)
- [API Template](../api/TEMPLATE.md)
