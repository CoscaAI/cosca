# Cosca CLI Audit Report

> **Date**: 2026-07-28 | **Auditor**: cosca-cli | **Level**: 1 → 2  
> **Scope**: Full CLI command coverage, UX, and gap analysis

---

## Executive Summary

| Metric | Value |
|--------|-------|
| Top-level commands | **37** (all registered in `root.go`) |
| Leaf commands (total) | **105** (counting `memory snapshot {create,list,restore}` as 3) |
| Build status | ✅ Compiles cleanly |
| Test status | ✅ `go test ./internal/cli/ -short` passes |
| Commands with real implementation | **~90** (~85%) |
| Commands with stub/placeholder output | **~15** (~15%) |
| Overall assessment | **Functional. Solid foundation. UX improvements needed.** |

---

## 1. Command Inventory

### 1.1 Top-Level Commands (37/37 — All Verified)

| # | Command | Status | Has Subcommands | Notes |
|---|---------|--------|-----------------|-------|
| 1 | `agent` | ✅ | 5 | Full implementation via agent manager |
| 2 | `benchmark` | ✅ | 0 | Has flags for skip-search/index/memory |
| 3 | `bootstrap` | ✅ | 0 | Uses runtime adapter |
| 4 | `cache` | ✅ | 4 (clear, warm, stats, inspect) | Cache adapter connects to real cache |
| 5 | `chat` | ✅ | 0 | Full interactive session with orchestration |
| 6 | `completion` | ✅ | 0 | Shell completion (bash/zsh/fish/powershell) |
| 7 | `config` | ✅ | 6 (get, set, list, edit, reset, validate) | Full implementation |
| 8 | `context` | ✅ | 4 (build, show, clear, stats) | Uses context builder adapter |
| 9 | `docs` | ✅ | 0 | Opens browser |
| 10 | `doctor` | ✅ | 0 | Full diagnostic with subsystem filtering |
| 11 | `editor` | ✅ | 3 (setup, status, adapt) | Editor integration |
| 12 | `graph` | ✅ | 4 (show, query, stats, export) | Knowledge graph |
| 13 | `health` | ✅ | 0 | Basic health check (works) |
| 14 | `index` | ✅ | 5 (rebuild, update, status, stats, verify) | Filesystem scanner adapter |
| 15 | `init` | ✅ | 0 | Creates .cosca/ structure, detects env |
| 16 | `install` | ✅ | 0 | Full 14-step install pipeline |
| 17 | `knowledge` | ✅ | 9 | Knowledge engine with benchmark, explain, vacuum |
| 18 | `memory` | ✅ | 6 + 3 snapshot | Memory engine with snapshots |
| 19 | `metrics` | ✅ | 0 | Orchestration metrics |
| 20 | `pipeline` | ✅ | 2 (list, run) | Pipeline execution |
| 21 | `plugin` | ✅ | 8 | Plugin lifecycle management |
| 22 | `prompt` | ✅ | 4 (list, show, search, create) | Prompt management |
| 23 | `provider` | ✅ | 5 (list, set, test, info, watch) | Provider management |
| 24 | `run` | ✅ | 0 | Full AI orchestration with streaming, memory, semantic routing |
| 25 | `runtime` | ⚠️ | 6 (status, start, stop, restart, logs, info) | Most work; `logs` returns placeholder |
| 26 | `search` | ✅ | 0 | Cross-content search |
| 27 | `serve` | ✅ | 0 | Full REST + gRPC + WebSocket server |
| 28 | `skill` | ✅ | 4 (list, show, search, install) | Skill management |
| 29 | `status` | ✅ | 0 | Full system status |
| 30 | `sync` | ✅ | 0 | Incremental/full index sync |
| 31 | `template` | ✅ | 4 (list, show, search, create) | Template management |
| 32 | `uninstall` | ✅ | 0 | Clean uninstall with dry-run |
| 33 | `update` | ✅ | 0 | Channel-based update check |
| 34 | `upgrade` | ✅ | 0 | Config migration |
| 35 | `validate` | ✅ | 0 | Project validation |
| 36 | `version` | ✅ | 0 | Version info with JSON support |
| 37 | `workflow` | ✅ | 4 (list, show, run, search) | Workflow management |

**Legend**: ✅ = Functional | ⚠️ = Functional with caveat | ❌ = Broken

### 1.2 Full Leaf Command Count

Top-level (no subcommands): 16  
Subcommands: 89 (including memory snapshot sub-sub-commands)  
**Total leaf commands**: 105

---

## 2. UX Audit

### 2.1 Critical Issues

| Severity | Issue | Affected Commands | Recommendation |
|----------|-------|-------------------|----------------|
| 🔴 HIGH | **No command aliases** | All | Add `Aliases:` to every command with common abbreviations (e.g., `ls` for `list`, `rm` for `uninstall`, `cfg` for `config`, `kb` for `knowledge`) |
| 🔴 HIGH | **`-v` / `-V` reversal from Unix convention** | Root (persistent flags) | `-v` = `--verbose` (standard), `-V` = `--version` (standard). Currently swapped. |
| 🟡 MEDIUM | **`--all` flag unused in `cache clear`** | `cache clear` | The `--all` flag is registered but its value is never checked in RunE |
| 🟡 MEDIUM | **`runtime logs` returns placeholder** | `runtime` | Returns `[runtime adapter] no logs available` — should connect to real log store |
| 🟡 MEDIUM | **No command grouping** | Root | Use `cobra.Command.GroupID` to organize commands: "Resource Management", "System", "AI Engine", etc. |
| 🟡 MEDIUM | **`--dry-run` short flag inconsistency** | `sync`, `uninstall`, `memory prune` | `sync --dry-run` has `-n` shorthand; `uninstall --dry-run` and `memory prune --dry-run` do NOT |
| 🟢 LOW | **No color for JSON output on some commands** | `version` | `version --json` works but `formatKey`/`formatValue` still inject ANSI codes in some code paths |
| 🟢 LOW | **Missing shell completion hints** | Most subcommands | Only `completion` has `ValidArgsFunction`; add them for commands like `provider`, `cache`, etc. |

### 2.2 Flag Consistency

| Flag | Commands That Have It | Inconsistency |
|------|----------------------|---------------|
| `--dry-run` / `-n` | `sync`, `uninstall`, `memory prune`, `run` | `sync` has `-n` shorthand; others don't |
| `--force` / `-f` | `init`, `config reset` | Consistent |
| `--global` | `install` | Only one command has it |
| `--limit` / `-l` | `search`, `memory list`, `memory search` | Consistent |
| `--type` / `-t` | `memory list`, `memory search` | Consistent |
| `--json` / `-j` | Global flag | Consistent ✅ |

### 2.3 Output Formatting

| Aspect | Status | Notes |
|--------|--------|-------|
| Color scheme | ✅ Good | ANSI colors via `OutputFormatter`, respects `NO_COLOR` env |
| `--no-color` | ✅ Good | Registered as persistent flag |
| `--json` / `-j` | ✅ Good | Most commands support it |
| `--format text/json/yaml/table` | ✅ Good | Registered but YAML/table effectively unused |
| `--quiet` / `-q` | ⚠️ Partial | Flag exists but no `formatter.Quiet` checks in commands |
| `--verbose` / `-V` | ⚠️ Partial | Some commands use `formatter.Verbose()`; inconsistent |

---

## 3. Implementation Quality

### 3.1 Adapter Layer Assessment

The `internal/cli/adapters_*.go` files provide a bridge between CLI command signatures and internal packages.

| Adapter | File | Quality | Notes |
|---------|------|---------|-------|
| Memory | `adapters_memory_adapter.go` | ✅ Good | Full chain: list, get, search, snapshot, prune, stats |
| Indexer | `adapters_indexer_adapter.go` | ⚠️ Partial | Scans filesystem for counts; no real index database. Good for status but rebuild/update don't create a real index. |
| Runtime | `adapters_runtime_adapter.go` | ⚠️ Partial | Start/stop/restart work; logs and info are placeholders |
| Knowledge | `adapters_knowledge_adapter.go` | ✅ Good | Database-backed knowledge engine |
| Graph | `adapters_graph_adapter.go` | ✅ Good | Knowledge graph with relationships |
| Providers | `adapters_providers_adapter.go` | ✅ Good | Provider listing with status |
| Plugins | `adapters_plugins_adapter.go` | ✅ Good | Plugin lifecycle |
| Editors | `adapters_editors_adapter.go` | ✅ Good | Editor detection and setup |
| Installer | `adapters_installer_validator_adapter.go` | ⚠️ Partial | Some installer steps are stubs |
| Diagnostics | `adapters_diagnostics_adapter.go` | ✅ Good | Doctor subsystem checks |

### 3.2 Commands with Real vs. Stub Output

**Real implementations** (connected to internal engines): `agent`, `cache`, `chat`, `config`, `doctor`, `editor`, `graph`, `health`, `init`, `install`, `knowledge`, `memory`, `pipeline`, `plugin`, `provider`, `run`, `serve`, `skill`, `status`, `template`, `uninstall`, `validate`, `version`, `workflow`

**Partial/stub output**: `runtime logs` (placeholder), `runtime info` (hardcoded), `context build` (context builder placeholder), `index rebuild/update` (filesystem scan only, no real DB), `cache warm` (no-op), `benchmark` (skeleton)

---

## 4. Missing / Suggested Commands

### 4.1 High Priority

| Command | Rationale | Suggested `Use` |
|---------|-----------|-----------------|
| `cosca backup` | No way to backup `.cosca/` state | `cosca backup [--output <path>]` |
| `cosca restore` | Complements backup | `cosca restore <backup-file>` |
| `cosca logs` (top-level) | Aggregate logs from all subsystems | `cosca logs [--follow] [--tail N]` |
| `cosca env` | Show environment info (Go, OS, providers, paths) | `cosca env [--json]` |

### 4.2 Medium Priority

| Command | Rationale | Suggested `Use` |
|---------|-----------|-----------------|
| `cosca shell` | Start a Cosca-managed shell session | `cosca shell [--agent <name>]` |
| `cosca watch` | Watch filesystem for changes, auto-sync | `cosca watch [--debounce 500ms]` |
| `cosca export` | Export knowledge/memory/config in various formats | `cosca export [--type knowledge|memory|config] [--format json|yaml|csv]` |
| `cosca import` | Import data into knowledge/memory | `cosca import <file> [--type knowledge|memory]` |

### 4.3 Low Priority / Nice-to-Have

| Command | Rationale |
|---------|-----------|
| `cosca completion --install` | Auto-install completion for current shell |
| `cosca stats` (top-level) | Unified stats (knowledge + memory + cache + index) |
| `cosca reset` (top-level) | Reset entire system state |
| `cosca dashboard` | Open local web dashboard |
| `cosca migrate` | Database migration management |

---

## 5. Quick Wins

These are low-effort, high-impact improvements:

1. **Add aliases to all commands** — Estimated 15min. Example:
   ```go
   cmd := &cobra.Command{
       Use:     "knowledge",
       Aliases: []string{"kb", "kn"},
       ...
   }
   ```

2. **Fix `-v`/`-V` swap** — Change short flags: `-v` → verbose, `-V` → version (standard convention).

3. **Add command grouping** — Use `cmd.GroupID`:
   ```go
   rootCmd.AddGroup(&cobra.Group{ID: "resource", Title: "Resource Management:"})
   rootCmd.AddGroup(&cobra.Group{ID: "system", Title: "System Operations:"})
   rootCmd.AddGroup(&cobra.Group{ID: "ai", Title: "AI Engine:"})
   ```

4. **Consistent `--dry-run` shorthand** — Add `-n` to `uninstall --dry-run` and `memory prune --dry-run`.

5. **Add shell completion hints** — Add `ValidArgsFunction` to commands that accept typed arguments (`provider set`, `agent show`, `cache inspect`, etc.).

6. **Add `--quiet` support** — Check `formatter.Quiet()` in non-critical output paths.

7. **Fix `cache clear --all`** — Actually use the `all` variable in RunE to clear persistent caches too.

8. **Improve `runtime logs`** — Connect to actual log store or provide clear message about log configuration.

---

## 6. Test Coverage

| Test File | Lines | Focus |
|-----------|-------|-------|
| `cli_test.go` | ~900+ | Command construction, flags, adapter tests |
| `cli_coverage_test.go` | ~3241 | Comprehensive coverage targeting <70% gaps |
| `cli_root_test.go` | ~400+ | Root command, global flags, config |
| `cli_completion_test.go` | ~100 | Completion command |
| `cli_config_test.go` | ~250+ | Config commands |
| `cli_doctor_test.go` | ~500+ | Doctor diagnostics |
| `cli_version_test.go` | ~100 | Version command |
| `cli_bench_test.go` | ~100 | Benchmark command |
| `cli_validate_test.go` | ~100 | Validate command |
| `cli_adapter_test.go` | ~500+ | Adapter layer |

**Result**: `go test ./internal/cli/ -short` → **PASS** (1.287s)

---

## 7. Recommendations Summary

| Priority | Action | Effort | Impact |
|----------|--------|--------|--------|
| P0 | Add command aliases | Low | High |
| P0 | Fix -v/-V flag swap | Low | High |
| P1 | Add command grouping | Low | High |
| P1 | Consistent --dry-run shorthand | Low | Medium |
| P1 | Fix cache clear --all | Low | Medium |
| P2 | Add shell completion hints | Medium | Medium |
| P2 | Implement runtime logs properly | Medium | Medium |
| P2 | Add backup/restore commands | Medium | High |
| P3 | Add --quiet support in commands | Medium | Low |
| P3 | Add env/shell/watch commands | Medium | Medium |

---

## Appendix A: Full Command Tree

```
cosca
├── agent
│   ├── list
│   ├── show <name>
│   ├── search <query>
│   ├── run <agent-name> <prompt>
│   └── capabilities [agent-name]
├── benchmark [--skip-search] [--skip-index] [--skip-memory]
├── bootstrap
├── cache
│   ├── clear [--all]
│   ├── warm
│   ├── stats
│   └── inspect [--key] [--prefix]
├── chat [--agent] [--provider] [--model] [--stream] [--metrics]
├── completion [bash|zsh|fish|powershell]
├── config
│   ├── get <key>
│   ├── set <key> <val>
│   ├── list
│   ├── edit
│   ├── reset [--force]
│   └── validate
├── context
│   ├── build <query>
│   ├── show
│   ├── clear
│   └── stats
├── docs [--offline]
├── doctor [runtime|editor|plugins|memory|knowledge|providers]
├── editor
│   ├── setup
│   ├── status
│   └── adapt <editor>
├── graph
│   ├── show
│   ├── query <entity> [--depth N]
│   ├── stats
│   └── export [--format json|graphml]
├── health
├── index
│   ├── rebuild
│   ├── update
│   ├── status
│   ├── stats
│   └── verify
├── init [--force|-f]
├── install [--global]
├── knowledge
│   ├── search <query> [--type] [--limit]
│   ├── graph
│   ├── stats
│   ├── rebuild
│   ├── verify
│   ├── benchmark
│   ├── explain <id>
│   ├── relations <id>
│   └── vacuum
├── memory
│   ├── list [--type|-t] [--limit|-l]
│   ├── show <id>
│   ├── search <query> [--type|-t] [--limit|-l]
│   ├── snapshot
│   │   ├── create
│   │   ├── list
│   │   └── restore <id>
│   ├── prune [--dry-run]
│   └── stats
├── metrics
├── pipeline
│   ├── list
│   └── run <name> [--prompt]
├── plugin
│   ├── install <name>
│   ├── uninstall <name>
│   ├── list
│   ├── update <name>
│   ├── search <query>
│   ├── info <name>
│   ├── enable <name>
│   └── disable <name>
├── prompt
│   ├── list
│   ├── show <name>
│   ├── search <query>
│   └── create <name> [--template]
├── provider
│   ├── list
│   ├── set <name>
│   ├── test <name>
│   ├── info <name>
│   └── watch
├── run <prompt> [--agent] [--provider] [--model] [--stream] [--dry-run] [--semantic] [--no-mag] [--metrics]
├── runtime
│   ├── status
│   ├── start
│   ├── stop
│   ├── restart
│   ├── logs [--tail N]
│   └── info
├── search [type] <query> [--limit|-l] [--offset|-o]
├── serve [--host] [--port] [--metrics-port] [--cors-origins] [--data-dir] [--tls-cert-file] [--tls-key-file]
├── skill
│   ├── list
│   ├── show <name>
│   ├── search <query>
│   └── install <name> [--source]
├── status
├── sync [--full|-f] [--dry-run|-n]
├── template
│   ├── list
│   ├── show <name>
│   ├── search <query>
│   └── create <name> [--type]
├── uninstall [--dry-run] [--keep-config]
├── update [--channel] [--check-only]
├── upgrade
├── validate
├── version
└── workflow
    ├── list
    ├── show <name>
    ├── run <name> [--prompt]
    └── search <query>
```
