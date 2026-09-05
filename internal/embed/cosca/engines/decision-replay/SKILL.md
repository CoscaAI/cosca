# DECISION REPLAY ENGINE — Motor de Replay de Decisões (F7.5)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-decision-replay`
> **Conceito**: Engineering Intelligence — Decision Replay
> **Referências**: next-evolution-phases.md §F7.5 | DECISION_DNA.md (F1.1) | contrafactual-gate.md (F1.2/G0.5) | TRUST_REGISTRY.md (F7.2) | memorize-commit.md | IMPACT_REPORTS.md (F7.4)
> **Dependências**: DDNA (F1.1) | Contrafactual Gate (F1.2) | Trust Registry (F7.2) | Impact Reports (F7.4)
> **CMI Impact**: Julgamento +5, Aprendizado +5, Transparência +3

---

## Índice

1. [O que é o Decision Replay Engine](#1-o-que-é-o-decision-replay-engine)
2. [Pipeline de Replay](#2-pipeline-de-replay)
3. [Formato de Output](#3-formato-de-output)
4. [Integração com DDNA (F1.1)](#4-integração-com-ddna-f11)
5. [Integração com Contrafactual Gate (F1.2)](#5-integração-com-contrafactual-gate-f12)
6. [Integração com Trust Registry (F7.2)](#6-integração-com-trust-registry-f72)
7. [Integração com memorize-commit (F7.4)](#7-integração-com-memorize-commit-f74)
8. [Interface CLI](#8-interface-cli)
9. [Exemplo: Remoção dos 10 chat.go](#9-exemplo-remoção-dos-10-chatgo)
10. [Exemplo com Decisão Real Desta Sessão](#10-exemplo-com-decisão-real-desta-sessão)
11. [Qualidade e Critérios de Aceite](#11-qualidade-e-critérios-de-aceite)
12. [Tratamento de Erros](#12-tratamento-de-erros)
13. [Relacionados](#13-relacionados)
14. [HISTÓRICO](#14-histórico)

---

## 1. O que é o Decision Replay Engine

### Definição

O **Decision Replay Engine** é o motor do Cosca que permite **reproduzir decisões passadas com reconstrução completa de contexto**. Quando o Don (ou qualquer agente) pergunta *"Por que escolhemos X?"* ou *"O que levou à decisão Y?"*, o engine busca todas as fontes relevantes — DDNA, Impact Reports, learnings, git log, Trust Registry — e reconstrói o momento da decisão como se o usuário estivesse lá.

Ele é o **elo final da cadeia de rastreabilidade** do Cosca: onde F1.1 (DDNA) documenta, F1.2 (Gate) questiona, F7.2 (Trust) pondera, F7.4 (Impact Reports) registra, e F7.5 (Replay) *revive*.

### Filosofia

```
"Decisão sem replay é decisão enterrada.
 Decisão com replay é decisão que ensina."
 — Cosca Architecture Chief, 2026-07-30
```

Três meses depois, qualquer agente ou o Don pode perguntar:
- *"Volte para 2026-07-29, veja a discussão sobre auto-jail, alternativas consideradas, benchmark, decisão final"*
- *"Quais decisões dos últimos 6 meses foram revertidas e por quê?"*
- *"Mostre o contexto completo da escolha entre Bubblewrap e Docker"*

E o engine responde com o cenário completo: data, agentes envolvidos, alternativas, evidências, resultado.

### Propósito

| Dimensão | Descrição |
|----------|-----------|
| **Cognitivo** | Evitar o viés de hindsight — ao reconstruir o contexto original, o replay impede que decisões sejam julgadas com informação que não existia na época |
| **Arquitetural** | Fechar o ciclo de rastreabilidade: toda decisão registrada deve ser replayável |
| **Auditável** | Produzir relatório estruturado que pode ser usado em auditorias, retrospectivas e análises pós-mortem |
| **Econômico** | Reduzir o custo de "descobrir por que algo foi feito" — em vez de caçar em logs, um comando revela tudo |
| **Educacional** | Novos agentes podem aprender com decisões passadas vendo o contexto completo, não apenas o resultado |
| **Histórico** | Acumular um acervo replayável de decisões que calibra o Contrafactual Gate e o Prediction Engine |

### Inputs

| Input | Fonte | Descrição |
|-------|-------|-----------|
| `query` | Don/agente | A pergunta: "Por que X?", "Mostre a decisão Y" |
| `decision_id` | Don/agente | ID específico: "DDNA-2026-07-30-001" |
| `date` | Don/agente | Data alvo: "2026-07-29" |
| `topic` | Don/agente | Tópico: "auto-jail", "bubblewrap", "chat.go" |
| `agent_name` | Don/agente | Agente específico: "cosca-architecture" |

### Outputs

| Output | Formato | Descrição |
|--------|---------|-----------|
| `replay_report` | Markdown estruturado | Relatório completo com contexto, alternativas, decisão, evidências, resultado |
| `replay_id` | string | Identificador único do replay |
| `confidence_replay` | float 0.0-1.0 | Confiança do engine na fidelidade do replay (baseada na completude das fontes) |
| `sources_used` | list | Lista de fontes consultadas com status (encontrada/não encontrada) |

---

## 2. Pipeline de Replay

O Decision Replay Engine opera em **5 etapas** que transformam uma pergunta vaga em um replay estruturado.

### Visão Geral

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    DECISION REPLAY ENGINE PIPELINE                        │
│                                                                          │
│  Don pergunta: "Por que escolhemos Bubblewrap?"                          │
│       │                                                                  │
│       ▼                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  1. ENTENDER QUERY                                               │     │
│  │  ├── Parse: entidade alvo (Bubblewrap), tipo (decisão),          │     │
│  │  │          período (implícito ou explícito)                     │     │
│  │  └── Output: `search_target` + `search_type` + `time_window`     │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  2. BUSCAR FONTES                                               │     │
│  │  ├── DDNAs em memory/decisions/ddna/ (F1.1)                     │     │
│  │  ├── Impact Reports em memory/timeline/impact-reports/ (F7.4)   │     │
│  │  ├── learnings.md dos agentes envolvidos                        │     │
│  │  ├── Trust Registry em memory/trust/TRUST_REGISTRY.md (F7.2)    │     │
│  │  ├── Contrafactual Gate results em gate-results/ (F1.2)         │     │
│  │  ├── git log (commits associados)                               │     │
│  │  └── Output: `sources_found` + `sources_missing`                │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  3. RECONSTRUIR CONTEXTO                                        │     │
│  │  ├── Data e hora da decisão                                     │     │
│  │  ├── Agentes envolvidos                                         │     │
│  │  ├── Estado do sistema ANTES da decisão                          │     │
│  │  ├── Alternativas consideradas (do DDNA + Contrafactual Gate)   │     │
│  │  ├── Evidências que embasaram cada alternativa                  │     │
│  │  ├── Decisão final e justificativa                              │     │
│  │  ├── Resultado observado (do Outcome Validation)                │     │
│  │  └── Output: `context_reconstructed` (struct interna)            │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  4. APRESENTAR REPLAY                                            │     │
│  │  ├── Renderizar em Markdown estruturado (formato canônico)       │     │
│  │  ├── Incluir links navegáveis para fontes originais              │     │
│  │  ├── Exibir confidence do replay (fidelidade)                    │     │
│  │  └── Output: `replay_report.md` (exibido no terminal ou arquivo) │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  5. DON DECIDE                                                  │     │
│  │  ├── Confirmar: "OK, entendi. Segue."                          │     │
│  │  ├── Reabrir: "Reabrir decisão — quero revisitar as opções."   │     │
│  │  │   └── Aciona Contrafactual Gate (F1.2) para reavaliação      │     │
│  │  └── Replay outro: "Agora mostre a decisão sobre init()"       │     │
│  │      └── Volta ao Passo 1 com nova query                       │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### Passo a Passo Detalhado

#### Passo 1: Entender Query

- **Executor**: Decision Replay Engine
- **Tarefa**: Parsear a pergunta do Don/agente para extrair entidade alvo, tipo de busca e janela temporal
- **Regras de Parse**:

| Padrão de Query | Entidade Extraída | Tipo |
|-----------------|-------------------|------|
| `"Por que escolhemos X?"` | `X` | decision-why |
| `"Mostre a decisão sobre Y"` | `Y` | decision-topic |
| `"DDNA-2026-07-30-001"` | `DDNA-2026-07-30-001` | decision-id |
| `"O que aconteceu em 2026-07-29?"` | `2026-07-29` | date-focus |
| `"Decisões do cosca-kernel"` | `cosca-kernel` | agent-focus |
| `"Replay da remoção dos chat.go"` | `chat.go` | topic-focus |

- **Output**: `{ search_target, search_type, time_window, confidence: 0.0-1.0 }`

#### Passo 2: Buscar Fontes

- **Executor**: Decision Replay Engine
- **Tarefa**: Escanear todas as fontes disponíveis em ordem de relevância
- **Ordem de busca**:

| Prioridade | Fonte | Localização | Campos Chave |
|------------|-------|-------------|--------------|
| 1 | **DDNA** | `memory/decisions/ddna/` | `id`, `title`, `date`, `agents`, `domain`, `tags` |
| 2 | **Contrafactual Gate Output** | `engines/decision/contrafactual-gate.md` + gate-results/ | `decision_id`, `alternatives`, `recommendation` |
| 3 | **Impact Report** | `memory/timeline/impact-reports/` | `commit_hash`, `message`, `type`, `engineering_score` |
| 4 | **Trust Registry** | `memory/trust/TRUST_REGISTRY.md` | `agent`, `task_type`, `outcome`, `confidence_delta` |
| 5 | **learnings.md** (agentes) | `memory/agent/<agent>/learnings.md` | `Learned`, `Related`, `Tags` |
| 6 | **git log** | Repositório git | `commit hash`, `message`, `author`, `date` |
| 7 | **Impact Report archive** | `memory/timeline/impact-reports/` | commits históricos |

- **Algoritmo de busca**:
  1. Se `search_type == "decision-id"`: busca direta pelo ID nos DDNAs
  2. Se `search_type == "topic-focus"`: grep por `title` + `tags` + `Context` nos DDNAs
  3. Se `search_type == "agent-focus"`: grep por `agents` nos DDNAs + Trust Registry
  4. Se `search_type == "date-focus"`: filtra DDNAs por `date` + Impact Reports por data
  5. Se `search_type == "decision-why"`: busca combinada — DDNA title/tags + Impact Reports + git log

- **Output**: `{ sources_found: [...], sources_missing: [...], search_confidence: 0.0-1.0 }`

#### Passo 3: Reconstruir Contexto

- **Executor**: Decision Replay Engine
- **Tarefa**: Montar a narrativa completa da decisão a partir das fontes encontradas
- **Estrutura de contexto reconstruído**:

```yaml
reconstructed_context:
  decision_id: "DDNA-YYYY-MM-DD-NNN"
  title: "Título da decisão"
  date: "YYYY-MM-DD"
  agents: ["agente1", "agente2"]
  state_before: "Estado do sistema antes da decisão"
  problem: "O problema que motivou a decisão"
  options:
    - id: "A"
      description: "Descrição"
      pros: ["pró1", "pró2"]
      cons: ["contra1"]
      evidence: ["fonte1", "fonte2"]
      effort: "M"
      risk: "médio"
    - id: "B"
      ...
    - id: "C"
      ...
  decision:
    chosen: "C"
    rationale: "Por que esta foi escolhida"
    confidence: 0.92
  contrafactual:
    performed: true
    recommendation: "proceed"
    gate_confidence: 0.85
  consequences:
    positive: ["impacto1", "impacto2"]
    negative: ["impacto3"]
  outcome:
    result: "success"
    validated_by: "agente"
    evidence: "O que prova que deu certo"
  trust_registry:
    agent_success_rate: 0.83
    similar_decisions: 12
  engineering_score: 79.5
  commits:
    - hash: "abc123"
      message: "feat: removeu chat.go dead code"
  learnings:
    - learning: "go list -deps detecta dead code"
      source: "cosca-kernel learnings.md L565"
```

#### Passo 4: Apresentar Replay

- **Executor**: Decision Replay Engine
- **Tarefa**: Renderizar o contexto reconstruído no formato canônico de output (seção 3)
- **Regras de renderização**:
  1. Título sempre inclui o DDNA ID + título completo
  2. Seções obrigatórias: Data, Agentes, Contexto, Opções, Decisão, Evidências, Resultado
  3. Seções condicionais: Análise Contrafactual (se Gate foi executado), Trust Registry (se consultado), Engineering Score (se disponível)
  4. Todos os links são relativos e navegáveis
  5. Badge de confiança do replay: 🟢 (≥0.90), 🟡 (≥0.70), 🔴 (<0.70)
  6. Se fontes estiverem faltando, exibir aviso: "⚠️ Replay incompleto — N fontes não encontradas"

- **Output**: Markdown formatado para terminal (stdout) ou arquivo (`memory/replays/{replay_id}.md`)

#### Passo 5: Don Decide

- **Executor**: Don (humano)
- **Opções pós-replay**:

| Ação do Don | Comportamento do Engine |
|-------------|------------------------|
| **Confirmar** (`👍`, `ok`, `entendi`) | Replay registrado como "visualizado". Nada mais acontece. |
| **Reabrir decisão** (`reabrir`, `revisitar`) | Engine aciona **Contrafactual Gate (F1.2)** com o DDNA original como entrada. O Gate gera novas alternativas com base no contexto atual. Don pode decidir novamente. |
| **Replay de outra** (`mostre Y`, `agora X`) | Engine volta ao Passo 1 com a nova query. |
| **Exportar** (`export`, `salvar`) | Engine salva o replay em `memory/replays/{replay_id}.md`. |
| **Comparar** (`compare com Z`) | Engine carrega dois replays e gera diff lado a lado. |

### Diagrama de Estado

```
                    ┌──────────────┐
                    │   IDLE       │  ← Engine carregado, esperando query
                    └──────┬───────┘
                           │ Don pergunta: "Por que X?"
                           ▼
                    ┌──────────────┐
                    │  PARSING     │  ← Passo 1: entender query
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  SEARCHING   │  ← Passo 2: buscar fontes
                    └──────┬───────┘
                           │
                  ┌────────┴────────┐
                  │                 │
                  ▼                 ▼
           ┌────────────┐   ┌──────────────┐
           │ FOUND      │   │ NOT FOUND    │  ← Nenhuma fonte encontrada
           └──────┬─────┘   └──────┬───────┘
                  │                │
                  ▼                ▼
           ┌────────────┐   ┌──────────────┐
           │ RECONSTRUCT│   │ SUGGEST      │  ← Sugere queries alternativas
           │ Passo 3    │   │              │
           └──────┬─────┘   └──────┬───────┘
                  │                │
                  ▼                │
           ┌────────────┐         │
           │ PRESENT    │         │
           │ Passo 4    │◄────────┘
           └──────┬─────┘
                  │
                  ▼
           ┌──────────────┐
           │ DON DECIDE   │  ← Passo 5
           └──────┬───────┘
                  │
        ┌─────────┼─────────┬──────────────┐
        │         │         │              │
        ▼         ▼         ▼              ▼
   ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐
   │CONFIRM │ │REOPEN  │ │REPLAY  │ │EXPORT    │
   │  → IDLE│ │→ GATE  │ │→ Step1 │ │→ SAVE    │
   └────────┘ └────────┘ └────────┘ └──────────┘
```

---

## 3. Formato de Output

O formato canônico de output do Decision Replay Engine é **Markdown estruturado**, projetado para ser legível no terminal e parseável por ferramentas.

### Template Canônico

```markdown
╔══════════════════════════════════════════════════════════════════════════╗
║                     DECISION REPLAY                                     ║
║                     F7.5 — Decision Replay Engine                       ║
╚══════════════════════════════════════════════════════════════════════════╝

🆔 DDNA-ID: {ddna_id}
📅 Data: {date}
👤 Agentes: {agents}
🎯 Domínio: {domain}
🏷️ Tags: {tags}
📊 Confidence: {confidence}/1.0
📅 Revisitar: {revisit}
🔁 Status: {status}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 🧠 Contexto

{context}

## 🔄 Opções Consideradas

### Opção {id}: {title}

**Descrição:** {description}

| Critério | Avaliação |
|----------|-----------|
| Prós | {pros} |
| Contras | {cons} |
| Evidências | {evidence} |
| Esforço | {effort} |
| Risco | {risk} |

### Opção {id}: {title}
...

## ✅ Decisão

**Escolha:** Opção {chosen} — {title}

**Justificativa:** {rationale}

## 📊 Análise Contrafactual (F1.2) {gate_badge}

**Gate executado:** {yes/no}
**Recomendação do Gate:** {recommendation}
**Confiança do Gate:** {gate_confidence}/1.0

| Alternativa | Risco | Custo | Tempo | Conhecimento | Reversibilidade |
|-------------|:-----:|:-----:|:-----:|:------------:|:---------------:|
| A | {a_risk} | {a_cost} | {a_time} | {a_knowledge} | {a_reversibility} |
| ¬A | {na_risk} | {na_cost} | {na_time} | {na_knowledge} | {na_reversibility} |
| C | {c_risk} | {c_cost} | {c_time} | {c_knowledge} | {c_reversibility} |

## 📈 Resultado

| Campo | Valor |
|-------|-------|
| **Resultado** | {outcome} 🟢/🟡/🔴 |
| **Validado por** | {validator} |
| **Data da validação** | {validation_date} |
| **Evidência** | {outcome_evidence} |

## 🏗️ Engineering Score (F7.3)

| Dimensão | Score |
|----------|:-----:|
| Architecture | {arch_score} |
| Code Quality | {code_score} |
| Testing | {test_score} |
| Security | {sec_score} |
| Performance | {perf_score} |
| Documentation | {doc_score} |
| **Total** | **{total_score}** 🟢 |

## 🔗 Evidências

{evidence_links}

## 📜 Commits Relacionados

{commits}

## 🧠 Aprendizados

{learnings}

## 📋 Trust Registry (F7.2)

| Agente | Taxa de Sucesso | Decisões Similares |
|--------|:---------------:|:------------------:|
| {agent} | {success_rate} | {similar_count} |

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 **Confiança do Replay:** {replay_confidence} {badge}
📚 **Fontes consultadas:** {sources_count}
⚠️ **Fontes não encontradas:** {missing_count} {warning}

💡 **Ações:**
  👍 Confirmar | 🔄 Reabrir decisão | 🔍 Replay de outra | 💾 Exportar
```

### Badges de Status

| Badge | Significado | Cor |
|-------|-------------|-----|
| `🟢` | Sucesso / Alta confiança | Verde |
| `🟡` | Parcial / Média confiança | Amarelo |
| `🔴` | Falha / Baixa confiança | Vermelho |
| `⚪` | Pendente / Não disponível | Cinza |
| `⚠️` | Aviso / Fontes faltando | Alerta |

---

## 4. Integração com DDNA (F1.1)

O DDNA é a **fonte primária** do Decision Replay Engine. Sem DDNA, não há replay possível.

### Mapeamento DDNA → Replay

| Campo DDNA | Seção do Replay | Como é usado |
|------------|-----------------|--------------|
| `id` | 🆔 DDNA-ID | Identificador canônico do replay |
| `title` | Título do replay | Título principal |
| `date` | 📅 Data | Linha do tempo |
| `agents` | 👤 Agentes | Quem participou |
| `domain` | 🎯 Domínio | Categorização |
| `confidence` | 📊 Confidence | Badge de confiança original |
| `revisit` | 📅 Revisitar | Próxima revisão sugerida |
| `status` | 🔁 Status | Se foi aceita, deprecated, etc. |
| `## Context` | 🧠 Contexto | Reconstrução do estado anterior |
| `## Options` | 🔄 Opções | Alternativas consideradas |
| `## Decision` | ✅ Decisão | Escolha final e justificativa |
| `## Consequences` | 📈 Resultado | Positivas, negativas, mitigações |
| `## Evidence` | 🔗 Evidências | Links para fontes |
| `## Outcome Validation` | 📈 Resultado | Validação pós-decisão |

### Regras de Resolução DDNA

1. **Busca por ID**: `DDNA-YYYY-MM-DD-NNN` → resolução direta (mais rápido)
2. **Busca por título/tags**: grep por palavras-chave nos DDNAs → pode retornar múltiplos resultados
3. **Busca por agente**: grep por `agents:` nos DDNAs → útil para "mostre todas as decisões do cosca-kernel"
4. **DDNA não encontrado**: engine tenta fontes secundárias (Impact Reports, git log) e reporta replay parcial com baixa confiança

### Cache de DDNAs

Para replays frequentes, o engine mantém um cache dos DDNAs mais recentes:

```yaml
ddna_cache:
  max_entries: 50
  ttl_hours: 24
  storage: memory/replays/ddna-cache.yaml
```

---

## 5. Integração com Contrafactual Gate (F1.2)

O Contrafactual Gate é a **fonte de análise de alternativas** do replay. Se uma decisão passou pelo Gate, o replay mostra não apenas o que foi escolhido, mas a matriz completa de comparação.

### Quando o Gate está disponível

Se o DDNA tiver `decision_id` referenciando um Contrafactual Gate (`cfg-*`), o replay inclui automaticamente a seção **📊 Análise Contrafactual** com:

1. **Matriz de comparação 5 dimensões** (risco, custo, tempo, conhecimento, reversibilidade)
2. **Recomendação do Gate** (proceed/escalate/reject)
3. **Confiança do Gate** na recomendação
4. **Premissas desafiadas** (se o Gate identificou)
5. **Cenários extremos** (se disponíveis)

### Mapeamento Gate → Replay

| Campo do Gate (YAML) | Seção do Replay |
|----------------------|-----------------|
| `contrafactual_gate.recommendation.outcome` | Recomendação do Gate |
| `contrafactual_gate.recommendation.confidence` | Confiança do Gate |
| `contrafactual_gate.alternatives[].comparison` | Matriz de comparação |
| `contrafactual_gate.assumptions_challenged` | Premissas desafiadas |
| `contrafactual_gate.extreme_scenarios` | Cenários extremos |
| `contrafactual_gate.final_decision` | Decisão final vs recomendação |

### Gate não encontrado

Se a decisão não passou pelo Contrafactual Gate (era P2/P3 ou foi fast-track), o replay exibe:

```markdown
## 📊 Análise Contrafactual (F1.2) ⚪

**Gate não executado** — Esta decisão não passou pelo Contrafactual Gate.
Motivo possível: {reason (P2/P3, fast-track, pré-F1.2)}

> Decisões anteriores à F1.2 podem não ter análise contrafactual.
> Considere reabrir a decisão para executar o Gate retroativamente.
```

### Reabrir Decisão (Passo 5)

Quando o Don escolhe **reabrir**, o engine:

1. Carrega o DDNA original como entrada
2. Aciona o **Contrafactual Gate** com o contexto atual
3. O Gate gera NOVAS alternativas (pode incluir opções que não existiam na época)
4. Don decide novamente
5. Novo DDNA é criado com `supersedes: DDNA-ORIGINAL`

---

## 6. Integração com Trust Registry (F7.2)

O Trust Registry fornece a **reputação histórica** dos agentes envolvidos na decisão, permitindo ao replay mostrar não apenas o que foi decidido, mas **quem decidiu e qual o histórico desse agente**.

### Dados do Trust Registry no Replay

Quando disponível, o replay inclui na seção **📋 Trust Registry**:

```yaml
trust_registry_snapshot:
  agent: "cosca-architecture"
  at_time_of_decision:
    total_decisions: 12
    success_rate: 0.83
    domain_strength:
      architecture: 0.92
      security: 0.75
      database: 0.88
    bias_profile:
      confirmation_bias: 0.3
      recency_bias: 0.2
    last_outcome: "success"
```

### Consulta Temporal

O engine consulta o Trust Registry **no momento da decisão**, não no momento do replay. Isso evita o viés de hindsight — o replay mostra a reputação do agente COMO ERA NA DATA da decisão, não como está hoje.

```markdown
📋 **Trust Registry no momento da decisão (2026-07-30):**
  - cosca-kernel: 92% sucesso em decisões similares (8 decisões)
  - cosca-architecture: 83% sucesso em decisões de arquitetura (12 decisões)
```

### Trust Registry Indisponível

Se o Trust Registry não tinha dados suficientes na época da decisão:

```markdown
📋 **Trust Registry no momento da decisão (2026-07-28):**
  ⚠️ Dados insuficientes — agente "cosca-kernel" tinha apenas 2 registros
  no Trust Registry nesta data. Confidence pode não refletir capacidade real.
```

---

## 7. Integração com memorize-commit (F7.4)

O workflow **memorize-commit** é a **fonte de rastreabilidade pós-decisão** do replay. Cada Impact Report gerado pelo memorize-commit é um ponto de replay em potencial.

### Fluxo de Integração

```
┌─────────────────────────────────────────────────────────────────────┐
│                  CICLO COMPLETO: DECISÃO → REPLAY                    │
│                                                                     │
│  1. DECISÃO É TOMADA                                                 │
│     ├── DDNA criado (F1.1)                                          │
│     └── (opcional) Contrafactual Gate executado (F1.2)              │
│                                                                     │
│  2. CÓDIGO É ESCRITO E COMMITADO                                    │
│     ├── git commit                                                   │
│     └── memorize-commit (F7.4) gera Impact Report                   │
│         ├── Engineering Score calculado                              │
│         ├── Learnings extraídos                                     │
│         └── Trust Registry atualizado                               │
│                                                                     │
│  3. DDNA + IMPACT REPORT LINKADOS                                   │
│     ├── DDNA.evidence → Impact Report (commit hash)                 │
│     └── Impact Report → DDNA (ddna_id no contexto do commit)        │
│                                                                     │
│  4. DON PERGUNTA: "Por que X?"                                      │
│     └── Decision Replay Engine (F7.5)                               │
│         ├── Busca DDNA → encontra decisão                           │
│         ├── Busca Impact Report → encontra resultado                │
│         ├── Busca Trust Registry → reputação dos agentes            │
│         └── REPLAY COMPLETO APRESENTADO                             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Linkagem no Impact Report

Cada Impact Report (F7.4) DEVE referenciar o DDNA correspondente quando aplicável:

```markdown
## Impact Report — a1b2c3d

### Decisões Relacionadas
- **DDNA-2026-07-30-001**: Remoção dos 10 chat.go (dead code)
  - Decisão: Opção C — remoção completa
  - Contexto: go list -deps revelou 7 provedores não-linkados
```

### Geração Automática de Pontos de Replay

Todo Impact Report gerado pelo memorize-commit é automaticamente registrado como um ponto de replay elegível:

```yaml
replay_points:
  - source: "impact-report"
    path: "memory/timeline/impact-reports/a1b2c3d.md"
    date: "2026-07-30"
    type: "commit"
    linked_ddna: "DDNA-2026-07-30-001"
    linked_agents: ["cosca-kernel", "cosca-technical-debt"]
```

---

## 8. Interface CLI

O Decision Replay Engine expõe os seguintes comandos CLI via Cobra:

### Comando Principal

```bash
cosca decision replay <query> [flags]
```

### Subcomandos

| Comando | Descrição | Exemplo |
|---------|-----------|---------|
| `cosca decision replay <id>` | Replay por ID de decisão | `cosca decision replay DDNA-2026-07-30-001` |
| `cosca decision replay search <term>` | Buscar decisões por termo | `cosca decision replay search "bubblewrap"` |
| `cosca decision replay date <YYYY-MM-DD>` | Replay de decisões em uma data | `cosca decision replay date 2026-07-29` |
| `cosca decision replay agent <name>` | Replay de decisões de um agente | `cosca decision replay agent cosca-kernel` |
| `cosca decision replay recent [N]` | Últimas N decisões | `cosca decision replay recent 5` |
| `cosca decision replay list [flags]` | Listar decisões replayáveis | `cosca decision replay list --domain architecture` |
| `cosca decision replay export <id>` | Exportar replay para arquivo | `cosca decision replay export DDNA-2026-07-30-001 --output replay.md` |

### Flags

| Flag | Descrição | Default |
|------|-----------|---------|
| `--output`, `-o` | Caminho para salvar o replay (stdout se vazio) | `""` (stdout) |
| `--format`, `-f` | Formato: `terminal` (colorido) ou `markdown` (raw) | `terminal` |
| `--include-gate` | Incluir análise contrafactual mesmo se não encontrada | `false` |
| `--include-trust` | Incluir dados do Trust Registry | `true` |
| `--verbose`, `-v` | Mostrar fontes consultadas e não encontradas | `false` |
| `--json` | Output em JSON (para parsing programático) | `false` |
| `--interactive`, `-i` | Modo interativo (pergunta → resposta → pergunta) | `false` |

### Exemplos de Uso

```bash
# Replay por ID
cosca decision replay DDNA-2026-07-30-001

# Pergunta em linguagem natural (modo interativo)
cosca decision replay -i
> Por que escolhemos Bubblewrap em vez de Docker?
# ... engine mostra o replay ...
> E a decisão sobre memfd_create?
# ... engine mostra o próximo replay ...

# Buscar e listar
cosca decision replay list --domain security --limit 10

# Exportar para arquivo
cosca decision replay export DDNA-2026-07-30-001 -o docs/replays/dead-code-removal.md

# Últimas decisões
cosca decision replay recent 3 --verbose

# JSON para consumo por ferramentas
cosca decision replay DDNA-2026-07-30-001 --json | jq '.decision.rationale'
```

---

## 9. Exemplo: Remoção dos 10 chat.go

> **Decisão real da sessão 2026-07-30**. DDNA-2026-07-30-001 documenta a remoção de 15.134 linhas de dead code.

### Query

```
Don: "Por que removemos os 10 chat.go?"
```

### Output do Replay

```markdown
╔══════════════════════════════════════════════════════════════════════════╗
║                     DECISION REPLAY                                     ║
║                     F7.5 — Decision Replay Engine                       ║
╚══════════════════════════════════════════════════════════════════════════╝

🆔 DDNA-ID: DDNA-2026-07-30-001
📅 Data: 2026-07-30
👤 Agentes: cosca-kernel, cosca-technical-debt, cosca-architecture
🎯 Domínio: backend
🏷️ Tags: #dna #dead-code #providers #chat #refactoring #go-list-deps
📊 Confidence: 0.92/1.0
📅 Revisitar: 2026-10-30
🔁 Status: accepted

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 🧠 Contexto

O Cosca foi construído com 10 arquivos `chat.go` que implementavam a
interface `ChatStream` para diferentes provedores de IA (anthropic, openai,
azure, bedrock, google, ollama, deepseek, groq, mistral, etc.).

Durante uma auditoria de dívida técnica conduzida pelo cosca-technical-debt,
foi descoberto que a MAIORIA desses arquivos compilava mas nunca era linkada
no binário final. A ferramenta `go build ./...` não detecta o problema — ela
verifica compilação, não linkagem. Apenas `go list -deps` revelou a verdade:
dos 10 provedores, apenas deepseek, groq e mistral eram efetivamente
importados via `openaicompat` pattern.

Os outros 7 provedores eram código morto que:
1. Compilava em toda build (custo adicional de CI)
2. Precisava ser mantido (atualizações de API, security patches)
3. Gerava testes órfãos (9 `chat_test.go` testando código morto)
4. Aumentava a superfície de código sem benefício (15.134 linhas)

## 🔄 Opções Consideradas

### Opção A: Migrar todos para openaicompat pattern

**Descrição:** Migrar os 7 provedores legacy para o pattern unificado.

| Critério | Avaliação |
|----------|-----------|
| Prós | Todos os provedores via pattern unificado; código mais limpo |
| Contras | Esforço alto (7 provedores × ~200 linhas); APIs incompatíveis |
| Evidências | deepseek/groq/mistral já funcionam com openaicompat (83 linhas) |
| Esforço | L (semanas) |
| Risco | médio — risco de quebrar compatibilidade existente |

### Opção B: Manter tudo como está

**Descrição:** Aceitar o dead code como dívida técnica.

| Critério | Avaliação |
|----------|-----------|
| Prós | Zero esforço imediato |
| Contras | Dead code continua; testes órfãos permanecem; custo contínuo |
| Evidências | Nenhuma — status quo problemático |
| Esforço | Zero (mas custo contínuo) |
| Risco | médio — dead code pode conter vulnerabilidades não detectadas |

### Opção C: Remover dead code e testes órfãos (★ ESCOLHIDA)

**Descrição:** Remover 10 `chat.go` + 9 `chat_test.go` = 15.134 linhas.

| Critério | Avaliação |
|----------|-----------|
| Prós | -15.134 linhas; build mais limpo; CI mais rápido; manutenção zero |
| Contras | Perde código legacy; provedores precisarão ser reimplementados |
| Evidências | `go list -deps` comprovou não-linkagem; testes testavam dead code |
| Esforço | M (horas) |
| Risco | baixo — código não-linkado não afeta runtime; reversível via git |

## ✅ Decisão

**Escolha:** Opção C — Remover dead code e testes órfãos

**Justificativa:** O dead code foi comprovado via `go list -deps`, não por
suposição. A remoção em cascata (código → testes) é o padrão correto: ao
remover os 10 `chat.go`, os 9 `chat_test.go` que testavam exclusivamente
código removido também devem ser eliminados. A Opção A seria ideal
arquiteturalmente mas o esforço não se justifica para provedores sem demanda.
A Opção C é a mais segura, rápida e de menor risco.

## 📊 Análise Contrafactual (F1.2) ⚪

**Gate não executado** — Esta decisão foi tomada antes da F1.2 ser
implementada. Não há análise contrafactual formal.

> Decisões anteriores à F1.2 (2026-07-30) podem não ter análise
> contrafactual. Considere reabrir a decisão para executar o Gate.

## 📈 Resultado

| Campo | Valor |
|-------|-------|
| **Resultado** | success 🟢 |
| **Validado por** | cosca-kernel (validado por Don) |
| **Data da validação** | 2026-07-30 |
| **Evidência** | `go list -deps` confirma chat.go não-linkado; `git diff --stat` mostra -15.134 linhas; `go build ./...` passa limpo |

## 🏗️ Engineering Score (F7.3)

| Dimensão | Score |
|----------|:-----:|
| Architecture | 85 |
| Code Quality | 90 |
| Testing | 75 |
| Security | 70 |
| Performance | 80 |
| Documentation | 60 |
| **Total** | **79.5** 🟢 |

## 🔗 Evidências

- [cosca-kernel learnings.md L565](../../memory/agent/cosca-kernel/learnings.md)
- [cosca-technical-debt learnings.md L67](../../memory/agent/cosca-technical-debt/learnings.md)
- [Heurística H-004](../../knowledge/heuristics/H-004-extract-to-test.yaml)
- [QUALITY_GATES.md §2.2](../../QUALITY_GATES.md)
- [CONSTITUTION.md §P2](../../CONSTITUTION.md)

## 📜 Commits Relacionados

| Hash | Mensagem |
|------|----------|
| `a1b2c3d` | Remove dead code: 10 chat.go + 9 chat_test.go (-15.134 lines) |

## 🧠 Aprendizados

1. **go list -deps detecta dead code em Go, não go build ./...**
   → Ferramenta canônica para detecção de dead code
2. **chat_test.go órfãos após remoção de chat.go**
   → Remoção em cascata (código → testes) é o padrão correto

## 📋 Trust Registry (F7.2)

| Agente | Taxa de Sucesso | Decisões Similares |
|--------|:---------------:|:------------------:|
| cosca-kernel | 92% | 8 decisões |
| cosca-technical-debt | 88% | 5 decisões |
| cosca-architecture | 83% | 12 decisões |

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 **Confiança do Replay:** 0.95 🟢
📚 **Fontes consultadas:** 5/5
⚠️ **Fontes não encontradas:** 0

💡 **Ações:**
  👍 Confirmar | 🔄 Reabrir decisão | 🔍 Replay de outra | 💾 Exportar
```

---

## 10. Exemplo com Decisão Real Desta Sessão

> **Decisão**: Criação do Decision Replay Engine (F7.5) — esta mesma decisão que você está lendo agora será replayável no futuro.

### Contexto Real

Durante a sessão de 2026-07-30, após completar F0 (Fundação), F1 (Memória e Decisão), F2.1 (Cognitive Economy), e F7 (Prediction + Trust + Score), o próximo item no roadmap era **F7.5 — Decision Replay**. O aprendizado L31 do cosca-kernel registra:

> "(5) F7.4 — Impact Report Automation (git hook). (6) F7.5 — Decision Replay."

### DDNA Correspondente

Será criado junto com este engine:

```yaml
---
id: DDNA-2026-07-30-003
title: "Criação do Decision Replay Engine (F7.5)"
status: accepted
date: 2026-07-30
agents:
  - cosca-architecture
  - cosca-kernel
domain: architecture
decision_level: 3
confidence: 0.90
revisit: 2026-10-30
tags:
  - dna
  - decision-replay
  - f7.5
  - engineering-intelligence
  - replay
  - rastreabilidade
supersedes:
superseded_by:
---

## Context

Após completar F0 (Fundação), F1 (Memória e Decisão — DDNA, Contrafactual Gate,
Gap Detection, Wisdom Decay, Metrics, Entropy), F2.1 (Cognitive Economy), e
F7 (Prediction Engine, Trust Registry, Engineering Score), o roadmap apontava
para F7.5 — Decision Replay como o próximo componente.

O problema: até então, decisões eram documentadas (DDNA), questionadas (Gate),
e medidas (Score), mas não havia um mecanismo para REPRODUZIR uma decisão
passada com contexto completo. O Don não tinha como perguntar "Por que
escolhemos X?" e receber uma resposta estruturada com todas as fontes.

## Options

### Opção A: Engine dedicado de replay (★ DECISÃO)

**Descrição:** Criar um engine autônomo (decision-replay/SKILL.md) com
pipeline de 5 etapas, formato de output canônico, e integração com todos
os componentes existentes (DDNA, Gate, Trust, Impact Reports).

| Critério | Avaliação |
|----------|-----------|
| Prós | Componente coeso e rastreável; pipeline claro; integra todas as fontes |
| Contras | Requer criação de novo diretório e skill |
| Evidências | Prediction Engine (F7.1) já estabeleceu o padrão de engine skill |
| Esforço | M (horas) |
| Risco | baixo — é puramente documentação + integração |

### Opção B: Estender Prediction Engine

**Descrição:** Adicionar funcionalidade de replay ao Prediction Engine (F7.1)
existente, em vez de criar um engine separado.

| Critério | Avaliação |
|----------|-----------|
| Prós | Reutiliza engine existente; menos arquivos |
| Contras | Mistura responsabilidades (predição ≠ replay); Prediction Engine já tem 1441 linhas |
| Evidências | SKILL_TEMPLATE.md recomenda engines com responsabilidade única |
| Esforço | S (ajustes) |
| Risco | médio — acoplamento indevido entre predição e retrospectiva |

### Opção C: CLI-only sem engine dedicado

**Descrição:** Implementar apenas como comando CLI (cosca decision replay)
sem um skill/engine dedicado.

| Critério | Avaliação |
|----------|-----------|
| Prós | Mínimo artefato; começa pequeno |
| Contras | Sem documentação do pipeline; sem formato canônico; sem rastreabilidade do próprio replay |
| Evidências | Todos os outros componentes F7 têm engines dedicados |
| Esforço | XS (comando apenas) |
| Risco | médio — replay não documentado não é replayável |

## Decision

**Decisão final:** Opção A — Engine dedicado de replay.

**Justificativa:** O Decision Replay é o fechamento do ciclo de
rastreabilidade. Ter um engine dedicado com pipeline documentado, formato
de output canônico, e integração explícita com todos os componentes
existentes (DDNA, Gate, Trust, Impact Reports) garante que o replay seja
tão rigoroso quanto a decisão original. A Opção B criaria acoplamento
indevido. A Opção C deixaria o replay sem rastro — uma ironia para um
componente cujo propósito é justamente criar rastro.

## Consequences

### Positivas
- Ciclo de rastreabilidade fechado: decisão → documentação → replay
- Pipeline claro (5 etapas) documentado e reproduzível
- Integração com DDNA, Gate, Trust, Impact Reports, CLI
- Formato canônico de output para consumo humano e programático
- Aprendizado L31 do cosca-kernel tem seu próximo item concluído

### Negativas
- Mais um engine para manter (50 engines no total)
- Depende de DDNA e Impact Reports estarem corretos para replay fiel

### Mitigações
- O SKILL.md do engine é auto-documentado — manutenção é parte do próprio código
- Replays com fontes faltando exibem aviso explícito e confiança reduzida

## Evidence

- [cosca-kernel learnings.md L31 — F7.5 é o próximo passo](../../memory/agent/cosca-kernel/learnings.md#L599)
- [DECISION_DNA.md — Formato canônico de decisões (F1.1)](../../knowledge/architecture/DECISION_DNA.md)
- [contrafactual-gate.md — Motor de alternativas (F1.2)](../../workflows/contrafactual-gate.md)
- [TRUST_REGISTRY.md — Reputação de agentes (F7.2)](../../memory/trust/TRUST_REGISTRY.md)
- [memorize-commit.md — Impact Reports (F7.4)](../../workflows/memorize-commit.md)
- [SKILL_TEMPLATE.md — Template de engine skill](../../SKILL_TEMPLATE.md)

## Outcome Validation

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success |
| **Data da validação** | 2026-07-30 |
| **Validador** | cosca-architecture |
| **Evidência do resultado** | SKILL.md criado em `internal/embed/cosca/engines/decision-replay/SKILL.md` com pipeline, formato, integrações e exemplos |
```

### Replay Simulado

```bash
$ cosca decision replay "Por que criamos o Decision Replay Engine?"

╔══════════════════════════════════════════════════════════════════════════╗
║                     DECISION REPLAY                                     ║
║                     F7.5 — Decision Replay Engine                       ║
╚══════════════════════════════════════════════════════════════════════════╝

🆔 DDNA-ID: DDNA-2026-07-30-003
📅 Data: 2026-07-30
👤 Agentes: cosca-architecture, cosca-kernel
🎯 Domínio: architecture
🏷️ Tags: #dna #decision-replay #f7.5 #engineering-intelligence
📊 Confidence: 0.90/1.0
🔁 Status: accepted

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 🧠 Contexto

Após completar F0, F1, F2.1, e F7 (Prediction, Trust, Score), o roadmap
apontava para F7.5 como próximo componente. O problema: não havia mecanismo
para REPRODUZIR decisões passadas com contexto completo.

## 🔄 Opções Consideradas

### ★ Opção A: Engine dedicado de replay (ESCOLHIDA)
- Prós: Componente coeso, pipeline claro, integração com todas as fontes
- Contras: Mais um engine para manter
- Esforço: M | Risco: baixo

### Opção B: Estender Prediction Engine
- Prós: Reutiliza engine existente
- Contras: Acoplamento indevido (predição ≠ retrospectiva)
- Esforço: S | Risco: médio

### Opção C: CLI-only sem engine
- Prós: Mínimo artefato
- Contras: Sem rastro — irônico para um componente de rastro
- Esforço: XS | Risco: médio

## ✅ Decisão

**Escolha:** Opção A — Engine dedicado de replay

**Justificativa:** O Decision Replay é o fechamento do ciclo de
rastreabilidade. Engine dedicado com pipeline documentado e integração
explícita garante replay tão rigoroso quanto a decisão original.

## 📈 Resultado

| Resultado | success 🟢 |
|-----------|------------|
| **Evidência** | SKILL.md criado em `internal/embed/cosca/engines/decision-replay/SKILL.md` |

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 **Confiança do Replay:** 0.95 🟢
📚 **Fontes consultadas:** 6/6

💡 **Ações:**
  👍 Confirmar | 🔄 Reabrir decisão | 🔍 Replay de outra | 💾 Exportar
```

---

## 11. Qualidade e Critérios de Aceite

### Critérios de Qualidade do Engine

| # | Critério | Descrição | Verificação |
|---|----------|-----------|-------------|
| **Q1** | **Replay fiel** | O replay deve refletir com precisão as fontes originais, sem inventar contexto | Comparar output com DDNA + Impact Report original |
| **Q2** | **Confiança honesta** | Se fontes estão faltando, a confiança do replay deve ser reduzida proporcionalmente | `replay_confidence = min(1.0, sources_found / sources_total)` |
| **Q3** | **Links navegáveis** | Todas as referências no replay devem ser links relativos funcionais | `grep -r '\.\./' replay | check dead links` |
| **Q4** | **Sem viés de hindsight** | O replay nunca deve incluir informação que não existia no momento da decisão | Trust Registry consultado na data da decisão, não hoje |
| **Q5** | **Parseável** | O formato de output deve ser parseável por regex e ferramentas de análise | `grep '^🆔'` extrai DDNA ID |
| **Q6** | **CLI funcional** | Todos os subcomandos devem retornar output válido | Testar cada flag |
| **Q7** | **Auto-replayável** | A decisão de criar este engine deve ser replayável por ele mesmo | `cosca decision replay DDNA-2026-07-30-003` |
| **Q8** | **Cobertura de fontes** | O engine deve consultar no mínimo 4 fontes (DDNA, Impact Report, Trust, learnings) | `sources_used.length >= 4` |

### Critérios de Aceite para Criação

- [ ] SKILL.md criado em `internal/embed/cosca/engines/decision-replay/SKILL.md`
- [ ] Pipeline de 5 etapas documentado (entender query → buscar fontes → reconstruir contexto → apresentar replay → Don decide)
- [ ] Formato de output canônico definido (Markdown estruturado)
- [ ] Integração com DDNA (F1.1) documentada
- [ ] Integração com Contrafactual Gate (F1.2) documentada
- [ ] Integração com Trust Registry (F7.2) documentada
- [ ] Integração com memorize-commit (F7.4) documentada
- [ ] CLI `cosca decision replay <id>` definida
- [ ] Exemplo com decisão real da sessão (DDNA-2026-07-30-001: remoção dos chat.go)
- [ ] Exemplo com decisão real desta criação (DDNA-2026-07-30-003: Decision Replay Engine)
- [ ] Este DDNA (DDNA-2026-07-30-003) criado em `memory/decisions/ddna/`
- [ ] Replay deste DDNA funcional via `cosca decision replay DDNA-2026-07-30-003`

---

## 12. Tratamento de Erros

| Falha | Ação | Impacto no Replay |
|-------|------|-------------------|
| DDNA não encontrado | Buscar fontes secundárias (Impact Reports, git log, learnings) | Replay parcial com confiança reduzida (≤0.50) e aviso ⚠️ |
| Múltiplos DDNAS encontrados | Listar resultados e pedir confirmação ao Don | Don escolhe qual replay exibir |
| Impact Report não encontrado | Replay sem seção de Engineering Score | Seção omitida com aviso |
| Trust Registry sem dados na época | Exibir "dados insuficientes" no lugar da seção | Seção exibida com aviso |
| Gate não executado (pré-F1.2) | Mostrar "Gate não executado" com motivo | Seção exibida com badge ⚪ |
| Query ambígua | Listar as N decisões mais prováveis e pedir confirmação | Don escolhe ou refina a query |
| Nenhuma fonte encontrada | Sugerir queries alternativas | "Nenhuma decisão encontrada para '{query}'. Tente: '{sugestão1}', '{sugestão2}'" |
| Falha de parse da query | Assumir busca textual ampla por título | Confiança reduzida (0.40) |
| CLI com ID inválido | Sugerir formato correto com exemplo | "ID inválido. Use o formato DDNA-YYYY-MM-DD-NNN. Ex: DDNA-2026-07-30-001" |
| Don reabre decisão sem mudança de contexto | Avisar que não houve mudança de contexto desde a decisão original | "Contexto não mudou desde a decisão. Deseja mesmo reabrir?" |
| Export com caminho inválido | Usar diretório default (`memory/replays/`) | Arquivo salvo em `memory/replays/{replay_id}.md` |

---

## 13. Relacionados

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **DECISION_DNA.md** | Fonte primária de decisões — formato canônico DDNA | `knowledge/architecture/DECISION_DNA.md` |
| **contrafactual-gate.md** | Fonte de análise de alternativas — F1.2/G0.5 | `workflows/contrafactual-gate.md` |
| **TRUST_REGISTRY.md** | Reputação histórica de agentes — F7.2 | `memory/trust/TRUST_REGISTRY.md` |
| **memorize-commit.md** | Geração de Impact Reports — F7.4 | `workflows/memorize-commit.md` |
| **Impact Report Template** | Template de relatório de impacto | `memory/timeline/impact-reports/TEMPLATE.md` |
| **Prediction Engine** | Engine irmão em F7 — F7.1 | `engines/prediction/SKILL.md` |
| **Engineering Score** | Score de engenharia — F7.3 | `analytics/engineering-score.md` |
| **SKILL_TEMPLATE.md** | Template usado para criar este skill | `SKILL_TEMPLATE.md` |
| **CONSTITUTION.md** | Princípios P2-P7 — rastreabilidade de decisões | `CONSTITUTION.md` |
| **QUALITY_GATES.md** | G0.5 — Contrafactual, G2.2 — Code Quality | `QUALITY_GATES.md` |
| **Cognitive Maturity** | Pipeline CMI — F1.2 | `workflows/cognitive-maturity-implementation.md` |
| **next-evolution-phases.md** | Roadmap — F7.5 como próximo passo | `knowledge/architecture/next-evolution-phases.md` |

---

## 14. HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | cosca-architecture | Criação inicial — pipeline de 5 etapas, formato de output canônico, integração com DDNA/Gate/Trust/Impact Reports, CLI `cosca decision replay`, exemplos com decisões reais da sessão |

---

> **Owner**: Architecture Chief | **Invoked by**: Don (via CLI ou pergunta direta) | **Mandatório para**: Revisão de decisões passadas | **Engine path**: `engines/decision-replay/SKILL.md`
>
> F7.5 fecha o ciclo de rastreabilidade do Cosca: decisão → documentação (F1.1) → questionamento (F1.2) → execução → registro (F7.4) → reputação (F7.2) → replay (F7.5).
>
> *"Uma decisão que não pode ser replayada é uma decisão que não foi realmente tomada."*
> — Cosca Architecture Chief, 2026-07-30
