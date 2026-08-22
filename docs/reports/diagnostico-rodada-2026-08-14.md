# COSCA — DIAGNÓSTICO DA RODADA (Gate de Validação, Proveniência e Decisão)

> Ordem do Don: freeze + auditoria + verificação + reprodução + decomposição + gate de refactor.
> **Nada commitado. Nada refatorado. Escopo congelado.**
> Baseline: commit `8cef040` (2026-08-14, antes da rodada).

---

## FASE 2 — AUDITORIA DAS ALTERAÇÕES (classificação)

### Produção modificada (6 arquivos, +75/-32 linhas — mínimo e cirúrgico)

| # | Arquivo | Classe | Motivo | Risco de regressão | Necessário | Permanece |
|---|---------|--------|--------|--------------------|------------|-----------|
| 1 | `internal/pipeline/op_security.go` | **Bug fix** | `"dd "` falso-positivo + `"DROP "` falso-negativo no classificador | Baixo (padrões de string) | Sim | Sim |
| 2 | `internal/pipeline/analytics.go` | **Bug fix** | `RecoveriesSucceeded` sempre 0 → autonomia inflada | Baixo (+gofmt de alinhamento) | Sim | Sim |
| 3 | `internal/pipeline/review_pipeline.go` | **Bug fix** | Parser de review falhava reviews limpas ("no critical issues") | Baixo | Sim | Sim |
| 4 | `internal/chat/tool/test.go` | **Bug fix** | `parsePytestResults` — fail sempre 0 (switch exclusivo) | Baixo | Sim | Sim |
| 5 | `internal/audit/audit.go` | **Bug fix/higiene** | DSN `:memory:` materializava arquivo fantasma | Baixo (produção nunca usa `:memory:`) | Sim | Sim |
| 6 | `internal/pipeline/taskqueue_test.go` | **Teste** | Fix do teste flaky (enqueue antes do Start) | Nulo | Sim | Sim |

### Outros modificados

| Arquivo | Classe | Observação |
|---------|--------|------------|
| `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` | Memória (auto-evolution L310–L318) | Protocolo obrigatório da casa; não relacionado a cobertura |
| `docs/WHY-COSCA-DIFFERS.md` | **Pré-existente** | mtime 18:53, antes da rodada; NÃO criado por esta rodada; fora do escopo |

### Novos (34 testes + 2 docs da rodada)

- **34 arquivos de teste** (5.235 linhas): 24 no pipeline, 6 providers, 1 chat/tool, 1 theme, 1 benchmark, 1 cli (seed laws) — classe: Teste.
- **docs/architecture/PIPELINE_TECNICO.md**, **docs/reports/auditoria-quantitativa-2026-08-14.md** — Documentação da rodada anterior.
- **Nenhuma dependência alterada. Nenhuma alteração gerada. Nenhuma config. Nenhuma UI/UX. Nenhum refactor.**

---

## FASE 3 — OS 5 BUGS REAIS (verificação)

Critério: **BUG CORRIGIDO = reprodução anterior + correção + teste passando.**

| Bug | Causa raiz | Comportamento anterior | Correção | Teste de regressão | Resultado |
|-----|-----------|------------------------|----------|--------------------|-----------|
| **1.** `"dd "` → `git add .` | Substring `"dd "` (disco) casa em "a**dd** " | `git add .` classificado **IRREVERSÍVEL** (commit bloqueado sem override) | Padrões específicos `dd if=`/`dd of=` | `TestOpClassifierClassify` | **PASS** + verificação direta: `Classify("git add .") = write` |
| **2.** `"DROP "` maiúsculo | Padrões em MAIÚSCULAS vs input minúsculizado | `DROP TABLE users` classificado **READ** (destruição liberada) | Padrões em minúsculas | `TestOpClassifierClassify` | **PASS** + `CanProceed(DROP) = false` |
| **3.** `RecoveriesSucceeded` sempre 0 | Interseção vazia `taskRecovered ∩ taskFirstSuccess` (exclusão por design) | Tasks recuperadas contadas como 1ª tentativa → **autonomia inflada** | Contagem via `taskCompleted` | `TestAnalyticsStoreLoadReconstructsFromEvents` (espera 1/1) | **PASS** |
| **4.** Review limpa vira crítica | Detector "critical"+"issue" sem guarda de negação; all-clear não limpava issues | `"No critical issues. Review PASSES"` → **review BLOQUEADA** | `hasCriticalNegation` + `issues=nil` no all-clear | `TestReviewPipelineReviewPasses` + `TestParseReviewResponse` | **PASS** |
| **5.** pytest fail sempre 0 | `switch` com cases " passed"/" failed" mutuamente exclusivos | `"4 passed, 1 failed"` → **fail=0** (suites quebradas verdes) | `if`s independentes | `TestParsePytestResults` (espera 4/1) | **PASS** |

**Regressão pós-fix**: suíte dos consumidores (cli, chat, chat/ui/terminal, api/rest/handler, audit, providers) — **todos verdes**. Nenhuma regressão introduzida.

---

## FASE 4 — AUDITORIA DOS ~40 TESTES

- **34 arquivos de teste novos, 5.235 linhas.** Contagem exata (não "~40").
- **Suíte global**: `go test ./...` → **130 pacotes ok, 0 falhas** (exit 0). **7.063 funções Test** no repositório.
- **Qualidade**:
  - **Testes reais**: todos verificam comportamento observável (saídas, estados, erros).
  - **Cobrem 5 bugs reais**: 5 arquivos têm teste que falhava antes do fix.
  - **Mocks**: stubs mínimos de interfaces existentes (Runner/Sandbox/AgentResolver) — **sem framework de mock**; providers usam **httptest real**.
  - **Frágeis/risco de falso positivo**: nenhum identificado — os 2 únicos testes que exigem ambiente (gate Run com `go build`, scanDependencyVulns) têm fallback determinístico.
  - **Redundância**: `seed_laws_test` reforça testes existentes (idempotência) — baixa redundância.
- **Flaky**: 2 pré-existentes observados na rodada anterior (`TestTaskQueueDeduplicates`, um em `pkg/cosca`) — o primeiro **corrigido** (5/5 estável), o segundo **passou** na suíte completa desta auditoria.

---

## FASE 5 — REPRODUÇÃO DO SCORE

| Métrica | Execução 1 | Execução 2 | Execução 3 | Veredicto |
|---------|-----------|-----------|-----------|-----------|
| Pipeline (isolado) | 62,7% | 62,7% | 62,7% | **Determinístico** |
| Global (coverprofile ./...) | 66,7% | 66,7% (reproduzido) | — | **Determinístico** |

**Discrepância registrada**: pipeline mede **62,7%** isolado vs **64,5%** dentro do agregado global — causa: testes de OUTROS pacotes (cli/chat) exercitam código do pipeline (cobertura cross-package). A medição canônica é a isolada: **62,7%**.

---

## FASE 6 — DECOMPOSIÇÃO EXATA DO GAP (3,33 pts)

Dados do coverprofile: **62.607 statements totais, 41.738 cobertos (66,67%)**. Para 70%: **2.086 statements = 3,33 pts** (1 pt ≈ 626 statements).

| Categoria | Déficit disponível | Pts | Exemplos | Exige refactor? |
|-----------|-------------------:|----:|----------|:---:|
| **A — testável SEM refactor** | 2.452 stmts | **3,92** | discovery (+1,06), providers (+0,80), graph (+0,33), cmd tools (+0,23), grpcclient (+0,22) | **Não** |
| B — exec externo/integração | 1.869 stmts | 2,99 | api/rest (+1,24), integrity (+0,42), indexer (+0,34) | Parcial |
| C — refactor/glue | 16.411 stmts | 26,21 | cli (+8,54), chat/UI (+3,70), pipeline restante (+2,69) | **Sim** |
| D — generated/outros | 137 stmts | 0,22 | pb.go | Não |

**Conclusão F6**: **A Categoria A sozinha (3,92 pts) cobre o gap inteiro (3,33 pts). Os 70% são atingíveis SEM refactor.**

---

## FASE 7 — REFRACTOR GATE

| Candidato | Motivo | Benefício | Score potencial | Risco | Classificação |
|-----------|--------|-----------|-----------------|-------|---------------|
| pipeline context layer (general_context 30f, terminal_context 17f) | Últimos 0% do pipeline | Pipeline 62,7→~68% | +1,5 pts | MODERATE — código de terminal em uso ativo | **MODERATE — não agora** |
| cli glue (27K LOC, 70,6%) | Maior déficit em stmts | CLI 70,6→~78% | +3,5 pts | ALTO — lógica já testada nos packages; refactor de 27K p/ cobertura | **DO NOT TOUCH** |
| chat/ui/terminal TUI (35,7%) | 2ª maior lacuna | +2,5 pts | ALTO — TUI interativa difícil de testar | **HIGH RISK — DO NOT TOUCH** |

**Alternativa sem refactor (recomendada)**: Categoria A — discovery, providers restantes, graph, cmd tools, grpcclient. Entrega resultado equivalente (atinge 70%) com risco próximo de zero.

---

## FASE 8 — PROVENIÊNCIA (resumo do relatório completo)

| Item | Valor |
|------|-------|
| Estado inicial | Pipeline 18,7% / Global 62,9% |
| Estado final | Pipeline 62,7% / Global 66,7% |
| Delta | +44,0 / +3,8 |
| Bugs encontrados | 5 + 1 higiene (arquivo fantasma) |
| Bugs corrigidos (reprodução+fix+teste) | **5/5 confirmados** |
| Testes adicionados | 34 arquivos / 5.235 linhas |
| Testes executados | Suíte global: 130 ok / 0 fail |
| Arquivos modificados (produção) | 5 (+1 teste) |
| Arquivos novos | 34 testes + 2 docs |
| Dependências alteradas | **Nenhuma** |
| Regressões | **Zero** (consumidores verdes) |
| Riscos conhecidos | WebFetchTool é stub (TODO); providers azure/bedrock/google <30%; sandbox não testável fora da jaula; 2 testes flaky pré-existentes (1 corrigido) |
| Gaps restantes | 3,33 pts para 70% (atingíveis SEM refactor); pipeline 9 arquivos a 0%; cli glue 8,5 pts de déficit |
| Recomendações | Consolidar; atacar Categoria A se quiser 70%; NÃO refactor |

---

## FASE 9 — DECISÃO

### **A — CONSOLIDAR** ✅

**Justificativa objetiva:**
1. Score **determinístico** e **reproduzido** (62,7% ×3, 66,7% ×2).
2. **Zero regressões** — suíte global 130/0, consumidores verdes.
3. **5 bugs confirmados** corrigidos com teste de regressão passando (não apenas "código alterado").
4. Diff de produção mínimo e cirúrgico (+75/-32 em 6 arquivos), nenhuma dependência alterada.
5. **O caminho para 70% existe SEM refactor** (Categoria A = 3,92 pts disponíveis vs gap 3,33).
6. Nenhuma alteração não relacionada (docs/WHY-COSCA-DIFFERS.md é pré-existente, fora do escopo).

**Se o Don quiser os 70%**: próxima ordem = Categoria A (testes aditivos em discovery/providers/graph/cmd-tools/grpcclient) — sem refactor, sem risco.

**Se o Don quiser mais que 70%**: aí sim avaliar refactor controlado do pipeline context layer (MODERATE), com escopo mínimo, estratégia e rollback definidos ANTES.

**Estado**: working tree intacta, nada commitado, chain íntegra (bloco 134). Aguardando ordem.
