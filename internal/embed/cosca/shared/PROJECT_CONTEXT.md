# PROJECT CONTEXT — Cosca v1.5.0

You are building the **Cosca AI Orchestration System Enterprise Platform** — a self-contained AI-native organization that can build software, media, 3D, games, music, science and trading.

## Stack
| Component | Detail |
|-----------|--------|
| Backend | Go 1.26.7 (monorepo, `github.com/CoscaAI/cosca`) |
| Frontend | React/Vite + Wails (desktop ~15MB); Next.js (web), shadcn/ui |
| Database | SQLite (FTS5 + vector, embedded via modernc.org) — `knowledge.db` |
| CLI | Cobra — **106 command groups** |
| API | REST on port 14120 (serve, WSL2) |
| Auth | JWT, DPAPI machine-bound identity + consent-to-content (Don gate) |
| AI | Local-first: ollama + ROCm GPU (RX 6700 XT). Cloud = last resort (Cofre) |
| Plugins | pipeline plugins (build, checkpoint, lint, test, trace) |
| Memory | Semantic modular: knowledge.db (documents/chunks/vectors/entities) + blockchain chain per agent |

## Architecture (Cosca = cognition, Unreal = body, Adapters = nervous system)
5-layer Go monolith, interface-driven. 53 agents, 34 engines, 28 skill dirs, 41 departments.

## The family's portfolio
- **Cosca (this repo)** — the framework/orchestrator.
- **cosca-code** (`projects/cosca-code`) — AI-native code assistant (Wails + 8 exec engines: sci/td3d/cinema/music/media/diff/game/cross). Currently READ-ONLY (execGuard).
- **cosca-trader** (`projects/cosca-trader`) — crypto trading platform (OMS, risk, scientific gate).
- **HornFit** (`projects/hornfit`) — Academy/condo management OS (NestJS + Next.js + Prisma, fintech).
- **Unreal Projects** — 3D/game world (MetaHumans, ALS, SICKAMANSION), Cosca↔Unreal bridge (WebSocket 14120).

## Key Resources
- Module: `github.com/CoscaAI/cosca`
- Codebase map: `internal/embed/cosca/memory/codebase/overview.md`
- Constitution: `internal/embed/cosca/CONSTITUTION.md` (v1.1.0, 8 immutable principles)
- Kernel: `internal/embed/cosca/KERNEL.md`
- **Tool Execution Policy**: `internal/embed/cosca/shared/TOOL_EXECUTION_POLICY.md` (contrato universal de execução de tools — rigoroso para segurança, tolerante para execução recuperável; TODOS os agentes obedecem)
- Despertar (wake): `cosca despertar` — reads identity/state from knowledge.db
- Memory: `internal/embed/cosca/memory/agent/cosca-kernel/` (chain + learnings + failures)
