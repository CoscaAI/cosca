# cosca-kernel — Reusable Patterns

> Discovered patterns that can be reapplied across tasks, projects, and domains.
>
> **Versioned with**: [Pattern Evolution Engine](../../../engines/pattern-evolution/SKILL.md) — lifecycle, versioning, health metrics, deprecation protocol.
>
> **Canonical source**: `internal/embed/cosca/engines/pattern-evolution/patterns/active/*.yaml` — YAML completo com evolution trail.
> **Este arquivo**: Visão simplificada para consumo rápido por agentes e humanos.

## Patterns Discovered

---

### Pattern 007: Session-Concurrency Safe Editing

> **Versão atual**: 1.0.0 | **Lifecycle**: PROPOSED | **ID**: PATTERN-007 | **Confidence**: 0.96

**Domain**: Git safety, multi-session workspaces, delegated implementation
**Level**: 4

**Context**: Quando mais de uma sessão, agente ou processo pode modificar o mesmo workspace.

**Solution**:
```text
1. Antes de editar: git status --short + diff por arquivo.
2. Identificar arquivos mistos e marcar ownership incerto.
3. Capturar o diff da sessão concorrente antes de qualquer restore/reset.
4. Nunca restaurar arquivo inteiro compartilhado.
5. Stage somente arquivos próprios ou hunks explicitamente próprios.
6. Validar staged diff, testes e status final.
7. Se a origem não puder ser determinada: parar e perguntar ao Don.
```

**Known Pitfalls**:
- `git restore HEAD -- arquivo` apaga trabalho não commitado de outra sessão.
- `git add arquivo` pode incluir alterações alheias em arquivo misto.
- Um commit verde em arquivos errados ainda é uma violação de ownership.

**Result**: Preserva trabalho concorrente e impede que correções de uma sessão destruam outra.

---

### Pattern 001: Cross-Agent Parallel Audit

> **Versão atual**: 4.0.0 | **Lifecycle**: STABLE | **ID**: PATTERN-001 | **Confidence**: 0.95 | **Health**: EXCELLENT

**Domain**: Orchestration, Auditing, Quality Assurance
**Level**: 4
**Times Applied**: 4 (L18 Runtime Coverage, L19 CLI Coverage, L20 Systemic Platform, L21 Coverage + Doc Expurgo)
**Times Succeeded**: 4
**Owner**: cosca-kernel
**First Extracted**: 2026-07-29
**Last Validated**: 2026-07-30

**Context**: When you need a comprehensive assessment of a system across multiple dimensions (coverage, security, architecture, docs, etc.).

**Solution**:
```
1. Deploy 4 agents IN PARALLEL:
   - cosca-discovery: maps codebase structure, line counts, package boundaries
   - cosca-qa: audits quality gates, thresholds, governance compliance
   - cosca-testing: runs real coverage/tests with -race flag
   - cosca-architecture: identifies architectural barriers and patterns

2. Kernel aggregates results into unified report.

3. Present findings → Don approves attack plan.

Result: Complete audit in < 3 minutes (vs hours sequentially).
```

**Known Pitfalls**:
- Agents need complete context in prompt — dependencies between agents don't require sequential execution if context is provided upfront.
- Discovery must finish before Testing can run real coverage (Testing needs the map). This is the only sequential dependency.
- Use structured output format requirement in prompts for consistent aggregation.

**Version History (full evolution trail)**:

| Version | Date | Lifecycle | Agents | Trigger | Change | Validated By |
|---------|------|-----------|--------|---------|--------|--------------|
| **1.0.0** | 2026-07-29 | DRAFT | 3 (discovery, qa, testing) | L18 — Runtime Coverage Audit | Extração inicial do padrão cross-agent. Discovery mapeou 3.623 linhas, QA identificou threshold crisis, Testing rodou cobertura real com -race. Resultado em < 3 min. | cosca-kernel |
| **2.0.0** | 2026-07-29 | STABLE | 4 (+architecture) | L19 — CLI Coverage Breakthrough | Adicionada dimensão arquitetural. Architecture identificou 13 barreiras de testabilidade (3 P0, 5 P1, 5 P2). Padrão extract-then-test emergiu desta auditoria. | cosca-kernel |
| **3.0.0** | 2026-07-29 | STABLE | 8 (discovery, architecture, qa, security, devops, docs, memory-chief, technical-debt) | L20 — Systemic Platform Audit | Escala máxima: 8 agentes, 10 dimensões. Auditoria completa em < 5 min. Síntese consolidada em nota 6.8/10. Experimento valioso de escala. | cosca-kernel |
| **4.0.0** ★ | 2026-07-30 | STABLE | 4 (discovery, qa, testing, architecture) | L21 — Coverage Audit + Doc Expurgo | ★ Cognitive Economy consolidation. Análise cost×value: 4 agentes capturam 95% do valor com 50% do custo de 8. Este é o sweet spot. Domínios adicionais (security, devops, docs) são auditados separadamente quando necessário. | cosca-kernel |

**Evolution Tree**:
```
v1.0.0 (DRAFT, 3 agents) ── L18
  └── v2.0.0 (STABLE, 4 agents) ── L19
        ├── v3.0.0 (STABLE, 8 agents) ── L20  [SUPERSEDED — escala excessiva]
        │     └── v4.0.0 (STABLE, 4 agents) ── L21  [★ CURRENT]
        └── (v2.0.0 permanece como referência histórica)
```

**Health Metrics**:
- **Freshness**: 1.00 (0 dias desde última validação)
- **Velocity**: 4.0 versões/mês (fase de descoberta — espera-se estabilização para 0.5-1.0/mês)
- **Staleness Risk**: NONE
- **Adoption**: 4 usos, 1 agente (kernel como orquestrador — padrão de orquestração, não de execução)
- **Success Rate**: 1.00 (4/4)

**Key Insight (v3→v4)**: A transição de 8→4 agentes não foi um retrocesso — foi maturidade. Cognitive Economy analysis provou que "mais agentes" não significa "melhor auditoria". Discovery + QA + Testing + Architecture é a combinação ótima. Este padrão agora incorpora o princípio de economia cognitiva como parte de sua definição.

---

### Pattern 002: Extract-Then-Test (Refactor for Testability)

> **Versão atual**: 2.0.0 | **Lifecycle**: STABLE | **ID**: PATTERN-002 | **Confidence**: 0.90 | **Health**: GOOD

**Domain**: Testing, Refactoring, Coverage
**Level**: 4
**Times Applied**: 2 (L19 CLI Coverage, applicable to chat.go/config.go/doctor.go)
**Times Succeeded**: 2
**Owner**: cosca-kernel
**First Extracted**: 2026-07-29
**Last Validated**: 2026-07-30

**Context**: When a large function (monolith) has near-zero test coverage because it's too complex to test directly.

**Solution**:
```
1. Identify extractable BLOCKS within the monolith:
   - Pure functions (input → output, no side effects)
   - Small configuration blocks (~10-20 lines)
   - Isolated logic that can stand alone

2. EXTRACT each block as an independent function:
   - loadDotEnv() — ~15 lines extracted from runServe()
   - resolveDataDir() — ~10 lines
   - configureCORSFromEnv() — ~3 lines

3. Test each extracted function with table-driven tests:
   - Happy path
   - Edge cases
   - Error conditions

4. Refactor monolith to call extracted functions.

5. Test monolith as INTEGRATION (not unit):
   - Spin up real server with temp dir
   - Send SIGINT
   - Verify graceful shutdown

Result: runServe 0.5% → 61.8% coverage. CLI total 68.5% → 71.5%.
```

**Known Pitfalls**:
- Do NOT try to test the monolith directly — you'll write fragile tests that break on any refactor.
- Extract FIRST, test SECOND. The order matters.
- Keep extracted functions small (10-20 lines). Small = trivial to cover. Large = impossible.
- Integration test for the monolith (not unit test) — test behavior, not implementation.

**Anti-Pattern**: "Testar-para-cobrir" — writing tests against the monolith just to hit coverage numbers. These tests are brittle, slow, and don't actually verify correctness.

**Version History (full evolution trail)**:

| Version | Date | Lifecycle | Trigger | Change | Validated By |
|---------|------|-----------|---------|--------|--------------|
| **1.0.0** | 2026-07-29 | DRAFT | L19 — CLI Coverage Breakthrough | Extração inicial do padrão. 5 passos: identificar → extrair → testar → refatorar → verificar. runServe 0.5%→61.8%. | cosca-kernel |
| **2.0.0** ★ | 2026-07-30 | STABLE | L21 — Coverage Audit + Doc Expurgo | Passo 5 expandido: integration test com servidor real + SIGINT + graceful shutdown verification. Padrão validado em segundo contexto (coverage audit do L21, não só refactor do L19). Anti-pattern "testar-para-cobrir" documentado como negative knowledge. | cosca-kernel |

**Evolution Tree**:
```
v1.0.0 (DRAFT) ── L19 (CLI refactor específico)
  └── v2.0.0 (STABLE) ── L21 (★ CURRENT — padrão generalizado + integração com cross-agent-audit)
```

**Health Metrics**:
- **Freshness**: 1.00 (0 dias desde última validação)
- **Velocity**: 2.0 versões/mês
- **Staleness Risk**: NONE
- **Adoption**: 2 usos, 1 agente. Aplicável a 3 targets pendentes (chat.go, config.go, doctor.go)
- **Success Rate**: 1.00 (2/2)

**Pending Application Targets**:
- `chat.go` (11.6% coverage) — múltiplos blocos extraíveis nos handlers de chat
- `config.go` edit (30% coverage) — lógica de validação de config extraível
- `doctor.go` checks (55-58% coverage) — funções de verificação independentes

---

### Pattern 003: Three-Wave Parallel Activation

> **Versão atual**: 3.0.0 | **Lifecycle**: STABLE | **ID**: PATTERN-003 | **Confidence**: 0.88 | **Health**: GOOD

**Domain**: Orchestration, Agent Activation, Project Bootstrap
**Level**: 3
**Times Applied**: 3 (Onda 2, Onda 5, Onda 6)
**Times Succeeded**: 3
**Owner**: cosca-kernel
**First Extracted**: 2026-07-28
**Last Validated**: 2026-07-28

**Context**: When activating multiple agents for the first time, especially seed agents with no execution history.

**Solution**:
```
Wave A: Analytical agents FIRST (3-5 agents)
  - Define standards, quality gates, benchmarks
  - Establish baseline metrics
  - Create reference documentation

Wave B: Implementation agents SECOND (3-5 agents)
  - Build with standards from Wave A
  - Generate tests, CI pipelines, dashboards
  - Use structured prompts with context from Wave A outputs

Wave C: Review agents THIRD (2-3 agents)
  - Validate against Wave A standards
  - Cross-reference findings
  - Produce synthesis report

Result: 54/55 agents activated (98%), CMI gains across all domains.
```

**Known Pitfalls**:
- First-execution failures in Wave B are VALUABLE — they generate learnings and negative memory.
- Don't skip Wave A — implementation without standards produces inconsistent quality.
- Wave C needs Wave A's standards AND Wave B's outputs to cross-reference effectively.
- Provide complete context in each wave's prompts — agents in later waves need to know what earlier waves produced.

**Version History (full evolution trail)**:

| Version | Date | Lifecycle | Agents | Trigger | Change | Validated By |
|---------|------|-----------|--------|---------|--------|--------------|
| **1.0.0** | 2026-07-28 | DRAFT | 10 agentes (Onda A: 5, B: 3, C: 2) | Onda 2 — 10-Agent Parallel Activation | Extração inicial. 3 ondas (analítica→implementação→revisão). 20+ arquivos, 34 benchmarks, CI/CD pipeline. | cosca-kernel |
| **2.0.0** | 2026-07-28 | STABLE | 6 business agents | Onda 5 — Multi-Agent Activation Wave | Padrão aplicado a domínio diferente (business agents, não seed agents). Cross-audit synthesis emergiu naturalmente — platform correlacionou achados de infra e provider. 47/55 agentes ativos (85%). | cosca-kernel |
| **3.0.0** ★ | 2026-07-28 | STABLE | 8 agentes (liderança + órfãos) | Onda 6 — Liderança + Órfãos Activation Wave | Escalado para incluir agentes de liderança (cto, product, memory-chief, ceo). 7/8 ativados, 1 gated (paradigm — requer 3 meses de Confidence Model). 54/55 agentes ativos (98%). | cosca-kernel |

**Evolution Tree**:
```
v1.0.0 (DRAFT, 10 seed agents) ── Onda 2
  ├── v2.0.0 (STABLE, 6 business agents) ── Onda 5
  │     └── v3.0.0 (STABLE, 8 leadership agents) ── Onda 6  [★ CURRENT]
  └── (v1.0.0 SUPERSEDED)
```

**Health Metrics**:
- **Freshness**: 0.98 (~2 dias desde última validação)
- **Velocity**: 1.5 versões/mês (em apenas 1 dia de evolução — ritmo de descoberta)
- **Staleness Risk**: NONE
- **Adoption**: 3 usos (Onda 2, Onda 5, Onda 6)
- **Success Rate**: 1.00 (3/3)

**Key Insight (v2→v3)**: O padrão provou-se aplicável a diferentes TIPOS de agentes (seed, business, leadership) sem mudança na estrutura de 3 ondas. A evolução não foi na estrutura, mas na escala e diversidade dos agentes ativados. O gating do cosca-paradigm (Onda 6, v3.0.0) validou que o padrão lida corretamente com pré-condições de ativação — agentes com gates legítimos são identificados na Wave A e não são forçados nas Waves B/C.

---

### Pattern 004: Backup Recovery — Restauração Cirúrgica com Diff Direcional

> **Versão atual**: 1.0.0 | **Lifecycle**: ACTIVE | **ID**: PATTERN-004 | **Confidence**: 0.96 | **Health**: EXCELLENT

**Domain**: Memory Recovery, Disaster Recovery, Audit, Orchestration
**Level**: 4
**Times Applied**: 1 (2026-07-31 — Restauração de memória completa, L41)
**Times Succeeded**: 1
**Owner**: cosca-kernel
**First Extracted**: 2026-07-31
**Last Validated**: 2026-07-31

**Context**: Workspace perdeu memória de agentes (54 dirs), 13 diretórios de memória, knowledge.db vazio, e links quebrados. Dois candidatos de fonte existem: snapshot (incompleto — parava em L17) e backup completo (cosca-test-bk--noop — 37 learnings L9-L40).

**Solution**:
```
1. ESCOLHER A FONTE CERITA (decisão do Don)
   - Comparar cobertura: snapshot vs backup por contagem de artefatos
   - O backup vence quando tem mais conteúdo REAL (142KB vs nada)
   - Snapshot é último recurso, nunca fonte primária

2. DIFF DIRECIONAL (backup → workspace)
   - diff -rq para mapear: (a) arquivos só no backup, (b) só no workspace, (c) nos dois com conteúdo diferente
   - Para (c): comparar mtimes — o mais novo vence, MAS verificar semântica
   - NUNCA copiar cego: decidir por arquivo o que restaura e o que preserva

3. RESTAURAR POR BLOCOS (ordem de dependência)
   - Bloco 1: memory/agent/ (a fundação — sem ela nada indexa)
   - Bloco 2: diretórios de memória ausentes (13 dirs)
   - Bloco 3: arquivos divergentes (mais novos do backup)
   - Bloco 4: extras (skills, scaffold, sdk)
   - Bloco 5: knowledge.db (runtime semântico)
   - Verificar cada bloco com contagens antes de avançar

4. PRESERVAR O QUE O WORKSPACE TEM DE MAIS NOVO
   - sync.go/sync_test.go: workspace 05:26 tinha anti-loop guard; backup 04:21 tinha a versão antiga
   - Diferença de mtime + conteúdo = workspace correto. Backup NÃO regride framework
   - Registros históricos (CHANGELOG v3.0.1, audits antigos): não tocar — são verdade histórica

5. CORRIGIR LINKS HERDADOS
   - Links quebrados herdam do backup: 179 no total
   - Resolução automática: basename → relpath canônico (77 correções)
   - Padrões dominantes: profundidade errada (../../../LEARNING_PROTOCOL, memorize-commit com ../ a menos)
   - Falsos positivos: artefatos de runtime (auctions, predictions, gate-results), code blocks YAML

6. REGISTRAR (protocolo stages 7-8)
   - Learning (L41), pattern (este), evolution.md, INDEX.md, CHANGELOG
```

**Why It Works**:
- O diff direcional transforma "copiar tudo" em "decidir por evidência"
- A verificação por bloco evita cascata de erros (o que matou o outro Kernel)
- Preservar o mais novo do workspace impede regressão de segurança (anti-loop guard)

**Evidence**:
- **Adoption**: 1 uso (L41 — restauração completa 2026-07-31)
- **Success Rate**: 1.00 (1/1)
- **Impact**: 54 agentes com memória, 37 learnings do kernel recuperados, 389+ arquivos, knowledge.db 459 docs/8.370 chunks, links 179→4, permissões corrigidas

**Key Insight (v1)**: A ordem do Don foi precisa — "não usa o snapshot, utiliza o backup". O snapshot era um save point de emergência (parava em L17), mas o backup tinha a memória viva (L9-L40 com tags, confidence, next actions). Lição: sempre comparar cobertura de fontes antes de restaurar, e confirmar a fonte com o Don antes de executar.

---

---
### Pattern 005: Evidence-Bounded Competitive Synthesis

> **Lifecycle**: CANDIDATE | **ID**: PATTERN-005 | **Owner**: cosca-kernel | **First Extracted**: 2026-08-04

**Domain**: Competitive Research, Architecture, Evidence Calibration
**Level**: 4

**Context**: Pesquisa profunda que compara o Cosca com frameworks, repositórios ou variantes de produto sem transformar claims externos em fatos nem sugerir adoção cega.

**Solution**:
```
1. Fixar cada snapshot/repositório por SHA e registrar escopo e variante.
2. Separar cada achado em FATO, INFERÊNCIA e HIPÓTESE.
3. Comparar contratos comportamentais (durabilidade, ownership, idempotência,
   sandbox, memória, ACL e recuperação), não apenas listas de features.
4. Marcar ausência de evidência como desconhecido; não converter ? em score baixo.
5. Extrair cross-pollination seletiva como experimento, sem copiar dependências
   nem relaxar invariantes de segurança e governança.
6. Consolidar recomendações em artefato separado, sem alterar código de produto.
```

**Known Pitfalls**:
- Taxonomias diferentes não são globalmente comparáveis; preservar o rótulo de origem e declarar a harmonização.
- “Persistência” pode significar histórico, snapshot, checkpoint, durable run ou efeito externo confirmado; não prometer exactly-once/resume sem distinguir as semânticas.
- Se a primeira redação travar/cancelar, reduzir o escopo e consolidar em uma segunda passada, registrando a falha operacional.

**Reusable Output**: matriz com citações/SHAs, inconsistências, hipóteses, contratos comparados, gaps priorizados e experimentos de validação.

---

### Pattern 008: Manual-First Uncertainty Handling

> **Versão atual**: 1.0.0 | **Lifecycle**: PROPOSED | **ID**: PATTERN-008 | **Confidence**: 0.96

**Domain**: Epistemic governance, decision hygiene, operational documentation

**Context**: Quando existe dúvida, contradição, informação incompleta ou risco de agir sobre uma hipótese.

**Solution**:
```text
1. Consultar o Manual de Autoajuda do Cosca.
2. Formular o objetivo e o critério de sucesso.
3. Separar fatos, conhecimento recuperado, inferências, hipóteses e desconhecidos.
4. Comparar evidências por procedência, recência, escopo e reprodução.
5. Consultar especialista quando o risco ou domínio exigir.
6. Escolher ação reversível e observável quando a incerteza persistir.
7. Escalar ao Don para risco alto, conflito de autoridade ou ação irreversível.
```

**Invariante**: o manual orienta o processo, mas não substitui código executado,
testes, evidência rastreável ou a autoridade do Don.

---

## Pattern Registry Summary

| ID | Name | Version | Lifecycle | Health | First Extracted | Last Validated |
|----|------|---------|-----------|--------|-----------------|-----------------|
| PATTERN-001 | cross-agent-audit | 4.0.0 | STABLE | EXCELLENT | 2026-07-29 | 2026-07-30 |
| PATTERN-002 | extract-then-test | 2.0.0 | STABLE | GOOD | 2026-07-29 | 2026-07-30 |
| PATTERN-003 | three-wave-activation | 3.0.0 | STABLE | GOOD | 2026-07-28 | 2026-07-28 |
| PATTERN-004 | backup-recovery | 1.0.0 | ACTIVE | EXCELLENT | 2026-07-31 | 2026-07-31 |
| PATTERN-005 | evidence-bounded-competitive-synthesis | candidate | — | — | 2026-08-04 | 2026-08-04 |
| PATTERN-008 | manual-first-uncertainty | 1.0.0 | PROPOSED | — | 2026-08-04 | 2026-08-04 |
| PATTERN-009 | cognitive-continuity-benchmark | 1.0.0 | PROPOSED | — | 2026-08-04 | 2026-08-04 |

> **Protocol**: [Pattern Evolution Engine](../../../engines/pattern-evolution/SKILL.md) — versionamento, ciclo de vida, métricas de saúde e protocolo de depreciação.
> **Canonical YAML**: `internal/embed/cosca/engines/pattern-evolution/patterns/active/*.yaml` — fonte canônica com evolution trail completo.
> **Pipeline**: Stages 7-8 do [Cognitive Audit Loop](../../../engines/gap-detection/SKILL.md) — EXTRACT PATTERN + UPDATE CAPABILITY.
> **Next Extraction**: Fase 2 — formalize additional patterns from L9-L21 learnings. Candidates: parallel-ci-fix (L9), structured-agent-prompt (L10-L11), cross-source-audit (L12, L16), build-time-protection (L14).

---

### Pattern 005: Defense-in-Depth para Integridade de Memória

> **Versão atual**: 1.0.0 | **Lifecycle**: PROPOSED | **ID**: PATTERN-005 | **Confidence**: não recalculada

**Domain**: Memory Integrity, Security, Recovery, Governance
**Level**: Não recalculado
**Times Applied**: 1 (missão de hardening de integridade, 2026-08-04)
**Times Succeeded**: Não estabelecido
**Owner**: cosca-kernel

**Context**: Proteção documental sozinha não impede alteração indevida nem garante recuperação após corrupção ou sobrescrita.

**Solution**:
```
1. OpenCode permissions: restringir operações e superfícies autorizadas.
2. Manifesto SHA-256 advisory: registrar e comparar integridade para detecção/auditoria.
3. Backup: manter cópia recuperável para restauração verificada.
4. Runtime enforcement: completar o bloqueio/validação efetiva no caminho de execução.
```

**Boundaries**:
- O manifesto SHA-256 é advisory: detecta e informa, mas não é enforcement nem substitui permissões.
- Backup é recuperação, não prevenção.
- O padrão permanece PROPOSED até o enforcement de runtime ser concluído e validado.

**Q2 — Descobri um padrão reutilizável?** Sim: camadas independentes de prevenção, detecção e recuperação devem ser combinadas, com limites explícitos para não confundir documentação com enforcement.

**Next Validation**: Demonstrar tentativa bloqueada em runtime, detecção por hash e restauração a partir de backup, cada uma com evidência independente.

### Pattern 006: Safe Project Archive

> **Versão atual**: 1.0.0 | **Lifecycle**: PROPOSED | **ID**: PATTERN-006 | **Confidence**: 0.95

**Domain**: Backup, Packaging, Security

**Solution**: Arquivar o código com exclusão explícita de `.git`, dependências, build artifacts, bancos `.db` e arquivos WAL/SHM; validar com teste estrutural e varredura de conteúdo proibido. Estado operacional deve ser reconstruído por bootstrap/migrações.

**Boundary**: excluir `.db` preserva segurança e portabilidade, mas não preserva os dados neles armazenados.

### Pattern 009: Cognitive Continuity Benchmark

> **Versão atual**: 1.0.0 | **Lifecycle**: PROPOSED | **ID**: PATTERN-009 | **Confidence**: 0.95

**Domain**: Filogênese Cognitiva, avaliação comportamental, regressão cognitiva

**Solution**: Executar a mesma tarefa sob versões v1→v4 com baseline e ambiente controlados; registrar trajetória decisória completa, comparar mudanças de estratégia e classificar cada diferença como melhoria, regressão, especialização ou variação. Avaliar Stability, Adaptability, Calibration, Recovery, Consistency, Specialization e Regression.

**Boundary**: diferença de texto, provider, contexto, ferramenta ou ambiente não pode ser chamada de mudança cognitiva sem controle ou evidência. O benchmark é documental/dry-run até aprovação; não altera agents, skills, embeddings, memória ou routing.

### Pattern 010: KV Tuning for Small Dense GPU-Resident Models

> **Versão atual**: 1.0.0 | **Lifecycle**: VALIDATED | **ID**: PATTERN-010 | **Confidence**: 0.90

**Domain**: GPU inference tuning, KV cache, llama.cpp/llama-bench, VRAM-constrained LLM serving

**Solution**: Para modelos densos pequenos (≤ ~2.5GiB) que cabem INTEIROS na GPU (`-ngl 99`), a configuração ótima de geração é o **KV cache f16 default + `-fa on`** — NUNCA quantizar KV (q8_0/turbo4 penalizam 5-20%), NUNCA usar `-nkvo`/`-mmp 0` (penalizam 30-40%). Para romper barreiras de t/s, reduzir a QUANT do modelo (Q4_K_M → Q4_0 → Q3_K_M) em vez de mexer no KV: teto prático da gfx1031 ≈ 57-63% do teórico (384 GB/s ÷ modelo GiB). Eficiência real: Q4_K_M 2.32GiB → 95 t/s; Q4_0 2.21GiB → 107 t/s.

**Boundary**: o inverso vale para modelos grandes com offload (30B): aí KV turbo4/q8_0 + `-nkvo` são essenciais (L125). A regra depende do modelo estar inteiro na GPU. Contexto grande (128K+) em modelo pequeno exige re-avaliar KV quantizado — trade-off de t/s vs contexto, decisão do Don.

**Q2 — Descobri um padrão reutilizável?** Sim: "todo-GPU = KV f16; offload = KV quantizado" — o ponto de virada é a localização dos pesos, não o tamanho do modelo.

**Next Validation**: validar em outro modelo pequeno (ex.: Qwen3-8B, Llama-3.2-3B) e em contexto real 16K/32K com KV f16.

---

### Pattern 011: GitHub Star-Rank Discovery (regra de negócio do Don)

> **Versão atual**: 1.0.0 | **Lifecycle**: ACTIVE | **ID**: PATTERN-011 | **Confidence**: 0.95

**Domain**: Research, Knowledge Acquisition, Competitive Reconnaissance

**Context**: Sempre que faltar conhecimento técnico, padrão de código ou referência de arquitetura — antes de inventar, buscar no GitHub o melhor do ecossistema por estrelas.

**Solution**:
```text
1. Regra canônica do Don: https://github.com/search?q=($search)&type=repositories&s=stars&o=desc
2. Equivalente programático (devolve JSON limpo): 
   curl -G https://api.github.com/search/repositories \
     --data-urlencode "q=$search" --data-urlencode "sort=stars" \
     --data-urlencode "order=desc" --data-urlencode "per_page=N"
3. O web search é SPA JavaScript — webfetch NÃO parseia; usar a API.
4. Extrair name/stars/language/description (jq).
5. Rate limit não-autenticado: 10 req/min, 60/h → sleep 6s entre queries.
6. Triar por relevância ao stack real (não adotar cego — PATTERN-005).
```

**Known Pitfalls**:
- Query genérica ("go", "aws") retorna ruído — refinar com `language:` ou termo de domínio.
- Estrela ≠ adequação: triar pelo que transfere para o stack (desktop-only ≠ web framework).
- Sem token, respeitar o rate limit — estourar bloqueia a busca por 1h.

**Result**: Mapa de referências de elite (freqtrade/ccxt/hummingbot/Lean/StockSharp/shudcn/KLineChart/cinar/indicator/swarm/n8n) consolidado em ~2 min por domínio.
