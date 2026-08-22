# Activation Report — cosca-release (Release Chief)

> **Date**: 2026-07-28 | **Status**: ATIVADO | **Level**: L1→L2 | **Confidence**: 0.75

---

## 1. Versioning Audit

### Source of Truth: `pkg/cosca/cosca.go`
```
Version        = "1.0.0-rc.1"    ← STALE — should be "1.4.0-dev"
ReleaseChannel = "dev"            ← correct
```

### Build-time Override (ldflags)
The version is **overridden at build time** by ldflags in Makefile, GoReleaser, and Dockerfile:
```
-X 'github.com/CoscaAI/cosca/pkg/cosca.Version={{ .Version }}'
```
So the binary reports the correct version **when built through the proper pipeline**. But the hardcoded default is dangerously misleading.

### Consistency Cross-Check

| Source | Version | Status |
|--------|---------|--------|
| `pkg/cosca/cosca.go` | `1.0.0-rc.1` | ❌ Stale |
| `CHANGELOG.md` | `1.4.0-dev` | ✅ Current |
| `go.mod` | `go 1.25.0` | ✅ Current |
| CI header comment | `v1.4.0-dev` | ✅ Current |
| Git tags | **(none)** | ❌ No tags for any version |
| `git describe` | `3971856-dirty` | ⚠️ Working copy, no tags |

**Verdict**: Semantic versioning is broken at the source level. The hardcoded constant in `pkg/cosca/cosca.go` is 3 minor versions behind the actual development state. This creates risk if someone builds without ldflags.

---

## 2. Build Pipeline Audit

### GoReleaser (`.goreleaser.yaml`)
| Aspect | Status | Notes |
|--------|--------|-------|
| Cross-platform builds | ✅ | linux/darwin/windows, amd64/arm64 |
| ldflags injection | ✅ | Version, CommitHash, BuildDate |
| Changelog filtering | ✅ | Excludes docs/test/ci/merge commits |
| Checksums | ✅ | `checksums.txt` |
| **GitHub repo reference** | ❌ | `github.com/cosca/cli` (line 58-59) — should be `github.com/CoscaAI/cosca` |
| Snapshot naming | ⚠️ | Uses `{{ .Tag }}-next` — requires proper tag |

### Dockerfile (Root)
- ✅ Multi-stage (scratch, ~12MB)
- ✅ Build args for VERSION/COMMIT_HASH/BUILD_DATE
- ✅ Ports exposed (14120, 14121)
- ✅ .dockerignore exists

### Dockerfile (Web)
- ✅ Multi-stage (Node 22-alpine)
- ✅ pnpm, standalone output
- ✅ Non-root user (nextjs)

### Makefile
- ✅ Comprehensive: 30+ targets
- ✅ Version auto-detection from git (`git describe --tags --always --dirty`)
- ✅ Cross-compilation via `build-all`
- ✅ Test suites (unit, integration, e2e, race, coverage, benchmark)
- ✅ Lint, vet, fmt, tidy, docs, proto
- ❌ **Missing `release` target** — no automated tag/bump workflow
- ❌ **No CHANGELOG validation target**

### GitHub Actions

#### CI (`ci.yml`)
- 7 quality gates: Build (G0), Lint (G1), Vet (G2), Test (G3), Security (G4), Coverage (G5), Docs (G6)
- Well-organized with parallel jobs
- Coverage threshold at 55% (below documented 70% goal)
- ✅ Docker build smoke test
- ✅ gosec severity filtering

#### CD (`cd.yml`)
- Trigger: tag push `v*`
- Stage 1: CI re-verify
- Stage 2: Multi-platform Docker build + push to GHCR
- Stage 3: GitHub Release via GoReleaser
- Stage 4: Deploy (placeholder)
- ✅ Proper permissions (contents: write, packages: write)
- ✅ Build args passed through

#### CI Runbook (`CI.md`)
- ✅ Excellent documentation: triggers, quality gates, local validation, troubleshooting, secrets

---

## 3. Release Process Assessment

### Current Process
```
1. Developer manually creates git tag (e.g., v1.4.0)
2. Developer pushes tag to GitHub
3. CD pipeline triggers automatically:
   a. CI re-verify (re-runs all gates)
   b. Docker build & push (linux/amd64 + linux/arm64)
   c. GoReleaser creates GitHub Release with binaries
   d. Deploy step exists as placeholder only
```

### Gaps Identified
1. **No automated version bump** — no script/target updates `pkg/cosca/cosca.go` version constant
2. **No CHANGELOG verification** — no check that CHANGELOG is updated for the new version
3. **No release candidate flow** — no pre-release/prerelease testing pipeline
4. **No tag automation** — tags are created manually, prone to human error
5. **No dry-run capability** — no way to preview what a release would do without executing
6. **No rollback procedure documented** — deploy step is placeholder with no rollback strategy
7. **Deploy is a placeholder** — Stage 4 in CD pipeline just echoes instructions

---

## 4. CHANGELOG Audit

### Product CHANGELOG (`CHANGELOG.md`)
- ✅ Follows Keep a Changelog loosely
- ✅ Dates present for all entries
- ✅ Well-structured: Added, Changed, Fixed, Security, Quality sections
- ✅ Lists changed files for detailed tracking
- ✅ Known issues documented transparently
- ❌ **Versions in CHANGELOG have no corresponding git tags**: v1.0.0-rc.1, v1.1.0, v1.1.1, v1.2.0, v1.3.0, v1.4.0-dev
- ❌ **No `Unreleased` section** per Keep a Changelog spec
- ⚠️ `v1.4.0-dev` is a development version, not a release — mixing dev and release versions

### Framework CHANGELOG (`internal/embed/cosca/CHANGELOG.md`)
- Separate versioning: 3.0.1, 3.0.0, 2.0.0, 1.x
- Internal framework tracking, not related to product releases
- ✅ Properly maintained

---

## 5. Recommendations

### Recommendation 1: Fix Version Constant Drift (Critical)
**Problem**: `pkg/cosca/cosca.go` hardcodes `Version = "1.0.0-rc.1"` — 3 versions behind.

**Action**:
- Update the constant to match the current development state: `Version = "1.4.0-dev"`
- Add a comment warning that this is the **fallback default** and is overridden by ldflags in production builds
- Alternatively, detect at init() time if ldflags were set and log a warning if using the fallback

**Risk if unaddressed**: A developer running `go run` or `go build` without the Makefile gets an incorrect version string, potentially causing confusion in bug reports or telemetry.

### Recommendation 2: Fix GoReleaser GitHub Reference (High)
**Problem**: `.goreleaser.yaml` line 58-59:
```yaml
release:
  github:
    owner: cosca
    name: cli
```
This references `github.com/cosca/cli` instead of `github.com/CoscaAI/cosca`.

**Action**: Change to:
```yaml
release:
  github:
    owner: CoscaAI
    name: cosca
```

**Risk if unaddressed**: GoReleaser will attempt to publish the release to the wrong repository, causing CD pipeline failure.

### Recommendation 3: Add `make release` Workflow (Medium)
**Problem**: Release process is entirely manual — no automation for tag creation, version bump, or CHANGELOG verification.

**Action**: Create a `make release` target that:
1. Validates working directory is clean (`git diff --stat --exit-code`)
2. Validates CHANGELOG has an entry for the target version
3. Updates the hardcoded version in `pkg/cosca/cosca.go`
4. Optionally runs tests, lint, build
5. Creates an annotated git tag (`git tag -a v{VERSION} -m "Release v{VERSION}"`)
6. Optional: pushes tag to origin
7. Supports `DRY_RUN=1` for preview

Plus: add `make release-dry-run` as a safe preview.

---

## Summary

| Domain | Verdict | Confidence |
|--------|---------|-----------|
| Versioning | ⚠️ Broken (stale constant, no tags) | 0.30 |
| Build Pipeline | ✅ Solid (GoReleaser, Docker, Makefile, CI/CD) | 0.85 |
| Release Process | ❌ Manual, no automation | 0.25 |
| CHANGELOG | ⚠️ Well-written but untagged | 0.60 |
| **Overall** | **Needs 3 targeted fixes** | **0.75** |

**Overall Assessment**: The Cosca project has an excellent CI/CD infrastructure (GoReleaser, multi-platform Docker, quality gates, runbook). The critical gaps are in version consistency (Recommendation 1), a wrong GoReleaser repo path that would break releases (Recommendation 2), and the absence of automated release orchestration (Recommendation 3). Fixing these three items will bring the release process from Level 1 (manual/ad-hoc) to Level 3 (automated/validated).

---

## Files Examined
- `pkg/cosca/cosca.go` — version constants
- `cmd/cosca/main.go` — build info initialization
- `.goreleaser.yaml` — GoReleaser configuration
- `Makefile` — build automation
- `Dockerfile` — backend container build
- `web/Dockerfile` — frontend container build
- `CHANGELOG.md` — product changelog
- `internal/embed/cosca/CHANGELOG.md` — framework changelog
- `.github/workflows/ci.yml` — CI pipeline
- `.github/workflows/cd.yml` — CD pipeline
- `.github/workflows/CI.md` — CI/CD runbook
- `go.mod` — Go module definition
