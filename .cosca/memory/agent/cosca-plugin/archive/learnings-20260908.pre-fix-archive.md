# cosca-plugin - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-plugin — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-plugin |
| **Task** | Initial capability establishment |
| **Technique** | Standard plugin patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #plugin #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

---

## Real Task Learnings

### 2026-07-28T09:15:00Z — Full-Stack Plugin Audit

| Field | Value |
|-------|-------|
| **Agent** | cosca-plugin |
| **Task** | Audited the entire Cosca plugin system — 12 source files, 4 runtimes, 6 integration points. Produced comprehensive audit report at `.cosca/memory/plugin/audit-report.md`. |
| **Technique** | Multi-layer codebase audit: structure mapping → runtime analysis → gap identification → risk correlation → prioritization |
| **Level** | 2 |
| **Outcome** | success |
| **Confidence** | 0.60 |
| **Tags** | #audit #plugin #wasm #wazero #epic002 #runtime-sandbox #architecture-review |
| **Learned** | 1) The plugin system is architecturally sound with 65% real completeness (not 75% reported). 2) WASM runtime (wazero v1.7.0) compiles/instantiates modules but has ZERO host function bindings — this is the single critical gap preventing WASM plugins from being useful. 3) External process runtime is the most complete component (90%) with full JSON protocol and rlimit sandbox. 4) The Hook system (8 points) and EventBus are production-ready. 5) Security posture is strong: Go plugins disabled-by-default, WASM FS restricted, archive path traversal protection, checksum verification. 6) 59.5% test coverage, all tests pass. 7) No plugin examples exist anywhere in the codebase. 8) Seccomp is configured but not implemented. 9) Plugin configuration (GetConfig/SetConfig) returns stubs. 10) Version comparison is string-based, not semver. |
| **What to try next** | Implement WASM host function bindings using wazero's `ModuleBuilder.ExportFunction` to expose RuntimeAPI. Create a TinyGo reference plugin. Add `WithMemoryLimitPages` enforcement. Target: 80% completeness. |
| **Failure modes identified** | 1) WASM plugin isolation without host bindings = completely useless, not just limited. 2) Seccomp config misleadingly reports as enabled. 3) Missing semver comparison means version constraints like `>=1.0.0` don't work. |

