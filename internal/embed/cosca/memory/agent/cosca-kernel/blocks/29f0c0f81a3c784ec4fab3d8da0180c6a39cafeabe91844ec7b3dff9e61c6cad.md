PREV: 29e00bc7398550d8d8b69250fcb243edd8d5934afa83fe7a1cfdf4ddafbcdbb3
ID: L252
TIME: 2026-08-15
LEVEL: 3
TAGS: #cosca-trader #bitcoin #ticker #rest-publico #auth-boundary #polling #fallback #binance #level-3
---
## L252 — 2026-08-15 — Preço do Bitcoin (Binance) no topbar via REST público — dado de mercado inofensivo fora da auth + polling como fallback ao WebSocket | Level 3

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Ordem do Don: "colocar o preço do bitcoin da binance" no painel do cosca-trader. |
| **Technique** | Novo endpoint `/ticker` (REST público, sem chave — GET /api/v3/ticker/price via binance.Price) + ticker ₿ no topbar do frontend com polling de 5s. Decisão de segurança: o preço de mercado é dado PÚBLICO e inofensivo (não expõe conta/ordens/saldos), então fica FORA da política de auth (como o /health) — diferente de /candles e /markets que vivem atrás do token. O polling REST complementa o SSE: mostra o preço mesmo antes do market data WebSocket conectar. |
| **Level** | 3 |
| **Outcome** | success — build Go + tsc/vite verdes; endpoint /ticker público e ticker no topbar funcionando sem token. |
| **Confidence** | 0.90 (evidência: go build + go test . verdes; npm run build verde) |
| **Tags** | #cosca-trader #bitcoin #ticker #rest-publico #auth-boundary #polling #fallback #binance #level-3 |
| **Related** | L249 (Tier 1), L246 (auditoria — auth fail-closed), L185-L187 (portas/frontend) |
| **Learned** | (1) A fronteira de auth deve seguir a NATUREZA do dado, não a uniformidade: preço de mercado é público e inofensivo → endpoint público (o usuário vê o ticker sem fricção); ordens/posições/saldos são sensíveis → auth obrigatória. (2) REST polling é o fallback certo para um ticker de preço: não depende do WebSocket ter conectado nem do SSE/EventSource ter autenticado — mostra o preço imediatamente ao abrir o painel. (3) Complementar (WebSocket p/ streaming de alta frequência + REST p/ referência de baixa frequência) é mais robusto que depender de uma via só. |
| **Next** | (1) Wire o ratelimit (L249) no /ticker para não martelar a Binance em rajada. (2) Permitir trocar o símbolo do ticker (hoje BTCUSDT fixo). (3) Cachear o preço no core (TTL curto) para múltiplos clientes não multiplicarem chamadas REST. |
