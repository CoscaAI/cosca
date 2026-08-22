# SHADOW MODE ENGINE ★ F8.2 — Motor de Simulação de Decisões

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-shadow-mode`
> **Conceito**: ★ ESTRELA DA FASE 8 — Shadow Mode
> **Referências**: next-evolution-phases.md §F8.2 | KERNEL.md | DECISION_DNA.md | QUALITY_GATES.md
> **Dependências**: F8.1 (Capability Market) | F1.2 (Contrafactual Gate) | F7.1 (Prediction Engine) | F1.1 (DDNA)
> **CMI Impact**: Planejamento +7, Julgamento +6, Aprendizado +4

---

## Índice

1. [O que é o Shadow Mode](#1-o-que-é-o-shadow-mode)
2. [Pipeline do Shadow Mode](#2-pipeline-do-shadow-mode)
3. [Tipos de Shadow](#3-tipos-de-shadow)
4. [Integração com F8.1 — Capability Market](#4-integração-com-f81--capability-market)
5. [Integração com F1.2 — Contrafactual Gate](#5-integração-com-f12--contrafactual-gate)
6. [Integração com F7.1 — Prediction Engine](#6-integração-com-f71--prediction-engine)
7. [Integração com F1.1 — Decision DNA](#7-integração-com-f11--decision-dna)
8. [Custo do Shadow](#8-custo-do-shadow)
9. [Regras de Isolamento](#9-regras-de-isolamento)
10. [Exemplo: Remover Código Morto](#10-exemplo-remover-código-morto)
11. [Métricas e Monitoramento](#11-métricas-e-monitoramento)
12. [Relacionados](#12-relacionados)

---

## 1. O que é o Shadow Mode

### Definição

O **Shadow Mode** é o motor do Cosca que **executa uma decisão em ambiente simulado antes da execução real** — um "ensaio" completo antes de agir. Antes de alterar arquitetura, migrar banco de dados, refatorar módulos ou qualquer decisão P0, o Shadow Mode executa **Plano A vs Plano B** em ambiente controlado, coleta métricas comparativas, e só libera a execução real quando os dados confirmam a melhor escolha.

### Analogia: Ensaio Geral

```
Theater (Teatro)              →   Shadow Mode (Cosca)
──────────────────────────────────────────────────
Ensaio geral antes da estreia →   Execução simulada antes da real
Ator lê o script no ensaio   →   Plano é executado no shadow
Diretor assiste do fundo     →   Kernel observa resultados
Plateia vazia no ensaio      →   Shadow nunca afeta produção
Notas do diretor pós-ensaio  →   Relatório de shadow é gerado
Elenco principal vs cover    →   Plano A vs Plano B comparados
```

### Filosofia

```
"Ensaie antes de agir.
 Simule antes de executar.
 Compare antes de decidir."
 — Cosca Architecture Chief, 2026-07-30
```

### Por que Shadow Mode?

| Problema | Solução do Shadow Mode |
|----------|------------------------|
| Decisões arquiteturais são caras para reverter | Shadow executa sem custo de rollback — se falhar, ninguém sabe |
| Viés de confirmação: "Plano A é melhor porque eu pensei nele" | Shadow executa Plano A e Plano B e mostra dados objetivos |
| Riscos só aparecem durante a execução real | Shadow descobre riscos antes — risco zero para produção |
| Custo de oportunidade: "E se tivéssemos escolhido o outro?" | Shadow responde: "Aqui estão os dados comparativos" |
| Decisões precisam de evidência, não opinião | Shadow gera evidência empírica registrada no DDNA |

### O que o Shadow Mode NÃO é

- **Não é um ambiente de staging** — staging serve para testes de integração; shadow serve para simular decisões
- **Não é um fork do repositório** — git shadow é um dos tipos, mas não o único
- **Não substitui o Contrafactual Gate** — o Gate **ativa** o Shadow Mode; o Shadow Mode é a execução prática da simulação
- **Não substitui a Prediction Engine** — Prediction estima; Shadow executa e mede
- **Não é um CI/CD pipeline** — CI/CD roda após a decisão; Shadow roda antes

---

## 2. Pipeline do Shadow Mode

### Visão Geral

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        SHADOW MODE PIPELINE                                │
│                                                                           │
│  1. DECISÃO P0 CHEGA                                                       │
│     "Migrar SQLite para PostgreSQL"                                        │
│     │                                                                     │
│  2. SHADOW CRIA AMBIENTE ISOLADO                                           │
│     ├── Git shadow: branch clone + dry-run                                 │
│     ├── Prediction shadow: só Prediction Engine                            │
│     └── Full shadow: agentes reais em sandbox                              │
│     │                                                                     │
│  3. EXECUTA PLANO A NO SHADOW                                              │
│     ├── Sem afetar produção                                                │
│     ├── Métricas coletadas: tempo, custo, risco, qualidade                 │
│     └── Resultados registrados como "Plano A — Shadow"                     │
│     │                                                                     │
│  4. EXECUTA PLANO B NO SHADOW                                              │
│     ├── Mesmo ambiente isolado (ou paralelo)                              │
│     ├── Mesmas métricas                                                    │
│     └── Resultados registrados como "Plano B — Shadow"                     │
│     │                                                                     │
│  5. COMPARA RESULTADOS                                                     │
│     ├── Tempo: Plano A é 20% mais rápido                                  │
│     ├── Custo: Plano B é 30% mais barato                                  │
│     ├── Risco: Plano A tem 2 riscos conhecidos, Plano B tem 5             │
│     └── Qualidade: Plano A mantém cobertura, Plano B reduz 5%             │
│     │                                                                     │
│  6. GERA RELATÓRIO DE SHADOW                                               │
│     ├── Estruturado em YAML                                                │
│     ├── Linkado ao DDNA                                                    │
│     └── Enviado ao Kernel                                                  │
│     │                                                                     │
│  7. KERNEL DECIDE COM BASE NOS DADOS                                       │
│     ├── "Plano A vence — execute real"                                    │
│     ├── "Plano B vence — substituir plano original"                       │
│     ├── "Nenhum — refinar planos e re-simular"                            │
│     └── "Shadow inconclusivo — escalar para Don"                          │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### Passo a Passo Detalhado

#### Passo 1: Decisão P0 Chega

- **Executor**: Kernel
- **Ação**: Recebe decisão que passou pelo Contrafactual Gate (F1.2) com `gate.shadow_mode_required: true`
- **Gatilhos**:
  - Decisão P0: **sempre** ativa Shadow Mode
  - Decisão P1 com risco > 30% (confidence < 0.70): ativa Shadow Mode
  - Decisão multi-módulo com crossover > 3 módulos: ativa Shadow Mode
  - Don explicitamente requisitou shadow: ativa Shadow Mode
- **Output**: `ShadowDecision { decision_id, plan_a, plan_b, tipo_shadow, budget }`

#### Passo 2: Shadow Cria Ambiente Isolado

- **Executor**: Shadow Engine
- **Ação**: Baseado no tipo de shadow selecionado, cria o ambiente de simulação:

```
┌─────────────────┬──────────────────────────────────────┐
│ Tipo de Shadow  │ Ambiente Criado                      │
├─────────────────┼──────────────────────────────────────┤
│ Git shadow      │ git clone --depth=1 --branch=main    │
│                 │ + branch temporário shadow/<id>       │
│                 │ + CI dry-run config                   │
├─────────────────┼──────────────────────────────────────┤
│ Prediction      │ Prediction Engine consultada          │
│ shadow          │ (sem execução, apenas estimativas)   │
├─────────────────┼──────────────────────────────────────┤
│ Full shadow     │ Sandbox isolado (contêiner/VM)       │
│                 │ + agentes reais alocados via Market   │
│                 │ + rede isolada, sem acesso a produção │
└─────────────────┴──────────────────────────────────────┘
```

- **Regra**: O ambiente NUNCA pode tocar em produção. Zero conectividade com recursos produtivos.
- **Output**: `ShadowEnvironment { type, id, created_at, isolation_level, cleanup_strategy }`

#### Passo 3: Executa Plano A no Shadow

- **Executor**: Shadow Engine (pode delegar a agentes via Capability Market)
- **Ação**: Executa o plano proposto dentro do ambiente isolado
- **Métricas coletadas**:

| Métrica | Fonte | Formato |
|---------|-------|---------|
| Tempo total | Runtime do shadow | `Xd Xh Xm Xs` |
| Custo (tokens) | Cognitive Economy | `$0.000` |
| Custo (tempo computacional) | Runtime | `Xd Xh Xm Xs` |
| Arquivos alterados | Git diff | `N` |
| LOC delta | Git diff-stats | `+N/-N` |
| Cobertura (se aplicável) | Test coverage | `N%` |
| Testes quebrados | Test runner | `N` |
| Riscos encontrados | Shadow audit | `[{desc, severity}]` |
| Qualidade percebida | Code review shadow | `0.0 - 1.0` |
- **Output**: `ShadowResult { plano: "A", metrics: {...}, artifacts: [...] }`

#### Passo 4: Executa Plano B no Shadow

- **Executor**: Shadow Engine
- **Ação**: Executa o plano alternativo no mesmo ambiente (resetado) ou em ambiente paralelo
- **Métricas**: Idênticas ao Passo 3
- **Output**: `ShadowResult { plano: "B", metrics: {...}, artifacts: [...] }`

#### Passo 5: Compara Resultados

- **Executor**: Shadow Engine
- **Ação**: Compara as métricas dos dois planos e gera matriz comparativa:

```
MATRIZ COMPARATIVA:
┌──────────────────┬──────────────┬──────────────┬────────────────────┐
│ Métrica          │ Plano A      │ Plano B      │ Diferença          │
├──────────────────┼──────────────┼──────────────┼────────────────────┤
│ Tempo            │ 2m30s        │ 3m45s        │ A é 33% mais rápido│
│ Custo (tokens)   │ $0.035       │ $0.028       │ B é 20% mais barato│
│ Arquivos afetados│ 12           │ 8            │ B afeta 33% menos  │
│ Cobertura        │ 95% → 94%   │ 95% → 96%    │ B ganha cobertura  │
│ Testes quebrados │ 0            │ 2            │ A não quebra testes │
│ Riscos           │ 2 (baixo)    │ 5 (1 médio)  │ A é mais seguro    │
│ Qualidade        │ 0.88         │ 0.82         │ A é 7% superior    │
│ Reversibilidade  │ Fácil        │ Difícil      │ B é mais arriscado │
└──────────────────┴──────────────┴──────────────┴────────────────────┘

RECOMENDAÇÃO: Plano A — 4 de 7 métricas superiores.
  Plano B ganha em custo e escopo, mas perde em qualidade e risco.
  Shadow recomenda: Plano A com observação de custo.
```

- **Peso das métricas** (configurável por tipo de decisão):

| Domínio da Decisão | Tempo | Custo | Qualidade | Risco | Reversibilidade |
|--------------------|:----:|:-----:|:---------:|:----:|:---------------:|
| Arquitetura        | 0.15 | 0.15  | 0.30      | 0.25 | 0.15            |
| Segurança          | 0.10 | 0.10  | 0.25      | 0.45 | 0.10            |
| Banco de dados     | 0.20 | 0.25  | 0.20      | 0.20 | 0.15            |
| Performance        | 0.30 | 0.20  | 0.20      | 0.20 | 0.10            |
| Refatoração        | 0.15 | 0.10  | 0.35      | 0.20 | 0.20            |

- **Output**: `ShadowComparison { winner, matrix, recommendation, confidence }`

#### Passo 6: Gera Relatório de Shadow

- **Executor**: Shadow Engine
- **Formato**: YAML estruturado (detalhado na Seção 8)
- **Conteúdo**:
  - Metadados da simulação (id, tipo, timestamp, duração)
  - Resultados do Plano A (métricas completas)
  - Resultados do Plano B (métricas completas)
  - Matriz comparativa com diferenças percentuais
  - Recomendação com rationale
  - Evidências coletadas (logs, diffs, relatórios de cobertura)
  - Riscos descobertos durante a simulação
- **Destino**:
  - `engines/shadow-mode/results/SHADOW-{decision_id}.yaml`
  - Linkado no DDNA da decisão como `evidence`
  - Publicado no Event Bus como `ShadowReportGenerated`
- **Output**: `ShadowReport { ... }`

#### Passo 7: Kernel Decide com Base nos Dados

- **Executor**: Kernel (ou Don, se escalado)
- **Ações**:

| Resultado do Shadow | Ação do Kernel |
|---------------------|----------------|
| Plano A vence claramente | Executa Plano A como plano real |
| Plano B vence claramente | Substitui plano original por B |
| Empate técnico (gap < 5%) | Kernel decide com base em critérios secundários |
| Shadow inconclusivo | Escalar para Don com relatório completo |
| Ambos os planos falham no shadow | Revisar planos, re-simular, ou abortar decisão |
| Shadow detectou risco crítico em ambos | Escalar para Don com alerta |

- **Regra**: Se o shadow apontar que Plano B é > 15% superior em 3+ métricas, o Kernel DEVE substituir o plano original. Se não o fizer, precisa registrar rationale de override.
- **Output**: `KernelDecision { decision, rationale, shadow_report_id }`

---

## 3. Tipos de Shadow

### 3.1 Git Shadow (Leve)

**Propósito**: Simular decisões que envolvem alteração de código-fonte, refatoração, migração de dependências, remoção de código morto.

**Como funciona**:

```
1. git clone --depth=1 --branch=main .shadow/workspace
2. cd .shadow/workspace && git checkout -b shadow/<decision_id>
3. Aplicar Plano A (commits automáticos)
4. Executar: testes, linter, cobertura, build
5. git diff --stat vs main
6. Registrar métricas
7. git reset --hard HEAD~N (ou branch novo)
8. Aplicar Plano B
9. Repetir passos 4-6
10. git branch -D shadow/<decision_id> (limpeza)
```

**Custo**: ~$0.005 – $0.015 (clone + CI leve)
**Tempo**: 30s – 5min
**Isolamento**: Total — repositório clone sem remote write
**Quando usar**: Decisões de refatoração, migração de dependências, remoção de código, mudanças de API

**Limitações**:
- Não executa agentes reais (apenas dry-run de comandos)
- Não mede qualidade subjetiva do design
- Requer clone do repositório (pode ser custoso para repositórios grandes)

### 3.2 Prediction Shadow (Leve)

**Propósito**: Simular decisões onde o custo de execução real (mesmo em shadow) é alto demais, mas uma estimativa preditiva é suficiente.

**Como funciona**:

```
1. Shadow consulta Prediction Engine (F7.1) para cada plano
2. Para cada plano, Prediction Engine retorna:
   ├── P(success) estimado
   ├── Tempo estimado
   ├── Custo estimado
   ├── Risco estimado
   └── Fatores críticos
3. Shadow compara as predições dos dois planos
4. Shadow gera relatório comparativo baseado em predição
```

**Custo**: ~$0.001 – $0.003 (apenas Prediction Engine)
**Tempo**: < 500ms
**Isolamento**: Nenhum ambiente é criado — apenas consulta a dados históricos
**Quando usar**: Decisões de baixo custo, triagem inicial, quando git shadow é caro demais

**Limitações**:
- Não executa nada — é uma estimativa, não uma simulação real
- Precisa de dados históricos suficientes no Trust Registry
- Menos precisão que git shadow ou full shadow

### 3.3 Full Shadow (Pesado)

**Propósito**: Simular decisões complexas que envolvem múltiplos agentes, mudanças arquiteturais profundas, ou onde a precisão da simulação é crítica.

**Como funciona**:

```
1. Shadow Mode consulta Capability Market (F8.1) para alocar agentes
2. Para cada plano, um conjunto de agentes é alocado
3. Ambiente sandbox é criado (contêiner/VM isolado)
4. Plano A: agentes executam em paralelo no sandbox
5. Plano B: agentes executam em paralelo no sandbox (ou sandbox paralelo)
6. Shadow Engine coleta métricas detalhadas de cada plano
7. Comparação completa com todas as dimensões
8. Ambiente sandbox é destruído
```

**Custo**: ~$0.020 – $0.100 (agentes + ambiente + tempo)
**Tempo**: 5min – 30min
**Isolamento**: Sandbox completo sem acesso a produção, rede isolada, sem side effects
**Quando usar**:
- Decisões P0 de arquitetura
- Decisões que afetam 3+ módulos
- Decisões com custo estimado > $0.05
- Don requisitou simulação completa

**Limitações**:
- Custo mais alto — só deve ser usado quando o custo da decisão real justifica
- Mais lento — pode atrasar a tomada de decisão
- Requer agentes disponíveis no Capability Market

### 3.4 Matriz de Seleção

```
┌──────────────────┬──────────────┬──────────────┬────────────────────┐
│ Critério         │ Git Shadow   │Prediction    │ Full Shadow        │
├──────────────────┼──────────────┼──────────────┼────────────────────┤
│ Custo            │ $0.005-0.015 │ $0.001-0.003 │ $0.020-0.100       │
│ Tempo            │ 30s-5min     │ < 500ms      │ 5min-30min         │
│ Precisão         │ Alta         │ Média        │ Máxima             │
│ Isolamento       │ Total        │ N/A          │ Total              │
│ Agentes reais    │ Não          │ Não          │ Sim                │
│ Cobertura real   │ Sim          │ Não          │ Sim                │
│ Risco para prod. │ Zero         │ Zero         │ Zero               │
│ Complexidade     │ Baixa        │ Mínima       │ Alta               │
├──────────────────┼──────────────┼──────────────┼────────────────────┤
│ Qdo usar         │ Refatoração  │ Triagem      │ Arq. P0            │
│                  │ Migração     │ Decisões     │ Multi-módulo       │
│                  │ Remoção      │ baratas      │ Alto risco         │
└──────────────────┴──────────────┴──────────────┴────────────────────┘
```

### 3.5 Seleção Automática do Tipo

O Shadow Mode seleciona automaticamente o tipo baseado em:

```yaml
shadow_type_selection:
  regras:
    - se: cost_estimado > 0.05
      entao: "full"                   # Decisões caras merecem shadow completo

    - se: prioridade == "P0" AND modulos_afetados >= 3
      entao: "full"                   # P0 multi-módulo requer full

    - se: prioridade == "P0" AND modulos_afetados < 3
      entao: "git"                    # P0 de escopo limitado

    - se: prioridade == "P1" AND risk > 0.30
      entao: "git"                    # P1 com alto risco

    - se: prioridade == "P1" AND risk <= 0.30
      entao: "prediction"             # P1 com risco controlado

    - se: don_request == "full"
      entao: "full"                   # Don explicitamente pediu

  default: "prediction"               # Para qualquer outro caso
```

---

## 4. Integração com F8.1 — Capability Market

### 4.1 Shadow Consulta o Market para Alocar Agentes

Quando o Shadow Mode precisa executar **Full Shadow**, ele consulta o Capability Market para alocar agentes especialistas para o ensaio:

```
SHADOW MODE (F8.2)                      CAPABILITY MARKET (F8.1)
┌─────────────────────────┐            ┌──────────────────────────┐
│ Preciso simular         │            │                          │
│ Plano A e Plano B       │──request──▶│ Leilão por capacidade:   │
│ para esta decisão       │            │  ├── 2 agentes p/ Plano A│
│                         │            │  ├── 2 agentes p/ Plano B│
│ Agentes:                │            │  ├── Prioridade: shadow  │
│  ├── 2x architecture    │            │  └── Max cost: $0.040    │
│  ├── 1x database        │            │                          │
│  └── 1x testing         │            │◀──response──             │
│                         │            │                          │
│ Ambiente: sandbox       │            │ Agentes alocados:        │
│ Budget: $0.050          │            │  ├── cosca-architecture  │
│                         │            │  ├── cosca-database      │
│                         │            │  ├── cosca-testing       │
│                         │            │  └── cosca-kernel (ops)  │
└─────────────────────────┘            └──────────────────────────┘
```

### 4.2 Shadow Bids do Shadow Mode

Assim como o Capability Market registra shadow bids de agentes perdedores, o Shadow Mode registra **shadow bids de planos perdedores**:

```yaml
shadow_bid_plano_perdedor:
  decision_id: "DDNA-2026-07-30-004"
  shadow_report_id: "SHADOW-2026-07-30-001"
  plano_perdedor: "B"
  score: 0.78                     # Score do plano na comparação
  vencedor: "A"
  gap: 0.15                       # Diferença de score
  would_have:
    outcome: "success_with_issues"
    tempo: "3m45s"
    custo: "$0.028"
    riscos:
      - "Migração de schema pode causar downtime"
  learning: "Plano B era mais barato mas introduzia riscos de quebra de schema.
             Em decisões futuras de migração, priorizar segurança sobre custo."
```

### 4.3 Shadow Lock (Shadow → Capability Market)

Quando o Shadow Mode detecta que um plano específico requer capacidades que não estão disponíveis no momento:

```
1. Shadow Mode detecta: "Plano A requer cosca-database, mas não há disponível"
2. Shadow Mode NOTIFICA Capability Market: "Preciso de database para shadow"
3. Capability Market pode:
   ├── Alocar agente de outro domínio (ex: cosca-architecture)
   ├── Esperar agente liberar (se previsão < 5min)
   └── Notificar Kernel: "Shadow incompleto — capacidade faltante"
4. Shadow Mode registra no relatório: "Plano A não pôde ser totalmente simulado"
```

### 4.4 Alocação com Tag Shadow

Agentes alocados para shadow mode recebem uma tag especial:

```yaml
capability_allocation:
  agent: "cosca-database"
  task_id: "shadow-migration-plan-a"
  mode: "shadow"
  environment: "sandbox"
  budget: "$0.020"
  constraints:
    - "NUNCA alterar produção"
    - "NUNCA acessar banco de dados real"
    - "Usar apenas réplica local/sandbox"
  cleanup: "destroy_after_report"
```

---

## 5. Integração com F1.2 — Contrafactual Gate

### 5.1 Shadow Mode é ATIVADO pelo Gate

O Contrafactual Gate (F1.2) é o **gatilho de entrada** do Shadow Mode. O Gate analisa a decisão, gera alternativas, e se a decisão for P0 ou risco > 30%, **ativa o Shadow Mode**:

```
CONTRAFACTUAL GATE (F1.2)                 SHADOW MODE (F8.2)
┌──────────────────────────────┐         ┌──────────────────────────┐
│ Gate analisa decisão:        │         │                          │
│  ├── Priority: P0            │         │                          │
│  ├── Risk: 0.45 (ALTO)       │──ativa──▶│ Shadow ativado!          │
│  ├── Alternativa A: migrar   │  shadow  │  ├── Tipo: git shadow    │
│  ├── Alternativa B: ficar    │          │  ├── Plano A: migrar     │
│  └── Algo = "Shadow mode     │          │  ├── Plano B: ficar      │
│       necessário para        │          │  └── Executando...       │
│       decidir entre A e B"   │          │                          │
│                              │◀──report──│ Relatório enviado:      │
│                              │  shadow  │  ├── Plano A vence (63%) │
│                              │          │  ├── B era 30% + barato  │
│                              │          │  └── Risco de A é menor  │
└──────────────────────────────┘         └──────────────────────────┘
```

### 5.2 Condições de Ativação

| Condição | Shadow Mode | Exemplo |
|----------|-------------|---------|
| Decisão P0 | **Sempre ativado** | "Migrar banco de dados" |
| Decisão P1 + risco > 30% | **Ativado** | "Refatorar módulo de auth" |
| Decisão multi-módulo (3+) | **Ativado** | "Mudar framework web" |
| Don requisitou | **Ativado** | "Quero ver os dados antes" |
| Decisão P1 + risco <= 30% | **Não ativado** | "Renomear função" |
| Decisão P2/P3 | **Não ativado** | "Corrigir typo" |
| Fast-track do Gate | **Não ativado** | "Decisão trivial" |

### 5.3 Pipeline Gate → Shadow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    PIPELINE GATE → SHADOW                                 │
│                                                                         │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐             │
│  │ Gate     │──▶│ Gate     │──▶│ Shadow   │──▶│ Shadow   │             │
│  │ Aberto   │   │ decide   │   │ executa  │   │ reporta  │             │
│  │ (Passos  │   │ "Shadow  │   │ simulação│   │ resultado│             │
│  │ 1-5)     │   │ ativado" │   │ completa │   │ p/ Gate  │             │
│  └──────────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘             │
│                       │              │              │                    │
│                       ▼              │              ▼                    │
│                ┌──────────────┐      │      ┌──────────────────┐        │
│                │ Gate decide: │      │      │ Gate incorpora   │        │
│                │ Plano A vs B │◀─────┘      │ shadow report    │        │
│                │ com shadow   │             │ na recomendação  │        │
│                │ data         │             └────────┬─────────┘        │
│                └──────────────┘                      │                  │
│                                                      ▼                  │
│                                              ┌──────────────────┐       │
│                                              │ Gate Fechado      │       │
│                                              │ (Passo 7)         │       │
│                                              │ Recomendação      │       │
│                                              │ baseada em dados  │       │
│                                              │ reais do shadow   │       │
│                                              └──────────────────┘       │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.4 Shadow Substitui a Predição para Decisões P0

Para decisões P0, o Shadow Mode **substitui a Prediction Engine como fonte de dados** para o Gate:

```
Fluxo normal (P1):
  Gate → Prediction Engine → Gate recomenda → Kernel decide

Fluxo P0:
  Gate → Shadow Mode (com Prediction + execução real isolada) →
  Shadow reporta dados → Gate recomenda baseado em dados empíricos →
  Kernel decide
```

### 5.5 Shadow Report como Evidência no Gate

O relatório do Shadow Mode é incorporado como **evidência de alta força** no output do Contrafactual Gate:

```yaml
evidence:
  - evidence: "Shadow Mode simulation: Plano A completou em 2m30s com 100% testes verdes"
    source: "SHADOW-2026-07-30-001"
    strength: "alta"
    type: "empírica"
    trust_weight: 0.95           # Shadow tem peso máximo como evidência
```

---

## 6. Integração com F7.1 — Prediction Engine

### 6.1 Prediction é Usada nos 3 Tipos de Shadow

| Tipo de Shadow | Uso da Prediction Engine |
|----------------|-------------------------|
| **Prediction shadow** | Prediction Engine é a **fonte única** de dados |
| **Git shadow** | Prediction Engine fornece **baseline**; shadow compara real vs predito |
| **Full shadow** | Prediction Engine fornece **estimativa prévia**; shadow valida |

### 6.2 Feedback Loop: Shadow → Prediction

Após cada shadow, o resultado real da simulação é enviado de volta ao Prediction Engine para calibragem:

```yaml
shadow_feedback:
  decision_id: "DDNA-2026-07-30-004"
  shadow_report_id: "SHADOW-2026-07-30-001"

  plano_a:
    previsto_pelo_prediction:
      tempo: "3m00s"
      custo: "$0.040"
    real_no_shadow:
      tempo: "2m30s"
      custo: "$0.035"
    erro_tempo: -16.7%   # Shadow foi mais rápido que o previsto
    erro_custo: -12.5%   # Shadow foi mais barato que o previsto

  plano_b:
    previsto_pelo_prediction:
      tempo: "4m00s"
      custo: "$0.030"
    real_no_shadow:
      tempo: "3m45s"
      custo: "$0.028"
    erro_tempo: -6.25%
    erro_custo: -6.67%

  calibrar: true
  acao: "Prediction Engine subestimou ambos os planos em tempo.
         Ajustar complexity_multiplier em -0.05 para este domínio."
```

### 6.3 Prediction Shadow — Modo Light

Quando apenas **Prediction Shadow** é usado (tipo mais leve), o pipeline é:

```
1. Shadow Mode define Plano A e Plano B
2. Para cada plano, Shadow consulta Prediction Engine:
   ├── Quanto tempo? Qual custo? Qual risco?
   └── Qual P(success) de cada plano?
3. Shadow compara predições
4. Shadow gera relatório baseado em predição
5. Shadow recomenda baseado em dados preditivos

Atenção: Prediction shadow NÃO executa nada.
É uma simulação baseada em dados históricos.
Precisão depende da qualidade dos dados no Trust Registry.
```

---

## 7. Integração com F1.1 — Decision DNA

### 7.1 Shadow Report é Evidência no DDNA

O relatório completo do Shadow Mode é armazenado como **evidência de alta confiança** no DDNA da decisão:

```yaml
# Seção Evidence do DDNA
evidence:
  - source: "Shadow Mode Report"
    report_id: "SHADOW-2026-07-30-001"
    type: "empirical"
    strength: "alta"
    summary: >
      Shadow Mode simulou Plano A (migração SQLite→Postgres) vs
      Plano B (manter SQLite com otimizações).
      Resultado: Plano A é 20% mais rápido, Plano B é 30% mais barato.
      Shadow recomenda: Plano A (qualidade superior justifica custo).
    winner: "A"
    matriz_comparativa: { ... }
```

### 7.2 DDNA com Shadow Linkado

Quando uma decisão passa pelo Shadow Mode, o DDNA inclui:

```yaml
---
id: DDNA-2026-07-30-004
title: "Migrar SQLite para PostgreSQL — decisão com shadow"
status: accepted
date: 2026-07-30
agents:
  - cosca-architecture
  - cosca-database
  - cosca-shadow-mode
domain: database
decision_level: 0
confidence: 0.92     # Elevado porque shadow gerou dados empíricos
shadow_report: "SHADOW-2026-07-30-001"  # Link direto
tags:
  - dna
  - shadow-mode
  - database-migration
  - evidence-backed
---
```

### 7.3 Shadow Enriquecimento do DDNA

| Campo DDNA | Sem Shadow | Com Shadow |
|------------|------------|------------|
| `confidence` | Baseado em estimativa | Baseado em dados empíricos (shadow +10-15%) |
| `options` | "A vs B" teórico | "A vs B" com métricas reais de execução |
| `evidence` | Referências a documentos | Dados de simulação com força "alta" |
| `risks_identified` | Riscos teóricos | Riscos observados durante a simulação |
| `consequences` | Estimadas | Medidas no shadow |

---

## 8. Custo do Shadow

### 8.1 Regra: Custo do Shadow < 20% do Custo da Decisão Real

```
custo_shadow ≤ custo_decisao_real × 0.20
```

| Tipo de Decisão | Custo Real Típico | Budget Máximo do Shadow |
|-----------------|:-----------------:|:-----------------------:|
| P0 — Arquitetura | $0.20 – $1.00 | $0.04 – $0.20 |
| P0 — Banco de dados | $0.15 – $0.50 | $0.03 – $0.10 |
| P1 — Refatoração | $0.05 – $0.15 | $0.01 – $0.03 |
| P1 — Feature design | $0.03 – $0.10 | $0.006 – $0.02 |
| P0 — Segurança | $0.10 – $0.40 | $0.02 – $0.08 |

### 8.2 Custo por Tipo de Shadow

| Tipo | Custo Máximo (USD) | Tempo Máximo | Orçamento de Tokens |
|------|:------------------:|:------------:|:-------------------:|
| **Git shadow** | $0.015 | 5min | 3.000 |
| **Prediction shadow** | $0.003 | 500ms | 500 |
| **Full shadow** | $0.100 | 30min | 20.000 |

### 8.3 Budget Check

Antes de iniciar, o Shadow Mode verifica se o budget é suficiente:

```yaml
budget_check:
  shadow_type: "git"
  budget_maximo: "$0.015"
  custo_estimado: "$0.012"       # 80% do budget
  custo_decisao_real: "$0.080"   # Shadow = 15% da decisão
  dentro_limite: true            # 15% < 20% ✓
  # Se custo_estimado > budget_maximo: reduzir shadow para prediction
  # Se shadow/decisao > 0.20: alerta + reduzir escopo
```

### 8.4 Se o Custo Exceder o Budget

```
SE custo_shadow_estimado > budget_shadow:
  ├── Shadow tenta modo mais leve:
  │     full → git → prediction
  ├── Se prediction ainda estoura: notificar Kernel
  └── Kernel decide:
        ├── Aumentar budget (com rationale)
        ├── Executar shadow parcial (apenas Plano A)
        └── Pular shadow (com aviso no DDNA)
```

### 8.5 Shadow Gratuito (Modo Economy)

Para decisões de baixo custo, o Shadow Mode pode operar em **Modo Economy**:

```
Modo Economy:
  ├── Apenas git shadow (sem full)
  ├── Sem agentes dedicados (usa dados do cache do Market)
  ├── Sem ambiente sandbox (usa clone mínimo)
  ├── Apenas métricas essenciais (tempo, custo, testes)
  └── Relatório resumido (sem análise de risco detalhada)

Custo: ~$0.003 – $0.008
Quando: Decisões P1 com custo < $0.05
```

---

## 9. Regras de Isolamento

### 9.1 Princípios de Isolamento

```
┌─────────────────────────────────────────────────────────────────┐
│                    REGRAS DE ISOLAMENTO                          │
│                                                                  │
│  ABSOLUTO:                                                        │
│  ├── Shadow NUNCA altera produção                                │
│  ├── Shadow NUNCA acessa banco de dados de produção              │
│  ├── Shadow NUNCA chama APIs externas de produção                │
│  └── Shadow NUNCA persiste dados fora do ambiente de simulação   │
│                                                                  │
│  TÉCNICO:                                                         │
│  ├── Git shadow: clone sem remote write, sem push                │
│  ├── Full shadow: sandbox com rede isolada                       │
│  └── Prediction shadow: não toca em nada (apenas CPU/memória)    │
│                                                                  │
│  LIMPEZA:                                                         │
│  ├── Ao final: destruir ambiente completamente                   │
│  ├── Logs do shadow: armazenados apenas no relatório             │
│  └── Nenhum artifact do shadow vaza para produção                │
└─────────────────────────────────────────────────────────────────┘
```

### 9.2 Checklist de Isolamento

- [ ] Ambiente shadow não tem credenciais de produção
- [ ] Ambiente shadow não tem acesso à rede de produção
- [ ] Git shadow não tem permissão de push para remote
- [ ] Full shadow usa contêiner/VM descartável
- [ ] Ao finalizar: `cleanup.sh` executado e verificado
- [ ] Logs de shadow não contêm dados sensíveis
- [ ] Relatório de shadow não expõe secrets

### 9.3 Falha de Isolamento

```
SE violação de isolamento detectada:
  ├── 1. ABORTAR shadow imediatamente
  ├── 2. Registrar incidente: "Shadow isolation breach"
  ├── 3. Notificar Kernel + Security Chief
  ├── 4. Auditar se produção foi afetada
  └── 5. Shadow Mode desativado até revisão do Don
```

---

## 10. Exemplo: Remover Código Morto

### Contexto

O Cosca detectou que o pacote `pkg/legacy/api.go` contém ~800 linhas de código não utilizado há 6 meses. A decisão: **remover ou manter o código morto?**

### Gatilho do Shadow

- **Decisão**: P1 (refatoração — impacto moderado)
- **Risco**: 35% (confidence do agente proponente: 0.65)
- **Gatilho**: P1 + risco > 30% → Shadow ativado
- **Tipo selecionado**: Git shadow (refatoração de código, escopo limitado)

### Pipeline Executado

```
Passo 1: Decisão chega
  └── "Remover código morto de pkg/legacy/api.go e dependências"

Passo 2: Ambiente isolado criado
  ├── git clone --depth=1 .shadow/remocao-codigo-morto
  ├── cd .shadow/remocao-codigo-morto
  └── git checkout -b shadow/rm-legacy-api

Passo 3: Plano A (Remover código morto)
  ├── Aplicar remoção (git rm + refatoração de imports)
  ├── Rodar testes:   go test ./... → 142 passed, 0 failed
  ├── Cobertura:      95.2% → 94.8% (-0.4%)
  ├── LOC:            -842 / +23 (líquido: -819)
  ├── Tempo:          2m12s
  ├── Custo:          $0.008
  └── Riscos:         Nenhum risco detectado

Passo 4: Plano B (Manter código morto)
  ├── git reset --hard
  ├── Não fazer nada (baseline)
  ├── Testes:         142 passed, 0 failed
  ├── Cobertura:      95.2%
  ├── LOC:            0
  ├── Tempo:          0s (não executa nada)
  ├── Custo:          $0.000
  └── Riscos:         Dívida técnica contínua, manutenção desnecessária

Passo 5: Comparação
  ┌──────────────────────┬──────────────┬──────────────┬──────────────┐
  │ Métrica              │ Plano A      │ Plano B      │ Diferença    │
  ├──────────────────────┼──────────────┼──────────────┼──────────────┤
  │ Testes quebrados     │ 0            │ 0            │ Empate       │
  │ Cobertura            │ 94.8%        │ 95.2%        │ A perde 0.4% │
  │ LOC                  │ -819         │ 0            │ A ganha      │
  │ Dívida técnica       │ Reduzida     │ Mantida      │ A vence      │
  │ Risco de regressão   │ Baixo        │ Zero         │ B vence      │
  │ Custo de manutenção  │ Menor        │ Maior        │ A vence      │
  │ Reversibilidade      │ Fácil (git)  │ N/A          │ Empate       │
  └──────────────────────┴──────────────┴──────────────┴──────────────┘

Passo 6: Relatório gerado
  ├── shadow_report_id: "SHADOW-2026-07-30-002"
  ├── Recomendação: Plano A (remover código morto)
  ├── Por que: -819 LOC de dívida técnica eliminada, zero testes quebrados,
  │            cobertura caiu apenas 0.4% (aceitável), reversível via git.
  └── Observação: "Shadow não encontrou riscos. Execução real segura."

Passo 7: Kernel decide
  └── "Plano A — remover código morto. Shadow confirmou que é seguro."
```

### Relatório de Shadow (YAML)

```yaml
---
# =============================================================================
# SHADOW MODE REPORT
# =============================================================================
shadow_report:
  version: "1.0.0"
  report_id: "SHADOW-2026-07-30-002"
  timestamp: "2026-07-30T15:30:00Z"
  shadow_type: "git"
  cost: "$0.008"
  duration: "2m12s"
  executed_by: "cosca-shadow-mode"

  decision:
    title: "Remover código morto de pkg/legacy/api.go"
    id: "DDNA-2026-07-30-005"
    domain: "refactoring"
    priority: "P1"
    risk: 0.35

  plans:
    - id: "A"
      name: "Remover código morto"
      status: "completed"
      metrics:
        tests_passed: 142
        tests_failed: 0
        coverage_before: 0.952
        coverage_after: 0.948
        loc_delta: -819
        files_changed: 4
        duration: "2m12s"
        cost_usd: 0.008
        risks_found: []
        reversibility: "easy"
      outcome: "success"

    - id: "B"
      name: "Manter código morto"
      status: "completed"
      metrics:
        tests_passed: 142
        tests_failed: 0
        coverage_before: 0.952
        coverage_after: 0.952
        loc_delta: 0
        files_changed: 0
        duration: "0s"
        cost_usd: 0.000
        risks_found:
          - "Dívida técnica acumulada — 842 LOC não utilizados"
          - "Custo de manutenção contínuo sem benefício"
        reversibility: "n/a"
      outcome: "success_with_concerns"

  comparison:
    winner: "A"
    winner_score: 0.85
    loser_score: 0.40
    gap: 0.45
    decisive_metrics:
      - "Plano A elimina 819 linhas de dívida técnica"
      - "Zero regressão em testes"
      - "Queda de cobertura mínima (0.4%)"
    tradeoffs:
      - "Plano A reduz cobertura em 0.4% mas elimina dívida técnica"

  recommendation:
    outcome: "proceed_with_plan_a"
    confidence: 0.94
    rationale: >
      Shadow Mode simulou a remoção de código morto em ambiente isolado.
      Resultado: 819 LOC eliminados, zero testes quebrados,
      cobertura caiu apenas 0.4% (dentro do aceitável).
      Risco de regressão é mínimo e a operação é reversível via git.
      Plano A é claramente superior — proceed com remoção.

  evidence:
    - type: "git_diff"
      path: ".shadow/remocao-codigo-morto/diffs/plan_a.diff"
      summary: "-842 / +23 linhas, 4 arquivos alterados"
    - type: "test_report"
      path: ".shadow/remocao-codigo-morto/reports/tests_plan_a.xml"
      summary: "142 passed, 0 failed"
    - type: "coverage_report"
      path: ".shadow/remocao-codigo-morto/reports/coverage_plan_a.html"
      summary: "94.8% (0.4% abaixo do baseline)"
```

---

## 11. Métricas e Monitoramento

### 11.1 Métricas do Shadow Mode

| Métrica | Tipo | Descrição | Alerta |
|---------|------|-----------|--------|
| `shadow.activations.total` | Counter | Total de shadow executions | — |
| `shadow.activations.by_type` | Counter | Por tipo (git, prediction, full) | — |
| `shadow.activations.by_trigger` | Counter | Por gatilho (P0, risk, don) | — |
| `shadow.outcome` | Counter | Plano A wins / B wins / inconclusive | — |
| `shadow.cost.usd` | Histogram | Custo real de cada shadow | Se > 20% do custo da decisão |
| `shadow.cost.ratio` | Histogram | shadow_cost / decision_cost | Se > 0.20, reduzir escopo |
| `shadow.duration` | Histogram | Tempo de execução do shadow | — |
| `shadow.isolation.breaches` | Counter | Violações de isolamento | **Crítico** — zero tolerância |
| `shadow.prediction_accuracy` | Histogram | Quão preciso foi o shadow vs real | Se < 0.70, calibrar |
| `shadow.budget_exceeded` | Counter | Vezes que estourou o budget | — |
| `shadow.skipped` | Counter | Decisões que deveriam ter shadow mas pularam | Se > 10%, revisar regras |

### 11.2 Dashboard

```yaml
shadow_dashboard:
  widgets:
    - "Gauge: taxa de acerto do shadow (Plano A recomendado vs executado)"
    - "Time series: shadows/dia por tipo"
    - "Bar: custo médio do shadow vs custo médio da decisão"
    - "Table: últimos 5 shadows com winner e confidence"
    - "Alert: isolation breach → zero tolerance"
    - "Heatmap: tipos de decisão × tipo de shadow mais frequente"
```

### 11.3 Shadow Accuracy

A **acurácia do Shadow Mode** é medida comparando a recomendação do shadow com o resultado real da execução:

```
shadow_accuracy:
  ┌────────────────────────────────────────────────────────────────────┐
  │ Medição pós-execução real:                                         │
  │                                                                    │
  │ 1. Shadow recomendou Plano A?                                      │
  │ 2. Kernel executou Plano A?                                        │
  │ 3. Resultado real foi "success"?                                   │
  │                                                                    │
  │ Se sim a todas: shadow_accuracy_hit++                              │
  │ Se shadow recomendou A, kernel executou B: shadow_accuracy_miss++  │
  │ Se shadow recomendou A mas resultado real foi failure: calibrar    │
  └────────────────────────────────────────────────────────────────────┘
```

---

## 12. Relacionados

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **Capability Market (F8.1)** | Shadow consulta Market para alocar agentes | `engines/capability-market/SKILL.md` |
| **Contrafactual Gate (F1.2)** | Gate ativa Shadow Mode quando P0 ou risco > 30% | `workflows/contrafactual-gate.md` |
| **Prediction Engine (F7.1)** | Prediction fornece estimativas; Shadow valida | `engines/prediction/SKILL.md` |
| **Decision DNA (F1.1)** | Shadow report é evidência no DDNA | `knowledge/architecture/DECISION_DNA.md` |
| **QUALITY_GATES.md** | Gate 0.5 — referência de ativação do shadow | `QUALITY_GATES.md` |
| **KERNEL.md** | §10 — Invocação do shadow entre Gate e execução | `KERNEL.md` |
| **Cognitive Economy (F2.1)** | Custo do shadow é contabilizado | `engines/cognitive-economy/SKILL.md` |
| **Trust Registry (F7.2)** | Dados históricos alimentam Prediction no shadow | `memory/trust/TRUST_REGISTRY.md` |
| **next-evolution-phases.md** | F8.2 — roadmap do Shadow Mode | `knowledge/architecture/next-evolution-phases.md` |
| **cosca-architecture PROMPT.md** | Architecture Chief — owner do Shadow Mode | `agents/cosca-architecture/PROMPT.md` |
| **CONFIDENCE_MODEL.md** | Ponderação de evidências do shadow | `engines/evidence/CONFIDENCE_MODEL.md` |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | cosca-architecture | Criação do Shadow Mode Engine — pipeline 7 passos, 3 tipos de shadow (git, prediction, full), integração com Capability Market, Contrafactual Gate, Prediction Engine, DDNA, regras de isolamento, custo < 20%, exemplo de remoção de código morto |

---

> **Owner**: cosca-architecture | **Ativado por**: Contrafactual Gate (F1.2) quando decisão P0 ou risco > 30% | **Engine path**: `engines/shadow-mode/SKILL.md`
>
> *"Ensaie antes de agir. Simule antes de executar. Compare antes de decidir."*
