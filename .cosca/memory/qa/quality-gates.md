# Quality Gate Standard — Cosca v1.4.0-dev

> **Version**: 1.0.0 | **Status**: active | **Owner**: QA Chief (cosca-qa)
> **Ratified**: 2026-07-28 | **Scope**: Todos os 55 agentes, CI pipeline, release workflow

---

## Preâmbulo

Este documento estabelece os quality gates operacionais (G0–G9) aplicados ao ciclo de desenvolvimento da plataforma Cosca. Ele implementa o passo 7 (VALIDAÇÃO) do ciclo de decisão definido na CONSTITUTION.md Parte IV, sendo complementar ao documento [QUALITY_GATES.md](../../QUALITY_GATES.md) que define os gates mais amplos de ciclo de vida (G0–G4 por fase de projeto).

**Relação com CONSTITUTION.md**: Os gates G0–G9 aqui definidos são o mecanismo de enforcement do princípio P6 (Evolução sem Regressão) e do passo 7 do Ciclo de Decisão. Nenhum artefato pode avançar para a próxima etapa do ciclo sem passar pelos gates aplicáveis ao seu tipo.

---

## PARTE I — Bug Registry Audit

### 1.1 Auditoria Completa (7 Bugs)

O bug registry contém 7 bugs no total: 5 registrados formalmente no INDEX + 2 identificados pelo cosca-runtime durante análise de código mas ainda não formalizados como bugs individuais.

#### Bug-001: Hardcoded `/tmp` Paths (FIXED)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Major** (não blocker porque só afetava cross-platform; Linux principal não quebrava) |
| **Impacto** | Portabilidade — Windows incompatível, Linux restricted quebrava. ~3 arquivos afetados. |
| **Owner sugerido** | cosca-runtime (owner do `internal/runtime/daemon.go`) |
| **Classificação do INDEX** | medium |
| **Status** | ✅ Fixado (c30fac3) — mudou `/tmp/` → `./tmp/` |
| **Prevenção (N4)** | CI matrix Linux+macOS+Windows; lint rule forbidigo `/tmp/`; `t.TempDir()` padrão |

#### Bug-002: 959 Linter Warnings (FIXED)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Minor** (não afetava funcionalidade; débito estético acumulado) |
| **Impacto** | Legibilidade, consistência, manutenibilidade. ~350 arquivos com estilo inconsistente. |
| **Owner sugerido** | cosca-devops (CI pipeline owner) + cosca-technical-debt (owner de qualidade de código) |
| **Classificação do INDEX** | low |
| **Status** | ✅ Fixado (03860c2) |
| **Prevenção (N4)** | `golangci-lint` required check no CI; pre-commit hooks; política zero warnings |

#### Bug-003: Race Conditions in Parallel Execution (FIXED)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Critical** (data races são bugs de corretude — comportamento não determinístico) |
| **Impacto** | Provider calls e active chats competindo por estado. Testes intermitentes. Afetava `internal/runtime/` e pipeline de execução paralela. |
| **Owner sugerido** | cosca-runtime (owner do pipeline de execução) |
| **Classificação do INDEX** | high |
| **Status** | ✅ Fixado (9a950ff) |
| **Prevenção (N4)** | `go test -race` obrigatório no CI; documentação de thread-safety por pacote; `sync.RWMutex` + channels como padrão |

#### Bug-004: Provider Instance Caching Leak (FIXED)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Major** (memory leak progressivo; afetava long-running sessions) |
| **Impacto** | Crescimento de memória ilimitado. Provider config changes ignorados até restart. |
| **Owner sugerido** | cosca-provider (owner de provider lifecycle) + cosca-cache (owner de caching strategies) |
| **Classificação do INDEX** | medium |
| **Status** | ✅ Fixado (f3dbdc2) |
| **Prevenção (N4)** | TTL obrigatório em todos os caches; soak test 1h no CI com monitoramento de memória; separação config vs instance |

#### Bug-005: SQLite Migration Panics on First Run (FIXED)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Blocker** (impossibilita fresh install — bloqueia novos usuários completamente) |
| **Impacto** | `panic()` em `getCurrentVersion()` quando `schema_version` não existe. New users não conseguem iniciar. |
| **Owner sugerido** | cosca-database (owner de migrations) + cosca-runtime (owner do startup) |
| **Classificação do INDEX** | high |
| **Status** | ✅ Fixado (0ffb4da + c30fac3) |
| **Prevenção (N4)** | Teste de "clean state" obrigatório no CI; forbidigo `panic(` em `internal/`; migration template com zero-state handling |
| **Nota QA** | Este bug foi fixed mas o root cause pattern ("assumir estado pré-existente") pode se repetir. Recomendo: audit de todos os initializers da plataforma para zero-state safety. |

#### Bug-006: Restart() Quebrado (NÃO REGISTRADO — ABERTO)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Blocker** (CLI command `cosca runtime restart` funcionalmente quebrado) |
| **Impacto** | `Restart()` chama `Stop()` → state = `StateStopped`. Depois chama `Start()` → `Start()` requer `StateUninitialized`. A transição `Stopped→Uninitialized` está definida no mapa de transições (state.go:106) mas **nunca é executada** dentro de `Restart()`. Resultado: restart sempre falha com "runtime already started (state: stopped)". |
| **Evidência** | `internal/runtime/runtime.go:408-431` e teste `TestRestartReportsStateIssue` em `runtime_extended_test.go:449-464` |
| **Owner sugerido** | cosca-runtime (owner primário) |
| **Fix esperado** | Entre `Stop()` e `Start()` dentro de `Restart()`, executar transição explícita `Stopped→Uninitialized` (ou modificar `Start()` para aceitar `StateStopped` como estado válido de partida). |
| **Prevenção (N4)** | Integration test para o fluxo completo Start→Stop→Restart→Stop; CI gate que executa `cosca runtime restart` como smoke test. |

#### Bug-007: EventStartupComplete Prematuro (NÃO REGISTRADO — ABERTO)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Critical** (subscribers recebem evento de startup quando nada está rodando — decisões baseadas em estado falso) |
| **Impacto** | `EventStartupComplete` publicado em runtime.go:333 durante `StateInitializing`, ANTES de `lifecycle.ExecuteInit()` (linha 336) e ANTES de `lifecycle.ExecuteStart()` (linha 347). Qualquer subscriber que reaja a este evento vai operar com runtime não-inicializado. |
| **Evidência** | `internal/runtime/runtime.go:317-365` |
| **Owner sugerido** | cosca-runtime (owner primário) |
| **Fix esperado** | Mover `r.events.Publish(ctx, EventStartupComplete, "runtime", nil)` para depois de `lifecycle.ExecuteStart()` e health check loop, garantindo que subscribers só recebam o evento quando o runtime estiver completamente operacional (state = `StateRunning`). |
| **Prevenção (N4)** | Integration test que subscreve a `EventStartupComplete` e verifica que o estado do runtime é `StateRunning` no momento do evento; audit de todos os eventos para verificar timing correto. |

#### Bug não numerado: Metrics Misdocumented (NÃO REGISTRADO — ABERTO)

| Atributo | Valor |
|----------|-------|
| **Severidade QA** | **Major** (documentação falsa leva a decisões erradas de monitoring e operação) |
| **Impacto** | Documentação (`docs/runtime/overview.md`) descreve 7 métricas que não existem no código (ex: `runtime.state gauge`, `runtime.health gauge`). O código tem 12 métricas reais (8 atomic counters: index, search, context, memory, plugin, error, sync, event + 4 duration histograms: index, search, context, memory) que não estão documentadas. |
| **Evidência** | `internal/runtime/metrics.go:28-41` (8 counters + 4 histograms) vs `docs/runtime/overview.md` |
| **Owner sugerido** | cosca-documentation (owner de documentação) + cosca-runtime (validação de claims) |
| **Fix esperado** | Sincronizar documentação com código: documentar as 12 métricas reais, remover as 7 fictícias, e adicionar doc-code validator no CI que compara claims de documentação com símbolos reais do código. |
| **Prevenção (N4)** | G6 (Docs sincronizados) — doc-code validator cross-referencing claims with actual code symbols. |

### 1.2 Resumo de Classificação QA

| Bug | Severidade QA | Status | Blocker? | Owner Primário |
|-----|--------------|--------|----------|----------------|
| bug-001 | Major | ✅ Fixed | Não | cosca-runtime |
| bug-002 | Minor | ✅ Fixed | Não | cosca-devops / cosca-technical-debt |
| bug-003 | Critical | ✅ Fixed | Não | cosca-runtime |
| bug-004 | Major | ✅ Fixed | Não | cosca-provider / cosca-cache |
| bug-005 | Blocker | ✅ Fixed | Sim (era) | cosca-database / cosca-runtime |
| Restart() | **Blocker** | 🔴 Aberto | **Sim** | cosca-runtime |
| EventStartupComplete | **Critical** | 🔴 Aberto | Não | cosca-runtime |
| Metrics misdocumented | **Major** | 🔴 Aberto | Não | cosca-documentation / cosca-runtime |

**Total**: 8 bugs — 5 fixados, 3 abertos (1 blocker, 1 critical, 1 major).

### 1.3 Discrepância com o INDEX

O `INDEX.md` reporta 5 bugs ativos com todos fixados. No entanto, 3 bugs adicionais foram identificados pelo cosca-runtime durante análise de código (Restart quebrado, EventStartupComplete prematuro, métricas mal documentadas) e não foram formalmente registrados no bug registry. **Recomendação**: Criar bug-006, bug-007 e bug-008 para tracking formal.

---

## PARTE II — G0–G9 Pipeline Gates

### Arquitetura dos Gates

```
G0: BUILD       ──► go build ./... OK
G1: LINT        ──► golangci-lint clean
G2: VET         ──► go vet ./... OK
G3: TEST        ──► go test -race ./... OK
G4: SECURITY    ──► govulncheck + gosec limpos
G5: COVERAGE    ──► coverage ≥ threshold
G6: DOCS        ──► doc-code validator passa
G7: PERF        ──► benchmarks não degradaram
G8: BREAKING    ──► CHANGELOG + migration guide
G9: REVIEW      ──► ≥ 1 review approval
```

Os gates são sequenciais: G(N) só executa se G(N-1) passou. G0–G5 são automatizados (CI). G6–G9 são semi-automatizados (CI + humana/agente). Qualquer gate pode ser rejeitado automaticamente (ver seção de rejeição automática abaixo).

---

### G0 — Build Passa

**O que verifica**: O código compila sem erros em todos os packages.

| Parâmetro | Valor |
|-----------|-------|
| **Comando** | `go build ./...` |
| **Critério de sucesso** | Exit code 0, zero erros de compilação |
| **Timeout** | 5 minutos |
| **Automação** | CI — roda em todo push e PR |
| **Rejeição automática** | Sim — se `go build ./...` falhar, merge bloqueado |

**Justificativa**: Gate mínimo de sanidade. Sem build, nada funciona. Corresponde ao princípio P2 (código executado é a verdade absoluta).

---

### G1 — Lint Passa

**O que verifica**: Estilo de código, boas práticas, problemas de qualidade estática.

| Parâmetro | Valor |
|-----------|-------|
| **Ferramenta** | `golangci-lint run` |
| **Configuração** | `.golangci.yml` na raiz do projeto |
| **Critério de sucesso** | Zero issues (política zero warnings) |
| **Escopo** | Todos os packages Go modificados no diff (para PRs); todos os packages (para main) |
| **Automação** | CI — required check em todo PR |
| **Rejeição automática** | Sim — lint não limpo bloqueia merge |

**Justificativa**: Previne bug-002 (959 warnings acumuladas). A política zero-warnings impede que débito de estilo se acumule.

**Regras mínimas obrigatórias**:
- `gofmt` / `goimports` — formatação consistente
- `govet` — problemas detectáveis estaticamente
- `staticcheck` — bugs sutis, código não-idiomático
- `errcheck` — erros não verificados
- `forbidigo` — proibir `panic(` em `internal/` (bug-005), proibir `/tmp/` absoluto (bug-001)

---

### G2 — Vet Passa

**O que verifica**: Problemas de corretude detectáveis estaticamente pelo compilador Go.

| Parâmetro | Valor |
|-----------|-------|
| **Comando** | `go vet ./...` |
| **Critério de sucesso** | Exit code 0, zero warnings |
| **Automação** | CI — roda em todo push e PR |
| **Rejeição automática** | Sim — vet com falha bloqueia merge |

**Justificativa**: `go vet` detecta bugs sutis que o compilador não pega (ex: unreachable code, suspicious constructs, format string mismatches). É parte do G1 (golangci-lint já inclui govet) mas mantido como gate separado por clareza e para cenários onde golangci-lint não está disponível.

---

### G3 — Testes Passam

**O que verifica**: Todos os testes (unitários, integração, race detection) passam.

| Parâmetro | Valor |
|-----------|-------|
| **Comando** | `go test -race -count=1 ./...` |
| **Critério de sucesso** | Exit code 0, zero test failures, zero race conditions |
| **Timeout** | 15 minutos |
| **Flaky test detection** | Se um teste falha intermitentemente (>1 vez em 10 runs), é marcado como flaky e deve ser corrigido antes do merge |
| **Automação** | CI — required check |
| **Rejeição automática** | Sim — qualquer teste falhando bloqueia merge |

**Justificativa**: Previne bug-003 (race conditions). `-race` flag é obrigatória — detecta data races que só aparecem em execução. Previne regressões via suite de testes existente.

**Regras adicionais**:
- Testes devem ser determinísticos (zero flaky tests)
- Testes devem ser independentes (sem dependência entre si)
- Testes devem seguir o padrão AAA (Arrange-Act-Assert)
- Nenhum teste pode depender de estado externo (rede, filesystem fora de t.TempDir())

---

### G4 — Security Scan Passa

**O que verifica**: Vulnerabilidades conhecidas em dependências e no código.

| Parâmetro | Valor |
|-----------|-------|
| **govulncheck** | `govulncheck ./...` — zero vulnerabilidades conhecidas |
| **gosec** | `gosec ./...` — zero issues de severidade High ou Critical |
| **Critério de sucesso** | Ambos os scanners retornam limpo |
| **Automação** | CI — roda em todo PR |
| **Rejeição automática** | **Sim — security scan falha = rejeição automática SEM exceções** (P1: Segurança acima de funcionalidade) |

**Justificativa**: Implementa P1 (Segurança acima de funcionalidade). Qualquer CVE conhecida ou vulnerabilidade de código bloqueia o merge, independentemente da urgência da feature.

**Escalation path**: Se o security scan falhar, notificar imediatamente cosca-security. Apenas cosca-security pode autorizar um bypass temporário com justificativa documentada e prazo de correção.

---

### G5 — Coverage ≥ Threshold

**O que verifica**: Cobertura de testes atende ao piso mínimo.

| Parâmetro | Valor |
|-----------|-------|
| **Comando** | `go test -coverprofile=coverage.out ./...` |
| **Piso (threshold)** | **≥ 70% de line coverage** (baseline atual: ~78%) |
| **Cobertura em código alterado** | ≥ 80% nas linhas modificadas no diff do PR |
| **Critério de sucesso** | Coverage total ≥ 70% E coverage do diff ≥ 80% |
| **Automação** | CI — warning se coverage cair abaixo do piso; error se coverage do diff < 80% |
| **Rejeição automática** | Condicional: coverage do diff < 80% = rejeição. Coverage total < 70% = warning (não bloqueia, mas gera ticket de technical debt) |

**Justificativa**: O piso de 70% reconhece que o baseline atual é ~78% e evita regressão. O threshold de 80% no código alterado garante que novo código é bem testado. A meta de longo prazo é ≥ 80% global (conforme PROMPT.md do cosca-qa).

**Roadmap de coverage**:
- Curto prazo (Onda 2): manter ≥ 70%, mirar 80% em código novo
- Médio prazo (Onda 3-4): elevar piso global para ≥ 80%
- Longo prazo (v1.5.0): ≥ 85% com branch coverage ≥ 75%

---

### G6 — Docs Sincronizados

**O que verifica**: Documentação não referencia arquivos, endpoints, ou símbolos inexistentes.

| Parâmetro | Valor |
|-----------|-------|
| **Ferramenta** | Doc-code validator (script que cruza claims em docs/ com paths reais) |
| **Verificações** | Referências a arquivos existem? Endpoints documentados existem na API? Métricas documentadas existem no código? Comandos CLI documentados existem no Cobra? |
| **Critério de sucesso** | Zero broken references |
| **Automação** | CI — roda no PR; semi-automatizado (alguns checks dependem de heurísticas) |
| **Rejeição automática** | Condicional: broken reference para arquivo/endpoint crítico = rejeição. Broken reference para doc não-crítica = warning com ticket. |

**Justificativa**: Previne o bug de "metrics misdocumented" (docs dizem 7 métricas, código tem 12). Implementa P2 (código é a verdade) — documentação que contradiz o código é pior que ausência de documentação. Resolve risco R8 (docs rot).

**Regras de validação**:
- Todo endpoint REST documentado em docs/ deve existir no router
- Toda métrica documentada deve existir no código (campo ou função exportada)
- Todo comando CLI documentado deve existir no Cobra command tree
- Arquivos referenciados por path em docs/ devem existir no repositório

---

### G7 — Performance Baseline Não Degradou

**O que verifica**: Benchmarks não pioraram em relação ao baseline anterior.

| Parâmetro | Valor |
|-----------|-------|
| **Ferramenta** | `go test -bench=. -benchmem -count=5` + `benchstat` |
| **Critério de sucesso** | Nenhum benchmark com degradação > 10% no p95 (ou > 20% no p50) sem justificativa documentada |
| **Baseline** | Armazenado em `cosca_benchmark_baseline.txt` (gerado pelo cosca-performance) |
| **Automação** | CI — roda em PRs que alteram código em hot paths (runtime, search, database); opcional para outros PRs |
| **Rejeição automática** | Condicional: degradação > 10% sem justificativa = rejeição. Degradação ≤ 10% = warning com ticket de investigation. |

**Justificativa**: Implementa P6 (Evolução sem Regressão) no domínio de performance. Uma regressão de performance é tão grave quanto uma regressão funcional. O baseline atual será estabelecido pelo cosca-performance na Onda 2.

**Hot paths sujeitos a G7 obrigatório**:
- Search pipeline (FTS5 + vector fusion)
- API request/response serialization
- Knowledge index rebuild
- Session bootstrap (Phase 0–4)
- Provider call dispatch

---

### G8 — Breaking Change Documentado

**O que verifica**: Mudanças que quebram compatibilidade têm documentação de migração.

| Parâmetro | Valor |
|-----------|-------|
| **Detecção** | Análise de diff: API endpoint removido? Schema de banco alterado? Comando CLI renomeado? Config key alterada? |
| **Critério de sucesso** | Para cada breaking change detectado: entrada no CHANGELOG.md + migration guide (se aplicável) |
| **Formato CHANGELOG** | Seguir [Keep a Changelog](https://keepachangelog.com/) — seções: Added, Changed, Deprecated, Removed, Fixed, Security |
| **Automação** | Semi-automatizado — CI detecta breaking changes (diff de API schema, CLI commands, config keys) e verifica se CHANGELOG foi atualizado |
| **Rejeição automática** | Sim — breaking change sem entrada no CHANGELOG = rejeição. API breaking change sem migration guide = rejeição. |

**Justificativa**: Protege usuários e agentes que dependem de contratos estáveis. Breaking changes não documentados causam falhas em cadeia (um agente quebra → outros agentes que dependem dele quebram).

**O que constitui breaking change**:
- Remoção de endpoint REST público
- Alteração de schema de resposta (campo removido, tipo alterado)
- Renomeação de comando CLI
- Alteração de config key
- Mudança de comportamento de função exportada em `pkg/cosca/`
- Remoção de event type do runtime
- Alteração de formato de arquivo (ex: schema do knowledge.db)

---

### G9 — Review Approval

**O que verifica**: O código foi revisado por outro agente ou humano qualificado.

| Parâmetro | Valor |
|-----------|-------|
| **Reviewer mínimo** | 1 review de agente Chief do domínio afetado (ou cosca-review para cross-domain) |
| **Critério de sucesso** | Review aprovada sem issues blocker |
| **Checklist do reviewer** | SOLID, security, error handling, tests, DRY, naming, documentation (conforme QUALITY_GATES.md Gate 2) |
| **Automação** | GitHub required reviewer; cosca-review pode ser invocado como reviewer automático |
| **Rejeição automática** | Sim — sem review approval, merge bloqueado |

**Justificativa**: Review é o último gate humano/agente antes do merge. Mesmo com G0–G8 passando, um olhar humano/agente pode detectar problemas de design que ferramentas automatizadas não capturam (ex: complexidade acidental, naming ruim, architectural drift).

**Quem pode aprovar por tipo de artefato**:
| Tipo de artefato | Reviewer qualificado |
|------------------|---------------------|
| Código Go (runtime) | cosca-runtime ou cosca-review |
| Código Go (API) | cosca-backend ou cosca-review |
| Código Frontend | cosca-frontend ou cosca-review |
| Config (CI, Docker) | cosca-devops ou cosca-review |
| Documentação | cosca-documentation |
| Schema/SQL | cosca-database |
| Security-sensitive | cosca-security (obrigatório) |

---

## PARTE III — Acceptance Criteria por Tipo de Deliverable

### 3.1 Bug Fix

| Gate | Requerido? | Threshold |
|------|-----------|-----------|
| **G0** Build | ✅ Sim | `go build ./...` OK |
| **G1** Lint | ✅ Sim | Zero issues |
| **G2** Vet | ✅ Sim | `go vet ./...` OK |
| **G3** Test | ✅ Sim | **Unit test que reproduz o bug** (obrigatório) + todos os testes existentes passam |
| **G4** Security | ✅ Sim | Scan limpo |
| **G5** Coverage | ✅ Sim | Coverage do fix ≥ 80% |
| **G6** Docs | Condicional | Se o bug afetava API pública: atualizar docs com a correção |
| **G7** Perf | Condicional | Se o fix afeta hot path: benchmark comparativo |
| **G8** Breaking | Condicional | Se o fix altera comportamento público: documentar |
| **G9** Review | ✅ Sim | ≥ 1 review |

**Regras específicas para bug fix**:
1. **Teste de regressão obrigatório**: Todo bug fix deve incluir um teste que reproduz o bug (falha antes do fix, passa depois).
2. **Referência ao bug**: O commit/PR deve referenciar o bug key (ex: `Fixes bug-006`).
3. **Root cause documentada**: O bug fix deve incluir (no commit message ou PR description) a root cause e por que o fix escolhido é a solução correta (não apenas um workaround).
4. **Verificação de bugs similares**: O autor do fix deve verificar se o mesmo padrão de bug existe em outras partes do código.

### 3.2 Nova Feature

| Gate | Requerido? | Threshold |
|------|-----------|-----------|
| **G0** Build | ✅ Sim | `go build ./...` OK |
| **G1** Lint | ✅ Sim | Zero issues |
| **G2** Vet | ✅ Sim | `go vet ./...` OK |
| **G3** Test | ✅ Sim | Unit tests + integration tests (se aplicável). Happy path + edge cases + error paths. |
| **G4** Security | ✅ Sim | Scan limpo. Se a feature envolve auth, dados, ou rede: security review adicional por cosca-security. |
| **G5** Coverage | ✅ Sim | Coverage da nova feature ≥ 80% |
| **G6** Docs | ✅ Sim | API docs (se novos endpoints), ADR (se decisão arquitetural), README update (se aplicável) |
| **G7** Perf | ✅ Sim | **Benchmark obrigatório** se a feature está em hot path. Se não: benchmark recomendado mas não bloqueante. |
| **G8** Breaking | ✅ Sim | Se a feature introduz breaking change: CHANGELOG + migration guide |
| **G9** Review | ✅ Sim | ≥ 1 review (≥ 2 se a feature é cross-domain) |

**Regras específicas para nova feature**:
1. **ADR obrigatório** se a feature introduz nova dependência, novo padrão arquitetural, ou altera fluxo de dados existente.
2. **A11y check** (acessibilidade) para features de frontend: contraste, navegação por teclado, screen reader.
3. **i18n/l10n** check para features com strings visíveis ao usuário: todas as strings são externalizadas?
4. **Feature flag** se a feature é experimental ou de alto risco: deve poder ser desabilitada sem deploy.

### 3.3 Infra Change (CI Pipeline, Config, Deploy)

| Gate | Requerido? | Threshold |
|------|-----------|-----------|
| **G0** Build | ✅ Sim | Config/pipeline não quebra o build |
| **G1** Lint | ✅ Sim | YAML/JSON/TOML validados (yamllint, jsonlint) |
| **G2** Vet | N/A | — |
| **G3** Test | ✅ Sim | **Dry-run do pipeline** (ex: `act` para GitHub Actions local). Smoke test pós-deploy. |
| **G4** Security | ✅ Sim | **Obrigatório**: secrets scan (detect-secrets, gitleaks). Permissions mínimas nos workflows. Nenhum token hardcoded. |
| **G5** Coverage | N/A | — |
| **G6** Docs | ✅ Sim | Workflow documentado. Runbook de troubleshooting. |
| **G7** Perf | Condicional | Se a infra change afeta tempo de build/deploy: benchmark de pipeline (antes/depois). |
| **G8** Breaking | ✅ Sim | Se a infra change altera comportamento esperado (ex: novo required check que vai falhar em PRs existentes): documentado e comunicado. |
| **G9** Review | ✅ Sim | ≥ 1 review (cosca-devops como reviewer obrigatório para CI/CD changes). |

**Regras específicas para infra change**:
1. **Secret scanning obrigatório**: Nenhum token, chave, ou credential pode ser commitado. Use GitHub Secrets ou Vault.
2. **Rollback testado**: Toda infra change deve ter um plano de rollback testado (revert commit ou rollback automatizado).
3. **Princípio do menor privilégio**: Workflows devem ter `permissions:` mínimo necessário.
4. **Sem hardcoded paths**: CI deve funcionar em qualquer branch/fork (sem paths absolutos, sem secrets de ambiente específico).

### 3.4 Documentation Change

| Gate | Requerido? | Threshold |
|------|-----------|-----------|
| **G0** Build | N/A | — |
| **G1** Lint | ✅ Sim | Markdown lint (markdownlint), spell check (misspell) |
| **G2** Vet | N/A | — |
| **G3** Test | ✅ Sim | **Doc-code validator**: claims na documentação correspondem a símbolos reais no código? |
| **G4** Security | Condicional | Se a doc contém exemplos de código: verificar que não incluem secrets, tokens, ou más práticas de segurança |
| **G5** Coverage | N/A | — |
| **G6** Docs | ✅ Sim | Links internos não quebrados. Imagens referenciadas existem. |
| **G7** Perf | N/A | — |
| **G8** Breaking | Condicional | Se a doc change deprecia ou remove documentação de feature existente |
| **G9** Review | ✅ Sim | ≥ 1 review (cosca-documentation ou domain Chief) |

**Regras específicas para documentation change**:
1. **Broken link check**: Todos os links internos devem ser válidos.
2. **Code samples verificados**: Todo exemplo de código na documentação deve compilar/executar.
3. **Consistência terminológica**: Mesmos termos usados em todo o ecossistema de docs.

---

## PARTE IV — Critérios de Rejeição Automática

Os seguintes cenários resultam em **rejeição automática do merge**, sem possibilidade de bypass sem escalação:

| # | Cenário | Gate | Escalação necessária |
|---|---------|------|---------------------|
| R1 | `go build ./...` falha | G0 | Nenhuma — corrigir o build |
| R2 | `golangci-lint` não limpo | G1 | Nenhuma — corrigir warnings |
| R3 | `go test -race ./...` falha (test failure ou race detected) | G3 | Nenhuma — corrigir testes/races |
| R4 | **Security scan falha** (govulncheck ou gosec com high/critical) | G4 | **cosca-security** — apenas security chief pode autorizar bypass temporário |
| R5 | Coverage do código alterado < 80% | G5 | **cosca-qa** — QA chief pode autorizar exceção com justificativa documentada |
| R6 | Breaking change sem CHANGELOG entry | G8 | **cosca-release** — release chief aprova formato e completude |
| R7 | API breaking change sem migration guide | G8 | **cosca-backend** + **cosca-documentation** |
| R8 | Token/secret detectado no diff | G4 | **cosca-security** — revogar token imediatamente |
| R9 | `panic(` em código `internal/` | G1 | **cosca-runtime** — exceção apenas se for startup em `main()` |
| R10 | Sem review approval | G9 | Review pendente — não é bypass, é bloqueio até review |

**Bypass temporário**: Apenas possível para R5 (coverage) com:
- Justificativa documentada (por que não é possível testar agora?)
- Ticket de technical debt criado com prazo (máximo 2 sprints)
- Aprovação do QA Chief (cosca-qa)

---

## PARTE V — Sign-Off do Plano de Qualidade — Onda 2

### 5.1 Validação dos 9 Tasks da Onda 2

Cada task dos outros 9 agentes da Onda 2 foi avaliada quanto à clareza e verificabilidade de seus critérios de aceitação.

#### cosca-governance — Auditoria de Conformidade (Fase 1, Onda A)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| 55 capability profiles auditados (100%) | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| Report com ≥ 5 não-conformidades | ✅ Sim | ✅ Sim (threshold) | ✅ APROVADO |
| Arquivos órfãos identificados | ✅ Sim | ✅ Sim (lista) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim (valor no learnings.md) | ✅ APROVADO |

**Parecer QA**: Critérios bem definidos e verificáveis. Recomendo adicionar um critério de "severidade classificada" (não apenas encontrar não-conformidades, mas classificá-las como blocker/warning/info).

#### cosca-technical-debt — Scorecard de Dívida (Fase 1, Onda A)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| Fontes de dívida catalogadas | ✅ Sim | ✅ Sim (checklist) | ✅ APROVADO |
| Debt Score calculado | ✅ Sim | ✅ Sim (valor numérico) | ✅ APROVADO |
| Top 10 prioridades com ROI | ✅ Sim | ✅ Sim (lista ranqueada) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: Critérios sólidos. A métrica de Debt Score composto é bem definida. Recomendo que o Debt Score seja armazenado em local consultável por outros agentes (ex: `.cosca/metrics/tech-debt-score.json`).

#### cosca-critic — 5-Question Challenge (Fase 1, Onda A)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| 5 questões respondidas com profundidade | ✅ Sim | Parcial — "profundidade analítica" é subjetivo | ⚠️ AJUSTE |
| ≥ 2 alternativas propostas com trade-offs | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| Cross-reference com bug registry, ADRs, risks | ✅ Sim | ✅ Sim (citações) | ✅ APROVADO |
| Veredito fundamentado | ✅ Sim | Parcial — "fundamentado" é subjetivo | ⚠️ AJUSTE |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: ⚠️ Precisa de ajuste. Dois critérios usam termos subjetivos ("profundidade analítica", "fundamentado"). Recomendo substituir por critérios objetivos: "Cada questão respondida com ≥ 3 parágrafos de análise referenciando evidências concretas (código, dados, ou riscos documentados)" e "Veredito inclui pelo menos 3 condições explícitas (se APPROVE WITH CONDITIONS) ou 3 razões específicas (se REJECT)".

#### cosca-compliance — Self-Assessment GDPR/LGPD (Fase 1, Onda A)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| GDPR com ≥ 20 controles verificados | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| LGPD com ≥ 15 controles verificados | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| Cross-reference com 5 passos cosca-security | ✅ Sim | ✅ Sim (checklist) | ✅ APROVADO |
| Roadmap de remediação priorizado | ✅ Sim | ✅ Sim (lista com prazos) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: ✅ Aprovado. Critérios com thresholds numéricos claros, cross-reference verificável, e deliverable concreto (roadmap). Sem ajustes necessários.

#### cosca-testing — Suite de Integração (Fase 2, Onda B)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| ≥ 15/20 transições cobertas | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| 3 bugs reproduzidos com casos de teste | ✅ Sim | ✅ Sim (testes que falham) | ✅ APROVADO |
| Quality gates do cosca-qa aplicados | ✅ Sim | Parcial — depende deste documento existir | ⚠️ DEPENDÊNCIA |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |
| ≥ 1 failure registrado | ✅ Sim | ✅ Sim (failures.md) | ✅ APROVADO |

**Parecer QA**: ⚠️ Dependência em cosca-qa. O critério "Quality gates do cosca-qa aplicados" depende deste documento existir. **Este documento agora existe — dependência satisfeita.** O cosca-testing deve referenciar este Quality Gate Standard e seguir G3 (testes), G5 (coverage ≥ 80% no código de teste — sim, código de teste também deve ser testável e bem escrito), e G9 (review approval).

#### cosca-performance — Baseline + Bug-005 Root Cause (Fase 2, Onda B)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| ≥ 3 benchmarks com p50/p95/p99 | ✅ Sim | ✅ Sim (dados de latência) | ✅ APROVADO |
| Memory profile com alocações por operação | ✅ Sim | ✅ Sim (pprof output) | ✅ APROVADO |
| Bug-005 root cause com EXPLAIN | ✅ Sim | ✅ Sim (EXPLAIN QUERY PLAN output) | ✅ APROVADO |
| Top 5 CPU hot paths | ✅ Sim | ✅ Sim (pprof top) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |
| ≥ 1 failure registrado | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: ✅ Aprovado. Critérios técnicos, mensuráveis, com ferramentas específicas (pprof, EXPLAIN QUERY PLAN, benchstat). Sem ajustes necessários. Nota: O baseline gerado aqui será a referência para o G7 (Performance) nos PRs futuros.

#### cosca-devops — CI/CD Pipeline (Fase 2, Onda B)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| CI com ≥ 4 stages (lint, test, build, doc-validate) | ✅ Sim | ✅ Sim (contagem de stages) | ✅ APROVADO |
| CD pipeline definido (deploy + rollback) | ✅ Sim | ✅ Sim (workflow file) | ✅ APROVADO |
| ≥ 1 build bem-sucedido com todos os gates | ✅ Sim | ✅ Sim (CI run green) | ✅ APROVADO |
| Doc-code validator integrado | ✅ Sim | ✅ Sim (stage exists + passes) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |
| ≥ 1 failure registrado | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: ✅ Aprovado. Este é o task que implementa os gates G0–G6 como automação. Critérios bem alinhados com este Quality Gate Standard. Recomendo que o CI pipeline referencie este documento como fonte dos thresholds (ex: coverage ≥ 70%, lint zero warnings).

#### cosca-review — Revisão de Código (Fase 3, Onda C)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| ≥ 2 artefatos revisados | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| ≥ 5 issues encontrados | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| Checklist aplicado (SOLID, security, etc.) | ✅ Sim | ✅ Sim (checklist preenchido) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |
| ≥ 1 padrão "a evitar" em patterns.md | ✅ Sim | ✅ Sim (patterns.md entry) | ✅ APROVADO |

**Parecer QA**: ✅ Aprovado. Nota: O cosca-review depende de outputs da Fase 2 (cosca-devops CI pipeline + cosca-testing integration tests). Se esses outputs não existirem, o review não pode ser executado — isso é uma dependência de schedule, não de qualidade.

#### cosca-monitoring — SLOs + Prometheus (Fase 3, Onda C)

| Critério | Definido? | Verificável? | Veredito |
|----------|-----------|-------------|----------|
| 5 SLOs com SLI, target, error budget | ✅ Sim | ✅ Sim (documento de SLOs) | ✅ APROVADO |
| Prometheus /metrics com ≥ 10 métricas | ✅ Sim | ✅ Sim (curl /metrics + contagem) | ✅ APROVADO |
| Dashboard Grafana (JSON) | ✅ Sim | ✅ Sim (JSON válido) | ✅ APROVADO |
| ≥ 3 alerting rules | ✅ Sim | ✅ Sim (contagem) | ✅ APROVADO |
| Learning com confiança ≥ 0.40 | ✅ Sim | ✅ Sim | ✅ APROVADO |

**Parecer QA**: ✅ Aprovado. Depende de cosca-performance (baselines de latência para calibrar SLOs) — esta dependência é crítica e deve ser verificada antes da execução.

### 5.2 Resumo de Aprovação

| # | Agente | Status | Ação necessária |
|---|--------|--------|-----------------|
| 1 | cosca-qa | ✅ EXECUTANDO | Este documento é o deliverable |
| 2 | cosca-governance | ✅ APROVADO | Nenhuma — prosseguir |
| 3 | cosca-technical-debt | ✅ APROVADO | Nenhuma — prosseguir |
| 4 | cosca-critic | ⚠️ APROVADO COM RESSALVAS | Refinar 2 critérios subjetivos (ver 5.1) |
| 5 | cosca-compliance | ✅ APROVADO | Nenhuma — prosseguir |
| 6 | cosca-testing | ✅ APROVADO (dependência satisfeita) | Referenciar este Quality Gate Standard |
| 7 | cosca-performance | ✅ APROVADO | Nenhuma — prosseguir |
| 8 | cosca-devops | ✅ APROVADO | Alinhar thresholds do CI com este documento |
| 9 | cosca-review | ✅ APROVADO (dependência de schedule) | Aguardar outputs da Fase 2 |
| 10 | cosca-monitoring | ✅ APROVADO (dependência de schedule) | Aguardar baselines do cosca-performance |

**Veredito final**: 8 tasks APROVADOS, 1 APROVADO COM RESSALVAS (cosca-critic), 1 em execução (cosca-qa). **Onda 2 pode prosseguir.**

---

## PARTE VI — Métricas de Qualidade (QA Scorecard)

### Baselines atuais

| Métrica | Baseline | Target (pós-Onda 2) | Meta longo prazo |
|---------|----------|---------------------|-----------------|
| Line Coverage | ~78% | ≥ 70% (piso mantido) | ≥ 85% (v1.5.0) |
| Bugs abertos | 3 (2 críticos, 1 major) | 0 (todos fixados ou com owner) | 0 |
| G0–G9 gates implementados no CI | 0 de 10 | ≥ 6 de 10 (G0–G5) | 10 de 10 |
| Testes de integração (runtime) | 0 transições testadas | ≥ 15/20 transições | 20/20 |
| Performance baseline | Inexistente | 3+ benchmarks | Todos os hot paths |
| Security scan automation | Manual | govulncheck + gosec no CI | SAST + DAST + dependency audit |
| Doc-code sync | Nenhum | Validator integrado no CI | Zero broken references |
| Flaky tests | Desconhecido | 0 (rastreados) | 0 |

---

## Referências

| Documento | Relação |
|-----------|---------|
| [CONSTITUTION.md](../../CONSTITUTION.md) | Princípios P1, P2, P6; Ciclo de Decisão passo 7 |
| [QUALITY_GATES.md](../../QUALITY_GATES.md) | Gates de ciclo de vida G0–G4 (fases de projeto) |
| [onda-2-plan.md](../roadmap/onda-2-plan.md) | Plano de ativação dos 10 agentes da Onda 2 |
| [INDEX.md](../bug/INDEX.md) | Bug registry — auditoria e classificação |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato de registro de aprendizados |

---

## Histórico

| Versão | Data | Autor | Alterações |
|--------|------|-------|-----------|
| 1.0.0 | 2026-07-28 | cosca-qa | Criação inicial: auditoria de 8 bugs, definição G0–G9, acceptance criteria por tipo, sign-off Onda 2 |

---

> **"Quality is not an act, it is a habit."** — Aristotle
> **Enforced by**: cosca-qa (QA Chief) | **Next review**: Após conclusão da Onda 2 (Fase 3) ou 14 dias, o que ocorrer primeiro.
