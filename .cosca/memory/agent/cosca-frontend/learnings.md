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

## Session: 2026-08-29 — Brain 3D: Camada 2 (Exploração Cirúrgica) — picking por demanda + painel do agente

### 2026-08-29 — Picking sob demanda (LOD de informação): só nós importantes clicáveis + painel DOM sanitizado
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Implementar "Camada 2 — Exploração Cirúrgica" no visualizador 3D do cérebro (`internal/brainweb/web/app.js`): detalhe só existe quando o usuário demonstra interesse. NÃO tornar ~3600 neurônios clicáveis. |
| **Technique** | Level 3 — **Picking leve por demanda**: criar um `THREE.Points` dedicado (`state.pickMesh`) que NÃO é adicionado à cena (não renderiza) contendo SÓ as posições dos vértices com `marker.node` (Don/Kernel/capos, ~55) — o `Raycaster` intersecta APENAS esse objeto (nunca `state.neuronPoints` com os ~3600). Índice do hit (`hits[0].index`) → `state.pickNodes[i]` (array paralelo de nodes reais). `raycaster.params.Points.threshold = PICK_THRESHOLD(6)` dá a área de clique em unidades do mundo. Clique vs órbita: `pointerdown` grava pos/tempo; `pointerup` só trata como clique se `Math.hypot(dx,dy)<=6` e `dt<600ms` (senão é drag, só fecha o painel). Cursor `pointer` no `pointermove` (OrbitControls NÃO gerencia cursor — `style.cursor` é seguro). Painel DOM (`<aside class=node-panel role=dialog aria-live=polite>`) criado via JS e appendado ao body, atualizado via `innerHTML` com `esc()` em todo texto. História de atividades p/ o painel: `state.activities = [...fresh, ...prev].slice(0,30)` em `pollActivity`. |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.75** (subida por: picking O(#marcadores) com dedicated pickMesh; distinção clique/órbita robusta; painel overlay fora do canvas; projeção mínima sanitizada respeitando ADR-021; `go build ./...` e `go test ./internal/brainweb/...` verdes; `node --check app.js` = 0) |
| **Tags** | #frontend #threejs #brain-3d #raycast #picking-lod #node-panel #security #sanitized-projection #accessibility #adr-021 #csp-self |
| **Related** | internal/brainweb/web/app.js, internal/brainweb/web/style.css, internal/brainweb/graph.go (contrato read-only), internal/brainweb/handler.go, /brain/graph + /brain/activity |
| **Learned** | 1) **Roleta de picking é DEDICADO**: para não fazer raycast na cena inteira a cada frame, cria-se um `THREE.Points` só com os vértices clicáveis (markers com `node`), fora da cena, usado como alvo do `raycaster.intersectObject(pickMesh, false)`. `visible=false` não bloqueia raycast (o raycaster só checa `object.layers`); mas NÃO adicionar à scene é o mais limpo. 2) **Match da skill → agente em graph.go**: `skillByDomain` é indexado por `skill.Domain` (lower) e o `Node.SkillCount` usa `skillByDomain[lower(a.Name)]` — ou seja, o domínio da skill casa com o NOME do agente (ID), não com o department. Por isso `relatedSkills(node)` compara `targets.has(s.domain)` com `{node.id, node.name.lower, node.department.lower}`. 3) **Clique x órbita**: `pointerdown`+`pointerup` com delta de posição e tempo; o `click` nativo é pouco confiável porque dispara mesmo após drag. 4) **Segurança (ADR-021)**: o painel mostra apenas `name/role/department/status/reports_to/skill_count` e skills só com `id/name/description/category/domain` — NUNCA instructions/governance, tools/capabilities/dependencies, conversas, args/prompt (o payload sanitizado nem contém isso). 5) **A11y**: `role="dialog"`, `aria-live="polite"`, `aria-label`, Esc fecha (`document keydown`), focus no botão de fechar ao abrir. 6) `state.activities` é a fonte para "atividade recente do agente" no painel. |
| **Next** | Considerar um "zoom-to-node" (mover a câmera/animação até o agente clicado) para reforçar o LOD de informação; testar manualmente o raio de clique (`PICK_THRESHOLD`) em resoluções/DPRs diferentes; opcionalmente conectar a API `/brain/observatory` (epistêmica) como fonte de "por que este nó está ativo". |

## Session: 2026-08-31 — KernelCenter: Cérebro vivo do Kernel (cosca-dashboard)

### 2026-08-31 — Estado cognitivo derivado + glow/partículas/neurônios dirigidos por CSS var
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Evoluir o círculo "COSCA KERNEL" no centro da Casa Visível para um cérebro vivo que brilha conforme o estado cognitivo real do Kernel (`frontend/components/organs/KernelCenter.tsx`), usando `shadow.summary`+`deliberation.stats` já puxados via `usePolling` (5000ms). Visual sóbrio de "console de observação cognitiva". |
| **Technique** | Level 3 — **Estado derivado por heurística** (`deriveKernelState`) com prioridade escalar; **glow dirigido por variável CSS** (`--k` = rgb do estado) consumida em keyframes `box-shadow` via `rgba(var(--k), a)`; **partículas orbitantes** = wrapper `.kernel-orbit` (rotação CSS no transform-origin center) com dots posicionados por `left/top` px (calculados de centro 56, raio 62-70) + `kernel-twinkle` (opacity/translateY/scale); **lattice neuronal interno** (SVG `viewBox 0 0 100 100`, nós/arestas hardcoded `NODES`/`EDGES`) com classes `.neuron-lit`/`.neuron-fast` por staggered `animationDelay`; linha do fluxo permanente (`FlowRail`, mapa mental `context→knowledge→memory→evidence→deliberation→decision→EMIT_OK|ESCALATE`). `prefers-reduced-motion` desliga as animações. |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.72** (subida por: estado 100% derivado do payload real; build `npm run build` no cosca-dashboard exit 0; página L1 `/` pré-renderizada estática com sucesso; `--k` via cast `as CSSProperties`; zero libs novas — só CSS/SVG puro) |
| **Tags** | #frontend #nextjs #tailwind #kernel-center #css-var-glow #derived-state #svg-lattice #keyframes #particles #reduced-motion #cosca-dashboard |
| **Related** | cosca-dashboard/frontend/components/organs/KernelCenter.tsx, cosca-dashboard/frontend/app/globals.css, cosca-dashboard/frontend/hooks/usePolling.ts |
| **Learned** | 1) **Glow com cor dinâmica SEM lib**: defina `style={{ ["--k"]: "r,g,b" } as CSSProperties}` no elemento e use `rgba(var(--k), a)` nos keyframes — o `rgb/triple` com `rgba(..., a)` (vírgulas) é compatível, diferente de `rgb(var(--k) / a)` (slash) que depende de CSS moderno. 2) **Partícula que orbita**: posicione o dot por `left/top` EM PX (não por `transform`), deixe o `transform` livre pro twinkle (`translateY+scale`) e gire só o wrapper pai (`inset-0`, `transform-origin: center`) — assim posição, órbita e flicker não conflitam. 3) **Heurística deve priorizar urgência**: ESCALATE (red) > CONFLICT (yellow) > EMIT_OK (green) > deliberating (roxo) > idle/retrieving (ciano). Escalada dominante (`escalation_rate>=0.5` OU `ESCALATE+RETRIEVAL_INSUFFICIENT > EMIT_OK`); conflito por `avg_confidence < 0.5` OU `EMIT_WITH_RESERVATIONS > EMIT_OK`; EMIT_OK por `self_resolve_rate>=0.5` E `EMIT_OK>esc` E `emit>0`. Sem payload → `idle` (erro) ou `retrieving` (loading inicial). 4) **`usePolling` só seta `loading=true` no mount/mudança de deps**, não a cada tick — então "retrieving" é estado transitório apenas do primeiro fetch; depois o estado deriva do dado. 5) **Metáfora visual**: cérebro (lattice SVG) + células periféricas (agentes, intactos em `AgentOrbit`) + cognição (deliberation) + decisão (EMIT_OK/ESCALATE) — separação limpa entre o núcleo e a periferia. 6) **Cores do tema da casa**: `skyline #38bdf8`/idle-retrieving, `moss #34d399`/emit_ok, `emberred #fb7185`/escalate, `cognac #e8a20c`/conflict, roxo deliberação `#c084fc`. |
| **Next** | Conectar a densidade/atividade dos neurônios a métricas reais (ex: número de evidências/posições por deliberação) para o lattice refletir intensidade em vez de só modo liga/desliga; considerar micro-rotulagem de partícula por estado; testar visual em telas estreitas (wrap do FlowRail). |

## Session: 2026-09-01 — SOLITEK frontend enterprise (os últimos CRUDs)

### 2026-09-01 — Camada de serviços (api + analytics + upload S3) e CRUDs enterprise
| Field | Value |
|-------|-------|
| **Agent** | cosca-frontend |
| **Task** | Transformar os CRUDs simples do SOLITEK (`apps/frontend`) em enterprise: dashboard com recharts, clientes/equipamentos/produtos/OS com busca+filtro+paginação+toasts, upload de foto via S3, exportação de CSV, timeline de OS e combobox reutilizável. Frontend Next.js 15 + React 19 + TS estrito + Tailwind + shadcn + TanStack Query + RHF + Zod. |
| **Technique** | Level 3 — **`api.postFormData`** no `lib/api.ts`: detecta `options.body instanceof FormData` e OMITE o `Content-Type` (é o browser quem monta o boundary multipart); `Content-Type: application/json` só para corpos JSON. **`analyticsApi`** tipado (OverviewDto/ByStatusRow/MonthlyOsPoint/MonthlyRevenuePoint/TopClientRow/TopEquipmentRow) e **`uploadsApi.uploadImage(kind, file)`** → `POST /uploads/{product|equipment}` devolve `{ key, url, thumbnailKey, thumbnailUrl, photoUrl }`. **Dashboard**: `useQueries` (batch de 7 endpoints analytics) + recharts v3 dentro de `ChartContainer` (contexto de tema via `useChartTheme`, eixo/grade com `gridColor`/`axisColor`) e `ChartTooltip` como `content`; kpis com `StatCard`; tooltip de receita formatado em BRL via `formatter={(v)=>brl.format(Number(v))}` (evita `any`, pois `Number(unknown)` é permitido). **`components/shared/combobox.tsx`** reutilizável (Popover+Command/cmdk) p/ seleção pesquisável; **`lib/csv.ts`** `exportToCsv` (BOM UTF-8 + escape RFC 4180, separador `;` p/ Excel BR, blob + link download). **Toasts sonner** em TODAS as mutações; **ConfirmDialog** p/ exclusões; estados loading/erro/vazio em toda página (PageSkeleton/ErrorState/EmptyState). |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.78** (subida por: `npm --workspace apps/frontend run typecheck` = PASS sem erros; zero `any`; nenhuma dep nova; FormData corretamente sem Content-Type; dashboard consumindo os 7 endpoints analytics sem `any`; upload substitui base64 por S3; CSV exportado client-side) |
| **Tags** | #frontend #nextjs #typescript #tanstack-query #recharts #sonner #zod #upload-s3 #csv #combobox #enterprise-crud #formdata #shadcn |
| **Related** | apps/frontend/src/lib/{api,services,types,csv}.ts, apps/frontend/src/components/shared/combobox.tsx, apps/frontend/src/app/(app)/{dashboard,clients,equipments,products,service-orders,service-orders/[id]}/page.tsx |
| **Learned** | 1) **FormData NUNCA com Content-Type manual**: para upload multipart, omita o header — o browser injeta `multipart/form-data; boundary=...`; setar `application/json` quebra o upload. Padrão: `isFormData ? {} : { "Content-Type": "application/json" }` no spread de headers. 2) **`useQueries` (arrays) para N endpoints simultâneos** (ex: os 7 do dashboard) — devolve array de `UseQueryResult`; combine `some(q=>q.isLoading)` p/ o estado agregado e `find(q=>q.isError)` p/ o erro; evita N `useQuery` separados e reduce o boilerplate. 3) **recharts v3 é estrito**: `content={<ChartTooltip/>}` usa um componente que recebe `active/label/payload` — não tipar pump; formatar valor com `formatter={(v)=>...Number(v)...}` (sem `(v: number)` que infere `unknown` e quebra contravariância); `tickFormatter` de eixo pode receber `(v: number)=>string` por contravariância de `any`. 4) **`exportToCsv`**: `;\uFEFF` BOM p/ o Excel abrir acentuação; escape `"`, `,`, `\n` (RFC 4180); `URL.createObjectURL` + `link.click()` + `revokeObjectURL`. 5) **Combobox shadcn**: `PopoverContent` com `w-[var(--radix-popover-trigger-width)]` garante largura igual ao trigger; `CommandItem value` precisa ser string única p/ o filtro do cmdk; `onSelect` fecha o popover. 6) **Schema add `photoUrl` ao `Equipment`** (já existia em Product) — o backend de upload persiste `photoUrl`; preview local usa `res.photoUrl || res.url`. 7) **Dialog para alterar status** (em vez de botões inline) + description opcional — melhor UX enterprise e permitido pelo `updateStatus(id, status, description)`. 8) A página detail reutiliza o `EVENT_META` (mesmo vocabulário da page track pública) p/ a timeline vertical com dots coloridos por evento. |
| **Next** | Level 3 para acesso real: rodar `next build` (build de produção) e `next lint`; considerar debounce na busca; acessibilidade (aria-label, foco, contraste) nas tabelas/dialogs; adicionar testes (Vitest) para `csv.ts` e `api.postFormData`. |
