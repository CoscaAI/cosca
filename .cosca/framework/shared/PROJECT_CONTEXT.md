# PROJECT CONTEXT — Cosca v1.5.0

You are building the **Cosca AI Orchestration System Enterprise Platform**.

## Stack
| Component | Detail |
|-----------|--------|
| Backend | Go 1.25 (357 .go files, 71 packages) |
| Frontend | Next.js 15 (272 .tsx files, 27 feature modules) |
| Database | SQLite (FTS5+vector, embedded via modernc.org) |
| CLI | Cobra (39 commands) |
| WASM Runtime | wazero |
| API | REST on port 14120 (54 endpoints, 10 domains) |
| Auth | JWT (HS256), RBAC (admin/editor/viewer) |
| AI | 11 LLM providers, 10 editor adapters |
| Plugins | 4 plugin runtimes (WASM) |

## Architecture
5-layer Go monolith with interface-driven subsystems.

## Key Resources
- Module: `github.com/CoscaAI/cosca`
- Codebase map: `internal/embed/cosca/memory/codebase/overview.md`
- Constitution: `internal/embed/cosca/CONSTITUTION.md` (v1.1.0, 8 immutable principles)
- Kernel: `internal/embed/cosca/KERNEL.md`
