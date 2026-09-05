# cosca-backend — Patterns

## P1 — Handler Middleware Chain
**Quando usar**: Todo handler HTTP.
**Padrão**: Recovery → CORS → SecurityHeaders → Auth → CSRF → RateLimiter → Logging → Handler.
**Exemplo**: server.go registerRoutes() aplica middleware na ordem correta.

## P2 — Nil-Safe Adapter
**Quando usar**: Componente opcional no motor de orquestração.
**Padrão**: Campo nil-safe + setter + fallback no build. Se nil, feature desabilitada sem erro.
**Exemplo**: RunHandler.knowledgeEngine — se nil, buildEngine pula knowledge search.

## P3 — gRPC Client (Lazy Dial)
**Quando usar**: Cliente gRPC para serviço opcional.
**Padrão**: addr + mutex + conn lazy + service client. Dial só no primeiro RPC. Timeout curto (2s). Erro claro quando indisponível.
**Exemplo**: KnowledgeClient, MemoryClient, RuntimeClient.
