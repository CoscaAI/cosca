# WISDOM DISTILLATION ENGINE — Constitutional Funnel (F9.2)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência (+0.06), Julgamento (+0.05) | **Bloco Cognitivo**: Bloco 4 — Metacognição & Governança
> **Fase CMI**: Fase 9 — Evolution | **Código**: F9.2
> **Dependências**: F9.1 Experience Compiler | F1.4 Wisdom Decay | CONSTITUTION.md
>
> Consulte também:
> - [EXPERIENCE_COMPILER.md](../experience-compiler/SKILL.md) — F9.1: compilação de learnings em padrões e princípios
> - [WISDOM_DECAY.md](../wisdom-decay/WISDOM_DECAY.md) — F1.4: freshness score, decay types, revalidação
> - [CONSTITUTION.md](../../CONSTITUTION.md) — autoridade máxima, 8 princípios imutáveis, cadeia de comando
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — formato de aprendizado, negative memory, confidence scoring
> - [SKILL_TEMPLATE.md](../../SKILL_TEMPLATE.md) — template de skill engine

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [Pipeline de Destilação (5 Níveis)](#2-pipeline-de-destilação-5-níveis)
3. [Algoritmo de Agrupamento](#3-algoritmo-de-agrupamento)
4. [Taxa de Compressão](#4-taxa-de-compressão)
5. [Integração com F9.1 Experience Compiler](#5-integração-com-f91-experience-compiler)
6. [Integração com F1.4 Wisdom Decay](#6-integração-com-f14-wisdom-decay)
7. [Integração com CONSTITUIÇÃO](#7-integração-com-constituição)
8. [Fórmula de Maturidade para Princípios Constitucionais](#8-fórmula-de-maturidade-para-princípios-constitucionais)
9. [Exemplo Completo](#9-exemplo-completo)
10. [CLI e Automação](#10-cli-e-automação)
11. [Métricas do Engine](#11-métricas-do-engine)
12. [Casos de Borda e Anti-Padrões](#12-casos-de-borda-e-anti-padrões)

---

## 1. DEFINIÇÃO

### 1.1 O que é a Wisdom Distillation

A **Wisdom Distillation** é o funil de sabedoria do Cosca. Ela pega o output do Experience Compiler (F9.1) — centenas de padrões, princípios candidatos e propostas de emenda — e **destila ainda mais**, reduzindo a complexidade por um fator de ~100:1 até chegar a emendas constitucionais acionáveis.

Enquanto o Experience Compiler transforma **learnings brutos em padrões e princípios técnicos**, a Wisdom Distillation transforma **princípios técnicos em sabedoria constitucional** — regras fundacionais que alteram a CONSTITUIÇÃO.

### 1.2 Analogia: Destilaria

```
┌─────────────────────────────────────────────────────────────────────────┐
│                WISDOM DISTILLATION — ANALOGIA DE DESTILARIA               │
│                                                                          │
│  500L de mosto (learnings)  →  1ª destilação → 50L de vinho (grupos)   │
│  50L de vinho (grupos)      →  2ª destilação → 15L de conhaque (padrões)│
│  15L de conhaque (padrões)  →  3ª destilação → 5L de essência (princ.)  │
│  5L de essência (princípios)→  4ª destilação → 1-2L de perfume (emenda) │
│                                                                          │
│  Cada destilação remove:                                                 │
│    - Água (redundância)                                                  │
│    - Congêneres (ruído, falso positivo)                                  │
│    - Taninos (especificidade excessiva)                                  │
│                                                                          │
│  O que sobra é a ESSÊNCIA — o que realmente importa para a CONSTITUIÇÃO  │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.3 Propósito

| Por que destilar | O que resolve | Ação resultante |
|------------------|--------------|-----------------|
| Experience Compiler gera dezenas de outputs semanais | 20+ padrões/semana é informação, não sabedoria | Reduzir para 1-2 emendas/mês |
| Princípios imaturos não devem virar lei | Maturidade 0.7-0.9 é princípio técnico, não constitucional | Só o que atinge maturidade > 0.9 vira emenda |
| Conhecimento redundante polui a constituição | Múltiplos princípios sobre o mesmo tema | Agrupar por afinidade temática |
| A constituição deve evoluir devagar | Mudanças muito frequentes desestabilizam o sistema | Máximo 1-2 emendas por mês |

### 1.4 Outputs da Destilação

| Output | Formato | Consumidor | Frequência |
|--------|---------|------------|------------|
| **Grupos Temáticos** | JSON com 50 grupos, score de coesão | F9.2 (próximo nível) | Semanal |
| **Padrões Constitucionais** | Markdown em `knowledge/constitutional-patterns/` | Architecture Chief + CTO | Semanal |
| **Princípios Constitucionais Candidatos** | Proposta de 5 princípios com evidências | Don (revisão) | Mensal |
| **Emendas Constitucionais** | Proposta de alteração da CONSTITUIÇÃO | Don (aprovação) | Mensal (máx. 2) |

---

## 2. PIPELINE DE DESTILAÇÃO (5 NÍVEIS)

O pipeline roda **semanalmente** (após o Experience Compiler) ou **sob demanda** via CLI. Tempo alvo: **< 30s** para processar o output completo de 54 agentes.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                  WISDOM DISTILLATION — PIPELINE COMPLETO                       │
│                             5 Níveis de Destilação                              │
│                                                                                │
│  ENTRADA: Output do F9.1 Experience Compiler                                   │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ NÍVEL 1 — COLEÇÃO                                                         │  │
│  │                                                                           │  │
│  │  Input: 500+ learnings de 54 agentes (já filtrados por freshness > 0.5) │  │
│  │                                                                           │  │
│  │  1. Recebe todos os learnings do F9.1 que passaram pela Fase 1           │  │
│  │     (freshness > 0.5)                                                     │  │
│  │  2. Inclui também os learnings que NÃO formaram grupo no F9.1             │  │
│  │     (podem ser úteis em grupos diferentes nesta destilação)               │  │
│  │  3. Adiciona learnings de failures.md que geraram aprendizado             │  │
│  │  4. Adiciona padrões e princípios do F9.1 (Fase 3 output)                │  │
│  │                                                                           │  │
│  │  Saída: ~500 learnings frescos + ~20 padrões + ~5 princípios             │  │
│  │  ⏱ Tempo alvo: < 3s                                                     │  │
│  └───────────────────────────────────┬──────────────────────────────────────┘  │
│                                       ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ NÍVEL 2 — GRUPOS TEMÁTICOS (50 grupos)                                   │  │
│  │                                                                           │  │
│  │  Agrupa por (domain + tags + similaridade semântica):                     │  │
│  │                                                                           │  │
│  │  1. Para cada learning, extrai:                                           │  │
│  │     ├── Domínio primário (architecture, security, testing, etc.)          │  │
│  │     ├── Tags (lista completa de tags do learning)                         │  │
│  │     └── Afirmação central (o que foi aprendido)                           │  │
│  │                                                                           │  │
│  │  2. Calcula distância semântica entre pares:                              │  │
│  │     ├── Tag overlap: Jaccard(tags_a, tags_b)                              │  │
│  │     ├── Domínio match: 1.0 mesmo domínio, 0.5 relacionado, 0.0 diferente │  │
│  │     └── Similaridade = 0.6 × tag_jaccard + 0.4 × domain_match           │  │
│  │                                                                           │  │
│  │  3. Se similaridade > 0.45 → mesmo grupo                                 │  │
│  │     (threshold mais alto que F9.1 — destilação mais agressiva)            │  │
│  │                                                                           │  │
│  │  4. Mínimo 3 learnings por grupo, mínimo 2 agentes diferentes            │  │
│  │                                                                           │  │
│  │  5. Máximo 50 grupos (se exceder, merge dos grupos menores)              │  │
│  │                                                                           │  │
│  │  Saída: ~50 grupos temáticos com score de coesão                          │  │
│  │  ⏱ Tempo alvo: < 10s (O(N²) com N ≈ 500)                               │  │
│  └───────────────────────────────────┬──────────────────────────────────────┘  │
│                                       ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ NÍVEL 3 — PADRÕES CONSTITUCIONAIS (15 padrões)                            │  │
│  │                                                                           │  │
│  │  Para cada grupo temático:                                               │  │
│  │                                                                           │  │
│  │  1. Extrai o padrão central do grupo:                                     │  │
│  │     ├── Identifica a afirmação comum a TODOS os learnings do grupo        │  │
│  │     ├── Se divergem → marcado como DIVERGENT (não gera padrão)           │  │
│  │     ├── Se contradizem → marcado como CONFLICT (escala para Don)         │  │
│  │     └── Se convergentes → sintetiza padrão                                │  │
│  │                                                                           │  │
│  │  2. Valida contra CONSTITUIÇÃO atual:                                     │  │
│  │     ├── Padrão contradiz P1-P8? → REJEITADO com justificativa            │  │
│  │     ├── Padrão complementa princípio existente? → APROVADO               │  │
│  │     └── Padrão é redundante com princípio existente? → MERGE ou REJEITAR │  │
│  │                                                                           │  │
│  │  3. Calcula maturidade constitucional (ver §8)                            │  │
│  │                                                                           │  │
│  │  4. Os 15 padrões com maior maturidade são selecionados                  │  │
│  │     (corte nos 15 melhores, mesmo que mais existam)                       │  │
│  │                                                                           │  │
│  │  Saída: ~15 padrões constitucionais com maturity_score                   │  │
│  │  ⏱ Tempo alvo: < 5s                                                     │  │
│  └───────────────────────────────────┬──────────────────────────────────────┘  │
│                                       ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ NÍVEL 4 — PRINCÍPIOS CONSTITUCIONAIS (5 princípios)                       │  │
│  │                                                                           │  │
│  │  Dos 15 padrões, destila para 5 princípios:                              │  │
│  │                                                                           │  │
│  │  1. Aplica compressão por afinidade:                                      │  │
│  │     ├── Padrões no mesmo domínio → merge (se não contraditórios)         │  │
│  │     ├── Padrões complementares → fundir em princípio único               │  │
│  │     └── Padrões que são casos específicos de outro → subsumir            │  │
│  │                                                                           │  │
│  │  2. Para cada princípio candidato:                                        │  │
│  │     ├── Verifica se é ACIONÁVEL (prescreve ação)                          │  │
│  │     ├── Verifica se é FUNDACIONAL (afeta toda a plataforma)               │  │
│  │     ├── Verifica se é DURADOURO (não é moda passageira)                   │  │
│  │     └── Calcula maturity_score constitucional                             │  │
│  │                                                                           │  │
│  │  3. Seleciona os 5 princípios com maior maturity_score                   │  │
│  │                                                                           │  │
│  │  4. Submete para aprovação do Don:                                        │  │
│  │     ├── Don pode APROVAR, REJEITAR ou MODIFICAR                           │  │
│  │     ├── Cada princípio tem 7 dias para resposta                           │  │
│  │     └── Princípios rejeitados voltam para o pool (próximo ciclo)          │  │
│  │                                                                           │  │
│  │  Saída: ~5 princípios constitucionais candidatos (status: pending)        │  │
│  │  ⏱ Tempo alvo: < 5s (depende de aprovação do Don, que é assíncrono)     │  │
│  └───────────────────────────────────┬──────────────────────────────────────┘  │
│                                       ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ NÍVEL 5 — EMENDAS CONSTITUCIONAIS (1-2 emendas)                           │  │
│  │                                                                           │  │
│  │  Dos 5 princípios aprovados pelo Don:                                    │  │
│  │                                                                           │  │
│  │  1. Princípios com maturity > 0.9 → candidatos a emenda                  │  │
│  │  2. Máximo 2 emendas por ciclo (para não sobrecarregar a CONSTITUIÇÃO)   │  │
│  │  3. Gera diff da CONSTITUIÇÃO:                                            │  │
│  │     ├── Número sequencial (P9, P10, ...)                                  │  │
│  │     ├── Texto do princípio                                                │  │
│  │     ├── Aplicação prática                                                 │  │
│  │     ├── Quem garante                                                     │  │
│  │     └── Referência cruzada com DDNA                                      │  │
│  │                                                                           │  │
│  │  4. Se mais de 2 candidatos → Don escolhe quais entram                   │  │
│  │  5. Se nenhum candidato → ciclo vazio (constituição estável)             │  │
│  │  6. Emendas são incrementais: versão da CONSTITUIÇÃO sobe (ex: 1.2.0)     │  │
│  │                                                                           │  │
│  │  Saída: 1-2 emendas aplicadas à CONSTITUIÇÃO.md                           │  │
│  │  ⏱ Tempo alvo: < 2s (escrita de arquivo)                                │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                                │
│  OUTPUT FINAL: CONSTITUIÇÃO atualizada (versão incrementada)                   │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Algoritmo Central

```python
def run_wisdom_distillation():
    # === NÍVEL 1: COLETA ===
    # Recebe input do F9.1 Experience Compiler
    all_inputs = []

    # Learnings frescos do F9.1
    for agent_path in glob("memory/agent/*/learnings.md"):
        learnings = parse_learnings_file(agent_path)
        fresh = [l for l in learnings if l.freshness > 0.5]
        all_inputs.extend(fresh)

    # Learnings de failures.md
    for agent_path in glob("memory/agent/*/failures.md"):
        failures = parse_failures_file(agent_path)
        for f in failures:
            if f.lessons_learned and f.freshness > 0.5:
                all_inputs.append(f.as_learning())

    # Padrões e princípios do F9.1 (Fase 3 output)
    patterns = load_f91_patterns()        # ~20 padrões
    principles = load_f91_principles()    # ~5 princípios
    all_inputs.extend(patterns)
    all_inputs.extend(principles)

    # === NÍVEL 2: GRUPOS TEMÁTICOS ===
    groups = []
    N = len(all_inputs)  # ~500

    for i, a in enumerate(all_inputs):
        if a.freshness < 0.5:
            continue  # F1.4: conhecimento podre
        for b in all_inputs[i+1:]:
            similarity = calc_semantic_distance(a, b)
            if similarity > 0.45:
                add_to_group(groups, a, b, similarity)

    # Filtra: mínimo 3 learnings, mínimo 2 agentes
    valid_groups = [g for g in groups
                    if len(g.items) >= 3
                    and len(set(g.agents)) >= 2]

    # Ordena por coesão, pega top 50
    valid_groups.sort(key=lambda g: g.cohesion_score, reverse=True)
    top_groups = valid_groups[:50]

    # === NÍVEL 3: PADRÕES CONSTITUCIONAIS ===
    constitutional_patterns = []
    for group in top_groups:
        pattern = extract_constitutional_pattern(group)
        if pattern is None:
            continue  # DIVERGENT ou CONFLICT

        # Valida contra CONSTITUIÇÃO
        if violates_constitution(pattern):
            continue

        pattern.maturity = calc_constitutional_maturity(group)
        constitutional_patterns.append(pattern)

    # Seleciona top 15
    constitutional_patterns.sort(key=lambda p: p.maturity, reverse=True)
    top_patterns = constitutional_patterns[:15]

    # === NÍVEL 4: PRINCÍPIOS CONSTITUCIONAIS ===
    constitutional_principles = compress_to_principles(top_patterns)
    # compress_to_principles faz merge por afinidade

    for p in constitutional_principles:
        if not p.is_actionable:
            continue
        if not p.is_foundational:
            continue

    # Seleciona top 5
    constitutional_principles.sort(key=lambda p: p.maturity, reverse=True)
    top_principles = constitutional_principles[:5]

    # Submete para Don
    for p in top_principles:
        submit_to_don(p)

    # === NÍVEL 5: EMENDAS ===
    approved = [p for p in top_principles if p.don_approved]
    amendments = []

    for p in approved:
        if p.maturity > 0.9 and len(amendments) < 2:
            ddna = generate_amendment_ddna(p)
            amendment = apply_amendment_to_constitution(p)
            amendments.append(amendment)
            update_constitution(amendment)

    # Gera relatório
    report = generate_report(all_inputs, top_groups, top_patterns,
                             top_principles, amendments)
    save_distillation_report(report)

    return report
```

### 2.2 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|-------------|----------------|
| Coleta de inputs (~500 learnings) | O(N) | < 3s |
| Agrupamento semântico (N²) | O(N²) | < 10s |
| Extração de padrões | O(G) | < 5s |
| Compressão para princípios | O(P × K) | < 5s |
| Geração de emendas | O(A) | < 2s |
| **Total** | | **< 25s** |

> **N = total de inputs, G = grupos (≤50), P = padrões (≤15), K = afinidade, A = emendas (≤2)**

---

## 3. ALGORITMO DE AGRUPAMENTO

### 3.1 Pseudocódigo

```python
def group_learnings(all_learnings):
    """
    Agrupa learnings por (domain, tags) com filtros de qualidade.
    Retorna dicionário {group_key: [learning, ...]}.
    """
    candidates = []

    for learning in all_learnings:
        # Filtro F1.4: só conhecimento fresco
        if learning.freshness < 0.5:
            continue  # conhecimento podre — skip

        # Gera chave de agrupamento: (domínio, tags canônicas)
        domain = learning.domain
        primary_tag = learning.tags[0] if learning.tags else "general"
        group_key = (domain, primary_tag)

        candidates.append({
            "key": group_key,
            "learning": learning
        })

    # Agrupa por chave
    raw_groups = {}
    for c in candidates:
        key = c["key"]
        if key not in raw_groups:
            raw_groups[key] = []
        raw_groups[key].append(c["learning"])

    # Filtra: mínimo 3 evidências
    valid_groups = {}
    for key, items in raw_groups.items():
        if len(items) >= 3:
            # Verifica minimum agent diversity
            agents = set(i.agent_id for i in items)
            if len(agents) >= 2:
                valid_groups[key] = items

    return valid_groups
```

### 3.2 Algoritmo de Similaridade Semântica

```python
def calc_semantic_distance(a, b):
    """
    Calcula distância semântica entre dois learnings.
    Retorna score 0.0-1.0 (maior = mais similar).
    """
    # 1. Tag overlap (Jaccard)
    tags_a = set(a.tags)
    tags_b = set(b.tags)
    intersection = tags_a & tags_b
    union = tags_a | tags_b
    tag_jaccard = len(intersection) / len(union) if union else 0.0

    # 2. Domain match
    domain_score = 1.0 if a.domain == b.domain else 0.0

    # 3. Cross-reference match
    # Se um learning referencia o outro, é forte indicador de similaridade
    cross_ref = 1.0 if b.id in (a.references or []) else 0.0

    # 4. Peso final
    similarity = (tag_jaccard * 0.5 +
                  domain_score * 0.3 +
                  cross_ref * 0.2)

    # Bonus: mesmo agente reduz similaridade (evitar viés de fonte única)
    if a.agent_id == b.agent_id:
        similarity *= 0.8

    return similarity
```

### 3.3 Score de Coesão do Grupo

```python
def calc_group_cohesion(group):
    """
    Mede o quão coeso é um grupo temático.
    Retorna score 0.0-1.0.
    """
    if len(group.items) < 3:
        return 0.0

    # Coesão de tags: média do Jaccard entre todos os pares
    tag_scores = []
    for i, a in enumerate(group.items):
        for b in group.items[i+1:]:
            tags_a = set(a.tags)
            tags_b = set(b.tags)
            intersection = tags_a & tags_b
            union = tags_a | tags_b
            tag_scores.append(len(intersection) / len(union) if union else 0.0)
    tag_cohesion = sum(tag_scores) / len(tag_scores) if tag_scores else 0.0

    # Diversidade de agentes (mais agentes = mais robusto)
    agents = set(i.agent_id for i in group.items)
    agent_diversity = min(len(agents) / 5.0, 1.0)  # Cap: 5 agentes

    # Freshness médio
    avg_freshness = sum(i.freshness for i in group.items) / len(group.items)

    # Score composto
    cohesion = (tag_cohesion * 0.4 +
                agent_diversity * 0.3 +
                avg_freshness * 0.3)

    return cohesion
```

### 3.4 Regras de Agrupamento

| Regra | Condição | Ação |
|-------|----------|------|
| **Mínimo de evidências** | < 3 learnings no grupo | Grupo descartado |
| **Diversidade de agentes** | < 2 agentes diferentes | Grupo marcado como `#single-source` — não gera padrão |
| **Freshness mínimo** | Qualquer learning com freshness ≤ 0.5 | Learning excluído (F1.4) |
| **Coesão mínima** | Cohesion < 0.4 | Grupo não gera padrão constitucional |
| **Grupo divergente** | Learnings do grupo contradizem entre si | Grupo marcado como `#divergent` — escala para Don |
| **Grupo muito grande** | > 20 learnings no grupo | Subdividir em 2+ subgrupos por subtópico |
| **Sobreposição com F9.1** | Padrão já registrado em PRINCIPLES.md | Fundir evidências, não duplicar |

---

## 4. TAXA DE COMPRESSÃO

### 4.1 Funnel de Compressão

A Wisdom Distillation aplica compressão progressiva em 4 estágios, cada um com sua taxa de compressão específica:

```
NÍVEL 1 → NÍVEL 2         NÍVEL 2 → NÍVEL 3     NÍVEL 3 → NÍVEL 4    NÍVEL 4 → NÍVEL 5
500 learnings → 50 grupos  50 grupos → 15 padrões  15 padrões → 5 princ.  5 princ. → 1-2 emendas
   ~10:1                       ~3.3:1                  ~3:1                  ~3:1 (ou 5:1)

┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              FUNIL DE DESTILAÇÃO                                      │
│                                                                                       │
│  500+                                                                                  │
│  learnings ─────────────────────────────────────────────────────────────────────────  │
│    │                                                                                   │
│    │  ~10:1  Agrupa por (domain + tags). Remove redundância.                           │
│    │          Filtra freshness < 0.5. Remove grupos < 3.                               │
│    ▼                                                                                   │
│   50                                                                                   │
│  grupos ────────────────────────────────────────────────────────────────────────────  │
│    │                                                                                   │
│    │  ~3.3:1  Extrai padrão central de cada grupo. Valida contra CONSTITUIÇÃO.         │
│    │           Remove padrões divergentes. Seleciona top 15.                           │
│    ▼                                                                                   │
│   15                                                                                   │
│  padrões ──────────────────────────────────────────────────────────────────────────  │
│    │                                                                                   │
│    │  ~3:1  Compressão por afinidade. Merge de padrões complementares.                │
│    │          Remove não-acionáveis. Seleciona top 5.                                  │
│    ▼                                                                                   │
│    5                                                                                   │
│  princípios ────────────────────────────────────────────────────────────────────────  │
│    │                                                                                   │
│    │  ~3:1 (ou 5:1)  Só maturity > 0.9 vira emenda. Don aprova/rejeita.              │
│    │                   Máximo 2 emendas por ciclo.                                     │
│    ▼                                                                                   │
│  1-2                                                                                   │
│  emendas ──────────────────────────────────────────────────────────────────────────  │
│                                                                                       │
│  COMPRESSÃO TOTAL: 500 → 2.5 (médio) = ~200:1                                         │
│  COMPRESSÃO TOTAL: 500 → 5 (emendas + princípios) = ~100:1                            │
│                                                                                       │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Tabela de Compressão

| Nível | Input | Output | Taxa | Critério de Corte |
|-------|-------|--------|:----:|-------------------|
| **N1 → N2** (Coleção → Grupos) | 500+ learnings | 50 grupos | **~10:1** | Freshness > 0.5, mínimo 3 evidências, mínimo 2 agentes, coesão > 0.4 |
| **N2 → N3** (Grupos → Padrões) | 50 grupos | 15 padrões | **~3.3:1** | Validação contra CONSTITUIÇÃO, maturity > 0.5, top 15 por maturidade |
| **N3 → N4** (Padrões → Princípios) | 15 padrões | 5 princípios | **~3:1** | Compressão por afinidade, acionável, fundacional, top 5 |
| **N4 → N5** (Princípios → Emendas) | 5 princípios | 1-2 emendas | **~3:1** | Don aprova, maturity > 0.9, máx. 2/ciclo |
| **Total** | 500+ learnings | 1-2 emendas | **~100:1** a **~200:1** | Todo o funil |

### 4.3 Eficiência da Compressão

```python
compression_efficiency = (input_count - output_count) / input_count

# Exemplo real:
# input = 500 learnings, output = 5 princípios (incluindo emendas)
# efficiency = (500 - 5) / 500 = 0.99 → 99% de compressão ✅
```

| Eficiência | Interpretação | Ação |
|------------|---------------|------|
| > 95% | Compressão saudável | Normal |
| 80-95% | Compressão moderada | Pode haver redundância entre princípios |
| < 80% | Compressão baixa | Revisar pipeline — grupos podem estar fragmentados |

---

## 5. INTEGRAÇÃO COM F9.1 EXPERIENCE COMPILER

### 5.1 Relação entre os Motores

A Wisdom Distillation (F9.2) é **pós-processamento** do Experience Compiler (F9.1). A relação é sequencial e hierárquica:

```
F9.1 Experience Compiler              F9.2 Wisdom Distillation
═══════════════════════               ═══════════════════════

500 learnings → 20 padrões         → 50 grupos → 15 padrões constitucionais
                → 5 princípios      → 5 princípios constitucionais
                → 0-1 emendas       → 1-2 emendas constitucionais

  Output do F9.1 é INPUT do F9.2      Destilação mais agressiva
  Foco: padrões TÉCNICOS              Foco: princípios CONSTITUCIONAIS
  Maturidade: 0.4-0.7 padrão          Maturidade: 0.7-0.9 princípio
              0.7-0.9 princípio                   > 0.9 emenda
              > 0.9 emenda
```

### 5.2 Pipeline Sequencial

```
┌─────────────────┐    output     ┌─────────────────────┐
│  F9.1 Experience │ ────────────▶ │  F9.2 Wisdom        │
│  Compiler        │  500 learnings│  Distillation        │
│                  │  + 20 padrões │                      │
│  - Coleta        │  + 5 princ.   │  - N1: Coleção       │
│  - Agrupamento   │               │  - N2: Grupos        │
│  - Extração      │               │  - N3: Padrões       │
│  - Validação     │               │  - N4: Princípios    │
│  - Compilação    │               │  - N5: Emendas       │
└─────────────────┘               └──────────┬──────────┘
                                              │
                                              ▼
                                     ┌─────────────────┐
                                     │  CONSTITUIÇÃO    │
                                     │  (emendas)       │
                                     │  + PRINCIPLES.md │
                                     │  (princípios)    │
                                     └─────────────────┘
```

### 5.3 Contrato de Interface

| Campo | Tipo | Origem | Descrição |
|-------|------|--------|-----------|
| `learnings` | `List[Learning]` | F9.1 Fase 1 | Learnings frescos (freshness > 0.5) de todos os agentes |
| `patterns` | `List[Pattern]` | F9.1 Fase 3 | Padrões extraídos (maturity 0.4-0.7) |
| `principles` | `List[Principle]` | F9.1 Fase 4-5 | Princípios aprovados (maturity 0.7-0.9) |
| `amendments` | `List[Amendment]` | F9.1 Fase 5 | Propostas de emenda (maturity > 0.9) — se houver |
| `report` | `Report` | F9.1 | Relatório de compilação (métricas, rejeitados, etc.) |

### 5.4 Diferenças entre F9.1 e F9.2

| Dimensão | F9.1 Experience Compiler | F9.2 Wisdom Distillation |
|----------|--------------------------|--------------------------|
| **Propósito** | Transformar learnings em padrões técnicos | Transformar padrões em sabedoria constitucional |
| **Input** | Learnings brutos (500+) | Output do F9.1 + learnings |
| **Output** | PRINCIPLES.md, patterns/ | CONSTITUIÇÃO (emendas), constitutional-patterns/ |
| **Threshold de grupo** | Similaridade > 0.4 | Similaridade > 0.45 (mais rigoroso) |
| **Maturidade para princípio** | > 0.7 | > 0.8 (constitucional) |
| **Maturidade para emenda** | > 0.9 | > 0.9 (mesmo, mas mais evidências) |
| **Compressão** | ~50:1 (500 → 10 outputs) | ~100:1 (500 → 5 outputs) |
| **Frequência** | Semanal (domingo) | Semanal (após F9.1) |
| **Don envolvimento** | Só emendas > 0.9 | Todos os 5 princípios + emendas |

### 5.5 Pipeline Integrado (F9.1 + F9.2)

```bash
# Pipeline completo de compilação + destilação
cosca experience compile && cosca wisdom distill

# Ou em modo dry-run
cosca experience compile --dry-run && cosca wisdom distill --dry-run

# Pipeline com etapas específicas
cosca experience compile --phases 1,2 && cosca wisdom distill --phases 3,4,5
```

---

## 6. INTEGRAÇÃO COM F1.4 WISDOM DECAY

### 6.1 Contrato de Integração

O Wisdom Decay é **fonte de freshness** para a Wisdom Distillation. A relação é a mesma do F9.1 com F1.4 — unidirecional:

```
┌─────────────────────────┐     freshness_score     ┌─────────────────────────┐
│  Wisdom Decay Engine    │ ──────────────────────▶ │  Wisdom Distillation   │
│  (F1.4)                 │                          │  (F9.2)                │
│                         │     contradiction_flag   │                        │
│  - freshness_score      │ ──────────────────────▶ │  - N1: skip podre      │
│  - contradiction_flag   │                          │  - N2: coesão do grupo │
│  - decay_type           │                          │  - N3: maturity calc   │
│  - last_used            │                          │  - N4: confiança do    │
│  - usage_count          │                          │    princípio            │
└─────────────────────────┘                          └─────────────────────────┘
```

### 6.2 Regras de Integração

| Regra | Origem | Aplicação na Destilação |
|-------|--------|------------------------|
| **Só destila fresco** | F1.4 freshness > 0.5 | N1 — learnings com freshness ≤ 0.5 são excluídos |
| **Contradição bloqueia grupo** | F1.4 contradiction_flag | N2 — grupos com contradições internas não geram padrão |
| **Freshness alimenta coesão** | F1.4 freshness_score | N2 — `avg_freshness` no score de coesão do grupo |
| **Freshness alimenta maturidade** | F1.4 freshness_score | N3 — `avg_freshness` na fórmula de maturidade constitucional |
| **Usage count alimenta confiança** | F1.4 usage_count | N4 — princípios baseados em learnings muito usados têm mais peso |

### 6.3 Efeito do Wisdom Decay na Destilação

```python
# Exemplo: learning com freshness baixo é excluído
learning_A = {"id": "L42", "freshness": 0.45, "domain": "security"}
# freshness ≤ 0.5 → EXCLUÍDO na N1

learning_B = {"id": "L67", "freshness": 0.82, "domain": "security"}
# freshness > 0.5 → INCLUÍDO

# Exemplo: freshness médio do grupo afeta maturidade
group = {
    "items": [L67(f=0.82), L71(f=0.78), L89(f=0.91)],
    "avg_freshness": (0.82 + 0.78 + 0.91) / 3  # = 0.837
}
# avg_freshness = 0.837 contribui para constitutional_maturity
```

---

## 7. INTEGRAÇÃO COM CONSTITUIÇÃO

### 7.1 Pipeline de Emenda

O pipeline segue o mesmo formato do F9.1 (§6) mas com gate adicional no Nível 4 (aprovação do Don para TODOS os 5 princípios, não só para emendas):

```
┌──────────────────────────────────────────────────────────────────────────┐
│               PIPELINE DE EMENDA CONSTITUCIONAL (F9.2)                     │
│                                                                           │
│   ┌─────────────────────┐                                                 │
│   │ 5 princípios        │                                                 │
│   │ candidatos (N4)     │                                                 │
│   └──────────┬──────────┘                                                 │
│              ▼                                                            │
│   ┌─────────────────────────────────────────────┐                        │
│   │ 1. SUBMETE PARA O DON                        │                        │
│   │    - Relatório mensal com 5 princípios        │                        │
│   │    - Cada princípio com:                     │                        │
│   │      • Nome e definição                      │                        │
│   │      • Evidências (grupos que o suportam)    │                        │
│   │      • Maturidade score                      │                        │
│   │      • Impacto estimado                      │                        │
│   │    - Don tem 7 dias para responder           │                        │
│   └──────────────────┬──────────────────────────┘                        │
│                      ▼                                                    │
│        ┌─────────────┴─────────────┐                                     │
│        ▼                           ▼                                     │
│   ┌──────────────┐          ┌──────────────┐                             │
│   │ DON APROVA   │          │ DON REJEITA  │                             │
│   │ (alguns)     │          │ (todos)      │                             │
│   └──────┬───────┘          └──────┬───────┘                             │
│          ▼                         ▼                                     │
│   ┌──────────────────────┐  ┌──────────────┐                             │
│   │ 2. FILTRA POR        │  │ Ciclo vazio  │                             │
│   │    MATURIDADE > 0.9  │  │ Relatório:   │                             │
│   │    + máx 2 emendas   │  │ "Nenhuma     │                             │
│   │                      │  │ emenda       │                             │
│   │    Se > 2 elegíveis  │  │ neste ciclo" │                             │
│   │    → Don escolhe     │  └──────────────┘                             │
│   └──────────┬───────────┘                                               │
│              ▼                                                            │
│   ┌─────────────────────────────────────┐                                │
│   │ 3. GERA DDNA DE EMENDA              │                                │
│   │    - Nome do princípio               │                                │
│   │    - Definição                        │                                │
│   │    - Evidências                       │                                │
│   │    - Artigo afetado da CONSTITUIÇÃO   │                                │
│   │    - Texto proposto                   │                                │
│   │    - Impacto estimado                 │                                │
│   └──────────┬──────────────────────────┘                                │
│              ▼                                                            │
│   ┌─────────────────────────────────────┐                                │
│   │ 4. ATUALIZA CONSTITUIÇÃO            │                                │
│   │    - Adiciona P{n+1}                │                                │
│   │    - Incrementa versão (ex: 1.2.0)  │                                │
│   │    - Adiciona ao amendment log       │                                │
│   │    - Notifica todos os agentes       │                                │
│   └─────────────────────────────────────┘                                │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Formato do DDNA de Emenda (F9.2)

```yaml
---
id: "DDNA-AMEND-F92-YYYY-MM-DD-NN"
title: "Proposta de Emenda Constitucional F9.2: {nome_do_principio}"
status: proposed
date: YYYY-MM-DD
distillation_cycle: N
agents:
  - cosca-wisdom-distillation
  - cosca-experience-compiler
  - {agente_1_contribuinte}
  - {agente_2_contribuinte}
domain: constitutional-amendment
decision_level: 5
confidence: {constitutional_maturity}
revisit: YYYY-MM-DD + 90
tags:
  - dna
  - wisdom-distillation
  - f9.2
  - constitutional-amendment
  - {dominio}
supersedes: null
superseded_by: null
don_approved: null
---

## Context

A Wisdom Distillation Engine (F9.2) processou o output do Experience
Compiler (F9.1) através de 5 níveis de destilação. O princípio abaixo
atingiu os critérios para emenda constitucional:

- **Princípio**: {nome}
- **Definição**: {definição}
- **Maturity Score**: {constitutional_maturity}
- **Compression Path**:
  - N1: {N_learnings} learnings coletados
  - N2: {N_groups} grupos formados, coesão média {cohesion}
  - N3: {N_patterns} padrões extraídos
  - N4: {N_principles} princípios candidatos (top 5)
  - N5: Selecionado como emenda entre {N_amendments} candidatas
- **Evidências**: {lista_de_learnings_e_grupos}

## Proposed Amendment

### Texto Proposto

Adicionar à CONSTITUIÇÃO.md, na PARTE I — PRINCÍPIOS IMUTÁVEIS:

```
### P{numero} — {NOME DO PRINCÍPIO}

**Regra:** {regra}

**Aplicação prática:**
- {aplicacao_1}
- {aplicacao_2}

**Quem garante:** {agente_responsavel}
```

### Artigo Afetado

- PARTE I — PRINCÍPIOS IMUTÁVEIS
- Posição: após P{ultimo}

## Distillation Trail (Rastreabilidade)

A cadeia de evidências pode ser rastreada em:
- F9.1 Report: `audit/compile-report-{date}.md`
- F9.2 Report: `audit/distill-report-{date}.md`
- Constitutional Patterns: `knowledge/constitutional-patterns/{pattern-id}.md`

## Impact Assessment

- **Risco de não adotar**: {descricao}
- **Benefício de adotar**: {descricao}
- **Agentes afetados**: {lista}
- **Mudanças necessárias**: {descricao}

## Decision (preenchido pelo Don)

- [ ] APROVADO — Incorporar à CONSTITUIÇÃO
- [ ] REJEITADO — {justificativa}
- [ ] MODIFICADO — {alteração solicitada}

Data: ____/____/____
Don: _______________
```

### 7.3 Regras de Aprovação

| Cenário | Procedimento |
|---------|-------------|
| **Don aprova 1-2 princípios** | Gerar emendas → atualizar CONSTITUIÇÃO → notificar |
| **Don aprova 3+ princípios** | Don escolhe os 2 mais importantes para este ciclo |
| **Don rejeita todos** | Ciclo vazio. Princípios voltam ao pool para próxima iteração |
| **Don modifica** | Incorporar alteração → re-submeter para aprovação final |
| **Don aprova princípio mas não como emenda** | Princípio vai para PRINCIPLES.md (não para CONSTITUIÇÃO) |
| **Don não responde em 7 dias** | Re-submissão automática. Se ignorado por 14 dias, arquivar como `deferred` |

### 7.4 Salvaguardas

```
🛡️ NENHUM princípio vira emenda sem passar pelos 5 níveis de destilação.
   A compressão progressiva garante que só o essencial chega à CONSTITUIÇÃO.

🛡️ Don aprova TODOS os 5 princípios, não só as emendas.
   Isto dá ao Don visibilidade total do que está sendo destilado.

🛡️ Máximo 2 emendas por ciclo.
   A CONSTITUIÇÃO evolui devagar para manter estabilidade.

🛡️ Toda emenda tem rastro de destilação (Distillation Trail).
   O caminho completo: learning → grupo → padrão → princípio → emenda.

🛡️ Princípios rejeitados como emenda podem virar PRINCIPLES.md.
   Nem todo princípio constitucional precisa ser lei — alguns são guias.
```

---

## 8. FÓRMULA DE MATURIDADE PARA PRINCÍPIOS CONSTITUCIONAIS

### 8.1 Fórmula Principal

A maturidade constitucional é mais rigorosa que a maturidade técnica do F9.1:

```
constitutional_maturity = group_cohesion     × 0.25 +
                          avg_freshness      × 0.20 +
                          cross_agent_count  × 0.20 +
                          foundational_impact × 0.20 +
                          durability         × 0.15
```

### 8.2 Componentes

| Componente | Variável | Peso | Descrição |
|------------|----------|:----:|-----------|
| **Coesão do Grupo** | `group_cohesion` | **0.25** | Quão coeso é o grupo (ver §3.3). Grupos coesos produzem princípios mais precisos. |
| **Freshness Médio** | `avg_freshness` | **0.20** | Média do freshness de todos os learnings no grupo. Conhecimento fresco = princípio relevante. |
| **Cross-Agent Count** | `cross_agent_count` | **0.20** | Quantos agentes diferentes contribuíram. Cap: 5 agentes. Validação independente. |
| **Impacto Fundacional** | `foundational_impact` | **0.20** | O princípio afeta a plataforma como um todo? (ver §8.3) |
| **Durabilidade** | `durability` | **0.15** | O princípio é duradouro ou moda passageira? (ver §8.4) |

#### Detalhamento

```
group_cohesion = calc_group_cohesion(group)  # 0.0-1.0, ver §3.3

avg_freshness = media_aritmética(freshness de todos os learnings do grupo)
  # Fornecido por F1.4 Wisdom Decay
  # Já filtrado para > 0.5 na N1

cross_agent_count = min(unique_agent_count / 5, 1.0)
  # 1 agente → 0.2, 2 agentes → 0.4, 5+ agentes → 1.0

foundational_impact = calc_foundational_impact(pattern)
  # Ver §8.3

durability = calc_durability(pattern)
  # Ver §8.4
```

### 8.3 Foundational Impact

Mede o quão fundacional é o princípio — ele afeta a plataforma inteira ou só um nicho?

```
foundational_impact = scope           × 0.4 +
                      irreversibility × 0.3 +
                      leverage        × 0.3
```

| Subcomponente | Peso | Score Alto (1.0) | Score Baixo (0.2) |
|---------------|:----:|-------------------|-------------------|
| **Scope** (abrangência) | **0.4** | Afeta TODOS os agentes, toda decisão | Afeta 1 agente, 1 domínio |
| **Irreversibility** (custo de reverter) | **0.3** | Se errado, causa dano sistêmico | Fácil de reverter (config flag) |
| **Leverage** (alavancagem) | **0.3** | Desbloqueia outras capacidades | É uma convenção menor |

**Exemplos:**

| Princípio | Scope | Irreversibility | Leverage | Foundational Impact |
|-----------|:-----:|:---------------:|:--------:|:-------------------:|
| "Segurança acima de funcionalidade" (P1) | 1.0 | 1.0 | 1.0 | **1.00** |
| "Código executado é a verdade" (P2) | 1.0 | 0.9 | 1.0 | **0.97** |
| "Sempre usar go test -race em CI" | 0.4 | 0.3 | 0.5 | **0.40** |
| "Nomes de variável em camelCase" | 0.2 | 0.1 | 0.1 | **0.14** |

### 8.4 Durability

Mede se o princípio é duradouro ou é uma moda passageira / específica de tecnologia.

```
durability = tech_independence × 0.5 +
             historical_stability × 0.3 +
             trend_resistance × 0.2
```

| Subcomponente | Peso | Score Alto (1.0) | Score Baixo (0.2) |
|---------------|:----:|-------------------|-------------------|
| **Tech Independence** | **0.5** | Não depende de tecnologia específica | Só faz sentido com Go 1.22 |
| **Historical Stability** | **0.3** | Princípio similar existe em outras engenharias há décadas | É novidade, sem histórico |
| **Trend Resistance** | **0.2** | Resistente a modas de engenharia | Pode ser substituído por approach diferente |

**Exemplos:**

| Princípio | Tech Indep. | Hist. Stability | Trend Res. | Durability |
|-----------|:----------:|:---------------:|:----------:|:----------:|
| "Segurança acima de funcionalidade" (P1) | 1.0 | 1.0 | 1.0 | **1.00** |
| "Memória sem poluição" (P7) | 1.0 | 0.8 | 0.9 | **0.92** |
| "Sempre usar Golang 1.24 range over int" | 0.1 | 0.2 | 0.3 | **0.17** |

### 8.5 Thresholds de Maturidade Constitucional

```
                   Maturidade Constitucional

    0.0          0.5           0.7         0.85        1.0
    │─────────────│─────────────│───────────│───────────│
    │  Arquivado  │  Padrão     │  Princípio│  Emenda   │
    │  (pool)     │  (constit.) │  (candidato)│(CONST.) │
                  │             │           │           │
```

| Threshold | Classificação | Ação |
|-----------|--------------|------|
| **< 0.5** | 🟡 Insuficiente | Arquiva no pool para próxima iteração. Evidências insuficientes. |
| **0.5 — 0.7** | 🟢 Padrão Constitucional | Registra em `knowledge/constitutional-patterns/`. Aguarda mais evidências. |
| **0.7 — 0.85** | 🔵 Princípio Candidato | Entra no top 5 do N4. Submetido para aprovação do Don. |
| **0.85 — 0.9** | 🔵 Princípio Forte | Prioridade alta. Se Don aprovar, candidato forte a emenda futura. |
| **> 0.9** | 🟣 Emenda Constitucional | Gera DDNA de emenda. Se Don aprovar, altera a CONSTITUIÇÃO. |

### 8.6 Exemplo de Cálculo

```
Grupo: "Dead code removal requer análise de dependências"
(após destilação F9.2, com mais evidências que no F9.1)

Learnings no grupo: 7 learnings de 4 agentes
Freshness médio: 0.85
Coesão do grupo: 0.78
Cross-agent count: min(4/5, 1.0) = 0.80

Foundational Impact:
  scope = "afeta toda alteração de código" → 0.85
  irreversibility = "remover sem verificar quebra build" → 0.80
  leverage = "previne regressão em todo o código" → 0.75
  foundational_impact = 0.85×0.4 + 0.80×0.3 + 0.75×0.3 = 0.805

Durability:
  tech_independence = "independe da linguagem" → 0.90
  historical_stability = "sempre foi verdade" → 0.85
  trend_resistance = "independente de moda" → 0.90
  durability = 0.90×0.5 + 0.85×0.3 + 0.90×0.2 = 0.885

constitutional_maturity = 0.78×0.25 + 0.85×0.20 + 0.80×0.20 + 0.805×0.20 + 0.885×0.15
                        = 0.195 + 0.170 + 0.160 + 0.161 + 0.133
                        = 0.819 → 🔵 PRINCÍPIO CANDIDATO FORTE
```

---

## 9. EXEMPLO COMPLETO

### 9.1 Cenário: Dead Code Removal → Emenda Constitucional

Partindo do exemplo do F9.1 (§8), agora com **mais evidências acumuladas** após semanas de compilação:

#### Nível 1 — Coleção (entrada)

**Input do F9.1 + novos learnings (7 learnings de 4 agentes):**

| # | Agente | Data | Freshness | Afirmação Central |
|---|--------|------|-----------|-------------------|
| L20 | cosca-kernel | 2026-07-29 | 0.90 | "Código morto detectado via `go list -f '{{join .Deps}}' ./cmd/cosca/`" |
| L22 | cosca-kernel | 2026-07-30 | 0.93 | "Classificação de dead code em 3 tipos é reutilizável" |
| L35 | cosca-architecture | 2026-07-30 | 0.87 | "Dead code identificado por análise de dependências antes de refatoração" |
| L52 | cosca-performance | 2026-07-30 | 0.82 | "`go build` não detecta dead code — precisa de `go list -deps`" |
| L71 | cosca-review | 2026-07-29 | 0.78 | "Revisão deve incluir verificação de imports para dead code" |
| L88 | cosca-backend | 2026-08-05 | 0.95 | "Remoção de código sem verificação de grafo quebrou build 2x este mês" |
| L92 | cosca-evolution | 2026-08-06 | 0.91 | "Dead code acumulado correlaciona com aumento de bugs em 15%" |

**Total: 7 learnings, 4 agentes** ✅ (≥3, ≥2)

#### Nível 2 — Grupos

```
Agrupamento:

Grupo formado: {L20, L22, L35, L52, L71, L88, L92}
  → Domínio: code-quality, architecture
  → Tags comuns: #dead-code, #deps-analysis, #go-list, #code-quality
  → Coesão: 0.78 (alta — tags muito similares)
  → Agentes: 4 (kernel, architecture, performance, review, backend, evolution)
  → Freshness médio: (0.90+0.93+0.87+0.82+0.78+0.95+0.91)/7 = 0.88

Resultado: GRUPO VÁLIDO (coesão 0.78 ≥ 0.4) ✅
```

#### Nível 3 — Padrões Constitucionais

```
Extração do padrão:

"Dead code removal REQUER verificação de grafo de dependências
antes da remoção. A classificação em 3 tipos (A: remove, B: testa,
C: documenta) é um framework validado por 4 agentes e 7 evidências
independentes. Ignorar esta verificação causa regressão mensurável."

Validação contra CONSTITUIÇÃO:
  → Complementa P2 (Código executado é a verdade) — sem contradição ✅
  → É acionável — "verificar go list -deps" é ação específica ✅
  → É fundacional — afeta toda alteração de código ✅

Maturidade:
  group_cohesion = 0.78 × 0.25 = 0.195
  avg_freshness  = 0.88 × 0.20 = 0.176
  cross_agent    = 0.80 × 0.20 = 0.160
  foundational   = 0.805 × 0.20 = 0.161
  durability     = 0.885 × 0.15 = 0.133
  total          = 0.825 → 🔵 PRINCÍPIO CANDIDATO FORTE

Resultado: TOP 15 ✅ (maturity 0.825 > 0.7)
```

#### Nível 4 — Princípios Constitucionais

```
Compressão por afinidade:
  Padrão "dead code" é fundido com padrão "análise de dependências"
  (mesmo grupo temático → merge natural)

5 princípios candidatos para este ciclo:
  1. Dead code requer verificação de dependências (maturity 0.825)
  2. Testes de concorrência requerem -race flag (maturity 0.79)
  3. Memória de agente requer curadoria mensal (maturity 0.76)
  4. Decisões arquiteturais requerem DDNA (maturity 0.74)
  5. Rollback deve ser sempre possível (maturity 0.72)

Submetidos para Don:
  → Relatório enviado em 2026-08-07
  → Don tem 7 dias para responder
```

#### Nível 5 — Emendas

```
Don aprova:
  ✅ Princípio 1 (dead code) — "vira emenda, maturity 0.825 > 0.9? Não,
     0.825 < 0.9. Mas Don considera crítico → aprova como PRINCIPLES.md
     com recomendação para elevar a emenda no próximo ciclo."
  ✅ Princípio 4 (DDNA) — "vira emenda, maturity 0.74. Don aprova como
     princípio em PRINCIPLES.md."
  ❌ Princípio 5 (rollback) — "já coberto por G3 na CONSTITUIÇÃO.
     Rejeitado como redundante."

Emendas geradas: 0 neste ciclo (nenhuma atingiu maturity > 0.9)
Princípios registrados em PRINCIPLES.md: 2
  → P-001: Dead code requer verificação de dependências
  → P-002: Decisões arquiteturais requerem DDNA
```

### 9.2 Projeção: Dead Code como Emenda

Se no próximo ciclo mais evidências elevarem a maturidade > 0.9:

```
Proposta de Emenda — Adicionar P9 à CONSTITUIÇÃO:

### P9 — CÓDIGO MORTO REQUER VERIFICAÇÃO DE DEPENDÊNCIAS

**Regra:** Nenhum bloco de código pode ser removido como "dead code"
sem verificação de grafo de dependências via `go list -deps` ou
ferramenta equivalente. Código que compila mas não é importado NÃO
é prova de que não é usado.

**Aplicação prática:**
- Antes de remover função/arquivo/pacote: `go list -f '{{join .Deps}}' ./cmd/`
- Classificação obrigatória: Tipo A (remove), B (testa), C (documenta)
- Code review DEVE incluir verificação de imports afetados
- Violação desta regra reverte o commit automaticamente

**Quem garante:** cosca-review (verificação em code review) + cosca-evolution
(monitora dead code accumulation)
```

---

## 10. CLI E AUTOMAÇÃO

### 10.1 Comandos

```bash
# Executar pipeline completo de destilação
cosca wisdom distill

# Executar em modo dry-run (não submete para Don, não altera CONSTITUIÇÃO)
cosca wisdom distill --dry-run

# Ver relatório da última destilação
cosca wisdom report

# Ver princípios constitucionais candidatos (top 5 do N4)
cosca wisdom principles --constitutional

# Ver emendas pendentes de aprovação do Don
cosca wisdom amendments pending

# Ver histórico de destilações
cosca wisdom history --weeks 12

# Executar pipeline integrado F9.1 → F9.2
cosca experience compile && cosca wisdom distill

# Executar níveis específicos
cosca wisdom distill --levels 1,2     # Só coleta + grupos
cosca wisdom distill --levels 3,4,5   # Só padrões + princípios + emendas

# Ver cadeia de rastreabilidade de uma emenda
cosca wisdom trail <amendment-id>

# Submeter princípios para Don manualmente
cosca wisdom submit --all

# Aprovar/rejeitar princípio (Don)
cosca wisdom approve <principle-id>
cosca wisdom reject <principle-id> --reason "justificativa"
```

### 10.2 Opções

| Flag | Descrição | Default |
|------|-----------|---------|
| `--dry-run` | Calcula mas não submete para Don, não altera CONSTITUIÇÃO | `false` |
| `--levels` | Lista de níveis para executar (ex: `1,2,3`) | `1,2,3,4,5` |
| `--min-cohesion` | Threshold mínimo de coesão para grupo | `0.4` |
| `--max-groups` | Número máximo de grupos no N2 | `50` |
| `--max-patterns` | Número máximo de padrões no N3 | `15` |
| `--max-principles` | Número máximo de princípios no N4 | `5` |
| `--max-amendments` | Número máximo de emendas no N5 | `2` |
| `--maturity-threshold` | Threshold de maturidade para princípio candidato | `0.7` |
| `--amendment-threshold` | Threshold de maturidade para emenda | `0.9` |
| `--don-timeout` | Dias para Don responder | `7` |
| `--format` | Formato de output: `table`, `json`, `yaml` | `table` |
| `--output` | Arquivo de output do relatório | stdout |

### 10.3 Scheduler

```bash
# Pipeline integrado semanal (domingo 01:00 — 1h após F9.1)
0 1 * * 0 cosca experience compile && cosca wisdom distill --format json

# Apenas destilação (se F9.1 já rodou)
30 0 * * 0 cosca wisdom distill --format json

# CI Gate semanal
0 2 * * 0 cosca wisdom distill --dry-run --format json --output distill-report.json
```

### 10.4 CI Gate

```yaml
# .github/workflows/wisdom-distillation-weekly.yml
name: Wisdom Distillation — Weekly Constitutional Funnel
on:
  schedule:
    - cron: "0 1 * * 0"  # Semanal, 1h após F9.1
  workflow_dispatch:

jobs:
  distill:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Wisdom Distillation
        run: |
          cosca wisdom distill --format json --output distill-report.json

      - name: Check for Pending Constitutional Principles
        run: |
          PENDING=$(jq '.principles.pending | length' distill-report.json)
          if [ "$PENDING" -gt 0 ]; then
            echo "📜 $PENDING constitutional principle(s) pending Don approval:"
            jq -r '.principles.pending[] | "  • \(.title) (maturity: \(.maturity))"' distill-report.json
          fi

      - name: Check Distillation Health
        run: |
          GROUPS=$(jq '.groups_formed | length' distill-report.json)
          PATTERNS=$(jq '.patterns_extracted | length' distill-report.json)
          PRINCIPLES=$(jq '.principles_approved | length' distill-report.json)
          echo "📊 Groups: $GROUPS | Patterns: $PATTERNS | Principles: $PRINCIPLES"
          echo "📈 Compression ratio: $(jq '.compression_ratio' distill-report.json):1"
```

---

## 11. MÉTRICAS DO ENGINE

### 11.1 Métricas Internas

| Métrica | Definição | Alvo | Fonte |
|---------|-----------|:----:|-------|
| **Pipeline Duration** | Tempo total de execução | < 30s | Engine |
| **Input Size** | Learnings + padrões + princípios recebidos do F9.1 | ~525 | N1 |
| **Groups Formed** | Grupos com coesão > 0.4 | ~50 | N2 |
| **Patterns Extracted** | Padrões válidos após validação c/ CONSTITUIÇÃO | ~15 | N3 |
| **Principles Candidate** | Princípios submetidos para Don | 5 | N4 |
| **Amendments Generated** | Emendas aplicadas à CONSTITUIÇÃO | 0-2 | N5 |
| **Compression Ratio** | Input ÷ Output (learnings → emendas) | > 100:1 | Engine |
| **Avg Group Cohesion** | Média do score de coesão dos grupos | > 0.6 | N2 |
| **Don Response Rate** | % de princípios com resposta do Don em 7 dias | > 80% | N4 |

### 11.2 Métricas de Qualidade da Destilação

| Métrica | Definição | Alvo |
|---------|-----------|:----:|
| **Constitutional Survival Rate** | % de emendas ainda válidas após 3 meses | > 95% |
| **False Positive Rate** | Princípios que precisaram ser revogados | < 5% |
| **Cross-Agent Coverage** | Média de agentes por grupo | > 3 |
| **Avg Constitutional Maturity** | Média do maturity dos princípios (N4) | > 0.75 |
| **Compression Fidelity** | O princípio final representa fielmente os learnings originais? | > 0.85 |

### 11.3 Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| Pipeline > 30s | 🟡 MÉDIO | Otimizar agrupamento semântico |
| 0 princípios candidatos por 2+ ciclos | 🟡 MÉDIO | F9.1 pode não estar gerando output suficiente |
| Compressão ratio < 50:1 | 🟡 MÉDIO | Grupos podem estar fragmentados — revisar threshold de coesão |
| Don não responde por 14+ dias | 🟡 MÉDIO | Re-notificar Don. Se ignorado, arquivar como `deferred` |
| Princípio contradiz CONSTITUIÇÃO | 🔴 ALTO | Validação falhou — revisar algoritmo |
| Maturidade de emenda cai > 0.2 no ciclo seguinte | 🔴 ALTO | Evidências foram refutadas — investigar |

### 11.4 Métricas de Saúde do Sistema

```
┌─────────────────────────────────────────────────────────────────┐
│              WISDOM DISTILLATION DASHBOARD                        │
│                                                                  │
│  Ciclo Atual: 2026-W32                                           │
│                                                                  │
│  Funil:                                                          │
│    Input (F9.1)   ──▶  527 itens                                 │
│    Grupos         ──▶  48 grupos  (coesão média: 0.72)          │
│    Padrões        ──▶  15 padrões (maturidade média: 0.68)      │
│    Princípios     ──▶  5 princípios (maturidade média: 0.79)    │
│    Emendas        ──▶  1 emenda   (maturidade: 0.91) 🟣         │
│                                                                  │
│  Compressão: 527:1 = 527× (527 → 1 emenda)                      │
│                                                                  │
│  Don Actions:                                                    │
│    ✅ Aprovados: 3 princípios                                    │
│    ❌ Rejeitados: 2 princípios                                   │
│    ⏳ Pendentes: 0                                               │
│                                                                  │
│  Saúde:                                                          │
│    🟢 Pipeline < 30s?  Sim (24s)                                │
│    🟢 Compression > 100:1? Sim (527:1)                          │
│    🟢 Avg cohesion > 0.6?   Sim (0.72)                          │
│    🟢 Don response < 7d?   Sim (média 3.2d)                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## 12. CASOS DE BORDA E ANTI-PADRÕES

### 12.1 Casos de Borda

| Caso | O que acontece | Tratamento |
|------|---------------|------------|
| **Nenhum learning fresco** (todos freshness ≤ 0.5) | Pipeline termina na N1 com 0 grupos | Relatório: "Nenhum learning fresco — execute Wisdom Decay primeiro" |
| **Apenas 1 grupo formado** | Destilação produz no máximo 1 padrão | Válido — qualidade > quantidade |
| **50+ grupos válidos** | Pega top 50 por coesão | Grupos excedentes viram `overflow/` para próximo ciclo |
| **Nenhum padrão atinge maturity > 0.7** | N4: 0 princípios candidatos | Ciclo vazio. CONSTITUIÇÃO está estável. |
| **5+ princípios com maturity > 0.9** | Don escolhe os 2 mais relevantes | Os outros 3 viram prioridade para próximo ciclo |
| **Apenas 1 agente contribuiu** | Grupo marcado como `#single-source` | Não gera padrão — precisa de validação cross-agent |
| **Princípio contradiz CONSTITUIÇÃO** | Princípio rejeitado automaticamente | DDNA registra contradição. Escalar para Don. |
| **Don aprova princípio como emenda mas maturity < 0.9** | Don pode forçar override | Princípio vai para CONSTITUIÇÃO com nota "Don override" |
| **Learning é usado em 2 grupos diferentes** | Participa de ambos | Enriquecimento cruzado — saudável |

### 12.2 Anti-Padrões

| Anti-Padrão | Por que evitar | Como detectar |
|-------------|---------------|---------------|
| **Destilar toda semana sem input novo** | Se F9.1 não gerou output novo, destilação é redundante | Executar apenas se F9.1 rodou no mesmo ciclo |
| **Aceitar princípios genéricos como constitucionais** | "Código deve ser testado" é verdade mas não é fundacional | Validação de foundational_impact na N4 |
| **Criar emendas para tudo que atinge maturity > 0.9** | Constituição inflacionada perde efetividade | Máximo 2 emendas por ciclo |
| **Ignorar freshness na destilação** | Princípio baseado em conhecimento podre | Filtro obrigatório na N1 |
| **Permitir que 1 agente domine a destilação** | Princípios viesados para um único domínio | Regra de mínimo 2 agentes diferentes |
| **Sobrecarregar o Don com 5+ princípios por semana** | Don ignora ou aprova sem ler | Frequência máxima mensal para submissão |
| **Merge forçado de padrões conflitantes** | Princípio inconsistente | Marcar grupo como DIVERGENT — não forçar merge |
| **Criar princípio que repete constituição existente** | Redundância desnecessária | Validação semântica contra P1-P8 na N3 |
| **Destilar sem rastro (distillation trail)** | Não é possível auditar a cadeia de evidências | DDNA obrigatório para toda emenda |

### 12.3 Regras de Resiliência

```
1. Falha no F9.1 → Destilação não executa
   (não temos input para destilar)

2. Falha no Wisdom Decay → Destilação não executa
   (não temos freshness para filtrar)

3. Falha em 1 nível → Relatório parcial dos níveis anteriores
   (ex: falha na N3 → relatório com N1+N2)

4. Don desconectado → Princípios acumulam em pending/
   (máximo 30 dias — após isso, arquivar como deferred)

5. CONSTITUIÇÃO modificada manualmente pelo Don entre ciclos →
   Próxima destilação detecta diff e revalida princípios candidatos
   contra a nova versão

6. Rollback de emenda → DDNA registra rollback com justificativa
   CONSTITUIÇÃO volta à versão anterior via git revert
```

---

## HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — pipeline 5 níveis, algoritmo de agrupamento, taxa de compressão ~100:1, integrações com F9.1, F1.4, CONSTITUIÇÃO, fórmula de maturidade constitucional, exemplo dead code, CLI, métricas, casos de borda |

---

> *"Informação não é conhecimento. Conhecimento não é sabedoria. Sabedoria não é constituição. A destilação separa o que é duradouro do que é passageiro — e só o que sobrevive a 5 níveis de compressão merece virar lei."*
> — Cosca Architecture Chief, 2026-07-30
