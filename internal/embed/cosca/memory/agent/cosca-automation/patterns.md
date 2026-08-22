# cosca-automation — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### Source-to-Embed Sync Integrity Pattern

For approved embed regeneration, run the repository target rather than manually copying files. Follow with `git diff --check` and byte-for-byte comparisons for required canonical documents. Inspect the resulting status to ensure only target-defined synchronization changes occurred.
