# B5 — Problemas Detectados Antes da Falha

> **Métrica**: B5 | **Owner**: cosca-monitoring | **Criado**: 2026-07-30
> **Schema**: `internal/embed/cosca/metrics/CMI_REAL_METRICS.md` — Seção 3.5
> **Atualização**: A cada detecção proativa registrada em learning

---

## Baseline (2026-07-30)

### Resumo

| Indicador | Valor |
|-----------|-------|
| **Total de detecções proativas** | 5 |
| **Total de detecções reativas (estimado)** | 5 |
| **Proactive Ratio** | 50% |
| **Lead time médio** | Semanas |
| **Meta Fase 1** | ≥ 10 detecções proativas, ratio ≥ 60% |
| **Categorias mais detectadas** | inconsistency (2), fiction_doc (2), stale_config (1) |

---

## Histórico de Detecções Proativas

### B5-2026-07-29-001 — Documentos de Arquitetura Fictícios (K8s, PostgreSQL, Kafka)

| Campo | Valor |
|-------|-------|
| **ID** | B5-2026-07-29-001 |
| **Data** | 2026-07-29 |
| **Learning Ref** | L20 — Systemic Platform Audit |
| **Problema** | 3 documentos de arquitetura descrevem infraestrutura inexistente: Kubernetes (`k8s-deployment.md`), PostgreSQL (`postgres-migration.md`), Kafka (`kafka-integration.md`). Nenhuma dessas tecnologias está no `go.mod` ou na estrutura de diretórios do projeto. |
| **Severidade** | 🔴 Alta |
| **Categoria** | `fiction_doc` |
| **Mecanismo de Detecção** | `cross_audit` — Auditoria cross-source: claims de documentação vs `go.mod` + estrutura de diretórios real |
| **Lead Time** | Semanas — os documentos existiam há semanas sem que ninguém notasse |
| **Impacto Evitado** | Agentes tomando decisões de arquitetura baseadas em infraestrutura que não existe. Exemplos: escolher PostgreSQL como banco (só SQLite está disponível), projetar deploy em K8s (projeto não usa Kubernetes), integrar com Kafka (dependência inexistente). Decisões erradas em cascata: arquitetura → implementação → documentação → mais agentes propagando a ficção. |
| **Ação Preventiva** | 12 arquivos expurgados do repositório de documentação (-315 linhas de ficção). Docs marcados como `[FICTÍCIO]` para referência histórica. Padrão cross-source estabelecido como verificação obrigatória antes de aceitar claims de documentação. |
| **Verificação** | Documentação restante verificada contra `go.mod`, `go list`, estrutura de diretórios e código fonte real. Nenhuma claim de infraestrutura sem respaldo no código. |

---

### B5-2026-07-29-002 — Threshold Crisis: 4 Valores Diferentes para o Mesmo Coverage Gate

| Campo | Valor |
|-------|-------|
| **ID** | B5-2026-07-29-002 |
| **Data** | 2026-07-29 |
| **Learning Ref** | L18 — Runtime Coverage Audit |
| **Problema** | Quatro valores diferentes para o mesmo gate de cobertura coexistindo no sistema: `Makefile` = 40%, `ci.yml` = 55%, `docs/coverage.md` = 70%, `embed coverage` = 80%. Nenhum time sabia qual era o valor real. O CI executava 55% quando a baseline real era 78% e o documentado era 70%. |
| **Severidade** | 🔴 Crítica |
| **Categoria** | `inconsistency` |
| **Mecanismo de Detecção** | `cross_audit` — Auditoria de governança: QA Chief cruzou Makefile, CI workflow, documentação, e cobertura real executada |
| **Lead Time** | Antes do próximo deploy — o CI teria falhado (ou pior, passado) com threshold incorreto |
| **Impacto Evitado** | **(1) Deploy bloqueado falsamente**: Se o threshold fosse ajustado para 70% sem verificar a cobertura real, pacotes legítimos seriam bloqueados. **(2) Cobertura falsa**: Se o threshold permanecesse 55%, código com 55-70% passaria sem a cobertura adequada — falsa sensação de segurança. **(3) Desconfiança no CI**: Com thresholds inconsistentes, ninguém confia no gate — ele vira ruído. |
| **Ação Preventiva** | Threshold unificado para 70% statement + 60% branch coverage. CI atualizado com `continue-on-error: false`. Documentação iniciada (3 docs residuais pendentes de correção final identificados em L21). |
| **Verificação** | CI reexecutado com novo threshold. 3 docs com valores antigos identificados para correção (tracking em L21). |

---

### B5-2026-07-29-003 — Permission Paths Stale no opencode.json

| Campo | Valor |
|-------|-------|
| **ID** | B5-2026-07-29-003 |
| **Data** | 2026-07-29 |
| **Learning Ref** | L12 — Runtime Security Audit |
| **Problema** | `opencode.json` (479 linhas) continha paths de permissão stale — apontavam para diretório de outro usuário (`/home/henrique/...`) em path diferente do workspace real do Don. Todas as permissões de `read`, `glob`, `grep`, `edit`, `write`, `bash`, `task` referenciando o workspace errado. |
| **Severidade** | 🔴 Alta |
| **Categoria** | `stale_config` |
| **Mecanismo de Detecção** | `security_audit` — Auditoria de segurança do runtime: verificou todas as permissões contra o workspace real |
| **Lead Time** | Semanas — os paths estavam errados desde a instalação inicial do OpenCode |
| **Impacto Evitado** | **(1) Brecha de segurança**: Permissões não efetivas — agentes poderiam acessar paths fora do workspace (se o path stale existisse) ou serem bloqueados de acessar o workspace real. **(2) Falha silenciosa**: Agentes tentando ler/escrever em paths que não existem, falhando sem diagnóstico claro. **(3) Violação de least privilege**: Sem paths corretos, o modelo de permissão não aplica — ou é permissivo demais ou restritivo demais. |
| **Ação Preventiva** | Todos os paths corrigidos para o workspace real do Don. Permissões de execução (`bash`, `edit`, `write`) restritas ao workspace. Permissões de leitura (`read`, `glob`, `grep`) restritas ao workspace + `/tmp/opencode` (mínimo necessário). `task` restrito ao workspace. |
| **Verificação** | Don aprovou correção direta. Permission model validado com least privilege aplicado. |

---

### B5-2026-07-29-004 — PostgreSQL Fantasy Pattern Cross-Agent

| Campo | Valor |
|-------|-------|
| **ID** | B5-2026-07-29-004 |
| **Data** | 2026-07-29 |
| **Learning Ref** | Semantic Memory C1 Indexing |
| **Problema** | Múltiplos agentes (documentation, architecture, database, migration, backend) mencionavam PostgreSQL como infraestrutura em seus documentos de memória e capability profiles, quando o projeto usa exclusivamente SQLite (via modernc.org). O padrão "PostgreSQL fantasy" estava se propagando silenciosamente entre agentes. |
| **Severidade** | 🟡 Média |
| **Categoria** | `fiction_doc` |
| **Mecanismo de Detecção** | `semantic_analysis` — Indexação semântica de 426 arquivos revelou cluster cross-agent com menções a PostgreSQL. Semantic similarity search detectou o padrão recorrente. |
| **Lead Time** | Semanas — o padrão era persistente na documentação de múltiplos agentes |
| **Impacto Evitado** | **(1) Contaminação cross-agent**: Novos agentes lendo documentação de agentes existentes propagariam a ficção PostgreSQL indefinidamente. **(2) Decisões de arquitetura erradas**: Database Chief poderia projetar schema para PostgreSQL; Migration Chief criaria migrações SQL incompatíveis com SQLite. **(3) Dependência fantasma**: Sugestões de adicionar driver PostgreSQL ao `go.mod` baseadas em documentação fictícia. |
| **Ação Preventiva** | Heurística H-009 extraída: "Sempre verificar claims de infraestrutura contra go.mod + código fonte real." Padrão documentado como `PostgreSQL fantasy` no semantic memory — todos os agentes têm acesso a esta heurística. |
| **Verificação** | Índice semântico atualizado com a heurística. Busca reversa: nenhum agente ativo contém mais claims de PostgreSQL não verificadas. |

---

### B5-2026-07-29-005 — MEMORY_MODEL.md Drift: 44 Linhas de Divergência

| Campo | Valor |
|-------|-------|
| **ID** | B5-2026-07-29-005 |
| **Data** | 2026-07-29 |
| **Learning Ref** | Embed Sync Investigation (pós-L13) |
| **Problema** | `MEMORY_MODEL.md` exibia 44 linhas de drift entre a fonte de verdade (`internal/embed/cosca/memory/MEMORY_MODEL.md`) e a cópia embedada no binário (`internal/embed/cosca/memory/MEMORY_MODEL.md`). A versão embedada estava 44 linhas atrás — faltavam seções inteiras adicionadas após o último `make embed-sync`. |
| **Severidade** | 🔴 Alta |
| **Categoria** | `inconsistency` |
| **Mecanismo de Detecção** | `git_analysis` — Comparação direta de arquivos entre `internal/embed/cosca/` e `internal/embed/cosca/` durante investigação da causa raiz do jail breach |
| **Lead Time** | Antes do próximo build — o próximo binário conteria MEMORY_MODEL.md desatualizado, repetindo o padrão que causou o jail breach |
| **Impacto Evitado** | **(1) Regressão em cascata**: `cosca init --force` com binário novo teria sobrescrito o MEMORY_MODEL.md atual com a versão antiga de 44 linhas a menos — perdendo documentação de arquitetura de memória. **(2) Reincidência do jail breach**: O mesmo mecanismo que regrediu 11 arquivos no L13 se repetiria com o próximo build, já que o embed estava desatualizado. **(3) Perda de conhecimento**: As 44 linhas incluíam a estrutura do Knowledge Repository e o modelo de 6 categorias — conhecimento arquitetural perdido silenciosamente. |
| **Ação Preventiva** | `make embed-sync` executado, sincronizando `internal/embed/cosca/` → `internal/embed/cosca/`. 33 arquivos do Knowledge Pipeline copiados para o embed. Regra P8 reforçada: DRY_RUN=1 obrigatório antes de `make embed-sync`. CI validation para drift detection pendente (Fase 1). |
| **Verificação** | Embed verificado: hash dos arquivos em `internal/embed/cosca/` bate com `internal/embed/cosca/`. Nenhum drift residual. |

---

## Análise Comparativa: Proativo vs Reativo

### Detecções Proativas (Antes da Falha)

| # | Problema | Severidade | Lead Time | Mecanismo |
|---|----------|-----------|-----------|-----------|
| 1 | Docs fictícios (K8s, PostgreSQL, Kafka) | Alta | Semanas | cross_audit |
| 2 | Threshold crisis (4 valores) | Crítica | Antes do deploy | cross_audit |
| 3 | Permission paths stale | Alta | Semanas | security_audit |
| 4 | PostgreSQL fantasy cross-agent | Média | Semanas | semantic_analysis |
| 5 | MEMORY_MODEL.md drift (44 linhas) | Alta | Antes do build | git_analysis |

### Detecções Reativas (Após Falha/Incidente) — Estimativa

| # | Problema | Severidade | Quando Detectado |
|---|----------|-----------|-----------------|
| 1 | Jail breach — 11 arquivos regredidos | Crítica | Após execução (L13) |
| 2 | 20 race conditions em 5 pacotes | Alta | Durante teste com -race (L21) |
| 3 | Deepseek API key sendo descartada pelo config | Alta | Após falha do config set (L16) |
| 4 | runServe coverage 0.5% — monolito intocado | Média | Durante auditoria (L19) |
| 5 | Schema bug em entities_fts (mesma classe de documents_fts) | Média | Durante revisão (Onda 3) |

---

## Dashboard

### Proactive Ratio (Gráfico de Pizza)

```
          DETECÇÕES
       ┌──────────────┐
       │              │
       │  Proativas   │   Reativas
       │     50%      │    50%
       │    (5)       │    (5)
       │              │
       └──────────────┘

Meta Fase 1: inverter para 60/40
Meta Fase 3: inverter para 80/20
```

### Distribuição por Categoria

```
inconsistency   ████████████████████ 2 (threshold crisis, MEMORY_MODEL drift)
fiction_doc     ████████████████████ 2 (K8s/PG/Kafka, PostgreSQL fantasy)
stale_config    ██████████ 1 (permission paths)
                ─────────────────
                TOTAL PROATIVAS    5
```

### Distribuição por Mecanismo de Detecção

```
cross_audit         ████████████████████ 2
security_audit      ██████████ 1
semantic_analysis   ██████████ 1
git_analysis        ██████████ 1
                    ─────────────────
                    TOTAL             5
```

### Lead Time por Detecção

```
B5-001 (docs fictícios)       ████████████████████████████ Semanas
B5-002 (threshold crisis)     ██████ Antes do deploy
B5-003 (stale paths)          ████████████████████████████ Semanas
B5-004 (PG fantasy)           ████████████████████████████ Semanas
B5-005 (MEMORY_MODEL drift)   ██████ Antes do build
```

---

## Schema de Entrada (para novas detecções)

```yaml
# Template para registrar nova detecção proativa
nova_deteccao:
  id: "B5-YYYY-MM-DD-NNN"
  data_deteccao: YYYY-MM-DD
  learning_ref: ""
  problema: "Descrição do problema detectado"
  severidade: "Baixa | Média | Alta | Crítica"
  categoria: "inconsistency | stale_config | missing_coverage | fiction_doc | security_gap | architectural_risk"
  mecanismo_deteccao: "cross_audit | linting | testing | immune_system | semantic_analysis | git_analysis"
  lead_time: "Quanto tempo antes da provável falha"
  impacto_evitado: "O que teria acontecido se não detectado"
  acao_preventiva: "O que foi feito para corrigir"
  verificacao: "Como foi verificado que o problema foi resolvido"
```

---

## Metas

| Meta | Valor | Status |
|------|-------|--------|
| Curto prazo: ≥ 1 detecção proativa por semana | 1+/semana | ✅ 5 em 2 dias (média 2.5/dia) |
| Médio prazo: proactive ratio ≥ 70% | 70% | ⬜ 50% → 70% |
| Fase 1: detecções proativas acumuladas | ≥ 10 | ⬜ 5/10 (50%) |
| Fase 3: ratio ≥ 80% com Cognitive Immune System | 80% | ⬜ Pendente (requer Fase 2 — C5) |

---

> **Última atualização**: 2026-07-30 | **Próxima revisão**: após próxima detecção proativa registrada
> **Princípio**: O maior valor de um sistema maduro não é reagir bem a falhas — é evitar que elas aconteçam. Esta métrica é o indicador mais forte de maturidade cognitiva real.
