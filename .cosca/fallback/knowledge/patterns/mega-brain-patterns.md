# Mega Brain Patterns — Conclave (deliberação), DNA Cognitivo, RAG Grounded, Orquestração

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: AI Agent Patterns | **Created**: 2026-08-23 | **Source**: https://github.com/thiagofinch/mega-brain

> **Mined by**: cosca-kernel (ordem do Don). Extraído de 4 batedores paralelos (agents/system/conclave, engine/jarvis+intelligence/pipeline/mce, engine/intelligence/rag, squads/orquestrador-global). Sistema de gestão de conhecimento por IA: ingestão (MCE) → DNA cognitivo em 10 camadas → RAG híbrido "zero achismo" → **Conclave** (conselho multi-agente que delibera estratégias fundamentado em evidências). Três lições estruturais: (1) **separar decisão de domínio da meta-cognição** (conselho NÃO tem DNA de domínio); (2) **"zero achismo" = evidência rastreável obrigatória**; (3) **planejar ≠ executar** (plan-only + executor DAG).

## Purpose

O Mega Brain resolve o problema de **transformar material bruto em conhecimento operável, consultável e deliberável** — com a feature hero **Conclave**: um conselho multi-agente (Crítico Metodológico, Advogado do Diabo, Sintetizador) que delibera decisões estratégicas fundamentado **somente** no conhecimento ingerido. O Cosca tem RAG/semantic-memory, critic e orquestração — mas não tem: (1) deliberação multi-agente com evidência rastreável e convergência calculada; (2) extração de "DNA cognitivo"; (3) gates de fidelidade anti-alucinação; (4) separação rígida plan-only vs executor.

---

## A. Conclave — deliberação multi-agente fundamentada (a feature hero)

### A1. Separação Estrutural Domínio (L2) vs Meta-Cognição (L3)
- **O que resolve**: o viés de confirmação do especialista — quem é dono do tema defende a própria resposta. O antídoto não é "mais especialistas", é alguém **sem** o dog.
- **Como funciona**: duas camadas com papéis opostos. Os **cargos** (CRO/CFO) têm DNA de domínio e respondem "O QUE fazer". O **Conselho** (Crítico, Advogado, Sintetizador) **NÃO tem DNA de domínio** e avalia "COMO raciocinaram". Regra inviolável: "CONSELHO NÃO TEM DNA DE DOMÍNIO... conhecimento de domínio vem dos CARGOS". O Crítico escora *processo*, não mérito — nunca opina sobre o tema, pontua se premissas foram declaradas e se a lógica é consistente. Ninguém é juiz e parte ao mesmo tempo.
- **Onde**: `agents/system/conclave/README.md`, `engine/jarvis/protocols/conclave/CONCLAVE-PROTOCOL.md:55-75,481-514`, `critic.md`.
- **Aplicação no Cosca**: o `cosca-critic` e um conselho cosca devem ser **sem-expertise-de-domínio** e escorar processo/meta-cognição. `cosca-backend`/`frontend` produzem; um conselho meta valida se o raciocínio foi robusto. Nunca deixar um executor revisar a própria classe de decisão.

### A2. "Zero achismo" como rastreabilidade de evidência obrigatória
- **O que resolve**: alucinação e decisões por intuição sem base.
- **Como funciona**: (a) regra do debate: "Toda afirmação deve citar DNA (ID). Sem evidência = opinião, não posição fundamentada" — citação `HEUR-AH-025`, `[RAG:chunk_id]`, `MEMORY:source:chunk`. (b) Constituição (Empirismo) obriga citar FONTES e NÚMEROS; violação é sinalizada. (c) O RAG é **só interno** (máx 5 queries, timeout 15s); busca web externa **PROIBIDA**. O Crítico pontua "Evidências com IDs rastreáveis".
- **Onde**: `DEBATE-PROTOCOL.md:261-266,286-327`, `CONCLAVE-PROTOCOL.md:181-204`, `critic.md:33-36`.
- **Aplicação no Cosca**: toda posição deve carrear um ID de evidência citável (chunk/embedding ID da base RAG). Posição sem ID → rejeitar ou rebaixar confiança. `cosca-qa`/`critic` escoram "evidência rastreável"; `cosca-cto` jamais recomenda por "melhor prática" sem fonte ingerida.

### A3. Convergência calculada + circuit breaker (anti-consenso forçado)
- **O que resolve**: loops infinitos de debate e consenso artificial.
- **Como funciona**: convergência NÃO é declarada, é **calculada**: `Σ(peso × concordância)` sobre critérios ponderados (recomendação 0.30, premissas 0.25, riscos 0.25, timing 0.20), threshold 70%. Convergiu → síntese; não → Rodada 2 (rebatidas cruzadas) → Rodada 3 (só divergências). Se ainda <70%, **circuit breaker** (max_rounds=3, max_iterations=5, timeout=300s, **detecção de loop por hash das posições** — mesmo hash 2× = loop). Tensões produtivas são features, não bugs — divergência explícita > consenso artificial.
- **Onde**: `DEBATE-DYNAMICS-CONFIG.yaml:38-61,92-122`, `DEBATE-DYNAMICS-PROTOCOL.md:463-498`.
- **Aplicação no Cosca**: `cosca-orchestrator` calcula convergência numericamente + circuit breaker rígido (max N rodadas + hash de posições). Preservar divergência não-resolvida como tensão registrada.

### A4. Calibração de confiança aritmética + thresholds de emissão
- **O que resolve**: confiança inflada por coragem; viés de "decisivo" que esconde risco.
- **Como funciona**: confiança é cálculo explícito com breakdown auditável: `base (convergência) ± ajustes`. Ajustes tipados: Crítico <70 → −20%; risco Alta/Catastrófico → −15%; Alta → −10%; Média → −5%; contradição não resolvida → −10%; evidência em 3+ fontes → +10%. Anti-pattern AP-4: "Confianca: Alta" PROIBIDO. **Thresholds**: ≥70% emitir; 50-69% emitir com ressalvas + plano de mitigação; <50% não emitir, escalar humano (sem re-rodar — anti-loop). O Sintetizador dá critérios de reversão (`SE X ENTAO reconsiderar`).
- **Onde**: `sintetizador-conclave.md:97-145`, `CONCLAVE-PROTOCOL.md:289-315`, `synthesizer.md:68-90`.
- **Aplicação no Cosca**: substituir "acho que está bom" por fórmula transparente com breakdown + thresholds EMITIR/EMITIR-COM-RESSALVAS/ESCALAR. Block `cosca-cto` de emitir abaixo do threshold.

### A5. Votação cruzada sem auto-voto + debate adversarial com juiz-relay
- **O que resolve**: autocorroboracão e agregação cega de notas.
- **Como funciona**: Battle 5 fases: briefing → produção paralela isolada → **votação cruzada** (cada votante avalia SÓ os outros; auto-voto PROIBIDO; score = Σ(score×peso)) → Top 2 viram Defensor/Desafiante, Chief é **Juiz**; debate em 3 rodadas (apresentação→ataque→síntese), toda comunicação passa pelo Chief como relay (contendores não falam direto). Veredito YAML: `winner, score, must_fix, nice_to_have, judge_notes`. Margem ≤5% → tiebreaker. Board de revisores fecha com gate `unanimous`/`majority`, máx N rodadas, escalação `human`/`force_approve`.
- **Onde**: `squads/orquestrador-global/workflows/battle-round.md:177-341`, `templates/battle-logging/04-voting-results.md:48-51`.
- **Aplicação no Cosca**: auto-voto-proibido (revisores avaliam os outros) + debate com juiz-relay neutro. `cosca-specialist-review-code`/`cosca-qa` são candidatos; o Orchestrator faz o relay e emite veredito com `must_fix`.

### A6. Revisão adversarial por arquétipos + FMEA (RPN)
- **O que resolve**: pontos cegos sistêmicos; priorizar risco por matemática, não opinião.
- **Como funciona**: 5+1 camadas de detecção. A mais transferível é a **Revisão Adversarial**: 3 adversários arquetípicos — **Crítico Técnico** (erros factuais), **Competidor** (inferioridade), **Usuário Frustrado** (pior recepção) — gerar 5-10 ataques de cada e verificar se o processo previne. **Taxonomia Universal de Defeitos (D1-D10)** classifica o tipo de falha. **FMEA**: `RPN = Severidade × Ocorrência × Detectabilidade` (RPN>200 urgente, 100-200 alto, 50-100 médio). **Pre-mortem prospectivo** ("é data+90d, o projeto falhou — por quê?").
- **Onde**: `BLIND-SPOT-DETECTION-FRAMEWORK.md:13-38,272-334`, `devils-advocate.md:46-113`.
- **Aplicação no Cosca**: `cosca-critic` instancia os 3 arquétipos + prioriza com FMEA RPN; transformar cada ponto cego em checkpoint verificável; pre-mortem em todo plano do Orchestrator/CTO.

### A7. Síntese como SPEC estruturado + anti-padrões verificados programaticamente
- **O que resolve**: deliberação que degenera em prosa e não vira ação auditável.
- **Como funciona**: Sintetizador NÃO produz resumo narrativo ("Baseado nos achados..." é PROIBIDO — AP-1). Emite **Synthesis Spec** com 5 seções: `[FINDINGS]`, `[CROSS-AGENT ANALYSIS]` (convergência/divergência/lacunas/delta de confiança), `[SPEC]` (por item: file + Action(imperativo) + Metric + Acceptance), `[ACTIONS]`, `[CONFIDENCE]`. Anti-patterns enumerados (AP-2 refs vagas, AP-3 pular cross-agent, AP-4 confiança intuitiva, AP-5 ação vaga, AP-6 sem critérios de reversão). Validado por `validate_spec()` — se falta seção ou anti-pattern, o output é rejeitado. Incorporação de feedback obrigatória (gap endereçado OU justificado).
- **Onde**: `sintetizador-conclave.md:30-39,121-145,240-249`, `agents/system/conclave/sintetizador/agent.md`.
- **Aplicação no Cosca**: exigir SPEC acionável (arquivo + ação + métrica + acceptance), nunca resumo narrativo. `cosca-documentation`/`qa` validam anti-patterns; `cosca-workflow-chief` converte SPEC/ACTIONS em pipeline.

---

## B. DNA Cognitivo + Pipeline MCE (extração de conhecimento)

### B1. DNA Cognitivo em Camadas L1-L10 — separar "como pensa" de "o que sabe"
- **O que resolve**: o maior desperdício em KBs é armazenar conteúdo em vez de *estrutura de pensamento*.
- **Como funciona**: eixo 1 **DNA Cognitivo (L1-L5)**: `filosofias` (crenças sem número), `modelos-mentais` (lentes de percepção), `heurísticas` (regras com threshold), `frameworks` (esqueletos sem ordem), `metodologias` (processos com ordem rígida). Eixo 2 **DNA de Identidade (L6-L10)**: `behavioral_patterns`, `values_hierarchy` (Tier 1/2), `voice_dna`, `obsessions` (só 1 MASTER/pessoa), `paradoxes`. Princípio: **extração exaustiva 1x, uso seletivo Nx** (economia de tokens no consumo, não na extração).
- **Onde**: `engine/jarvis/dna.yaml`, `DNA-EXTRACTION-PROTOCOL.md`, `agent_generator.py:72-86`.
- **Aplicação no Cosca**: `knowledge-extraction` modela cada fonte como 2 artefatos: "insight" (factual) + "perfil cognitivo" (camadas). Permite gerar agentes/especialistas a partir de DNA.

### B2. Classificação por Sinais Discriminativos + "Classification Pressure"
- **O que resolve**: ambiguidade de rotulagem entre camadas adjacentes (filosofia vs heurística; framework vs metodologia).
- **Como funciona**: cada camada tem um discriminador. **Filosofia ≠ Heurística**: com número/threshold → heurística; sem número e com "sempre/acredito" → filosofia. **Framework ≠ Metodologia**: ordem rígida → metodologia; esqueleto → framework. Heurísticas **quantitativas** priorizadas. **Classification Pressure**: extrator esgotar as 10 camadas antes de cair em `unmapped_observations` (máx 3; `unmapped_ratio > 0.20` → QUALITY-WARNING) — anti-lazy-routing.
- **Onde**: `DNA-EXTRACTION-PROTOCOL.md:558-642`, `insights.prompt.md:16-47`, `insights-state.schema.json:41-57`.
- **Aplicação no Cosca**: taxonomia com discriminadores objetivos + gate de "não classificável" limitado e rastreável, em vez do LLM rotular livremente.

### B3. Peso + Genealogia: portão de uso e proveniência imutável
- **O que resolve**: conhecimento sem origem confiável é ruído; citações livres geram alucinação.
- **Como funciona**: **Peso = f(evidências)**: base 0.50; +0.15 citação com chunk_id confirmado; +0.10 threshold numérico; −0.20 se inferido; peso ≥0.70 → citável; <0.70 → só enriquecimento interno. **Genealogia obrigatória**: todo item carrega `insight_origem` + `chunk_ids` + `source_id` + `evidências[]` (verbatim, nunca modificada). Insight sem chunk_id é descartado.
- **Onde**: `DNA-EXTRACTION-PROTOCOL.md:319-393`, `canonical-map.schema.json`, `insights-state.schema.json:64-74`.
- **Aplicação no Cosca**: todo item de conhecimento carrega score de confiança + âncora de proveniência; recuperação prioriza por peso; auditoria reverte até o chunk original.

### B4. Pipeline MCE de Ingestão: cascata progressiva com checkpoints como guardas
- **O que resolve**: encadear extração de conhecimento custosa/irreversível sem falha silenciosa.
- **Como funciona**: 12 passos (`INGEST → BATCH → CHUNK → ENTITY RESOLUTION → INSIGHT EXTRACTION → MCE-1 behavior → MCE-2 identity → MCE-3 voice → CONSOLIDAÇÃO → FINALIZE → REPORT`). Cada passo tem **checkpoints `pre`/`post`** (192 no registry), `deterministic` vs `llm`, `blocking: true|false`, `threshold`, desfecho `⛔ PARAR`/`⚠️ WARN`. `blocking` impede avanço. Checkpoint humano (`CP-9.0.B` exige APPROVE).
- **Onde**: `pipeline/mce/mce_checkpoints.yaml`, `prompt-mce-{behavioral,identity,voice}.md`, `orchestrate.py`.
- **Aplicação no Cosca**: pipeline de ingestão como state machine com gates `pre/post`, `blocking`, thresholds, num registry versionável.

### B5. Cascata de Granularidade: chunk → insight → padrão → identidade → voz
- **O que resolve**: perda de contexto ao pular direto do texto bruto para síntese.
- **Como funciona**: `chunk` → `insight` (0-3/chunk) → `behavioral pattern` (precisa 2+ chunks p/ HIGH) → `value/obsession/paradox` (multi-evidence: MASTER exige ~50% dos insights + conexão com todos Tier-1) → `voice` (6 dimensões tonais 0-10, signature phrases occurrence_count≥2). Regra: "identidade é inferida de múltiplas evidências, não de uma única frase".
- **Onde**: `prompt-mce-behavioral.md:43-45`, `prompt-mce-identity.md:53,155-170`, `prompt-mce-voice.md:113-213`.
- **Aplicação no Cosca**: KB como pirâmide de granularidade, cada camada com requisito mínimo de recorrência/evidências.

### B6. Aprendizado Append-Only + Idempotente (merge por dedup)
- **O que resolve**: re-processamento incremental sem duplicar nem quebrar o que existe.
- **Como funciona**: escrita de YAMLs L1-L5 é **append-only e idempotente**: dedup por `name` normalizado (lowercase+strip+50 chars); match → **union** de `source_insight_ids`/`chunk_refs` sem sobrescrever outros campos; sem match → append com `{pref}_NNN`; nunca deleta; rodar 2x = saída idêntica.
- **Onde**: `pipeline/mce/dna_regenerator.py:76-78,166-264`, `migrate_identity_layers.py`.
- **Aplicação no Cosca**: operações de conhecimento como *merge* (append-only, union, dedup), nunca overwrite destrutivo.

### B7. Agente como Projeção Compilada do DNA → SOUL/ACTIVATION
- **O que resolve**: transformar conhecimento acumulado em especialista invocável com voz e gatilhos.
- **Como funciona**: `agent_generator.py` lê YAMLs L1-L10 + CONFIG + dossier e **compila** 4 artefatos: `AGENT.md` (DNA SUMMARY + MCE), `SOUL.md` (narrativa 1ª pessoa — "Identidade Viva"), `MEMORY.md`, `DNA-CONFIG.yaml`. **Degradação graciosa** (sem MCE, output = v1). `activation_generator.py` registra `nickname/archetype` → tom default e assinatura de fechamento.
- **Onde**: `engine/intelligence/agents/agent_generator.py:977-1056`, `activation_generator.py:54-226`.
- **Aplicação no Cosca**: cada perfil compila para um "especialista" consultável (SOUL/AGENT) consumindo os mesmos YAMLs canônicos — evita duplicação KB↔agentes.

---

## C. RAG Híbrido Grounded + Gates de Fidelidade

### C1. Fusão Híbrida RRF com Re-score Cosine no Espaço Ativo
- **O que resolve**: recuperar por semântica E termos sem que lexical domine; ranking estável e comparável.
- **Como funciona**: `rrf_retrieval.py` faz RRF (k=60) sobre denso (pgvector) e esparso (BM25); **normaliza RRF para [0,1]** (divide por maxScore — monotônico, resscala sem reordenar). Pós-fusão, **blend `0.7·rrf + 0.3·cosine`** antes do corte (chunk semântico sobe). Cada resultado carrega `chunk_id`, `score`, `source_path`, `person`, `section`, `metadata`, **bucket-scoped**. `hybrid_query.py` adiciona **dedup 4 camadas**, **reranker cross-encoder** (só head top-30 com "tail recall-preserving") e **strategic_order** (mais relevante no início E fim — mitiga "lost in the middle").
- **Onde**: `engine/intelligence/rag/rrf_retrieval.py:69-188`, `hybrid_query.py:511-700`.
- **Aplicação no Cosca**: `cosca-semantic-memory` adota RRF+cosine resscore normalizado; `cosca-rag` pela "tail recall-preserving" + strategic_order.

### C2. Invariante de Espaço Único de Embedding + Quarentena por Assinatura
- **O que resolve**: o "achismo" silencioso de vetores em espaços divergentes (dimensão A vs B) que corrompe o cosine.
- **Como funciona**: um único gateway canônico (`embedding_config.py`) resolve `provider:model:dim`. Todo vetor carrega assinatura `model:dim`. Ao reusar vetores antigos, `_load_prior_vector_map` faz **quarentena por assinatura**: assinatura on-disk ≠ ativa → descarta TODO o mapa prior (re-embed completo). Reuso exige `content_sha` + assinatura.
- **Onde**: `hybrid_index.py:464-528,884-1001`, `embedding_config.py`, `pgvector_store.py:311-393`.
- **Aplicação no Cosca**: manter assinatura `model:dim` + quarentena de reuso; gate de cache por `content_sha`+assinatura.

### C3. Fail-open como Invariante Estruturante (best-effort nunca derruba busca)
- **O que resolve**: camada de melhoria (expansão/rerank/síntese) nunca quebra a resposta.
- **Como funciona**: `query_expansion.expand_query` → qualquer falha retorna `[query]` original (duplo try/except). `apply_reranker` → fail-open à ordem RRF, nunca lança, loga por hash de query. `answer_builder` → falha → degrade para resposta extrativa. Contrato com kill-switch por env (`MCE_RERANKER_ENABLED`) com precedência.
- **Onde**: `query_expansion.py:147-266`, `hybrid_query.py:387-505`, `answer_builder.py:127-228`.
- **Aplicação no Cosca**: "expansão só soma recall, nunca derruba retrieval"; kill-switch env com precedência p/ feature-gates.

### C4. Cascade de Gates de Fidelidade: self-RAG → HHEM → block/flag
- **O que resolve**: evitar alucinação sem custo e sem falsos positivos.
- **Como funciona**: `pipeline.verify()`: (1) `self_rag.verify_response` — heurístico ~1ms zero-LLM (extrai claims, factual/speculative, token-overlap + bônus n-grama/número, `faithfulness = supported/total`); (2) `hhem_gate.check_response` — só quando `faithfulness < 0.60`: NLI modelo local HHEM-2.1-Open (~400MB, zero custo API); (3) `pipeline.attach_verification` — decisão block vs flag no **caller**: `0 ≤ faithfulness < threshold` (0.60) → **BLOCK** (delivered:false, placeholder seguro, original em blocked_response p/ auditoria); senão FLAG. `faithfulness == -1` nunca bloqueia (fail-open no scorer, fail-closed só em score real baixo).
- **Onde**: `pipeline.py:160-381`, `self_rag.py:52-280`, `hhem_gate.py:93-173`.
- **Aplicação no Cosca**: cascata heurística barata + NLI condicional + threshold de bloqueio no chamador. "Nunca bloqueie por ausência de evidência, só por evidência positiva de baixa fidelidade".

### C5. Atribuição no Nível de Claim + Citação Renderizada + "Sem Evidência Não Gera"
- **O que resolve**: cada afirmação rastreável a uma fonte — o núcleo do "zero achismo".
- **Como funciona**: `self_rag.extract_claims` quebra em sentenças com `has_citation` (`[RAG:...]`); `verify_claim` retorna `best_chunk_idx`, `matching_terms`, `score`; resposta verificada anexa `source_chunk` por claim. `build_rag_context` monta bloco com prefixo `[RAG:{chunk_id}]` + lista `sources`. `answer_builder` injeta chunks como `[Chunk N]` e **recusa gerar sem evidência** (AC2): sem chunks → `None` → extrativo, nunca deixa o LLM inventar de conhecimento paramétrico.
- **Onde**: `self_rag.py:52-100`, `hybrid_query.py:980-1035`, `answer_builder.py:105-228`.
- **Aplicação no Cosca**: `cosca-semantic-memory`/`rag` com atribuição por claim + renderizar citação; "sem evidência → não gera" é regra de citação obrigatória.

### C6. Gabarito Congelado + Gate de Correção Bloqueando CI
- **O que resolve**: impedir que mudanças no retrieval regridam recall imperceptivelmente.
- **Como funciona**: `qrels-baseline.json` é gabarito congelado (query, relevant_chunk_ids, first_relevant_chunk_id). `eval_gate.run_gate` calcula `recall@K`, `first_relevant_hit`, `expected_top1_hit`, `nDCG@k`, **falha o processo se cair abaixo do piso**. Postura fail-closed: query que lança é `errored` e vira breach (descartar query fácil inflaria a nota). `IDBasedContextPrecision` zero-LLM por interseção de chunk_id.
- **Onde**: `eval_gate.py:161-644`, `ragas_evaluator.py:31-237`, `qrels-baseline.json`.
- **Aplicação no Cosca**: `cosca-qa` com gabarito congelado + gate `queries_errored>0 ⇒ fail` antes de mudar o `cosca-rag`. nDCG premia posição.

### C7. Graph/Ontology: Hyperedge Lossless + Lei do Backlink + Evidência Atômica + Saúde da Base Viva
- **O que resolve**: linkar conhecimento com rastreabilidade e impedir que a base envelheça.
- **Como funciona**: `graph_builder` constrói grafo por bucket (isolamento). Relação N-ária **lossless via `Hyperedge`** (identidade = set ordenado de participantes); star-expansion O(N) é o view do PPR (walker nunca editado). Toda `Edge` carrega `atomic_facts` (frases-fonte verbatim que justificam), `t_start/t_end`. **Lei do Backlink** bidirecional. `ontology_layer` ancora na hierarquia FILOSOFIA→MODELO_MENTAL→HEURISTICA→FRAMEWORK→METODOLOGIA com travessia direcional e decaimento `0.8^(depth-1)`. **Saúde** (BrainHealth 6-métrica): staleness, chunks órfãos, dead_links, consistência de dimensão, invalida cache de RRF.
- **Onde**: `graph_builder.py:324-460,605-656`, `ontology_layer.py:133-268`, `health_check.py:45-506`.
- **Aplicação no Cosca**: hyperedge N-ário + `atomic_facts` (trilha de auditoria "zero achismo") + BrainHealth 6-métrica p/ `cosca-monitoring`.

---

## D. Orquestração Multi-squad + Maturidade

### D1. Pipeline de Orquestração em Cadeia — "Plan-only" separado da Execução
- **O que resolve**: o acoplamento entre "decidir" e "fazer" (orquestrador virar executor perde auditoria).
- **Como funciona**: `plan-architect` é o **único ponto de entrada** e **nunca executa** — só emite o plano. Cadeia explícita `L1 intent-parser → L2 capability-cartographer → L3 roteador → L4 dag-architect → L5 validate/audit (determinísticos) → L6 handoff`. Três defesas: flag `plan_only: true`, hook `pre-execution-block.sh`, lista "o que NUNCA fazer". Output: `plan.yaml/md/json` + `audit.jsonl` + `plan-registry.yaml` com `success_criteria` e `falsifiable_assumptions`.
- **Onde**: `squads/orquestrador-global/agents/{plan-architect,dag-architect}.md`, `squad.yaml`, `agent-registry.yaml`, `docs/reference/forca-total-orchestration.md`.
- **Aplicação no Cosca**: planejador (`plan-architect`) que emite DAG + custos + riscos para aprovação, e executor separado que consome o plano. `plan_only` + gate humano obrigatório.

### D2. Registry Canônico + Capability Cache (discovery filesystem-first)
- **O que resolve**: o orquestrador saber o que existe sem alucinar capacidades nem varrer a cada demanda; sobreviver a renomes.
- **Como funciona**: registros declarativos espelhados (`agent-registry.yaml`, `squad.yaml`, `config.yaml`) com `tier`, `plan_only`, `expertise`, `best_for`, `legacy_aliases` (back-compat após rename). `capability-cartographer` é **cache-first** (`capability-cache.json`, TTL 3600s), só invoca `scan-capabilities.js --force` se stale. Classifica 11 categorias. Princípio P2: "nunca alucina capability".
- **Onde**: `squad.yaml`, `agent-registry.yaml`, `agents/capability-cartographer.md`, `scripts/scan-capabilities.js`.
- **Aplicação no Cosca**: registry de agentes como fonte de verdade p/ roteamento (tiers + best_for); cartógrafo/índice cacheado determinístico; aliases p/ back-compat.

### D3. Roteamento por Intenção — Score Ponderado + Thresholds Multi-Banda + Elicitation Gate
- **O que resolve**: decidir para onde mandar cada demanda sem as cegas nem perguntar demais.
- **Como funciona**: `intent-parser` produz 8 campos com `confidence ∈ [0,1]`; se <0.7, **elicitation inline** (info-gain, máx 3 perguntas). `roteador` calcula match multi-componente (domínio 40%, problemas 35%, tipo 15%, keywords 10%) e decide por faixas: ≥0.80 roteia direto; 0.60-0.79 confirmação humana; <0.60 escala/cria squad. `RoutingDecision v2` canônico (`decision_id`, `selected.primary_executor`, `selected_by`).
- **Onde**: `agents/{intent-parser,roteador}.md`, `data/intent-taxonomy.yaml`, `data/scoring-weights.yaml`.
- **Aplicação no Cosca**: classificador de intenção com confidence + elicitação limitada; roteador com pesos + faixas + escalada humana como padrão.

### D4. DAG Architect + CPM + FMEA — Planejamento Determinístico e Escalonável
- **O que resolve**: transformar demanda em grafo de execução ótimo de forma reproduzível.
- **Como funciona**: `dag-architect` decompõe → nós de capability → arestas por output→input → `parallelizable_with` → **Kahn's topological sort** (ciclo obrigatório) → **CPM** (forward/backward, slack, `critical_path`) → **FMEA RPN por nó**. Loop-nodes anotados via `classify-loop-node.js` (verb-shape/verificação/terminação): high-confidence → `execution_kind: loop`; ambíguo → `loop_candidate: true` e **nunca auto-tipa** (gate humano Class-C). `validate-plan.js` valida schema/ciclo/constitucional.
- **Onde**: `agents/dag-architect.md`, `scripts/{classify-loop-node,validate-plan}.js`, `checklists/dag-validation-checklist.md`.
- **Aplicação no Cosca**: arquiteto de DAG que emite grafo com caminho crítico + risco, validado por script; gate humano p/ nós ambíguos.

### D5. Modelo de Maturidade de Orquestração (0→1 / 1→10 / 10→100)
- **O que resolve**: avaliar/evoluir o sistema de orquestração progressivamente.
- **Como funciona**: 3 estágios — **Single Router (0-to-1)**, **Pipeline Orchestrator (1-to-10)**, **Autonomous Orchestration Engine (10-to-100)** — com goals e métricas (0→1: routing accuracy>80%; 1→10: >90% + gates manuais+auto; 10→100: >95% + self-healing). Espinha ontológica Demanda→Classificação→Orquestração→Gate Review→Consolidação→Melhoria. **Mode-based behavior**: `SIMPLE/STANDARD/COMPLEX/CRITICAL` (modo altera gates, pre-mortem, roundtable, require_human_signoff), por complexity e escala.
- **Onde**: `ORCHESTRATION-MATURITY-FRAMEWORK.md`, `ORCHESTRATION-PLAYBOOK.md`, `config.yaml`.
- **Aplicação no Cosca**: roadmap explícito de maturidade + "modo" por complexidade — simples leve, crítico com gates/roundtable humanos.

### D6. Quality Gate em 3 Estados (APPROVE/REVIEW/VETO) com Veto Conditions Hard-Stop
- **O que resolve**: impedir outputs ruins de propagar sem bloquear tudo.
- **Como funciona**: `data/quality-gates.yaml`: cada gate define `phase`, `metric`, 3 score-boards — `approve_score` (auto-aprovar), `review_score` (faixa de revisão → request_rework), `veto_score` (abaixo → block/revert). `veto_conditions[]` são **HARD STOPS** (critical_error, schema_violation, pii_leakage, authorized_mutation). Config de gates escala com a demanda (QUICK sem gates; EPIC após cada squad). Cada gate tem `owner`.
- **Onde**: `squads/orquestrador-global/data/quality-gates.yaml`, `ORCHESTRATION-PLAYBOOK.md`.
- **Aplicação no Cosca**: gates de qualidade como pontos de revisão entre fases com 3 estados + condições de veto duras p/ segurança.

### D7. Executor de DAG Paralelo + Job Queue Durável Token-Fenced + FSM com Resume
- **O que resolve**: executar DAG paralelo seguro; dois workers nunca processam o mesmo; pipeline retoma de onde parou.
- **Como funciona**: **`DAGExecutor`**: `get_ready_steps()`, `asyncio.gather` sob `Semaphore(max_parallel=3)`, particiona safe/unsafe (`is_concurrency_safe` — inseguros serial sob `_write_lock`), pré-checa `PolicyLimiter`, isola erro, mede `speedup_ratio`. **`JobQueue`**: fila durável Postgres com **token-fencing** (`FOR UPDATE SKIP LOCKED`, `lock_token = pid:time_ns`), Heartbeat TTL/2, **reaper** distinguindo `stall` (lease expirou → requeue com stalled_counter) de `timeout` (wall-clock com lease viva → dead direto, sem requeue). **`PipelineStateMachine`**: FSM de 18 estados persistida, distingue `resume_from_pause` de `recover_to_last` (crash).
- **Onde**: `engine/intelligence/pipeline/mce/dag.py`, `dag_executor.py`, `job_queue.py`, `state_machine.py`.
- **Aplicação no Cosca**: runner DAG com teto de concorrência + steps não-thread-safe separados; fila durável com fencing token (banco como árbitro); FSM distinguindo pausa de crash.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Gap | Aplicação no Cosca |
|---|--------|-----|--------------------|
| 1 | A1/A2 — conselho sem DNA de domínio + evidência rastreável obrigatória | deliberação fundamentada | `cosca-critic` meta-cognitivo + posição exige ID de evidência |
| 2 | A3/A4 — convergência calculada + confiança aritmética + thresholds de emissão | tomada de decisão anti-consenso | `cosca-orchestrator`/`cto` com fórmulas + escalar humano |
| 3 | C4/C5/C6 — cascata de fidelidade (self-RAG→HHEM→block) + atribuição por claim + gabarito congelado | RAG "zero achismo" | `cosca-rag`/`semantic-memory`/`qa` |
| 4 | D1 — plan-only separado da execução | orquestração auditável/sem abuso | `plan-architect` + executor separado |
| 5 | B1/B6 — DNA cognitivo + append-only idempotente | conhecimento estruturado sem duplicar | `knowledge-extraction` merges idempotentes |
| 6 | D7 — token-fencing + FSM resume | execução durável segura | `cosca-workflow-chief`/runtime |
| 7 | C2 — invariante de espaço único de embedding + quarentena | embeddings divergentes corrompem busca | `cosca-semantic-memory` |
| 8 | A7 — síntese como SPEC acionável (file+action+metric+acceptance) | deliberação vira ação | `cosca-documentation`/`workflow-chief` |

## Known Uses (referência)

- `thiagofinch/mega-brain` — gestão de conhecimento por IA: ingestão MCE + DNA cognitivo + RAG híbrido + Conclave multi-agente.

## Related Patterns

- [`ruflo-patterns.md`](ruflo-patterns.md) — memória self-learning + capability inventory
- [`hermes-self-evolution-patterns.md`](hermes-self-evolution-patterns.md) — benchmarks como gates + self-evolution
- [`kubernetes-org-patterns.md`](kubernetes-org-patterns.md) — gate de regressão + observabilidade de estado
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — session durável + gates de sistemas
