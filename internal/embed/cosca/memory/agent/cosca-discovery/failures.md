# cosca-discovery — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

## F001 — Frontend test command stopped in package-manager bootstrap

- **Date:** 2026-08-04
- **Attempt:** `pnpm --dir web test -- --run`
- **Result:** pnpm aborted with `ERR_PNPM_IGNORED_BUILDS` for `esbuild`, `msw`, `sharp`, and `unrs-resolver` before Vitest ran; it also temporarily wrote `allowBuilds` to `web/pnpm-workspace.yaml`.
- **Root cause:** dependency build approval/pnpm environment, not a test assertion.
- **Instead:** run in a prepared environment with explicit, reviewed build approvals; always inspect and restore git status after package-manager commands.

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
