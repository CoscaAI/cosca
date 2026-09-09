# API Reference: Overview

> **Status**: active | **Owner**: Backend Chief | **Last Updated**: 2026-07-28 | **Version**: 1.4.0-dev

This document provides an overview of the Cosca API surface, including gRPC services, REST endpoints (daemon mode), the Model Context Protocol (MCP) interface, and SDK APIs.

---

## API Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        API SURFACE                             │
│                                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │    gRPC API   │  │   REST API   │  │   MCP Protocol     │  │
│  │  (internal)   │  │  (daemon)    │  │  (editor-facing)   │  │
│  └──────────────┘  └──────────────┘  └────────────────────┘  │
│         │                │                     │              │
│         └────────────────┼─────────────────────┘              │
│                          ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │                     CORE SERVICES                          │ │
│  │  Knowledge  │  Memory  │  Runtime  │  Plugins  │  Cache  │ │
│  └──────────────────────────────────────────────────────────┘ │
│                          │                                     │
│                          ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │                     TRANSPORT LAYERS                       │ │
│  │  Unix Domain Socket  │  TCP  │  STDIO  │  HTTP/SSE       │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                │
└──────────────────────────────────────────────────────────────┘
```

### Transport Comparison

| Protocol | Transport | Use Case | Authentication |
|----------|-----------|----------|----------------|
| gRPC | Unix socket | Internal CLI ↔ Runtime | None (filesystem) |
| REST | TCP (HTTP) | External daemon clients | JWT Bearer token or API key |
| MCP | STDIO / HTTP/SSE | Editor integration | None (local) |
| SDK | Direct Go/TS calls | Application embedding | None (in-process) |

---

## gRPC Service Definitions

The gRPC API is used for internal communication between the CLI and the runtime daemon via a Unix domain socket.

### Service: `Knowledge`

```protobuf
service Knowledge {
    // Search the knowledge base
    rpc Search(SearchRequest) returns (SearchResponse);

    // Index a file or directory
    rpc Index(IndexRequest) returns (IndexResponse);

    // Sync with filesystem
    rpc Sync(SyncRequest) returns (SyncResponse);

    // Get engine statistics
    rpc Stats(StatsRequest) returns (StatsResponse);

    // Rebuild all indexes
    rpc Rebuild(RebuildRequest) returns (RebuildResponse);

    // Create a snapshot
    rpc Snapshot(SnapshotRequest) returns (SnapshotResponse);

    // Verify database integrity
    rpc Verify(VerifyRequest) returns (VerifyResponse);

    // Vacuum database
    rpc Vacuum(VacuumRequest) returns (VacuumResponse);
}

message SearchRequest {
    string query = 1;
    repeated string types = 2;       // document, chunk, entity, code_block
    string path = 3;                 // Filter by path prefix
    int32 limit = 4;                 // Default: 20
    int32 offset = 5;                // Default: 0
    string mode = 6;                 // hybrid, fts5, vector, graph
}

message SearchResponse {
    repeated Result results = 1;
    int32 total = 2;
    int32 offset = 3;
    repeated Facet facets = 4;
    repeated string suggestions = 5;
}
```

### Service: `Memory`

```protobuf
service Memory {
    // Store a memory record
    rpc Store(StoreRequest) returns (StoreResponse);

    // Search memory records
    rpc Search(MemorySearchRequest) returns (MemorySearchResponse);

    // Get a specific memory record
    rpc Get(GetRequest) returns (GetResponse);

    // Delete a memory record
    rpc Delete(DeleteRequest) returns (DeleteResponse);

    // Promote short-term memory to long-term
    rpc Promote(PromoteRequest) returns (PromoteResponse);

    // Prune expired memory records
    rpc Prune(PruneRequest) returns (PruneResponse);

    // Get memory statistics
    rpc Stats(MemoryStatsRequest) returns (MemoryStatsResponse);
}

message StoreRequest {
    string key = 1;
    string type = 2;                 // short, long, project, architecture, decision
    string content = 3;
    map<string, string> metadata = 4;
    repeated string tags = 5;
}
```

### Service: `Runtime`

```protobuf
service Runtime {
    // Start the runtime daemon
    rpc Start(StartRequest) returns (StartResponse);

    // Stop the runtime daemon
    rpc Stop(StopRequest) returns (StopResponse);

    // Get runtime status and health
    rpc Status(StatusRequest) returns (StatusResponse);

    // Restart the runtime
    rpc Restart(RestartRequest) returns (RestartResponse);

    // Subscribe to runtime events (server-sent)
    rpc Events(EventsRequest) returns (stream RuntimeEvent);
}

message StatusResponse {
    string state = 1;                // uninitialized, running, stopped, error
    string health = 2;               // unknown, healthy, degraded, unhealthy
    string uptime = 3;
    string started_at = 4;
    string version = 5;
    map<string, ComponentStatus> components = 6;
}
```

### Service: `Plugins`

```protobuf
service Plugins {
    // Install a plugin
    rpc Install(InstallRequest) returns (InstallResponse);

    // List installed plugins
    rpc List(ListRequest) returns (ListResponse);

    // Get plugin information
    rpc Get(GetPluginRequest) returns (GetPluginResponse);

    // Remove a plugin
    rpc Remove(RemoveRequest) returns (RemoveResponse);

    // Enable a plugin
    rpc Enable(EnableRequest) returns (EnableResponse);

    // Disable a plugin
    rpc Disable(DisableRequest) returns (DisableResponse);

    // Validate a plugin manifest
    rpc Validate(ValidateRequest) returns (ValidateResponse);
}
```

---

## REST API (Daemon Mode)

When running in daemon mode (`cosca serve`), Cosca exposes a REST API on the configured port (default: `http://localhost:14120`).

### Endpoints

> Port: All endpoints served on `http://localhost:14120` under `/v1/` prefix (36 registered endpoints).
> For the full OpenAPI 3.0 specification with 50 operations and 62 schemas, see `api/rest/openapi.yaml`.

#### Health & Readiness

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/health` | Liveness probe (Kubernetes) | None |
| `GET` | `/ready` | Readiness probe (subsystem checks) | None |

#### Knowledge

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/knowledge/search` | Search the knowledge base |
| `POST` | `/v1/knowledge/index` | Index a file or directory |
| `POST` | `/v1/knowledge/sync` | Sync with filesystem |
| `GET` | `/v1/knowledge/stats` | Get engine statistics |

**Example:**

```bash
# Search via REST API (POST for rich query)
curl -s -X POST "http://localhost:14120/v1/knowledge/search" \
  -H "Content-Type: application/json" \
  -d '{"query":"authentication","limit":5}' | jq

# Response
{
  "results": [
    {
      "id": "doc-001",
      "title": "Authentication Flow",
      "snippet": "The authentication flow uses OAuth 2.0...",
      "score": 0.95,
      "type": "document",
      "path": "docs/api-reference/auth.md"
    }
  ],
  "total": 1,
  "facets": {
    "types": [{"value": "document", "count": 1}]
  }
}
```

#### Memory

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/memory/store` | Store a memory record |
| `GET` | `/v1/memory/search` | Search memory records |
| `GET` | `/v1/memory/get` | Get a memory record |
| `DELETE` | `/v1/memory/delete` | Delete a memory record |
| `POST` | `/v1/memory/promote` | Promote short-term memory |
| `GET` | `/v1/memory/stats` | Get memory statistics |

#### Runtime

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/status` | Get runtime status |
| `GET` | `/v1/health` | Runtime health check |

#### Agents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/agents` | List all agents |
| `GET` | `/v1/agents/search` | Search agents |
| `GET` | `/v1/agents/{name}` | Get agent detail |

#### Skills

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/skills` | List all skills |
| `GET` | `/v1/skills/search` | Search skills |
| `GET` | `/v1/skills/{name}` | Get skill detail |

#### Providers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/providers` | List all providers |
| `GET` | `/v1/providers/{name}` | Get provider detail |
| `POST` | `/v1/providers/{name}/test` | Test provider connection |
| `PUT` | `/v1/providers/active` | Set active provider |

#### Workflows

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/workflows` | List all workflows |
| `GET` | `/v1/workflows/search` | Search workflows |
| `GET` | `/v1/workflows/{name}` | Get workflow detail |
| `POST` | `/v1/workflows/{name}/run` | Run a workflow |

#### Authentication

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/v1/auth/login` | Login (username + password → tokens) | None |
| `POST` | `/v1/auth/refresh` | Refresh token pair | None |
| `GET` | `/v1/auth/me` | Get current user profile | Bearer token |

#### User Management (admin-only)

| Method | Endpoint | Description | Role |
|--------|----------|-------------|------|
| `GET` | `/v1/users` | List all users | admin |
| `POST` | `/v1/users` | Create a user | admin |
| `DELETE` | `/v1/users/{id}` | Delete a user | admin |
| `PUT` | `/v1/users/{id}/role` | Update user role | admin |

---

## MCP (Model Context Protocol) Tools and Resources

Cosca exposes its capabilities through the Model Context Protocol, enabling AI-powered editors to interact with the knowledge base, memory, and runtime.

### MCP Transport

| Transport | Mode | Default Endpoint |
|-----------|------|-----------------|
| STDIO | CLI-integrated (stdio) | `cosca mcp` |
| HTTP/SSE | Daemon mode | `http://localhost:14120/mcp` |

### MCP Tools

| Tool Name | Description | Input Schema |
|-----------|-------------|--------------|
| `cosca_knowledge_search` | Search the project knowledge base | `{ query: string, type?: string, limit?: number }` |
| `cosca_knowledge_index` | Index a file or directory | `{ path: string }` |
| `cosca_memory_store` | Store a memory record | `{ key: string, content: string, type?: string }` |
| `cosca_memory_search` | Search memory records | `{ query: string, type?: string }` |
| `cosca_memory_get` | Get a specific memory record | `{ key: string }` |
| `cosca_runtime_status` | Get runtime status | `{}` |
| `cosca_plugin_list` | List installed plugins | `{}` |
| `cosca_config_get` | Get configuration value | `{ key: string }` |

### MCP Resources

| Resource URI | Description | Schema |
|-------------|-------------|--------|
| `cosca://knowledge/stats` | Knowledge engine statistics | `{ total_docs: number, total_chunks: number, ... }` |
| `cosca://memory/stats` | Memory engine statistics | `{ total_records: number, types: { ... } }` |
| `cosca://runtime/status` | Current runtime status | `{ state: string, health: string, ... }` |

### MCP Example

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "cosca_knowledge_search",
    "arguments": {
      "query": "authentication flow",
      "type": "document",
      "limit": 5
    }
  }
}
```

---

## SDK API Summary

### Go SDK

```go
import "github.com/CoscaAI/cosca/sdk"

// Create a client
client, err := cosca.NewClient()
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Search knowledge
results, err := client.Knowledge.Search(ctx, cosca.SearchQuery{
    Text:  "authentication",
    Type:  "document",
    Limit: 10,
})

// Store memory
err = client.Memory.Store(ctx, cosca.MemoryRecord{
    Key:     "user-preference",
    Content: "User prefers dark mode",
    Type:    "long",
    Tags:    []string{"preference"},
})

// Get runtime status
status, err := client.Runtime.Status(ctx)
fmt.Printf("Runtime: %s (%s)\n", status.State, status.Health)
```

### TypeScript SDK

```typescript
import { AOSClient } from 'cosca-sdk';

const client = new AOSClient();

// Search knowledge
const results = await client.knowledge.search({
  query: 'authentication',
  type: 'document',
  limit: 10
});

// Store memory
await client.memory.store({
  key: 'user-preference',
  content: 'User prefers dark mode',
  type: 'long',
  tags: ['preference']
});

// Get runtime status
const status = await client.runtime.status();
console.log(`Runtime: ${status.state} (${status.health})`);
```

---

## Authentication

### gRPC (Unix Socket)

No authentication required. Access is controlled by filesystem permissions on the Unix domain socket.

### REST API (Daemon Mode)

Authentication is via **JWT Bearer tokens** (preferred) or **API keys**. JWT tokens are obtained via the login endpoint.

```bash
# Obtain JWT token pair
curl -X POST http://localhost:14120/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Use the access token for authenticated requests
curl -H "Authorization: Bearer <access_token>" \
     "http://localhost:14120/v1/knowledge/search" \
     -H "Content-Type: application/json" \
     -d '{"query":"test","limit":5}'

# Or use an API key
curl -H "X-API-Key: your-api-key-here" \
     "http://localhost:14120/v1/knowledge/search" \
     -H "Content-Type: application/json" \
     -d '{"query":"test","limit":5}'
```

**JWT Configuration:**
- Algorithm: HMAC-SHA256 (HS256)
- Secret: set via `COSCA_JWT_SECRET` environment variable
- Access token lifetime: 24 hours
- Refresh token lifetime: 7 days
- RBAC roles: admin, editor, viewer

### MCP

No authentication required. MCP runs on localhost and is intended for local editor integration only.

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "INDEX_FAILED",
    "message": "Failed to index directory: permission denied",
    "details": {
      "path": "/path/to/dir",
      "cause": "permission_denied"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request |
| `NOT_FOUND` | 404 | Resource not found |
| `INDEX_FAILED` | 500 | Indexing operation failed |
| `SEARCH_FAILED` | 500 | Search operation failed |
| `ENGINE_NOT_READY` | 503 | Runtime not yet initialized |
| `PERMISSION_DENIED` | 403 | API key missing or invalid |
| `RATE_LIMITED` | 429 | Too many requests |
| `PLUGIN_ERROR` | 500 | Plugin operation failed |

### gRPC Error Mapping

| gRPC Code | HTTP Status | Description |
|-----------|-------------|-------------|
| `InvalidArgument` | 400 | Invalid request parameters |
| `NotFound` | 404 | Resource not found |
| `Internal` | 500 | Internal server error |
| `Unavailable` | 503 | Service temporarily unavailable |
| `DeadlineExceeded` | 504 | Request timed out |

---

## Rate Limiting

When running in daemon mode, the REST API applies rate limiting:

| Limit | Scope | Default |
|-------|-------|---------|
| 100 requests/second | Per client IP | Configurable via `api.rest.rate_limit` |
| 50 concurrent requests | Global | Configurable via `api.rest.max_concurrent` |

Rate limit headers are returned with every response:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1627000000
```

---

**Related**: [Go SDK Reference](../sdk/go.md) | [TypeScript SDK Reference](../sdk/typescript.md) | [MCP Server Example](../../examples/editors/mcp-server.md) | [Runtime Overview](../runtime/overview.md)

---

## Complete REST API Endpoint Map (v1)

### Health & Readiness
| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/health` | Liveness probe | None |
| GET | `/ready` | Readiness probe | None |

### Knowledge Engine
| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/knowledge/search` | Hybrid search (FTS5 + Vector + Graph) |
| POST | `/v1/knowledge/index` | Index documents |
| GET | `/v1/knowledge/stats` | Knowledge engine statistics |
| POST | `/v1/knowledge/sync` | Sync index with filesystem |

### Memory Engine
| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/memory/store` | Store memory record |
| GET | `/v1/memory/search` | Search memory records |
| GET | `/v1/memory/get` | Get memory record by ID |
| DELETE | `/v1/memory/delete` | Delete memory record |
| POST | `/v1/memory/promote` | Promote memory to higher layer |
| GET | `/v1/memory/stats` | Memory layer statistics |

### Runtime
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/status` | Runtime status |
| GET | `/v1/health` | Runtime health check |

### Agents
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/agents` | List all agents |
| GET | `/v1/agents/search` | Search agents |
| GET | `/v1/agents/{name}` | Get agent detail |

### Skills
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/skills` | List all skills |
| GET | `/v1/skills/search` | Search skills |
| GET | `/v1/skills/{name}` | Get skill detail |

### Providers
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/providers` | List all providers |
| GET | `/v1/providers/{name}` | Get provider detail |
| POST | `/v1/providers/{name}/test` | Test provider connection |
| PUT | `/v1/providers/active` | Set active provider |

### Workflows
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/workflows` | List all workflows |
| GET | `/v1/workflows/search` | Search workflows |
| GET | `/v1/workflows/{name}` | Get workflow detail |
| POST | `/v1/workflows/{name}/run` | Run a workflow |

### Authentication
| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| POST | `/v1/auth/login` | Login (returns token pair) | No |
| POST | `/v1/auth/refresh` | Refresh token pair | No |
| GET | `/v1/auth/me` | Get current user profile | Yes (Bearer) |

### User Management (Admin Only)
| Method | Path | Description | Role |
|--------|------|-------------|------|
| GET | `/v1/users` | List all users | admin |
| POST | `/v1/users` | Create a user | admin |
| DELETE | `/v1/users/{id}` | Delete a user | admin |
| PUT | `/v1/users/{id}/role` | Update user role | admin |

> **Total: 36 registered REST endpoints across 10 domains**

---

> **Related**: [Authentication Reference](auth.md) | [Middleware Reference](middleware.md) | [Runtime Configuration](../runtime/configuration.md)
