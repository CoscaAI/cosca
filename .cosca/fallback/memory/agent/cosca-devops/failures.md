# cosca-devops — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-08-04 — pnpm Build Approval Blocked E2E Validation
| Field | Value |
|-------|-------|
| **Agent** | cosca-devops |
| **Task** | Validate the corrected `make test-e2e` target |
| **Failed Approach** | Run `make test-e2e` in the current workspace without pre-approving pnpm dependency build scripts |
| **Root Cause** | pnpm rejected ignored build scripts for esbuild, msw, sharp, and unrs-resolver before Playwright launched |
| **Consequence** | E2E test execution could not reach test discovery or browser startup |
| **Lesson** | Prepare pnpm build approvals in the environment, then rerun the target; do not broaden the Makefile change to work around package-manager policy |
| **Confidence Impact** | -0.10 |
| **Tags** | #failure #learned #devops #e2e #pnpm #environment |
| **Related Success** | `learnings.md` — Make E2E Target and CODEOWNERS Baseline |
| **Avoidance Pattern** | Check dependency-manager policy and installed browser/tooling prerequisites before treating an E2E command failure as a target-routing regression |

### 2026-08-04 — Conditional E2E and CODEOWNERS Follow-up
Task completed without failures; `make -n test-e2e` and `git diff --check` passed.

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
