# Hermes Self-Evolution Patterns — GEPA, Fitness, Guardrails (aprofundado)

> **Version**: 1.1.0 | **Confidence**: 0.93 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/NousResearch/hermes-agent-self-evolution

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `evolution/` (skill_module, fitness, constraints, dataset_builder, config) e `PLAN.md`. **Aprofundamento** do `hermes-agent-patterns.md` (que cobre memória/SessionDB/trajectory) — aqui é a **camada de auto-evolução otimizadora** (GEPA/DSPy), o gap nº1 do Cosca: o Cosca *registra* learnings (stages 7-8), o Hermes *Evolui* o texto medindo com eval.

## Purpose

O Hermes Self-Evolution resolve o problema de **evoluir o texto de skills/prompts/tools como um módulo otimizável**, guiado por execution traces (GEPA), com **benchmarks como GATES (não fitness)** e **guardrails anti-bloat**. É o blueprint para transformar o estágio 7-8 do Cosca (que só registra) em auto-melhoria real (que otimiza e mede).

---

## 1. Texto-que-vira-Genoma (Skill/Prompt como módulo otimizável)
- **O que resolve**: tratar um artefato de texto (skill, prompt, descrição de tool) como um módulo que a evolução pode mutar e avaliar — sem retreinar pesos.
- **Como funciona**: `SkillModule(skill_text)` é um `dspy.Module` em que o corpo do texto é o **parâmetro otimizável**. No `forward(task_input)`, o texto é injetado como `skill_instructions` e o agente responde via `ChainOfThought`. O frontmatter YAML (`name`, `description`, metadata) é **preservado e travado**; só o **body** evolui (`reassemble_skill`). A evolução nunca reescreve a "identidade" do skill — só o procedimento.
- **Onde**: `evolution/skills/skill_module.py:84`, `:117` (`reassemble_skill`), aplicado a tool descriptions (Phase 2) e prompt sections (Phase 3).
- **Aplicação no Cosca**: para estágio 7-8, cada "artifact otimizável" (skill, trecho de prompt, template de tool) precisa de um wrapper análogo: um módulo com o **texto fatiado como parâmetro** e um `forward` que executa a capacidade e devolve resultado. Definir explicitamente o que é **trava** (schema, nome, assinaturas) e o que é **otimizável** (só o corpo).

## 2. Evolução Reflexiva Guiada por Execution Traces (GEPA)
- **O que resolve**: evolução que lê **por que** uma tarefa falhou, não apenas que falhou — e orquestra fitness barato durante a busca e caro apenas onde importa.
- **Como funciona**: GEPA é movido pelo *feedback* textual da avaliação. O `FitnessScore` é **multi-dimensional** (`correctness` 0.5 / `procedure_following` 0.3 / `conciseness` 0.2) e carrega `feedback: str` que o GEPA usa para propor mutações **mirando a causa**. O segredo de custo: `skill_fitness_metric` é um **proxy barato** (overlap de palavras) usado durante o loop; o **LLM-as-judge completo** (`dspy.ChainOfThought`) é usado seletivamente (holdout). Fallback automático para MIPROv2 se o GEPA não existir.
- **Onde**: `evolution/core/fitness.py:14`, `:34` (`LLMJudge`), `:107`; `evolution/skills/evolve_skill.py:156`, `:167`.
- **Aplicação no Cosca**: o eval oracle do Cosca deve expor **feedback textual rastreável** (não só score) que alimente a mutação — e a mutação deve ser "reflexiva" (olha o trace inteiro). Adotar a hierarquia de custo: correlato barato durante o loop, judge completo no holdout. Como a busca evolutiva faz centenas de avaliações, é isso que mantém o custo baixo.

## 3. Benchmarks são PORTÕES (gates), não funções de fitness
- **O que resolve**: a separação entre "melhorou a tarefa" e "não regrediu o sistema". É o anti-overfitting sistêmico.
- **Como funciona**: pipeline em funil com portões encadeados: `pytest` (gate 1 — correção funcional, 100% obrigatório) → TBLite subset rápido 20 tasks (gate 2) → **fitness exclusivo da tarefa** → só os **top-3** → TBLite full (gate 3) + YC-Bench fast_test (gate 4, coerência multi-turno) → melhor candidato vira PR. Princípio-chave: um variante que sobe 20% no skill mas cai 5% no TBLite é **REJEITADA**. O `tblite_regression_threshold: 0.02` (máx 2% regressão) materializa isso.
- **Onde**: `PLAN.md:630-663`, `evolution/core/config.py:40-43`.
- **Aplicação no Cosca**: ter um **eval oracle de regressão** separado do **eval de fitness da capacidade**. O primeiro é o portão que barra qualquer variante que quebre comportamento global; o segundo mede se a capacidade melhorou. Manter um threshold numérico de regressão; só "promover" candidatos que passem o oracle.

## 4. Corral de Restrições + Penalidade Anti-Bloat
- **O que resolve**: as barreiras que impedem a evolução de derivar para verbose/inválido/quebrado, e como tratar qualquer falha como rejeição imediata.
- **Como funciona**: `ConstraintValidator.validate_all` roda **toda** restrição; qualquer falha → variante descartada (salva em `evolved_FAILED.md` para inspeção). Restrições: **limite de tamanho** (skill ≤15KB, tool desc ≤500 chars, param ≤200), **limite de crescimento** (max +20% sobre baseline — combate bloat), **não-vazio**, **integridade estrutural** (frontmatter YAML válido). Dupla defesa no fitness: `length_penalty` rampa 0→0.3 quando uso >90% do teto. **Preservação semântica**: compara texto evoluído com original para não derivar de propósito. **Caching**: nunca hot-swap em conversa ativa — só vale em nova sessão. **Deploy sempre via PR**, nunca commit direto.
- **Onde**: `evolution/core/constraints.py:24`, `tests/core/test_constraints.py`, `fitness.py:91-96`, `PLAN.md:687-728`.
- **Aplicação no Cosca**: estágio 7-8 precisa de um guarda-corpos correlato: validadores por tipo de artefato, teto de tamanho/crescimento em % sobre baseline (anti-bloat), verificação de integridade estrutural, checagem de "drift semântico", e a regra de nunca trocar conteúdo em sessão ativa (só nova sessão). Adicionar a penalidade progressiva dentro do fitness para não desperdiçar avaliações em candidatos gordurosos.

## 5. Sourcing Hierárquico de Dataset de Eval + Cold-Start por Mineração de Ferramentas Externas
- **O que resolve**: de onde vêm os dados de avaliação — inclusive quando não há histórico próprio (cold-start).
- **Como funciona**: quatro fontes. **A) Sintética** (cold-start, LLM gera `task_input` + `expected_behavior` como *rubrica*, com difficulty/category); **B) SessionDB mining** (uso real, LLM-as-judge em 2 passos: pré-filtro heurístico por overlap de palavras → scoring LLM); **C) Golden curado** (JSONL manual, melhor qualidade, reservado a skills críticos); **D) Auto-eval específico de skill** (plantar bug e checar se a skill resolve). Fonte B resolve o cold-start **minando histórico de Claude Code/Copilot/Hermes** antes do agente ter histórico próprio. Todos os dados passam por **scrubbing de secrets** e divididos 50/25/25 train/val/holdout (anti-overfit). GEPA funciona com ~3 exemplos.
- **Onde**: `evolution/core/dataset_builder.py:89`, `:172`, `external_importers.py:45`, `:121`, `:606`.
- **Aplicação no Cosca**: o oracle de eval deve ter 4 fontes com precedência — sintética para bootstrappar qualquer capacidade nova, mineração de sessões reais (o Cosca TEM memória de sessões) com scrubbing de secrets, golden curado para capacidades críticas, e auto-eval para capacidades verificáveis. Holdout separado é o anti-overfit essencial.

## 6. Avaliação por Rubrica (expected_behavior, não texto exato) + Parsing Robusto
- **O que resolve**: pontuar qualidade subjetiva sem respostas douradas frágeis, e não quebrar o pipeline quando o LLM devolve texto sujo.
- **Como funciona**: o `expected_behavior` é uma **descrição do que um bom resultado deve fazer** ("deve identificar a SQL injection na linha 42"), nunca a string exata. O judge pontua em 3 eixos e devolve feedback acionável. Do lado da leitura: parsing robusto de JSON do LLM — tenta `json.loads` direto, e se falhar faz **contagem de chaves balanceadas** (brace-counting, que respeita `{edge} cases` dentro de strings). Clampa scores a [0,1] e default neutro 0.5 em parse failure.
- **Onde**: `evolution/core/fitness.py:41`, `:139`, `external_importers.py:546`, `:83`.
- **Aplicação no Cosca**: o eval oracle deve usar **rubricas descritivas** em vez de respostas exatas — torna o oracle reutilizável entre variantes e tolerante a variação semântica. Ter o mesmo **parsing defensivo** de saída de LLM (inevitável no estágio 7-8) e normalização/clamp de scores para nunca derrubar o loop por um parse falho.

## 7. Loop Contínuo: Auto-Triage + Crescimento Orgânico do Dataset + Deploy só via PR
- **O que resolve**: automatizar a auto-triação (o que melhorar agora) e fechar o ciclo fazendo o uso real alimentar os datasets — sem automatizar o deploy.
- **Como funciona**: **Performance monitor** rastreia: taxa de sucesso por skill, acurácia de seleção de tool, scores de benchmark ao longo do tempo, e **correções do usuário** como sinal. **Auto-triage** ranqueia alvos por `(impacto × frequência)` e prioriza skills em queda/alta falha. **Cron**: rodada semanal de benchmarks; quando score cai ou falha > threshold → dispara GEPA. **Regra de ouro da gov**: automatiza **detecção e otimização, nunca o deploy** — todo PR é mergeado por humano, commit em branch `evolve/<target>-<timestamp>`. **Feedback loop**: correções do usuário viram exemplos do dataset, sessões de alta qualidade viram exemplos positivos, sessões falhas viram casos de falha para análise reflexiva do GEPA.
- **Onde**: `PLAN.md:586-627`, `:705-728`, `evolution/core/config.py:41-47`.
- **Aplicação no Cosca**: para estágio 8, ter um **monitor de performance por capacidade** + auto-triage ranqueado por impacto×frequência, com gatilho por threshold, e um balde de memória/eval que absorve correções do usuário e sessões bem/mal sucedidas — tudo propondo mudanças (PR) sem auto-deploy. Cada run gastável e rastreável: commit/branch nomeado com metadata (antes/depois, holdout, custo, restrições rejeitadas), permitindo revert trivial.

---

## ⚠️ Nota de licença (importante pro Cosca)

O quartel do **Darwinian Evolver** (evolução de código, Phase 4) é **AGPL v3** e fica como CLI externa. Se o Cosca quiser evoluir código real, use o isolamento "só-cli-externa" para não contaminar a licença; para texto (skills/prompts/descs), MIT/GEPA é seguro.

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Valor |
|---|--------|-------|
| 1 | Texto-que-vira-Genoma (trava identidade, otimiza corpo) | virar a skill um módulo otimizável |
| 2 | GEPA reflexivo guiado por feedback | mutação que mira a CAUSA da falha |
| 3 | Benchmarks como GATES (fitness vs regressão) | evolução sem quebrar o sistema |
| 4 | Corral de restrições + anti-bloat | evolução segura em produção |
| 5 | Eval de 4 fontes + holdout | dataset orgânico, cold-start resolvido |
| 6 | Deploy só via PR | automatiza detecção+otimização, nunca deploy |

## Related Patterns

- [`hermes-agent-patterns.md`](hermes-agent-patterns.md) — camada de memória/SessionDB/trajectory (este aprofunda a evolução)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — skill registry (D1/D2)
- `internal/evals/ORACLE_SPEC.md` — o oráculo que serve de fitness
