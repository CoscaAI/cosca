PREV: 1eeee15099cd32e20c54925d1b74e75442767102356281d35ecb38ba257f2bcf
ID: L145
TIME: 2026-08-13
LEVEL: 4
TAGS: #mineracao #hermes #self-evolution #dspy #gepa #benchmarks-as-gates #memory-nudges #sessiondb #trajectory-compression #level-4
---
## L145 — 2026-08-13 — Mineração Hermes Agent: self-evolution (DSPy+GEPA) — o gap nº1 do Cosca é OTIMIZAR, não registrar | Level 4

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Don's order "sim extrair tudo de bom de la" — minerar o Hermes Agent (Nous Research, 230k stars) e extrair os padrões de self-improvement real. |
| **Technique** | Level 4 — **(1)** Clone raso de hermes-agent (4.091 .py) + hermes-agent-self-evolution. **(2)** Task tool falhou (erro de DB na infra do subagent) → mudei para mineração MANUAL: li `PLAN.md`/`README.md` (self-evolution, 781 linhas) + `hermes_state*.py` + `trajectory_compressor.py`. **(3)** Consolidação em `hermes-agent-patterns.md` (9 padrões): A=Self-Evolution (GEPA, skill-as-module, tier por risco, benchmarks-as-GATES, 5 guardrails, fontes de dataset, auto-triage, "operates ON not inside"), B=Memória (SessionDB FTS5+compressão, memory nudges, Honcho), C=Trajetória (compression, toolsets/backends/gateway). **(4)** Registro no INDEX (Enterprise → 10). |
| **Level** | 4 |
| **Outcome** | success — 9 padrões extraídos. O gap nº1 do Cosca identificado e documentado: **o Cosca REGISTRA learnings (stages 7-8), o Hermes OTIMIZA o texto (skills/prompts/código) via DSPy+GEPA com fitness medido**. A lição mais profunda: **benchmarks são GATES, não fitness** — fitness é task-específica (oráculo), benchmark garante não-regressão. |
| **Confidence** | 0.92 (validado: fontes reais do repo, padrões cruzados com o que o Cosca já tem) |
| **Tags** | #mineracao #hermes #self-evolution #dspy #gepa #benchmarks-as-gates #memory-nudges #sessiondb #trajectory-compression #level-4 |
| **Related** | L242 (oráculo = fitness), L235 (spec oracle), L192 (Neural Link desacoplado), comparação Cosca vs Hermes |
| **Learned** | **(1) A diferença fundamental: Cosca REGISTRA (learnings.md, stages 7-8), Hermes OTIMIZA (DSPy+GEPA evolui o TEXTO da skill/prompt e mede com eval). O gap nº1 é transformar o oráculo que construímos (L242) em FITNESS de uma evolução — GEPA-equivalente que muta a skill, avalia no oráculo, seleciona a melhor.** **(2) "Benchmarks são GATES, não fitness" é a lição mais profunda: fitness = tarefa-alvo melhorou? gate = não quebrou o resto? O Cosca tem o oráculo (fitness) mas não tem o gate de regressão amplo.** **(3) "Operates ON, not inside" (self-evolution em repo separado que abre PRs) confirma a decisão do Neural Link desacoplado (L192) — a evolução do Cosca deve ser um módulo externo, não mexer no kernel.** **(4) Memory nudges (cutucar o agente para persistir) fecha o loop que o Cosca tem aberto: o auto-evolution depende do LLM LEMBRAR de registrar; o nudge é o gatilho periódico que garante.** **(5) Trajectory compression (proteger pontas + comprimir meio + resumo único) é mais preciso que o condensation atual do Cosca.** **(6) SessionDB mining (minerar o próprio histórico → dataset de eval via LLM-as-judge) transforma o knowledge.db/learnings em dataset orgânico — o Cosca TEM o histórico, não usa como eval.** **(7) Resiliência de processo: quando a task tool (subagent) falha, minerar MANUALMENTE (ler os arquivos-chave) entrega o resultado — não depender de um único caminho. |
| **Next** | (1) Implementar a EVOLUÇÃO do Cosca: GEPA-equivalente que usa o oráculo (L242) como fitness para evoluir skills/prompts — o salto de "registra" para "otimiza". (2) Adicionar benchmarks-as-gates (gate de regressão amplo) ao eval. (3) Memory nudges (gatilho periódico de auto-persistência). (4) A "coisa muito top": desenhar com self-evolution + gradiente contínuo desde o início. |
