---
type: bug
key: bug-004-provider-caching
tags: [provider, cache, instance, memory-leak]
severity: medium
timestamp: 2026-07-21T00:00:00Z
detected_in: cosca
fixed_in: commit f3dbdc2
confidence: 1.0
times_seen: 1
---

# Bug-004: Provider Instance Caching Leak

## Symptoms
- ChatRegistry and ProviderRegistry holding stale provider instances
- Memory growth over long-running daemon sessions
- Provider config changes not reflected until restart

## Causality Tree

### N1 — Causa Direta
`ChatRegistry` e `ProviderRegistry` mantinham instâncias de provider em cache sem TTL, sem invalidação, e sem health check. Providers obsoletos nunca eram removidos da memória.

### N2 — Causa Arquitetural
O registry não implementava o padrão Cache-Aside com TTL. A arquitetura de providers não separava "configuração" (imutável, do usuário) de "instância" (mutável, runtime). Toda mudança de config exigia restart — o sistema não tinha hot-reload de providers.

### N3 — Causa de Processo
- Testes cobriam apenas sessões curtas (< 1 minuto) — memory leak invisível
- Sem teste de long-running (> 24h) no pipeline
- Métricas de memória não eram coletadas

### N4 — Prevenção Sistêmica
- **Padrão de cache:** TTL obrigatório + invalidação hooks em todo cache do sistema
- **CI gate:** soak test de 1h com monitoramento de memória (alerta se crescimento > 10%)
- **Métricas:** cache size + cache hit rate expostos no dashboard
- **Arquitetura:** separar config (fonte da verdade) de instance (efêmera, derivada)
