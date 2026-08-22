# GOVERNANCIA — Versionamento, Ciclo de Vida e Descontinuacao

> **Versao**: 1.0.0 | **Status**: active | **Dono**: Cosca Kernel

## PROPOSITO
Definir como skills, engines, workflows e templates do Cosca sao versionados, mantidos e descontinuados. Esta e a fonte unica de verdade para politicas de governanca.

---

## Politica de Versionamento

### Versionamento Semantico (SemVer)

Todos os arquivos do Cosca seguem o formato **X.Y.Z**:

| Parte | Atualizar Quando |
|-------|------------------|
| **X** (Major) | Mudancas quebra-contrato na skill, entradas ou saidas |
| **Y** (Minor) | Nova secao, nova responsabilidade, novo especialista adicionado |
| **Z** (Patch) | Clarificacoes, correcoes de erros, melhorias de formatacao |

### Historico de Versoes

Todo arquivo rastreia mudancas em sua secao `HISTORY`:

```markdown
## HISTORICO

| Versao | Data | Autor | Mudancas |
|--------|------|-------|----------|
| 1.0.0 | YYYY-MM-DD | nome | Lancamento inicial |
| 1.1.0 | YYYY-MM-DD | nome | Adicionada secao X |
| 1.0.1 | YYYY-MM-DD | nome | Corrigido erro em Y |
```

---

## Estados do Ciclo de Vida

```
  ┌─────────┐
  │  DRAFT  │  → Em desenvolvimento ativo, pode mudar frequentemente
  └────┬────┘
       │ Aprovado
       ▼
  ┌─────────┐
  │  ACTIVE  │  → Pronto para producao, usado por agentes
  └────┬────┘
       │
       ├──→ Nao e mais necessario
       │
       ▼
  ┌────────────┐
  │ DEPRECATED  │  → Ainda disponivel, aviso de migracao emitido
  └────┬───────┘
       │ Apos periodo de migracao
       ▼
  ┌──────────┐
  │ RETIRED   │  → Removido do registro ativo
  └──────────┘
```

| Estado | Significado | Comportamento do Agente |
|--------|------------|------------------------|
| **draft** | Em andamento | Nao carregar em contexto de producao |
| **active** | Pronto para producao | Carregamento e execucao normais |
| **deprecated** | Agendado para remocao | Carregar com aviso de descontinuacao |
| **retired** | Arquivado | Nunca carregar, arquivo movido para `archive/` |

---

## Politica de Descontinuacao

### Quando Descontinuar
- Skill e substituida por outra skill
- Department e fundido ou reestruturado
- Workflow e substituido por nova versao
- Template nao e mais relevante

### Processo de Descontinuacao
1. Marcar status como `deprecated` nos metadados
2. Adicionar aviso de descontinuacao no topo do arquivo
3. Especificar substituto (se houver)
4. Definir data de desligamento (minimo 30 dias)
5. Atualizar COSCA_INDEX.md
6. Anunciar no CHANGELOG.md

### Formato do Aviso de Descontinuacao
```markdown
> Aviso: **DESCONTINUADO** desde YYYY-MM-DD. Sera desligado apos YYYY-MM-DD.
> Caminho de migracao: Use [arquivo-substituto.md] em vez deste.
```

---

## Politica de Compatibilidade

### Compatibilidade Retroativa
- **Versoes de patch** (0.0.X): Sempre retroativamente compativel
- **Versoes minor** (0.X.0): Retroativamente compativel para entradas/saidas; novas secoes permitidas
- **Versoes major** (X.0.0): Podem quebrar compatibilidade; requer guia de migracao

### Mudancas Quebra-Contrato
Uma mudanca e quebra-contrato se:
1. Remove uma secao obrigatoria
2. Muda formato de entrada
3. Muda formato de saida
4. Renomeia o arquivo
5. Remove um tipo de especialista
6. Muda o proposito da skill

Mudancas quebra-contrato requerem:
- Atualizacao de versao major
- Guia de migracao
- Periodo de descontinuacao de 30 dias para a versao antiga
- Notificacao para todas as skills dependentes

---

## Propriedade

### Tipos de Dono
| Tipo | Descricao |
|------|-----------|
| **Department** | Chefe do departamento e dono do SKILL.md do department |
| **Engine** | Engine e dono de seu SKILL.md |
| **Cosca Kernel** | Dono de docs de governanca, INDEX, CHANGELOG |
| **Workflow Chief** | Dono de definicoes de workflow |

### Responsabilidades do Dono
1. Revisar skill anualmente por relevancia
2. Atualizar versao em cada mudanca
3. Manter secao HISTORICO
4. Responder a solicitacoes de descontinuacao
5. Corrigir problemas de qualidade encontrados pelo Evolution Engine

---

## Aprovacao de Mudancas

| Tipo de Mudanca | Aprovacao Necessaria |
|-----------------|---------------------|
| Patch (erro, formatacao) | Apenas dono |
| Minor (nova secao, novo especialista) | Dono + revisor |
| Major (mudancas quebra-contrato) | Dono + CTO + departments afetados |
| Nova skill/arquivo | Aprovacao CTO + conformidade CONVENTIONS |
| Descontinuacao | Dono + CTO |
| Desligamento | Dono + CTO + aviso de 30 dias |

---

## Auditoria e Conformidade

O Evolution Engine periodicamente audita:
- [ ] Todos os arquivos tem bloco de metadados
- [ ] Todos os arquivos tem numero de versao valido
- [ ] Nenhum arquivo descontinuado apos data de desligamento
- [ ] Todos os arquivos ativos tem entradas HISTORICO
- [ ] Referencias cruzadas sao validas
- [ ] Sem responsabilidades duplicadas
- [ ] Pontuacao de conformidade CONVENTIONS.md > 80%

---

## Diretorio de Registros

| Registro | Localizacao |
|----------|-------------|
| Versoes de skills | Secao HISTORICO de cada arquivo |
| Mudancas quebra-contrato | Registro de migracao deste arquivo |
| Agenda de descontinuacao | Coluna de status do COSCA_INDEX.md |
| Registro de propriedades | Coluna Owner do COSCA_INDEX.md |
| Relatorios de conformidade | Gerados pelo Evolution Engine |

---

## RELACIONADOS

- [CONSTITUTION.md](CONSTITUTION.md) — Documento de autoridade suprema

---

## HISTORICO

| Versao | Data | Autor | Mudancas |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Framework inicial de governanca |

---

> **Executado por**: Skills Engine + Evolution Engine | **Ultima revisao**: 2026-07-10
