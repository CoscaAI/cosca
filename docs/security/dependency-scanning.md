# Dependency Security Scanning

> **Status**: active | **Owner**: Security | **Last Updated**: 2026-08-11

Cosca scans project dependencies for known vulnerabilities against the
**OSV.dev** vulnerability database using Google's **osv-scanner** (Apache-2.0).
This document explains how to use `cosca security scan`, how results are
interpreted, and how the scan is wired into `cosca doctor` and `cosca qgate`.

---

## The OSV database

[OSV.dev](https://osv.dev) is Google's open-source vulnerability database. It
aggregates advisories from many sources (GitHub Security Advisories, Go
vulnerability database, npm, PyPI, RustSec, CVE Program, and more) into a
single, machine-readable schema. Every advisory has a stable identifier such
as:

- `CVE-2026-12345` — Common Vulnerabilities and Exposures
- `GHSA-xxxx-yyyy-zzzz` — GitHub Security Advisory
- `GO-2026-1234` — Go vulnerability database
- `OSV-2026-123` — OSV native advisory

Each advisory includes the affected package name, version ranges, a summary,
references, and — when available — CVSS severity vectors.

## `cosca security scan`

```
cosca security scan [dir] [--severity low|medium|high|critical]
                         [--recursive] [--exit-zero] [--json]
```

The command discovers dependency files in the target directory (default:
current dir) and queries OSV.dev for known vulnerabilities.

### Output

For each vulnerable package the scan prints:

```
  brace-expansion@1.1.16
    ecosystem=npm source=web/pnpm-lock.yaml
    [HIGH] GHSA-mh99-v99m-4gvg — brace-expansion: DoS via unbounded expansion length
      https://osv.dev/vulnerability/GHSA-mh99-v99m-4gvg
```

A summary of vulnerabilities by severity is printed at the end:

```
  CRITICAL  2
  HIGH      35
  MEDIUM    18
  LOW       3
  UNKNOWN   56
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0    | Scan completed, no CRITICAL/HIGH vulnerabilities (or `--exit-zero`) |
| 1    | CRITICAL or HIGH vulnerabilities found |
| 2    | Scan error (network failure, invalid arguments, ...) |

Use `--exit-zero` in CI pipelines that only want the report, not the gate.

### Flags

| Flag | Description |
|------|-------------|
| `--severity` | Minimum severity to report: `low` (default), `medium`, `high`, `critical`. Vulnerabilities without a CVSS rating (`UNKNOWN`) are always reported. |
| `--recursive` | Recursively scan subdirectories for dependency files (default `true`). |
| `--exit-zero` | Always exit with status 0 (CI friendly). |
| `--json` | Emit the full report as JSON (`-j` / `--json` global flag). |

### Supported dependency files

The scan detects lockfiles and manifests across ecosystems, including:
`go.mod`, `go.sum`, `package-lock.json`, `npm-shrinkwrap.json`,
`yarn.lock`, `pnpm-lock.yaml`, `requirements.txt`, `Pipfile.lock`,
`poetry.lock`, `Gemfile.lock`, `Cargo.lock`, `composer.lock`, `pom.xml`,
`build.gradle`, and others.

### JSON output

`--json` emits a structured report:

```json
{
  "dir": ".",
  "sources": 4,
  "packages_scanned": 1598,
  "vulnerable_packages": 32,
  "total_vulnerabilities": 114,
  "severity_counts": {"CRITICAL": 2, "HIGH": 35, "LOW": 3, "MEDIUM": 18, "UNKNOWN": 56},
  "vulnerabilities": [
    {
      "package": "brace-expansion",
      "version": "1.1.16",
      "ecosystem": "npm",
      "source": "web/pnpm-lock.yaml",
      "vulnerabilities": [
        {
          "id": "GHSA-mh99-v99m-4gvg",
          "aliases": ["CVE-2026-14257"],
          "severity": "HIGH",
          "score": "7.5",
          "summary": "brace-expansion: DoS via unbounded expansion length...",
          "url": "https://osv.dev/vulnerability/GHSA-mh99-v99m-4gvg"
        }
      ]
    }
  ]
}
```

## Interpreting results

Severity levels follow the CVSS ratings:

| Severity | CVSS score range |
|----------|------------------|
| CRITICAL | 9.0 – 10.0 |
| HIGH     | 7.0 – 8.9 |
| MEDIUM   | 4.0 – 6.9 |
| LOW      | 0.1 – 3.9 |
| UNKNOWN  | no CVSS vector published |

Guidance:

- **CRITICAL / HIGH** — block merges and deployments (`cosca qgate` blocks the
  commit). Upgrade the affected package to a fixed version as soon as possible.
- **MEDIUM / LOW** — plan remediation, no immediate action required.
- **UNKNOWN** — the advisory carries no CVSS vector. The advisory is still a
  real finding; review it manually via the OSV URL.

## Integration

### `cosca doctor`

`cosca doctor` includes a **Security** check that scans the workspace root for
dependency vulnerabilities. It reports the package count and vulnerability
count, and only *fails* the diagnostic when CRITICAL/HIGH vulnerabilities are
found. Network outages degrade to a warning so `doctor` still works offline.

Run it standalone with `cosca doctor security`.

### `cosca qgate` (pre-commit quality gate)

`cosca qgate` runs the OSV scan as part of the pre-commit gate. If the
workspace has **CRITICAL or HIGH** dependency vulnerabilities, the gate blocks
the commit:

```
  BLOCKED: ... 11 CRITICAL/HIGH DEPENDENCY VULNERABILITIES
```

To skip the scan (e.g. offline CI or a deliberately accepted risk):

```
cosca qgate --skip-security-vulns
```

## Implementation notes

- The scan engine lives in `internal/security` and is shared by `security
  scan`, `doctor`, and `qgate`.
- `cosca security scan` runs outside the runtime jail so it has network access
  to query the OSV API.
- Severity is derived from CVSS vectors using `github.com/pandatix/go-cvss`,
  mirroring osv-scanner's own severity logic.
- Advisory URL: `https://osv.dev/vulnerability/<id>`.
