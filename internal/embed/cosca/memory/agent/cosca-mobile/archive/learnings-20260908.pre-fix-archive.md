# cosca-mobile - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-mobile — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-mobile |
| **Task** | Initial capability establishment |
| **Technique** | Standard mobile patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #mobile #baseline #initialization |
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core mobile patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Active Learnings

### 2026-07-28 — ONDA 5 Activation — Deep Platform Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-mobile |
| **Task** | Full-stack mobile readiness audit of CoscaAI platform |
| **Technique** | Systematic analysis of API, SDK, architecture, ADRs, and mobile infrastructure |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #activation #audit #ondas #platform-analysis #sdk-mobile-incompatibility |
| **Related** | ADR-007, ADR-2745, api/rest/openapi.yaml, sdk/typescript/src/client.ts |
| **Learned** | 1) CoscaAI is a Go backend + TS SDK + Next.js web console. NO mobile code exists. 2) The `@cosca/sdk` uses axios — incompatible with React Native and native mobile. 3) The REST API (52 endpoints, JWT + API Key auth) is fully mobile-compatible. 4) gRPC protos exist but no gRPC-Web or mobile stubs. 5) The embed framework has full mobile department, template (2.0.0), and React Native audit skill — ready for mobile project generation. 6) Platform maturity for mobile is 1/5 — concepts and templates exist, but zero implementation. 7) No ADR mentions mobile. Mobile is listed in the platform overview as "can be built" but no plan exists. |
| **Next** | 1) Propose axios→fetch migration in `@cosca/sdk` (unblocks React Native). 2) Draft Mobile ADR. 3) Prototype CoscaAI Mobile Dashboard (Expo + @cosca/sdk). |

### 2026-07-28 — SDK Mobile Compatibility Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-mobile |
| **Task** | Analyze `@cosca/sdk` for mobile compatibility |
| **Technique** | Source code audit of sdk/typescript/src/client.ts — HTTP client dependency analysis |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #sdk #axios #react-native #compatibility #http-client |
| **Related** | sdk/typescript/src/client.ts, sdk/typescript/package.json |
| **Learned** | The `@cosca/sdk` uses axios as its HTTP client (line 4: `import axios`). This makes it incompatible with React Native because: 1) axios uses Node.js `http` module internally — React Native's JavaScript engine doesn't have it. 2) Even with polyfills, axios adds ~15KB to bundle size. 3) The `@cosca/sdk` has `"engines": {"node": ">=18.0.0"}` — explicitly node-only. 4) Axios interceptors (line 154) use patterns that don't translate to React Native's fetch API. 5) Axios error handling (line 274) uses `axios.isAxiosError()` — a tree-shaking blocker. The solution is to refactor the SDK to use a fetch adapter pattern (like ky or a custom fetch wrapper) that works in Node.js, browser, and React Native. This is a P0 blocker for any mobile app that wants to use the Cosca API directly. |
| **Next** | Draft migration plan: fetch adapter with retry + interceptors. Coordinate with SDK Chief (cosca-sdk). |

