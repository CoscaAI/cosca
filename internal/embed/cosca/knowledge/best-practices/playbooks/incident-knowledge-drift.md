---
id: playbook-005
title: "Resposta a Divergencia entre Documentacao e Codigo"
type: incident-response
severity: critical
owner: cosca-documentation
created: 2026-07-29
tags:
  - documentation
  - drift
  - memory
  - audit
  - fabrication
  - cross-reference
  - verification
related_incidents:
  - PostgreSQL fantasy corrigido por cosca-database (2026-07-28)
  - GDPR/SOC2/ISO fabrication corrigido por cosca-security (2026-07-28)
  - coverage.md stale (underreported runtime coverage) (2026-07-29)
  - 63 broken refs encontrados pelo doc-validator (2026-07-28)
  - 3 critically stale files: PostgreSQL fantasy, Go SDK fiction, compliance fabrication
  - version drift: README v1.3.0 vs CHANGELOG v1.4.0-dev
  - badge drift: 52→51 agents, 34→39 CLI, 336→357 Go files
  - kernel L3 cross-source audit (2026-07-28)
  - opencode.json 16 stale numbers corrigidos (2026-07-28)
---

# Resposta a Divergencia entre Documentacao e Codigo

## Trigger

Qualquer um destes eventos dispara o playbook:

1. **doc-validator CI falha** — script `.github/workflows/scripts/doc-validator.sh` encontra broken references (3 checks: file paths, internal links, package references)
2. **Auditoria manual detecta discrepância** — comparação entre claims de documento e realidade (go.mod, source code, filesystem)
3. **Agente reporta "aspirational drift"** — padrão detectado por cosca-security ou cosca-documentation: documento descreve funcionalidade que não existe
4. **Números não batem** — contagem de agentes, arquivos, comandos, workflows em docs não confere com contagem real

**Contexto real**: Entre 2026-07-24 e 2026-07-29, três auditorias independentes revelaram um padrão de drift documental no projeto. O caso mais grave foi o "PostgreSQL fantasy" — `memory/architecture/database-architecture.md` descrevia PostgreSQL 16 RDS Multi-AZ quando o projeto usa SQLite embutido (modernc.org/sqlite). O segundo foi a "GDPR fabrication" — datas de conformidade GDPR/SOC2/ISO 27001 que não existiam. O terceiro foi `coverage.md` reportando runtime como ">70%" quando estava em 97.9%. Ao todo, 52 issues documentais foram corrigidas em 19 arquivos (kernel L3 doc sync, 2026-07-28).

## Passos

### Passo 1: Comparar claims do documento vs realidade

```bash
# 1a. Verificar claims de dependencias contra go.mod
# Exemplo real: database-architecture.md dizia "PostgreSQL 16 RDS Multi-AZ"
# Realidade go.mod:
grep -i "postgres\|pgx\|sqlite" go.mod
# Output real: modernc.org/sqlite v1.37.2 — SQLite, nao PostgreSQL

# 1b. Verificar claims de contagem contra filesystem
# Exemplo real: README dizia "52 agents", "34 CLI commands", "336 Go files"
# Realidade:
ls -d internal/embed/cosca/memory/agent/cosca-*/ | wc -l     # 55 agentes
ls internal/cli/*.go | wc -l                              # comparar com claim
find . -name "*.go" -not -path "./vendor/*" | wc -l      # comparar com claim

# 1c. Verificar claims de versao
# Exemplo real: docs/README.md dizia v1.3.0, CHANGELOG.md dizia v1.4.0-dev
grep -r "version\|v[0-9]\.[0-9]\.[0-9]" docs/ CHANGELOG.md pkg/cosca/cosca.go

# 1d. Verificar claims de SDK/API contra codigo
# Exemplo real: docs/sdk/go.md descrevia Go client library
# Realidade: pkg/cosca/ tem tipos mas nao um client library compilavel separado
ls sdk/go/ 2>/dev/null || echo "SDK Go nao existe como pacote separado"
```

**Output esperado**: Lista de discrepâncias com path do documento, claim falsa, e realidade do código.

### Passo 2: Classificar severidade

| Severidade | Categoria | Ação | Exemplo real |
|-----------|-----------|------|-------------|
| **Fabricacao** (critical) | Documento descreve feature/status que NUNCA existiu | **Remover ou marcar como aspirational** | PostgreSQL fantasy: database-architecture.md descrevia RDS Multi-AZ, pgvector — Cosca usa SQLite embedded. Corrigido por cosca-database (2026-07-28) |
| **Fabricacao** (critical) | Documento reivindica conformidade legal inexistente | **Remover imediatamente** | GDPR fabrication: memory reivindicava conformidade GDPR/SOC2/ISO 27001 com datas fabricadas. Corrigido por cosca-security (2026-07-28) |
| **Desatualizacao** (high) | Documento esta correto conceitualmente mas com dados velhos | **Atualizar numeros** | coverage.md: runtime reportado como ">70%" estava em 97.9%. Corrigido na auditoria 2026-07-29 |
| **Desatualizacao** (high) | Versao no doc nao confere com codigo | **Sincronizar com fonte canonica** | Version drift: README v1.3.0 vs CHANGELOG v1.4.0-dev. README corrigido para v1.4.0-dev (2026-07-28) |
| **Contagem errada** (medium) | Badges/stats com numeros desatualizados | **Recontar e corrigir** | Badge drift: 52→51 agents, 34→39 CLI commands, 336→357 Go files, 25→26 workflows (2026-07-28) |
| **Broken reference** (medium) | Link para arquivo que nao existe mais | **Corrigir ou remover** | 63 broken refs encontrados pelo doc-validator, 27 reais, 36 falsos positivos (path resolution) (2026-07-28) |

**Regra de decisao**: Se o claim descreve algo que NUNCA existiu (fabrication), remover ou marcar explicitamente como `[ASPIRATIONAL]`. Se descreve algo que EXISTIU mas esta desatualizado, atualizar com dados reais. Nunca manter ficcao documental — ela corrompe decisoes de todos os agentes (kernel L3, critic L1: "foundational document inconsistency is a critical risk vector").

### Passo 3: Documentar o delta

Criar ou atualizar arquivo em `internal/embed/cosca/memory/audit/`:

```markdown
---
type: audit
key: drift-<data>-<slug>
tags: [drift, documentation, audit, <dominio>]
timestamp: 2026-07-29T00:00:00Z
status: completed
---

# Audit: Document Drift — <descricao>

## Discrepâncias encontradas

| Arquivo | Claim | Realidade | Severidade | Acao |
|---------|-------|-----------|------------|------|
| memory/architecture/database-architecture.md | PostgreSQL 16 RDS | SQLite embedded (modernc.org/sqlite) | Fabricacao | Substituido por arquitetura real |
| memory/long/compliance-framework.md | GDPR compliant 2024 | Nao implementado | Fabricacao | Marcado como [ASPIRATIONAL] com 5 passos |
| memory/testing/coverage.md | runtime >70% | 97.9% | Desatualizacao | Atualizado com valores reais da auditoria |
```

**Padrao real**: O arquivo `memory/audit/coverage-audit-2026-07-29.md` documenta 183 linhas de auditoria incluindo a threshold crisis de 4 valores conflitantes. Seguir o mesmo formato.

### Passo 4: Corrigir na fonte canonica

A fonte canonica depende do tipo de discrepância:

| Tipo de claim | Fonte canonica | Exemplo |
|--------------|----------------|---------|
| Stack/dependencia | `go.mod` / `package.json` | PostgreSQL → SQLite: verificar go.mod |
| Feature/API | Codigo fonte (`.go`, `.tsx`) | Go SDK fiction: verificar se `sdk/go/` existe |
| Conformidade legal | Implementacao real no codigo | GDPR: verificar se ha audit logging, encryption, data mapping |
| Contagens | Filesystem (`find`, `ls`, `wc -l`) | 336 Go files → 357: recontagem real |
| Versao | `CHANGELOG.md` (single source of truth) | v1.3.0 → v1.4.0-dev |
| Embed sync | `make embed-sync --dry-run` | MEMORY_MODEL.md sync gap (44 linhas) |

**Para cada discrepância**:

```bash
# Se for fabricacao: remover ou marcar [ASPIRATIONAL]
# Exemplo real: compliance-framework.md reescrito com 5 passos concretos:
# 1. at-rest encryption, 2. audit logging, 3. data mapping,
# 4. retention automation, 5. right-to-erasure

# Se for desatualizacao: atualizar com valor real
# Exemplo real: coverage.md runtime ">70%" → "97.9%"

# Se for broken reference: corrigir path ou remover
# Exemplo real: 11 broken links de wrong ADR filename corrigidos
```

**IMPORTANTE — P8 Compliance (CONSTITUTION.md)**:
```bash
# NUNCA modificar internal/embed/cosca/ diretamente.
# Sempre:
# 1. Modificar a fonte em internal/embed/cosca/
# 2. Rodar make embed-sync --dry-run para verificar
# 3. Apresentar mudancas ao Don
# 4. Somente entao: make embed-sync (com aprovacao)
```

### Passo 5: Adicionar check no CI para impedir recorrencia

```bash
# Verificar se doc-validator ja esta no CI
grep -r "doc-validator" .github/workflows/

# Se nao estiver, adicionar job:
# .github/workflows/doc-check.yml
# - Roda doc-validator.sh
# - Falha CI em broken references (exit code != 0)
# - Ignora false positives conhecidos (path resolution)

# Verificar coverage do doc-validator
# Onda 2 review (2026-07-28) identificou que o script retorna exit 0
# mesmo com broken refs — "non-blocking warning for now"
# ISSUE ABERTA: promover a blocking quando refs existentes forem corrigidas
```

**Gates automatizados implementados** (quality-gates.md G0):

| Check | Ferramenta | Bloqueia merge? |
|-------|-----------|:---:|
| File paths existem | `doc-validator.sh` | ✅ (quando promovido) |
| Internal links validos | `doc-validator.sh` | ✅ |
| Package references corretas | `doc-validator.sh` | ✅ |
| Badge counts vs realidade | A implementar | ❌ |
| Version consistency | A implementar | ❌ |
| Embed sync status | `make embed-sync --dry-run` | ❌ |

## Verificacao

```bash
# Verificar consistencia embed (fonte vs build)
make embed-sync --dry-run
# Se houver diff entre internal/embed/cosca/ e internal/embed/cosca/:
# 1. Corrigir na fonte primeiro
# 2. Reexecutar dry-run
# 3. Comitar a fonte corrigida
# 4. Executar make embed-sync com aprovacao

# Verificar se nao ha mais fabricacoes
# Cross-check rapido de claims comuns:
grep -ri "postgresql\|rds\|multi-az" internal/embed/cosca/memory/
# Esperado: 0 resultados (ou apenas em contexto historico/correcao)

grep -ri "gdpr compliant\|soc2 certified\|iso 27001" internal/embed/cosca/memory/
# Esperado: apenas marcacoes [ASPIRATIONAL] ou contexto de correcao

# Verificar numeros contra realidade
echo "Agentes reais: $(ls -d internal/embed/cosca/memory/agent/cosca-*/ | wc -l)"
echo "Go files reais: $(find . -name '*.go' -not -path './vendor/*' | wc -l)"
# Comparar com badges/docs
```

## Escalacao

| Gatilho | Escalar para | Razao |
|---------|-------------|-------|
| Fabricacao de conformidade legal (GDPR, SOC2, ISO) | `cosca-compliance` + `cosca-security` | Risco legal e reputacional |
| Fabricacao de stack/arquitetura | `cosca-architecture` + `cosca-database` | PostgreSQL fantasy exigiu reescrita completa da arquitetura de banco |
| Multiplas fabricacoes (3+) em um unico componente | `cosca-kernel` | Indica falha sistemica de verificacao — padrao encontrado em 2026-07-28 |
| Drift de versao em docs facing | `cosca-release` + `cosca-documentation` | Version drift (README v1.3.0 vs CHANGELOG v1.4.0-dev) confunde usuarios |
| Embed sync quebrado (> 10 arquivos com diff) | `cosca-kernel` | P8 violation — embed desincronizado da fonte |

## Prevencao

1. **Sempre verificar claims contra go.mod + source code** — padrao estabelecido pelo cosca-database (PostgreSQL fantasy) e cosca-security (GDPR fabrication). Nenhum agente deve confiar em claims de memoria sem verificacao cruzada.

2. **doc-validator como gate blocking no CI** — atualmente roda como non-blocking warning. Promover a blocking quando broken references existentes forem corrigidas (issue da Onda 2 review).

3. **Single source of truth para versao** — CHANGELOG.md como fonte canonica. README e docs devem referenciar CHANGELOG, nunca hardcodar versao propria.

4. **Auditoria periodica de badges e contagens** — README badges e stats table ficam stale sem verificacao automatizada. Kernel L3 estabeleceu o padrao: catalogar → cross-reference → detectar → corrigir.

5. **Nunca escrever claims aspiracionais como fatos** — se uma feature nao existe, marcar `[ASPIRATIONAL]` ou `[PLANNED]` com link para roadmap/milestone. Nao escrever no presente como se ja existisse.

6. **Embed sync como parte do ciclo de correcao** — toda correcao em `internal/embed/cosca/` deve ser seguida de `make embed-sync --dry-run` para verificar propagacao ao build artifact.

## Referencias

- **PostgreSQL fantasy**: `internal/embed/cosca/memory/agent/cosca-database/learnings.md` (2026-07-28)
- **GDPR fabrication**: `internal/embed/cosca/memory/agent/cosca-security/learnings.md` (2026-07-28)
- **3 critically stale files**: `internal/embed/cosca/memory/agent/cosca-documentation/learnings.md` (2026-07-28)
- **Doc sync Fases 1-3**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L3 (2026-07-28)
- **52 doc issues fix**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L3 (2026-07-28)
- **Version drift**: `internal/embed/cosca/memory/technical-debt/scorecard.md` DOC-02
- **Quality gates doc-code**: `internal/embed/cosca/memory/qa/quality-gates.md` G0
- **CONSTITUTION.md P8**: `internal/embed/cosca/CONSTITUTION.md` — Embed Sync Protocol
- **Semantic memory clustering**: `internal/embed/cosca/memory/semantic/INDEX.md` — documentation integrity cluster
