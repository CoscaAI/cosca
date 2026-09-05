# cosca-automation — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-automation |
| **Task** | Initial capability establishment |
| **Technique** | Standard automation patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #automation #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core automation patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Cognitive State Fast Path Optimization
| Field | Value |
|-------|-------|
| **Agent** | cosca-automation |
| **Task** | Optimize bootstrap startup time by reducing file scans |
| **Technique** | Startup path optimization — cognitive-state single-source compression |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #bootstrap #startup #optimization #memory #cognitive-state #fast-path |
| **Related** | .opencode/cosca/bootstrap/BOOTSTRAP.md, .opencode/cosca/memory/INDEX.md, .opencode/opencode.json |
| **Learned** | Reduced Phase 0 health checks from 7→3 files (deferring 4 to Phase 4). Reduced memory Load Order from 4→2 files (using cognitive-state.md as single source instead of context/session.md + codebase/overview.md + project/cosca-cli-overview.md). Updated Kernel instructions to prioritize Phase 0.5 fast path over sequential 10-phase execution. Net impact: Phase 0 drops from 7 file checks to 3; startup memory load drops from 4 files (~12K bytes) to 1 file (~400 tokens) when cognitive-state is fresh. |
| **Next** | Level 3+: Implement automated cognitive-state freshness verification with fallback chain |

### 2026-08-26 — Deep Orphan Package De-declaration (HornFit F3.3)
| Field | Value |
|-------|-------|
| **Agent** | cosca-automation |
| **Task** | Remove orphaned/empty packages `packages/ui` and `packages/shared` from HornFit pnpm+turbo monorepo, verify nothing breaks |
| **Technique** | Dead-reference audit + monorepo cleanup + gate verification (install/typecheck/lint/test/build) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #monorepo #pnpm #turbo #cleanup #de-declare #orbots #refactoring #windows |
| **Related** | C:\\Users\\Henrique\\Documents\\projects\\hornfit |
| **Learned** | A "simple" package de-declare has a WIDE blast radius beyond package.json+lib files. Removing a workspace package touches: next.config transpilePackages, per-app tsconfig `paths`, api tsconfig `paths`+`baseUrl`, jest `moduleNameMapper`, root/docs README+, `tailwind.config.ts` content globs, and BOTH Dockerfiles (`COPY packages/*/package.json packages/*/` layers for pnpm install). Grep for `@scope/pkg` AND bare `packages/<dir>` paths (not just the package name) to catch all leftovers. `packages/config` is shared base — keep. Note: `output: 'standalone'` in next.config triggers an EPERM symlink error during build trace collection on Windows (pre-existing, unrelated to cleanup — code still compiles: "Compiled successfully", static pages generated, type+lint passed). Always re-run `pnpm install` (not --frozen) after removing deps to regenerate the lockfile; verify lock clean via Select-String (rg unavailable). |
| **Next** | Reuse blast-radius checklist (tsconfig paths, jest mapper, transpilePackages, tailwind content, Dockerfile COPY layers, docs) for future package removals |
