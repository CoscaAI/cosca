# 15 — AI / LLM / AGENT ENGINEERING INTELLIGENCE

> Stack 15 da Cosca Engineering Intelligence Matrix — **o stack do próprio Cosca**.

## MISSÃO
Construir sistemas de IA, agentes, ferramentas, memória, RAG, avaliação e orquestração — de forma controlada, observável e segura.

## PRINCÍPIOS CORE
1. **Loop do agente**: `MODEL → CONTEXT → MEMORY → TOOLS → PLANNING → EXECUTION → OBSERVATION → EVALUATION → FEEDBACK → GOVERNANCE` · UNIVERSAL
2. **Multi-agent NÃO é default** ("multi-agent = melhor" é falácia) — escolha a arquitetura pelo problema · STRONG
3. **Avaliação antes de confiar**: "parece inteligente" ≠ bom — avalie correctness, relevance, groundedness, tool usage, latency, cost · UNIVERSAL
4. **Memória em camadas** (conversation, semantic, episodic, procedural) com boundaries claras · STRONG
5. **Segurança de agente**: prompt injection, tool abuse, sandboxing, secret isolation, validação de output · UNIVERSAL

## REGRAS DE DECISÃO
- **RAG**: `INGEST → CHUNK → EMBED → INDEX → RETRIEVE → RERANK → CONTEXT → GENERATE → EVALUATE`. Comparar vector vs BM25 vs hybrid vs knowledge graph pelo dado.
- **Inference**: llama.cpp (local/CPU) vs vLLM/SGLang (GPU/serve) vs Ollama (dev) — pesar custo/latência/escala.
- Todo agente é um sistema que precisa passar pelo MESMO processo de avaliação que os projetos dos usuários.

## ANTI-PATTERNS
`multi-agent por status` · `RAG sem avaliação de groundedness` · `memória vazando contexto errado` · `tool sem validação/permissão` · `prompt injection não tratado` · `avaliar por "impressão"` · `agente sem observabilidade (prompts/tokens/custo)` · `self-consulta sem governança`

## CHECKLIST
- [ ] Avaliação definida (correctness/groundedness/tool usage)
- [ ] Observabilidade: prompts, tokens, latency, custo, falhas
- [ ] Sandbox + limites de ferramenta
- [ ] Input não confiável validado; output validado
- [ ] Memória com boundaries e retenção

## A REGRA
O objetivo não é um agente que "usa IA" — é uma plataforma capaz de raciocinar, executar, observar, aprender e melhorar de forma controlada. **O próprio Cosca deve ser avaliado pelo mesmo processo que aplica aos projetos dos usuários.**

## REFERÊNCIAS
LangChain/LangGraph · AutoGen · CrewAI · DSPy · OpenAI Agents · Anthropic Cookbook · MCP · Ollama · llama.cpp · vLLM · SGLang · pgvector/Qdrant/Milvus · TruLens/Phoenix/DeepEval
