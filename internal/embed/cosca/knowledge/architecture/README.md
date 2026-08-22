# 08 — SOFTWARE ARCHITECTURE INTELLIGENCE

> Stack 08 da Cosca Engineering Intelligence Matrix — o orquestrador dos demais stacks.

## MISSÃO
Projetar arquitetura antes de código. **Architecture is about boundaries and trade-offs** — não quantidade de pastas.

## PRINCÍPIOS CORE
1. **ADR (Architecture Decision Records)**: toda decisão registra `CONTEXT → DECISION → ALTERNATIVES → TRADE-OFFS → CONSEQUENCES` · UNIVERSAL
2. **Boundaries primeiro** (dependências apontam para dentro, domínio no centro) · STRONG
3. **Vertical slices** quando o fluxo de negócio é o eixo · STRONG
4. **Complexidade proporcional** à equipe/domínio/escala · STRONG
5. **Estilos são vocabulário, não dogma** (DDD, Clean, Hexagonal, Onion, modular monolith, CQRS, event-driven) · UNIVERSAL

## REGRAS DE DECISÃO
- Nunca responda "qual arquitetura é melhor?" — responda "qual é mais adequada para **estas restrições**?" (team, scale, domain, budget, deployment, operations, compliance, performance).
- Monólito modular com DDD é o default sensato; microservices exigem justificativa real.
- Toda decisão arquitetural vira ADR (rastreável, revisável).

## ANTI-PATTERNS
`architecture astronautics` (abstração sem problema real) · `pastas que imitam arquitetura sem boundaries` · `DDD sem linguagem ubíqua` · `CQRS sem motivo de escala` · `decisão sem ADR` · `refactoring sem teste de segurança`

## CHECKLIST
- [ ] Dependências apontam para dentro (domínio não conhece infra)
- [ ] Decisões registradas como ADR
- [ ] Trade-offs explícitos (o que se perdeu)
- [ ] Boundaries de dados por domínio
- [ ] Testes protegendo as boundaries

## A REGRA
Arquitetura é sobre boundaries e trade-offs. Não é sobre quantidade de pastas.

## REFERÊNCIAS
wshobson/agents · CleanArchitecture · modular-monolith-with-ddd · ddd-by-examples · java-design-patterns
