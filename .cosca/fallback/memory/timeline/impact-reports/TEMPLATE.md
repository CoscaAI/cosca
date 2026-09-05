# Impact Report Template

> **Propósito**: Template canônico para relatórios de impacto de commit.
> **Workflow**: [memorize-commit](../../../workflows/memorize-commit.md)
> **Geração**: Automática via post-commit hook ou manual via memorize-commit workflow
> **Versão do Template**: 1.0.0

---

## Impact Report — `{commit_hash}`

> Gerado por: {generator} | Data: {timestamp}
> Workflow: [memorize-commit](../../../workflows/memorize-commit.md)

| Métrica | Valor |
|---------|-------|
| **Commit** | `{commit_hash}` |
| **Hash Full** | `{commit_hash_full}` |
| **Data** | {commit_date} |
| **Autor** | {author} |
| **Email** | {email} |
| **Mensagem** | {message} |
| **Tipo** | {type} {emoji} |
| **Categoria** | {category} |
| **Arquivos** | {files_changed} |
| **Inserções** | +{insertions} |
| **Deleções** | -{deletions} |
| **LOC Líquido** | {net_loc} |
| **Cobertura** | {coverage} |
| **Tempo Est.** | {estimated_time} |
| **Custo Est.** | ${estimated_cost} |
| **Agente** | {agent_name} |

### Engineering Score (F7.3)

| Dimensão | Score | Peso | Contribuição |
|----------|:-----:|:----:|:------------:|
| Architecture | {arch_score} | 0.20 | {arch_contrib} |
| Code Quality | {code_score} | 0.25 | {code_contrib} |
| Testing | {test_score} | 0.20 | {test_contrib} |
| Security | {sec_score} | 0.15 | {sec_contrib} |
| Performance | {perf_score} | 0.10 | {perf_contrib} |
| Documentation | {doc_score} | 0.10 | {doc_contrib} |
| **TOTAL** | **{total_score}** | **1.00** | **{total_contrib}** |

**Grade**: {grade}
**Score**: {total_score}/100
**Formula**: (Architecture×0.20 + Code Quality×0.25 + Testing×0.20 + Security×0.15 + Performance×0.10 + Documentation×0.10) × 100
**Versão da Regra**: engineering-score.md v1.0.0

### Arquivos Modificados

| Arquivo | Tipo |
|---------|------|
| `{file_path}` | {file_icon} |

### Análise

- **Tipo de Mudança**: {change_type} — {change_category}
- **Testes**: {test_summary}
- **Documentação**: {doc_summary}
- **Segurança**: {sec_summary}

### Trust Registry (F7.2)

```yaml
agent: {agent_name}
task_type: {task_type}
task_id: {task_id}
outcome: {outcome}
engineering_score: {total_score}
confidence_delta: {confidence_delta}
commit_hash: {commit_hash}
evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/{commit_hash}.md
```

### Auto-Evolution

- **Stages 7-8**: Pendente (executado pelo workflow memorize-commit manualmente)
- **Learnings**: A extrair pelo workflow memorize-commit

---

## Fields Reference

| Field | Description | Source |
|-------|-------------|--------|
| `{commit_hash}` | Short SHA (7 chars) | `git rev-parse --short HEAD` |
| `{commit_hash_full}` | Full SHA | `git rev-parse HEAD` |
| `{timestamp}` | ISO 8601 UTC | `date -u +"%Y-%m-%dT%H:%M:%SZ"` |
| `{commit_date}` | Author date | `git log -1 --format="%ad" --date=iso` |
| `{author}` | Commit author name | `git log -1 --format="%an"` |
| `{email}` | Commit author email | `git log -1 --format="%ae"` |
| `{message}` | Commit subject | `git log -1 --format="%s"` |
| `{type}` | Change type (feature, fix, etc.) | Classified from commit prefix |
| `{emoji}` | Type emoji | Mapping table in memorize-commit.md |
| `{category}` | Category (Qualidade, Capacidade, etc.) | Mapping table |
| `{files_changed}` | Number of files | `git diff --stat` |
| `{insertions}` | Insertions count | `git diff --stat` |
| `{deletions}` | Deletions count | `git diff --stat` |
| `{net_loc}` | Net LOC delta | insertions - deletions |
| `{coverage}` | Coverage delta or N/A | `go test -cover` (optional) |
| `{estimated_time}` | Estimated time | Formula: insertions×0.5 + files×2 |
| `{estimated_cost}` | Estimated cost USD | Formula: (insertions×0.02 + files×0.5)/1000 |
| `{agent_name}` | Detected agent | Mapped from git author |
| `{total_score}` | Engineering Score (0-100) | F7.3 formula |
| `{grade}` | Grade (🔥 🟢 🟡 🔴) | Based on score thresholds |
| `{confidence_delta}` | Trust delta | Based on engineering score |
| `{task_id}` | Unique task identifier | `commit-auto-{date}-{hash}` |
| `{outcome}` | success / partial / failure | Based on engineering score |

## Related

- [memorize-commit workflow](../../../workflows/memorize-commit.md)
- [Engineering Score (F7.3)](../../../analytics/engineering-score.md)
- [Trust Registry (F7.2)](../../trust/TRUST_REGISTRY.md)
- [Engineering Timeline](../ENGINEERING_TIMELINE.md)
- [Post-Commit Hook](../../../scripts/hooks/post-commit)
