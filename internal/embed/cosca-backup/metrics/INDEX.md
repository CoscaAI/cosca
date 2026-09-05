# Framework Metrics — Cosca Health Tracking + Real Metrics (B1-B5)

> **Versão**: 2.0.0 | **Status**: active | **Owner**: cosca-monitoring | **Atualizado**: 2026-07-30
> **Referência**: `internal/embed/cosca/architecture/COGNITIVE_MATURITY.md` — Seção 3

---

## Métricas Reais de Maturidade Cognitiva (B1-B5)

Estas são as 5 métricas objetivas que medem melhoria real de capacidade do Cosca Runtime. Diferentemente de scores abstratos, são observáveis, contáveis e diretamente ligadas ao valor entregue.

| Métrica | Arquivo | O Que Mede | Baseline | Target Fase 1 |
|---------|---------|------------|----------|:---:|
| **B1 — Autonomia** | [B1_autonomous_problems.md](B1_autonomous_problems.md) | Tasks completadas sem intervenção humana | 20/26 (76.9%), streak 12 | 50 tasks |
| **B2 — Previsões** | [B2_predictions.md](B2_predictions.md) | Acurácia de previsões pré-task | 7/8 (87.5%) | ≥ 80% |
| **B3 — Decisões Revertidas** | [B3_reverted_decisions.md](B3_reverted_decisions.md) | Decisões que precisaram ser desfeitas | 1 reversão | ≤ 2 total |
| **B4 — Conhecimento Reutilizado** | [B4_reused_knowledge.md](B4_reused_knowledge.md) | Reaplicações de padrões/heurísticas | 12 reusos (3 padrões) | ≥ 15 reusos |
| **B5 — Detecção Preventiva** | [B5_prefailure_detection.md](B5_prefailure_detection.md) | Problemas detectados antes da falha | 5 proativas (ratio 50%) | ≥ 10 proativas |

---

## Especificação

| Documento | Descrição |
|-----------|-----------|
| [CMI_REAL_METRICS.md](CMI_REAL_METRICS.md) | Especificação completa: schema, coleta, dashboard, baseline, target de cada métrica |
| `internal/embed/cosca/architecture/COGNITIVE_MATURITY.md` | Arquitetura da Maturidade Cognitiva — Seção 3 define as métricas |
| `internal/embed/cosca/workflows/cognitive-maturity-implementation.md` | Workflow de implementação — Tarefa F1.5 |

---

## Métricas de Framework (Legado)

| Métrica | Arquivo | Tipo | Frequência |
|---------|---------|------|:----------:|
| Framework Health Dashboard | [framework-health-dashboard.md](framework-health-dashboard.md) | Dashboard | Por release |
| Platform Health Dashboard | [platform-health-dashboard.md](platform-health-dashboard.md) | Dashboard | Por release |

---

## Propósito

Rastrear e melhorar a qualidade, cobertura e saúde do framework Cosca. O framework deve "comer sua própria dog food" — usando os mesmos quality gates e métricas que define para projetos.

A partir de v2.0.0, o foco primário são as **5 Métricas Reais (B1-B5)** que alimentam o **Cognitive Maturity Index (CMI)**.

---

## Integração CMI

```
Métricas Reais (B1-B5) ──► Dimensões CMI ──► CMI Score
                                                          
B1 (Autonomia) ───────────► Julgamento + Autocrítica       
B2 (Previsões) ───────────► Julgamento + Consistência      
B3 (Reversões) ───────────► Julgamento + Autocrítica       
B4 (Reuso) ───────────────► Transferência + Aprendizado    
B5 (Detecção) ────────────► Planejamento + Consistência    
                                                          
CMI = (Aprendizado×0.20) + (Julgamento×0.25) +            
      (Planejamento×0.15) + (Autocrítica×0.15) +          
      (Transferência×0.15) + (Consistência×0.10)           
```

---

## Relacionados

- [Cognitive Maturity Architecture](../architecture/COGNITIVE_MATURITY.md)
- [Cognitive Maturity Implementation](../workflows/cognitive-maturity-implementation.md)
- [Kernel Learnings](../memory/agent/cosca-kernel/learnings.md)
- [Governance Chief](../departments/governance/SKILL.md)
