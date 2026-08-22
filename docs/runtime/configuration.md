# Runtime Configuration Reference

> **Status**: active | **Owner**: Runtime Chief | **Last Updated**: 2026-07-23

## Configuration Sources

Configuration is loaded with the following precedence (highest to lowest):

1. **Command-line flags** (`--verbose`, `--config`, etc.)
2. **Environment variables** (`COSCA_*` prefix)
3. **User config file** (`~/.config/cosca/config.yaml`)
4. **Project config file** (`.cosca/config.yaml`)
5. **Default values**

---

## Environment Variables

All environment variables use the `COSCA_` prefix. Nested keys use double underscores.

### General

| Variable | Default | Description |
|----------|---------|-------------|
| `COSCA_HOME` | `~/.config/cosca` | Cosca home directory |
| `COSCA_MODE` | `development` | Runtime mode (dev, test, staging, production) |
| `COSCA_VERBOSE` | `false` | Enable verbose debug output |
| `COSCA_DEV` | `false` | Enable development mode (debug logging) |
| `COSCA_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `COSCA_PROFILE` | `default` | Configuration profile |

### Paths

| Variable | Default | Description |
|----------|---------|-------------|
| `COSCA_PROJECT` | `.` | Project root directory |
| `COSCA_DB__PATH` | `$COSCA_HOME/cosca.db` | SQLite database path |
| `COSCA_CACHE_DIR` | `$COSCA_HOME/cache` | Cache directory |
| `COSCA_DATA_DIR` | `$COSCA_HOME/data` | Data directory |
| `COSCA_RUNTIME_DIR` | `$COSCA_HOME/runtime` | Runtime directory |
| `COSCA_PLUGINS_DIR` | `$COSCA_HOME/plugins` | Plugins directory |

### Provider

| Variable | Description |
|----------|-------------|
| `COSCA_PROVIDER__API_KEY` | Provider API key |
| `COSCA_PROVIDER__MODEL` | Model identifier |
| `COSCA_PROVIDER__BASE_URL` | Provider base URL |

### Network

| Variable | Description |
|----------|-------------|
| `COSCA_NETWORK__PROXY_URL` | HTTP proxy URL |
| `COSCA_FEATURES__TELEMETRY` | Enable telemetry (true/false) |
| `COSCA_FEATURES__METRICS` | Enable metrics (true/false) |

---

## Configuration File (YAML)

The configuration file can be located at:
- Project level: `.cosca/config.yaml`
- User level: `~/.config/cosca/config.yaml`

### Complete Schema

```yaml
# Cosca Configuration
# Version: 1.0

# Schema version
version: "1.0"

# Active configuration profile
profile: "default"

# Runtime mode: development, test, staging, production
mode: "development"

# Enable verbose output
verbose: false

# ── Filesystem Paths ──────────────────────────────────────────
paths:
  # Cosca home directory
  home: ~/.config/cosca

  # Project root directory
  project: "."

  # Runtime directory (ephemeral state)
  runtime: ~/.config/cosca/runtime

  # Data directory (persistent state)
  data: ~/.config/cosca/data

  # Cache directory
  cache: ~/.config/cosca/cache

  # Log output directory
  logs: ~/.config/cosca/logs

  # Temporary files directory
  temp: ~/.config/cosca/tmp

  # Plugins directory
  plugins: ~/.config/cosca/plugins

  # Backups directory
  backups: ~/.config/cosca/backups

# ── Database Configuration ────────────────────────────────────
db:
  # Path to SQLite database file
  path: ~/.config/cosca/cosca.db

  # Enable WAL mode for better concurrency
  wal_mode: true

  # Database page size in bytes (512-65536)
  page_size: 4096

  # Database cache size in KB
  cache_size_kb: 32768

  # Maximum open database connections
  max_open_conns: 4

  # Maximum idle database connections
  max_idle_conns: 2

  # Connection max lifetime
  conn_max_lifetime: 1h

  # Connection max idle time
  conn_max_idle_time: 30m

  # Enable FTS5 full-text search extension
  enable_fts5: true

  # Enable sqlite-vec vector search extension
  enable_vector_ext: false

  # How often to backup the database
  backup_interval: 1h

# ── LLM Provider Configuration ────────────────────────────────
provider:
  # Provider name: openai, anthropic, ollama, azure, google, aws-bedrock, custom
  name: ""

  # Model identifier (e.g., gpt-4o, claude-3-opus)
  model: ""

  # API key (also settable via COSCA_PROVIDER__API_KEY env var)
  api_key: ""

  # Base URL for the provider API
  base_url: ""

  # Maximum tokens for completions
  max_tokens: 4096

  # Sampling temperature (0.0 - 2.0)
  temperature: 0.7

  # Request timeout
  timeout: 60s

  # Maximum retries on failure
  max_retries: 3

  # Rate limit (requests per minute)
  rate_limit_per_min: 60

# ── Embedding Configuration ───────────────────────────────────
embedding:
  # Embedding model name
  model: "text-embedding-3-small"

  # Vector dimensions
  dimensions: 128

  # Batch size for embedding requests
  batch_size: 32

  # Embedding provider name
  provider: ""

# ── Editor Configuration ──────────────────────────────────────
editor:
  # Editor executable name
  name: ""

  # Color theme
  theme: "default"

  # Font size
  font_size: 14

  # Tab width in spaces
  tab_size: 4

  # Show line numbers
  line_numbers: true

  # Enable word wrapping
  word_wrap: true

  # Enable auto-save
  auto_save: true

  # Auto-save interval
  auto_save_interval: 30s

# ── File Watcher Configuration ────────────────────────────────
watch:
  # Enable file watching
  enabled: true

  # Event debounce duration
  debounce: 500ms

  # Watch subdirectories recursively
  recursive: true

  # Glob patterns to exclude
  exclude_patterns:
    - .git/**
    - node_modules/**
    - "*.log"
    - "*.tmp"

  # Glob patterns to include (empty = all)
  include_patterns: []

  # Maximum file size in bytes to watch (10 MB)
  max_file_size: 10485760

# ── Search Configuration ──────────────────────────────────────
search:
  # Default search result limit
  default_limit: 20

  # Maximum allowed results
  max_results: 100

  # Minimum similarity score (0.0 - 1.0)
  min_score: 0.7

  # Enable vector search
  enable_vector: true

  # Enable full-text search
  enable_fulltext: true

  # Enable graph-based search
  enable_graph: false

  # Indexing interval
  index_interval: 5m

# ── Server Configuration ──────────────────────────────────────
server:
  # Bind address
  host: "127.0.0.1"

  # HTTP API port
  api_port: 14120

  # Metrics endpoint port
  metrics_port: 14121

  # gRPC port
  rpc_port: 14122

  # Enable TLS
  tls: false

  # TLS certificate path
  tls_cert: ""

  # TLS key path
  tls_key: ""

  # Read timeout
  read_timeout: 30s

  # Write timeout
  write_timeout: 30s

  # Maximum request header size
  max_header_bytes: 1048576

# ── Logging Configuration ─────────────────────────────────────
log:
  # Log level: debug, info, warn, error, fatal, panic
  level: "info"

  # Log format: console, json
  format: "console"

  # Log output path (empty = stderr)
  output: ""

  # Maximum log file size in MB before rotation
  max_size_mb: 100

  # Maximum number of rotated log files
  max_backups: 3

  # Maximum age of log files in days
  max_age_days: 30

  # Enable log file compression
  compress: true

  # Show caller in log entries
  show_caller: false

# ── Cache Configuration ───────────────────────────────────────
cache:
  # Cache backend type: memory, sqlite, redis
  type: "memory"

  # Maximum number of cache entries
  size: 10000

  # Default TTL for cache entries
  ttl: 3600s

  # Enable value compression
  enable_compression: false

# ── Network Configuration ─────────────────────────────────────
network:
  # HTTP proxy URL
  proxy_url: ""

  # Hosts to exclude from proxy
  no_proxy: []

  # Skip TLS verification (development only)
  insecure_skip_verify: false

  # TCP dial timeout
  dial_timeout: 10s

  # TCP keep-alive interval
  keep_alive: 30s

  # Maximum idle HTTP connections
  max_idle_conns: 100

  # Idle connection timeout
  idle_conn_timeout: 90s

  # TLS handshake timeout
  tls_handshake_timeout: 10s

  # Response header timeout
  response_header_timeout: 30s

# ── Feature Flags ─────────────────────────────────────────────
features:
  # Enable metrics collection and export
  metrics: true

  # Enable anonymous usage telemetry
  telemetry: false

  # Enable automatic update checks
  auto_update: true

  # Enable vector search capabilities
  vector_search: true

  # Enable graph-based search
  graph_search: false

  # Enable file system watcher
  file_watching: true

  # Enable the plugin system
  plugin_system: true

  # Enable shell completion generation
  shell_completion: true

  # Enable experimental features
  experimental: false

# ── Plugin System Configuration ───────────────────────────────
plugins:
  # Enable the plugin system
  enabled: true

  # Plugins directory
  dir: ~/.config/cosca/plugins

  # Allowed plugin security policies
  allowed_policies:
    - network
    - filesystem
    - exec

  # Maximum execution time for plugins
  timeout: 30s

  # Maximum memory per plugin in MB
  max_memory_mb: 128

# ── Performance Configuration ─────────────────────────────────
performance:
  # Maximum memory usage target in MB
  max_memory_mb: 512

  # Maximum open file descriptors
  max_open_files: 1024

  # Maximum concurrent operations
  max_concurrent_ops: 50

  # Default batch processing size
  batch_size: 100

  # Worker pool goroutine count (default: CPU count)
  worker_pool_size: 0

  # Prefetch buffer size
  prefetch_size: 1024

# ── Timeout Configuration ─────────────────────────────────────
timeouts:
  # Default command execution timeout
  command: 300s

  # Agent execution timeout
  agent: 300s

  # Skill execution timeout
  skill: 120s

  # Workflow execution timeout
  workflow: 600s

  # HTTP request timeout
  request: 60s

  # Graceful shutdown timeout
  graceful_shutdown: 60s

  # Health check interval
  health_check: 30s
```

---

## Minimal Configuration File

```yaml
version: "1.0"
mode: "development"
db:
  path: ~/.config/cosca/cosca.db
```

---

## Example: Project-Level Configuration

Create `.cosca/config.yaml` in your project root:

```yaml
version: "1.0"
mode: "development"
paths:
  project: "/path/to/my-project"
watch:
  enabled: true
  exclude_patterns:
    - .git/**
    - node_modules/**
    - dist/**
    - "*.generated.*"
search:
  default_limit: 30
  enable_graph: true
features:
  telemetry: false
```

---

## Default Values Reference

| Setting | Default | Environment Variable |
|---------|---------|---------------------|
| `mode` | `development` | `COSCA_MODE` |
| `paths.home` | `~/.config/cosca` | `COSCA_HOME` |
| `db.path` | `$COSCA_HOME/cosca.db` | `COSCA_DB__PATH` |
| `db.wal_mode` | `true` | |
| `provider.name` | `""` | |
| `search.default_limit` | `20` | |
| `search.min_score` | `0.7` | |
| `watch.enabled` | `true` | |
| `features.metrics` | `true` | `COSCA_FEATURES__METRICS` |
| `features.telemetry` | `false` | `COSCA_FEATURES__TELEMETRY` |
| `plugins.enabled` | `true` | |
| `log.level` | `info` | `COSCA_LOG_LEVEL` |
| `server.api_port` | `14120` | |
| `server.rpc_port` | `14122` | |
| `performance.max_memory_mb` | `512` | |
| `timeouts.command` | `300s` | |

---

## Configuration Validation

The configuration is validated on load. Validation checks include:

- `version` is required
- `paths.home` is required
- `db.max_open_conns` must be >= 1
- `db.page_size` must be between 512 and 65536
- `provider.temperature` must be between 0 and 2
- `provider.max_tokens` must be >= 1
- `server.api_port` and `server.rpc_port` must be in valid range and different
- `performance.max_memory_mb` must be >= 64
- `search.default_limit` must be between 1 and `search.max_results`

---

## CLI Commands

```bash
# Show current configuration
cosca config show

# Show current configuration in JSON
cosca config show --json

# Initialize default configuration
cosca config init

# Validate configuration
cosca config validate
```

---

> **Related**: [Runtime Overview](overview.md) | [Architecture Overview](../architecture/overview.md)
