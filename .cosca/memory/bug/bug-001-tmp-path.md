---
type: bug
key: bug-001-tmp-path
tags: [tmp, path, linux, daemon, cross-platform]
severity: medium
timestamp: 2026-07-25T00:00:00Z
detected_in: cosca
fixed_in: commit c30fac3
confidence: 1.0
times_seen: 1
---

# Bug-001: Hardcoded /tmp Path Breaks Portability

## Symptoms
- Daemon PID file written to `/tmp/cosca.pid`
- Tests creating temp directories in `/tmp/`
- Cross-platform issues on Windows (no /tmp)
- Permission issues on restricted Linux systems

## Causality Tree

### N1 — Causa Direta
Hardcoded absolute paths (`/tmp/cosca.pid`, `os.MkdirTemp("")`) em 3 arquivos:
- `internal/runtime/daemon.go` — PID file path fixo
- `internal/runtime/daemon_test.go` — temp dir no sistema
- Múltiplos arquivos de teste — mesmo padrão

### N2 — Causa Arquitetural
O runtime não tinha uma abstração de "project temp directory". Cada módulo decidia seu próprio path, sem contrato. A arquitetura não definia onde recursos de runtime (PID, sockets, temp files) devem viver.

### N3 — Causa de Processo
- CI testava apenas Linux — paths `/tmp/` funcionavam, escondendo o problema
- Nenhuma lint rule proibia paths absolutos
- Review de PR não questionou portabilidade de paths

### N4 — Prevenção Sistêmica
- **CI gate:** `go test -race` em matrix Linux + macOS + Windows
- **Lint rule:** proibir `/tmp/`, `/var/`, `C:\` em código Go (via `forbidigo` ou `revive`)
- **Template:** `t.TempDir()` como padrão em todos os testes (Go 1.15+)
- **Padrão de projeto:** `filepath.Join(".", "tmp", ...)` documentado no CONTRIBUTING.md

## Detection Pattern
```bash
rg '/tmp/cosca' --type go
rg 'os.MkdirTemp("")' --type go
```

## Fix Applied (c30fac3)
1. `daemon.go`: `/tmp/cosca.pid` → `./tmp/cosca.pid`
2. All tests: `os.MkdirTemp("")` → `os.MkdirTemp(".")`
3. All tests: `/tmp/cosca*` → `./tmp/cosca*`
4. `.gitignore`: ensured `tmp/` is ignored

## Affected Files
- `internal/runtime/daemon.go`
- `internal/runtime/daemon_test.go`
- Multiple test files
