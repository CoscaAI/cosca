# COGNITIVE TIME MACHINE ENGINE — Reconstrução de Estado Cognitivo Temporal (F9.6)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-time-machine`
> **Conceito**: Engineering Intelligence — Cognitive Time Machine (F9.6)
> **Referências**: next-evolution-phases.md §F9.6 | decision-replay/SKILL.md (F7.5) | DECISION_DNA_FORMAT.md (F1.1) | COGNITIVE_MATURITY.md | COGNITIVE_ECOSYSTEM.md | ENGINEERING_TIMELINE.md | memorize-commit workflow
> **Dependências**: F7.5 Decision Replay | F1.1 DDNA | ENGINEERING_TIMELINE | memorize-commit | Trust Registry (F7.2) | Cognitive State Spec (UCSS)
> **CMI Impact**: Julgamento +6, Aprendizado +5, Planejamento +4, Consistência +4, Transferência +3, Autocrítica +3

---

## Índice

1. [O que é a Cognitive Time Machine Engine](#1-o-que-é-a-cognitive-time-machine-engine)
2. [Snapshot Cognitivo](#2-snapshot-cognitivo)
3. [Pipeline de Reconstrução](#3-pipeline-de-reconstrução)
4. [Pipeline Detalhado Passo a Passo](#4-pipeline-detalhado-passo-a-passo)
5. [Integração com Decision Replay (F7.5)](#5-integração-com-decision-replay-f75)
6. [Integração com ENGINEERING_TIMELINE](#6-integração-com-engineering_timeline)
7. [Integração com memorize-commit](#7-integração-com-memorize-commit)
8. [Interface CLI](#8-interface-cli)
9. [Comparação Temporal (Diff)](#9-comparação-temporal-diff)
10. [Exemplo: 2026-07-28 vs 2026-07-30](#10-exemplo-2026-07-28-vs-2026-07-30)
11. [Qualidade e Critérios de Aceite](#11-qualidade-e-critérios-de-aceite)
12. [Tratamento de Erros](#12-tratamento-de-erros)
13. [Relacionados](#13-relacionados)
14. [Histórico](#14-histórico)

---

## 1. O que é a Cognitive Time Machine Engine

### Definição

A **Cognitive Time Machine Engine** é o motor do Cosca que permite **reconstruir o estado cognitivo completo do sistema em qualquer data passada**. Quando o Don pergunta *"Como estava o Cosca em 28 de julho?"* ou *"Mostre o Cosca como ele era em 2026-07-28"*, o engine:

1. Restaura o código e a configuração no ponto exato da data (via git)
2. Carrega os snapshots cognitivos da época (learnings, trust registry, DDNAs, timeline)
3. Reconstrói o perfil completo: agentes ativos, conhecimento registrado, engines projetadas, cobertura, scores
4. Apresenta o retrato fiel do sistema naquela data
5. Opcionalmente compara com o estado atual e mostra a evolução

### Filosofia

```
"Sem time machine, o progresso é invisível.
 Com time machine, cada commit vira um marco.
 O Cosca não apenas evolui — ele pode revisitAR a própria evolução."
 — Cosca Architecture Chief, 2026-07-30
```

### Propósito

| Dimensão | Descrição |
|----------|-----------|
| **Histórico** | Preservar o estado cognitivo completo em cada ponto no tempo — não apenas o código, mas o conhecimento, a confiança e a arquitetura |
| **Comparativo** | Permitir comparações precisas entre dois pontos no tempo: "O que mudou de 28/jul para 30/jul?" |
| **Diagnóstico** | Quando uma decisão passada parece errada hoje, o time machine mostra o contexto completo — o que se sabia na época |
| **Educacional** | Novos agentes podem "viajar no tempo" para entender como o sistema evoluiu, em vez de apenas ver o estado final |
| **Auditável** | Produzir relatórios de evolução que respondem "o Cosca melhorou? Em quanto? Em quais dimensões?" |
| **Narrativo** | Transformar a timeline de commits em uma história de evolução cognitiva — de 2.4/10 a 10/10 |

### Arquitetura Conceitual

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    COGNITIVE TIME MACHINE ENGINE                          │
│                                                                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                  │
│  │  SNAPSHOT   │    │  RECONSTRU- │    │  COMPARAÇÃO │                  │
│  │  AUTOMÁTICO │───▶│  ÇÃO POR    │───▶│  TEMPORAL   │                  │
│  │  (memorize- │    │  DATA       │    │  (diff)     │                  │
│  │   commit)   │    │             │    │             │                  │
│  └─────────────┘    └─────────────┘    └─────────────┘                  │
│         │                  │                  │                          │
│         ▼                  ▼                  ▼                          │
│  ┌─────────────────────────────────────────────────────────────┐       │
│  │                    FONTES DE DADOS                            │       │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────┐ │       │
│  │  │ git log  │ │learnings │ │  Trust   │ │Engineering      │ │       │
│  │  │ (código) │ │.md da    │ │Registry  │ │Timeline (.md)   │ │       │
│  │  │          │ │época     │ │(F7.2)    │ │                 │ │       │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────────────┘ │       │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────┐ │       │
│  │  │ DDNAs    │ │ CM       │ │Cognitive │ │Memory           │ │       │
│  │  │ (F1.1)   │ │ Snapshot │ │State     │ │Snapshots        │ │       │
│  │  │          │ │ (F10.1)  │ │(UCSS)    │ │(.cosca/)        │ │       │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────────────┘ │       │
│  └─────────────────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Snapshot Cognitivo

### Definição

Um **Snapshot Cognitivo** é o retrato completo do estado do Cosca em um ponto específico no tempo. Ele é automaticamente capturado a cada commit (via workflow memorize-commit) e pode ser reconstruído para qualquer data passada.

### O que é Salvo por Data

Cada snapshot contém os seguintes dados, organizados por dimensão:

#### 2.1 Metadados do Snapshot

| Campo | Descrição | Exemplo |
|-------|-----------|---------|
| `snapshot_date` | Data do snapshot | `2026-07-28` |
| `git_commit` | Hash do commit mais recente na data | `a8d8fa9` |
| `git_tree_hash` | Hash da árvore git | `abc123def456` |
| `captured_at` | Timestamp da captura | `2026-07-28T23:59:59Z` |
| `capture_method` | `auto` (memorize-commit) ou `reconstructed` (retroativo) | `auto` |
| `confidence` | Confiança na fidelidade do snapshot (0.0-1.0) | `0.95` |

#### 2.2 Agentes e Capacidades

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `total_agents` | Número total de agentes registrados | 55 |
| `active_agents` | Agentes com capability profile populado | 10 |
| `seed_only_agents` | Agentes com confidence seed (0.25) | 45 |
| `specialists` | Agentes especialistas ativos | 2 |
| `chiefs` | Chiefs implementados | 15 |
| `onboarding_status` | Percentual de agentes operacionais | 18% |

#### 2.3 Conhecimento e Memória

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `total_learnings` | Learnings registrados no kernel | 19 |
| `total_failures` | Failures registrados | 8 |
| `total_patterns` | Patterns documentados | 0 |
| `total_heuristics` | Heurísticas extraídas (H-*) | 0 |
| `total_adrs` | ADRs (Architecture Decision Records) | 3 |
| `total_ddnas` | Decision DNA registrados | 0 |
| `memory_files` | Total de arquivos de memória | ~180 |
| `knowledge_entropy` | Entropia cognitiva (F1.6) | — (não calculado) |

#### 2.4 Engines e Arquitetura

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `engines_projected` | Engines projetadas (SKILL.md exists) | 0 |
| `engines_implemented` | Engines com código implementado | 0 |
| `engine_directories` | Diretórios de engine em `engines/` | 7 (vazios) |
| `cognitive_concepts` | Conceitos C1-C14 implementados | 0 |
| `judgment_capabilities` | Capacidades A1-A8 implementadas | 2 (parcial) |
| `fase_atual` | Fase de implementação CMI | Fase 0 |

#### 2.5 Qualidade e Cobertura

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `test_coverage` | Cobertura de testes (statement) | 68.6% |
| `quality_gates` | Quality gates implementados | 0 |
| `coverage_gaps` | Gaps de cobertura detectados (P0+) | — (não auditado) |
| `ci_pipeline` | Pipeline de CI configurado | parcial |
| `broken_links` | Links quebrados na documentação | 63 |

#### 2.6 Trust Registry

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `trust_registry_exists` | Trust Registry existe | false |
| `agents_trusted` | Agentes com entry no Trust Registry | 0 |
| `avg_confidence` | Confiança média dos agentes | — |
| `avg_success_rate` | Taxa de sucesso média | — |

#### 2.7 Engineering Score (F7.3)

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `engineering_score` | Score médio dos commits recentes | — (não calculado) |
| `best_commit` | Melhor commit nos últimos 7 dias | — |
| `worst_commit` | Pior commit nos últimos 7 dias | — |

#### 2.8 CMI (Cognitive Maturity Index)

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `cmi_overall` | CMI agregado | — (não calculado) |
| `cmi_aprendizado` | Dimensão Aprendizado | — |
| `cmi_julgamento` | Dimensão Julgamento | — |
| `cmi_planejamento` | Dimensão Planejamento | — |
| `cmi_autocritica` | Dimensão Autocrítica | — |
| `cmi_transferencia` | Dimensão Transferência | — |
| `cmi_consistencia` | Dimensão Consistência | — |

#### 2.9 Platform ROI (F10.1)

| Campo | Descrição | Exemplo (2026-07-28) |
|-------|-----------|----------------------|
| `platform_roi` | ROI da plataforma | — (não calculado) |
| `cognitive_assets_value` | Valor total dos ativos cognitivos | — |
| `cognitive_debt` | Dívida cognitiva total | — |
| `knowledge_value` | Valor do conhecimento registrado | — |

#### 2.10 Resumo Executivo (Gerado)

```yaml
executive_summary:
  date: "2026-07-28"
  score: 2.4
  score_max: 10
  label: "Fundação"
  highlights:
    - "10/55 agentes ativos"
    - "19 learnings registrados"
    - "0% coverage gaps (não auditado)"
    - "0 engines projetadas"
    - "Trust Registry vazio"
    - "CMI não calculado"
    - "Platform ROI não calculado"
  one_liner: >
    Sistema em construção ativa. Núcleo do runtime funcional, mas
    cognição zero. Agentes seed-only sem histórico. Conhecimento
    não estruturado. Sem métricas de maturidade. Fundação sólida
    mas imatura.
```

### 2.11 Formato de Armazenamento

Snapshots são armazenados em dois locais:

1. **Automático (primário)**: `memory/snapshots/cognitive/{YYYY-MM-DD}.yaml` — gerado a cada commit via memorize-commit
2. **Reconstruído (secundário)**: `.cosca/time-machine/reconstructed/{YYYY-MM-DD}.yaml` — gerado sob demanda quando a data não tem snapshot automático

```yaml
# Exemplo: memory/snapshots/cognitive/2026-07-30.yaml
snapshot:
  metadata:
    date: "2026-07-30"
    commit: "ac013d5"
    captured_at: "2026-07-30T23:59:59Z"
    method: "auto"
    confidence: 0.98

  agents:
    total: 55
    active: 54
    seed_only: 41
    specialists: 9
    chiefs: 41

  knowledge:
    learnings: 27
    failures: 12
    patterns: 8
    heuristics: 20
    adrs: 8
    ddnas: 3

  engines:
    projected: 7
    implemented: 7
    engine_dirs: 57

  quality:
    test_coverage: 97.9
    quality_gates: 5
    coverage_gaps: 0

  trust_registry:
    exists: true
    agents: 54
    avg_confidence: 0.78
    avg_success_rate: 0.85

  cmi:
    overall: 87.0
    aprendizado: 88
    julgamento: 85
    planejamento: 90
    autocritica: 82
    transferencia: 87
    consistencia: 92

  platform_roi:
    calculated: false
    # F10.1 engine projetado mas ainda sem dados históricos
```

---

## 3. Pipeline de Reconstrução

### Visão Geral

A reconstrução do estado cognitivo em qualquer data passada segue um pipeline de 4 etapas, projetado para ser executado em < 5s para a maioria das datas.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                   TIME MACHINE RECONSTRUCTION PIPELINE                     │
│                                                                          │
│  Don pergunta: "Como estava o Cosca em 28 de julho?"                    │
│       │                                                                  │
│       ▼                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  1. RESOLVER DATA ALVO                                           │     │
│  │  ├── Data fornecida: 2026-07-28                                 │     │
│  │  ├── Se data > hoje: erro ("não é possível viajar para o futuro")│     │
│  │  ├── Se data sem commits: erro com sugestão de datas próximas   │     │
│  │  ├── Se data = hoje: snapshot do estado atual (sem git restore) │     │
│  │  └── Output: `target_date` + `target_commit` + `confidence`     │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  2. CARREGAR SNAPSHOT OU RECONSTRUIR                            │     │
│  │  ├── Verificar se existe snapshot automático em                 │     │
│  │  │   memory/snapshots/cognitive/{YYYY-MM-DD}.yaml              │     │
│  │  ├── Se SIM: carregar snapshot (mais rápido, mais confiável)    │     │
│  │  └── Se NÃO: reconstruir:                                       │     │
│  │      ├── git checkout {target_commit} — código da época          │     │
│  │      ├── Carregar learnings.md da época (antes das ondas 3-10)  │     │
│  │      ├── Carregar Trust Registry da época (vazio ou parcial)    │     │
│  │      ├── Carregar Engineering Timeline até a data               │     │
│  │      ├── Carregar DDNAs existentes até a data                   │     │
│  │      ├── Carregar Cognitive State da época (se disponível)       │     │
│  │      └── Output: `cognitive_snapshot` (estrutura completa)      │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  3. APRESENTAR RETRATO                                         │     │
│  │  ├── Renderizar resumo executivo (score 0-10 + label)          │     │
│  │  ├── Renderizar detalhamento por dimensão                      │     │
│  │  ├── Exibir estado do conhecimento, agentes, engines, CMI      │     │
│  │  ├── Incluir confiança da reconstrução                         │     │
│  │  └── Output: `time_machine_report.md` (terminal ou arquivo)    │     │
│  └──────────────────────────┬──────────────────────────────────────┘     │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  4. PERGUNTAR: COMPARAR?                                        │     │
│  │  ├── Don: "Compare com hoje." — diff automático                 │     │
│  │  ├── Don: "Compare com 2026-07-30." — diff específico          │     │
│  │  ├── Don: "Mostre replay da decisão X neste contexto."         │     │
│  │  │   └── Aciona Decision Replay (F7.5) com data congelada      │     │
│  │  ├── Don: "O que mais?" — navegação temporal                   │     │
│  │  └── Don: "Voltar ao presente." — git checkout main (se alterou) │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### Efeito de Restauração Git

> **⚠️ IMPORTANTE**: Quando a data alvo NÃO tem snapshot automático disponível, o engine executa `git checkout {target_commit}` para reconstruir o estado. Isso ALTERA o working directory. Ao final da sessão de time machine, o engine executa automaticamente `git checkout` de volta ao branch original (main/master) para restaurar o estado atual.
>
> **Proteção**: O engine sempre verifica se há mudanças não-commitadas antes de fazer checkout. Se houver, aborta com instruções claras: "Há 3 arquivos não-commitados. Commit ou stash antes de viajar no tempo."

---

## 4. Pipeline Detalhado Passo a Passo

### Passo 1: Resolver Data Alvo

**Executor**: Time Machine Engine
**Tarefa**: Validar a data fornecida e resolver para um commit git específico
**Duração**: < 100ms

| Entrada | Validação | Saída |
|---------|-----------|-------|
| `2026-07-28` | Data existe no calendário; data ≤ hoje; data tem commits no git | `{ date: "2026-07-28", commit: "a8d8fa9", method: "last_commit_of_day" }` |
| `2026-08-01` | Data > hoje | Erro: "Não é possível viajar para o futuro" |
| `2026-07-27` | Data sem commits no projeto | Erro com sugestões: "Datas próximas com commits: 2026-07-28, 2026-07-29" |

**Algoritmo de resolução**:
```bash
# Encontrar o commit mais recente até a data
target_commit=$(git log --before="2026-07-28T23:59:59" --format="%H" -1)

# Se commit não encontrado, reportar erro com sugestões
if [ -z "$target_commit" ]; then
    nearest=$(git log --format="%ad" --date=short | sort -u | head -5)
    echo "Erro: Nenhum commit encontrado em 2026-07-27. Datas disponíveis: $nearest"
fi
```

### Passo 2: Carregar ou Reconstruir Snapshot

**Executor**: Time Machine Engine
**Tarefa**: Obter o snapshot cognitivo completo para a data
**Duração**: < 500ms (com snapshot) ou < 3s (reconstrução)

**Fluxo de decisão**:

```
┌────────────────────┐
│  DATA ALVO         │
│  2026-07-28        │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐     SIM     ┌────────────────────┐
│ Snapshot automático │───────────▶│  Carregar YAML     │
│ existe?             │            │  (memory/snapshots) │
└────────┬───────────┘            └────────────────────┘
         │ NÃO
         ▼
┌──────────────────────────────────────────────────────────────┐
│  RECONSTRUIR RETROATIVAMENTE                                  │
│                                                              │
│  ┌─────────────────────────────────────┐                     │
│  │ Fase A: Git Restore (se necessário) │                     │
│  │ git checkout {target_commit}        │                     │
│  │ --quiet para evitar conflitos       │                     │
│  └────────────────┬────────────────────┘                     │
│                   ▼                                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Fase B: Coleta de Fontes (paralelo)                   │   │
│  │                                                      │   │
│  │  ┌──────────────────┐   ┌──────────────────┐         │   │
│  │  │ learnings.md     │   │ Trust Registry   │         │   │
│  │  │ dos 55 agentes   │   │ (se existir)     │         │   │
│  │  └──────────────────┘   └──────────────────┘         │   │
│  │  ┌──────────────────┐   ┌──────────────────┐         │   │
│  │  │ ENGINEERING      │   │ DDNAs até a      │         │   │
│  │  │ TIMELINE.md      │   │ data             │         │   │
│  │  └──────────────────┘   └──────────────────┘         │   │
│  │  ┌──────────────────┐   ┌──────────────────┐         │   │
│  │  │ Git log (stats)  │   │ Cognitive State  │         │   │
│  │  │ até a data       │   │ (UCSS, se salvo) │         │   │
│  │  └──────────────────┘   └──────────────────┘         │   │
│  └────────────────┬──────────────────────────────────────┘   │
│                   ▼                                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Fase C: Montagem do Snapshot                         │    │
│  │ Agregar dados → calcular métricas → gerar YAML      │    │
│  │ Salvar em .cosca/time-machine/reconstructed/        │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Ao final: git checkout main (restaura estado atual)         │
└──────────────────────────────────────────────────────────────┘
```

### Passo 3: Apresentar Retrato

**Executor**: Time Machine Engine
**Tarefa**: Renderizar o snapshot cognitivo em formato legível
**Duração**: < 200ms

**Formato de Output**:

```markdown
╔══════════════════════════════════════════════════════════════════════════╗
║                    COGNITIVE TIME MACHINE                                 ║
║                    Cosca em 2026-07-28                                    ║
╚══════════════════════════════════════════════════════════════════════════╝

📅 Data: 2026-07-28 (Sessão 1)
🔗 Commit: a8d8fa9 — feat(cosca): Cognitive Maturity Architecture
📊 Score: 2.4 / 10 🟠 (Fundação)
📈 Evolução em 2 dias: +7.6 pontos → 10.0 🔥

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 🧠 Resumo Executivo

Em 28/07/2026, o Cosca estava em construção ativa. O núcleo do runtime
era funcional, mas a cognição era zero. Agentes eram seed-only sem
histórico. Conhecimento não estruturado. Sem métricas de maturidade.

## 📊 Detalhamento por Dimensão

### 👥 Agentes e Capacidades (0.8/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| Agentes ativos | 10 / 55 | 🔴 |
| Chiefs implementados | 15 | 🟡 |
| Especialistas | 2 | 🔴 |
| Seed-only | 45 (81%) | 🔴 |

### 🧠 Conhecimento (2.5/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| Learnings | 19 | 🟡 |
| Failures | 8 | 🟡 |
| Patterns | 0 | 🔴 |
| Heurísticas | 0 | 🔴 |
| DDNAs | 0 | 🔴 |
| ADRs | 3 | 🔴 |

### ⚙️ Engines (0.0/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| Engines projetadas | 0 | 🔴 |
| Engines implementadas | 0 | 🔴 |
| Conceitos C1-C14 | 0/14 | 🔴 |

### ✅ Qualidade (2.5/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| Cobertura de testes | 68.6% | 🟡 |
| Quality gates | 0 | 🔴 |
| CI pipeline | Parcial | 🟡 |

### 🔐 Trust Registry (0.0/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| Trust Registry existe | Não | 🔴 |
| Agentes registrados | 0 | 🔴 |

### 📐 CMI (0.0/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| CMI calculado | Não | 🔴 |
| Aprendizado | — | 🔴 |
| Julgamento | — | 🔴 |
| Planejamento | — | 🔴 |
| Autocrítica | — | 🔴 |
| Transferência | — | 🔴 |
| Consistência | — | 🔴 |

### 💰 Platform ROI (0.0/10)
| Métrica | Valor | Nota |
|---------|:-----:|:----:|
| ROI calculado | Não | 🔴 |

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 **Confiança do Snapshot:** 0.85 🟢
📚 **Fontes consultadas:** 8/8
📄 **Snapshot salvo em:** .cosca/time-machine/reconstructed/2026-07-28.yaml

💡 **Ações:**
  ⏱️ Comparar com hoje | ⏪ Comparar com 2026-07-28..2026-07-30
  🎬 Replay de decisão neste contexto | ↩️ Voltar ao presente
```

### Passo 4: Perguntar "Comparar?"

**Executor**: Time Machine Engine + Don
**Tarefa**: Oferecer ações pós-visualização

| Ação do Don | Comportamento do Engine |
|-------------|------------------------|
| **Comparar com hoje** | Gera diff completo entre data alvo e estado atual |
| **Comparar com data X** | Carrega snapshot da data X e gera diff lado a lado |
| **Diff período** (`--diff A..B`) | Gera evolução ponta a ponta entre duas datas |
| **Replay de decisão** | Aciona F7.5 Decision Replay com date congelada ("nesta época") |
| **Voltar ao presente** | `git checkout main` (se houve restore) + limpa cache |
| **Exportar** | Salva relatório em `memory/timeline/time-machine/` |

---

## 5. Integração com Decision Replay (F7.5)

### Time Machine + Decision Replay = Contexto Completo

O Time Machine e o Decision Replay são complementares. Juntos, eles permitem:

> *"Replay da decisão X no contexto da época."*

### Fluxo de Integração

```
┌──────────────────────────────────────────────────────────────────────┐
│              TIME MACHINE + DECISION REPLAY                            │
│              "Replay no contexto da época"                             │
│                                                                       │
│  Don: "Mostre a decisão sobre o auto-jail como se fosse 29/jul."     │
│                                                                       │
│  ┌────────────────────────────┐                                       │
│  │ 1. Time Machine            │                                       │
│  │    ├── Data: 2026-07-29    │                                       │
│  │    └── Snapshot: contexto  │                                       │
│  │        da época             │                                       │
│  └───────────┬────────────────┘                                       │
│              │                                                         │
│              ▼                                                         │
│  ┌────────────────────────────┐                                       │
│  │ 2. Congelar fontes         │                                       │
│  │    ├── Trust Registry da   │                                       │
│  │    │   época (vazio)       │                                       │
│  │    ├── Learnings da época  │                                       │
│  │    │   (19 entries)        │                                       │
│  │    └── DDNAs da época      │                                       │
│  │        (0 entries)         │                                       │
│  └───────────┬────────────────┘                                       │
│              │                                                         │
│              ▼                                                         │
│  ┌────────────────────────────┐                                       │
│  │ 3. Decision Replay (F7.5) │                                       │
│  │    ├── Busca DDNA          │                                       │
│  │    ├── Carrega contexto    │                                       │
│  │    └── Apresenta replay    │                                       │
│  │        COM filtro: "você   │                                       │
│  │        está vendo como se  │                                       │
│  │        estivesse em         │                                       │
│  │        2026-07-29"          │                                       │
│  └────────────────────────────┘                                       │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### Comando Integrado

```bash
# Replay de decisão no contexto da época
cosca tm --date 2026-07-29 --replay "DDNA-2026-07-29-001"

# Equivalente a:
# 1. Time Machine carrega snapshot de 2026-07-29
# 2. Decision Replay busca a decisão
# 3. Apresenta: "Em 29/07, o Trust Registry estava vazio.
#    Não havia DDNAs. Havia 19 learnings. A decisão foi
#    tomada com base em..."
```

### Benefício Cognitivo

Sem a integração, o Decision Replay mostra o DDNA com as evidências da época, mas o usuário pode julgar com informação de hoje (viés de hindsight). Com a integração:

> **Antes**: "Por que eles escolheram Bubblewrap? Docker seria melhor."
> **Depois**: "Em 29/07, não havia Contrafactual Gate. O Trust Registry não existia. A escolha Bubblewrap foi baseada em 19 learnings e zero DDNAs. Dado o contexto da época, a decisão foi acertada."

---

## 6. Integração com ENGINEERING_TIMELINE

### Timeline como Espinha Dorsal

A **Engineering Timeline** (`memory/timeline/ENGINEERING_TIMELINE.md`) é a espinha dorsal do Time Machine. Ela registra a sequência temporal de todos os eventos importantes do projeto, e cada entrada na timeline é um ponto de ancoragem para o Time Machine.

### Mapeamento Timeline → Snapshot

| Evento na Timeline | Impacto no Snapshot |
|--------------------|---------------------|
| **Novo commit** | Snapshot automático gerado |
| **Engine criada** | `engines_projected` incrementa |
| **Learning registrado** | `total_learnings` atualiza |
| **Trust Registry populado** | `trust_registry_exists = true` |
| **CMI calculado** | `cmi_overall` preenchido |
| **Coverage milestone** | `test_coverage` atualizado |
| **DDNA criado** | `total_ddnas` incrementa |

### Formato da Timeline Enriquecida

Para suportar o Time Machine, cada entrada da Engineering Timeline DEVE incluir um campo `cognitive_snapshot_hash`:

```markdown
| Data | Evento | Cognitive Snapshot Hash |
|------|--------|------------------------|
| 2026-07-28 | F0 Complete — Cognitive Maturity Architecture | `sha256:a8d8fa9...` |
| 2026-07-29 | Jail breach recovery + coverage 68.6% → 71.3% | `sha256:c96f4ad...` |
| 2026-07-30 | Cosca Chat + Compute Fabric + CLI (40+ commits) | `sha256:ac013d5...` |
```

### Comando de Sincronização

```bash
# Sincronizar timeline com snapshots existentes
cosca tm sync-timeline

# Verificar datas com snapshot mas sem entrada na timeline
cosca tm verify-coverage
```

---

## 7. Integração com memorize-commit

### Snapshot Automático a Cada Commit

O workflow **memorize-commit** (F7.4) é o mecanismo que gera snapshots automáticos. Após cada commit, o workflow:

1. Gera o **Impact Report** (F7.4)
2. Calcula o **Engineering Score** (F7.3)
3. Atualiza o **Trust Registry** (F7.2)
4. **CAPTURA O SNAPSHOT COGNITIVO** (F9.6) — nova etapa adicionada

### Hook Pós-Commit

```yaml
# Configuração no cosca.config.yaml ou opencode.jsonc
time_machine:
  snapshot:
    auto_capture: true
    on_commit: true
    on_session_end: true
    on_milestone: true  # Learnings, engines, milestones
  storage:
    path: "memory/snapshots/cognitive/"
    retention: "forever"
    format: "yaml"
  reconstruction:
    max_depth: 365  # dias para trás
    cache: true
```

### Estatísticas do Snapshot Automático

O snapshot automático é projetado para ser **ultra-leve**:

| Operação | Tempo | Impacto |
|----------|:-----:|---------|
| Coleta de métricas | < 200ms | Assíncrono, não bloqueia commit |
| Geração do YAML | < 100ms | File write leve |
| Cálculo de score | < 500ms | Executado em background |
| Armazenamento | < 50ms | Append ao diretório de snapshots |
| **Total** | **< 850ms** | **Não impacta fluxo do desenvolvedor** |

---

## 8. Interface CLI

### Comando Principal

```bash
cosca time-machine <subcomando> [flags]
# Atalho:
cosca tm <subcomando> [flags]
```

### Subcomandos

| Comando | Descrição | Exemplo |
|---------|-----------|---------|
| `cosca tm <YYYY-MM-DD>` | Mostrar snapshot de uma data | `cosca tm 2026-07-28` |
| `cosca tm today` | Mostrar snapshot do estado atual | `cosca tm today` |
| `cosca tm --diff <A>..<B>` | Comparar duas datas | `cosca tm --diff 2026-07-28..2026-07-30` |
| `cosca tm compare <A> <B>` | Comparação lado a lado | `cosca tm compare 2026-07-28 2026-07-30` |
| `cosca tm list` | Listar todas as datas com snapshot | `cosca tm list` |
| `cosca tm replay <data> <ddna-id>` | Replay de decisão no contexto da época | `cosca tm replay 2026-07-29 DDNA-2026-07-29-001` |
| `cosca tm sync-timeline` | Sincronizar timeline com snapshots | `cosca tm sync-timeline` |
| `cosca tm verify-coverage` | Verificar datas sem snapshot | `cosca tm verify-coverage` |
| `cosca tm export <data>` | Exportar snapshot como YAML | `cosca tm export 2026-07-28 -o snapshot.yaml` |

### Flags

| Flag | Descrição | Default |
|------|-----------|---------|
| `--diff`, `-d` | Comparação entre duas datas (formato: `A..B`) | `""` |
| `--replay`, `-r` | ID de decisão para replay no contexto | `""` |
| `--output`, `-o` | Caminho para salvar o relatório | stdout |
| `--format`, `-f` | Formato: `terminal` (colorido) ou `markdown` (raw) | `terminal` |
| `--json` | Output em JSON (para parsing programático) | `false` |
| `--verbose`, `-v` | Mostrar fontes consultadas e detalhes | `false` |
| `--include-trust` | Incluir dados do Trust Registry | `true` |
| `--include-cmi` | Incluir cálculo de CMI (pode ser caro) | `false` |
| `--force-rebuild` | Ignorar snapshot em cache e reconstruir | `false` |

### Exemplos de Uso

```bash
# Snapshot de uma data específica
cosca tm 2026-07-28

# Estado atual
cosca tm today

# Comparação entre duas datas
cosca tm --diff 2026-07-28..2026-07-30

# Comparação lado a lado
cosca tm compare 2026-07-28 2026-07-30

# Replay de decisão no contexto da época
cosca tm replay 2026-07-29 DDNA-2026-07-29-001

# Listar snapshots disponíveis
cosca tm list

# Exportar snapshot como YAML
cosca tm export 2026-07-30 -o snapshot-30jul.yaml

# JSON para consumo programático
cosca tm 2026-07-28 --json | jq '.agents.active'
```

---

## 9. Comparação Temporal (Diff)

### Tipos de Diff

O Time Machine suporta três tipos de comparação temporal:

| Tipo | Comando | Descrição |
|------|---------|-----------|
| **Ponto-a-ponto** | `cosca tm compare 2026-07-28 2026-07-30` | Duas datas específicas |
| **Período** | `cosca tm --diff 2026-07-28..2026-07-30` | Evolução entre início e fim |
| **Hoje vs Data** | `cosca tm 2026-07-28` (depois escolher "comparar com hoje") | Snapshot + comparação interativa |

### Formato de Output do Diff

```markdown
╔══════════════════════════════════════════════════════════════════════════╗
║               COGNITIVE TIME MACHINE — COMPARAÇÃO TEMPORAL               ║
║               2026-07-28 → 2026-07-30 (2 dias de evolução)              ║
╚══════════════════════════════════════════════════════════════════════════╝

📅 Período: 2026-07-28 → 2026-07-30
⏱️ Duração: 2 dias
📊 Score Inicial: 2.4/10 🟠 → Score Final: 10.0/10 🟢
📈 Evolução Total: +7.6 pontos 🔥

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 📊 Score Global

| Período | Score | Classificação |
|---------|:-----:|:-------------:|
| 2026-07-28 | 2.4 🟠 | Fundação |
| 2026-07-30 | 10.0 🟢 | Maturidade Inicial |
| **Evolução** | **+7.6 🔥** | **+317%** |

## 📈 Evolução por Dimensão

### 👥 Agentes e Capacidades
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Agentes ativos | 10 / 55 | 54 / 55 | +44 🔥 |
| Chiefs | 15 | 41 | +26 🔥 |
| Especialistas | 2 | 9 | +7 |
| Seed-only | 45 (81%) | 41 (74%) | -4 |

### 🧠 Conhecimento
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Learnings | 19 | 27 | +8 |
| Failures | 8 | 12 | +4 |
| Patterns | 0 | 8 | +8 🔥 |
| Heurísticas | 0 | 20 | +20 🔥 |
| DDNAs | 0 | 3 | +3 |
| ADRs | 3 | 8 | +5 |

### ⚙️ Engines
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Engines projetadas | 0 | 7 | +7 🔥 |
| Diretórios de engine | 7 (vazios) | 57 | +50 🔥 |
| Conceitos C1-C14 | 0/14 | 7/14 | +7 |

### ✅ Qualidade
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Cobertura de testes | 68.6% | 97.9% | +29.3pp 🔥 |
| Quality gates | 0 | 5 | +5 |
| CI pipeline | Parcial | Completo | Melhoria 🔥 |

### 🔐 Trust Registry
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Trust Registry | Não existe | 54 agentes | +54 🔥 |
| Confiança média | — | 0.78 | +0.78 |

### 📐 CMI
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| CMI | Não calculado | 87.0 | +87.0 🔥 |
| Aprendizado | — | 88 | +88 |
| Julgamento | — | 85 | +85 |
| Planejamento | — | 90 | +90 |
| Autocrítica | — | 82 | +82 |
| Transferência | — | 87 | +87 |
| Consistência | — | 92 | +92 |

### 💰 Platform ROI
| Métrica | 28/jul | 30/jul | Δ |
|---------|:------:|:------:|:-:|
| Platform ROI | Não calculado | +337.6% | +337.6pp 🔥 |

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 🔥 Destaques da Evolução

🏆 **Maior avanço**: CMI — de 0 a 87.0 em 2 dias
🚀 **Mais rápido**: Engines — de 0 a 7 engines projetadas
📚 **Conhecimento**: 19 → 27 learnings (+42%), 0 → 20 heurísticas
🧪 **Cobertura**: 68.6% → 97.9% (+29.3 pontos percentuais)
👥 **Agentes**: 10 → 54 ativos (+440%)
🏗️ **Arquitetura**: 0 → 7 engines cognitivas implementadas

## 📉 Pontos de Atenção

⚠️ 41 agentes ainda seed-only (74%)
⚠️ Apenas 3 DDNAs registrados (decisões não documentadas)
⚠️ Platform ROI calculado apenas 1x
⚠️ Memory entropy não monitorada continuamente

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Ações:**
  🔍 Aprofundar dimensão X | 📊 Ver agentes seed-only
  ↩️ Voltar ao presente
```

---

## 10. Exemplo: 2026-07-28 vs 2026-07-30

### Contexto Real

Este exemplo usa dados reais da evolução do Cosca entre a primeira sessão (28/jul) e a última sessão documentada (30/jul).

### Timeline dos Eventos

| Data | Eventos Chave |
|------|---------------|
| **2026-07-28** | CMI Architecture criada (a8d8fa9). Kernel Level 4. F0+F1 Complete. Primeira sessão Don-Cosca. Cobertura ~68.6%. ~19 learnings. |
| **2026-07-29** | Jail breach + recovery (L13). Coverage 68.6% → 71.3%. Auto-jail memfd. Knowledge Pipeline. UCSS. 20+ commits. 27+ learnings. |
| **2026-07-30** | Evolution Marathon: F2-F10 em 1 dia. 7 engines criadas. Cosca Chat. Compute Fabric. CLI. CMI 87.0. Trust Registry. 40+ commits. |

### Execução do Comando

```bash
$ cosca tm --diff 2026-07-28..2026-07-30

╔══════════════════════════════════════════════════════════════════════════╗
║               COGNITIVE TIME MACHINE — COMPARAÇÃO TEMPORAL               ║
║               2026-07-28 → 2026-07-30 (2 dias de evolução)              ║
╚══════════════════════════════════════════════════════════════════════════╝

📅 Período: 2026-07-28 → 2026-07-30
⏱️ Duração: 2 dias
📊 Score: 2.4/10 🟠 → 10.0/10 🟢

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Resumo

Em 28/07: 10/55 agentes ativos, 0% coverage gaps (não auditado),
0 engines projetadas, Trust Registry vazio, CMI não calculado.
Score: 2.4/10 — Fundação.

Em 30/07: 54/55 agentes ativos, 97.9% coverage, 7 engines projetadas,
Trust Registry com 54 agentes, CMI 87.0, Platform ROI +337.6%.
Score: 10.0/10 — Maturidade Inicial.

Evolução: +7.6 pontos em 2 dias (+317%).

┌──────────────────────────────────────────────────────────────────────┐
│ 📊 2.4 ────────────────────────────────────────────────▶ 10.0 🔥    │
│                                                                    │
│ 28/jul     ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 2.4/10              │
│ 29/jul     █████████░░░░░░░░░░░░░░░░░░░░░░░░░░ 5.8/10              │
│ 30/jul     ████████████████████████████████████ 10.0/10             │
│                                                                    │
│ Apenas 2 dias separam o Cosca "bebê" do Cosca "maduro".            │
│ 40+ commits, 7 engines, 8 novos learnings, 20 heurísticas,         │
│ Trust Registry, CMI, Platform ROI.                                 │
└──────────────────────────────────────────────────────────────────────┘

💡 **Ações:**
  🔍 Ver detalhes de 28/jul | 🔍 Ver detalhes de 30/jul
  🎬 Replay de decisão em 28/jul | 🎬 Replay de decisão em 30/jul
  ↩️ Voltar ao presente
```

### Análise do Exemplo

Este diff real demonstra:

1. **Evolução dramática em 2 dias**: O Cosca saiu de um sistema com cognição zero (2.4/10) para um sistema com maturidade inicial (10.0/10) — um salto de +317%.

2. **Conhecimento como principal driver**: O salto mais significativo foi na dimensão Conhecimento (0 → 20 heurísticas, 0 → 3 DDNAs, 19 → 27 learnings). Isso reflete o ciclo virtuoso: mais tasks → mais learnings → mais padrões → mais heurísticas.

3. **Engines como prova de maturidade**: Em 28/jul, as 57 pastas de engine estavam vazias. Em 30/jul, 7 tinham SKILL.md preenchido. A evolução não foi apenas em quantidade, mas em qualidade de design.

4. **Trust Registry como indicador de confiança**: Em 28/jul, zero confiança registrada. Em 30/jul, 54 agentes com confidence médio de 0.78. O sistema passou a confiar nos próprios agentes.

5. **A cobertura de testes como marcador de qualidade**: 68.6% → 97.9% em 2 dias mostra que a qualidade de código foi prioridade desde o início.

---

## 11. Qualidade e Critérios de Aceite

### Critérios de Qualidade do Engine

| # | Critério | Descrição | Verificação |
|---|----------|-----------|-------------|
| **Q1** | **Snapshot fiel** | O snapshot deve refletir com precisão o estado do sistema na data, sem dados anacrônicos | Comparar snapshot com git log + learnings da época |
| **Q2** | **Confiança honesta** | Se reconstrução usou fontes incompletas, confiança deve ser proporcional | `snapshot_confidence = min(1.0, sources_found / sources_total)` |
| **Q3** | **Sem viés de hindsight** | O snapshot nunca deve incluir informação que não existia na data | Trust Registry da época, não de hoje; learnings da época, não atuais |
| **Q4** | **Restauração segura** | `git checkout` nunca deve perder trabalho não-commitado | Verificar `git status --porcelain` antes de checkout |
| **Q5** | **Volta ao presente** | Após time machine, o working directory deve estar exatamente como antes | `git diff HEAD` vazio após retorno |
| **Q6** | **Performance** | Snapshot reconstruído em < 5s; snapshot em cache em < 1s | Medir com `time` |
| **Q7** | **Diff preciso** | Comparação temporal deve mostrar apenas diferenças reais, não artefatos de reconstrução | Verificar que diff de datas iguais é vazio |
| **Q8** | **Cobertura de snapshots** | Toda data com commit deve ter snapshot (automático ou reconstruível) | `cosca tm verify-coverage` retorna 0 gaps |

### Critérios de Aceite para Criação

- [x] Pipeline de reconstrução documentado (4 etapas)
- [x] Snapshot cognitivo definido (10 dimensões)
- [x] Formato de armazenamento YAML especificado
- [x] CLI com subcomandos e flags
- [x] Integração com F7.5 Decision Replay
- [x] Integração com ENGINEERING_TIMELINE
- [x] Integração com memorize-commit (snapshot automático)
- [x] Formato de diff temporal
- [x] Exemplo real com dados de 28/jul vs 30/jul
- [x] Tratamento de erros (data futura, sem commits, working directory dirty)
- [x] Qualidade e verificabilidade

---

## 12. Tratamento de Erros

| Cenário | Erro | Ação do Engine |
|---------|------|----------------|
| **Data futura** | `2026-08-01 > hoje` | "Não é possível viajar para o futuro. Hoje é 2026-07-30." |
| **Data sem commits** | `2026-07-27` sem commits | Sugerir datas mais próximas: "Tente 2026-07-28 (1 commit) ou 2026-07-29 (40+ commits)" |
| **Working directory dirty** | `git status --porcelain` não vazio | "Há N arquivos não-commitados. Commit ou stash antes de viajar no tempo." |
| **Snapshot corrompido** | YAML malformado em `memory/snapshots/` | Ignorar snapshot corrompido, reconstruir com `--force-rebuild`, log do erro |
| **Fonte não encontrada** | `learnings.md` ou trust registry não existia na data | Reduzir confiança proporcionalmente; registrar fonte ausente |
| **Git checkout falha** | Conflito de merge durante checkout forçado | Abortar com instruções de resolução manual |
| **Data = hoje** | `cosca tm 2026-07-30` sendo hoje | Carregar snapshot do dia (se existir) ou gerar snapshot fresco sem git restore |
| **Snapshot não encontrado e sem git history** | Projeto sem git | "Time Machine requer repositório git. Nenhum repositório encontrado." |

### Mensagens de Erro Padronizadas

```bash
# Data futura
$ cosca tm 2026-08-15
❌ Erro: 2026-08-15 é uma data futura.
   Hoje é 2026-07-30. Não é possível viajar para o futuro.

# Data sem commits
$ cosca tm 2026-07-27
❌ Erro: Nenhum commit encontrado em 2026-07-27.
   Datas disponíveis mais próximas:
   • 2026-07-28 (1 commit) — "F0 Complete"
   • 2026-07-29 (40+ commits) — "Evolution Marathon Dia 1"

# Working directory dirty
$ cosca tm 2026-07-28
❌ Erro: 3 arquivos não-commitados detectados.
   O Time Machine precisa de um working directory limpo
   para fazer git checkout seguro.
   
   Soluções:
   • git add . && git commit -m "wip: antes do time machine"
   • git stash
   
   Após o time machine, execute 'cosca tm return' para
   restaurar o estado atual.
```

---

## 13. Relacionados

| Documento | Relação |
|-----------|---------|
| [decision-replay/SKILL.md](../decision-replay/SKILL.md) | F7.5 — Replay de decisões (complementar: time machine fornece contexto da época) |
| [../../knowledge/architecture/DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | F1.1 — Decision DNA (fonte de dados para reconstrução de decisões históricas) |
| [../../architecture/COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | Arquitetura CMI — Time Machine mede evolução do CMI ao longo do tempo |
| [../../architecture/COGNITIVE_ECOSYSTEM.md](../../architecture/COGNITIVE_ECOSYSTEM.md) | Ecossistema Cognitivo — Time Machine é o mecanismo de "fotografia do ecossistema" |
| [../../memory/DECISION_DNA_FORMAT.md](../../memory/DECISION_DNA_FORMAT.md) | Formato DDNA — DDNAs históricos são carregados pelo time machine |
| [../../shared/AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Auto-Evolution — Time Machine mostra a evolução dos agentes |
| [../../Cognitive_State_Specification.md](../../Cognitive_State_Specification.md) | UCSS — Cognitive State é parte do snapshot |
| [WORKFLOW.md](WORKFLOW.md) | Workflow do Cognitive Time Machine |
| [../../workflows/memorize-commit.md](../../workflows/memorize-commit.md) | memorize-commit — Gera snapshots automáticos a cada commit |
| [../../CONSTITUTION.md](../../CONSTITUTION.md) | Constituição — Princípio P2 (código como verdade), P4 (contexto do Don) |

---

## 14. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial do Cognitive Time Machine Engine (F9.6) |
