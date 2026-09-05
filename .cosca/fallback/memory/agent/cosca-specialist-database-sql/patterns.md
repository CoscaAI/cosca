# cosca-specialist-database-sql — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### Isolated opt-in ledger migration
Register capability-specific migrations on a capability-owned MigrationManager rather
than changing the application's default migration set. Keep all durable records as
references/hashes, and guard every mutation in one transaction with generation plus
hashed fencing token.

### Atomic SQLite sequence and drift gate
Use a per-parent counter table with `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`
inside the same transaction as the append. Validate every applied migration's name and
checksum before applying or reporting state, with narrowly-scoped exceptions only for
explicit opt-in migrations absent from the default manager.
