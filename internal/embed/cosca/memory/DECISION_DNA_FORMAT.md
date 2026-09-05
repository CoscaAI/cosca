# DECISION DNA FORMAT — Formato Canônico de Decisões com DNA Completo

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Criado**: 2026-07-30
> **CMI Dimensão**: Decision Quality (Julgamento) | **Bloco Cognitivo**: Bloco 2 — Decisão & Raciocínio
> **Dependência**: LEARNING_PROTOCOL.md v2.0.0+ | **Fase CMI**: Fase 1 — Foundation

---

## Propósito

Toda decisão não-trivial do Cosca Runtime — arquitetura, design, routing, segurança, tooling, governança — deixa um registro estruturado e queryable. O **Decision DNA** captura o ciclo completo da decisão:

```
Decisão → Evidências → Riscos → Alternativas → Resultado
```

Seis meses depois, qualquer agente ou o Don pode perguntar:

> *"Por que decidimos X em Janeiro de 2026?"*
> *"Quais decisões dos últimos 6 meses foram revertidas?"*
> *"Quais decisões tinham confidence < 0.70 e mesmo assim foram tomadas?"*

E o sistema responde com o registro completo: evidências da época, riscos previstos, alternativas descartadas, o que realmente aconteceu e o que foi aprendido.

---

## Princípios

| # | Princípio | Descrição |
|---|-----------|-----------|
| **P1** | **Rastreabilidade total** | Toda decisão não-trivial (confidence < 0.95 ou impacto cross-domain) DEVE ter um registro DNA |
| **P2** | **Imutabilidade histórica** | O registro original NUNCA é alterado — o campo `Validação` pode ser atualizado posteriormente, mas o registro da decisão permanece como foi feito na época |
| **P3** | **Honestidade radical** | Riscos e evidências contrárias DEVEM ser registrados com o mesmo rigor das evidências favoráveis |
| **P4** | **Queryabilidade** | Todo campo de DNA é indexável por tag, domínio, data, agente, outcome, confidence — via grep, FTS5 ou vector search |
| **P5** | **Compatibilidade** | O formato DNA é compatível com o Learning Entry Format existente em learnings.md, podendo coexistir no mesmo arquivo com o prefixo `## DNA` |

---

## Formato Canônico

### Estrutura de Arquivo

Decision DNA entries podem ser armazenadas de duas formas:

1. **Embedded em learnings.md** (recomendado para agentes) — prefixadas com `## DNA`
2. **Arquivo separado** em `internal/embed/cosca/memory/decisions/` — para decisões inter-agentes ou de domínio cross-cutting

A forma embedded é recomendada para a maioria dos casos. Use arquivo separado quando a decisão envolver 3+ agentes ou tiver impacto em múltiplos domínios.

### Template

```markdown
## DNA — {id}: {decisão-resumo}

> **DNA ID**: DDNA-{YYYY-MM-DD}-{NNN}
> **Status**: {active | revisado | revertido | obsoleto}
> **Versão do registro**: 1.0.0

### Decisão
{O que foi decidido — uma frase, concisa e assertiva. Sem ambiguidade.}

### Contexto
{Por que esta decisão era necessária. O problema que estava sendo resolvido. O estado do sistema ANTES da decisão. Incluir constraints relevantes (prazo, recursos, dependências).}

### Metadados

| Campo | Valor |
|-------|-------|
| **Data da decisão** | YYYY-MM-DD |
| **Agente decisor** | cosca-{nome} |
| **Domínio** | {security, architecture, testing, devops, performance, etc.} |
| **Confiança na decisão** | 0.XX |
| **Nível da decisão** | {1=tático-operacional, 2=design local, 3=arquitetura, 4=estratégico, 5=fundacional} |
| **CMI impact** | {Aprendizado: ±X, Julgamento: ±X, Planejamento: ±X, Autocrítica: ±X, Transferência: ±X, Consistência: ±X} |

### Evidências

#### A favor
| # | Evidência | Fonte | Peso (1-5) |
|---|-----------|-------|------------|
| 1 | {Descrição da evidência} | {código, benchmark, doc, auditoria, experimento} | {1-5} |
| 2 | {Descrição da evidência} | {fonte} | {1-5} |

#### Contra
| # | Evidência | Fonte | Peso (1-5) |
|---|-----------|-------|------------|
| 1 | {Descrição da evidência contrária} | {fonte} | {1-5} |

### Riscos

| # | Risco | Probabilidade | Impacto | Severidade (P×I) | Mitigação |
|---|-------|---------------|---------|--------------------|-----------|
| 1 | {O que pode dar errado} | {0-1.0} | {0-1.0} | {0-1.0} | {Como o risco está sendo mitigado} |
| 2 | {Risco 2} | {P} | {I} | {S} | {Mitigação} |

### Alternativas Consideradas

| # | Alternativa | Prós | Contras | Por que foi rejeitada |
|---|-------------|------|---------|-----------------------|
| 1 | {Alternativa A} | {Vantagens} | {Desvantagens} | {Razão específica da rejeição} |
| 2 | {Alternativa B (se houver)} | {Vantagens} | {Desvantagens} | {Razão específica} |

### Gatilhos de Reconsideração
- [ ] Se {condição X} acontecer, reavaliar esta decisão
- [ ] Se a métrica {Y} cruzar o threshold {Z}, reavaliar
- [ ] Revisão programada: {data ou condição}

### Resultado

| Campo | Valor |
|-------|-------|
| **Resultado observado** | {success / partial / failure / mixed} |
| **Data da validação** | YYYY-MM-DD (pode ser atualizada posteriormente) |
| **Validador** | {agente ou Don} |
| **Evidência do resultado** | {O que prova que deu certo, errado ou misto} |

### Lições Aprendidas
- {Lição 1: o que aprendemos com esta decisão e seu resultado}
- {Lição 2}
- {Lição 3: o que faríamos diferente se pudéssemos refazer}

### Tags
`#dna` `#{domain}` `#{specific-tag-1}` `#{specific-tag-2}`

### Relacionado
- **Learnings**: [{agent}/learnings.md#L{num}](agent/{agent}/learnings.md)
- **Patterns**: [{agent}/patterns.md](agent/{agent}/patterns.md)
- **Failures**: [{agent}/failures.md](agent/{agent}/failures.md) (se houve falha relacionada)
- **ADRs**: [decisions/cosca-cli/adr-{num}.md](decisions/cosca-cli/adr-{num}.md)
- **Heurísticas**: [knowledge/heuristics/H-{num}.yaml](../knowledge/heuristics/H-{num}.yaml)
```

---

## Especificação dos Campos

### Campos Obrigatórios

| Campo | Tipo | Descrição | Regra de Validação |
|-------|------|-----------|---------------------|
| `DNA ID` | string | Identificador único no formato `DDNA-YYYY-MM-DD-NNN` | NNN sequencial por data; IDs não podem ser reutilizados mesmo após reversão |
| `Status` | enum | `active`, `revisado`, `revertido`, `obsoleto` | Uma decisão revertida MANTÉM seu registro — apenas o status muda |
| `Decisão` | string | Uma frase assertiva do que foi decidido | Máximo 280 caracteres; sem ambiguidade; sem "talvez" ou "provavelmente" |
| `Contexto` | string | O problema que levou à decisão | Deve responder "por que esta decisão era necessária AGORA?" |
| `Data da decisão` | date | YYYY-MM-DD | Data real em que a decisão foi tomada, não a data do registro |
| `Agente decisor` | string | Nome do agente/chief que tomou a decisão | Deve corresponder a um agente existente no diretório `agent/` |
| `Domínio` | string | Domínio primário da decisão | Usar domínios do CMI: security, architecture, testing, devops, performance, etc. |
| `Confiança na decisão` | float | 0.00 a 1.00 | Confiança do agente NO MOMENTO da decisão (não revisado depois) |
| `Nível da decisão` | int | 1 a 5 | 1=tático, 2=design local, 3=arquitetura, 4=estratégico, 5=fundacional |
| `Evidências a favor` | table | Mínimo 1 evidência com fonte e peso | Toda evidência DEVE ter fonte verificável |
| `Evidências contra` | table | Mínimo 1 evidência (pode ser "nenhuma encontrada" com peso 0) | Honestidade radical — se não encontrou evidência contra, registre isso |
| `Riscos` | table | Mínimo 1 risco com P, I, S e mitigação | Probabilidade × Impacto = Severidade |
| `Alternativas consideradas` | table | Mínimo 1 alternativa (pode ser "manter status quo") | Toda alternativa rejeitada precisa de razão documentada |
| `Resultado` | table | Outcome, data de validação, validador, evidência | Pode ser preenchido como "pendente" e atualizado depois |
| `Tags` | list | Mínimo 3 tags incluindo `#dna` | Tags sem `#` são aceitas mas não recomendadas |

### Campos Opcionais (Altamente Recomendados)

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `Gatilhos de reconsideração` | checklist | Condições objetivas que devem disparar reavaliação |
| `CMI impact` | table | Impacto estimado nas 6 dimensões do CMI (±X cada) |
| `Lições aprendidas` | list | O que foi aprendido (preenchido após validação do resultado) |
| `Relacionado` | links | Referências cruzadas para learnings, patterns, failures, ADRs, heurísticas |
| `Versão do registro` | semver | Versão do template usado (para migrações futuras de formato) |

---

## Queryabilidade

### Por grep (imediato)

```bash
# Todas as decisões do domínio security
rg "^## DNA.*security" internal/embed/cosca/memory/

# Todas as decisões revertidas
rg "Status.*revertido" internal/embed/cosca/memory/

# Decisões com confidence < 0.70
rg "Confiança na decisão.*0\.[0-6]" internal/embed/cosca/memory/

# Decisões de um agente específico
rg "Agente decisor.*cosca-security" internal/embed/cosca/memory/

# Decisões por data
rg "DDNA-2026-07-" internal/embed/cosca/memory/

# Decisões com resultado "failure"
rg "Resultado observado.*failure" internal/embed/cosca/memory/
```

### Por FTS5 (SQLite — Fase 2)

```sql
-- Decisões sobre memfd_create
SELECT * FROM decision_dna WHERE decision MATCH 'memfd_create';

-- Decisões de alto risco com resultado negativo
SELECT * FROM decision_dna WHERE max_severity > 0.7 AND outcome = 'failure';

-- Confiança média por domínio
SELECT domain, AVG(confidence) FROM decision_dna GROUP BY domain;
```

### Por Vector Search (Fase 3)

```python
# "Quais decisões passadas são relevantes para o problema atual?"
results = semantic_search.decisions_similar_to(current_context, k=5)
```

---

## Integração com o Learning Protocol

Decisões DNA podem coexistir com learnings no mesmo arquivo `learnings.md`. A distinção é feita pelo prefixo:

- `### {timestamp} — {technique-name}` → Learning Entry (formato existente)
- `## DNA — {id}: {decisão-resumo}` → Decision DNA Entry (formato novo)

Isso permite que um agente tenha tanto seus learnings operacionais (técnicas, descobertas, padrões) quanto suas decisões estratégicas (arquitetura, design, routing) no mesmo arquivo, mantendo queryabilidade independente.

### Quando usar DNA vs Learning

| Critério | Learning Entry | Decision DNA |
|----------|---------------|--------------|
| **Natureza** | Descoberta, técnica, habilidade | Escolha entre alternativas |
| **Pergunta respondida** | "O que aprendi?" | "Por que escolhi X em vez de Y?" |
| **Reversível?** | Não (aprendizado é cumulativo) | Sim (decisões podem ser revertidas) |
| **Estrutura** | Leve (10 campos) | Completa (25+ campos) |
| **Prefix** | `### {date} — {name}` | `## DNA — {id}: {summary}` |

### Regra de Ouro

> Se a tarefa envolveu ESCOLHER entre duas ou mais alternativas com impacto cross-domain, é Decision DNA.
> Se a tarefa envolveu DESCOBRIR ou APLICAR uma técnica, é Learning Entry.

---

## Exemplo Preenchido

Ver [DECISION_DNA_EXAMPLE.md](DECISION_DNA_EXAMPLE.md) para um exemplo completo usando a decisão real do auto-jail com memfd_create.

---

## Ciclo de Vida de uma Decisão DNA

```
┌──────────────────────────────────────────────────────────────────┐
│                    DECISION DNA LIFECYCLE                         │
│                                                                   │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐ │
│  │ PROMPT   │     │ DECISÃO  │     │ REGISTRO │     │VALIDAÇÃO │ │
│  │ Gatilho  │────▶│ Tomada   │────▶│ DNA      │────▶│ Resultado│ │
│  │          │     │ pelo     │     │ criado   │     │observado │ │
│  │ Problema │     │ agente   │     │          │     │          │ │
│  │ surge    │     │          │     │          │     │          │ │
│  └──────────┘     └──────────┘     └──────────┘     └─────┬────┘ │
│                                                           │      │
│                           ┌───────────────────────────────┘      │
│                           ▼                                       │
│                    ┌──────────────┐                               │
│                    │  RESULTADO   │                               │
│                    │              │                               │
│            ┌───────┤   success?   ├────────┐                      │
│            │       │              │        │                      │
│            ▼       └──────────────┘        ▼                      │
│   ┌────────────┐                  ┌────────────┐                  │
│   │ CONFIRMA   │                  │  REAVALIA  │                  │
│   │ Status:    │                  │ Status:    │                  │
│   │ revisado   │                  │ revertido  │                  │
│   │ Lições:    │                  │ ou revisado│                  │
│   │ positivas  │                  │ Lições:    │                  │
│   │            │                  │ corretivas │                  │
│   └────────────┘                  └────────────┘                  │
│                                                                   │
│   NOTA: Em qualquer caso, o registro ORIGINAL é preservado.       │
│   Apenas o status e os campos de resultado/lições são atualizados.│
└──────────────────────────────────────────────────────────────────┘
```

---

## Governança

### Quando registrar uma decisão DNA

Uma decisão DEVE ser registrada como DNA quando atende a PELO MENOS UM destes critérios:

1. **Confiança < 0.95**: Há incerteza real na decisão
2. **Impacto cross-domain**: Afeta 2+ domínios do CMI
3. **Irreversível ou custosa de reverter**: Mudar depois é caro
4. **Nível ≥ 3**: Decisão de arquitetura, estratégica ou fundacional
5. **Envolve trade-off explícito**: Duas ou mais alternativas razoáveis competindo
6. **O Don pediu**: Se o Don perguntar "por que fizemos isso?", a resposta deve estar no DNA

### Quando NÃO registrar

- Decisões puramente operacionais (nível 1) com confidence ≥ 0.95
- Escolhas triviais sem alternativas reais (ex: "usar `const` em vez de `let`")
- Aplicação direta de um padrão já consolidado sem variação

### Revisão e atualização

- **Registro original**: Imutável após criação
- **Campo `Resultado`**: Atualizável a qualquer momento (com timestamp)
- **Campo `Lições`**: Atualizável a qualquer momento (com timestamp)
- **Campo `Status`**: Pode mudar de `active` → `revisado` → `obsoleto`; ou `active` → `revertido`
- **Gatilhos de reconsideração**: Podem ser adicionados, nunca removidos

### Auditoria

Toda alteração em campos atualizáveis deve registrar:
- Data da atualização
- Agente que atualizou
- Evidência que motivou a atualização

---

## Exemplos de Query

### "Por que decidimos usar memfd_create para o auto-jail?"

```bash
rg -l "memfd_create" internal/embed/cosca/memory/agent/*/learnings.md
# → cosca-kernel/learnings.md (L15)
# → DECISION_DNA_EXAMPLE.md (DDNA-2026-07-29-001)
```

### "Quais decisões do cosca-security foram revertidas?"

```bash
rg "Agente decisor.*cosca-security" -A 50 internal/embed/cosca/memory/ | rg "Status.*revertido"
```

### "Quais decisões de Julho/2026 tinham confidence baixa?"

```bash
rg "Confiança na decisão.*0\.[0-6]" internal/embed/cosca/memory/agent/*/learnings.md
```

### "Que lições aprendemos com decisões de architecture?"

```bash
rg "^## DNA" -A 100 internal/embed/cosca/memory/ | rg -B 2 "Domínio.*architecture" | rg "Lições Aprendidas" -A 5
```

---

## Relacionado

| Documento | Relação |
|-----------|---------|
| [LEARNING_PROTOCOL.md](LEARNING_PROTOCOL.md) | Protocolo de aprendizado — Decision DNA é uma extensão especializada |
| [../architecture/COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) | Arquitetura CMI — Decision DNA é o conceito C4 |
| [../workflows/cognitive-maturity-implementation.md](../workflows/cognitive-maturity-implementation.md) | Workflow de implementação — F1.1 |
| [MEMORY_MODEL.md](MEMORY_MODEL.md) | Modelo de memória — onde decisões DNA são armazenadas |
| [../CONVENTIONS.md](../CONVENTIONS.md) | Convenções do framework |
| [DECISION_DNA_EXAMPLE.md](DECISION_DNA_EXAMPLE.md) | Exemplo preenchido com decisão real |

---

> **"Decisões sem DNA são esquecidas. Decisões com DNA são ativos de conhecimento que se acumulam com o tempo."**
> — Cosca Kernel, 2026-07-30
