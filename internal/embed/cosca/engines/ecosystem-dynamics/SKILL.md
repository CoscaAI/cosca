# ECOSYSTEM DYNAMICS ENGINE ★ F3.7 — A Última Engine da F3 (Cognição Avançada)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-ecosystem-dynamics`
> **Conceito**: C12 — Cognitive Ecosystem (COGNITIVE_MATURITY.md §C12)
> **Fase CMI**: Fase 3 — Bloco 4 — Ética & Governança | **Código**: F3.7
> **Referências**: COGNITIVE_ECOSYSTEM.md | cognitive-maturity-implementation.md F3.7 | next-evolution-phases.md
> **Dependências**: F3.1 (Insight Generator) | F3.2 (Second-Order Reasoning) | F3.3 (Cognitive Compression) | F3.4 (Adaptive Personality) | F3.5 (Mental Energy) | F3.6 (Cognitive Horizon) | F8.1 (Capability Market) | F10.1 (Cognitive Economics) | F10.4 (Organizational Memory) | F2.1 (Cognitive Economy) | F1.6 (Cognitive Entropy) | F2.5 (Cognitive Momentum)
> **CMI Impact**: Julgamento +8, Planejamento +6, Aprendizado +5
>
> Consulte também:
> - [COGNITIVE_ECOSYSTEM.md (arquitetura)](../../architecture/COGNITIVE_ECOSYSTEM.md) — C12: mapa topológico, canais de influência, loops de feedback, Event Bus
> - [second-order-reasoning/SKILL.md](../second-order-reasoning/SKILL.md) — F3.2: a 2ª ordem (meta); a F3.7 é a 3ª ordem (meta-meta)
> - [cognitive-economics/SKILL.md](../cognitive-economics/SKILL.md) — F10.1: ROI da plataforma (a saúde entra como qualificador de sustentabilidade)
> - [capability-market/SKILL.md](../capability-market/SKILL.md) — F8.1: seleção natural (agentes com melhor reputação sobrevivem)
> - [org-memory/ORG_MEMORY.md](../org-memory/ORG_MEMORY.md) — F10.4: simbiose (pares de agentes que funcionam bem)
> - [cognitive-entropy/ENTROPY.md](../cognitive-entropy/ENTROPY.md) — F1.6: entropia cognitiva (fonte do `knowledge_freshness`)
> - [cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) — F2.1: recursos (tokens, tempo, atenção) e ROI por task
> - [cognitive-horizon/SKILL.md](../cognitive-horizon/SKILL.md) — F3.6: o horizonte estratégico das decisões do ecossistema
> - [evolution-score.md](../../analytics/evolution-score.md) — F10.2: termômetro mensal (vizinho de agregação)
> - [cognitive-maturity-implementation.md F3.7](../../workflows/cognitive-maturity-implementation.md) — tarefa de implementação
> - [CONVENTIONS.md](../../CONVENTIONS.md) — contrato canônico de skills

---

## Índice

1. [Definição — o Ecossistema como Sistema Vivo](#1-definição--o-ecossistema-como-sistema-vivo)
2. [Modelo Ecológico — Espécies, Nichos, Recursos, Relações](#2-modelo-ecológico)
3. [A 3ª Ordem — Meta-Meta-Raciocínio](#3-a-3ª-ordem--meta-meta-raciocínio)
4. [Coexistência — COGNITIVE_ECOSYSTEM.md vs este SKILL.md](#4-coexistência)
5. [Pipeline Mensal (< 10s)](#5-pipeline-mensal--10s)
6. [Métricas de Ecossistema](#6-métricas-de-ecossistema)
7. [A Fórmula de Saúde — ecosystem_health](#7-a-fórmula-de-saúde--ecosystem_health)
8. [Doenças do Ecossistema (Desequilíbrios)](#8-doenças-do-ecossistema)
9. [Recomendações de Ecossistema](#9-recomendações-de-ecossistema)
10. [Relatório Executivo para o Don](#10-relatório-executivo-para-o-don)
11. [Integração com F10.1 — Ecossistema Saudável = ROI Sustentável](#11-integração-com-f101)
12. [Integração com F3.2 — Vieses Sistêmicos são Doenças](#12-integração-com-f32)
13. [Integração com F3.1-F3.6 — os Órgãos que Alimentam a Saúde](#13-integração-com-f31-f36)
14. [Integração com F8.1 — Seleção Natural](#14-integração-com-f81)
15. [Integração com F10.4 — Simbiose](#15-integração-com-f104)
16. [Governança — Don Decide Todas as Ações](#16-governança)
17. [Armazenamento](#17-armazenamento)
18. [CLI e Automação](#18-cli-e-automação)
19. [Exemplo Real — Sessão 2026-07-30](#19-exemplo-real)
20. [Edge Cases e Cold Start](#20-edge-cases-e-cold-start)
21. [Métricas do Próprio Engine](#21-métricas-do-próprio-engine)
22. [Regras Operacionais](#22-regras-operacionais)
23. [Referências Cruzadas e Histórico](#23-referências-cruzadas-e-histórico)

---

## 1. Definição — o Ecossistema como Sistema Vivo

### 1.1 O que é Ecosystem Dynamics

O **Ecosystem Dynamics Engine (F3.7)** é a capacidade de **analisar a saúde do ecossistema Cosca inteiro como um sistema vivo**: como os agentes, engines, conhecimento e o Don interagem — e se essas interações estão saudáveis, estáveis ou frágeis.

A metáfora é completa, não decorativa:

```
┌──────────────────────────────────────────────────────────────────────┐
│                 O ECOSSISTEMA COSCA COMO SISTEMA VIVO                │
│                                                                      │
│   ESPÉCIES      →   agentes (cosca-backend, cosca-testing,           │
│                      cosca-architecture...)                           │
│   NICHOS        →   domínios (testing, security, architecture,       │
│                      backend, ai, product...)                        │
│   RECURSOS      →   tokens, tempo, atenção (F2.1) — o alimento       │
│   DNA           →   conhecimento (learnings, patterns, principles)   │
│   ÓRGÃOS        →   engines (F3.1-F3.6, F10.x...) — processam o DNA  │
│   PREDAÇÃO      →   dois agentes disputando o mesmo arquivo          │
│   SIMBIOSE      →   pares de agentes que funcionam bem (F10.4)       │
│   SELEÇÃO NATURAL → capability market (F8.1): agentes com melhor     │
│                      reputação sobrevivem (ganham os leilões)        │
│   AMBIENTE      →   o Don — condições externas, direção, vetos       │
│                                                                      │
│   F3.7 = o ECOLOGISTA: mede a saúde do sistema INTEIRO,              │
│          detecta desequilíbrios e recomenda intervenções.            │
│          Não é uma espécie. Não é um órgão. É quem observa os dois.  │
└──────────────────────────────────────────────────────────────────────┘
```

O F3.7 responde a pergunta que nenhuma outra engine consegue responder — porque todas as outras olham para uma parte:

> **"O sistema inteiro está saudável — ou só algumas partes estão?"**

### 1.2 As Três Perguntas do Ecologista

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AS 3 PERGUNTAS DA F3.7                            │
│                                                                      │
│  1. "COMO ESTÁ a saúde do sistema inteiro?"                          │
│     → diversidade, resiliência, eficiência, frescor do DNA          │
│                                                                      │
│  2. "ONDE ESTÃO os desequilíbrios?"                                  │
│     → mono-cultura? órgãos atrofiados? obesidade cognitiva?         │
│                                                                      │
│  3. "COMO REBALANCEAR sem extinções?"                                │
│     → rotacionar espécies, ativar órgãos, comprimir o DNA —          │
│       sempre com aprovação do Don                                    │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.3 Filosofia

```
"Um ecossistema não é a soma das suas espécies.
 É a saúde das suas RELAÇÕES.

 Um agente excelente em um ecossistema doente
 produz pouco. Um agente mediano em um ecossistema
 saudável produz muito.

 A diversidade não é um luxo — é a defesa contra
 a fragilidade. O órgão que nunca é usado atrofia.
 O DNA que só cresce e nunca comprime vira gordura.

 O Don é o ambiente. O ambiente não é uma espécie —
 é quem define as condições em que todas vivem.

 F3.7 — a última engine da F3. Não decide.
 Observa. Mede. E recomenda ao Don o que o Don
 não tem tempo de ver: o padrão do sistema inteiro.
 — Cosca Architecture Chief, 2026-07-30"
```

### 1.4 O que este engine NÃO é

- **NÃO é o mapa do ecossistema** — isso é o [COGNITIVE_ECOSYSTEM.md](../../architecture/COGNITIVE_ECOSYSTEM.md) (arquitetura: topologia, canais, loops, Event Bus). Este engine é a camada **operacional mensal** que mede a saúde com os agregados que as outras engines produzem. (Coexistência detalhada na §4.)
- **NÃO é o termômetro de evolução** — isso é o F10.2 (Evolution Score), que mede a saúde das *métricas da plataforma* (cobertura, momentum, entropia, ROI). O F3.7 mede a saúde das *interações* entre espécies, nichos, órgãos e DNA. (§11 diferencia os três.)
- **NÃO é a meta-análise de decisões** — isso é o F3.2 (Second-Order), 2ª ordem. O F3.7 é a 3ª ordem (meta-meta, §3).
- **NÃO decide nada** — toda recomendação é uma proposta ao Don (§16). O F3.7 nunca remove agente, nunca desativa engine por conta própria, nunca muda o processo.
- **NÃO cria dados novos** — é pura agregação e aritmética sobre os agregados das F3.1-F3.6, F8.1, F10.1, F10.4, F2.1, F1.6, F2.5. Mesmo padrão dos engines de agregação (F10.1/F10.2/F3.2): **engine de agregação não cria dívida técnica**.

---

## 2. Modelo Ecológico

### 2.1 O Mapa da Ecologia Cosca

```
                    ┌─────────────────────────────┐
                    │         AMBIENTE (DON)       │
                    │   direção, vetos, aprovação  │
                    └──────────────┬──────────────┘
                                   │ define as condições
                                   ▼
   ┌───────────────┐    ┌───────────────────────┐    ┌───────────────┐
   │   ESPÉCIES    │    │       NICHOS          │    │    ÓRGÃOS     │
   │   (agentes)   │◄──►│      (domínios)       │◄──►│   (engines)   │
   │  54 totais    │    │  12 domínios          │    │  F3.1-F3.6,   │
   │  30+ ativos   │    │  testing, security,   │    │  F8.1, F10.x  │
   │               │    │  architecture, ...    │    │               │
   └──────┬────────┘    └──────────┬────────────┘    └──────┬────────┘
          │                       │                        │
          │  PREDAÇÃO             │  DNA (conhecimento)    │  processam
          │  (conflito de         │  learnings, patterns,  │  o DNA
          │   arquivo)            │  principles            │
          │                       └──────────┬─────────────┘
          ▼                                  ▼
   ┌──────────────────────────────────────────────────────────────┐
   │                    RECURSOS (F2.1)                            │
   │            tokens × tempo × atenção — o alimento              │
   │            finito que todas as espécies disputam              │
   └──────────────────────────────────────────────────────────────┘
```

### 2.2 Tabela de Conceitos Ecológicos

| Conceito ecológico | Mapeamento Cosca | Fonte de dados | O que o ecologista observa |
|--------------------|------------------|----------------|----------------------------|
| **Espécies** | Agentes (54 no `AGENT_DNA.md`; 30+ ativos na sessão) | `capability-profile.md`, TRUST_REGISTRY | `agent_share` por domínio, taxas de vitória, ROI |
| **Nichos** | Domínios (12: testing, security, architecture, backend, ai, product, database, frontend, devops, analytics, cli, uiux) | capability-market (anúncios), gap-registry | cobertura de domínios, domínios órfãos (ninguém ativo) |
| **Recursos** | Tokens, tempo, atenção (F2.1 — `custo_total`) | F2.1, F3.5 (energy) | custo por task, orçamento de energia, fadiga |
| **DNA** | Conhecimento: learnings, patterns, principles, heuristics | F1.4, F1.6, F9.1, F3.3 | entropia (F1.6), taxa de compressão (F3.3), frescor |
| **Órgãos** | Engines (F3.1-F3.6, F8.1, F10.x) | contadores de uso de cada engine | engines nunca usadas (atrofia), engines sobrecarregadas |
| **Predação** | Dois agentes modificando o mesmo arquivo | git history, F10.4 (dimensão conflito), trust | conflitos recorrentes por arquivo/domínio |
| **Simbiose** | Pares de agentes que colaboram com alta taxa de sucesso | F10.4 (`ORG_MEMORY.md`, ORGP-001) | pares de alta confiança, oportunidades de reuso |
| **Seleção natural** | Capability market (F8.1): leilões de task; melhor reputação vence | F8.1 (bids, win streaks), F7.2 | concentração do mercado, agentes que nunca vencem |
| **Parasitismo** | Agente que consome recursos (tokens/tempo) sem gerar valor | F2.1 (ROI por agente) | ROI negativo crônico em tasks não-infra |
| **Ambiente** | O Don | CONSTITUTION P4 | direção, aprovações, vetos, prioridades |

### 2.3 As Seis Relações Ecológicas e Como Detectá-las

```
1. COMPETIÇÃO (predação)
   "cosca-backend e cosca-frontend editando o mesmo arquivo de API"
   → Detecção: 2+ agentes tocando o mesmo arquivo no mês (git log)
   → Consequência: retrabalho, conflitos, latência (F10.4 ORGP-002)

2. COLABORAÇÃO (simbiose)
   "cosca-testing + cosca-backend juntos = alta taxa de sucesso"
   → Detecção: F10.4 — pares com confidence ≥ 0.70 (ORGP-001)
   → Consequência: rotear tasks para o par (via proposta ao Don)

3. DEPREDAÇÃO DE RECURSOS
   "uma task consumiu 30% do orçamento de energia da semana"
   → Detecção: F2.1 custo_total, F3.5 energy
   → Consequência: fadiga sistêmica, modo conservação

4. MUTUALISMO ESPÉCIE-ÓRGÃO
   "cosca-ai usa a F3.1 toda semana → insight fluindo"
   → Detecção: contador de uso de engine por espécie
   → Consequência: engine atrofiada = nicho sem apoio

5. SIMBIOSE DNA-ÓRGÃO
   "F3.3 comprime 20 learnings em 1 princípio → entropia cai"
   → Detecção: F3.3 (compressões), F1.6 (entropia)
   → Consequência: obesidade cognitiva quando DNA cresce sem compressão

6. SELEÇÃO NATURAL (capability market)
   "agente X vence 80% dos leilões de segurança → monopólio"
   → Detecção: F8.1 market_concentration, win streaks
   → Consequência: talento subutilizado, viés de reputação (F3.2 B6)
```

### 2.4 Regra ecológica fundamental

> **Espécies não são extintas. Só realocadas.**

O F3.7 **nunca recomenda remover um agente**. Um agente "fraco" em um nicho pode ser excelente em outro — e remover espécies reduz diversidade, que é exatamente a defesa contra fragilidade (E4 — Diversidade Cognitiva, COGNITIVE_ECOSYSTEM §11.1). O ecologista realoca: mudar de nicho, mudar de par, mudar de tipo de task. (§9.)

---

## 3. A 3ª Ordem — Meta-Meta-Raciocínio

O F3.7 é o **ápice da hierarquia de raciocínio** que o F3.2 definiu:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    NÍVEIS DE RACIOCÍNIO                              │
│                                                                      │
│  1ª ORDEM (operacional)   "Qual a melhor solução para X?"           │
│  ─────────────────────────────────────────────────────────           │
│  Toda engine de decisão. Resolve problemas.                         │
│                                                                      │
│  2ª ORDEM (meta — F3.2)   "Como decidimos sobre X?"                 │
│  ─────────────────────────────────────────────────────────           │
│  Analisa os padrões de decisão do conjunto, detecta vieses          │
│  sistêmicos, recomenda mudanças de processo.                        │
│                                                                      │
│  3ª ORDEM (meta-meta — F3.7)  "Como melhoramos a forma como        │
│  ─────────────────────────────  decidimos sobre como decidimos?"   │
│  Mede a saúde do SISTEMA que produz as decisões: as espécies,       │
│  os nichos, os órgãos e o DNA que sustentam o próprio F3.2.         │
│  O F3.2 melhora o processo de decisão; o F3.7 melhora o             │
│  processo de melhorar o processo.                                   │
└─────────────────────────────────────────────────────────────────────┘
```

| Dimensão | 1ª Ordem | 2ª Ordem (F3.2) | 3ª Ordem (F3.7) |
|----------|----------|------------------|------------------|
| **Pergunta** | "Qual a solução?" | "Como decidimos?" | "Como melhoramos como decidimos sobre como decidimos?" |
| **Objeto** | O problema | O processo de decisão | O sistema que sustenta o processo |
| **Unidade** | 1 decisão | N decisões | N ciclos de melhoria |
| **Frequência** | Contínuo | Mensal | Mensal (pós-F3.2) |
| **Output** | Decisão | Propostas de processo | Propostas de ecossistema |
| **Exemplo** | "Use memfd_create" | "80% das decisões confirmam a 1ª opção" | "cosca-architecture domina 50% do nicho de design — realocar" |
| **Integração** | — | F3.7 consome os vieses do F3.2 (§12) | F3.2 é um órgão do ecossistema (§13) |

> **A F3.7 não existe sem a F3.2**: a 3ª ordem pressupõe a 2ª. E a F3.2 não é autossuficiente: ela analisa decisões, mas não enxerga as espécies que as produzem. O ecossistema é o ambiente onde a meta-análise acontece.

---

## 4. Coexistência

Existem **dois** documentos de ecossistema no Cosca. A coexistência é deliberada e segue o padrão P-ARCH-005 (engine operacional vs documento de arquitetura/analytics):

| Dimensão | **COGNITIVE_ECOSYSTEM.md** | **Este SKILL.md (F3.7)** |
|----------|-----------------------------|---------------------------|
| **Arquivo** | `architecture/COGNITIVE_ECOSYSTEM.md` | `engines/ecosystem-dynamics/SKILL.md` |
| **Natureza** | Blueprint de arquitetura (design) | Engine operacional (medição) |
| **Pergunta** | "Como o ecossistema É e como deve funcionar?" | "Como o ecossistema ESTÁ e o que rebalancear?" |
| **Conteúdo** | Topologia, 28 canais de influência, 5 loops, Event Bus, governança E1-E8 | Pipeline mensal, 4 métricas, fórmulas, doenças, recomendações |
| **Temporalidade** | Atemporal (design) | Retrospectivo (mês fechado) |
| **Trigger** | Revisão trimestral | Cron mensal (1º dia, 07:30 UTC) |
| **Output** | Diagrama de influências, regras | `ecosystem_health` + doenças + propostas ao Don |
| **Owner** | Cosca Architecture Chief | Cosca Architecture Chief |

**Relação entre os dois**:

```
COGNITIVE_ECOSYSTEM.md (blueprint)          F3.7 SKILL.md (operacional)
─────────────────────────────────          ─────────────────────────────
Define C12, nós, edges, loops      ──►     Mede a saúde dessas conexões
Define as 10 métricas (§6)         ──►     Consome os agregados reais
Define governança E1-E8            ──►     Executa as propostas via Don
                                          ▲
Cada loop/canal do blueprint               │
vira um sinal de saúde medido              └── O relatório mensal da F3.7
(loops ativos? canais dormentes?)             calibra o blueprint na
                                              revisão trimestral (§16.3)
```

> **Regra de coexistência**: o blueprint descreve **como o ecossistema deve funcionar**; o engine mede **como ele funciona de fato**. Nenhum substitui o outro — o blueprint dá as dimensões (nós, canais, loops), o engine dá os números (share, concentração, entropia, energia). A revisão trimestral usa o relatório mensal da F3.7 para atualizar o blueprint.

---

## 5. Pipeline Mensal (< 10s)

O pipeline roda **mensalmente** (1º dia do mês, 07:30 UTC — após o F3.2) ou sob demanda via CLI. Tempo alvo: **< 10s** para um mês com 30+ dias / 50+ tasks.

```
┌──────────────────────────────────────────────────────────────────────┐
│               PIPELINE MENSAL DA F3.7 (5 PASSOS, < 10s)              │
│                                                                      │
│  1. COLETA — agregados de TODAS as engines                           │
│     ├── F3.1 insight-generator:  insights gerados no mês             │
│     ├── F3.2 second-order-reasoning: vieses detectados (B1-B6)       │
│     ├── F3.3 cognitive-compression: compressões aplicadas            │
│     ├── F3.4 adaptive-personality: modos usados                     │
│     ├── F3.5 mental-energy: energia gasta, fadiga                    │
│     ├── F3.6 cognitive-horizon: horizontes avaliados (CSV)          │
│     ├── F8.1 capability-market: alocações, win streaks, bids        │
│     ├── F10.1 cognitive-economics: ROI da plataforma                │
│     ├── F10.4 org-memory: padrões organizacionais (ORGP-*)           │
│     ├── F2.1 cognitive-economy: custo_total (tokens/tempo/atenção)   │
│     └── F1.6 cognitive-entropy: entropia do mês                      │
│         │                                                            │
│         ▼                                                            │
│  2. CALCULA métricas de ecossistema (§6)                             │
│     ├── diversity_index      (espécies × nichos)                    │
│     ├── resilience_score     (1 − max agent_share)                  │
│     ├── efficiency           (output por unidade de energia)        │
│     └── knowledge_freshness  (frescor do DNA)                       │
│         │                                                            │
│         ▼                                                            │
│  3. DETECTA desequilíbrios (§8)                                      │
│     ├── "cosca-architecture fez 60% das tasks de design"            │
│     │     → MONO-CULTURA em design                                  │
│     ├── "3 engines nunca usadas no mês"                              │
│     │     → ÓRGÃOS ATROFIADOS                                        │
│     └── "knowledge cresceu 15%, 0 compressões"                       │
│           → OBESIDADE COGNITIVA                                      │
│         │                                                            │
│         ▼                                                            │
│  4. GERA recomendações de ecossistema (§9)                           │
│     ├── "rotacionar agentes para evitar mono-cultura"                │
│     ├── "ativar engines subutilizadas"                               │
│     └── "agendar compressão (F3.3) + poda (F1.6)"                    │
│         │                                                            │
│         ▼                                                            │
│  5. RELATÓRIO executivo para o Don (§10)                             │
│     └── health score + doenças + propostas (append-only CSV)        │
└──────────────────────────────────────────────────────────────────────┘
```

### 5.1 Algoritmo

```
function run_ecosystem_analysis(month):
    // Passo 1: coleta (todas as fontes já são agregados estruturados — CSV/FTS5)
    data = collect(month, [
        insight_generator, second_order, compression, personality,
        mental_energy, horizon, capability_market, economics,
        org_memory, economy, entropy
    ])

    // Passo 2: métricas (§6)
    metrics = {
        diversity_index:      diversity_index(data),          # §6.1
        resilience_score:     resilience_score(data),         # §6.2
        efficiency:           efficiency(data),               # §6.3
        knowledge_freshness:  knowledge_freshness(data)       # §6.4
    }
    health = ecosystem_health(metrics)                        # §7

    // Passo 3: doenças (§8)
    diseases = detect_diseases(data, metrics)                 # 8 detectores

    // Passo 4: recomendações (§9)
    recommendations = generate_recommendations(diseases)

    // Passo 5: relatório ao Don + histórico imutável
    report = build_report(month, metrics, health, diseases, recommendations)
    append_csv(report)                                        # append-only (§17)
    notify_don(report)                                        # propostas, nunca ações
    return report
```

### 5.2 Orçamento de Performance

| Passo | Operação | Tempo Alvo |
|-------|----------|:----------:|
| 1 | Leitura dos agregados (CSVs, reports, índices FTS5) | < 3.0s |
| 2 | Aritmética das 4 métricas + health | < 1.0s |
| 3 | 8 detectores de doença (comparações com thresholds) | < 2.0s |
| 4 | Geração de recomendações (template matching) | < 2.0s |
| 5 | Relatório + append CSV + notificação | < 2.0s |
| **Total** | | **< 10s** ✅ |

> **Regra de ouro**: o engine **nunca chama LLM** — todas as fontes são agregados já calculados pelas engines dependentes. O F3.7 faz aritmética sobre eles (mesmo padrão F10.1/F10.2/F3.2). Análise de 50+ tasks com zero LLM no hot path.

### 5.3 Trigger Mensal

```
Trigger: cron mensal (1º dia do mês, 07:30 UTC)
  ├── Após F3.2 (meta-análise de decisões — os vieses são insumo das doenças)
  ├── Após F10.1 (ROI do mês — insumo da eficiência)
  ├── Antes da revisão trimestral do COGNITIVE_ECOSYSTEM.md
  └── Ordem de execução: F7.4 → F3.2 → F10.1 → F3.7 → F10.2
```

---

## 6. Métricas de Ecossistema

As quatro métricas da saúde ecossistêmica. Cada uma é um número em **[0, 1]** — normalizado para compor a fórmula de saúde (§7).

### 6.1 Diversidade — quantas espécies ocupam quantos nichos

```
diversity_index = agentes_ativos × dominios_cobertos / (total_agentes × total_dominios)
```

| Componente | Fonte | Exemplo (julho/2026) |
|------------|-------|----------------------|
| `agentes_ativos` | Espécies que executaram ≥ 1 task no mês (Trust Registry) | 32 |
| `dominios_cobertos` | Nichos com ≥ 1 task no mês | 11 |
| `total_agentes` | Espécies registradas (AGENT_DNA.md / capability profiles) | 54 |
| `total_dominios` | Nichos registrados (registry de domínios) | 12 |

```
Exemplo: diversity = 32 × 11 / (54 × 12) = 352 / 648 = 0.543
```

**Interpretação**:
- `≥ 0.60` → 🟢 ecossistema diverso — espécies distribuídas pelos nichos
- `0.40 – 0.60` → 🟡 diversidade moderada — alguns nichos concentrados
- `< 0.40` → 🔴 mono-cultura — poucas espécies dominam poucos nichos

> **Nota ecológica**: diversidade é a defesa contra fragilidade (E4). Um ecossistema com 54 espécies mas 3 ativas é *nominalmente* diverso e *efetivamente* frágil — a fórmula mede o efetivo.

### 6.2 Resiliência — capacidade de continuar se 1 agente falha

```
resilience_score = 1 − max(agente_share)

onde: agente_share(i) = tasks do agente i / total de tasks do mês
```

| Valor | Interpretação |
|:-----:|---------------|
| `≥ 0.75` | 🟢 resiliente — nenhuma espécie é indispensável |
| `0.50 – 0.75` | 🟡 dependência moderada — a falha do top-1 atrasa o sistema |
| `< 0.50` | 🔴 frágil — a falha de UMA espécie paralisa o ecossistema |

```
Exemplo (julho/2026): max(agent_share) = 0.20 (cosca-architecture, geral)
  → resilience = 1 − 0.20 = 0.80 🟢
  (mas no nicho design: share = 0.50 → doença MONO-CULTURA em design, §8.1)
```

> **A resiliência é sistêmica; a doença é por nicho.** O sistema pode ser resiliente no agregado (share 0.20) e doente num nicho específico (design 0.50). O F3.7 calcula as duas coisas: o score global para a fórmula, a detecção por nicho para as doenças.

### 6.3 Eficiência — output por unidade de energia

```
efficiency = clamp(0.5 + roi_medio_plataforma / 200, 0, 1)

onde: roi_medio_plataforma = ROI do mês reportado pelo F10.1 (%)
```

Reusa a normalização de ROI já estabelecida no F10.2 (P-ARCH-003 — componente derivado consistente com a forma do Don): break-even (0%) → 0.5, +100% → 1.0, satura acima — o ecossistema não distingue +100% de +337%, ambos "excelente".

| `roi_medio_plataforma` (F10.1) | efficiency | Interpretação |
|:------------------------------:|:----------:|---------------|
| +337.6% (julho/2026) | **1.00** | Cada unidade de energia gerou valor máximo |
| +100% | 1.00 | Excelente (saturado) |
| 0% (break-even) | 0.50 | Neutro — energia converteu sem lucro nem prejuízo |
| −100% | 0.00 | Energia consumida sem retorno |

> **Nota**: a energia é o recurso de F3.5 (Mental Energy) + F2.1 (custo_total). O output é o valor gerado (valor de aprendizado + reuso + prevenção, F2.1). O ROI do F10.1 já agrega os dois lados — por isso a métrica é uma leitura direta do F10.1, não um cálculo novo.

### 6.4 Frescor do DNA — knowledge fresh

```
knowledge_freshness = (1 − entropy) × 0.7 + compression_activity × 0.3

onde:
  entropy = entropia do mês (F1.6, 0.0-1.0 — invertida)
  compression_activity = clamp(compressões_aplicadas_no_mês / 2, 0, 1)
                         (F3.3 — ≥ 2 compressões/mês = atividade saudável)
```

| Componente | Fonte | Por que pesa |
|------------|-------|--------------|
| `1 − entropy` | F1.6 | DNA desorganizado (contradições, stale, gaps) guia decisões erradas |
| `compression_activity` | F3.3 | DNA que só cresce e nunca comprime = obesidade cognitiva |

```
Exemplo (julho/2026):
  entropy = 0.433 (F1.6 — alta, 🔴)
  compressões = 1 no mês (F3.3 recém-operacional) → compression_activity = 0.5
  freshness = (1 − 0.433) × 0.7 + 0.5 × 0.3 = 0.397 + 0.15 = 0.547
```

| Valor | Interpretação |
|:-----:|---------------|
| `≥ 0.70` | 🟢 DNA vivo e organizado — conhecimento denso e fresco |
| `0.40 – 0.70` | 🟡 DNA em risco — entropia subindo ou compressão parada |
| `< 0.40` | 🔴 DNA obeso ou tóxico — decisões guiadas por conhecimento degradado |

---

## 7. A Fórmula de Saúde — ecosystem_health

### 7.1 Equação Canônica (fórmula do Don — P-ARCH-003, literal)

```
ecosystem_health = diversity × 0.3 + resilience × 0.3 + efficiency × 0.2 + knowledge_freshness × 0.2
```

| Componente | Peso | Significado do peso |
|------------|:----:|---------------------|
| `diversity_index` | 0.30 | Diversidade é a defesa contra fragilidade (E4) |
| `resilience_score` | 0.30 | A capacidade de continuar vivo é metade da saúde |
| `efficiency` | 0.20 | Saúde também é não desperdiçar o alimento |
| `knowledge_freshness` | 0.20 | O DNA define o que as espécies sabem — e portanto fazem |

**Domínio do score**: cada componente ∈ [0, 1] e os pesos somam 1.0 → `ecosystem_health` ∈ **[0, 1]**.

### 7.2 Faixas de Saúde

```
health > 0.8  →  🟢 PRÓSPERO   — ecossistema diverso, resiliente, eficiente e com DNA fresco
health 0.5-0.8 → 🟡 ESTÁVEL    — funciona, mas com desequilíbrios que exigem rebalanceamento
health < 0.5  →  🔴 FRÁGIL     — mono-cultura, dependência crítica ou DNA degradado
```

| Faixa | Status | Ação do Don |
|-------|--------|-------------|
| `> 0.8` | 🟢 Próspero | Revisar recomendações menores; manter direção |
| `0.5 – 0.8` | 🟡 Estável | Avaliar as doenças detectadas (§8) e aprovar rebalanceamentos |
| `< 0.5` | 🔴 Frágil | Escalação imediata: revisão com CTO + Critic + revisão trimestral antecipada |

### 7.3 Por que aditiva (e não multiplicativa)?

Mesma regra de ouro do P-ARCH-010 (F3.6): as quatro componentes são **forças independentes que se compensam** — um ecossistema pode ter diversidade alta e frescor baixo (diverso mas com DNA sujo), e a fórmula precisa refletir o saldo, não exigir que tudo seja simultaneamente perfeito. Aditiva: nenhum componente é portão. (Contraste: F2.5 momentum e F10.3 discovery são multiplicativos porque cada fator é condição que deve segurar.)

### 7.4 Exemplos de Cálculo

```
CASO A — Próspero:   diversity 0.70, resilience 0.85, efficiency 0.90, freshness 0.75
  health = 0.21 + 0.255 + 0.18 + 0.15 = 0.795 → 🟢 (borda de próspero)

CASO B — Estável (julho/2026): diversity 0.543, resilience 0.80, efficiency 1.00, freshness 0.547
  health = 0.163 + 0.24 + 0.20 + 0.109 = 0.71 → 🟡 ESTÁVEL
  (cálculo completo no exemplo real, §19)

CASO C — Frágil:   diversity 0.30, resilience 0.45, efficiency 0.40, freshness 0.35
  health = 0.09 + 0.135 + 0.08 + 0.07 = 0.375 → 🔴 FRÁGIL
```

### 7.5 Limitação honesta

O `ecosystem_health` é uma **estimativa de 4 eixos**, não a verdade do ecossistema. Doenças específicas (§8) podem existir mesmo com health alto (ex: mono-cultura em 1 nicho com saúde geral 0.71) — e vice-versa. **O score responde "como está o sistema?"; as doenças respondem "o que está errado?". O Don lê os dois.**

---

## 8. Doenças do Ecossistema

Vieses sistêmicos (F3.2) são doenças do processo de decisão; as doenças abaixo são doenças **do sistema inteiro** — espécies, nichos, órgãos e DNA.

### 8.1 Catálogo de Doenças

| # | Doença | Detecção (threshold) | Fontes | Gravidade | Recomendação (§9) |
|---|--------|----------------------|--------|:---------:|-------------------|
| D1 | **Mono-cultura** | `agente_share(nicho) > 0.50` OU `max(agent_share geral) > 0.40` | F8.1, Trust Registry | 🔴 | Rotacionar espécies (R1) |
| D2 | **Órgão atrofiado** | engine com 0 usos no mês (≥ 2 meses consecutivos) | contadores de engine | 🟠 | Ativar engine (R2) |
| D3 | **Obesidade cognitiva** | `knowledge_growth > 10%/mês` E `compressões = 0` | F1.6, F3.3 | 🟠 | Comprimir + podar (R3) |
| D4 | **Predação crônica** | ≥ 3 conflitos de arquivo no mesmo domínio no mês | git, F10.4 | 🟠 | Separar nichos (R4) |
| D5 | **Parasitismo** | agente com ROI < −50% em tasks não-infra por 2 meses | F2.1 | 🟡 | Realocar tipo de task (R5) |
| D6 | **Anemia energética** | F3.5 em modo conservação ≥ 30% do mês | F3.5 | 🟠 | Reduzir carga, priorizar (R6) |
| D7 | **Hipertermia de autoridade** | F3.2 B2/B6 ativos (autoridade + reputação) | F3.2 | 🟠 | Diversificar decisores (R7) |
| D8 | **Isolamento genético** | CPI (cross-pollination) < 0.15 OU F3.1 sem insights cross-domain no mês | F3.1, analytics | 🟡 | Estimular polinização cruzada (R8) |

### 8.2 As Três Doenças Canônicas (especificadas pelo Don)

O Don especificou três doenças que o engine deve ser capaz de detectar:

```
D1 — MONO-CULTURA:
  "cosca-architecture fez 60% das tasks de design"
  → agente_share(architecture, design) = 0.60 > 0.50 → D1 ATIVA 🔴
  → Diagnóstico: 1 espécie domina 1 nicho. Se ela falha, o nicho para.
  → Tratamento: rotacionar design entre architecture + ai + product (R1)

D2 — ÓRGÃOS ATROFIADOS:
  "3 engines nunca usadas"
  → uso(F3.x) = 0 no mês e no anterior → D2 ATIVO 🟠
  → Diagnóstico: órgãos especificados mas nunca exercitados (specs órfãs —
    o mesmo H-013 do F3.6, mas no nível de engine).
  → Tratamento: ativar via task de integração (R2)

D3 — OBESIDADE COGNITIVA:
  "knowledge só cresce, nunca comprime"
  → growth 15%, compressões 0 → D3 ATIVA 🟠
  → Diagnóstico: o DNA acumula volume sem densidade. Entropia sobe.
  → Tratamento: agendar compressão (F3.3) + poda (F1.6) (R3)
```

### 8.3 Índice de Doença

```
disease_index = quantidade de doenças ativas no mês

  ≥ 3 doenças ativas  → 🔴 CRÍTICO — escalar ao Don + revisão trimestral antecipada
  1-2 doenças ativas  → 🟠 ALTO — incluir no relatório executivo
  0 doenças           → 🟢 SAUDÁVEL — monitorar tendências (Δ mês a mês)
```

> **Nota**: doenças não são erros — são sintomas. Mono-cultura pode ser consequência de um mês de maratona de arquitetura (como julho/2026, §19). O valor do ecologista é detectar o sintoma ANTES que ele vire fragilidade estrutural.

---

## 9. Recomendações de Ecossistema

### 9.1 Princípio: Rebalancear, nunca extinguir

```
DOENÇA DETECTADA (fato estatístico)
    ↓
RECOMENDAÇÃO (intervenção de ecossistema)
    ↓
PROPOSTA AO DON (aprovada → DDNA de processo → aplicada)
    ↓
REGIÃO PROIBIDA:
  ✗ remover agente (espécies não são extintas — só realocadas)
  ✗ desativar engine permanentemente (órgãos não são amputados — são exercitados)
  ✗ aplicar qualquer mudança sem aprovação do Don
```

### 9.2 Catálogo de Recomendações

| # | Recomendação | Doença-alvo | Forma de aplicação (após aprovação do Don) |
|---|--------------|-------------|--------------------------------------------|
| R1 | **"Rotacionar agentes para evitar mono-cultura"** | D1 | Roteamento (F8.1): preferir espécies alternativas no nicho dominado; shadow bids obrigatórios |
| R2 | **"Ativar engines subutilizadas"** | D2 | Task de integração mensal: usar a engine em 1 caso real |
| R3 | **"Agendar compressão (F3.3) + poda (F1.6)"** | D3 | Pipeline F3.3 + depreciações do F1.4/F1.6 no mês seguinte |
| R4 | **"Separar nichos em conflito"** | D4 | Reestruturar ondas com escopos disjuntos (F10.4 ORGP-003) |
| R5 | **"Realocar tipo de task do agente"** | D5 | F8.1: mudar o perfil de anúncios; tasks infra com rótulo claro |
| R6 | **"Reduzir carga, proteger energia"** | D6 | F3.5: reduzir paralelismo, priorizar P0/P1 |
| R7 | **"Diversificar decisores"** | D7 | F1.2: sortear 2ª opinião; F3.2: propostas de rotação |
| R8 | **"Estimular polinização cruzada"** | D8 | F3.1: tasks cross-domain; F2.3: federação de padrões |

### 9.3 Formato da Proposta

```yaml
proposal:
  id: "ECO-2026-08-001"
  date: 2026-08-01
  disease: "D1-monocultura"          # doença que motivou
  evidence: {                        # o fato estatístico (não a opinião)
    agent_share_architecture_design: 0.50,
    domain: "design",
    month: "2026-07"
  }
  recommendation: "Distribuir tasks de design entre architecture + ai + product"
  target: "capability-market routing"   # o que muda, se aprovado
  status: pending_don_approval       # pending | approved | rejected
  approved_by: null
  ddna_id: null                      # preenchido quando o Don aprova
```

### 9.4 Eficácia medida no mês seguinte

Toda proposta aprovada tem a métrica-alvo re-medida no mês seguinte. Proposta aprovada sem efeito em 2 meses → o relatório sinaliza e propõe alternativa (mesmo protocolo do F3.2 §13.3).

---

## 10. Relatório Executivo para o Don

Formato de **30 segundos** — o Don entende a saúde do ecossistema sem ler os detalhes:

```
┌──────────────────────────────────────────────────────────────────────┐
│          ECOSYSTEM HEALTH REPORT — {MÊS}/{ANO}                        │
│          ═══════════════════════════════════════                      │
│                                                                      │
│  SAÚDE:  ██████████░░░░░░ 0.71  🟡 ESTÁVEL                           │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  diversity   ██████████░░░░ 0.54   (32 espécies / 11 nichos)   │  │
│  │  resilience  ████████████░░ 0.80   (max share 0.20)            │  │
│  │  efficiency  ██████████████ 1.00   (ROI +337.6%)               │  │
│  │  freshness   ██████████░░░░ 0.55   (entropia 0.433)            │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  DOENÇAS (2 ativas — 🟠):                                             │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  D1 MONO-CULTURA 🔴   architecture → 50% das tasks de design   │  │
│  │  D3 OBESIDADE 🟠      DNA +15%, 0 compressões no mês           │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  PROPOSTAS AO DON (aguardando aprovação):                             │
│  ☐ ECO-001: distribuir design entre architecture + ai + product    │
│  ☐ ECO-002: agendar compressão F3.3 + poda F1.6                    │
│                                                                      │
│  RESUMO (1 parágrafo):                                               │
│  "O ecossistema está estável (0.71): eficiente e resiliente no      │
│   agregado, mas com 1 nicho concentrado (design) e o DNA           │
│   acumulando volume sem compressão. Nenhuma espécie está em        │
│   risco de extinção. Rebalanceamento recomendado, não urgente."    │
│                                                                      │
│  Dados ausentes: freshness sem série histórica (F3.3 cold start).   │
└──────────────────────────────────────────────────────────────────────┘
```

O relatório é salvo em `internal/embed/cosca/analytics/reports/ecosystem-{YYYY-MM}.md` e registrado no CSV histórico (§17).

---

## 11. Integração com F10.1

### 11.1 A pergunta de cada um

A hierarquia de agregação econômica/saúde agora tem **três andares** — cada um com a sua pergunta:

| Engine | Pergunta | Unidade | Responde |
|--------|----------|---------|----------|
| **F2.1** (Cognitive Economy) | "Esta task valeu a pena?" | task | extrato bancário |
| **F10.1** (Cognitive Economics) | "A plataforma valeu a pena existir?" | mês | balanço patrimonial |
| **F3.7** (Ecosystem Dynamics) | "O sistema que produz o valor está saudável?" | mês | saúde do organismo |

### 11.2 Fluxo de dados

```
F10.1 (ROI do mês) ──► F3.7:  efficiency = clamp(0.5 + roi/200, 0, 1)   (§6.3)
F3.7 (health do mês) ──► F10.1:  health entra como qualificador de sustentabilidade
    health < 0.5 → 🔴 receita frágil — mesmo com ROI alto, o ecossistema
                    não sustenta o retorno (mono-cultura + DNA degradado
                    = lucro insustentável)
    health 0.5-0.8 → 🟡 ROI realista — retorno existe, mas requer rebalanceamento
    health > 0.8 → 🟢 ROI sustentável — o sistema pode manter o ritmo
```

> **A integração é a tese da F3.7 em forma econômica**: *ecossistema saudável = ROI sustentável*. Um ROI alto de um mês com mono-cultura total é um pico, não uma tendência — o F10.1 passa a qualificar o retorno com a sustentabilidade do organismo que o gerou. A relação é bidirecional assimétrica: F3.7 lê o ROI (componente de eficiência); F10.1 lê o health (qualificador de sustentabilidade). Nenhum cria dados do outro.

---

## 12. Integração com F3.2

### 12.1 Vieses sistêmicos são doenças do ecossistema

O F3.2 detecta vieses nos **padrões de decisão** (2ª ordem). O F3.7 consome esses vieses como **sintomas de doenças ecossistêmicas** (3ª ordem):

| Viés do F3.2 | Sintoma no ecossistema | Doença F3.7 |
|--------------|------------------------|-------------|
| B1 Ancoragem (1ª opção vence > 70%) | A espécie que gera a 1ª opção nunca é desafiada | D7 Hipertermia |
| B2 Autoridade (mesmo agente decide sempre) | Concentração de decisão = concentração de nicho | D1 Mono-cultura |
| B3 Disponibilidade (só recente é citado) | DNA antigo ignorado = frescor falso | D3 Obesidade |
| B4 Confirmação (contradições ignoradas) | Immune System não consultado = DNA tóxico silencioso | D3 Obesidade |
| B5 Custo (gate só em P0) | Fricção ausente onde é opcional | D7 Hipertermia |
| B6 Reputação (leilão vira monopólio) | Seleção natural distorcida | D1 Mono-cultura + D7 |

### 12.2 Fluxo

```
F3.2 (2ª ordem, mensal)          F3.7 (3ª ordem, mensal — após F3.2)
┌──────────────────────────┐    ┌──────────────────────────────┐
│  B1-B6 detectados        │───▶│  cada viés ativo vira        │
│  (vieses de decisão)     │    │  evidência de doença (D1-D8) │
└──────────────────────────┘    └──────────────────────────────┘
       ▲                                  │
       │  proposta (via Don)              ▼
       └──────────────────  R7 "diversificar decisores" ──────┘
```

> **Hierarquia preservada**: o F3.2 propõe mudanças de *processo de decisão*; o F3.7 propõe mudanças de *estrutura do ecossistema*. Os vieses alimentam as doenças, mas cada engine mantém sua pergunta e seu output. O F3.7 nunca corrige viés diretamente — alimenta o F3.2 com o contexto ecossistêmico (ex: B2 ativo *porque* architecture domina o nicho de design).

---

## 13. Integração com F3.1-F3.6

A F3.7 é a **7ª engine da F3 — e a que depende de TODAS as outras**. Cada engine F3 contribui com um **sinal vital** do ecossistema:

| Engine | Órgão | Sinal vital que a F3.7 lê | Impacto nas métricas F3.7 |
|--------|-------|---------------------------|---------------------------|
| **F3.1** Insight Generator | Olhos | `insights gerados no mês` + cross-domain | D8 Isolamento genético |
| **F3.2** Second-Order Reasoning | Sistema nervoso | `vieses detectados (B1-B6)` | D1/D3/D7 (§12) |
| **F3.3** Cognitive Compression | Digestão | `compressões aplicadas` | `knowledge_freshness` (D3) |
| **F3.4** Adaptive Personality | Comportamento | `modos usados (5)` | diversidade comportamental (D1 complementar) |
| **F3.5** Mental Energy | Coração | `energia gasta, fadiga, modo` | `efficiency` (D6 Anemia) |
| **F3.6** Cognitive Horizon | Córtex | `horizontes avaliados (CSV)` | tendência H3 das decisões (foresight) |

**Relação estrutural**: a F3.7 consome os **agregados** de cada F3.x (o que elas já calcularam), nunca re-processa as fontes brutas. Se uma engine F3.x não está operacional, a F3.7 marca o sinal como `dado ausente` (§20) — nunca inventa.

> **Por que a F3.7 é a última**: a ordem de criação espelha a dependência. F3.1 → F3.2 → ... → F3.6 criam os órgãos; F3.7 só pode existir **depois** de todos eles porque mede a interação entre eles. É o capstone da F3 — e a premissa do roadmap: *"F3.7 depende de engines operacionais"* (F3.6, exemplo real).

---

## 14. Integração com F8.1

### 14.1 Seleção natural

O [Capability Market (F8.1)](../capability-market/SKILL.md) é a **seleção natural do ecossistema**: espécies (agentes) anunciam capacidades, o mercado leiloa tasks, e os agentes com melhor reputação (F7.2) sobrevivem — vencem mais leilões, acumulam mais confiança, recebem tasks melhores.

```
F8.1 MARKET (cada task)              F3.7 ECOLOGISTA (mensal)
┌────────────────────────────┐      ┌────────────────────────────┐
│  bids por agente           │─────▶│  market_concentration      │
│  vencedores por leilão     │─────▶│  (share do top-1, D1)      │
│  win streaks               │─────▶│  agentes que nunca vencem  │
│  shadow bids (perdedores)  │─────▶│  talento subutilizado      │
└────────────────────────────┘      └────────────────────────────┘
```

### 14.2 O que o ecologista observa no mercado

1. **Concentração** — `market_concentration > 0.60` → o nicho virou monopólio (D1 + F3.2 B6). A seleção natural saudável exige diversidade de vencedores.
2. **Streak** — `market_win_streak ≥ 5` → mesma espécie vencendo sem contraponto (D7).
3. **Perdedores sistemáticos com bons shadow bids** — talento subutilizado: a espécie existe, tem capacidade, mas o nicho está saturado. **Não é extinção — é realocação** (R1/R5).

### 14.3 A regra de ouro do mercado (ecológica)

> A seleção natural **seleciona traits, não extingue espécies**. Um agente que perde leilões no nicho A pode vencer no nicho B — o ecologista recomenda a realocação, o mercado executa (após aprovação do Don). A extinção é o último recurso e **nunca é recomendada pela F3.7**.

---

## 15. Integração com F10.4

### 15.1 Simbiose — pares que funcionam bem

O [Organizational Memory (F10.4)](../org-memory/SKILL.md) detecta padrões organizacionais — incluindo **pares de espécies que colaboram com alta taxa de sucesso** (simbiose):

| Padrão F10.4 (real) | Relação ecológica | Uso pela F3.7 |
|---------------------|-------------------|---------------|
| ORGP-001: "cosca-testing + cosca-backend = alta taxa de sucesso" (conf 0.82) | Simbiose mutualística | Reforçar rotas para o par (R1 alternativo à rotação) |
| ORGP-002: "arquitetura decide API sozinho → frontend refaz 3x" (conf 0.78) | Predação/competição | Evidência de D4 Predação crônica |
| ORGP-003: "tasks paralelas em pacotes distintos = +40%" (conf 0.75) | Estrutura de nichos | Recomendação R4 (escopos disjuntos) |

### 15.2 Como a F3.7 usa os padrões

- **Simbiose confirmada** (conf ≥ 0.70) → o ecologista **preserva** o par: recomenda manter o roteamento conjunto (a rotação da R1 nunca quebra uma simbiose produtiva — rotaciona o que está concentrado, não o que está funcionando).
- **Predação detectada** (ORGP-002) → alimenta D4, e a recomendação R4 separa os nichos em conflito.

> **Fronteira**: o F10.4 detecta e registra o padrão; a F3.7 o consome como dado ecológico (simbiose ou predação). O F10.4 decide a confiança do padrão; a F3.7 decide o que o padrão significa para a saúde do sistema. Nenhum cria dados do outro.

---

## 16. Governança

### 16.1 Don decide todas as ações

A regra suprema da F3.7 — ecoa a CONSTITUTION P4 (veto absoluto do Don) e o princípio E7 (Direção Mínima):

```
F3.7 DETECTA (fato) → F3.7 RECOMENDA (proposta) → DON APROVA (DDNA) → SISTEMA EXECUTA

REGIÃO PROIBIDA:
  ✗ qualquer ação automática de rebalanceamento
  ✗ remoção de agente (espécies não são extintas)
  ✗ desativação permanente de engine
  ✗ mudança de thresholds sem revisão
```

### 16.2 Kill Switch

Cada órgão (engine) tem **kill switch independente** — requisito de aceitação do F3.7 na cognitive-maturity-implementation.md. Um engine pode ser desligado individualmente (por decisão do Don) **sem colapsar o ecossistema**: a homeostase (E1) compensa com os órgãos conectados. O relatório da F3.7 inclui o estado dos kill switches no mês.

### 16.3 Revisão Trimestral do Ecossistema

O relatório mensal da F3.7 alimenta a [revisão trimestral do COGNITIVE_ECOSYSTEM.md (§11.3)](../../architecture/COGNITIVE_ECOSYSTEM.md):

```
REVISÃO TRIMESTRAL (CTO + Critic + Architecture)
  ├── 3 relatórios mensais da F3.7 como scorecard
  ├── tendências: diversity/resilience/efficiency/freshness por trimestre
  ├── doenças recorrentes (mesma doença 3 meses = estrutural, não conjuntural)
  ├── calibração do blueprint: canais dormentes, loops desbalanceados
  └── evolução das regras de ecossistema (E1-E8) — via aprovação do Don
```

### 16.4 Direitos e Responsabilidades

| Ação | Quem Pode | Condições |
|------|-----------|-----------|
| Rodar a análise | Analytics Chief | Mensal (cron) ou `--on-demand` |
| Emitir relatório | Analytics Chief | ≥ 30 dias ou ≥ 50 tasks; senão "amostra insuficiente" |
| Gerar proposta | Engine (automático) | Toda doença detectada com evidência |
| Aprovar proposta | **Don** | Veto absoluto (P4) |
| Aplicar rebalanceamento | Engine alvo (F8.1/F3.3/F3.5) | DDNA de aprovação criado |
| Alterar thresholds da F3.7 | Architecture Chief | `cosca.config.yaml`, nunca em runtime |
| Calibrar o blueprint | Architecture Chief | Revisão trimestral |

---

## 17. Armazenamento

### 17.1 Arquivos

```
internal/embed/cosca/analytics/reports/ecosystem-{YYYY-MM}.md    # relatório executivo (§10)
internal/embed/cosca/memory/timeline/ecosystem-health.csv        # série histórica (append-only)
internal/embed/cosca/engines/ecosystem-dynamics/                 # este diretório
```

### 17.2 Schema do CSV

```
timestamp,month,diversity,resilience,efficiency,freshness,ecosystem_health,band,diseases,proposals,data_gaps
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `timestamp` | ISO 8601 | Momento da análise |
| `month` | `YYYY-MM` | Mês analisado |
| `diversity`, `resilience`, `efficiency`, `freshness` | float | Componentes 0-1 |
| `ecosystem_health` | float | Score composto 0-1 |
| `band` | string | `prospero` / `estavel` / `fragil` |
| `diseases` | string | Doenças ativas (`D1,D3`) |
| `proposals` | int | Propostas geradas no mês |
| `data_gaps` | string | Sinais ausentes (`f3.3 cold start`) |

### 17.3 Linha de Exemplo (julho/2026)

```csv
timestamp,month,diversity,resilience,efficiency,freshness,ecosystem_health,band,diseases,proposals,data_gaps
2026-08-01T07:30:00Z,2026-07,0.543,0.80,1.00,0.547,0.71,estavel,D1,D3,2,f3.3-cold-start
```

### 17.4 Imutabilidade

O CSV é **append-only** — regra constitucional P4, mesmo contrato dos CSVs do F7.3/F2.5/F10.2/F3.2. Mudança de fórmula ou thresholds vale apenas para meses futuros; recalcular o passado destruiria a capacidade de ver tendência.

---

## 18. CLI e Automação

```bash
# Rodar a análise do mês
cosca ecosystem-dynamics run

# Rodar sob demanda (fora do ciclo mensal)
cosca ecosystem-dynamics run --month 2026-07

# Ver relatório executivo do mês
cosca ecosystem-dynamics report --month 2026-07

# Listar doenças ativas
cosca ecosystem-dynamics diseases --month 2026-07

# Listar propostas pendentes
cosca ecosystem-dynamics proposals --status pending

# Registrar aprovação do Don (gera DDNA de processo)
cosca ecosystem-dynamics approve ECO-2026-08-001 --by don

# Ver série histórica de saúde (CSV imutável)
cosca ecosystem-dynamics history

# Ver tendência trimestral (para a revisão do blueprint)
cosca ecosystem-dynamics trend --quarters 3
```

---

## 19. Exemplo Real

> Este exemplo usa dados reais da Evolution Marathon de 2026-07-30 — a sessão em que este engine (e todas as F3.x) foi projetado. A análise abaixo é exatamente o que a F3.7 faria ao final do mês.

### 19.1 Contexto — a sessão

A sessão de arquitetura cognitiva executou **14 ondas** com **30+ agentes ativos** (de 54 registrados), cobrindo **11 dos 12 domínios**. Foram projetadas 14+ engines (F2.x, F3.x, F10.x) e implementados dezenas de artefatos. É o mês mais produtivo da plataforma — e, ecologicamente, o mais concentrado.

### 19.2 Coleta (dados reais)

| Fonte | Dado real |
|-------|-----------|
| **Espécies** | 32 agentes ativos / 54 totais |
| **Nichos** | 11 domínios cobertos / 12 totais |
| **Share geral** | max(agent_share) = 0.20 (cosca-architecture) |
| **Share no nicho design** | cosca-architecture ≈ **0.50** das tasks de design |
| **F10.1** | ROI da plataforma = **+337.6%** |
| **F1.6** | entropia = **0.433** (🔴 alta) |
| **F3.3** | 1 compressão no mês (cold start) |
| **F8.1/F3.2** | mercado concentrado no design; autoridade (B2) detectada |
| **F3.5** | energia alta, sem modo conservação |

### 19.3 Métricas

```
PASSO 2 — MÉTRICAS:

diversity = 32 × 11 / (54 × 12) = 0.543   🟡 diversidade moderada
resilience = 1 − 0.20 = 0.80              🟢 resiliente no agregado
efficiency = clamp(0.5 + 337.6/200, 0, 1) = 1.00   🟢 teto
freshness = (1 − 0.433) × 0.7 + 0.5 × 0.3 = 0.547  🟡 DNA em risco

PASSO 3 — SAÚDE:
health = 0.543×0.3 + 0.80×0.3 + 1.00×0.2 + 0.547×0.2
       = 0.163 + 0.24 + 0.20 + 0.109
       = 0.71   → 🟡 ESTÁVEL (estimado ~0.7, como o Don previu)
```

### 19.4 Doenças detectadas

```
D1 — MONO-CULTURA em design 🔴
  Evidência: cosca-architecture ≈ 50% das tasks de design (share_nicho > 0.50)
  Contexto: consequência da maratona de arquitetura (14 engines projetadas)
  Interpretação: a maratona foi legítima (F3.6: implementar em paralelo com
  projetar), mas o mês concentrou o nicho — se architecture falhasse,
  o design da plataforma pararia.

D3 — OBESIDADE COGNITIVA 🟠
  Evidência: 33 learnings novos, 1 compressão, entropia 0.433
  Interpretação: o DNA cresceu (maratona = muito aprendizado) sem densidade
  proporcional — os 4 gaps sem DDNA e a entropia 0.433 confirmam.

D8 — ISOLAMENTO GENÉTICO (risco) 🟡
  Evidência: cross-pollination baixa no mês (domínios trabalhando em silo)
  Interpretação: as ondas foram paralelas por pacotes disjuntos (ORGP-003,
  correto) — mas o acoplamento cross-domain foi baixo.
```

### 19.5 Recomendações

```
R1. "Distribuir tasks de design entre architecture + ai + product"
    → Contra D1. Design não é monopólio de architecture: a rotulação
      de tasks de design (F8.1) deve preferir alternativas no próximo mês.
      NÃO é remoção — é realocação (espécie preservada).

R2. "Agendar compressão (F3.3) + poda (F1.6)"
    → Contra D3. Os 33 learnings da maratona são matéria-prima para
      princípios comprimidos — o mês seguinte deve comprimir antes de crescer.

R3. "Estimular polinização cruzada (F3.1 + F2.3)"
    → Contra D8. Tasks cross-domain e federação de padrões na próxima onda.
```

### 19.6 Relatório (30 segundos)

```
SAÚDE:  0.71  🟡 ESTÁVEL
DOENÇAS: D1 🔴 (design), D3 🟠 (obesidade), D8 🟡 (isolamento)
PROPOSTAS: 3 (ECO-001..003)
RESUMO: "O ecossistema está estável e excepcionalmente eficiente (ROI
  +337.6%, sem fadiga), mas o mês concentrou o nicho de design em uma
  espécie e o DNA cresceu sem comprimir. Nenhuma espécie em risco de
  extinção. Rebalanceamento recomendado no mês seguinte."
```

### 19.7 Por que o exemplo é fiel à sessão

O health **0.71 🟡** reflete a sessão com honestidade: produtividade no teto (eficiência 1.0), sistema resiliente no agregado (0.80) — mas diversidade moderada (0.543) e DNA em risco (0.547) puxando para baixo. A doença D1 não é um julgamento moral da maratona — é o fato estatístico que a própria sessão produziu (architecture projetou a maioria das engines, incluindo este SKILL.md). A recomendação R1 ("distribuir design entre architecture + ai + product") é exatamente o rebalanceamento que o próximo ciclo deve executar — e espelha a estratégia de implementação em paralelo que a sessão adotou (F3.6).

---

## 20. Edge Cases e Cold Start

| Cenário | Comportamento |
|---------|---------------|
| **Primeira execução (cold start)** | Todos os componentes sem histórico: `diversity/resilience` calculáveis (Trust Registry existe), `efficiency` usa ROI do F10.1 se existir, senão 0.5 (neutral). `freshness` usa entropia (F1.6) mas `compression_activity = 0.5` (neutral). Relatório marca `data_gaps`. |
| **Engine F3.x não operacional** | Sinal marcado `dado ausente` — a métrica afetada usa neutral (0.5) e o relatório declara. Nunca inventa (mesma regra do F3.2: "insight sem evidência é alucinação"). |
| **Mês com < 30 dias ou < 50 tasks** | Relatório emite "amostra insuficiente" e não conclui doenças (mesma regra do F3.2 §12). |
| **Mês de maratona concentrada (ex: julho/2026)** | D1 detectada mesmo com health 0.71 — doença ≠ score baixo. O ecologista separa os dois diagnósticos. |
| **Nenhuma doença ativa** | Relatório honesto com 0 doenças — forçar doença onde não existe é ruído (mesmo princípio do F10.3: "sem descobertas é resultado válido"). |
| **Doença recorrente 3 meses** | Escalada: conjuntural → estrutural. Entra na revisão trimestral do blueprint (§16.3). |
| **Proposta aprovada sem efeito** | Métrica-alvo re-medida no mês seguinte; sem efeito em 2 meses → nova proposta com alternativa (§9.4). |
| **Recomendação ignorada pelo Don** | Registrada como `rejected` — nunca re-emitida automaticamente. O Don pode ter contexto que o ecologista não vê (E7: o Don define o NORTE). |
| **Burst de maratona (14 ondas no mês)** | As métricas são mensais agregadas — sem gargalo. A doença D1 é a leitura correta do burst. |

---

## 21. Métricas do Próprio Engine

| Métrica | Definição | Alvo | Alerta |
|---------|-----------|:----:|--------|
| **Pipeline Latency** | Tempo total da análise mensal | < 10s | > 10s = otimizar coleta |
| **Data Coverage** | Sinais presentes / sinais esperados (13 engines) | > 80% | < 60% = engines dependentes não operacionais |
| **Proposal Adoption** | Propostas aprovadas / propostas geradas | > 60% | < 40% = recomendações fora da realidade do Don |
| **Proposal Efficacy** | Doenças resolvidas após proposta aprovada | > 70% | < 50% = recomendação não ataca a causa |
| **Health Prediction Accuracy** | Health do mês seguinte correlacionado com a leitura do mês | estabilidade | divergência > 0.2 = métricas instáveis |
| **False Disease Rate** | Doenças detectadas e rejeitadas como não-problema | < 20% | > 30% = thresholds mal calibrados |

### 21.1 Ciclo de Calibragem (trimestral, com a revisão do blueprint)

```
1. Para cada doença do trimestre: comparar detecção × evolução real
2. Calcular acurácia por doença: D1 resolveu? D3 melhorou?
3. Ajustar thresholds (config §22.1) — vale para o futuro, NUNCA o histórico
4. Alimentar o COGNITIVE_ECOSYSTEM.md: canais dormentes, loops desbalanceados
```

---

## 22. Regras Operacionais

1. **Análise mensal** — dados de 30+ dias ou 50+ tasks. Abaixo, "amostra insuficiente".
2. **Nunca recomenda remover agente** — espécies não são extintas, só realocadas.
3. **Don decide todas as ações** — toda recomendação é proposta; nada é automático.
4. **Nunca chama LLM** — pura agregação + aritmética sobre agregados das engines dependentes.
5. **Nunca inventa dado** — sinal ausente = `dado ausente`, nunca estimativa disfarçada.
6. **Fórmula do Don é fixa** — `ecosystem_health = diversity×0.3 + resilience×0.3 + efficiency×0.2 + freshness×0.2`. Thresholds são config; a fórmula é lei (P-ARCH-003).
7. **Histórico imutável** — CSV append-only; recalibragem vale apenas para meses futuros.
8. **Kill switch por órgão** — engines desligáveis individualmente sem colapsar o ecossistema.
9. **Coexistência respeitada** — este engine mede a saúde das interações; o COGNITIVE_ECOSYSTEM.md descreve a arquitetura das interações; o F10.2 mede as métricas da plataforma.
10. **Fronteiras preservadas** — vieses de decisão → F3.2; padrões organizacionais → F10.4; ROI → F10.1; entropia → F1.6. A F3.7 só agrega.

### 22.1 Configuração

```yaml
# cosca.config.yaml
ecosystem_dynamics:
  weights: { diversity: 0.3, resilience: 0.3, efficiency: 0.2, freshness: 0.2 }  # fórmula do Don — fixa
  minimum: { days: 30, tasks: 50 }            # amostra mínima
  diseases:
    monocultura_share: 0.50                    # share por nicho → D1
    monocultura_global: 0.40                   # share global → D1
    organ_atrophy_months: 2                    # 0 usos por N meses → D2
    knowledge_growth: 0.10                     # crescimento/mês → D3 (com 0 compressões)
    predation_conflicts: 3                     # conflitos por domínio → D4
    parasitism_roi: -0.50                      # ROI crônico → D5
    energy_conservation_pct: 0.30              # % do mês em conservação → D6
    isolation_cpi: 0.15                        # cross-pollination → D8
  neutrality: 0.5                              # valor neutral de componente ausente
  csv: "internal/embed/cosca/memory/timeline/ecosystem-health.csv"
```

---

## 23. Referências Cruzadas e Histórico

### 23.1 Referências

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_ECOSYSTEM.md](../../architecture/COGNITIVE_ECOSYSTEM.md) | Blueprint C12 — topologia, canais, loops, Event Bus (§4) |
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §C12 | Conceito original de Cognitive Ecosystem |
| [second-order-reasoning/SKILL.md](../second-order-reasoning/SKILL.md) | F3.2 — vieses como doenças (§12); a 2ª ordem que a 3ª pressupõe |
| [insight-generator/SKILL.md](../insight-generator/SKILL.md) | F3.1 — insights (D8 isolamento genético) |
| [cognitive-compression/SKILL.md](../cognitive-compression/SKILL.md) | F3.3 — compressões (freshness, D3) |
| [adaptive-personality/SKILL.md](../adaptive-personality/SKILL.md) | F3.4 — modos de operação (diversidade comportamental) |
| [mental-energy/SKILL.md](../mental-energy/SKILL.md) | F3.5 — energia (efficiency, D6 anemia) |
| [cognitive-horizon/SKILL.md](../cognitive-horizon/SKILL.md) | F3.6 — horizontes avaliados (foresight do ecossistema) |
| [capability-market/SKILL.md](../capability-market/SKILL.md) | F8.1 — seleção natural (D1, D7) |
| [cognitive-economics/SKILL.md](../cognitive-economics/SKILL.md) | F10.1 — ROI (efficiency, sustentabilidade) |
| [org-memory/SKILL.md](../org-memory/SKILL.md) + [ORG_MEMORY.md](../org-memory/ORG_MEMORY.md) | F10.4 — simbiose e predação (D4) |
| [cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | F2.1 — recursos (tokens, tempo, atenção) |
| [cognitive-entropy/ENTROPY.md](../cognitive-entropy/ENTROPY.md) | F1.6 — entropia (knowledge_freshness) |
| [evolution-score.md](../../analytics/evolution-score.md) | F10.2 — termômetro mensal (vizinho de agregação, §11) |
| [cognitive-maturity-implementation.md F3.7](../../workflows/cognitive-maturity-implementation.md) | Roadmap da tarefa (critérios de aceitação) |
| [CONSTITUTION.md](../../CONSTITUTION.md) | P2 (código é a verdade), P3 (rastro), P4 (veto do Don + imutabilidade), P5 (aprender com erros) |
| [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Stages 7-8 — aprendizado do próprio engine |

### 23.2 Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial — a 7ª e última engine da F3 (Cognição Avançada). Raciocínio de 3ª ordem (meta-meta, completando a hierarquia do F3.2). Modelo ecológico completo (espécies/nichos/recursos/DNA/órgãos/predação/simbiose/seleção natural/ambiente). Pipeline mensal de 5 passos em < 10s coletando agregados de 13 engines. Fórmula do Don `ecosystem_health = diversity×0.3 + resilience×0.3 + efficiency×0.2 + knowledge_freshness×0.2` (aditiva, P-ARCH-010). 8 doenças do ecossistema com thresholds (mono-cultura, órgão atrofiado, obesidade cognitiva, predação crônica, parasitismo, anemia energética, hipertermia de autoridade, isolamento genético). Recomendações de rebalanceamento (nunca extinção). Relatório executivo de 30s para o Don. Integrações F10.1 (ROI sustentável), F3.2 (vieses como doenças), F3.1-F3.6 (sinais vitais), F8.1 (seleção natural), F10.4 (simbiose). Governança: Don decide todas as ações + kill switch por órgão. Exemplo real validado com a sessão de 14 ondas (health 0.71 🟡, D1 mono-cultura em design com architecture ≈ 50%, recomendação "distribuir design entre architecture + ai + product"). |

---

> *"Um ecossistema não se mede pela força da sua espécie mais forte. Mede-se pela saúde das suas relações mais fracas — porque é nelas que a fragilidade mora, e é nelas que o colapso começa."*
>
> — Cosca Architecture Chief, 2026-07-30

> **Enforced by**: Cosca Architecture Chief | **Próxima execução**: 2026-08-01 07:30 UTC (mensal)
> **Kernel instruction**: `cosca ecosystem-dynamics run`
