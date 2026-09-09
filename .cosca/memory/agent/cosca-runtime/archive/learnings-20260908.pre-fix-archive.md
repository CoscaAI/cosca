# cosca-runtime - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-runtime — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Runtime Architecture Deep Dive

### 2026-07-28 — Full Runtime Code Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Compare runtime documentation against actual source code, document all discrepancies |
| **Technique** | Level 2 — Code-to-documentation cross-validation: read all 5 runtime source files (runtime.go 682L, daemon.go 494L, lifecycle.go 606L, state.go 510L, metrics.go 461L), compared against docs/runtime/overview.md |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #runtime #state-machine #lifecycle #daemon #metrics #documentation #audit |
| **Related** | internal/runtime/*.go, pkg/cosca/runtime.go, docs/runtime/overview.md |
| **Learned** | 8 states and 20 transitions all confirmed exact match between code and docs. Three critical discrepancies found: 1) Restart() functionally broken — Start() requires Uninitialized state but Restart() calls Stop() (leaves state=Stopped) then Start() immediately fails. Transition Stopped→Uninitialized defined but never executed. 2) EventStartupComplete fires at wrong point — line 333 of runtime.go publishes it during Initializing state, BEFORE any init hooks execute. Subscribers receive startup event when nothing is running. 3) Docs describe 7 metrics that don't exist (runtime.state gauge, runtime.health gauge, runtime.start.duration histogram etc.), but omit 12 real metrics that DO exist (8 atomic counters: index/search/context/memory/plugin/error/sync/event counts; 4 duration histograms with p50/p95/p99: index/search/context/memory; system: goroutines min/max/avg, MemoryUsage, TotalAllocated). Daemon features undocumented: watchdog auto-restarts unhealthy subsystems (Stop+Start, 30s timeout each), stale PID detection via signal 0 probe, sync loop (5min default). Dual signal handling in daemon mode: both Runtime.HandleSignals() and Daemon.handleSignals() register for SIGINT/SIGTERM/SIGHUP — concurrent double-firing is functionally safe (second Stop() hits early-return) but fragile. |
| **Next** | Level 3: Fix Restart() bug, fix EventStartupComplete timing, implement hot reload trigger (documented but missing), create runtime integration test suite for all 20 state transitions |

### 2026-07-28 — Metrics System Architecture
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Document actual runtime metrics collection architecture |
| **Technique** | Level 1 — Source code tracing: followed Metrics struct fields, atomic operations, histogram implementation, and snapshot generation |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #metrics #runtime #atomic #histogram #go |
| **Related** | internal/runtime/metrics.go |
| **Learned** | Metrics uses atomic operations for counters (8x sync/atomic), custom sorted-slice durationHistogram for percentile calculation (insertion-sort on Record, binary search on Snapshot), sync.Map for component health tracking. No external metrics library (no Prometheus client, no OpenTelemetry SDK). MetricsSnapshot is the public API type (generated on-demand). Some public RuntimeStatus fields (ActiveAgents, ActiveWorkflows, LoadedPlugins, Mode) exist only in pkg/cosca/ API layer — not backed by engine metrics. |
| **Next** | Level 2: Add OpenTelemetry export for metrics, implement Prometheus /metrics endpoint |

### 2026-08-29 — MCP Serve Hot-Rebuild & Surgical Restart
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Rebuild bin\cosca.exe with flight-recorder writes fix in internal/mcpserver/tools.go and restart ONLY the MCP server (`mcp serve`), never touching the 14139 daemon |
| **Technique** | Level 2 — Exact-PID surgical restart with file-lock awareness |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #mcp #build #restart #go #windows #pid |
| **Related** | internal/mcpserver/tools.go, cmd/cosca |
| **Learned** | 1) `go build -o bin\cosca.exe ./...` FAILS on this repo (multiple main pkgs: cmd/cosca, cmd/acquireall, cmd/cosca-check, cmd/cosca-indexer, cmd/cosca-merkle, internal/voice/cmd/demo). The real binary is `./cmd/cosca` — build with `go build -o bin\cosca.exe ./cmd/cosca`. 2) The output `bin\cosca.exe` is file-LOCKED while any cosca.exe process runs, so `go build` fails with "O arquivo já está sendo usado por outro processo." → must Stop-Process the target ADJUST FIRST, then build. 3) On Windows, `mcp serve` is a stdio server — but `Start-Process -WindowStyle Hidden` returns immediately (no pipeline block) and spawns it detached with inherited env; `$env:COSCA_ENABLE_MCP=1; $env:COSCA_ALLOW_NO_ROOT=1`. 4) `Get-CimInstance Win32_Process -Filter "Name='cosca.exe'"` distinguishes mcp target vs daemon by CommandLine matching. 5) opencode does NOT auto-relaunch mcp serve on the same session; a fresh client session is required for it to come back as opencode's child (as opencode is what ties the stdio pipes). |
| **Next** | Persist a helper that maps `mcp serve` vs `serve --host` PID by CommandLine; document the stdio caveat that a real reconnect requires a new opencode session. |

### 2026-09-01 — Process-Tree Executor Fix (idle/hard timeout; Windows Job Object)
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Fix shell/sandbox executor that hangs forever when a command (e.g. `$env:JAVA_HOME=...; & gradle.bat wrapper` on Windows via cmd.exe→bat→java→Gradle Daemon) leaves a background child holding the captured stdout pipe open |
| **Technique** | Level 3 — cross-package refactor + new shared execution layer |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #runtime #processutil #idle-timeout #jobobject #windows #orchestration |
| **Related** | internal/processutil/*, internal/chat/sandbox/gate.go, gate_linux.go, internal/chat/tool/shell.go, internal/evals/verify_procgroup.go, internal/chat/ports.go |
| **Learned** | 1) CAUSE: naive `bytes.Buffer` + `exec.CommandContext(ctx,..).Run()` hangs because (a) the whole process tree is NOT killed on cancel/grandchildren keep the stdout pipe open so `Run()`/`cmd.Wait()` never returns even after the deadline, and (b) `bytes.Buffer` emits no per-chunk event so idle cannot be detected. 2) FIX: new `internal/processutil` package with `Run(ctx, cmd, Config)` that STREAMS stdout/stderr through a mutex-guarded `safeBuffer` (each Write stamps `lastActivity`), runs an independent idle monitor (default `IdleTimeout=120s`, `MaxRuntime=30min`), and kills the WHOLE tree. Idle and hard are independent; a caller ctx deadline is reported as `hard_timeout`, a manual cancel as `cancelled`. 3) TREE KILL: Unix = `Setpgid` + `SIGKILL(-pid)` (reused the pattern that used to live in `evals/verify_unix.go`); Windows = Job Object (`CreateJobObject`→`SetInformationJobObject` `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`→`AssignProcessToJobObject`→`TerminateJobObject`) with a documented `taskkill /T /F /PID` fallback that also closes the assign race. 4) Go 1.20+ validation: `exec.Cmd.Start()` REJECTS a non-nil `Cancel` unless the command was created via `exec.CommandContext` — so commands that set `cmd.Cancel` MUST be built with `CommandContext`; `Run` deliberately does NOT set `cmd.Cancel` (the monitor is the single cancellation owner) so it accepts either. The status is reconciled from `ctx.Err()` to survive the exec-watchCtx race. 5) Test gotcha: `-race` slowness blows fixture inter-tick gaps past tight `IdleTimeout`s — give output-keeps-alive tests a generous idle window (5s). 6) Manual .cmd repro gotcha: `Start-Process -NoNewWindow` + `-WindowStyle Hidden` is an invalid parameter-set combo; and `Start-Process`/`start /b` do NOT reliably re-inherit the CAPTURED pipe to a grandchild across layers — but through the PRODUCTION gate path (cmd with captured stdout) `start /b` DOES inherit it, returning `idle_timeout`. 7) Additive contract: extended `chat.Command` (`Timeout`,`IdleTimeout`) and `chat.SandboxResult` (`Status`,`IdleFor`) as optional fields (no caller breakage). |
| **Next** | Consider surfacing `SandboxResult.Status` to the UI/tool JSON output; add a bwrap-path integration test on Linux; keep the orphan fixture as a permanent Windows guard. |

