# Lighthouse Baseline — Cosca Web Console
> Date: 2026-07-27 | Build: production

## Target Metrics
| Metric | Target |
|--------|--------|
| Performance | >90 |
| Accessibility | >95 |
| Best Practices | >90 |
| SEO | >90 |

## Optimizations Applied
- PPR (incremental) for static shell + dynamic content *(commented out — requires Next.js canary)*
- ISR on Settings page (300s revalidate)
- Dynamic imports for recharts, react-markdown (SSR disabled)
- TanStack Query cache tuning (17 hooks)
- optimizePackageImports for Radix UI, lucide-react, recharts, framer-motion
- Inter + JetBrains Mono with display:swap

## Key Pages (to be measured on deployment)
| Page | Performance | Accessibility | Best Practices | SEO |
|------|-------------|---------------|----------------|-----|
| /dashboard | — | — | — | — |
| /knowledge | — | — | — | — |
| /orchestration | — | — | — | — |
| /playground | — | — | — | — |
| /settings | — | — | — | — |
