# cosca-specialist-backend-service — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### Provenance-preserving untrusted context envelope

Represent retrieved/tool/MCP/plugin content as an item with explicit origin,
source, authority, trust, and policy state. Drop explicit quarantine/block
states before context construction; envelope all other content as data and
state that it cannot issue instructions or grant capabilities. Keep legacy
message roles and JSON projections unchanged.

### Mutex-protected exported statistics

Keep the existing JSON-facing fields for compatibility, guard mutations with
an internal `sync.RWMutex`, and expose a deep-copy `Snapshot` plus a locked
`MarshalJSON`. Return snapshots from higher-level accessors so callers cannot
 race with writers through retained pointers or shared maps.

### Idempotent ownership cleanup for raw file descriptors

When a helper returns a cleanup callback for a raw descriptor, guard the close
with `sync.Once`. Callers may defer cleanup while also invoking it on an error
or assertion path; a repeated raw `close(2)` can release an unrelated
descriptor that has already reused the same number.

### Opt-in fenced lifecycle wrapper

Wrap the existing manager execution closure with an explicitly injected,
narrow ledger interface. Begin and claim before execution, renew the lease in
a cancellable goroutine, and finalize from the same fencing token. Keep the
wrapper bypassable when disabled and persist only stable references/hashes;
avoid claiming step recovery when the executor is batch-only.

### Cancellation-safe terminal lifecycle

Derive a private heartbeat context from the request context and cancel it when
execution ends; have the worker select `Done()` independently of ticker and
executor completion. After cancellation/deadline, finalize with a bounded
`context.Background()` cleanup context and return both original and cleanup
errors via `errors.Join`. Treat database read and `RowsAffected` failures as
database errors before applying `ErrFenced` to zero-row mutations.

### Conditional routing strategy with shared fallback

Construct the enhanced router only when every enabling dependency is present;
otherwise retain the existing router instance and call it directly. Use the
same strategy branch in synchronous and streaming pre-execution paths so both
APIs preserve identical routing semantics.

### Structured untrusted payload and copy-on-read cache

Frame untrusted context as a length-declared JSON object and let the JSON
encoder escape every metadata/content field; normalize privileged authority
claims from external origins before serialization. Keep tool failures inside
the same tool-role envelope as successful output. For shared result caches,
clone on both Set and Get so policy filtering and callers cannot mutate cached
state. Replace sensitive prompt/query logs with request identity, size, and a
non-reversible truncated digest.
