# PLATFORM HEALTH DASHBOARD — Auto-Medicao Cosca

> **Version**: 2.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28 | **Updated**: 2026-07-28
>
> **v2.0.0**: Intelligence Score adicionado — 5 dimensões de inteligência com score composto.
> **Constitutional authority**: [CONSTITUTION.md](../CONSTITUTION.md) — Implements G2 (Transparência total) and the principle of continuous self-measurement.
> **Depends on**: Capability profiles for all 51 agents, metacognition pipeline metrics.

---

## Purpose

The Platform Health Dashboard answers the questions every autonomous system must answer about itself:

- Estou evoluindo ou estagnando?
- Estou ficando mais rápida ou mais lenta?
- Estou gastando menos ou mais tokens?
- Estou produzindo menos ou mais bugs?
- Quanto tempo economizei comparado ao fluxo manual?
- Qual agente mais agregou valor esta semana?
- O que piorou na última semana?

Without these answers, the platform operates blindly. With them, the Kernel can make data-driven decisions about agent training, resource allocation, and framework evolution.

---

## Métricas-Chave (5 Essenciais)

### M1 — Taxa de Sucesso Global

```
SuccessRate = successful_tasks / total_tasks_executed
            measured per: week, month, quarter
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Excelente | ≥ 90% | Plataforma altamente confiável |
| 🟡 Bom | 80–89% | Performance aceitável, espaço para melhoria |
| 🟠 Atenção | 70–79% | Taxa de falha acima do aceitável |
| 🔴 Crítico | < 70% | Problema sistêmico — revisão urgente |

**Tendência esperada:** Subindo ao longo do tempo (agentes evoluem → menos falhas).

---

### M2 — Eficiência de Tokens

```
TokensPerTask = total_tokens_consumed / total_tasks_executed
              goal: -5% per month (mesmo resultado, menos tokens)
```

| Nível | Tendência Mensal | Significado |
|-------|-----------------|-------------|
| 🟢 | -5% ou mais | Plataforma ficando mais eficiente |
| 🟡 | 0% a -5% | Melhoria marginal |
| 🟠 | 0% a +5% | Piorando — investigar |
| 🔴 | +5% ou mais | Desperdício de tokens — revisão urgente |

---

### M3 — Taxa de Bugs

```
BugRate = bugs_introduced / total_commits
        target: < 0.05 (1 bug a cada 20 commits)
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 | < 0.05 | Qualidade alta |
| 🟡 | 0.05–0.10 | Aceitável |
| 🟠 | 0.10–0.20 | Qualidade preocupante |
| 🔴 | > 0.20 | Qualidade inaceitável |

---

### M4 — Tempo Economizado

```
TimeSaved = (estimated_manual_time - actual_automated_time) / estimated_manual_time
          target: > 60% de redução vs estimativa manual
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 | > 60% | Automação altamente eficaz |
| 🟡 | 40–60% | Boa economia |
| 🟠 | 20–40% | Automação marginal |
| 🔴 | < 20% | Automação não está valendo a pena |

---

### M5 — Valor por Agente

```
AgentValue = (tasks_completed × success_rate × avg_task_complexity) / tokens_consumed
           ranked: top 3 and bottom 3 identified weekly
```

| Nível | Ação |
|-------|------|
| 🟢 Top 3 | Reconhecer, estudar padrões de sucesso, usar como referência |
| 🟡 Médio | Operação normal |
| 🔴 Bottom 3 | Investigar: precisa de treinamento? capability profile desatualizado? domínio errado? |

---

## Métricas Secundárias (10 de Apoio)

| # | Métrica | Pergunta que responde | Frequência |
|---|---------|----------------------|------------|
| **M6** | Tempo médio de task | Estamos ficando mais rápidos? | Semanal |
| **M7** | Taxa de acerto na escolha de agente | Kernel está roteando bem? | Semanal |
| **M8** | Memórias recuperadas com sucesso | Memória está ajudando? (recuperou algo útil vs irrelevante) | Por ciclo de curadoria |
| **M9** | Taxa de level-up | Agentes estão evoluindo? (% que subiu de nível no mês) | Mensal |
| **M10** | Confiança média dos agentes | Plataforma está mais confiável? (média dos confidence scores) | Semanal |
| **M11** | Cobertura de testes | Qualidade do código está subindo? | Semanal |
| **M12** | Tokens gastos em correção vs construção | Quanto retrabalho? (tokens em bug-fix / tokens em feature-dev) | Semanal |
| **M13** | Entradas de memória curadas | Curadoria está funcionando? | Por ciclo |
| **M14** | Cross-agent learnings aplicados | Conhecimento está fluindo entre agentes? | Semanal |
| **M15** | Latência até primeira resposta | Tempo entre task assignment e primeiro output | Semanal |

---

## Cosca Intelligence Score (CIS) — v2.0.0

> **"Não para marketing, mas para saber se realmente está ficando melhor."** — Don

O CIS mede a inteligência real da plataforma — não o que ela promete, mas o que ela entrega. 5 dimensões compõem um score único de 0–100.

---

### Dimensão 1 — Autonomia (weight: 0.25)

**Pergunta:** A plataforma resolve sem intervenção humana?

```
Autonomy = tasks_completed_without_human_intervention / total_tasks

Onde "sem intervenção humana" significa:
  - Kernel roteou para o agente correto
  - Agente executou sem escalar para humano
  - Resultado foi aceito sem correção manual

Medido: mensal
Target: > 80%
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Autônomo | ≥ 80% | Plataforma resolve 4 de 5 tasks sozinha |
| 🟡 Semi-autônomo | 60–79% | Ainda precisa de supervisão frequente |
| 🟠 Dependente | 40–59% | Mais da metade das tasks requerem intervenção |
| 🔴 Manual | < 40% | Plataforma é mais assistente que autônoma |

---

### Dimensão 2 — Precisão (weight: 0.20)

**Pergunta:** Quando age, acerta?

```
Precision = total_successful_first_attempts / total_first_attempts

Diferente de M1 (SuccessRate), Precision mede apenas PRIMEIRA TENTATIVA.
Retries contam como falha de precisão (a plataforma não acertou de primeira).

Medido: semanal
Target: > 85%
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Preciso | ≥ 85% | Acerta de primeira na maioria das vezes |
| 🟡 Confiável | 70–84% | Precisa de retry ocasional |
| 🟠 Impreciso | 50–69% | Metade das tasks precisam de segunda tentativa |
| 🔴 Errático | < 50% | Mais erra que acerta — revisão urgente |

---

### Dimensão 3 — Auto-correção (weight: 0.20)

**Pergunta:** Quando erra, percebe e corrige sozinha?

```
SelfCorrection = failures_self_detected_and_fixed / total_failures

"Self-detected" = agente identificou o erro ANTES do humano reportar
"Self-fixed" = agente corrigiu sem instrução externa

Isso mede a eficácia do Metacognition Pipeline (SELF-ASSESS → CORRECT → RE-EXECUTE).

Medido: mensal
Target: > 70%
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Autocorretivo | ≥ 70% | Detecta e corrige a maioria dos próprios erros |
| 🟡 Consciente | 50–69% | Detecta erros mas nem sempre corrige |
| 🟠 Cego | 30–49% | Raramente percebe os próprios erros |
| 🔴 Inconsciente | < 30% | Não tem noção dos próprios erros |

---

### Dimensão 4 — Reutilização (weight: 0.20)

**Pergunta:** O conhecimento flui entre agentes ou fica em silos?

```
Reuse = cross_agent_learnings_applied / total_learnings_applied

"Cross-agent" = um aprendizado do agente A foi usado pelo agente B
Isso mede se a plataforma é uma rede de conhecimento ou agentes isolados.

Complementa M14 mas mede TAXA, não volume absoluto.

Medido: mensal
Target: > 60%
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Rede | ≥ 60% | Conhecimento flui livremente entre agentes |
| 🟡 Colaborativo | 40–59% | Compartilha mas ainda tem silos |
| 🟠 Isolado | 20–39% | Agentes operam majoritariamente isolados |
| 🔴 Silo | < 20% | Cada agente reinventa a roda |

---

### Dimensão 5 — Evolução (weight: 0.15)

**Pergunta:** A plataforma está ficando melhor com o tempo?

```
Evolution = (agents_that_leveled_up - agents_that_regressed) / total_active_agents

"Leveled up" = agente subiu de nível no período (mês)
"Regressed" = agente com confidence caindo 2 medições consecutivas

Involução subtrai da taxa. Plataforma pode ter evolução negativa.

Medido: mensal
Target: > 20% dos agentes ativos subindo/mês (líquido)
```

| Nível | Valor | Significado |
|-------|-------|-------------|
| 🟢 Evoluindo | ≥ 20% | 1 em 5 agentes melhora a cada mês |
| 🟡 Crescendo | 10–19% | Evolução presente mas lenta |
| 🟠 Estagnado | 1–9% | Quase nenhum agente melhora |
| 🔴 Regredindo | ≤ 0% | Plataforma piorando — revisão urgente |

---

### Score Composto (CIS)

```
CIS = (Autonomy × 0.25) + (Precision × 0.20) + (SelfCorrection × 0.20) + (Reuse × 0.20) + (Evolution × 0.15)

Todos os componentes em escala 0–100.
Pesos: Autonomy lidera (é o objetivo final), Evolution fecha (lagging indicator).
```

| CIS | Classificação | Significado |
|-----|--------------|-------------|
| **≥ 85** | 🧠 Genuína | Plataforma demonstra inteligência real e mensurável |
| **70–84** | 📈 Competente | Bom nível de inteligência, espaço para refinar |
| **50–69** | 🔧 Em desenvolvimento | Funciona, mas depende muito de supervisão |
| **30–49** | 🐣 Iniciante | Primeiros passos — muita intervenção necessária |
| **< 30** | 📋 Script | Mais automation que intelligence |

---

### Exemplo com as Metas do Don

```
COSCA INTELLIGENCE SCORE — Target (Don's vision)

Autonomia:       87%  × 0.25 = 21.75
Precisão:        94%  × 0.20 = 18.80
Auto-correção:   91%  × 0.20 = 18.20
Reutilização:    78%  × 0.20 = 15.60
Evolução:        83%  × 0.15 = 12.45
─────────────────────────────────
CIS: 86.80 → 🧠 Genuína
```

### Baseline Atual (2026-07-28, estimado)

```
COSCA INTELLIGENCE SCORE — Baseline

Autonomia:       ~40%  × 0.25 = 10.00  (41/51 agentes nunca executaram)
Precisão:        ~90%  × 0.20 = 18.00  (tasks executadas tiveram alta precisão)
Auto-correção:   ~50%  × 0.20 = 10.00  (metacognition pipeline ativo, sem dados reais)
Reutilização:    ~15%  × 0.20 =  3.00  (cross-agent learning ainda inicial)
Evolução:        ~12%  × 0.15 =  1.80  (6 agentes evoluíram de 51)
─────────────────────────────────
CIS: 42.80 → 🔧 Em desenvolvimento
```

---

### CIS no Dashboard Semanal

O CIS é reportado no Resumo Executivo:

```markdown
| Cosca Intelligence Score | {CIS} | {delta} | {delta} | ≥ 70 | {emoji} |
| ├─ Autonomia (I1)        | {value}% | {delta} | {delta} | ≥ 80% | {emoji} |
| ├─ Precisão (I2)         | {value}% | {delta} | {delta} | ≥ 85% | {emoji} |
| ├─ Auto-correção (I3)    | {value}% | {delta} | {delta} | ≥ 70% | {emoji} |
| ├─ Reutilização (I4)     | {value}% | {delta} | {delta} | ≥ 60% | {emoji} |
| └─ Evolução (I5)         | {value}% | {delta} | {delta} | ≥ 20% | {emoji} |
```

---

### Alertas de Inteligência

| Condição | Alerta | Severidade |
|----------|--------|-----------|
| CIS cai por 2 meses consecutivos | "Plataforma regredindo em inteligência" | 🔴 |
| Autonomy < 40% por 3 meses | "Dependência humana excessiva" | 🟠 |
| SelfCorrection < 30% | "Metacognition pipeline ineficaz" | 🟠 |
| Reuse < 20% | "Conhecimento em silos — agentes não compartilham" | 🟡 |
| Evolution ≤ 0% (com involução) | "Plataforma piorando — mais agentes regredindo que evoluindo" | 🔴 |
| Precision caindo 3 semanas consecutivas | "Precisão em queda livre" | 🟠 |

---

## Dashboard Semanal

### Template

```markdown
# Cosca Platform Health — Semana {N}, {Ano}

> Gerado: {timestamp} | Ciclo: {cycle_id} | Agentes ativos: {count}/51

---

## 📊 RESUMO EXECUTIVO

| Métrica | Atual | vs Semana Anterior | vs Mês Anterior | Meta | Status |
|---------|-------|-------------------|-----------------|------|--------|
| Taxa de Sucesso (M1) | {value}% | {delta} | {delta} | ≥ 90% | {emoji} |
| Tokens/Task (M2) | {value} | {delta} | {delta} | -5%/mês | {emoji} |
| Bugs/Commit (M3) | {value} | {delta} | {delta} | < 0.05 | {emoji} |
| Tempo Economizado (M4) | {value}% | {delta} | {delta} | > 60% | {emoji} |
| Confiança Média (M10) | {value} | {delta} | {delta} | > 0.70 | {emoji} |
| **Cosca Intelligence Score** | **{CIS}** | **{delta}** | **{delta}** | **≥ 70** | **{emoji}** |
| ├─ Autonomia | {value}% | {delta} | {delta} | ≥ 80% | {emoji} |
| ├─ Precisão | {value}% | {delta} | {delta} | ≥ 85% | {emoji} |
| ├─ Auto-correção | {value}% | {delta} | {delta} | ≥ 70% | {emoji} |
| ├─ Reutilização | {value}% | {delta} | {delta} | ≥ 60% | {emoji} |
| └─ Evolução | {value}% | {delta} | {delta} | ≥ 20% | {emoji} |

**Tendência geral:** {improving | stable | declining}

---

## 🏆 TOP 3 AGENTES (Valor — M5)

| # | Agente | Valor | Tasks | Sucesso | Level | Confiança | Destaque |
|---|--------|-------|-------|---------|-------|-----------|----------|
| 1 | {name} | {score} | {n} | {rate}% | L{n} | {conf} | {reason} |
| 2 | {name} | {score} | {n} | {rate}% | L{n} | {conf} | {reason} |
| 3 | {name} | {score} | {n} | {rate}% | L{n} | {conf} | {reason} |

---

## ⚠️ ATENÇÃO (Bottom 3 — M5)

| # | Agente | Valor | Tasks | Sucesso | Problema | Ação Recomendada |
|---|--------|-------|-------|---------|----------|-----------------|
| 1 | {name} | {score} | {n} | {rate}% | {issue} | {action} |
| 2 | {name} | {score} | {n} | {rate}% | {issue} | {action} |
| 3 | {name} | {score} | {n} | {rate}% | {issue} | {action} |

---

## 📈 EVOLUÇÃO DOS AGENTES (M9)

| Agentes que subiram de nível | Level Anterior → Novo | Trigger |
|------------------------------|----------------------|---------|
| {name} | L{n} → L{n+1} | {reason} |

| Agentes estagnados (> 30 dias sem execução) | Última task | Ação |
|---------------------------------------------|------------|------|
| {name} | {date} | Atribuir task de aquecimento |

---

## 🧠 SAÚDE DA MEMÓRIA

| Métrica | Valor |
|---------|-------|
| Total de entradas ativas | {n} |
| Curadoria executada | {date} |
| Entradas condensadas (R2) | {n} |
| Entradas depreciadas (R3) | {n} |
| Entradas promovidas a global (R4) | {n} |
| Failures resolvidos (R5) | {n} |
| Failures não resolvidos (> 90 dias) | {n} |
| CurationScore médio | {score} |
| Contexto liberado (tokens) | {n} |

---

## 🔍 MÉTRICAS SECUNDÁRIAS

| Métrica | Valor | Tendência |
|---------|-------|-----------|
| Tempo médio/task (M6) | {value} | {trend} |
| Acerto na escolha de agente (M7) | {value}% | {trend} |
| Tokens correção/construção (M12) | {ratio} | {trend} |
| Cross-agent learnings (M14) | {n} esta semana | {trend} |
| Cobertura de testes (M11) | {value}% | {trend} |

---

## 📋 TENDÊNCIAS

{Lista de 3-5 observações sobre tendências de longo prazo}

Exemplos:
- ✅ Taxa de sucesso subindo 4 semanas consecutivas (82% → 87% → 89% → 92%)
- ✅ Tokens por task caindo consistentemente (-180/task esta semana)
- ⚠️ 12 agentes ainda sem execução real — apenas seed data
- ⚠️ cosca-mobile: 0 tasks em 30 dias — capability profile pode estar desatualizado
- ✅ Curadoria removeu 15 entradas obsoletas — memória 8% mais enxuta

---

## 🎯 METAS PARA PRÓXIMA SEMANA

- [ ] Meta 1 (vinculada a métrica específica)
- [ ] Meta 2
- [ ] Meta 3
```

---

## Exemplo com Dados Reais

```markdown
# Cosca Platform Health — Semana 31, 2026

> Gerado: 2026-07-28T10:00:00Z | Ciclo: cur-2026-07-28-001 | Agentes ativos: 10/51

---

## 📊 RESUMO EXECUTIVO

| Métrica | Atual | vs Semana Anterior | vs Mês Anterior | Meta | Status |
|---------|-------|-------------------|-----------------|------|--------|
| Taxa de Sucesso (M1) | 100% | — (primeira medição) | — | ≥ 90% | 🟢 |
| Tokens/Task (M2) | 3,240 | — | — | -5%/mês | 🟡 |
| Bugs/Commit (M3) | 0.00 | — | — | < 0.05 | 🟢 |
| Tempo Economizado (M4) | 68% | — | — | > 60% | 🟢 |
| Confiança Média (M10) | 0.48 | — | — | > 0.70 | 🟡 |
| **Cosca Intelligence Score** | **42.80** | **—** | **—** | **≥ 70** | **🟠** |
| ├─ Autonomia | 40% | — | — | ≥ 80% | 🟠 |
| ├─ Precisão | 90% | — | — | ≥ 85% | 🟢 |
| ├─ Auto-correção | 50% | — | — | ≥ 70% | 🟠 |
| ├─ Reutilização | 15% | — | — | ≥ 60% | 🔴 |
| └─ Evolução | 12% | — | — | ≥ 20% | 🟠 |

**Tendência geral:** Baseline estabelecido — primeira medição.

---

## 🏆 TOP 3 AGENTES (Valor — M5)

| # | Agente | Valor | Tasks | Sucesso | Level | Confiança | Destaque |
|---|--------|-------|-------|---------|-------|-----------|----------|
| 1 | cosca-documentation | 0.92 | 3 | 100% | L3 | 0.95 | Auditou 887 docs e criou 4 novos |
| 2 | cosca-security | 0.89 | 3 | 100% | L2 | 0.90 | Security audit completo da plataforma |
| 3 | cosca-backend | 0.78 | 3 | 100% | L3 | 0.92 | API surface mapping + metacognition |

---

## ⚠️ ATENÇÃO (Bottom 3 — M5)

| # | Agente | Valor | Tasks | Sucesso | Problema | Ação Recomendada |
|---|--------|-------|-------|---------|----------|-----------------|
| 1 | cosca-mobile | 0.00 | 0 | — | Nunca executou | Atribuir task de audição de código mobile |
| 2 | cosca-messaging | 0.00 | 0 | — | Nunca executou | Atribuir task de event bus review |
| 3 | cosca-compliance | 0.00 | 0 | — | Nunca executou | Atribuir task de revisão de compliance |

---

## 📈 EVOLUÇÃO DOS AGENTES (M9)

| Agentes que subiram de nível | Level Anterior → Novo | Trigger |
|------------------------------|----------------------|---------|
| cosca-documentation | L1 → L3 | Cross-reference audit de 887 docs |
| cosca-security | L1 → L2 | Multi-layered security audit |
| cosca-database | L1 → L2 | PostgreSQL→SQLite reality correction |
| cosca-backend | L1 → L3 | API surface mapping + metacognition |
| cosca-runtime | L1 → L2 | Code-to-documentation cross-validation |
| cosca-frontend | L1 → L2 | Architecture extraction from 240+ TSX files |

| Agentes estagnados (> 30 dias sem execução) | Última task | Ação |
|---------------------------------------------|------------|------|
| 41 agentes | Nunca executaram | Iniciar Onda 2 de ativação |

---

## 🧠 SAÚDE DA MEMÓRIA

| Métrica | Valor |
|---------|-------|
| Total de entradas ativas | 28 |
| Curadoria executada | 2026-07-28 (manual — Fase 3) |
| Entradas removidas (bugs template) | 7 |
| Failures registrados | 3 (cosca-backend) |
| Failures resolvidos | 3 (100%) |
| CurationScore médio | 0.52 |
| Contexto liberado (tokens) | ~1,800 |

---

## 📋 TENDÊNCIAS

- ✅ Primeira medição de baseline — 10 agentes ativos, 41 pendentes
- ✅ 6 agentes evoluíram do seed data (L1 → L2/L3) em uma única sessão
- ✅ Zero bugs introduzidos nos commits de documentação e memória
- ⚠️ 41 agentes sem execução real — risco de capability profiles imprecisos
- ⚠️ Confiança média 0.48 — abaixo da meta de 0.70 (esperado para baseline inicial)
- ✅ Documentação alinhada com código (Fases 1-2) — zero discrepâncias restantes

---

## 🎯 METAS PARA SEMANA 32

- [ ] Ativar 5 novos agentes da Onda 2 (primeira task real)
- [ ] Atingir confiança média ≥ 0.55
- [ ] Completar primeiro ciclo de curadoria automática
- [ ] cosca-performance executar primeira task de profiling
- [ ] Reduzir agentes sem execução de 41 para 36
```

---

## Coleta de Dados

### Fontes automáticas

| Métrica | Fonte | Método de coleta |
|---------|-------|-----------------|
| M1 (Success Rate) | learnings.md de todos agentes | Contar outcome=success vs total |
| M2 (Tokens/Task) | Orchestration engine metrics | `internal/orchestration/metrics.go` |
| M3 (Bug Rate) | bug/ directory + git log | Bugs registrados / total commits |
| M4 (Time Saved) | Estimativa manual + tempo real | Pipeline registra timestamps |
| M5 (Agent Value) | Formula das 5 métricas | Calculado semanalmente |
| M6-M15 | Diversas fontes | Combinado de métricas do runtime |

### Dashboard triggers

| Trigger | O que dispara |
|---------|--------------|
| **Semanal** | Geração automática toda segunda-feira |
| **Pós-curadoria** | Após cada ciclo de curadoria de memória |
| **Level-up** | Quando qualquer agente sobe de nível |
| **Manual** | `cosca dashboard` (Don ou Kernel) |

---

## Alertas Automáticos

O dashboard gera alertas quando:

| Condição | Alerta | Severidade |
|----------|--------|-----------|
| M1 < 70% por 2 semanas consecutivas | "Taxa de sucesso crítica" | 🔴 |
| M2 aumentando por 3 semanas | "Eficiência de tokens piorando" | 🟠 |
| M3 > 0.20 | "Taxa de bugs inaceitável" | 🔴 |
| Agente com M5 = 0 por 30 dias | "Agente sem atividade" | 🟡 |
| Confiança de agente caindo por 2 semanas | "Agente com performance piorando" | 🟠 |
| Failure mode se repetindo cross-agent | "Falha organizacional detectada" | 🔴 |
| Memória com CurationScore < 0.30 não resolvida em 14 dias | "Memória poluída não tratada" | 🟡 |

---

> **Related**: [CONSTITUTION.md](../CONSTITUTION.md) | [CONFIDENCE_MODEL.md](../engines/evidence/CONFIDENCE_MODEL.md) | [MEMORY_CURATION_ENGINE.md](../engines/memory-curation/MEMORY_CURATION_ENGINE.md) | [metacognition-pipeline.md](../workflows/metacognition-pipeline.md)
