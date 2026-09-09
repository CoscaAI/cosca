# MCP Tools Expansion Plan — Cosca v1.4.0-dev

> **Version**: 1.0.0 | **Status**: draft | **Owner**: Backend Chief (cosca-backend) | **Date**: 2026-07-28

## 1. Discovery Summary

### 1.1. Location of Current MCP Tools

| Layer | File | Purpose |
|-------|------|---------|
| **MCP Server (runtime)** | `api/mcp/server.go` | JSON-RPC 2.0 server over stdin/stdout. The actual MCP tools are defined and handled here. |
| **Editor adapter (config)** | `internal/editors/generic_mcp/generic_mcp.go` | Generates MCP config files for editors. Defines tool **schemas only** — no implementation. |
| **LLM Tool Executor** | `internal/orchestration/tool_exec.go` | Internal tool execution for LLM agent loops (6 filesystem/command/memory tools). **Not MCP.** |
| **Chat Tool Definitions** | `internal/chat/types.go` (line 134-168) | `ToolDefinition` for LLM function calling. Used by providers. **Not MCP.** |

**Key insight**: The MCP server at `api/mcp/server.go` is the **single source of truth** for MCP tools exposed to external editors/agents. It is currently **not wired into the runtime** (`cmd/cosca/main.go` uses CLI, not MCP serve). The `generic_mcp` adapter generates static configuration files but the actual runtime tools pass through `api/mcp/server.go`.

### 1.2. Three Existing MCP Tools (Implemented)

All defined in `api/mcp/server.go`:

| # | Tool Name | Description | Handler | Manager Used |
|---|-----------|-------------|---------|-------------|
| 1 | `cosca_knowledge_search` | Search Cosca knowledge base (hybrid FTS + vector + graph) | `handleKnowledgeSearch` | `*knowledge.Engine` |
| 2 | `cosca_memory_store` | Store a memory record | `handleMemoryStore` | `*memory.MemoryEngine` |
| 3 | `cosca_runtime_status` | Get runtime status and health | `handleRuntimeStatus` | `*runtime.Runtime` |

### 1.3. Four Missing Tool Groups (To Implement)

| Group | Proposed Tools | Related Manager |
|-------|---------------|-----------------|
| **agent** | `cosca_agent_list`, `cosca_agent_inspect`, `cosca_agent_search` | `internal/agents.Manager` |
| **skill** | `cosca_skill_list`, `cosca_skill_inspect`, `cosca_skill_search` | `internal/skills.Manager` |
| **workflow** | `cosca_workflow_list`, `cosca_workflow_inspect`, `cosca_workflow_run`, `cosca_workflow_search` | `internal/workflows.Manager` |
| **provider** | `cosca_provider_list`, `cosca_provider_info`, `cosca_provider_test`, `cosca_provider_set_active` | `internal/providers.Manager` |

**Total new tools**: 14

---

## 2. Implementation Pattern

### 2.1. How a Tool is Defined (Registration)

Each tool requires three pieces in `api/mcp/server.go`:

```go
// STEP 1: Define the JSON Schema for input parameters
var coscaAgentListInputSchema = json.RawMessage(`{
    "type": "object",
    "properties": {},
    "required": []
}`)

// STEP 2: Add to the listToolsResponse global var
var listToolsResponse = toolsListResponse{
    Tools: []ToolDefinition{
        // ... existing tools ...
        {
            Name:        "cosca_agent_list",
            Description: "List all available Cosca agents",
            InputSchema: coscaAgentListInputSchema,
        },
    },
}

// STEP 3: Add case in handleToolCall switch
func (s *Server) handleToolCall(req Request) {
    // ...
    switch params.Name {
    // ... existing cases ...
    case "cosca_agent_list":
        s.handleAgentList(req.ID, params.Arguments)
    default:
        s.writeError(req.ID, -32601, "Method not found", ...)
    }
}
```

### 2.2. How a Tool is Handled

```go
// STEP 4: Handler method on *Server
func (s *Server) handleAgentList(id json.RawMessage, args json.RawMessage) {
    if s.agentManager == nil {
        s.writeError(id, -32000, "Agent manager not available", nil)
        return
    }

    // Parse optional args
    var params struct{ /* optional filters */ }
    if args != nil { json.Unmarshal(args, &params) }

    // Call manager API
    agents := s.agentManager.List()

    // Format result as JSON
    s.writeResult(id, map[string]interface{}{
        "agents": agents,
        "total":  len(agents),
    })
}
```

### 2.3. Server Struct Change

The `Server` struct currently holds 3 managers. It needs to be extended:

```go
type Server struct {
    // Existing
    knowledgeEngine *knowledge.Engine
    memoryEngine    *memory.MemoryEngine
    runtimeInstance *runtime.Runtime

    // NEW - to be added
    agentManager    *agents.Manager
    skillManager    *skills.Manager
    workflowManager *workflows.Manager
    providerManager *providers.Manager
}
```

And `New()` must accept the new managers. All can be nil (graceful "not available" errors).

### 2.4. Naming Convention

| Prefix | Purpose | Pattern |
|--------|---------|---------|
| `cosca_agent_*` | Agent operations | `cosca_agent_{list,inspect,search}` |
| `cosca_skill_*` | Skill operations | `cosca_skill_{list,inspect,search}` |
| `cosca_workflow_*` | Workflow operations | `cosca_workflow_{list,inspect,run,search}` |
| `cosca_provider_*` | Provider operations | `cosca_provider_{list,info,test,set_active}` |

The existing tools use `cosca_knowledge_search`, `cosca_memory_store`, `cosca_runtime_status` — the new tools follow the same `cosca_<domain>_<action>` convention.

---

## 3. Tool Specifications

### 3.1. Cosca Agent Tools

#### `cosca_agent_list`
- **Description**: List all available Cosca agents with their departments, roles, and statuses.
- **Parameters**: None
- **Returns**: `{"agents": [...Agent], "total": N}`
- **Manager API**: `agents.Manager.List() []Agent`
- **Agent struct returned**: Name, Role, Mission, Status, Version, Department, ReportsTo, Description

#### `cosca_agent_inspect`
- **Description**: Get detailed information about a specific agent including capabilities, tools, responsibilities, and dependencies.
- **Parameters**:
  - `name` (string, required) — Agent name (case-insensitive)
- **Returns**: `{...Agent (full details)}`
- **Manager API**: `agents.Manager.Get(name string) (*Agent, error)`
- **Note**: Returns all fields including Capabilities[], Tools[], Responsibilities[], Dependencies[], Inputs[], Outputs[]

#### `cosca_agent_search`
- **Description**: Search for agents by query string matching name, role, department, or description.
- **Parameters**:
  - `query` (string, required) — Search term (case-insensitive)
- **Returns**: `{"agents": [...Agent], "total": N}`
- **Manager API**: `agents.Manager.Search(query string) ([]Agent, error)`

---

### 3.2. Cosca Skill Tools

#### `cosca_skill_list`
- **Description**: List all available skills with their categories, versions, and descriptions.
- **Parameters**:
  - `category` (string, optional) — Filter by category
- **Returns**: `{"skills": [...Skill], "total": N}`
- **Manager API**: `skills.Manager.List() []Skill`
- **Skill struct returned**: Name, Description, Version, Category, Source

#### `cosca_skill_inspect`
- **Description**: Get detailed information about a specific skill including instructions and available tools.
- **Parameters**:
  - `name` (string, required) — Skill name (case-insensitive)
- **Returns**: `{...Skill (with Instructions and Tools[])}`
- **Manager API**: `skills.Manager.Get(name string) (*Skill, error)`
- **Note**: Instructions can be large (markdown text) — consider truncation

#### `cosca_skill_search`
- **Description**: Search for skills by query matching name, description, or category.
- **Parameters**:
  - `query` (string, required) — Search term
- **Returns**: `{"skills": [...Skill], "total": N}`
- **Manager API**: `skills.Manager.Search(query string) ([]Skill, error)`

---

### 3.3. Cosca Workflow Tools

#### `cosca_workflow_list`
- **Description**: List all available workflows with status, steps count, and enabled state.
- **Parameters**:
  - `enabled_only` (boolean, optional, default: false) — Show only enabled workflows
- **Returns**: `{"workflows": [...Workflow], "total": N}`
- **Manager API**: `workflows.Manager.List() []Workflow`
- **Workflow struct returned**: Name, Description, Version, Status, Enabled, Steps (count)

#### `cosca_workflow_inspect`
- **Description**: Get detailed workflow definition including all steps, inputs, and outputs.
- **Parameters**:
  - `name` (string, required) — Workflow name (case-insensitive)
- **Returns**: `{...Workflow (with StepList[], Inputs[], Outputs[])}`
- **Manager API**: `workflows.Manager.Get(name string) (*Workflow, error)`

#### `cosca_workflow_run`
- **Description**: Execute a workflow by name. Returns execution result with status, duration, and completed steps.
- **Parameters**:
  - `name` (string, required) — Workflow name (case-insensitive)
  - `timeout_seconds` (integer, optional, default: 300) — Maximum execution time
- **Returns**: `{"status": "...", "duration": "...", "steps_completed": N, "total_steps": N}`
- **Manager API**: `workflows.Manager.Run(ctx context.Context, name string) (*Result, error)`
- **Note**: Creates a context with timeout. Workflow execution may be long-running.

#### `cosca_workflow_search`
- **Description**: Search for workflows by query string matching name or description.
- **Parameters**:
  - `query` (string, required) — Search term
- **Returns**: `{"workflows": [...Workflow], "total": N}`
- **Manager API**: `workflows.Manager.Search(query string) ([]Workflow, error)`

---

### 3.4. Cosca Provider Tools

#### `cosca_provider_list`
- **Description**: List all available LLM providers with their status, configured state, and available models.
- **Parameters**: None
- **Returns**: `{"providers": [...ProviderInfo], "total": N}`
- **Manager API**: `providers.Manager.List() []ProviderInfo`
- **ProviderInfo fields**: Name, Status, Model, Active, Configured, BaseURL, Models[], Capabilities[]

#### `cosca_provider_info`
- **Description**: Get detailed information about a specific provider.
- **Parameters**:
  - `name` (string, required) — Provider name (e.g., "openai", "anthropic", "ollama")
- **Returns**: `{...ProviderInfo (full details)}`
- **Manager API**: `providers.Manager.Info(name string) (ProviderInfo, error)`

#### `cosca_provider_test`
- **Description**: Test connectivity to a provider and return response time and status.
- **Parameters**:
  - `name` (string, required) — Provider name
- **Returns**: `{"response_time": "...", "model": "...", "status": "..."}`
- **Manager API**: `providers.Manager.Test(name string) (TestResult, error)`
- **TestResult fields**: ResponseTime, Model, Status

#### `cosca_provider_set_active`
- **Description**: Set the active LLM provider and optionally the model.
- **Parameters**:
  - `provider` (string, required) — Provider name
  - `model` (string, optional) — Model name (e.g., "gpt-4o", "claude-3-5-sonnet")
- **Returns**: `{"active": "provider_name", "model": "model_name"}`
- **Manager API**: `providers.Manager.SetActive(provider, model string) error`

---

## 4. Manager APIs Reference

### 4.1. Agents Manager (`internal/agents/agents.go`)

```go
type Manager struct { /* agents map[string]*Agent */ }
func NewManager(coscaDir string) *Manager
func (m *Manager) List() []Agent
func (m *Manager) Get(name string) (*Agent, error)           // case-insensitive
func (m *Manager) Search(query string) ([]Agent, error)      // name/role/dept/description
func (m *Manager) Register(a *Agent)                         // or Add(a Agent)
```

**Agent struct** (public fields):
`Name, Role, Mission, Status, Version, Department, ReportsTo, Capabilities[], Tools[], Responsibilities[], Description, Dependencies[], Inputs[], Outputs[]`

### 4.2. Skills Manager (`internal/skills/skills.go`)

```go
type Manager struct { /* skills map[string]*Skill + sync.RWMutex */ }
func NewManager(coscaDir string) *Manager
func (m *Manager) List() []Skill
func (m *Manager) Get(name string) (*Skill, error)           // case-insensitive
func (m *Manager) Search(query string) ([]Skill, error)      // name/description/category
func (m *Manager) Install(name, source string) (*Skill, error) // file or URL
func (m *Manager) Add(skill Skill)
```

**Skill struct** (public fields):
`Name, Description, Version, Category, Instructions, Tools[], Source`

### 4.3. Workflows Manager (`internal/workflows/workflows.go`)

```go
type Manager struct { /* workflows map[string]*Workflow + sync.RWMutex */ }
func NewManager(coscaDir string, opts ...ManagerOption) *Manager
func (m *Manager) List() []Workflow
func (m *Manager) Get(name string) (*Workflow, error)        // case-insensitive
func (m *Manager) Search(query string) ([]Workflow, error)   // name/description
func (m *Manager) Run(ctx context.Context, name string) (*Result, error) // execute
func (m *Manager) Add(workflow Workflow)
func (m *Manager) SetStepRunner(runner StepRunner)
```

**Workflow struct**: `Name, Description, Version, Status, Enabled, Steps, StepList[], Inputs[], Outputs[]`
**Result struct**: `Status, Duration, StepsCompleted, TotalSteps, Outputs[]`

### 4.4. Providers Manager (`internal/providers/providers.go`)

```go
type Manager struct { /* active string, activeModel string, registry ProviderRegistry */ }
func NewManager() *Manager
func (m *Manager) List() []ProviderInfo
func (m *Manager) Info(provider string) (ProviderInfo, error)
func (m *Manager) Test(provider string) (TestResult, error)
func (m *Manager) SetActive(provider, model string) error
func (m *Manager) Status() ProviderStatus
func (m *Manager) SetRegistry(registry ProviderRegistry)
```

**ProviderInfo**: `Name, Status, Model, Active, Configured, BaseURL, APIVersion, Models[], Capabilities[]`
**TestResult**: `ResponseTime, Model, Status`
**ProviderStatus**: `Active, Configured, Available, Statuses[]`

---

## 5. Implementation Order

### Phase 1 — Simple List/Info Tools (low risk)

These tools only read from in-memory maps. No side effects. Each is ~50 lines.

| Priority | Tool | Reason |
|----------|------|--------|
| 1 | `cosca_agent_list` + `cosca_agent_inspect` + `cosca_agent_search` | Agent list is foundational; agents orchestrate everything else |
| 2 | `cosca_skill_list` + `cosca_skill_inspect` + `cosca_skill_search` | Skills are used by agents; natural second step |
| 3 | `cosca_provider_list` + `cosca_provider_info` | Read-only provider info; no side effects |
| 4 | `cosca_provider_test` | Has network side effects; more complex but self-contained |
| 5 | `cosca_provider_set_active` | State mutation; test carefully |

### Phase 2 — Workflow Tools (medium risk)

Workflows involve execution (`Run`) which can be long-running and has complex state.

| Priority | Tool | Reason |
|----------|------|--------|
| 6 | `cosca_workflow_list` + `cosca_workflow_inspect` + `cosca_workflow_search` | Read-only, safe |
| 7 | `cosca_workflow_run` | Execution tool; needs timeout handling, context propagation |

### Phase 3 — Server Wiring

After all tools are implemented, wire the expanded `Server` struct into the CLI/runtime entry point. The `New()` constructor must be updated to accept the 4 new managers.

---

## 6. Architectural Notes

### 6.1. Manager Lifecycle

All four managers (`agents`, `skills`, `workflows`, `providers`) are typically created during application bootstrap. They should be:
1. Created early (before the MCP server)
2. Injected into the MCP `Server` via constructor
3. Nil-safe: if a manager is nil, the tool returns a graceful "not available" error

### 6.2. Thread Safety

- `skills.Manager` — already has `sync.RWMutex`; calls to List/Get/Search are safe from MCP goroutine
- `workflows.Manager` — already has `sync.RWMutex`; same
- `agents.Manager` — **no mutex** (agents are loaded once at startup and read-only after); safe for concurrent reads
- `providers.Manager` — **no mutex**; `SetActive` mutates `m.active`/`m.activeModel` (string assignment is atomic in Go); otherwise read-only

### 6.3. `generic_mcp` vs `api/mcp` Distinction

- `internal/editors/generic_mcp/` — Generates **static MCP config files** (`.mcp/cosca-server.json`) for editor setup. Its `MCPToolDefinition` is a schema-only type. The tools defined here (`cosca_search`, `cosca_index`, `cosca_context`, `cosca_status`, `cosca_memory`) are a **different set** from the runtime MCP server. They may need alignment in a future version.
- `api/mcp/server.go` — The **runtime MCP server**. This is where all new tools should be added.

### 6.4. JSON Marshaling

All managers return structs with `json` tags. They serialize cleanly via `json.Marshal`. No custom serialization needed. The `Agent` struct is the largest (~15 fields) but still well under typical MCP message size limits.

### 6.5. Error Handling Pattern

Follow the existing pattern:
```go
s.writeError(id, -32000, "Agent manager not available", nil)   // missing dependency
s.writeError(id, -32602, "Invalid arguments", "name is required")  // bad input
s.writeError(id, -32000, "Search failed", err.Error())             // runtime error
```

---

## 7. Files to Modify

| File | Change |
|------|--------|
| `api/mcp/server.go` | Add 4 manager fields to `Server` struct; update `New()` constructor; add 14 tool definitions; add 14 handler methods; extend `handleToolCall` switch; add 14 input schemas |

**No other files need modification.** All managers are consumed as-is through their existing public APIs.

---

## 8. Risks & Dependencies

| Risk | Mitigation |
|------|-----------|
| `workflows.Manager.Run()` can hang | Always pass `context.WithTimeout` from within the handler |
| `agents.Manager` lacks mutex but is mutated by `Register/Add` | Add `sync.RWMutex` to agents.Manager if concurrent register calls are possible; otherwise document that agents are loaded at startup |
| MCP server not wired to runtime | Phase 3 handles wiring; all tools still functional even when manager is nil (returns "not available") |
| `skills.Manager.Instructions` field can be very large | Truncate to 10KB in `cosca_skill_inspect` response |

---

## 9. Next Steps

1. **Review this plan** — Confirm tool names, parameters, and order
2. **Implement Phase 1** — Agent tools (3 tools, ~150 lines)
3. **Implement Phase 1** — Skill tools (3 tools, ~150 lines)
4. **Implement Phase 1** — Provider tools (5 tools, ~250 lines)
5. **Implement Phase 2** — Workflow tools (4 tools, ~200 lines)
6. **Implement Phase 3** — Wire MCP server into CLI/runtime
7. **Test** — Use `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | cosca mcp --stdio`
