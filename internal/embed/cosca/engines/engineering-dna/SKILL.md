# ENGINEERING DNA ENGINE — Hereditariedade da Engenharia (F9.5)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência (+0.07), Reproducibilidade (+0.06) | **Bloco Cognitivo**: Bloco 9 — Evolution & Herança
> **Fase CMI**: Fase 9 — Evolution | **Código**: F9.5
> **Dependências**: F9.1 Experience Compiler | CONSTITUTION.md | QUALITY_GATES.md
>
> Consulte também:
> - [Experience Compiler](../experience-compiler/SKILL.md) — F9.1: princípios e padrões destilados que alimentam o DNA
> - [CONSTITUTION.md](../../CONSTITUTION.md) — regras fundacionais herdadas por todo projeto filho
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — gates de qualidade que cada projeto carrega no DNA
> - [AGENT_DNA.md](../../AGENT_DNA.md) — 28 campos obrigatórios que todo agente carrega (paralelo ao DNA de engenharia)
> - [SKILL_TEMPLATE.md](../../SKILL_TEMPLATE.md) — template de skill engine
> - [project-init.md](../../workflows/project-init.md) — workflow de inicialização que invoca o Engineering DNA

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [O que o DNA Contém](#2-o-que-o-dna-contém)
3. [Pipeline de Herança (6 Fases)](#3-pipeline-de-herança-6-fases)
4. [Mutação — Don Override](#4-mutação-—-don-override)
5. [Evolução — Propagação de Atualizações](#5-evolução-—-propagação-de-atualizações)
6. [Versionamento do DNA](#6-versionamento-do-dna)
7. [Exemplo Completo: cosca-web](#7-exemplo-completo-cosca-web)
8. [CLI e Automação](#8-cli-e-automação)
9. [Métricas do Engine](#9-métricas-do-engine)
10. [Casos de Borda e Anti-Padrões](#10-casos-de-borda-e-anti-padrões)

---

## 1. DEFINIÇÃO

### 1.1 O que é o Engineering DNA

O **Engineering DNA Engine** é o mecanismo de hereditariedade de engenharia do Cosca. Quando um novo projeto é criado via `cosca init --dna <project>`, ele **herda** o DNA de engenharia do projeto pai — arquitetura, padrões de teste, templates de documentação, regras de segurança, e pipeline de CI.

Assim como no código genético, cada projeto carrega um genoma completo de engenharia que define como o projeto é construído, testado, documentado e operado. A herança permite que:

- Projetos irmãos compartilhem a mesma base de qualidade
- Novos projetos comecem com décadas de engenharia acumulada
- Padrões compilados pelo F9.1 Experience Compiler sejam propagados automaticamente
- O Don decida quais genes manter, modificar ou remover em cada filho

### 1.2 Analogia: Genoma de Engenharia

```
┌─────────────────────────────────────────────────────────────────────┐
│              ENGINEERING DNA — ANALOGIA DE GENOMA                     │
│                                                                      │
│  Genoma Humano           →  Engineering DNA                          │
│  Cromossomo              →  Categoria (architecture, testing, etc.)  │
│  Gene                    →  Regra/Configuração individual             │
│  Alelo                   →  Variação de uma regra                     │
│  Expressão Gênica        →  Aplicação da regra no projeto            │
│  Mutação                 →  Don override em um gene específico        │
│  Hereditariedade         →  Herança pai → filho via --dna            │
│  Evolução                →  Atualizações propagadas do pai para os filhos│
│  Genótipo                →  DNA completo (source of truth)           │
│  Fenótipo                →  Projeto concreto gerado a partir do DNA  │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.3 Propósito

| Por que herdar DNA | O que resolve | Ação resultante |
|--------------------|--------------|-----------------|
| Consistência cross-projeto | Cada projeto reinventa padrões de engenharia | Base comum herdada automaticamente |
| Aceleração de setup | Projetos novos demoram dias para configurar CI, testes, docs | `cosca init --dna` gera em segundos |
| Propagação de melhores práticas | Aprendizados do F9.1 ficam isolados no projeto pai | Compilação vira DNA → filhos herdam |
| Governança centralizada | Don precisa auditar N projetos com padrões diferentes | Todos os filhos seguem o mesmo genoma base |
| Evolução controlada | Atualizar padrão em N projetos é manual | Pull do DNA pai → filhos recebem atualização |

### 1.4 Outputs do Engine

| Output | Formato | Consumidor | Frequência |
|--------|---------|------------|------------|
| **Projeto Filho** | Diretório com estrutura + config + templates | Don / desenvolvedor | Sob demanda (`cosca init --dna`) |
| **Relatório de Herança** | `dna-inheritance-report.md` gerado no projeto filho | Don / Architecture Chief | A cada `--dna` |
| **DNA Manifest** | `.cosca/dna/manifest.yaml` no projeto filho | Engineering DNA Engine | A cada herança |
| **Mutações** | `.cosca/dna/mutations.yaml` no projeto filho | Engineering DNA Engine + Don | A cada override |
| **Evolução Pendente** | Lista de genes desatualizados entre pai e filho | Don | `cosca dna sync --check` |

---

## 2. O QUE O DNA CONTÉM

O Engineering DNA é organizado em 5 cromossomos. Cada cromossomo contém genes específicos que definem um aspecto da engenharia do projeto.

```
ENGINEERING DNA v1.0
│
├── 🏗️ Cromossomo 1: ARQUITETURA
│   ├── Gene: directory-structure   →  padrão de diretórios do projeto
│   ├── Gene: code-patterns         →  padrões de código (manager/store, handler, etc.)
│   ├── Gene: conventions           →  naming, style, lint rules
│   ├── Gene: module-boundaries     →  camadas e direções de dependência
│   └── Gene: tech-stack            →  linguagem, framework, bibliotecas core
│
├── 🧪 Cromossomo 2: TESTING
│   ├── Gene: coverage-threshold    →  cobertura mínima (linha, branch)
│   ├── Gene: test-patterns         →  AAA, table-driven, mock strategies
│   ├── Gene: mock-preferences      →  testify, mockgen, interfaces preferidas
│   ├── Gene: test-levels           →  unit / integration / e2e estrutura
│   └── Gene: performance-bench     →  thresholds de performance em testes
│
├── 📖 Cromossomo 3: DOCUMENTAÇÃO
│   ├── Gene: readme-template       →  template de README.md
│   ├── Gene: adr-template          →  template de ADR
│   ├── Gene: ddna-template         →  template de DDNA
│   ├── Gene: changelog-format      →  formato de changelog (Keep a Changelog)
│   └── Gene: api-docs-format       →  formato de documentação de API
│
├── 🔒 Cromossomo 4: SEGURANÇA
│   ├── Gene: permission-model      →  RBAC / ACL padrão
│   ├── Gene: deny-patterns         →  padrões proibidos (hardcoded secrets, SQL injection)
│   ├── Gene: audit-requirements    →  o que deve ser auditado
│   └── Gene: secret-management     →  vault/env/ SOP
│
└── ⚙️ Cromossomo 5: CI
    ├── Gene: pipeline-template      →  template de CI (GitHub Actions, GitLab CI)
    ├── Gene: quality-gates          →  gates habilitados e thresholds
    ├── Gene: hooks                  →  pre-commit, pre-push hooks
    └── Gene: release-workflow       →  versionamento, changelog auto, release drafter
```

### 2.1 Formato do DNA Manifest

```yaml
# .cosca/dna/manifest.yaml
dna:
  version: "1.0.0"
  source:
    project: "cosca-test"
    path: "internal/embed/cosca/engines/engineering-dna/"
    commit: "a1b2c3d4e5f6..."
    inherited_at: "2026-07-30T10:00:00Z"
  genes:
    architecture:
      directory-structure:
        value: "standard-go"
        source: "parent"
        mutable: true
        inherited_at: "2026-07-30T10:00:00Z"
      code-patterns:
        value: "manager-store"
        source: "parent"
        mutable: true
        inherited_at: "2026-07-30T10:00:00Z"
    testing:
      coverage-threshold:
        value: 80
        source: "parent"
        mutable: true
        inherited_at: "2026-07-30T10:00:00Z"
    documentation:
      readme-template:
        value: "cosca-standard"
        source: "parent"
        mutable: true
    security:
      deny-patterns:
        - "hardcoded-secrets"
        - "sql-concatenation"
        - "eval-user-input"
        source: "parent"
        mutable: true
    ci:
      pipeline-template:
        value: "go-ci.yml"
        source: "parent"
        mutable: true
```

### 2.2 Genes Imutáveis vs Mutáveis

| Tipo | Pode ser sobrescrito? | Exemplo |
|------|----------------------|---------|
| **Imutável** | ❌ — protegido pela CONSTITUIÇÃO | P1-P8, regras de segurança críticas |
| **Mutável** | ✅ — Don pode alterar via override | coverage-threshold, template de README |
| **Opcional** | ✅ — Pode ser desativado | release-workflow (se não for fazer releases) |

---

## 3. PIPELINE DE HERANÇA (6 FASES)

O pipeline executa quando `cosca init --dna <source-project>` é invocado. Tempo alvo: **< 10s**.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                ENGINEERING DNA — PIPELINE DE HERANÇA COMPLETO                  │
│                              Executado no init --dna                            │
│                                                                                │
│  TRIGGERS:                                                                     │
│  ┌─────────────────┐                                                           │
│  │ cosca init      │                                                           │
│  │ --dna <project> │                                                           │
│  └────────┬────────┘                                                           │
│           ▼                                                                     │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 1 — LEITURA DO GENOMA PAI                                            │  │
│  │                                                                           │  │
│  │  1. Localiza o projeto pai:                                              │  │
│  │     ├── Busca em COSCA_HOME/projects/<source-project>/                    │  │
│  │     ├── Ou path absoluto se fornecido                                     │  │
│  │     └── Falha se projeto pai não existir ou não tiver DNA                │  │
│  │                                                                           │  │
│  │  2. Lê o DNA do pai:                                                      │  │
│  │     ├── internal/embed/cosca/CONSTITUTION.md (princípios fundacionais)         │  │
│  │     ├── internal/embed/cosca/QUALITY_GATES.md (gates e thresholds)             │  │
│  │     ├── internal/embed/cosca/workflows/ (templates de workflow)                │  │
│  │     ├── internal/embed/cosca/knowledge/ (princípios compilados pelo F9.1)     │  │
│  │     ├── internal/embed/cosca/cosca.config.yaml (configurações)                │  │
│  │     └── internal/embed/cosca/engines/engineering-dna/ (genes do pai)          │  │
│  │                                                                           │  │
│  │  3. Registra commit atual do pai para rastreabilidade                     │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 2s                                                     │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 2 — RESOLUÇÃO DE PLACEHOLDERS                                        │  │
│  │                                                                           │  │
│  │  Para cada arquivo copiado, substitui placeholders:                      │  │
│  │                                                                           │  │
│  │  │ Placeholder            │ Substituído por                     │         │
│  │  │────────────────────────│─────────────────────────────────────│         │
│  │  │ {{project_name}}       │ Nome do novo projeto               │         │
│  │  │ {{project_slug}}       │ Nome em formato slug (cosca-web)   │         │
│  │  │ {{owner}}              │ Don do novo projeto (ou herda)     │         │
│  │  │ {{description}}        │ Descrição do novo projeto          │         │
│  │  │ {{dna_source}}         │ Nome do projeto pai                │         │
│  │  │ {{dna_version}}        │ Versão do DNA herdado              │         │
│  │  │ {{dna_inherited_at}}   │ Timestamp da herança               │         │
│  │  │ {{year}}               │ Ano atual                          │         │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 2s                                                     │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 3 — GERAÇÃO DO PROJETO FILHO                                        │  │
│  │                                                                           │  │
│  │  1. Cria estrutura de diretórios baseada no gene directory-structure     │  │
│  │                                                                           │  │
│  │  2. Copia e processa templates:                                           │  │
│  │     ├── internal/embed/cosca/CONSTITUTION.md → internal/embed/cosca/CONSTITUTION.md  │  │
│  │     ├── internal/embed/cosca/QUALITY_GATES.md → internal/embed/cosca/QUALITY_GATES.md│  │
│  │     ├── internal/embed/cosca/workflows/* → internal/embed/cosca/workflows/*         │  │
│  │     ├── internal/embed/cosca/knowledge/* → internal/embed/cosca/knowledge/*         │  │
│  │     ├── .github/workflows/ci.yml → .github/workflows/ci.yml              │  │
│  │     ├── templates/README.md → README.md                                   │  │
│  │     ├── templates/ADR-0001.md → docs/adr/ADR-0001.md                      │  │
│  │     ├── templates/CHANGELOG.md → CHANGELOG.md                             │  │
│  │     └── templates/.gitignore → .gitignore                                 │  │
│  │                                                                           │  │
│  │  3. Aplica mutações do Don (se houver) — ver §4                          │  │
│  │                                                                           │  │
│  │  4. Gera DNA manifest no filho:                                           │  │
│  │     └── .cosca/dna/manifest.yaml                                          │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 4s                                                     │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 4 — CUSTOMIZAÇÃO DO FILHO                                            │  │
│  │                                                                           │  │
│  │  1. Cria .cosca/dna/mutations.yaml vazio (pronto para mutações)          │  │
│  │                                                                           │  │
│  │  2. Cria .cosca/dna/evolution.yaml com:                                   │  │
│  │     ├── parent_commit: commit atual do pai                                │  │
│  │     ├── last_sync: timestamp                                              │  │
│  │     └── pending_updates: []                                               │  │
│  │                                                                           │  │
│  │  3. Aplica template de opencode.json para o novo projeto                  │  │
│  │     ├── Registra agentes padrão                                           │  │
│  │     ├── Configura permissões                                              │  │
│  │     └── Ajusta paths para o novo projeto                                 │  │
│  │                                                                           │  │
│  │  4. Se --git, inicializa repositório git e faz commit inicial             │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 2s                                                     │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 5 — VALIDAÇÃO PÓS-HERANÇA                                            │  │
│  │                                                                           │  │
│  │  Verificações:                                                            │  │
│  │                                                                           │  │
│  │  1. ✅ Estrutura de diretórios gerada corretamente                        │  │
│  │     ├── Verifica diretórios obrigatórios existem                          │  │
│  │     └── Verifica arquivos obrigatórios existem                            │  │
│  │                                                                           │  │
│  │  2. ✅ Placeholders resolvidos (nenhum {{...}} residual)                  │  │
│  │     └── Varre todos os arquivos gerados em busca de placeholders          │  │
│  │                                                                           │  │
│  │  3. ✅ DNA manifest válido                                                 │  │
│  │     └── schema validation do manifest.yaml                                │  │
│  │                                                                           │  │
│  │  4. ✅ Mutações aplicadas (se houver)                                     │  │
│  │     └── Confirma que genes mutados estão com valor correto                │  │
│  │                                                                           │  │
│  │  5. ⚠️ Aviso se genes imutáveis foram modificados                         │  │
│  │     └── (Não bloqueia — Don tem veto absoluto, mas alerta)               │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 2s                                                     │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 6 — RELATÓRIO DE HERANÇA                                             │  │
│  │                                                                           │  │
│  │  Gera .cosca/reports/dna-inheritance-report.md:                         │  │
│  │                                                                           │  │
│  │  ┌──────────────────────────────────────────────────────────┐            │  │
│  │  │ # DNA INHERITANCE REPORT                                  │            │  │
│  │  │                                                           │            │  │
│  │  │ Project: cosca-web                                        │            │  │
│  │  │ Source: cosca-test                                        │            │  │
│  │  │ DNA Version: 1.0.0                                        │            │  │
│  │  │ Inherited at: 2026-07-30T10:00:00Z                        │            │  │
│  │  │                                                           │            │  │
│  │  │ Genes inherited: 18/18 (100%)                             │            │  │
│  │  │ Mutations applied: 2                                      │            │  │
│  │  │ Immutable genes: 3 (protected)                            │            │  │
│  │  │                                                           │            │  │
│  │  │ ## Genes Inherited                                        │            │  │
│  │  │ - architecture/directory-structure: standard-go ✅        │            │  │
│  │  │ - architecture/code-patterns: manager-store ✅            │  │            │
│  │  │ - testing/coverage-threshold: 80% ✅                      │            │  │
│  │  │ - ... (full list)                                         │            │  │
│  │  │                                                           │            │  │
│  │  │ ## Mutations Applied                                      │            │  │
│  │  │ - testing/coverage-threshold: 80% → 75% (Don override)   │            │  │
│  │  │ - ci/pipeline-template: go-ci.yml → node-ci.yml           │            │  │
│  │  │                                                           │            │  │
│  │  │ ## Next Steps                                             │            │  │
│  │  │ 1. Review inherited DNA: cosca dna status                 │            │  │
│  │  │ 2. Customize genes: cosca dna mutate <gene> <value>      │            │  │
│  │  │ 3. Check for parent evolution: cosca dna sync --check     │            │  │
│  │  └──────────────────────────────────────────────────────────┘            │  │
│  │                                                                           │  │
│  ⏱ Tempo alvo: < 1s                                                       │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.1 Algoritmo Central

```
function run_dna_inheritance(project_name, source_project, mutations):
    // === FASE 1: LEITURA DO GENOMA PAI ===
    parent_path = resolve_project_path(source_project)
    parent_dna = load_dna_manifest(parent_path)
    constitution = read_file(parent_path + "/internal/embed/cosca/CONSTITUTION.md")
    quality_gates = read_file(parent_path + "/internal/embed/cosca/QUALITY_GATES.md")
    workflows = read_dir(parent_path + "/internal/embed/cosca/workflows/")
    knowledge = read_dir(parent_path + "/internal/embed/cosca/knowledge/")
    parent_commit = get_git_commit(parent_path)

    // === FASE 2: RESOLUÇÃO DE PLACEHOLDERS ===
    placeholders = {
        "{{project_name}}": project_name,
        "{{project_slug}}": slugify(project_name),
        "{{owner}}": get_current_don(),
        "{{dna_source}}": source_project,
        "{{dna_version}}": parent_dna.dna.version,
        "{{dna_inherited_at}}": now(),
        "{{year}}": year(now()),
    }

    // === FASE 3: GERAÇÃO DO FILHO ===
    child_path = create_project_directory(project_name)
    scaffold_structure(child_path, parent_dna.dna.genes.architecture.directory_structure)
    copy_with_placeholders(child_path, constitution, placeholders)
    copy_with_placeholders(child_path, quality_gates, placeholders)
    for wf in workflows:
        copy_with_placeholders(child_path, wf, placeholders)
    for k in knowledge:
        copy_with_placeholders(child_path, k, placeholders)
    generate_ci_pipeline(child_path, parent_dna.dna.genes.ci.pipeline_template, placeholders)

    // Aplica mutações
    for gene_path, new_value in mutations:
        apply_mutation(child_path, gene_path, new_value)

    // Gera manifest
    write_dna_manifest(child_path, parent_dna, parent_commit, mutations)

    // === FASE 4: CUSTOMIZAÇÃO ===
    create_mutations_yaml(child_path, mutations)
    create_evolution_yaml(child_path, parent_commit)
    generate_opencode_config(child_path, project_name, placeholders)
    if options.git:
        git_init_and_commit(child_path)

    // === FASE 5: VALIDAÇÃO ===
    errors = []
    errors += validate_directory_structure(child_path)
    errors += validate_placeholders_resolved(child_path)
    errors += validate_manifest(child_path)
    errors += validate_mutations_applied(child_path, mutations)
    if errors:
        log_warnings(errors)

    // === FASE 6: RELATÓRIO ===
    report = generate_inheritance_report(child_path, parent_dna, mutations, errors)
    write_report(child_path, report)

    return report
```

### 3.2 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|-------------|----------------|
| Leitura do genoma pai | O(F) onde F = arquivos do DNA | < 2s |
| Resolução de placeholders | O(F × P) onde P = placeholders | < 2s |
| Geração do projeto filho | O(F + D) onde D = diretórios | < 4s |
| Customização | O(1) | < 2s |
| Validação | O(F) | < 2s |
| Relatório | O(1) | < 1s |
| **Total** | | **< 13s** |

---

## 4. MUTAÇÃO — DON OVERRIDE

### 4.1 Conceito

Assim como na genética, uma **mutação** é uma alteração em um gene específico do DNA herdado. O Don (autoridade máxima) pode mutar qualquer gene mutável a qualquer momento.

### 4.2 Mecanismo de Mutação

```
cosca dna mutate <gene-path> <value> [--reason "<justificativa>"]
```

Exemplos:
```bash
# Mutar threshold de cobertura
cosca dna mutate testing/coverage-threshold 75 --reason "Projeto tem muitos testes de integração que não contam na cobertura de linha"

# Mutar template de CI (Go → Node.js)
cosca dna mutate ci/pipeline-template node-ci.yml --reason "Projeto frontend não precisa de pipeline Go"

# Mutar template de README
cosca dna mutate documentation/readme-template web-readme --reason "README precisa ter seção de design system"

# Ver mutações atuais
cosca dna mutations

# Histórico de mutações
cosca dna mutations --history
```

### 4.3 Formato do Arquivo de Mutações

```yaml
# .cosca/dna/mutations.yaml
dna_mutations:
  - gene: testing/coverage-threshold
    original: 80
    current: 75
    reason: "Projeto tem muitos testes de integração que não contam na cobertura de linha"
    mutated_at: "2026-07-30T11:00:00Z"
    mutated_by: "Don"
    status: active

  - gene: ci/pipeline-template
    original: "go-ci.yml"
    current: "node-ci.yml"
    reason: "Projeto frontend não precisa de pipeline Go"
    mutated_at: "2026-07-30T11:05:00Z"
    mutated_by: "Don"
    status: active

  - gene: testing/mock-preferences
    original: "testify"
    current: "vitest-msw"
    reason: "Ecossistema TypeScript tem MSW como padrão"
    mutated_at: "2026-07-30T11:10:00Z"
    mutated_by: "Don"
    status: active
    supersedes: "mutation-20260730-001"
```

### 4.4 Regras de Mutação

| Regra | Descrição | Ação |
|-------|-----------|------|
| **Gene existe** | O gene deve existir no DNA manifesto | Erro se não existir |
| **Gene é mutável** | Genes imutáveis não podem ser alterados | Erro + explicação + sugestão de emendar CONSTITUIÇÃO |
| **Tipo correto** | Valor deve ter o tipo correto (int, string, list) | Erro de tipo |
| **Rastro obrigatório** | Toda mutação deve ter reason | Warn (não bloqueia) |
| **Mutação reversível** | Toda mutação pode ser revertida | `cosca dna revert <gene>` |
| **Notificação de conflito** | Se gene foi atualizado no pai desde a última mutação | Warn + diff |

### 4.5 Reversão de Mutação

```bash
# Reverter uma mutação ao valor original do pai
cosca dna revert testing/coverage-threshold

# Reverter e manter no histórico
cosca dna revert testing/coverage-threshold --keep-history

# Reverter todas as mutações (voltar ao DNA puro do pai)
cosca dna revert --all
```

### 4.6 Genes Imutáveis (Protegidos pela CONSTITUIÇÃO)

Estes genes **NÃO** podem ser mutados por `cosca dna mutate` — apenas por emenda constitucional:

| Gene | Protegido por | Por quê |
|------|--------------|---------|
| `security/deny-patterns` | P1 (Segurança acima de funcionalidade) | Padrões de segurança nunca devem ser relaxados |
| `security/permission-model` | P1 + G1-G6 | Modelo de permissão é fundacional |
| `architecture/module-boundaries` | P2 (Código executado é a verdade) | Quebrar boundaries quebra a arquitetura |
| `ci/quality-gates` | QUALITY_GATES.md (G0-G4) | Gates protegem a qualidade mínima |

> Se o Don deseja alterar um gene imutável, o caminho é:
> 1. `cosca experience amendments propose` — propõe emenda constitucional
> 2. Don aprova
> 3. CONSTITUIÇÃO é alterada
> 4. Gene torna-se mutável

---

## 5. EVOLUÇÃO — PROPAGAÇÃO DE ATUALIZAÇÕES

### 5.1 Conceito

Quando o projeto pai evolui (novos princípios compilados pelo F9.1, gates atualizados, workflows melhorados), os projetos filhos podem **puxar** essas atualizações. O DNA Engine detecta genes desatualizados e oferece sincronização seletiva.

### 5.2 Pipeline de Sincronização

```bash
# Verificar se há atualizações disponíveis
cosca dna sync --check

# Aplicar atualizações (interativo — Don escolhe quais genes aceitar)
cosca dna sync

# Aplicar atualizações automaticamente (aceita todas não-conflitantes)
cosca dna sync --auto

# Ver diff detalhado entre pai e filho
cosca dna sync --diff
```

### 5.3 Algoritmo de Sincronização

```
function check_dna_evolution(child_path):
    child_manifest = load_dna_manifest(child_path)
    parent_path = resolve_project_path(child_manifest.dna.source.project)
    parent_manifest = load_dna_manifest(parent_path)
    parent_current_commit = get_git_commit(parent_path)

    if parent_current_commit == child_manifest.dna.source.commit:
        return {"status": "up-to-date", "updates": []}

    // Calcula diff gene-by-gene
    updates = []
    for category, genes in parent_manifest.dna.genes:
        for gene_name, parent_gene in genes:
            child_gene = child_manifest.dna.genes[category][gene_name]

            if child_gene.source == "mutated":
                // Gene foi mutado pelo Don → verificar conflito
                if parent_gene.value != child_gene.original:
                    updates.append({
                        "gene": f"{category}/{gene_name}",
                        "type": "conflict",
                        "parent_value": parent_gene.value,
                        "child_value": child_gene.current,
                        "child_original": child_gene.original,
                        "recommendation": "review"
                    })
                continue

            if parent_gene.value != child_gene.value:
                updates.append({
                    "gene": f"{category}/{gene_name}",
                    "type": "update",
                    "old": child_gene.value,
                    "new": parent_gene.value,
                    "recommendation": "accept"
                })

    return {"status": "updates-available", "updates": updates}
```

### 5.4 Estratégias de Merge

| Cenário | Estratégia | Ação |
|---------|-----------|------|
| Gene herdado sem mutação | **Auto-merge** | Atualiza silenciosamente |
| Gene mutado pelo Don, pai também mudou | **Don decide** | Mostra diff, Don escolhe qual versão manter |
| Gene mutado pelo Don, pai não mudou | **Preserva** | Mantém mutação do Don |
| Gene removido do pai | **Pergunta** | Don decide se remove ou mantém |
| Novo gene adicionado no pai | **Auto-adiciona** | Novo gene é incorporado |

### 5.5 Exemplo de Output de Sync

```bash
$ cosca dna sync --check

🧬 DNA Evolution Check: cosca-web → cosca-test
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📌 Parent commit: a1b2c3d (2026-07-30) → e5f6g7h (2026-08-15)
   Last sync: 2026-07-30

📦 Updates available: 4

  ⬆️ AUTO  testing/coverage-threshold: 70% → 80%
       (parent evolved: +10% threshold based on F9.1 data)
       → Will be applied automatically.

  ⬆️ AUTO  architecture/directory-structure: standard-go → standard-go-v2
       (parent evolved: added web/ and proto/ directories)
       → Will be applied automatically.

  ⚠️ CONFLICT  ci/pipeline-template:
       Parent: go-ci-v2.yml
       You:    node-ci.yml (mutated)
       → You need to decide which version to keep.

  🆕 NEW  security/secret-scan-frequency: weekly
       (New gene added in parent v1.1.0)
       → Will be added automatically.

Run `cosca dna sync` to apply.
```

---

## 6. VERSIONAMENTO DO DNA

### 6.1 Ciclo de Vida do DNA

```
DNA v1.0.0 (stable)
    │
    ├── cosca init --dna → filhos herdam v1.0.0
    │
    ├── Pai evolui (novos genes, thresholds atualizados)
    │     │
    │     └── DNA v1.1.0
    │           │
    │           ├── sync --check → filhos detectam v1.1.0
    │           └── sync → filhos sobem para v1.1.0
    │
    ├── Mudança breaking (gene removido, estrutura alterada)
    │     │
    │     └── DNA v2.0.0
    │           │
    │           └── sync --check → MAJOR upgrade detected
    │               └── Don must explicitly approve
    │
    └── DNA deprecated → projeto pai define novo DNA source
```

### 6.2 Versionamento Semântico

| Componente | Quando incrementar | Exemplo |
|-----------|-------------------|---------|
| **MAJOR** | Gene removido, breaking change, CONSTITUIÇÃO alterada | v1.0.0 → v2.0.0 |
| **MINOR** | Novo gene adicionado, threshold ajustado, workflow adicionado | v1.0.0 → v1.1.0 |
| **PATCH** | Bug fix em template, placeholder corrigido, docs melhoradas | v1.0.0 → v1.0.1 |

### 6.3 Comandos de Versionamento

```bash
# Ver versão atual do DNA
cosca dna version

# Histórico de versões do DNA do projeto
cosca dna history

# Definir versão específica (voltar)
cosca dna checkout v1.0.0

# Ver changelog do DNA
cosca dna changelog
```

### 6.4 DNA Source Chain

```
cosca-test (DNA source)
    │
    ├── cosca-web (herdou v1.0.0, atualmente v1.2.0)
    │     └── cosca-web-admin (herdou de cosca-web v1.2.0)
    │
    ├── cosca-api (herdou v1.0.0, atualmente v1.1.0)
    │
    └── cosca-mobile (herdou v1.0.0, mutado para React Native)
          └── cosca-mobile-android (herdou de cosca-mobile)
```

Projetos podem herdar de qualquer outro projeto Cosca que tenha DNA — não apenas do source original. Isso permite **cadeias de herança**:

```
cosca-platform (DNA fundacional)
    └── cosca-saas (herdou, customizou para SaaS)
         └── cosca-saas-admin (herdou do SaaS, adicionou admin)
         └── cosca-saas-api (herdou do SaaS, focou em API)
```

---

## 7. EXEMPLO COMPLETO: cosca-web

### 7.1 Cenário

O Don quer criar um novo projeto chamado **cosca-web** (frontend Next.js) que herde o DNA de engenharia do **cosca-test** (projeto Go existente).

```bash
cosca init cosca-web --dna cosca-test --description "Web console for Cosca platform" --owner "Don"
```

### 7.2 Pipeline em Ação

#### Fase 1 — Leitura do Genoma Pai
```
Projeto pai encontrado: cosca-test (internal/embed/cosca/engines/engineering-dna/)
CONSTITUTION.md lida: 8 princípios imutáveis (P1-P8)
QUALITY_GATES.md lido: 10 gates (G0-G4 + G0.5)
Workflows encontrados: 36 workflows
Knowledge encontrado: 8 diretórios (architecture, patterns, best-practices, etc.)
Parent commit: a1b2c3d4e5f6g7h8i9j
```

#### Fase 2 — Resolução de Placeholders
```
{{project_name}} → cosca-web
{{project_slug}} → cosca-web
{{owner}} → Don
{{description}} → Web console for Cosca platform
{{dna_source}} → cosca-test
{{dna_version}} → 1.0.0
{{dna_inherited_at}} → 2026-07-30T10:00:00Z
{{year}} → 2026
```

#### Fase 3 — Geração do Projeto Filho
```
Diretório criado: /home/cosca/Documents/cosca-web/
Estrutura gerada (gene: standard-go):
├── cmd/cosca-web/
├── internal/
├── pkg/
├── api/
├── web/
├── test/
├── docs/
├── internal/embed/cosca/
│   ├── CONSTITUTION.md ✅ (placeholders resolvidos)
│   ├── QUALITY_GATES.md ✅
│   ├── workflows/ ✅ (36 workflows herdados)
│   ├── knowledge/ ✅ (8 diretórios)
│   └── engines/engineering-dna/ ✅
├── .github/workflows/ci.yml ✅
├── README.md ✅
├── CHANGELOG.md ✅
├── docs/adr/ADR-0001.md ✅
├── .gitignore ✅
└── opencode.json ✅
```

#### Fase 4 — Customização
```
.cosca/dna/manifest.yaml criado
.cosca/dna/mutations.yaml criado (vazio)
.cosca/dna/evolution.yaml criado
Git inicializado: commit "Initial project: cosca-web (inherited from cosca-test DNA v1.0.0)"
```

#### Fase 5 — Validação
```
✅ Estrutura de diretórios: OK (18/18 diretórios esperados)
✅ Placeholders resolvidos: OK (0 resíduos de {{...}})
✅ DNA manifest: OK (schema válido)
✅ Mutações: N/A (sem mutações neste exemplo)
⚠️ Aviso: Nenhuma mutação aplicada — Don pode querer ajustar thresholds
```

#### Fase 6 — Relatório
```
📄 DNA Inheritance Report gerado: .cosca/reports/dna-inheritance-report.md
```

### 7.3 Resultado Final

```
cosca-web/
├── internal/embed/cosca/
│   ├── CONSTITUTION.md          # Herdado: 8 princípios imutáveis
│   ├── QUALITY_GATES.md         # Herdado: G0-G4 + G0.5
│   ├── workflows/               # Herdado: 36 workflows
│   ├── knowledge/               # Herdado: princípios compilados do F9.1
│   └── engines/engineering-dna/ # DNA manifesto
├── .cosca/dna/
│   ├── manifest.yaml            # DNA herdado
│   ├── mutations.yaml           # Vazio (pronto para mutações)
│   └── evolution.yaml           # Rastro de evolução
├── .cosca/reports/
│   └── dna-inheritance-report.md
├── .github/workflows/
│   └── ci.yml                   # Pipeline herdado e processado
├── README.md                    # README com placeholders resolvidos
├── docs/adr/ADR-0001.md         # ADR template herdado
├── CHANGELOG.md                 # Changelog format herdado
├── .gitignore                   # Regras herdadas
└── opencode.json                # Config Cosca personalizada
```

### 7.4 Don Faz Override

```bash
# Don decide que cobertura mínima deve ser 75% (não 80%)
cosca dna mutate testing/coverage-threshold 75 --reason "Projeto frontend: cobertura de linha é menos relevante que cobertura de integração"

# Don decide trocar pipeline Go por Node.js
cosca dna mutate ci/pipeline-template node-ci.yml --reason "Projeto é Next.js, não Go"

# Don adiciona gene customizado
cosca dna mutate documentation/readme-template web-readme --reason "README precisa ter seção de design system e acessibilidade"
```

### 7.5 Pai Evolui, Filho Sincroniza

Três meses depois, `cosca-test` evoluiu:
- F9.1 compilou novos princípios de testing
- QUALITY_GATES.md atualizado com Gate 5
- Novos workflows adicionados: `accessibility-audit.md`, `design-review.md`

```bash
$ cosca dna sync --check

🧬 DNA Evolution Check: cosca-web → cosca-test
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Updates available: 5

  ⬆️ AUTO  testing/coverage-threshold: (mutado, sem conflito)
  ⬆️ AUTO  testing/test-patterns → novas patterns de component testing
  🆕 NEW  quality/gate-5 → Gate 5 de acessibilidade
  🆕 NEW  workflows/accessibility-audit.md
  🆕 NEW  workflows/design-review.md

$ cosca dna sync
✅ Sync complete: cosca-web updated to DNA v1.3.0
```

---

## 8. CLI E AUTOMAÇÃO

### 8.1 Comandos

```bash
# Inicializar novo projeto com herança de DNA
cosca init <project> --dna <source> [--description <desc>] [--owner <owner>] [--git]

# Ver status do DNA do projeto atual
cosca dna status

# Ver genes do DNA
cosca dna genes [--category architecture|testing|documentation|security|ci]
cosca dna genes --json

# Mutar um gene
cosca dna mutate <gene-path> <value> [--reason "<reason>"]

# Reverter mutação
cosca dna revert <gene-path> [--keep-history]
cosca dna revert --all

# Ver mutações atuais
cosca dna mutations [--history]

# Verificar evolução disponível
cosca dna sync --check [--diff]
cosca dna sync --diff

# Sincronizar com pai
cosca dna sync [--auto] [--dry-run]

# Versionamento
cosca dna version
cosca dna history
cosca dna checkout <version>
cosca dna changelog

# Chain de herança
cosca dna tree
cosca dna parents
cosca dna children

# Relatório
cosca dna report
```

### 8.2 Opções

| Flag | Descrição | Default |
|------|-----------|---------|
| `--dna` | Projeto pai para herdar DNA | N/A (obrigatório para --dna) |
| `--description` | Descrição do novo projeto | "" |
| `--owner` | Don do novo projeto | Don atual |
| `--git` | Inicializar repositório git | false |
| `--dry-run` | Simula sem criar arquivos | false |
| `--auto` | Aceita automaticamente atualizações não conflitantes | false |
| `--diff` | Mostra diff detalhado | false |
| `--keep-history` | Mantém mutação no histórico mesmo ao reverter | false |
| `--category` | Filtra genes por categoria | all |
| `--json` | Output em JSON | false |

### 8.3 CI Gate

```yaml
# .github/workflows/dna-evolution-check.yml
name: DNA Evolution Check
on:
  schedule:
    - cron: "0 6 * * 1"  # Semanal (segunda 06:00)
  workflow_dispatch:

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Check DNA Evolution
        run: |
          cosca dna sync --check --json > dna-check.json

      - name: Notify if Updates Available
        run: |
          UPDATES=$(jq '.updates | length' dna-check.json)
          if [ "$UPDATES" -gt 0 ]; then
            echo "🧬 $UPDATES DNA update(s) available from parent project"
            jq -r '.updates[] | "  \(.type) \(.gene): \(.old // "—") → \(.new // "—")"' dna-check.json
            echo "Run 'cosca dna sync' to apply."
          else
            echo "✅ DNA is up to date"
          fi
```

---

## 9. MÉTRICAS DO ENGINE

### 9.1 Métricas Internas

| Métrica | Definição | Alvo | Fonte |
|---------|-----------|:----:|-------|
| **Pipeline Duration** | Tempo total de herança | < 10s | Engine |
| **Genes Inherited** | Genes transferidos do pai | 18/18 | Fase 3 |
| **Placeholders Resolved** | Placeholders substituídos | 100% | Fase 2 |
| **Mutations Applied** | Mutações do Don aplicadas | 0-N (informativo) | Fase 3 |
| **Validation Errors** | Erros na validação pós-herança | 0 | Fase 5 |
| **Sync Latency** | Tempo entre evolução do pai e sync do filho | < 7 dias | Evolução |

### 9.2 Métricas de Saúde do DNA

| Métrica | Definição | Alvo |
|---------|-----------|:----:|
| **DNA Divergence** | % de genes mutados vs herdados | < 30% |
| **Sync Freshness** | Dias desde último sync com o pai | < 30 dias |
| **Mutation Revert Rate** | % de mutações revertidas | < 10% |
| **Sync Conflict Rate** | % de syncs com conflitos | < 5% |
| **Child Projects** | Número de projetos que herdaram deste | > 0 |

### 9.3 Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| Pipeline > 15s | 🟡 MÉDIO | Verificar leitura de templates (muitos arquivos?) |
| Placeholder não resolvido | 🔴 ALTO | Template tem placeholder sem substituição — bug |
| Gene imutável mutado | 🔴 ALTO | Don tentou mutar gene protegido — educar sobre fluxo |
| Sync atrasado > 60 dias | 🟡 MÉDIO | Notificar Don: DNA do pai evoluiu e filho está desatualizado |
| Mais de 50% dos genes mutados | 🟡 MÉDIO | Filho divergiu muito do pai — considerar mudar source |
| Conflito de sync não resolvido | 🟡 MÉDIO | Don precisa decidir sobre gene conflitante |

---

## 10. CASOS DE BORDA E ANTI-PADRÕES

### 10.1 Casos de Borda

| Caso | O que acontece | Tratamento |
|------|---------------|------------|
| **Projeto pai não tem DNA** | `--dna` falha com erro claro | Mensagem: "Projeto 'X' não tem Engineering DNA. Execute 'cosca dna init' no pai primeiro." |
| **Cadeia circular** (A herda de B que herda de A) | Detectado na Fase 1 | Erro: "Herança circular detectada. DNA chain: A → B → A." |
| **Gene removido do pai entre versões** | Filho tem gene órfão | Sync pergunta se Don quer remover ou manter |
| **Pai deletado / inacessível** | Sync não consegue ler o pai | Warn: "Parent project not found. Last known commit: .... DNA frozen at v1.0.0." |
| **Mutação em gene que não existe mais** | Gene foi removido na nova versão do pai | Sync marca mutação como `orphaned` — Don decide destino |
| **Múltiplos pais** (herdar de 2+ fontes) | Não suportado na v1.0 | Erro: "Multi-parent inheritance not supported in DNA v1.0" |
| **Filho vira pai de outro** | Cadeia de herança natural | Permitido — cada projeto pode ser source de DNA |
| **Don muta gene imutável** | Engine bloqueia | Explica o fluxo de emenda constitucional |

### 10.2 Anti-Padrões

| Anti-Padrão | Por que evitar | Como detectar |
|-------------|---------------|---------------|
| **Mutar tudo** | Se 90% dos genes são mutados, por que herdar deste pai? | Divergence > 50% → sugerir criar DNA source próprio |
| **Nunca sincronizar** | Filho perde atualizações de segurança e qualidade | Sync freshness > 90 dias → notificar |
| **Herança profunda** (A → B → C → D → E) | Dificuldade de rastrear origem de um gene | Alertar se chain > 3 níveis |
| **Ignorar genes imutáveis** | Tentar mutar security/deny-patterns é risco de segurança | Bloqueio + explicação educativa |
| **Criar DNA source sem CONSTITUIÇÃO** | DNA sem base constitucional não tem fundação | Validar CONSTITUTION.md existe no source |
| **Sobrescrever DNA do filho manualmente** | Editar .cosca/dna/manifest.yaml à mão | Engine detecta divergência e pergunta se Don quer atualizar via mutate |

### 10.3 Regras de Resiliência

```
1. Falha no projeto pai → Herança não executa
   (sem genoma, não há herança)

2. Falha em 1 template → Os outros templates continuam
   (resiliência por template individual)

3. Falha na escrita do relatório → Herança concluída sem relatório
   (warn: relatório não gerado, mas projeto foi criado)

4. Falha no git init → Projeto criado sem git
   (warn: git não inicializado, executar git init manualmente)

5. Falha no sync (pai inacessível) → DNA permanece na versão atual
   (estado anterior preservado, sem perda de dados)

6. Mutação inválida → Gene mantém valor original
   (rollback automático, mutação não aplicada)
```

---

## HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — definição, 5 cromossomos (18 genes), pipeline de herança 6 fases, mutação com Don override, evolução com sync, versionamento semântico, exemplo cosca-web, CLI, métricas, casos de borda |

---

> *"Um novo projeto não precisa redescobrir o que a engenharia já aprendeu. O DNA carrega décadas de acertos e erros em cada gene. Herde, mute, evolua — mas nunca comece do zero."*
> — Cosca Architecture Chief, 2026-07-30
