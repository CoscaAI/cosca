# Multi Runtime — Extracted from KERNEL.md §17

> **Source**: KERNEL.md v3.0.1 §17 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 17. MULTI RUNTIME

The Kernel is **fully independent** of any specific tool or runtime. The same Kernel specification runs identically across all runtimes. Every runtime implements the **Runtime Abstraction Layer** defined by `RUNTIME_CONTRACT.md` and MUST pass the **Runtime Certification Suite** before being considered compliant.

---

### 17.1 Multi Runtime Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Cosca KERNEL SPECIFICATION                            │
│  (Defines behavior, state machine, events, contracts, pipeline, DAG, etc.)  │
└──────────────────────────┬───────────────────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         RUNTIME ABSTRACTION LAYER                            │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                     RUNTIME CONTRACT                                   │  │
│  │  (RUNTIME_CONTRACT.md — formal interface between Kernel and Runtime)  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │  Adapter │  │  Adapter │  │  Adapter │  │  Adapter │  │  Adapter │    │
│  │  Cosca Go  │  │ OpenCode │  │ClaudeCode│  │  Codex   │  │  SDK (TS)│    │
│  │ Runtime  │  │          │  │          │  │          │  │          │    │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘    │
│       │             │             │             │             │           │
│       ▼             ▼             ▼             ▼             ▼           │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    IMPLEMENTATION LAYER                               │  │
│  │  Go code,  │  OpenCode  │  Claude    │  Codex     │  TypeScript  │  │
│  │  plugins   │  skills    │  projects  │  actions   │  packages    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

### 17.2 Runtime Adapter Contract

Every runtime MUST implement the Runtime Adapter Contract to be Cosca-compliant.

```yaml
runtime_adapter_contract:
  version: "1.0.0"
  
  mandatory_interfaces:
    - name: "Session Lifecycle"
      methods:
        - "init(config) → session_id"
        - "discover(workspace) → discovery_result"
        - "route(request) → execution_plan"
        - "execute(plan) → execution_result"
        - "teardown(session_id) → void"
        
    - name: "Event Bus"
      methods:
        - "publish(event) → void"
        - "subscribe(topic, handler) → subscription_id"
        - "unsubscribe(subscription_id) → void"
        
    - name: "State Machine"
      methods:
        - "get_state() → state"
        - "transition(event) → state"
        - "persist_state() → void"
        - "restore_state() → state"
        
    - name: "Capability Resolution"
      methods:
        - "resolve_capabilities(request) → capability[]"
        - "resolve_provider(capability) → provider"
        
    - name: "Memory Management"
      methods:
        - "load_memory(types) → memory_map"
        - "store_memory(record) → void"
        - "search_memory(query) → results"
        
    - name: "Quality Gates"
      methods:
        - "enforce_gate(gate_id, artifacts) → gate_result"
        
    - name: "Tool Interface"
      methods:
        - "execute_tool(tool_name, params) → tool_result"
        - "list_tools() → tool[]"
        
  optional_interfaces:
    - name: "Dashboard Integration"
      methods:
        - "push_event(event) → void"
        - "get_status() → status"
        
    - name: "Hot Reload"
      methods:
        - "watch_files(paths) → void"
        - "reload_file(file) → void"
        
  compliance:
    mandatory: "ALL mandatory interfaces MUST be implemented"
    optional: "Optional interfaces SHOULD be implemented for full compliance"
    certification: "Runtime MUST pass Runtime Certification Suite"
```

---

### 17.3 Supported Runtimes — Detailed Profiles

Each runtime has a detailed profile with capabilities, limitations, and integration specifics.

#### Cosca Go Runtime

```yaml
runtime_aos_go:
  name: "Cosca Go Runtime"
  type: "Native"
  language: "Go"
  since: "v2.0"
  status: "active"
  
  capabilities:
    - "Full Kernel implementation"
    - "Native Event Bus (Redis)"
    - "Native State Machine"
    - "Native DAG Execution"
    - "Native Scheduler"
    - "Hot Reload (inotify)"
    - "Dashboard integration (SSE)"
    - "REST API"
    - "CLI"
    
  limitations:
    - "Requires Go 1.25+"
    - "Requires Redis + PostgreSQL"
    
  integration:
    type: "direct"
    package: "github.com/cosca/runtime"
    documentation: "cosca/runtime/README.md"
```

#### OpenCode Adapter

```yaml
runtime_opencode:
  name: "OpenCode"
  type: "IDE Agent"
  since: "v1.0"
  status: "active"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "Skill-based engine loading"
    - "Subagent spawning (task tool)"
    - "File I/O via tools"
    - "Git integration"
    - "Memory management"
    
  limitations:
    - "No native event bus (simulated via in-memory)"
    - "No native scheduler (sequential execution model)"
    - "No native dashboard"
    - "Session lifecycle managed by IDE"
    
  adapter:
    mechanism: "SKILL.md directives + tool interface"
    bootstrapping: "KERNEL.md loaded as system prompt"
    execution: "Sequential via tool calls"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    config: ".opencode/cosca/cosca.config.yaml"
    contracts: "RUNTIME_CONTRACT.md"
```

#### Claude Code Adapter

```yaml
runtime_claude_code:
  name: "Claude Code"
  type: "CLI Agent"
  since: "v1.2"
  status: "active"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "CLAUDE.md-based configuration"
    - "Tool-based execution"
    - "File I/O"
    - "Git operations"
    - "Session persistence"
    
  limitations:
    - "No native event bus"
    - "Sequential execution model"
    - "No dashboard integration"
    - "Limited parallel execution"
    
  adapter:
    mechanism: "CLAUDE.md directives + tool interface"
    bootstrapping: "KERNEL.md loaded as system prompt"
    execution: "Sequential via tool calls"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    config: "CLAUDE.md"
    contracts: "RUNTIME_CONTRACT.md"
```

#### Codex Adapter

```yaml
runtime_codex:
  name: "Codex"
  type: "CLI Agent"
  since: "v2.0"
  status: "beta"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "Plugin-based execution"
    - "Tool interface"
    - "File I/O"
    - "Git operations"
    
  limitations:
    - "No native event bus"
    - "Sequential execution"
    - "Limited memory management"
    - "Beta status — some features pending"
    
  adapter:
    mechanism: "Codex actions + tool interface"
    bootstrapping: "KERNEL.md loaded as system prompt"
    execution: "Action-based execution"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    contracts: "RUNTIME_CONTRACT.md"
```

#### Gemini CLI Adapter

```yaml
runtime_gemini_cli:
  name: "Gemini CLI"
  type: "CLI Agent"
  since: "v2.0"
  status: "beta"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "Tool-based execution"
    - "File I/O"
    - "Git operations"
    
  limitations:
    - "No native event bus"
    - "Sequential execution"
    - "Limited parallel capabilities"
    - "Beta status"
    
  adapter:
    mechanism: "Gemini CLI tools + prompt directives"
    bootstrapping: "KERNEL.md loaded as system prompt"
    execution: "Sequential via tool calls"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    contracts: "RUNTIME_CONTRACT.md"
```

#### Cursor Adapter

```yaml
runtime_cursor:
  name: "Cursor"
  type: "IDE Agent"
  since: "v2.0"
  status: "beta"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "IDE-integrated execution"
    - "File I/O"
    - "Git integration"
    - "Inline editing"
    
  limitations:
    - "No native event bus"
    - "IDE-dependent lifecycle"
    - "Limited background execution"
    
  adapter:
    mechanism: "Cursor rules + agent interface"
    bootstrapping: "KERNEL.md loaded as context"
    execution: "Sequential via agent"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    contracts: "RUNTIME_CONTRACT.md"
```

#### Continue Adapter

```yaml
runtime_continue:
  name: "Continue"
  type: "IDE Plugin"
  since: "v2.0"
  status: "beta"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "Plugin-based execution"
    - "File I/O"
    - "IDE integration"
    
  limitations:
    - "No native event bus"
    - "Plugin lifecycle constraints"
    - "Limited execution scope"
    
  adapter:
    mechanism: "Continue.json config + plugin API"
    bootstrapping: "KERNEL.md loaded as context"
    execution: "Sequential via plugin"
    events: "In-memory event bus"
    state: "Session-scoped state"
    
  integration:
    type: "adapter"
    contracts: "RUNTIME_CONTRACT.md"
```

#### ADK-Go Adapter

```yaml
runtime_adk_go:
  name: "ADK-Go"
  type: "Go SDK"
  language: "Go"
  since: "v2.0"
  status: "active"
  
  capabilities:
    - "Full Kernel specification compliance"
    - "Programmatic API"
    - "Embeddable in Go applications"
    - "Custom provider support"
    - "Custom plugin support"
    
  limitations:
    - "Go only"
    - "Requires manual integration"
    
  integration:
    type: "sdk"
    package: "github.com/cosca/adk-go"
    contracts: "RUNTIME_CONTRACT.md"
```

---

### 17.4 Runtime Compliance Matrix

| Feature | Cosca Go | OpenCode | Claude Code | Codex | Gemini CLI | Cursor | Continue | ADK-Go |
|---------|--------|----------|-------------|-------|------------|--------|----------|--------|
| **Kernel bootstrap** | ✅ Native | ✅ Skill | ✅ CLAUDE.md | ✅ Action | ✅ Prompt | ✅ Rule | ✅ Plugin | ✅ API |
| **State machine** | ✅ Native | ✅ Simulated | ✅ Simulated | ✅ Simulated | ✅ Simulated | ✅ Simulated | ✅ Simulated | ✅ Native |
| **Event bus** | ✅ Redis | 🔶 In-memory | 🔶 In-memory | 🔶 In-memory | 🔶 In-memory | 🔶 In-memory | 🔶 In-memory | ✅ Custom |
| **Capability resolution** | ✅ Native | ✅ Skill | ✅ Tool | ✅ Action | ✅ Tool | ✅ Agent | ✅ Plugin | ✅ API |
| **DAG execution** | ✅ Native | ⛔ Sequential | ⛔ Sequential | ⛔ Sequential | ⛔ Sequential | ⛔ Sequential | ⛔ Sequential | ✅ Native |
| **Scheduler** | ✅ Native | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ✅ Custom |
| **Quality gates** | ✅ Native | ✅ Tool | ✅ Tool | ✅ Action | ✅ Tool | ✅ Agent | ✅ Plugin | ✅ API |
| **Memory** | ✅ DB+Redis | ✅ Files | ✅ Files | ✅ Files | ✅ Files | ✅ Files | ✅ Files | ✅ Custom |
| **Hot reload** | ✅ inotify | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ✅ Custom |
| **Dashboard** | ✅ SSE | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ✅ API |
| **REST API** | ✅ Native | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ⛔ N/A | ✅ Custom |
| **CLI** | ✅ Native | ✅ Built-in | ✅ Built-in | ✅ Built-in | ✅ Built-in | ✅ Built-in | ✅ Built-in | ⛔ N/A |
| **Multi-session** | ✅ Yes | 🔶 Limited | 🔶 Limited | 🔶 Limited | 🔶 Limited | 🔶 Limited | 🔶 Limited | ✅ Yes |
| **Persistence** | ✅ Full | 🔶 Session | 🔶 Session | 🔶 Session | 🔶 Session | 🔶 Session | 🔶 Session | ✅ Full |

Legend: ✅ = Full support, 🔶 = Partial/simulated, ⛔ = Not available

---

### 17.5 Runtime Detection & Auto-Configuration

The Kernel SHOULD auto-detect which runtime it is executing on and adapt accordingly.

```yaml
runtime_detection:
  methods:
    - detection: "Environment variable"
      variable: "COSCA_RUNTIME"
      values: ["cosca-go", "opencode", "claude-code", "codex", "gemini-cli", "cursor", "continue", "adk-go"]
      
    - detection: "Tool availability"
      checks:
        - "Has 'skill' tool? → OpenCode"
        - "Has 'task' tool? → OpenCode"
        - "Has 'Claude' in user-agent? → Claude Code"
        - "Has 'Codex' in user-agent? → Codex"
        
    - detection: "Configuration file"
      checks:
        - "Has .opencode/ directory? → OpenCode"
        - "Has CLAUDE.md? → Claude Code"
        - "Has .cursor/ directory? → Cursor"
        - "Has .continue/ directory? → Continue"
        
  auto_configuration:
    on_detect:
      - "Load runtime-specific adapter"
      - "Configure event bus (in-memory for adapters)"
      - "Configure scheduler (sequential for adapters)"
      - "Set feature flags based on compliance matrix"
      - "Publish RuntimeDetected event"
      
  fallback: "Assume full Cosca Go Runtime capabilities"
```

---

### 17.6 Runtime-Specific Configuration

Each runtime type may have specific configuration requirements:

| Runtime | Config File | Key Settings |
|---------|-------------|--------------|
| Cosca Go Runtime | `cosca.yaml` or environment | `event_bus.type`, `scheduler.type`, `database.url` |
| OpenCode | `.opencode/config.json` | `model`, `max_tokens`, `tools` |
| Claude Code | `CLAUDE.md` | Agent DNA, allowed tools |
| Codex | Codex config | Agent configuration |
| Gemini CLI | Gemini config | Tool permissions |
| Cursor | `.cursor/rules/` | Agent rules |
| Continue | `continue.json` | Model, provider |
| ADK-Go | Programmatic | SDK configuration |

---

### 17.7 Building a New Runtime Adapter

#### Adapter Development Guide

```yaml
adapter_development:
  prerequisites:
    - "Read RUNTIME_CONTRACT.md"
    - "Read KERNEL.md (this specification)"
    - "Understand target runtime's extension model"
    
  steps:
    step_1: "Implement Session Lifecycle (init, discover, route, execute, teardown)"
    step_2: "Implement Event Bus (publish, subscribe, unsubscribe)"
    step_3: "Implement State Machine (get_state, transition, persist, restore)"
    step_4: "Implement Capability Resolution (resolve_capabilities, resolve_provider)"
    step_5: "Implement Memory Management (load, store, search)"
    step_6: "Implement Quality Gates (enforce_gate)"
    step_7: "Implement Tool Interface (execute_tool, list_tools)"
    step_8: "Implement optional interfaces (Dashboard, Hot Reload)"
    step_9: "Pass Runtime Certification Suite"
    step_10: "Register in COSCA_INDEX.md"
    
  certification:
    suite: "cosca/runtime-certification/"
    tests: 50+
    coverage: "All mandatory interfaces"
    output: "RuntimeCertificationReport"
```

---

### 17.8 Runtime Certification Suite

```yaml
runtime_certification:
  description: "Automated test suite for runtime compliance"
  
  test_categories:
    - category: "Session Lifecycle"
      tests: 8
      mandatory: true
      
    - category: "Event Bus"
      tests: 6
      mandatory: true
      
    - category: "State Machine"
      tests: 10
      mandatory: true
      
    - category: "Capability Resolution"
      tests: 6
      mandatory: true
      
    - category: "Memory Management"
      tests: 6
      mandatory: true
      
    - category: "Quality Gates"
      tests: 4
      mandatory: true
      
    - category: "Tool Interface"
      tests: 6
      mandatory: true
      
    - category: "Dashboard (optional)"
      tests: 4
      mandatory: false
      
    - category: "Hot Reload (optional)"
      tests: 4
      mandatory: false
      
  passing_score: "100% on mandatory tests"
  certification_validity: "12 months"
  recertification: "On major runtime or contract changes"
```

---

### 17.9 Multi Runtime Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `RuntimeDetected` | Runtime auto-detection | runtime_type, version, capabilities[] | Kernel, Config |
| `RuntimeAdapterLoaded` | Adapter initialization | adapter_name, interfaces_implemented[] | Dashboard |
| `RuntimeCapabilityChecked` | Capability query | feature, supported, limitation | Feature Flag Engine |

---

### 17.10 Multi Runtime Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `runtime.detected` | Counter | runtime_type | Runtime detection count |
| `runtime.adapter.loaded` | Counter | adapter_name | Adapter loaded count |
| `runtime.compliance.score` | Gauge | runtime_type | Compliance score |
| `runtime.certification.passed` | Counter | runtime_type | Certifications passed |
| `runtime.certification.failed` | Counter | runtime_type | Certifications failed |


