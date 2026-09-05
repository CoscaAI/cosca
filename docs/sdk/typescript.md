# TypeScript SDK Reference

> **Status**: active | **Owner**: SDK Chief | **Last Updated**: 2026-07-23

## Overview

The TypeScript SDK provides programmatic access to the Cosca platform from Node.js applications, enabling knowledge search, memory management, and runtime operations.

---

## Installation

```bash
npm install @cosca/sdk
```

---

## Client Setup

### Basic Client

```typescript
import { AOSClient } from "@cosca/sdk";

const client = new AOSClient({
  dataDir: "/path/to/data",
});

// Check health
const health = await client.health();
console.log(`Health: ${health.status}`);
```

### With Options

```typescript
const client = new AOSClient({
  dataDir: "/path/to/data",
  runtimeDir: "/path/to/runtime",
  configPath: "/path/to/config.yaml",
  logLevel: "debug",
});
```

---

## API Reference

### Client

```typescript
class AOSClient {
  constructor(options?: ClientOptions);

  // Lifecycle
  async connect(): Promise<void>;
  async close(): Promise<void>;

  // Knowledge
  async search(query: string, options?: SearchOptions): Promise<SearchResult>;
  async searchWithParams(params: SearchParams): Promise<SearchResult>;
  async index(path: string, recursive?: boolean): Promise<void>;
  async sync(): Promise<SyncResult>;
  async getKnowledgeStats(): Promise<KnowledgeStats>;
  async rebuildIndex(): Promise<void>;

  // Memory
  async storeMemory(record: MemoryRecord): Promise<MemoryRecord>;
  async searchMemory(query: string, options?: MemorySearchOptions): Promise<MemoryRecord[]>;
  async getMemory(id: string, layer: string): Promise<MemoryRecord | null>;
  async deleteMemory(id: string, layer: string): Promise<void>;
  async promoteMemory(id: string, from: string, to: string): Promise<MemoryRecord>;
  async getMemoryStats(): Promise<Record<string, LayerStats>>;

  // Runtime
  async health(): Promise<HealthReport>;
  async status(): Promise<RuntimeStatus>;
  async startRuntime(daemon?: boolean): Promise<void>;
  async stopRuntime(): Promise<void>;
  async restartRuntime(): Promise<void>;

  // Plugins
  async installPlugin(path: string): Promise<void>;
  async listPlugins(): Promise<PluginInfo[]>;
  async getPluginInfo(id: string): Promise<PluginInfo>;
  async removePlugin(id: string): Promise<void>;
  async enablePlugin(id: string): Promise<void>;
  async disablePlugin(id: string): Promise<void>;
}
```

### ClientOptions

```typescript
interface ClientOptions {
  dataDir?: string;
  runtimeDir?: string;
  configPath?: string;
  logLevel?: "debug" | "info" | "warn" | "error";
}
```

### Search

```typescript
interface SearchOptions {
  limit?: number;
  offset?: number;
  types?: ("document" | "chunk" | "entity" | "code")[];
  path?: string;
  tags?: Record<string, string>;
  minScore?: number;
  graph?: boolean;
  facets?: boolean;
}

interface SearchResult {
  results: Result[];
  totalCount: number;
  query: string;
  facets?: Record<string, Record<string, number>>;
  duration: number;
  suggestions?: string[];
}

interface Result {
  id: string;
  type: string;
  score: number;
  title: string;
  content: string;
  snippet: string;
  documentPath?: string;
  language?: string;
  entityType?: string;
  metadata?: Record<string, string>;
  source: string;
  rank: number;
}
```

### Memory

```typescript
interface MemoryRecord {
  id?: string;
  type: "decision" | "pattern" | "bug" | "agent" | "project" | "architecture" | "session";
  layer: "global" | "workspace" | "project" | "session" | "temp";
  scope?: string;
  content: string;
  metadata?: Record<string, string>;
  createdAt?: Date;
  ttl?: number;
  priority?: number;
}

interface MemorySearchOptions {
  types?: string[];
  layers?: string[];
  limit?: number;
  offset?: number;
}

interface LayerStats {
  name: string;
  count: number;
  totalSize: number;
  highestPriority: number;
}
```

### Runtime

```typescript
interface HealthReport {
  state: string;
  health: string;
  uptime: string;
  startedAt: string;
  version: string;
  components: Record<string, ComponentInfo>;
}

interface ComponentInfo {
  name: string;
  status: string;
  message?: string;
  uptime: number;
}

interface RuntimeStatus {
  state: string;
  health: string;
  version: string;
  uptime: string;
  components: Record<string, ComponentInfo>;
}
```

### Plugins

```typescript
interface PluginInfo {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  runtime: "go" | "wasm" | "external" | "sharedlib";
  state: "installed" | "initialized" | "started" | "stopped" | "error";
  enabled: boolean;
  permissions: string[];
  hooks: string[];
}
```

### Sync

```typescript
interface SyncResult {
  added: string[];
  removed: string[];
  updated: string[];
  errors: string[];
  duration: number;
}
```

---

## Examples

### Searching Knowledge

```typescript
import { AOSClient } from "@cosca/sdk";

const client = new AOSClient({ dataDir: "./tmp/cosca-data" });

// Basic search
const results = await client.search("authentication flow");
console.log(`Found ${results.totalCount} results`);
results.results.forEach(r => {
  console.log(`#${r.rank} [${r.score.toFixed(2)}] ${r.title}`);
});

// Advanced search
const filtered = await client.search("database", {
  limit: 10,
  types: ["document"],
  path: "docs/",
  minScore: 0.8,
  facets: true,
});

console.log("Facets:", filtered.facets);
```

### Managing Memory

```typescript
// Store a memory
const record = await client.storeMemory({
  type: "decision",
  layer: "project",
  scope: "backend",
  content: "Use PostgreSQL with PgBouncer",
  priority: 8,
  metadata: { author: "developer" },
});

console.log(`Stored: ${record.id}`);

// Search memory
const memories = await client.searchMemory("database", {
  types: ["decision"],
  limit: 5,
});

// Promote memory
await client.promoteMemory(record.id!, "session", "project");
```

### Runtime Status

```typescript
const status = await client.status();
console.log(`Runtime: ${status.state} (${status.health})`);

for (const [name, component] of Object.entries(status.components)) {
  console.log(`  ${name}: ${component.status}`);
}
```

### Error Handling

```typescript
try {
  const results = await client.search("query", { types: ["invalid"] });
} catch (error) {
  if (error instanceof AOSAuthError) {
    console.error("Authentication failed");
  } else if (error instanceof AOSConfigError) {
    console.error("Configuration error:", error.message);
  } else {
    console.error("Unknown error:", error);
  }
}
```

---

> **Related**: [Go SDK Reference](go.md) | [CLI Overview](../cli/overview.md)
