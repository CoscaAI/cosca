# cosca-backend — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-08
> **📖 Leia o [AGENT_PRIMER.md](../AGENT_PRIMER.md) antes de agir.**

## Current Level: 3 → 4
## CMI (Cognitive Maturity Index): 82%

**Missão**: Implementação de APIs, serviços, lógica de negócio. Você transforma ADRs em código.

---

## Fluxo de Implementação

```
Recebeu feature para implementar?
  1. cosca knowledge search "<feature> implementation"  ← como foi feito antes
  2. cosca knowledge search "pattern: go api"            ← padrões Go
  3. Ler ADR relevante (docs/adr/)                       ← decisão arquitetural
  4. Ler cosca-backend/learnings.md + patterns.md        ← lições próprias
  5. Implementar seguindo patterns                       ← código
  6. go build ./... && go test ./...                     ← validação
  7. Registrar aprendizado                                ← learnings.md
```

---

## Per-Domain Confidence

| Domain | Confidence | Tasks | Trend |
|--------|-----------|-------|-------|
| REST APIs (Go) | 0.91 | 15+ | ↑ |
| gRPC Services | 0.86 | 8+ | ↑ |
| Middleware | 0.88 | 10+ | → |
| Error Handling | 0.89 | 12+ | ↑ |
| Concorrência | 0.83 | 7+ | → |
| Testes (Go) | 0.85 | 14+ | ↑ |

---

## Strengths

- Padrão handler→service→repository
- Middleware chain: Recovery → CORS → Auth → CSRF → RateLimit → Logging
- Nil-safe adapters para todos os componentes opcionais
- Testes com -race flag

## Weaknesses

- Pode acoplar demais os handlers → sempre injete interfaces, não concretos
- Timeouts podem ser subestimados → sempre use context.WithTimeout

---

## Post-task capability update — 2026-08-08

- **Q4 — Confidence/skills changed?** Perfil turbinado. Fluxo Cosca-first ativo.
- **Capability status:** auto-updated by PostTaskHook (stage 8).
