# MODELO DE SKILL — Guia de Criação de Novas Skills

> **Version**: 1.0.0 | **Status**: active | **Owner**: Skills Engine | **Last Updated**: 2026-07-11

## Propósito
Use este modelo ao criar qualquer nova skill de departamento, skill de engine, workflow ou modelo. Siga exatamente o contrato do [CONVENTIONS.md](CONVENTIONS.md).

Todos os caminhos neste modelo usam notação de Caminho Virtual. Substitua os placeholders pelos valores reais. Nunca use caminhos hardcoded. Veja [engines/resource-resolver/SKILL.md](engines/resource-resolver/SKILL.md).

---

## Modelo de Skill de Departamento

```markdown
# NOME DO DEPARTAMENTO — Descrição Curta

> **Version**: 1.0.0 | **Status**: draft | **Owner**: [Department] Chief | **Last Updated**: YYYY-MM-DD

## PROPÓSITO
Um parágrafo descrevendo por que este departamento existe.

## ESCOPO
O que este departamento possui e é responsável.

## FORA DO ESCOPO
O que este departamento explicitamente NÃO possui.

## RESPONSABILIDADES
1. Responsabilidade 1
2. Responsabilidade 2

## DELEGAÇÃO
- Tipo de tarefa → Departamento alvo

## ESPECIALISTAS
| Especialista | Função |
|-----------|------|
| Nome | Descrição |

## DEPENDÊNCIAS
| Depende De | Por quê |
|-----------|-----|
| Departamento/Engine | Razão |

## ENTRADAS
| Entrada | De | Formato |
|-------|------|--------|
| Nome | Fonte | Tipo |

## SAÍDAS
| Saída | Para | Formato |
|--------|-----|--------|
| Nome | Consumidor | Tipo |

## RESTRIÇÕES
- Restrição ou padrão que deve ser seguido

## CRITÉRIOS DE QUALIDADE
- [ ] Critério 1
- [ ] Critério 2

## ESCALAÇÃO
| Problema | Escalar Para |
|-------|-------------|
| Tipo de problema | Departamento alvo |

## AÇÕES PROIBIDAS
- Ação 1
- Ação 2

## RELACIONADOS
- [Arquivo relacionado](../path/to/file.md)

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Autor | Versão inicial |
```

---

## Modelo de Skill de Engine

```markdown
# NOME DA ENGINE

> **Version**: 1.0.0 | **Status**: draft | **Owner**: [Engine] Engine | **Last Updated**: YYYY-MM-DD

## PROPÓSITO
Um parágrafo descrevendo por que esta engine existe.

## ATIVAÇÃO
Quando esta engine é disparada (eventos, condições).

## ESCOPO
O que esta engine cobre.

## FORA DO ESCOPO
O que esta engine NÃO cobre.

## PROCESSO
Descrição passo a passo de como a engine funciona.

## ENTRADAS
| Entrada | De | Formato |
|-------|------|--------|
| Nome | Fonte | Tipo |

## SAÍDAS
| Saída | Para | Formato |
|--------|-----|--------|
| Nome | Consumidor | Tipo |

## DEPENDÊNCIAS
| Engine | Por quê |
|--------|-----|
| Nome | Razão |

## RESTRIÇÕES
- Restrição 1

## CRITÉRIOS DE QUALIDADE
- [ ] Critério 1

## RELACIONADOS
- [Arquivo relacionado](../path/to/file.md)

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Autor | Versão inicial |
```

---

## Modelo de Workflow

```markdown
# WORKFLOW: workflow-name

> **Version**: 1.0.0 | **Status**: draft | **Category**: [init|feature|bug|refactor|review|deploy|maintenance|security|performance] | **Last Updated**: YYYY-MM-DD

## OBJETIVO
Um parágrafo.

## ENTRADAS
| Nome | Tipo | Obrigatório | Descrição |
|------|------|----------|-------------|

## SAÍDAS
| Nome | Tipo | Descrição |
|------|------|-------------|

## PRÉ-CONDIÇÕES
1. Condição

## PÓS-CONDIÇÕES
1. Condição

## DEPENDÊNCIAS
| Workflow | Razão |
|----------|--------|

## PASSOS
### Passo 1: Nome
- **Chief**: Departamento
- **Especialistas**: Função(ões)
- **Tarefa**: Descrição
- **Saída**: Resultado esperado

## VALIDAÇÃO
1. Verificação

## CRITÉRIOS DE SUCESSO
- [ ] Critério

## TRATAMENTO DE ERROS
| Falha | Ação |
|---------|--------|

## RELACIONADOS
- [Arquivo relacionado](../path/to/file.md)

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Autor | Versão inicial |
```

---

## Modelo de Scaffold de Template

```markdown
# TEMPLATE NAME

> **Version**: 1.0.0 | **Status**: draft | **Last Updated**: YYYY-MM-DD

## DOMÍNIO
Que tipo de aplicação este template serve.

## STACK RECOMENDADA
| Camada | Tecnologia |
|-------|-----------|

## ESTRUTURA DO MÓDULO
```
project/
├── src/
├── tests/
└── README.md
```

## PRINCIPAIS FUNCIONALIDADES
- Funcionalidade 1

## NOTAS DE ARQUITETURA
- Nota 1

## RELACIONADOS
- [Template relacionado](../template-name/TEMPLATE.md)

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Autor | Versão inicial |
```

---

## Checklist Antes de Enviar

- [ ] Bloco de metadados presente com versão, status, proprietário, data
- [ ] Todas as seções obrigatórias presentes (conforme CONVENTIONS.md)
- [ ] Sem conteúdo duplicado com arquivos existentes
- [ ] Referências cruzadas usam caminhos relativos
- [ ] Tabelas formatadas corretamente
- [ ] Seção HISTÓRICO preenchida
- [ ] Adicionado ao COSCA_INDEX.md
- [ ] Adicionado aos caminhos de skills no opencode.jsonc (se departamento/engine)
- [ ] Configuração do agente criada no opencode.jsonc (se departamento com agentes)

---

> **Aplicado por**: Skills Engine | **Última revisão**: 2026-07-10
