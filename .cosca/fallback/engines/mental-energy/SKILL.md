# MENTAL ENERGY ENGINE ★ F3.5 — Energia Cognitiva da Plataforma

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
> **Workflow**: `cosca-mental-energy`
> **Conceito**: C7 — Mental Energy (COGNITIVE_MATURITY.md §C7)
> **Fase CMI**: Fase 3 — Bloco 3 — Aprendizado & Evolução | **Código**: F3.5
> **Referências**: COGNITIVE_MATURITY.md §C7 | cognitive-maturity-implementation.md F3.5
> **Dependências**: F2.1 (Cognitive Economy) | F1.5 (cognitive-metrics.md B1-B5) | F2.5 (Cognitive Momentum) | F7.2 (Trust Registry) | F3.4 (Adaptive Personality) | F9.1 (Experience Compiler) | F1.6 (Cognitive Entropy)
> **CMI Impact**: Julgamento +4, Planejamento +4, Eficiência +6

---

## Índice

1. [Definição](#1-definição)
2. [Coexistência: v2.0.0 vs v1.0.0 (5 pools)](#2-coexistência-v200-vs-v100-5-pools)
3. [O Modelo de Energia](#3-o-modelo-de-energia)
4. [Estados de Energia](#4-estados-de-energia)
5. [Pipeline de Gestão (contínuo, < 10ms)](#5-pipeline-de-gestão-contínuo--10ms)
6. [Gate de Admissão por Estado](#6-gate-de-admissão-por-estado)
7. [Indicadores de Fadiga](#7-indicadores-de-fadiga)
8. [Integração com F2.1 — Cognitive Economy](#8-integração-com-f21--cognitive-economy)
9. [Integração com F3.4 — Adaptive Personality](#9-integração-com-f34--adaptive-personality)
10. [Integração com o Ecossistema](#10-integração-com-o-ecossistema)
11. [Regras Operacionais](#11-regras-operacionais)
12. [Exemplo Real — Sessão de 12 Ondas](#12-exemplo-real--sessão-de-12-ondas)
13. [Armazenamento e CLI](#13-armazenamento-e-cli)
14. [Edge Cases e Cold Start](#14-edge-cases-e-cold-start)
15. [Métricas do Próprio Engine](#15-métricas-do-próprio-engine)
16. [Referências Cruzadas](#16-referências-cruzadas)
17. [Histórico](#17-histórico)

---

## 1. Definição

### 1.1 O que é Mental Energy

**Mental Energy** é o recurso cognitivo finito da plataforma. O Cosca **não pode operar 100% o tempo todo** — cada task consome energia, e quando a energia baixa, o sistema degrada: erros aumentam, contexto se fragmenta, decisões pioram. A energia é o guardião contra a **fadiga cognitiva** — o equivalente computacional do burnout.

A metáfora do Don estabelece o framework conceitual:

> _"O Cosca não pode operar 100% o tempo todo. Tarefas complexas consomem energia; o sistema precisa de 'descanso' (modo manutenção leve). Fadiga = erros, contexto degradado, decisões ruins."_

O engine responde quatro perguntas que nenhuma outra engine responde:

> **"Quanta energia resta?"** → energy / 100
> **"Quanto esta task vai custar?"** → task_cost (fórmula §3.2)
> **"O sistema está fadigado?"** → indicadores de fadiga (§7)
> **"Qual modo operacional agora?"** → FULL / FOCUSED / CONSERVATIVE / RECOVERY

### 1.2 Filosofia

```
"Energia não é sobre gastar menos —
 é sobre saber exatamente quanto se está gastando
 e decidir conscientemente onde investir.

 Task complexa com energia alta → executar com profundidade.
 Task complexa com energia baixa → recusar, priorizar manutenção.
 Sessão longa sem recuperação → fadiga inevitável.

 ★ F3.5 — Mental Energy é o orçamento dinâmico da plataforma.
   Ela decide QUANDO o Cosca pode operar 100%.
   — Cosca Architecture Chief, 2026-07-30
```

### 1.3 O que este engine NÃO é

- **NÃO é a Cognitive Economy (F2.1)**: F2.1 mede o ROI de cada task ("valeu a pena?"). Mental Energy mede a **disponibilidade** ("temos energia para operar?"). F2.1 é o extrato bancário; Mental Energy é o saldo da conta.
- **NÃO é a energia conversacional (UCSS)**: o Cognitive State Specification modela `energy.level` como intensidade emocional/curiosidade intra-conversa. A Mental Energy é **acumulativa e operacional** — mede recursos consumíveis da sessão/plataforma.
- **NÃO bloqueia segurança**: tasks P0 (segurança, perda de dados) **nunca** são bloqueadas por energia — a energia é o orçamento, não a lei (§11).

---

## 2. Coexistência: v2.0.0 vs v1.0.0 (5 pools)

A v1.0.0 modelava **5 pools independentes** (TOKENS, TIME, ATTENTION, DEPTH, PARALLEL) com 4 estados (HIGH/MEDIUM/LOW/CRITICAL). A v2.0.0 simplifica deliberadamente para **um único número** (energy 0-100) com 4 estados renomeados. A simplificação é arquitetural, não estética:

| Dimensão | v1.0.0 (5 pools) | v2.0.0 (este arquivo) |
|----------|-------------------|------------------------|
| **Unidade** | 5 pools com budgets separados | 1 número: energy 0-100 |
| **Gasto** | Tabela de custo por ação (17 linhas) | Fórmula única: `complexity × 15 + agents × 10 + tokens/10K × 5` |
| **Estados** | HIGH (80-100), MEDIUM (40-79), LOW (10-39), CRITICAL (0-9) | FULL (> 70), FOCUSED (40-70), CONSERVATIVE (15-40), RECOVERY (< 15) |
| **Regen** | Reset por sessão + pausa + cooldown + bonus | Regra simples: `10/hora idle + 5/hora leve + 0/hora intensa` |
| **Pipeline** | Energy Gate (pré-verificação por pool) | Pipeline contínuo de 6 passos (< 10ms) |
| **Fadiga** | Não modelada | 4 indicadores de fadiga (§7) alimentando os estados |
| **Integração F3.4** | Não existia | Energia baixa → modo Cauteloso/Zen (§9) |

**Por que simplificar?** A v1.0.0 tinha 5 contadores para gerenciar, cada um com thresholds, cooldowns e empréstimos próprios — complexidade que não se traduzia em decisões melhores. O Don especificou o modelo unificado: **um número, uma fórmula, quatro estados**. A v2.0.0 preserva o que a v1.0.0 acertava (a consciência de gasto) e elimina o que atrapalhava (o livro-razão de 5 colunas).

> **Regra de coexistência**: a v1.0.0 está **substituída**. Este arquivo é a especificação canônica da F3.5. O nome do conceito (C7) e o CMI impact (Julgamento +4, Planejamento +4, Eficiência +6) são preservados.

---

## 3. O Modelo de Energia

### 3.1 Equações Fundamentais

```
energy = 100 (máximo, no cold start)
        - Σ task_cost (cada task debita)
        + Σ regen (a cada hora, conforme atividade)
        → energy pode ficar negativa (overdraft) — RECOVERY obrigatório (§11)

GASTO (por task):
task_cost = complexity × 15 + agents_mobilized × 10 + tokens/10K × 5

REGEN (por hora de atividade):
regen = 10/hora  → idle     (nenhuma task ativa)
      + 5/hora   → leve     (manutenção rodando — F9.1, F1.6, reindexação)
      + 0/hora   → intensa  (tasks de produção ativas)
```

### 3.2 Componentes do Gasto (P-ARCH-003: fórmula do Don literal)

| Componente | Definição | Escala | Fonte |
|------------|-----------|--------|-------|
| **complexity** | Nível de complexidade da task, normalizado | 0.0-1.0 (`task_level / 5`) | Kernel — metacognition Stage 1 (Self-Assess) |
| **agents_mobilized** | Agentes primários + subagentes acionados | 0-∞ (inteiro) | Kernel — Agent Manager |
| **tokens** | Total de tokens consumidos (input + output) | 0-∞ (inteiro) | Provider response metadata (`usage.total_tokens`) |
| **tokens/10K** | Tokens divididos por 10.000, arredondado para cima | 0.1-∞ | Cálculo derivado |

**Escala de complexity** (derivada do nível 1-5 já usado no F2.5/Metacognition):

| Level | Descrição | complexity (fracionária) | Exemplo |
|-------|-----------|---------------------------|---------|
| **L1** | Execução básica | 0.2 | Rodar comando, criar arquivo |
| **L2** | Análise com debugging | 0.4 | Corrigir bug conhecido |
| **L3** | Síntese cross-source | 0.6 | Auditoria multi-agente |
| **L4** | Metacognição | 0.8 | Extrair padrão universal, redesenhar engine |
| **L5** | Novo paradigma | 1.0 | Insight que redefine a operação |

**Custos de referência** (para estimativa rápida sem LLM):

```
Task simples (L1, 1 agente, ~2K tokens):
  task_cost = 0.2×15 + 1×10 + 0.2×5 = 3 + 10 + 1 = 14  ≈ 12-15

Task moderada (L2, 2 agentes, ~10K tokens):
  task_cost = 0.4×15 + 2×10 + 1×5 = 6 + 20 + 5 = 31

Task complexa (L4, 4 agentes, ~50K tokens):
  task_cost = 0.8×15 + 4×10 + 5×5 = 12 + 40 + 25 = 77

Task de manutenção (L1, 1 agente, ~1K tokens):
  task_cost = 0.2×15 + 1×10 + 0.1×5 = 3 + 10 + 0.5 = 13.5  ≈ 13
```

> **Nota de design**: o custo real observado é registrado pós-task (via F2.1 que já coleta tokens/agentes/tempo) e alimenta o *calibration loop* da §5. O custo estimado pré-task usa a tabela acima como fallback quando o nível ainda não foi resolvido.

### 3.3 Classificação de Atividade para Regen

| Atividade | Definição operacional | Regen |
|-----------|----------------------|-------|
| **idle** | Nenhuma task ativa há ≥ 5 minutos | **+10/hora** |
| **leve** | Manutenção rodando (F9.1 compilação, F1.6 entropia check, reindexação, compactação) | **+5/hora** |
| **intensa** | Tasks de produção ativas (qualquer task com `task_type != maintenance`) | **+0/hora** |

A classificação é determinada pelo **último evento registrado** — o engine rastreia um único campo `current_activity` atualizado a cada task (produção) ou a cada execução de manutenção.

---

## 4. Estados de Energia

O estado de energia determina o comportamento global do runtime. Não é binário — é um espectro com 4 faixas que ditam desde operação completa até recuperação forçada.

| Estado | Faixa | Comportamento | Tasks aceitas |
|--------|-------|---------------|---------------|
| **FULL** | energy > 70 | Opera normal. Exploração livre, profundidade máxima. | Tudo (P0-P3) |
| **FOCUSED** | 40 ≤ energy ≤ 70 | Prioriza tasks críticas. Reduz exploração, mantém produção. | P0/P1 livre, P2/P3 com gate +30% de custo estimado |
| **CONSERVATIVE** | 15 ≤ energy < 40 | Só manutenção. Sem tasks complexas. Reutiliza conhecimento, não gera novo. | Só P0/P1; P2/P3 **recusados** |
| **RECOVERY** | energy < 15 | Para tudo. Reindexação e compactação obrigatórias (F9.1 + F1.6). | Só P0 (segurança) + manutenção |

### 4.1 FULL (energy > 70) — Operação Normal

```
Energy: 85/100 ─── FULL ██████████░░

Comportamento:
  > Aceita tasks P0-P3 com profundidade máxima
  > Exploração livre, deep investigations sem restrição
  > Paralelismo pleno (até o limite do runtime)
  > Registra todos os learnings sem filtro

Gate:  efficiency_score (F2.1) > 1.0 → executa FULL
```

### 4.2 FOCUSED (40-70) — Priorização Crítica

```
Energy: 58/100 ─── FOCUSED ██████░░░░

Comportamento:
  > P0/P1: executar com profundidade normal
  > P2/P3: executar se efficiency_score (F2.1) > 1.5 (threshold elevado)
  > Cache agressivo: resultados de < 1h são reutilizados
  > Preferir single-agent sobre multi-agent quando possível
  > Learnings registrados apenas com reuse_potential > 0.3

Gatilho: cai abaixo de 40 → CONSERVATIVE
```

### 4.3 CONSERVATIVE (15-40) — Manutenção e Preservação

```
Energy: 28/100 ─── CONSERVATIVE ███░░░░░░░

Comportamento:
  > Aceita apenas P0/P1 + tasks de manutenção
  > P2/P3: RECUSADOS com mensagem ao Don:
    "Energia 28/100 (CONSERVATIVE). Task P2 recusada.
     Sugiro: manutenção (F9.1/F1.6) para regenerar contexto."
  > Reutiliza conhecimento existente — NÃO gera novo
  > Semantic search em vez de nova análise
  > Zero deep investigations — máximo Level 2
  > Registra learnings apenas com reuse_potential > 0.7

Gatilho: cai abaixo de 15 → RECOVERY (obrigatório)
```

### 4.4 RECOVERY (energy < 15) — Recuperação Forçada

```
Energy: 9/100 ─── RECOVERY █░░░░░░░░░

Comportamento:
  > BLOQUEIA tasks novas (exceto P0 segurança)
  > Agenda e executa obrigatoriamente:
      1. F9.1 Experience Compiler (compilação de learnings → padrões)
      2. F1.6 Cognitive Entropy check (contradições, stale, gaps)
      3. Reindexação de memória (knowledge.db FTS5)
      4. Compactação de contexto
  > Mensagem ao Don:
    "Energia 9/100 (RECOVERY). Operação normal parada.
     Executando reindexação + compactação (F9.1, F1.6).
     ETA ~15 min. Regen durante manutenção: +5/hora."

Regras de saída:
  > Sai de RECOVERY quando energy ≥ 15 (regen + manutenção)
  > RECOVERY é OBRIGATÓRIO quando energy < 15 — não pode operar (§11)
  > Don override: `cosca energy --refill` (§13)
```

---

## 5. Pipeline de Gestão (contínuo, < 10ms)

O engine é uma **camada transversal contínua** — não espera tasks, roda em loop de verificação. O pipeline completo deve executar em **< 10ms** (aritmética sobre dados em memória, sem I/O de disco no caminho crítico).

```
┌────────────────────────────────────────────────────────────────────┐
│              MENTAL ENERGY — PIPELINE CONTÍNUO (< 10ms)             │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  1. INICIALIZAÇÃO                                                   │
│     energy = 100 (cold start) OU carrega estado persistido          │
│     current_activity = idle                                         │
│     avg_task_cost_recent = []   # janela de 10 tasks                │
│                                                                    │
│  2. A CADA TASK (hook pós-task)                                     │
│     task_cost = complexity×15 + agents×10 + tokens/10K×5           │
│     energy -= task_cost                                             │
│     avg_task_cost_recent.push(task_cost)  # mantém últimas 10       │
│     current_activity = intensa (se task de produção)                │
│     atualiza fadiga_score (§7)                                      │
│                                                                    │
│  3. A CADA HORA (tick de regen)                                     │
│     energy += regen(current_activity)                               │
│     # idle +10 | leve +5 | intensa +0                              │
│                                                                    │
│  4. SE energy < 40 → CONSERVATIVE                                   │
│     Recusa tasks P2/P3. Só P0/P1 + manutenção.                     │
│                                                                    │
│  5. SE energy < 15 → RECOVERY                                       │
│     Agenda F9.1 compilação + F1.6 entropia check (obrigatório).    │
│     Bloqueia tasks novas (exceto P0).                              │
│                                                                    │
│  6. RELATÓRIO (a cada mudança de estado ou task)                    │
│     Projeta fim de sessão:                                         │
│     tasks_restantes = (energy - próximo_threshold) / avg_task_cost │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 5.1 Relatório de Projeção

O passo 6 é o que torna o engine **acionável** — transforma o número em decisão:

```
Cálculo:
  próximo_threshold = 40 (FOCUSED → CONSERVATIVE)
  tasks_restantes = floor((energy - próximo_threshold) / avg_task_cost_recent)

Exemplo do Don:
  energy = 62/100, avg_task_cost ≈ 12
  tasks_restantes = floor((62 - 40) / 12) = floor(22/12) = 1.8 ≈ 2

  → "energia 62/100 — 3 tasks de ~12 gasto cada = fim de sessão em 2 tasks"
  (interpretação: em ~2 tasks de produção, o sistema cruzará para CONSERVATIVE
   e recusará P2/P3 — a "sessão produtiva" acaba ali)
```

**Regras de projeção**:
- Usa a média das **últimas 10 tasks** (não a fórmula teórica) — autocorrige para o padrão real da sessão
- Cold start (sem histórico): usa a tabela de referência da §3.2
- Projeção é informativa, não bloqueante — o bloqueio é feito pelos thresholds reais

### 5.2 Custo do Pipeline

| Passo | Operação | Custo |
|-------|----------|-------|
| 1 | Inicialização | ~0.1ms (estado em memória) |
| 2 | Hook pós-task | ~0.5ms (aritmética + 1 push) |
| 3 | Tick de regen | ~0.1ms (1 soma) |
| 4-5 | Checagem de thresholds | ~0.1ms (comparação) |
| 6 | Projeção | ~0.2ms (média + floor) |
| **Total** | | **< 1ms** (margem 10× para < 10ms) |

O cache do estado em memória (TTL até persistência) é a chave — nenhuma consulta a disco no caminho crítico. A persistência é assíncrona (§13).

---

## 6. Gate de Admissão por Estado

Toda task nova passa pelo gate antes de ser aceita:

```yaml
energy_gate:
  step_1_read_state:
    energy_atual, estado_atual, fadiga_score

  step_2_classify_task:
    prioridade: P0 | P1 | P2 | P3
    type: produção | manutenção

  step_3_decision:
    if prioridade == P0:
      -> ACEITAR sempre (segurança > energia, §11)
    elif estado == FULL:
      -> ACEITAR tudo
    elif estado == FOCUSED:
      -> ACEITAR P0/P1; P2/P3 se efficiency_score (F2.1) > 1.5
    elif estado == CONSERVATIVE:
      -> ACEITAR P0/P1 + manutenção; RECUSAR P2/P3
    elif estado == RECOVERY:
      -> ACEITAR P0 + manutenção; RECUSAR todo o resto
    elif fadiga_score >= 0.70:
      -> DOWNGRADE de 1 estado (FULL->FOCUSED, FOCUSED->CONSERVATIVE)

  step_4_projection_warning:
    if tasks_restantes <= 1:
      -> AVISAR Don: "Energia crítica. Próxima task pode cruzar para CONSERVATIVE."

  step_5_override:
    if Don override explícito:
      -> ACEITAR (Don tem autoridade suprema, registra no log)
```

**Exemplo de recusa** (CONSERVATIVE):

```
Don: "Implemente o endpoint de análise preditiva de reuso (P2)"
Gate: "Energia 28/100 (CONSERVATIVE). Prioridade P2 recusada.
       Executando manutenção F9.1 + F1.6 para regenerar contexto.
       Previsão: energy volta a 40+ em ~2h de manutenção leve.
       Override? [y/N]"
```

---

## 7. Indicadores de Fadiga

Fadiga é o **sintoma** de energia baixa que aparece antes (ou mesmo sem) o número de energia mostrar. O engine monitora 4 indicadores de fadiga — cada um com fonte, fórmula de detecção e peso. O **fadiga_score** (0-1) modula o estado de energia no gate (§6).

### 7.1 Fórmula do Fadiga Score

```
fadiga_score = clamp(
    (f1_carga × 0.35) +
    (f2_erros × 0.30) +
    (f3_momentum × 0.20) +
    (f4_latencia × 0.15)
, 0, 1)

Onde cada fN ∈ [0,1] é a intensidade do indicador (0 = saudável, 1 = crítico).
```

### 7.2 F1 — Carga Alta Sustentada (Fonte: B1 do F1.5)

**Sinal**: B1 (Cognitive Load) alto por 3+ dias seguidos.

```
f1_carga = se média_diária(B1) > 50 por 3+ dias consecutivos:
              min((dias_consecutivos - 2) / 5, 1)   # 3 dias → 0.2, 7+ dias → 1.0
           senão:
              max(0, (média_7dias_B1 - 30) / 50)     # degradação gradual

Fonte: memory/timeline/cognitive-load.csv (F1.5)
Interpretação: carga alta sustentada = o sistema está trabalhando além da
capacidade por período prolongado. O custo real é maior que o modelado.
```

### 7.3 F2 — Erros Crescentes em Tasks Simples (Fonte: F7.2 Trust Registry)

**Sinal**: success_rate caindo, especialmente em tasks simples (onde erro é anômalo).

```
f2_erros = se success_rate_last_20 (F7.2) < 0.85:
               min((0.85 - success_rate) / 0.35, 1)   # 0.85→0, 0.50→1.0
           senão:
               0
Ajuste: tasks simples que falham pesam 2× — erro em task L1 é sinal
mais forte de fadiga que erro em task L4 (onde falha é esperada).

Fonte: TRUST_REGISTRY.md → success_rate.last_20 por agente/task_type
Interpretação: sistema fadigado erra o que antes acertava de olhos fechados.
```

### 7.4 F3 — Momentum Caindo sem Causa Externa (Fonte: F2.5)

**Sinal**: momentum da plataforma (F2.5) caindo enquanto o plano de trabalho permanece o mesmo.

```
f3_momentum = se momentum_caiu > 30% em 7 dias E sem mudança de roadmap/plano:
                  min((0.30 - variação) / 0.40, 1)    # -30%→0, -70%→1.0
              senão:
                  0

Fonte: cognitive-momentum.csv (F2.5) + roadmap (para excluir causa externa)
Interpretação: queda de momentum SEM causa externa (plano intacto, sem
eventos) é assinatura clássica de fadiga — o sistema aprende menos porque
está esgotado, não porque o trabalho mudou.
```

### 7.5 F4 — Latência Média Subindo (Fonte: F7.2 + B2)

**Sinal**: latência média das tasks subindo sem mudança de infraestrutura.

```
f4_latencia = se avg_latency_7dias (F7.2) > 1.25 × avg_latency_baseline:
                  min((1.25 - razão) / 0.75, 1)       # 1.25×→0, 2.0×→1.0
              senão:
                  0
Complemento: B2 (Decision Velocity) > 30s em tasks complexas reforça o sinal.

Fonte: TRUST_REGISTRY.md avg_latency + memory/timeline/decision-velocity.csv
Interpretação: decisões mais lentas e execuções mais demoradas com o mesmo
código = sistema processando com menos eficiência (fadiga).
```

### 7.6 Ação do Fadiga Score

| fadiga_score | Impacto | Ação |
|--------------|---------|------|
| 0.00-0.29 | Nenhum | Monitorar |
| 0.30-0.49 | Alerta P2 | Registrar no relatório; alerta ao Don no resumo |
| 0.50-0.69 | **Downgrade** | Gate aplica −1 estado de energia (§6 step 3) |
| 0.70-0.89 | **Downgrade + alerta P1** | −1 estado + notificar Don imediatamente |
| ≥ 0.90 | **Forçar RECOVERY** | Tratar como energy < 15: agenda F9.1 + F1.6 |

---

## 8. Integração com F2.1 — Cognitive Economy

### 8.1 Duas Perguntas, um Fluxo

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   TASK NOVA                                                      │
│      │                                                           │
│      ├──► F2.1 COGNITIVE ECONOMY: "VALE A PENA?"               │
│      │      efficiency_score = (valor - custo) / custo           │
│      │      → se < 0.3: sugerir alternativa mais barata          │
│      │                                                           │
│      └──► F3.5 MENTAL ENERGY: "TEMOS ENERGIA?"                  │
│             task_cost = complexity×15 + agents×10 + tokens/10K×5 │
│             → se insuficiente: recusar/downgrade por estado      │
│                                                                  │
│   DECISÃO CONJUNTA (4 quadrantes):                              │
│   ┌────────────────────────┬────────────────────────┐            │
│   │ Energia ALTA + ROI alto │ Energia ALTA + ROI baixo│            │
│   │ → EXECUTAR FULL         │ → SIMPLIFICAR           │            │
│   ├────────────────────────┼────────────────────────┤            │
│   │ Energia BAIXA + ROI alto│ Energia BAIXA + ROI baixo│           │
│   │ → EXECUTAR COM CUIDADO   │ → DEFERIR               │            │
│   │   (gate CONSERVATIVE)    │   (recusar, manutenção) │            │
│   └────────────────────────┴────────────────────────┘            │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 8.2 Manutenção = Recuperação (o ponto de integração crítico)

As tasks de manutenção — **F9.1 Experience Compiler** e **F1.6 Cognitive Entropy check** — são classificadas como **recuperação**, não como produção:

```
manutenção_é_recuperação:
  classificação: task_type = "maintenance" → current_activity = "leve"
  custo: baixo (L1, 1 agente, ~1K tokens → ~13 unidades, §3.2)
  efeito: regenera contexto (reindexa energia)

  F9.1 (Experience Compiler):
    - Compila learnings brutos → padrões/princípios
    - Efeito energético: contexto futuro mais enxuto → tasks futuras gastam
      MENOS tokens (menos contexto para relembrar) → task_cost futuro menor
    - ROI (F2.1): negativo imediato, positivo massivo no reuso (mesmo padrão
      do F7-memorize-workflow na sessão real: -88.8% imediato)

  F1.6 (Cognitive Entropy check):
    - Detecta contradições, stale, gaps sem DDNA
    - Efeito energético: decisões futuras não repetem trabalho errado →
      elimina retrabalho (a maior drenagem de energia)
    - Entropia alta (0.433 real) → mais retrabalho → mais gasto → fadiga
```

**Regra de ouro da integração**: quando o sistema entra em CONSERVATIVE/RECOVERY, a manutenção **não é punida** — é a cura. As tasks F9.1/F1.6 custam pouco (~13), aplicam regen leve (+5/hora) e reduzem o custo futuro das tasks de produção. É o ciclo virtuoso:

```
Fadiga (energy < 40)
   ↓
RECOVERY: agenda F9.1 + F1.6
   ↓
Manutenção compila conhecimento + reduz entropia
   ↓
Contexto enxuto → task_cost futuro menor (menos tokens)
   ↓
Energy regen (+5/hora leve) + gasto futuro menor
   ↓
Sistema volta a FULL/FOCUSED
```

### 8.3 Troca de Dados

| Direção | Dado | Uso |
|---------|------|-----|
| F2.1 → F3.5 | `efficiency_score` | Gate: FOCUSED exige > 1.5 para P2/P3 |
| F2.1 → F3.5 | `estimated_cost` (tokens/agentes) | Pré-cálculo de task_cost |
| F3.5 → F2.1 | `energy_state` | F2.1 eleva thresholds de eficiência em estados baixos |
| F3.5 → F2.1 | `task_cost` real registrado | Calibration loop: F2.1 usa para ROI (custo real) |

---

## 9. Integração com F3.4 — Adaptive Personality

### 9.1 Princípio: energia baixa → Cauteloso/Zen

Energia baixa muda **como** o Cosca opera, não apenas **o que** aceita. A integração com a F3.4 mapeia cada estado de energia para um ajuste de personalidade — **menos velocidade, mais precisão**. Isso implementa o insight do Don: fadiga leva a decisões ruins; a personalidade compensa tornando o sistema mais cauteloso quando a energia escasseia.

### 9.2 Mapeamento Energia → Dimensões de Personalidade (F3.4)

O F3.4 define 6 dimensões (risk_tolerance, autonomy, creativity, rigor, verbosity, empathy). O mapeamento ajusta **deltas** sobre a personalidade base (não redefine o modo):

| Dimensão F3.4 | FULL | FOCUSED | CONSERVATIVE | RECOVERY |
|---------------|------|---------|--------------|----------|
| **risk_tolerance** | base (50) | −15 → 35 | −25 → 25 | −35 → 15 |
| **autonomy** | base (50) | −10 → 40 | −20 → 30 | −35 → 15 |
| **creativity** | base (50) | −10 → 40 | −25 → 25 | −30 → 20 |
| **rigor** | base (50) | +15 → 65 | +30 → 80 | +40 → 90 |
| **verbosity** | base (50) | −10 → 40 | −20 → 30 | −30 → 20 |
| **empathy** | base (50) | base | +5 | +5 |

**Interpretação** (mapeando para a semântica do F3.4):

```
CONSERVATIVE/RECOVERY = modo "Cauteloso/Zen":
  > rigor alto (80-90): toda afirmação requer evidência, verificação exaustiva
    → "Zen": menos velocidade, mais precisão
  > risk_tolerance baixo (15-25): só age com confiança ≥ 90%
  > creativity baixo (20-25): usa apenas padrões comprovados (gravidade ≥ 70)
  > verbosity baixo (20-30): respostas mínimas — economiza tokens também
  > autonomy baixo (15-30): requer confirmação do Don para ações impactantes
```

### 9.3 Fluxo de Ativação

```
F3.5 detecta transição de estado (ex: FOCUSED → CONSERVATIVE)
   ↓
Publica evento energy.state_changed { estado: conservative, fadiga_score: 0.62 }
   ↓
F3.4 aplica deltas sobre a personalidade ativa (modo base permanece)
   ↓
Próximas tasks executam com rigor 80, risk_tolerance 25, creativity 25
   ↓
F3.5 detecta retorno (energy ≥ 40) → publica state_changed { full }
   ↓
F3.4 restaura personalidade base
```

### 9.4 Por que isto é diferente de um "modo de humor"

O ajuste é **reversível, automático e baseado em evidência** (o estado de energia é um número rastreado). Não é o F3.4 inventando uma persona — é a F3.5 impondo um **limite operacional** que a personalidade respeita. O Don pode override com `cosca mode <modo>` (F3.4) — o modo manual prevalece sobre o ajuste automático.

---

## 10. Integração com o Ecossistema

### 10.1 Posição na Arquitetura

```
┌────────────────────────────────────────────────────────────────┐
│                  CAMADA TRANSVERSAL CONTINUA                    │
│                                                                │
│   F3.5 MENTAL ENERGY                                           │
│   ├── lê: F1.5 B1-B5 (carga, velocidade, freshness, reuse)     │
│   ├── lê: F2.5 momentum (velocidade de aprendizado)            │
│   ├── lê: F7.2 success_rate + avg_latency (saúde de agentes)   │
│   ├── lê: F2.1 efficiency_score (vale a pena?)                 │
│   ├── escreve: energy_state para F3.4 (personalidade)          │
│   ├── agenda: F9.1 + F1.6 em RECOVERY (manutenção)             │
│   └── publica: eventos energy.* no barramento interno          │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### 10.2 Tabela de Integrações

| Engine | Direção | Dado | Uso na F3.5 |
|--------|---------|------|-------------|
| **F2.1** Cognitive Economy | ← lê | efficiency_score | Gate de admissão (FOCUSED > 1.5) |
| **F2.1** | → escreve | energy_state | F2.1 eleva thresholds de eficiência |
| **F1.5** B1-B5 | ← lê | B1, B2 | Indicadores de fadiga F1 e F4 |
| **F2.5** Momentum | ← lê | momentum | Indicador de fadiga F3 |
| **F7.2** Trust Registry | ← lê | success_rate, avg_latency | Indicadores de fadiga F2 e F4 |
| **F3.4** Adaptive Personality | → escreve | energy_state_changed | Ajuste Cauteloso/Zen (§9) |
| **F9.1** Experience Compiler | agenda | manutenção | Recuperação em RECOVERY (§8.2) |
| **F1.6** Cognitive Entropy | agenda | manutenção | Recuperação em RECOVERY (§8.2) |
| **F1.5** B5 Cognitive Debt | ← lê | debt_rate | Amplificador de fadiga (dívida = energia mal gasta) |

### 10.3 Consumo de B1-B5 (padrão de agregação P-ARCH-005)

A F3.5 é **consumidora pura** de métricas existentes — nunca cria dados novos, apenas lê agregados do F1.5/F2.5/F7.2 e aplica aritmética. Mesmo padrão do F10.1/F10.2/F2.5: "lê agregados que já existem, nunca cria dados".

---

## 11. Regras Operacionais

### 11.1 As Três Leis da Energia

```
LEI 1 — SEGURANÇA SEMPRE:
  Task P0 (segurança, perda de dados, violação constitucional)
  NUNCA é bloqueada por energia — em qualquer estado, incluindo RECOVERY.

LEI 2 — RECOVERY É OBRIGATÓRIO:
  energy < 15 → o sistema NÃO pode operar tasks de produção.
  Executa F9.1 + F1.6 + reindexação. Não é sugestão — é a lei.

LEI 3 — DON OVERRIDE:
  cosca energy --refill  → reabastece energy para 100 (manual, exclusivo Don)
  O refill é registrado no histórico com timestamp e rationale.
```

### 11.2 Overdraft (energia negativa)

```
overdraft:
  quando: task P0 executada em RECOVERY esgota o que restava
  comportamento: energy fica negativa; RECOVERY estendido
  regra: 1 task P0 em overdraft é aceitável; 2+ em sequência é alerta P1
         → "Energia em overdraft após 2 tasks P0. Manutenção estendida."
  recuperação: regen + manutenção até energy ≥ 15
  nunca: overdraft autoriza tasks P1/P2/P3 — apenas P0
```

### 11.3 Hierarquia de Autoridade

```
1. Don (override total: --refill, aceitar task recusada)
2. Segurança P0 (bloqueada apenas por P0 mais crítico — nenhum caso conhecido)
3. F3.5 Mental Energy (estados, gates, RECOVERY)
4. F2.1 Cognitive Economy (efficiency thresholds)
5. F3.4 Adaptive Personality (deltas de comportamento)
```

---

## 12. Exemplo Real — Sessão de 12 Ondas

### 12.1 Cenário

A sessão de arquitetura cognitiva (Evolution Marathon) — **12 ondas, 30+ agentes, ~50 tasks**. Este é o cenário de estresse que motivou a F3.5.

### 12.2 Cálculo Agregado (dados do Don)

```
GASTO:
  gasto médio por task ≈ 15 unidades (tasks majoritariamente L1-L2,
  1-2 agentes, ~2-10K tokens — ver breakdown §12.3)
  50 tasks × 15 = 750 unidades gastas

REGEN:
  10/hora idle × 8h = 80 unidades recuperadas
  (assumindo janelas idle entre ondas; se intensa contínua, regen = 0)

CAPACIDADE:
  energy máxima = 100

BALANÇO LÍQUIDO:
  750 (gasto) − 80 (regen) = 670 unidades de déficit
  → energia líquida NEGATIVA (-570 vs capacidade de 100)
  → o sistema DEVERIA entrar em RECOVERY no fim
    (reindexação + compactação: F9.1 + F1.6)
```

### 12.3 Breakdown Realista por Onda

| Ondas | Tasks | Tipo predominante | Custo médio estimado | Gasto total |
|-------|-------|-------------------|----------------------|-------------|
| 1-3 (foundation) | 12 | L2, 2 agentes, ~8K tokens | ~25 | 300 |
| 4-6 (engines F1-F2) | 14 | L3, 3 agentes, ~15K tokens | ~48 | 672 |
| 7-9 (engines F3) | 14 | L2, 2 agentes, ~10K tokens | ~31 | 434 |
| 10-12 (integração) | 10 | L1-L2, 1-2 agentes, ~5K tokens | ~20 | 200 |
| **Total** | **50** | | | **~1606** |

> **Nota honesta**: o breakdown real (L2-L3, multi-agente) produz custo médio MUITO acima do 15/task do exemplo do Don. Os dois números contam histórias complementares: o exemplo do Don (15/task × 50 = 750) é a versão otimista (tasks leves, 1-2 agentes); o breakdown real (1606) é a versão pessimista. **Ambos levam à mesma conclusão**: uma sessão de 12 ondas consome de 7 a 16× a capacidade energética de 100, com regen incapaz de compensar (80-0). A conclusão arquitetural é invariante.

### 12.4 Diagnóstico do Engine (o que a F3.5 diria)

```
SIMULAÇÃO (com regen de 10/hora entre ondas):

  Ondas 1-2:  energy 100 → 50   (6 tasks × ~25 = 150 gasto + 20 regen)
  Ondas 3-4:  energy 50 → 25    (FOCUSED → CONSERVATIVE na onda 4)
  Onda 4:     GATE RECUSA P2/P3 — "Energia 25/100 (CONSERVATIVE)"
  Ondas 5-6:  Só P0/P1 + manutenção; energy 25 → 35 (regen leve)
  Onda 7:     energy 35 → 30 → CONSERVATIVE persistente
  Ondas 8-12: RECOVERY forçado em algum ponto (energy < 15)

  VEREDICTO:
  "A sessão de 12 ondas é energeticamente insustentável.
   Com energy=100 e gasto médio 15-32/task, a capacidade
   produtiva real é de 3-6 tasks antes de CONSERVATIVE.
   O sistema DEVE intercalar manutenção (F9.1 + F1.6) entre
   ondas — ou a fadiga degrada o contexto e as decisões."

  RECOMENDAÇÃO:
  1. Dividir a sessão em blocos de ~4-5 tasks produtivas
  2. Entre blocos: 1 manutenção leve (regen +5/hora, contexto enxuto)
  3. Fim da sessão: RECOVERY obrigatório (reindexação + compactação)
  4. Se fadiga_score ≥ 0.50 (erros, momentum caindo): parar antes
```

### 12.5 Lição Arquitetural do Exemplo

A sessão de 12 ondas **funcionou na prática** — mas à custa de energia negativa não contabilizada. A F3.5 torna esse custo visível e transforma "a sessão terminou exausta" em "a sessão terminou **no momento certo**". A reindexação + compactação que o Don prevê no fim não é limpeza — é **reparação obrigatória de fadiga**.

---

## 13. Armazenamento e CLI

### 13.1 Estado Persistido

```yaml
# engines/mental-energy/state/energy-state.yaml
energy_state:
  version: "2.0.0"
  session_id: "session-2026-07-30-001"
  energy: 62                    # 0-100, pode ser negativo (overdraft)
  estado: "FOCUSED"             # FULL | FOCUSED | CONSERVATIVE | RECOVERY
  current_activity: "intensa"   # idle | leve | intensa
  last_task_cost: 14.0
  avg_task_cost_recent: [14, 31, 77, 13, 22, 18, 15, 12, 19, 16]  # janela 10
  fadiga_score: 0.34
  fadiga_detail: { f1_carga: 0.2, f2_erros: 0.5, f3_momentum: 0.1, f4_latencia: 0.3 }
  last_regen_at: "2026-07-30T18:00:00Z"
  last_persist: "2026-07-30T18:05:00Z"
```

### 13.2 Histórico (CSV append-only — padrão de imutabilidade P4)

```csv
# engines/mental-energy/state/energy-history.csv
timestamp,event,energy,delta,estado,fadiga_score,detail
2026-07-30T14:00:00Z,init,100,0,FULL,0.00,cold_start
2026-07-30T14:32:00Z,task_done,86,-14,FULL,0.05,"F0-cosca-chat"
2026-07-30T15:10:00Z,task_done,55,-31,FOCUSED,0.22,"F2-tool-system"
2026-07-30T16:00:00Z,regen,65,10,FOCUSED,0.20,"idle_period"
2026-07-30T17:45:00Z,task_done,34,-31,CONSERVATIVE,0.51,"F7-memorize-workflow"
2026-07-30T18:00:00Z,refill,100,66,FULL,0.10,"don_override"
```

### 13.3 CLI

```bash
# Status atual da energia (dashboard resumido)
cosca energy status
# → "energia 62/100 — FOCUSED — 3 tasks de ~12 gasto cada = fim de sessão em 2 tasks"

# Estimar custo de uma task antes de executar
cosca energy estimate --complexity 3 --agents 2 --tokens 10000
# → "custo estimado: 31 unidades (0.4×15 + 2×10 + 1×5)"

# Forçar reabastecimento (override do Don — Lei 3)
cosca energy --refill

# Histórico de energia
cosca energy history --limit 20

# Estado detalhado (fadiga, janela de custos)
cosca energy status --verbose

# Simular cenário: "e se eu aceitar esta task P2 agora?"
cosca energy simulate --priority P2 --complexity 3 --agents 3
```

---

## 14. Edge Cases e Cold Start

### 14.1 Cold Start (sem histórico)

```
energy = 100, avg_task_cost_recent = [] (vazio)
comportamento: projeção usa a tabela de referência (§3.2) até a janela
de 10 tasks ser preenchida. Estado inicial FULL.
fadiga_score = 0 (sem dados B1/F7.2/F2.5 suficientes → neutro, não zero
por "saúde", mas por "desconhecido" — marcado no relatório).
```

### 14.2 Falha de Fonte (CSV ausente/corrompido)

```
regra: nenhuma fonte é bloqueante. Se B1 CSV ausente → f1_carga = 0.3 (neutral).
Se todas as 4 fontes falham → fadiga_score = 0.5 (presumir fadiga moderada
é mais seguro que presumir saúde — a energia não pode ser falsamente alta).
```

### 14.3 Sessão Interrompida (crash)

```
grace_recovery: estado persistido com staleness > 24h → tratar como cold start
(energy = 100). Sessão interrompida com staleness < 24h → retomar do estado
persistido, marcando fadiga_score +0.1 (interrupção = estresse).
```

### 14.4 Don Pedindo Task P2 em RECOVERY

```
fluxo: gate recusa → Don override (aceitar) → registra no histórico
com rationale obrigatório → energy entra em overdraft se necessário →
RECOVERY estendido. O Don vence, mas o custo fica visível.
```

### 14.5 Tasks de Manutenção em FULL (energia alta)

```
manutenção em FULL: executa normalmente (custa ~13, regen leve não se aplica
em intensa... mas manutenção classifica como "leve" → +5/hora). Manutenção
preventiva em FULL é incentivada — reduz entropia antes que ela vire fadiga.
```

---

## 15. Métricas do Próprio Engine

```yaml
engine_self_metrics:
  prediction_accuracy:
    cost_estimation_error: "|custo estimado - custo real| / custo real (alvo < 25%)"
    projection_accuracy: "% das projeções de fim de sessão que se concretizaram (±1 task)"
  state_transition_accuracy: "transições previstas vs reais (alvo > 80%)"
  session_health:
    sessions_without_recovery: "sessões que NÃO precisaram de RECOVERY (alvo: maioria)"
    avg_final_energy: "energia média ao fim da sessão (alvo: 15-40 = usou bem, sem estourar)"
    wasted_sessions: "sessões que terminaram > 70 (poderiam ter produzido mais)"
    exhausted_sessions: "sessões que terminaram < 0 (overdraft)"
  refill_rate: "refills do Don por semana (alvo < 2 — refill frequente = thresholds errados)"
  fadiga_catch_rate: "% de fadiga detectada ANTES de erro real (alvo > 70%)"
```

---

## 16. Referências Cruzadas

| Documento | Seção | Relação |
|-----------|-------|---------|
| COGNITIVE_MATURITY.md | §C7 | Conceito original de Mental Energy |
| cognitive-maturity-implementation.md | F3.5 | Tarefa de implementação |
| [cognitive-economy/SKILL.md](../../engines/cognitive-economy/SKILL.md) | F2.1 | ROI por task — gate de admissão + calibration |
| [cognitive-metrics.md](../../analytics/cognitive-metrics.md) | F1.5 | B1-B5 — fontes dos indicadores de fadiga |
| [cognitive-momentum/SKILL.md](../../engines/cognitive-momentum/SKILL.md) | F2.5 | Momentum — indicador de fadiga F3 |
| [TRUST_REGISTRY.md](../../memory/trust/TRUST_REGISTRY.md) | F7.2 | success_rate + avg_latency — indicadores F2/F4 |
| [adaptive-personality/SKILL.md](../../engines/adaptive-personality/SKILL.md) | F3.4 | Mapeamento energia → Cauteloso/Zen (§9) |
| [experience-compiler/SKILL.md](../../engines/experience-compiler/SKILL.md) | F9.1 | Manutenção de recuperação (§8.2) |
| [cognitive-entropy/ENTROPY.md](../../engines/cognitive-entropy/ENTROPY.md) | F1.6 | Manutenção de recuperação (§8.2) |
| COGNITIVE_MATURITY.md | §5 | CMI Impact (Julgamento +4, Planejamento +4, Eficiência +6) |
| CONVENTIONS.md | Engine Skills | Contrato de formato deste documento |
| CONSTITUTION.md | P4 | Imutabilidade do histórico (CSV append-only) |

---

## 17. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Runtime Chief | Especificação inicial. 5 pools de energia (TOKENS, TIME, ATTENTION, DEPTH, PARALLEL). 4 estados (HIGH, MEDIUM, LOW, CRITICAL). Energy Gate com pré-verificação. Energy Debt com juros. Matriz 4 quadrantes com F2.1. Dashboard ASCII. Perfis de orçamento. |
| **2.0.0** | **2026-07-30** | **Cosca Architecture Chief** | **★ F3.5 — Reformulação completa para o modelo unificado do Don.** Energia única 0-100. Fórmula literal do Don: `task_cost = complexity×15 + agents×10 + tokens/10K×5` e `regen = 10/hora idle + 5/hora leve + 0/hora intensa`. 4 estados renomeados (FULL > 70, FOCUSED 40-70, CONSERVATIVE 15-40, RECOVERY < 15). Pipeline contínuo de 6 passos < 10ms com projeção de fim de sessão. 4 indicadores de fadiga (B1 carga, F7.2 success_rate, F2.5 momentum, latência) com fadiga_score modulando o gate. Integração F2.1: manutenção F9.1/F1.6 como recuperação (ciclo virtuoso). Integração F3.4: mapa energia → Cauteloso/Zen por dimensões de personalidade. 3 leis (P0 nunca bloqueado, RECOVERY obrigatório, Don override `--refill`). Exemplo real: sessão de 12 ondas → déficit de 670 unidades → RECOVERY obrigatório no fim. CLI, armazenamento YAML/CSV append-only. |