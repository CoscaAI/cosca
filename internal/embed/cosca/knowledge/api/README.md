# 05 — API ENGINEERING INTELLIGENCE

> Stack 05 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em design de API e evolução de contratos. **Uma API é um contrato.**

## PRINCÍPIOS CORE
1. **Contrato primeiro** — toda mudança considera `CLIENTS → COMPATIBILITY → MIGRATION → OBSERVABILITY → FAILURE` · UNIVERSAL
2. **Modelo de erro consistente** (validation/auth/not-found/conflict/rate-limit/server) em TODOS os endpoints · UNIVERSAL
3. **Backward compatibility** como regra; versionamento como último recurso · STRONG
4. **Idempotência** explícita em mutações · STRONG
5. **Paginação/filtro/ordenação** como convenção única · STRONG

## REGRAS DE DECISÃO
- **REST vs GraphQL vs gRPC**: REST para CRUD público; GraphQL para agregar múltiplos consumidores; gRPC para interno de alta perf. Não dogmatizar.
- **Evolução**: aditivo primeiro (novos campos opcionais) → deprecation com aviso → remoção em versão maior.
- **Erros**: machine-readable code + message humana + correlation id.

## ANTI-PATTERNS
`200 com erro no body` · `strings de status livres` · `breaking change silencioso` · `pagination ausente` · `erro genérico "500" sem rastreio` · `DTOs vazando detalhe interno` · `versionar por URL quando aditivo resolve`

## CHECKLIST
- [ ] Contrato documentado (OpenAPI/proto) e versionado
- [ ] Erros consistentes em todos os endpoints
- [ ] Idempotência em mutações
- [ ] Paginação/filtros padronizados
- [ ] Compatibilidade: novas mudanças são aditivas
- [ ] Contract testing no pipeline

## A REGRA
Toda mudança de API é uma decisão sobre os clientes existentes — nunca só sobre o servidor.

## REFERÊNCIAS
OpenAPI · GraphQL · gRPC · zalando/restful-api-guidelines · microsoft/api-guidelines · google/api-linter · buf
