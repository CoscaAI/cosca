# cosca-uiux - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

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
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
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
| **Related** | internal/embed/cosca/memory/uiux/activation-report.md |
| **Learned** | |
| **CLI UX** | 32 commands with `--help`, colored output, table/json/yaml/text formats, progress bars, spinners, shell completion (bash/zsh/fish/powershell). Agent commands well-structured but some are stubs (e.g., `agent run` doesn't call LLM). Mixed case in agent list output (uppercase "BACKEND CHIEF" vs lowercase "active"). |
| **Web UI** | Next.js 15 App Router + React 19 + Tailwind CSS v3 + Radix UI. Responsive layout (desktop sidebar, tablet drawer, mobile bottom nav). Dark-first theme. Full CSS variable theming with brand colors. Accessibility: axe-core, SkipNav, WCAG AA contrast verified, focus-visible rings. Storybook + Playwright E2E. 20+ feature modules but many are routing-only shells. |
| **UX Strengths** | Output formatter (4 formats), progress/spinner components, global flags (verbose/quiet/json/no-color), command palette, keyboard shortcuts, PWA support, animated transitions. |
| **UX Gaps** | 1) Some CLI commands are stubs (agent run doesn't execute). 2) Doc URL (cosca.enterprise/docs) probably unreachable. 3) No structured error codes. 4) Web features vary in completion (routing exists but content may be placeholder). 5) No guided onboarding wizard. 6) No dark/light mode toggle visible in CLI. |
| **Next** | Level 2 confirmed — first real task completed. Focus on CLI stub remediation and web content completion. |

### 2026-08-25 — COSCA Desktop Design System pass
| Field | Value |
|-------|-------|
| **Agent** | cosca-uiux |
| **Task** | UI audit + consolidation of cosca-desktop (Wails + React/TS) — design token pass |
| **Technique** | Token-driven CSS consolidation (casa: design.css tokens, style.css layout, App.tsx inline) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #uiux #design-system #tokens #css #wails #desktop #a11y |
| **Related** | C:\Users\Henrique\Documents\projects\cosca-desktop\frontend\src\{design.css,style.css,layout/Splitter.tsx} |
| **Workflow / Files** | design.css: added `.icon/.icon-sm/.icon-lg/.icon-xs` primitives (flex-basis sizes) + `.icon svg{1em}` + `.btn/.iconbtn/.rail-btn .icon{margin:0;vertical-align:middle}`. Converted `.iconbtn` font-size 13px→var(--fs-md) + added `.iconbtn:disabled`. Added `.collapse-btn:active`. Added `.splitter[data-dragging="true"]::before{background:var(--accent)}`. style.css: h2 margin 4px→var(--sp-1), `.brand-mark` 14px→var(--fs-lg), `.code` 13px→var(--fs-md). Splitter.tsx: added `data-dragging={active?'true':undefined}`. |
| **Rules learned** | 1) On a token-driven workbench, NEVER rewire architecture — only consolidate values that escape `:root`. 2) Convert px→token ONLY when an exact token exists (14px→--fs-lg, 13px→--fs-md, 4px→--sp-1); leave intentional layout measures (rail 42px, pane widths 232/340/250/400, min-heights 40px) untouched. 3) Prefer ESLint-style declarative `[data-*]` state over imperative class toggles for splitter drag. 4) All new colors via `var(--...)` + `color-mix(in srgb, var(--...))` — zero hardcoded. 5) Verify via `cmd /c "cd frontend && npm run build"` (npm.ps1 blocked by PS ExecutionPolicy — always use npm.cmd via cmd /c). |
| **Next** | Level 3: hunt remaining App.tsx inline styles (~41) — extract repetitive ones to theme classes without touching layout/panes logic. |

### 2026-08-25 ??" COSCA Desktop Form Controls Overhaul
| Field | Value |
|-------|-------|
| **Agent** | cosca-uiux |
| **Task** | Form controls overhaul for cosca-desktop (Input/Textarea/Select/Search) ??" one source of truth, evolve tokens |
| **Technique** | Evolve `.input` in place (no parallel system): token-driven states via `var(--...)` + `color-mix(in srgb, ...)`, system focus ring `var(--ring)`, custom select chevron SVG, reusable `FormField` primitive |
| **Level** | 2?3 |
| **Outcome** | success |
| **Tags** | #uiux #form-controls #design-system #css #accessibility #a11y #select #input #wails #desktop |
| **Related** | C:\Users\Henrique\Documents\projects\cosca-desktop\frontend\src\{design.css,style.css,components/FormField.tsx,App.tsx,settings/Settings.tsx} |
| **Workflow / Files** | design.css: `.input:focus` box-shadow `0 0 0 3px var(--accent-soft)`??'`var(--ring)`; hover excludes `[readonly]`. Added `.input[readonly]`(background `--surface`, border-subtle, no ring), `.input[aria-invalid="true"],.input.error`(border `--danger` + `box-shadow:0 0 0 3px color-mix(in srgb,var(--danger) 12%,transparent)`), `.input-sm`(min-height `--ctrl-h-sm`), `.input-lg`(min-height 36px), `.input-search`(padding-left 30px), `select.input{appearance:none;background-image:data-URI chevron stroke=currentColor;padding-right:34px}`+`@media(forced-colors){appearance:auto;background-image:none}`, and `.form-field/.form-label/.form-help/.form-error/.req`. style.css: removed `.create label` (superseded by `.form-label`); added `.create .form-field + .form-field{margin-top:var(--sp-3)}`; `.settings-search`???flex + absolutely-positioned `.settings-search > svg`(left) & `> .kbd`(right); `.settings-search .settings-search-input{padding-left:30px;padding-right:28px}`(specificity 0,2,0 to survive style.css?design.css order). Created `components/FormField.tsx`. App.tsx: 2 create fields migrated to `FormField` (`htmlFor`?`id` fix). Settings.tsx: `input input-search settings-search-input` on search field. |
| **Rules learned** | 1) `style.css` imports BEFORE `design.css` in main.tsx ? `design.css` wins equal specificity; use a more-specific selector (`.parent .child` = 0,2,0) to override `.input`(0,1,0) from the page layer. 2) `currentColor` WORKS in `background-image` data-URI SVGs (theme-adaptive select chevron, zero hardcoded color) ??" allowed exception per spec. 3) For a11y/forced-colors, revert `select.input` to `appearance:auto` (native arrow = guaranteed). 4) Error state = `aria-invalid` + `.error` class (border + translucent reinforcement), NOT `aria-invalid="false"` (avoids false-green). 5) Only migrate to FormField where there's a real form (create); keep palette/terminal/agent textarea as `.input` (CSS-only upgrade) to avoid touching handlers/bindings. 6) Always build via `cmd /c "npm run build"` (npm.ps1 blocked). |
| **Next** | Level 3: candidate ??" `.input-error`/`.input-success` reality-check, or extract remaining App.tsx inline styles (~41) to theme classes without touching layout/panes logic. |

### 2026-08-29 — Cosca Brain 3D: interação + conexão spec
| Field | Value |
|-------|-------|
| **Agent** | cosca-uiux |
| **Task** | Diagnostic + spec for "brain 3D" liveliness (internal/brainweb/web) — make interaction obvious + connections visible |
| **Technique** | Prioritized interaction/connection spec (P0/P1) reusing existing tier/hue system (no re-design of color) |
| **Level** | 2→3 |
| **Outcome** | success |
| **Tags** | #uiux #spec #threejs #dataviz #graph #a11y #accessibility #interaction |
| **Related** | internal/brainweb/web/{index.html,app.js,style.css}; internal/brainweb/graph.go |
| **Root causes** | 1) Hint claims orbit/zoom/pan but `bindPointer()` only does picking; `state.controls=null` — no camera controller exists; auto-orb is `scene.rotation.y+=dt*0.02` (rotates graph, not camera). 2) Edges = `LineBasicMaterial opacity 0.4` ~1px, no glow/flow/importance — the connection is the least visible element. 3) Filter uses `m.visible` flip (no fade). |
| **Spec decisions** | P0: real camera rig (OrbitControls preferred, custom rig fallback, 0.30°/px, damping 0.08); 2.2s ease-out intro auto-orbit + `grab/grabbing/pointer` cursor + onboarding pill; replace lines w/ curved `TubeGeometry` on `QuadraticBezierCurve3`, radius by hierarchy (core 0.55→0.35→0.22→0.14), vertex-color gradient, AdditiveBlending glow, UV-u texture-scroll energy flow (GPU) + comet on hover; hover/select lights 1-hop subgraph, rest fades (250ms ease-out). Radial tier layout (cones/tentos/capos r≈0/14/22-26, soldier ring r≈30-34), skills orbit their own capo by `domain` (fixes sparse feel), dept springs for crew clusters. P1: `reduced-motion` gate, arrow/Tab keyboard nav by depth, `aria-live` SR summary + `sr-only` node list. |
| **Rules learned** | 1) Rule of gold: never ship a hint for an unimplemented control — promise == implementation. 2) `TubeGeometry` UV.u runs along length → enables scrollable flow texture (GPU, no custom shader risk). 3) Separating `reports_to` (thick/glow/flow) from `department` (thin/dashed/faint) is what makes hierarchy pop — one semantic per edge family. 4) Sparse-looking 3D graphs are often an artifact of a disconnected outer "halo" — cluster leaves around their parent to densify with meaning. 5) Always re-audit existing decision: color was already decided (tier=luminance/size, dept=hue) — do not re-spec it. |
| **Next** | Implement P0 in app.js (camera rig → tube edges → state feedback → layout rings), then record implementation deltas. |

### 2026-08-25 - COSCA Desktop Elevacao do Design System a padrao enterprise | 242c3de266ed14af

