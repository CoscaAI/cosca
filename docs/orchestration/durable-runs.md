# Durable runs

The workflow manager has an opt-in durable lifecycle via
`WithDurableRun(DurableRunConfig, RunLedger)`. Disabled configuration, or a
nil ledger, executes the existing path unchanged. The application must create
and inject a `durable.Ledger`; no hidden singleton is used.

The integration records begin, claim, lease heartbeats, and fenced terminal
completion/failure. It stores only a workflow reference and SHA-256 hash. It
does not persist prompts, tokens, model output, or other sensitive payloads.

This slice is **run lifecycle only** for batch/pipeline execution. The current
pipeline executor does not expose safe per-step commit boundaries, so this is
not crash-resume of individual steps and must not be advertised as such.
