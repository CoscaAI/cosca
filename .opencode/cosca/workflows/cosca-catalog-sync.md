# CATALOG SYNC WORKFLOW — Generate-and-Diff

> **Version**: 1.0.0 | **Status**: active | **Owner**: Backend Service Specialist (cosca-specialist-backend-service) | **Created**: 2026-08-22
>
> **Purpose**: Manter o snapshot canônico do catálogo (`.opencode/cosca/catalog.manifest`) sincronizado com a realidade viva do catálogo (agents/skills/engines/departments). O ciclo é generate-and-diff: o snapshot é o CONTRATO versionado; `--check` enforça SÓ novo drift (não os invariantes); `--audit` expõe os 3 invariantes como débito não-bloqueante.

---

## Objetivo

O catálogo do framework Cosca é o conjunto de colunas (agents/skills/engines/departments) que devem ter INDEX.md, frontmatter canônico (name/description/level) em SKILL.md/PROMPT.md e cross-references internos válidos. Esse workflow define o ciclo de manutenção para que:

- **O snapshot seja a verdade**: `--generate` produz a lista ordenada de INDEX esperados + nomes canônicos, commitada como contrato.
- **Somente novo drift bloqueie**: `--check` (default) difa a árvore viva contra o snapshot. Deve PASSSAR (drift=0); só drift NOVO (coluna nova/removida/nome mudado) falha.
- **Débito vire backlog, não trave a esteira**: `--audit` roda os 3 invariantes e os reporta como NÃO-BLOQUEANTE (exit 0; `--strict` para travar).

---

## Trigger

| Trigger | Descrição |
|---------|-----------|
| **Pós-commit** | `.githooks/post-commit` / `post-commit.cmd` rodam `cosca gate catalog --check --summary` quando `.opencode/cosca/` muda no último commit |
| **Manual (dev)** | Alterou colunas/INDEX/frontmatter/cross-refs do catálogo → rodar `--generate` + commitar o snapshot |
| **CI/qualidade** | `cosca gate catalog --audit --strict` para travar em dívida bloqueante (ou `--audit` para apenas reportar) |
| **Pré-release** | `cosca gate catalog --check` antes de qualquer tag de release |

---

## Passos

### Passo 1 — GERAR O SNAPSHOT (generate)

Quando o catálogo vivo muda, regenerar o contrato a partir da realidade ATUAL (ordenado, determinístico):

```bash
cosca gate catalog --generate
```

- Escreve `.opencode/cosca/catalog.manifest` (lista ordenada de INDEX + títulos canônicos).
- **Commite o snapshot junto com a mudança do catálogo** — é o contrato.

### Passo 2 — CHECAR DRIFT (check, default)

```bash
cosca gate catalog --check
cosca gate catalog --check --summary   # 1 linha p/ hook: "catalog: OK" | "catalog: DRIFT (n)"
```

- Difa a árvore viva contra o snapshot. **Não** avalia os 3 invariantes.
- **Deve PASSSAR (drift=0)**. Se falhar → existe drift de contrato → voltar ao Passo 1 e commitar o snapshot.

### Passo 3 — AUDITAR INVARIANTES (audit, débito)

```bash
cosca gate catalog --audit
cosca gate catalog --audit --strict   # travar (exit 1) em dívida
```

- Roda os 3 invariantes e os reporta como **débito não-bloqueante** (contagens + exemplos):
  - **A. index-missing**      coluna de agents/skills/engines/departments sem INDEX.md.
  - **B. frontmatter**        SKILL.md/PROMPT.md sem name/description (ou level fora de 1-5).
  - **C. dangling-link**      link relativo em *.md apontando para alvo inexistente.
- Exit 0 por padrão; `--strict` transforma achados em exit 1.

### Passo 4 — TRATAR DÉBITO (backlog)

- Registrar os achados como backlog priorizável (catálogo do QA).
- Corrigir em ondas; re-rodar `--audit` após cada correção.
- Ao corrigir estrutura (colunas/INDEX), re-rodar `--generate` e commitar o snapshot (Passo 1 → 2).

---

## Gate

| Gate | Comando | Critério | Bloqueante? |
|------|---------|----------|-------------|
| **G-cat-drift** | `cosca gate catalog --check` | Live == snapshot (drift=0) | ✅ Sim (default é bloqueante) |
| **G-cat-audit** | `cosca gate catalog --audit` | 0 achados de invariant | ❌ Não (débito; `--strict` trava) |
| **G-cat-generate** | `cosca gate catalog --generate` | Snapshot regenerado e commitado | — (ação de correção) |

- **Pós-commit**: `.githooks` roda `--check --summary`. Se falhar, **avisa** (imprime "catálogo desatualizado — rode cosca gate catalog --generate e commite") mas **NUNCA trava o commit** (post-commit sempre retorna 0).
- **CI/qualidade**: use `--audit --strict` para travar em dívida, ou `--check` para travar em drift de contrato.

---

## Éxito

- [ ] `cosca gate catalog --generate` produz snapshot ordenado (colunas == realidade).
- [ ] `cosca gate catalog --check` passa (drift=0) após commitar o snapshot.
- [ ] `cosca gate catalog --audit` reporta contagens por tipo (index-missing / frontmatter / dangling-link) e exit 0 (ou exit 1 com `--strict`).
- [ ] Mudanças de catálogo são commitadas JUNTO com o snapshot atualizado.
- [ ] `.githooks/post-commit` (bash + .cmd) chama o check e avisa — sem travar o commit.
