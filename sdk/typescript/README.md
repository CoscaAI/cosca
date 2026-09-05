# @cosca/sdk

**Official TypeScript SDK for the Cosca Enterprise Platform.**

Provides a programmatic API for interacting with the Cosca Runtime. Use it to build plugins, integrations, and automation workflows that leverage the full power of the Cosca platform.

---

## Installation

```bash
npm install @cosca/sdk
# or
yarn add @cosca/sdk
# or
pnpm add @cosca/sdk
```

---

## Quick Start

```typescript
import { CoscaClient } from '@cosca/sdk';

// Create a client connected to the Cosca Runtime
const client = new CoscaClient({
  baseUrl: 'http://localhost:9090',
  apiKey: 'my-api-key',
  timeout: 30000,
});

// Search the knowledge base
const results = await client.knowledge.search('authentication flow', {
  limit: 10,
  minScore: 0.7,
});

console.log(results);

// Check runtime health
const health = await client.runtime.health();
console.log(`Runtime status: ${health.status}`);

// List installed plugins
const plugins = await client.plugins.list();
console.log(`Installed plugins: ${plugins.length}`);
```

---

## API Reference

### CoscaClient

The main entry point. Provides access to all sub-APIs.

```typescript
const client = new CoscaClient({
  baseUrl: string,     // Runtime URL (required)
  apiKey?: string,     // Authentication key
  timeout?: number,    // Request timeout (ms, default: 30000)
  retryCount?: number, // Max retries (default: 3)
  headers?: object,    // Additional HTTP headers
});
```

### Knowledge API

Search and manage the Cosca knowledge base.

| Method | Description |
|--------|-------------|
| `knowledge.index(path)` | Index a document |
| `knowledge.search(query, opts?)` | Search the knowledge base |
| `knowledge.searchByType(type, query)` | Search by entity type |
| `knowledge.stats()` | Get knowledge base statistics |
| `knowledge.rebuild()` | Rebuild the entire index |
| `knowledge.hybridSearch(query, opts?)` | Hybrid vector + keyword search |

### Memory API

Store and retrieve agent memories.

| Method | Description |
|--------|-------------|
| `memory.store(record)` | Store a memory record |
| `memory.retrieve(id)` | Retrieve a memory by ID |
| `memory.search(query, layer?)` | Search memories |
| `memory.listSnapshots()` | List all snapshots |
| `memory.createSnapshot(name)` | Create a new snapshot |
| `memory.restoreSnapshot(id)` | Restore from snapshot |

### Context API

Build and manage runtime context.

| Method | Description |
|--------|-------------|
| `context.build(query, opts?)` | Build a runtime context |
| `context.getCurrent()` | Get the current context |
| `context.clear()` | Clear the current context |

### Runtime API

Control and monitor the runtime.

| Method | Description |
|--------|-------------|
| `runtime.start()` | Start the runtime |
| `runtime.stop()` | Stop the runtime |
| `runtime.status()` | Get runtime status |
| `runtime.health()` | Get detailed health report |

### Plugins API

Manage runtime plugins.

| Method | Description |
|--------|-------------|
| `plugins.install(source)` | Install a plugin |
| `plugins.uninstall(id)` | Uninstall a plugin |
| `plugins.list()` | List all plugins |
| `plugins.get(id)` | Get plugin details |

### Discovery API

Auto-detect environment information.

| Method | Description |
|--------|-------------|
| `discovery.project()` | Detect project info |
| `discovery.workspace()` | Detect workspace info |
| `discovery.editor()` | Detect editor info |
| `discovery.discover()` | Detect everything |

### Orchestration API

Execute prompts through AI agents and stream responses.

| Method | Description |
|--------|-------------|
| `orchestration.run(prompt, opts?)` | Execute a prompt synchronously |
| `orchestration.stream(prompt, opts?)` | Stream response as SSE events |

### Agents API

Discover and inspect Cosca agents.

| Method | Description |
|--------|-------------|
| `agents.list()` | List all available agents |
| `agents.search(query)` | Search agents by name/role/department |
| `agents.get(name)` | Get agent details by name |

### Skills API

Discover, inspect, and install Cosca skills.

| Method | Description |
|--------|-------------|
| `skills.list()` | List all available skills |
| `skills.search(query)` | Search skills by name/description/category |
| `skills.get(name)` | Get skill details by name |
| `skills.install(name, source)` | Install a skill from file or URL |

### Providers API

Manage AI/LLM providers (OpenAI, Anthropic, Ollama, etc.).

| Method | Description |
|--------|-------------|
| `providers.list()` | List all available providers |
| `providers.get(name)` | Get provider details by name |
| `providers.test(name)` | Test provider connectivity |
| `providers.setActive(name, model?)` | Set the active provider |

### Workflows API

Discover, inspect, and execute Cosca workflows.

| Method | Description |
|--------|-------------|
| `workflows.list()` | List all available workflows |
| `workflows.search(query)` | Search workflows by name/description |
| `workflows.get(name)` | Get workflow details by name |
| `workflows.run(name)` | Execute a workflow by name |

---

## Error Handling

All SDK methods throw `CoscaError` on failure, which includes the HTTP status code and an Cosca error code:

```typescript
import { CoscaClient, CoscaError } from '@cosca/sdk';

const client = new CoscaClient({ baseUrl: 'http://localhost:9090' });

try {
  const results = await client.knowledge.search('query');
} catch (err) {
  if (err instanceof CoscaError) {
    console.error(`Error ${err.code}: ${err.message} (HTTP ${err.statusCode})`);
  }
}
```

Common error codes:
- `E1005` — Request timed out
- `E2004` — Missing required configuration
- `E4005` — No search results found
- `E5001` — Authentication failed
- `E10002` — Connection refused

---

## Examples

### Orchestration — Run and Stream

```typescript
// Execute a prompt synchronously
const result = await client.orchestration.run('Explain dependency injection', {
  agent: 'assistant',
  provider: 'openai',
});
console.log(result.response);
console.log(`Skills used: ${result.skillsUsed?.join(', ')}`);

// Stream the response in real time
for await (const event of client.orchestration.stream('Write a hello world server', {
  agent: 'coder',
})) {
  if (event.type === 'thinking') {
    console.log('[thinking]', event.content);
  } else if (event.type === 'response') {
    process.stdout.write(event.content);
  } else if (event.type === 'done') {
    console.log(`\nCompleted in ${event.durationMs}ms`);
  }
}
```

### Index Documents and Search

```typescript
// Index a document
const indexResult = await client.knowledge.index('/path/to/document.md');
console.log(`Indexed ${indexResult.documentsIndexed} documents`);

// Search with filters
const results = await client.knowledge.search('API design', {
  limit: 20,
  minScore: 0.6,
  types: ['file'],
});

// Hybrid search (FTS + vector)
const hybridResults = await client.knowledge.searchHybrid(
  'vector database',
  { limit: 5, enableGraph: true },
);
```

### Memory Management

```typescript
// Store a memory record
const { id } = await client.memory.store({
  type: 'decision',
  layer: 'session',
  content: 'Chose PostgreSQL for the primary database',
  priority: 3,
  metadata: { context: 'architecture-decision' },
});

// Promote to project layer
const promoted = await client.memory.promote(id, 'session', 'project');
console.log(`Promoted to layer: ${promoted.record.layer}`);

// Search relevant memories
const memories = await client.memory.search('database', {
  limit: 5,
  layers: ['project'],
});
```

### Automatic Environment Discovery

```typescript
const report = await client.discovery.discover();

console.log(`Project: ${report.project.name} (${report.project.language})`);
console.log(`Editor: ${report.editor.name}`);
console.log(`Git branch: ${report.workspace.gitBranch}`);
```

### Agent and Workflow Execution

```typescript
// Find available agents
const agents = await client.agents.search('code');
console.log(`Found ${agents.length} code-related agents`);

// List and run a workflow
const workflows = await client.workflows.list();
const result = await client.workflows.run('code-review');
console.log(`Workflow ${result.status} — ${result.stepsCompleted}/${result.totalSteps} steps`);
```

### Provider Management

```typescript
// Test a provider
const test = await client.providers.test('openai');
console.log(`OpenAI: ${test.status} (${test.responseTime})`);

// Activate a provider with a specific model
const status = await client.providers.setActive('openai', 'gpt-4o-mini');
console.log(`Active: ${status.active}, Configured: ${status.configured}/${status.available}`);
```

---

## License

MIT — © 2024 Cosca Contributors
