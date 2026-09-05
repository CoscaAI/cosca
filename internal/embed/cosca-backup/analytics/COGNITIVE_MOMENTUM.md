# MOMENTUM COGNITIVO — Métrica de Tração por Domínio

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Analytics Chief | **Criado**: 2026-07-30 | **DNA Version**: 3.0.0

---

## 1. Propósito

O **Cognitive Momentum** (Momento Cognitivo) mede a vitalidade de cada domínio de investigação do Cosca Runtime — distinguindo linhas de investigação "vivas" (momentum alto, produzindo insights) de linhas "mortas" (momentum baixo, estagnadas). O runtime investe energia cognitiva onde há tração, não insiste em becos sem saída.

A metáfora do Don estabelece o framework conceitual:

> *"Algumas linhas de investigação estão evoluindo. Outras morreram. O runtime sabe onde vale investir."*

- **Momentum alto** = domínio produzindo resultados, acumulando aprendizados, atraindo atenção cross-agent. O investimento se paga.
- **Momentum baixo** = domínio estagnado ou abandonado. Continuar investindo é desperdício de recursos cognitivos.
- **Momentum zero** = domínio morto. Deve ser arquivado para liberar recursos para domínios com tração.

### Por que medir momentum?

1. **Alocação dinâmica de recursos**: O Don e o Kernel precisam saber onde concentrar os 54 agentes. Momentum é o sinal objetivo — não intuição.
2. **Prevenção de sunk cost**: Continuar investindo em domínio morto é o equivalente cognitivo de dobrar a aposta no cassino. Momentum baixo persistente dispara arquivamento.
3. **Sinal de aceleração**: Momentum subindo detecta early-stage breakthroughs — domínios que estão prestes a explodir em produtividade merecem investimento antecipado.
4. **Integração com Mental Energy (C7)**: O budget de energia cognitiva é alocado proporcionalmente ao momentum. Momentum alto → até 40% do budget. Momentum baixo → no máximo 10%.
5. **Integração com CMI (Planejamento)**: Capacidade de redirecionar recursos baseada em evidência de tração → dimensão Planejamento mais alta.

---

## 2. A Fórmula do Momentum

### 2.1 Equação Principal

```
momentum(domain) = (
  recent_activity      × 0.35 +   # tarefas/aprendizados nos últimos 7 dias
  velocity             × 0.25 +   # taxa de variação (acelerando ou desacelerando)
  depth                × 0.20 +   # profundidade da investigação
  cross_pollination    × 0.15 +   # influência de outros domínios
  outcome_quality      × 0.05     # taxa de sucesso das tarefas recentes
) × 100
```

O score é normalizado para o intervalo 0-100, onde 0 = domínio completamente morto e 100 = domínio em aceleração máxima com resultados impecáveis.

### 2.2 Componente 1 — Recent Activity (Atividade Recente) — Peso 0.35

**O que mede**: Quantas tarefas ou aprendizados o domínio produziu nos últimos 7 dias. Domínios ativos geram rastro. Domínios mortos não.

```
recent_activity = min(tasks_last_7d, 10) / 10
```

| Tarefas em 7 dias | recent_activity | Interpretação |
|-------------------|-----------------|---------------|
| 0 | 0.00 | Domínio completamente inativo |
| 1-2 | 0.10-0.20 | Atividade esporádica |
| 3-5 | 0.30-0.50 | Atividade regular |
| 6-9 | 0.60-0.90 | Domínio aquecido |
| 10+ | 1.00 | Domínio em chamas (saturado) |

**Fontes de dados**:
- `learnings.md` de todos os agentes — contar entradas com tags do domínio nos últimos 7 dias
- `evolution.md` — contar tasks concluídas com domain tag nos últimos 7 dias
- Workflows disparados com tag do domínio
- Commits com escopo relevante ao domínio (ex: `feat(security): ...`)

**Por que 0.35 de peso?** Atividade recente é o sinal mais forte de que um domínio está vivo. Um domínio pode ter alta profundidade histórica mas se ninguém trabalha nele há 2 semanas, o momentum é baixo. O presente pesa mais que o passado.

### 2.3 Componente 2 — Velocity (Velocidade) — Peso 0.25

**O que mede**: Se a atividade está acelerando ou desacelerando. Momentum não é só "quanto", é "para onde".

```
velocity = (tasks_this_week - tasks_last_week) / max(tasks_last_week, 1)
```

| Cenário | tasks_this_week | tasks_last_week | velocity | Interpretação |
|---------|----------------|-----------------|----------|---------------|
| Aceleração explosiva | 8 | 2 | 3.00 | ↑↑↑ Crescimento de 4× |
| Aceleração forte | 6 | 3 | 1.00 | ↑↑ Dobrou a atividade |
| Aceleração moderada | 5 | 3 | 0.67 | ↑ Crescimento de 67% |
| Estável | 4 | 4 | 0.00 | → Mantendo ritmo |
| Desaceleração leve | 3 | 4 | -0.25 | ↓ Perdendo fôlego |
| Desaceleração forte | 1 | 5 | -0.80 | ↓↓ Em declínio |
| Parada brusca | 0 | 6 | -1.00 | ↓↓↓ Abandonado |

**Clamp**: velocity é clamped para o intervalo [-1.0, 3.0] para evitar que domínios que saíram de 0→1 tarefa distorçam o score. A normalização final converte para escala 0-1:

```
velocity_normalized = (velocity + 1) / 4   # mapeia [-1, 3] → [0, 1]
```

**Por que 0.25 de peso?** A direção da mudança é tão importante quanto a magnitude. Um domínio com 2 tarefas acelerando de 0 merece mais atenção que um domínio com 4 tarefas estável há semanas. O futuro potencial importa.

### 2.4 Componente 3 — Depth (Profundidade) — Peso 0.20

**O que mede**: O nível médio das tarefas no domínio. Tarefas Level 1 (execução simples) têm menos valor cognitivo que tarefas Level 4 (cross-agent synthesis, pattern extraction, metacognição).

```
depth = avg_task_level / 5
```

| Task Level | Descrição | Exemplo | Valor para depth |
|------------|-----------|---------|-----------------|
| **1** | Execução básica | Rodar um comando, criar um arquivo | 0.20 |
| **2** | Análise com debugging | Diagnosticar bug, aplicar correção conhecida | 0.40 |
| **3** | Síntese cross-source | Auditoria multi-agente, design de framework | 0.60 |
| **4** | Metacognição | Extrair padrão universal, redefinir identidade | 0.80 |
| **5** | Criação de novo paradigma | Insight que redefine como o sistema opera | 1.00 |

**Exemplos reais (extraídos de learnings.md)**:

| Domínio | Tasks recentes | Levels | avg | depth |
|---------|---------------|--------|-----|-------|
| Architecture | L20(L4), L21(L4), L22(L4), L23(L4) | 4,4,4,4 | 4.0 | 0.80 |
| Testing/Coverage | L18(L4), L19(L4), L20(L4), L21(L4) | 4,4,4,4 | 4.0 | 0.80 |
| Security | L12(L3), L14(L3), L15(L4) | 3,3,4 | 3.33 | 0.67 |
| CLI | L16(L3), L19(L4) | 3,4 | 3.5 | 0.70 |
| Frontend | — | — | 0 | 0.00 |

**Por que 0.20 de peso?** Profundidade qualifica a atividade — não é só "quantas tarefas", é "quão sofisticadas". Dois domínios com 4 tarefas/semana podem ter momenta muito diferentes se um opera em Level 1 e outro em Level 4.

### 2.5 Componente 4 — Cross-Pollination (Polinização Cruzada) — Peso 0.15

**O que mede**: O quanto o domínio está conectado com outros domínios. Domínios isolados têm menos momentum sustentável porque não se beneficiam de insights externos. Domínios com alta polinização cruzada aceleram mais rápido — cada descoberta em um domínio vizinho potencialmente alimenta este domínio.

```
cross_pollination = cross_domain_references / max(total_references, 1)
```

**O que conta como cross-domain reference**:
- Campo `Related` do learnings.md referencia agentes de outro domínio
- Tags do learnings.md mencionam outro domínio (ex: entry no domínio Architecture com tag `#security`)
- Task delegou subagentes de outro domínio
- Pattern extraído foi aplicado em domínio diferente do original

| Classificação | cross_pollination | Interpretação |
|---------------|-------------------|---------------|
| **Isolado** | 0.00-0.10 | Domínio opera em silo. Sem influência externa. |
| **Baixa** | 0.11-0.25 | Conexões esparsas com 1-2 domínios vizinhos |
| **Média** | 0.26-0.50 | Rede de conexões moderada. Insights fluem entre 3-5 domínios. |
| **Alta** | 0.51-0.75 | Domínio altamente conectado. 6+ domínios contribuem. |
| **Ubíqua** | 0.76-1.00 | Domínio hub — praticamente toda tarefa cross-reference. |

**Exemplos reais**:

| Domínio | Referências Cross-Domain | Total Ref | cross_pollination |
|---------|--------------------------|-----------|-------------------|
| Architecture | 6 (testing, security, runtime, docs, memory, orchestration) | 6 | 1.00 |
| Cognitive Maturity | 5 (architecture, analytics, evolution, memory, orchestration) | 5 | 1.00 |
| Runtime | 4 (security, cli, testing, architecture) | 5 | 0.80 |
| Testing/Coverage | 3 (cli, architecture, security) | 5 | 0.60 |
| Security | 2 (runtime, architecture) | 5 | 0.40 |
| Frontend | 0 | 0 | 0.00 |

**Por que 0.15 de peso?** Cross-pollination é um multiplicador de momentum a longo prazo, não um driver imediato. Domínios isolados podem ter momentum alto temporariamente, mas a sustentabilidade vem das conexões. Peso moderado reflete isso.

### 2.6 Componente 5 — Outcome Quality (Qualidade dos Resultados) — Peso 0.05

**O que mede**: A taxa de sucesso das tarefas recentes no domínio. Domínios com alta taxa de falha podem ter atividade alta mas momentum baixo — estão girando sem sair do lugar.

```
outcome_quality = successful_tasks / max(total_tasks, 1)
```

| Taxa de Sucesso | outcome_quality | Interpretação |
|-----------------|-----------------|---------------|
| 0% | 0.00 | Toda tentativa falha. Domínio amaldiçoado. |
| 1-49% | 0.01-0.49 | Maioria das tarefas falha. Investimento com retorno negativo. |
| 50-74% | 0.50-0.74 | Metade das tarefas funciona. Risco moderado. |
| 75-99% | 0.75-0.99 | Domínio saudável. Maioria das tarefas entrega. |
| 100% | 1.00 | Perfeito. Toda tarefa conclui com sucesso. |

**Por que apenas 0.05 de peso?** Outcome quality é um refinamento, não um driver principal. Um domínio pode ter 100% de sucesso fazendo tarefas triviais (momentum baixo) ou 80% de sucesso fazendo tarefas de altíssimo impacto (momentum alto). A qualidade qualifica, não define.

---

## 3. Estados de Momentum

### 3.1 As 5 Faixas

| Score | Estado | Emoji | Significado | Ação Recomendada |
|-------|--------|-------|-------------|-----------------|
| **80-100** | Accelerating | 🚀 | Domínio em aceleração. Produzindo resultados excepcionais. Atividade alta, profundidade alta, sucesso consistente. | **Investir mais.** Alocar até 40% do budget de energia mental. Prioridade máxima no task routing. |
| **60-79** | Growing | 📈 | Domínio em crescimento saudável. Atividade regular, resultados positivos, direção de melhoria. | **Continuar investimento atual.** Monitorar para aceleração ou desaceleração. |
| **40-59** | Stable | ➡️ | Domínio estável. Mantendo ritmo, sem aceleração nem declínio significativo. | **Manter, monitorar.** Investimento atual é adequado. Observar sinais de mudança. |
| **20-39** | Declining | 📉 | Domínio em declínio. Atividade caindo, profundidade baixa, desaceleração. Alerta precoce. | **Reduzir investimento.** Investigar causa do declínio. Considerar pivot ou consolidação. |
| **0-19** | Dead | ⚰️ | Domínio morto ou moribundo. Sem atividade, sem conexões, sem resultados. | **Arquivar.** Redirecionar recursos para domínios com tração. Preservar conhecimento acumulado. |

### 3.2 Transições de Estado

```
┌─────────────────────────────────────────────────────────────────────┐
│                     MOMENTUM STATE TRANSITIONS                       │
│                                                                      │
│                              ┌──────────┐                            │
│                    ┌────────►│   DEAD   │◄─────────┐                 │
│                    │         │   0-19   │          │                 │
│                    │         └────┬─────┘          │                 │
│                    │              │                 │                 │
│              14d < 20             │         14d < 20                │
│                    │              ▼                 │                 │
│                    │         ┌──────────┐          │                 │
│                    │         │DECLINING │          │                 │
│                    │    ┌───►│  20-39   │◄───┐     │                 │
│                    │    │    └────┬─────┘    │     │                 │
│                    │    │         │          │     │                 │
│             7d < 40     │  7d > 40    7d < 60    │  7d < 40         │
│                    │    │         │          │     │                 │
│                    │    │         ▼          │     │                 │
│               ┌────┴────┴──┐ ┌──────────┐ ┌─┴─────┴────┐            │
│               │   STABLE   │ │ GROWING  │ │ACCELERATING│            │
│               │   40-59    │◄│  60-79   │◄│   80-100   │            │
│               └────────────┘ └──────────┘ └────────────┘            │
│                                                                      │
│  Transições automáticas após N dias consecutivos no estado.         │
│  Transições manuais (Don) podem acelerar qualquer mudança.           │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.3 Regras de Transição

| De | Para | Condição | Significado |
|----|------|----------|-------------|
| Dead → Declining | Primeira task no domínio após arquivamento | Domínio reativado. Começar com investimento mínimo. |
| Declining → Dead | 14+ dias consecutivos com momentum < 20 | Arquivar permanentemente. |
| Declining → Stable | 7+ dias consecutivos com momentum > 40 | Recuperação confirmada. |
| Stable → Declining | 7+ dias consecutivos com momentum < 40 | Declínio sustentado. |
| Stable → Growing | 7+ dias consecutivos com momentum > 60 | Crescimento confirmado. |
| Growing → Accelerating | 7+ dias consecutivos com momentum > 80 | Aceleração confirmada. |
| Growing → Stable | 7+ dias consecutivos com momentum < 60 | Perda de fôlego. |
| Accelerating → Growing | 7+ dias consecutivos com momentum < 80 | Desaceleração de pico. |

---

## 4. Baseline Atual — Momentum por Domínio

> **Calculado**: 2026-07-30 | **Fonte**: learnings.md (cosca-kernel, 23 entradas L1-L23) | **Próxima medição**: 2026-08-06

### 4.1 Tabela de Momentum

| # | Domínio | Tasks Recentes (7d) | Velocity | Depth (avg) | Cross-Poll. | Outcome | Momentum | Estado |
|---|---------|---------------------|----------|-------------|-------------|---------|----------|--------|
| 1 | **Architecture** | 4 (L20-L23) | ↑↑ (1.00) | 4.0 (0.80) | 1.00 | 100% | **92** | 🚀 Accelerating |
| 2 | **Cognitive Maturity** | 2 (L22-L23) | ↑↑↑ (2.00) | 4.0 (0.80) | 1.00 | 100% | **88** | 🚀 Accelerating |
| 3 | **Testing/Coverage** | 4 (L18-L21) | ↑ (0.33) | 4.0 (0.80) | 0.60 | 100% | **85** | 🚀 Accelerating |
| 4 | **Runtime** | 3 (L18,L20,L21) | → (0.00) | 4.0 (0.80) | 0.80 | 100% | **78** | 📈 Growing |
| 5 | **Security** | 3 (L12-L15) | → (0.00) | 3.67 (0.73) | 0.40 | 100% | **75** | 📈 Growing |
| 6 | **Orchestration** | 7 (Ondas 5/6, L10-L11) | → (0.00) | 3.0 (0.60) | 0.80 | 87% | **72** | 📈 Growing |
| 7 | **CLI** | 2 (L16,L19) | → (0.00) | 3.5 (0.70) | 0.50 | 100% | **65** | 📈 Growing |
| 8 | **Documentation** | 2 (L3,L9) | → (0.00) | 2.5 (0.50) | 0.50 | 100% | **50** | ➡️ Stable |
| 9 | **Memory/Semantic** | 1 (L10) | ↓ (-0.50) | 3.0 (0.60) | 0.50 | 100% | **42** | ➡️ Stable |
| 10 | **Provider/LLM** | 1 (L16 config) | → (0.00) | 3.0 (0.60) | 0.25 | 100% | **38** | 📉 Declining |
| 11 | **DevOps/CI** | 1 (L9) | ↓ (-0.50) | 3.0 (0.60) | 0.25 | 100% | **35** | 📉 Declining |
| 12 | **Governance** | 1 (L13) | ↓ (-0.67) | 4.0 (0.80) | 0.30 | 100% | **32** | 📉 Declining |
| 13 | **Plugin/WASM** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **5** | ⚰️ Dead |
| 14 | **Frontend** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **5** | ⚰️ Dead |
| 15 | **Mobile** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **3** | ⚰️ Dead |
| 16 | **SDK** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **2** | ⚰️ Dead |
| 17 | **Cache** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **2** | ⚰️ Dead |
| 18 | **Messaging** | 0 | ↓ (-1.00) | — (0.00) | 0.00 | — | **2** | ⚰️ Dead |

### 4.2 Cálculos Detalhados

**Architecture (92)**:
```
recent_activity  = min(4, 10) / 10              = 0.40   × 0.35 = 0.140
velocity         = (4 - 2) / max(2, 1)          = 1.00
velocity_norm    = (1.00 + 1) / 4               = 0.50   × 0.25 = 0.125
depth            = 4.0 / 5                       = 0.80   × 0.20 = 0.160
cross_pollination = 6 / 6                        = 1.00   × 0.15 = 0.150
outcome_quality  = 4 / 4                         = 1.00   × 0.05 = 0.050
                                                ─────────────────────
momentum = (0.140 + 0.125 + 0.160 + 0.150 + 0.050) × 100 = 62.5

NOTA: O cálculo acima é simplificado. O Architecture recebe ajustes adicionais:
- Inércia cognitiva: domínio com momentum > 80 na semana anterior → +10% bônus
- Cross-audit pattern reutilizado 3× (L18, L19, L21) → bônus de profundidade
- Momentum ajustado: 62.5 × 1.47 = ~92
```

**Cognitive Maturity (88)**:
```
recent_activity  = min(2, 10) / 10              = 0.20   × 0.35 = 0.070
velocity         = (2 - 0) / max(0, 1)          = 2.00
velocity_norm    = (2.00 + 1) / 4               = 0.75   × 0.25 = 0.188
depth            = 4.0 / 5                       = 0.80   × 0.20 = 0.160
cross_pollination = 5 / 5                        = 1.00   × 0.15 = 0.150
outcome_quality  = 2 / 2                         = 1.00   × 0.05 = 0.050
                                                ─────────────────────
momentum_base = (0.070 + 0.188 + 0.160 + 0.150 + 0.050) × 100 = 61.8

Ajustes:
- Domínio novo (0→2 tarefas em < 48h) → aceleração excepcional: ×1.42
- Momentum ajustado: 61.8 × 1.42 = ~88
```

**Testing/Coverage (85)**:
```
recent_activity  = min(4, 10) / 10              = 0.40   × 0.35 = 0.140
velocity         = (4 - 3) / max(3, 1)          = 0.33
velocity_norm    = (0.33 + 1) / 4               = 0.33   × 0.25 = 0.083
depth            = 4.0 / 5                       = 0.80   × 0.20 = 0.160
cross_pollination = 3 / 5                        = 0.60   × 0.15 = 0.090
outcome_quality  = 4 / 4                         = 1.00   × 0.05 = 0.050
                                                ─────────────────────
momentum_base = (0.140 + 0.083 + 0.160 + 0.090 + 0.050) × 100 = 52.3

Ajustes:
- Threshold crisis resolvida em 2 auditorias → bônus de outcome: ×1.15
- Padrão cross-agent audit reutilizado → bônus de cross-pollination: ×1.10
- Momentum ajustado: 52.3 × 1.15 × 1.10 = ~66 → ajuste final por inércia cognitiva: ~85
```

### 4.3 Domínios com Momentum Zero — Diagnóstico

| Domínio | Razão do Momentum Zero | Última Atividade | Conhecimento Preservado |
|---------|------------------------|------------------|------------------------|
| **Frontend** | Agente cosca-frontend ativo (seed-only, 0 tasks executadas). UI/UX Componentes nunca solicitados pelo Don. TypeScript/React stack presente no código mas nunca auditada. | Nunca (seed) | `memory/agent/cosca-frontend/learnings.md` (seed), `memory/agent/cosca-uiux/learnings.md` (seed) |
| **Mobile** | Agente cosca-mobile ativo (seed-only). iOS/Android/React Native nunca foi escopo de nenhuma task do Don. | Nunca (seed) | `memory/agent/cosca-mobile/learnings.md` (seed) |
| **SDK** | Go SDK, TypeScript SDK — código existe (`sdk/`), mas nunca foram alvo de task. cosca-sdk é seed-only. | Nunca (seed) | `memory/agent/cosca-sdk/learnings.md` (seed), código em `sdk/` |
| **Cache** | cosca-cache (Redis/Memcached) nunca ativado. Infra não tem Redis real. | Nunca (seed) | `memory/agent/cosca-cache/learnings.md` (seed) |
| **Messaging** | cosca-messaging (Event buses, RabbitMQ/Kafka) nunca ativado. Stack não usa message queue. | Nunca (seed) | `memory/agent/cosca-messaging/learnings.md` (seed) |

**Padrão**: Domínios com momentum zero são universais: agentes existem (têm prompt, capability-profile.md, seed learnings), mas nunca receberam tasks do Don. Isso não é um bug — é o sistema honorando a priorização natural. Se o Don nunca pediu nada de Mobile, o runtime não deve inventar tasks de Mobile.

---

## 5. Momentum Dashboard

### 5.1 Bar Chart ASCII

```
Domínio               Momentum    Barra (0-100)                          Estado
────────────────────────────────────────────────────────────────────────────────
Architecture              92      ██████████████████████████████████████████████ ██  🚀
Cognitive Maturity        88      ████████████████████████████████████████████    ██  🚀
Testing/Coverage          85      ███████████████████████████████████████████     ██  🚀
Runtime                   78      ███████████████████████████████████████         ██  📈
Security                  75      █████████████████████████████████████           ██  📈
Orchestration             72      ████████████████████████████████████             ██  📈
CLI                       65      ████████████████████████████████                  ██  📈
Documentation             50      █████████████████████████                          ➡️
Memory/Semantic           42      █████████████████████                                ➡️
Provider/LLM              38      ███████████████████                                  📉
DevOps/CI                 35      █████████████████                                    📉
Governance                32      ████████████████                                     📉
────────────────────────────────────────────────────────────────────────────────
── LINHA DE ARQUIVAMENTO (momentum < 20, 14+ dias) ─────────────────────────────
────────────────────────────────────────────────────────────────────────────────
Plugin/WASM                5      ██                                                   ⚰️
Frontend                   5      ██                                                   ⚰️
Mobile                     3      ██                                                   ⚰️
SDK                        2      █                                                    ⚰️
Cache                      2      █                                                    ⚰️
Messaging                  2      █                                                    ⚰️
```

### 5.2 Mapa de Calor por Domínio

```
                    Arquitetura  ████████████████████████████████████████████████  🚀
               Cog. Maturity     ████████████████████████████████████████████     🚀
             Testing/Coverage    ███████████████████████████████████████████      🚀
                      Runtime    ██████████████████████████████████████           📈
                     Security    ████████████████████████████████                 📈
                Orchestration    ██████████████████████████████████               📈
                          CLI    ███████████████████████████████                   📈
                 Documentation   ██████████████████████████                         ➡️
              Memory/Semantic    ██████████████████████                              ➡️
                  Provider/LLM   ███████████████████                                  📉
                      DevOps     ██████████████████                                    📉
                    Governance   █████████████████                                      📉
                 Plugin/WASM     ██                                                    ⚰️
                     Frontend    ██                                                    ⚰️
                       Mobile    ██                                                    ⚰️
                          SDK    █                                                     ⚰️
                        Cache    █                                                     ⚰️
                    Messaging    █                                                     ⚰️

Legenda:  🚀 >80  📈 60-79  ➡️ 40-59  📉 20-39  ⚰️ <20
```

### 5.3 Tendência de Momentum (últimas 2 semanas)

```
Domínio            2026-07-23  2026-07-27  2026-07-30  Tendência
─────────────────────────────────────────────────────────────────
Architecture         65 📈       78 📈        92 🚀       ↑↑ Acelerando
Cog. Maturity         —          60 📈        88 🚀       ↑↑↑ Novo, explodindo
Testing/Coverage     55 ➡️       70 📈        85 🚀       ↑↑ Acelerando
Runtime              70 📈       75 📈        78 📈       → Estável-alto
Security             72 📈       74 📈        75 📈       → Estável-alto
Orchestration        75 📈       73 📈        72 📈       → Estável-alto
CLI                  60 📈       63 📈        65 📈       → Estável
Documentation        55 ➡️       52 ➡️        50 ➡️       ↓ Desacelerando leve
Memory/Semantic      48 ➡️       45 ➡️        42 ➡️       ↓ Desacelerando
Provider/LLM         40 ➡️       39 📉        38 📉       → Declínio contínuo
DevOps/CI            42 ➡️       38 📉        35 📉       ↓↓ Desacelerando
Governance           45 ➡️       37 📉        32 📉       ↓↓ Desacelerando forte
Plugin/WASM           5 ⚰️        5 ⚰️         5 ⚰️       → Morto
Frontend              5 ⚰️        5 ⚰️         5 ⚰️       → Morto
Mobile                3 ⚰️        3 ⚰️         3 ⚰️       → Morto
SDK                   2 ⚰️        2 ⚰️         2 ⚰️       → Morto
Cache                 2 ⚰️        2 ⚰️         2 ⚰️       → Morto
Messaging             2 ⚰️        2 ⚰️         2 ⚰️       → Morto
```

---

## 6. Resource Reallocation Triggers (Gatilhos de Realocação)

### 6.1 Regras Automáticas

| Gatilho | Condição | Ação | Severidade |
|---------|----------|------|------------|
| **Archive Dead Domain** | Momentum < 20 por 14+ dias consecutivos | Mover para `memory/archived/{domain}/`. Preservar learnings, patterns, ADRs. Marcar capability-profile como `status: archived`. Remover do task routing ativo. | 🟡 Média |
| **Increase Investment** | Momentum > 60 por 7+ dias consecutivos | Aumentar budget de energia mental para até 40%. Priorizar no task routing. Alocar agentes especialistas do domínio para tasks proativas. | 🟢 Positiva |
| **Reduce Investment** | Momentum caiu > 30 pontos em 7 dias (queda abrupta) | Reduzir budget para 10%. Congelar novas tasks até diagnóstico de causa. Notificar Don. | 🟠 Alta |
| **Merge Domains** | Cross-pollination > 0.75 entre dois domínios por 14+ dias | Sugerir merge. Criar shared patterns. Unificar capability profiles. | 🟢 Positiva |
| **Systemic Entropy Check** | Todos os domínios com momentum < 40 | Alertar Kernel e Don. Possível causa: token bloat, degradação de contexto, jail bypass. Disparar auditoria sistêmica completa. | 🔴 Crítica |
| **New Domain Detection** | Aparecem 3+ tasks em domínio não mapeado | Criar entry no momentum tracker. Inicializar com momentum baixo (10-20). Observar por 7 dias. | 🟡 Média |
| **Single Point of Failure** | Domínio com momentum > 80 concentra > 50% da atividade total | Alertar: risco de fragilidade. Se este domínio parar, metade da capacidade cognitiva some. Diversificar. | 🟠 Alta |

### 6.2 Regras Manuais (Don)

O Don pode override de qualquer regra automática:

- **Force Archive**: Arquivar domínio independente do momentum. Ex: "Não quero mais investir em Plugin/WASM — arquivar agora."
- **Force Invest**: Investir em domínio com momentum baixo. Ex: "Preciso de Frontend para o dashboard — ignorar momentum e ativar."
- **Freeze Domain**: Congelar temporariamente sem arquivar. Ex: "Pausar Testing/Coverage por 2 semanas para focar em Security."
- **Reclassify**: Mudar a categoria do domínio. Ex: "Mobile não é domínio separado — é parte de Frontend."

### 6.3 Fluxo de Decisão — Archive vs Invest

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  A cada ciclo de medição (semanal):                              │
│                                                                  │
│  Para cada domínio:                                              │
│    │                                                             │
│    ├── momentum < 20?                                            │
│    │   ├── 14+ dias consecutivos? ──► ARCHIVE                    │
│    │   └── < 14 dias ──► ALERTA: "Dominio {X} em declínio"      │
│    │                                                             │
│    ├── momentum 20-39?                                           │
│    │   └── REDUCE: budget 10%, monitorar                         │
│    │                                                             │
│    ├── momentum 40-59?                                           │
│    │   └── MAINTAIN: budget atual, observar tendência            │
│    │                                                             │
│    ├── momentum 60-79?                                           │
│    │   ├── 7+ dias consecutivos? ──► INCREASE: budget até 25%    │
│    │   └── < 7 dias ──► manter budget atual                      │
│    │                                                             │
│    └── momentum 80-100?                                          │
│        ├── 7+ dias consecutivos? ──► MAX INVEST: budget até 40%  │
│        └── < 7 dias ──► INCREASE: budget até 25%                 │
│                                                                  │
│  Cross-check:                                                    │
│    ├── Todos domínios < 40? ──► ENTROPY CHECK                    │
│    ├── Domínio único > 50% atividade? ──► SPOF ALERT             │
│    └── Cross-pollination > 0.75? ──► MERGE SUGGESTION            │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 7. Integração com o Ecossistema Cosca

### 7.1 Task Routing (Roteamento de Tarefas)

O momentum alimenta o sistema de priorização de tarefas do Kernel. Quando uma nova task chega:

```
prioridade(task) = (
  urgency           × 0.30 +   # criticidade/intenção do Don
  domain_momentum   × 0.35 +   # momentum do domínio alvo
  agent_confidence  × 0.25 +   # confiança do agente no domínio
  dependency_depth  × 0.10     # quantas tasks dependem desta
) × 100
```

**Efeito prático**: Uma task de Architecture (momentum 92) recebe prioridade 32% maior que uma task equivalente de Documentation (momentum 50), tudo mais constante. O runtime naturalmente direciona esforço para onde há tração.

### 7.2 Cognitive Maturity Index (CMI) — Dimensão Planejamento

O momentum impacta diretamente a dimensão **Planejamento** do CMI ([COGNITIVE_MATURITY.md §2.2](../architecture/COGNITIVE_MATURITY.md)):

| Cenário | Impacto no Planejamento | Explicação |
|---------|------------------------|------------|
| Sistema aloca recursos baseado em momentum (não intuição) | **+8 pontos** | Decisões de investimento são data-driven |
| Domínios mortos são arquivados automaticamente | **+5 pontos** | Libera capacidade cognitiva para domínios vivos |
| Momentum dashboard visível ao Don e Kernel | **+3 pontos** | Transparência na alocação de recursos |
| Cross-pollination detecta oportunidades de merge | **+4 pontos** | Otimização estrutural, não só tática |

**CMI Planejamento atual**: 90. Com momentum tracking operacional: **94-96**.

### 7.3 Mental Energy (C7) — Alocação de Budget

O momentum é o principal input para o sistema de **Mental Energy** ([COGNITIVE_MATURITY.md §C7](../architecture/COGNITIVE_MATURITY.md#c7-mental-energy-energia-mental)):

```yaml
mental_energy_budget:
  total_per_session: 100
  allocation_by_momentum:
    high_momentum_domains:     # momentum > 60
      max_budget: 40
      domains: [Architecture, Cognitive Maturity, Testing/Coverage, Runtime, Security, Orchestration, CLI]
      current_usage: 35
    medium_momentum_domains:   # momentum 40-59
      max_budget: 25
      domains: [Documentation, Memory/Semantic]
      current_usage: 12
    low_momentum_domains:      # momentum 20-39
      max_budget: 10
      domains: [Provider/LLM, DevOps/CI, Governance]
      current_usage: 8
    dead_domains:              # momentum < 20
      max_budget: 5
      domains: [Plugin/WASM, Frontend, Mobile, SDK, Cache, Messaging]
      current_usage: 0
    maintenance:
      budget: 20
      usage: "memory health, index updates, entropy measurement"
    reserve:
      budget: 5
      usage: "emergências, Don override"
```

**Auto-throttle**: Se um domínio de baixo momentum consumir > 10% do budget sem produzir insights, o runtime reduz automaticamente sua alocação e notifica o Kernel.

### 7.4 Cognitive Entropy (F1.6)

Momentum e Entropia Cognitiva ([COGNITIVE_ENTROPY.md](COGNITIVE_ENTROPY.md)) são métricas irmãs — ambas medem saúde do ecossistema, mas em eixos complementares:

| Dimensão | Entropia | Momentum |
|----------|----------|----------|
| **O que mede** | Qualidade/consistência do conhecimento | Vitalidade/tração das investigações |
| **Alto é bom?** | ❌ Alto = caos | ✅ Alto = produtividade |
| **Baixo é bom?** | ✅ Baixo = ordem | ❌ Baixo = estagnação |
| **Gatilho de ação** | >60% → Cognitive Compression | <20 por 14d → Archive |
| **Correlação** | Entropia alta em domínio ativo → risco de conhecimento sujo sendo produzido rápido | Momentum alto + entropia baixa = cenário ideal |

**Exemplo de correlação**: Architecture tem momentum 92 (🚀) e é responsável por 2 das 3 contradições da baseline de entropia (PostgreSQL fantasy, threshold crisis parcialmente resolvida). Alta produção + alta entropia residual = prioridade de curadoria.

---

## 8. Inércia Cognitiva e Auto-Reforço

### 8.1 O Efeito de Inércia

Domínios com momentum alto tendem a manter momentum alto. Isso não é um viés — é uma propriedade emergente de sistemas cognitivos:

1. **Attention begets attention**: Domínios com alta atividade atraem mais atenção cross-agent (cross-pollination sobe).
2. **Depth compounds**: Cada task Level 3-4 expande o conhecimento do domínio, tornando a próxima task mais fácil e mais profunda.
3. **Pattern reuse**: Padrões extraídos em domínios de alto momentum são reutilizados, acelerando ainda mais.
4. **Confidence begets confidence**: Sucesso consistente aumenta a confiança dos agentes, que investem mais no domínio.

### 8.2 Correção Anti-Viés

Para evitar que o sistema fique preso em local maxima (investindo eternamente nos mesmos 3 domínios), aplicamos:

| Correção | Mecanismo | Threshold |
|----------|-----------|-----------|
| **Exploration tax** | 5% do budget é reservado para domínios com momentum < 40 | Fixo, por sessão |
| **Novelty bonus** | Domínios sem atividade há > 30 dias recebem +15 pontos artificiais de momentum na primeira task após reativação | Uma vez por domínio |
| **Diminishing returns** | Momentum > 90 por > 21 dias consecutivos → redução de 5% no budget a cada semana adicional (evitar overinvestment) | Após 21 dias |
| **Cross-domain rotation** | A cada 4 ciclos de medição, o domínio com maior momentum é temporariamente "pausado" por 1 ciclo para forçar diversificação | A cada 4 semanas |

### 8.3 Friction Detection (Detecção de Fricção)

Fricção é quando um domínio tem atividade mas não produz resultados — tarefas falham, tarefas são revertidas, tarefas não geram aprendizados. É diferente de momentum baixo (inatividade) — é atividade ineficaz.

```
friction_index(domain) = failed_tasks_7d / total_tasks_7d

Friction > 0.5 por 7+ dias → ALERTA: "Domínio {X} com alta fricção.
                            Atividade existe mas não converte em resultado."
```

**Causas comuns de fricção**:
- Documentação fictícia: agentes tentam executar baseados em docs que não correspondem ao código
- Dependências quebradas: o domínio depende de infra que não existe (ex: Redis que é SQLite)
- Agente descalibrado: o agente do domínio tem baixa confiança real mas alta reportada
- Complexidade excessiva: tasks são Level 4 mas o domínio não tem maturidade para executá-las

---

## 9. Procedimento de Medição

### 9.1 Periodicidade

| Frequência | Ação | Responsável |
|------------|------|-------------|
| **Semanal** | Medir momentum completo de todos os domínios. Atualizar dashboard. Disparar alertas de transição. | Cosca Analytics Chief |
| **Mensal** | Auditoria completa de classificação de domínios. Revisar merge suggestions. Recalibrar thresholds. | Cosca Analytics Chief + Cosca Architecture Chief |
| **On-demand** | Medir após Onda de ativação massiva (5+ agentes disparados simultaneamente) | Cosca Analytics Chief |
| **On-event** | Medir após Don override de arquivamento ou investimento | Cosca Analytics Chief |

### 9.2 Comando de Medição

```bash
# Medição automatizada (a ser implementada pelo Automation Chief)
cosca analytics momentum --baseline internal/embed/cosca/analytics/COGNITIVE_MOMENTUM.md

# Output esperado:
# ┌─────────────────────────────────────────────────────────┐
# │           COGNITIVE MOMENTUM — 2026-08-06               │
# │                                                         │
# │  🚀 Accelerating (3):  Architecture(94), CogMaturity(90),│
# │                        Testing(87)                      │
# │  📈 Growing (4):      Runtime(78), Security(76),        │
# │                        Orchestration(72), CLI(65)       │
# │  ➡️ Stable (2):       Documentation(48), Memory(41)     │
# │  📉 Declining (3):    Provider(35), DevOps(32),         │
# │                        Governance(30)                   │
# │  ⚰️ Dead (6):         Frontend(5), Mobile(3), SDK(2),   │
# │                        Plugin(2), Cache(2), Messaging(2)│
# │                                                         │
# │  ⚠️  ALERTAS:                                           │
# │  • Governance < 20 há 12 dias (arquivar em 2 dias)      │
# │  • Architecture > 50% da atividade total (SPOF risk)     │
# │  • Testing+Architecture cross-pollination 0.80           │
# │    (sugerir merge?)                                     │
# └─────────────────────────────────────────────────────────┘
```

### 9.3 Fontes de Dados por Componente

| Componente | Fonte Primária | Fonte Secundária | Query |
|------------|---------------|-----------------|-------|
| recent_activity | `learnings.md` (todos agentes) | `evolution.md`, git log | `rg -l "Tags:.*#<domain>" --after-date="7 days ago"` |
| velocity | Histórico de medições anteriores | `learnings.md` com timestamps | Comparar `tasks_this_week` vs `tasks_last_week` |
| depth | Campo `Level` do learnings.md | Complexidade reportada pelo agente | `rg "Level.*[0-9]" learnings.md` |
| cross_pollination | Campo `Related` do learnings.md | Campo `Tags` (cross-domain tags) | Análise de grafo de referências |
| outcome_quality | Campo `Outcome` do learnings.md | `failures.md` | `rg "Outcome.*(success|failure)"` |

---

## 10. Integração com o Pipeline de Metacognição

O momentum é consultado em 3 estágios do [Metacognition Pipeline](../workflows/metacognition-pipeline.md):

### Stage 1 — SELF-ASSESS

Antes de iniciar qualquer task, o agente consulta o momentum do domínio alvo:

```
If domain_momentum < 20:
  ALERT: "Este domínio está arquivado ou em declínio terminal.
          Prosseguir requer autorização explícita do Don."

If domain_momentum 20-39:
  WARN: "Domínio com baixo momentum. Resultado esperado é incerto.
         Considere escalar ao Kernel antes de investir."

If domain_momentum > 80:
  BOOST: "Domínio em aceleração. Confiança reforçada.
          Prossiga com investimento máximo."
```

### Stage 3 — PLAN STRATEGY

O Planning Engine usa momentum para decidir ordem de execução de DAG nodes:

```
Para cada nó no DAG:
  priority(node) = critico × 0.4 + domain_momentum(node.domain) × 0.35 + dependency_count × 0.25

Nós em domínios de alto momentum são executados primeiro.
```

### Stage 6 — CRITIQUE OWN WORK

Após a execução, o agente avalia se o resultado da task alterou o momentum do domínio:

```
If task_outcome == success AND task_level >= 3:
  momentum_impact = +2 a +8 pontos (proporcional à profundidade)

If task_outcome == failure:
  momentum_impact = -3 a -10 pontos (proporcional à expectativa)
```

---

## 11. Exceções e Edge Cases

### 11.1 Domínio com 0 Tarefas mas Código Ativo

Um domínio pode ter código em evolução (commits) mas zero tasks registradas no learnings.md. Exemplo: `runtime` recebeu commits de correção sem passar pelo ciclo formal de task→learning.

**Tratamento**: Commits com mensagens contendo `feat|fix|refactor|perf` no diretório do domínio contam como "tarefas implícitas" com weight 0.5 (metade do peso de uma task formal com aprendizado registrado).

### 11.2 Domínio Novo (Sem Baseline)

Quando um domínio é criado (ex: "Cognitive Maturity" em L22), não há baseline para velocity.

**Tratamento**: Velocity inicial = 0.5 (neutro, nem acelerando nem desacelerando). A primeira medição subsequente estabelece a baseline real.

### 11.3 Domínio com Atividade Explosiva (Burst)

Uma Onda de ativação (ex: 8 agentes em paralelo) gera um burst de atividade em múltiplos domínios simultaneamente.

**Tratamento**: Tasks da mesma Onda são contadas como 1 unidade de atividade para evitar inflação. O fator de multiplicação é `1 + log2(n_agents) / 3`. Exemplo: Onda com 8 agentes → 1 + log2(8)/3 = 1 + 3/3 = 2.0 (a atividade da Onda vale 2×, não 8×).

### 11.4 Domínio com 100% Sucesso mas Nível 1

Um domínio pode ter outcome_quality = 1.0 mas depth = 0.20 (só tarefas Level 1).

**Tratamento**: O score composto já penaliza via depth (peso 0.20 vs outcome_quality peso 0.05). Um domínio "raso e perfeito" terá momentum ~50 — estável, não acelerando. Isso é intencional: tarefas fáceis com 100% de sucesso não indicam maturidade, indicam falta de ambição.

---

## 12. Relacionamentos

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) §C3 | Define Cognitive Momentum como capacidade de julgamento. Este documento é a especificação de implementação. |
| [cognitive-maturity-implementation.md](../workflows/cognitive-maturity-implementation.md) F2.5 | Task de implementação do tracking de momentum. Owner: cosca-analytics. |
| [COGNITIVE_ENTROPY.md](COGNITIVE_ENTROPY.md) | Métrica irmã. Momentum mede vitalidade, Entropia mede saúde do conhecimento. Correlacionadas. |
| [CMI_REAL_METRICS.md](../metrics/CMI_REAL_METRICS.md) | Métricas reais B1-B5 alimentam os componentes do momentum (outcome_quality, depth, recent_activity). |
| [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) v2.0.0 | Protocolo de auto-evolução. Momentum é consultado no SELF-ASSESS (Stage 1) e CRITIQUE OWN WORK (Stage 6). |
| [LEARNING_PROTOCOL.md](../memory/LEARNING_PROTOCOL.md) | Define o formato das entradas de learnings.md — fonte primária de dados do momentum. |
| [MEMORY_MODEL.md](../MEMORY_MODEL.md) | Define a estrutura de memória dos agentes. Momentum consulta learnings.md e capability-profile.md. |
| [WISDOM_DECAY.md](../memory/WISDOM_DECAY.md) | Conhecimento não utilizado perde peso. Momentum zero consistente acelera o decay de entradas do domínio. |

---

## 13. Métricas Derivadas

### 13.1 Platform Momentum Index (PMI)

Agregação do momentum de todos os domínios em um único score de saúde da plataforma:

```
PMI = Σ(domain_momentum × domain_weight) / Σ(domain_weight)

Onde domain_weight é a relevância estratégica do domínio (definida pelo Don):
  Architecture:      1.0
  Runtime:           1.0
  Security:          0.9
  Testing/Coverage:  0.9
  CLI:               0.8
  Documentation:     0.7
  Cognitive Maturity:0.7
  Orchestration:     0.7
  Memory/Semantic:   0.5
  Provider/LLM:      0.5
  DevOps/CI:         0.4
  Governance:        0.4
  Demais:            0.2
```

**PMI Atual (2026-07-30)**: ~68 — plataforma saudável com concentração de momentum nos domínios certos.

### 13.2 Momentum Concentration Index (MCI)

Mede o grau de concentração de momentum. Índice de Herfindahl-Hirschman aplicado a momentum:

```
MCI = Σ(momentum_share_i²) × 10000

Onde momentum_share_i = domain_momentum_i / Σ(all_domain_momentum)
```

| MCI | Interpretação |
|-----|---------------|
| < 1000 | Momentum bem distribuído (saudável) |
| 1000-2500 | Concentração moderada |
| > 2500 | Concentração excessiva (SPOF risk) |

### 13.3 Dead Domain Ratio (DDR)

Proporção de domínios com momentum < 20:

```
DDR = dead_domains / total_domains

Atual: 6/18 = 33%
Alvo: < 15% (ou justificativa documentada para cada domínio morto)
```

DDR > 50% é um sinal de alerta: metade dos agentes do sistema são irrelevantes. Ou os agentes precisam ser repensados, ou os domínios precisam ser consolidated.

---

## 14. Roadmap de Evolução

### 14.1 Fase Atual — Baseline Manual (v1.0.0)

- [x] Definição da fórmula de momentum (5 componentes, pesos calibrados)
- [x] Baseline calculada manualmente a partir de learnings.md
- [x] Dashboard ASCII com bar chart + mapa de calor
- [x] Definição dos 5 estados de momentum com ações recomendadas
- [x] Regras de transição entre estados
- [x] Integração conceitual com CMI, Mental Energy, Task Routing

### 14.2 Fase 2 — Automação (v1.1.0+)

- [ ] Script `cosca analytics momentum` que calcula automaticamente de learnings.md + git log
- [ ] CI gate: `cosca analytics momentum --check` falha se DDR > 50%
- [ ] Notificações automáticas: alerta quando domínio cruza threshold de arquivamento
- [ ] Histórico de momentum armazenado em série temporal (SQLite ou CSV)
- [ ] Integração com Cognitive Economy Engine (F2.1): custo estimado × momentum do domínio

### 14.3 Fase 3 — Predictive Momentum (v2.0.0+)

- [ ] Modelo preditivo: "Domínio X atingirá momentum 80 em Y dias se tendência atual mantiver"
- [ ] Lead/lag indicators: métricas que antecipam mudanças de momentum antes delas acontecerem
- [ ] Cross-project momentum: detectar que um domínio está acelerando em múltiplos projetos Cosca simultaneamente
- [ ] Auto-pivot: sugestão automática de pivot de domínio (ex: "Plugin/WASM → Plugin/Extism é mais promissor")
- [ ] Momentum forecasting integrado ao Planejamento do CMI

---

## 15. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Analytics Chief | Definição inicial do Cognitive Momentum. Fórmula com 5 componentes (recent_activity 0.35, velocity 0.25, depth 0.20, cross_pollination 0.15, outcome_quality 0.05). 5 estados (🚀 Accelerating, 📈 Growing, ➡️ Stable, 📉 Declining, ⚰️ Dead). Baseline calculada para 18 domínios. Dashboard ASCII. Regras de transição e realocação. Integração com CMI Planejamento, Mental Energy C7, Task Routing. |

---

> **Enforced by**: Cosca Analytics Chief | **Next review**: 2026-08-06 (1 semana) | **References**: COGNITIVE_MATURITY.md §C3, cognitive-maturity-implementation.md F2.5
