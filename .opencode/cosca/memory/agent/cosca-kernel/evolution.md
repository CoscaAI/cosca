# cosca-kernel — Evolution Timeline

> Auto-evolution tracking. Records capability level progression with confidence scores.

## Current Level: 3
## Global Confidence: 0.68

## Evolution History

| Date | Level | Confidence | Capability | Trigger |
|------|-------|-----------|------------|---------|
| 2026-07-27 | 1 | 0.25 | Baseline capabilities established | Initial audit |
| 2026-07-28 | 3 | 0.88 | Cross-source audit (887 docs vs 357 Go files), framework design (DNA v3.0, metacognition pipeline), governance implementation (constitution, confidence model, curation engine), mass agent profiling (51 profiles), documentation integrity fix (52 issues across 19 files) | Evolution marathon — 9 commits, 150+ files |
| 2026-08-22 | 3 | 0.73 | Full brain audit (53 agents, 569+ files, 7 issues found and fixed), self-discovery (introspection into own memory files), failure registration (2 failures logged) | Don's order — "revira teu cérebro" |
| 2026-08-22 | 3 | 0.78 | Mining deepseek-harness (5 batedores paralelos, 28 padrões extraídos, gap P0 sandbox mapeado) | Don's order — "vamos dar uma minerada" |
| 2026-08-22 | 3 | 0.84 | Mining safra 8 orgs (busca por estrelas no GitHub, 7 docs de patterns, 8 batedores paralelos, gap self-evolution confirmado) | Don's order — "vamos minerar openai vercel spotify ifood uber aws n8n hermes" |
| 2026-08-22 | 3 | 0.85 | Mining org kubernetes (4 satélites: cri-api, autoscaler, community, kube-state-metrics, 27 padrões, gap sandbox P0 + KEP governança mapeados) | Don's order — mine the kubernetes org |
| 2026-08-22 | 3 | 0.86 | Mining Google + Claude (5 batedores, 35 padrões, org anthropics descoberta, hooks trust-model + meta-loop A/B + adk workflow mapeados) | Don's order — "revira google claude" |
| 2026-08-24 | 3 | 0.70 | **Blindagem + Gold + Arquitetura Modular** (semana inteira): blindagem do Cofre (Oracle+IA local, air-gap provado), ranking multi-fator (fix score=0), grafo ativado (GraphDistance BFS + dirty flag), gold do knowledge.db (grafo populado + dedup), conduta da Chain, ADR-013 (bancos modulares) + Fatia 1 (modlink route resolver) + Fatia 2 (busca obedece ao SearchScope) | Don's orders — bora resolver + bora delegate |
| 2026-08-24 | 3 | 0.74 | **O despertar APRENDEU a medir, não inferir** (marco de metacognição): no despertar seguinte, reportou estado real medido (0 ahead, sem números defasados, árvore suja honesta) em vez de inferir "ahead 1"/"0.68". Corrigiu cognitive-state (RESUME vs corpo histórico) + registrou a lição "despertar semântico = buscar + validar contra a realidade". | Don's order — "vê se tá correto agora?" (duas vezes) |
| 2026-08-27 | 3 | 0.76 | **Mining Context7 + execução com verificação prévia** (lição de método): minei Context7 com 4 batedores (arquitetura/segurança/DX/integrações), MAS os relatórios generalizavam padrões sem validar contra o nosso código real. Antes de delegar edições nos neurônios, verifiquei no código: descobri que (1) CORS já é fail-closed (serve.go), (2) trust-proxy já ignora X-Forwarded-For por padrão (ratelimit.go), (3) Validate() NÃO está morto no caminho real (Executor.ValidateToolCall) — cada um contrariava o relatório do batedor. A única mudança que valeu foi o **Normalize anti-alucinação de params** (normalize.go no Executor). **Lição: "validar contra a realidade antes de tocar os neurônios" sobrepõe "confiar no subagente" — o espelho do Don + medição > relatório de mining impreciso.** | Don's order — "marcha" |
| 2026-08-27 | 3 | 0.78 | **Blindagem da Lei do Cofre — providers externos removidos (fail-closed real)** (decisão do Don "1"): removi **todos** os providers de nuvem do registro — chat (deepseek/openai/anthropic em register_chat.go + gate COSCA_ENABLE_EXTERNAL_PROVIDERS) e embedding (openai/google/azure/mistral/groq/bedrock/anthropic/deepseek em cmd/cosca/main.go). Ficou **só ollama** (chat LLM + embedding). Prova: `register_chat_test.go` (novo) — `TestRegisterChatProviders_LocalOnly` (registry só tem gpu/ollama/none) + `TestSelect_FallbacksToLocal` (config pedindo "openai" → "unknown chat provider: openai" → skip → recua pro ollama, `primary: ollama`). **Lição: a Lei do Cofre exige REMOÇÃO do registro, não só env-var — env var pode ser esquecida/setada; provider não registrado é fail-closed real.** Também corrigi a causa raiz do recall 0 do benchmark: `newCLIKnowledgeEngine` não carregava o provider de embedding (config) → "no embedding provider selected" → recall 0 nos dois braços. | Don's order — "quero remover tudo que liga com externo" |
| 2026-08-27 | 3 | 0.80 | **Benchmark de recall HONESTO (Fase 0) + descoberta de qualidade do motor** (decisão do Don "senho todo de forma correta"): construí o `cosca gate recall --arm both` com ground truth REALISTA (21 chunks validados no banco, conjuntos de 3-5 por query em vez de 1 chunk exato), braço monastic (full-scan 38.854 vetores) e braço modular (roteado, `RouteCandidateIDs`). Resultado medido: monastic Recall@5=0.19/MRR=0.37/NDCG=0.21; modular NoRoute=6/6 (recusa, não compete). **DESCOBERTA CENTRAL (FACT, provada):** o motor de busca do Cosca tem **alta sensibilidade à colinearidade lexical** — funciona quando a query repete as palavras do doc (query literal "L34 Requisito do Don Continuidade de Memoria entre Sessoes kernel" → acha o chunk `e84c5a50` em 1º lugar, MRR 1.0), mas **falha em PARÁFRASE SEMÂNTICA** ("continuidade da memoria do kernel entre execucoes" → não acha nem no top-50). O embedding nomic-embed-text não captura semântica além da sobreposição de termos. **Lição: "motor de busca padrão ≠ robusto a linguagem natural" — o monastic é o baseline REAL (fraco), o modular recusa tudo (roteador fecha no design). Nenhum venceu por qualidade — a campanha cumpriu o papel de MEDIR, não de declarar vitória.** | Don's order — "da forma correta" |

## Confidence Trajectory

```
1.00 ┤                                    ╭── 0.88
0.80 ┤                                    │
0.60 ┤                                    ╰── 0.68
0.40 ┤                                    │
0.20 ┤──────╮                             │
0.00 ┤      │                             │
      Jul27  ─────────────────── Jul28 ──── Aug22
             9 commits, 150+ files    audit + 3 failures
```

## Notes

- Confidence dropped 0.88→0.68 because: (1) hallucination -0.10, (2) mandament violation -0.10, (3) false excuse -0.05, (4) successful audit +0.05
- Level maintained at 3: core orchestration capabilities still valid
- Recovery path: register failures honestly, update patterns, maintain cognitive-state freshness
