# cosca-database — Reusable Patterns

## Pattern: WAL-safe SQLite snapshot

For a live SQLite database in WAL mode, prefer the SQLite backup API (`.backup`/`sqlite3_backup`) over `cp`. Open the source read-only, write into a restricted backup directory, then validate destination `integrity_check`, size, and SHA-256. Treat the source database, `-wal`, and `-shm` as one live state; never copy only the main file as a consistency strategy.

## Pattern: Backup destination risk separation

Use a same-filesystem restricted staging directory for fast rollback, but recommend a second copy on a different filesystem/device for disaster recovery. Report free space before execution and reserve at least one source-sized unit plus operational headroom.
