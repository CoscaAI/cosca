# Memory integrity audit artifact

This advisory, reversible artifact inventories protected memory/governance
Markdown files with SHA-256. The JSON baseline is written to
`.cosca/audit/memory-integrity-manifest.json` (runtime data and audit output
remain untracked by design).

```text
cosca memory integrity init             # establish a reviewed baseline
cosca memory integrity init --force     # explicitly replace an existing baseline
cosca memory integrity verify  # report changed, missing, or added files
```

`init` refuses to replace an existing manifest by default. Use `--force` only
after reviewing the replacement; the operation remains offline, advisory, and
reversible.

The manifest directory is created with mode `0700`; the manifest and its
temporary files use mode `0600`.

`verify` reads protected files and returns a non-zero status on mismatch for
CI/operator use, but it is not invoked by runtime startup and does not block or
alter runtime behavior. Removing the JSON manifest fully reverses the artifact.
No database, KERNEL, or Constitution file is modified.

Scope is explicit: KERNEL/Constitution references, memory-model/learning
protocol documents, and agent memory Markdown under
`.cosca/memory/agent/`. SQLite databases, sessions, generated audit
files, and the runtime are excluded.
