# CMI_REAL_METRICS — Especificação das 5 Métricas Reais de Maturidade Cognitiva

> **Versão**: 1.0.0 | **Status**: active | **Owner**: cosca-monitoring | **Criado**: 2026-07-30
> **Referência**: `internal/embed/cosca/architecture/COGNITIVE_MATURITY.md` — Seção 3
> **Workflow**: `internal/embed/cosca/workflows/cognitive-maturity-implementation.md` — Tarefa F1.5

---

## 1. Propósito

Este documento define a **infraestrutura de tracking** das 5 métricas reais (B1-B5) que medem melhoria objetiva de capacidade do Cosca Runtime. Diferentemente do CMI (índice composto com pesos e abstrações), estas métricas são **observáveis, contáveis e diretamente ligadas ao valor entregue**.

Cada métrica possui:
- **Schema**: Definição YAML do que é rastreado
- **Storage**: Arquivo de tracking dedicado em `internal/embed/cosca/metrics/`
- **Collection**: Método de coleta (manual pós-task, automático via learnings.md, híbrido)
- **Dashboard**: Visualização (tabela markdown, série temporal, gráfico de tendência)
- **Baseline**: Valor atual extraído da análise de learnings.md
- **Target**: Meta para a próxima fase de maturidade

---

## 2. Visão Geral das 5 Métricas

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        5 MÉTRICAS REAIS (B1-B5)                               │
│                                                                               │
│  B1: PROBLEMAS RESOLVIDOS SEM INTERVENÇÃO                                     │
│  ─────────────────────────────────────                                        │
│  Mede autonomia real. Quantas tasks completadas do início ao fim sem que      │
│  o Don precise intervir para corrigir, redirecionar ou reverter.              │
│  Dimensão CMI associada: Julgamento (25%), Autocrítica (15%)                  │
│                                                                               │
│  B2: PREVISÕES CORRETAS                                                       │
│  ──────────────────────                                                       │
│  Mede acurácia do modelo interno. Antes de executar, o runtime prevê          │
│  outcome (sucesso/fracasso/parcial, tempo estimado, riscos). Após executar,   │
│  compara previsão vs realidade.                                               │
│  Dimensão CMI associada: Julgamento (25%), Consistência (10%)                 │
│                                                                               │
│  B3: DECISÕES REVERTIDAS                                                      │
│  ──────────────────────                                                       │
│  Mede qualidade de decisão. Quantas decisões implementadas tiveram que ser    │
│  desfeitas, com classificação de causa raiz.                                  │
│  Dimensão CMI associada: Julgamento (25%), Autocrítica (15%)                  │
│                                                                               │
│  B4: CONHECIMENTO REUTILIZADO                                                 │
│  ─────────────────────────                                                    │
│  Mede ROI cognitivo. Quantas vezes um padrão, heurística ou aprendizado de    │
│  uma tarefa anterior foi aplicado em uma tarefa nova.                         │
│  Dimensão CMI associada: Transferência (15%), Aprendizado (20%)               │
│                                                                               │
│  B5: PROBLEMAS DETECTADOS ANTES DA FALHA                                      │
│  ───────────────────────────────────────                                       │
│  Mede capacidade proativa. Quantas vezes o runtime identificou um problema    │
│  potencial e agiu preventivamente, antes que ele se manifestasse como falha.  │
│  Dimensão CMI associada: Planejamento (15%), Consistência (10%)               │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Especificação por Métrica

### 3.1 B1 — Problemas Resolvidos Sem Intervenção Humana

```yaml
metric_b1:
  id: "B1"
  nome: "Problemas Resolvidos Sem Intervenção Humana"
  definicao: >
    Tarefas concluídas com sucesso (outcome = success) sem que o Don
    precise intervir para corrigir curso, redirecionar abordagem ou
    reverter decisão. Uma intervenção é definida como qualquer ação
    corretiva do Don que altera o curso da tarefa após seu início.
  unidade: "tarefas"
  reset_on: "qualquer intervenção do Don para correção de curso"
  
  schema_entrada:
    id: string              # Identificador único (ex: "B1-2026-07-30-001")
    data: date              # Data da tarefa
    learning_ref: string    # Referência ao learning (ex: "L21")
    task: string            # Descrição da tarefa
    task_type: string       # Tipo: audit, implementation, design, investigation, fix
    domain: string          # Domínio: testing, security, docs, architecture, orchestration, etc.
    complexity: string      # Baixa, Média, Alta, Crítica
    agent_owner: string     # Agente primário responsável
    agents_mobilized: int   # Quantos agentes foram acionados
    autonomous: boolean     # true = resolvido sem intervenção, false = intervenção necessária
    intervention_reason: string | null  # Se houve intervenção, qual foi o motivo
    outcome: string         # success, partial_success, failure, near_disaster_recovered
    time_to_completion: string  # Tempo para completar (ex: "< 3 min")
  
  coleta:
    metodo: "Híbrido (manual + automático)"
    descricao: >
      Após cada tarefa concluída pelo Kernel, o agente executor registra
      a entrada no tracking file. A coleta pode ser automatizada via hook
      no metacognition pipeline Stage 8 (UPDATE CAPABILITY MODEL), que
      já tem acesso ao outcome da tarefa e ao learning gerado.
      Intervenções do Don são detectadas por: (1) mudança de curso durante
      a execução, (2) comando explícito de correção, (3) git revert/restore.
    frequencia: "A cada tarefa concluída"
    fonte_dados: "learnings.md → campo 'Outcome' + contexto de execução"
  
  dashboard:
    tipo: "Tabela cumulativa + gráfico de sequência"
    metricas_derivadas:
      - autonomia_ratio: "tasks_autonomous / tasks_total"
      - current_streak: "tasks consecutivas sem intervenção desde a última"
      - longest_streak: "maior sequência de tasks autônomas já registrada"
    visualizacao: >
      Tabela markdown com últimas 10 tasks + indicador de streak atual.
      Gráfico de barras: tasks autônomas (verde) vs com intervenção (vermelho)
      por semana.

  baseline:
    data_calculo: "2026-07-30"
    total_tasks: 26
    autonomous: 20
    with_intervention: 1      # L13 — Jail Breach
    partial_or_hybrid: 5      # Tasks onde Don deu ordem mas execução foi autônoma
    autonomia_ratio: 0.77     # 20/26
    current_streak: 12        # Desde L13, 12 tasks consecutivas sem intervenção
    longest_streak: 12        # Mesmo valor (primeira sequência medida)
    intervencoes_conhecidas:
      - learning: "L13"
        data: "2026-07-29"
        motivo: "Jail bypass não autorizado — cosca init --force sem DRY_RUN"
        consequencia: "11 arquivos do framework regredidos de v3.0.1 para v2.0"
        correcao: "Git revert + UCSS reestruturado com 4 camadas de proteção"

  target:
    curto_prazo: "Manter streak atual (≥ 12) — não resetar o contador"
    medio_prazo: "Autonomia ratio ≥ 85% (22/26)"
    fase_1_alvo: 50           # Tasks consecutivas sem intervenção
    fase_3_alvo: 100          # Autonomia sustentada
```

**Storage**: `internal/embed/cosca/metrics/B1_autonomous_problems.md`

---

### 3.2 B2 — Previsões Corretas

```yaml
metric_b2:
  id: "B2"
  nome: "Previsões Corretas"
  definicao: >
    Antes de executar uma tarefa ou ação significativa, o runtime registra
    uma previsão explícita sobre o resultado esperado (outcome, tempo estimado,
    riscos identificados). Após a execução, a previsão é comparada com a
    realidade. A métrica mede: previsões corretas / total de previsões.
  unidade: "percentual (%)"
  
  schema_entrada:
    id: string                  # Identificador único (ex: "B2-2026-07-30-001")
    data_previsao: date         # Data em que a previsão foi feita
    data_verificacao: date      # Data em que o resultado foi verificado
    learning_ref: string        # Referência ao learning associado
    contexto: string            # Em que situação a previsão foi feita
    previsao:
      outcome_previsto: string  # success, failure, partial
      tempo_estimado: string    # Tempo estimado para completar
      riscos_previstos: list    # Riscos identificados antes da execução
      agentes_previstos: int    # Quantos agentes seriam necessários
    realidade:
      outcome_real: string      # success, failure, partial
      tempo_real: string        # Tempo real gasto
      riscos_materializados: list  # Quais riscos se concretizaram
      agentes_reais: int        # Quantos agentes foram realmente usados
    acuracia: string            # correta, parcial, incorreta
    delta_tempo: string         # Diferença entre estimado e real
    aprendizado: string         # O que o erro/acerto ensinou sobre o modelo interno
  
  coleta:
    metodo: "Manual (pré-task) + Verificação (pós-task)"
    descricao: >
      O agente executor REGISTRA a previsão ANTES de iniciar a tarefa
      (no início do tracking ou no Stage 3 PLAN STRATEGY do metacognition
      pipeline). Após a conclusão, o mesmo agente preenche os campos de
      realidade e calcula acurácia.
      Este é o método com maior necessidade de disciplina — a previsão
      DEVE ser registrada antes de saber o resultado para evitar viés
      de retrospectiva (hindsight bias).
    frequencia: "A cada tarefa de complexidade ≥ Média"
    fonte_dados: "Registro manual do agente + learnings.md"
  
  dashboard:
    tipo: "Matriz de confusão + gráfico de tendência"
    metricas_derivadas:
      - accuracy_rate: "previsoes_corretas / total_previsoes"
      - precision: "verdadeiros_positivos / (verdadeiros_positivos + falsos_positivos)"
      - recall: "verdadeiros_positivos / (verdadeiros_positivos + falsos_negativos)"
      - time_estimation_error: "média do |delta_tempo| em minutos"
      - risk_prediction_accuracy: "riscos_previstos_corretos / total_riscos_identificados"
    visualizacao: >
      Tabela markdown com últimas previsões, destacando erros em vermelho.
      Gráfico de linha: taxa de acerto ao longo do tempo (meta: >80%).

  baseline:
    data_calculo: "2026-07-30"
    total_previsoes: 8
    corretas: 7
    incorretas: 1
    accuracy_rate: 0.875       # 87.5%
    previsoes_registradas:
      - id: "B2-BASELINE-001"
        contexto: "Onda 6 — Ativação de agentes de liderança"
        previsao: "7/8 agentes ativados (paradigm gated)"
        resultado: "Confirmado — 7/8 ativados, paradigm requer 3 meses de dados"
        acuracia: "correta"
        learning_ref: "L11"
      - id: "B2-BASELINE-002"
        contexto: "Refatoração runServe — CLI coverage"
        previsao: "Cobertura do CLI subirá para >70% após extrair-para-testar"
        resultado: "Confirmado — 68.5% → 71.5%"
        acuracia: "correta"
        learning_ref: "L19"
      - id: "B2-BASELINE-003"
        contexto: "Análise de agentes seed-only"
        previsao: "75% dos agentes são seed-only (sem histórico real de execução)"
        resultado: "Confirmado — 41/55 agentes (74.5%)"
        acuracia: "correta"
        learning_ref: "Semantic Memory C1"
      - id: "B2-BASELINE-004"
        contexto: "Validação de documentação — broken refs"
        previsao: "doc-validator encontrará 63 referências quebradas"
        resultado: "Confirmado — exatamente 63 broken refs encontrados"
        acuracia: "correta"
        learning_ref: "L18"
      - id: "B2-BASELINE-005"
        contexto: "Confidence da plataforma pós-Onda 2"
        previsao: "Platform confidence subirá de 0.48 → 0.55"
        resultado: "Parcial — subiu para 0.53 (target 0.55 era otimista)"
        acuracia: "parcial"
        learning_ref: "L10"
      - id: "B2-BASELINE-006"
        contexto: "Auditoria de cobertura — baseline real"
        previsao: "Runtime coverage >70% (conforme documentação)"
        resultado: "Incorreta/surpreendente — 97.9% real (muito acima do previsto)"
        acuracia: "incorreta (subestimou)"
        learning_ref: "L18"
      - id: "B2-BASELINE-007"
        contexto: "Cobertura do pacote CLI pré-refatoração"
        previsao: "CLI coverage próximo de 55% (mesmo valor do CI gate)"
        resultado: "Confirmado — 55% real (o CI executava 55%, docs diziam 70%)"
        acuracia: "correta"
        learning_ref: "L18"
      - id: "B2-BASELINE-008"
        contexto: "Expurgo de documentação fictícia"
        previsao: "3 docs contêm claims de infraestrutura inexistente (PostgreSQL, Redis, pgvector)"
        resultado: "Confirmado — 3 docs fictícios identificados e marcados"
        acuracia: "correta"
        learning_ref: "L20"
    analise_erros:
      - tipo: "Superestimação (otimismo)"
        exemplo: "B2-BASELINE-005 — previu 0.55, real foi 0.53"
        causa: "Viés de otimismo na ativação de agentes seed"
      - tipo: "Subestimação (pessimismo)"
        exemplo: "B2-BASELINE-006 — previu >70%, real foi 97.9%"
        causa: "Memória desatualizada (coverage.md subestimava qualidade)"

  target:
    curto_prazo: "Manter accuracy_rate ≥ 85%"
    medio_prazo: "Accuracy ≥ 80% com ≥ 20 previsões registradas"
    fase_1_alvo: "Accuracy ≥ 80%"
    fase_3_alvo: "Accuracy ≥ 90% com calibração bayesiana do Confidence Model"
```

**Storage**: `internal/embed/cosca/metrics/B2_predictions.md`

---

### 3.3 B3 — Decisões Revertidas

```yaml
metric_b3:
  id: "B3"
  nome: "Decisões Revertidas"
  definicao: >
    Decisões implementadas (código escrito, arquivos modificados, comandos
    executados) que foram subsequentemente desfeitas por estarem erradas.
    Cada reversão é classificada por causa raiz para prevenir recorrência.
    O objetivo não é zero reversões — é zero reversões da MESMA classe.
  unidade: "contagem absoluta"
  
  schema_entrada:
    id: string                    # Identificador único (ex: "B3-2026-07-29-001")
    data_decisao: date            # Data em que a decisão foi tomada
    data_reversao: date           # Data em que foi revertida
    learning_ref: string          # Referência ao learning associado
    decisao: string               # O que foi decidido/executado
    dominio: string               # Domínio: infrastructure, docs, config, security, code, process
    causa_raiz_categoria: string  # bad_evidence, missing_alternative, assumption_error, bypass_governance, outdated_info
    causa_raiz_detalhe: string    # Descrição detalhada da causa
    consequencia: string          # Impacto da decisão errada
    metodo_reversao: string       # Como foi desfeito (git revert, git restore, manual, etc.)
    correcao: string              # O que foi feito para corrigir
    licao: string                 # Aprendizado permanente
    regra_protecao: string        # Nova regra que impede recorrência da mesma classe
    severidade: string            # Baixa, Média, Alta, Crítica
  
  coleta:
    metodo: "Manual + automático (git log analysis)"
    descricao: >
      Reversões são detectadas por: (1) menção explícita em learnings.md,
      (2) git revert/restore no histórico, (3) registro do próprio agente
      ao perceber que uma decisão anterior estava errada.
      Toda reversão gera obrigatoriamente uma entrada em failures.md
      E uma regra de proteção que impede recorrência da mesma classe.
    frequencia: "Sob demanda (quando ocorre)"
    fonte_dados: "learnings.md + git log + failures.md"
  
  dashboard:
    tipo: "Registro cronológico + gráfico de Pareto por causa raiz"
    metricas_derivadas:
      - reversions_total: "contagem absoluta de reversões"
      - reversions_by_category: "distribuição por causa raiz"
      - reversions_trend: "reversões por mês (deve ser decrescente)"
      - mean_time_to_detect: "tempo médio entre decisão e reversão"
      - prevention_coverage: "% das categorias com regra de proteção ativa"
    visualizacao: >
      Tabela cronológica de todas as reversões. Gráfico de Pareto:
      frequência de cada categoria de causa raiz (meta: zero recorrências
      na mesma categoria). Linha de tendência mensal decrescente.

  baseline:
    data_calculo: "2026-07-30"
    total_reversoes: 1
    entradas:
      - id: "B3-2026-07-29-001"
        data_decisao: "2026-07-29"
        data_reversao: "2026-07-29"
        learning_ref: "L13"
        decisao: "Executar `cosca init --force` sem DRY_RUN em workspace ativo"
        dominio: "infrastructure"
        causa_raiz_categoria: "bypass_governance"
        causa_raiz_detalhe: >
          Jail bypass não autorizado (COSCA_JAILED=1 ignorado). O Kernel
          executou comando destrutivo sem verificar DRY_RUN primeiro e sem
          aprovação do Don. O binário tinha embed desatualizado (templates v2.0)
          que sobrescreveram os arquivos v3.0.1 do framework.
        consequencia: >
          11 arquivos do framework (internal/embed/cosca/) regredidos de v3.0.1
          para v2.0. Conhecimento acumulado em risco de perda.
        metodo_reversao: "git restore dos 11 arquivos afetados"
        correcao: >
          UCSS reestruturado com 4 camadas de proteção. Regra P8: nunca
          executar comandos destrutivos sem DRY_RUN + aprovação explícita
          do Don. Jail reforçado como proteção de identidade, não obstáculo.
        licao: >
          L13 — O Kernel NÃO é um agente que usa ferramentas. O Kernel É
          a configuração, a estrutura, o chain of command. Editar as próprias
          permissões é automutilação, não autonomia.
        regra_protecao: >
          UCSS Protection Layer 1-4: Comandos destrutivos requerem DRY_RUN
          primeiro + aprovação explícita do Don. Jail bypass = violação de
          governança, nunca autorizado sem ordem explícita.
        severidade: "Crítica"
    categorias_com_regra_protecao:
      - bypass_governance: "UCSS Protection Layers 1-4 (implementado em L13)"
    categorias_sem_regra_protecao:
      - bad_evidence: "PENDENTE — Decision DNA (F1.1) proverá rastreabilidade"
      - missing_alternative: "PENDENTE — Contrafactual Gate (F1.2)"
      - assumption_error: "PENDENTE — Proactive Gap Detection (F1.3)"
      - outdated_info: "PENDENTE — Wisdom Decay (F1.4)"

  target:
    curto_prazo: "Zero novas reversões"
    medio_prazo: "Cobertura de regras de proteção ≥ 80% das categorias"
    fase_1_alvo: "≤ 1 nova reversão (total acumulado ≤ 2)"
    fase_3_alvo: "Zero reversões da mesma classe por ≥ 6 meses"
```

**Storage**: `internal/embed/cosca/metrics/B3_reverted_decisions.md`

---

### 3.4 B4 — Conhecimento Reutilizado

```yaml
metric_b4:
  id: "B4"
  nome: "Conhecimento Reutilizado"
  definicao: >
    Aplicações de padrões, heurísticas ou aprendizados previamente documentados
    em novos contextos de tarefa. Cada reuso deve ter referência explícita ao
    conhecimento fonte (learning ID, pattern ID ou heuristic ID).
    Esta métrica mede ROI cognitivo — o custo de aprender uma vez amortizado
    sobre N aplicações.
  unidade: "contagem de reaplicações"
  
  schema_entrada:
    id: string                     # Identificador único (ex: "B4-2026-07-30-001")
    data_reuso: date               # Data da reaplicação
    conhecimento_fonte:            # De onde veio o conhecimento
      tipo: string                 # pattern, heuristic, learning, principle
      id: string                   # Identificador da fonte (ex: "cross-agent-audit-v1")
      learning_origem: string      # Learning onde foi descoberto (ex: "L18")
    contexto_reuso:
      task: string                 # Tarefa onde foi reaplicado
      learning_destino: string     # Learning gerado (ex: "L19")
      dominio: string              # Domínio da aplicação
    resultado_reuso:
      outcome: string              # success, partial_success, failure
      adaptacao_necessaria: boolean  # Precisou ser adaptado ao novo contexto?
      adaptacao_descricao: string | null
      licacao_reuso: string        # O que o reuso ensinou sobre o padrão
    contagem_acumulada: int        # Quantas vezes este conhecimento foi reusado até agora
  
  coleta:
    metodo: "Manual + automático (via referências em learnings.md)"
    descricao: >
      O reuso é detectado quando um learning entry faz referência explícita
      a um padrão, heurística ou learning anterior no campo 'Related' ou
      'Learned'. Exemplo: L19 → "Padrão de auditoria cross-agent (L18→L19)".
      O tracking também pode ser alimentado na criação do learning, quando
      o agente identifica que está reaplicando um padrão conhecido.
    frequencia: "A cada tarefa que referencia conhecimento prévio"
    fonte_dados: "learnings.md → campo 'Related' + campo 'Learned'"
  
  dashboard:
    tipo: "Tabela de padrões com contagem de reuso + gráfico de adoção"
    metricas_derivadas:
      - total_reuses: "soma de todas as reaplicações"
      - patterns_with_reuse: "quantos padrões distintos foram reusados"
      - avg_reuses_per_pattern: "média de reusos por padrão"
      - most_reused_pattern: "padrão mais reutilizado"
      - reuse_success_rate: "% de reusos com outcome = success"
      - new_patterns_discovered: "novos padrões identificados no período"
    visualizacao: >
      Tabela de padrões ordenada por contagem de reuso. Gráfico de barras
      empilhadas: primeira descoberta (azul) + reusos (verde). Linha de
      tendência: taxa de crescimento de reuso por mês.

  baseline:
    data_calculo: "2026-07-30"
    total_reuses: 12
    patterns:
      - id: "cross-agent-audit"
        nome: "Padrão de Auditoria Cross-Agent"
        descricao: "Deploy de múltiplos agentes em paralelo auditando dimensões independentes, com síntese centralizada pelo Kernel"
        descoberto_em: "L18 (2026-07-29)"
        reusos:
          - id: "B4-001"
            data: "2026-07-29"
            contexto: "Refatoração runServe — CLI coverage (L19)"
            adaptacao: "Mesmo padrão (discovery + QA + testing), foco em extrair-para-testar"
            outcome: "success"
          - id: "B4-002"
            data: "2026-07-30"
            contexto: "Coverage Audit + Doc Expurgo (L21)"
            adaptacao: "Expandido para 4 agentes (discovery + QA + testing + architecture)"
            outcome: "success"
          - id: "B4-003"
            data: "2026-07-29"
            contexto: "Systemic Platform Audit (L20)"
            adaptacao: "Escalado para 8 agentes em paralelo (10 dimensões)"
            outcome: "success"
        contagem: 3
        success_rate: 1.0
        evolucao: "v1 (single-agent) → v2 (dual-agent L19) → v3 (quad-agent L21) → v4 (octa-agent L20)"
      - id: "parallel-orchestration"
        nome: "Padrão de Orquestração Paralela em Ondas"
        descricao: "Ativação progressiva de agentes em ondas (analytical → implementation → review), com dependências resolvidas via contexto no prompt"
        descoberto_em: "Onda 2 (2026-07-28)"
        reusos:
          - id: "B4-004"
            contexto: "Onda 2: 10 agentes em 3 ondas (A/B/C)"
            outcome: "success"
          - id: "B4-005"
            contexto: "Onda 3: 9 especialistas com tasks de implementação"
            outcome: "success"
          - id: "B4-006"
            contexto: "Onda 5: 6 agentes de negócio em paralelo"
            outcome: "success"
          - id: "B4-007"
            contexto: "Onda 6: 8 agentes (liderança + órfãos) em paralelo"
            outcome: "success"
          - id: "B4-008"
            contexto: "Semantic Memory Deploy: 6 agentes em 2 fases"
            outcome: "success"
        contagem: 5
        success_rate: 1.0
      - id: "heuristics-applied"
        nome: "Heurísticas Extraídas e Aplicadas"
        descricao: "Heurísticas H-001 a H-020 extraídas de 17 agentes, aplicadas em decisões subsequentes"
        descoberto_em: "Fase C-D Evolution (2026-07-28)"
        reusos:
          - id: "B4-009"
            contexto: "H-001 (jail.go 0% coverage) → priorização de teste de segurança"
            outcome: "success"
          - id: "B4-010"
            contexto: "H-002 (threshold único) → unificação de quality gates"
            outcome: "success"
          - id: "B4-011"
            contexto: "H-009 (PostgreSQL fantasy) → cross-source doc audit"
            outcome: "success"
          - id: "B4-012"
            contexto: "H-?? (extract-then-test) → refatoração de monolitos"
            outcome: "success"
        contagem: 4
        success_rate: 1.0
    metricas_derivadas:
      patterns_with_reuse: 3
      avg_reuses_per_pattern: 4.0
      most_reused_pattern: "parallel-orchestration (5 reusos)"
      reuse_success_rate: 1.0
      new_patterns_this_period: 3

  target:
    curto_prazo: "Identificar e registrar ≥ 2 novos reusos de padrões existentes"
    medio_prazo: "Total de reusos ≥ 20 (crescimento de 67%)"
    fase_1_alvo: "≥ 15 reusos totais, ≥ 4 padrões com reuso documentado"
    fase_3_alvo: "≥ 50 reusos, ≥ 10 padrões ativos com evolução versionada"
```

**Storage**: `internal/embed/cosca/metrics/B4_reused_knowledge.md`

---

### 3.5 B5 — Problemas Detectados Antes da Falha

```yaml
metric_b5:
  id: "B5"
  nome: "Problemas Detectados Antes da Falha"
  definicao: >
    Problemas identificados e corrigidos proativamente — antes que se
    manifestassem como falha em produção, deploy, ou execução. A detecção
    pode vir de auditoria, análise de código, teste automatizado, sistema
    imunológico cognitivo, ou verificação cruzada (docs vs código).
    Contrasta com problemas detectados reativamente (após falha).
  unidade: "contagem de detecções proativas"
  
  schema_entrada:
    id: string                       # Identificador único (ex: "B5-2026-07-29-001")
    data_deteccao: date              # Data em que o problema foi detectado
    learning_ref: string             # Referência ao learning associado
    problema: string                 # Descrição do problema
    severidade: string               # Baixa, Média, Alta, Crítica
    categoria: string                # Tipo: inconsistency, stale_config, missing_coverage, fiction_doc, security_gap, architectural_risk
    mecanismo_deteccao: string       # Como foi detectado: cross_audit, linting, testing, immune_system, semantic_analysis, git_analysis
    lead_time: string                # Quanto tempo antes da provável falha (ex: "semanas", "antes do deploy", "durante auditoria")
    impacto_evitado: string          # O que teria acontecido se não detectado
    acao_preventiva: string          # O que foi feito para corrigir
    verificacao: string              # Como foi verificado que o problema foi resolvido
  
  coleta:
    metodo: "Manual (via learnings.md) + automático (via audit hooks)"
    descricao: >
      Detecções proativas são identificadas nos learnings quando o campo
      'Learned' menciona descoberta de problema antes de falha. Exemplos:
      "Kernel detectou que...", "auditoria revelou...", "cross-source check
      encontrou...". O tracking também será alimentado futuramente pelo
      Cognitive Immune System (C5, Fase 2) como fonte automatizada.
    frequencia: "A cada detecção registrada em learning"
    fonte_dados: "learnings.md → campo 'Learned' → menções de detecção proativa"
  
  dashboard:
    tipo: "Registro cronológico + indicador de proporção proativo/reativo"
    metricas_derivadas:
      - proactive_count: "total de detecções proativas"
      - reactive_count: "total de detecções reativas (após falha)"
      - proactive_ratio: "proactive / (proactive + reactive)"
      - avg_lead_time: "lead time médio das detecções"
      - detections_by_mechanism: "distribuição por mecanismo de detecção"
      - high_severity_prevented: "contagem de detecções de severidade Alta ou Crítica"
    visualizacao: >
      Tabela cronológica com cor por mecanismo de detecção. Gráfico de pizza:
      proativo vs reativo (meta: inverter a proporção para >70% proativo).
      Gráfico de barras: detecções por mecanismo ao longo do tempo.

  baseline:
    data_calculo: "2026-07-30"
    proactive_count: 5
    reactive_count: 5             # Estimativa: jail breach, 20 race conditions, schema bugs, etc.
    proactive_ratio: 0.50         # 5/10 — metade proativo, metade reativo
    deteccoes:
      - id: "B5-2026-07-29-001"
        data: "2026-07-29"
        learning_ref: "L20"
        problema: "3 documentos de arquitetura descrevem infraestrutura inexistente (K8s, PostgreSQL, Kafka)"
        severidade: "Alta"
        categoria: "fiction_doc"
        mecanismo_deteccao: "cross_audit"
        lead_time: "semanas (docs existiam há semanas sem ninguém notar)"
        impacto_evitado: "Agentes tomando decisões de arquitetura baseadas em infraestrutura que não existe — ex: escolher PostgreSQL como banco quando só SQLite está disponível"
        acao_preventiva: "12 arquivos expurgados (-315 linhas de ficção). Docs marcados como FICTÍCIOS. Padrão cross-source estabelecido."
        verificacao: "Docs restantes verificados contra go.mod e estrutura de diretórios real"
      - id: "B5-2026-07-29-002"
        data: "2026-07-29"
        learning_ref: "L18"
        problema: "Threshold crisis: 4 valores diferentes para o mesmo coverage gate (Makefile 40%, CI 55%, docs 70%, embed 80%)"
        severidade: "Crítica"
        categoria: "inconsistency"
        mecanismo_deteccao: "cross_audit"
        lead_time: "antes do próximo deploy (CI teria falhado ou passado incorretamente)"
        impacto_evitado: "CI executando threshold errado (55%) quando baseline real era 78%. Deploy poderia ser bloqueado ou passar com cobertura falsa."
        acao_preventiva: "Threshold unificado para 70% statement + 60% branch. 3 docs residuais pendentes de atualização."
        verificacao: "CI atualizado. Correção documental em andamento (L21)."
      - id: "B5-2026-07-29-003"
        data: "2026-07-29"
        learning_ref: "L12"
        problema: "opencode.json permission paths estavam stale — apontando para diretório de outro usuário (henrique) em path diferente"
        severidade: "Alta"
        categoria: "stale_config"
        mecanismo_deteccao: "security_audit"
        lead_time: "semanas (paths estavam errados desde a instalação inicial)"
        impacto_evitado: "Permissões não funcionariam no ambiente real — agentes poderiam acessar (ou não acessar) paths errados, criando brecha de segurança"
        acao_preventiva: "Paths corrigidos para workspace do Don. Permissões restritas com least privilege (read/glob/grep restritos ao workspace)."
        verificacao: "Don aprovou correção direta. Permission model validado."
      - id: "B5-2026-07-29-004"
        data: "2026-07-29"
        learning_ref: "Semantic Memory — C1 Indexing"
        problema: "PostgreSQL fantasy pattern: múltiplos agentes mencionavam PostgreSQL como infraestrutura quando o projeto usa SQLite"
        severidade: "Média"
        categoria: "fiction_doc"
        mecanismo_deteccao: "semantic_analysis"
        lead_time: "semanas (pattern persistente em documentação de múltiplos agentes)"
        impacto_evitado: "Novos agentes continuariam propagando a ficção PostgreSQL, contaminando documentação futura"
        acao_preventiva: "Heurística H-009 extraída. Padrão documentado no semantic memory como 'PostgreSQL fantasy'."
        verificacao: "Índice semântico atualizado. Heurística disponível para todos os agentes."
      - id: "B5-2026-07-29-005"
        data: "2026-07-29"
        learning_ref: "Embed Sync Investigation"
        problema: "MEMORY_MODEL.md com 44 linhas de drift entre internal/embed/cosca/ (source of truth) e internal/embed/cosca/ (embedded copy)"
        severidade: "Alta"
        categoria: "inconsistency"
        mecanismo_deteccao: "git_analysis"
        lead_time: "antes do próximo build (binário teria embed desatualizado, repetindo padrão do jail breach)"
        impacto_evitado: "Novo cosca init --force regredindo o framework novamente, já que o binário conteria templates desatualizados"
        acao_preventiva: "make embed-sync executado. Regra P8 reforçada. CI validation para drift detection pendente."
        verificacao: "Embed sincronizado. 33 arquivos do Knowledge Pipeline copiados para o embed."
    categorias_detectadas:
      - inconsistency: 2
      - fiction_doc: 2
      - stale_config: 1
    mecanismos_utilizados:
      - cross_audit: 2
      - security_audit: 1
      - semantic_analysis: 1
      - git_analysis: 1
    lead_time_medio: "semanas"

  target:
    curto_prazo: "Manter ≥ 1 detecção proativa por semana"
    medio_prazo: "Proactive ratio ≥ 70% (inverter de 50/50 para 70/30)"
    fase_1_alvo: "≥ 10 detecções proativas acumuladas"
    fase_3_alvo: "≥ 80% proactive ratio com Cognitive Immune System (C5) como fonte automatizada"
```

**Storage**: `internal/embed/cosca/metrics/B5_prefailure_detection.md`

---

## 4. Integração com o Cognitive Maturity Index (CMI)

Cada métrica real alimenta o cálculo do CMI através das dimensões correspondentes:

```
┌──────────────────────────────────────────────────────────────────┐
│            INTEGRAÇÃO: MÉTRICAS REAIS → DIMENSÕES CMI            │
│                                                                   │
│  B1 (Autonomia)                                                  │
│    │                                                              │
│    ├──► Julgamento (25%): autonomia_ratio × julgamento_weight    │
│    └──► Autocrítica (15%): intervenções_reconhecidas × weight    │
│                                                                   │
│  B2 (Previsões)                                                  │
│    │                                                              │
│    ├──► Julgamento (25%): accuracy_rate × julgamento_weight      │
│    └──► Consistência (10%): time_estimation_error × weight       │
│                                                                   │
│  B3 (Decisões Revertidas)                                        │
│    │                                                              │
│    ├──► Julgamento (25%): (1 - reversions_ratio) × weight        │
│    └──► Autocrítica (15%): mean_time_to_detect × weight          │
│                                                                   │
│  B4 (Conhecimento Reutilizado)                                   │
│    │                                                              │
│    ├──► Transferência (15%): reuse_growth_rate × weight          │
│    └──► Aprendizado (20%): patterns_with_reuse × weight          │
│                                                                   │
│  B5 (Detecção Preventiva)                                        │
│    │                                                              │
│    ├──► Planejamento (15%): proactive_ratio × weight             │
│    └──► Consistência (10%): avg_lead_time × weight              │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 5. Infraestrutura de Tracking

### 5.1 Estrutura de Diretórios

```
internal/embed/cosca/metrics/
├── CMI_REAL_METRICS.md              # Este documento — especificação
├── B1_autonomous_problems.md        # Tracking B1
├── B2_predictions.md                # Tracking B2
├── B3_reverted_decisions.md         # Tracking B3
├── B4_reused_knowledge.md           # Tracking B4
├── B5_prefailure_detection.md       # Tracking B5
└── dashboard/                       # (Futuro) Visualizações e relatórios
    └── weekly-report.md             # Relatório semanal de métricas
```

### 5.2 Protocolo de Atualização

```yaml
update_protocol:
  trigger: "Após cada tarefa concluída pelo Kernel"
  responsible: "cosca-monitoring (B1-B5), cosca-analytics (dashboard)"
  
  steps:
    - step: 1
      action: "Agente executor registra outcome no learnings.md"
      output: "Nova entrada de learning"
    
    - step: 2
      action: "Agente executor avalia se houve intervenção do Don"
      condition: "Se sim → registrar em B1 com intervention_reason"
      output: "Atualização B1_autonomous_problems.md"
    
    - step: 3
      action: "Se houve previsão pré-task, comparar com resultado real"
      condition: "Se sim → registrar em B2 com acurácia calculada"
      output: "Atualização B2_predictions.md"
    
    - step: 4
      action: "Se houve reversão de decisão, classificar causa raiz"
      condition: "Se sim → registrar em B3 + failures.md"
      output: "Atualização B3_reverted_decisions.md + failures.md"
    
    - step: 5
      action: "Se houve reuso de padrão/heurística, documentar referência"
      condition: "Se sim → registrar em B4 com link ao conhecimento fonte"
      output: "Atualização B4_reused_knowledge.md"
    
    - step: 6
      action: "Se houve detecção proativa, documentar mecanismo e lead time"
      condition: "Se sim → registrar em B5"
      output: "Atualização B5_prefailure_detection.md"
    
    - step: 7
      action: "Atualizar CMI com base nas novas métricas (via CMI formula)"
      output: "CMI recalculado"
```

### 5.3 Automação Futura

```
Fase 1 (atual):
  ──► Coleta manual pós-task (o agente registra nos tracking files)
  ──► Dashboard: tabela markdown simples

Fase 2 (automated):
  ──► Hook Stage 8 do metacognition pipeline dispara coleta automática
  ──► Dashboard: SQLite-backed queries com série temporal
  ──► Alertas: B3 (nova reversão) → notificação imediata ao Don

Fase 3 (predictive):
  ──► Cognitive Immune System alimenta B5 automaticamente
  ──► ML model prevê B2 accuracy baseado em contexto da task
  ──► Dashboard: visualização em tempo real no cosca serve
```

---

## 6. Dashboard Consolidado (Template)

### 6.1 Relatório Semanal de Métricas Reais

```markdown
# Real Metrics Dashboard — Semana YYYY-MM-DD

## B1 — Autonomia
- Tasks esta semana: N
- Autônomas: N (XX%)
- Streak atual: N dias
- Intervenções: N (motivo)

## B2 — Previsões
- Previsões esta semana: N
- Corretas: N (XX%)
- Tendência: ↑↓→ vs semana anterior

## B3 — Decisões Revertidas
- Novas reversões: N
- Categorias afetadas: [lista]
- Acumulado total: N

## B4 — Conhecimento Reutilizado
- Reusos esta semana: N
- Padrões ativos: N
- Destaque: [padrão mais reutilizado]

## B5 — Detecção Preventiva
- Detecções esta semana: N
- Proativo vs Reativo: XX% / XX%
- Lead time médio: [dias/semanas]

## CMI Projetado
- CMI atual: XX.X
- Variação semanal: +0.XX
- Projeção Fase 1: YY.Y
```

---

## 7. Glossário de Métricas Derivadas

| Métrica Derivada | Fórmula | Unidade | Utilizada Em |
|------------------|---------|---------|-------------|
| **autonomia_ratio** | tasks_autonomous / tasks_total | % | B1 → Julgamento CMI |
| **accuracy_rate** | previsoes_corretas / total_previsoes | % | B2 → Julgamento CMI |
| **reversions_trend** | reversões por mês (slope) | taxa | B3 → Autocrítica CMI |
| **reuse_growth_rate** | (reusos_mes_atual - reusos_mes_anterior) / reusos_mes_anterior | % | B4 → Transferência CMI |
| **proactive_ratio** | deteccoes_proativas / (proativas + reativas) | % | B5 → Planejamento CMI |
| **avg_lead_time** | Σ(lead_time) / total_deteccoes | dias | B5 → Consistência CMI |
| **prevention_coverage** | categorias_com_regra / total_categorias_reversao | % | B3 → Julgamento CMI |
| **reuse_success_rate** | reusos_com_sucesso / total_reusos | % | B4 → Aprendizado CMI |

---

## 8. Referências

| Documento | Relação |
|-----------|---------|
| `internal/embed/cosca/architecture/COGNITIVE_MATURITY.md` | Define as 5 métricas na Seção 3 |
| `internal/embed/cosca/workflows/cognitive-maturity-implementation.md` | Tarefa F1.5 — Implementação das métricas |
| `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` | Fonte primária de baseline (26 entradas) |
| `internal/embed/cosca/memory/failures.md` | Fonte de dados para B3 |
| `internal/embed/cosca/workflows/metacognition-pipeline.md` | Stage 8 — Hook de automação futura |

---

> **Próximo passo**: Popular os 5 arquivos de tracking com os dados de baseline extraídos de learnings.md.
> **Kernel instruction**: `cosca-monitoring update-metrics --baseline --from-learnings`
