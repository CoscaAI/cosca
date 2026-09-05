# WORKFLOW: memorize-commit

> **Version**: 1.0.0 | **Status**: active | **Category**: memory | **Last Updated**: 2026-07-30
>
> **Descrição**: Após cada commit, captura o impacto da mudança e registra na Timeline de Engenharia. Gera relatório estruturado com métricas de arquivos, LOC, cobertura, tempo, custo, aprendizados e débitos. Alimenta o Engineering Score e o Evolution Timeline.
>
> **Trigger**: Pós-commit (manual ou via git hook) | **Executor**: Cosca Kernel

## OBJECTIVE

Registrar todo commit como um evento de engenharia com métricas quantitativas e qualitativas, criando uma base histórica para auditoria, evolução e tomada de decisão.

## INPUTS

| Name | Type | Required | Description |
|------|------|----------|-------------|
| commit_hash | string | No | Hash do commit (default: HEAD) |
| commit_msg | string | No | Mensagem do commit (default: extraída do git) |
| session_id | string | No | ID da sessão atual (default: data ISO) |
| force | boolean | No | Se true, recalcula mesmo para commits já memorizados |

## OUTPUTS

| Name | Type | Description |
|------|------|-------------|
| impact_report | object | Relatório estruturado do impacto do commit |
| timeline_entry | string | Entrada na Timeline de Engenharia |
| score_delta | object | Delta nos scores de engenharia (se houver) |
| learnings | array | Novos aprendizados extraídos do commit |

## PRECONDITIONS

1. Git commit existe (HEAD ou hash especificado)
2. Diretório `internal/embed/cosca/memory/timeline/` existe
3. `git diff` funcional no workspace

## POSTCONDITIONS

1. Commit registrado na Timeline de Engenharia
2. Impact Report armazenado em `memory/timeline/`
3. Learnings extraídos e registrados (se aplicável)
4. Engineering Score atualizado (se configured)

## DEPENDENCIES

- `git` (log, diff, show)
- `go test` (para cobertura, se disponível)

## STEPS

### Step 1: Collect Commit Data

- **Chief**: Kernel
- **Specialists**: N/A (execução direta)
- **Task**: Extrair dados brutos do commit via git
- **Output**: Dados brutos do commit

```bash
# Coletar metadados do commit
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
COMMIT_DATE=$(git log -1 --format="%ad" --date=iso 2>/dev/null)
COMMIT_MSG=$(git log -1 --format="%s" 2>/dev/null)
COMMIT_AUTHOR=$(git log -1 --format="%an" 2>/dev/null)

# Estatísticas de diff
FILES_CHANGED=$(git diff --stat HEAD~1..HEAD 2>/dev/null | tail -1 | grep -oP '\d+(?= file)' || echo "0")
INSERTIONS=$(git diff --stat HEAD~1..HEAD 2>/dev/null | tail -1 | grep -oP '\d+(?= insertion)' || echo "0")
DELETIONS=$(git diff --stat HEAD~1..HEAD 2>/dev/null | tail -1 | grep -oP '\d+(?= deletion)' || echo "0")

# Lista de arquivos modificados
FILES_LIST=$(git diff --name-only HEAD~1..HEAD 2>/dev/null)
```

### Step 2: Classify Change Type

- **Chief**: Kernel
- **Specialists**: N/A
- **Task**: Classificar o tipo de mudança baseado na mensagem e nos arquivos
- **Output**: Tipo de mudança categorizada

| Prefixo | Tipo | Categoria |
|---------|------|-----------|
| `feat:` | Feature | Capacidade |
| `fix:` | Bug Fix | Qualidade |
| `test:` | Teste | Qualidade |
| `docs:` | Documentação | Conhecimento |
| `refactor:` | Refatoração | Arquitetura |
| `chore:` | Manutenção | Infraestrutura |
| `learn:` | Aprendizado | Conhecimento |
| `perf:` | Performance | Performance |
| `sec:` | Segurança | Segurança |
| `ci:` | CI/CD | Infraestrutura |

### Step 3: Measure Coverage Delta (Optional)

- **Chief**: Testing
- **Specialists**: Unit Test Specialist
- **Task**: Se o projeto tiver cobertura configurada, medir delta
- **Output**: Delta de cobertura (ou skip)

### Step 4: Estimate Time & Cost

- **Chief**: Kernel
- **Specialists**: Analytics
- **Task**: Estimar tempo e custo com base no escopo da mudança
- **Output**: Estimativa de tempo e custo

> **Fórmula de custo**: `(INSERTIONS × 0.02 + FILES_CHANGED × 0.5) / 1000` USD
> **Fórmula de tempo**: `(INSERTIONS × 0.5 + FILES_CHANGED × 2)` segundos

### Step 5: Detect Learnings

- **Chief**: Memory
- **Specialists**: Semantic Memory
- **Task**: Analisar diff e mensagem em busca de padrões, técnicas ou descobertas
- **Output**: Lista de aprendizados (0..N)

> Critérios para aprendizado:
> - Nova técnica ou padrão identificado
> - Bug recorrente com nova causa raiz
> - Melhoria de processo ou workflow
> - Decisão arquitetural com trade-offs documentados

### Step 6: Detect Technical Debt Changes

- **Chief**: Technical Debt
- **Specialists**: N/A
- **Task**: Identificar se o commit introduz ou paga dívida técnica
- **Output**: Delta de débito técnico

### Step 7: Generate Impact Report

- **Chief**: Kernel
- **Specialists**: Documentation
- **Task**: Consolidar todos os dados no formato de relatório de impacto
- **Output**: Impact Report completo

```markdown
## Impact Report — {commit_hash}

| Métrica | Valor |
|---------|-------|
| **Commit** | `{hash}` |
| **Data** | {date} |
| **Autor** | {author} |
| **Mensagem** | {message} |
| **Tipo** | {type} |
| **Arquivos** | {files_changed} |
| **Inserções** | +{insertions} |
| **Deleções** | -{deletions} |
| **LOC Líquido** | +{net_loc} |
| **Cobertura** | {coverage_before}% → {coverage_after}% |
| **Tempo Est.** | {estimated_time} |
| **Custo Est.** | ${estimated_cost} |
| **Aprendizados** | {learnings_count} |
| **Débito Téc.** | {debt_delta} |
| **Regressões** | {regressions} |
```

### Step 8: Record in Engineering Timeline

- **Chief**: Memory
- **Specialists**: Memory Chief
- **Task**: Adicionar entrada cronológica na Timeline de Engenharia
- **Output**: Entrada na timeline

Formato da entrada:
```
| {time} | {emoji_tipo} {tipo} | {message} | +{ins}/-{del} | {files} | ${cost} | {learnings} | {hash} |
```

### Step 9: Update Engineering Score (Optional)

- **Chief**: Kernel
- **Specialists**: Analytics
- **Task**: Se Engineering Score estiver configurado, recalcular
- **Output**: Score atualizado ou skip

### Step 10: Auto-Evolution (Stages 7-8)

- **Chief**: Kernel
- **Specialists**: N/A
- **Task**: Executar Stages 7-8 do metacognition pipeline OBRIGATORIAMENTE
- **Output**: Learnings + Capability Profile atualizados

> **MANDATORY**: Este workflow só é considerado completo após Stages 7-8 executados.

## VALIDATION

1. Commit hash é válido e existe no git
2. Impact Report gerado com campos obrigatórios preenchidos
3. Timeline contém entrada para este commit
4. Nenhum erro de git durante coleta
5. Stages 7-8 executados

## SUCCESS CRITERIA

- [ ] Dados do commit coletados com sucesso
- [ ] Impact Report gerado e armazenado
- [ ] Timeline de Engenharia atualizada
- [ ] Learnings extraídos (se aplicável)
- [ ] Stages 7-8 do metacognition pipeline executados

## ERROR HANDLING

| Failure | Action |
|---------|--------|
| Commit hash inválido | Usar HEAD como fallback |
| `git diff` sem parent commit | Primeiro commit — registrar como "initial" |
| Cobertura não disponível | Skip coverage delta, marcar como N/A |
| Timestamp conflitante | Usar timestamp do git como autoridade |
| Commit já memorizado | Skip se não `force=true` |

## ENGINEERING TIMELINE FORMAT

A Timeline de Engenharia é um registro cronológico de todos os eventos significativos do runtime:

```
# Engineering Timeline

> Versão: 1.0.0 | Última atualização: {date}

| Horário | Tipo | Descrição | Impacto | Arquivos | Custo | Aprendizados | Commit |
|---------|------|-----------|---------|----------|-------|--------------|--------|
```

### Tipos de Evento

| Tipo | Emoji | Descrição |
|------|-------|-----------|
| feature | ✨ | Nova capacidade |
| fix | 🐛 | Correção de bug |
| test | 🧪 | Testes |
| docs | 📝 | Documentação |
| refactor | ♻️ | Refatoração |
| chore | 🔧 | Manutenção |
| learn | 🧠 | Aprendizado |
| perf | ⚡ | Performance |
| security | 🛡️ | Segurança |
| ci | 👷 | CI/CD |
| architecture | 🏗️ | Arquitetura |
| release | 🚀 | Release |
| decision | ⚖️ | Decisão |
| audit | 🔍 | Auditoria |

## IMPACT REPORT STORAGE

Cada Impact Report é armazenado em:

```
internal/embed/cosca/memory/timeline/impact-reports/{commit_hash}.md
```

## RELATED

- [Memory Model](../memory/MEMORY_MODEL.md) — Memory architecture
- [Learning Protocol](../memory/LEARNING_PROTOCOL.md) — Learning entry format
- [Knowledge Pipeline](knowledge-pipeline.md) — Knowledge compilation
- [Evolution Autonomy](cosca-evolution-autonomy.md) — Agent evolution
- [Quality Gates](../QUALITY_GATES.md) — Quality enforcement
- [CMI](../architecture/COGNITIVE_MATURITY.md) — Cognitive Maturity Index

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-30 | Cosca Kernel | Workflow inicial: 10 steps, Impact Report, Engineering Timeline, Auto-Evolution integration |
