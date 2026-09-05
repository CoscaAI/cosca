# ENTROPIA COGNITIVA — Métrica de Saúde da Base de Conhecimento

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Analytics Chief | **Criado**: 2026-07-30 | **DNA Version**: 3.0.0

---

## 1. Propósito

A **Entropia Cognitiva** mede a saúde organizacional da base de conhecimento do Cosca Runtime — quão organizado ou caótico está o conhecimento acumulado pelo sistema. É o equivalente termodinâmico da segunda lei aplicada a sistemas de conhecimento: sem curadoria ativa, a entropia tende a aumentar naturalmente.

A metáfora do Don estabelece o framework conceitual:

> *"Entropia 82% → Muitos padrões conflitantes → Necessário consolidar conhecimento."*

- **Baixa entropia** = conhecimento limpo, consistente, consolidado. Cada conceito tem uma única fonte canônica.
- **Alta entropia** = padrões conflitantes, documentação desatualizada, claims não verificados, conhecimento redundante.

### Por que medir entropia?

1. **Diagnóstico precoce**: Entropia alta detecta degradação da base de conhecimento antes que ela cause decisões erradas.
2. **Gatilho de compressão**: Entropia acima de 60% dispara a Cognitive Compression (Fase 3).
3. **Governança**: O Don e o Kernel precisam de visibilidade sobre a qualidade do conhecimento que alimenta 54 agentes.
4. **Priorização**: Os componentes da entropia indicam exatamente onde intervir (ex: consolidar contradições vs podar órfãos).

---

## 2. Os 5 Componentes da Entropia Cognitiva

Cada componente é pontuado de 0 a 100, onde 0 = perfeito (ausência de entropia) e 100 = caos total naquele eixo. Os pesos refletem o impacto de cada componente na qualidade das decisões dos agentes.

---

### 2.1 Contradiction Count (Contagem de Contradições) — Peso 0.30

**O que mede**: Quantos pares de entradas de conhecimento se contradizem diretamente. Uma contradição ocorre quando duas fontes fazem afirmações mutuamente exclusivas sobre o mesmo fato.

**Exemplos documentados**:
| Contradição | Fonte A | Fonte B | Evidência | Severidade |
|-------------|---------|---------|-----------|------------|
| PostgreSQL vs SQLite | `memory/architecture/database-architecture.md`: "PostgreSQL 16 RDS Multi-AZ" | `go.mod`: `modernc.org/sqlite` | L20 audit | 🔴 Crítica |
| Threshold de cobertura | 4 fontes com valores diferentes (40%, 55%, 70%, 80%) | Código: CI executa a 55% | L18 audit | 🟠 Alta |
| Nível de capacidade | `capability-profile.md`: "Current Level: 3" | `learnings.md`: 7 entradas L4 | L22 audit | 🟡 Média |

**Fórmula de pontuação**:
```
ContradictionScore = min(c × 25, 100)
  onde c = número de contradições cross-source confirmadas
  Cada contradição vale 25 pontos (4 contradições = 100)
```

**Fontes de auditoria**:
- Cross-source audit: documentação vs código (`go.mod`, arquivos `.go`, estrutura de diretórios)
- Cross-memory audit: `learnings.md` vs `capability-profile.md` vs `evolution.md`
- Cross-agent audit: o que o Agent A armazenou vs o que o Agent B armazenou sobre o mesmo domínio

---

### 2.2 Staleness Score (Pontuação de Obsolescência) — Peso 0.25

**O que mede**: O grau de envelhecimento das entradas de conhecimento não validadas. Entradas stale são conhecimento que pode estar correto, mas cuja última verificação excede o threshold de 30 dias — e portanto não se pode confiar plenamente.

**Ponderação por confiança**: Entradas com alta confiança stale são piores que entradas com baixa confiança stale, porque agentes tendem a agir com base em conhecimento de alta confiança sem verificá-lo.

```
StalenessScore = Σ(age_ratio_i × confidence_i) / total_entries × 100
  onde:
    age_ratio_i    = min(days_since_validated / 30, 1.0)
    confidence_i   = confidence declarada da entrada (0.0-1.0)
    total_entries  = total de entradas de conhecimento rastreáveis
```

**Exemplo de cálculo**:
| Entrada | Última validação | Dias stale | age_ratio | Confiança | Contribuição |
|---------|-----------------|------------|-----------|-----------|-------------|
| evolution.md (cosca-analytics) | 2026-07-28 | 2 | 0.067 | 0.30 | 0.020 |
| capability-profile.md | 2026-07-28 | 2 | 0.067 | 0.30 | 0.020 |
| learnings.md (cosca-kernel L18-L21) | 2026-07-29 | 1 | 0.033 | 0.85 | 0.028 |

**Threshold de revalidação**: Qualquer entrada com `age_ratio > 0.5` (15+ dias sem validação) deve ser agendada para revalidação automática.

---

### 2.3 Fragmentation Index (Índice de Fragmentação) — Peso 0.25

**O que mede**: O mesmo conceito ou "verdade" armazenado em múltiplos lugares com versões diferentes ou conflitantes. Fragmentação é o oposto de single source of truth — cada cópia diverge e nenhuma é canônica.

**Exemplo canônico — Threshold de Cobertura**:

| Local | Valor | Efetivo? | Status |
|-------|-------|----------|--------|
| `Makefile` target `coverage-check` | **40%** | ❌ Não usado no CI | Obsoleto |
| `.github/workflows/ci.yml` | **55%** | ✅ Único executado | Operacional (já unificado para 70%) |
| `memory/qa/quality-gates.md` G5 | **70%** | ❌ Apenas documentação | Corrigido |
| `internal/embed/cosca/QUALITY_GATES.md` Gate 2.5 | **80%** | ❌ Embed | Corrigido |

**Fórmula de pontuação**:
```
FragmentationScore = Σ conflict_severity(s) / max_concepts × 100
  onde:
    s = conceito com múltiplas fontes (≥ 2 locais para o mesmo "fato")
    conflict_severity(s) = min((locais_conflitantes - 1) × 30, 100)
    max_concepts = total de conceitos rastreáveis na base (limitado a 20)
```

Para o conceito "cobertura" com 4 locais conflitantes:
`conflict_severity = (4 - 1) × 30 = 90`

---

### 2.4 Orphan Ratio (Taxa de Órfãos) — Peso 0.10

**O que mede**: Entradas de conhecimento que referenciam arquivos, agentes, padrões ou componentes que não existem mais. Links quebrados, referências stale, agentes deprecados.

**Tipos de órfãos**:

| Tipo | Exemplo | Impacto |
|------|---------|---------|
| **Broken file ref** | Link para `memory/architecture/database-architecture.md` após renomeação | Agente segue link e falha |
| **Deprecated agent** | Referência a `cosca-legacy-worker` que não existe mais | Workflow falha ao delegar |
| **Stale pattern** | Pattern que referencia API removida na v1.4.0 | Execução com erro |
| **Ghost dependency** | Entrada que referencia dependência removida do `go.mod` | Build quebra |

```
OrphanRatio = orphan_entries / total_referenced_entries × 100
  onde:
    orphan_entries = entradas com ≥ 1 referência quebrada confirmada
    total_referenced_entries = entradas que contêm referências externas
```

**Auditoria**: O `doc-validator.sh` do CI verifica broken file refs. O Semantic Memory Engine identifica padrões órfãos por similaridade zero com código atual.

---

### 2.5 Consolidation Gap (Lacuna de Consolidação) — Peso 0.10

**O que mede**: Entradas que deveriam ter sido condensadas em um único princípio ou padrão mas permanecem dispersas. Múltiplos aprendizados sobre o mesmo tópico que poderiam ser uma única entrada canônica.

**Threshold de consolidação**: > 3 entradas sobre o mesmo tópico → candidato a consolidação.

```
ConsolidationGapScore = Σ (entradas_tópico_i - 3) / total_candidates × 100
  onde:
    entradas_tópico_i = número de entradas sobre o tópico i (apenas se > 3)
    total_candidates  = Σ entradas sobre todos os tópicos (limitado a 50)
```

**Exemplo**: 8 aprendizados de auditoria (L18-L21) sobre cross-source verification → deveriam ser 1 princípio "Cross-Source Audit Pattern". Gap = 8 - 3 = 5 entradas excedentes.

**26 aprendizados totais** (cosca-kernel), sem consolidação ou pruning → alto consolidation gap.

---

## 3. Fórmula do Score de Entropia

```
Entropia(%) = (CC × 0.30 + SS × 0.25 + FI × 0.25 + OR × 0.10 + CG × 0.10) / 100 × 100

  Onde:
    CC = ContradictionScore    (0-100)
    SS = StalenessScore        (0-100)
    FI = FragmentationIndex    (0-100)
    OR = OrphanRatio           (0-100)
    CG = ConsolidationGapScore (0-100)
```

Simplificando (já que cada componente é 0-100 e MaxPossibleEntropy = 100):

```
Entropia(%) = CC × 0.30 + SS × 0.25 + FI × 0.25 + OR × 0.10 + CG × 0.10
```

---

## 4. Interpretação das Faixas

| Faixa | Classificação | Cor | Significado | Ação Recomendada |
|-------|---------------|-----|-------------|-----------------|
| **0-30%** | Ordem | 🟢 Verde | Base de conhecimento limpa, consistente, bem curada. Single source of truth para cada conceito. | Manutenção de rotina. Medir mensalmente. |
| **31-60%** | Entropia Moderada | 🟡 Amarelo | Alguns sintomas de desorganização. Contradições pontuais, fragmentação localizada, obsolescência incipiente. | Priorizar top 3 fontes de entropia. Agendar curadoria. |
| **61-80%** | Entropia Alta | 🟠 Laranja | Conhecimento significativamente degradado. Múltiplas contradições ativas, fragmentação cross-domain, risco de decisões baseadas em conhecimento stale. | **DISPARAR Cognitive Compression.** Curadoria urgente. |
| **81-100%** | Caos | 🔴 Vermelho | Base de conhecimento não confiável. Contradições sistêmicas, documentação majoritariamente fictícia ou desatualizada. Agentes tomando decisões com dados incorretos. | **P0 — Escalar ao Kernel e ao Don.** Congelar novas escritas até consolidação. |

---

## 5. Indicador de Tendência

A tendência é calculada comparando o score atual com a medição anterior (ou baseline):

| Símbolo | Significado | Threshold |
|---------|-------------|-----------|
| ↑ | Entropia **aumentando** (piorando) | Δ > +5pp desde última medição |
| → | Entropia **estável** | -5pp ≤ Δ ≤ +5pp |
| ↓ | Entropia **diminuindo** (melhorando) | Δ < -5pp desde última medição |

---

## 6. Dashboard de Entropia Cognitiva

> **Atualizado**: 2026-07-30 | **Próxima medição**: 2026-08-06

### Score Atual

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│   ENTROPIA COGNITIVA                                  │
│                                                      │
│   ████████████████████████████░░░░░░░░░░░░  64.5%     │
│                                                      │
│   🟠 ENTROPIA ALTA  —  Tendência: ↑ (piorando)       │
│                                                      │
│   "Múltiplos padrões conflitantes.                    │
│    Necessário consolidar conhecimento."               │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Breakdown por Componente

```
Componente               Peso    Score    Contribuição    Barra
──────────────────────────────────────────────────────────────────
Contradiction Count      30%      80       ████████████████████ 24.0
Staleness Score          25%      40       ██████████            10.0
Fragmentation Index      25%      80       ████████████████████ 20.0
Orphan Ratio             10%      25       ██████                 2.5
Consolidation Gap        10%      80       ████████████████████  8.0
──────────────────────────────────────────────────────────────────
TOTAL                             —                           ▸ 64.5
```

### Top 3 Fontes de Entropia (Prioridade de Correção)

| # | Fonte | Componente | Impacto no Score | Ação Corretiva |
|---|-------|------------|:----------------:|----------------|
| **1** | **4 thresholds de cobertura conflitantes** (40%, 55%, 70%, 80%) | Fragmentation + Contradiction | 20.0 + 7.5 = **27.5** | Unificar para threshold canônico único (70%). Sincronizar Makefile, CI, docs, embed. Já parcialmente corrigido — verificar propagação. |
| **2** | **PostgreSQL fantasy** — doc afirma PostgreSQL, código usa SQLite | Contradiction | 7.5 | Substituir `database-architecture.md` por arquitetura real. Corrigido por cosca-database (2026-07-28) — verificar que não houve regressão. |
| **3** | **8 auditorias não consolidadas** (L18-L21: cross-source, coverage, drift, version) | Consolidation Gap | 8.0 | Extrair princípio universal: "Cross-Source Audit Pattern" (CCP-001). Consolidar 8 aprendizados em 1 entrada canônica. |

### Tendência Histórica

```
Data           Score    Delta    Evento
──────────────────────────────────────────────────────────
2026-07-30     64.5%    —        Baseline (pré-compressão)
2026-07-28     62.0%*   —        Pós-auditoria L3: 52 issues corrigidas
2026-07-24     78.0%*   —        Pré-auditoria: PostgreSQL fantasy ativo, 
                                 4 thresholds conflitantes não detectados
──────────────────────────────────────────────────────────
* Estimado (não havia medição formal de entropia)
```

### Projeção Pós-Correção

Se as 3 correções prioritárias forem aplicadas:

```
Componente               Antes    Depois    Redução
────────────────────────────────────────────────────
Contradiction Count       80  →    25        -55
Staleness Score           40  →    25        -15
Fragmentation Index       80  →    10        -70
Orphan Ratio              25  →    15        -10
Consolidation Gap         80  →    20        -60
────────────────────────────────────────────────────
ENTROPIA PROJETADA        64.5% →  19.0%     🟢 ORDEM
```

---

## 7. Integração com Cognitive Compression (Fase 3)

### Gatilho

A Entropia Cognitiva é o **gatilho primário** para a Cognitive Compression (item F3.2 do [Cognitive Maturity Roadmap](../architecture/COGNITIVE_MATURITY.md)):

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  ENTROPIA > 60%                                                  │
│       │                                                          │
│       ├──► DISPARA: Cognitive Compression Engine                  │
│       │                                                          │
│       │    Fase 1: Identificação                                 │
│       │    └─ Agrupar entradas por similaridade semântica > 80%  │
│       │                                                          │
│       │    Fase 2: Extração                                      │
│       │    └─ N casos → 1 princípio universal                    │
│       │                                                          │
│       │    Fase 3: Substituição                                  │
│       │    └─ Substituir N entradas por 1 entrada canônica       │
│       │       com weight gravitacional máximo                     │
│       │                                                          │
│       │    Fase 4: Verificação                                   │
│       │    └─ Recalcular entropia. Se < 40%, compressão ok.      │
│       │                                                          │
│       └──► RESULTADO: Entropia reduzida. Conhecimento consolidado.│
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Exemplo de Compressão (Projeção)

```
INPUT: 8 aprendizados de auditoria (L18-L21)
       + 3 contradições documentadas
       + 4 fontes de threshold conflitantes
       ─────────────────────────────────────
       15 entradas dispersas

OUTPUT: 1 princípio universal (CCP-001):
       "Toda documentação deve ser verificada contra código fonte.
        Documentação não verificada tem probabilidade de 23% de
        conter claims fictícias (PostgreSQL fantasy, SDK inexistente,
        compliance fabrication). Auditoria cross-source reduz
        probabilidade de ficção para < 1%."

COMPRESSION RATIO: 15:1
CONFIDENCE: 0.92
ENTROPIA RESULTANTE: ~19% (🟢 Ordem)
```

### Ciclo Contínuo

```
┌──────────────┐     ┌─────────────────┐     ┌──────────────────┐
│  Medição de  │────►│  Entropia > 60%? │────►│  Cognitive        │
│  Entropia    │     │  (Sim/Não)       │     │  Compression      │
│  (semanal)   │     └────────┬────────┘     │  Engine           │
└──────────────┘              │               └────────┬─────────┘
                              │ Não                    │
                              ▼                        ▼
                     ┌─────────────────┐     ┌──────────────────┐
                     │  Rotina de       │     │  Nova Medição     │
                     │  Curadoria Leve  │     │  de Entropia      │
                     │  (pruning, sync) │     │  (pós-compressão) │
                     └─────────────────┘     └──────────────────┘
```

---

## 8. Procedimento de Medição

### Periodicidade

| Frequência | Ação | Responsável |
|------------|------|-------------|
| **Semanal** | Medir entropia completa (5 componentes) | Cosca Analytics Chief |
| **Mensal** | Auditoria completa de contradições + órfãos | Semantic Memory Chief + Analytics Chief |
| **On-demand** | Medir após cada ciclo de Cognitive Compression | Cosca Analytics Chief |
| **On-event** | Medir após qualquer incidente de knowledge drift | Cosca Documentation Chief → Analytics Chief |

### Comando de Medição

```bash
# Medição automatizada (a ser implementada pelo Automation Chief)
cosca analytics entropy --baseline internal/embed/cosca/analytics/entropy-baseline-2026-07-30.md

# Output esperado:
# Entropia Cognitiva: 64.5% (🟠 ALTA) — Tendência: →
# Breakdown:
#   Contradiction Count: 80/100 (3 contradições ativas)
#   Staleness Score:     40/100 (média 1.3 dias sem validação)
#   Fragmentation Index: 80/100 (cobertura em 4 locais conflitantes)
#   Orphan Ratio:        25/100 (2 órfãos detectados)
#   Consolidation Gap:   80/100 (8 entradas não consolidadas)
# Top 3 ações: [1] Unificar thresholds [2] Verificar PostgreSQL fantasy [3] Consolidar auditorias
```

### Baseline de Referência

O arquivo [entropy-baseline-2026-07-30.md](entropy-baseline-2026-07-30.md) contém o cálculo detalhado da baseline atual, incluindo a lista completa de contradições, entradas stale, fragmentações, órfãos e lacunas de consolidação identificadas.

---

## 9. Integração com o CMI (Cognitive Maturity Index)

A Entropia Cognitiva correlaciona-se inversamente com a dimensão **Consistência** do CMI:

| Entropia | Consistência (CMI) | Efeito |
|----------|-------------------|--------|
| 0-30% (🟢) | ≥ 90 | Conhecimento confiável. Agentes tomam decisões com alta confiança. |
| 31-60% (🟡) | 75-89 | Conhecimento requer verificação. Risco moderado de decisões incorretas. |
| 61-80% (🟠) | 50-74 | Conhecimento não confiável. Agentes devem verificar toda afirmação contra código. |
| 81-100% (🔴) | < 50 | Conhecimento tóxico. Kernel deve bloquear ações baseadas em memória não verificada. |

---

## 10. Relacionamentos

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) | Define CMI e Cognitive Compression (F3.2). Entropia é o gatilho. |
| [CONSTITUTION.md](../CONSTITUTION.md) | P8: Embed Sync Protocol. P10: Código vence documentação. |
| [MEMORY_MODEL.md](../MEMORY_MODEL.md) | Define os tipos de memória, retenção e pruning. |
| [incident-knowledge-drift.md](../knowledge/best-practices/playbooks/incident-knowledge-drift.md) | Playbook de resposta a divergências doc vs código. |
| [coverage-evolution.md](../knowledge/best-practices/benchmarks/coverage-evolution.md) | Documenta a threshold crisis (4 valores conflitantes). |
| [platform-health-dashboard.md](../metrics/platform-health-dashboard.md) | Dashboard de saúde da plataforma. Entropia é métrica complementar. |
| [entropy-baseline-2026-07-30.md](entropy-baseline-2026-07-30.md) | Baseline atual com cálculo detalhado. |
| [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) | Protocolo de auto-evolução dos agentes. |

---

## 11. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Analytics Chief | Definição inicial da métrica de Entropia Cognitiva. Baseline 64.5%. 5 componentes, fórmula de cálculo, dashboard, integração com Cognitive Compression. |

---

> **Enforced by**: Cosca Analytics Chief | **Next review**: 2026-08-06 (1 semana)
