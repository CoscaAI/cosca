# cosca-mobile — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### P001: Mobile Platform Audit Checklist
**Discovered**: 2026-07-28 | **Context**: CoscaAI activation audit
**Tags**: #audit #methodology #mobile-readiness

**Pattern**: When assessing a platform for mobile readiness, systematically check:
1. API compatibility (auth methods, response format, error handling, streaming support)
2. SDK compatibility (HTTP client, tree-shaking, bundle size, platform targets)
3. Build infrastructure (CI/CD for mobile, app store configs, signing)
4. Mobile-specific concerns (push notifications, offline support, biometrics, deep linking)
5. Frontend framework (web PWA vs native vs cross-platform gap analysis)
6. Governance (ADRs, roadmap mentions, organizational readiness)

**Score formula**: API compat (30%) + SDK compat (25%) + Infra (20%) + Mobile features (15%) + Governance (10%)

### P002: HTTP Client Abstraction for Cross-Platform SDK
**Discovered**: 2026-07-28 | **Context**: @cosca/sdk axios→fetch analysis
**Tags**: #sdk #http-client #cross-platform #fetch-adapter

**Pattern**: When designing SDKs that need to run on Node.js, browser, AND React Native:
1. Do NOT use axios (node-only `http` module dependency, polyfill overhead)
2. Use the `fetch` API as the baseline (available in all environments since Node 18, React Native 0.71+)
3. Implement retry logic as a wrapper function (not interceptor pattern — cleaner tree-shaking)
4. Use a simple adapter pattern: `createHttpClient(options) → { get, post, put, delete }`
5. Keep error normalization as standalone functions (not class methods on interceptors)
6. Bundle size target: <5KB gzipped for the HTTP layer
