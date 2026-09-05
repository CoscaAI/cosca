# Knowledge Pipeline — Workflow de Construção

> **Workflow**: `cosca-knowledge-pipeline`
> **Versão**: 1.0.0 | **Status**: active
> **Owner**: cosca-kernel (coordenação), cosca-memory-chief (execução)
> **Triggers**: Don's order, periodic (semanal), post-evolution session
> **Dependências**: Nenhuma — este workflow CONSTRÓI as dependências

---

## Objetivo

Transformar o pipeline de conhecimento do Cosca de "especificação pesada, implementação leve" para **operacional e automatizado**, priorizando o que entrega valor imediato.

---

## Fases

### Fase 1 — Fundação (HOJE)
> **Meta**: Conteúdo real nos diretórios vazios + validação básica

| # | Tarefa | Agente | Artefato | Status |
|---|--------|--------|----------|--------|
| 1.1 | Extrair heurísticas dos 44 agentes com learnings reais | cosca-semantic-memory | `internal/embed/cosca/knowledge/heuristics/*.yaml` (15-20) | pending |
| 1.2 | Criar 3 playbooks operacionais | cosca-memory-chief | `internal/embed/cosca/knowledge/playbooks/*.md` | pending |
| 1.3 | Popular 2-3 benchmarks de performance | cosca-performance | `internal/embed/cosca/knowledge/benchmarks/*.md` | pending |
| 1.4 | CI job de validação de memória | cosca-devops | `.github/workflows/scripts/validate-memory.sh` | pending |
| 1.5 | Schema de versionamento da KB | cosca-database | `.cosca/schema/` (migration inicial) | pending |

### Fase 2 — Sync Pipeline (esta semana)
> **Meta**: Conhecimento trafega entre runtimes

| # | Tarefa | Agente | Artefato | Status |
|---|--------|--------|----------|--------|
| 2.1 | Implementar Source stage (file watcher + hash) | cosca-runtime | `internal/runtime/sync_source.go` | pending |
| 2.2 | Implementar Parse stage (YAML + markdown parser) | cosca-runtime | `internal/runtime/sync_parse.go` | pending |
| 2.3 | Implementar Validate stage (schema validation) | cosca-runtime | `internal/runtime/sync_validate.go` | pending |
| 2.4 | Integrar com hot reload (fsnotify) | cosca-runtime | Modificação em `runtime.go` | pending |

### Fase 3 — Automação (próxima semana)
> **Meta**: Pipeline fecha o ciclo sem intervenção humana

| # | Tarefa | Agente | Artefato | Status |
|---|--------|--------|----------|--------|
| 3.1 | Knowledge Council (governança) | cosca-governance | `internal/embed/cosca/councils/knowledge/` | pending |
| 3.2 | Auto-commit de learnings | cosca-automation | Script/hook | pending |
| 3.3 | Review automático de memória | cosca-review | Extensão do agente | pending |
| 3.4 | Extração automática (metacognition engine) | cosca-evolution | `engines/metacognition/` em Go | pending |

---

## Métricas de Sucesso

| Fase | Métrica | Alvo |
|------|---------|------|
| Fase 1 | Heurísticas populadas | 15+ YAML files |
| Fase 1 | Playbooks criados | 3+ playbooks |
| Fase 1 | CI validação ativa | 1 job no ci.yml |
| Fase 2 | Sync pipeline funcional | 3 estágios (Source, Parse, Validate) |
| Fase 2 | Hot reload integrado | < 5s entre alteração e disponibilidade |
| Fase 3 | Ciclo fechado | 0 intervenção manual para knowledge flow |

---

## Princípios

1. **Código > especificação.** Não criar mais SKILL.md vazio. Cada artefato é código funcional ou conteúdo real.
2. **Extrair do que existe.** As heurísticas vêm dos 44 agentes com aprendizado real — não são inventadas.
3. **Valor visível a cada commit.** Cada fase entrega algo que o Don pode ver funcionando.
4. **Pipeline do Don como Norte.** A visão (Experiência → Extração → KB → Sync) é o destino. As fases são o caminho.
