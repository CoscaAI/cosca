# cosca-uiux — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-uiux |
| **Task** | Initial capability establishment |
| **Technique** | Standard uiux patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #uiux #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core uiux patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Full Activation & UX Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-uiux |
| **Task** | Full activation — UX audit of Cosca CLI + Web |
| **Technique** | Cross-surface UX audit (CLI + Web + Personas + Gaps) |
| **Level** | 1→2 |
| **Outcome** | success |
| **Tags** | #uiux #audit #activation #cli #web #accessibility |
| **Related** | .opencode/cosca/memory/uiux/activation-report.md |
| **Learned** | |
| **CLI UX** | 32 commands with `--help`, colored output, table/json/yaml/text formats, progress bars, spinners, shell completion (bash/zsh/fish/powershell). Agent commands well-structured but some are stubs (e.g., `agent run` doesn't call LLM). Mixed case in agent list output (uppercase "BACKEND CHIEF" vs lowercase "active"). |
| **Web UI** | Next.js 15 App Router + React 19 + Tailwind CSS v3 + Radix UI. Responsive layout (desktop sidebar, tablet drawer, mobile bottom nav). Dark-first theme. Full CSS variable theming with brand colors. Accessibility: axe-core, SkipNav, WCAG AA contrast verified, focus-visible rings. Storybook + Playwright E2E. 20+ feature modules but many are routing-only shells. |
| **UX Strengths** | Output formatter (4 formats), progress/spinner components, global flags (verbose/quiet/json/no-color), command palette, keyboard shortcuts, PWA support, animated transitions. |
| **UX Gaps** | 1) Some CLI commands are stubs (agent run doesn't execute). 2) Doc URL (cosca.enterprise/docs) probably unreachable. 3) No structured error codes. 4) Web features vary in completion (routing exists but content may be placeholder). 5) No guided onboarding wizard. 6) No dark/light mode toggle visible in CLI. |
| **Next** | Level 2 confirmed — first real task completed. Focus on CLI stub remediation and web content completion. |
