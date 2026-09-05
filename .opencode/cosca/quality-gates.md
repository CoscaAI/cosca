# QUALITY GATES — Catalog (catalog-sync)

> **Version**: 1.0.0 | **Status**: active | **Owner**: Backend Service Specialist (cosca-specialist-backend-service)
>
> **Relationship**: este documento define o gate de catálogo **catalog-sync** (Generate-and-Diff), complementar ao
> [QUALITY_GATES.md](QUALITY_GATES.md) (definições canônicas G0–G4) e ao
> [Quality Gate Standard](memory/qa/quality-gates.md) (G0–G9 de pipeline). Ele NÃO duplica gates existentes —
> registra o gate de domínio do catálogo do framework Cosca.

---

## catalog-sync — Generate-and-Diff

O gate **`catalog-sync`** garante que o snapshot canônico do catálogo
(`.opencode/cosca/catalog.manifest`) permanece sincronizado com a realidade viva do
catálogo (agents/skills/engines/departments). É um ciclo generate-and-diff:
o snapshot é o CONTRATO versionado; o `--check` enforça SÓ novo drift; o `--audit`
expõe os 3 invariantes como débito não-bloqueante.

### Contrato (snapshot)

| Item | Valor |
|------|-------|
| **Snapshot** | `.opencode/cosca/catalog.manifest` |
| **Formato** | `version <n>` + linha `index <path> <título>` por coluna (ordenado por coleção → nome) |
| **Versão de schema** | `1` |
| **Geração** | `cosca gate catalog --generate` (determinística, ordenada) |
| **Commit** | Sempre junto com a mudança do catálogo que o causou |

### Operações

| Modo | Comando | Escopo | Bloqueante? |
|------|---------|--------|-------------|
| **check** (default) | `cosca gate catalog --check` | Dif de DRIFT: árvore viva vs snapshot | ✅ Sim — deve passar (drift=0); enforça só novo drift |
| **audit** | `cosca gate catalog --audit` | 3 invariantes (index-missing / frontmatter / dangling-link) | ❌ Não — débito (reporta contagens + exemplos, exit 0) |
| **audit --strict** | `cosca gate catalog --audit --strict` | idem audit | ✅ Sim — achados travam (exit 1) |
| **generate** | `cosca gate catalog --generate` | Regenera o snapshot | — ação de correção |

### Invariantes (audit)

| Invariante | Regra | Exemplo de achado |
|------------|-------|-------------------|
| **A. index** | Toda coluna (nível imediato) de agents/skills/engines/departments deve ter `INDEX.md` | `coluna "skills/plugin" sem INDEX.md` |
| **B. frontmatter** | Todo `SKILL.md`/`PROMPT.md` deve ter `name` (kebab-case, não-vazio) + `description` (não-vazio); `level` (quando presente) entre 1-5 | `frontmatter sem name` |
| **C. cross-refs** | Todo link relativo em `*.md` deve apontar para alvo existente (esquemas externos/âncoras ignorados) | `link sem alvo: ../path -> ../path` |

### Enforcement (esteira)

| Contexto | Comando | Comportamento |
|----------|---------|---------------|
| **Pós-commit** | `.githooks/post-commit` (bash) / `post-commit.cmd` (Windows) | Quando `.opencode/cosca/` muda no último commit, roda `cosca gate catalog --check --summary`; se falhar, **avisa** ("catálogo desatualizado — rode cosca gate catalog --generate e commite") mas **nunca trava o commit** |
| **CI/qualidade** | `cosca gate catalog --check` | Trava em drift de contrato |
| **CI/qualidade (débito)** | `cosca gate catalog --audit --strict` | Trava em dívida de invariante |

### Critérios de sucesso

- [ ] `cosca gate catalog --check` passa (drift=0) após commitar o snapshot.
- [ ] `cosca gate catalog --audit` reporta contagens por tipo e exit 0 (ou exit 1 com `--strict`).
- [ ] Mudanças de catálogo são commitadas JUNTO com o snapshot atualizado.
- [ ] `.githooks` chama o check e não bloqueia o commit.

---

## Histórico

| Versão | Data | Autor | Alterações |
|--------|------|-------|-----------|
| 1.0.0 | 2026-08-22 | Backend Service Specialist | Registro do gate de catálogo `catalog-sync` (generate-and-diff) |
