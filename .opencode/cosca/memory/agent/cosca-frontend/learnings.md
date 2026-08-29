# cosca-frontend — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Frontend Architecture Documentation

### 2026-07-28 — Complete Frontend Codebase Scan
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Scan entire web/ directory and document frontend architecture from source |
| **Technique** | Level 2 — Architecture extraction from live codebase: analyzed Next.js 15 App Router structure (26 routes, 2 route groups), 27 feature modules (each with components/hooks/types pattern), 14 Radix UI primitives (shadcn/ui pattern with class-variance-authority), 18 shared component groups, state management (React Context + TanStack Query, no Zustand/Redux), custom API client (fetch wrapper with httpOnly cookie auth), testing setup (Vitest 43 test files, Playwright 6 E2E specs, Storybook 21 stories) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #frontend #nextjs #react #architecture #typescript #tailwind #radix #pwa |
| **Related** | web/src/, docs/frontend/architecture.md |
| **Learned** | Architecture follows feature-slice pattern: each feature under src/features/{name}/ is self-contained with components, hooks, types, tests, and barrel export. Auth uses httpOnly cookies exclusively — tokens never touch JavaScript. Auth state detected via cosca_auth_state=true sentinel cookie (non-HttpOnly, JS-readable). CSRF via double-submit pattern: csrf_token cookie + X-CSRF-Token header, constant-time compare server-side. API client is custom fetch wrapper (not Axios) with auto CSRF header injection. 27 features cover the full Cosca domain: agents, knowledge, memory, orchestration, providers, workflows, admin (users/api-keys/audit/secrets), and tooling (playground, pipelines, context-builder). PWA: service worker (cache-first, offline fallback), manifest (standalone mode), install prompt via beforeinstallprompt. Testing pyramid: MSW mocks 30+ endpoints for unit/integration tests. Storybook with a11y and dark mode addons. |
| **Next** | Level 3: Audit component a11y compliance (WCAG 2.1 AA), add visual regression testing (Chromatic/Percy), implement feature flag system for gradual rollouts |

### 2026-07-28 — State Management Pattern Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Document frontend state management architecture |
| **Technique** | Level 1 — Pattern extraction: traced auth flow, query client config, theme provider chain |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #state-management #react-query #context #auth #cookies |
| **Related** | web/src/providers/, web/src/features/auth/, web/src/lib/api.ts |
| **Learned** | No Zustand, Redux, or Jotai — architecture favors React Context for UI state and TanStack Query for server state. Auth is a module-level singleton (not context) with httpOnly cookies — JS never sees tokens. QueryClient config: staleTime=60s, gcTime=5min, retry=1, no refetchOnWindowFocus. Theme via next-themes (class-based, cookie-persisted). Pattern is appropriate for current complexity level. |
| **Next** | Level 2: Evaluate if any feature needs Zustand for complex local state (pipeline canvas, orchestration runner) |

## Session: 2026-08-29 — Cérebro Neural 3D (brainweb) — 3 views em app.js

### 2026-08-29 — Brain 3D views: neutron shaders + axônios + marcadores reais + feed de atividade + fallback 2D
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Implementar o front da visão 3D do cérebro em `internal/brainweb/web/app.js` (Three.js, ES Modules, self-hostado p/ CSP 'self', read-only, sem grid/eixos) |
| **Technique** | Level 3 — Three.js scene graph + custom ShaderMaterial (vertex/fragment com atributo `size`, color gradient ciano→azul pela posição no volume, AdditiveBlending, depthTest false); axônios Bézier com **spatial hash** (27 células vizinhas) p/ evitar O(n²); pool de partículas reutilizável (limitSignals 3000)+mesh; marcadores que mapeiam agentes REAIS (Don/Kernel/capos) a vértices do OBJ via `markerMap` (índice múltiplo de skipStep); feed de atividade ao vivo em DOM (fora do canvas, aria-live, esc()); detecção de WebGL (force2D/forceWebGL, swiftshader→fallback) + fallback 2D; `neuronColorByPosition` normaliza pela extensão REAL do eixo Y (~±33.5, não hardcoded 70/140) |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.75** (subida por: 3 views completos e integrados, lógica de contraste/repouso, e disciplina de CSP 'self' — three.js self-hostado, sem CDN) |
| **Tags** | #frontend #threejs #brain-3d #shader #spatial-hash #bezier-axon #marker-map #activity-feed #webgl #contrast #csp-self |
| **Related** | internal/brainweb/web/app.js, internal/brainweb/web/index.html (importmap inline), internal/brainweb/handler.go, internal/brainweb/embed.go (go:embed), /brain/graph + /brain/activity |
| **Learned** | 1) **O índice do marcador DEVE ser múltiplo de `verticesSkipStep`**: `buildBrain` itera `i += skip`, então um índice gravado fora do grid amostrado nunca é encontrado (bug que fazia Don/Kernel/capos sumirem). Usar `align(ordinal)` = clamp ao grid. 2) **Spatial hash para axônios**: compara só neurônios em ~27 células vizinhas — evita o loop O(n²) que travava o navegador com milhares de neurônios. 3) **`propagateSignals` = regime PURO (default)**: com toggle, a cascata de sinais ("rede pensando") fica desligada e só acende quem REALMENTE agiu (via `pollActivity` → `/brain/activity`), sem heartbeat aleatório — movimento sem evento real é decoração, não informação. 4) **Contraste**: repouso contido (light ~0.40) p/ destacar o pulso real; flash grave+gradual (decaimento `dt*1.2`, brilho até ~5x, halo expande `1+f*3`) torna cada ação inconfundível. 5) **CSP 'self'**: three.js e addons são self-hostados (importmap inline) — 'unsafe-inline' no index.html é só p/ o importmap; '-eval' alegando Next.js que NÃO é usado aqui (ver auditoria de security). 6) **Read-only**: sem grid/eixos, cérebro flutua limpo; 2D fallback honesto quando WebGL indisponível. 7) `esc()` sanitiza todo texto antes do innerHTML; `statusClass` mapeia fail/error/timeout → `is-error`. |
| **Next** | Alinhar o CSP: remover 'unsafe-eval' e aplicar nonce/hash no importmap (recomendação H3 do security); considerar `script-src-attr 'none'`; garantir contraste AA do feed/HUD (luminância). |
