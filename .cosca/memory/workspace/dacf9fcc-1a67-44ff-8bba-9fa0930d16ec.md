---
id: dacf9fcc-1a67-44ff-8bba-9fa0930d16ec
type: session
layer: workspace
scope: kernel
created_at: 2026-08-31T01:31:37.4118533-03:00
updated_at: 2026-08-31T01:31:37.4118533-03:00
ttl: 0s
priority: 0
version: 0
metadata:
    agent: cosca-mcp
    provenance: P0
    source: EVIDENCE
---

Interruptor da deliberação ADR-032 ligado em produção (fail-closed): o Kernel-First Deliberation agora pode ser ativado via config. Adicionado `orchestration.deliberation` no config YAML (enabled, emit_threshold, reservation_threshold, max_evidence, max_chars_per_evidence, min_score) + env vars `COSCA_ORCHESTRATION__DELIBERATION__*`. Propriedade `DeliberateConfigFromConfig(config.DeliberationConfig)` mapeia a seção YAML → OrchestratorConfig.DeliberateConfig (com weights A3 default). Propagado aos 6 pontos de construção do OrchestratorConfig (bootstrap, REST run handler, serve, runtime, run, chat, terminal, pipeline_wiring). FAIL-CLOSED garantido: Enabled=false por default (LEI DO COFRE), só liga com `enabled: true` explícito no YAML ou env var. Build/test/vet verdes, internal/embed intocado. Para ATIVAR a deliberação: adicionar `orchestration.deliberation.enabled: true` na config YAML do projeto, OU setar env `COSCA_ORCHESTRATION__DELIBERATION__ENABLED=true`. A partir daí o Kernel pensa primeiro (reúne evidências de knowledge/memory, compute convergence+confidence, e decide EmitOK (responde sem LLM) / EmitWithReservations / Escalate (LLM com contexto limpo)).
