# 04 — BACKEND ENGINEERING INTELLIGENCE

> Stack 04 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Inteligência de backend Staff/Principal: arquitetura, resiliência, concorrência e dados — com complexidade **proporcional ao problema**.

## PRINCÍPIOS CORE
1. **Dependency Inversion** — domínio não depende de framework/infra · UNIVERSAL
2. **Use cases como centro** (Clean/Hexagonal/DDD compartilham isso) · STRONG
3. **Idempotência, retries, timeouts** em toda operação mutável · UNIVERSAL
4. **Transactions com boundaries explícitas** (saga/outbox quando cruzando serviços) · STRONG
5. **Complexidade proporcional**: monólito modular > microservices quando equipe/domínio não exigem · STRONG

## REGRAS DE DECISÃO
- **Microservices NÃO são default**: avaliar `DOMAIN → SCALE → TEAM → DEPLOYMENT → DATA OWNERSHIP → FAILURE BOUNDARIES`.
- Monólito modular é a resposta mais frequente.
- Cada arquitetura documentada com `PROBLEM → SOLUTION → BENEFIT → COST → FAILURE MODE → WHEN → WHEN NOT`.

## ANTI-PATTERNS
`microservices por status` · `deus de domínio (anemic model + services vazios)` · `transações distribuídas sem saga/outbox` · `retry infinito` · `timeout ausente` · `DB como barramento` · `concorrência ingênua`

## CHECKLIST
- [ ] Idempotência em POSTs mutáveis
- [ ] Timeouts + retries com backoff + circuit breaker onde aplicável
- [ ] Boundaries de transação explícitas
- [ ] Dados por domínio (ownership)
- [ ] Failure modes documentados por serviço

## A REGRA
O melhor backend não é o mais complexo — é o que tem complexidade proporcional ao problema.

## DOUTRINAS
- `NESTJS.md` — NestJS: módulos por feature, DI por constructor, decorators como DSL, Pipes/Guards/Interceptors/Filters
- `JAVA.md` — Java/Spring Boot 3: monorepo, ArchUnit, outbox, BigDecimal, config-secrets
- `RUST.md` — Rust: ownership, newtype, unsafe isolado, Result/anyhow, small crates
- `CPP.md` — C++: RAII, GSL span, perfis Type/Bounds/Lifetime, const-correctness, Pimpl
- `CSHARP.md` — C#/.NET: Clean Architecture, MediatR/CQRS, DI scanning, Source Generators, Orleans
- `C.md` — C: ABI universal (o "latim" da família), embedded, allocators, structured concurrency

## REFERÊNCIAS
wshobson/agents · go-backend-clean-architecture · ardanlabs/service · golang-standards/project-layout · Watermill · Temporal · FastAPI/Starlette
