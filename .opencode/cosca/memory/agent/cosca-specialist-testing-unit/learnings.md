# cosca-specialist-testing-unit — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-unit |
| **Task** | Initial capability establishment |
| **Technique** | Standard testing-unit patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #testing-unit #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core testing-unit patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-09-05 — SOLITEK backend: first unit tests (0 → 48)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-unit |
| **Task** | Close test gap (0 tests) for SOLITEK NestJS + Prisma backend |
| **Technique** | Mock PrismaService (no DB); mock `$transaction` to invoke callback with tx object; jest.mock('bcrypt') for auth; jest.config.js with ts-jest + rootDir 'src' |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #testing-unit #nestjs #prisma #jest #crud #transaction |
| **Related** | apps/backend/src/**/*.service.spec.ts (5 specs, 48 tests) |
| **Learned** | Mock `$transaction` as `prisma.$transaction.mockImplementation(async cb => cb(tx))` where tx holds `.serviceOrder{create,update,findUnique}` + `.serviceOrderHistory.create`. ServiceOrdersService takes (prisma, niimbot) — mock both. `updateStatus(id, dto, userId?, description?)` reads custom description as 4th ARG (not from dto) — test must pass it as param to assert correctly. AUTH: auth.service has ONLY `login` (no `register` method exists). |
| **Next** | Add specs for products, analytics, storage, public, printing services |
