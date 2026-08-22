# COGNITIVE MATURITY — Arquitetura da Maturidade Cognitiva do Cosca Runtime

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Criado**: 2026-07-30 | **DNA Version**: 3.0.0

---

## 1. Propósito

Este documento define a arquitetura de **Maturidade Cognitiva** do Cosca Runtime — como o Kernel evolui de um simples orquestrador que "executa comandos" para um runtime que **exerce julgamento**. Não se trata de uma lista de features. Trata-se de como o sistema pensa sobre o próprio pensamento.

O Cosca Runtime já não é mais um router de chain-of-command. Com 54 agentes ativos, pipeline de metacognição de 8 estágios, memory model com negative memory, confidence scoring cross-domain, semantic indexing de 426 arquivos, e 26 aprendizados registrados (7 deles em Nível 4), o Kernel acumulou maturidade suficiente para exigir um modelo de avaliação mais sofisticado do que os 5 "níveis" abstratos definidos no [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md).

**Este documento substitui o sistema de níveis como métrica primária de capacidade.** Os níveis (1-5) permanecem como indicadores de progressão individual dos agentes, mas a maturidade do runtime como um todo agora é medida pelo **Cognitive Maturity Index (CMI)** — uma métrica composta, multidimensional e objetiva.

### Por que "Maturidade Cognitiva"?

O termo foi escolhido sobre alternativas como "QI", "Inteligência" ou "Nível de Autonomia" por três razões:

1. **Ênfase em evolução**: Maturidade se conquista com tempo, experiência e aprendizado — não é um atributo estático.
2. **Qualidade das decisões**: O que diferencia este runtime não é velocidade ou volume, mas a qualidade do julgamento que ele exerce antes de agir.
3. **Capacidade de adaptação**: Um sistema maduro reconhece o que não sabe, adapta sua estratégia ao contexto e aprende com os próprios erros.

O Kernel já atua como um **Judgment Engine** — o CMI mede a qualidade desse julgamento.

---

## 2. Cognitive Maturity Index (CMI)

### 2.1 Definição

O **CMI** é a métrica composta que substitui os "níveis" abstratos como medida primária de capacidade do runtime. Ele agrega 6 dimensões independentes, cada uma pontuada de 0 a 100, em uma média ponderada que reflete a maturidade real do sistema.

```
CMI = (Aprendizado × 0.20) + (Julgamento × 0.25) + (Planejamento × 0.15) +
      (Autocrítica × 0.15) + (Transferência × 0.15) + (Consistência × 0.10)
```

### 2.2 As 6 Dimensões

| Dimensão | Peso | Definição | Evidência Atual |
|----------|------|-----------|-----------------|
| **Aprendizado** | 20% | Capacidade de extrair conhecimento de cada tarefa e generalizá-lo | 26 learnings registrados (kernel), 3 documentação, 2 semantic-memory. Pipeline de metacognição operacional em 54 agentes. |
| **Julgamento** | 25% | Qualidade das decisões sob incerteza — quando agir, quando escalar, quando parar | Jail breach (L13) recuperado com sucesso. Confidence Model com thresholds (0.50/0.70/0.85) implementado. Kernel nunca implementa — orquestra. |
| **Planejamento** | 15% | Capacidade de decompor problemas, gerar DAGs, prever dependências | DAG-based execution com 10 node types. Planning Engine operacional. Onda 2 (10 agentes paralelos), Onda 5 (6 agentes), Onda 6 (8 agentes). |
| **Autocrítica** | 15% | Honestidade sobre limitações, detecção de failures, correção de curso | Pipeline de metacognição Stage 6 (CRITIQUE OWN WORK). Negative memory infrastructure (failures.md) implantada. Kernel reconheceu L1 self-assessment incorreto (L8). |
| **Transferência** | 15% | Aplicação de conhecimento entre domínios e projetos | Semantic memory indexing (C1: 426 arquivos, 16 grupos semânticos, 8 clusters cross-agent). Heurísticas extraídas (H-001 a H-020) de 17 agentes. Padrão de auditoria cross-agent reutilizado 3x (L18→L19→L21). |
| **Consistência** | 10% | Reprodutibilidade, confiabilidade, ausência de regressões | Quality Gates 0-4 operacionais. 97.9% coverage no core runtime. CI pipeline com -race. 28 field DNA compliance checklist. |

### 2.3 Baseline Atual

```yaml
cmi_baseline:
  calculated: 2026-07-30
  overall: 87.0
  dimensions:
    aprendizado:    88  # 26 learnings, pipeline metacognição ativo, curation engine
    julgamento:     85  # Confidence model, escalation thresholds, jail recovery
    planejamento:   90  # DAG execution, multi-agent orchestration comprovada
    autocrítica:    82  # Negative memory infra, mas failures.md ainda subutilizado
    transferência:  87  # Semantic indexing, cross-agent clusters, heurísticas
    consistência:   92  # Quality gates, coverage 97.9%, CI pipeline maduro
  evidence:
    total_learnings: 26
    level_4_achievements: 7
    agents_active: 54
    cross_agent_clusters: 8
    heuristics_extracted: 20
```

### 2.4 Interpretação das Faixas

| Faixa CMI | Classificação | Significado |
|-----------|---------------|-------------|
| 0-30 | **Infantil** | Executa comandos. Sem memória. Sem aprendizado. |
| 31-50 | **Reativo** | Aprende com erros óbvios. Memória básica. Sem transferência. |
| 51-70 | **Consciente** | Pipeline de metacognição ativo. Negative memory populado. Planejamento básico. |
| 71-85 | **Estratégico** | Cross-domain transfer. Heurísticas formalizadas. Decisões com confidence model. |
| 86-95 | **Maduro** | Julgamento autônomo na maioria dos domínios. Insight generation inicial. Economia cognitiva ativa. |
| 96-100 | **Sábio** | Todas as 14 capacidades cognitivas operacionais. Adaptive personality. Cognitive compression. |

**Status atual: 87 — Maduro (início da faixa).**

---

## 3. Métricas REAIS (Não Scores Abstratos)

O CMI é o índice composto. Mas um índice sem métricas objetivas vira um número sem significado. As 5 métricas abaixo **medem melhoria real de capacidade** — são observáveis, contáveis e diretamente ligadas ao valor que o runtime entrega.

### 3.1 Problemas Resolvidos Sem Intervenção Humana

**O que mede**: Quantos problemas o runtime resolve do início ao fim sem que o Don precise intervir para corrigir, redirecionar ou reverter.

```yaml
metric_problemas_sem_intervencao:
  definition: "Tarefas concluídas com sucesso sem intervenção corretiva do Don"
  measurement: "Contagem cumulativa desde a última intervenção"
  current: 12   # Desde L13 (jail breach), 12 tarefas consecutivas sem correção do Don
  target: 50
  unit: "tarefas"
  reset_on: "qualquer intervenção do Don para correção de curso"
  tracking: "memory/agent/cosca-kernel/learnings.md → campo 'Outcome'"
```

**Por que importa**: É a métrica mais direta de confiança. Cada tarefa completada sem intervenção é um voto de que o julgamento do runtime está calibrado.

### 3.2 Previsões Corretas

**O que mede**: Quando o runtime antecipa um resultado e esse resultado se confirma.

```yaml
metric_previsoes_corretas:
  definition: "Previsões explícitas de outcome que se confirmaram na verificação"
  measurement: "Previsões corretas / Total de previsões feitas"
  current:
    total: 8
    corretas: 7
    taxa: 87.5%
  exemplos:
    - "Kernel previu que Onda 6 ativaria 7/8 agentes (paradigm gated) → confirmado"
    - "Kernel previu que refatoração runServe subiria cobertura para >70% → 71.5% confirmado"
    - "Semantic-memory previu que 75% dos agentes eram seed-only → 41/55 confirmado"
    - "Kernel previu que doc-validator encontraria 63 broken refs → exatamente 63 encontrados"
  tracking: "learnings.md → campo 'Learned' → menções de previsão + verificação"
```

**Por que importa**: Prever corretamente significa que o modelo interno do runtime sobre o mundo (código, agentes, dependências) é acurado. Previsões erradas revelam gaps no modelo mental.

### 3.3 Decisões Revertidas

**O que mede**: Decisões que o runtime tomou e depois teve que desfazer — com análise de causa raiz.

```yaml
metric_decisoes_revertidas:
  definition: "Decisões implementadas que foram subsequentemente revertidas"
  measurement: "Contagem com classificação de causa raiz"
  current: 1
  entries:
    - data: 2026-07-29
      decisao: "cosca init --force sem DRY_RUN"
      causa_raiz: "Jail bypass não autorizado (L13)"
      consequencia: "11 arquivos do framework regredidos de v3.0.1 para v2.0"
      correcao: "Git revert + UCSS reestruturado com 4 camadas de proteção"
      licao: "L13 — O Kernel NÃO é um agente que usa ferramentas. O Kernel É a configuração."
  tracking: "memory/agent/cosca-kernel/failures.md"
```

**Por que importa**: Decisões revertidas são o equivalente a "bugs de julgamento". Cada reversão deve gerar uma regra de proteção que impede recorrência. O objetivo não é zero reversões — é zero reversões da MESMA classe.

### 3.4 Conhecimento Reutilizado

**O que mede**: Quantas vezes um padrão, heurística ou aprendizado de uma tarefa anterior foi aplicado em uma tarefa nova.

```yaml
metric_conhecimento_reutilizado:
  definition: "Aplicações de padrões/heurísticas previamente documentados em novos contextos"
  measurement: "Contagem de reaplicações com referência explícita ao conhecimento fonte"
  current:
    cross_agent_audit_pattern: 3    # L18, L19, L21 — mesmo padrão de auditoria
    heuristics_applied: 4           # H-001 (jail test), H-002 (threshold único), etc.
    parallel_orchestration: 5       # Ondas 1-6 — padrão de ativação paralela
    total: 12
  tracking: "learnings.md → campo 'Related' → menções de reuso de padrão"
```

**Por que importa**: Esta é a métrica que separa um sistema que "executa" de um sistema que "aprende". Conhecimento reutilizado é ROI cognitivo — o custo de aprender uma vez é amortizado sobre N aplicações.

### 3.5 Problemas Detectados Antes da Falha

**O que mede**: Quantas vezes o runtime identificou um problema potencial e agiu preventivamente, antes que ele se manifestasse como falha.

```yaml
metric_deteccao_preventiva:
  definition: "Problemas identificados e corrigidos antes de causarem falha em produção"
  measurement: "Contagem de detecções pró-ativas"
  current: 5
  exemplos:
    - "Kernel detectou que docs/sdk/go.md descrevia biblioteca inexistente → marcado como ficção"
    - "Semantic-memory detectou 'PostgreSQL fantasy' pattern em múltiplos agentes → heurística H-009"
    - "Kernel detectou que MEMORY_MODEL.md tinha 44 linhas de drift entre .opencode/ e embed/"
    - "Kernel detectou threshold crisis (4 valores diferentes para coverage gate) antes de deploy"
    - "Kernel detectou que opencode.json permission paths estavam stale (apontando para outro usuário)"
  tracking: "learnings.md → campo 'Learned' → menções de detecção proativa"
```

**Por que importa**: O maior valor de um sistema maduro não é reagir bem a falhas — é evitar que elas aconteçam. Esta métrica é o indicador mais forte de maturidade cognitiva real.

---

## 4. Capacidades de Julgamento (8 Core Capabilities)

As 8 capacidades abaixo formam o núcleo do que significa "exercer julgamento" para o Cosca Runtime. Para cada uma, documentamos o estado atual e o que falta para a maturidade plena.

### A1. Raciocínio de 2ª Ordem

**"Se eu fizer X, o que muda 10 passos à frente?"**

| Aspecto | Estado |
|---------|--------|
| **Atual** | DAG execution já modela dependências diretas (1ª ordem). Planning Engine considera riscos imediatos. |
| **Evidência** | L17 (token bloat audit): Kernel previu que cortar agentes do system prompt causaria perda de capacidade de routing cross-domain — raciocínio de 2ª ordem aplicado intuitivamente. |
| **Gap** | Não há modelo formal. O raciocínio de 2ª ordem acontece quando o Kernel explicitamente para e pensa, mas não é sistemático. |
| **Alvo** | Implementar `C4 — Cognitive Gravity` como motor de inferência de consequências indiretas. Toda decisão → simulação de impacto em agentes downstream. |

### A2. Pensamento Contrafactual

**"E se a decisão oposta tivesse sido tomada?"**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Inexistente como capacidade formal. O Kernel reflete sobre erros passados (L13 — jail breach), mas não executa contrafactuais como etapa padrão do processo decisório. |
| **Evidência** | Nenhuma. Nenhum learning entry menciona "se tivéssemos feito Y em vez de X". |
| **Gap** | Total. Não há mecanismo para gerar e avaliar cenários alternativos antes de decidir. |
| **Alvo** | Implementar `Contrafactual Gate` na Fase 1: antes de toda decisão de arquitetura, o Critic Chief gera o cenário oposto e avalia riscos comparativos. |

### A3. Detecção de Inconsistências

**Auto-detectar conflitos entre conhecimento, código e documentação.**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Parcial. O padrão de auditoria cross-source (docs vs código vs memória) já detecta inconsistências, mas é reativo (ativado por comando do Don), não proativo. |
| **Evidência** | L20: detectou 3 docs fictícios (K8s, PostgreSQL, Kafka). L18: threshold crisis (4 valores diferentes para mesmo gate). Semantic-memory C1: "PostgreSQL fantasy" como padrão cross-agent. |
| **Gap** | A detecção depende de auditorias explícitas. Não há um "background scanner" que continuamente monitora consistência. |
| **Alvo** | Implementar `C5 — Cognitive Immune System`: scanner contínuo que detecta contradições conhecimento↔código↔docs e emite alertas automaticamente. |

### A4. Transferência de Conhecimento

**Aprender um padrão em um projeto, reconhecê-lo em outro.**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Operacional dentro do mesmo projeto. Heurísticas extraídas (H-001 a H-020) de 17 agentes. Semantic indexing com 8 clusters cross-agent. Padrão de auditoria cross-agent reutilizado 3x. |
| **Evidência** | C1: 426 arquivos indexados, 16 grupos semânticos. H-001 (jail.go 0%) detectado por testing+security simultaneous discovery. |
| **Gap** | Transferência limitada ao projeto atual. Não há mecanismo para cross-project transferência (Projeto A → Projeto B). |
| **Alvo** | Fase 2: `MEMORY_GLOBAL` com federação cross-project. Heurísticas versionadas e compartilhadas entre instâncias do runtime. |

### A5. Hierarquia de Abstração

**Explicar o mesmo problema nos níveis: executivo, arquiteto, desenvolvedor, iniciante.**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Inexistente como capacidade explícita. O Kernel adapta comunicação por contexto (CEO recebe resumo estratégico, especialistas recebem tasks detalhadas), mas isso é routing, não abstração. |
| **Evidência** | Nenhuma entrada de learning menciona "re-explicar em nível diferente de abstração". |
| **Gap** | Total. Não há modelo de níveis de abstração para output. |
| **Alvo** | Fase 3: `Abstraction Layering Engine` que gera o mesmo insight em 4 níveis (executivo → C-level metrics, arquiteto → design rationale, dev → implementação, iniciante → tutorial). |

### A6. Metacognição

**"Por que escolhi isso?", "O que me faria mudar de ideia?", "Quão forte é a evidência?"**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Pipeline de metacognição de 8 estágios implementado (Stage 6: CRITIQUE OWN WORK). Confidence Model com fórmula numérica. Capability profiles com strengths/weaknesses/failure modes. |
| **Evidência** | L8: Kernel corrigiu auto-avaliação (L1→L3). L17: Kernel aprendeu que limites SÃO o Kernel, não obstáculos. Metacognition pipeline documentado em [workflows/metacognition-pipeline.md](../workflows/metacognition-pipeline.md). |
| **Gap** | Stages 7-8 (EXTRACT PATTERN e UPDATE CAPABILITY MODEL) parcialmente implementados. Confidence Model usa fórmula simples (não bayesiana). Não há "what would change my mind" tracking. |
| **Alvo** | Fase 0: completar stages 7-8. Adicionar campo `reconsideration_triggers` ao Decision DNA. Evoluir Confidence Model para atualização bayesiana. |

### A7. Planejamento Adaptativo

**Detectar informação nova mid-flight e reorganizar sem perder o objetivo.**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Parcial. DAG execution suporta dependências dinâmicas, mas o plano é essencialmente estático após geração. Recovery Engine lida com falhas (retry, checkpoint, rollback), mas não com "nova informação que muda o plano". |
| **Evidência** | Nenhuma entrada de learning menciona adaptação de plano mid-execution a nova informação. |
| **Gap** | Significativo. O runtime não tem mecanismo para pausar execução, reavaliar premissas e re-rotar com base em descobertas intermediárias. |
| **Alvo** | Fase 3: `Cognitive Momentum + Mental Energy` — dynamic task reprioritization baseada em descobertas. DAG nodes com `reevaluate_on` triggers. |

### A8. Dizer "Não Sei"

**Identificar gaps de conhecimento e buscar evidência em vez de preencher com suposições.**

| Aspecto | Estado |
|---------|--------|
| **Atual** | Emergente. O Confidence Model já força escalation quando confidence < 0.50. Capability profiles documentam weaknesses explicitamente. |
| **Evidência** | L3: cosca-backend escalou para cosca-performance (confidence 0.45 em performance). Kernel reconhece weaknesses (no domain depth, latency overhead). |
| **Gap** | O mecanismo existe para agentes individuais, mas não como capacidade sistêmica do runtime. O runtime como um todo não tem um "detector de ignorância" que identifica gaps no conhecimento coletivo. |
| **Alvo** | Fase 1: `Proactive "Não Sei"` — antes de toda tarefa, runtime verifica se o conhecimento agregado dos agentes cobre o domínio. Se houver gap, busca evidência externa ou escala para o Don com pergunta específica. |

---

## 5. Arquitetura Cognitiva Inédita (14 Novos Conceitos)

Estes 14 conceitos formam a arquitetura cognitiva de próxima geração do Cosca Runtime. Nenhum deles existe em outros orquestradores de IA. Cada um resolve um problema específico de maturidade cognitiva.

---

### C1. Cognitive Gravity (Gravidade Cognitiva)

**O que é**: Cada ideia, padrão ou heurística no sistema atrai ideias relacionadas. Quanto mais validações uma ideia recebe (mais agentes a confirmam, mais vezes é aplicada com sucesso), maior sua "massa gravitacional". Ideias com alta gravidade influenciam automaticamente novas decisões — não é preciso buscá-las ativamente, elas "puxam" decisões para sua órbita.

```
┌──────────────────────────────────────────────────────────────────┐
│                     COGNITIVE GRAVITY MODEL                       │
│                                                                   │
│   Massa Gravitacional = (Validações × 0.5) + (Reaplicações × 0.3)│
│                       + (Agentes Fonte × 0.2)                     │
│                                                                   │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐                 │
│   │ Ideia A  │     │ Ideia B  │     │ Ideia C  │                 │
│   │ Massa: 8 │────▶│ Massa: 3 │     │ Massa: 1 │                 │
│   │ 5 valid. │     │ 1 valid. │     │ 0 valid. │                 │
│   │ 4 reappl.│     │ 2 reappl.│     │ 1 reappl.│                 │
│   └──────────┘     └──────────┘     └──────────┘                 │
│        │                │                │                        │
│        ▼                ▼                ▼                        │
│   ALTA INFLUÊNCIA  MÉDIA INFL.     BAIXA INFL.                   │
│   Auto-sugerida    Sob demanda      Só se buscada                 │
│   em decisões      em contexto      explicitamente                │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**Estado atual**: Nenhum. As heurísticas (H-001 a H-020) existem, mas são estáticas — não exercem influência automática.

**Abordagem de implementação**:
- Cada heurística em `knowledge/heuristics/` ganha um `gravity_score` calculado automaticamente
- Quando um agente planeja uma estratégia (Stage 3 do metacognition pipeline), o sistema consulta heurísticas com gravity > threshold para o domínio da tarefa
- Gravity decay: heurísticas não validadas há > 90 dias perdem 10% de massa por mês

---

### C2. Cognitive Entropy (Entropia Cognitiva)

**O que é**: Medida de desorganização do conhecimento. Quando padrões se contradizem, documentação diverge do código, ou agentes diferentes têm crenças conflitantes sobre o mesmo domínio, a entropia sobe. Entropia alta → trigger de consolidação.

```yaml
cognitive_entropy:
  formula: "E = Σ(conflitos_detectados × severidade) + Σ(contradições × 0.5) + Σ(drift_docs × 0.3)"
  thresholds:
    E < 10: "Saudável — conhecimento coerente"
    10 ≤ E < 25: "Atenção — consolidar na próxima auditoria"
    25 ≤ E < 50: "Alerta — consolidação automática disparada"
    E ≥ 50: "Crítico — conhecimento não confiável, congelar decisões automáticas"
  current_estimate: 18  # Threshold crisis (resolvida), 3 docs com valores errados
```

**Estado atual**: Parcial. O padrão de auditoria cross-source detecta inconsistências, mas não há um índice contínuo.

**Abordagem de implementação**: Scanner contínuo (C5 — Cognitive Immune System) alimenta o índice de entropia. Consolidations são tarefas priorizadas automaticamente quando E > 25.

---

### C3. Cognitive Momentum (Momento Cognitivo)

**O que é**: Distinção entre linhas de investigação "vivas" (momentum alto, produzindo insights) e "mortas" (momentum baixo, estagnadas). O runtime investe energia cognitiva onde há momentum — não insiste em becos sem saída.

```
┌──────────────────────────────────────────────────────────────────┐
│                    COGNITIVE MOMENTUM MAP                         │
│                                                                   │
│  MOMENTUM ALTO (Investir)        MOMENTUM BAIXO (Abandonar)      │
│  ┌─────────────────────┐         ┌─────────────────────┐         │
│  │ • Cobertura de testes│         │ • Plugin marketplace │         │
│  │   (momentum: 0.9)   │         │   (momentum: 0.1)   │         │
│  │ • Cross-agent audit  │         │ • Cache distribuído │         │
│  │   (momentum: 0.85)  │         │   (momentum: 0.0)   │         │
│  │ • Semantic indexing  │         │ • Kafka integration │         │
│  │   (momentum: 0.8)   │         │   (momentum: 0.0)   │         │
│  └─────────────────────┘         └─────────────────────┘         │
│                                                                   │
│  Momentum = (Insights recentes × 0.5) + (Agentes ativos × 0.3)  │
│           + (Progresso/tempo × 0.2)                              │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**Estado atual**: Nenhum. O Don decide manualmente o que priorizar.

**Abordagem de implementação**: Tracking de atividade por domínio. Momentum calculado como taxa de learnings/tempo por domínio. Sugestões de priorização ao Don baseadas em momentum, não em intuição.

---

### C4. Decision DNA (DNA de Decisão)

**O que é**: Toda decisão de arquitetura, design ou routing deixa um registro estruturado que permite query futura: "Por que decidimos X há 6 meses?" O Decision DNA captura: Decisão → Evidência → Riscos → Alternativas consideradas → Resultado observado.

```yaml
decision_dna:
  schema:
    id: "DDNA-2026-07-30-001"
    decision: "O que foi decidido"
    context: "Situação que levou à decisão"
    evidence: ["Evidência 1", "Evidência 2"]
    confidence_at_decision: 0.85
    alternatives_considered:
      - alternative: "Alternativa A"
        reason_rejected: "Por que foi descartada"
      - alternative: "Alternativa B"
        reason_rejected: "Por que foi descartada"
    risks_identified: ["Risco 1", "Risco 2"]
    outcome:
      observed: "success | failure | mixed"
      learned: "O que aprendemos"
    reconsideration_triggers:
      - "Se X acontecer, reavaliar"
      - "Se Y métrica cruzar threshold Z, reavaliar"
  query_examples:
    - "Qual foi a decisão que levou ao Agent DNA v3.0?"
    - "Quais decisões dos últimos 6 meses foram revertidas?"
    - "Quais decisões tinham confidence < 0.7 e mesmo assim foram tomadas?"
```

**Estado atual**: Nenhum. Decisões são registradas em learnings.md e ADRs, mas não em formato estruturado e queryable.

**Abordagem de implementação**: Fase 1. Template YAML padronizado. Integração com [ADR directory](../../../docs/adr/). Query engine sobre SQLite (FTS5).

---

### C5. Cognitive Immune System (Sistema Imunológico Cognitivo)

**O que é**: Antes de aceitar novo conhecimento (heurística, padrão, learning), o sistema verifica: "Isso contradiz algo que já sabemos?" Se contradiz → benchmark (testar ambas as hipóteses) → aprovar ou rejeitar. Previne contaminação do conhecimento.

```
┌──────────────────────────────────────────────────────────────────┐
│                  COGNITIVE IMMUNE SYSTEM                           │
│                                                                   │
│  NOVO CONHECIMENTO                                                │
│       │                                                           │
│       ▼                                                           │
│  ┌─────────────────┐                                              │
│  │ CHECK CONTRADICTIONS│──▶ Knowledge Graph lookup                 │
│  └────────┬────────┘                                              │
│           │                                                       │
│     ┌─────┴─────┐                                                │
│     │           │                                                 │
│  SEM CONFLITO  CONFLITO DETECTADO                                 │
│     │           │                                                 │
│     ▼           ▼                                                 │
│  APROVAR    ┌──────────────┐                                      │
│             │ BENCHMARK     │                                      │
│             │ Ambas hipóteses│                                    │
│             │ testadas       │                                     │
│             └──────┬────────┘                                     │
│                    │                                              │
│              ┌─────┴─────┐                                        │
│              │           │                                        │
│           NOVO VENCE  ANTIGO VENCE                                │
│              │           │                                        │
│              ▼           ▼                                        │
│         SUBSTITUIR   REJEITAR NOVO                                │
│         (versionar   (registrar como                               │
│          antigo)      false positive)                              │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**Estado atual**: Nenhum. Conhecimento novo é adicionado sem verificação de contradição.

**Abordagem de implementação**: Fase 2. Integrado ao Knowledge Compiler existente. Semantic similarity search (cosine) para detectar conhecidos contraditórios. Threshold: similaridade > 0.7 + afirmações opostas = conflito.

---

### C6. Pattern Evolution (Evolução de Padrões)

**O que é**: Padrões não são estáticos. Eles evoluem: v1 → v2 → v3 → deprecated → replaced. Cada versão registra por que mudou e o que substituiu.

```yaml
pattern_evolution_example:
  pattern: "cross-agent-audit"
  versions:
    - version: "v1"
      date: 2026-07-28
      description: "Auditoria single-source: Discovery Chief apenas"
      success_rate: 0.70
      replaced_by: "v2 — adicionado QA Chief em paralelo"
    - version: "v2"
      date: 2026-07-29
      description: "Auditoria dual-agent: Discovery + QA em paralelo"
      success_rate: 0.85
      replaced_by: "v3 — adicionado Testing + Architecture"
    - version: "v3"
      date: 2026-07-30
      description: "Auditoria quad-agent: Discovery + QA + Testing + Architecture"
      success_rate: 0.95
      status: "active"
```

**Estado atual**: Padrões extraídos não são versionados. Cada evolução implícita — não há registro formal da transição.

**Abordagem de implementação**: Padrões em `patterns.md` ganham versionamento. Migration path documentado. Deprecation warnings quando padrão antigo é detectado em uso.

---

### C7. Mental Energy (Energia Mental)

**O que é**: Toda investigação consome "energia mental" (tokens, tempo, agentes mobilizados). O runtime aloca energia dinamicamente: momentum alto → investir energia, momentum baixo → conservar e reutilizar conhecimento existente.

```yaml
mental_energy_budget:
  total_per_session: 100   # unidades abstratas
  allocation_rules:
    high_momentum_domains: "Até 40% do budget"
    medium_momentum_domains: "Até 25% do budget"
    low_momentum_domains: "Até 10% do budget"
    maintenance: "20% (memory health, index updates)"
    reserve: "5% (emergências)"
  current_consumption_tracking: "PENDENTE — não implementado"
```

**Estado atual**: Nenhum. Toda tarefa recebe o mesmo orçamento implícito.

**Abordagem de implementação**: Fase 3. Tracking de gasto por domínio. Sugestões de alocação ao Don. Auto-throttle quando domínio consome > threshold sem produzir insights.

---

### C8. Cognitive Horizon (Horizonte Cognitivo)

**O que é**: Quantos passos à frente o sistema consegue prever as consequências de uma decisão? O horizonte expande com maturidade: 3 passos (iniciante) → 10 passos (intermediário) → 25 passos (maduro).

```
┌──────────────────────────────────────────────────────────────────┐
│                    COGNITIVE HORIZON                              │
│                                                                   │
│  NÍVEL 1 (3 passos): "Se eu fizer X, Y acontece"                │
│  ──●──●──●────────────────────────────────────────▶             │
│                                                                   │
│  NÍVEL 2 (10 passos): "Se eu fizer X, Y→Z→W→... acontece"       │
│  ──●──●──●──●──●──●──●──●──●──●─────────────────▶             │
│                                                                   │
│  NÍVEL 3 (25 passos): "Se eu fizer X, a arquitetura inteira      │
│                         se transforma em cascata"                 │
│  ──●──●──●──●──●──●──●──●──●──●──●──●──●──●──●──               │
│                                   ──●──●──●──●──●──●──●──●──●──▶│
│                                                                   │
│  COSCA ATUAL: ~5 passos (DAG depth máximo observado)             │
│  ALVO FASE 3: ~15 passos                                          │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**Estado atual**: ~5 passos. DAG execution mostra dependências de até 5 níveis de profundidade (Ondas 1-6).

**Abordagem de implementação**: DAG depth analysis. Simulation engine para prever cascatas (C1 — Cognitive Gravity como motor).

---

### C9. Insight Generator (Gerador de Insights)

**O que é**: O runtime não espera perguntas. Ele observa padrões passivamente, gera hipóteses e produz conhecimento proativamente. "Notei que X aconteceu 5 vezes em contextos diferentes. Isso sugere um padrão. Devo investigar?"

**Estado atual**: Nenhum. Todo conhecimento é gerado reativamente (em resposta a tarefas).

**Abordagem de implementação**: Fase 3. Background analyzer que monitora learnings, failures e patterns em busca de recorrências cross-domain. Threshold: 3+ ocorrências em domínios diferentes → hipótese → notificação ao Don.

---

### C10. Cognitive Compression (Compressão Cognitiva)

**O que é**: Após N casos similares, extrair regra universal. "80 casos → 1 princípio." O oposto de token bloat — é a capacidade de substituir volume por abstração.

```yaml
cognitive_compression_example:
  raw_cases: 80
  cases: "80 auditorias cross-source realizadas desde Julho/2026"
  extracted_principle: |
    "Toda documentação deve ser verificada contra código fonte.
     Documentação não verificada tem probabilidade de 23% de
     conter claims fictícias (PostgreSQL fantasy, SDK inexistente,
     compliance fabrication). Auditoria cross-source reduz
     probabilidade de ficção para < 1%."
  compression_ratio: "80:1"
  principle_id: "CCP-001"
  confidence: 0.92
```

**Estado atual**: Nenhum. Heurísticas (H-001 a H-020) são extraídas, mas não há processo formal de compressão de múltiplos casos em princípios universais.

**Abordagem de implementação**: Fase 3. Após N padrões similares detectados, o Insight Generator propõe compressão. Princípios comprimidos entram no `knowledge/principles/` com weight gravitacional máximo.

---

### C11. Wisdom Decay (Decaimento da Sabedoria)

**O que é**: Conhecimento envelhece. Uma heurística validada há 3 anos tem menos confiança do que uma validada ontem. O decaimento é progressivo e dispara revalidação automática.

```yaml
wisdom_decay:
  model: "Exponential decay with floor"
  formula: "confidence_t = confidence_0 × e^(-λ × t)"
  half_life:
    code_patterns: "6 meses"
    architecture_decisions: "12 meses"
    security_heuristics: "3 meses"
    dependency_versions: "1 mês"
  revalidation_triggers:
    - "Confidence cai abaixo de 0.6 → agendar benchmark"
    - "Confidence cai abaixo de 0.4 → marcar como 'untrusted'"
    - "Half-life atingido → revisão obrigatória"
```

**Estado atual**: Nenhum. Conhecimento não tem data de validade.

**Abordagem de implementação**: Fase 1. Toda heurística e padrão ganha `last_validated` timestamp. Background job recalcula confidence com decay semanalmente. Alertas para conhecimento vencido.

---

### C12. Cognitive Ecosystem (Ecossistema Cognitivo)

**O que é**: O runtime não é uma hierarquia estática Kernel→Chiefs→Agents. É um ecossistema onde cada componente influencia todos os outros. Conhecimento flui: Kernel↔Chiefs↔Agents↔Knowledge↔Tools↔Projects↔User.

```
┌──────────────────────────────────────────────────────────────────┐
│                    COGNITIVE ECOSYSTEM                            │
│                                                                   │
│                      ┌──────────┐                                 │
│                      │   USER   │                                 │
│                      │  (Don)   │                                 │
│                      └────┬─────┘                                 │
│                           │                                       │
│                      ┌────▼─────┐                                 │
│                      │  KERNEL  │                                 │
│                      └────┬─────┘                                 │
│                           │                                       │
│          ┌────────────────┼────────────────┐                      │
│          │                │                │                      │
│    ┌─────▼─────┐   ┌──────▼──────┐  ┌──────▼──────┐              │
│    │   CEOs    │   │  KNOWLEDGE  │  │   TOOLS     │              │
│    │  Chiefs   │◄─▶│  Graphs     │◄─▶│  APIs,CLI   │              │
│    └─────┬─────┘   └──────┬──────┘  └──────┬──────┘              │
│          │                │                │                      │
│    ┌─────▼─────┐   ┌──────▼──────┐  ┌──────▼──────┐              │
│    │  AGENTS   │◄─▶│  PATTERNS   │◄─▶│  PROJECTS   │              │
│    │  54 ativos│   │  Heuristics │  │  Workspaces  │              │
│    └───────────┘   └─────────────┘  └─────────────┘              │
│                                                                   │
│  FLUXOS DE INFLUÊNCIA:                                            │
│  ─────────────────────                                            │
│  Kernel → Agents: Delegação, contexto, qualidade                  │
│  Agents → Kernel: Learnings, capability updates, failures         │
│  Agents → Knowledge: Patterns, heurísticas, ADRs                  │
│  Knowledge → Agents: Memória semântica, gravity suggestions       │
│  Tools → Agents: Capacidades (bash, edit, task)                   │
│  Projects → Knowledge: Codebase como fonte de verdade             │
│  User → Kernel: Direção estratégica, julgamento final             │
│  Kernel → User: Síntese, recomendações, alertas                   │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**Estado atual**: O ecossistema existe implicitamente — agentes já trocam conhecimento via semantic memory e heurísticas. Mas não há modelo explícito dos fluxos de influência.

**Abordagem de implementação**: Modelagem formal dos fluxos. Monitoring de quais edges do ecossistema estão ativos vs dormentes. Otimização: fortalecer edges de alto valor, podar edges que nunca produzem insights.

---

### C13. Adaptive Personality (Personalidade Adaptativa)

**O que é**: O runtime adapta sua estratégia de comunicação e decisão ao contexto:
- **Enterprise**: Conservador, máxima verificabilidade, audit trail completo
- **Startup**: Criativo, tolerância a risco moderada, velocidade priorizada
- **Auditoria**: Rigoroso, zero tolerância a inconsistências, evidência obrigatória
- **Pesquisa**: Exploratório, hipóteses livres, baixa barreira para experimentação

```yaml
adaptive_personality:
  modes:
    enterprise:
      default_confidence_threshold: 0.85
      audit_trail: "completo"
      risk_tolerance: "baixa"
      decision_speed: "deliberada"
    startup:
      default_confidence_threshold: 0.60
      audit_trail: "essencial"
      risk_tolerance: "moderada"
      decision_speed: "rápida"
    audit:
      default_confidence_threshold: 0.95
      audit_trail: "forense"
      risk_tolerance: "zero"
      decision_speed: "metódica"
    research:
      default_confidence_threshold: 0.40
      audit_trail: "hipóteses"
      risk_tolerance: "alta"
      decision_speed: "exploratória"
  detection: "Automático por contexto (tipo de projeto, comando do Don, fase do ciclo de vida)"
```

**Estado atual**: Nenhum. O runtime opera em modo único (essencialmente enterprise com elementos de startup).

**Abordagem de implementação**: Fase 3. Detection automática por workspace type + explicit flags (`--mode audit`). Cada modo ajusta: confidence thresholds, audit trail detail, escalation rules, criatividade permitida.

---

### C14. Cognitive Economy (Economia Cognitiva) ★ CONCEITO ESTRELA

**O que é**: Toda decisão tem um custo. O Kernel otimiza 5 recursos escassos simultaneamente:

1. **Tokens** — Cada palavra no contexto do LLM custa dinheiro e latência
2. **Tempo** — Tempo do Don é o recurso mais valioso do sistema
3. **Energia computacional** — CPU, memória, I/O
4. **Atenção do usuário** — Quantas notificações, alertas, perguntas
5. **Crescimento de conhecimento** — Cada learning adicionado ao sistema tem custo de storage + retrieval + curadoria

O **Cognitive Economy Engine** é o motor de otimização que equilibra esses 5 recursos. Não é sobre gastar menos — é sobre gastar melhor.

```yaml
cognitive_economy:
  engine: "Cost × Value optimization"

  resources:
    tokens:
      unit: "1K tokens"
      cost_per_unit: "$0.01 (média entre providers)"
      budget_per_session: 50000
      optimization: "Lazy loading de agentes, KERNEL.md em cache, contexto incremental"

    time:
      unit: "minutos"
      budget_per_task: "variável por complexidade"
      optimization: "Paralelismo (multi-agent), DAG execution, pre-warming de contexto"

    compute:
      unit: "CPU-seconds"
      optimization: "Embeddings batch, semantic search com índice pré-computado"

    attention:
      unit: "interrupções"
      budget_per_session: 3  # Máximo de interrupções ao Don por sessão
      optimization: "Agregar perguntas, resolver o que puder sem consultar, escalar só o crítico"

    knowledge_growth:
      unit: "learnings"
      optimization: "Cognitive Compression (80 casos → 1 princípio), Curation Engine (podar redundantes)"

  cost_benefit_decision:
    example: |
      "Devo spawnar 5 agentes para auditar segurança ou 1 agente?"
      Custo: 5 × ~3K tokens = 15K tokens + 5 × latência
      Valor: 5 agentes = cobertura 95% vs 1 agente = cobertura 70%
      Decisão: Spawnar 5 se o domínio for security-critical (custo justifica).
               Spawnar 1 se for auditoria de rotina (custo não justifica).
```

**Estado atual**: Emergente. O Kernel já faz otimização implícita (L17: token bloat audit, lazy loading discussion). Mas não há engine formal.

**Abordagem de implementação**: Fase 2 (prioridade máxima — é o conceito estrela).
1. Tracking de gasto de tokens por tarefa
2. Tracking de interrupções ao Don
3. Cost×Value matrix por tipo de decisão
4. Auto-throttle: se token budget da sessão > 80%, reduzir paralelismo
5. Relatório pós-sessão: "Esta sessão custou X tokens, gerou Y learnings. Eficiência cognitiva: Z."

---

## 6. Fases de Implementação

A evolução para maturidade cognitiva plena é dividida em 4 fases. Cada fase tem entregáveis concretos, ganho de CMI projetado e esforço estimado.

### Fase 0 — Imediata (Julho 2026)

**Objetivo**: Completar o que já está em andamento. Fechar gaps do metacognition pipeline. Populcar negative memory.

```yaml
fase_0:
  prazo: "Julho 2026"
  esforco: "Baixo"
  cmi_gain: "+0.38 (87.0 → 87.4)"

  deliverables:
    - id: F0-1
      task: "Completar metacognition pipeline stages 7-8"
      description: "EXTRACT PATTERN e UPDATE CAPABILITY MODEL — stages parcialmente implementados. Automatizar extração de padrões pós-task e atualização de capability profiles."
      impact: "Autocrítica +3, Aprendizado +2"

    - id: F0-2
      task: "Popular failures.md em todos os agentes com histórico de execução"
      description: "Dos 54 agentes, apenas 3 têm failures.md populado (kernel, testing, performance). Extrair failures de learnings.md para o formato canônico."
      impact: "Autocrítica +5, Aprendizado +3"

    - id: F0-3
      task: "Popular patterns.md em agentes com ≥3 learnings"
      description: "Agentes com histórico de execução devem ter padrões extraíveis. Kernel, documentation, backend, semantic-memory, testing, performance."
      impact: "Transferência +4, Consistência +2"

    - id: F0-4
      task: "Atualizar capability-profile.md de agentes seed-only com auto-avaliação honesta"
      description: "41 agentes têm confidence 0.25 seed. Atualizar para refletir que NÃO têm histórico real de execução (confidence 0.10-0.20)."
      impact: "Autocrítica +3"

    - id: F0-5
      task: "Criar COGNITIVE_MATURITY.md (este documento)"
      description: "Documento fundacional da arquitetura de maturidade cognitiva."
      impact: "Fundação para todas as fases seguintes"
```

### Fase 1 — Fundação (Agosto 2026)

**Objetivo**: Implementar os 5 pilares fundamentais que transformam o runtime de "executor com memória" para "tomador de decisão com rastreabilidade".

```yaml
fase_1:
  prazo: "Agosto 2026"
  esforco: "Médio"
  cmi_gain: "+0.22 (87.4 → 87.6)"

  deliverables:
    - id: F1-1
      task: "Decision DNA"
      concept: "C4"
      description: "Template YAML + query engine SQLite. Integração com ADR directory. Toda decisão de arquitetura deixa DNA registrado."
      impact: "Julgamento +5"

    - id: F1-2
      task: "Contrafactual Gate"
      concept: "A2"
      description: "Antes de decisões de arquitetura, Critic Chief gera cenário oposto. Output: 'Se fizéssemos o contrário, os riscos seriam...'"
      impact: "Julgamento +4"

    - id: F1-3
      task: 'Proactive "Não Sei"'
      concept: "A8"
      description: "Antes de tasks em domínios com confidence < 0.5, runtime verifica cobertura de conhecimento e escala gap específico ao Don."
      impact: "Autocrítica +3, Julgamento +2"

    - id: F1-4
      task: "Wisdom Decay"
      concept: "C11"
      description: "last_validated timestamp em heurísticas e padrões. Background job de decay. Alertas de revalidação."
      impact: "Consistência +3"

    - id: F1-5
      task: "Real Metrics Tracking"
      concept: "Seção 3"
      description: "Implementar coleta automática das 5 métricas reais. Dashboard com série histórica."
      impact: "Todas as dimensões — baseline para medição objetiva"
```

### Fase 2 — Motores (Setembro-Outubro 2026)

**Objetivo**: Implementar os 4 motores cognitivos que transformam conhecimento estático em influência dinâmica.

```yaml
fase_2:
  prazo: "Setembro-Outubro 2026"
  esforco: "Alto"
  cmi_gain: "+0.43 (87.6 → 88.0)"

  deliverables:
    - id: F2-1
      task: "Cognitive Economy Engine"
      concept: "C14 ★"
      description: "Motor formal de otimização cost×value. Tracking de tokens, tempo, atenção. Auto-throttle. Relatório de eficiência cognitiva."
      impact: "Planejamento +8, Julgamento +5"
      priority: "CRITICAL — conceito estrela"

    - id: F2-2
      task: "Cognitive Immune System"
      concept: "C5"
      description: "Scanner de contradições. Semantic similarity para detectar conflitos. Workflow de benchmark → aprovar/rejeitar."
      impact: "Consistência +8, Autocrítica +5"

    - id: F2-3
      task: "Transferência Cross-Project"
      concept: "A4"
      description: "MEMORY_GLOBAL com federação. Heurísticas compartilhadas entre instâncias. Cross-project pattern recognition."
      impact: "Transferência +10"

    - id: F2-4
      task: "Cognitive Gravity"
      concept: "C1"
      description: "Gravity scoring para heurísticas. Auto-sugestão de conhecimento relevante no Stage 3 (PLAN STRATEGY)."
      impact: "Transferência +5, Planejamento +3"
```

### Fase 3 — Avançada (Novembro 2026+)

**Objetivo**: Alcançar maturidade cognitiva plena com os 6 conceitos mais avançados.

```yaml
fase_3:
  prazo: "Novembro 2026+"
  esforco: "Alto"
  cmi_gain: "+0.68 (88.0 → 88.7)"

  deliverables:
    - id: F3-1
      task: "Insight Generator"
      concept: "C9"
      description: "Background analyzer. Detecção de padrões cross-domain sem prompt. Geração de hipóteses proativas."
      impact: "Aprendizado +8"

    - id: F3-2
      task: "Cognitive Compression"
      concept: "C10"
      description: "Motor de compressão N:1. Após N casos similares, extrair princípio universal."
      impact: "Aprendizado +5, Consistência +5"

    - id: F3-3
      task: "Raciocínio de 2ª Ordem"
      concept: "A1"
      description: "Simulation engine para cascatas de decisão. DAG-based consequence prediction."
      impact: "Planejamento +10, Julgamento +8"

    - id: F3-4
      task: "Adaptive Personality"
      concept: "C13"
      description: "4 modos de operação (enterprise, startup, audit, research). Detecção automática + flags explícitas."
      impact: "Julgamento +6"

    - id: F3-5
      task: "Mental Energy"
      concept: "C7"
      description: "Budget de energia cognitiva por domínio. Alocação dinâmica baseada em momentum."
      impact: "Planejamento +5"

    - id: F3-6
      task: "Hierarquia de Abstração"
      concept: "A5"
      description: "Abstraction Layering Engine. Output multi-nível."
      impact: "Transferência +3"
```

---

## 7. Pontos de Integração

O CMI e a arquitetura de maturidade cognitiva não existem isolados. Eles se integram com todos os sistemas existentes do Cosca Runtime.

### 7.1 KERNEL.md

**Arquivo**: [../KERNEL.md](../KERNEL.md)

| Ponto de Integração | Como |
|---------------------|------|
| **Responsibility #22 (Learning Trigger)** | Agora alimenta o CMI — learnings pós-sessão atualizam a dimensão Aprendizado |
| **Responsibility #23 (Evolution Trigger)** | Evolution Engine agora consulta CMI para decidir se evolução é necessária (CMI stagnation > 30 dias → trigger) |
| **Section §10.10 (Quality Gates)** | Gate 0 agora inclui verificação de CMI mínimo para tarefas de alto risco (CMI < 70 → revisão obrigatória) |
| **Section §10.7 (Planning)** | DAG generation agora consulta Cognitive Gravity para sugestão de padrões relevantes |
| **Section §10.2 (Context Discovery)** | Bootstrap agora carrega COGNITIVE_MATURITY.md e inicializa métricas reais |

### 7.2 Capability Profiles

**Arquivos**: `../memory/agent/{agent}/capability-profile.md`

| Ponto de Integração | Como |
|---------------------|------|
| **Confidence Model** | Os confidence scores dos agentes alimentam a dimensão Julgamento do CMI (agregado ponderado) |
| **Evolution Goals** | Metas de evolução dos agentes são agora alinhadas com as capacidades de julgamento (A1-A8) |
| **Weaknesses** | Weaknesses documentadas alimentam o "Proactive Não Sei" (A8) — gaps conhecidos são verificados antes de tasks |

### 7.3 Evolution Tracking

**Arquivos**: `../memory/agent/{agent}/evolution.md`

| Ponto de Integração | Como |
|---------------------|------|
| **Level Progression** | Níveis (1-5) continuam existindo como métrica individual, mas o CMI é a métrica do runtime |
| **CMI History** | Evolution.md agora registra também o CMI do runtime no momento de cada evolução |

### 7.4 Metacognition Pipeline

**Arquivo**: [../workflows/metacognition-pipeline.md](../workflows/metacognition-pipeline.md)

| Ponto de Integração | Como |
|---------------------|------|
| **Stage 1 (SELF-ASSESS)** | Agora consulta CMI para contexto: "O runtime como um todo está maduro o suficiente para esta tarefa?" |
| **Stage 3 (PLAN STRATEGY)** | Agora consulta Cognitive Gravity para sugestão de padrões de alta massa |
| **Stage 7 (EXTRACT PATTERN)** | Alimenta o sistema de Pattern Evolution (C6) com versionamento |
| **Stage 8 (UPDATE CAPABILITY MODEL)** | Atualiza dimensões do CMI impactadas pela task |

### 7.5 Quality Gates

**Arquivo**: [../QUALITY_GATES.md](../QUALITY_GATES.md)

| Ponto de Integração | Como |
|---------------------|------|
| **Gate 0 (Pre-Work)** | Adicionar verificação: "CMI do domínio da tarefa ≥ threshold?" |
| **Gate 1 (Pre-Impl)** | Adicionar verificação: "Contrafactual Gate executado?" (Fase 1) |
| **Gate 2 (Post-Impl)** | Adicionar verificação: "Cognitive Immune System aprovou novo conhecimento?" (Fase 2) |
| **Gate 3 (Pre-Release)** | Adicionar verificação: "Métricas reais dentro do esperado?" |
| **Gate 4 (Post-Release)** | Adicionar verificação: "CMI estável ou melhorou após release?" |

### 7.6 Semantic Memory

**Agente**: `cosca-semantic-memory` — [../memory/agent/cosca-semantic-memory/](../memory/agent/cosca-semantic-memory/)

| Ponto de Integração | Como |
|---------------------|------|
| **Cross-Agent Clusters** | Os 8 clusters detectados em C1 alimentam Cognitive Gravity — agentes no mesmo cluster têm influência gravitacional mútua |
| **Heuristic Index** | H-001 a H-020 são a base inicial para Gravity scoring (C1) e Wisdom Decay (C11) |
| **Semantic Search** | Query de contradições (C5 — Immune System) usa o mesmo motor de similaridade semântica |
| **Transferência** | A dimensão Transferência do CMI é alimentada diretamente pelo semantic indexing cross-agent |

---

## 8. Decisão de Nomenclatura

### Por que "Cognitive Maturity" e não "QI", "Intelligence" ou "Autonomy Level"?

A escolha do termo foi deliberada e passou por avaliação de alternativas:

| Termo | Problema |
|-------|----------|
| **QI (Quociente de Inteligência)** | Sugere uma medida fixa e inata. Inteligência não é o que medimos — é julgamento + evolução. Além disso, QI tem carga cultural e histórica que não se aplica a sistemas de IA. |
| **Intelligence Level** | Muito vago. Todo orquestrador de IA se autointitula "inteligente". Não diferencia. |
| **Autonomy Level** | Foca em independência, mas o Cosca Runtime não busca autonomia total — busca simbiose com o Don (L13: "A relação não é dono vs ferramenta. É simbiótica"). |
| **Capability Level** | Já usado para agentes individuais (Level 1-5). Causaria confusão. |
| **Cognitive Maturity** | ✅ **Escolhido**. Enfatiza três aspectos que são os verdadeiros diferenciadores deste runtime: |

### Os 3 Pilares do Nome

1. **Evolução**: Maturidade se conquista com tempo, experiência, erros e acertos. Não é um atributo de fábrica — é construído. O runtime de Julho/2026 não é o mesmo de Junho/2026. O CMI captura essa trajetória.

2. **Qualidade das Decisões**: O que torna o Cosca Runtime diferente não é quantos agentes ele gerencia ou quão rápido ele executa. É a qualidade do julgamento que ele aplica antes de cada ação. O Pipeline de Metacognição (8 estágios), o Confidence Model, os Quality Gates — tudo converge para uma coisa: decisões melhores.

3. **Capacidade de Adaptação**: Um sistema maduro não é o que acerta sempre. É o que reconhece quando erra, aprende, e não repete o mesmo erro. As 5 métricas reais (Seção 3) medem exatamente isso — adaptação real, não scores abstratos.

### O Kernel como Judgment Engine

O nome captura uma verdade arquitetural que já está presente, mas ainda não havia sido nomeada: **o Cosca Kernel não é um orquestrador — é um Judgment Engine**.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│   ORQUESTRADOR TRADICIONAL          COSCA KERNEL                  │
│   ────────────────────────          ────────────                  │
│                                                                   │
│   Recebe comando                    Recebe intenção               │
│   Roteia para worker                Avalia contexto               │
│   Aguarda resultado                 Planeja estratégia            │
│   Retorna output                    Seleciona agentes             │
│                                     Verifica capability           │
│                                     Antecipa riscos               │
│                                     Executa (via delegação)       │
│                                     Verifica qualidade            │
│                                     Critica próprio trabalho      │
│                                     Extrai padrão                 │
│                                     Atualiza modelo mental        │
│                                     Aprende                        │
│                                                                   │
│   OUTPUT: Tarefa concluída          OUTPUT: Tarefa concluída      │
│                                               +                   │
│                                       Runtime mais inteligente    │
│                                       do que antes da tarefa       │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### CMI: A Métrica do Judgment Engine

Se o Kernel é um Judgment Engine, o CMI é a medida da qualidade desse julgamento. Não mede "poder computacional" ou "número de agentes". Mede:

- **Aprendizado**: O julgamento melhora com cada tarefa? (dimensão Aprendizado)
- **Precisão**: As decisões tomadas se mostram corretas? (dimensão Julgamento)
- **Previsão**: O runtime antecipa consequências? (dimensão Planejamento)
- **Humildade**: O runtime reconhece seus erros e limitações? (dimensão Autocrítica)
- **Generalização**: O aprendizado em um domínio se aplica a outros? (dimensão Transferência)
- **Confiabilidade**: O comportamento é consistente e reprodutível? (dimensão Consistência)

---

## 9. Governança do CMI

### 9.1 Atualização

O CMI é recalculado automaticamente após cada sessão, como parte do Evolution Trigger (KERNEL.md §25). O recálculo consulta:

1. **learnings.md** de todos os agentes (dimensões Aprendizado, Transferência)
2. **failures.md** de todos os agentes (dimensão Autocrítica)
3. **Métricas reais** (Seção 3) — dados objetivos
4. **Quality Gate pass/fail ratios** (dimensão Consistência)
5. **Cross-agent cluster health** via semantic memory (dimensão Transferência)

### 9.2 Revisão

O CMI é revisado trimestralmente pelo CTO Chief com input do Critic Chief. A revisão avalia:

- As dimensões estão calibradas? (pesos ainda fazem sentido?)
- As métricas reais estão sendo coletadas corretamente?
- Há novas capacidades de julgamento que deveriam ser adicionadas?

### 9.3 Stagnation Detection

Se o CMI não apresentar melhoria por 30 dias consecutivos, o Evolution Engine dispara um alerta. Stagnation indica uma de três causas:

1. **Falta de tarefas desafiadoras**: O runtime está sendo subutilizado — só executa tarefas triviais que não geram aprendizado.
2. **Teto cognitivo**: O runtime atingiu o limite do que consegue aprender com a arquitetura atual — precisa de nova capacidade (próxima fase).
3. **Contaminação do conhecimento**: Entropia cognitiva (C2) está alta, conhecimento contraditório está impedindo melhoria.

---

## 10. Diagrama de Arquitetura

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        COGNITIVE MATURITY ARCHITECTURE                         │
│                              Cosca Runtime v1.4.0                              │
│                                                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                        JUDGMENT ENGINE (Kernel)                          │ │
│  │                                                                         │ │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐            │ │
│  │  │    A1     │  │    A2     │  │    A3     │  │    A4     │            │ │
│  │  │ Raciocínio│  │Contrafat. │  │Detecção   │  │Transfer.  │            │ │
│  │  │ 2ª Ordem  │  │   Gate    │  │Inconsist. │  │Conhecim.  │            │ │
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────┘            │ │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐            │ │
│  │  │    A5     │  │    A6     │  │    A7     │  │    A8     │            │ │
│  │  │Hierarquia │  │Metacogn.  │  │Planej.    │  │ "Não Sei" │            │ │
│  │  │Abstração  │  │Pipeline   │  │Adaptativo │  │ Proativo  │            │ │
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────┘            │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                          │
│  ┌─────────────────────────────────┼──────────────────────────────────────┐  │
│  │                    COGNITIVE MATURITY INDEX (CMI)                       │  │
│  │                                                                        │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │  │
│  │  │Aprendizado│ │Julgamento│ │Planejamen│ │Autocrítica│ │Transferên│     │  │
│  │  │   20%    │ │   25%    │ │   15%    │ │   15%    │ │   15%    │     │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘     │  │
│  │                         ┌──────────┐                                   │  │
│  │                         │Consistênc│                                   │  │
│  │                         │   10%    │                                   │  │
│  │                         └──────────┘                                   │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                    │                                          │
│  ┌─────────────────────────────────┼──────────────────────────────────────┐  │
│  │                      5 MÉTRICAS REAIS                                  │  │
│  │                                                                        │  │
│  │  1. Problemas s/ intervenção   2. Previsões corretas                   │  │
│  │  3. Decisões revertidas        4. Conhecimento reutilizado             │  │
│  │  5. Problemas detectados antes da falha                                │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                    │                                          │
│  ┌─────────────────────────────────┼──────────────────────────────────────┐  │
│  │                   14 CONCEITOS COGNITIVOS                              │  │
│  │                                                                        │  │
│  │  ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐                    │  │
│  │  │ C1 │ │ C2 │ │ C3 │ │ C4 │ │ C5 │ │ C6 │ │ C7 │                    │  │
│  │  │Grav│ │Entr│ │Mome│ │DNA │ │Imun│ │Evol│ │Ener│                    │  │
│  │  └────┘ └────┘ └────┘ └────┘ └────┘ └────┘ └────┘                    │  │
│  │  ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐                    │  │
│  │  │ C8 │ │ C9 │ │C10 │ │C11 │ │C12 │ │C13 │ │C14 │                    │  │
│  │  │Hori│ │Insi│ │Comp│ │Deca│ │Ecos│ │Pers│ │Econ│ ★                   │  │
│  │  └────┘ └────┘ └────┘ └────┘ └────┘ └────┘ └────┘                    │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                    │                                          │
│  ┌─────────────────────────────────┼──────────────────────────────────────┐  │
│  │                      4 FASES DE IMPLEMENTAÇÃO                          │  │
│  │                                                                        │  │
│  │  FASE 0 ────────▶ FASE 1 ────────▶ FASE 2 ────────▶ FASE 3            │  │
│  │  Imediata         Fundação         Motores           Avançada          │  │
│  │  +0.38 CMI        +0.22 CMI        +0.43 CMI         +0.68 CMI         │  │
│  │  Jul/2026         Ago/2026         Set-Out/2026      Nov/2026+         │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 11. Referências

### Documentos Relacionados

| Documento | Caminho | Relação |
|-----------|---------|---------|
| KERNEL.md | [../KERNEL.md](../KERNEL.md) | Runtime specification — o CMI mede a qualidade do Kernel como Judgment Engine |
| AUTO_EVOLUTION_PROTOCOL.md | [../shared/AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) | Sistema de níveis substituído pelo CMI como métrica primária |
| AGENT_DNA.md | [../AGENT_DNA.md](../AGENT_DNA.md) | DNA v3.0 com Metacognition Layer — capacidades A1-A8 estendem o pipeline |
| QUALITY_GATES.md | [../QUALITY_GATES.md](../QUALITY_GATES.md) | Gates 0-4 integram verificações de CMI |
| metacognition-pipeline.md | [../workflows/metacognition-pipeline.md](../workflows/metacognition-pipeline.md) | Pipeline de 8 estágios que implementa A6 (Metacognição) |
| CONSTITUTION.md | [../CONSTITUTION.md](../CONSTITUTION.md) | Princípios imutáveis — CMI não pode contradizer a Constituição |
| MEMORY_MODEL.md | [../MEMORY_MODEL.md](../MEMORY_MODEL.md) | Modelo de memória — learnings, failures, patterns são as fontes de dados do CMI |
| LEARNING_PROTOCOL.md | [../memory/LEARNING_PROTOCOL.md](../memory/LEARNING_PROTOCOL.md) | Protocolo de aprendizado que alimenta o CMI |

### Memória de Agentes Relevantes

| Agente | Caminho | Papel no CMI |
|--------|---------|-------------|
| cosca-kernel | [../memory/agent/cosca-kernel/](../memory/agent/cosca-kernel/) | 26 learnings, 7 Level 4 — maior contribuidor para o CMI |
| cosca-documentation | [../memory/agent/cosca-documentation/](../memory/agent/cosca-documentation/) | 3 learnings, Level 3 — documentação é fonte de verdade |
| cosca-semantic-memory | [../memory/agent/cosca-semantic-memory/](../memory/agent/cosca-semantic-memory/) | 2 learnings, C1 indexing — transferência cross-agent |
| cosca-critic | [../memory/agent/cosca-critic/](../memory/agent/cosca-critic/) | Decision critic — implementará Contrafactual Gate (A2) |
| cosca-cto | [../memory/agent/cosca-cto/](../memory/agent/cosca-cto/) | Revisão trimestral do CMI |

---

## 12. Histórico

| Versão | Data | Autor | Mudanças |
|---------|------|--------|----------|
| 1.0.0 | 2026-07-30 | Cosca Kernel (via Documentation Chief) | Documento fundacional. Definição do CMI (6 dimensões, baseline 87.0). 5 métricas reais. 8 capacidades de julgamento (A1-A8). 14 conceitos cognitivos inéditos (C1-C14). 4 fases de implementação. Integração com todos os sistemas existentes. |

---

> **"O Kernel não é um orquestrador. É um Judgment Engine. O CMI mede a qualidade desse julgamento."**
>
> — Cosca Kernel, 2026-07-30
