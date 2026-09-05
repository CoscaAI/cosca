# Knowledge Graph

> **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23

## Overview

The Knowledge Graph models the entities and relationships discovered in your project. It enables relationship-aware search, visual exploration, and connection discovery.

```
┌──────────────────────────────────────────────────────────────────┐
│                      KNOWLEDGE GRAPH                              │
│                                                                   │
│       ┌─────────────┐                                            │
│       │  Component  │◀──────────── contains ───────────────┐    │
│       │  Knowledge  │                                        │    │
│       │  Engine     │─── defines ───▶┌─────────────┐       │    │
│       └─────────────┘                │  Interface  │       │    │
│              │                       │  Engine     │       │    │
│              │ depends_on            └─────────────┘       │    │
│              ▼                                             │    │
│       ┌─────────────┐                                      │    │
│       │  Component  │─── defines ───▶┌─────────────┐       │    │
│       │  Search     │                │  Interface  │       │    │
│       │  Engine     │                │  Engine     │       │    │
│       └─────────────┘                └─────────────┘       │    │
│              │                                             │    │
│              │ implements                                  │    │
│              ▼                                             │    │
│       ┌─────────────┐                                      │    │
│       │    Class    │─── defined_in ──▶┌─────────────┐    │    │
│       │ HybridEngine│                  │   File      │    │    │
│       └─────────────┘                  │ search.go   │    │    │
│                                        └─────────────┘    │    │
└──────────────────────────────────────────────────────────────┘
```

---

## Entity Types

The graph supports the following entity types, automatically extracted during indexing:

| Entity Type | Description | Examples |
|-------------|-------------|----------|
| `component` | A system component or module | `KnowledgeEngine`, `Runtime`, `PluginManager` |
| `interface` | A Go interface or abstraction | `Engine`, `Store`, `Subsystem` |
| `class` | A concrete type or struct | `HybridEngine`, `FileStore`, `RuntimeState` |
| `function` | A function or method | `Search()`, `IndexDocument()`, `Start()` |
| `package` | A Go package or module | `internal/knowledge`, `pkg/cosca` |
| `file` | A source file | `knowledge.go`, `runtime.go` |
| `config` | A configuration schema | `Config`, `RuntimeConfig`, `SearchParams` |
| `concept` | A domain concept | `Hybrid search`, `State machine`, `Plugin lifecycle` |
| `dependency` | An external dependency | `cobra`, `zerolog`, `sqlite` |
| `document` | A documentation file | `README.md`, `docs/architecture/overview.md` |

---

## Relationship Types

| Relationship | Direction | Description | Example |
|-------------|-----------|-------------|---------|
| `contains` | Parent → Child | Parent contains child | Package contains file |
| `defines` | Source → Target | Source defines target | File defines interface |
| `implements` | Implementer → Interface | Implementer implements | Struct implements interface |
| `depends_on` | Dependent → Dependency | Depends on another | Package depends on package |
| `imports` | Importer → Imported | Imports another | File imports package |
| `extends` | Child → Parent | Extends or inherits | Interface extends interface |
| `uses` | User → Used | Uses another | Function uses type |
| `references` | Referencer → Referenced | References by name | Document references concept |
| `defined_in` | Definition → Location | Defined in location | Type defined in file |
| `called_by` | Callee → Caller | Called by another | Function called by function |

---

## Graph Operations

### CLI Commands

```bash
# Show graph statistics
cosca knowledge graph stats

# Search entities in the graph
cosca knowledge search "Runtime" --graph --type entity

# Get entity details with connections
cosca knowledge graph node <entity-id>

# Get entity neighbors
cosca knowledge graph neighbors <entity-id>

# Visualize graph (text-based)
cosca knowledge graph viz --depth 2

# Export graph
cosca knowledge graph export --format json
cosca knowledge graph export --format dot
```

### Searching with Graph

```bash
# Search entities
cosca knowledge search "KnowledgeEngine" --type entity

# Search with graph traversal enabled
cosca knowledge search "search" --graph

# Search with both graph and text
cosca knowledge search "hybrid search" --graph --type document --type entity
```

### Graph Statistics

```bash
$ cosca knowledge graph stats

Graph Statistics:
  Nodes:            1,247
  Edges:            3,892
  Entity Types:     10
  Relationship Types: 10
  Avg Connections/Node: 3.1
  Most Connected:   "Runtime" (24 connections)
  Graph Density:    0.003
```

### Node Details

```bash
$ cosca knowledge graph node "node-abc123"

Entity: "KnowledgeEngine"
  Type: Component
  Metadata:
    description: "Central knowledge engine coordinating all subsystems"
    package: "internal/knowledge"
    file: "internal/knowledge/knowledge.go"
  Connections (6):
    → "Engine" (implements interface)
    → "internal/knowledge" (contained in package)
    → "SQLiteDB" (uses)
    → "Search Engine" (contains)
    → "indexer" (contains)
    → "Config" (configures)
```

---

## Visualization

### Text-based visualization

```bash
$ cosca knowledge graph viz "Runtime" --depth 1

Runtime (Component)
├── contains → RuntimeState (StateMachine)
├── contains → EventBus (EventSystem)
├── contains → Metrics (MetricsCollector)
├── contains → Lifecycle (LifecycleManager)
├── configures → RuntimeConfig (Config)
├── implements → Subsystem (Interface)
└── depends_on → KnowledgeEngine (Component)
```

### DOT format for Graphviz

```bash
cosca knowledge graph export --format dot > graph.dot
dot -Tsvg graph.dot -o graph.svg
```

---

## Built-in Queries

### Find orphaned entities
Entities with no relationships:

```bash
cosca knowledge search "" --type entity --graph
# Results with score < 0.1 are likely orphans
```

### Find hub entities
Most connected entities (potential core components):

```bash
cosca knowledge graph stats
# Look for highest "Avg Connections/Node"
```

### Find isolated components
Nodes with few connections:

```bash
cosca knowledge search "" --type entity --graph | grep "connections.*[0-2]"
```

---

## Examples

### Example 1: Component Dependency Analysis

```bash
$ cosca knowledge graph node "PluginManager"

Entity: "PluginManager"
  Type: Component
  Connections (12):
    → "Plugin" (manages interface)
    → "PluginManifest" (validates)
    → "LifecycleManager" (coordinates)
    → "HookRegistry" (uses)
    → "EventBus" (publishes to)
    → "internal/plugins" (contained in)
    → "plugin.go" (defined in)
    → "lifecycle.go" (defined in)
    → "PluginState" (manages enum)
    → "PluginRuntime" (supports runtime)
    → "PluginContext" (provides)
    → "RuntimeAPI" (exposes)
```

### Example 2: Finding Implementation Patterns

```bash
$ cosca knowledge graph viz "Engine" --depth 2

Engine (Interface)
├── implements → KnowledgeEngine (Component)
│   └── defined_in → knowledge.go (File)
├── implements → SearchEngine (Component)
│   └── defined_in → search.go (File)
├── implements → DiscoveryEngine (Component)
│   └── defined_in → discovery.go (File)
└── uses → SearchParams (Config)
    └── defined_in → search.go (File)
```

### Example 3: Cross-Reference Discovery

```bash
$ cosca knowledge search "EventBus" --graph --type entity

Entity: "EventBus"
  Type: Component
  Used by:
    - Runtime (publishes events)
    - PluginManager (publishes events)
    - LifecycleManager (publishes events)
  Uses:
    - EventType (enum)
    - Event (struct)
    - EventHandler (func type)
```

---

## Implementation Details

### Graph Storage
- Graph is maintained in-memory during runtime
- Persisted to SQLite cache table on shutdown
- Loaded from cache on startup
- Serialized as JSON via `graph.Serialize()` / `graph.Deserialize()`

### Entity Extraction
Entities are extracted during indexing from:
- Go source code (types, interfaces, functions, packages)
- Markdown documents (concepts via heading analysis)
- YAML configuration schemas
- Import statements
- Code comments and documentation references

### Graph Construction
```go
// Graph builder adds entities and relationships during indexing
graphBuilder.AddEntity("Runtime", "component", metadata)
graphBuilder.AddEntity("StateMachine", "statemachine", metadata)
graphBuilder.AddRelationship("Runtime", "contains", "StateMachine")
```

---

> **Related**: [Knowledge Engine Overview](overview.md) | [Search Guide](search.md)
