# ADR-043: Incorporação Adaptada de Padrões do Hermes Agent (Nous Research) — decisões de design COSCA-shaped

> **Status:** APROVADO (decisão do Don 2026-09-08 — "marcha") | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-08
> **Natureza:** ADR de discussão/decisão. **NÃO** cria código. Consolida a análise comparativa do repositório externo `github.com/nousresearch/hermes-agent` (clonado em `%TEMP%\opencode\hermes-agent`, ~12.195 arquivos) contra o Cosca e propõe **adaptações** (nunca transplante) das ideias que fecham gaps reais.
> **Princípio (ordem do Don):** "buscamos a ideia e incorporamos da melhor forma" — incorporar adaptado ao nosso desenho (chain Ed25519, jail/fail-closed, hierarquia Don→Kernel→Chiefs, conhecimento curado, memória com guard/integrity), nunca transplantar.
> **Relação:** estende/complementa **ADR-014** (auto-recovery) e **ADR-031** (token efficiency); dialoga com **ADR-013** (revisão 2026-09-08: bancos derivados regeneráveis), **ADR-011** (gabarito de recall), **ADR-036** (capacidade > provider), **ADR-037/038** (verdict/evidência).

---

## 1. Contexto

O Don ordenou revisão do **Hermes Agent** (Nous Research — atualização diária, base grande de engenharia de agente) para extrair ideias onde o Cosca é fraco. O confronto honesto:

**Onde o Cosca já é superior (NÃO mexer):** identidade imutável por chain Ed25519 + blocks (ADR-013 §3.4); jail/fail-closed e Lei do Cofre; hierarquia Don→Kernel→Chiefs→Specialists com kernel que roteia e não implementa; conhecimento curado com FTS+vetor+grafo e âncora; memória com guard/integrity (`internal/memoryintegrity`, chain, watchdog); harness de evals com canário/verdict (ADR-037/038); gates determinísticos e de regressão (`internal/skilleval`, catálogo ADR-041). Hermes é single-agent com state.db central; não tem chain nem curadoria — transplantar qualquer coisa disso seria regressão.

**Onde o Cosca é fraco (oportunidades):** (1) estado que audita mas **não se repara**; (2) evals com harness robusto mas ~1 suite em disco (cultura de evals inexistente); (3) 11 adaptadores declarados vs 4 registrados no chat (estado "ready, not wired" não documentado); (4) compressão só em memória — não vira memória de longo prazo com gate de recall.

---

## 2. Oportunidade 1 — Estado auto-curável (repair assistido, não mágica)

**O que o Hermes faz:** `hermes_state_repair.py` (929 linhas) repara `state.db` com: **fingerprint do arquivo doente** (tamanho + amostra de conteúdo de 64 KiB com máscara dos ranges voláteis do header SQLite — bytes 24-28 e 92-96 — para escrita viva não "re-armar" o ledger); **ledger persistente de tentativas** sidecar `<db>.repair-attempts.json` que recusa nova cirurgia após 3 falhas no **mesmo** fingerprint; **backup forense deduplicado por conteúdo** com retenção de 3 cópias (lição do incidente: 105 tentativas / 89 GB de cópias idênticas). Complementos: `hermes_state_guard.py` (guarda de isolamento de teste vs produção) e `hermes_state_registry.py` (registry de sessões/estado).

**Gap no Cosca:** `internal/memoryintegrity/integrity.go` **audita** (manifest sha256 de arquivos de governança + memória) mas é **advisory e offline** — "Databases, sessions, and generated audit files are intentionally outside this scope" (doc do pacote) e **não repara**. O incidente real de hoje (banco corrompido/ausente → rebuild manual) é exatamente essa classe. O **ADR-014** já propôs a doutrina (Camada C: `cosca recover`; "corrupção de conhecimento → recover --auto re-index + re-embed com digest verificado") mas **não foi implementado** e não desce ao design de repair de SQLite. Reconciliando: este item **implementa a Camada C do ADR-014 para a classe derivado/estado**, com as salvaguardas do Hermes adaptadas.

**Opções de desenho COSCA-shaped:**
- **A (recomendada) — Repair assistido com salvaguardas, aplicado a bancos de estado derivados/regeneráveis** (`session`/`index.db`/`knowledge.db` + `vector-*.db`, todos regeneráveis desde a revisão do ADR-013): fingerprint estilo Hermes + ledger de tentativas + backup forense deduplicado; **auto apenas para derivado regenerável** (rebuild com digest verificado pós); fonte de verdade (chain/embed/identidade) **nunca** auto — `--manual + Don` (regra 3 e 4 do ADR-014). Extende o ADR-014 sem duplicá-lo: vira a especificação concreta da Camada C para SQLite.
- **B — Só estender `memoryintegrity` com modo repair explícito do Don** (`cosca memory repair --db X --from backup|rebuild`): menor escopo, zero auto; não fecha o "auto" que o ADR-014 promete e deixa o rebuild manual no boot.
- **C — Status quo (rebuild manual):** rejeitado — o incidente de hoje é a evidência do custo.

**Trade-offs:** A adiciona código de repair + risco de falso positivo (escrita viva tratada como corrupção) — mitigado pelo fingerprint com máscara de header e pelo ledger; B é mais seguro mas mantém o Don como repair-man; C é o mais caro operacionalmente.

**Recomendação:** **A**, como fatia de implementação do ADR-014 (ver §8).

---

## 3. Oportunidade 2 — Evals como cultura (regressão de capacidade por área)

**O que o Hermes faz:** `evals/` é **cultura**: dezenas de áreas offline, cada uma com `runner.py` + `tasks.py` + `fixtures.py` + `report.py` (ex.: `evals/session_search_schema/`, `evals/readtool/`, `evals/compaction/`, `evals/providers/`, `evals/postmortem/`), mais probes de regressão e A/B ao vivo — o código muda **contra** um acervo de capacidades mensuráveis.

**Gap no Cosca:** `internal/evals/` tem **harness robusto** (`suite.go`, `runner.go`, `canary.go`, `oracle.go`, `metrics.go`, `report.go`, `promote.go`, `ablation.go` — pipeline REAL, verify por comandos, reward, métricas com recall/F1/MCC) mas há **~1 suite em disco** (`.cosca/evals/skills/adr-generation.eval.yaml`; embed tem só `suites/esteira.yml` + `suites/smoke.yml`). Não há acervo de regressão por capacidade nem ligação ao `qgate`/CI além do existente em `internal/skilleval` (RegressionGate com holdout, tolerância 0.02 — o análogo estrutural do gate de recall do ADR-011).

**Opções de desenho:**
- **A (recomendada) — Suites de regressão de capacidade por área reaproveitando o harness existente**, começando por: **session_search** (FTS determinístico em `.cosca/memory/session/` — barato, estável, sem LLM) e **codebase recall** (busca semântica real do repo — gabarito congelado, ADR-011). Ligação ao `qgate`: `cosca eval --suite X` + falha por regressão > tolerância (reusar RegressionGate/skilleval).
- **B — Portar o modelo de scorecards do Hermes (runner/tasks/fixtures/report por área):** rejeitado por duplicação — o harness Go já cobre runner/tasks/verify/report; portar seria reimplementar.
- **C — Só suites de CI, sem cultura:** insuficiente — o ganho do Hermes é o acervo vivo por área.

**Trade-offs:** suites exigem manutenção (gabarito precisa revalidar quando o conhecimento legítimo muda); A concentra o custo inicial na 1ª área. **Recomendação:** **A**, primeira área = **session_search** (determinístico) seguida de **codebase recall** com baseline antes/depois.

---

## 4. Oportunidade 3 — Providers: wire unificado vs poda vs "ready, not wired"

**O que o Hermes faz:** `providers/` + `evals/provider_wire/` + `evals/provider_fallback/` — provedores como cidadãos de primeira classe, com probes de wire e fallback.

**Gap no Cosca:** o catálogo estático do `Manager` (`internal/providers/providers.go`, `buildDefaultProviders`) declara **~11 adaptadores** (openai, anthropic, google, azure, deepseek, ollama, groq, mistral, bedrock, local/gpu…), e há adaptadores implementados em `internal/chat/provider/` (`openai.go` — inclusive DeepSeek OpenAI-compatível, `anthropic.go`, `ollama.go`, `gpu.go`, `null.go`) — mas `RegisterChatProviders` (`internal/chat/provider/register_chat.go`) registra **apenas deepseek/ollama/gpu/none** (por design: Lei do Cofre — nuvem só com chave explícita; `Select()` pula factory sem chave). O catálogo estático e o registry dinâmico **duplicam** a lista; o estado "pronto, não ligado" não está documentado.

**Opções de desenho:**
- **A — Wire unificado dos 11 com interface comum tool-calling/vision:** alto custo; esbarra na Lei do Cofre e no princípio do ADR-036 (capacidade > provider; o Cosca escolhe capacidade, não modelo). Não é necessário para a máquina atual.
- **B — Podar o que não é usado (openai/anthropic/google/azure/groq/mistral/bedrock…):** "código morto é dívida", mas aqui os adaptadores são testados e **ativos sob chave** (fail-closed) — podar destrói capacidade enterprise validada por economia de ~0.
- **C (recomendada) — Manter e documentar "ready, not wired" + eliminar a duplicação de lista:** o `Manager` já enriquece o catálogo com o registry (`SetRegistry`) — tornar o catálogo **fonte única derivada do registry**, e documentar explicitamente quais adaptadores são "pronto, não ligado" e por quê (decisão do Don), com um teste de paridade garantindo que adaptadores OpenAI-compatíveis servem deepseek/groq/mistral sem wire novo.

**Recomendação:** **C** — alinhado a "pronto para enterprise" com dívida zero de duplicação. Decisão final do Don (§7).

---

## 5. Oportunidade 4 — Compressão persistente/hierárquica com gate de recall

**O que o Hermes faz:** `trajectory_compressor.py` (775 linhas) pós-processa trajetórias num **orçamento de tokens**: protege head (system/human/primeiro tool) e últimas N; do meio, resume só o necessário **sem nunca partir um par `<tool_call>/<tool_response>`**; e `evals/compaction/` (runner/policies/fixtures/report) **mede recall vs tokens** da política de compactação — a compactação é uma capacidade com regressão mensurável, não um hack.

**Gap no Cosca:** `internal/engine/compaction.go` compacta **só em memória**: o `buildSummary` vira uma mensagem system descartável no contexto; o resumo **não é persistido** nem promovido. `internal/pipeline/trajectory.go` é **event-sourced** (thought/action/observation, `Summary()`) mas a trajetória **não realimenta a memória de longo prazo** (camadas session/short/long de `.cosca/memory/`). Não há gate de recall para a condensação.

**Opções de desenho COSCA-shaped:**
- **A (recomendada) — Condensação incremental com gate de recall que vira memória de longo prazo:** persistir o summary da compactação como **artefato de sessão** (não descartável, com metadata: tokens salvos, trecho-fonte); ao final da sessão, **promoção seletiva** a memória de longo prazo quando o delta tiver valor (critério `knowledge_gain`/Useful Work do ADR-031); **gate de recall** (perguntas de verificação respondíveis a partir do resumo; medir com métricas do `internal/evals`) antes da promoção. Liga ADR-031 (token efficiency) e ADR-011/036.
- **B — Portar `trajectory_compressor.py`:** rejeitado — o Cosca já tem política própria (manter N recentes + sumarizar meio, com detecção de thrashing); o que falta é **persistência + gate**, não outra política.
- **C — Job assíncrono pós-sessão que transforma trajectory em memória longa (sem tocar compaction):** viável como fase 2 de A; sozinho deixa a compactação em memória sem medição.

**Trade-offs:** A toca o hot-path da sessão (risco de regressão — exige baseline antes/depois) e adiciona estado persistente; o gate de recall custa uma passada de verificação por sessão promovida.

**Recomendação:** **A**, com o gate de recall como critério de aceite e baseline de session_search/codebase antes e depois (§9).

---

## 6. Itens P2/P3 (resumidos — registrar, não detalhar)

| Ideia (Hermes) | Gap/estado no Cosca | Prioridade |
|---|---|---|
| Portabilidade de sessões (`hermes_state_portability.py`) | `memory/session/` local; sem export/import | P2 |
| Runner de lote/trajetória p/ treino (`batch_runner.py`, `mini_swe_runner.py`) | `trajectory.go` event-sourced serve; laboratory/LoRA qwen3 ainda não consome | P3 |
| MCP remoto streamable HTTP+OAuth (`optional-mcps/`) | ADR-028 local-only; remoto não iniciado | P3 |
| Drift de skills (89 embed vs 98 `.opencode`) | Falta teste de paridade — reusar invariante/generate-and-diff do catálogo (ADR-041) | P2 |
| Watchdog externo com respawn (`hermes_startup_watchdog.py`) | Serve via systemd cobre; watchdog externo dedicado não existe | P3 |
| Prompt-cache como invariante (`evals/token_accounting/`, `cache_prefix`) | ResultCache version-safe (ADR-031 Fase 1) existe; invariante em evals não | P2 |

---

## 7. Decisões em aberto para o Don

1. **Este ADR é aprovado e vira plano?** Recomendação: aprovar e iniciar pelo pacote da §8 (item 1 — estado auto-curável), por ressonância com o incidente de hoje (rebuild manual).
2. **Providers: wire, podar ou "ready, not wired"?** Recomendação: opção C (§4) — manter + documentar + eliminar duplicação de lista; **não** podar agora, **não** wire dos 11.
3. **Evals: qual área primeiro e liga ao qgate?** Recomendação: session_search primeiro (determinístico), depois codebase recall; ligar ao `qgate` com tolerância de regressão (holdout skilleval).
4. **Repair: escopo dos bancos alvo e auto vs manual?** Recomendação: session/index/knowledge + `vector-*.db`; **auto só para derivado regenerável**; chain/identidade sempre `--manual + Don` (ADR-014).

---

## 8. Pacote de trabalho inicial recomendado — Repair assistido de estado (item 1)

**Escopo delimitado:** repair de **bancos de estado derivados/regeneráveis** somente. **Fora de escopo por construção:** chain, embed, identidade, memória-bloco (nunca auto — ADR-014).

**Arquivos-alvo (Cosca):** novo pacote de repair (ex.: `internal/staterepair/`) com fingerprint de arquivo (tamanho + amostra com máscara de header volátil), ledger sidecar `<db>.repair-attempts.json` (máx 3 falhas no mesmo fingerprint), backup forense deduplicado por hash (retenção 3); comando CLI `cosca state repair --db session|index|knowledge|vector --dry-run|--apply` + healthcheck SQLite (`PRAGMA quick_check`) no boot; **sem tocar** `internal/embed/cosca/`. Referência de desenho: `hermes_state_repair.py` (adaptado).

**Critérios de aceite:**
1. Escrita viva (WAL ativo) **não** "queima" o ledger (fingerprint com máscara).
2. Mesmo fingerprint com falha persistente → ledger recusa cirurgia após 3 tentativas.
3. Backup deduplicado: arquivo doente idêntico nunca é copiado 2×; retenção máx 3.
4. Derivado corrompido → rebuild idempotente com digest verificado pós-repair.
5. Chain/identidade: nenhum caminho automático — sempre `--manual + Don`.
6. Testes unitários + integração (`cosca state repair --dry-run` em db corrompido sintético).

**Quem executa (especialistas):** implementação Go+SQLite → `cosca-specialist-backend-service` + `cosca-specialist-database-sql`; testes → `cosca-specialist-testing-unit`; revisão → `cosca-specialist-review-code` + cosca-database; gate de qualidade → cosca-qa. Coordenação: cosca-architecture (este ADR) e cosca-kernel (ADR-014).

---

## 9. Validação (baseline antes/depois — padrão da casa)

O item 1 toca zona sensível (bancos de estado servidos ao boot). Portanto: **antes** de qualquer mudança, rodar baseline das suites existentes (`.cosca/evals/` + `internal/embed/cosca/evals/suites/`) e, quando as suites da §3 existirem, session_search/codebase recall; **depois**, re-rodar as mesmas suites e exigir **zero regressão** acima da tolerância (RegressionGate, tolerância 0.02). Repair de `knowledge.db`/`vector-*.db` exige ainda baseline de busca semântica (gabarito congelado, ADR-011) antes e depois do rebuild. Sem baseline medido, o pacote não é aceito.

---

> **Autor:** Ordem do Don (revisão Hermes Agent, Nous Research) | **Formalizado por:** cosca-architecture | **Revisão pendente:** cosca-cto + cosca-kernel + Don | **Status:** Proposed — aguarda as decisões da §7.
