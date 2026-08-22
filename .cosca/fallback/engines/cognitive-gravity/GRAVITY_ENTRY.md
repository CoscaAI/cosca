# COGNITIVE GRAVITY — Micro-Modelo de Entrada (v1.0.0, complementar ao F2.4)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Memory Chief | **Criado**: 2026-07-30
> **Fase**: Fase 2 — Core Engines (F2.4)
> **Depende de**: F2.1 (Cognitive Economy), F2.3 (Cross-project Knowledge Transfer)
> **Referências**: [COGNITIVE_MATURITY.md §5 C1](../../architecture/COGNITIVE_MATURITY.md), [cognitive-maturity-implementation.md F2.4](../../workflows/cognitive-maturity-implementation.md)
> **CMI Impact**: +0.06 (Transferência +5, Planejamento +3)
>
> ⚠️ **Documento complementar**: Este arquivo preserva o modelo **v1.0.0** — gravidade por **entrada de conhecimento** (influência individual). O modelo canônico do Don (F2.4, atração de conhecimento afim por domínio) está em [SKILL.md](./SKILL.md). Relação e configuração de integração: SKILL.md §13.

---

## 1. PROPÓSITO

O **Cognitive Gravity Engine** resolve o problema fundamental de que todo conhecimento no sistema Cosca tem peso igual, independentemente de quantas vezes foi validado, em quantos contextos diferentes foi aplicado, ou quantos projetos o confirmaram. Um padrão usado 4 vezes com 100% de sucesso não deveria ter a mesma influência que uma hipótese nunca testada.

A Gravidade Cognitiva é o mecanismo que transforma conhecimento validado em **influência automática** sobre o processo decisório do runtime. Quanto mais um padrão ou heurística é validado(a) — por mais agentes, em mais contextos, em mais projetos — maior sua "massa gravitacional". Padrões com alta gravidade não precisam ser buscados ativamente: eles **atraem** decisões para sua órbita, sendo sugeridos automaticamente em contextos relevantes.

### 1.1 Distinção Fundamental: Gravidade vs Confiança

| Dimensão | **Confiança (Confidence)** | **Gravidade (Gravity)** |
|----------|---------------------------|------------------------|
| **O que mede** | Certeza interna sobre UMA entrada de conhecimento | Influência que essa entrada exerce sobre OUTRAS decisões |
| **Escopo** | Intra-entrada (self-referential) | Inter-entrada (relacional) |
| **Pergunta** | "Quão certo estou desta afirmação?" | "Quanto esta afirmação deve influenciar minhas decisões?" |
| **Decaimento** | Por tempo (Wisdom Decay, C11) | Por inatividade + falhas de validação |
| **Aumenta com** | Mais evidências diretas | Mais validações em contextos DIFERENTES |
| **Exemplo** | Confiança alta = "Sei que este padrão funciona" | Gravidade alta = "Este padrão DEVE ser considerado em toda decisão similar" |

Um conhecimento pode ter alta confiança (0.95 — "tenho certeza que funciona") mas baixa gravidade (só foi validado em 1 contexto). Ou pode ter confiança moderada (0.70) mas alta gravidade (validado em 5 projetos diferentes — exerce muita influência).

**Relação**: `confiança` alimenta `gravidade` (validações que aumentam confiança também aumentam gravidade), mas `gravidade` é uma métrica independente que mede **influência**, não certeza.

---

## 2. FÓRMULA DE GRAVIDADE

### 2.1 Equação Principal

```
gravity_score = (
    validation_count      × 0.30 +
    validation_diversity  × 0.25 +
    cross_project_count   × 0.20 +
    time_factor           × 0.10 +
    evidence_strength     × 0.15
) / max_possible × 100
```

**Onde `max_possible` é a soma dos pesos (0.30 + 0.25 + 0.20 + 0.10 + 0.15 = 1.00).** Na prática, cada componente é normalizado para [0, 1], então gravity_score varia de 0 a 100.

### 2.2 Componentes Detalhados

#### 2.2.1 Validation Count (`validation_count` — peso 0.30)

Mede quantas vezes o conhecimento foi validado com sucesso. Usa saturação logarítmica para evitar que validações infinitas dominem o score.

```
validation_count = min(total_validations, 10) / 10
```

| Validações | Score normalizado | Significado |
|-----------|-------------------|-------------|
| 0 | 0.00 | Nunca validado — hipótese pura |
| 1 | 0.10 | Validado uma vez — evidência inicial |
| 2 | 0.20 | Duas validações — ganhando tração |
| 3 | 0.30 | Três validações — padrão emergente |
| 5 | 0.50 | Cinco validações — padrão estabelecido |
| 10+ | 1.00 | Saturação — validação máxima |

> **Nota**: A saturação em 10 existe por design. Após 10 validações bem-sucedidas, validações adicionais não aumentam significativamente a gravidade — o conhecimento já provou seu valor. O que IMPORTA após a saturação é `validation_diversity` (ser validado em contextos DIFERENTES).

#### 2.2.2 Validation Diversity (`validation_diversity` — peso 0.25)

Mede em quantos domínios DIFERENTES o conhecimento foi validado. Esta é a dimensão MAIS IMPORTANTE para Transferência (CMI dimensão de 15%). Um padrão validado 5 vezes no mesmo domínio tem diversidade baixa; validado em 5 domínios diferentes tem diversidade alta.

```
validation_diversity = unique_domains_validated / total_relevant_domains
```

**Domínios reconhecidos pelo Cosca Runtime**:

```yaml
domains:
  backend:       [api, services, business-logic, database]
  frontend:      [ui, components, state, routing]
  testing:       [unit, integration, e2e, coverage]
  devops:        [ci-cd, containers, iac, deployment]
  architecture:  [design, patterns, adrs, modularity]
  security:      [vulnerabilities, auth, compliance, encryption]
  performance:   [profiling, benchmarking, optimization]
  documentation: [readme, adrs, api-docs, guides]
  data:          [schema, migrations, queries, storage]
  orchestration: [agent-routing, workflows, pipelines, parallelism]
  governance:    [policies, conventions, quality-gates, compliance]
  sdk:           [client-libraries, api-wrappers, toolkits]
  platform:      [dx, tooling, automation, scaffolding]
  monitoring:    [observability, alerting, slos, dashboards]
  ai_ml:         [embeddings, rag, models, prompts]
```

`total_relevant_domains` é o número de domínios onde este conhecimento PODERIA ser aplicado (não o total de 15 domínios — seria injusto para conhecimento especializado). Por padrão, `total_relevant_domains = min(unique_domains_validated + 1, 15)`.

| Domínios validados | Score | Interpretação |
|-------------------|-------|---------------|
| 1 | 0.20 | Single-domain — conhecimento local |
| 3 | 0.60 | Multi-domain — transfere entre áreas |
| 5 | 1.00 | Cross-domain — conhecimento universal |

#### 2.2.3 Cross-Project Count (`cross_project_count` — peso 0.20)

Mede em quantos projetos DIFERENTES o conhecimento foi confirmado. Esta é a dimensão mais difícil de aumentar — requer que o mesmo padrão seja descoberto e validado em projetos independentes. É a evidência mais forte de que o conhecimento é fundamental, não acidental.

```
cross_project_count = min(projects_validated, 5) / 5
```

| Projetos | Score | Interpretação |
|----------|-------|---------------|
| 0 | 0.00 | Single-project — pode ser idiossincrático |
| 1 | 0.20 | Confirmado em 1 projeto externo |
| 2 | 0.40 | Dois projetos — evidência de generalização |
| 3 | 0.60 | Forte evidência cross-project |
| 5+ | 1.00 | Saturação — conhecimento universal comprovado |

> **Dependência**: Esta dimensão requer F2.3 (Cross-project Knowledge Transfer / Semantic Federation) operacional. Sem federação cross-project, `cross_project_count` é sempre 0 (ou 1 para o projeto atual).

#### 2.2.4 Time Factor (`time_factor` — peso 0.10)

Mede a recência da última validação. Conhecimento validado recentemente tem mais relevância que conhecimento validado há meses. Esta dimensão é deliberadamente leve (peso 0.10) para evitar que conhecimento novo sem substância domine conhecimento profundamente validado.

```
time_factor:
  last_validated ≤ 7 days:   1.0   # Fresco — plena influência
  last_validated ≤ 30 days:  0.7   # Recente — alta influência
  last_validated ≤ 90 days:  0.4   # Envelhecendo — influência moderada
  last_validated ≤ 365 days: 0.1   # Antigo — precisa revalidação
  last_validated > 365 days: 0.0   # Expirado — sem influência até revalidar
```

**Interação com Wisdom Decay (C11/F1.4)**: Quando o Wisdom Decay detecta que um conhecimento atingiu 80% do TTL, a revalidação automática dispara. Se a revalidação confirma o conhecimento, `last_validated` é atualizado e `time_factor` volta a 1.0. Se a revalidação falha, a gravidade colapsa (ver Seção 3.3 — Acumulação de Gravidade).

#### 2.2.5 Evidence Strength (`evidence_strength` — peso 0.15)

Mede a QUALIDADE da evidência que sustenta o conhecimento. Nem toda validação é igual — um benchmark reproduzível tem mais peso que uma afirmação não verificada.

| Tipo de Evidência | Score | Descrição |
|------------------|-------|-----------|
| **CODE** | 1.0 | Código executável que demonstra o padrão. Testes passando. Coverage > 70%. |
| **BENCHMARK** | 0.8 | Métricas quantitativas (latência, throughput, consumo). Reprodutível. |
| **AUDIT** | 0.6 | Auditoria cross-source (docs vs código vs claims). Múltiplos agentes. |
| **ASSERTION** | 0.3 | Afirmação de um agente sem verificação externa. Confiança baseada em reputation. |

```
evidence_strength = max(evidence_type_score, 0.0)

# Se múltiplos tipos de evidência existem, usa-se o maior score
# Exemplo: conhecimento tem CODE (1.0) + AUDIT (0.6) → usa 1.0
```

**Regra de ouro**: Nenhum conhecimento com `evidence_strength = ASSERTION (0.3)` pode atingir gravidade > 50 (Stone). Isso impede que hipóteses não verificadas alcancem influência moderada sem validação concreta.

---

## 3. NÍVEIS DE GRAVIDADE

Os níveis de gravidade determinam como o conhecimento é tratado pelo runtime durante o processo decisório.

### 3.1 Tabela de Níveis

| Gravity Score | Label | Ícone | Comportamento |
|--------------|-------|-------|---------------|
| **0-20** | **Dust** (Poeira) | ░░ | **Sem influência** — Ignorado a menos que diretamente relevante. Não aparece em sugestões automáticas. Existe no knowledge graph mas não exerce atração gravitacional. |
| **21-40** | **Pebble** (Seixo) | ◌ | **Sugestão fraca** — Mencionado como "possível referência" mas sem peso na decisão. Aparece em buscas explícitas. Não influencia o ranking de resultados. |
| **41-60** | **Stone** (Pedra) | ● | **Influência moderada** — Considerado ativamente em decisões do mesmo domínio. Aparece nos top-K resultados de busca semântica. Peso moderado no gravitational field. |
| **61-80** | **Boulder** (Rocha) | ◉ | **Influência forte** — Ativamente molda decisões. Sugerido automaticamente em contextos relevantes (same domain + adjacent). Difícil de ignorar — requer justificativa explícita para descartar. |
| **81-100** | **Planet** (Planeta) | ◎ | **Influência dominante** — Auto-aplicado em decisões. Hard to override — só pode ser descartado com contrafactual gate (A2) + aprovação do Critic Chief. Define o "chão" de qualidade para o domínio. |

### 3.2 Interpretação Visual

```
GRAVITY SPECTRUM
═══════════════════════════════════════════════════════════════════════

  0 ─────── 20 ─────── 40 ─────── 60 ─────── 80 ─────── 100
  │   DUST   │  PEBBLE  │  STONE   │ BOULDER  │  PLANET  │
  │          │          │          │          │          │
  ░░        ◌          ●          ◉          ◎
  Ignorado   Sugerido   Considerado Influente  Dominante

  Gravidade cresce ──────────────────────────────────────▶
  Inércia contra mudança cresce ─────────────────────────▶
  Exigência de justificativa para ignorar cresce ────────▶
```

### 3.3 Comportamento por Nível — Regras de Decisão

#### Dust (0-20): Conhecimento Inerte

```yaml
dust_behavior:
  retrieval: "Não aparece em buscas automáticas"
  suggestion: "Nunca sugerido proativamente"
  decision_weight: 0.0  # Não influencia gravitational field
  override: "Pode ser ignorado sem justificativa"
  typical_entries:
    - "Hipóteses nunca testadas"
    - "Primeira versão de um padrão (0 validações)"
    - "Conhecimento marcado como 'untrusted' após falha de validação"
    - "Entradas experimentais com evidence_strength = ASSERTION"
```

#### Pebble (21-40): Conhecimento Emergente

```yaml
pebble_behavior:
  retrieval: "Aparece em buscas explícitas (se relevante)"
  suggestion: "Sugerido como 'referência possível' se semanticamente próximo"
  decision_weight: 0.15  # Peso leve no gravitational field
  override: "Pode ser ignorado com nota breve"
  typical_entries:
    - "Padrão validado 1-2 vezes no mesmo contexto"
    - "Conhecimento novo (menos de 7 dias)"
    - "Padrões que eram Stone/Boulder mas sofreram gravity decay"
```

#### Stone (41-60): Conhecimento Estabelecido

```yaml
stone_behavior:
  retrieval: "Top-10 em buscas semânticas do domínio"
  suggestion: "Sugerido ativamente em contextos do mesmo domínio"
  decision_weight: 0.40  # Peso moderado no gravitational field
  override: "Requer justificativa documentada para ignorar"
  typical_entries:
    - "Padrão validado 3-5 vezes, 1-2 domínios diferentes"
    - "Conhecimento com evidence_strength ≥ BENCHMARK"
    - "Padrões cross-project com 1 confirmação externa"
  example: |
    Cross-Agent Audit Pattern (como está HOJE):
    - 4 validações no mesmo projeto → validation_count = 0.40
    - Validado em 2 domínios (orchestration, testing) → diversity = 0.40
    - 0 projetos externos → cross_project = 0.00
    - Última validação: 1 dia atrás → time_factor = 1.0
    - Evidência: AUDIT → evidence = 0.60
    - Score = (0.40×0.30 + 0.40×0.25 + 0.00×0.20 + 1.0×0.10 + 0.60×0.15) × 100
    - Score = (0.12 + 0.10 + 0.00 + 0.10 + 0.09) × 100 = 41 → Stone (limite inferior)
```

#### Boulder (61-80): Conhecimento de Alto Impacto

```yaml
boulder_behavior:
  retrieval: "Top-3 em buscas semânticas do domínio e domínios adjacentes"
  suggestion: "Auto-sugerido em todo o metacognition pipeline Stage 3 (PLAN STRATEGY)"
  decision_weight: 0.70  # Peso forte no gravitational field
  override: "Requer Contrafactual Gate (A2) + justificativa formal + aprovação do Critic Chief"
  typical_entries:
    - "Padrão validado 6-8 vezes, 3-4 domínios diferentes"
    - "Conhecimento com evidence_strength = CODE (1.0)"
    - "Padrões cross-project com 3+ confirmações externas"
    - "Heurísticas que geraram ≥ 3 decisões corretas (B2 — Previsões Corretas)"
  example: |
    Cross-Agent Audit Pattern (após 2 cross-project validations):
    - 6 validações → validation_count = 0.60
    - Validado em 4 domínios → diversity = 0.80
    - 2 projetos confirmaram → cross_project = 0.40
    - Última validação: 3 dias atrás → time_factor = 1.0
    - Evidência: CODE (testado com benchmark) → evidence = 1.0
    - Score = (0.60×0.30 + 0.80×0.25 + 0.40×0.20 + 1.0×0.10 + 1.0×0.15) × 100
    - Score = (0.18 + 0.20 + 0.08 + 0.10 + 0.15) × 100 = 71 → Boulder
    - Comportamento: automaticamente sugerido para TODAS as tarefas de auditoria
```

#### Planet (81-100): Conhecimento Fundacional

```yaml
planet_behavior:
  retrieval: "Sempre no topo de buscas no domínio — inescapável"
  suggestion: "Auto-aplicado — agentes NÃO precisam buscar, o conhecimento 'puxa' a decisão"
  decision_weight: 1.0  # Peso máximo no gravitational field
  override: "Extremamente difícil — requer:"
  override_requirements:
    - "Contrafactual Gate (A2) demonstrando que ignorar é melhor que aplicar"
    - "Evidência concreta de que o contexto atual é fundamentalmente diferente"
    - "Aprovação do Critic Chief + CTO Chief"
    - "Registro em Decision DNA (C4) com rationale detalhado"
    - "Se a override falhar (decisão pior), penalidade de -30 na gravidade do agente"
  typical_entries:
    - "Princípios extraídos por Cognitive Compression (C10) — 80 casos → 1 princípio"
    - "Padrões com 10+ validações em 5+ domínios diferentes"
    - "Conhecimento cross-project com 5+ confirmações"
    - "Heurísticas que NUNCA falharam (0% failure rate em 20+ aplicações)"
  example: |
    Hypothetical "Test-First" Principle após Cognitive Compression:
    - 10+ validações → validation_count = 1.00
    - 6 domínios validados → diversity = 1.00
    - 5 projetos confirmaram → cross_project = 1.00
    - Última validação: 1 dia → time_factor = 1.0
    - Evidência: CODE + BENCHMARK → evidence = 1.0
    - Score = (1.0×0.30 + 1.0×0.25 + 1.0×0.20 + 1.0×0.10 + 1.0×0.15) × 100
    - Score = 100 → Planet
    - Comportamento: NENHUM agente implementa código sem testes. Regra do runtime.
```

---

## 4. ACUMULAÇÃO DE GRAVIDADE (Gravity Accumulation)

### 4.1 Estado Inicial

```
Todo novo conhecimento começa com:
  gravity_score = 10 (Dust → limite superior de Dust, quase Pebble)
  
Isso garante que:
  - Existe no knowledge graph (não é invisível)
  - Pode ser encontrado em buscas explícitas
  - Mas NÃO influencia decisões automaticamente
  - Precisa PROVAR seu valor antes de exercer influência
```

### 4.2 Ganhos por Validação

| Evento | Delta Gravity | Explicação |
|--------|---------------|------------|
| **Validação no mesmo contexto** | +5 | Reforça o que já sabemos — ganho modesto |
| **Validação em contexto DIFERENTE** | +15 | Demonstra transferência — ganho SIGNIFICATIVO |
| **Validação cross-project** | +20 | Generalização comprovada — ganho MÁXIMO |
| **Upgrade de evidência** (ASSERTION → AUDIT → BENCHMARK → CODE) | +10 por nível | Melhora a qualidade da evidência |
| **Primeira validação** (0 → 1) | +10 (bônus) | Sai de hipótese para conhecimento validado |

### 4.3 Perdas por Falha

| Evento | Delta Gravity | Explicação |
|--------|---------------|------------|
| **Falha de validação** | -30 | Gravidade colapsa RAPIDAMENTE quando desprovada |
| **Falha cross-project** | -15 (apenas cross_project_count) | O conhecimento pode ser válido no projeto original mas não generalizável |
| **Timeout de revalidação** (Wisdom Decay expirou) | -2 por mês sem revalidar | Decaimento progressivo |

**Regra de colapso**: Se uma falha de validação reduzir gravity abaixo de 0, o conhecimento é movido para `quarantine/` (Cognitive Immune System — C5/F2.2). Não é deletado — serve como "memória negativa" do que NÃO funciona.

### 4.4 Decaimento Temporal (Time Decay)

```
gravity_decay = -2 por mês sem revalidação

Aplicado quando:
  - last_validated > 90 dias (após o time_factor cair para 0.4)
  - O conhecimento NÃO foi usado em nenhuma decisão nos últimos 30 dias

Decaimento mínimo: gravity_score NUNCA cai abaixo de 10 (Dust superior)
  - Conhecimento "morto" ainda existe, só não influencia
  
Decaimento PARA se:
  - Uma revalidação é bem-sucedida → gravity restaurado ao valor pré-decay + bônus de revalidação (+5)
  - Uma revalidação FALHA → colapso (-30), podendo ir para quarantine
```

### 4.5 Exemplo de Trajetória

```yaml
cross_agent_audit_pattern_trajectory:
  day_0:
    event: "Padrão extraído pela primeira vez (F0.2 — patterns.md)"
    gravity: 10  # Dust — novo, não validado
    
  day_1:
    event: "Primeira aplicação bem-sucedida (L18 — Runtime Coverage Audit)"
    gravity: 25  # Pebble — +10 (primeira validação bônus) +5 (mesmo contexto)
    
  day_3:
    event: "Segunda aplicação (L19 — CLI Coverage) + domínio DIFERENTE (cli/testing)"
    gravity: 45  # Stone — +15 (contexto diferente) +5 (validação adicional)
    note: "Já exerce influência moderada em decisões de auditoria"
    
  day_5:
    event: "Terceira aplicação (L20 — Systemic Platform Audit) + terceiro domínio (platform)"
    gravity: 60  # Stone (limite superior) — +15 (contexto diferente)
    
  day_7:
    event: "Quarta aplicação (L21 — Coverage + Doc Expurgo)"
    gravity: 65  # Boulder — +5 (mesmo contexto, saturação próxima)
    
  day_30:
    event: "Quinta aplicação + evidência CODE (testes automatizados do padrão)"
    gravity: 75  # Boulder — +5 (validação) +10 (upgrade evidência: AUDIT→CODE) -2 (decay 1 mês)
    
  day_60:
    event: "Primeira validação cross-project (Projeto B confirma o padrão)"
    gravity: 88  # Planet — +20 (cross-project) -2 (decay 2 meses) +5 (novo projeto)
    note: "Auto-aplicado — toda auditoria usa este padrão por default"
    
  day_90:
    event: "Segunda validação cross-project (Projeto C) + Cognitive Compression aplicada"
    gravity: 100  # Planet (máximo) — +20 (cross-project) +10 (compression upgrade)
    note: "Fundacional. Nenhum agente audita sem este padrão."
```

---

## 5. CAMPO GRAVITACIONAL (Gravitational Field)

### 5.1 Conceito

Quando o runtime toma uma decisão, o Cognitive Gravity Engine calcula o **campo gravitacional** — a influência combinada de TODAS as entradas de conhecimento relevantes sobre aquela decisão específica. Não é uma busca simples (ranked results). É um **campo de força** onde cada conhecimento exerce atração proporcional à sua gravidade e relevância.

### 5.2 Equação do Campo

```
field_strength(decision) = Σ (gravity_score(k) × relevance_score(k, decision) × distance_factor(k, decision))

Para cada entrada de conhecimento k relevante à decisão, onde:

  gravity_score(k):     gravidade da entrada (0-100, normalizado para 0-1 na fórmula)
  relevance_score:      similaridade semântica (0-1) entre k e o contexto da decisão
  distance_factor:      proximidade de domínio entre k e a decisão
```

### 5.3 Distance Factor

| Relação de Domínio | Factor | Significado |
|-------------------|--------|-------------|
| **Same domain** | 1.0 | Conhecimento do MESMO domínio — atração máxima |
| **Adjacent domain** | 0.7 | Domínio vizinho (ex: backend ↔ database, testing ↔ devops) |
| **Distant domain** | 0.3 | Domínio distante (ex: frontend ↔ security, documentation ↔ performance) |
| **Unrelated domain** | 0.0 | Sem relação — não exerce atração (ex: mobile ↔ orchestration) |

### 5.4 Mapa de Adjacência de Domínios

```yaml
domain_adjacency:
  backend:        [database, testing, architecture, sdk, performance]
  frontend:       [testing, uiux, performance, mobile]
  testing:        [backend, frontend, devops, performance, qa]
  devops:         [testing, monitoring, platform, security, infrastructure]
  architecture:   [backend, governance, documentation, platform, sdk]
  security:       [devops, governance, compliance, backend, infrastructure]
  performance:    [backend, frontend, testing, database, monitoring]
  documentation:  [architecture, sdk, governance, platform]
  data:           [backend, performance, ai_ml, monitoring]
  orchestration:  [architecture, platform, devops, workflows]
  governance:     [security, architecture, documentation, compliance]
  sdk:            [backend, frontend, documentation, mobile, platform]
  platform:       [devops, sdk, orchestration, documentation]
  monitoring:     [devops, performance, data, platform]
  ai_ml:          [data, backend, performance, platform]
```

### 5.5 Exemplo de Campo Gravitacional

**Contexto**: O Kernel está planejando uma auditoria de segurança. Stage 3 (PLAN STRATEGY) do metacognition pipeline consulta o Cognitive Gravity Engine.

```yaml
gravitational_field_example:
  decision: "Planejar auditoria de segurança do runtime"
  decision_domain: "security"
  
  field_calculation:
    - knowledge: "Cross-Agent Audit Pattern"
      gravity: 65 (Boulder)
      relevance: 0.85  # "auditoria" é semanticamente próximo
      distance: 0.7  # orchestration → security = adjacent domain
      contribution: 65/100 × 0.85 × 0.7 = 0.387
      
    - knowledge: "Security Audit Heuristic H-012"
      gravity: 45 (Stone)
      relevance: 0.95  # "segurança" é diretamente relevante
      distance: 1.0  # security → security = same domain
      contribution: 45/100 × 0.95 × 1.0 = 0.428
      
    - knowledge: "Extract-Then-Test Pattern"
      gravity: 55 (Stone)
      relevance: 0.30  # Pouca relação com auditoria de segurança
      distance: 0.3  # testing → security = distant domain
      contribution: 55/100 × 0.30 × 0.3 = 0.050
      
    - knowledge: "Database Migration Pattern (H-007)"
      gravity: 70 (Boulder)
      relevance: 0.05  # Nada a ver com segurança
      distance: 0.3  # data → security = distant domain
      contribution: 70/100 × 0.05 × 0.3 = 0.011
      
  total_field_strength: 0.876
  dominant_influence: "Security Audit Heuristic H-012" (0.428) — same domain, alta relevância
  secondary_influence: "Cross-Agent Audit Pattern" (0.387) — adjacent domain, alta gravidade
  
  decision_outcome: |
    O campo gravitacional sugere FORTEMENTE usar o Security Audit Heuristic
    (same domain) COMBINADO com o Cross-Agent Audit Pattern (padrão de execução).
    O Extract-Then-Test e Database Migration exercem atração desprezível (não aparecem
    nas sugestões automáticas).
    
    Output do engine para o Stage 3:
    "Gravitational field strength: 0.876. Dominant: H-012 (security audit).
     Secondary: Cross-Agent Audit Pattern (execution strategy).
     Suggested plan: Aplicar H-012 como metodologia + Cross-Agent como deployment pattern."
```

---

## 6. INTEGRAÇÃO COM O METACOGNITION PIPELINE

### 6.1 Onde a Gravidade Atua

O Cognitive Gravity Engine se integra em DOIS pontos do metacognition pipeline de 8 estágios:

```
┌──────────────────────────────────────────────────────────────────────┐
│                 METACOGNITION PIPELINE INTEGRATION                     │
│                                                                       │
│  Stage 1: SELF-ASSESS                                                 │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY CHECK (NOVO)                         │                  │
│  │  │                                              │                  │
│  │  │ "Qual o campo gravitacional para este        │                  │
│  │  │  domínio de tarefa?"                          │                  │
│  │  │                                              │                  │
│  │  │ field_strength < 0.20 → "Domínio sem          │                  │
│  │  │   conhecimento validado. Proceder com cautela."│                  │
│  │  │                                              │                  │
│  │  │ field_strength > 0.70 → "Domínio com forte    │                  │
│  │  │   campo gravitacional. Confiar nos padrões."  │                  │
│  │  └─────────────────────────────────────────────┘                  │
│  │                                                                    │
│  ▼                                                                    │
│  Stage 2: RETRIEVE MEMORY (MODIFICADO)                               │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY-WEIGHTED RETRIEVAL                    │                  │
│  │  │                                              │                  │
│  │  │ Resultados de busca semântica são REORDENADOS │                  │
│  │  │ por gravity_score × relevance_score.          │                  │
│  │  │                                              │                  │
│  │  │ Antes: ranking puro por similaridade          │                  │
│  │  │ Depois: ranking por gravidade × similaridade  │                  │
│  │  │                                              │                  │
│  │  │ Planet/Boulder > Stone > Pebble > Dust        │                  │
│  │  └─────────────────────────────────────────────┘                  │
│  │                                                                    │
│  ▼                                                                    │
│  Stage 3: PLAN STRATEGY (MODIFICADO)                                 │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY SUGGESTIONS                           │                  │
│  │  │                                              │                  │
│  │  │ Conhecimentos com gravity ≥ Stone (40+) no   │                  │
│  │  │ domínio da tarefa são AUTO-SUGERIDOS como     │                  │
│  │  │ parte da estratégia.                          │                  │
│  │  │                                              │                  │
│  │  │ Boulder (60+): "Considere aplicar X"         │                  │
│  │  │ Planet (80+): "X é o padrão estabelecido"    │                  │
│  │  └─────────────────────────────────────────────┘                  │
│  │                                                                    │
│  ▼                                                                    │
│  Stage 4-6: EXECUTE, VERIFY, CRITIQUE                                │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY ANCHORING                             │                  │
│  │  │                                              │                  │
│  │  │ Durante a execução, se o agente se desvia de  │                  │
│  │  │ um padrão Planet/Boulder, o Critic Chief é    │                  │
│  │  │ automaticamente acionado:                      │                  │
│  │  │                                              │                  │
│  │  │ "⚠️ Desviando de padrão Planet (gravity 85).  │                  │
│  │  │  Justificativa necessária."                   │                  │
│  │  └─────────────────────────────────────────────┘                  │
│  │                                                                    │
│  ▼                                                                    │
│  Stage 7: EXTRACT PATTERN                                            │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY UPDATE                                │                  │
│  │  │                                              │                  │
│  │  │ Se o padrão extraído JÁ EXISTE no knowledge   │                  │
│  │  │ graph:                                        │                  │
│  │  │   - Mesmo contexto → +5 gravity               │                  │
│  │  │   - Contexto diferente → +15 gravity          │                  │
│  │  │                                              │                  │
│  │  │ Se o padrão é NOVO:                           │                  │
│  │  │   - Criar com gravity = 10 (Dust)             │                  │
│  │  │   - Vincular ao domínio da task               │                  │
│  │  └─────────────────────────────────────────────┘                  │
│  │                                                                    │
│  ▼                                                                    │
│  Stage 8: UPDATE CAPABILITY MODEL                                    │
│  │                                                                    │
│  │  ┌─────────────────────────────────────────────┐                  │
│  │  │ GRAVITY TRACKING NO CMI                       │                  │
│  │  │                                              │                  │
│  │  │ Atualiza dimensão Transferência (15%) do CMI: │                  │
│  │  │   - avg_gravity_cross_domain = média de       │                  │
│  │  │     gravidade de padrões cross-domain          │                  │
│  │  │   - gravity_momentum = delta de gravidade     │                  │
│  │  │     nos últimos 30 dias                        │                  │
│  │  │                                              │                  │
│  │  │ Atualiza dimensão Consistência (10%) do CMI:  │                  │
│  │  │   - gravity_collapse_count = padrões que      │                  │
│  │  │     colapsaram (falha de validação)            │                  │
│  │  └─────────────────────────────────────────────┘                  │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 6.2 Algoritmo de Retrieval Modificado (Stage 2)

```yaml
gravity_weighted_retrieval:
  input:
    query: "contexto da decisão atual"
    top_k: 10
    min_relevance: 0.20
    
  process:
    1. semantic_search(query, top_k=50, min_relevance=0.20)
       # Busca ampla primeiro — pode trazer até 50 resultados
    
    2. for each result:
         gravity_boost = gravity_score(result) / 100  # 0.0 a 1.0
         distance_penalty = distance_factor(result.domain, decision.domain)  # 1.0, 0.7, 0.3, 0.0
         final_score = relevance(result) × 0.4 + gravity_boost × 0.35 + (1 - distance_penalty) × 0.25
       
       # 40% relevância semântica + 35% gravidade + 25% proximidade de domínio
    
    3. sort by final_score DESC
    4. return top_k results
    
  output:
    ranked_results:
      - entry: "Security Audit Heuristic H-012"
        relevance: 0.85
        gravity: 45 (Stone)
        distance: 1.0 (same domain)
        final_score: 0.85×0.4 + 0.45×0.35 + 0.0×0.25 = 0.498
        
      - entry: "Cross-Agent Audit Pattern"
        relevance: 0.75
        gravity: 65 (Boulder)
        distance: 0.7 (adjacent domain)
        final_score: 0.75×0.4 + 0.65×0.35 + 0.3×0.25 = 0.603  # MAIOR — gravity compensa
        
      # Cross-Agent Audit aparece ANTES apesar de menor relevância semântica,
      # porque sua gravidade (Boulder, 65) compensa a distância de domínio
```

---

## 7. INTERAÇÃO COM OUTROS MOTORES COGNITIVOS

### 7.1 Mapa de Interdependências

```
┌──────────────────────────────────────────────────────────────────────┐
│              COGNITIVE GRAVITY — DEPENDENCY WEB                        │
│                                                                       │
│                    ┌─────────────────────┐                            │
│                    │   COGNITIVE GRAVITY  │                            │
│                    │        (C1)          │                            │
│                    └──────────┬──────────┘                            │
│                               │                                       │
│     ┌─────────────────────────┼─────────────────────────┐            │
│     │                         │                         │            │
│     ▼                         ▼                         ▼            │
│  ┌──────────────┐    ┌─────────────────┐    ┌──────────────────┐     │
│  │ FEEDS FROM:  │    │ INFLUENCES:     │    │ CONSTRAINS:      │     │
│  │              │    │                 │    │                  │     │
│  │ C5 Immune    │    │ C3 Momentum     │    │ C2 Entropy       │     │
│  │ System       │    │ (high gravity → │    │ (collapsed       │     │
│  │ (validações) │    │  high momentum) │    │  gravity =       │     │
│  │              │    │                 │    │  contradiction   │     │
│  │ C4 Decision  │    │ C7 Mental       │    │  flag)           │     │
│  │ DNA          │    │ Energy          │    │                  │     │
│  │ (outcomes)   │    │ (Planet gravity │    │ C5 Immune        │     │
│  │              │    │  = low energy   │    │ System           │     │
│  │ C6 Pattern   │    │  cost to apply) │    │ (gravity < 0     │     │
│  │ Evolution    │    │                 │    │  → quarantine)   │     │
│  │ (versions)   │    │ C9 Insight      │    │                  │     │
│  │              │    │ Generator       │    │ C10 Compression  │     │
│  │ F2.3 Cross-  │    │ (gravity        │    │ (compressed      │     │
│  │ Project      │    │  clusters =     │    │  principles      │     │
│  │ Transfer     │    │  insight        │    │  inherit max     │     │
│  │ (projects)   │    │  sources)       │    │  gravity)        │     │
│  └──────────────┘    └─────────────────┘    └──────────────────┘     │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 7.2 Tabela de Interações

| Motor | Direção | Como Interage |
|-------|---------|---------------|
| **C5 — Cognitive Immune System** | ← Alimenta | Cada validação do Immune System (source_check → consistency_check → contradiction_check) atualiza `validation_count`, `evidence_strength` e `last_validated`. Falhas de validação disparam colapso de gravidade (-30). |
| **C4 — Decision DNA** | ← Alimenta | Outcomes de decisões (success/failure/mixed) alimentam `validation_count`. Se uma decisão baseada em conhecimento X falha, gravity de X sofre colapso. Se sucede, gravity aumenta. |
| **C6 — Pattern Evolution** | ← Alimenta | Cada nova versão de padrão (v1→v2→v3) conta como validação adicional. Upgrade de versão (v2→v3) com sucesso comprovado = +10 gravity. |
| **F2.3 — Cross-Project Transfer** | ← Alimenta | Cada projeto que valida o conhecimento incrementa `cross_project_count`. Este é o ganho de gravidade mais valioso (+20 por projeto). |
| **C11 — Wisdom Decay (F1.4)** | ↔ Bidirecional | Wisdom Decay dispara revalidações. Revalidação bem-sucedida = time_factor reset + validação adicional. Revalidação falha = colapso. Gravidade também alimenta Decay: conhecimentos com gravity < 30 têm TTL reduzido pela metade. |
| **C3 — Cognitive Momentum** | → Influencia | Gravidade alimenta momentum: domínios com alta gravidade média têm momentum positivo. Padrões Planet criam "inércia cognitiva" — o domínio tende a manter direção. |
| **C7 — Mental Energy** | → Influencia | Aplicar conhecimento de alta gravidade custa MENOS energia mental (não precisa reavaliar, testar, validar — já está provado). Planet gravity = custo energético mínimo. |
| **C9 — Insight Generator** | → Influencia | Clusters de conhecimento com gravidade similar no mesmo domínio são fontes de insight. O Insight Generator prioriza domínios com alta gravidade para gerar hipóteses. |
| **C10 — Cognitive Compression** | → Influencia | Quando N conhecimentos similares atingem gravity ≥ Stone (40+), o Compression Engine é acionado. O princípio comprimido HERDA a maior gravidade entre os N casos + bônus de compressão (+10). |
| **C2 — Cognitive Entropy** | → Restringe | Colapso de gravidade por falha de validação é um sinal de contradição. Múltiplos colapsos no mesmo domínio aumentam a entropia cognitiva — dispara consolidação. |
| **C14 — Cognitive Economy** | ↔ Bidirecional | Gravidade reduz custo cognitivo (não precisa reprocessar). Economy Engine aloca MENOS recursos para decisões em domínios com alta gravidade (confiança no campo). Em contrapartida, Economy Engine pode BLOQUEAR revalidações de conhecimento Planet se o custo de revalidar > valor da confirmação adicional. |

### 7.3 Integração com CMI

O Cognitive Gravity Engine contribui diretamente para DUAS dimensões do CMI:

```yaml
cmi_contribution:
  transferencia:
    weight_in_cmi: 15%
    how_gravity_feeds:
      - "validation_diversity mede diretamente transferência entre domínios"
      - "cross_project_count mede transferência entre projetos"
      - "avg_cross_domain_gravity é uma métrica de health da dimensão"
    target: "avg_cross_domain_gravity > 40 (Stone) em 5+ domínios"
    
  consistencia:
    weight_in_cmi: 10%
    how_gravity_feeds:
      - "Baixo gravity_collapse_count = sistema consistente"
      - "Alto gravity_collapse_count = contradições frequentes"
      - "Planet gravity padrões = baseline de consistência do runtime"
    target: "gravity_collapse_count < 2 por mês"
```

---

## 8. API DO ENGINE

### 8.1 Operações

#### CALCULATE_GRAVITY

```yaml
operation: calculate_gravity
description: "Calcula ou recalcula o gravity_score de uma entrada de conhecimento"
input:
  knowledge_id: "cross-agent-audit-pattern"
  knowledge_data:
    validation_count: 4
    unique_domains_validated: ["orchestration", "testing"]
    total_relevant_domains: 5
    cross_project_count: 0
    last_validated: "2026-07-30"
    evidence_type: "AUDIT"
output:
  gravity_score: 41
  gravity_level: "Stone"
  gravity_label: "●"
  components:
    validation_count_score: 0.40
    validation_diversity_score: 0.40
    cross_project_score: 0.00
    time_factor_score: 1.00
    evidence_strength_score: 0.60
  max_theoretical: "100 (requires cross-project + CODE evidence + 10+ validations)"
```

#### GET_GRAVITATIONAL_FIELD

```yaml
operation: get_gravitational_field
description: "Calcula o campo gravitacional para uma decisão específica"
input:
  decision_context: "Auditar segurança do runtime Cosca"
  decision_domain: "security"
  top_k: 15
  min_gravity: 20  # Ignorar Dust
output:
  field_strength: 0.876
  dominant_knowledge:
    - entry: "Security Audit Heuristic H-012"
      contribution: 0.428
      gravity: 45
      level: "Stone"
    - entry: "Cross-Agent Audit Pattern"
      contribution: 0.387
      gravity: 65
      level: "Boulder"
  recommendations:
    - "Aplicar H-012 como metodologia de auditoria"
    - "Usar Cross-Agent Audit Pattern para deployment paralelo"
  warnings:
    - "Nenhum conhecimento Planet no domínio security — não há baseline obrigatório"
    - "Field strength 0.876: confiável mas não blindado — revisar decisão"
```

#### UPDATE_GRAVITY

```yaml
operation: update_gravity
description: "Atualiza gravidade após um evento de validação ou falha"
input:
  knowledge_id: "cross-agent-audit-pattern"
  event:
    type: "validation_success"  # ou "validation_failure", "cross_project", "evidence_upgrade"
    context: "security_audit_2026_q3"
    domain: "security"
    is_new_domain: true  # security é um domínio novo para este padrão
    evidence_type: "AUDIT"
    project: "cosca-test"
output:
  previous_gravity: 41
  delta: +15  # contexto diferente (security vs orchestration/testing)
  new_gravity: 56
  new_level: "Stone" (↑ de 41 para 56, ainda Stone mas mais próximo de Boulder)
  updated_fields:
    - validation_count: 4 → 5
    - unique_domains_validated: ["orchestration", "testing"] → ["orchestration", "testing", "security"]
    - last_validated: "2026-07-30"
```

#### DETECT_GRAVITY_ANCHORING_VIOLATION

```yaml
operation: detect_gravity_anchoring_violation
description: "Verifica se o agente está se desviando de conhecimento de alta gravidade"
input:
  current_action: "Auditar manualmente sem usar H-012"
  domain: "security"
  gravitational_field: 0.876
output:
  violations:
    - knowledge: "Security Audit Heuristic H-012"
      gravity: 45
      level: "Stone"
      severity: "WARNING"  # Stone não é bloqueante, apenas aviso
    - knowledge: "Cross-Agent Audit Pattern"
      gravity: 65
      level: "Boulder"
      severity: "CRITICAL"  # Boulder requer justificativa formal
      required_action: "Acionar Contrafactual Gate (A2) + justificativa do Critic Chief"
```

### 8.2 CLI (Runtime Go)

```bash
# Calcular gravidade de um padrão específico
cosca gravity calculate --knowledge-id "cross-agent-audit-pattern"

# Visualizar campo gravitacional para um domínio
cosca gravity field --domain "security" --top-k 10

# Atualizar gravidade após evento
cosca gravity update \
  --knowledge-id "cross-agent-audit-pattern" \
  --event validation_success \
  --domain "security" \
  --context "security_audit_2026_q3"

# Health check — padrões com gravidade estagnada
cosca gravity health --stale-days 90

# Dashboard: top 10 conhecimentos por gravidade
cosca gravity top --limit 10 --min-level Stone

# Simular: "O que acontece com a gravidade se este padrão falhar?"
cosca gravity simulate \
  --knowledge-id "cross-agent-audit-pattern" \
  --event validation_failure
```

---

## 9. ESTRUTURA DE DADOS

### 9.1 Gravity Entry (Armazenamento por Entrada de Conhecimento)

```yaml
gravity_entry:
  knowledge_id: "cross-agent-audit-pattern"
  knowledge_type: "pattern"  # pattern | heuristic | principle | learning | decision
  knowledge_source: "internal/embed/cosca/memory/agent/cosca-kernel/patterns.md"
  
  # Componentes da Fórmula
  validation_count: 4
  validation_history:
    - date: "2026-07-28"
      context: "runtime_coverage_audit"
      domain: "orchestration"
      outcome: "success"
      agent: "cosca-kernel"
      evidence: "AUDIT"
    - date: "2026-07-29"
      context: "cli_coverage_audit"
      domain: "testing"
      outcome: "success"
      agent: "cosca-kernel"
      evidence: "AUDIT"
    - date: "2026-07-30"
      context: "systemic_platform_audit"
      domain: "orchestration"  # mesmo domínio → +5, não +15
      outcome: "success"
      agent: "cosca-kernel"
      evidence: "AUDIT"
    - date: "2026-07-30"
      context: "coverage_doc_expurgo"
      domain: "documentation"  # domínio diferente → +15
      outcome: "success"
      agent: "cosca-kernel"
      evidence: "AUDIT"
  
  unique_domains_validated: ["orchestration", "testing", "documentation"]
  total_relevant_domains: 5  # orchestration, testing, documentation, quality, platform
  cross_project_validations:
    []  # Nenhum projeto externo ainda validou
  
  last_validated: "2026-07-30"
  evidence_type: "AUDIT"
  evidence_score: 0.6
  
  # Scores Calculados
  gravity_score: 44  # Recalculado com 3 domínios (orchestration, testing, documentation)
  gravity_level: "Stone"
  gravity_label: "●"
  
  # Metadados de Influência
  times_suggested: 8  # Quantas vezes foi sugerido a agentes
  times_adopted: 6    # Quantas vezes foi efetivamente usado
  times_overridden: 1  # Quantas vezes foi ignorado (com justificativa)
  adoption_rate: 0.75  # times_adopted / times_suggested
  
  # Histórico de Gravidade
  gravity_history:
    - date: "2026-07-28"
      score: 25
      level: "Pebble"
      event: "Primeira validação"
    - date: "2026-07-29"
      score: 40
      level: "Stone"
      event: "Validação em domínio diferente"
    - date: "2026-07-30"
      score: 44
      level: "Stone"
      event: "Validação em documentation domain"
```

### 9.2 Gravitational Field Snapshot (Cache por Sessão)

```yaml
field_snapshot:
  session_id: "session-2026-07-30-001"
  calculated_at: "2026-07-30T14:30:00Z"
  ttl: 3600  # 1 hora — recalculado se expirar
  
  domains:
    security:
      field_strength: 0.876
      knowledge_count: 5
      dominant_entries:
        - knowledge_id: "H-012"
          contribution: 0.428
          gravity_level: "Stone"
        - knowledge_id: "cross-agent-audit-pattern"
          contribution: 0.387
          gravity_level: "Boulder"
      health: "healthy"  # healthy | weak | critical
      
    testing:
      field_strength: 0.920
      knowledge_count: 8
      dominant_entries:
        - knowledge_id: "extract-then-test"
          contribution: 0.510
          gravity_level: "Stone"
        - knowledge_id: "cross-agent-audit-pattern"
          contribution: 0.340
          gravity_level: "Boulder"
      health: "healthy"
      
    frontend:
      field_strength: 0.120
      knowledge_count: 1
      dominant_entries: []
      health: "critical"  # Campo gravitacional muito fraco — domínio subdesenvolvido
      recommendation: "Priorizar validação de conhecimento no domínio frontend"
```

---

## 10. EXEMPLOS PRÁTICOS DETALHADOS

### 10.1 Exemplo A: Cross-Agent Audit Pattern (Trajetória Real)

```yaml
example_cross_agent_audit:
  knowledge: "Cross-Agent Parallel Audit (Pattern 001)"
  source: "cosca-kernel/patterns.md"
  
  # ─── DIA 0: Extração Inicial ───
  day_0:
    gravity: 10
    level: "Dust"
    status: "Hipótese — extraído do learning L18, nunca aplicado standalone"
    
  # ─── DIA 1: Primeira Validação ───
  day_1:
    event: "Aplicado em L18 — Runtime Coverage Audit"
    domain: "orchestration"
    outcome: "SUCCESS — audit completo em < 3 minutos"
    gravity_gain: +10 (primeira validação) +5 (mesmo contexto) = +15
    new_gravity: 25
    new_level: "Pebble"
    
  # ─── DIA 3: Validação em Contexto Diferente ───
  day_3:
    event: "Aplicado em L19 — CLI Coverage"
    domain: "testing"  # DOMÍNIO DIFERENTE!
    outcome: "SUCCESS"
    gravity_gain: +15 (contexto diferente)
    new_gravity: 40
    new_level: "Stone"  # ATINGE INFLUÊNCIA MODERADA
    note: "Começa a ser sugerido em decisões de auditoria e testing"
    
  # ─── DIA 5: Expansão para Platform ───
  day_5:
    event: "Aplicado em L20 — Systemic Platform Audit"
    domain: "platform"  # TERCEIRO DOMÍNIO!
    outcome: "SUCCESS"
    gravity_gain: +15 (contexto diferente)
    new_gravity: 55
    new_level: "Stone" (limite superior)
    
  # ─── DIA 7: Consolidação ───
  day_7:
    event: "Aplicado em L21 — Coverage + Doc Expurgo"
    domain: "documentation"  # QUARTO DOMÍNIO!
    outcome: "SUCCESS"
    gravity_gain: +15 (contexto diferente)
    new_gravity: 65  # Nota: 55 + 15 - 5 (saturação começa a aplicar)
    new_level: "Boulder"  # INFLUÊNCIA FORTE
    note: "Auto-sugerido para TODAS as tarefas de auditoria, testing, platform, documentation"
    
  # ─── DIA 30: Upgrade de Evidência ───
  day_30:
    event: "Evidência atualizada para CODE (testes automatizados do padrão)"
    gravity_gain: +10 (upgrade AUDIT → CODE) -2 (decay 1 mês)
    new_gravity: 73
    new_level: "Boulder"
    
  # ─── DIA 60: Primeira Validação Cross-Project (FUTURO) ───
  day_60_future:
    event: "Projeto B (cosca-analytics-dashboard) valida o padrão"
    domain: "orchestration"
    project: "cosca-analytics-dashboard"
    outcome: "SUCCESS"
    gravity_gain: +20 (cross-project) -2 (decay)
    new_gravity: 88  # 73 + 20 - 2 (decay 2 meses) - 3 (saturação)
    new_level: "Planet"
    note: "FUNDACIONAL. Nenhum agente audita sem este padrão."
    
  # ─── DIA 90: Cenário de Falha (HIPOTÉTICO) ───
  day_90_failure_scenario:
    event: "FALHA — padrão causou race condition em auditoria paralela"
    gravity_loss: -30 (colapso por falha)
    new_gravity: 58  # Cairia de 88 para 58
    new_level: "Stone"  # De Planet para Stone em UMA falha
    note: "Gravidade colapsa rápido. Recuperação requer 2+ validações bem-sucedidas."
```

### 10.2 Exemplo B: Hipótese Nova (Nunca Validada)

```yaml
example_new_hypothesis:
  knowledge: "Hipótese: Spawnar 8 agentes é sempre melhor que 4 agentes"
  type: "hypothesis"
  source: "cosca-architecture/learnings.md"
  evidence: "ASSERTION"
  
  current_state:
    gravity: 10
    level: "Dust"
    validation_count: 0
    validation_diversity: 0
    cross_project_count: 0
    last_validated: null  # Nunca validado
    evidence_strength: 0.3  # ASSERTION
    
  behavior:
    - "Aparece em buscas explícitas sobre 'paralelismo de agentes'"
    - "NUNCA sugerido automaticamente"
    - "NÃO influencia o gravitational field"
    - "Se um agente tentar aplicá-lo, alerta: 'Conhecimento não validado (Dust). Proceder com cautela.'"
    
  path_to_influence:
    - "1ª validação → +15 (bônus primeira + mesmo contexto) → Gravity 25 (Pebble)"
    - "2ª validação em contexto diferente → +15 → Gravity 40 (Stone)"
    - "3ª validação em contexto diferente → +15 → Gravity 55 (Stone)"
    - "Cross-project → +20 → Gravity 75 (Boulder)"
    - "5+ validações + CODE evidence → Gravity 90+ (Planet)"
```

### 10.3 Exemplo C: Padrão Desprovado (Colapso de Gravidade)

```yaml
example_disproven_pattern:
  knowledge: "Hypothetical Pattern X — Database connection pooling manual"
  history:
    - day_0: gravity 10 (Dust)
    - day_5: 3 validações bem-sucedidas, mesmo domínio → gravity 25 (Pebble)
    - day_15: 2 validações cross-domain → gravity 55 (Stone)
    - day_30: 1 validação cross-project → gravity 75 (Boulder)
    
  collapse_event:
    day_45:
      event: "FALHA CRÍTICA — connection pool causou deadlock em produção"
      domain: "database"
      evidence: "BENCHMARK (reproduzido)"
      gravity_loss: -30
      new_gravity: 45  # De Boulder (75) para Stone baixo (45)
      new_level: "Stone"
      
  aftermath:
    - "Padrão perdeu 40% da gravidade em UMA falha"
    - "Não é mais auto-sugerido (precisa ser Boulder/Planet)"
    - "Marcado com flag: 'failed_validation: 2026-07-30, reason: deadlock'"
    - "Se falhar NOVAMENTE → gravity 15 (Dust) → movido para quarantine/"
    
  recovery_path:
    - "Corrigir o padrão (v2)"
    - "Revalidar com sucesso em 2+ contextos diferentes"
    - "Recuperar gravity para Stone (40+)"
    - "NÃO recupera automaticamente — precisa PROVAR que a correção funciona"
```

---

## 11. GOVERNANÇA E CICLO DE VIDA

### 11.1 Atualização de Gravidade

```yaml
gravity_update_policy:
  trigger: "Automático — após cada task (metacognition pipeline Stage 7-8)"
  manual_override: "Permitido apenas para Memory Chief + Critic Chief em consenso"
  
  automatic_updates:
    - "Validação bem-sucedida → +5 a +20 (calculado pelo engine)"
    - "Falha de validação → -30 (colapso)"
    - "Decaimento mensal → -2/mês após 90 dias sem revalidação"
    - "Upgrade de evidência → +10/nível (ASSERTION→AUDIT→BENCHMARK→CODE)"
    
  manual_overrides:
    - "Ajuste de gravity por decisão do Critic Chief (ex: padrão com viés conhecido)"
    - "Reset de gravity em migração de knowledge graph"
    - "Forçar gravity mínima para conhecimento crítico de compliance"
    
  audit_trail:
    - "Toda mudança de gravity é registrada em gravity_history"
    - "Inclui: timestamp, evento, delta, novo_score, agente responsável"
    - "Queryable: 'Quais padrões perderam mais gravidade este mês?'"
```

### 11.2 Revisão Periódica

```yaml
gravity_review:
  frequency: "Mensal (pelo Memory Chief)"
  scope:
    - "Top 10 conhecimentos por gravidade — ainda são válidos?"
    - "Bottom 10 conhecimentos (Dust há > 90 dias) — descartar ou promover?"
    - "Padrões com adoption_rate < 0.3 — por que não são adotados?"
    - "Domínios com field_strength < 0.30 — precisam de investimento?"
    - "Colapsos de gravidade no período — causas raiz?"
    
  escalation:
    - "Padrão Planet com 0 adoções em 30 dias → alerta ao CTO Chief"
    - "Domínio com field_strength < 0.15 → prioridade de investimento"
    - "5+ colapsos no mesmo mês → Cognitive Entropy (C2) alert"
```

### 11.3 Staleness e Expiração

```yaml
gravity_staleness:
  detection: "Background job semanal (domingo 00:00 UTC)"
  
  stale_criteria:
    - "Gravity > 50 (Stone+) mas last_validated > 180 dias"
    - "Nenhuma sugestão adotada nos últimos 90 dias"
    - "adoption_rate caiu abaixo de 0.2"
    
  action:
    - "Marcar como 'stale'"
    - "Reduzir time_factor para 0.1 (penalidade de recência)"
    - "Disparar revalidação automática na próxima task do domínio"
    - "Se revalidação falhar → colapso (-30)"
    - "Se revalidação suceder → time_factor reset + gravity restaurado"
```

---

## 12. IMPLEMENTAÇÃO TÉCNICA

### 12.1 Stack

```yaml
implementation_stack:
  storage: "SQLite (mesmo DB do Semantic Memory — .cosca/memory/vectors.db)"
  tables:
    - gravity_entries: "gravity_score, componentes, histórico"
    - gravity_history: "log de todas as mudanças de gravidade"
    - domain_adjacency: "matriz de adjacência de domínios"
    - field_snapshots: "cache de campos gravitacionais por sessão"
    
  computation: "Go runtime (engine/gravity/)"
  integration:
    - "Semantic Memory Engine: consulta similarity para relevance_score"
    - "Provider Chief: embeddings para cálculo de similaridade semântica"
    - "Context Chief: detecção de domínio da task atual"
    - "Critic Chief: validação de overrides (Contrafactual Gate)"
```

### 12.2 Esquema SQLite

```sql
-- Tabela principal de gravidade
CREATE TABLE IF NOT EXISTS gravity_entries (
    knowledge_id TEXT PRIMARY KEY,
    knowledge_type TEXT NOT NULL,  -- pattern, heuristic, principle, learning, decision
    knowledge_source TEXT NOT NULL,
    
    -- Componentes da fórmula
    validation_count INTEGER DEFAULT 0,
    unique_domains_validated TEXT,  -- JSON array
    total_relevant_domains INTEGER DEFAULT 1,
    cross_project_count INTEGER DEFAULT 0,
    last_validated TEXT,  -- ISO 8601
    evidence_type TEXT DEFAULT 'ASSERTION',
    
    -- Scores calculados
    gravity_score REAL DEFAULT 10.0,
    gravity_level TEXT DEFAULT 'Dust',
    gravity_label TEXT DEFAULT '░░',
    
    -- Métricas de influência
    times_suggested INTEGER DEFAULT 0,
    times_adopted INTEGER DEFAULT 0,
    times_overridden INTEGER DEFAULT 0,
    adoption_rate REAL DEFAULT 0.0,
    
    -- Metadados
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    stale_since TEXT,
    quarantine_flag BOOLEAN DEFAULT 0
);

-- Histórico de mudanças de gravidade (audit trail)
CREATE TABLE IF NOT EXISTS gravity_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- validation_success, validation_failure, evidence_upgrade, decay, manual
    delta REAL NOT NULL,
    previous_score REAL NOT NULL,
    new_score REAL NOT NULL,
    domain TEXT,
    context TEXT,
    agent TEXT,
    timestamp TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (knowledge_id) REFERENCES gravity_entries(knowledge_id)
);

-- Matriz de adjacência de domínios
CREATE TABLE IF NOT EXISTS domain_adjacency (
    domain_a TEXT NOT NULL,
    domain_b TEXT NOT NULL,
    distance_factor REAL NOT NULL,  -- 1.0, 0.7, 0.3, 0.0
    PRIMARY KEY (domain_a, domain_b)
);

-- Cache de campos gravitacionais
CREATE TABLE IF NOT EXISTS field_snapshots (
    session_id TEXT NOT NULL,
    domain TEXT NOT NULL,
    field_strength REAL NOT NULL,
    knowledge_count INTEGER NOT NULL,
    dominant_entries TEXT,  -- JSON array
    calculated_at TEXT DEFAULT (datetime('now')),
    ttl INTEGER DEFAULT 3600,
    PRIMARY KEY (session_id, domain)
);

-- Índices
CREATE INDEX IF NOT EXISTS idx_gravity_score ON gravity_entries(gravity_score DESC);
CREATE INDEX IF NOT EXISTS idx_gravity_level ON gravity_entries(gravity_level);
CREATE INDEX IF NOT EXISTS idx_gravity_domain ON gravity_entries(unique_domains_validated);
CREATE INDEX IF NOT EXISTS idx_gravity_stale ON gravity_entries(stale_since);
CREATE INDEX IF NOT EXISTS idx_gravity_history_knowledge ON gravity_history(knowledge_id, timestamp);
```

### 12.3 Fluxo de Execução no Runtime Go

```go
// Pseudo-código do fluxo principal

// 1. Stage 1: SELF-ASSESS — calcular campo gravitacional
func (g *GravityEngine) AssessDomain(domain string) (*FieldStrength, error) {
    snapshot, err := g.getCachedField(domain)
    if err != nil || snapshot.IsExpired() {
        field := g.calculateGravitationalField(domain)
        g.cacheField(domain, field)
        return field, nil
    }
    return snapshot, nil
}

// 2. Stage 2: RETRIEVE MEMORY — busca ponderada por gravidade
func (g *GravityEngine) GravityWeightedSearch(query string, domain string, topK int) ([]RankedResult, error) {
    // Busca semântica ampla (até 50 resultados)
    semanticResults := g.semanticMemory.Search(query, 50, 0.20)
    
    // Reordenar por gravidade × relevância × distância
    for i, result := range semanticResults {
        gravity := g.getGravity(result.KnowledgeID)
        distance := g.getDomainDistance(result.Domain, domain)
        semanticResults[i].FinalScore = 
            result.Relevance * 0.4 + 
            (gravity.Score / 100.0) * 0.35 + 
            (1.0 - distance) * 0.25
    }
    
    sort.Slice(semanticResults, func(i, j int) bool {
        return semanticResults[i].FinalScore > semanticResults[j].FinalScore
    })
    
    return semanticResults[:topK], nil
}

// 3. Stage 3: PLAN STRATEGY — sugestões de gravidade
func (g *GravityEngine) GetStrategySuggestions(domain string) ([]Suggestion, error) {
    // Conhecimentos com gravity ≥ 40 (Stone+) no domínio ou adjacentes
    entries := g.getGravityEntries(domain, 40)
    
    var suggestions []Suggestion
    for _, entry := range entries {
        if entry.Level == "Boulder" || entry.Level == "Planet" {
            suggestions = append(suggestions, Suggestion{
                KnowledgeID: entry.KnowledgeID,
                Priority:    "HIGH",
                Message:     fmt.Sprintf("%s é o padrão estabelecido para %s", entry.KnowledgeID, domain),
                Overridable: entry.Level != "Planet",
            })
        }
    }
    return suggestions, nil
}

// 4. Stage 7: EXTRACT PATTERN — atualizar gravidade
func (g *GravityEngine) UpdateGravity(knowledgeID string, event ValidationEvent) (*GravityEntry, error) {
    entry := g.getGravityEntry(knowledgeID)
    
    var delta float64
    switch event.Type {
    case "validation_success":
        if event.IsNewDomain {
            delta = 15.0
        } else {
            delta = 5.0
        }
        if entry.ValidationCount == 0 {
            delta += 10.0 // bônus primeira validação
        }
    case "validation_failure":
        delta = -30.0
    case "cross_project":
        delta = 20.0
    case "evidence_upgrade":
        delta = 10.0
    case "decay":
        delta = -2.0
    }
    
    // Aplicar saturação
    newScore := math.Min(math.Max(entry.GravityScore + delta, 10.0), 100.0)
    
    entry.GravityScore = newScore
    entry.Level = g.classifyGravity(newScore)
    entry.LastValidated = time.Now()
    
    g.saveGravityEntry(entry)
    g.logGravityHistory(knowledgeID, event, delta, entry)
    
    // Verificar se colapsou para quarantine
    if newScore <= 0 {
        g.quarantine(knowledgeID, "gravity_collapse")
    }
    
    return entry, nil
}
```

---

## 13. CONSTRAINTS E REGRAS DE OURO

### 13.1 Regras Imutáveis

1. **Nenhum conhecimento com `evidence_strength = ASSERTION` pode atingir gravity > 50 (Stone).** Sem evidência concreta, sem influência moderada.

2. **Falha de validação SEMPRE causa colapso (-30), sem exceções.** Gravidade colapsa rápido quando desprovada.

3. **Conhecimento em `quarantine/` (Cognitive Immune System) tem gravity = 0.** Não exerce influência até ser reabilitado.

4. **Override de Planet gravity requer Contrafactual Gate (A2) + Critic Chief + registro em Decision DNA.** Não se ignora conhecimento fundacional sem ritual formal.

5. **Gravity NUNCA cai abaixo de 10 (Dust superior) por decaimento temporal.** Conhecimento "morto" ainda existe — só não influencia.

6. **`cross_project_count` requer F2.3 (Cross-project Knowledge Transfer) operacional.** Sem federação cross-project, esta dimensão é 0.

### 13.2 Vieses Conhecidos e Mitigações

| Viés | Risco | Mitigação |
|------|-------|-----------|
| **Rich-get-richer** | Padrões com alta gravidade recebem mais atenção → mais validações → gravidade ainda maior. Padrões novos não conseguem competir. | `validation_count` satura em 10. Após saturação, só `diversity` e `cross_project` aumentam gravidade — impossible de farmar no mesmo contexto. |
| **Recency bias** | `time_factor` pode fazer conhecimento novo (não testado) parecer melhor que conhecimento antigo (muito testado). | `time_factor` tem peso 0.10 (o menor de todos). Um padrão com 10 validações e time_factor 0.1 ainda tem gravity 30+ (Pebble alto). |
| **Domain siloing** | Conhecimento de domínios pouco usados nunca ganha gravidade. | `distance_factor` permite que conhecimento de domínios adjacentes influencie. Campo gravitacional fraco gera alerta de investimento. |
| **Echo chamber** | Agentes validam o mesmo conhecimento repetidamente sem escrutínio crítico. | Cognitive Immune System (C5) verifica contradições antes da validação. Critic Chief revisa padrões Planet trimestralmente. |
| **Overfitting cross-project** | Projetos similares (mesmo stack, mesmo domínio) inflam `cross_project_count`. | `cross_project_count` satura em 5. Projetos precisam ser SUBSTANCIALMENTE diferentes para contar (verificação de dissimilaridade de stack). |

---

## 14. MÉTRICAS DE HEALTH DO ENGINE

### 14.1 KPIs do Cognitive Gravity Engine

```yaml
gravity_engine_kpis:
  k1_avg_gravity:
    description: "Gravidade média de todos os conhecimentos ativos"
    target: "> 30 (média entre Pebble e Stone)"
    current: "A ser medido após Fase 2 deployment"
    alert: "< 20 → conhecimento não está sendo validado"
    
  k2_gravity_momentum:
    description: "Delta da gravidade média nos últimos 30 dias"
    target: "> 0 (gravidade crescendo)"
    alert: "< -5 → mais colapsos que validações — sistema regredindo"
    
  k3_domain_field_coverage:
    description: "% de domínios com field_strength > 0.50"
    target: "> 70%"
    alert: "< 40% → muitos domínios sem conhecimento gravitacional"
    
  k4_adoption_rate:
    description: "Taxa média de adoção de sugestões de gravidade"
    target: "> 0.60 (60% das sugestões Boulder/Planet são adotadas)"
    alert: "< 0.30 → sugestões irrelevantes — recalibrar relevance_score"
    
  k5_collapse_rate:
    description: "Colapsos de gravidade por mês"
    target: "< 2"
    alert: "> 5 → conhecimento instável — verificar Cognitive Entropy (C2)"
    
  k6_staleness_ratio:
    description: "% de conhecimentos com gravity > 40 mas last_validated > 180 dias"
    target: "< 10%"
    alert: "> 25% → conhecimento envelhecendo sem revalidação — integridade comprometida"
```

### 14.2 Dashboard (Console)

```
┌─────────────────────────────────────────────────────────────────┐
│                 COGNITIVE GRAVITY DASHBOARD                       │
│                                                                  │
│  Avg Gravity: 38.2 (▲ +2.1 este mês)          Health: 🟡 FAIR    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ DISTRIBUIÇÃO DE GRAVIDADE                                    │ │
│  │                                                              │ │
│  │ Dust:    12 conhecimentos  ░░░░░░░░░░░░ 12                   │ │
│  │ Pebble:   8 conhecimentos  ◌◌◌◌◌◌◌◌ 8                       │ │
│  │ Stone:    5 conhecimentos  ●●●●● 5                           │ │
│  │ Boulder:  2 conhecimentos  ◉◉ 2                              │ │
│  │ Planet:   0 conhecimentos  (nenhum — alvo: 1 até Fase 3)     │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  CAMPO GRAVITACIONAL POR DOMÍNIO:                                │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ testing        ████████████████████████ 0.92 🟢           │   │
│  │ orchestration  ███████████████████ 0.87 🟢                │   │
│  │ security       ████████████████ 0.73 🟢                   │   │
│  │ documentation  ██████████████ 0.65 🟡                      │   │
│  │ backend        ████████ 0.38 🔴                            │   │
│  │ frontend       ██ 0.12 🔴                                  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  TOP 5 GRAVIDADE:                                                │
│  1. Cross-Agent Audit Pattern      65 ● Boulder                  │
│  2. Extract-Then-Test Pattern      55 ● Stone                    │
│  3. Three-Wave Activation          48 ● Stone                    │
│  4. Security Audit H-012           45 ● Stone                    │
│  5. Wisdom Decay Revalidation      40 ● Stone                    │
│                                                                  │
│  ALERTAS:                                                        │
│  ⚠️  frontend field_strength 0.12 — crítico (4 dias sem melhora) │
│  ⚠️  3 conhecimentos Dust > 90 dias — revisar ou descartar       │
│  ⚠️  Padrão 'Manual DB Pooling' colapsou (75→45) — investigar    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 15. ROADMAP DE EVOLUÇÃO

### 15.1 Versão 1.0.0 (Atual — Fase 2)

- [x] Fórmula de gravidade com 5 componentes
- [x] 5 níveis de gravidade (Dust → Planet)
- [x] Campo gravitacional com distance_factor
- [x] Integração com metacognition pipeline (Stages 1, 2, 3, 7, 8)
- [x] API completa (calculate, update, field, detect_violations)
- [x] Estrutura de dados SQLite
- [x] Métricas de health (6 KPIs)
- [x] Governança e ciclo de vida

### 15.2 Versão 1.1.0 (Planejado — Fase 2 avançada)

- [ ] **Gravity Clusters**: Agrupar conhecimentos por similaridade gravitacional para detecção de "órbitas" de conhecimento
- [ ] **Gravity Predictions**: Prever quais conhecimentos atingirão Boulder/Planet nos próximos 30 dias
- [ ] **Cross-Project Gravity Sync**: Sincronização automática de gravity scores entre projetos federados (requer F2.3 completo)
- [ ] **Gravity-aware Agent Routing**: Selecionar agentes com base na gravidade do conhecimento que eles possuem no domínio da task

### 15.3 Versão 2.0.0 (Planejado — Fase 3)

- [ ] **Dynamic Gravity Weights**: Pesos da fórmula (0.30, 0.25, 0.20, 0.10, 0.15) calibrados automaticamente por ML com base em outcomes reais
- [ ] **Gravity-based Insight Generation**: Clusters de alta gravidade como fonte primária para o Insight Generator (C9)
- [ ] **Gravitational Memory**: Conhecimento Planet é "comprimido" no system prompt dos agentes (não precisa ser buscado — está sempre presente)
- [ ] **Anti-Gravity Detection**: Identificar conhecimentos que sistematicamente repelem validação (sempre falham, mas nunca são descartados) — viés de confirmação

---

## 16. APÊNDICE: Glossário

| Termo | Definição |
|-------|-----------|
| **Gravidade Cognitiva** | Medida de influência que um conhecimento exerce sobre decisões do runtime, baseada em validações acumuladas. |
| **Massa Gravitacional** | Termo informal para `gravity_score` — quanto maior, mais o conhecimento "atrai" decisões. |
| **Campo Gravitacional** | Influência combinada de todos os conhecimentos relevantes sobre uma decisão específica. |
| **Gravity Collapse** | Queda abrupta de gravidade (-30) causada por falha de validação. |
| **Gravity Decay** | Perda gradual de gravidade (-2/mês) por inatividade ou falta de revalidação. |
| **Gravity Anchoring** | Mecanismo que alerta quando um agente se desvia de conhecimento de alta gravidade. |
| **Distance Factor** | Penalidade aplicada à influência de conhecimento de domínios diferentes. |
| **Adoption Rate** | Taxa com que sugestões de gravidade são efetivamente adotadas pelos agentes. |
| **Orbital Decay** | Termo informal para Gravity Decay — conhecimento "perde órbita" e deixa de influenciar. |
| **Gravitational Memory** | Conhecimento Planet que é tão fundamental que reside permanentemente no contexto dos agentes. |

---

## 17. REFERÊNCIAS

### Documentos Relacionados

| Documento | Caminho | Relação |
|-----------|---------|---------|
| COGNITIVE_MATURITY.md §5 C1 | `internal/embed/cosca/architecture/COGNITIVE_MATURITY.md` | Definição arquitetural original da Gravidade Cognitiva |
| cognitive-maturity-implementation.md F2.4 | `internal/embed/cosca/workflows/cognitive-maturity-implementation.md` | Task de implementação: "Construir Cognitive Gravity system" |
| Semantic Memory Engine | `../semantic-memory/SKILL.md` | Fornece `relevance_score` via busca semântica |
| Cognitive Immune System | `../cognitive-immune/IMMUNE_SYSTEM.md` | Validações alimentam `validation_count` e `evidence_strength` |
| Pattern Evolution | `../pattern-evolution/PATTERN_LIFECYCLE.md` | Versionamento de padrões conta como validações adicionais |
| Metacognition Pipeline | `../../workflows/metacognition-pipeline.md` | Integração nos Stages 1, 2, 3, 7, 8 |
| Wisdom Decay | `../memory/wisdom-decay.md` | Dispara revalidações que afetam gravidade |
| Decision DNA Format | `../../memory/DECISION_DNA_FORMAT.md` | Outcomes de decisões validam ou colapsam gravidade |

### Agentes Responsáveis

| Agente | Papel |
|--------|-------|
| **cosca-memory-chief** | Owner do engine. Mantém gravity scores, revisão mensal, health dashboard. |
| **cosca-semantic-memory** | Fornece `relevance_score` para cálculo de campo gravitacional. |
| **cosca-critic** | Valida overrides de Planet/Boulder (Contrafactual Gate). Aprova ou rejeita desvios. |
| **cosca-cto** | Revisão trimestral da calibração dos pesos da fórmula. |

---

## 18. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|---------|------|--------|----------|
| 1.0.0 | 2026-07-30 | Cosca Memory Chief | Documento fundacional. Fórmula de gravidade com 5 componentes (validation_count, validation_diversity, cross_project_count, time_factor, evidence_strength). 5 níveis (Dust, Pebble, Stone, Boulder, Planet). Campo gravitacional com distance_factor. Integração completa com metacognition pipeline. Acumulação e colapso de gravidade. API completa. Esquema SQLite. 6 KPIs de health. Glossário. Roadmap v1.1 e v2.0. |

---

> **"Cada ideia atrai outras relacionadas. Quanto mais evidências uma decisão recebe, maior sua gravidade. Nova decisão → Gravidade baixa → Mais projetos validam → Gravidade aumenta → Passa a influenciar automaticamente novas decisões."**
>
> — The Don, definindo o conceito de Gravidade Cognitiva

> **"Gravidade não é sobre estar certo. É sobre ser impossível de ignorar."**
>
> — Cosca Memory Chief, 2026-07-30
