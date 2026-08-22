# cosca-devops — Reusable Patterns

> Discovered patterns that can be reapplied. Grows with agent experience.

## Patterns Discovered

### Pattern: Package-local E2E delegation

When E2E tests and their runner configuration live under a frontend package, keep the root Make target as a thin delegate (`pnpm --dir <package> test:e2e`). This preserves the package's Playwright `testDir`, web-server lifecycle, and dependency context while avoiding duplicate runner flags in Make.

### Pattern: Optional test-suite prerequisite

When a test suite may not be checked out in every environment, expose it as a dedicated Make target guarded by `[ -d <path> ]`, then make the aggregate target depend on it. The aggregate remains stable while the suite becomes executable automatically when present.
