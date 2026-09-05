# PATTERN EVOLUTION ENGINE — F2.6 Sistema de Evolução de Padrões

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
>
> **Workflow**: `cosca-pattern-evolution` | **Fase CMI**: Fase 2 — Memória & Conhecimento | **Código**: F2.6
> **Dependências**: F9.1 (Experience Compiler) | F1.4 (Wisdom Decay) | F0.6 (Cognitive Audit Loop) | patterns.md de todos os 54 agentes
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implementa P1 (memória acima de tudo), P2 (código é a verdade), P5 (aprender com erros). Estende [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §5 C6 — Pattern Evolution.
>
> **Lema**: *"Padrões são seres vivos. Nascem de um learning, amadurecem com uso, evoluem com validação e morrem quando o contexto muda. O sistema não guarda padrões — guarda a evolução deles."*
>
> **Consulte também**:
> - [Experience Compiler (F9.1)](../experience-compiler/SKILL.md) — promove padrões maduros a princípios e emendas constitucionais
> - [WISDOM_DECAY.md (F1.4)](../wisdom-decay/WISDOM_DECAY.md) — freshness score, decay types, revalidação por falta de uso
> - [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §5 C6 — definição conceitual do Pattern Evolution
> - [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) — Stages 7-8, post-task extraction
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — formato de aprendizado que alimenta padrões

---

## SUMÁRIO

1. [Definição — Padrões como Seres Vivos](#1-definição--padrões-como-seres-vivos)
2. [Ciclo de Vida — NASCIMENTO → CANDIDATO → MADURO → PRINCÍPIO → MORTE](#2-ciclo-de-vida)
3. [Fórmula de Maturidade — pattern_evolution_score](#3-fórmula-de-maturidade)
4. [Pipeline Semanal (< 5s)](#4-pipeline-semanal--5s)
5. [Integração com F9.1 — Experience Compiler](#5-integração-com-f91--experience-compiler)
6. [Integração com F1.4 — Wisdom Decay](#6-integração-com-f14--wisdom-decay)
7. [Versionamento MAJOR.MINOR](#7-versionamento-majorminor)
8. [Formato do Registro de Evolução](#8-formato-do-registro-de-evolução)
9. [Métricas de Saúde do Padrão](#9-métricas-de-saúde-do-padrão)
10. [Protocolo de Depreciação (MORTE)](#10-protocolo-de-depreciação-morte)
11. [Governança e Regras Non-Negotiable](#11-governança-e-regras-non-negotiable)
12. [Armazenamento — patterns/ e pattern_history](#12-armazenamento)
13. [Consulta e Descoberta de Padrões](#13-consulta-e-descoberta-de-padrões)
14. [Exemplo Real — same-package-access (L22)](#14-exemplo-real--same-package-access-l22)
15. [Métricas do Sistema e Alertas](#15-métricas-do-sistema-e-alertas)
16. [CLI e Automação](#16-cli-e-automação)
17. [Referências](#17-referências)

---

## 1. DEFINIÇÃO — PADRÕES COMO SERES VIVOS

### 1.1 O que é o Pattern Evolution Engine

O **Pattern Evolution Engine** (F2.6) gerencia o **ciclo de vida dos padrões**: nascimento (descoberto), maturidade (validado 3+ vezes), evolução (melhorado), morte (obsoleto). Padrões não são documentos estáticos — são **seres vivos** com nascimento, crescimento, evolução e morte.

Um padrão nasce num **learning** (`internal/embed/cosca/memory/agent/*/learnings.md`), é promovido a `patterns.md` quando a repetição é observada, é validado com uso bem-sucedido repetido, evolui através de novas versões, e eventualmente **morre** (vira obsoleto) ou **vira princípio constitucional** (transcende a condição de padrão).

### 1.2 A Pergunta que Esta Engine Responde

> **"Qual a saúde do conhecimento reutilizável do Cosca?"**
> **"Quais padrões estão nascendo, quais estão maduros, quais estão morrendo?"**
> **"O que deve ser promovido a princípio constitucional?"**

### 1.3 Analogia: Ecossistema Biológico

```
┌─────────────────────────────────────────────────────────────────────────┐
│          PATTERN EVOLUTION — ANALOGIA DE ECOSSISTEMA                     │
│                                                                          │
│  Nascimento  → learning isolado observado no metacognition pipeline      │
│  Infância    → padrão candidato (#candidate) — frágil, precisa provar    │
│  Maturidade  → padrão validado (#validated) — confiável, cross-agent     │
│  Reprodução  → padrão maduro alimenta F9.1 → vira princípio (#principle) │
│  Evolução    → novas versões MAJOR.MINOR refinam a abordagem             │
│  Morte       → padrão obsoleto → #stale → pattern_history (nunca apaga)  │
│  Fóssil      → pattern_history preserva o que foi aprendido              │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.4 Estado Atual vs Estado Alvo

**Hoje**: `patterns.md` de 54 agentes contém padrões em formatos heterogêneos — alguns versionados (kernel), a maioria sem tag de ciclo de vida. Não existe fórmula de maturidade nem pipeline de varredura semanal. A transição learning → padrão → princípio é implícita.

**Amanhã**:
- Todo padrão tem tag de ciclo de vida: `#candidate`, `#validated`, `#principle`, `#stale`
- Toda semana, o pipeline calcula o `pattern_evolution_score` de todos os padrões em < 5s
- Padrões maduros fluem automaticamente para o F9.1 (Experience Compiler)
- Padrões mortos são movidos para `pattern_history/` — preservados, nunca excluídos

### 1.5 Escopo

**Cobre**:
- Ciclo de vida completo: NASCIMENTO → CANDIDATO → MADURO → PRINCÍPIO → MORTE
- Fórmula de maturidade (`pattern_evolution_score`) com thresholds
- Pipeline semanal de varredura e classificação (< 5s)
- Integração bidirecional com F9.1 (promoção) e F1.4 (decay/reavaliação)
- Versionamento MAJOR.MINOR e trilha de evolução
- Protocolo formal de depreciação com preservação histórica

**NÃO cobre**:
- Extração de padrões do metacognition pipeline (responsabilidade do F0.6, Stages 7-8)
- Armazenamento físico em SQLite (responsabilidade do Knowledge Compiler)
- A decisão de aprovar emenda constitucional (responsabilidade do Don via F9.1)
- Deleção de padrões (padrões NUNCA são deletados — vão para `pattern_history/`)

---

## 2. CICLO DE VIDA

### 2.1 Diagrama do Ciclo de Vida

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CICLO DE VIDA DE UM PADRÃO                        │
│                                                                      │
│  NASCIMENTO: learning isolado → observação de repetição              │
│      │                                                                │
│      │  3ª ocorrência do mesmo comportamento reutilizável            │
│      ▼                                                                │
│  CANDIDATO: registrado em patterns.md com tag #candidate             │
│      │                                                                │
│      │  3+ usos bem-sucedidos (cross-agent ou cross-context)        │
│      ▼                                                                │
│  MADURO: promovido a padrão validado (#validated)                    │
│      │                                                                │
│      ├──────────────────────► usado 10+ vezes                        │
│      │                          │                                    │
│      │                          ▼                                    │
│      │                    PRINCÍPIO: candidato a emenda               │
│      │                    constitucional (via F9.1 → Don)            │
│      │                          │                                    │
│      │                          ▼                                    │
│      │                    #principle (aprovado pelo Don)             │
│      │                                                                │
│      │  obsoleto / contradito / não usado há 90 dias                │
│      ▼                                                                │
│  MORTE: depreciado → move para pattern_history (NÃO exclui)          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Os Cinco Estados

| Estado | Tag | Critério de Entrada | Comportamento |
|--------|-----|--------------------|---------------|
| **NASCIMENTO** | (sem tag — no learnings.md) | Observação de comportamento reutilizável em 1 learning | Ainda não é padrão formal; vive no `learnings.md` do agente |
| **CANDIDATO** | `#candidate` | 3ª ocorrência da mesma observação de repetição | Registrado em `patterns.md`; sugerido com cautela; confidence inicial 0.70 |
| **MADURO** | `#validated` | 3+ usos bem-sucedidos (≥ 3 agentes ou 3 contextos distintos) | Sugerido para todos os agentes; confidence 0.85-1.00; elegível para promoção |
| **PRINCÍPIO** | `#principle` | Usado 10+ vezes E `pattern_evolution_score` > 0.7 | Alimenta F9.1; vira princípio em PRINCIPLES.md; pode gerar emenda constitucional (Don aprova) |
| **MORTE** | `#stale` (depois `#deprecated`) | `pattern_evolution_score` < 0.2 OU obsoleto/contradito OU não usado há 90 dias | Movido para `pattern_history/`; preservado integralmente |

### 2.3 Transições Detalhadas

#### NASCIMENTO → CANDIDATO

Um padrão nasce quando um learning isolado observa um comportamento reutilizável. A **transição para candidato** ocorre na **3ª ocorrência** da mesma observação de repetição — três tarefas ou três agentes notam o mesmo "isso funciona de novo".

```
Gatilhos de transição (qualquer um):
  - 3 learnings independentes mencionando a mesma técnica (tag overlap ≥ 60%)
  - 1 learning + 2 aplicações reais documentadas em tasks subsequentes
  - 3 agentes diferentes citando o mesmo approach no learnings.md
  - Insight Generator (F3.1) detecta padrão: "3+ similar observations → pattern detection"

Ação:
  - Memory Chief formaliza em patterns.md com tag #candidate
  - Registra id (PATTERN-NNN), domínio, primeira ocorrência, learnings de origem
  - Inicia contagem de pattern_age e pattern_uses
```

#### CANDIDATO → MADURO

A promoção a **#validated** exige prova de eficácia repetida:

```
Critérios:
  - 3+ usos bem-sucedidos documentados
  - Mínimo 2 agentes diferentes OU 3 contextos/tasks diferentes
  - success_rate ≥ 0.80 nas aplicações registradas
  - Nenhuma falha crítica registrada contra o padrão
  - pattern_evolution_score ≥ 0.4 (crescente)

Ação:
  - Tag atualizada para #validated
  - Confidence promovida para 0.85-1.00
  - Sugerido para todos os agentes (sem tag de aviso)
  - Revalidação programada a cada 90 dias (integração F1.4)
```

#### MADURO → PRINCÍPIO

Um padrão maduro **transcende** quando prova valor em escala:

```
Critérios:
  - Usado 10+ vezes (pattern_uses ≥ 10)
  - pattern_evolution_score > 0.7
  - Cross-agent: 3+ agentes ou 2+ projetos aplicaram
  - Success rate ≥ 0.85

Fluxo (via F9.1):
  1. F2.6 notifica o F9.1 Experience Compiler (evento pattern.mature)
  2. F9.1 agrupa o padrão com seus learnings de origem
  3. F9.1 calcula pattern_maturity (fórmula do F9.1, §3)
  4. Se maturity > 0.7 → registra em PRINCIPLES.md
  5. Se maturity > 0.9 → propõe emenda constitucional → DON APROVA
  6. Padrão recebe tag #principle (permanece como corolário vivo)
```

> **Regra**: A promoção a princípio **sempre** passa pelo Don (via F9.1). O F2.6 NUNCA promove diretamente — ele notifica e alimenta o F9.1 com evidências.

#### Qualquer Estado → MORTE

```
Gatilhos de morte:
  - pattern_evolution_score < 0.2 (moribundo — revisar ou arquivar)
  - Padrão não usado há 90 dias (F1.4 decay → reavaliação confirmada)
  - Contradito por outro padrão com maior massa gravitacional (F2.4)
  - Substituído por abordagem superior (nova versão MAJOR de outro padrão)
  - Revalidação falha (condições que sustentavam o padrão mudaram)

Ação:
  - Tag #stale aplicada → reavaliação (conteúdo ainda válido?)
  - Se confirmada obsolescência → tag #deprecated + DDNA de depreciação
  - Movido para pattern_history/ (NUNCA excluído)
  - Se tinha substituto → registro com superseded_by
```

### 2.4 Mapa de Compatibilidade com v1.0.0

O modelo anterior (v1.0.0) usava 6 estados. O mapeamento preserva compatibilidade com `patterns.md` existentes:

| v1.0.0 (antigo) | v2.0.0 (novo) | Tag |
|-----------------|---------------|-----|
| PROPOSED | NASCIMENTO | (sem tag) |
| DRAFT | CANDIDATO | `#candidate` |
| STABLE | MADURO | `#validated` |
| (novo) | PRINCÍPIO | `#principle` |
| DEPRECATED / SUPERSEDED / RETIRED | MORTE | `#stale` → `#deprecated` |
| REJECTED | MORTE (pattern_history) | `#deprecated` |

---

## 3. FÓRMULA DE MATURIDADE

### 3.1 Fórmula Principal — pattern_evolution_score

```
pattern_age = dias desde criação (desde o primeiro registro em patterns.md)
pattern_uses = vezes que o padrão foi aplicado com sucesso

pattern_evolution_score =
    min(pattern_uses / 10, 1.0) × 0.5 +          # Uso acumulado (dominante)
    min(pattern_age / 90, 1.0)   × 0.2 +          # Idade (tempo de sobrevivência)
    cross_agent_uses              × 0.2 +          # Validação independente
    review_score                  × 0.1            # Revisão recente (F1.4)
```

### 3.2 Componentes Detalhados

| Componente | Variável | Peso | Fórmula | Descrição |
|------------|----------|------|---------|-----------|
| **Uso Acumulado** | `pattern_uses` | **0.5** | `min(pattern_uses/10, 1.0)` | Cap em 10 usos (10+ satura). É o sinal mais forte de que o padrão funciona. |
| **Idade** | `pattern_age` | **0.2** | `min(pattern_age/90, 1.0)` | Cap em 90 dias. Padrão que sobreviveu 90 dias provou durabilidade. |
| **Validação Cross-Agent** | `cross_agent_uses` | **0.2** | `min(unique_agents_using/5, 1.0)` | Cap em 5 agentes. Evidência de independência — agentes diferentes confirmaram. |
| **Review Score** | `review_score` | **0.1** | Fornecido pelo F1.4 (ver §6) | `1 - min(days_since_review/180, 1.0)`. Revisão recente aumenta confiabilidade. |

### 3.3 Thresholds de Classificação

```
pattern_evolution_score

 0.0          0.2          0.4          0.7         1.0
 │────────────│────────────│────────────│───────────│
 │  MORIBUNDO │  CANDIDATO │  EM CRESCIMENTO │ MADURO │
 │  #stale    │  #candidate│  (candidato→    │ #validated│
 │            │            │   validated)    │ → #principle
```

| Score | Classificação | Ação |
|-------|---------------|------|
| **< 0.2** | 🔴 Moribundo | Revisar ou arquivar (→ MORTE). Tag `#stale`. |
| **0.2 — 0.4** | 🟡 Candidato | Manter `#candidate`. Incentivar uso para acumular evidência. |
| **0.4 — 0.7** | 🟢 Em crescimento | Promover a `#validated` se 3+ usos bem-sucedidos. |
| **> 0.7** | 🔵 Maduro | `#validated` pleno. Se `pattern_uses ≥ 10` → notificar F9.1 (promoção a princípio). |

### 3.4 Exemplo de Cálculo (com dados do ecossistema)

```
Padrão: "cross-agent-parallel-audit" (PATTERN-001)
  pattern_uses = 4 (L18, L19, L20, L21)
  pattern_age = 2 dias (nascido 2026-07-29)
  unique_agents = 1 (kernel orquestra; auditors executam sob ele)
  days_since_review = 0

pattern_evolution_score =
    min(4/10, 1.0) × 0.5 = 0.200
  + min(2/90, 1.0) × 0.2 = 0.004
  + min(1/5, 1.0)  × 0.2 = 0.040
  + 1.0            × 0.1 = 0.100
  = 0.344 → 🟢 EM CRESCIMENTO (candidato→validated)
```

**Análise**: O score é dominado por uso (4/10) e revisão recente. A idade ainda é baixa (2 dias) e cross-agent é limitado (1 orquestrador). Com mais 1 uso e mais agentes aplicando diretamente, cruza 0.4 → `#validated`. Este padrão ilustra a importância da **validação independente**: o kernel aplica, mas a validação real vem quando outros agentes aplicam o padrão sem o kernel.

### 3.5 Ajuste Fino (Configurável)

Os pesos e caps podem ser ajustados em `cosca.config.yaml` sem mudança de código:

```yaml
pattern_evolution:
  weights:
    usage: 0.5
    age: 0.2
    cross_agent: 0.2
    review: 0.1
  caps:
    uses: 10
    age_days: 90
    agents: 5
  thresholds:
    moribundo: 0.2
    validated: 0.4
    maduro: 0.7
```

---

## 4. PIPELINE SEMANAL (< 5S)

### 4.1 Pipeline

O pipeline roda **semanalmente** (domingo 00:00) ou sob demanda via CLI. Tempo alvo: **< 5s** para 54 agentes.

```
1. Varre patterns.md de todos os agentes
2. Para cada padrão, calcula evolution_score
3. Classifica: #candidate, #validated, #principle, #stale
4. Padrões #stale: reavalia (conteúdo ainda válido?)
5. Padrões promovidos: notifica F9.1 para possível emenda constitucional
6. Padrões mortos: move para pattern_history (não exclui)
```

### 4.2 Algoritmo

```
function run_pattern_evolution():
    // Passo 1: Varredura
    patterns = []
    for agent_path in glob("memory/agent/*/patterns.md"):
        patterns.extend(parse_patterns_file(agent_path))

    // Passo 2: Cálculo do evolution_score
    for pattern in patterns:
        pattern.pattern_uses = count_uses(pattern)           # grep nos learnings/tasks
        pattern.pattern_age = days_since(pattern.created_at)
        pattern.cross_agent_uses = unique_agents_using(pattern)
        pattern.review_score = wisdom_decay.freshness(pattern)   # F1.4
        pattern.evolution_score = calc_score(pattern)

    // Passo 3: Classificação
    for pattern in patterns:
        if pattern.evolution_score < 0.2:
            pattern.lifecycle = STALE          # #stale
        elif pattern.evolution_score < 0.4:
            pattern.lifecycle = CANDIDATE      # #candidate
        elif pattern.evolution_score < 0.7:
            if pattern.pattern_uses >= 3:
                pattern.lifecycle = VALIDATED  # #validated
            else:
                pattern.lifecycle = CANDIDATE
        else:
            pattern.lifecycle = VALIDATED      # #validated
            if pattern.pattern_uses >= 10:
                pattern.ready_for_principle = true

    // Passo 4: Reavaliação de #stale
    for pattern in patterns where lifecycle == STALE:
        report.append(revalidate_task(pattern))
        // "O conteúdo ainda é válido?" — task para o agente owner
        // Se sim → ressuscitar (reset review_score, manter)
        // Se não → avançar para depreciação

    // Passo 5: Notificação do F9.1
    for pattern in patterns where ready_for_principle:
        notify_experience_compiler(pattern)    // evento pattern.mature
        pattern.lifecycle = PRINCIPLE_CANDIDATE

    // Passo 6: Morte
    for pattern in patterns where pattern.deprecation_confirmed:
        move_to_pattern_history(pattern)       // NUNCA exclui
        pattern.lifecycle = DEPRECATED

    save_report(pattern_evolution_report)
    return report
```

### 4.3 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|--------------|----------------|
| Parse de 54 patterns.md | O(54 × P) | < 1.5s |
| Contagem de usos (grep em learnings.md indexados) | O(U) | < 1.5s |
| Cálculo do score (aritmética) | O(P) | < 0.1s |
| Classificação + reavaliação + notificações + movimentação | O(P) | < 1.5s |
| **Total** | | **< 5s** ✅ |

> **P = padrões por arquivo, U = usos a contar**. O segredo de performance: a contagem de usos usa o índice de tags (FTS5) em vez de ler learnings.md completos.

### 4.4 Trigger Semanal

```
Trigger: cron semanal (domingo 00:00 UTC)
  ├── Após F1.4 Wisdom Decay (que fornece freshness)
  ├── Antes do F9.1 Experience Compiler (que consome padrões maduros)
  └── Ordem de execução: F1.4 → F2.6 → F9.1
```

---

## 5. INTEGRAÇÃO COM F9.1 — EXPERIENCE COMPILER

### 5.1 Relação Bidirecional: F2.6 alimenta, F9.1 promove

```
┌─────────────────────┐   pattern.mature    ┌─────────────────────┐
│  Pattern Evolution  │ ───────────────────▶│  Experience Compiler│
│  (F2.6)             │  (evidências, tags, │  (F9.1)             │
│                     │   learnings origem) │                     │
│  - padrões maduros  │                     │  - agrupa learnings │
│  - evolution_score  │                     │  - calcula maturity │
│  - validação usos   │  ◀───────────────────│  - PRINCIPLES.md   │
│                     │  principle.approved │  - emenda → Don    │
└─────────────────────┘                     └─────────────────────┘
```

| Evento | Direção | Conteúdo | Ação |
|--------|---------|----------|------|
| `pattern.mature` | F2.6 → F9.1 | `{pattern_id, domain, learnings, evolution_score, uses}` | F9.1 inclui o padrão no próximo ciclo de compilação |
| `principle.approved` | F9.1 → F2.6 | `{principle_id, pattern_id}` | F2.6 marca o padrão como `#principle` (corolário vivo) |
| `amendment.rejected` | F9.1 → F2.6 | `{pattern_id, reason}` | F2.6 mantém o padrão como `#validated` (evidência insuficiente — acumular mais) |

### 5.2 Contrato de Dados

```yaml
# Evento pattern.mature emitido pelo F2.6
pattern.mature:
  pattern_id: "PATTERN-004"
  name: "same-package-access"
  domain: "testing, go, state-injection"
  lifecycle: "VALIDATED"
  evolution_score: 0.72
  pattern_uses: 11
  unique_agents: 3
  learnings: ["L22", "L24", "L31"]      # evidências de origem
  tags: ["#same-package", "#testing", "#go", "#level-4"]
  supersedes: null
```

### 5.3 Divisão de Responsabilidade

| O que | Quem | Detalhe |
|-------|------|---------|
| Detectar repetição (3ª ocorrência) | F0.6 Stages 7-8 + Insight Generator | Observação bruta |
| Registrar candidato com tags | F2.6 (Memory Chief executa) | `#candidate` em patterns.md |
| Validar com uso repetido | F2.6 (pipeline semanal) | evolution_score, contagem de usos |
| Decidir promoção a princípio | F9.1 (Experience Compiler) | maturity > 0.7 → PRINCIPLES.md |
| Aprovar emenda constitucional | **Don** (via F9.1) | maturity > 0.9 → proposta → Don |
| Depreciar padrão morto | F2.6 (após reavaliação) | pattern_history, nunca exclui |

> **Princípio de não-duplicação**: o F2.6 NÃO re-executa a fórmula de maturity do F9.1 (`pattern_maturity`). O F2.6 mede **saúde do padrão como ser vivo** (uso, idade, cross-agent, revisão). O F9.1 mede **maturidade de compilação** (learnings, freshness, cross-agent, impacto). São métricas complementares que respondem perguntas diferentes. A intersecção: ambos exigem evidência cross-agent.

---

## 6. INTEGRAÇÃO COM F1.4 — WISDOM DECAY

### 6.1 Padrões Também Envelhecem

A integração com o F1.4 aplica a curva de envelhecimento aos padrões. **Padrões não usados há 90 dias → decay → reavaliação**.

```
Regras de decay aplicadas aos padrões:

  1. Padrão sem uso documentado há 90 dias
     → F1.4 reduz o review_score (frescor) do padrão
     → review_score < 0.5 → revalidação obrigatória
     → Se revalidação confirmar obsolescência → MORTE

  2. Padrão com contradição ativa (F1.4 contradiction decay)
     → Ambos os padrões perdem frescor
     → Contradição não resolvida em 30 dias → reavaliação manual

  3. Padrão #principle ou com uso ≥ 50 vezes
     → decay rate reduzido (2% ao mês em vez de 10%) — conhecimento consolidado
```

### 6.2 Contrato de Integração

| Input do F1.4 | Uso no F2.6 |
|---------------|-------------|
| `freshness_score` | Componente `review_score` da fórmula (§3.2) |
| `days_since_last_use` | Gatilho de 90 dias sem uso → reavaliação |
| `contradiction_flag` | Gatilho de morte por contradição |
| `decay_type` | Decide se o padrão é candidato a MORTE ou apenas revalidação |
| `last_validated` | Atualiza `last_validated` do padrão (health metrics) |

### 6.3 Fluxo de Reavaliação (Padrão em Decay)

```
Padrão sem uso há 60 dias
  ├── F1.4: freshness cai (decay time-based)
  ├── F2.6 pipeline semanal: evolution_score cai abaixo de 0.4
  │     └── Tag: #stale-warning
  │
  ├── 90 dias sem uso
  │     ├── F1.4: freshness < 0.5 → revalidação obrigatória
  │     ├── F2.6: tag #stale + task de reavaliação para o owner
  │     │     ├── Reavaliação: "conteúdo ainda válido?"
  │     │     │     ├── SIM → ressuscitar (reset frescor, manter #validated)
  │     │     │     └── NÃO → confirmar depreciação
  │     │     │           ├── DDNA de depreciação criado
  │     │     │           ├── Movido para pattern_history/
  │     │     │           └── Tag: #deprecated
  │     │     └── F9.1 é notificado se o padrão era #principle candidate
  │
  └── Contradição detectada
        ├── F1.4: contradiction_flag
        └── F2.6: reavaliação manual imediata (Don notificado se P0)
```

### 6.4 Ciclo Virtuoso com F1.6

Quando o F2.6 deprecia um padrão obsoleto:
- Entropia cognitiva (F1.6) **reduz** — padrões contraditórios/stale saem do contexto ativo
- O F9.1 deixa de considerar learnings mortos como evidência
- O sistema fica mais confiável — decisões baseadas em padrões validados e frescos

---

## 7. VERSIONAMENTO MAJOR.MINOR

### 7.1 Formato

```
MAJOR.MINOR

MAJOR: Abordagem fundamentalmente alterada
  - Mudança de estratégia (sequential → parallel)
  - Mudança de domínio de aplicação
  - Versão anterior se torna SUPERSEDED

MINOR: Refinamento da mesma abordagem
  - Edge case handling, pitfalls, parâmetros, correções
  - Versão anterior substituída in-place
```

### 7.2 Regras de Incremento

| Mudança | Incremento | Versão Anterior |
|---------|------------|-----------------|
| Primeira formalização | 1.0.0 | — |
| Adiciona edge case / correção | +MINOR (1.0.0 → 1.1.0) | Substituída in-place |
| Muda estratégia fundamental | +MAJOR (1.1.0 → 2.0.0) | SUPERSEDED |
| Reescreve completamente | +MAJOR | SUPERSEDED |
| Deprecia | — (lifecycle change) | — |

### 7.3 Regras de Ouro

1. **Padrões nunca perdem versão** — mesmo morto, a versão permanece como referência.
2. **MINOR updates não criam ramos** — só a versão mais recente é "a verdade".
3. **MAJOR updates criam ramos** — a versão antiga é preservada como SUPERSEDED.
4. **Rollback é possível** — uma versão MAJOR que causa regressão pode ser revertida.
5. **Cada versão registra changelog**: o que mudou, por que, quem validou, trigger.

---

## 8. FORMATO DO REGISTRO DE EVOLUÇÃO

### 8.1 Formato YAML (fonte canônica)

```yaml
pattern:
  id: "PATTERN-004"
  name: "same-package-access"
  domain: "testing, go, state-injection"
  current_version: "1.0.0"
  lifecycle: VALIDATED          # BIRTH | CANDIDATE | VALIDATED | PRINCIPLE | DEPRECATED
  tag: "#validated"             # #candidate | #validated | #principle | #stale | #deprecated
  created_at: 2026-07-30
  first_extracted: 2026-07-30
  last_validated: 2026-07-30
  owner: "cosca-kernel"

  evolution_score:               # calculado pelo pipeline semanal
    pattern_uses: 3
    pattern_age_days: 2
    unique_agents: 2
    review_score: 1.00
    score: 0.51
    classification: "VALIDATED"

  description: >
    Padrão de testes em Go que acessa estado interno do package
    diretamente (same-package) para injetar estados impossíveis
    via API pública.

  evolution:
    - version: "1.0.0"
      date: 2026-07-30
      lifecycle: CANDIDATE
      trigger: "L22 — Runtime 100% Coverage"
      change: "Extração inicial — 3ª ocorrência de same-package state injection"
      validated_by: "cosca-kernel"
      success_rate: 1.00

  evidence:
    learnings: ["L22", "L24", "L31"]
    applications:
      - package: "internal/runtime"
        task: "state machine error paths"
        result: "success"
      - package: "internal/monitor"
        task: "goroutine sample injection"
        result: "success"
      - package: "internal/pool"
        task: "queue state injection"
        result: "success"

  health:
    success_rate: 1.00
    adoption: 3 uses / 2 agents
    staleness_risk: "LOW"

  integrations:
    wisdom_decay:
      freshness: 1.00
      last_validated: 2026-07-30
    experience_compiler:
      ready_for_principle: false   # needs pattern_uses ≥ 10

  related_patterns: ["PATTERN-001"]
  related_learnings: ["L22", "L24", "L31"]
```

### 8.2 Campos Obrigatórios

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `id` | string | Identificador único (PATTERN-NNN) |
| `name` | string | Nome canônico (slug) |
| `domain` | string | Domínio(s) de aplicação |
| `current_version` | string | MAJOR.MINOR atual |
| `lifecycle` | enum | Estado do ciclo de vida |
| `tag` | string | Tag de ciclo de vida |
| `created_at` | date | Data de nascimento do padrão |
| `evolution_score` | object | Score calculado pelo pipeline |
| `evolution` | list | Trilha de versões |

### 8.3 Sincronização com patterns.md

O `patterns.md` de cada agente é a **visão human-readable** derivada do YAML canônico. Contém: resumo, solução narrativa, pitfalls, trilha de evolução resumida e **tag de ciclo de vida visível no cabeçalho**:

```markdown
### Pattern 004: Same-Package Access (Testing)

> **Versão atual**: 1.0.0 | **Lifecycle**: VALIDATED | **ID**: PATTERN-004
> **Tag**: #validated | **Confidence**: 0.85 | **Health**: GOOD
> **Evolution Score**: 0.51 | **Uses**: 3 | **Age**: 2d
```

---

## 9. MÉTRICAS DE SAÚDE DO PADRÃO

### 9.1 Métricas Individuais

| Métrica | Fórmula | Interpretação |
|---------|---------|---------------|
| **Freshness** | `1 - days_since_validation/365` (F1.4) | ≥ 0.90 FRESH, ≥ 0.75 WARM, ≥ 0.50 STALE, < 0.50 COLD |
| **Success Rate** | `successful_applications / total_applications` | ≥ 0.90 altamente confiável, < 0.60 considerar depreciação |
| **Adoption** | `unique_agents_using / total_active_agents × (cross_project + 1)` | ≥ 0.50 amplamente adotado, < 0.10 marginal |
| **Velocity** | `total_versions / months_since_first_extraction` | 1-5/mês saudável, > 5 instável, 0 por 180d moribundo |
| **Evolution Score** | Fórmula §3 | ≥ 0.7 maduro, < 0.2 moribundo |

### 9.2 Health Score Composto

```
health_score = (freshness × 0.25) +
               (success_rate × 0.25) +
               (adoption × 0.15) +
               ((1 - staleness_risk_factor) × 0.25) +
               (normalized_velocity × 0.10)
```

---

## 10. PROTOCOLO DE DEPRECIAÇÃO (MORTE)

### 10.1 Quando um Padrão Morre

1. **Moribundo por score**: `pattern_evolution_score < 0.2` — revisar ou arquivar
2. **Obsolescência por decay**: não usado há 90 dias + revalidação confirma invalidez
3. **Contradição**: outro padrão/princípio com maior massa contradiz este
4. **Substituição**: abordagem superior validada (nova MAJOR de outro padrão)
5. **Falha crítica**: success_rate < 0.60 em 3+ aplicações consecutivas

### 10.2 Processo Formal de Morte

```
┌──────────────────────────────────────────────────────────────┐
│                 PATTERN DEATH PROTOCOL                        │
│                                                               │
│  1. DETECTAR gatilho (score < 0.2, decay 90d, contradição)   │
│       │                                                       │
│       ▼                                                       │
│  2. REAVALIAR "conteúdo ainda válido?"                        │
│     • Se SIM → ressuscitar (reset review, manter #validated)  │
│     • Se NÃO → prosseguir                                      │
│       │                                                       │
│       ▼                                                       │
│  3. DOCUMENTAR razão da morte                                 │
│     • Por que o padrão está obsoleto?                         │
│     • Existe substituto? Qual?                                │
│       │                                                       │
│       ▼                                                       │
│  4. CRIAR DDNA de depreciação                                 │
│     • Rastreabilidade completa de "por que morreu"            │
│       │                                                       │
│       ▼                                                       │
│  5. NOTIFICAR agentes afetados                                │
│     • Agentes que usaram o padrão recebem alerta + substituto │
│       │                                                       │
│       ▼                                                       │
│  6. MOVER para pattern_history/  ★ NUNCA EXCLUIR ★            │
│     • Arquivo preservado integralmente                        │
│     • Tag: #deprecated                                        │
│     • Se substituto existe → superseded_by populado           │
│                                                               │
│  7. NOTIFICAR F9.1 se o padrão alimentava princípio           │
│     • F9.1 remove evidência morta do pipeline de compilação   │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### 10.3 Formato do Registro de Depreciação

```yaml
deprecation:
  deprecated_date: 2026-10-01
  reason: "SUPERSEDED"        # SUPERSEDED | INVALIDATED | OBSOLETE | FAILED | CONTRADICTED
  reason_detail: "Substituído por PATTERN-009 (reflect-based state injection)"
  superseded_by:
    pattern_id: "PATTERN-009"
    version: "1.0.0"
  migration_guide: "Para migrar: use acessors de teste via reflect..."
  affected_agents: ["cosca-kernel", "cosca-testing"]
  audit_trail:
    - date: 2026-09-30
      action: "STALE — score 0.18, reavaliação iniciada"
      by: "cosca-memory-chief"
    - date: 2026-10-01
      action: "DEPRECATED — movido para pattern_history/"
      by: "cosca-memory-chief"
```

---

## 11. GOVERNANÇA E REGRAS NON-NEGOTIABLE

### 11.1 Propriedade

| Papel | Quem | Responsabilidade |
|-------|------|------------------|
| **Owner do Engine** | Architecture Chief | Design, fórmula, pipeline |
| **Executor do Pipeline** | Memory Chief | Varredura semanal, classificação, movimentação |
| **Validador de Transições** | Evolution Chief | CANDIDATO→MADURO, reavaliações de #stale |
| **Promotor a Princípio** | F9.1 (Experience Compiler) | maturity > 0.7 → PRINCIPLES.md |
| **Aprovador de Emenda** | **Don** | maturity > 0.9 → emenda constitucional |
| **Auditor** | Review Chief | Auditoria de health metrics |

### 11.2 Regras Non-Negotiable

1. **Padrão morto NÃO é excluído** — vai para `pattern_history/`, preservado integralmente.
2. **Promoção a princípio passa pelo Don** (via F9.1). O F2.6 nunca promove diretamente.
3. **Pipeline semanal, < 5s** — varredura não pode degradar a experiência do editor.
4. **Toda transição de lifecycle é registrada** — quem, quando, por que, com qual evidência.
5. **Tag de ciclo de vida é obrigatória** em todo padrão (`#candidate`, `#validated`, `#principle`, `#stale`).
6. **Contagem de usos é baseada em evidência** — usos documentados em learnings/tasks, não em suposição.

### 11.3 Direitos e Responsabilidades dos Agentes

| Ação | Quem Pode | Condições |
|------|-----------|-----------|
| Observar padrão (NASCIMENTO) | Qualquer agente | Stage 7 do metacognition pipeline |
| Registrar candidato (`#candidate`) | Agente que observou + Memory Chief | 3ª ocorrência da repetição |
| Promover a `#validated` | Memory Chief + Evolution Chief | 3+ usos bem-sucedidos, score ≥ 0.4 |
| Criar MINOR update | Qualquer agente que aplica | Refinamento, não alteração fundamental |
| Criar MAJOR update | Memory Chief + Evolution Chief | Mudança fundamental, requer validação |
| Notificar F9.1 (promoção) | Pipeline F2.6 (automático) | score > 0.7 e pattern_uses ≥ 10 |
| Depreciar (MORTE) | Memory Chief + Review | Protocolo formal §10, reavaliação confirmada |
| Ressuscitar padrão #stale | Memory Chief + Evolution Chief | Reavaliação: conteúdo ainda válido |

---

## 12. ARMAZENAMENTO

### 12.1 Estrutura de Diretórios

```
internal/embed/cosca/
├── engines/
│   └── pattern-evolution/
│       ├── SKILL.md                          ← Este arquivo (especificação)
│       └── patterns/
│           ├── active/                       ← Padrões #candidate, #validated, #principle
│           │   ├── PATTERN-001_cross-agent-audit.yaml
│           │   └── ...
│           └── pattern_history/              ← Padrões mortos (#stale → #deprecated)
│               ├── PATTERN-003_obsolete-approach.yaml
│               └── ...
│
├── memory/
│   └── agent/
│       └── cosca-{agent}/
│           ├── patterns.md                   ← Visão derivada + tag de ciclo de vida
│           └── learnings.md                  ← Nascimento dos padrões
│
└── knowledge/
    ├── PRINCIPLES.md                         ← Padrões promovidos pelo F9.1
    └── patterns/                             ← Registro do F9.1 (maturity ≥ 0.4)
```

### 12.2 Movimentação de Arquivos

| Transição | De | Para |
|-----------|----|------|
| NASCIMENTO → CANDIDATO | (learnings.md) | `patterns/active/` com tag `#candidate` |
| CANDIDATO → MADURO | `patterns/active/` | Permanece, tag → `#validated` |
| MADURO → PRINCÍPIO | `patterns/active/` | Permanece como corolário, tag → `#principle` |
| Qualquer → MORTE | `patterns/active/` | `patterns/pattern_history/`, tag → `#deprecated` |

### 12.3 Compatibilidade Retroativa

O pipeline lê **tanto** o YAML canônico (`patterns/*.yaml`) quanto os `patterns.md` derivados (para agentes que ainda não migraram para YAML). A varredura tolera formatos heterogêneos e normaliza em memória antes de calcular o score. A migração completa para YAML é gradual — cada padrão ativo ganha um `.yaml` quando transita de lifecycle.

---

## 13. CONSULTA E DESCOBERTA DE PADRÕES

### 13.1 Como Agentes Descobrem Padrões

```
1. Semantic search por padrões no domínio da task
   → Filtro: lifecycle IN [VALIDATED, PRINCIPLE] (não sugere candidatos frágeis
     nem padrões mortos)

2. Ordenar por evolution_score (maior primeiro)
   → Padrões mais usados e validados aparecem primeiro

3. Para cada padrão candidato:
   a. Verificar F1.4 freshness (review_score)
   b. Se freshness < 0.5 → alerta de revalidação
   c. Se tag #stale → alerta + sugestão do substituto
   d. Se tag #candidate → mostrar com aviso "[CANDIDATO — use com cautela]"
   e. Se tag #validated → mostrar com confidence atual
   f. Se tag #principle → mostrar como corolário (já compilado no F9.1)

4. Agente decide se aplica (julgamento, não obrigação)
   → Se aplica: registra uso (incrementa pattern_uses)
   → Se falha: registra falha (decrementa success_rate)
```

### 13.2 Queries Comuns

```yaml
# Padrões maduros prontos para promoção
query: "WHERE evolution_score > 0.7 AND pattern_uses >= 10 ORDER BY score DESC"

# Padrões moribundos (candidatos a morte)
query: "WHERE evolution_score < 0.2 ORDER BY score ASC"

# Padrões em risco de decay (60-90 dias sem uso)
query: "WHERE days_since_use BETWEEN 60 AND 90"

# Padrões sem revalidação há 90+ dias
query: "WHERE staleness_risk IN [HIGH, CRITICAL]"

# Trilha de evolução de um padrão
query: "pattern_id = 'PATTERN-004' → evolution[*] ORDER BY version"

# Padrões que alimentam princípios (corolários)
query: "WHERE tag = '#principle'"
```

---

## 14. EXEMPLO REAL — SAME-PACKAGE-ACCESS (L22)

### 14.1 Nascimento (NASCIMENTO → CANDIDATO)

**Origem**: O learning **L22** do cosca-kernel (2026-07-30, Runtime 100% + Compute Fabric, Level 4) registra:

> **(3) Same-package access é o superpoder do testing em Go.** Acessar `r.state.CurrentState`, `m.goroutineSamples`, `pool.queue` diretamente do teste (mesmo package) permitiu injetar estados que seriam impossíveis via API pública. Essencial para cobrir caminhos de erro em máquinas de estado. Sem isso, os 78.3%→100% do runtime não seriam possíveis.

**Primeira ocorrência**: L22 usa same-package access em 3 alvos do runtime:
- `internal/runtime` — `r.state.CurrentState` (state machine error paths)
- `internal/monitor` — `m.goroutineSamples` (goroutine sampler)
- `internal/pool` — `pool.queue` (work queue state injection)

Isso configura a **3ª ocorrência da observação de repetição** (3 packages diferentes, mesma técnica, mesmo learning). O padrão nasce e é formalizado como **CANDIDATO** em `patterns.md` com tag `#candidate`:

```yaml
pattern:
  id: "PATTERN-004"
  name: "same-package-access"
  lifecycle: CANDIDATE
  tag: "#candidate"
  created_at: 2026-07-30
  pattern_uses: 3
  unique_agents: 1
```

### 14.2 Maturidade (CANDIDATO → MADURO)

O padrão é usado em 3 packages dentro do runtime. Quando agentes de testing independentes (cosca-specialist-testing-unit, cosca-specialist-testing-integration) aplicam a mesma técnica em outros packages (ex: `internal/openaicompat`, `internal/embeddings` — cobertura 0%→88.3% e 0%→99.6% na mesma Wave 3), o padrão acumula:

```
pattern_uses = 5 (3 do L22 + 2 de agentes de testing)
pattern_age = 1 dia
unique_agents = 3 (kernel + 2 agentes de testing)
review_score = 1.00

pattern_evolution_score =
    min(5/10, 1.0) × 0.5 = 0.250
  + min(1/90, 1.0) × 0.2 = 0.002
  + min(3/5, 1.0)  × 0.2 = 0.120
  + 1.0            × 0.1 = 0.100
  = 0.472 → 🟢 VALIDATED (3+ usos, 3 agentes, score > 0.4)
```

**Promoção**: tag atualizada para `#validated`. Confidence 0.85. Sugerido para todos os agentes de testing.

### 14.3 Promoção a Princípio (MADURO → PRINCÍPIO — projeção)

Quando o padrão atingir 10+ usos em múltiplos packages (o template de teste "AAA + same-package access + fixtures" consolidado na Wave 3 indica que isso é questão de tempo):

```
pattern_uses = 12
pattern_age = 30 dias
unique_agents = 5
review_score = 1.00

pattern_evolution_score =
    min(12/10, 1.0) × 0.5 = 0.500
  + min(30/90, 1.0) × 0.2 = 0.067
  + min(5/5, 1.0)   × 0.2 = 0.200
  + 1.0             × 0.1 = 0.100
  = 0.867 → 🔵 MADURO → notifica F9.1
```

O F9.1 agrupa os learnings de origem (L22 + learnings dos agentes de testing), calcula `pattern_maturity`, e se > 0.7 registra em PRINCIPLES.md:

> **Princípio**: "Testes em Go devem acessar estado interno via same-package quando a injeção via API pública for impossível, preservando o teste na package original."

Se maturity > 0.9 → proposta de emenda constitucional → **Don decide**.

### 14.4 Morte (projeção — cenário de exemplo)

Se o Go introduzisse uma ferramenta de state-injection cross-package nativamente (ou se a arquitetura migrasse para packages separados de teste `_test`), o padrão seria contradito por uma abordagem superior:

```
Gatilho: F1.4 detecta contradição com novo padrão / decay por desuso
Reavaliação: "conteúdo ainda válido?" → NÃO (nova abordagem superior)
Morte: DDNA de depreciação criado
       Padrão movido para pattern_history/same-package-access.yaml
       Tag: #deprecated
       superseded_by: PATTERN-009 (novo padrão)
```

**O padrão não é excluído.** O YAML integral com trilha de evolução permanece acessível para consultas históricas — "o que sabíamos e quando".

### 14.5 Lição do Exemplo

O exemplo valida o design completo: um padrão nasceu num único learning (L22), foi formalizado como candidato pela repetição observada (3 packages), foi validado por agentes independentes (promoção), e está a caminho da promoção a princípio — ou da morte, se o contexto mudar. Cada transição é registrada, medida e preservada.

---

## 15. MÉTRICAS DO SISTEMA E ALERTAS

### 15.1 Métricas do Sistema de Padrões

| Métrica | Definição | Alvo |
|---------|-----------|------|
| **Padrões Ativos** | #candidate + #validated + #principle | Crescimento: +1-2/mês |
| **Taxa de Maturidade** | #validated / total ativos | > 40% |
| **Taxa de Promoção** | Padrões promovidos a princípio / total | > 5%/ciclo |
| **Taxa de Mortalidade** | Padrões mortos no ciclo / total | < 15% (excesso = conhecimento ruim) |
| **Score Médio** | Média do evolution_score dos ativos | Crescente |
| **Padrões Moribundos** | #stale / total | < 10% |
| **Cobertura de Domínio** | Domínios com ≥ 1 padrão validado / total | > 50% |

### 15.2 Alertas

| Condição | Severidade | Ação |
|----------|-----------|------|
| Padrão #stale por 2 ciclos consecutivos | 🔴 CRÍTICO | Depreciação automática (após reavaliação) |
| 3+ padrões no mesmo domínio sem consolidação | 🟠 ALTO | Análise de sobreposição → merge |
| 0 padrões novos em 30 dias | 🟠 ALTO | Stage 7 do pipeline pode estar quebrado |
| Padrão com success_rate < 0.60 | 🟠 ALTO | Revisão MAJOR ou depreciação |
| Padrão #validated com score < 0.4 | 🟡 MÉDIO | Investigar — regressão de uso |
| Padrão sem owner ativo | 🟡 MÉDIO | Reassignar para Memory Chief |

---

## 16. CLI E AUTOMAÇÃO

```bash
# Executar pipeline semanal completo
cosca pattern-evolution run

# Executar em modo dry-run (não modifica arquivos)
cosca pattern-evolution run --dry-run

# Ver relatório da última varredura
cosca pattern-evolution report

# Listar padrões por lifecycle
cosca pattern-evolution list --lifecycle validated
cosca pattern-evolution list --lifecycle stale

# Calcular score de um padrão específico
cosca pattern-evolution score PATTERN-004

# Registrar uso de um padrão (após task bem-sucedida)
cosca pattern-evolution use PATTERN-004 --agent cosca-testing --task T42

# Registrar falha de um padrão
cosca pattern-evolution fail PATTERN-004 --agent cosca-testing --task T42

# Depreciar manualmente (após reavaliação)
cosca pattern-evolution deprecate PATTERN-004 --reason SUPERSEDED --superseded-by PATTERN-009

# Ver árvore de evolução
cosca pattern-evolution tree PATTERN-001
```

---

## 17. REFERÊNCIAS

- [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §5 C6 — Definição conceitual do Pattern Evolution
- [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) F2.6 — Tarefa de implementação
- [Experience Compiler (F9.1)](../experience-compiler/SKILL.md) — Promoção de padrões a princípios e emendas
- [WISDOM_DECAY.md (F1.4)](../wisdom-decay/WISDOM_DECAY.md) — Freshness, decay, revalidação por desuso
- [Cognitive Entropy (F1.6)](../tools/SKILL.md) — Ciclo virtuoso de redução de entropia
- [Cognitive Gravity (F2.4)](../cognitive-gravity/SKILL.md) — Massa gravitacional e contradições
- [Insight Generator (F3.1)](../insight-generator/SKILL.md) — Detecção de padrões: 3+ similar observations
- [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) — Stages 7-8 (nascimento dos padrões)
- [CONSTITUTION.md](../../CONSTITUTION.md) — Princípios constitucionais (P1, P2, P5)
- [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — Formato de learning que alimenta padrões

---

## Changelog

> - **v2.0.0** (2026-07-30): Redesign completo do Pattern Evolution Engine pelo Architecture Chief. Novo ciclo de vida com 5 estados (NASCIMENTO → CANDIDATO → MADURO → PRINCÍPIO → MORTE) com tags `#candidate/#validated/#principle/#stale`. Nova fórmula `pattern_evolution_score` (uso 0.5, idade 0.2, cross-agent 0.2, review 0.1) com thresholds (0.2 moribundo / 0.4 validated / 0.7 maduro). Pipeline semanal de 6 passos em < 5s. Integração explícita com F9.1 (promoção a princípio via Don) e F1.4 (decay 90 dias → reavaliação). Protocolo de morte com preservação em `pattern_history/` (nunca exclui). Exemplo real do padrão same-package-access (L22) cobrindo todo o ciclo de vida. Compatibilidade retroativa com o modelo de 6 estados da v1.0.0.
> - **v1.0.0** (2026-07-30): Especificação inicial (Memory Chief). Ciclo de vida de 6 estados, versionamento MAJOR.MINOR, evolution record format, health metrics, protocolo de depreciação com sunset date.
