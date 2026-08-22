# AUDIT WORKFLOW — Runtime-Projeto-Cosca Sync

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28
>
> **Purpose**: Garantir que a documentação do framework (internal/embed/cosca/) reflete a realidade do projeto runtime. Detecta drift entre docs e código.

---

## Trigger

| Trigger | Description |
|---------|-------------|
| **Manual** | Don solicita: "audita o projeto" |
| **Semanal** | Após cada ciclo de dashboard |
| **Pós-commit** | Após commits significativos (≥10 arquivos) |
| **Pré-release** | Antes de qualquer tag de release |

---

## Workflow Steps

### Step 1 — COLLECT RUNTIME STATS

Executar comandos para coletar a realidade atual do projeto:

```bash
# Go files
find . -name "*.go" -not -path "./.git/*" -not -path "./.cosca/*" -not -path "./web/*" -not -path "./sdk/*" | wc -l

# Go packages
find internal -name "*.go" -not -path "*_test.go" | xargs dirname | sort -u | wc -l

# CLI commands (root)
grep -r "NewRootCommand\|AddCommand" internal/cli/root.go | wc -l

# CLI total commands
grep -r "cobra.Command" internal/cli/ | wc -l

# REST endpoints (OpenAPI operations)
grep -c "operationId:" api/rest/openapi.yaml

# API domains (tags in OpenAPI)
grep "tags:" api/rest/openapi.yaml | sort -u | wc -l

# Frontend files
find web/src -name "*.tsx" | wc -l
find web/src -name "*.ts" -not -name "*.tsx" | wc -l
find web/src/app -name "page.tsx" | wc -l

# Feature modules
ls -d web/src/features/*/ | wc -l

# Test packages
find . -name "*_test.go" -not -path "./.git/*" -not -path "./web/*" -not -path "./sdk/*" | xargs dirname | sort -u | wc -l

# Memory files
find internal/embed/cosca/memory -name "*.md" | wc -l

# Agent directories
ls -d internal/embed/cosca/memory/agent/cosca-*/ | wc -l

# Memory INDEX files
find internal/embed/cosca/memory -name "INDEX.md" | wc -l

# Engine count
ls -d internal/embed/cosca/engines/*/ | wc -l

# Workflow count
ls internal/embed/cosca/workflows/*.md 2>/dev/null | wc -l

# Department SKILL files
find internal/embed/cosca/departments -name "SKILL.md" | wc -l

# Provider count
ls -d internal/providers/*/ 2>/dev/null | wc -l

# Editor count
find internal/editors -name "*.go" -not -name "*_test.go" | wc -l
```

### Step 2 — COLLECT DOCUMENTED STATS

Ler os arquivos de documentação que declaram números:

```bash
# README stats
grep -E "^\| \*\*" README.md

# Session context
grep -E "Memory files|Agents:|Engines:|Workflows:" internal/embed/cosca/memory/context/session.md

# Codebase overview
grep -E "Go files|packages|endpoints|commands|TSX|providers|adapters" internal/embed/cosca/memory/codebase/overview.md

# INDEX health
grep -A3 "## Health" internal/embed/cosca/memory/INDEX.md
```

### Step 3 — COMPARE (Detect Drift)

Comparar runtime stats vs documented stats. Gerar tabela de discrepâncias:

```
| Stat | Runtime | Documented | Delta | File |
|------|---------|-----------|-------|------|
```

### Step 4 — REPORT

Gerar relatório de auditoria:
- Salvar em `docs/roadmap/audit-YYYY-MM-DD.md`
- Atualizar `memory/INDEX.md` health stats
- Listar correções necessárias

### Step 5 — FIX (se autorizado)

Se Don autorizar correção automática:
- Atualizar README.md
- Atualizar session.md
- Atualizar codebase/overview.md
- Atualizar INDEX.md health

### Step 6 — COMMIT

```bash
git add -A
git commit -m "audit: sync docs with runtime reality (YYYY-MM-DD)"
```

---

## Delegation

| Step | Agent | Reason |
|------|-------|--------|
| Step 1 (collect) | cosca-automation | Scripting, file counting |
| Step 2 (collect docs) | cosca-documentation | Document parsing |
| Step 3 (compare) | cosca-kernel | Cross-reference analysis |
| Step 4 (report) | cosca-documentation | Report generation |
| Step 5 (fix) | cosca-kernel (with approval) | Critical doc changes |
| Step 6 (commit) | cosca-kernel | Final authority |

---

## Success Criteria

- [ ] Runtime stats collected (all 15 metrics)
- [ ] Documented stats collected (4 files)
- [ ] Discrepancies table generated
- [ ] Audit report saved
- [ ] All discrepancies ≤ 0 delta (after fix) OR documented as intentional
- [ ] INDEX health stats updated
- [ ] Commit with audit trail
