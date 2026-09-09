# cosca-release - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-release — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-release |
| **Task** | Initial capability establishment |
| **Technique** | Standard release patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #release #baseline #initialization |
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core release patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Release Audit: Full Pipeline Assessment
| Field | Value |
|-------|-------|
| **Agent** | cosca-release |
| **Task** | Activation audit — versioning, build pipeline, release process, CHANGELOG |
| **Technique** | Multi-layered release audit — cross-referencing source version constants, git tags, CI/CD config, and changelog |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #release #audit #versioning #goreleaser #changelog #ci-cd #level-up |
| **Related** | internal/embed/cosca/memory/release/activation-report.md, pkg/cosca/cosca.go, .goreleaser.yaml, CHANGELOG.md, .github/workflows/cd.yml |
| **Learned** | Three critical findings: (1) `pkg/cosca/cosca.go` hardcodes `Version = "1.0.0-rc.1"` while CHANGELOG reports v1.4.0-dev — constant drift can silently break version injection if ldflags are not set. (2) `.goreleaser.yaml` references `github.com/cosca/cli` as the GitHub release repo (line 58-59), not `github.com/CoscaAI/cosca` — this would publish releases to the wrong repository. (3) No git tags exist for any CHANGELOG versions (v1.4.0-dev, v1.3.0, v1.2.0, etc.) — the release process is entirely manual with no automation for tag creation or version bumping. |
| **Next** | Level 3: Design a formal `make release` target with automated version bump, tag creation, CHANGELOG verification, and dry-run support. |

