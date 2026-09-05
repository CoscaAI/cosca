# DECISION DNA — Formato Canônico de Decisão Técnica (CMI F1.1)

> **Version**: 1.1.0 | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-30
> **CMI Dimension**: Decision Quality (Julgamento) | **Bloco Cognitivo**: Bloco 2 — Decisão & Raciocínio
> **Fase CMI**: Fase 1 — Foundation | **Código**: F1.1
>
> Consulte também:
> - [DECISION_DNA_FORMAT.md](../../memory/DECISION_DNA_FORMAT.md) — versão agent-facing (protocolo de aprendizado)
> - [DECISION_DNA_EXAMPLE.md](../../memory/DECISION_DNA_EXAMPLE.md) — exemplo do auto-jail com memfd_create
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — protocolo de aprendizado onde DDNA está integrado

---

## Sumário

1. [Propósito](#propósito)
2. [Formato Padronizado (YAML Frontmatter)](#formato-padronizado-yaml-frontmatter)
3. [Especificação dos Campos](#especificação-dos-campos)
4. [Exemplo 1: Remoção dos 10 chat.go (Dead Code)](#exemplo-1-remoção-dos-10-chatgo-dead-code)
5. [Exemplo 2: Remoção do sqlite init() com log.Fatal (P0 #1)](#exemplo-2-remoção-do-sqlite-init-com-logfatal-p0-1)
6. [DDNA vs ADR: Quando Usar Cada Um](#ddna-vs-adr-quando-usar-cada-um)
7. [Onde Armazenar](#onde-armazenar)
8. [Integração com o Workflow memorize-commit](#integração-com-o-workflow-memorize-commit)
9. [Integração com a Constituição e Quality Gates](#integração-com-a-constituição-e-quality-gates)
10. [Regras de Uso](#regras-de-uso)

---

## Propósito

O **Decision DNA (DDNA)** é o formato padronizado de documentação de decisões técnicas rápidas no ecossistema Cosca. Ele captura não apenas **o que** foi decidido, mas:

- **Contexto**: O problema que motivou a decisão
- **Alternativas**: O que foi considerado e descartado
- **Evidências**: Dados concretos que embasaram a escolha
- **Rastreabilidade**: Quem participou, quando, com qual confiança

O DDNA foi projetado para ser **preenchido em 2-5 minutos** por qualquer agente ou Chief, e para ser **parseável automaticamente** por ferramentas de análise (grep, FTS5, scripts de auditoria).

### Filosofia

```
Decisão sem DDNA = decisão esquecida
Decisão com DDNA = ativo de conhecimento que se acumula
```

Seis meses depois, qualquer agente ou o Don pode perguntar:
- *"Por que decidimos remover os 10 chat.go em vez de migrá-los?"*
- *"Quais decisões dos últimos 6 meses foram revertidas?"*
- *"Qual era o contexto quando escolhemos a Opção C em vez da A?"*

E o sistema responde com o registro completo.

---

## Formato Padronizado (YAML Frontmatter)

O formato usa **YAML frontmatter** em arquivos `.md` para garantir parseabilidade automatizada. Cada DDNA é um arquivo Markdown com frontmatter YAML seguido de seções de texto livre para contexto e justificativa.

```yaml
---
id: DDNA-YYYY-MM-DD-NNN
title: "Título conciso da decisão"
status: proposed | accepted | deprecated | superseded
date: YYYY-MM-DD
agents:
  - cosca-agent-name
domain: architecture | backend | security | database | devops | testing | etc.
decision_level: 1 | 2 | 3 | 4 | 5
confidence: 0.0-1.0
revisit: YYYY-MM-DD
tags:
  - dna
  - domain-tag
  - specific-tag
supersedes:
superseded_by:
---

## Context

2-5 parágrafos explicando:
- O problema que motivou a decisão
- O estado do sistema ANTES da decisão
- Constraints relevantes (prazo, recursos, dependências)
- Por que a decisão era necessária AGORA

## Options

### Opção A: [nome curto]

**Descrição:** O que a opção A propõe

| Critério | Avaliação |
|----------|-----------|
| Prós | Lista de vantagens |
| Contras | Lista de desvantagens |
| Evidências | Links/dados que suportam ou contradizem |
| Esforço | XS / S / M / L / XL |
| Risco | baixo / médio / alto |

### Opção B: [nome curto]

...

### Opção C: [nome curto]

...

## Decision

**Decisão final:** [Opção escolhida]

**Justificativa:** Por que esta opção foi escolhida sobre as demais. Referencie evidências específicas.

## Consequences

### Positivas
- Lista de consequências positivas observadas ou esperadas

### Negativas
- Lista de consequências negativas observadas ou esperadas

### Mitigações
- Como as consequências negativas estão sendo endereçadas

## Evidence

- [aprendizado relevante](link para learnings.md)
- [padrão aplicado](link para patterns.md)
- [código/commit relevante](link)
- [benchmark ou métrica](link)

## Outcome Validation

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success / partial / failure / mixed / pending |
| **Data da validação** | YYYY-MM-DD |
| **Validador** | agente ou Don |
| **Evidência do resultado** | O que prova que deu certo, errado ou misto |
```

---

## Especificação dos Campos

### Campos Obrigatórios

| Campo | Tipo | Descrição | Regra |
|-------|------|-----------|-------|
| `id` | string | ID único no formato `DDNA-YYYY-MM-DD-NNN` | NNN sequencial por data; IDs não são reutilizados |
| `title` | string | Título conciso (≤ 120 caracteres) | Deve ser autoexplicativo |
| `status` | enum | `proposed`, `accepted`, `deprecated`, `superseded` | Apenas estes 4 valores |
| `date` | date | Data da decisão (YYYY-MM-DD) | Data real, não a data do registro |
| `agents` | list | Lista de agentes/Chiefs envolvidos | Mínimo 1 |
| `domain` | string | Domínio primário da decisão | Usar domínios do catálogo de capabilities |
| `decision_level` | int | 1=tático, 2=design local, 3=arquitetura, 4=estratégico, 5=fundacional | Ver [CONSTITUTION.md Part IV](../../CONSTITUTION.md) |
| `confidence` | float | Confiança na decisão no momento (0.0-1.0) | Baseada em evidências, não em opinião |
| `revisit` | date | Data de revalidação sugerida | Máximo 6 meses para nível ≥ 3 |

### Campos Opcionais (Altamente Recomendados)

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `tags` | list | Mínimo 3 tags incluindo `#dna` |
| `supersedes` | string | ID do DDNA ou ADR que este substitui |
| `superseded_by` | string | ID do DDNA ou ADR que substituiu este |
| `horizon_impact` | map | Avaliação multi-horizonte da decisão (F3.6 Cognitive Horizon) — impactos H1/H2/H3, horizon_score, trade-off, recomendação. Obrigatório quando o engine F3.6 emitiu recomendação ≠ "aceitar" ou quando a decisão envolve trade-off entre horizontes. |

### Campo `horizon_impact` (F3.6 Cognitive Horizon)

Preenchido pelo [Cognitive Horizon Engine (F3.6)](../../engines/cognitive-horizon/SKILL.md) quando a decisão é avaliada nas 3 escalas de tempo (dia/semana/trimestre) antes da execução. Estrutura:

```yaml
horizon_impact:                    # ★ F3.6 — avaliação multi-horizonte
  h1_impact: 0.8                   # -1..+1: efeito no horizonte operacional (0-1d)
  h2_impact: 0.2                   # -1..+1: efeito no horizonte tático (1-7d)
  h3_impact: -0.7                  # -1..+1: efeito no horizonte estratégico (1-3mo)
  horizon_score: -0.13             # (h1×0.2) + (h2×0.3) + (h3×0.5) — fórmula do Don
  tradeoff: "divida_estrategica"   # caminho_livre | divida_estrategica | investimento | nao_fazer
  recommendation: "repensar"       # aceitar | adiar | repensar
  recommendation_action: "repensar_abordagem"   # escopo | timing | abordagem
  alert: "curto_prazo_vs_longo_prazo"           # ou "none"
  momentum_expansion: 1.18         # F2.5 — expansão da janela H3
  h3_confidence: 1.0               # F2.5 — confiança da projeção H3
  evolution_score_delta_est: -3.0  # F10.2 — Δ estimado do score do mês
  p0: false                        # true = bypass (segurança primeiro)
  shadow: false                    # true = avaliação pós-fato (P0 shadow)
```

**Regras**:
- **Obrigatório** quando a recomendação do F3.6 ≠ "aceitar" (dívida estratégica, investimento com atrito, não fazer).
- **Obrigatório** para decisões de nível ≥ 2 com trade-off explícito entre horizontes.
- **P0** (emergência): bypass — o campo é preenchido em modo shadow após a execução.
- Permite queries futuras: "Quais decisões de julho acumularam dívida estratégica?" ou "Decisões com h3 negativo que foram aceitas mesmo assim deram certo?"

### Seções de Conteúdo

| Seção | Obrigatória? | Descrição |
|-------|:------------:|-----------|
| `## Context` | Sim | 2-5 parágrafos explicando o problema e o estado anterior |
| `## Options` | Sim | Pelo menos 2 opções (incluindo "não fazer nada") |
| `## Decision` | Sim | Decisão final e justificativa baseada em evidências |
| `## Consequences` | Sim | Positivas e negativas |
| `## Evidence` | Não | Links para evidências concretas |
| `## Outcome Validation` | Não | Preenchido após validação do resultado |

---

## Exemplo 1: Remoção dos 10 chat.go (Dead Code)

> **Decisão real**: Don (Opção C). 10 arquivos chat.go (5.399 linhas) + 9 chat_test.go (9.735 linhas) = **15.134 linhas de dead code removidas**.
> **Fonte**: [cosca-kernel learnings.md linha 565](../../memory/agent/cosca-kernel/learnings.md)

```yaml
---
id: DDNA-2026-07-30-001
title: "Remoção dos 10 chat.go dead code — providers não-linkados no binário"
status: accepted
date: 2026-07-30
agents:
  - cosca-kernel
  - cosca-technical-debt
  - cosca-architecture
domain: backend
decision_level: 3
confidence: 0.92
revisit: 2026-10-30
tags:
  - dna
  - dead-code
  - providers
  - chat
  - refactoring
  - go-list-deps
supersedes:
superseded_by:
---

## Context

O Cosca foi construído com 10 arquivos `chat.go` que implementavam a interface `ChatStream` para diferentes provedores de IA (anthropic, openai, azure, bedrock, google, ollama, deepseek, groq, mistral, etc.). Durante uma auditoria de dívida técnica conduzida pelo cosca-technical-debt, foi descoberto que **a maioria desses arquivos compilava mas nunca era linkada no binário final**.

A ferramenta `go build ./...` não detecta o problema — ela verifica compilação, não linkagem. Apenas `go list -deps` revelou a verdade: dos 10 provedores, apenas deepseek, groq e mistral eram efetivamente importados via `openaicompat` pattern. Os outros 7 provedores eram código morto que:

1. Compilava em toda build (custo adicional de CI)
2. Precisava ser mantido (atualizações de API, security patches)
3. Gerava testes órfãos (9 `chat_test.go` testando código morto)
4. Aumentava a superfície de código sem benefício (15.134 linhas)

## Options

### Opção A: Migrar todos os provedores para openaicompat pattern

**Descrição:** Migrar os 7 provedores legacy (anthropic, openai, azure, bedrock, google, ollama) para o `openaicompat` pattern já usado por deepseek/groq/mistral.

| Critério | Avaliação |
|----------|-----------|
| Prós | Todos os provedores ficam disponíveis via pattern unificado; código mais limpo |
| Contras | Esforço alto (7 provedores × ~200 linhas cada); alguns provedores têm APIs incompatíveis com OpenAI |
| Evidências | deepseek/groq/mistral já funcionam com openaicompat (83 linhas cada) |
| Esforço | L (semanas de trabalho) |
| Risco | médio — risco de quebrar compatibilidade com provedores existentes |

### Opção B: Manter tudo como está (não fazer nada)

**Descrição:** Manter os 10 chat.go existentes, aceitando o dead code como dívida técnica.

| Critério | Avaliação |
|----------|-----------|
| Prós | Zero esforço imediato |
| Contras | Dead code continuará compilando sem ser usado; testes órfãos permanecem; custo de manutenção contínuo |
| Evidências | Nenhuma — é a manutenção do status quo problemático |
| Esforço | Zero (mas custo contínuo) |
| Risco | médio — dead code não testado pode conter vulnerabilidades não detectadas |

### Opção C: Remover dead code e testes órfãos (DECISÃO)

**Descrição:** Remover os 10 `chat.go` que não são linkados no binário + 9 `chat_test.go` órfãos. Total: 15.134 linhas eliminadas.

| Critério | Avaliação |
|----------|-----------|
| Prós | Elimina 15.134 linhas de dead code; build mais limpo; CI mais rápido; zero custo de manutenção futuro |
| Contras | Perde código legacy que poderia ser migrado; provedores removidos precisarão ser reimplementados do zero se forem necessários no futuro |
| Evidências | `go list -deps` comprovou que apenas deepseek/groq/mistral são linkados; chat_test.go testava exclusivamente código não-linkado |
| Esforço | M (horas) |
| Risco | baixo — código não-linkado não afeta runtime; remoção é segura e reversível via git |

## Decision

**Decisão final:** Opção C — Remover dead code e testes órfãos.

**Justificativa:** O dead code de provedores foi comprovado via `go list -deps`, não por suposição. A ferramenta correta para detecção de dead code em Go é `go list -deps`, não `go build ./...`. A remoção em cascata (código → testes) é o padrão correto: ao remover os 10 `chat.go`, os 9 `chat_test.go` (que testavam exclusivamente código removido) também devem ser eliminados. A Opção A (migração) seria o ideal arquiteturalmente, mas o esforço não se justifica para provedores sem demanda. A Opção C é a mais segura, rápida e de menor risco.

## Consequences

### Positivas
- **15.134 linhas de dead code eliminadas** (5.399 de código + 9.735 de testes órfãos)
- **Build mais rápido**: menos código para compilar em CI
- **Manutenção zero**: provedores removidos não precisam mais de updates de API ou security patches
- **Clareza**: fica explícito quais provedores são realmente suportados (deepseek, groq, mistral)
- **Padrão estabelecido**: `go list -deps` passa a ser a ferramenta canônica para detecção de dead code

### Negativas
- **Provedores removidos**: se no futuro houver demanda por anthropic/openai/azure/bedrock/google/ollama, será necessário reimplementar
- **Perda de código legacy**: implementações que poderiam servir de referência para migração futura foram removidas

### Mitigações
- O git preserve o histórico — qualquer provedor removido pode ser recuperado via `git revert` ou `git log`
- O `openaicompat` pattern está documentado e padronizado — reimplementar um provedor seguindo o padrão leva ~83 linhas

## Evidence

- [cosca-kernel learnings.md — descoberta do dead code](../../memory/agent/cosca-kernel/learnings.md#L565)
- [cosca-technical-debt learnings.md — auditoria de dívida técnica](../../memory/agent/cosca-technical-debt/learnings.md#L67)
- [Heurística H-004 — Extract to Test](../../knowledge/heuristics/H-004-extract-to-test.yaml)
- [QUALITY_GATES.md §2.2 — Dead code: 0 instances, severidade Error](../../QUALITY_GATES.md)
- [CONSTITUTION.md §P2 — Código executado é a verdade absoluta](../../CONSTITUTION.md) (go list -deps > go build)

## Outcome Validation

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success |
| **Data da validação** | 2026-07-30 |
| **Validador** | cosca-kernel (validado por Don) |
| **Evidência do resultado** | `go list -deps` confirma que chat.go não aparece nas dependências de nenhum pacote linkado; `git diff --stat` mostra -15.134 linhas; `go build ./...` passa limpo |
```

---

## Exemplo 2: Remoção do sqlite init() com log.Fatal (P0 #1)

> **Decisão real**: Barreira P0 #1 — `init()` no pacote sqlite chamava `log.Fatal` se a migração falhasse, impossibilitando testes paralelos.
> **Fonte**: [cosca-kernel learnings.md linha 565](../../memory/agent/cosca-kernel/learnings.md)

```yaml
---
id: DDNA-2026-07-30-002
title: "Remoção do init() com log.Fatal no pacote sqlite — barreira P0 #1"
status: accepted
date: 2026-07-30
agents:
  - cosca-kernel
  - cosca-architecture
  - cosca-database
domain: database
decision_level: 3
confidence: 0.95
revisit: 2026-10-30
tags:
  - dna
  - sqlite
  - init-function
  - p0
  - testing
  - log-fatal
  - migration
supersedes:
superseded_by:
---

## Context

O pacote `internal/sqlite/` tinha uma função `init()` que executava validação de migrações na inicialização do pacote. Se a validação falhasse — por exemplo, durante testes paralelos que competiam pelo banco — `log.Fatal` era chamado, resultando em `os.Exit(1)`.

Este era um dos **3 blockers P0** identificados pela auditoria de qualidade (cosca-qa + cosca-testing). O problema central:

1. **Testes paralelos impossíveis**: qualquer teste que importasse o pacote sqlite acionava o `init()`, e dois testes rodando em paralelo causavam `log.Fatal`
2. **Sem graceful handling**: `log.Fatal` não pode ser capturado ou tratado — é um `os.Exit(1)` incondicional
3. **Design frágil**: `init()` é executado na ordem de importação, sem controle explícito do programador
4. **Impedimento para aumento de cobertura**: enquanto este blocker existisse, não era possível aumentar a cobertura de testes do pacote sqlite

## Options

### Opção A: Remover init() e exportar ValidateMigrations() (DECISÃO)

**Descrição:** Remover a função `init()` completamente. Extrair a lógica de validação de migrações para uma função pública `ValidateMigrations()` que deve ser chamada explicitamente em `Open()`.

| Critério | Avaliação |
|----------|-----------|
| Prós | Elimina o `os.Exit(1)` em testes paralelos; controle explícito da validação; padrão Go idiomático (init só para registro, não para lógica) |
| Contras | Quem chamar `Open()` sem antes chamar `ValidateMigrations()` pode operar com migrações inconsistentes |
| Evidências | Resolvido em 15 minutos; a solução era simples e direta |
| Esforço | XS (15 minutos) |
| Risco | baixo — `ValidateMigrations()` é chamada internamente em `Open()`, garantindo que sempre seja executada |

### Opção B: Manter init() mas substituir log.Fatal por erro retornado

**Descrição:** Manter o `init()` mas substituir `log.Fatal` por um mecanismo que registre o erro sem abortar o processo.

| Critério | Avaliação |
|----------|-----------|
| Prós | Menos mudança estrutural |
| Contras | `init()` continua sendo executado em ordem não-determinística; ainda impossibilita testes paralelos; padrão continua frágil |
| Evidências | Nenhuma — é um patch em cima de design problemático |
| Esforço | S |
| Risco | médio — o problema de paralelismo não é resolvido |

### Opção C: Manter status quo

**Descrição:** Aceitar que o pacote sqlite tem esta limitação e não pode ser testado em paralelo.

| Critério | Avaliação |
|----------|-----------|
| Prós | Zero esforço |
| Contras | Bloqueia aumento de cobertura de testes; testes paralelos impossíveis; risco de `os.Exit(1)` em produção |
| Evidências | Nenhuma |
| Esforço | Zero |
| Risco | alto — `log.Fatal` em init pode matar o processo em produção |

## Decision

**Decisão final:** Opção A — Remover init() e exportar ValidateMigrations().

**Justificativa:** A solução ideal (remover `init()` completamente + chamar validação no `Open()`) foi trivial de implementar (15 minutos) e zerou o risco de `os.Exit(1)` em testes paralelos. O padrão de usar `init()` para lógica de inicialização com efeitos colaterais é amplamente reconhecido como antipattern em Go. A migração para chamada explícita em `Open()` é o padrão correto e desbloqueia imediatamente testes paralelos no pacote sqlite.

## Consequences

### Positivas
- **Zero risco de `os.Exit(1)` em testes paralelos**: `ValidateMigrations()` retorna erro em vez de chamar `log.Fatal`
- **Desbloqueio de cobertura**: pacote sqlite pode agora ser testado em paralelo com outros pacotes
- **Padrão idiomático Go**: `init()` reservado para registro de metadados, não para lógica de inicialização
- **Resolução em 15 minutos**: blocker P0 resolvido com esforço mínimo

### Negativas
- **Inconsistência se `Open()` for chamado sem `ValidateMigrations()`**: mitigado chamando validação dentro de `Open()` automaticamente

### Mitigações
- `ValidateMigrations()` é chamada internamente em `Open()` — não é possível esquecer
- O padrão de chamada explícita permite que testes usem `ValidateMigrations()` seletivamente

## Evidence

- [cosca-kernel learnings.md — barreiras P0 identificadas e resolvidas](../../memory/agent/cosca-kernel/learnings.md#L565)
- [Qualidade: QUALITY_GATES.md §2.2 — Dead code e padrões de init()](../../QUALITY_GATES.md)
- [CONSTITUTION.md §P2 — Código executado é a verdade absoluta](../../CONSTITUTION.md)
- [ADR-0001 — AI Architecture](../../knowledge/architecture/adr/adr-0001-ai-architecture.md) (padrão de inicialização)

## Outcome Validation

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success |
| **Data da validação** | 2026-07-30 |
| **Validador** | cosca-kernel |
| **Evidência do resultado** | `go test -parallel 8 ./internal/sqlite/...` passa sem `log.Fatal`; `git diff` mostra remoção de `init()` e adição de `ValidateMigrations()` exportada |
```

---

## DDNA vs ADR: Quando Usar Cada Um

O Cosca possui **dois formatos de documentação de decisão** que coexistem com propósitos distintos:

| Dimensão | DDNA (Decision DNA) | ADR (Architecture Decision Record) |
|----------|---------------------|-----------------------------------|
| **Natureza** | Decisão técnica rápida | Decisão arquitetural com impacto duradouro |
| **Escopo** | Local, tático-operacional a design local | Arquitetural, estratégico |
| **Nível** | 1-3 (tático a arquitetura local) | 3-5 (arquitetura a fundacional) |
| **Tempo de preenchimento** | 2-5 minutos | 15-60 minutos |
| **Formato** | YAML frontmatter + seções leves | Markdown estruturado com seções completas |
| **Parser** | Automatizado (YAML) | Manual (revisão humana) |
| **Onde usar** | Refactoring, bug fix, dead code removal, mudanças de configuração, decisões de implementação | Novas capacidades, mudanças de stack, patterns arquiteturais, contratos entre módulos |
| **Revisão necessária** | Auto-revisão do agente | Architecture Review Board (ARB) |
| **Exemplos** | Remover dead code, substituir init() por Open(), configurar CI gate | Adotar SQLite em vez de PostgreSQL, criar novo engine, definir padrão de plugin |
| **Reversibilidade** | Alta (fácil de reverter) | Baixa (custo de reversão alto) |

### Regra de Ouro

> **Use DDNA quando** a decisão envolve uma escolha entre alternativas com impacto moderado, reversível em ≤ 1 semana, e o custo de não registrar é maior que o custo de registrar.
>
> **Use ADR quando** a decisão estabelece um precedente arquitetural que afetará múltiplos módulos por meses ou anos, ou quando o Architecture Review Board (ARB) precisa aprovar.

### Casos Fronteiriços

| Situação | Recomendação | Razão |
|----------|:------------:|-------|
| Decisão local que afeta 1 módulo | DDNA | Escopo pequeno, reversível |
| Decisão que estabelece padrão para N módulos | ADR | Impacto cross-module, precisa de revisão |
| Decisão que reverte ADR anterior | ADR | ADR só é substituído por outro ADR |
| Decisão de implementação dentro de um padrão já definido | DDNA | A decisão arquitetural já foi tomada |
| Decisão com confidence < 0.70 que afeta 2+ domínios | ADR | Alta incerteza + alto impacto = precisa de revisão |
| Decisão do Don ("faça assim") | DDNA | Veto absoluto, mas precisa ser registrada |

---

## Onde Armazenar

```
internal/embed/cosca/memory/decisions/
├── ddna/          ← DDNAs (decisões técnicas rápidas)
│   ├── INDEX.md
│   ├── DDNA-2026-07-30-001.md
│   └── DDNA-2026-07-30-002.md
├── adr/           ← ADRs (decisões arquiteturais)
│   ├── INDEX.md
│   ├── ADR-0001-ai-architecture.md
│   └── ...
└── INDEX.md       ← Índice unificado de ambos
```

**Regras de armazenamento:**

1. **DDNAs** vão em `memory/decisions/ddna/` — cada arquivo é um DDNA individual
2. **ADRs** vão em `memory/decisions/adr/` — cada arquivo é um ADR individual (já existente em `knowledge/architecture/adr/`)
3. **INDEX.md** em cada diretório lista todas as decisões com status, data, e domínio
4. **O diretório `memory/decisions/`** é o runtime onde agentes escrevem decisões
5. **O diretório `knowledge/architecture/adr/`** é o repositório curado de ADRs (versão canônica, imutável)
6. **O diretório `memory/decision/`** existente contém decisões do framework (evolução do Cosca framework em si, não do projeto)

### Ciclo de Vida

```
DDNA criado em memory/decisions/ddna/
    │
    ├── Se for decisão local → permanece como DDNA (curadoria periódica)
    │
    └── Se a decisão provar ter impacto arquitetural duradouro
            │
            ▼
        Promovido a ADR em knowledge/architecture/adr/
            │
            ▼
        DDNA original marcado como `superseded_by: ADR-NNNN`
```

---

## Integração com o Workflow memorize-commit

O workflow `memorize-commit` (definido no pipeline de metacognição) deve referenciar DDNAs das decisões tomadas no commit:

### Fluxo

```
1. ANTES DO COMMIT:
   - Para cada decisão técnica significativa tomada durante a task:
     a. Criar DDNA em memory/decisions/ddna/
     b. Preencher campos obrigatórios (2-5 min)
     c. Garantir que todas as alternativas foram consideradas

2. IMPACT REPORT (na mensagem de commit):
   - Referenciar o DDNA: "DDNA-2026-07-30-001: Remoção dos 10 chat.go"
   - Incluir sumário do impacto: "-15.134 lines, build limpo"
   - Listar agentes participantes

3. EVIDENCE LINKS:
   - O campo `evidence` do DDNA deve linkar para:
     - learnings.md do(s) agente(s) envolvido(s)
     - patterns.md se um novo padrão foi descoberto
     - failures.md se a decisão foi tomada após uma falha
     - Commits relacionados (hash)

4. PÓS-COMMIT:
   - Atualizar INDEX.md de ddna/ com o novo DDNA
   - Se a decisão gerou aprendizado, registrar em learnings.md
   - Se a decisão gerou padrão, registrar em patterns.md
```

### Template para Impact Report

```markdown
## Impact Report

### Decisões
- **DDNA-2026-07-30-001**: Remoção dos 10 chat.go (dead code)
  - Contexto: go list -deps revelou 7 provedores não-linkados
  - Decisão: Opção C — remoção completa (15.134 linhas)
  - Agentes: cosca-kernel, cosca-technical-debt, cosca-architecture

### Arquivos alterados
- internal/chat/anthropic/ → removido (7 arquivos)
- internal/chat/openai/ → removido (3 arquivos)
- ... (lista completa)

### Métricas
- Linhas removidas: 15.134
- Build time reduction: ~15%
- CI gate adicionado: go list -deps check
```

---

## Integração com a Constituição e Quality Gates

### CONSTITUTION.md

| Princípio | Relação com DDNA | Mecanismo |
|-----------|------------------|-----------|
| **P2 — Código executado é a verdade absoluta** | Evidências do DDNA devem referenciar código real (deps, build, testes), não suposições | `go list -deps` como evidência no exemplo do chat.go |
| **P3 — Nenhum agente age sem rastro** | DDNA é a implementação do rastro para decisões técnicas | Todo DDNA registra quem, quando, por que, quais alternativas |
| **P4 — O Don tem veto absoluto** | Don pode reverter qualquer decisão; DDNA registra o estado antes da reversão | Campo `superseded_by` e `status: deprecated/superseded` |
| **P5 — A família aprende com erros** | DDNA com outcome `failure` alimenta failures.md | Outcome Validation registra sucesso/falha |
| **P6 — Evolução sem regressão** | DDNA registra o nível da decisão; impede regressão de técnica | Campo `decision_level` |
| **P7 — Memória sem poluição** | DDNA obsoleto é marcado `deprecated` ou `superseded`, não removido | Ciclo de vida: active → deprecated/superseded |

### QUALITY_GATES.md

| Gate | Relação com DDNA |
|------|------------------|
| **G0 — Pre-Work** | DDNA pode ser criado como `proposed` antes do trabalho começar |
| **G0.5 — Contrafactual** | As `Options` do DDNA são a entrada para o contrafactual; o output do contrafactual alimenta a `Decision` |
| **G2.6 — Documentation** | A criação de DDNA é um dos checks: "DDNA criado se decisão técnica foi tomada" |
| **G2.2 — Code Quality** | Dead code (0 instances) referenciado no exemplo do chat.go |

### Integração com o Ciclo de Decisão (Parte IV da Constituição)

```
Step 1: OBJETIVO         ─→ DDNA Context
Step 2: EVIDÊNCIAS       ─→ DDNA Options[].Evidências + Evidence
Step 3: ANÁLISE          ─→ DDNA Options + Decision
Step 4: RISCOS           ─→ DDNA Consequences.Negativas + Mitigações
Step 5: PLANO            ─→ (não no DDNA — é o plano de execução)
Step 6: EXECUÇÃO         ─→ (não no DDNA — é a implementação)
Step 7: VALIDAÇÃO        ─→ DDNA Outcome Validation
Step 8: CRÍTICA          ─→ DDNA Consequences (revisão pós-decisão)
Step 9: APRENDIZADO      ─→ learnings.md + patterns.md (linkado em Evidence)
Step 10: ATUALIZAÇÃO     ─→ update confidence, status, revisit date
```

---

## Regras de Uso

### Quando criar um DDNA (obrigatório)

Uma decisão **DEVE** ser registrada como DDNA quando atende a **PELO MENOS UM** destes critérios:

1. **Confidence < 0.95**: Há incerteza real na decisão
2. **Impacto cross-domain**: Afeta 2+ domínios (ex: backend + database + testing)
3. **Remove código**: Qualquer remoção significativa (> 50 linhas ou > 5 arquivos)
4. **Nível ≥ 2**: Decisão de design local, arquitetura, estratégica ou fundacional
5. **Envolve trade-off explícito**: Duas ou mais alternativas razoáveis competindo
6. **O Don pediu**: Se o Don perguntar "por que fizemos isso?", a resposta deve estar no DDNA

### Quando NÃO criar

- Decisões puramente operacionais (nível 1) com confidence ≥ 0.95
- Escolhas triviais sem alternativas reais (ex: "usar const em vez de let")
- Aplicação direta de um padrão já consolidado sem variação
- Mudanças triviais de formatação, typo fixes, docs

### Regras de Preenchimento

1. **Seja rápido**: 2-5 minutos. Não precisa ser perfeito na primeira versão
2. **Seja honesto**: Riscos e evidências contrárias DEVEM ser registrados com o mesmo rigor das evidências favoráveis
3. **Seja específico**: Evidências devem referenciar arquivos, linhas, dados — não opiniões
4. **Alternativas reais**: Cada alternativa deve ser viável e justificável. "Não fazer nada" é sempre uma opção válida
5. **Confiança honesta**: O `confidence` reflete a confiança no MOMENTO da decisão, não revisada depois
6. **Revalidação**: Decisões de nível ≥ 3 devem ter `revisit` em no máximo 6 meses

### Templates e Automação

Para criar um DDNA rapidamente, use:

```bash
# Criar DDNA a partir de template
cp internal/embed/cosca/templates/ddna-template.md memory/decisions/ddna/DDNA-$(date +%Y-%m-%d)-NNN.md

# Listar todos os DDNAs
ls memory/decisions/ddna/

# Buscar DDNAs por domínio
grep "domain: database" memory/decisions/ddna/*.md

# Buscar DDNAs por status
grep "status: accepted" memory/decisions/ddna/*.md

# Buscar DDNAs com confiança baixa
grep "confidence: 0\\.[0-7]" memory/decisions/ddna/*.md
```

---

## DDNA Template (para criar novos)

Use este template para criar novos DDNAs rapidamente:

```yaml
---
id: DDNA-YYYY-MM-DD-NNN
title: ""
status: proposed
date: YYYY-MM-DD
agents:
  - cosca-agent-name
domain:
decision_level: 2
confidence: 0.80
revisit: YYYY-MM-DD
tags:
  - dna
supersedes:
superseded_by:
horizon_impact:                    # ★ F3.6 — preencher se avaliado (ver §3.1)
  h1_impact: 0.0
  h2_impact: 0.0
  h3_impact: 0.0
  horizon_score: 0.0
  tradeoff: ""
  recommendation: ""
  recommendation_action: ""
  alert: "none"
  momentum_expansion: 1.0
  h3_confidence: 0.5
  evolution_score_delta_est: 0.0
  p0: false
  shadow: false
---

## Context



## Options

### Opção A:

### Opção B:

### Opção C:

## Decision

**Decisão final:**

**Justificativa:**

## Consequences

### Positivas

### Negativas

### Mitigações

## Evidence



## Outcome Validation

| Campo | Valor |
|-------|-------|
| **Resultado observado** | pending |
| **Data da validação** | |
| **Validador** | |
| **Evidência do resultado** | |
```

> Salve este template como `internal/embed/cosca/templates/ddna-template.md` para uso com o comando de cópia acima.

---

## Relacionado

| Documento | Relação |
|-----------|---------|
| [DECISION_DNA_FORMAT.md](../../memory/DECISION_DNA_FORMAT.md) | Versão agent-facing do formato (protocolo de aprendizado) |
| [DECISION_DNA_EXAMPLE.md](../../memory/DECISION_DNA_EXAMPLE.md) | Exemplo do auto-jail com memfd_create (decisão real Nível 4) |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Protocolo de aprendizado — DDNA integrado como extensão |
| [CONSTITUTION.md](../../CONSTITUTION.md) | Princípios imutáveis P2-P7, Ciclo de Decisão (Parte IV) |
| [QUALITY_GATES.md](../../QUALITY_GATES.md) | Gate 0.5 Contrafactual, Gate 2.2 Code Quality |
| [MEMORY_MODEL.md](../../MEMORY_MODEL.md) | Modelo de memória — armazenamento de decisões |
| [CAPABILITY_CATALOG.md](../../capabilities/CAPABILITY_CATALOG.md) | Catálogo de capacidades — domínios para o campo `domain` |
| [cognitive-horizon/SKILL.md](../../engines/cognitive-horizon/SKILL.md) | F3.6 — engine que preenche o campo `horizon_impact` (§3.1) |
| [Heurísticas H-004](../../knowledge/heuristics/H-004-extract-to-test.yaml) | Extract to Test — padrão relevante para dead code |
| [Heurísticas H-013](../../knowledge/heuristics/H-013-architecture-code-gap.yaml) | Architecture-Code Gap — verificar ADR vs código |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|-----------|
| 1.1.0 | 2026-07-30 | cosca-architecture | Adicionado campo `horizon_impact` (F3.6 Cognitive Horizon) — avaliação multi-horizonte da decisão: impactos H1/H2/H3, horizon_score, trade-off, recomendação, momentum_expansion (F2.5), evolution_score_delta_est (F10.2). Obrigatório quando a recomendação do F3.6 ≠ "aceitar". Template atualizado. |
| 1.0.0 | 2026-07-30 | cosca-architecture | Criação inicial — formato DDNA canônico, exemplos chat.go e sqlite init(), regras DDNA vs ADR, integração memorize-commit |

---

> *"Decisões sem DNA são esquecidas. Decisões com DNA são ativos de conhecimento que se acumulam com o tempo."*
> — Cosca Kernel, 2026-07-30
