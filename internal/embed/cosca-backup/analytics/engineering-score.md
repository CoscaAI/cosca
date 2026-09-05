# ENGINEERING SCORE POR COMMIT (F7.3)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Analytics Chief | **Criado**: 2026-07-30
> **DNA Version**: 1.0.0 | **Engine**: `cosca-analytics`
>
> **Autoridade constitucional**: [CONSTITUTION.md](../CONSTITUTION.md) — Art. P2 (Código executado é a verdade absoluta)
> **Integrações**: [TRUST_REGISTRY.md](../memory/trust/TRUST_REGISTRY.md) (F7.2) | [Prediction Engine](../engines/prediction/SKILL.md) (F7.1) | [ENGINEERING_TIMELINE.md](../memory/timeline/ENGINEERING_TIMELINE.md) | [memorize-commit workflow](../workflows/memorize-commit.md) | [QUALITY_GATES.md](../QUALITY_GATES.md)
>
> Cada commit recebe um **score técnico composto** (0-100) avaliando 6 dimensões críticas. O score alimenta o Trust Registry dos agentes, melhora a precisão do Prediction Engine, e dá ao Don visibilidade instantânea da qualidade do que está sendo entregue.

---

## Índice

1. [Fórmula do Score](#1-fórmula-do-score)
2. [Arquitetura do Engine](#2-arquitetura-do-engine)
3. [Dimensões de Avaliação](#3-dimensões-de-avaliação)
4. [Pipeline Pós-Commit](#4-pipeline-pós-commit)
5. [Thresholds e Interpretação](#5-thresholds-e-interpretação)
6. [Integração com Prediction Engine (F7.1)](#6-integração-com-prediction-engine-f71)
7. [Integração com Trust Registry (F7.2)](#7-integração-com-trust-registry-f72)
8. [Breakdown Transparente](#8-breakdown-transparente)
9. [Exemplo com Commit Real](#9-exemplo-com-commit-real)
10. [Automação](#10-automação)
11. [Regras do Engine](#11-regras-do-engine)
12. [Relacionamentos](#12-relacionamentos)
13. [HISTÓRICO](#13-histórico)

---

## 1. Fórmula do Score

### Fórmula Canônica

```
engineering_score = (
    architecture × 0.20 +
    code_quality × 0.25 +
    testing × 0.20 +
    security × 0.15 +
    performance × 0.10 +
    documentation × 0.10
) × 100
```

Onde cada dimensão é um valor **0.0 – 1.0** (normalizado), e o score final é **0–100**.

### Pesos (Por que esses valores?)

| Dimensão | Peso | Justificativa |
|----------|:----:|---------------|
| **Architecture** | 0.20 | Decisões arquiteturais são as mais caras de reverter. Um commit que fere a arquitetura pode gerar dívida técnica por meses. |
| **Code Quality** | 0.25 | **Maior peso** — código é o artefato primário. Qualidade de código afeta diretamente manutenibilidade, legibilidade e taxa de bugs. |
| **Testing** | 0.20 | Sem testes não há confiança. Cobertura, qualidade dos testes e ausência de raças são críticos para sustentar evolução. |
| **Security** | 0.15 | Segurança não é negociável, mas a maioria dos commits não toca em superfícies de ataque — por isso o peso é moderado. |
| **Performance** | 0.10 | Relevante apenas quando o commit altera hot paths ou introduz alocações/concorrência. Peso menor reflete frequência de impacto. |
| **Documentation** | 0.10 | Documentação é importante mas geralmente assíncrona ao commit de código. Peso reflete que docs podem vir em commits separados. |

**Relação com QUALITY_GATES.md**: A fórmula do Engineering Score NOBLEIA (adapta) o `OVERALL` do Quality Gates, mas com pesos diferentes porque:
- Engineering Score avalia **cada commit individualmente** (não o release)
- Code Quality tem peso maior (0.25 vs 0.20) — commits são sobre código
- Security tem peso menor (0.15 vs 0.25) — a maioria dos commits não introduz superfícies de ataque
- Testing tem peso maior (0.20 vs 0.15) — cada commit deve ser testado

---

## 2. Arquitetura do Engine

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ENGINEERING SCORE ENGINE                          │
│                                                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │
│  │  Collector   │  │  Analyzer   │  │  Scorer     │                  │
│  │  (Step 1-2)  │─►│  (Step 3-8) │─►│  (Step 9)   │                  │
│  └─────────────┘  └─────────────┘  └──────┬──────┘                  │
│         │                                  │                         │
│         ▼                                  ▼                         │
│  ┌─────────────┐                  ┌─────────────────┐               │
│  │  git diff    │                  │  Score Calculator│              │
│  │  git log     │                  │  6 dimensões     │              │
│  │  go test     │                  │  pesos aplicados │              │
│  └─────────────┘                  └────────┬────────┘               │
│                                            │                         │
│                                            ▼                         │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      Outputs                                  │   │
│  │  ┌─────────────────┐  ┌────────────────┐  ┌───────────────┐  │   │
│  │  │ Timeline Entry  │  │ Trust Registry  │  │ Alert (opcional)│  │
│  │  │ (ENGINEERING_   │  │ (confidence_    │  │ (se score<50)  │  │
│  │  │  TIMELINE.md)   │  │  delta update)  │  │               │  │
│  │  └─────────────────┘  └────────────────┘  └───────────────┘  │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### Três Subsistemas

| Subsistema | Função | Responsável |
|------------|--------|-------------|
| **Collector** | Coleta dados brutos do commit (diff, mensagem, arquivos, cobertura, etc.) | memorize-commit workflow (Steps 1-3) |
| **Analyzer** | Avalia cada dimensão com base nos dados coletados | Engineering Score Engine |
| **Scorer** | Calcula o score composto, aplica thresholds, gera outputs | Engineering Score Engine |

---

## 3. Dimensões de Avaliação

Cada dimensão é avaliada como um score **0.0 – 1.0** com critérios objetivos e verificáveis.

> **Nota**: Scores são calculados com base em dados extraídos automaticamente + análise heurística. Scores não precisam de julgamento humano para commits de rotina.

---

### 3.1 Architecture (α) — Peso 0.20

**O que mede**: Qualidade arquitetural da mudança — novas dependências são justificadas? O design respeita os limites modulares? A coesão do pacote melhora ou piora?

| Subcritério | Peso | Score = 1.0 (Excelente) | Score = 0.5 (Médio) | Score = 0.0 (Ruim) |
|------------|:----:|-------------------------|--------------------|--------------------|
| **Module boundaries** | 0.30 | Nenhuma violação cross-boundary; padrão do pacote mantido | Violação menor justificada | Violação grave sem justificativa |
| **Dependency direction** | 0.25 | Dependências fluem para abstrações estáveis; sem círculos | Novo ciclo menor criado mas documentado | Ciclo ou inversão de dependência introduzida |
| **Cohesion** | 0.25 | Código adicionado ao módulo correto; coesão do pacote mantida | Código em módulo próximo mas não ideal | Código em módulo errado; coesão reduzida |
| **DDNA creation** | 0.10 | Decisão arquitetural documentada em DDNA | Decisão mencionada no commit msg | Decisão não documentada |
| **Pattern consistency** | 0.10 | Mesmo padrão usado para problemas equivalentes | Padrão diferente mas razoável | Padrão inconsistente com codebase |

**Fórmula**:
```
architecture = (module_boundaries × 0.30 + dependency_direction × 0.25 +
                cohesion × 0.25 + ddna_creation × 0.10 +
                pattern_consistency × 0.10)
```

**Fontes de dados**: `git diff --name-only` (cross-boundary check), `git log` (DDNA ref), análise de estrutura de diretórios.

---

### 3.2 Code Quality (ϛ) — Peso 0.25

**O que mede**: Qualidade do código produzido — tamanho de arquivos/funções, complexidade, nomes, comentários, ausência de anti-patterns.

| Subcritério | Peso | Score = 1.0 | Score = 0.5 | Score = 0.0 |
|------------|:----:|-------------|-------------|-------------|
| **File length compliance** | 0.15 | Todos arquivos < 300 lines | 1 arquivo > 300 lines | Múltiplos arquivos > 300 lines |
| **Function length** | 0.15 | Todas funções < 50 lines | 1 função > 50 lines | Múltiplas funções > 50 lines |
| **Cyclomatic complexity** | 0.15 | Todas funções < 10 | 1 função 10-15 | Múltiplas funções > 15 |
| **Naming clarity** | 0.15 | Nomes descritivos, convenção do projeto | 1-2 nomes confusos | Nomes inconsistentes ou abreviaturas |
| **Dead/commented code** | 0.15 | 0 instâncias | 1 instância justificada | Múltiplas instâncias |
| **Magic numbers** | 0.10 | 0 magic numbers | 1-2 magic numbers (constantes locais) | Múltiplas constantes não nomeadas |
| **Code churn** | 0.15 | LOC líquido < 300 | LOC líquido 300-1000 | LOC líquido > 1000 (commits grandes demais) |

**Fórmula**:
```
code_quality = (file_length × 0.15 + function_length × 0.15 +
                cyclomatic × 0.15 + naming × 0.15 +
                dead_code × 0.15 + magic_numbers × 0.10 +
                code_churn × 0.15)
```

**Fontes de dados**: `git diff --stat`, `gocyclo` (se disponível), análise de diff (comentários, nomes).

---

### 3.3 Testing (τ) — Peso 0.20

**O que mede**: Cobertura de testes da mudança — o commit inclui testes? A cobertura mudou? Testes de race foram executados?

| Subcritério | Peso | Score = 1.0 | Score = 0.5 | Score = 0.0 |
|------------|:----:|-------------|-------------|-------------|
| **Tests included** | 0.25 | Testes acompanham o commit | Testes em commit separado mas próximo | Sem testes associados |
| **Coverage delta** | 0.20 | Cobertura aumentou ≥ 1% | Cobertura estável (±0.5%) | Cobertura caiu |
| **Race detection** | 0.15 | `-race` passou sem warnings | Race warnings menores não críticos | Race condition detectada |
| **Edge cases covered** | 0.15 | ≥ 2 edge cases testados | 1 edge case testado | Apenas happy path |
| **Error paths** | 0.15 | ≥ 1 error path testado | Error path implícito (não testado) | Nenhum error path |
| **Test independence** | 0.10 | Testes independentes, sem shared state | Testes compartilham setup mas são isolados | Testes acoplados (um falha todos falham) |

**Fórmula**:
```
testing = (tests_included × 0.25 + coverage_delta × 0.20 +
           race_detection × 0.15 + edge_cases × 0.15 +
           error_paths × 0.15 + test_independence × 0.10)
```

**Fontes de dados**: `git diff --name-only` (arquivos `_test.go`), `go test -race -count=1`, `go test -cover`.

---

### 3.4 Security (σ) — Peso 0.15

**O que mede**: Impacto de segurança da mudança — novas superfícies de ataque, secrets expostos, input validation.

| Subcritério | Peso | Score = 1.0 | Score = 0.5 | Score = 0.0 |
|------------|:----:|-------------|-------------|-------------|
| **New attack surface** | 0.25 | Nenhuma nova superfície | Superfície menor (ex: novo endpoint) justificada | Superfície nova sem justificativa ou proteção |
| **Secrets exposure** | 0.25 | 0 secrets expostos | Secret em variável de ambiente (aceitável) | Hardcoded secret no diff |
| **Input validation** | 0.20 | Toda entrada validada | Validação parcial | Sem validação em inputs externos |
| **OWASP compliance** | 0.15 | Sem violações OWASP | Violação menor documentada | Violação grave (SQLi, XSS, etc.) |
| **Dependency change** | 0.15 | Nenhuma nova dependência | Nova dependência com CVE baixo | Nova dependência com CVE crítico |

**Fórmula**:
```
security = (attack_surface × 0.25 + secrets × 0.25 +
            input_validation × 0.20 + owasp × 0.15 +
            dependency_change × 0.15)
```

**Fontes de dados**: `git diff` (busca de secrets, tokens), análise de novos endpoints, `go.mod` changes.

---

### 3.5 Performance (π) — Peso 0.10

**O que mede**: Impacto em performance — alocações desnecessárias, locks, padrões de concorrência.

| Subcritério | Peso | Score = 1.0 | Score = 0.5 | Score = 0.0 |
|------------|:----:|-------------|-------------|-------------|
| **Allocation awareness** | 0.25 | Sem novas alocações em hot paths | Alocações menores em cold paths | Grandes alocações em hot paths |
| **Concurrency correctness** | 0.25 | Padrões corretos de sincronização | Lock contention menor | Data races ou deadlocks introduzidos |
| **N+1 prevention** | 0.20 | Sem queries N+1 | 1 N+1 potencial (documentado) | N+1 em produção |
| **Sync vs async** | 0.15 | I/O-bound operations async | Algumas ops síncronas em async | Blocking call em async context |
| **Memory pressure** | 0.15 | Sem grandes alocações | Alocação moderada justificada | Grande alocação em hot path |

**Fórmula**:
```
performance = (allocation × 0.25 + concurrency × 0.25 +
               n_plus_one × 0.20 + sync_async × 0.15 +
               memory_pressure × 0.15)
```

**Fontes de dados**: `git diff` (análise de padrões), `go vet`, experiência do avaliador.

---

### 3.6 Documentation (δ) — Peso 0.10

**O que mede**: A documentação adequada acompanha a mudança — README, DDNA, docstrings, changelog.

| Subcritério | Peso | Score = 1.0 | Score = 0.5 | Score = 0.0 |
|------------|:----:|-------------|-------------|-------------|
| **README updated** | 0.20 | README atualizado se necessário | README menciona mudança parcialmente | README desatualizado |
| **DDNA created** | 0.20 | DDNA criado (se decisão arquitetural) | Decisão documentada no commit msg | Decisão não documentada |
| **Changelog entry** | 0.15 | Changelog atualizado | Mudança mencionada em PR/issue | Sem entrada |
| **Code comments (why)** | 0.20 | Comentários explicam "por que" | Comentários descrevem "o que" | Sem comentários onde necessários |
| **Docstrings/Godoc** | 0.15 | Funções públicas documentadas | Funções públicas parcialmente documentadas | Funções públicas sem doc |
| **TODOs tracked** | 0.10 | 0 novos TODOs sem issue | TODO com issue referenciada | TODO sem referência |

**Fórmula**:
```
documentation = (readme × 0.20 + ddna × 0.20 + changelog × 0.15 +
                 code_comments × 0.20 + docstrings × 0.15 +
                 todos_tracked × 0.10)
```

**Fontes de dados**: `git log`, `git diff --name-only` (README, CHANGELOG, DDNA paths), análise de arquivos tocados.

---

## 4. Pipeline Pós-Commit

```
FLUXO COMPLETO (pós-commit, < 1s):
────────────────────────────────────

  ┌──────────────────────────────────────────────────────────────────┐
  │                                                                  │
  │  1. memorize-commit workflow é disparado                         │
  │     ├── Coleta dados do commit (git diff --stat, git log)        │
  │     ├── Classifica tipo de mudança                               │
  │     └── Mede coverage delta (go test -cover)                     │
  │                                                                  │
  │  2. Engineering Score Engine recebe os dados coletados            │
  │     ├── Coleta dimensões:                                        │
  │     │   ├── Architecture: diff por diretório, DDNA ref           │
  │     │   ├── Code Quality: LOC, complexidade, naming              │
  │     │   ├── Testing: testes incluídos, race, coverage delta      │
  │     │   ├── Security: superfícies novas, secrets, input          │
  │     │   ├── Performance: alocações, locks, concorrência          │
  │     │   └── Documentation: README, DDNA, changelog              │
  │     │                                                           │
  │     ├── Calcula score de cada dimensão (0.0-1.0)                │
  │     └── Aplica fórmula composta → engineering_score (0-100)     │
  │                                                                  │
  │  3. Registra na ENGINEERING_TIMELINE.md                          │
  │     ├── Nova coluna "Score" adicionada na timeline               │
  │     └── Breakdown salvo em impact-report do commit               │
  │                                                                  │
  │  4. Alimenta Trust Registry (F7.2)                               │
  │     ├── Se score >= 70: confidence_delta positivo                │
  │     ├── Se score >= 90: confidence_delta = +0.05 bonus           │
  │     ├── Se score < 50: confidence_delta negativo                 │
  │     └── Se score < 30: alerta ao Don + confidence_delta = -0.10 │
  │                                                                  │
  │  5. Se score < 50: alerta ao Don                                 │
  │     ├── Mensagem: "⚠️ Engineering Score {score} — revisão        │
  │     │             obrigatória. Breakdown: [link]"                │
  │     └── Don pode: ignorar, pedir revisão, ou ajustar score       │
  │                                                                  │
  │  6. Prediction Engine (F7.1) atualiza P(success) do agente       │
  │     ├── Baseado em: score histórico × engineering_score          │
  │     └── Acurácia do Prediction Engine melhora com dados reais    │
  │                                                                  │
  └──────────────────────────────────────────────────────────────────┘
```

### Timing

| Fase | Tempo | Gatilho |
|------|:-----:|---------|
| **Coleta de dados** | < 200ms | memorize-commit workflow Step 1-3 |
| **Análise de dimensões** | < 400ms | Engineering Score Engine |
| **Cálculo do score** | < 10ms | Fórmula matemática |
| **Registro na timeline** | < 50ms | Append no ENGINEERING_TIMELINE.md |
| **Atualização Trust Registry** | < 50ms | Append no TRUST_REGISTRY.md |
| **Alerta (se necessário)** | < 100ms | Mensagem ao Don |
| **TOTAL** | **< 1s** | — |

---

## 5. Thresholds e Interpretação

### Tabela de Thresholds

| Score | Faixa | Classificação | Cor | Símbolo | Ação |
|:-----:|:-----:|:-------------:|:---:|:-------:|------|
| 90-100 | 🔥 | **Excelente** | 🟢 | A+ | Nenhuma. Commit exemplar. Agente ganha bonus no Trust Registry. |
| 70-89 | 🟢 | **Bom** | 🟢 | A/B | Nenhuma ação requerida. Commit dentro do padrão esperado. |
| 50-69 | 🟡 | **Médio** | 🟡 | C | **Revisar.** Recomenda-se revisão do código antes de prosseguir. Agenda como dívida técnica leve. |
| < 50 | 🔴 | **Ruim** | 🔴 | D/F | **Revisão obrigatória.** Bloqueante. Deve ser revisto pelo Don ou por cosca-review antes de qualquer trabalho dependente. Alertas enviados. |

### Interpretação por Faixa

#### 🔥 Excelente (90-100)
O commit é exemplar em todas as dimensões. Arquitetura respeitada, código limpo e bem nomeado, testes completos com edge cases, sem superfícies de ataque, performance consciente, documentação atualizada.
- **Impacto no Trust Registry**: `confidence_delta = +0.05`
- **Impacto no Prediction Engine**: Aumenta P(success) do agente para tarefas similares
- **Visibilidade**: 🏆 Registrado como "commit exemplar" na timeline

#### 🟢 Bom (70-89)
O commit atende aos padrões esperados. Pode ter pequenos desvios em dimensões específicas, mas nada crítico.
- **Impacto no Trust Registry**: `confidence_delta = +0.01` a `+0.03`
- **Impacto no Prediction Engine**: Confirma a confiabilidade do agente
- **Visibilidade**: Normal

#### 🟡 Médio (50-69)
O commit tem deficiências em uma ou mais dimensões. Recomenda-se revisão antes de avançar. Exemplos típicos:
- Commit grande demais (> 1000 LOC) que deveria ser dividido
- Poucos ou nenhum teste para funcionalidade crítica
- Documentação ausente para mudança significativa

- **Impacto no Trust Registry**: `confidence_delta = 0` (neutro) ou `-0.02` se recorrente
- **Impacto no Prediction Engine**: Penalidade leve P(success)
- **Visibilidade**: ⚠️ Tag "review recommended" na timeline

#### 🔴 Ruim (< 50)
O commit tem problemas graves que exigem revisão obrigatória. Pode conter:
- Violação arquitetural grave
- Secrets expostos no diff
- Cobertura caindo significativamente
- Dead code introduzido
- Nenhum teste para funcionalidade crítica

- **Impacto no Trust Registry**: `confidence_delta = -0.05` a `-0.10`
- **Impacto no Prediction Engine**: Penalidade significativa P(success)
- **Visibilidade**: 🚨 **ALERTA AO DON** — notificação imediata
- **Ação**: Commit não deve avançar sem revisão. Don pode override manual.

---

## 6. Integração com Prediction Engine (F7.1)

### Score Histórico do Agente

O Engineering Score alimenta o Prediction Engine (F7.1) como mais uma variável no cálculo de `P(success)`:

```
P(success) = base_success_rate × 0.50
           + domain_strength × 0.25
           + engineering_score_avg × 0.15     # ← NOVO: média histórica do engineering score
           + recency_bonus × 0.10
```

Onde:
- `engineering_score_avg` = média dos últimos 20 engineering scores do agente (normalizada 0.0-1.0)
- Commit com score alto → aumenta P(success) para tasks futuras
- Commit com score baixo → reduz P(success) para tasks similares

### Feedforward Loop

```
┌──────────────────┐         ┌────────────────────┐         ┌──────────────────┐
│  Commit feito    │────────►│  Engineering Score  │────────►│  Trust Registry   │
│  pelo agente X   │         │  = 85 (Bom)         │         │  confidence +0.02 │
└──────────────────┘         └────────────────────┘         └────────┬─────────┘
                                                                     │
                                                                     ▼
┌──────────────────┐         ┌────────────────────┐         ┌──────────────────┐
│  Próxima task    │◄────────│  Prediction Engine  │◄────────│  P(success) =     │
│  para agente X   │         │  recalcula P(success)│         │  92% (↑)          │
└──────────────────┘         └────────────────────┘         └──────────────────┘
```

### Correlação: Engineering Score → P(success)

| Avg Engineering Score | Fator multiplicador P(success) |
|:---------------------:|:-------------------------------:|
| 90-100 | × 1.10 (bônus de confiança) |
| 70-89 | × 1.00 (neutro) |
| 50-69 | × 0.90 (penalidade leve) |
| < 50 | × 0.75 (penalidade significativa) |

---

## 7. Integração com Trust Registry (F7.2)

### Mapeamento Engineering Score → Trust Registry

O score alimenta o campo `confidence_delta` do Trust Registry:

| Engineering Score | `confidence_delta` | Efeito no `reliability_score` |
|:-----------------:|:-------------------:|:----------------------------:|
| 90-100 | `+0.05` | Aumento significativo — commit exemplar |
| 80-89 | `+0.03` | Aumento moderado |
| 70-79 | `+0.01` | Aumento leve |
| 60-69 | `0.00` | Neutro — dentro do esperado |
| 50-59 | `-0.02` | Penalidade leve |
| 30-49 | `-0.05` | Penalidade moderada |
| < 30 | `-0.10` | Penalidade severa — alerta ao Don |

### Formato de Registro no Trust Registry

O Engineering Score é armazenado como campo adicional em cada entrada do Trust Registry:

```yaml
- agent: cosca-architecture
  task_type: specification
  task_id: spec-cognitive-economy-2026-07-30-001
  outcome: success
  confidence_before: 0.85
  confidence_after: 0.88
  confidence_delta: +0.03
  engineering_score: 92.4           # ← NOVO
  engineering_breakdown:            # ← NOVO
    architecture: 0.95
    code_quality: 0.90
    testing: 0.88
    security: 1.00
    performance: 0.95
    documentation: 0.85
  # ... demais campos existentes
```

---

## 8. Breakdown Transparente

### Formato de Output

O breakdown completo é gerado para cada commit e armazenado no Impact Report:

```yaml
engineering_score:
  total: 84.6
  grade: 🟢 Bom
  breakdown:
    architecture:    { score: 0.92, weight: 0.20, contribution: 18.4 }
    code_quality:    { score: 0.80, weight: 0.25, contribution: 20.0 }
    testing:         { score: 0.75, weight: 0.20, contribution: 15.0 }
    security:        { score: 0.95, weight: 0.15, contribution: 14.3 }
    performance:     { score: 0.90, weight: 0.10, contribution: 9.0 }
    documentation:   { score: 0.80, weight: 0.10, contribution: 8.0 }
  formula: "(0.92×0.20) + (0.80×0.25) + (0.75×0.20) + (0.95×0.15) + (0.90×0.10) + (0.80×0.10) = 0.846"
  thresholds:
    is_excellent: false    # >= 90
    is_good: true          # >= 70
    is_medium: false       # >= 50
    is_poor: false         # < 50
  alert_don: false         # true se score < 50
```

### Visualização na Timeline

A `ENGINEERING_TIMELINE.md` ganha uma nova coluna **Score**:

```
| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Score | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-----:|--------|
| 17:23   | 🏗️  | Integrate Compute Fabric | +8/-0 | 1 | $0.001 | 92.4🔥 | 90e864f |
| 17:30   | 🧪  | Compute Fabric Fase 2.5 | +82/-0 | 3 | $0.002 | 85.2🟢 | af51439 |
```

### Comando de Consulta

```bash
# Ver breakdown de um commit específico
cosca analytics score <commit-hash>

# Output:
# Engineering Score: 84.6 🟢 Bom
# Breakdown:
#   Architecture:    0.92 × 0.20 = 18.4
#   Code Quality:    0.80 × 0.25 = 20.0
#   Testing:         0.75 × 0.20 = 15.0
#   Security:        0.95 × 0.15 = 14.3
#   Performance:     0.90 × 0.10 = 9.0
#   Documentation:   0.80 × 0.10 = 8.0
# ──────────────────────────────────
#   TOTAL:                      84.6
```

---

## 9. Exemplo com Commit Real

### Commit: `792c000` — Expurgo de Infraestrutura Fictícia

```
Commit:   792c000
Mensagem: docs: expurgo de infraestrutura fictícia — PostgreSQL/Redis/pgvector
          removidos, v3.0.1 → v1.4.0-dev (Don's order)
Autor:    Cosca Kernel
Data:     2026-07-30 02:23:14 -0300
Arquivos: 12
Inserções: +114
Deleções:  -315
Tipo:     docs 📝
```

### Dimensão: Architecture (α)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| Module boundaries | 1.0 | Arquivos expurgados dentro de seus módulos corretos (`internal/embed/cosca/runtime/`, `internal/embed/cosca/runtime/`) |
| Dependency direction | 1.0 | Remoção de dependências fictícias não afeta direção de dependências reais |
| Cohesion | 0.9 | O expurgo removeu referências a PostgreSQL/Redis/pgvector de docs de runtime — coesão do pacote melhorou |
| DDNA creation | 0.0 | Decisão de expurgo documentada apenas no commit message, sem DDNA |
| Pattern consistency | 1.0 | Consistente com o padrão de "código vence documentação" (CONSTITUTION Art. P2) |

**Score Architecture**: `(1.0×0.30 + 1.0×0.25 + 0.9×0.25 + 0.0×0.10 + 1.0×0.10)` = **0.925**

### Dimensão: Code Quality (ϛ)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| File length | 1.0 | Maior arquivo: KNOWLEDGE.md com -185 lines (redução) |
| Function length | 1.0 | Sem funções (docs-only) |
| Cyclomatic complexity | 1.0 | Sem código executável alterado |
| Naming clarity | 1.0 | Nomes consistentes |
| Dead/commented code | 1.0 | Remoção de conteúdo fictício é exatamente o oposto de dead code |
| Magic numbers | 1.0 | N/A (documentação) |
| Code churn | 1.0 | LOC líquido -201 (redução líquida) — commit saudável |

**Score Code Quality**: `(1.0×0.15 + 1.0×0.15 + 1.0×0.15 + 1.0×0.15 + 1.0×0.15 + 1.0×0.10 + 1.0×0.15)` = **1.000**

### Dimensão: Testing (τ)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| Tests included | 0.0 | Sem testes (commit de documentação) |
| Coverage delta | 0.5 | Cobertura não foi medida (docs only) — neutro |
| Race detection | 0.5 | N/A — sem código executável |
| Edge cases covered | 0.0 | N/A |
| Error paths | 0.0 | N/A |
| Test independence | 0.5 | N/A |

**Score Testing**: `(0.0×0.25 + 0.5×0.20 + 0.5×0.15 + 0.0×0.15 + 0.0×0.15 + 0.5×0.10)` = **0.225**

### Dimensão: Security (σ)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| New attack surface | 1.0 | Nenhuma — remoção de conteúdo fictício |
| Secrets exposure | 1.0 | 0 secrets |
| Input validation | 1.0 | N/A |
| OWASP compliance | 1.0 | N/A |
| Dependency change | 1.0 | Nenhuma dependência alterada |

**Score Security**: `(1.0×0.25 + 1.0×0.25 + 1.0×0.20 + 1.0×0.15 + 1.0×0.15)` = **1.000**

### Dimensão: Performance (π)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| Allocation awareness | 1.0 | Sem código executável |
| Concurrency correctness | 1.0 | Sem código executável |
| N+1 prevention | 1.0 | Sem queries |
| Sync vs async | 1.0 | Sem código executável |
| Memory pressure | 1.0 | N/A |

**Score Performance**: `(1.0×0.25 + 1.0×0.25 + 1.0×0.20 + 1.0×0.15 + 1.0×0.15)` = **1.000**

### Dimensão: Documentation (δ)

| Subcritério | Score | Justificativa |
|-------------|:-----:|---------------|
| README updated | 0.5 | README não foi alterado diretamente, mas docs de runtime sim |
| DDNA created | 0.0 | Nenhum DDNA criado para esta decisão arquitetural |
| Changelog entry | 1.0 | CHANGELOG.md foi atualizado (implícito — Don's order) |
| Code comments (why) | 1.0 | Commit message explica "por que" (Don's order — infra fictícia) |
| Docstrings/Godoc | 1.0 | N/A (docs de runtime) |
| TODOs tracked | 1.0 | 0 novos TODOs |

**Score Documentation**: `(0.5×0.20 + 0.0×0.20 + 1.0×0.15 + 1.0×0.20 + 1.0×0.15 + 1.0×0.10)` = **0.650**

### Cálculo Final

```
engineering_score = (
    architecture × 0.20 +      # 0.925 × 0.20 = 0.185
    code_quality × 0.25 +      # 1.000 × 0.25 = 0.250
    testing × 0.20 +            # 0.225 × 0.20 = 0.045
    security × 0.15 +           # 1.000 × 0.15 = 0.150
    performance × 0.10 +        # 1.000 × 0.10 = 0.100
    documentation × 0.10        # 0.650 × 0.10 = 0.065
) × 100
```

```
engineering_score = (0.185 + 0.250 + 0.045 + 0.150 + 0.100 + 0.065) × 100

engineering_score = 0.795 × 100 = 79.5
```

### Resultado

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   Commit: 792c000 — Expurgo de infraestrutura fictícia      │
│                                                             │
│   ENGINEERING SCORE:  79.5  🟢 BOM                           │
│                                                             │
│   Breakdown:                                                │
│   ─────────────────────────────────────────────────         │
│   Architecture:    0.925 × 0.20  =  18.5                    │
│   Code Quality:    1.000 × 0.25  =  25.0  ✦                │
│   Testing:         0.225 × 0.20  =   4.5  ⚠                 │
│   Security:        1.000 × 0.15  =  15.0                    │
│   Performance:     1.000 × 0.10  =  10.0                    │
│   Documentation:   0.650 × 0.10  =   6.5                    │
│   ─────────────────────────────────────────────────         │
│                         TOTAL =  79.5                        │
│                                                             │
│   Thresholds:                                               │
│   ├── 90-100 Excelente?  ❌                                 │
│   ├── 70-89  Bom?         ✅ (está aqui)                     │
│   ├── 50-69  Médio?      ❌                                 │
│   └── < 50   Ruim?       ❌                                 │
│                                                             │
│   Alerta ao Don? ❌ (score >= 50)                           │
│                                                             │
│   Trust Registry Impact: confidence_delta = +0.01           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Análise do Exemplo

O score **79.5 (Bom)** reflete adequadamente a natureza do commit:

- **Pontos fortes**: Arquitetura bem respeitada (0.925), código de alta qualidade (1.000 — expurgo), segurança intacta (1.000), performance não afetada (1.000).
- **Fraquezas**: Testing (0.225) — commit de docs sem testes, o que é aceitável mas reduz o score. Documentation (0.650) — poderia ter DDNA e README update.
- **Insight**: Commits de documentação pura tendem a ter scores entre 70-85 porque as dimensões Testing e Code Quality (parcialmente) não se aplicam. Isso é esperado e não deve ser visto como defeito — o Don pode ajustar os pesos por tipo de commit (feature vs docs vs refactor) no futuro.

---

## 10. Automação

### Pós-Commit Hook (Recomendado)

O engineering score deve ser automaticamente calculado após cada commit, integrado ao workflow `memorize-commit`:

```bash
#!/bin/bash
# hooks/post-commit — Acoplado ao memorize-commit workflow
# Este hook é executado após cada commit bem-sucedido

# 1. Coletar dados do commit (já feito pelo memorize-commit Step 1)
COMMIT_HASH=$(git rev-parse --short HEAD)

# 2. Verificar se já foi avaliado (evitar reavaliação)
if [ -f "internal/embed/cosca/memory/timeline/impact-reports/${COMMIT_HASH}.md" ]; then
    if grep -q "engineering_score" "internal/embed/cosca/memory/timeline/impact-reports/${COMMIT_HASH}.md"; then
        exit 0  # Já avaliado
    fi
fi

# 3. Executar Engineering Score Engine
cosca analytics score "$COMMIT_HASH" --auto --output timeline

# 4. Verificar threshold e alertar
SCORE=$(cosca analytics score "$COMMIT_HASH" --quiet)
if [ "$SCORE" -lt 50 ]; then
    echo "⚠️  ENGINEERING SCORE: $SCORE — REVISÃO OBRIGATÓRIA"
    echo "   Execute: cosca analytics score $COMMIT_HASH --breakdown"
fi
```

### CI Step (Alternativa)

Quando hook não é viável (ex: CI/CD), o score pode ser calculado como step do CI:

```yaml
# .github/workflows/engineering-score.yml
name: Engineering Score
on: [push]
jobs:
  score:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 2
      - name: Calculate Engineering Score
        run: |
          cosca analytics score HEAD --auto --output timeline
          cosca analytics score HEAD --breakdown
```

### Comandos

| Comando | Descrição |
|---------|-----------|
| `cosca analytics score <commit>` | Calcula score para commit específico |
| `cosca analytics score HEAD` | Calcula score para o commit atual (HEAD) |
| `cosca analytics score <commit> --breakdown` | Mostra breakdown completo |
| `cosca analytics score <commit> --output timeline` | Registra na ENGINEERING_TIMELINE.md |
| `cosca analytics score <commit> --trust` | Alimenta Trust Registry |
| `cosca analytics score <range> --aggregate` | Score médio para range de commits |

---

## 11. Regras do Engine

### 11.1 Performance

> **Score deve ser calculável em < 1s**

| Requisito | Estratégia |
|-----------|------------|
| Coleta de dados | Apenas `git diff` (HEAD~1..HEAD) — < 200ms |
| Análise de diff | String-based, sem AST parsing completo — < 400ms |
| Cálculo do score | Matemática simples (multiplicação + soma) — < 10ms |
| Cache | Impact Reports já gerados não são recalculados |

### 11.2 Automatizado

> **Deve rodar pós-commit hook ou CI step**

- **Primeira opção**: Git hook `post-commit` (execução local, feedback imediato)
- **Fallback**: CI step (execução centralizada, visível no PR)
- Ambos usam os mesmos comandos `cosca analytics score`

### 11.3 Transparente

> **Don pode ver o breakdown de cada dimensão**

- Breakdown completo armazenado no Impact Report do commit
- Comando `cosca analytics score <commit> --breakdown` mostra cada dimensão
- Don pode override manual do score (ajuste no Trust Registry)
- Score é imutável após registro (apenas Don pode alterar)

### 11.4 Imparcialidade

- O score é calculado deterministicamente — mesmos inputs produzem mesmos outputs
- Não há viés por agente: o score depende exclusivamente do diff e metadados
- Commits de documentação não são penalizados por falta de testes (dimensão Testing é naturalmente lower-bound para docs-only)
- Commits de feature pura não são beneficiados por "boa documentação"

### 11.5 Versionamento

- O formato `engineering-score.md` versionado via Git
- Qualquer mudança nas regras de score (pesos, subcritérios) gera novo DDNA
- Scores históricos são mantidos mesmo após mudança nas regras (imutabilidade)
- A versão das regras usadas é registrada em cada Impact Report

### 11.6 Override Manual

O Don pode ajustar o score de qualquer commit:

```
# Don override: score manual de 79.5 → 85.0 para commit 792c000
cosca analytics score 792c000 --override 85.0 --reason "Expurgo executado sob ordens diretas do Don, qualidade da documentação subestimada"
```

- Override registrado no Trust Registry com `task_type: manual-adjustment`
- O score original é preservado no histórico
- O Prediction Engine usa o score ajustado (Don tem autoridade)

---

## 12. Relacionamentos

| Documento | Relação |
|-----------|---------|
| [QUALITY_GATES.md](../QUALITY_GATES.md) | Fonte de critérios de qualidade. Engineering Score é o "Gate 2.7" — score técnico pós-commit. |
| [TRUST_REGISTRY.md](../memory/trust/TRUST_REGISTRY.md) | F7.2 — O score alimenta `confidence_delta` e `reliability_score` do agente. |
| [Prediction Engine (F7.1)](../engines/prediction/SKILL.md) | F7.1 — O score histórico do agente alimenta `P(success)`. |
| [ENGINEERING_TIMELINE.md](../memory/timeline/ENGINEERING_TIMELINE.md) | Timeline recebe nova coluna "Score" com o engineering score de cada commit. |
| [memorize-commit workflow](../workflows/memorize-commit.md) | Workflow que coleta os dados e dispara o cálculo do score (Step 9). |
| [next-evolution-phases.md](../knowledge/architecture/next-evolution-phases.md) | F7.3 — Engineering Score por Commit. |
| [COGNITIVE_ENTROPY.md](COGNITIVE_ENTROPY.md) | Score complementar: entropia mede saúde da base de conhecimento; engineering score mede qualidade do commit. |
| [DECISION_DNA.md](../knowledge/architecture/DECISION_DNA.md) | Mudanças nas regras de score geram DDNA. DDNA creation é subcritério de Architecture. |
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) | Engineering score alto contribui para CMI (Consistência). |

---

## 13. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Analytics Chief | Criação inicial — fórmula composta, 6 dimensões, breakdown, pipeline, thresholds, integração F7.1/F7.2, exemplo com commit 792c000. |

---

> **Enforced by**: Analytics Chief + memorize-commit workflow | **Next review**: 2026-08-06 (1 semana)
> **Dependências**: `git diff`, `go test`, `cosca analytics score` command (a implementar)
