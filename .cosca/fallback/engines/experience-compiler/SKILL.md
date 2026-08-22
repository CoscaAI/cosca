# EXPERIENCE COMPILER ENGINE — Knowledge Distillation & Constitutional Evolution (F9.1)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência (+0.06), Julgamento (+0.05) | **Bloco Cognitivo**: Bloco 4 — Metacognição & Governança
> **Fase CMI**: Fase 9 — Evolution | **Código**: F9.1
> **Dependências**: F1.4 Wisdom Decay | F1.6 Cognitive Entropy | F7.3 Engineering Score | CONSTITUTION.md
>
> Consulte também:
> - [WISDOM_DECAY.md](../wisdom-decay/WISDOM_DECAY.md) — F1.4: freshness score, decay types, revalidação
> - [ENTROPY.md](../cognitive-entropy/ENTROPY.md) — F1.6: entropia cognitiva, contradições, stale knowledge
> - [CONSTITUTION.md](../../CONSTITUTION.md) — autoridade máxima, 8 princípios imutáveis, cadeia de comando
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — formato de aprendizado, negative memory, confidence scoring
> - [AGENT_DNA.md](../../AGENT_DNA.md) — 28 campos obrigatórios por agente, compliance checklist
> - [SKILL_TEMPLATE.md](../../SKILL_TEMPLATE.md) — template de skill engine

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [Pipeline de Compilação (5 Fases)](#2-pipeline-de-compilação-5-fases)
3. [Fórmula de Maturidade de Padrão](#3-fórmula-de-maturidade-de-padrão)
4. [Integração com F1.4 Wisdom Decay](#4-integração-com-f14-wisdom-decay)
5. [Integração com F1.6 Cognitive Entropy](#5-integração-com-f16-cognitive-entropy)
6. [Integração com CONSTITUIÇÃO](#6-integração-com-constituição)
7. [Integração com F7.3 Engineering Score](#7-integração-com-f73-engineering-score)
8. [Exemplo com Learnings Reais](#8-exemplo-com-learnings-reais)
9. [CLI e Automação](#9-cli-e-automação)
10. [Métricas do Engine](#10-métricas-do-engine)
11. [Casos de Borda e Anti-Padrões](#11-casos-de-borda-e-anti-padrões)

---

## 1. DEFINIÇÃO

### 1.1 O que é o Experience Compiler

O **Experience Compiler** é a máquina de destilação de conhecimento do Cosca. Ele pega centenas de learnings brutos dos 54 agentes — sucessos, falhas, técnicas descobertas — e os compila em:

1. **Padrões Reutilizáveis** — técnicas que funcionaram múltiplas vezes e podem ser aplicadas por qualquer agente
2. **Princípios** — regras consolidadas que guiam decisões em um domínio
3. **Emendas Constitucionais** — princípios fundacionais que alteram a CONSTITUIÇÃO

### 1.2 Analogia: Compilador de Código Fonte

```
┌─────────────────────────────────────────────────────────────────────────┐
│               EXPERIENCE COMPILER — ANALOGIA DE COMPILAÇÃO               │
│                                                                          │
│  Código Fonte (learnings.md)          →  Compilador (Experience Comp.)  │
│  Tokens (tags + contexto)             →  Lexer (extração de tags)       │
│  AST (grupos de similaridade)         →  Parser (agrupamento)           │
│  Otimizações (padrões candidatos)     →  Optimizer (síntese)            │
│  Código de Máquina (princípios)       →  Codegen (compilação final)     │
│  Linking (emenda constitucional)      →  Linker (integração c/ CONST.)  │
│                                                                          │
│  Erro de compilação: princípio sem evidências mínimas                    │
│  Warning: princípio contradiz constituição atual                         │
│  Otimização: padrão maduro vira princípio automaticamente               │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.3 Propósito

| Por que compilar | O que resolve | Ação resultante |
|------------------|--------------|-----------------|
| Conhecimento bruto é ruidoso | 54 agentes geram centenas de learnings — maioria descartável | Destilar apenas o que é reutilizável |
| Padrões ficam isolados em agentes | Cada agente aprende, mas o aprendizado não vira ativo da organização | Extrair padrões cross-agent |
| Princípios não são formalizados | "Sabemos que X funciona" mas não está escrito em lugar nenhum | Formalizar em PRINCIPLES.md |
| Constituição precisa evoluir | A organização aprende, mas a constituição só muda por decreto | Pipeline de propostas de emenda baseado em evidências |

### 1.4 Outputs do Compiler

| Output | Formato | Consumidor | Frequência |
|--------|---------|------------|------------|
| **PRINCIPLES.md** (global) | Markdown com princípios validados | Todos os agentes | Semanal |
| **Emenda Constitucional** | Proposta de alteração da CONSTITUIÇÃO | Don (aprovação) + Kernel (execução) | Mensal (ou quando maturity > 0.9) |
| **Relatório de Compilação** | Learnings processados, padrões extraídos, princípios rejeitados | Architecture Chief + CTO | A cada execução |
| **DDNA de Princípio** | Decision DNA para cada princípio aprovado | Decision Registry | A cada princípio |

---

## 2. PIPELINE DE COMPILAÇÃO (5 FASES)

O pipeline roda **semanalmente** (domingo 00:00) ou **sob demanda** via CLI. Tempo alvo: **< 30s** para 54 agentes.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                  EXPERIENCE COMPILER — PIPELINE COMPLETO                      │
│                                Execução Semanal                                │
│                                                                                │
│  TRIGGERS:                                                                     │
│  ┌────────────────┐  ┌──────────────┐  ┌──────────────────┐                    │
│  │ Cron: Semanal   │  │ Wisdom Decay │  │ CLI: cosca       │                    │
│  │ (Dom 00:00)    │  │ executado    │  │ experience compile│                   │
│  └───────┬────────┘  └──────┬───────┘  └────────┬─────────┘                    │
│          │                  │                     │                              │
│          └──────────────────┼─────────────────────┘                              │
│                             ▼                                                    │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 1 — COLETA                                                          │  │
│  │                                                                           │  │
│  │  1. Varre learnings.md de todos os 54 agentes                             │  │
│  │     ├── memory/agent/*/learnings.md (primário)                             │  │
│  │     └── memory/agent/*/failures.md (falhas que geraram aprendizado)       │  │
│  │                                                                           │  │
│  │  2. Filtra apenas learnings com freshness > 0.5                           │  │
│  │     ├── Freshness fornecido por F1.4 Wisdom Decay                          │  │
│  │     └── Learnings podres (freshness ≤ 0.5) são EXCLUÍDOS da compilação   │  │
│  │                                                                           │  │
│  │  3. Agrupa por domínio (testing, architecture, security, etc.)            │  │
│  │     ├── Usa tags do learning para inferir domínio primário                │  │
│  │     └── Learnings multi-domínio vão para todos os domínios relevantes     │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 5s (54 arquivos, parse + filtro)                       │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 2 — AGRUPAMENTO (SIMILARITY)                                        │  │
│  │                                                                           │  │
│  │  1. Para cada learning, extrai:                                           │  │
│  │     ├── Tags (lista de strings do campo `**Tags**`)                       │  │
│  │     ├── Domínio (inferido das tags + conteúdo)                            │  │
│  │     ├── Contexto (campo `**Task**` ou `**Contexto**`)                     │  │
│  │     └── Afirmação central (campo `**Learned**` ou `**Aprendizados**`)    │  │
│  │                                                                           │  │
│  │  2. Calcula similaridade entre pares de learnings:                        │  │
│  │     ├── Tag overlap: Jaccard(tags_a, tags_b)                              │  │
│  │     ├── Domínio match: 1.0 se mesmo domínio, 0.5 se domínio relacionado   │  │
│  │     └── Similaridade total = 0.7 × tag_overlap + 0.3 × domain_match      │  │
│  │                                                                           │  │
│  │  3. Se similaridade > 0.4 entre 2+ learnings → mesmo grupo               │  │
│  │                                                                           │  │
│  │  4. Se 3+ learnings no mesmo grupo → padrão candidato                     │  │
│  │     ├── Mínimo: 3 learnings                                              │  │
│  │     └── Mínimo: 2 agentes diferentes (evidência cross-agent)             │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 10s (O(n²) com n = aprendizado fresco total)          │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 3 — EXTRAÇÃO DE PADRÃO                                              │  │
│  │                                                                           │  │
│  │  Para cada grupo com 3+ learnings:                                       │  │
│  │                                                                           │  │
│  │  1. Sintetiza o padrão: "aprendemos que X"                               │  │
│  │     ├── Extrai a afirmação comum entre todos os learnings do grupo        │  │
│  │     ├── Se divergem → padrão é a interseção do que TODOS concordam      │  │
│  │     └── Se contradizem → grupo é marcado como CONFLICT e não gera padrão  │  │
│  │                                                                           │  │
│  │  2. Calcula maturity_score (ver §3)                                       │  │
│  │                                                                           │  │
│  │  3. Se maturity > 0.7 → princípio candidato                              │  │
│  │     ├── Se maturity > 0.9 → emenda constitucional candidata              │  │
│  │     └── Senão → registra como padrão em patterns/ (aguarda maturação)    │  │
│  │                                                                           │  │
│  │  4. Para cada princípio candidato, gera:                                  │  │
│  │     ├── Nome do princípio                                                 │  │
│  │     ├── Definição (o que diz)                                             │  │
│  │     ├── Evidências (lista de learnings que o suportam)                   │  │
│  │     ├── Agentes envolvidos                                                │  │
│  │     ├── Maturity score                                                    │  │
│  │     └── Impacto (se vira emenda → qual artigo da CONSTITUIÇÃO afeta)     │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 5s (síntese O(grupos))                                 │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 4 — VALIDAÇÃO                                                       │  │
│  │                                                                           │  │
│  │  Para cada princípio candidato:                                          │  │
│  │                                                                           │  │
│  │  1. 🛡️ Não contradiz CONSTITUIÇÃO atual                                  │  │
│  │     ├── Verifica se o princípio conflita com P1-P8 existentes            │  │
│  │     ├── Se conflita → rejeitado com justificativa                        │  │
│  │     └── Se for mais restritivo que princípio existente → OK (emenda)     │  │
│  │                                                                           │  │
│  │  2. 📊 Tem evidências suficientes                                         │  │
│  │     ├── Mínimo 3 learnings                                               │  │
│  │     ├── Mínimo 2 agentes diferentes                                       │  │
│  │     └── Todos os learnings têm freshness > 0.5 (já filtrado na Fase 1)   │  │
│  │                                                                           │  │
│  │  3. 🎯 É acionável (não genérico)                                         │  │
│  │     ├── O princípio deve prescrever uma AÇÃO específica                  │  │
│  │     ├── "Testes são importantes" ❌ genérico — rejeitado                 │  │
│  │     ├── "Sempre rodar `go test -race` em CI" ✅ acionável — aprovado    │  │
│  │     └── Validação por substring match + heurística de verbos de ação     │  │
│  │                                                                           │  │
│  │  4. 🔄 Regra de ouro: "Já salvou uma decisão?"                          │  │
│  │     ├── Se o princípio, se seguido, teria evitado um erro conhecido      │  │
│  │     ├── Se sim → prioridade alta                                        │  │
│  │     └── Se não → pode ser depriorizado                                   │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 5s (validações O(princípios))                          │  │
│  └────────────────────────────────┬─────────────────────────────────────────┘  │
│                                   ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 5 — COMPILAÇÃO                                                      │  │
│  │                                                                           │  │
│  │  Para cada princípio aprovado:                                           │  │
│  │                                                                           │  │
│  │  1. 📝 Registra em PRINCIPLES.md (global, em knowledge/)                 │  │
│  │     ├── Formato: nome, definição, evidências, maturity,                   │  │
│  │     │   agentes, data de compilação, validade (próxima revisão)          │  │
│  │     └── Atualiza INDEX.md do knowledge directory                          │  │
│  │                                                                           │  │
│  │  2. 🧬 Se for alteração constitucional (maturity > 0.9):                 │  │
│  │     ├── Gera DDNA de proposta de emenda (ver §6.2)                       │  │
│  │     ├── Prepara diff da CONSTITUIÇÃO: "Adicionar P9: {princípio}"        │  │
│  │     ├── Submete para o Don via relatório semanal + notificação           │  │
│  │     └── Don aprova/rejeita (ver §6.3)                                    │  │
│  │                                                                           │  │
│  │  3. 📊 Gera relatório de compilação:                                     │  │
│  │     ├── Total de learnings processados                                   │  │
│  │     ├── Learnings filtrados (freshness ≤ 0.5)                            │  │
│  │     ├── Grupos formados                                                  │  │
│  │     ├── Padrões candidatos                                               │  │
│  │     ├── Princípios aprovados                                             │  │
│  │     ├── Princípios rejeitados (com causa)                                │  │
│  │     ├── Propostas de emenda constitucional                               │  │
│  │     └── Entropia antes/depois da compilação (ver §5)                    │  │
│  │                                                                           │  │
│  │  4. 🔄 Feedback para agentes:                                            │  │
│  │     ├── Agentes cujos learnings viraram princípios recebem crédito       │  │
│  │     ├── Agentes cujos learnings foram rejeitados recebem justificativa   │  │
│  │     └── Atualiza confidence scores dos agentes contribuintes (+0.02)     │  │
│  │                                                                           │  │
│  │  ⏱ Tempo alvo: < 5s (escrita de arquivos + notificações)               │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Algoritmo Central

```
function run_experience_compiler():
    // === FASE 1: COLETA ===
    all_learnings = []
    for agent_path in glob("memory/agent/*/learnings.md"):
        learnings = parse_learnings_file(agent_path)
        fresh = [l for l in learnings if l.freshness > 0.5]  // F1.4 filter
        all_learnings.extend(fresh)

    // Também coleta failures.md que geraram aprendizado
    for agent_path in glob("memory/agent/*/failures.md"):
        failures = parse_failures_file(agent_path)
        for f in failures:
            if f.lessons_learned and f.freshness > 0.5:
                all_learnings.append(f.as_learning())

    // Agrupa por domínio
    by_domain = group_by_domain(all_learnings)

    // === FASE 2: AGRUPAMENTO ===
    groups = []
    for domain, learnings in by_domain:
        for i, a in enumerate(learnings):
            for b in learnings[i+1:]:
                similarity = calc_similarity(a, b)
                if similarity > 0.4:
                    add_to_group(groups, a, b, similarity)

    // Filtra grupos com 3+ learnings de 2+ agentes
    candidates = [g for g in groups if len(g.learnings) >= 3
                  and len(set(g.agents)) >= 2]

    // === FASE 3: EXTRAÇÃO ===
    principles = []
    for group in candidates:
        principle = synthesize_pattern(group)
        principle.maturity = calc_maturity(group)
        if principle.maturity > 0.7:
            principles.append(principle)

    // === FASE 4: VALIDAÇÃO ===
    approved = []
    rejected = []
    for p in principles:
        if not validate_against_constitution(p):
            rejected.append((p, "contradiz CONSTITUIÇÃO"))
            continue
        if not has_sufficient_evidence(p):
            rejected.append((p, "evidências insuficientes"))
            continue
        if not is_actionable(p):
            rejected.append((p, "princípio genérico — não acionável"))
            continue
        approved.append(p)

    // === FASE 5: COMPILAÇÃO ===
    for p in approved:
        register_in_principles_md(p)
        if p.maturity > 0.9:
            ddna = generate_amendment_ddna(p)
            submit_to_don(ddna, p)

    report = generate_report(all_learnings, candidates, approved, rejected)
    update_entropy_post_compilation(approved)  // F1.6 integration

    return report
```

### 2.2 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|-------------|----------------|
| Parse de 54 arquivos learnings.md | O(54 × L) | < 3s |
| Filtro freshness > 0.5 | O(N) | < 1s |
| Agrupamento por similaridade (N²) | O(N²) | < 8s (N ≤ 200) |
| Síntese de padrões | O(G) | < 5s |
| Validação | O(P) | < 3s |
| Compilação + relatório | O(P + A) | < 5s |
| **Total** | | **< 25s** |

> **N = total de learnings frescos, G = grupos formados, P = princípios candidatos, A = agentes**

---

## 3. FÓRMULA DE MATURIDADE DE PADRÃO

### 3.1 Fórmula Principal

```
pattern_maturity = count_learnings  × 0.3  +
                   avg_freshness    × 0.3  +
                   cross_agent_count × 0.2  +
                   impact_score     × 0.2
```

### 3.2 Componentes

| Componente | Variável | Peso | Descrição |
|------------|----------|------|-----------|
| **Quantidade de Learnings** | `count_learnings` | **0.3** | Quantos learnings suportam o padrão. Cap: 10 (satura — mais não aumenta) |
| **Freshness Médio** | `avg_freshness` | **0.3** | Média do freshness de todos os learnings no grupo. Quanto mais fresco, mais confiável. |
| **Cross-Agent Count** | `cross_agent_count` | **0.2** | Quantos agentes diferentes contribuíram. Cap: 5 agentes. Evidência de validação independente. |
| **Impact Score** | `impact_score` | **0.2** | Qual o impacto de seguir (ou não) este padrão. Ver §3.3. |

#### Detalhamento dos Componentes

```
count_learnings = min(learning_count / 10, 1.0)
  // 0 learnings → 0.0, 5 learnings → 0.5, 10+ learnings → 1.0

avg_freshness = media_aritmética(freshness de todos os learnings do grupo)
  // Fornecido por F1.4 Wisdom Decay
  // Já filtrado para > 0.5 na Fase 1

cross_agent_count = min(unique_agent_count / 5, 1.0)
  // 1 agente → 0.2, 2 agentes → 0.4, 5+ agentes → 1.0

impact_score = calc_impact(group_learnings)
  // Ver §3.3
```

### 3.3 Impact Score

O Impact Score mede o quão crítico é o padrão para a saúde do sistema:

```
impact_score = actionability × 0.4 +
               criticality  × 0.3 +
               scope        × 0.3
```

| Subcomponente | Descrição | Score Alto | Score Baixo |
|---------------|-----------|------------|-------------|
| **Actionability** (0.4) | O padrão prescreve uma ação específica? | "Sempre rodar `-race` em CI" = 1.0 | "Testes são importantes" = 0.2 |
| **Criticality** (0.3) | Ignorar o padrão causa dano real? | Violação de segurança = 1.0 | Convenção de estilo = 0.3 |
| **Scope** (0.3) | Quantos agentes/domínios são afetados? | Afeta runtime inteiro = 1.0 | Só um agente = 0.3 |

### 3.4 Thresholds de Maturidade

```
                   Maturidade do Padrão

    0.0           0.4            0.7         0.9         1.0
    │──────────────│──────────────│───────────│───────────│
    │  Insuficiente│  Padrão      │  Princípio│  Emenda   │
    │  (arquivado) │  (patterns/)│  (PRINCIPLES)│(CONST.) │
                  │              │           │           │
```

| Threshold | Classificação | Ação |
|-----------|--------------|------|
| **< 0.4** | 🟡 Insuficiente | Grupo existe mas não gera padrão. Arquiva para referência. |
| **0.4 — 0.7** | 🟢 Padrão | Registra em `knowledge/patterns/` como padrão reutilizável. |
| **0.7 — 0.9** | 🔵 Princípio | Registra em `PRINCIPLES.md` global. Todos os agentes devem seguir. |
| **> 0.9** | 🟣 Emenda | Gera proposta de emenda constitucional para aprovação do Don. |

### 3.5 Exemplo de Cálculo

```
Grupo: "Importância de rodar testes com -race flag"
Learnings:
  - L21 (cosca-kernel, freshness 0.92): "20 race conditions detectadas — -race flag revelou bugs"
  - L18 (cosca-qa, freshness 0.88): "Race conditions são invisíveis sem -race"
  - L45 (cosca-testing, freshness 0.85): "Testes sem -race dão falso negativo em concorrência"
  - L67 (cosca-security, freshness 0.80): "Race conditions podem ser vetor de ataque"

count_learnings = min(4/10, 1.0) = 0.400
avg_freshness = (0.92 + 0.88 + 0.85 + 0.80) / 4 = 0.863
cross_agent_count = min(4/5, 1.0) = 0.800
impact_score:
  actionability = "Sempre rodar go test -race em CI" → 1.0
  criticality = race conditions causam crashes em produção → 0.9
  scope = afeta QUALQUER pacote com concorrência → 1.0
  impact_score = 1.0×0.4 + 0.9×0.3 + 1.0×0.3 = 0.970

pattern_maturity = 0.400×0.3 + 0.863×0.3 + 0.800×0.2 + 0.970×0.2
                 = 0.120 + 0.259 + 0.160 + 0.194
                 = 0.733 → 🔵 PRINCÍPIO CANDIDATO
```

---

## 4. INTEGRAÇÃO COM F1.4 WISDOM DECAY

### 4.1 Contrato de Integração

O Experience Compiler é **consumidor** do Wisdom Decay. A relação é unidirecional: Wisdom Decay fornece freshness, Experience Compiler consome.

```
┌─────────────────────────┐     freshness_score     ┌─────────────────────────┐
│  Wisdom Decay Engine    │ ──────────────────────▶ │  Experience Compiler   │
│  (F1.4)                 │                          │  (F9.1)                │
│                         │     contradiction_flag   │                        │
│  - freshness_score      │ ──────────────────────▶ │  - Filtra na Fase 1    │
│  - contradiction_flag   │                          │  - Calcula avg_freshness│
│  - decay_type           │                          │  - Rejeita grupos       │
│  - last_used            │                          │    contraditórios      │
│  - usage_count          │                          │                        │
└─────────────────────────┘                          └─────────────────────────┘
```

### 4.2 Regras de Integração

| Regra | Origem | Aplicação no Compiler |
|-------|--------|----------------------|
| **Só compila fresco** | F1.4 freshness > 0.5 | Fase 1 — filtra learnings com freshness ≤ 0.5 |
| **Contradição bloqueia** | F1.4 contradiction_flag | Fase 2 — grupos com contradições internas não geram padrão |
| **Freshness alimenta maturity** | F1.4 freshness_score | Fase 3 — `avg_freshness` na fórmula de maturity |
| **Depreciação pós-compilação** | F1.4 | Após princípio aprovado, learnings podem ser marcados como `#compiled` |

### 4.3 Ciclo de Vida do Learning no Pipeline

```
Learning criado (freshness = 1.0)
  │
  ├── freshness > 0.5 → elegível para compilação
  │     │
  │     ├── Compilado com sucesso → tagged #compiled
  │     │     └── freshness ainda decai normalmente
  │     │
  │     └── Não compilado (não formou grupo) → tagged #uncompiled
  │           └── freshness decai → reavaliado na próxima semana
  │
  ├── 0.15 < freshness ≤ 0.5 → excluído da compilação
  │     └── Aguarda revalidação (REVALIDATE_NOW)
  │
  └── freshness ≤ 0.15 → DEPRECATED
        └── Excluído permanentemente
```

### 4.4 Efeito no Wisdom Decay

Quando um learning é compilado em um princípio, ele ganha um **bônus de freshness**:

```
freshness_bonus_compiled = 0.15
// Aplicado uma vez, imediatamente após compilação
// Reconhece que o learning foi validado como relevante
```

---

## 5. INTEGRAÇÃO COM F1.6 COGNITIVE ENTROPY

### 5.1 Compilação REDUZ Entropia

Esta é a relação mais importante: **a compilação de conhecimento desorganizado em princípios organizados reduz a entropia cognitiva do sistema.**

```
Entropia_antes = (contradictions×3 + stale×2 + gaps×1) / total_knowledge
                                                            │
                                     ┌──────────────────────┘
                                     ▼
                            Experience Compiler
                              (organiza conhecimento)
                                     │
                                     ▼
Entropia_depois = (contradictions'×3 + stale'×2 + gaps'×1) / total_knowledge'

Onde:
  contradictions' = contradictions - contradições_resolvidas_pela_compilação
  stale'          = stale (não muda diretamente)
  gaps'           = gaps - gaps_fechados_pelos_princípios
  total_knowledge' = total_knowledge (permanece igual — não removemos learnings)
```

### 5.2 Mecanismo de Redução de Entropia

| Ação do Compiler | Efeito na Entropia | Componente Afetado |
|-----------------|-------------------|-------------------|
| Agrupa learnings contraditórios e rejeita o grupo como padrão | Contradição é detectada e RESOLVIDA (grupo marcado como inválido) | contradictions ↓ |
| Aprova princípio que consolida múltiplos learnings em uma regra | Conhecimento organizado — menos afirmações soltas | gaps ↓ (indireto) |
| Gera DDNA para cada princípio aprovado | Gaps sem DDNA são fechados | gaps ↓ (direto) |
| Rejeita princípio genérico com justificativa | Decisão documentada sobre por que NÃO virou regra | gaps ↓ (decisão registrada) |

### 5.3 Fórmula de Redução de Entropia

```
entropy_reduction = Δentropy = entropy_after - entropy_before
                              = -reduction  (sempre negativo ou zero)

Onde:
  reduction = resolved_contradictions × weight_contradiction +
              closed_gaps              × weight_gap

  weight_contradiction = 3 / total_knowledge
  weight_gap           = 1 / total_knowledge
```

### 5.4 Exemplo

```
Antes da compilação:
  contradictions = 2, stale = 0, gaps = 4, total = 100
  entropy = (2×3 + 0×2 + 4×1) / 100 = 10/100 = 0.100

Após compilação (princípios aprovados consolidam 1 contradição e fecham 2 gaps):
  contradictions = 1, stale = 0, gaps = 2, total = 100
  entropy = (1×3 + 0×2 + 2×1) / 100 = 5/100 = 0.050

Δentropy = 0.050 - 0.100 = -0.050 → 🟢 ENTROPIA REDUZIDA
```

### 5.5 Meta-Métrica: Eficiência da Compilação

```
compilation_efficiency = entropy_reduction / compilation_cost

Onde:
  entropy_reduction = Δentropy (absoluto)
  compilation_cost = tempo_de_execução_em_segundos / 30s
                     (normalizado: 30s = 1.0, 15s = 0.5)

  efficiency > 0.01 → compilação valeu a pena
  efficiency > 0.05 → excelente retorno
  efficiency < 0.001 → compilação não está gerando valor
```

---

## 6. INTEGRAÇÃO COM CONSTITUIÇÃO

### 6.1 Pipeline de Emenda Constitucional

```
┌──────────────────────────────────────────────────────────────────────────┐
│               PIPELINE DE EMENDA CONSTITUCIONAL                           │
│                                                                           │
│                                                                           │
│   ┌─────────────────────┐                                                 │
│   │ Princípio aprovado  │                                                 │
│   │ com maturity > 0.9  │                                                 │
│   └──────────┬──────────┘                                                 │
│              ▼                                                            │
│   ┌─────────────────────────────────────────────┐                        │
│   │ 1. GERA DDNA DE PROPOSTA                    │                        │
│   │    - Nome do princípio                      │                        │
│   │    - Definição                               │                        │
│   │    - Evidências (learnings que suportam)     │                        │
│   │    - Artigo da CONSTITUIÇÃO afetado          │                        │
│   │    - Texto proposto da emenda                │                        │
│   │    - Impacto estimado                        │                        │
│   └──────────────────┬──────────────────────────┘                        │
│                      ▼                                                    │
│   ┌─────────────────────────────────────────────┐                        │
│   │ 2. SUBMETE PARA O DON                        │                        │
│   │    - Relatório semanal + notificação direta  │                        │
│   │    - Don tem 7 dias para responder           │                        │
│   │    - Se não responder → re-submissão após 7d │                        │
│   └──────────────────┬──────────────────────────┘                        │
│                      ▼                                                    │
│        ┌─────────────┴─────────────┐                                     │
│        ▼                           ▼                                     │
│   ┌──────────┐              ┌──────────────┐                             │
│   │ DON      │              │ DON REJEITA  │                             │
│   │ APROVA   │              │              │                             │
│   └────┬─────┘              │ Arquivar     │                             │
│        ▼                    │ DDNA com     │                             │
│   ┌────────────────────┐    │ status       │                             │
│   │ 3. ATUALIZA A      │    │ rejected     │                             │
│   │    CONSTITUIÇÃO    │    │ + justificativa                            │
│   │                    │    └──────────────┘                             │
│   │    - Adiciona novo │                                                │
│   │      princípio     │                                                │
│   │      (P9, P10...)  │                                                │
│   │    - Atualiza      │                                                │
│   │      amendment log │                                                │
│   │    - Data da       │                                                │
│   │      ratificação   │                                                │
│   └────────────────────┘                                                 │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Formato do DDNA de Proposta de Emenda

```yaml
---
id: "DDNA-AMEND-YYYY-MM-DD-NN"
title: "Proposta de Emenda Constitucional: {nome_do_principio}"
status: proposed
date: YYYY-MM-DD
agents:
  - cosca-experience-compiler
  - {agente_1_contribuinte}
  - {agente_2_contribuinte}
domain: constitutional-amendment
decision_level: 5
confidence: {pattern_maturity}
revisit: YYYY-MM-DD + 90
tags:
  - dna
  - experience-compiler
  - constitutional-amendment
  - {dominio}
supersedes: null
superseded_by: null
don_approved: null
---

## Context

O Experience Compiler identificou que o princípio abaixo atingiu
maturidade > 0.9, sendo candidato a emenda constitucional:

- **Princípio**: {nome}
- **Definição**: {definição}
- **Maturity Score**: {pattern_maturity}
- **Learnings que o suportam**: {lista}
- **Agentes envolvidos**: {lista}

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
- Posição: após P{ultimo} (entre P{ultimo} e PARTE II)

## Evidence

{lista_detalhada_dos_learnings_com_freshness_e_agente}

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

### 6.3 Regras de Aprovação do Don

| Cenário | Procedimento |
|---------|-------------|
| **Don aprova** | Atualizar CONSTITUIÇÃO.md → incrementar versão → adicionar amendment log → notificar todos os agentes |
| **Don rejeita** | Arquivar DDNA com status `rejected` + justificativa → learning que gerou a proposta ganha tag `#constitutional-rejected` |
| **Don modifica** | Incorporar alteração → re-submeter como DDNA revisado → Don aprova versão final |
| **Don não responde em 7 dias** | Re-submissão automática → se ignorado por 14 dias, arquivar como `deferred` |

### 6.4 Salvaguardas

```
🛡️ NENHUM agente pode modificar a CONSTITUIÇÃO sem aprovação do Don.
   Isto é garantido por P4 (Don tem veto absoluto).

🛡️ Propostas de emenda exigem maturity > 0.9.
   Abaixo disso, o princípio fica em PRINCIPLES.md como guia, não como lei.

🛡️ Toda emenda é rastreável via DDNA.
   O histórico de amendments na CONSTITUIÇÃO referencia o DDNA.

🛡️ O Don pode propor emendas diretamente (sem o pipeline).
   A CONSTITUIÇÃO existe para SERVIR o Don, não para limitá-lo.
```

---

## 7. INTEGRAÇÃO COM F7.3 ENGINEERING SCORE

O Engineering Score (F7.3) alimenta o Experience Compiler com **métrica de qualidade de commits** que valida se os princípios estão sendo seguidos.

### 7.1 Compliance Score por Princípio

```
compliance_score(principio_P) =
    commits_que_seguem_P / commits_que_deveriam_seguir_P

Onde:
  commits_que_seguem_P = commits que passaram no quality gate do princípio P
  commits_que_deveriam_seguir_P = commits que tocaram código no domínio de P

  compliance_score > 0.9 → 🟢 Princípio sendo seguido
  compliance_score < 0.5 → 🔴 Princípio ignorado — reavaliar necessidade
```

### 7.2 Feedback Loop

```
┌─────────────────┐     princípios     ┌──────────────────────┐
│  Experience      │ ────────────────▶ │  Engineering Score   │
│  Compiler (F9.1) │                    │  (F7.3)              │
│                  │                    │                      │
│  - PRINCIPLES.md │                    │  - Verifica compliance│
│  - Emendas       │                    │  - Ajusta scoring    │
└─────────────────┘                    └──────────┬───────────┘
         ▲                                        │
         │           compliance_data              │
         └────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │  Próxima compilação  │
              │  considera compliance │
              │  no impact_score     │
              └──────────────────────┘
```

---

## 8. EXEMPLO COM LEARNINGS REAIS

### 8.1 Cenário: Dead Code Removal Pattern

**5 learnings sobre "dead code removal" de 3 agentes diferentes:**

| # | Agente | Data | Freshness | Afirmação Central |
|---|--------|------|-----------|-------------------|
| L20 | cosca-kernel | 2026-07-29 | 0.90 | "Código morto só se detecta com análise de grafo de imports. `go list -f '{{join .Deps}}' ./cmd/cosca/ \| grep providers` provou que 6.615 linhas nunca entram no binário." |
| L22 | cosca-kernel | 2026-07-30 | 0.93 | "O padrão de classificação de gaps de cobertura em 3 categorias é reutilizável. Tipo A (código morto): remove." |
| L35 | cosca-architecture | 2026-07-30 | 0.87 | "Código morto aumenta custo de manutenção sem benefício. Deve ser identificado por análise de dependências (go list -deps) antes de qualquer refatoração." |
| L52 | cosca-performance | 2026-07-30 | 0.82 | "Dead code não é só código não usado — é código que COMPILA mas não é IMPORTADO. `go build` não detecta. Precisa de `go list -deps` ou `go vet` com análise de reachability." |
| L71 | cosca-review | 2026-07-29 | 0.78 | "Revisões de código devem incluir verificação de dead code. Commits que removem código morto sem verificar imports são risco de regressão." |

### 8.2 Pipeline em Ação

#### Fase 1 — Coleta
```
Total de learnings varridos: 54 agentes
Learnings com freshness > 0.5: 5 (todos acima do threshold)
Learnings filtrados (freshness ≤ 0.5): 0
Domínios identificados: architecture, testing, code-quality, maintenance
```

#### Fase 2 — Agrupamento
```
Similaridades calculadas:

L20 ↔ L22: tag_overlap = {#dead-code-removal, #runtime-coverage} → Jaccard = 0.40
           domain_match = 1.0 (ambos architecture)
           similaridade = 0.7×0.40 + 0.3×1.0 = 0.58 ✅

L20 ↔ L35: tag_overlap = {#dead-code-removal, #architecture} → Jaccard = 0.33
           domain_match = 1.0
           similaridade = 0.7×0.33 + 0.3×1.0 = 0.53 ✅

L20 ↔ L52: tag_overlap = {#dead-code-removal} → Jaccard = 0.20
           domain_match = 0.5 (architecture ↔ performance)
           similaridade = 0.7×0.20 + 0.3×0.5 = 0.29 ❌

L22 ↔ L35: tag_overlap = {#dead-code-removal, #coverage} → Jaccard = 0.40
           domain_match = 1.0
           similaridade = 0.7×0.40 + 0.3×1.0 = 0.58 ✅

... (demais pares)

Grupo formado: {L20, L22, L35, L71} — 4 learnings, 3 agentes ✅ (≥3, ≥2)
L52 ficou de fora (similaridade < 0.4 com o grupo)
```

#### Fase 3 — Extração de Padrão
```
Síntese do padrão:
  "Aprendemos que dead code removal REQUER análise de grafo de dependências
   (`go list -deps`) antes da remoção. Código que compila mas não é importado
   não é detectado por `go build`. A classificação em 3 tipos (A: remove,
   B: testa, C: documenta) é um framework de decisão validado."

Cálculo de Maturity:

count_learnings = min(4/10, 1.0) = 0.400
avg_freshness = (0.90 + 0.93 + 0.87 + 0.78) / 4 = 0.870
cross_agent_count = min(3/5, 1.0) = 0.600
impact_score:
  actionability = "Sempre verificar go list -deps antes de remover" → 1.0
  criticality = Remover sem verificar pode quebrar build → 0.85
  scope = Aplica a qualquer pacote Go → 1.0
  impact_score = 1.0×0.4 + 0.85×0.3 + 1.0×0.3 = 0.955

pattern_maturity = 0.400×0.3 + 0.870×0.3 + 0.600×0.2 + 0.955×0.2
                 = 0.120 + 0.261 + 0.120 + 0.191
                 = 0.692 → 🟢 PADRÃO (0.4-0.7)
```

**Resultado:** maturity 0.692 — fica como padrão em `knowledge/patterns/`. Não atinge threshold de princípio (0.7) por pouco — falta mais 1 learning cross-agent para subir o `cross_agent_count` ou mais 1 learning para subir o `count_learnings`.

#### Fase 4 — Validação
```
1. 🛡️ Contradiz CONSTITUIÇÃO?
   → P2 (Código executado é a verdade) é COMPLEMENTADO pelo padrão
   → P2 diz "código executado vence" — o padrão diz "verifique o que é executado"
   → Sem contradição ✅

2. 📊 Evidências suficientes?
   → 4 learnings ≥ 3 ✅
   → 3 agentes ≥ 2 ✅
   → Todos freshness > 0.5 ✅

3. 🎯 Acionável?
   → "Sempre verificar go list -deps antes de remover código morto"
   → Verbo de ação: "verificar"
   → Ação específica: rodar `go list -f '{{join .Deps}}' ./cmd/cosca/ | grep <pacote>`
   → ✅ Aprovado

Resultado: ✅ APROVADO (como padrão, maturity 0.692 < 0.7)
```

#### Fase 5 — Compilação
```
Registrado em knowledge/patterns/dead-code-removal.md
Relatório:
  - Learnings processados: 5 (L20, L22, L35, L52, L71)
  - Learnings no grupo: 4 (L20, L22, L35, L71)
  - L52 excluído (similaridade baixa)
  - Maturity: 0.692 — PADRÃO (não princípio)
  - Ação: aguardar mais evidências para próxima compilação
```

### 8.3 Projeção: Se Houvesse Mais Evidências

```
Se L52 (cosca-performance) tivesse similaridade > 0.4 com o grupo:

count_learnings = min(5/10, 1.0) = 0.500
avg_freshness = (0.90+0.93+0.87+0.82+0.78) / 5 = 0.860
cross_agent_count = min(4/5, 1.0) = 0.800
impact_score = 0.955 (mesmo)

pattern_maturity = 0.500×0.3 + 0.860×0.3 + 0.800×0.2 + 0.955×0.2
                 = 0.150 + 0.258 + 0.160 + 0.191
                 = 0.759 → 🔵 PRINCÍPIO CANDIDATO!
```

### 8.4 Exemplo de Emenda Constitucional

Se o princípio "Dead Code Requer `go list -deps`" atingisse maturity > 0.9:

```
Proposta de Emenda — Adicionar P9 à CONSTITUIÇÃO:

### P9 — CÓDIGO MORTO REQUER VERIFICAÇÃO DE DEPENDÊNCIAS

**Regra:** Nenhum bloco de código pode ser removido como "dead code" sem
verificação de grafo de dependências via `go list -deps` ou ferramenta
equivalente. `go build` não é prova suficiente de que código não é usado.

**Aplicação prática:**
- Antes de remover qualquer função, arquivo ou pacote, executar:
  `go list -f '{{join .Deps}}' ./cmd/cosca/ | grep <target>`
- Código classificado como Tipo A (dead/inatingível) pode ser removido
- Código Tipo B (edge case raro) deve ser testado, não removido
- Código Tipo C (defensivo/platform-specific) deve ser documentado

**Quem garante:** cosca-performance e cosca-review — verificação em code review.
```

---

## 9. CLI E AUTOMAÇÃO

### 9.1 Comandos

```bash
# Executar pipeline completo de compilação
cosca experience compile

# Executar em modo dry-run (não modifica arquivos)
cosca experience compile --dry-run

# Ver relatório da última compilação
cosca experience report

# Ver princípios ativos
cosca experience principles

# Ver histórico de compilações
cosca experience history --weeks 12

# Ver propostas de emenda pendentes
cosca experience amendments pending

# Ver propostas de emenda rejeitadas
cosca experience amendments rejected

# Executar apenas fases específicas
cosca experience compile --phases 1,2    # Só coleta + agrupamento
cosca experience compile --phases 3,4,5  # Só extração + validação + compilação

# Inspecionar um learning específico no contexto do compiler
cosca experience inspect <learning-id>

# Forçar recompilação de um grupo específico
cosca experience recompile --group <group-id>
```

### 9.2 Opções

| Flag | Descrição | Default |
|------|-----------|---------|
| `--dry-run` | Calcula mas não modifica arquivos | `false` |
| `--phases` | Lista de fases para executar (ex: `1,2,3`) | `1,2,3,4,5` |
| `--min-freshness` | Threshold mínimo de freshness | `0.5` |
| `--min-similarity` | Threshold de similaridade para agrupamento | `0.4` |
| `--min-maturity` | Threshold de maturity para princípio | `0.7` |
| `--amendment-threshold` | Threshold para emenda constitucional | `0.9` |
| `--agent` | Filtrar por agente específico | all |
| `--domain` | Filtrar por domínio | all |
| `--format` | Formato de output: `table`, `json`, `yaml` | `table` |
| `--output` | Arquivo de output do relatório | stdout |

### 9.3 Scheduler

```bash
# Executar semanalmente (domingo à meia-noite)
0 0 * * 0 cosca experience compile --format json >> /var/log/experience-compiler.log

# Executar após Wisdom Decay (via hook)
cosca wisdom-decay run && cosca experience compile
```

### 9.4 CI Gate

```yaml
# .github/workflows/experience-compiler-weekly.yml
name: Experience Compiler — Weekly Compilation
on:
  schedule:
    - cron: "0 0 * * 0"  # Semanal
  workflow_dispatch:

jobs:
  compile:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Experience Compiler
        run: |
          cosca experience compile --format json --output compile-report.json

      - name: Check for Pending Amendments
        run: |
          PENDING=$(jq '.amendments.pending | length' compile-report.json)
          if [ "$PENDING" -gt 0 ]; then
            echo "📜 $PENDING constitutional amendment(s) pending Don approval:"
            jq -r '.amendments.pending[] | "  • \(.title) (maturity: \(.maturity))"' compile-report.json
          fi

      - name: Check Compilation Health
        run: |
          PRINCIPLES=$(jq '.principles_approved | length' compile-report.json)
          REJECTED=$(jq '.principles_rejected | length' compile-report.json)
          echo "✅ Principles approved: $PRINCIPLES"
          echo "❌ Principles rejected: $REJECTED"
          echo "📊 Entropy reduction: $(jq '.entropy_reduction' compile-report.json)"
```

---

## 10. MÉTRICAS DO ENGINE

### 10.1 Métricas Internas

| Métrica | Definição | Alvo | Fonte |
|---------|-----------|:----:|-------|
| **Pipeline Duration** | Tempo total de execução | < 30s | Engine |
| **Agents Scanned** | Agentes varridos | 54/54 | Engine |
| **Learnings Processed** | Total de learnings com freshness > 0.5 | Incremental | Fase 1 |
| **Learnings Filtered** | Learnings com freshness ≤ 0.5 (excluídos) | < 20% | Fase 1 |
| **Groups Formed** | Grupos com similaridade > 0.4 | Informativo | Fase 2 |
| **Patterns Extracted** | Padrões com maturity ≥ 0.4 | > 5/semana | Fase 3 |
| **Principles Approved** | Princípios com maturity ≥ 0.7 | > 2/semana | Fase 4 |
| **Principles Rejected** | Princípios reprovados na validação | < 30% | Fase 4 |
| **Amendments Proposed** | Emendas com maturity > 0.9 submetidas ao Don | < 1/mês | Fase 5 |
| **Entropy Reduction** | Δentropy pós-compilação (ver §5.3) | > 0.01 | F1.6 |
| **Compilation Efficiency** | entropy_reduction / compilation_cost | > 0.01 | §5.5 |

### 10.2 Métricas de Qualidade dos Princípios

| Métrica | Definição | Alvo |
|---------|-----------|:----:|
| **Principle Adoption Rate** | % de agentes que seguem o princípio | > 80% |
| **Principle Survival Rate** | % de princípios ainda válidos após 3 meses | > 90% |
| **False Positive Rate** | Princípios que foram revogados | < 10% |
| **Cross-Agent Coverage** | Média de agentes por princípio | > 3 |
| **Avg Principle Maturity** | Média do maturity de todos os princípios ativos | > 0.75 |

### 10.3 Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| Pipeline > 30s | 🟡 MÉDIO | Otimizar agrupamento (N² é gargalo) |
| 0 princípios aprovados por 2+ semanas consecutivas | 🟡 MÉDIO | Verificar se learnings estão sendo registrados |
| Learnings filtered > 40% | 🟡 MÉDIO | Wisdom Decay pode estar muito agressivo — verificar thresholds |
| Entropy aumentou após compilação | 🔴 ALTO | Compilação está gerando mais desorganização — revisar pipeline |
| Emenda pendente > 14 dias sem resposta | 🟡 MÉDIO | Re-notificar Don |
| Princípio contradiz CONSTITUIÇÃO | 🔴 ALTO | Validação falhou — revisar algoritmo |

---

## 11. CASOS DE BORDA E ANTI-PADRÕES

### 11.1 Casos de Borda

| Caso | O que acontece | Tratamento |
|------|---------------|------------|
| **Nenhum learning fresco** (todos freshness ≤ 0.5) | Pipeline termina na Fase 1 com 0 aprendizado | Relatório: "Nenhum learning fresco — execute Wisdom Decay primeiro" |
| **Apenas 1 agente com learnings** | Grupo formado mas cross_agent_count = 0.2 | Padrão no máximo maturity 0.6-0.7. Não vira princípio sem 2+ agentes. |
| **Grupo com 10+ learnings** | count_learnings satura em 1.0 | Maturity limitado pelos outros componentes. Bom sinal — padrão consolidado. |
| **Grupo com learnings contraditórios** | Similaridade alta mas afirmações opostas | Grupo marcado como CONFLICT. Não gera padrão. Gera DDNA de contradição. |
| **Princípio aprovado mas depois refutado** | Novo learning contradiz princípio existente | Princípio marcado como `superseded`. Removido de PRINCIPLES.md, movido para archive. |
| **Don modifica emenda proposta** | Don edita o texto antes de aprovar | DDNA registra versão original + modificação do Don. CONSTITUIÇÃO recebe versão final. |
| **Learning compilado em 2+ grupos** | Learning multi-domínio participa de múltiplos padrões | Permitido — um learning pode suportar múltiplos princípios. |
| **Maturity exato no threshold (0.700)** | Borda entre padrão e princípio | Critério de desempate: se impact_score > 0.8 → princípio. Senão → padrão. |

### 11.2 Anti-Padrões

| Anti-Padrão | Por que evitar | Como detectar |
|-------------|---------------|---------------|
| **Compilar toda semana sem necessidade** | Se não há learnings novos, compilação é redundante | Executar apenas se houver learnings novos desde última compilação |
| **Aceitar princípios genéricos** | "Código deve ser testado" é verdade mas não é acionável | Validação de acionabilidade na Fase 4 |
| **Ignorar freshness na compilação** | Conhecimento podre vira princípio podre | Filtro obrigatório na Fase 1 |
| **Permitir que 1 agente domine** | Princípios viesados para um único domínio | Regra de mínimo 2 agentes diferentes |
| **Criar princípios conflitantes** | Dois princípios que se contradizem | Validação contra CONSTITUIÇÃO na Fase 4 |
| **Sobrecarregar o Don com emendas** | Toda semana uma proposta de emenda | Threshold alto (maturity > 0.9) e frequência máxima mensal |
| **Compilar sem contexto** | Extrair padrão de learnings sem entender o contexto | Fase 3 inclui campo `Context` na síntese |

### 11.3 Regras de Resiliência

```
1. Falha no Wisdom Decay → Compilação não executa
   (não temos freshness para filtrar)

2. Falha em 1 agente → Os outros 53 continuam
   (resiliência por design — 54 agentes)

3. Falha na Fase 3 → Relatório parcial com Fases 1-2
   (pelo menos sabemos o que foi coletado)

4. Falha na escrita do PRINCIPLES.md → Rollback automático
   (arquivo anterior preservado em backup)

5. Don desconectado → Emendas acumulam em pending/
   (máximo 30 dias — após isso, arquivar como deferred)
```

---

## 12. PRINCÍPIOS GLOBAIS DO COMPILER

```
┌─────────────────────────────────────────────────────────────────────┐
│               PRINCÍPIOS DO EXPERIENCE COMPILER                      │
│                                                                      │
│  1. Conhecimento podre não vira princípio                           │
│     (freshness ≤ 0.5 = excluído)                                    │
│                                                                      │
│  2. Um learning não é um padrão                                     │
│     (mínimo 3 evidências)                                            │
│                                                                      │
│  3. Um agente não é uma verdade universal                            │
│     (mínimo 2 agentes diferentes)                                    │
│                                                                      │
│  4. Princípio genérico não é princípio                              │
│     (deve ser acionável — prescrever uma ação)                       │
│                                                                      │
│  5. A CONSTITUIÇÃO não se altera sozinha                            │
│     (Don aprova toda emenda)                                         │
│                                                                      │
│  6. Compilar organiza — o oposto de entropia                        │
│     (cada princípio aprovado reduz desorganização)                   │
│                                                                      │
│  7. O Don pode ignorar o pipeline                                   │
│     (P4 — Don tem veto absoluto. Pode propor emendas diretamente)    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — pipeline 5 fases, fórmula de maturity, integrações com F1.4, F1.6, F7.3, CONSTITUIÇÃO, exemplo com dead code removal de 3 agentes, CLI, métricas, casos de borda |

---

> *"Centenas de learnings dispersos são ruído. Centenas de learnings compilados em princípios são sabedoria. A diferença entre ter dados e ter conhecimento é a compilação."*
> — Cosca Architecture Chief, 2026-07-30
