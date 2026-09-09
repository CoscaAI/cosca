---
type: roadmap
key: onda-2-plan
tags: [onda-2, agent-activation, risk-mitigation, roadmap]
timestamp: 2026-07-28T17:00:00Z
status: approved
author: cosca-ceo
approved_by: Don
depends_on:
  - memory/semantic/INDEX.md (C1)
  - memory/risk/RISK_REGISTRY.md
  - memory/roadmap/platform-evolution-v1.4.0.md
resolves:
  - R1 (41/55 agentes nunca executaram)
  - R2 (confiança média 0.48)
  - R12 (zero cross-agent validation data)
---

# Onda 2 — Plano de Ativação de Agentes

> **Objetivo**: Ativar os 10 agentes L1 seed mais críticos com pelo menos 1 tarefa real cada, registrando learnings e elevando a confiança média da plataforma de 0.48 para ≥ 0.55.
>
> **Escopo**: 10 agentes prioritários selecionados do universo de 41 agentes L1 seed (DNA v3.0, sem execução real). Fora de escopo: agentes L2+ (security, frontend, database) que já têm histórico de execução.
>
> **Duração estimada**: 4 ondas sequenciais dentro de 5-7 dias.

---

## Resumo Executivo

Dos 55 agentes da plataforma, 41 (75%) têm apenas seed data (baseline 0.25). Nenhum deles executou tarefa real. A confiança média da plataforma é 0.48 — abaixo da meta de 0.70. O Risk Registry classifica isso como Risco R1 (crítico, 90% probabilidade, alto impacto).

A Onda 1 ativou 14 agentes (Kernel, Backend, Documentation, Runtime, Security, Frontend, Database, Discovery, Memory Chief, Automation, Semantic Memory, e 4 líderes que foram implicitamente exercitados via decisões de arquitetura). A Onda 2 deve ativar mais 10 agentes críticos, priorizando aqueles com maior impacto na plataforma, mais dependências de outros agentes, e que fecham os gaps de conhecimento mais urgentes.

O critério de ativação é simples: **1+ tarefa real executada, learnings registrados em `learnings.md`, e confidence do domínio primário elevada de 0.25 para ≥ 0.40.**

---

## Critérios de Priorização

Cada agente foi avaliado em 4 dimensões (escala 0-10):

| Dimensão | Peso | Descrição |
|----------|------|-----------|
| **Impacto na plataforma** | 40% | Resolve gap crítico (P0/P1)? Desbloqueia outros agentes? Afeta métricas de confiança? |
| **Dependências de outros agentes** | 25% | Quantos outros agentes dependem deste para executar? É pré-requisito para outras ativações? |
| **Gap de conhecimento** | 20% | O domínio tem cobertura baixa no Semantic Index? É gap crítico (P0) ou importante (P1)? |
| **Rapidez de ativação** | 15% | Tarefa é bem definida? Pode ser executada sem esperar outros agentes? Esforço baixo? |

---

## Top 10 Agentes Prioritários (ordenados por fase de ativação)

---

### FASE 1 — ONDA A: Independentes, sem dependências (execução paralela)

Estes 5 agentes podem ser ativados simultaneamente. Nenhum deles precisa de outputs de outros agentes da Onda 2.

---

#### 1. cosca-qa — Quality Assurance Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 9.4/10 |
| **Impacto**: | 10/10 — Gatekeeper de qualidade. Define standards que todos os outros agentes seguirão. |
| **Dependências**: | 9/10 — cosca-testing, cosca-review, cosca-release, cosca-specialist-testing-* dependem dos standards de QA. |
| **Gap**: | P1 — Estratégia de teste definida mas sem padrões formais de aceitação ou quality gates documentados. |
| **Rapidez**: | 9/10 — Task bem definida, puramente analítica. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Quality assurance (test strategy, acceptance validation, quality metrics) |

**Tarefa Real Proposta**:
```
TASK: Definir quality gates para a plataforma Cosca (G0-G9 aplicados ao ciclo de desenvolvimento)

1. Auditar o bug registry (7 bugs, 3 críticos não resolvidos) e classificar por severidade
2. Definir acceptance criteria para cada tipo de deliverable:
   - Bug fix (ex: Restart() fix) → quais testes mínimos antes do merge?
   - Nova feature → quais gates (security scan, perf benchmark, a11y check)?
   - Infra change (CI pipeline) → quais validações?
3. Produzir documento Quality Gate Standard com:
   - G0-G9 aplicados concretamente (ex: G4 = govulncheck passa, G5 = ≥80% coverage)
   - Thresholds de aceitação por tipo de artefato
   - Critérios de rejeição automática
4. Assinar o plano de qualidade para a Onda 2, validando que os tasks dos outros 9 agentes têm critérios de aceitação definidos
```

**Critérios de Sucesso**:
- [ ] Quality Gate Standard documentado e referenciável por outros agentes
- [ ] Bug registry classificado (severidade, impacto, owner sugerido)
- [ ] 9 tasks da Onda 2 com acceptance criteria definidos
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 2-4 horas (task analítica, sem implementação)

**Depende de**: Nada (apenas acesso ao bug registry e capability profiles)

---

#### 2. cosca-governance — Governance Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 8.7/10 |
| **Impacto**: | 9/10 — Com 55 agentes e 426 arquivos de memória, drift de framework é risco real. Auditoria de contratos previne fragmentação. |
| **Dependências**: | 8/10 — Todos os agentes herdam CONVENTIONS. Se a auditoria falha, agentes futuros podem violar padrões. |
| **Gap**: | P1 — Sem auditoria de conformidade de contratos. 41 agentes L1 seed podem ter templates inconsistentes. |
| **Rapidez**: | 8/10 — Task analítica, mas 55 agentes para auditar é volume alto. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Framework governance (standards, conventions, duplication, evolution) |

**Tarefa Real Proposta**:
```
TASK: Auditar conformidade de todos os 55 agentes com o padrão CONVENTIONS e DNA v3.0

1. Verificar cada capability-profile.md dos 55 agentes para:
   - Campos obrigatórios DNA v3.0 (Current Level, Per-Domain Confidence, Strengths, Weaknesses, Evolution Goal)
   - Consistência de formato com o template padrão
   - Ausência de campos obsoletos (DNA v2.0)
2. Identificar duplicação cross-agent:
   - Agentes com domínios sobrepostos (ex: cosca-performance vs agent-performance-chief.md)
   - Skills duplicadas entre agentes
   - Arquivos órfãos (agent-*.md soltos em memory/agent/ vs diretórios cosca-*/)
3. Verificar cross-reference integrity:
   - Links entre capability profiles quebrados
   - Referências a agentes inexistentes
   - Inconsistências de nomenclatura
4. Produzir Governance Audit Report com:
   - % de conformidade por agente
   - Lista de não-conformidades (severidade: blocker/warning/info)
   - Recomendações de correção
   - Sugestão de remoção/merge de agentes duplicados
```

**Critérios de Sucesso**:
- [ ] 55 capability profiles auditados (100% coverage)
- [ ] Report com ≥ 5 não-conformidades encontradas e classificadas
- [ ] Arquivos órfãos identificados (agent-*.md soltos)
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 3-5 horas (55 agentes para auditar, volume alto mas padrão repetitivo)

**Depende de**: Nada (acesso a todos os capability profiles)

---

#### 3. cosca-technical-debt — Technical Debt Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 8.2/10 |
| **Impacto**: | 8/10 — 7 bugs conhecidos, 3 críticos, sem tracking estruturado. Dívida invisível = risco acumulativo. |
| **Dependências**: | 7/10 — cosca-evolution, cosca-performance, cosca-review usam métricas de dívida para priorização. |
| **Gap**: | P1 — Dívida técnica não quantificada. Bugs catalogados mas não priorizados por custo/benefício. |
| **Rapidez**: | 9/10 — Task bem delimitada, dados já existem (bug registry, semantic index gaps). |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Technical debt management (measurement, prioritization, refactoring) |

**Tarefa Real Proposta**:
```
TASK: Produzir o primeiro Technical Debt Scorecard da plataforma Cosca

1. Catalogar todas as fontes de dívida conhecidas:
   - Bug registry: 7 bugs (3 críticos: Restart(), EventStartupComplete, bug-005)
   - Semantic Index gaps: 17 gaps identificados (8 P0, 4 P1, 5 estruturais)
   - Agent memory gaps: 41 agentes sem learnings, 52 sem failures, 53 sem patterns
   - Code quality: cobertura de testes (~78%, mas 0% integração runtime)
2. Classificar cada item por:
   - Severidade (blocker/critical/major/minor/trivial)
   - Esforço estimado de correção (horas)
   - Impacto se não corrigido (descrição narrativa)
   - Juros técnicos (custo de postergar)
3. Calcular Debt Score composto com pesos:
   - Blocker = 100, Critical = 50, Major = 20, Minor = 5, Trivial = 1
   - Multiplicador de idade (quanto mais antigo, pior)
4. Produzir Top 10 Prioridades de Correção com justificativa de ROI
5. Recomendar cadência de revisão (sugestão: mensal)
```

**Critérios de Sucesso**:
- [ ] Todas as fontes de dívida catalogadas (bugs + gaps + memory + code)
- [ ] Debt Score calculado com baseline inicial
- [ ] Top 10 prioridades ranqueadas com ROI
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 2-3 horas (dados já existem, trabalho é de síntese e priorização)

**Depende de**: Nada (bug registry, semantic index, e agent INDEX já existem)

---

#### 4. cosca-critic — Decision Critic Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 8.0/10 |
| **Impacto**: | 7/10 — Valida o framework de metacognição. Sem crítico ativo, decisões não têm adversarial review. |
| **Dependências**: | 6/10 — Usado pelo Kernel para P0/P1 decisions. cosca-paradigm depende do critic para baseline. |
| **Gap**: | P2 — 5-question challenge definido mas nunca aplicado a uma decisão real. |
| **Rapidez**: | 7/10 — Task criativa, requer análise profunda. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Decision critique (risks, alternatives, scale implications) |

**Tarefa Real Proposta**:
```
TASK: Aplicar o 5-Question Challenge ao plano de ativação Onda 2 (este documento)

1. Aplicar as 5 questões do framework critic a esta decisão:
   Q1: Quais os riscos? (consultar bug registry — quais bugs similares podem ocorrer?)
   Q2: Que alternativa existe? (pelo menos 2 alternativas à ativação sequencial — ex: big bang, gradual por domínio)
   Q3: O que quebra em escala? (se ativarmos 41 agentes em vez de 10, o que falha?)
   Q4: Em que premissa isto se baseia? (a premissa de que 10 agentes elevam confiança para 0.55 é válida?)
   Q5: O que tornaria esta decisão errada em 6 meses? (condições de falha)
2. Cross-reference com:
   - Bug registry: algum bug similar a "ativação em ondas causou race condition"?
   - ADRs: alguma decisão arquitetural conflita com ativação em massa?
   - Risk Registry: R1, R2, R12 são mitigados? R9 (Kernel SPOF) é agravado?
3. Propor pelo menos 2 alternativas viáveis com trade-offs
4. Emitir veredito: APPROVE (com ressalvas) / REJECT (com justificativa) / APPROVE WITH CONDITIONS
```

**Critérios de Sucesso**:
- [ ] 5 questões respondidas com profundidade analítica
- [ ] ≥ 2 alternativas propostas com trade-offs
- [ ] Cross-reference com bug registry, ADRs, e risk registry
- [ ] Veredito fundamentado
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 2-3 horas (task analítica criativa)

**Depende de**: Este plano Onda 2 (já existe). Pode executar imediatamente.

---

#### 5. cosca-compliance — Compliance Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 7.8/10 |
| **Impacto**: | 7/10 — cosca-security já identificou compliance como "aspirational drift". Precisa de assessment real. |
| **Dependências**: | 5/10 — Relatórios de compliance alimentam cosca-security, cosca-governance, e decisões de produto. |
| **Gap**: | P1 — GDPR/SOC2/LGPD são aspiracionais. Nenhuma avaliação real de conformidade foi feita. |
| **Rapidez**: | 8/10 — Self-assessment baseado em checklist. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Regulatory compliance (GDPR, SOC2, HIPAA, PCI-DSS, LGPD) |

**Tarefa Real Proposta**:
```
TASK: Realizar autoavaliação de conformidade GDPR e LGPD para a plataforma Cosca

1. Auditar o estado atual de compliance:
   - Dados pessoais armazenados? (agent memory, session context, knowledge.db)
   - Consentimento documentado?
   - Right to erasure implementado? (cosca-security flag: 0.10 confidence — "não implementado")
   - Data retention policy definida?
   - Audit logs são tamper-proof?
2. Cross-reference com o compliance fix do cosca-security (5 passos aspiracionais):
   - At-rest encryption → implementado? verificar código
   - Audit logging → existe? verificar runtime
   - Data mapping → documentado? verificar schema
   - Retention automation → implementado? verificar memory decay engine
   - Right-to-erasure → endpoint existe? verificar API
3. Produzir Compliance Self-Assessment Report:
   - Score de conformidade GDPR (% dos controles atendidos)
   - Score de conformidade LGPD (% dos controles atendidos)
   - Gaps priorizados por severidade
   - Roadmap de remediação com estimativas de esforço
4. Recomendar: a plataforma está pronta para produção sob GDPR/LGPD? Se não, o que falta?
```

**Critérios de Sucesso**:
- [ ] GDPR self-assessment com ≥ 20 controles verificados
- [ ] LGPD self-assessment com ≥ 15 controles verificados
- [ ] Cross-reference com os 5 passos do cosca-security (status real vs aspiracional)
- [ ] Roadmap de remediação priorizado
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 3-4 horas (requer verificação de código, schema, e configurações)

**Depende de**: Nada (acesso a código, schema, configurações de segurança)

---

### FASE 2 — ONDA B: Dependem dos outputs da Fase 1

Estes 3 agentes precisam de outputs dos agentes da Fase 1 (principalmente QA e Technical Debt).

---

#### 6. cosca-testing — Testing Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 9.6/10 |
| **Impacto**: | 10/10 — Fecha o gap P0 "no runtime integration tests". Valida 5+ agentes simultaneamente (runtime, backend, database, security, monitoring). |
| **Dependências**: | 10/10 — cosca-specialist-testing-unit, integration, e2e dependem do Testing Chief para ativação. |
| **Gap**: | P0 — State machine tem 20 transições não testadas. 3 bugs críticos não validados. |
| **Rapidez**: | 6/10 — Task de implementação (escrever código de teste real). |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Test implementation (unit, integration, E2E, contracts) |

**Tarefa Real Proposta**:
```
TASK: Escrever suite de integração para a state machine do runtime (20 transições) e validar 3 bugs críticos

1. Setup do ambiente de teste:
   - Criar test harness para runtime.Run(), runtime.Stop(), runtime.Restart()
   - Mock de dependências externas (database, providers) para isolamento
   - Seguir padrão AAA (Arrange-Act-Assert) definido pelo cosca-qa

2. Testes de integração para a state machine:
   - Validar todas as 20 transições documentadas em runtime/state.go
   - Foco nas transições críticas:
     - Uninitialized → Initializing → Ready (happy path)
     - Ready → Stopping → Stopped (graceful shutdown)
     - Errored → Recovering → Ready (recovery path)
     - Stopped → Uninitialized → Initializing (Restart — bug conhecido!)

3. Reproduzir e documentar os 3 bugs críticos:
   - Bug Restart(): Stop() deixa state=Stopped, Start() requer Uninitialized → transição quebrada
   - Bug EventStartupComplete: evento dispara antes dos init hooks → subscribers recebem startup prematuro
   - Bug metrics misdocumented: docs dizem 7 métricas, código tem 12 → documentar discrepância real

4. Delegar testes unitários para cosca-specialist-testing-unit (se disponível):
   - Testes de unidade para funções auxiliares (signal handling, pid file, watchdog loop)

5. Produzir Test Report:
   - Cobertura de transições testadas (%)
   - Bugs reproduzidos com casos de teste
   - Recomendações de fix (descrever, não implementar)
   - Flaky tests identificados (se houver)
```

**Critérios de Sucesso**:
- [ ] ≥ 15 das 20 transições cobertas por testes de integração
- [ ] 3 bugs críticos reproduzidos com casos de teste documentados
- [ ] Quality gates do cosca-qa aplicados (AAA pattern, nomes descritivos, sem interdependência)
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40
- [ ] Pelo menos 1 failure registrado (esperado: Restart() bug confirmado como falha)

**Esforço estimado**: 6-10 horas (task de implementação, requer leitura de código e escrita de testes)

**Depende de**: cosca-qa (quality gates e acceptance criteria), cosca-technical-debt (bug prioritization context)

---

#### 7. cosca-performance — Performance Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 9.2/10 |
| **Impacto**: | 9/10 — Fecha gap P0 "no performance profiling". Baseline de performance é pré-requisito para otimização, CI gates, e SLAs. |
| **Dependências**: | 8/10 — cosca-database (L2) precisa dos resultados. cosca-monitoring precisa de métricas de baseline. |
| **Gap**: | P0 — Todas as hipóteses de performance são derivadas de arquitetura, não de medição. Bug-005 sem root cause. |
| **Rapidez**: | 5/10 — Task técnica que requer execução de benchmarks reais. |
| **DNA**: | v3.0.0 (L1, mas já tem learnings de análise estática — confiança parcial 0.70 em SQLite, 0.75 em bug triage) |
| **Domínio primário**: | Performance engineering (profiling, benchmarking, optimization) |

**Tarefa Real Proposta**:
```
TASK: Executar baseline de performance do search pipeline e root-cause do bug-005

1. Go Benchmarking do search pipeline:
   - Benchmark FTS5 queries (BM25 ranking): medir latência p50/p95/p99
   - Benchmark vector similarity (sqlite-vec): medir latência
   - Benchmark result fusion (FTS5 + vector): medir latência combinada
   - Benchmark knowledge index rebuild: medir tempo total
   - Usar go test -bench=. -benchmem -benchtime=10s
   - Gerar relatório com benchstat (comparação entre runs)

2. Memory profiling:
   - Executar pprof heap profile durante search pipeline stress
   - Identificar alocações de memória por operação
   - Detectar possíveis goroutine leaks (goroutine profile)

3. Root-cause do bug-005 (slow dashboard query):
   - Executar EXPLAIN QUERY PLAN na query do dashboard
   - Identificar: falta de índice? full table scan? query não otimizada?
   - Comparar com queries equivalentes que NÃO têm problema de performance
   - Propor fix (com estimativa de ganho)

4. CPU profiling:
   - pprof CPU profile do runtime sob carga sustentada (1000 requisições)
   - Identificar hot paths (funções que consomem mais CPU)
   - Documentar top 5 funções por consumo de CPU

5. Produzir Performance Baseline Report:
   - Tabela de latências por operação (p50/p95/p99)
   - Memory allocation por operação
   - CPU hot paths identificados
   - Bug-005 root cause com EXPLAIN output
   - Recomendações de otimização (top 3 quick wins)
```

**Critérios de Sucesso**:
- [ ] ≥ 3 benchmarks executados com dados de latência (p50/p95/p99)
- [ ] Memory profile com alocações identificadas por operação
- [ ] Bug-005 root cause identificada com EXPLAIN QUERY PLAN
- [ ] Top 5 CPU hot paths documentados
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40
- [ ] At least 1 failure registrado (expected: bug-005 root cause is a real performance issue)

**Esforço estimado**: 6-8 horas (requer execução de ferramentas de profiling e análise de resultados)

**Depende de**: cosca-technical-debt (priorização do bug-005), cosca-qa (acceptance criteria para benchmarks)

---

#### 8. cosca-devops — DevOps Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 9.0/10 |
| **Impacto**: | 9/10 — Fecha gap P0 "no CI/CD automation". Pipeline é backbone para security scanning, test gates, build validation. |
| **Dependências**: | 9/10 — cosca-security, cosca-testing, cosca-review, cosca-release, cosca-monitoring dependem do CI. |
| **Gap**: | P0 — 100% deploy manual. Sem quality gates automáticos. Sem CI para security scanning. |
| **Rapidez**: | 6/10 — Task de infraestrutura. GitHub Actions é bem conhecido mas requer configuração cuidadosa. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Delivery pipeline (CI/CD, containers, IaC, environments) |

**Tarefa Real Proposta**:
```
TASK: Projetar e implementar pipeline CI/CD com GitHub Actions para a plataforma Cosca

1. Pipeline CI (por PR e push para main):
   a. Lint & Format:
      - golangci-lint (com configuração adaptada ao código Cosca)
      - gofmt / goimports check
      - Prettier para frontend (TSX/TS)

   b. Security Scanning (stage preparado para cosca-security integrar):
      - govulncheck ./...
      - gosec ./... (com configuração inicial)
      - Placeholder para semgrep (a ser configurado pelo cosca-security)

   c. Test Suite:
      - go test -race -coverprofile=coverage.out ./...
      - Vitest para frontend (npm test)
      - Verificação de coverage threshold (≥ 70% inicial, subindo para 80%)

   d. Build Validation:
      - go build ./...
      - npm run build (Next.js)
      - Docker image build (smoke test)

   e. Doc-Code Validator (resolve R8):
      - Script que cruza referências em docs/ com paths reais no código
      - Alerta se documentação referencia arquivo inexistente

2. Pipeline CD (por tag/release):
   a. Build Docker images (multi-stage)
   b. Push para registro de container
   c. Helm chart lint e validação
   d. Deploy em staging (manual approval gate)
   e. Smoke tests pós-deploy
   f. Rollback automático se smoke tests falharem

3. Notificações:
   - Falha de CI → notificar no PR
   - Falha de CD → alerta no canal de monitoring
   - Success → badge de build no README

4. Documentação:
   - CI/CD pipeline documentado no diretório .github/workflows/
   - README com badges de build, coverage, security
   - Runbook de troubleshooting para falhas comuns
```

**Critérios de Sucesso**:
- [ ] CI pipeline funcional com ≥ 4 stages (lint, test, build, doc-validate)
- [ ] CD pipeline definido (deploy + rollback)
- [ ] Pelo menos 1 build bem-sucedido com todos os gates passando
- [ ] Doc-code validator integrado (resolve R8 parcialmente)
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40
- [ ] Pelo menos 1 failure registrado (esperado: lint ou test inicial vai falhar e precisar de ajustes)

**Esforço estimado**: 8-12 horas (task de infraestrutura, múltiplos estágios de pipeline)

**Depende de**: cosca-qa (quality gates para o pipeline), cosca-testing (testes precisam existir para o CI validar)

---

### FASE 3 — ONDA C: Dependem de implementações da Fase 2

Estes 2 agentes precisam que implementações existam para poderem agir.

---

#### 9. cosca-review — Review Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 8.3/10 |
| **Impacto**: | 8/10 — Valida a qualidade do código produzido pelos outros agentes. Sem review, bugs de implementação passam despercebidos. |
| **Dependências**: | 7/10 — Código precisa existir para ser revisado. Depende dos outputs da Fase 2. |
| **Gap**: | P1 — Nenhuma revisão de código sistemática foi feita. 7 bugs conhecidos sem revisão de causa raiz. |
| **Rapidez**: | 7/10 — Task de revisão (leitura de código, não implementação). |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Code review (quality, architecture compliance, security, standards) |

**Tarefa Real Proposta**:
```
TASK: Revisar as implementações críticas produzidas na Onda 2

1. Revisar o CI pipeline (cosca-devops):
   - SOLID principles aplicados? (configuração modular?)
   - Secrets management: tokens expostos? hardcoded secrets?
   - Performance: jobs paralelizáveis? caching adequado?
   - Error handling: falhas de stage têm mensagens claras?
   - DRY: duplicação entre workflows?

2. Revisar os testes de integração (cosca-testing):
   - AAA pattern seguido?
   - Cobertura de edge cases (transições inválidas, timeouts)?
   - Test isolation: testes independentes?
   - Nomenclatura: nomes descritivos?
   - Flakiness potential: race conditions nos testes?

3. Revisar configurações de segurança do CI:
   - Permissions mínimas nos workflows?
   - Secrets rotacionáveis?
   - Nenhum token exposto em logs?

4. Produzir Review Report:
   - Issues encontrados (severidade: blocker/critical/major/minor)
   - Recomendações de melhoria (específicas, com exemplos de código)
   - Aprovação condicional: quais issues precisam ser resolvidas antes do merge?
   - Pontos positivos: o que foi bem feito
```

**Critérios de Sucesso**:
- [ ] ≥ 2 artefatos revisados (CI pipeline + integration tests)
- [ ] ≥ 5 issues encontrados (esperado em primeira implementação)
- [ ] Checklist aplicado: SOLID, security, performance, tests, error handling, DRY
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40
- [ ] Pelo menos 1 padrão identificado como "a evitar" registrado em `patterns.md`

**Esforço estimado**: 3-5 horas (revisão de código, não implementação)

**Depende de**: cosca-devops (CI pipeline), cosca-testing (integration tests)

---

#### 10. cosca-monitoring — Monitoring Chief

| Atributo | Valor |
|----------|-------|
| **Score**: | 8.8/10 |
| **Impacto**: | 9/10 — Fecha gap P0 "no observability". Sem SLOs e Prometheus, plataforma é cega em produção. |
| **Dependências**: | 8/10 — cosca-runtime (métricas), cosca-performance (baselines), cosca-devops (CI pipeline para alertas). |
| **Gap**: | P0 — Sem OpenTelemetry, sem Prometheus /metrics endpoint, sem SLOs definidos. |
| **Rapidez**: | 5/10 — Task técnica que requer integração com runtime e definição de métricas. |
| **DNA**: | v3.0.0 (L1 seed, 0.25) |
| **Domínio primário**: | Observability (metrics, logging, alerting, SLOs, tracing) |

**Tarefa Real Proposta**:
```
TASK: Definir SLOs e implementar export de métricas Prometheus para a plataforma Cosca

1. Definir SLOs para 5 critical user journeys:
   a. Agent task routing (Kernel recebe task → roteia para Chief): p95 < 500ms
   b. Knowledge search (FTS5 + vector fusion): p95 < 200ms
   c. Session bootstrap (Phase 0-4): p95 < 5s
   d. API response (REST endpoint): p95 < 100ms
   e. Memory retrieval (PATH-based): p95 < 50ms

   Para cada SLO: definir SLI (métrica concreta), target (%), e error budget

2. Implementar Prometheus /metrics endpoint:
   - Expor 8 atomic counters do runtime como Prometheus gauges
   - Expor 4 durationHistograms como Prometheus histograms
   - Adicionar métricas de negócio:
     - cosca_tasks_total{agent, status}
     - cosca_agent_confidence{agent, domain}
     - cosca_memory_files_total{type}
     - cosca_session_duration_seconds

3. Dashboard Grafana (como código, JSON exportável):
   - Health overview: agent status, task throughput, error rate
   - Performance: search latency, API latency, bootstrap time
   - Quality: test coverage, lint errors, security vulnerabilities
   - Memory: file count, decay rate, curation score

4. Alerting Rules:
   - Error rate > 5% por 5min → alerta
   - P95 latency > 2x baseline por 10min → alerta
   - Agent confidence drop > 0.20 → alerta
   - Memory decay rate > 10%/ciclo → alerta

5. Integrar com CI pipeline:
   - Prometheus metrics validation no CI (endpoint responde 200?)
   - Alert rules testáveis (promtool test rules)
```

**Critérios de Sucesso**:
- [ ] 5 SLOs definidos com SLI, target, e error budget
- [ ] Prometheus /metrics endpoint funcional com ≥ 10 métricas
- [ ] Dashboard Grafana definido (JSON)
- [ ] ≥ 3 alerting rules configuradas
- [ ] Learning registrado em `learnings.md` com confiança ≥ 0.40

**Esforço estimado**: 6-10 horas (task técnica, requer integração com runtime e Prometheus)

**Depende de**: cosca-performance (baselines de latência para calibrar SLOs), cosca-runtime (métricas atômicas existentes), cosca-qa (acceptance criteria para SLOs)

---

## Ordem de Ativação (Resumo Visual)

```
FASE 1 — ONDA A (dia 1-2, paralelo)
├── cosca-qa          ──► Define quality gates
├── cosca-governance  ──► Audita 55 agentes
├── cosca-technical-debt ─► Scorecard de dívida
├── cosca-critic      ──► Critica o plano Onda 2
└── cosca-compliance  ──► Self-assessment GDPR/LGPD
         │
         │ (outputs: quality standards, bug priorities, governance report)
         ▼
FASE 2 — ONDA B (dia 2-5, parcialmente paralelo)
├── cosca-testing     ──► Integration tests (state machine)
├── cosca-performance ──► Benchmarks + bug-005 root cause
└── cosca-devops      ──► CI/CD pipeline
         │
         │ (outputs: test suite, perf baselines, CI pipeline)
         ▼
FASE 3 — ONDA C (dia 5-7, paralelo)
├── cosca-review      ──► Revisa CI + testes
└── cosca-monitoring  ──► SLOs + Prometheus
```

---

## Estimativa de Esforço por Agente

| # | Agente | Esforço (h) | Complexidade | Tipo de Task |
|---|--------|-------------|-------------|-------------|
| 1 | cosca-qa | 2-4 | Baixa | Analítica |
| 2 | cosca-governance | 3-5 | Média | Auditoria |
| 3 | cosca-technical-debt | 2-3 | Baixa | Síntese |
| 4 | cosca-critic | 2-3 | Média | Analítica criativa |
| 5 | cosca-compliance | 3-4 | Média | Auditoria + verificação |
| 6 | cosca-testing | 6-10 | Alta | Implementação |
| 7 | cosca-performance | 6-8 | Alta | Técnica (profiling) |
| 8 | cosca-devops | 8-12 | Alta | Infraestrutura |
| 9 | cosca-review | 3-5 | Média | Revisão |
| 10 | cosca-monitoring | 6-10 | Alta | Técnica (instrumentação) |
| **TOTAL** | **41-64h** | | | |

**Carga paralelizável**: Ondas A (5 agentes simultâneos) + Onda B (3 agentes, 2 parcialmente independentes). Tempo de relógio estimado: **5-7 dias** com execução paralela.

---

## Critérios de Sucesso da Onda 2

### Por Agente (micro)
- [ ] Cada agente executou ≥ 1 task real
- [ ] Learnings registrados em `learnings.md` (formato LEARNING_PROTOCOL.md)
- [ ] Confidence do domínio primário elevada de 0.25 para ≥ 0.40
- [ ] Pelo menos 3 dos 10 agentes registram failures (aprender com erros é sinal de maturidade)
- [ ] Padrões identificados registrados em `patterns.md` (≥ 2 agentes)

### Por Plataforma (macro)

| Métrica | Baseline (C1) | Target (pós-Onda 2) | Método de Medição |
|---------|--------------|---------------------|-------------------|
| Agentes com learnings substantivos | 10 (18%) | 20 (36%) | Contagem de `learnings.md` com ≥ 1 entrada |
| Agentes com failures registrados | 3 (5%) | 8 (15%) | Contagem de `failures.md` com ≥ 1 entrada |
| Confiança média | 0.48 | ≥ 0.55 | Média das confidences primárias dos 55 agentes |
| Agentes ≥ L2 | 10 | 20 | Contagem de capability profiles com Level ≥ 2 |
| Gaps P0 fechados | 0 de 8 | ≥ 4 de 8 | Semantic Index gap tracker |
| Cross-agent validation (M2) | 0 | ≥ 2 validações | Agentes validando outputs uns dos outros |

---

## Riscos e Mitigações

| Risco | Prob. | Impacto | Mitigação |
|-------|-------|---------|-----------|
| Task de implementação (testing, devops, performance) subestimar esforço | Alta | Médio | Tasks divididas em subtasks. Escopo pode ser reduzido (ex: testar 10 transições em vez de 20) sem perder critério de ativação. |
| Agente falhar na primeira task (confidence não sobe) | Média | Alto | Failure registrado é aprendizado válido. Confiança pode subir mesmo com falha (o agente aprendeu o que NÃO fazer). |
| Dependências bloquearem Onda B (QA atrasa → testing não começa) | Média | Médio | Onda A são 5 agentes em paralelo. Se QA atrasar, testing pode começar com standards provisórios (especificar no task). |
| Conflitos de domínio entre agentes (ex: DevOps vs Security vs Monitoring) | Baixa | Baixo | Cada agente tem escopo bem delimitado. Conflitos cross-domain são encaminhados ao CEO para arbitragem. |
| Agente gerar output de baixa qualidade (task "cumprida" mas sem valor) | Média | Médio | cosca-review (Fase 3) valida outputs das Fases 1-2. cosca-qa valida critérios de aceitação. |

---

## Próximos Passos (pós-Onda 2)

1. **Onda 3**: Ativar especialistas: cosca-specialist-testing-unit, integration, e2e; cosca-specialist-backend-api, service; cosca-specialist-database-sql; cosca-specialist-documentation-writer; cosca-specialist-frontend-component; cosca-specialist-review-code (10 agentes)
2. **Onda 4**: Ativar domínios especializados: cosca-sdk, cosca-cli, cosca-plugin, cosca-cache, cosca-messaging, cosca-migration, cosca-integrations, cosca-workflow-chief (8 agentes)
3. **Onda 5**: Ativar domínios de negócio/estratégia: cosca-ai, cosca-analytics, cosca-mobile, cosca-infrastructure, cosca-platform, cosca-provider, cosca-semantic-memory (7 agentes)
4. **Onda 6**: Ativar liderança e metacognição restante: cosca-ceo (já ativado por este plano), cosca-cto, cosca-product, cosca-architecture, cosca-evolution, cosca-paradigm (gated), cosca-context, cosca-bootstrap (6 agentes)
5. **Recalibrar**: Após Onda 2 completa, re-executar semantic indexing (Ciclo 2) e recalcular todos os scores de confiança

---

> **Aprovado por**: cosca-ceo (CEO Agent) em 2026-07-28
> **Próxima revisão**: Após conclusão da Fase 3 (Onda C) ou 7 dias, o que ocorrer primeiro.
> **Métricas serão publicadas em**: memory/roadmap/onda-2-results.md
