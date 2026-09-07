# CONVENCOES — Contrato Padrao de Skills

> **Versao**: 1.0.0 | **Status**: active | **Dono**: Skills Engine

## PROPOSITO
Todo arquivo de skill do Cosca (SKILL.md, workflow, template, engine, department) deve seguir um contrato consistente. Este documento define o formato canonico.

---

## Convencoes de Nomenclatura de Arquivos

| Tipo de Arquivo | Padrao | Exemplo |
|-----------------|--------|---------|
| Department Skill | `SKILL.md` dentro do diretorio do department | `departments/backend/SKILL.md` |
| Engine Skill | `SKILL.md` dentro do diretorio do engine | `engines/wizard/SKILL.md` |
| Workflow | `workflow-name.md` dentro de workflows/ | `workflows/feature-development.md` |
| Template | `TEMPLATE.md` dentro do diretorio do template | `templates/erp/TEMPLATE.md` |
| Governanca | `UPPER_SNAKE.md` na raiz de cosca/ | `GOVERNANCE.md` |
| Indice de Memoria | `INDEX.md` dentro do armazenamento de memoria | `memory/project/INDEX.md` |

---

## Secoes Obrigatorias — Department Skills

Todo `departments/*/SKILL.md` deve conter:

```markdown
# NOME DO DEPARTAMENTO — Descricao Curta

## METADADOS
- **Versao**: X.Y.Z
- **Status**: draft | active | deprecated
- **Dono**: Nome do departamento
- **Reporta Para**: Departamento pai

## PROPOSITO
Um paragrafo descrevendo por que este departamento existe.

## ESCOPO
O que este departamento e responsavel.

## FORA DO ESCOPO
O que este departamento explicitamente NAO e responsavel.

## RESPONSABILIDADES
1. Responsabilidade 1
2. Responsabilidade 2

## DELEGACAO
- Tipo de tarefa - Departamento alvo

## ESPECIALISTAS
| Especialista | Cargo |
|--------------|-------|

## DEPENDENCIAS
| Depende De | Por que |
|------------|---------|

## ENTRADAS
| Entrada | De | Formato |
|---------|-----|---------|

## SAIDAS
| Saida | Para | Formato |
|-------|------|---------|

## RESTRICOES
- Restricao 1

## CRITERIOS DE QUALIDADE
- [ ] Criterio 1

## ESCALONAMENTO
| Problema | Escalar Para |

## ACOES PROIBIDAS
- Acao 1

## RELACIONADOS
- [Arquivo relacionado](../path)
```

---

## Secoes Obrigatorias — Engine Skills

Todo `engines/*/SKILL.md` deve conter:

```markdown
# NOME DO ENGINE

## METADADOS
- **Versao**: X.Y.Z
- **Status**: draft | active | deprecated
- **Dono**: Nome do engine

## PROPOSITO
Um paragrafo descrevendo por que este engine existe.

## ATIVACAO
Quando este e ativado.

## ESCOPO
O que este engine cobre.

## FORA DO ESCOPO
O que este engine NAO cobre.

## PROCESSO
Descricao passo a passo de como o engine funciona.

## ENTRADAS
| Entrada | De | Formato |
|---------|-----|---------|

## SAIDAS
| Saida | Para | Formato |
|-------|------|---------|

## DEPENDENCIAS
| Engine | Por que |
|--------|---------|

## RESTRICOES
- Restricao 1

## CRITERIOS DE QUALIDADE
- [ ] Criterio 1

## RELACIONADOS
- [Arquivo relacionado](../path)
```

---

## Secoes Obrigatorias — Workflows

Todo `workflows/*.md` deve conter:

```markdown
# WORKFLOW: nome

## METADADOS
- **Versao**: X.Y.Z
- **Categoria**: init | feature | bug | refactor | review | deploy | audit
- **Duracao Estimada**: intervalo
- **Status**: draft | active | deprecated

## OBJETIVO
Um paragrafo.

## ENTRADAS
| Nome | Tipo | Obrigatorio | Descricao |

## SAIDAS
| Nome | Tipo | Descricao |

## PRECONDICOES
1. Condicao

## POSCONDICOES
1. Condicao

## DEPENDENCIAS
| Workflow | Motivo |

## ETAPAS
### Etapa N: Nome
- **Chefe**: Departamento
- **Especialistas**: Cargo(s)
- **Tarefa**: Descricao
- **Saida**: Resultado esperado

## VALIDACAO
1. Verificacao

## CRITERIOS DE SUCESSO
- [ ] Criterio

## TRATAMENTO DE ERROS
| Falha | Acao |

## RELACIONADOS
- [Arquivo relacionado](../path)
```

---

## Secoes Obrigatorias — Templates

Todo `templates/*/TEMPLATE.md` deve conter:

```markdown
# NOME DO TEMPLATE

## METADADOS
- **Versao**: X.Y.Z
- **Status**: draft | active | deprecated

## DOMINIO
Que tipo de aplicacao este template serve.

## STACK RECOMENDADA
| Camada | Tecnologia |

## ESTRUTURA DE MODULOS
(Arvore de diretorios)

## CARACTERISTICAS PRINCIPAIS
- Caracteristica

## NOTAS DE ARQUITETURA
- Nota

## RELACIONADOS
- [Template relacionado](../path)
```

---

## Requisitos de Metadados

Todo arquivo deve comecar com um bloco de metadados:

```markdown
> **Versao**: X.Y.Z | **Status**: draft | active | deprecated | **Dono**: nome | **Ultima Atualizacao**: YYYY-MM-DD
```

| Campo | Obrigatorio | Descricao |
|-------|-------------|-----------|
| Versao | Sim | Versao semantica (X.Y.Z) |
| Status | Sim | draft, active ou deprecated |
| Dono | Sim | Nome do departamento ou engine |
| Ultima Atualizacao | Sim | Data ISO da ultima modificacao |

---

## Convencoes de Nomenclatura

| Elemento | Convencao | Exemplo |
|----------|-----------|---------|
| Departments | lowercase, hifenizado | `backend`, `uiux`, `qa` |
| Engines | lowercase, palavra unica | `wizard`, `context`, `planning` |
| Workflows | lowercase, hifenizado | `feature-development`, `bug-fix` |
| Templates | lowercase, palavra unica | `erp`, `saas`, `mobile` |
| Documentos de governanca | UPPER_SNAKE_CASE | `GOVERNANCE.md` |
| Cabecalhos de secao | UPPERCASE | `## PROPOSITO` |

---

## Regras de Formatacao

1. Todos os arquivos usam GitHub-flavored Markdown
2. Cabecalhos usam `##` para secoes de nivel superior, `###` para subsecoes
3. Listas usam `-` para nao ordenadas, `1.` para ordenadas
4. Tabelas usam sintaxe padrao de tabelas Markdown
5. Blocos de codigo especificam linguagem: ` ```yaml `
6. Caminhos de arquivo em referencias sao relativos a raiz de `cosca/`
7. Referencias cruzadas usam `[Nome Exibido](../caminho/para/arquivo.md)`
8. Uma linha em branco entre secoes
9. Comprimento maximo da linha: 120 caracteres (para legibilidade)
10. Sem espacos em branco no final
11. **Todo conteudo do arquivo e escrito em ingles** — codigo, docs, skills, engines, workflows, registros de memoria. A unica excecao e a conversacao Kernel-Don, que sempre e conduzida em portugues brasileiro (PT-BR). Conteudo historico existente em portugues e preservado como esta (registros de sessoes passadas); apenas conteudo novo/editado deve seguir esta regra.

---

## Checklist de Qualidade

Antes de considerar uma skill completa, verificar:

- [ ] Todas as secoes obrigatorias presentes
- [ ] Bloco de metadados no topo
- [ ] Versao e semantica (X.Y.Z)
- [ ] Status e um de: draft, active, deprecated
- [ ] Sem conteudo duplicado com outros arquivos
- [ ] Referencias cruzadas usam caminhos relativos
- [ ] Tabelas formatadas corretamente
- [ ] Sem links internos quebrados
- [ ] Idioma consistente com outras skills
- [ ] Exemplos sao concretos, nao abstratos
- [ ] Restricoes sao explicitas, nao implicitas
- [ ] Acoes proibidas claramente declaradas (apenas departments)

---

> **Executado por**: Skills Engine | **Auditado por**: Evolution Engine | **Ultima revisao**: 2026-07-10
