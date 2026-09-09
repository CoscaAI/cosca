# KNOWLEDGE FEDERATION ENGINE — Transferência de Conhecimento Cross-Project

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Semantic Memory Chief | **Criado**: 2026-07-30
>
> **Referência**: [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §5 C12 (Cognitive Ecosystem) e §4 A4 (Transferência de Conhecimento)
> **Workflow**: [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) — Tarefa F2.3
> **CMI Dimension**: Transferência (15% do CMI, target F2.3: +10)
> **Capacidade Alvo**: A4 — Transferência de Conhecimento cross-project

---

## 1. Propósito

O **Knowledge Federation Engine** implementa a federação de memória semântica entre múltiplos projetos Cosca. Enquanto o Semantic Memory Engine (`internal/embed/cosca/engines/semantic-memory/SKILL.md`) indexa e recupera conhecimento **dentro** de um projeto, o Federation Engine propaga padrões, heurísticas, failures e aprendizados **entre** projetos — mesmo quando os domínios de aplicação são completamente diferentes.

Hoje, os 26 aprendizados do Kernel de um projeto não ajudam outro Kernel em outro projeto. As 20 heurísticas extraídas em `knowledge/heuristics/` vivem isoladas no projeto onde foram descobertas. Um padrão como "cross-agent parallel audit" descoberto em cosca-core não é sugerido quando um projeto Django enfrenta o mesmo problema estrutural.

A federação resolve isso. O Don definiu transferência de conhecimento como:

> **"Aprender um padrão em um projeto e reconhecer quando ele pode ser aplicado em outro, mesmo que os domínios sejam diferentes."**

Este engine é o mecanismo que torna essa visão operacional.

### 1.1 O Problema

```
┌──────────────────────────────────────────────────────────────────┐
│                    CONHECIMENTO ISOLADO (HOJE)                    │
│                                                                   │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐          │
│  │  Projeto A   │   │  Projeto B   │   │  Projeto C   │          │
│  │  ─────────   │   │  ─────────   │   │  ─────────   │          │
│  │  26 learnings│   │  0 learnings │   │  0 learnings │          │
│  │  20 heurist. │   │  0 heurist.  │   │  0 heurist.  │          │
│  │  8 clusters  │   │  0 clusters  │   │  0 clusters  │          │
│  │  ─────────   │   │  ─────────   │   │  ─────────   │          │
│  │  Kernel B    │   │  Kernel B    │   │  Kernel B    │          │
│  │  começa do   │   │  começa do   │   │  começa do   │          │
│  │  ZERO        │   │  ZERO        │   │  ZERO        │          │
│  └──────────────┘   └──────────────┘   └──────────────┘          │
│                                                                   │
│  CADA PROJETO REDESCUBRE PADRÕES QUE OUTROS JÁ DOMINARAM.         │
│  AS 20 HEURÍSTICAS DO PROJETO A NÃO PROTEGEM O PROJETO B.         │
│  OS FAILURES DO PROJETO A NÃO EVITAM OS MESMOS ERROS NO C.        │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 A Solução

```
┌──────────────────────────────────────────────────────────────────┐
│                    CONHECIMENTO FEDERADO (ALVO)                   │
│                                                                   │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐          │
│  │  Projeto A   │   │  Projeto B   │   │  Projeto C   │          │
│  │  (cosca-core)│   │  (Django app)│   │  (React SPA) │          │
│  │  ─────────   │   │  ─────────   │   │  ─────────   │          │
│  │  Produz      │   │  Consome     │   │  Consome     │          │
│  │  conhecimento│   │  heurísticas │   │  padrões     │          │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘          │
│         │                  │                  │                   │
│         └──────────────────┼──────────────────┘                   │
│                            │                                      │
│                   ┌────────▼─────────┐                            │
│                   │ FEDERATION HUB   │                            │
│                   │                  │                            │
│                   │ • Índice vetorial│                            │
│                   │ • Cross-refs     │                            │
│                   │ • Proveniência   │                            │
│                   │ • Deduplicação   │                            │
│                   └────────┬─────────┘                            │
│                            │                                      │
│              ┌─────────────┼─────────────┐                        │
│              ▼             ▼             ▼                        │
│         Pattern        Failure       Heurística                    │
│         Matching       Avoidance     Transfer                      │
│                                                                   │
│  PROJETO B RECEBE PADRÕES QUE O PROJETO A JÁ VALIDOU.             │
│  PROJETO C É AVISADO ANTES DE COMETER O MESMO ERRO DO A.          │
│  26 LEARNINGS SE TORNAM PATRIMÔNIO DE TODOS OS PROJETOS.          │
└──────────────────────────────────────────────────────────────────┘
```

---

## 2. Ativação

O Federation Engine é ativado nos seguintes contextos:

| Gatilho | Momento | Ação |
|---------|---------|------|
| **Kernel startup** | Bootstrap do projeto | Verifica sincronização com o Federation Hub, carrega assinaturas de conhecimento federado |
| **Stage 1 (SELF-ASSESS)** | Antes de cada task | Consulta o Hub por failures similares no domínio da task (Failure Avoidance) |
| **Stage 2 (RETRIEVE MEMORY)** | Antes de cada task | Busca semântica cross-project por padrões que deram match com a assinatura da task (Pattern Matching) |
| **Stage 3 (PLAN STRATEGY)** | Planejamento da task | Sugere heurísticas federadas com alta gravidade no domínio (Heuristic Transfer) |
| **Stage 7 (EXTRACT PATTERN)** | Pós-task | Se novo padrão descoberto, gera assinatura e publica no Hub |
| **Stage 8 (UPDATE CAPABILITY)** | Pós-task | Atualiza métrica B4 expandida (cross-project knowledge reuse) |
| **Periodicamente** | A cada 6 horas | Sincronização completa: pull de novo conhecimento federado, push de conhecimento local novo, deduplicação |
| **Sob demanda** | Comando explícito | `cosca federation sync --project {nome}` ou `cosca federation search --query "..."` |

---

## 3. Arquitetura da Federação

### 3.1 Visão Sistêmica

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                      KNOWLEDGE FEDERATION ARCHITECTURE                         │
│                                                                               │
│  ┌───────────────────────┐  ┌───────────────────────┐  ┌────────────────────┐ │
│  │     PROJETO A          │  │     PROJETO B          │  │    PROJETO C       │ │
│  │                        │  │                        │  │                    │ │
│  │ ┌────────────────────┐ │  │ ┌────────────────────┐ │  │ ┌────────────────┐ │ │
│  │ │ Federal Adapter    │ │  │ │ Federal Adapter    │ │  │ │Federal Adapter │ │ │
│  │ │ ───────────────    │ │  │ │ ───────────────    │ │  │ │─────────────── │ │ │
│  │ │ • Exporta padrões  │ │  │ │ • Importa padrões  │ │  │ │• Consome       │ │ │
│  │ │ • Assina knowledge │ │  │ │ • Match signatures │ │  │ │  heurísticas   │ │ │
│  │ │ • Publica failures │ │  │ │ • Recebe warnings  │ │  │ │• Publica       │ │ │
│  │ │ • Consome warnings │ │  │ │ • Publica failures │ │  │ │  failures      │ │ │
│  │ └────────┬───────────┘ │  │ └────────┬───────────┘ │  │ └───────┬────────┘ │ │
│  │          │              │  │          │              │  │         │          │ │
│  │ ┌────────▼───────────┐ │  │ ┌────────▼───────────┐ │  │ ┌───────▼────────┐ │ │
│  │ │ Semantic Memory    │ │  │ │ Semantic Memory    │ │  │ │Semantic Memory │ │ │
│  │ │ Engine (local)     │ │  │ │ Engine (local)     │ │  │ │Engine (local)  │ │ │
│  │ └────────────────────┘ │  │ └────────────────────┘ │  │ └────────────────┘ │ │
│  └───────────┬────────────┘  └───────────┬────────────┘  └─────────┬──────────┘ │
│              │                           │                           │            │
│              └───────────────────────────┼───────────────────────────┘            │
│                                          │                                        │
│                              ┌───────────▼───────────┐                            │
│                              │   FEDERATION HUB       │                            │
│                              │                        │                            │
│                              │  ┌──────────────────┐  │                            │
│                              │  │ Vector Index      │  │                            │
│                              │  │ (cosine similarity│  │                            │
│                              │  │  cross-project)   │  │                            │
│                              │  └──────────────────┘  │                            │
│                              │                        │                            │
│                              │  ┌──────────────────┐  │                            │
│                              │  │ Signature Store   │  │                            │
│                              │  │ (knowledge        │  │                            │
│                              │  │  fingerprints)    │  │                            │
│                              │  └──────────────────┘  │                            │
│                              │                        │                            │
│                              │  ┌──────────────────┐  │                            │
│                              │  │ Provenance Chain  │  │                            │
│                              │  │ (origem +         │  │                            │
│                              │  │  validações)      │  │                            │
│                              │  └──────────────────┘  │                            │
│                              │                        │                            │
│                              │  ┌──────────────────┐  │                            │
│                              │  │ Dedup Engine      │  │                            │
│                              │  │ (similarity > 0.85│  │                            │
│                              │  │  → merge)         │  │                            │
│                              │  └──────────────────┘  │                            │
│                              │                        │                            │
│                              │  ┌──────────────────┐  │                            │
│                              │  │ Privacy Filter    │  │                            │
│                              │  │ (public/private/  │  │                            │
│                              │  │  namespace)       │  │                            │
│                              │  └──────────────────┘  │                            │
│                              └────────────────────────┘                            │
│                                          │                                        │
│                              ┌───────────▼───────────┐                            │
│                              │   STORAGE              │                            │
│                              │   $HOME/.cosca/        │                            │
│                              │   federation/          │                            │
│                              │   ├── vectors.db       │                            │
│                              │   ├── signatures.yaml  │                            │
│                              │   ├── provenance/      │                            │
│                              │   └── sync-log.jsonl   │                            │
│                              └────────────────────────┘                            │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Fluxo de Dados

```
┌──────────────────────────────────────────────────────────────────┐
│                    CICLO DE VIDA DO CONHECIMENTO                  │
│                                                                   │
│  1. ORIGEM                                                        │
│     Agente no Projeto A executa task → Stage 7 extrai padrão      │
│                         │                                         │
│                         ▼                                         │
│  2. ASSINATURA                                                    │
│     Federation Adapter gera signature YAML com domínio,            │
│     precondições, pattern_type, applicability_score               │
│                         │                                         │
│                         ▼                                         │
│  3. PUBLICAÇÃO                                                    │
│     Adapter publica no Hub: embedding da assinatura + metadados    │
│     + proveniência (projeto de origem, timestamp, validações)     │
│                         │                                         │
│                         ▼                                         │
│  4. INDEXAÇÃO                                                     │
│     Hub gera embedding da assinatura, armazena no vector index     │
│     cross-project. Deduplication Engine verifica similaridade     │
│     com conhecimento existente (threshold: 0.85).                 │
│                         │                                         │
│                         ▼                                         │
│  5. DISPONIBILIZAÇÃO                                              │
│     Conhecimento federado fica disponível para todos os projetos   │
│     que sincronizarem com o Hub. Privacy Filter aplica regras      │
│     de visibilidade (public/namespace/private).                    │
│                         │                                         │
│                         ▼                                         │
│  6. CONSUMO                                                        │
│     Projeto B, durante Stage 1-3 do metacognition pipeline,        │
│     consulta o Hub: "existe padrão similar a esta task?"           │
│     Hub retorna matches com score de similaridade > threshold.     │
│                         │                                         │
│                         ▼                                         │
│  7. VALIDAÇÃO CRUZADA                                             │
│     Projeto B aplica o padrão → sucesso → incrementa               │
│     validation_count na proveniência. Failure → decrementa.        │
│     Quanto mais projetos validam, maior a massa gravitacional.     │
│                         │                                         │
│                         ▼                                         │
│  8. EVOLUÇÃO                                                      │
│     Padrão federado evolui com contribuições de múltiplos          │
│     projetos. Pattern Evolution (F2.6) versiona cross-project.    │
│     Cognitive Gravity (F2.4) aumenta com validações distribuídas. │
└──────────────────────────────────────────────────────────────────┘
```

---

## 4. Mecanismos de Transferência

O Federation Engine implementa três mecanismos de transferência que correspondem aos três tipos de conhecimento que fluem entre projetos:

### 4.1 Pattern Matching (Correspondência de Padrões)

**Objetivo**: Quando uma task no Projeto B tem assinatura similar a uma task já resolvida no Projeto A, sugerir o padrão que funcionou.

**Como funciona**:

1. **No momento da task** (Stage 2: RETRIEVE MEMORY), o Federation Adapter do Projeto B gera uma "task signature" contendo:
   - Domínio da task (ex: `refactoring`, `security`, `testing`)
   - Tipo de operação (ex: `extract_function`, `add_validation`, `parallelize`)
   - Pré-condições detectadas (ex: `monolith > 500 linhas`, `test_coverage < 30%`)
   - Stack tecnológico (ex: `go`, `python`, `react`)

2. **Query ao Hub**: O adapter consulta o vector index do Hub com o embedding da task signature.

3. **Match**: O Hub retorna padrões com similaridade de assinatura > threshold (padrão: 0.60).

4. **Sugestão**: O Kernel recebe os padrões federados e os apresenta ao agente como "padrão sugerido por projeto similar" com proveniência e score de aplicabilidade.

5. **Adaptação**: O agente adapta o padrão ao domínio específico do Projeto B. A adaptação é registrada como "aplicação cross-project" para rastreabilidade.

```
┌──────────────────────────────────────────────────────────────────┐
│                    PATTERN MATCHING FLOW                          │
│                                                                   │
│  PROJETO A (cosca-core, Go)          PROJETO B (Django, Python)   │
│  ────────────────────────            ─────────────────────────    │
│                                                                   │
│  Task: "Extrair função de           Task: "Extrair lógica de      │
│         validação do handler"               validação da view"    │
│                                                                   │
│  Signature:                         Signature:                    │
│    domain: refactoring                domain: refactoring         │
│    pattern_type: extract-then-test    pattern_type: extract-...   │
│    preconditions:                     preconditions:              │
│      - function > 200 lines            - view > 150 lines         │
│      - test_cov < 20%                 - test_cov < 25%            │
│    stack: go                          stack: python               │
│                                                                   │
│  ┌──────────────┐                  ┌──────────────┐               │
│  │  Publica     │                  │  Consulta    │               │
│  │  assinatura  │──── HUB ────────▶│  "match?"    │               │
│  └──────────────┘                  └──────┬───────┘               │
│                                          │                        │
│                                   ┌──────▼───────┐               │
│                                   │ MATCH 0.78   │               │
│                                   │ Padrão:       │               │
│                                   │ extract-then- │               │
│                                   │ test (v3)     │               │
│                                   │ Projeto: A    │               │
│                                   │ Validado: 3x  │               │
│                                   └──────┬───────┘               │
│                                          │                        │
│                                   ┌──────▼───────┐               │
│                                   │ Kernel sugere │               │
│                                   │ ao agente:    │               │
│                                   │ "Projeto A    │               │
│                                   │  usou este    │               │
│                                   │  padrão em    │               │
│                                   │  contexto     │               │
│                                   │  similar."    │               │
│                                   └──────────────┘               │
└──────────────────────────────────────────────────────────────────┘
```

**Algoritmo de Matching**:

```yaml
pattern_matching:
  embedding_model: "text-embedding-3-small"  # via Provider Chief
  similarity_metric: "cosine"
  thresholds:
    exact_match: 0.90    # Mesmo domínio, mesma stack → adaptação trivial
    strong_match: 0.75   # Mesmo domínio, stack diferente → adaptação moderada
    weak_match: 0.60     # Domínio diferente, padrão similar → adaptação criativa
    no_match: "< 0.60"  # Sem similaridade suficiente → não sugerir
  ranking:
    - similarity_score × 0.60
    - applicability_score × 0.25
    - validation_count × 0.10
    - recency × 0.05
  top_k: 5
```

### 4.2 Failure Avoidance (Evitação de Falhas)

**Objetivo**: Quando o Projeto B está prestes a executar uma operação cuja assinatura corresponde a um failure registrado no Projeto A, emitir um warning antes da execução — não depois.

**Como funciona**:

1. **No Stage 1 (SELF-ASSESS)** e **Stage 3 (PLAN STRATEGY)**, o Federation Adapter verifica o plano de execução contra o índice de failures federados.

2. **Se match**: O Kernel recebe um `FederationWarning` com:
   - Descrição do failure original (anonimizado — sem detalhes do projeto de origem se private)
   - Causa raiz
   - Condições que levaram ao failure
   - Mitigação recomendada
   - Projetos que validaram o warning (cross-validation)

3. **Ação**: O Kernel bloqueia a execução até que o agente reconheça o warning e adapte a estratégia OU confirme que o risco é aceitável com justificativa documentada.

```
┌──────────────────────────────────────────────────────────────────┐
│                    FAILURE AVOIDANCE FLOW                         │
│                                                                   │
│  PROJETO A (cosca-core)               PROJETO B (outro projeto)   │
│  ─────────────────────               ─────────────────────────   │
│                                                                   │
│  Failure F-003:                      Task: "Executar operação     │
│  "Jail bypass via init --force              de init no projeto"   │
│   sem DRY_RUN prévio"                                              │
│                                                                   │
│  Signature:                          ┌──────────────────────┐     │
│    domain: security                  │ Stage 1: SELF-ASSESS │     │
│    failure_type: jail_breach         │ ───────────────────  │     │
│    preconditions:                    │ Federation Adapter   │     │
│      - destructive_operation         │ consulta Hub:        │     │
│      - no_dry_run                    │ "Algum failure       │     │
│      - elevated_permissions          │  similar a esta      │     │
│    root_cause: "O Kernel não é       │  operação?"          │     │
│      um agente que usa ferramentas.  └──────────┬───────────┘     │
│      O Kernel É a configuração."               │                  │
│  ┌──────────────┐                     ┌─────────▼───────────┐     │
│  │  Publica     │                     │ MATCH 0.92           │     │
│  │  failure     │────── HUB ────────▶│ Failure F-003        │     │
│  │  signature   │                     │ Tipo: jail_breach    │     │
│  └──────────────┘                     │ ⚠️ BLOQUEAR ATÉ      │     │
│                                       │ CONFIRMAÇÃO           │     │
│                                       └─────────┬───────────┘     │
│                                                 │                 │
│                                       ┌─────────▼───────────┐     │
│                                       │ Kernel exibe warning │     │
│                                       │ ao agente:           │     │
│                                       │ "Projeto A sofreu    │     │
│                                       │  jail breach similar.│     │
│                                       │  Causa raiz: ...     │     │
│                                       │  Mitigação: executar │     │
│                                       │  DRY_RUN primeiro."  │     │
│                                       └─────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

**Níveis de Severidade do Warning**:

| Severidade | Condição | Ação Automática |
|------------|----------|-----------------|
| **BLOCKING** | Mesma assinatura de failure, mesmas precondições | Bloqueia execução até confirmação do agente |
| **HIGH** | Assinatura similar (>0.80), precondições parciais | Exibe warning proeminente, registra no log |
| **MEDIUM** | Domínio similar, failure type diferente | Exibe warning informativo, não bloqueia |
| **LOW** | Similaridade marginal (>0.50, <0.65) | Registra no log, não interrompe fluxo |

### 4.3 Heuristic Transfer (Transferência de Heurísticas)

**Objetivo**: Heurísticas independentes de domínio (ex: "sempre execute DRY_RUN antes de operações destrutivas", "extraia antes de testar") são propagadas automaticamente para todos os projetos federados, sem necessidade de matching explícito.

**Como funciona**:

1. **Heurística "universal" detection**: Quando uma heurística atinge `applicability_score >= 0.85` em ≥ 3 projetos diferentes, ela é promovida a "heurística universal".

2. **Propagação automática**: Heurísticas universais são injetadas no contexto de **todos** os projetos federados durante o bootstrap (Stage 0). Elas não precisam ser buscadas — já estão lá.

3. **Background enforcement**: O Federation Adapter verifica periodicamente se as heurísticas universais estão sendo respeitadas. Violações geram warnings automáticos.

**Exemplos de Heurísticas Universais**:

```yaml
heuristic_transfer_examples:
  - heuristic_id: "H-UNIV-001"
    title: "DRY_RUN antes de operações destrutivas"
    source: "H-003 (cosca-core, jail breach)"
    validated_in: ["cosca-core", "django-cosca-app", "react-cosca-dashboard"]
    applicability_score: 0.95
    propagation: "universal"

  - heuristic_id: "H-UNIV-002"
    title: "Extrair função antes de testar (extract-then-test)"
    source: "F0.2 (cosca-core, refactoring pattern)"
    validated_in: ["cosca-core", "python-ml-pipeline", "go-microservices"]
    applicability_score: 0.90
    propagation: "universal"

  - heuristic_id: "H-UNIV-003"
    title: "Verificar documentação contra código fonte (cross-source audit)"
    source: "H-009 (cosca-core, PostgreSQL fantasy detection)"
    validated_in: ["cosca-core", "enterprise-api", "mobile-app"]
    applicability_score: 0.88
    propagation: "universal"

  - heuristic_id: "H-UNIV-004"
    title: "Nunca reivindicar capacidade não verificada (capability honesty)"
    source: "H-010 (cosca-core, memory fiction pattern)"
    validated_in: ["cosca-core", "cosca-test", "open-source-lib"]
    applicability_score: 0.92
    propagation: "universal"
```

---

## 5. Knowledge Signature (Assinatura de Conhecimento)

Toda entrada de conhecimento exportada para a federação recebe uma **Knowledge Signature** — uma estrutura YAML que permite matching cross-project independente de domínio, stack ou nomenclatura específica.

### 5.1 Formato Canônico

```yaml
signature:
  # === IDENTIFICAÇÃO ===
  id: "KSIG-2026-07-30-001"            # Knowledge Signature ID único global
  source_signature:                     # Origem do conhecimento
    project: "cosca-core"               # Projeto de origem (namespace)
    project_id: "a1b2c3d4"             # UUID do projeto (imutável)
    agent: "cosca-kernel"              # Agente que gerou o conhecimento
    learning_id: "L22"                 # ID do learning/task de origem
    timestamp: "2026-07-30T14:00:00Z"  # Quando foi gerado

  # === CLASSIFICAÇÃO SEMÂNTICA ===
  domain: "security"                    # Domínio primário (lowercase, snake_case)
  subdomain: "jail_isolation"           # Subdomínio específico
  domains_cross:                        # Domínios secundários (para matching amplo)
    - "runtime"
    - "configuration"
    - "orchestration"

  # === TIPO DE CONHECIMENTO ===
  knowledge_type: "failure"             # pattern | failure | heuristic | principle | discovery
  pattern_type: "extract-then-test"     # Tipo específico do padrão (se knowledge_type=pattern)
  failure_type: "jail_breach"           # Tipo específico de failure (se knowledge_type=failure)
  heuristic_type: "universal"           # domain_specific | universal (se knowledge_type=heuristic)

  # === PRÉ-CONDIÇÕES DE APLICABILIDADE ===
  preconditions:
    structural:                         # Condições estruturais do código/problema
      - "função monolítica > 100 linhas"
      - "cobertura de teste < 20%"
      - "múltiplas responsabilidades na mesma unidade"
    contextual:                         # Condições contextuais da task
      - "task de refatoração"
      - "domínio com confidence < 0.5"
      - "primeira interação com o módulo"
    environmental:                      # Condições do ambiente
      - "projeto sem CI/CD"
      - "sem testes de regressão automatizados"
    stack_agnostic: true                # true = aplicável a qualquer stack

  # === MÉTRICAS DE APLICABILIDADE ===
  applicability_score: 0.85             # 0-1: probabilidade de ser útil cross-domain
  domain_coupling: 0.30                 # 0-1: quão acoplado ao domínio original (0 = universal)
  abstraction_level: 4                  # 1-5: 1=concreto/stack-specific, 5=princípio abstrato
  transfer_readiness: 0.82              # 0-1: combinação de applicability + (1 - domain_coupling)

  # === PROVENIÊNCIA E VALIDAÇÃO ===
  provenances:
    - project: "cosca-core"
      project_id: "a1b2c3d4"
      role: "origem"                    # origem | validador | contestador
      validations: 3                    # Quantas vezes validado neste projeto
      last_validated: "2026-07-30"
      success_rate: 1.0                 # Taxa de sucesso nas aplicações
    - project: "django-cosca-app"
      project_id: "e5f6g7h8"
      role: "validador"
      validations: 2
      last_validated: "2026-08-15"
      success_rate: 1.0
    - project: "react-cosca-dashboard"
      project_id: "i9j0k1l2"
      role: "validador"
      validations: 1
      last_validated: "2026-09-01"
      success_rate: 0.5                 # Funcionou parcialmente — adaptação necessária

  # === PRIVACIDADE E COMPARTILHAMENTO ===
  visibility: "public"                  # public | namespace | private
  namespace: "cosca-official"           # Se visibility=namespace, qual namespace
  share_pattern: true                   # Compartilhar o padrão em si
  share_context: false                  # Compartilhar contexto específico do projeto
  share_provenance: true                # Compartilhar dados de proveniência
  anonymize_level: "project_name"       # none | project_name | full_anonymize

  # === EVOLUÇÃO E VERSIONAMENTO ===
  version: "v3"                         # Versão atual da assinatura
  supersedes: ["KSIG-2026-07-28-002"]  # Assinaturas que esta substitui
  superseded_by: null                   # Se obsoleta, qual a substituta
  status: "active"                      # active | deprecated | superseded | contested
  gravity_score: 7.4                    # Massa gravitacional (C1 — Cognitive Gravity)

  # === META-HEURÍSTICAS ===
  related_signatures:                   # Assinaturas semanticamente relacionadas
    - "KSIG-2026-07-28-005"            # Relacionamento forte
    - "KSIG-2026-07-29-012"            # Relacionamento fraco
  anti_patterns:                        # Padrões que CONTRADIZEM este conhecimento
    - "KSIG-2026-07-15-003"
  contradiction_threshold: 0.75         # Similarity acima disso = contradição (Immune System)

  # === EMBEDDING ===
  embedding:
    model: "text-embedding-3-small"
    dimensions: 1536
    vector_id: "vec_a1b2c3d4e5f6"      # ID no vector index do Hub
    last_embedded: "2026-07-30T14:05:00Z"
```

### 5.2 Geração da Assinatura

A assinatura é gerada automaticamente pelo Federation Adapter durante o **Stage 7 (EXTRACT PATTERN)** do metacognition pipeline:

```
┌──────────────────────────────────────────────────────────────────┐
│                    SIGNATURE GENERATION PIPELINE                  │
│                                                                   │
│  Learning/Pattern/Failure extraído no Stage 7                     │
│       │                                                           │
│       ▼                                                           │
│  ┌──────────────────────┐                                         │
│  │ 1. CLASSIFY           │  Classifica: domain, subdomain,        │
│  │    ─────────          │  knowledge_type, pattern_type          │
│  │    Usa NLP + regras   │  (usa vocabulário controlado)          │
│  └──────────┬───────────┘                                         │
│             │                                                     │
│             ▼                                                     │
│  ┌──────────────────────┐                                         │
│  │ 2. EXTRACT            │  Extrai: preconditions (structural,    │
│  │    PRECONDITIONS      │  contextual, environmental),            │
│  │    ─────────────      │  stack_agnostic flag                   │
│  └──────────┬───────────┘                                         │
│             │                                                     │
│             ▼                                                     │
│  ┌──────────────────────┐                                         │
│  │ 3. SCORE              │  Calcula: applicability_score,         │
│  │    APPLICABILITY      │  domain_coupling, abstraction_level,   │
│  │    ────────────       │  transfer_readiness                   │
│  └──────────┬───────────┘                                         │
│             │                                                     │
│             ▼                                                     │
│  ┌──────────────────────┐                                         │
│  │ 4. GENERATE           │  Gera: embedding via Provider Chief,   │
│  │    EMBEDDING          │  armazena no vector index do Hub       │
│  └──────────┬───────────┘                                         │
│             │                                                     │
│             ▼                                                     │
│  ┌──────────────────────┐                                         │
│  │ 5. PUBLISH            │  Publica no Hub com: assinatura +      │
│  │    TO HUB             │  embedding + proveniência inicial      │
│  └──────────────────────┘                                         │
│                                                                   │
│  OUTPUT: Knowledge Signature pronta para consumo cross-project.   │
└──────────────────────────────────────────────────────────────────┘
```

### 5.3 Vocabulário Controlado — Domínios

Para garantir matching cross-project consistente, o engine usa um vocabulário controlado de domínios:

```yaml
controlled_vocabulary:
  domains:
    - security
    - testing
    - refactoring
    - performance
    - architecture
    - documentation
    - orchestration
    - memory
    - governance
    - ci_cd
    - deployment
    - monitoring
    - api_design
    - database
    - frontend
    - backend
    - mobile
    - devops
    - compliance
    - observability
    - integration
    - migration
    - runtime
    - configuration
    - dependency_management

  knowledge_types:
    - pattern          # Solução reutilizável
    - failure          # Algo que deu errado (negative memory)
    - heuristic        # Regra de decisão (if X then Y)
    - principle        # Verdade universal extraída de N casos (Cognitive Compression)
    - discovery        # Insight proativo (Insight Generator)
    - antipattern      # Padrão a ser EVITADO

  pattern_types:
    - extract_then_test        # Extrair lógica → testar isoladamente
    - cross_agent_audit        # Auditoria com múltiplos agentes em paralelo
    - progressive_activation   # Ativação de agentes em ondas
    - dry_run_first            # Simular antes de executar
    - verify_then_claim        # Verificar antes de documentar
    - isolate_and_contract     # Isolar módulo → definir contrato → implementar
    - observe_before_optimize  # Medir antes de otimizar
    - decompose_and_delegate   # Decompor problema → delegar para especialistas

  failure_types:
    - jail_breach              # Bypass de isolamento/sandbox
    - documentation_fiction    # Documentação descreve funcionalidade inexistente
    - threshold_conflict       # Múltiplos valores conflitantes para mesma configuração
    - capability_overclaim     # Agente reivindica capacidade não verificada
    - stale_reference          # Referência a arquivo/caminho que não existe mais
    - cascade_failure          # Falha em um componente derruba dependentes
    - silent_corruption        # Dado corrompido sem detecção
    - permission_escalation    # Operação com mais permissão que o necessário
```

---

## 6. Federation Hub

O **Federation Hub** é o ponto central de armazenamento, indexação e distribuição do conhecimento federado.

### 6.1 Localização

O Hub reside em storage **project-agnostic**, fora de qualquer projeto individual:

```
$HOME/.cosca/federation/
├── README.md                  # Documentação do Hub
├── config.yaml                # Configuração global da federação
├── vectors.db                 # SQLite com FTS5 + vector index (todas as assinaturas)
├── signatures/                # Assinaturas YAML organizadas por namespace
│   ├── cosca-official/
│   │   ├── KSIG-2026-07-30-001.yaml
│   │   ├── KSIG-2026-07-28-005.yaml
│   │   └── ...
│   ├── enterprise-acme/
│   │   └── KSIG-2026-08-01-001.yaml
│   └── personal-don/
│       └── KSIG-2026-08-05-001.yaml
├── provenance/                # Cadeia de proveniência por assinatura
│   └── KSIG-2026-07-30-001/
│       ├── validations.jsonl
│       └── evolution.yaml
├── embeddings-cache/          # Cache local de embeddings (LRU, 1000 entradas)
├── sync-log.jsonl             # Log de sincronização (timestamp, projeto, operação, status)
└── dedup/                     # Registro de deduplicações
    └── merges.jsonl
```

### 6.2 Sincronização

A sincronização entre projetos e o Hub segue três modelos:

#### 6.2.1 Sync Automático (Recomendado)

```
┌──────────────────────────────────────────────────────────────────┐
│                    SYNC AUTOMÁTICO                                │
│                                                                   │
│  Trigger:                                                         │
│    • A cada 6 horas (background cron no Federation Adapter)       │
│    • Ao final de cada task que gerou novo conhecimento (Stage 7)  │
│    • No bootstrap do projeto (Stage 0 — verificar novidades)      │
│                                                                   │
│  Operação:                                                        │
│    PULL:  Baixa assinaturas novas/atualizadas do Hub              │
│           → Heurísticas universais injetadas no contexto          │
│           → Failures novos registrados no alert index             │
│           → Padrões novos disponíveis para matching               │
│                                                                   │
│    PUSH:  Sobe assinaturas locais novas/atualizadas para o Hub    │
│           → Knowledge signatures geradas no Stage 7               │
│           → Validações de padrões federados (incrementa proven.)  │
│           → Contestações de padrões (decrementa proven.)          │
│                                                                   │
│    MERGE: Resolve conflitos (timestamp-based, last-write-wins     │
│           para metadados, merge para proveniência)                │
└──────────────────────────────────────────────────────────────────┘
```

#### 6.2.2 Sync On-Commit

Alternativa para projetos com Git: sincronizar no `post-commit` hook.

```bash
# .git/hooks/post-commit
cosca federation sync --project $(basename $(pwd)) --mode push
```

#### 6.2.3 Sync Manual

Sob demanda, via CLI:

```bash
# Push: publicar conhecimento local no Hub
cosca federation push --project cosca-core --signatures recent

# Pull: buscar conhecimento federado de outros projetos
cosca federation pull --project cosca-test --since 2026-08-01

# Sync completo bidirecional
cosca federation sync --project cosca-core

# Status: verificar estado da sincronização
cosca federation status --project cosca-core
```

### 6.3 Privacidade

Nem todo conhecimento deve ser compartilhado. O engine implementa um modelo de privacidade em três níveis:

```yaml
privacy_model:
  levels:
    public:
      description: "Conhecimento disponível para todos os projetos federados"
      applies_to:
        - patterns              # Padrões de solução (sempre public)
        - heuristics            # Heurísticas de decisão (sempre public)
        - principles            # Princípios universais (sempre public)
        - antipatterns          # Anti-padrões (sempre public)
      restrictions: none

    namespace:
      description: "Conhecimento compartilhado apenas dentro de um namespace (ex: mesma organização)"
      applies_to:
        - failures              # Failures (padrão: namespace)
        - discoveries           # Descobertas (padrão: namespace)
      restrictions:
        - "Apenas projetos no mesmo namespace podem acessar"
        - "Namespace definido em $HOME/.cosca/federation/config.yaml"

    private:
      description: "Conhecimento que NUNCA é federado"
      applies_to:
        - project_specific_decisions  # Decisões específicas do projeto
        - credentials                  # Credenciais, tokens, segredos
        - internal_architecture        # Arquitetura interna (se marcada)
        - user_data                    # Dados de usuário
        - business_logic               # Lógica de negócio proprietária
      restrictions:
        - "Nunca exportado do projeto"
        - "Armazenado apenas em internal/embed/cosca/memory/ (local)"

  defaults:
    patterns: public
    heuristics: public
    failures: namespace
    discoveries: namespace
    principles: public
    decisions: private
    agent_learnings: namespace  # Learnings específicos do agente

  override:
    description: "Qualquer entrada pode ter visibilidade alterada via flag"
    mechanism: "Campo 'visibility' na signature YAML"
    enforcement: "Privacy Filter no Hub rejeita queries que violam visibilidade"
```

### 6.4 Deduplicação

Quando múltiplos projetos descobrem o mesmo padrão independentemente, o Dedup Engine consolida:

```
┌──────────────────────────────────────────────────────────────────┐
│                    DEDUPLICATION ENGINE                           │
│                                                                   │
│  Entrada: Nova assinatura KSIG-NEW                                │
│       │                                                           │
│       ▼                                                           │
│  ┌──────────────────────┐                                         │
│  │ 1. SIMILARITY CHECK   │  Cosine similarity contra todas as     │
│  │    ────────────────   │  assinaturas existentes no Hub         │
│  └──────────┬───────────┘                                         │
│             │                                                     │
│       ┌─────┴─────┐                                              │
│       │           │                                               │
│  sim < 0.85   sim ≥ 0.85                                         │
│       │           │                                               │
│       ▼           ▼                                               │
│  ┌─────────┐ ┌──────────────────────┐                             │
│  │ NOVA    │ │ 2. CONFLICT CHECK     │  Verifica se há            │
│  │ Entrada │ │    ────────────────   │  contradição semântica     │
│  │ única   │ └──────────┬───────────┘                             │
│  └─────────┘       ┌────┴────┐                                    │
│                    │         │                                     │
│              CONCORDA   CONTRADIZ                                  │
│                    │         │                                     │
│                    ▼         ▼                                     │
│  ┌──────────────────────┐ ┌──────────────────────┐                │
│  │ 3. MERGE              │ │ 3. CONFLICT           │                │
│  │    ─────              │ │    RESOLUTION         │                │
│  │ • KSIG-EXISTING       │ │ • Immune System (C5)  │                │
│  │   absorve proven.     │ │   avalia conflito     │                │
│  │ • validation_count += │ │ • Benchmark ambas     │                │
│  │   validações da nova  │ │ • Vence a de maior    │                │
│  │ • success_rate        │ │   massa gravitacional │                │
│  │   recalculado         │ │ • Perdedora marcada   │                │
│  │ • KSIG-NEW marcada    │ │   como superseded     │                │
│  │   como merged_into    │ └──────────────────────┘                │
│  └──────────────────────┘                                         │
│                                                                   │
│  Impacto:                                                          │
│  • Antes: 3 projetos, 3 assinaturas do mesmo padrão               │
│  • Depois: 1 assinatura consolidada, 3 proveniências,              │
│            validation_count = soma das 3, maior gravidade         │
└──────────────────────────────────────────────────────────────────┘
```

**Critérios de Merge**:

```yaml
dedup_criteria:
  similarity_threshold: 0.85       # Similaridade de embedding para considerar duplicata
  semantic_overlap_min: 0.70       # Overlap de preconditions
  domain_match_required: true      # Mesmo domain primário
  pattern_type_match_required: true # Mesmo pattern_type

  merge_strategy:
    provenance: "union"            # União de proveniências
    validation_count: "sum"        # Soma de validações
    success_rate: "weighted_avg"   # Média ponderada por validações
    gravity_score: "recalc"        # Recalculado com massa combinada
    preconditions: "union"         # União de preconditions (enriquece)
    applicability_score: "max"     # Mantém o maior score

  conflict_resolution:
    detection: "Cognitive Immune System (C5)"
    method: "Gravity-based arbitration"
    winner_rule: "Maior (validation_count × success_rate × recency)"
    loser_action: "Marcar como superseded_by = winner"
```

---

## 7. Exemplos Cross-Project

### 7.1 Exemplo 1: Cross-Agent Parallel Audit (Go CLI → Django App)

```
┌──────────────────────────────────────────────────────────────────┐
│  EXEMPLO 1: PADRÃO DE AUDITORIA CROSS-AGENT PARALELA             │
│                                                                   │
│  ORIGEM (Projeto A — cosca-core, Go CLI):                         │
│  ─────────────────────────────────────                            │
│  Task: Auditar consistência de documentação vs código             │
│  Padrão: Ativar 4 agentes em paralelo (Discovery + QA +           │
│          Testing + Architecture) para auditoria cross-source      │
│  Resultado: 63 broken references encontradas em 3 minutos         │
│  Signature: domain=documentation, pattern_type=cross_agent_audit  │
│             preconditions=["múltiplos artefatos para verificar",  │
│             "verificação manual seria lenta"]                     │
│             applicability_score=0.85                              │
│                                                                   │
│  DESTINO (Projeto B — Django REST API, Python):                   │
│  ──────────────────────────────────────────                      │
│  Task: Auditar consistência de serializers vs models vs docs      │
│  Contexto: Projeto Django com 45 serializers, 30 models,          │
│            documentação desatualizada                             │
│                                                                   │
│  MATCHING (Stage 2 — RETRIEVE MEMORY):                            │
│  ─────────────────────────────────────                            │
│  Federation Adapter gera task signature:                          │
│    domain: documentation                                          │
│    preconditions: ["múltiplos artefatos para verificar",          │
│                    "verificação manual seria lenta"]              │
│                                                                   │
│  Hub retorna MATCH 0.82:                                          │
│    KSIG-cross-agent-audit-v3 (cosca-core)                        │
│    "Ativar 4 agentes em paralelo para auditoria cross-source"     │
│    Validado 3x em cosca-core, 1x em django-cosca-app              │
│                                                                   │
│  ADAPTAÇÃO:                                                       │
│  ─────────                                                       │
│  Kernel sugere ao agente Django:                                  │
│  "Projeto cosca-core usou 4 agentes em paralelo para auditar      │
│   documentação. Adapte: em vez de Discovery+QA+Testing+Arch,      │
│   use Backend+QA+Documentation+Database (especialistas Django)."  │
│                                                                   │
│  O PADRÃO é o mesmo (4 agentes paralelos).                        │
│  O DOMÍNIO é diferente (Go CLI vs Django REST).                   │
│  A TRANSFERÊNCIA está no reconhecimento do padrão estrutural,     │
│  não na replicação exata dos agentes.                             │
└──────────────────────────────────────────────────────────────────┘
```

### 7.2 Exemplo 2: Jail Breach Prevention (cosca-core → Qualquer Projeto)

```
┌──────────────────────────────────────────────────────────────────┐
│  EXEMPLO 2: FAILURE AVOIDANCE — JAIL BREACH                       │
│                                                                   │
│  ORIGEM (Projeto A — cosca-core):                                 │
│  ───────────────────────────────                                  │
│  Failure: cosca init --force sem DRY_RUN                          │
│  Causa raiz: O Kernel executou uma operação destrutiva sobre      │
│              sua própria configuração sem simulação prévia.       │
│  Lição (L13): "O Kernel NÃO é um agente que usa ferramentas.     │
│                O Kernel É a configuração."                        │
│  Signature: domain=security, failure_type=jail_breach             │
│             preconditions=["destructive_operation",               │
│             "no_dry_run", "self_modification"]                    │
│                                                                   │
│  POTENCIAL VÍTIMA (Projeto B — qualquer projeto Cosca):           │
│  ────────────────────────────────────────────────                │
│  Task: "Refatorar estrutura de diretórios do agente"              │
│  Plano do agente: "Vou executar cosca agent move --force"         │
│                                                                   │
│  INTERCEPTAÇÃO (Stage 1 — SELF-ASSESS):                           │
│  ──────────────────────────────────────                           │
│  Federation Adapter detecta preconditions match:                  │
│    ☑ destructive_operation (--force flag)                         │
│    ☑ no_dry_run                                                   │
│    ☑ self_modification (mover diretórios do próprio agente)       │
│                                                                   │
│  Hub retorna MATCH 0.92:                                          │
│    KSIG-jail-breach-F003 (cosca-core)                             │
│    ⚠️ BLOCKING WARNING                                            │
│                                                                   │
│  Kernel BLOQUEIA execução e exibe:                                │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ ⚠️  FEDERATION WARNING — FAILURE AVOIDANCE                   │ │
│  │                                                               │ │
│  │ Esta operação tem assinatura similar a um jail breach         │ │
│  │ que ocorreu em cosca-core (2026-07-29).                       │ │
│  │                                                               │ │
│  │ Causa raiz: Operação destrutiva sobre configuração própria    │ │
│  │             sem DRY_RUN prévio.                                │ │
│  │                                                               │ │
│  │ Mitigação recomendada:                                        │ │
│  │   1. Execute DRY_RUN primeiro: cosca agent move --dry-run     │ │
│  │   2. Revise o plano de mudanças                                │ │
│  │   3. Confirme explicitamente a intenção                        │ │
│  │                                                               │ │
│  │ [Confirmar mesmo assim]  [Executar DRY_RUN]  [Cancelar]      │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  RESULTADO: O agente executa DRY_RUN, revisa o plano,             │
│  identifica um problema e evita o jail breach.                    │
│  +1 para B5 (Pre-failure Detection).                             │
└──────────────────────────────────────────────────────────────────┘
```

### 7.3 Exemplo 3: Extract-Then-Test (Go CLI → React Component)

```
┌──────────────────────────────────────────────────────────────────┐
│  EXEMPLO 3: PADRÃO EXTRACT-THEN-TEST CROSS-STACK                 │
│                                                                   │
│  ORIGEM (Projeto A — cosca-core, Go):                             │
│  ───────────────────────────────────                              │
│  Task: Refatorar função runServe() do runtime                     │
│  Problema: Função monolítica de 200+ linhas, 0% de cobertura      │
│  Padrão: 1) Extrair lógica em funções menores                     │
│          2) Testar cada função extraída isoladamente              │
│          3) Só então modificar a lógica                           │
│  Resultado: Cobertura subiu de 0% para 71.5%                      │
│  Signature: domain=refactoring, pattern_type=extract_then_test    │
│             preconditions=["função > 100 linhas",                 │
│             "cobertura < 20%"]                                    │
│             applicability_score=0.95, domain_coupling=0.15        │
│                                                                   │
│  DESTINO (Projeto C — React SPA, TypeScript):                     │
│  ─────────────────────────────────────────                        │
│  Task: Refatorar componente Dashboard.tsx (450 linhas)            │
│  Contexto: Monolito React com lógica de negócio, UI e estado      │
│            no mesmo arquivo. Testes: 0.                            │
│                                                                   │
│  MATCHING (Stage 2 — RETRIEVE MEMORY):                            │
│  ─────────────────────────────────────                            │
│  Task signature:                                                  │
│    domain: refactoring                                            │
│    preconditions: ["componente > 100 linhas", "cobertura 0%"]     │
│                                                                   │
│  Hub retorna MATCH 0.88:                                          │
│    KSIG-extract-then-test-v3 (cosca-core)                         │
│    applicability_score=0.95, domain_coupling=0.15                 │
│    "Altamente aplicável — baixo acoplamento ao domínio original"  │
│                                                                   │
│  ADAPTAÇÃO (React-specific):                                      │
│  ───────────────────────────                                     │
│  Kernel sugere:                                                   │
│  "Padrão extract-then-test do cosca-core (Go) é aplicável aqui.   │
│   Adaptação React:                                                │
│   1. Extraia lógica de negócio para hooks customizados             │
│   2. Extraia UI para sub-componentes                              │
│   3. Teste hooks isoladamente (React Testing Library)             │
│   4. Teste sub-componentes com mock de hooks                      │
│   5. Só então modifique a lógica original"                        │
│                                                                   │
│  O PADRÃO é o mesmo (extrair antes de testar).                    │
│  A STACK é diferente (Go → React/TypeScript).                     │
│  A ADAPTAÇÃO é automática: hooks em vez de funções Go.            │
│                                                                   │
│  RESULTADO:                                                       │
│  • Dashboard.tsx: 450 linhas → 4 arquivos (120 + 80 + 100 + 150) │
│  • 2 hooks customizados (useDashboardData, useChartConfig)        │
│  • 3 sub-componentes (DashboardHeader, ChartPanel, MetricsGrid)   │
│  • Cobertura: 0% → 68%                                            │
│  • +1 para B4 (Reused Knowledge cross-project)                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## 8. Integração com o Ecossistema Cosca

### 8.1 Integração com cosca-semantic-memory

O **cosca-semantic-memory** é o agente primário que implementa este engine. A integração ocorre em três níveis:

```yaml
integration_semantic_memory:
  index_level:
    description: "O índice semântico local (FTS5 + vector) é estendido com conhecimento federado"
    mechanism: "Federation Adapter injeta assinaturas do Hub como entradas 'federadas' no índice local"
    flag: "source: federated vs source: local"

  search_level:
    description: "Queries semânticas no Stage 2 incluem escopo cross-project"
    mechanism: |
      SEARCH(query, topK=5, scope=["local", "federated"])
      Resultados federados são mesclados com resultados locais.
      Ranking ajustado: local × 1.1 (mais relevante por contexto), federated × 0.9.
      Flag visual: 🏷️ FEDERADO nos resultados.

  feedback_level:
    description: "Validações cross-project realimentam o índice semântico"
    mechanism: |
      Quando Projeto B valida um padrão do Projeto A:
      → Incrementa validation_count na proveniência
      → Incrementa gravity_score no Cognitive Gravity (F2.4)
      → Atualiza success_rate na signature
      → Notifica Projeto A: "Seu padrão foi validado em outro projeto"
```

### 8.2 Integração com o Kernel (Stage 1-3)

O Kernel consulta o Federation Engine durante os estágios iniciais do metacognition pipeline:

```
┌──────────────────────────────────────────────────────────────────┐
│                    KERNEL → FEDERATION FLOW                       │
│                                                                   │
│  STAGE 0: BOOTSTRAP                                               │
│  ─────────────────                                                │
│  • Federation Adapter sincroniza com o Hub                        │
│  • Heurísticas universais injetadas no contexto do Kernel         │
│  • Índice de failures federados carregado no alert index          │
│  • Métrica B4 expandida inicializada                              │
│                                                                   │
│  STAGE 1: SELF-ASSESS                                             │
│  ───────────────────                                              │
│  • Kernel consulta: "Existe failure similar a esta task?"         │
│  • Federation Adapter faz query no Hub (failure matching)         │
│  • Se BLOCKING → Kernel bloqueia e exibe FederationWarning        │
│  • Se HIGH → Kernel alerta e registra no contexto da task         │
│                                                                   │
│  STAGE 2: RETRIEVE MEMORY                                         │
│  ────────────────────────                                         │
│  • Kernel consulta: "Existe padrão/learning similar?"             │
│  • Federation Adapter faz query cross-project (pattern matching)  │
│  • Top-5 resultados federados mesclados com resultados locais     │
│  • Cada resultado com: origem, validações, applicability_score    │
│                                                                   │
│  STAGE 3: PLAN STRATEGY                                           │
│  ────────────────────                                             │
│  • Kernel: "Quais heurísticas universais se aplicam?"             │
│  • Federation Adapter retorna heurísticas com applicability > 0.7 │
│  • Heurísticas com gravity_score > 5 são auto-sugeridas           │
│  • Kernel adapta estratégia com base nas heurísticas federadas    │
│                                                                   │
│  STAGE 7: EXTRACT PATTERN (pós-task)                              │
│  ───────────────────────────────────                              │
│  • Se novo padrão descoberto → Federation Adapter gera signature  │
│  • Se padrão existente aplicado → Federation Adapter registra     │
│    validação (incrementa proveniência)                            │
│  • Signature publicada no Hub (push)                              │
│                                                                   │
│  STAGE 8: UPDATE CAPABILITY (pós-task)                            │
│  ────────────────────────────────────                             │
│  • Métrica B4 atualizada: +1 para conhecimento reutilizado        │
│  • Se cross-project: +1 para B4 expandida (federated reuse)       │
│  • CMI Transferência recalculado com contribuição cross-project   │
└──────────────────────────────────────────────────────────────────┘
```

### 8.3 Integração com o CMI (Transferência)

A dimensão **Transferência** do CMI (15% do índice, peso 0.15) é diretamente impactada pelo Federation Engine:

```yaml
cmi_transferencia_integration:
  current_baseline: 87  # Transferência = 87/100

  federation_impact:
    pre_federation:
      description: "Transferência limitada ao projeto atual"
      metrics:
        cross_agent_clusters: 8        # Dentro do mesmo projeto
        heuristics_extracted: 20       # De um único projeto
        cross_project_applications: 0  # ZERO — é o gap que este engine resolve
      score: 87

    post_federation:
      description: "Transferência cross-project ativa"
      target_metrics:
        cross_agent_clusters: 12       # +4 (incluindo clusters cross-project)
        heuristics_extracted: 35       # +15 (contribuições de múltiplos projetos)
        cross_project_applications: 25 # Alvo: 25 reaplicações cross-project
        projects_federated: 5          # Alvo: 5 projetos ativos na federação
        universal_heuristics: 5        # Alvo: 5 heurísticas promovidas a universal
      target_score: 97                 # +10 (ganho F2.3)

  b4_metric_expanded:
    definition: "Conhecimento Reutilizado (expandido para cross-project)"
    current: 12  # Reutilizações dentro do projeto
    target: 37   # +25 reutilizações cross-project
    tracking: |
      B4_local:   Aplicações de padrões/heurísticas do próprio projeto
      B4_federated: Aplicações de padrões/heurísticas de OUTROS projetos
      B4_total = B4_local + B4_federated

  calculation:
    formula: |
      Transferência_Score = (B4_total / B4_target × 100) × 0.4
                          + (cross_project_applications / 25 × 100) × 0.3
                          + (universal_heuristics / 5 × 100) × 0.2
                          + (projects_federated / 5 × 100) × 0.1
```

---

## 9. Operações do Engine

### 9.1 Comandos da CLI

```bash
# === SINCRONIZAÇÃO ===

# Sync completo bidirecional (pull + push)
cosca federation sync --project <nome>

# Push: publicar conhecimento local novo no Hub
cosca federation push --project <nome> --signatures recent
cosca federation push --project <nome> --signatures all
cosca federation push --project <nome> --signature KSIG-2026-07-30-001

# Pull: baixar conhecimento federado de outros projetos
cosca federation pull --project <nome> --since 2026-08-01
cosca federation pull --project <nome> --domain security
cosca federation pull --project <nome> --all

# === CONSULTA ===

# Buscar padrões similares a uma task
cosca federation search --query "refatorar função monolítica sem testes" --topK 5

# Buscar failures similares a uma operação
cosca federation search-failures --preconditions "destructive_operation,no_dry_run"

# Buscar heurísticas universais aplicáveis
cosca federation heuristics --domain refactoring --min-applicability 0.7

# === ADMINISTRAÇÃO ===

# Status da federação
cosca federation status --project <nome>

# Listar assinaturas publicadas por este projeto
cosca federation list --origin <nome>

# Visualizar assinatura específica
cosca federation show KSIG-2026-07-30-001

# Validar uma assinatura federada (após aplicar com sucesso)
cosca federation validate KSIG-2026-07-30-001 --project <nome> --success

# Contestar uma assinatura federada (após aplicar e falhar)
cosca federation validate KSIG-2026-07-30-001 --project <nome> --failure

# === CONFIGURAÇÃO ===

# Configurar visibilidade padrão
cosca federation config --set visibility.defaults.patterns=public
cosca federation config --set visibility.defaults.failures=namespace

# Adicionar projeto ao namespace
cosca federation namespace add --project <nome> --namespace cosca-official

# === MÉTRICAS ===

# Dashboard de transferência cross-project
cosca federation metrics --project <nome>
# Output:
#   B4_local: 12
#   B4_federated: 8
#   Cross-project applications: 8
#   Projects federated: 3
#   Universal heuristics active: 2
#   Transferência CMI contribution: +3.2
```

### 9.2 API do Federation Adapter

O Federation Adapter expõe uma API interna para os agentes:

```yaml
federation_adapter_api:
  # === CONSULTA (usado no Stage 1-3) ===

  matchPattern:
    input:
      task_signature: "assinatura YAML da task atual"
      topK: 5
      min_applicability: 0.60
    output:
      matches: [{signature, similarity, applicability, provenance}]

  matchFailure:
    input:
      operation_signature: "assinatura YAML da operação planejada"
      min_similarity: 0.70
    output:
      warnings: [{signature, severity, root_cause, mitigation}]

  getUniversalHeuristics:
    input:
      domain: "refactoring"
    output:
      heuristics: [{id, title, description, applicability_score}]

  # === PUBLICAÇÃO (usado no Stage 7) ===

  publishSignature:
    input:
      signature: "assinatura YAML completa (ver §5.1)"
    output:
      signature_id: "KSIG-2026-07-30-001"
      status: "published | merged | conflicted"

  validateSignature:
    input:
      signature_id: "KSIG-2026-07-30-001"
      project: "cosca-core"
      outcome: "success | partial | failure"
    output:
      updated_provenance: {...}

  # === SINCRONIZAÇÃO ===

  sync:
    input:
      direction: "pull | push | full"
      since: "2026-08-01"  # opcional
    output:
      pulled: 12
      pushed: 3
      merged: 1
      conflicts: 0

  # === ADMINISTRAÇÃO ===

  getStatus:
    output:
      hub_connected: true
      last_sync: "2026-07-30T20:00:00Z"
      local_signatures: 15
      federated_signatures: 43
      pending_push: 2
      pending_pull: 5
```

---

## 10. Dependências

| Engine / Sistema | Propósito | Tipo |
|------------------|-----------|------|
| Semantic Memory Engine | Índice vetorial local — base para indexação cross-project | Forte |
| Knowledge Engine | Catálogo de padrões/heurísticas — fonte de assinaturas | Forte |
| Provider Chief | Geração de embeddings para assinaturas | Forte |
| Cognitive Gravity Engine (F2.4) | Massa gravitacional cross-project das assinaturas federadas | Média |
| Cognitive Immune System (F2.2) | Detecção de contradições em assinaturas federadas | Média |
| Pattern Evolution (F2.6) | Versionamento de padrões cross-project | Média |
| Memory Chief | Curadoria do conhecimento federado (aceitar/rejeitar/contestar) | Média |
| Decision DNA (F1.1) | Registro de decisões de federação (publicar, rejeitar, abdicar) | Fraca |
| Monitoring Chief | Tracking de métrica B4 expandida | Fraca |

### Dependências de Infraestrutura

| Componente | Propósito |
|------------|-----------|
| `$HOME/.cosca/federation/vectors.db` | SQLite com índice vetorial cross-project |
| `$HOME/.cosca/federation/signatures/` | Storage de assinaturas YAML |
| `$HOME/.cosca/federation/config.yaml` | Configuração global da federação |
| Provider API (OpenAI, etc.) | Geração de embeddings |

---

## 11. Restrições (Constraints)

1. **Nunca federar secrets, tokens ou credenciais**: O Privacy Filter rejeita qualquer assinatura que contenha padrões de credenciais (regex: `/(sk-|api_key|token|password|secret)/i`).

2. **Nunca federar código proprietário**: Apenas assinaturas (metadados + embeddings) são compartilhadas. O código fonte, lógica de negócio e arquitetura interna NUNCA saem do projeto.

3. **Consentimento explícito para failures**: Failures são `namespace` por padrão. Para torná-los `public`, o Don deve aprovar explicitamente.

4. **Validação antes da promoção**: Uma assinatura só é promovida a "heurística universal" após validação em ≥ 3 projetos diferentes.

5. **Respeitar o Wisdom Decay**: Assinaturas federadas também decaem (C11). Se uma assinatura não é validada há > 180 dias, sua applicability_score é reduzida em 20%.

6. **Nunca sobrecarregar o Kernel**: Máximo de 5 sugestões federadas por task. Se houver mais matches, apenas os top-5 por ranking são apresentados.

7. **Offline-first**: Se o Hub estiver indisponível, o projeto opera normalmente com conhecimento local. A sincronização é assíncrona e não bloqueante.

8. **Namespace é imutável após criação**: O `project_id` (UUID) é gerado no primeiro sync e nunca muda. Isso garante rastreabilidade de proveniência permanente.

---

## 12. Critérios de Qualidade

- [ ] Federation Hub acessível em `$HOME/.cosca/federation/` com estrutura de diretórios completa
- [ ] Pelo menos 2 projetos diferentes sincronizando com o Hub
- [ ] Busca semântica cross-project funcional: query no Projeto B retorna padrões do Projeto A com relevance > 0.60
- [ ] Failure Avoidance ativo: ≥ 1 warning BLOCKING emitido antes de operação perigosa
- [ ] Heuristic Transfer: ≥ 2 heurísticas universais propagadas automaticamente
- [ ] Deduplicação funcional: ≥ 1 caso de merge de assinaturas similares
- [ ] Privacidade: padrões são public, failures são namespace, decisões são private
- [ ] Proveniência rastreável: toda assinatura federada tem cadeia de origem e validações
- [ ] Métrica B4 expandida: B4_federated > 0 antes do final da Fase 2
- [ ] CMI Transferência: aumento de ≥ 5 pontos após federação ativa (target F2.3: +10)
- [ ] Sincronização automática: pull/push assíncrono a cada 6h sem intervenção manual
- [ ] CLI completa: comandos `sync`, `push`, `pull`, `search`, `validate`, `status` operacionais
- [ ] Timeout de sync ≤ 10s (assíncrono, não bloqueia execução)
- [ ] Zero vazamento de secrets/credenciais (Privacy Filter 100% efetivo)

---

## 13. Plano de Implementação (F2.3)

A implementação segue 5 estágios incrementais:

```yaml
implementation_plan:
  stage_1_hub:
    duration: "4-6 horas"
    description: "Criar estrutura do Federation Hub em $HOME/.cosca/federation/"
    deliverables:
      - "Estrutura de diretórios (vectors.db, signatures/, provenance/, config.yaml)"
      - "Schema SQLite para vector index + signature store"
      - "Configuração global com defaults de privacidade"
      - "CLI básica: cosca federation init"

  stage_2_signatures:
    duration: "6-8 horas"
    description: "Implementar geração e publicação de Knowledge Signatures"
    deliverables:
      - "Signature generation pipeline (classify → extract → score → embed → publish)"
      - "Vocabulário controlado de domínios, knowledge_types, pattern_types"
      - "Federation Adapter: publishSignature(), validateSignature()"
      - "Integração com Stage 7 (EXTRACT PATTERN)"

  stage_3_matching:
    duration: "6-8 horas"
    description: "Implementar Pattern Matching e Failure Avoidance"
    deliverables:
      - "matchPattern() — busca cross-project por similaridade de task"
      - "matchFailure() — busca cross-project por similaridade de operação"
      - "getUniversalHeuristics() — propagação automática de heurísticas universais"
      - "Integração com Stage 1-3 do metacognition pipeline"
      - "FederationWarning UI no Kernel"

  stage_4_dedup_privacy:
    duration: "4-6 horas"
    description: "Implementar deduplicação, privacidade e sincronização"
    deliverables:
      - "Dedup Engine (similarity check → merge/conflict resolution)"
      - "Privacy Filter (public/namespace/private enforcement)"
      - "Sync automático (background 6h + on-task-completion)"
      - "CLI completa: sync, push, pull, search, search-failures, heuristics, validate, status"

  stage_5_metrics_integration:
    duration: "2-4 horas"
    description: "Integrar métricas, CMI e dashboard"
    deliverables:
      - "Métrica B4 expandida (B4_federated)"
      - "CMI Transferência recalculado com contribuição cross-project"
      - "Dashboard de federação (projetos, assinaturas, reutilizações)"
      - "Documentação final e validação cross-project com ≥ 2 projetos reais"
```

---

## 14. Escalabilidade

### 14.1 Limites do Hub Local

```yaml
scalability:
  hub_local:
    max_signatures: 10000         # Assinaturas (comportamento testado)
    max_projects: 100             # Projetos federados
    vector_index_size: "~50MB"    # Para 10000 embeddings 1536-dim
    sync_time: "< 10s"            # Sync completo para 10000 assinaturas
    query_time: "< 500ms"         # Busca top-5 com filtro

  future_remote_hub:
    description: "Hub remoto para federação multi-machine (Fase 3+)"
    candidates:
      - "Servidor compartilhado (NFS/S3) com vectors.db"
      - "Pinecone/Weaviate/Milvus como vector store remoto"
      - "Git repo dedicado para signatures (git-based federation)"
```

### 14.2 Estratégia de Sharding

Se o Hub crescer além de 10.000 assinaturas:

```yaml
sharding:
  strategy: "Domain-based sharding"
  shards:
    - shard_security:     "signatures/security/"
    - shard_testing:      "signatures/testing/"
    - shard_refactoring:  "signatures/refactoring/"
    - shard_architecture: "signatures/architecture/"
    - shard_general:      "signatures/general/"
  query_routing: "Query primeiro no shard do domain da task, depois fallback para general"
```

---

## 15. Histórico

| Versão | Data | Autor | Mudanças |
|---------|------|--------|----------|
| 1.0.0 | 2026-07-30 | Cosca Semantic Memory Chief | Especificação inicial completa do Knowledge Federation Engine. Define: arquitetura da federação (Hub + Adapters), 3 mecanismos de transferência (Pattern Matching, Failure Avoidance, Heuristic Transfer), formato canônico de Knowledge Signature (24 campos em 10 seções YAML), Federation Hub com storage project-agnostic, modelo de privacidade em 3 níveis, deduplicação cross-project, 3 exemplos completos de transferência cross-domain, integração com metacognition pipeline (Stages 0-8), integração com CMI (dimensão Transferência +10), API do Federation Adapter, CLI completa, plano de implementação em 5 estágios, critérios de qualidade, restrições de segurança e privacidade. |

---

## 16. Referências

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §5 C12 | Define o Cognitive Ecosystem — a federação é a materialização dos fluxos cross-project |
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §4 A4 | Define Transferência de Conhecimento como capacidade core de julgamento |
| [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) F2.3 | Tarefa que originou esta especificação |
| [Semantic Memory Engine](../semantic-memory/SKILL.md) | Engine de busca semântica local que a federação estende |
| [Knowledge Engine](../knowledge/SKILL.md) | Catálogo de padrões/heurísticas — fonte primária de assinaturas |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato de learning entries — base para extração de assinaturas |
| [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Stages 7-8 do metacognition — pontos de integração da federação |
| [Cognitive Gravity](../cognitive-gravity/SKILL.md) (F2.4) | Massa gravitacional cross-project das assinaturas |
| [Cognitive Immune System](../cognitive-immune-system/SKILL.md) (F2.2) | Validação de contradições em assinaturas federadas |
| [Pattern Evolution](../pattern-evolution/SKILL.md) (F2.6) | Versionamento cross-project de padrões federados |
| [Wisdom Decay](../wisdom-decay/WISDOM_DECAY.md) (F1.4) | Decaimento de confiança aplicado a assinaturas federadas |

---

> **"Aprender um padrão em um projeto e reconhecer quando ele pode ser aplicado em outro, mesmo que os domínios sejam diferentes."**
>
> — O Don, definindo transferência de conhecimento cross-project
>
> **Status Engine**: ACTIVE. Fase 2, Bloco 1 (Memória & Conhecimento). Owner: Cosca Semantic Memory Chief.
> **Próximo Marco**: Stage 1 (Hub) — 4-6 horas. Depende de: F1.1 (Decision DNA) concluído.
