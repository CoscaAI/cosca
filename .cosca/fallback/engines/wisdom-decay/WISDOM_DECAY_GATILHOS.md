# WISDOM DECAY GATILHOS — Knowledge Aging Triggers & Revalidation Pipeline (F9.4)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência (+0.04), Julgamento (+0.03) | **Bloco Cognitivo**: Bloco 4 — Metacognição & Governança
> **Fase CMI**: Fase 9 — Evolution | **Código**: F9.4
> **Dependências**: F1.4 Wisdom Decay | F1.6 Cognitive Entropy | F9.2 Wisdom Distillation
>
> Consulte também:
> - [WISDOM_DECAY.md](WISDOM_DECAY.md) — Freshness Score, 3 decay types, pipeline principal (F1.4)
> - [ENTROPY.md](../cognitive-entropy/ENTROPY.md) — Cognitive Entropy, stale como componente de entropia (F1.6)
> - [SKILL.md](../wisdom-distillation/SKILL.md) — Wisdom Distillation, constitutional funnel (F9.2)
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — Formato de aprendizado com campos de decaimento

---

## Sumário

1. [O que são Gatilhos de Revalidação](#1-o-que-são-gatilhos-de-revalidação)
2. [Tabela de Gatilhos](#2-tabela-de-gatilhos)
3. [Pipeline de Revalidação](#3-pipeline-de-revalidação)
4. [Scheduler Semanal](#4-scheduler-semanal)
5. [Sistema de Notificação](#5-sistema-de-notificação)
6. [Integração com F1.4 Wisdom Decay](#6-integração-com-f14-wisdom-decay)
7. [Integração com F1.6 Cognitive Entropy](#7-integração-com-f16-cognitive-entropy)
8. [Integração com F9.2 Wisdom Distillation](#8-integração-com-f92-wisdom-distillation)
9. [Don Override: Regras de Exceção](#9-don-override-regras-de-exceção)
10. [Métricas do Sistema de Gatilhos](#10-métricas-do-sistema-de-gatilhos)
11. [Exemplo com Learning Real](#11-exemplo-com-learning-real)
12. [Casos de Borda](#12-casos-de-borda)

---

## 1. O que são Gatilhos de Revalidação

### Definição

**Gatilhos de Revalidação** (F9.4) são o "despertador do conhecimento" — mecanismos que detectam quando a confiança de um learning cai abaixo de um threshold e **disparam revalidação automática**. Eles transformam o conceito passivo de "freshness score" (F1.4) em ação: "esse learning está velho, revisa ou perde o valor."

### Analogia: O Despertador do Conhecimento

```
┌──────────────────────────────────────────────────────────────────────────┐
│                   GATILHOS DE REVALIDAÇÃO — O DESPERTADOR                 │
│                                                                           │
│  Conhecimento sem revalidação é como um alarme de incêndio sem pilha:     │
│  está lá, parece funcionar, mas quando o fogo começa, não apita.          │
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  Fresco (freshness ≥ 0.70)                                     │     │
│  │  "Alarme OK — pilha nova."                                     │     │
│  │  → Sem ação                                                     │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                           │                                               │
│                           ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  Atenção (0.30 ≤ freshness < 0.70)                              │     │
│  │  "Alarme com pilha fraca — trocar em 30 dias."                  │     │
│  │  → #revalidate-by-{date}                                        │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                           │                                               │
│                           ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  Crítico (0.15 ≤ freshness < 0.30)                               │     │
│  │  "ALARME TOCANDO — fogo pode ter começado."                      │     │
│  │  → Task de revalidação IMEDIATA + Notifica Don                  │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                           │                                               │
│                           ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  Depreciado (freshness < 0.15)                                   │     │
│  │  "Prédio queimou — sinistro consumado."                          │     │
│  │  → Move para learning_history                                    │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                           │
│  ⚡ Contradição detectada:                                                 │
│  "Alarme de CO₂ e alarme de fumaça no mesmo teto — um dos dois           │
│   está com defeito. Não usar nenhum até verificar."                       │
│  → Bloqueia ambos + Escala para Don                                       │
└──────────────────────────────────────────────────────────────────────────┘
```

### Diferença do F1.4 Wisdom Decay

| Aspecto | F1.4 Wisdom Decay | F9.4 Gatilhos (este doc) |
|---------|------------------|--------------------------|
| **Propósito** | Calcular freshness, detectar decay, gerar relatório | **Agir** — disparar revalidação, criar tasks, notificar |
| **O que produz** | Freshness Score, relatório, tags | Tasks de revalidação, notificações, depreciação |
| **Pipeline** | Varredura → Cálculo → Classificação → Relatório | Gatilho → Task → Assign → Revalidação → Resolução |
| **Frequência** | A cada 50 tasks ou semanal | **Contínuo** — gatilhos disparam quando threshold é violado |
| **Output** | `decay-report-{date}.md` | Tasks no sistema + notificações no canal do Don |
| **Responsável** | Wisdom Decay Engine | **Este engine** (F9.4) — consome dados do F1.4 |

### Arquitetura dos Gatilhos

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      ARQUITETURA DE GATILHOS                              │
│                                                                           │
│  ┌─────────────────┐                                                      │
│  │  F1.4 Wisdom     │  freshness_score     ┌────────────────────────┐    │
│  │  Decay Engine    │ ───────────────────▶ │  F9.4 Gatilhos         │    │
│  │                  │                      │                         │    │
│  │  - freshness     │  contradiction_flag  │  ⏰ Gatilho Temporal    │    │
│  │  - contradictions│ ───────────────────▶ │     freshness < 0.30   │    │
│  │  - decay_type    │                      │     freshness < 0.20   │    │
│  │  - last_used     │                      │     freshness < 0.15   │    │
│  └─────────────────┘                       │                         │    │
│                                            │  ⚡ Gatilho Contradição │    │
│  ┌─────────────────┐                      │     contradição ativa   │    │
│  │  Eventos         │  external_event      │                         │    │
│  │  Externos        │ ───────────────────▶ │  🌐 Gatilho Evento      │    │
│  │  (go.mod, novas  │                      │     nova tech version   │    │
│  │   versões, etc)  │                      │     breaking change     │    │
│  └─────────────────┘                       │     stack migration     │    │
│                                            │                         │    │
│                                            │  ┌─────────────────────┐│    │
│                                            │  │ 🛠️ ACTION ENGINE    ││    │
│                                            │  │                     ││    │
│                                            │  │ 1. Cria task         ││    │
│                                            │  │ 2. Assigna agente    ││    │
│                                            │  │ 3. Notifica Don      ││    │
│                                            │  │ 4. Se revalidado →   ││    │
│                                            │  │    reset freshness   ││    │
│                                            │  │ 5. Se N dias → depre-││    │
│                                            │  │    cia automática    ││    │
│                                            │  └─────────────────────┘│    │
│                                            └────────────────────────┘    │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │  OUTPUT                                                          │    │
│  │                                                                   │    │
│  │  ├── Task de revalidação (no agent owner)                        │    │
│  │  ├── Notificação P0/P1 (Don ou log)                              │    │
│  │  ├── Depreciação → learning_history (nunca exclui)               │    │
│  │  └── Redução de entropia (stale cai → F1.6 recalculado)         │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Tabela de Gatilhos

### 2.1 Gatilhos de Freshness (Temporais)

Estes gatilhos são calculados a partir do **Freshness Score** fornecido pelo F1.4 Wisdom Decay. A verificação ocorre a cada execução do pipeline (semanal) ou sempre que um learning é consultado.

| # | Gatilho | Freshness | Prioridade | Ação Imediata | Prazo | Notifica |
|---|---------|-----------|------------|---------------|-------|----------|
| **T1** | Fresco em risco | `0.30 ≤ f < 0.70` | 🟡 P2 | Adicionar tag `#revalidate-by-{hoje+30d}` | 30 dias | Agent owner (log) |
| **T2** | Vencendo AGORA | `0.15 ≤ f < 0.30` | 🟠 P1 | Criar task de revalidação imediata | 7 dias | Agent owner + Don |
| **T3** | Estragado | `f < 0.15` | 🔴 P0 | Depreciar automaticamente | Imediato | Don urgente |
| **T4** | Nunca usado | `usage_count = 0` E `days_since_creation > 90` | 🟡 P2 | Marcar para auditoria (#never-used) | 30 dias | Agent owner (log) |
| **T5** | Esquecido | `days_since_last_use > 180` E `usage_count < 3` | 🟡 P2 | Revalidar se ainda relevante | 30 dias | Agent owner (log) |

### 2.2 Gatilhos de Contradição

| # | Gatilho | Condição | Prioridade | Ação Imediata | Notifica |
|---|---------|----------|------------|---------------|----------|
| **C1** | Contradição detectada | 2+ learnings com tags 60%+ overlap e afirmações opostas | 🔴 P1 | Bloquear ambos; criar DDNA de contradição; escalar para Don | Don urgente |
| **C2** | Contradição recorrente | Mesmo par de learnings em contradição por 2+ ciclos sem resolução | 🔴 P0 | Forçar revisão manual com deadline de 7 dias | Don + Governance Chief |
| **C3** | Contradição com learning depreciado | Um dos pares já está deprecated | 🟢 Info | Ignorar — o learning depreciado não conta mais | Log apenas |

### 2.3 Gatilhos de Evento Externo

| # | Gatilho | Evento | Prioridade | Ação | Notifica |
|---|---------|--------|------------|------|----------|
| **E1** | Nova versão de linguagem | `go.mod` ou `package.json` atualizado com versão major | 🟠 P1 | Revalidar todos learnings com tags da linguagem (#golang, #node, etc.) | Agent owner |
| **E2** | Depreciação externa | Dependência marcada como deprecated no ecossistema | 🟠 P1 | Freshness dos learnings relacionados cai 50%; revalidar | Agent owner |
| **E3** | Breaking change | Release note com `breaking` ou `BREAKING` | 🔴 P0 | Freshness de learnings da API afetada zera; revalidar AGORA | Don urgente |
| **E4** | Stack migration | ADR aprovada que define migração de stack | 🔴 P0 | Revalidar todos learnings da stack antiga; freshness -0.7 | Don + Architecture Chief |
| **E5** | Nova ADR superseding | ADR aprovada que supersedes decisão anterior | 🟠 P1 | Revalidar learnings vinculados à ADR antiga | Architecture Chief |
| **E6** | Security advisory | CVE publicado afetando dependência do projeto | 🔴 P0 | Revalidar learnings de segurança relacionados; freshness zera | Don urgente + Security Chief |

### 2.4 Gatilhos de Schedule (Auditoria Programada)

| # | Gatilho | Categoria do Learning | Intervalo | Ação | Notifica |
|---|---------|----------------------|-----------|------|----------|
| **S1** | Revisão de CRITICAL | CRITICAL | A cada 180 dias | Revalidar completamente | Agent owner + Governance Chief |
| **S2** | Revisão de STABLE | STABLE | A cada 90 dias | Revalidar completamente | Agent owner |
| **S3** | Revisão de EXPERIMENTAL | EXPERIMENTAL | A cada 30 dias | Revalidar ou promover para STABLE | Agent owner |
| **S4** | Revisão de CONSTITUTIONAL | #constitutional ou #immutable | A cada 365 dias | Revisão de relevância (não de validade — não expiram) | Don + Governance Chief |

### 2.5 Matriz de Prioridades e Prazos

```
┌────────────────┬──────────┬──────────┬──────────────┬──────────────────┐
│ Prioridade     │ Código   │ Prazo    │ Notificação  │ Exemplo           │
├────────────────┼──────────┼──────────┼──────────────┼──────────────────┤
│ 🔴 P0 — Crítico│ T3, C2,  │ Imediato │ Don notificado│ freshness < 0.15 │
│                │ E3, E4,  │ (< 24h)  │ com urgência  │ breaking change   │
│                │ E6       │          │              │ CVE publicado     │
├────────────────┼──────────┼──────────┼──────────────┼──────────────────┤
│ 🟠 P1 — Alto   │ T2, C1,  │ 7 dias   │ Don notificado│ freshness < 0.30 │
│                │ E1, E2,  │          │ (não urgente) │ contradição nova  │
│                │ E5       │          │              │ nova versão Go    │
├────────────────┼──────────┼──────────┼──────────────┼──────────────────┤
│ 🟡 P2 — Médio  │ T1, T4,  │ 30 dias  │ Agent owner   │ freshness < 0.70 │
│                │ T5, S1-  │          │ (log)         │ revisão STABLE    │
│                │ S4       │          │              │                   │
├────────────────┼──────────┼──────────┼──────────────┼──────────────────┤
│ 🟢 Info         │ C3       │ N/A      │ Log apenas    │ contradição com  │
│                │          │          │              │ deprecated        │
└────────────────┴──────────┴──────────┴──────────────┴──────────────────┘
```

### 2.6 Gatilho de Escalação (Escape Route)

Se um learning em **REVALIDATE_NOW** (T2) não for revalidado em **7 dias**, ele escala automaticamente:

```
T2 ativado → 7 dias sem revalidação → ESCALA para P0
  ├── Notificação urgente para Don
  ├── Se Don não responde em +7 dias → DEPRECIAÇÃO AUTOMÁTICA (T3)
  └── Learning vai para learning_history
```

---

## 3. Pipeline de Revalidação

### 3.1 Pipeline Completo (6 Passos)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                      PIPELINE DE REVALIDAÇÃO F9.4                              │
│                                                                                │
│   ┌──────────────────────────────────────────────────────────────────────┐    │
│   │ PASSO 1: GATILHO DISPARA                                              │    │
│   │                                                                       │    │
│   │  O gatilho é ativado por uma das 3 fontes:                           │    │
│   │  ├── F1.4 Wisdom Decay: freshness < threshold (T1-T5)                 │    │
│   │  ├── F1.4 Contradiction Decay: contradição detectada (C1-C3)          │    │
│   │  └── Evento externo: monitor de dependências/versões (E1-E6)          │    │
│   │                                                                       │    │
│   │  Saída: { learning_id, trigger_type, freshness, priority }            │    │
│   └──────────────────────────────────┬───────────────────────────────────┘    │
│                                      ▼                                        │
│   ┌──────────────────────────────────────────────────────────────────────┐    │
│   │ PASSO 2: ENGINE CRIA TASK DE REVALIDAÇÃO                             │    │
│   │                                                                       │    │
│   │  Para cada gatilho disparado, uma task é gerada:                      │    │
│   │                                                                       │    │
│   │  ```yaml                                                              │    │
│   │  task_id: REVAL-{YYYYMMDD}-{NNNN}                                     │    │
│   │  trigger: T2 (freshness < 0.30)                                       │    │
│   │  learning_id: 2026-07-28 — Onda 2 Agent Activation                   │    │
│   │  learning_agent: cosca-kernel                                         │    │
│   │  created_at: 2026-07-30T00:00:00Z                                     │    │
│   │  priority: P1                                                         │    │
│   │  deadline: 2026-08-06T00:00:00Z (7 dias)                              │    │
│   │  status: pending                                                      │    │
│   │  assigned_to: null                                                     │    │
│   │  ```                                                                  │    │
│   │                                                                       │    │
│   │  ⏱ Tempo alvo: < 100ms por task                                      │    │
│   └──────────────────────────────────┬───────────────────────────────────┘    │
│                                      ▼                                        │
│   ┌──────────────────────────────────────────────────────────────────────┐    │
│   │ PASSO 3: ASSIGNA PARA AGENTE MAIS RELEVANTE (OU DON)                │    │
│   │                                                                       │    │
│   │  A task é atribuída seguindo esta ordem de precedência:              │    │
│   │                                                                       │    │
│   │  1. Don do learning (agent owner) — se freshness > 0.15              │    │
│   │  2. Architecture Chief — se learning de domínio architecture          │    │
│   │  3. Memory Chief — se learning de domínio conhecimento/memória       │    │
│   │  4. Don — se P0 (freshness < 0.15 OU contradição ativa)              │    │
│   │  5. Qualquer agente disponível com tags compatíveis                  │    │
│   │                                                                       │    │
│   │  Regra: se o learning é de nível 4-5, assignar para agente            │    │
│   │  de nível equivalente (não delegar revisão de Level 4 para L1)       │    │
│   │                                                                       │    │
│   │  ⏱ Tempo alvo: < 50ms por task                                       │    │
│   └──────────────────────────────────┬───────────────────────────────────┘    │
│                                      ▼                                        │
│   ┌──────────────────────────────────────────────────────────────────────┐    │
│   │ PASSO 4: REVALIDAÇÃO EXECUTADA (pelo agente assignado)              │    │
│   │                                                                       │    │
│   │  O agente executa os checks de validação:                            │    │
│   │                                                                       │    │
│   │  ┌─────────────────┐                                                  │    │
│   │  │ 4a. Código      │ O código referenciado ainda existe?              │    │
│   │  │ 4b. Docs        │ A documentação referenciada é precisa?           │    │
│   │  │ 4c. Padrão      │ O padrão descrito ainda é aplicável?             │    │
│   │  │ 4d. Contexto    │ O contexto original ainda é relevante?           │    │
│   │  │ 4e. Resultado   │ O outcome ainda se sustentaria hoje?             │    │
│   │  └─────────────────┘                                                  │    │
│   │                                                                       │    │
│   │  3 resultados possíveis (ver Passo 4a/4b/4c):                         │    │
│   │                                                                       │    │
│   │  ⏱ Tempo alvo: < 30s por revalidação                                 │    │
│   └──────────────────────────────────┬───────────────────────────────────┘    │
│                                      ▼                                        │
│        ┌─────────────────────────────┼─────────────────────────────┐          │
│        ▼                             ▼                             ▼          │
│  ┌──────────────────┐       ┌──────────────────┐       ┌──────────────────┐   │
│  │ 4a. ✅ VÁLIDO    │       │ 4b. ⚠️ PARCIAL    │       │ 4c. ❌ INVÁLIDO  │   │
│  │                  │       │                  │       │                  │   │
│  │ Freshness = 1.00 │       │ Freshness = 0.85 │       │ DEPRECATED       │   │
│  │ last_validated = │       │ last_validated = │       │ freshness = 0.00 │   │
│  │ hoje             │       │ hoje             │       │ tag: #deprecated │   │
│  │ Tag: #fresh      │       │ Tag: #refreshed  │       │ superseded_by =  │   │
│  │                  │       │ Nota atualizada  │       │ {nova entrada}   │   │
│  └────────┬─────────┘       └────────┬─────────┘       └────────┬─────────┘   │
│           ▼                          ▼                          ▼             │
│      ┌──────────────────────────────────────────────────────────────────┐    │
│      │ PASSO 5: RESOLUÇÃO                                                │    │
│      │                                                                   │    │
│      │  ├── Se VÁLIDO: freshness resetado para 1.00 no F1.4             │    │
│      │  ├── Se PARCIAL: freshness setado para 0.85, entrada atualizada  │    │
│      │  └── Se INVÁLIDO: learning movido para learning_history           │    │
│      │                                                                   │    │
│      │  Em todos os casos:                                               │    │
│      │  ├── Task status = completed / deprecated                         │    │
│      │  ├── Evento registrado no audit log                              │    │
│      │  └── Don notificado (se P0) ou log (se P1/P2)                   │    │
│      └──────────────────────────────────┬────────────────────────────────┘    │
│                                         ▼                                     │
│   ┌──────────────────────────────────────────────────────────────────────┐    │
│   │ PASSO 6: LEARNING NÃO REVALIDADO EM N DIAS → DEPRECIA AUTOMÁTICA    │    │
│   │                                                                       │    │
│   │  Se uma task de revalidação fica em pending além do deadline:        │    │
│   │                                                                       │    │
│   │  ├── T1 (REVALIDATE_30): após 30 dias sem ação → escala para T2     │    │
│   │  ├── T2 (REVALIDATE_NOW): após 7 dias sem ação → escala para P0     │    │
│   │  │   └── após +7 dias sem Don response → DEPRECIAÇÃO AUTOMÁTICA     │    │
│   │  └── T3 (DEPRECATED): task fechada como "auto-deprecated"           │    │
│   │                                                                       │    │
│   │  Learning depreciado:                                                 │    │
│   │  ├── NÃO é excluído — vai para learning_history                     │    │
│   │  ├── Tag `#deprecated` + campo `deprecated_at`                       │    │
│   │  ├── Referência cruzada para substituto (se existir)                 │    │
│   │  └── F1.6 Cognitive Entropy recalcula (stale reduziu)               │    │
│   └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Detalhamento dos Resultados da Revalidação

#### 4a. Válido — Reset Total

O conhecimento ainda é preciso, o código referenciado existe, o padrão funciona.

```yaml
result: valid
actions:
  freshness: 1.00
  last_validated: today
  expires_at: today + (365 × category_multiplier) dias
  tag: "#fresh"
  note: "Revalidado em {data}: conhecimento confirmado como preciso"
  freshness_status: healthy
  task_status: completed
```

#### 4b. Parcial — Atualização Parcial

O conhecimento base ainda vale, mas detalhes mudaram.

```yaml
result: partial
actions:
  freshness: 0.85
  last_validated: today
  expires_at: today + (365 × category_multiplier × 0.5) dias
  tag: "#refreshed"
  note: "Revalidado em {data}: {o que mudou} — entrada atualizada"
  fields_updated: [next, learned, related]
  freshness_status: healthy
  task_status: completed
  promoted: false  # Se era EXPERIMENTAL → considerar promoção manual para STABLE
```

#### 4c. Inválido — Depreciação

O conhecimento não é mais verdadeiro. Foi substituído ou invalidado.

```yaml
result: invalid
actions:
  wisdom_decay_category: DEPRECATED
  freshness: 0.00
  tag: "#deprecated"
  deprecated_at: today
  superseded_by: "{timestamp da entrada substituta}"  # se existir
  note: "Invalidado em {data}: {razão técnica}"
  freshness_status: deprecated
  task_status: deprecated
  learning_history: true  # Movido para learning_history
  failure_entry:
    created: true
    reason: "{por que o conhecimento se tornou inválido}"
    lesson: "{lição aprendida}"
```

### 3.3 Pseudocódigo do Pipeline

```python
def run_revalidation_pipeline():
    """
    Pipeline principal de revalidação F9.4.
    Executado semanalmente (via scheduler) ou sob demanda.
    """
    # PASSO 1: Coletar gatilhos de todas as fontes
    triggers = []

    # Gatilhos de freshness (F1.4)
    for learning in wisdom_decay.get_all_learnings():
        if learning.freshness < 0.15:
            triggers.append(Trigger(T3, learning, priority=P0))
        elif learning.freshness < 0.30:
            triggers.append(Trigger(T2, learning, priority=P1))
        elif learning.freshness < 0.70:
            triggers.append(Trigger(T1, learning, priority=P2))

    # Gatilhos de contradição (F1.4)
    for contradiction in wisdom_decay.get_active_contradictions():
        triggers.append(Trigger(C1, contradiction, priority=P1))
        if contradiction.age_in_cycles >= 2:
            triggers.append(Trigger(C2, contradiction, priority=P0))

    # Gatilhos de evento externo
    for event in external_event_monitor.get_recent_events():
        for learning in event.affected_learnings:
            triggers.append(Trigger(event.type, learning, event.priority))

    # Gatilhos de schedule programado
    for learning in schedule_monitor.get_due_learnings():
        triggers.append(Trigger(learning.schedule_type, learning, priority=P2))

    # PASSO 2: Criar tasks (deduplicadas)
    tasks = []
    seen = set()
    for trigger in sorted(triggers, key=lambda t: t.priority):
        key = (trigger.learning.id, trigger.type)
        if key not in seen:
            seen.add(key)
            task = create_revalidation_task(trigger)
            tasks.append(task)

    # PASSO 3: Assignar tasks
    for task in tasks:
        assign_result = assign_task(task)
        if assign_result == "unassigned":
            # Se ninguém disponível, assigna para Don
            assign_to_don(task)

    # PASSO 4: Executar revalidação (para tasks automáticas)
    for task in tasks:
        if task.priority == P0:
            # P0: executa imediatamente
            result = execute_revalidation(task)
            process_result(task, result)
        elif task.priority == P1:
            # P1: agenda para execução na próxima oportunidade
            schedule_revalidation(task, deadline=7_days)
        else:
            # P2: agenda para execução em 30 dias
            schedule_revalidation(task, deadline=30_days)

    # PASSO 5: Verificar tasks vencidas
    for task in get_overdue_tasks():
        if task.days_since_creation > task.deadline_days:
            if task.priority == P2:
                escalate_to_next_level(task)  # P2 → P1
            elif task.priority == P1:
                escalate_to_don(task)         # P1 → P0 + Don
            elif task.priority == P0:
                auto_deprecate(task)          # P0 → Depreciado

    # PASSO 6: Gerar relatório
    report = generate_gatilhos_report(tasks, triggers)
    notify_don(report) if any(t.priority == P0 for t in tasks) else log(report)

    return report
```

### 3.4 Performance

| Operação | Tempo Estimado |
|----------|----------------|
| Coleta de triggers (F1.4 + eventos) | < 1s |
| Criação de tasks | < 100ms |
| Assignação | < 50ms |
| Execução de revalidação P0 (por task) | < 30s |
| Verificação de tasks vencidas | < 200ms |
| Geração de relatório | < 500ms |
| **Total** (sem execução de revalidação) | **< 2s** |

---

## 4. Scheduler Semanal

### 4.1 Execução Automática

O pipeline de gatilhos executa **semanalmente** (domingo à meia-noite), integrado ao F1.4 Wisdom Decay:

```bash
# Pipeline completo: Wisdom Decay → Gatilhos → Revalidação
0 0 * * 0 cosca wisdom-decay run && cosca wisdom-decay gatilhos run

# Ou comando único
0 0 * * 0 cosca wisdom-decay gatilhos run --auto-revalidate
```

### 4.2 Configuração do Scheduler

```yaml
# Configuração no opencode.json ou wisdom-decay config
wisdom_decay_gatilhos:
  scheduler:
    interval: weekly            # weekly | daily | manual
    day: sunday                 # day of week (se weekly)
    time: "00:00"               # hora de execução
    timezone: UTC
  
  auto_revalidate: true         # Se true, executa revalidação automática para P1/P2
                                # Se false, só cria tasks (Don decide quando executar)
  
  deadlines:
    p0_hours: 24               # P0 deve ser resolvido em 24h
    p1_days: 7                  # P1 deve ser resolvido em 7 dias
    p2_days: 30                 # P2 deve ser resolvido em 30 dias
    auto_deprecate_days: 14     # Após 14 dias em P0 sem resposta → deprecia automática
  
  notification:
    p0_channel: don_urgent      # P0 notifica Don com urgência
    p1_channel: don_digest      # P1 notifica Don no resumo semanal
    p2_channel: agent_log       # P2 só registra no log do agente
  
  exceptions:
    immutable_tags:             # Learnings com estas tags NUNCA são depreciados
      - "#constitutional"
      - "#immutable"
      - "#don-override-keep"
    max_revalidations_per_run: 50  # Máximo de revalidações por execução
```

### 4.3 Gatilhos Adicionais de Execução

Além do scheduler semanal, o pipeline pode ser disparado por:

| Gatilho | Disparado por | Quando | Ação |
|---------|--------------|--------|------|
| **Pós Wisdom Decay** | F1.4 pipeline completo | Após cada execução do F1.4 | Verificar novos thresholds violados |
| **Pós-task** | Task executada (contador) | A cada 50 tasks | Verificar se learnings usados na task precisam de revalidação |
| **Evento externo** | Monitor de dependências | Imediato | Disparar gatilhos E1-E6 |
| **Manual** | CLI `cosca wisdom-decay gatilhos run` | On-demand | Pipeline completo |
| **Manual (single)** | CLI `cosca wisdom-decay gatilhos run --learning <id>` | On-demand | Revalidar learning específico |

### 4.4 Comandos CLI

```bash
# Executar pipeline completo de gatilhos
cosca wisdom-decay gatilhos run

# Executar em dry-run (não cria tasks, não notifica)
cosca wisdom-decay gatilhos run --dry-run

# Executar para learning específico
cosca wisdom-decay gatilhos run --learning 2026-07-28

# Executar para agente específico
cosca wisdom-decay gatilhos run --agent cosca-kernel

# Ver gatilhos ativos (tasks pendentes)
cosca wisdom-decay gatilhos list

# Ver tasks de revalidação pendentes
cosca wisdom-decay gatilhos tasks --status pending

# Executar revalidação manual de um learning
cosca wisdom-decay gatilhos revalidate <learning-id>

# Forçar depreciação de um learning
cosca wisdom-decay gatilhos deprecate <learning-id> --reason "..."

# Ver relatório de gatilhos da última execução
cosca wisdom-decay gatilhos report

# Configurar scheduler
cosca wisdom-decay gatilhos schedule --interval weekly
```

---

## 5. Sistema de Notificação

### 5.1 Matriz de Notificação

| Prioridade | Canal | Formato | Frequência | Responsável |
|------------|-------|---------|------------|-------------|
| **🔴 P0** | Don (notificação direta) | Mensagem urgente com learning_id + trigger + ação esperada | Imediato (minutos) | Engine F9.4 |
| **🟠 P1** | Don (resumo semanal) | Relatório no `decay-report-{date}.md` | Semanal | Engine F9.4 |
| **🟡 P2** | Agent owner (log) | Entrada no log do agente + tag no learning | Assíncrono (próxima task do agente) | Engine F9.4 |
| **🟢 Info** | Log do sistema | Linha no log geral | Quando ocorrer | Engine F9.4 |

### 5.2 Formato de Notificação P0 (Don Urgente)

```markdown
# 🚨 REVALIDAÇÃO URGENTE — P0

**Gatilho**: T3 — Freshness < 0.15 (DEPRECATED)
**Learning**: 2026-07-28 — Onda 2: 10-Agent Parallel Activation
**Agent**: cosca-kernel
**Freshness**: 0.12 (decaído de 0.72 em 90 dias sem uso)
**Último uso**: 2026-04-30 (182 dias)
**Tags**: #onda-2 #agent-activation #orchestration

## Ação esperada
1. Revisar se este learning ainda é relevante
2. Se sim → revalidar (freshness volta a 1.00)
3. Se não → confirmar depreciação (vai para learning_history)

## Consequência da inação
Este learning será DEPRECIADO AUTOMATICAMENTE em 14 dias.

## Ações rápidas
- `cosca wisdom-decay gatilhos revalidate 2026-07-28` — revalidar agora
- `cosca wisdom-decay gatilhos deprecate 2026-07-28 --reason "..."` — depreciar
- `cosca wisdom-decay gatilhos override 2026-07-28 --keep --reason "..."` — manter mesmo que velho
```

### 5.3 Formato de Notificação P1 (Resumo Semanal)

Incluído no relatório semanal de conhecimento em risco (integrado ao `decay-report-{date}.md`):

```markdown
## ⏰ Gatilhos de Revalidação Ativos

### 🟠 P1 — Revalidar em 7 dias

| Learning | Agent | Freshness | Trigger | Deadline |
|----------|-------|-----------|---------|----------|
| 2026-07-28 — Onda 2 | cosca-kernel | 0.22 | T2 | 2026-08-06 |
| 2026-07-29 — Token Bloat | cosca-kernel | 0.28 | T2 | 2026-08-06 |

### 🟡 P2 — Revalidar em 30 dias

| Learning | Agent | Freshness | Trigger | Deadline |
|----------|-------|-----------|---------|----------|
| 2026-07-27 — Baseline | cosca-memory-chief | 0.45 | T1 | 2026-08-29 |
| 2026-07-28 — CI Fix | cosca-kernel | 0.52 | T1 | 2026-08-29 |
```

### 5.4 Regras de Notificação

| Regra | Descrição |
|-------|-----------|
| **Sem spam** | Um mesmo learning só gera notificação P0/P1 uma vez por ciclo. Re-notifica apenas se escalonou de prioridade. |
| **Agrupamento P1** | Todas as notificações P1 da semana são agrupadas em um único relatório (domingo). |
| **P0 não agrupa** | P0 é notificado individualmente e imediatamente — não espera o relatório semanal. |
| **Silêncio para imutáveis** | Learnings `#constitutional` ou `#immutable` nunca geram notificação P0/P1 (regra de Don override). |
| **Don pode silenciar** | Don pode silenciar notificações de um learning específico com `--silence 90d`. |

---

## 6. Integração com F1.4 Wisdom Decay

### 6.1 Contrato de Interface

A relação entre F9.4 e F1.4 é de **consumo**: F9.4 lê dados do F1.4 e age sobre eles.

```
┌────────────────────────────┐     ┌──────────────────────────────┐
│  F1.4 Wisdom Decay Engine  │     │  F9.4 Gatilhos Engine         │
│                            │     │                              │
│  Produz:                    │     │  Consome:                    │
│  ├── freshness_score        │────▶│  ├── freshness < threshold  │
│  ├── decay_type             │     │  ├── contradições ativas    │
│  ├── contradictions         │────▶│  ├── stale list             │
│  ├── stale_learnings        │     │  └── last_used + usage_count│
│  └── last_used / usage_count│     │                              │
│                            │     │  Produz:                     │
│  Consome:                   │     │  ├── freshness reset (1.00) │
│  ├── freshness reset        │◀────│  ├── depreciação            │
│  └── depreciação            │     │  └── revalidação            │
└────────────────────────────┘     └──────────────────────────────┘
```

### 6.2 Campos Utilizados do F1.4

| Campo F1.4 | Tipo | Uso no Gatilho |
|------------|------|----------------|
| `freshness_score` | float (0.0-1.0) | Comparar com thresholds T1/T2/T3 |
| `decay_type` | enum | Identificar se é time/event/contradiction decay |
| `contradictions` | list | Gatilhos C1/C2/C3 |
| `stale_learnings` | list | Learnings com freshness < 0.30 |
| `last_used` | date | Gatilho T5 (esquecido) |
| `usage_count` | int | Gatilho T4 (nunca usado) |
| `last_review` | date | Gatilhos S1-S4 (schedule) |
| `decay_category` | enum | CRITICAL/STABLE/EXPERIMENTAL — define scheduler S1-S3 |

### 6.3 Ações de Retorno no F1.4

Quando o F9.4 executa uma revalidação, ele **escreve de volta** no F1.4:

| Ação F9.4 | Efeito no F1.4 |
|-----------|----------------|
| Revalidado (válido) | `freshness = 1.00`, `last_validated = hoje`, `decay_type = healthy` |
| Revalidado (parcial) | `freshness = 0.85`, `last_validated = hoje` |
| Depreciado | `freshness = 0.00`, `decay_type = deprecated`, tag `#deprecated` |
| Don override (keep) | `freshness` congelado no valor atual, tag `#don-override-keep` |
| Revalidado por evento | `freshness` recalculado com base no novo contexto |

### 6.4 Pipeline Integrado

```bash
# Pipeline completo (F1.4 + F9.4)
cosca wisdom-decay run && cosca wisdom-decay gatilhos run

# Equivalente a:
# 1. F1.4: Varre learnings, calcula freshness, detecta contradições
# 2. F1.4: Gera relatório de conhecimento em risco
# 3. F9.4: Verifica thresholds violados
# 4. F9.4: Cria tasks de revalidação
# 5. F9.4: Assigna tasks para agentes
# 6. F9.4: Notifica Don (se P0) ou registra (se P1/P2)
```

---

## 7. Integração com F1.6 Cognitive Entropy

### 7.1 Efeito Direto: Depreciação Reduz Entropia

Quando um learning é depreciado pelo F9.4, ele **para de contribuir** para a entropia cognitiva:

```
Antes da depreciação:
  stale_learnings = 3 (incluindo este learning)
  entropia = (contradictions×3 + stale×2 + gaps×1) / total_knowledge

Após depreciação:
  stale_learnings = 2 (learning depreciado não conta mais como stale)
  entropia = (contradictions×3 + stale×2 + gaps×1) / total_knowledge
  ↓ entropia reduzida
```

### 7.2 Regras de Contribuição

| Estado do Learning | Contribui para stale? | Contribui para contradição? | Na entropia |
|-------------------|----------------------|---------------------------|-------------|
| 🟢 HEALTHY (f ≥ 0.70) | ❌ | ❌ | Não conta |
| 🟡 REVALIDATE_30 (0.30 ≤ f < 0.70) | ❌ | ❌ | Não conta (ainda não stale) |
| 🟠 REVALIDATE_NOW (0.15 ≤ f < 0.30) | ✅ | ❌ | Conta como stale (peso 2) |
| 🔴 DEPRECATED (f < 0.15) | ❌ | ❌ | Não conta (conhecimento podre não polui) |
| ⚡ CONFLICT | ❌ | ✅ | Conta como contradição (peso 3) |

### 7.3 Ciclo de Feedback: Gatilhos → Entropia

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  F9.4 Gatilhos   │     │  F1.6 Entropia   │     │  F2.1 Cognitive  │
│                  │     │                  │     │  Economy         │
│  Deprecia        │────▶│  stale reduzido  │────▶│  risk_penalty    │
│  learning        │     │  entropia cai    │     │  reduzido        │
│                  │     │                  │     │                  │
│  Revalidação     │────▶│  stale reduzido  │────▶│  efficiency      │
│  bem-sucedida    │     │  entropia cai    │     │  score sobe      │
│                  │     │                  │     │                  │
│  Gatilho T2      │────▶│  stale aumenta   │────▶│  risk_penalty    │
│  (freshness      │     │  entropia sobe   │     │  aumenta         │
│  caindo)         │     │                  │     │                  │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### 7.4 Exemplo Numérico

```
Cenário: 30 learnings, 1 contradição ativa, 2 stale, 4 gaps sem DDNA

Entropia antes da depreciação:
  raw = (1×3) + (2×2) + (4×1) = 3 + 4 + 4 = 11
  entropy = 11/30 = 0.367  🟡 MÉDIA (acima de 0.25)

Ação F9.4: Gatilho T3 dispara para 1 stale (freshness 0.10 → deprecated)
          Gatilho T2 dispara para 1 stale (freshness 0.18 → revalidado)

Após:
  stale = 1 (o T2 foi revalidado com sucesso, freshness resetou)
  raw = (1×3) + (1×2) + (4×1) = 3 + 2 + 4 = 9
  entropy = 9/30 = 0.300  🟡 MÉDIA (caiu de 0.367)

  Redução de entropia: 0.367 → 0.300 = -18.3%
  risk_penalty no F2.1: reduziu de 0.367 para 0.300
  efficiency_score ajustado: aumentou proporcionalmente
```

### 7.5 Alerta Integrado

Se a entropia está alta E há gatilhos de revalidação pendentes, o sistema gera um alerta combinado:

```markdown
## 🔴 Alerta Integrado: Entropia Alta + Gatilhos Pendentes

**Entropia atual**: 0.367 (🟡 MÉDIA) — acima do threshold de 0.25
**Gatilhos pendentes**: 3 (1× P1, 2× P2)

### Recomendação
Resolver os 3 gatilhos de revalidação reduzirá a entropia para ~0.300.
Prioridade: revalidar o learning com freshness 0.18 (T2) primeiro.

### Impacto no F2.1
- risk_penalty atual: 0.367
- risk_penalty após correção: 0.300
- Ganho no efficiency_score: +6.7%
```

---

## 8. Integração com F9.2 Wisdom Distillation

### 8.1 Relação entre Gatilhos e Destilação

A Wisdom Distillation (F9.2) **consome** learnings com freshness > 0.5. Os gatilhos (F9.4) **alimentam** a destilação ao:

1. **Remover learnings podres**: Learnings depreciados (freshness < 0.15) são excluídos da destilação
2. **Sinalizar learnings em risco**: Learnings com freshness entre 0.15 e 0.50 são marcados para atenção na destilação
3. **Forçar revalidação antes da destilação**: Se um learning candidato a princípio tem freshness baixo, a destilação pode solicitar revalidação via F9.4

```
┌────────────────────────────────────────────────────────────────────┐
│            F9.4 GATILHOS ↔ F9.2 DESTILAÇÃO                          │
│                                                                      │
│  F9.2 N1 — Coleção:                                                  │
│    ├── Só coleta learnings com freshness > 0.5                      │
│    ├── Se freshness ≤ 0.5 → EXCLUÍDO da destilação                  │
│    └── (Gatilhos F9.4 são os responsáveis por manter freshness alto) │
│                                                                      │
│  F9.2 N2 — Grupos:                                                   │
│    ├── Coesão do grupo inclui avg_freshness                          │
│    ├── Grupos com freshness médio baixo → coesão baixa → descarte   │
│    └── (Gatilhos F9.4 previnem que learnings cheguem aqui podres)   │
│                                                                      │
│  F9.2 N3 — Padrões:                                                  │
│    ├── maturity_score inclui avg_freshness (peso 0.20)              │
│    └── (Sem gatilhos, padrões seriam baseados em conhecimento velho)│
│                                                                      │
│  Feedback loop:                                                      │
│    ├── F9.2 identifica learning importante para a constituição      │
│    ├── Se freshness deste learning está caindo → F9.4 dispara       │
│    │   revalidação prioritária para preservar a cadeia de evidências│
│    └── Resultado: conhecimento constitucional sempre fresco         │
└────────────────────────────────────────────────────────────────────┘
```

### 8.2 Gatilho Especial: "Destillation Protection"

Quando a Wisdom Distillation identifica um learning como **evidência crítica** para um princípio constitucional candidato, ela pode marcar este learning com `#distillation-critical`. O F9.4 então trata este learning com prioridade especial:

```yaml
distillation_protection:
  # Se um learning marcado como #distillation-critical atinge freshness < 0.50
  # o gatilho escala para P1 (em vez de P2)
  trigger: freshness < 0.50 AND tag #distillation-critical
  priority: P1  # Escalado de P2 para P1
  action: >
    Revalidar antes do próximo ciclo de destilação.
    Este learning é evidência para um princípio constitucional candidato.
  notification: Don + Architecture Chief
```

---

## 9. Don Override: Regras de Exceção

### 9.1 "Mantém Mesmo que Velho"

O Don pode marcar um learning para **nunca expirar**, independentemente do freshness:

```yaml
don_override:
  learning_id: "2026-07-29 — A Verdade Sobre o Kernel"
  rule: "keep_forever"
  reason: "Este learning contém a identidade do Kernel — não expira nunca."
  tag: "#don-override-keep"
  effects:
    - freshness_score: congelado (não decai)
    - gatilhos T1/T2/T3: desativados
    - scheduler S1/S2/S3: ignorado
    - contradição: ainda detectada mas não bloqueia
    - still contributes to distillation: yes
  expires: never
  approved_by: Don
  date: 2026-07-30
```

### 9.2 Tipos de Override

| Tipo | Efeito | Quando usar | Exemplo |
|------|--------|-------------|---------|
| **`keep_forever`** | Learning nunca deprecia, freshness congelado | Conhecimento fundacional, identidade, CONSTITUIÇÃO | L13 — Kernel Identity |
| **`keep_for_duration`** | Não deprecia por N dias | Learning que se sabe que ainda é válido por um período | "Válido até próxima versão do Go" |
| **`silence_notifications`** | Gatilhos continuam ativos mas Don não é notificado | Don confia que o agente vai revalidar no próprio ritmo | Learnings de baixa prioridade |
| **`manual_only`** | Só Don pode revalidar este learning | Conhecimento sensível que agentes não devem revalidar sem supervisão | Decisões de segurança P0 |
| **`promote_to_constitutional`** | Learning promovido a princípio — nunca deprecia | Don decide que um learning é tão importante que vira regra | Insights de arquitetura fundamentais |

### 9.3 Regras de Override

```
1. Override NUNCA apaga o learning — apenas altera o comportamento dos gatilhos
2. Override é sempre explícito (registrado em DDNA ou diretiva do Don)
3. Override keep_forever requer aprovação do Don (ninguém mais pode fazer)
4. Override pode ser revogado pelo Don a qualquer momento
5. Learnings #constitutional e #immutable têm keep_forever implícito
6. Override é registrado no learning: campo don_override no metadata
```

### 9.4 Exceções Automáticas (Imunes por Natureza)

Certos tipos de learning são **imunes** à depreciação automática, sem necessidade de override explícito:

| Tipo | Tag | Razão |
|------|-----|-------|
| **Constitucional** | `#constitutional` | Princípios da CONSTITUIÇÃO não expiram |
| **Imutável** | `#immutable` | Marcado como verdade atemporal |
| **Identidade do Kernel** | `#kernel-identity` | Quem o Kernel é não muda |
| **Decisão DNA fundacional** | `#dna` + `decision_level: 5` | Decisões nível 5 são fundacionais |
| **Don override explícito** | `#don-override-keep` | Don mandou manter |

---

## 10. Métricas do Sistema de Gatilhos

### 10.1 Métricas Internas

| Métrica | Definição | Alvo | Fonte |
|---------|-----------|:----:|-------|
| **Trigger Accuracy** | % de gatilhos que resultaram em ação útil | > 90% | F9.4 |
| **Mean Time to Revalidate (MTTR)** | Tempo médio entre gatilho e revalidação | P0 < 24h, P1 < 7d, P2 < 30d | F9.4 |
| **Auto-revalidation Rate** | % de revalidações automáticas bem-sucedidas | > 80% | F9.4 |
| **Deprecation Rate** | Learnings depreciados por gatilho / total | < 5%/mês | F9.4 |
| **False Positive Rate** | Gatilhos que dispararam mas learning ainda era válido | < 10% | F9.4 |
| **Don Override Rate** | % de learnings com override (deveria ser baixo) | < 2% | F9.4 |
| **Escalation Rate** | % de P2 que escalaram para P1 | < 10% | F9.4 |
| **Stale Reduction** | Redução de stale após execução de gatilhos | > 50% | F1.6 |

### 10.2 Dashboard Conceitual

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    GATILHOS DE REVALIDAÇÃO — DASHBOARD                      │
│                               2026-07-30                                    │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│   GATILHOS ATIVOS:                                                        │
│   ┌──────────────┬──────┬────────┬──────────┬────────────┐               │
│   │ Trigger      │ Qtd  │ Prio   │ Prazo     │ Status     │               │
│   ├──────────────┼──────┼────────┼──────────┼────────────┤               │
│   │ T1 (30d)     │  3   │ 🟡 P2  │ 30 dias  │ pending    │               │
│   │ T2 (now)     │  1   │ 🟠 P1  │ 7 dias   │ pending    │               │
│   │ T3 (deprec)  │  0   │ 🔴 P0  │ imediato │ —          │               │
│   │ C1 (conflict)│  1   │ 🟠 P1  │ 7 dias   │ escalated  │               │
│   │ E1-E6        │  0   │ —      │ —        │ —          │               │
│   └──────────────┴──────┴────────┴──────────┴────────────┘               │
│                                                                           │
│   MÉTRICAS:                                                               │
│   ├── MTTR P0: N/A (sem P0 ativos)                                       │
│   ├── MTTR P1: 3.2 dias (média)                                          │
│   ├── Auto-revalidation: 85% ✅                                           │
│   ├── False positive: 5% ✅                                               │
│   └── Escalation rate: 8% ✅                                              │
│                                                                           │
│   IMPACTO NA ENTROPIA:                                                    │
│   ├── Entropia antes: 0.433                                              │
│   ├── Entropia após correção: 0.300 (projetado)                          │
│   └── Redução: -30.7%                                                    │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 10.3 Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| MTTR P0 > 48h | 🔴 ALTO | Pipeline de revalidação pode estar quebrado — investigar |
| False positive rate > 20% | 🟡 MÉDIO | Thresholds podem estar muito sensíveis — recalibrar |
| Escalation rate > 30% | 🟡 MÉDIO | Learnings T1 não estão sendo revalidados antes de virar T2 |
| Don Override rate > 5% | 🟡 MÉDIO | Don está fazendo override em vez de revalidar — revisar se thresholds estão adequados |
| 0 revalidações em 30 dias | 🔴 ALTO | Ninguém está revalidando — sistema pode estar ignorando os gatilhos |

---

## 11. Exemplo com Learning Real

### 11.1 Learning: L11 — Onda 6 Activation (2026-07-28)

Vamos simular o que acontece com um learning real ao longo do tempo, usando os dados do Kernel.

#### Dados do Learning no Momento da Criação

```markdown
### 2026-07-28 — Onda 6 — Liderança + Órfãos Activation Wave | Level 3

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Level** | 3 |
| **Confidence** | 0.78 |
| **Wisdom Decay Category** | STABLE |
| **Last Validated** | 2026-07-28 |
| **Freshness** | 1.00 |
| **Usage Count** | 2 |
| **Tags** | #onda-6 #agent-activation #leadership #parallel-delegation #orchestration |
```

#### Cenário 1: Learning Usado Regularmente (Caminho Saudável)

```
Data: 2026-07-30 (2 dias depois)
  freshness = 0.99 → 🟢 HEALTHY
  Nenhum gatilho disparado
  ▶ Comportamento normal

Data: 2026-08-28 (30 dias depois)
  freshness = 0.93 → 🟢 HEALTHY
  Nenhum gatilho disparado
  ▶ Ainda fresco, sem ação necessária

Data: 2026-10-26 (90 dias depois — schedule STABLE)
  freshness = 0.80 → 🟢 HEALTHY (mas próximo de 0.70)
  Gatilho S2 dispara: "Revisão de STABLE a cada 90 dias"
  Task criada: REVAL-20261026-0001
  Prioridade: 🟡 P2
  Assignada para: cosca-kernel
  Prazo: 30 dias

  Se revalidado (válido):
    freshness resetado para 1.00
    last_validated = 2026-10-26
    Tag: #fresh

  Se não revalidado em 30 dias:
    Escala para T2 (P1)
    Notifica Don
```

#### Cenário 2: Learning Esquecido (Decaimento até Depreciação)

```
Data: 2026-07-30 (2 dias depois)
  freshness = 0.99 → 🟢 HEALTHY

Data: 2026-08-28 (30 dias depois)
  freshness = 0.93 → 🟢 HEALTHY

Data: 2026-10-26 (90 dias depois)
  freshness = 0.80 → 🟢 HEALTHY
  Gatilho S2 dispara (revisão STABLE)

  ❌ Ninguém revalida. O learning continua sem uso.

Data: 2027-01-24 (180 dias depois)
  freshness = 0.60 → 🟡 REVALIDATE_30
  Gatilho T1 dispara pela primeira vez (freshness < 0.70)
  Task criada: REVAL-20270124-0001
  Prioridade: 🟡 P2
  Prazo: 30 dias

  ❌ Ninguém revalida.

Data: 2027-02-23 (210 dias depois)
  freshness = 0.42 → 🟡 REVALIDATE_30 (ainda)
  Escala: T1 venceu → vira T2 (P1)
  Task: REVAL-20270124-0001 escala para P1
  Notifica Don: "Learning Onda 6 não foi revalidado em 30 dias.
                 Freshness atual: 0.42. Ação necessária em 7 dias."

  ❌ Don não responde, ninguém revalida.

Data: 2027-03-02 (217 dias depois)
  freshness = 0.39 → 🟡 REVALIDATE_30
  Task P1 venceu (7 dias) → escala para P0
  Notifica Don URGENTE: "Learning será depreciado em 7 dias se não houver ação."

  ❌ Don não responde.

Data: 2027-03-09 (224 dias depois)
  freshness = 0.36 → 🟡 REVALIDATE_30
  ⚠️ DEPRECIAÇÃO AUTOMÁTICA
  Learning movido para learning_history
  Tag: #deprecated
  Nota: "Auto-depreciado em 2027-03-09: 224 dias sem revalidação."
```

#### Cenário 3: Contradição Detectada (C1)

```
Contexto: L11 diz "ativação de agentes funciona com prompts estruturados"
Um novo learning L99 diz "ativação de agentes requer validação prévia de capability"

Detecção (F1.4 Contradiction Decay):
  - Tags comuns: #agent-activation, #orchestration (overlap > 60%)
  - Afirmação L11: "prompts estruturados são suficientes"
  - Afirmação L99: "validação de capability é pré-requisito"
  - Ambos têm freshness > 0.30
  → Contradição detectada: C1

Ação F9.4:
  Gatilho C1 dispara
  Prioridade: 🟠 P1
  Ações:
    1. Bloquear ambos os learnings (não usar até resolução)
    2. Criar DDNA de contradição
    3. Notificar Don (resumo semanal)
    4. Escalar para Architecture Chief resolver

Resolução possível:
  Architecture Chief analisa: L11 se refere a ativação de agentes já existentes;
  L99 se refere a ativação de novos agentes. Não são contraditórios — são complementares.
  → Contradição resolvida: ambos liberados
  → Lição: adicionar contexto mais específico nos learnings
```

#### Cenário 4: Evento Externo (E1 — Nova Versão do Go)

```
Evento: Go 1.23 lançado (2027-02-01)
  Gatilho E1 dispara
  Learnings afetados: todos com tag #golang ou #go (incluindo L11 — usa paralelismo Go)

  Prioridade: 🟠 P1
  Ação: Revalidar todos learnings com tags #golang
  Tasks criadas: 12 (todos learnings do Kernel com tag #golang)
  Notifica: cosca-kernel + Architecture Chief

  Para L11 especificamente:
    - O padrão de paralelismo Go (goroutines + channels) ainda é a melhor prática?
    - Alguma mudança no runtime do Go 1.23 que afeta o padrão descrito?
    - Se sim: atualizar learning, freshness resetado
    - Se não: revalidar como válido
```

### 11.2 Resumo da Simulação

| Cenário | Gatilho | Prioridade | Ação | Tempo até resolução |
|---------|---------|------------|------|-------------------|
| Uso regular | Nenhum | — | Nenhuma | — |
| Schedule STABLE | S2 | P2 | Revisão programada | 90 dias |
| Esquecido (leve) | T1 | P2 | Agendar revalidação | 30 dias |
| Esquecido (grave) | T2 | P1 | Revalidar agora | 7 dias |
| Abandono total | T3 | P0 | Depreciação automática | 14 dias (após T2) |
| Contradição | C1 | P1 | Revisão manual | 7 dias |
| Evento externo | E1 | P1 | Revalidar lote | 7 dias |

---

## 12. Casos de Borda

### 12.1 Tabela de Casos de Borda

| Caso | O que acontece | Tratamento |
|------|---------------|------------|
| **Learning recém-criado com freshness 1.00** | Nenhum gatilho dispara | Normal — conhecimento fresco |
| **Learning com tag `#constitutional`** | Imune a T1/T2/T3 — nunca deprecia | Override automático (§9.4) |
| **Don cria learning e já marca `#immutable`** | Gatilhos ignorados permanentemente | Override preventivo |
| **Todos os 54 agentes com freshness < 0.30 ao mesmo tempo** | 54 tasks P0 criadas simultaneamente | Don notificado com relatório consolidado. Máximo 50 revalidações por execução. |
| **Contradição entre learning ativo e learning já depreciado** | Apenas o ativo é afetado | Gatilho C3 — contradição com deprecated é ignorada |
| **Evento externo (ex: CVE) dispara para 200+ learnings** | 200+ tasks P0 criadas | Batch processing: Don notificado com top 10 mais críticos |
| **Don override em learning que já está deprecated** | Override é rejeitado — learning deprecated não pode ser "desdepreciado" | Don deve criar novo learning |
| **Learning com freshness 0.16 é usado em uma task** | Uso recente aumenta recency_term | Freshness pode subir acima de 0.15 — gatilho T3 desarma |
| **Agente dono do learning não existe mais** | Task não pode ser assignada | Escala para Architecture Chief + Don |
| **Gatilho T2 dispara mas learning é referenciado em CONSTITUIÇÃO** | Prioridade mantida como P1 mas Don é notificado | CONSTITUIÇÃO não pode ter referências quebradas |
| **Revalidação automática falha (agente não consegue determinar validade)** | Learning marcado como `#needs-human-review` | Escala para Don como P1 |
| **Scheduler semanal falha por 2 semanas seguidas** | Gatilhos acumulam | Execução forçada na 3ª semana com relatório de catch-up |

### 12.2 Anti-Padrões

| Anti-Padrão | Por que evitar | Como detectar |
|-------------|---------------|---------------|
| **Revalidar tudo toda semana** | Custo cognitivo alto sem ganho proporcional | Se 70%+ das revalidações retornam "válido", o intervalo é curto demais |
| **Ignorar gatilhos P2 até virarem P0** | Atraso na revalidação aumenta risco de decisão baseada em conhecimento velho | Escalation rate > 20% |
| **Depreciar learning sem criar substituto** | Perde-se o conhecimento sem alternativa | `superseded_by` vazio em 30%+ das depreciações |
| **Don override para tudo que dá trabalho** | Override desvirtua o sistema de gatilhos | Don Override rate > 5% |
| **Não notificar Don em P0** | Don não sabe que conhecimento crítico está morrendo | Auditoria revela P0 sem notificação |
| **Revalidar sem verificar código** | Revalidação superficial não detecta obsolescência real | Taxa de falso positivo > 20% |
| **Depreciar learning que ainda é referenciado** | Links quebrados na base de conhecimento | `cross_references` não vazios em learning depreciado |

---

## Apêndice A: Estrutura de Dados da Task de Revalidação

```yaml
revalidation_task:
  task_id: "REVAL-20260730-0001"
  trigger:
    type: "T2"                    # T1-T5, C1-C3, E1-E6, S1-S4
    source: "wisdom_decay"        # wisdom_decay | contradiction | external_event | schedule
    freshness_at_trigger: 0.22
    priority: "P1"                # P0 | P1 | P2 | info
  
  learning:
    id: "2026-07-28 — Onda 6"
    agent: "cosca-kernel"
    tags: ["#onda-6", "#agent-activation"]
    level: 3
    decay_category: "STABLE"
  
  assignment:
    assigned_to: "cosca-kernel"   # agent_id
    assigned_at: "2026-07-30T00:00:00Z"
    deadline: "2026-08-06T00:00:00Z"
    status: "pending"             # pending | in_progress | completed | deprecated | escalated
  
  revalidation:
    result: null                  # valid | partial | invalid | null (se pendente)
    validated_at: null
    validated_by: null
    note: null
  
  escalation:
    original_priority: "P1"
    current_priority: "P1"
    escalated_at: null
    escalated_to: null
    escalation_reason: null
  
  history:
    - event: "created"
      at: "2026-07-30T00:00:00Z"
    - event: "assigned"
      at: "2026-07-30T00:00:05Z"
```

## Apêndice B: Caminhos de Escalação por Tipo de Gatilho

```
T1 (REVALIDATE_30):
  └── 30d sem ação → T2 (REVALIDATE_NOW, P1)

T2 (REVALIDATE_NOW):
  └── 7d sem ação → P0 (Don urgente)
      └── +7d sem Don response → DEPRECIAÇÃO AUTOMÁTICA

T3 (DEPRECATED): estado final, não escala

C1 (Contradição):
  └── 7d sem resolução → C2 (P0, revisão manual forçada)

E1-E6 (Evento externo):
  └── Prioridade do evento (P0 ou P1)
      Se P1 e 7d sem ação → escala para P0

S1-S4 (Schedule):
  └── Schedule vence → T1 ou T2 dependendo do freshness atual
```

## Apêndice C: Checklist para Implementação

- [ ] Integrar F9.4 com F1.4 Wisdom Decay (consumir freshness_score, contradictions)
- [ ] Implementar tabela de gatilhos (T1-T5, C1-C3, E1-E6, S1-S4) no código
- [ ] Implementar pipeline de 6 passos (gatilho → task → assign → revalidação → resolução → depreciação)
- [ ] Implementar scheduler semanal (cron ou task counter)
- [ ] Implementar sistema de notificação (P0 = Don urgente, P1 = digest, P2 = log)
- [ ] Implementar Don override (keep_forever, keep_for_duration, silence, manual_only)
- [ ] Integrar com F1.6 Cognitive Entropy (depreciação reduz entropia)
- [ ] Integrar com F9.2 Wisdom Distillation (proteção de evidências críticas)
- [ ] Implementar CLI (`cosca wisdom-decay gatilhos run`)
- [ ] Criar testes para cada tipo de gatilho (especialmente T2, T3, C1, E3)

---

## Changelog

| Versão | Data | Autor | Mudança |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — 4 tabelas de gatilhos (freshness, contradição, evento, schedule), pipeline de 6 passos, scheduler semanal, notificação P0/P1/P2, integrações com F1.4/F1.6/F9.2, Don override, exemplo real com L11, métricas, casos de borda |

---

> *"Conhecimento sem revalidação é como um mapa de 1985 — você pode segui-lo confiante e acabar no meio do oceano. O despertador não é inimigo do sono — é o que impede que você durma para sempre."*
> — Cosca Architecture Chief, 2026-07-30
