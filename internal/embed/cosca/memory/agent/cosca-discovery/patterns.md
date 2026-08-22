# cosca-discovery — Reusable Patterns

## Pattern P001 — Three-tier reality baseline

Use three independent evidence tiers before classifying a capability: **E2** static symbols and wiring, **E3** tests/contracts/commands, and **E4** an actual execution or runtime artifact. Keep `archived/`, untracked files, generated defaults, and documentation claims in separate buckets. This prevents counting a generated gRPC `Unimplemented` fallback, archived Python scripts, or README badges as active product behavior.

**Applied:** 2026-08-04, `/tmp/opencode/cosca-baseline.md`.
