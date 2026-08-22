# cosca-specialist-backend-api — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### Strict compatibility boundary pattern

For SDK responses with evolving envelopes, read the payload once, decode the
new envelope when present, and fall back to the legacy direct representation.
For indexed replacements, commit SQLite changes first and then remove external
records by the old document ID unconditionally; never gate stale-data cleanup
on the size of the replacement vector set. Validate provider dimensions before
constructing records, rather than truncating or padding vectors.
