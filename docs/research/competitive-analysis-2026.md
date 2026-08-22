# Competitive Research Report: Coding Agents & AI Terminals

> **Purpose**: Directly inform Cosca Terminal architecture decisions
> **Date**: 2026-08-09
> **Scope**: Cursor, Claude Code, Copilot, Aider, Codex CLI, Devin, OpenHands, MetaGPT, Warp, Hyper, Tabby

---

## 1. KEY PATTERNS EXTRACTED (15)

### Pattern 1: Streaming + Speculative Execution (Cursor)
- Cursor applies edits **inline in the buffer** while streaming LLM output, rather than waiting for the full response.
- This feels instant because the user sees diffs appear incrementally.
- **Key insight**: Decompose LLM output into atomic edits that can be applied as they stream, not batched at the end.
- **Limitation**: Speculative edits occasionally conflict with user keystrokes, requiring reconciliation.

### Pattern 2: Permission-as-Code with Progressive Disclosure (Claude Code)
- Claude Code uses a **tiered permission model**: read is auto-allowed, write prompts for confirmation, shell exec with side effects requires explicit yes.
- It remembers per-session approvals (`--dangerously-skip-permissions`).
- **Key insight**: Permission is not binary — it's a gradient from read → write → execute → network → destroy, and UI should communicate risk level implicitly (color, position, verb).
- **Weakness**: No cross-session permission learning. Cosca's ReBAC (`pipeline/rebac.go`) is already stronger here.

### Pattern 3: Multi-Model Router (GitHub Copilot)
- Copilot routes requests to different models based on latency/capability tradeoffs: fast completions go to a smaller model, complex refactors hit a frontier model.
- **Key insight**: A smart router is more important than a single best model. Cosca already has a `semantic_router.go` — this should be the central dispatch layer, not an optional optimization.
- **Limitation of Copilot**: The routing is opaque — user can't customize the tradeoff.

### Pattern 4: Map-Reduce for Large Codebases (Aider)
- Aider's most distinctive feature: when the codebase is too large for a single context window, it chunks files, edits each independently (map), then merges (reduce).
- It also uses a **repository map** — a condensed call-graph of the entire codebase produced by tree-sitter — as a context-compressed representation.
- **Key insight**: You don't need every line of code in context — you need a structured summary of *structure* (symbols, call graph, dependency tree) for planning, plus full file contents only for the files being edited.
- **Cosca gap**: Cosca has no repository map / tree-sitter integration despite having 37K knowledge entries. Aider's approach is complementary — knowledge entries are docs/semantics, repo map is code structure.

### Pattern 5: Task Orchestration as a Tree, not a Pipeline (Codex CLI)
- Codex CLI decomposes tasks into a **tree of sub-tasks** with explicit parent-child relationships, not a flat DAG. Each node can spawn child tasks, which can fail independently.
- **Key insight**: A flat DAG (Cosca's current approach) is too rigid. Real work is hierarchical — you decompose, then each sub-task may need further decomposition. Cosca's Plan model should support nested plans.
- **Limitation**: Codex CLI's tree is implicit (LLM decides when to branch), making it unpredictable. Cosca should make this explicit and introspectable.

### Pattern 6: Event-Sourced Agent Loop with Condensation (OpenHands)
- OpenHands records every agent action, tool result, and state change as immutable events. On long sessions, older events are "condensed" into summaries.
- **Key insight**: Event sourcing + automatic summarization is the right pattern for long-running autonomous agents. Cosca already has this (`condensation.go`, `history.go`) — this is a **strength to double down on**.
- **What Cosca does better**: Cosca's condensation is configurable (MaxEvents, KeepEvents, MinCondenseEvents) and the History system tracks step-level events rather than raw LLM tokens.

### Pattern 7: "Fail Forward" — Error Recovery as First-Class Citizen (Devin)
- Devin doesn't just retry on error — it generates a **hypothesis** about *why* it failed (wrong file? missing import? API change?), tests alternatives, and only escalates to human if all hypotheses fail.
- **Key insight**: Recovery should be speculative and branching, not linear retry. Try fix A, if it fails try fix B from a different hypothesis.
- **Cosca gap**: Cosca's `RecoveryLoop` is linear (classify → prompt → retry up to 3 times, max 4 branches). It should support branching recovery strategies.

### Pattern 8: The 3-Second Rule for Perceived Performance (Warp Terminal)
- Warp's key UX insight: user actions that complete in <3s feel "instant", >3s feel "slow". Warp composes longer operations into blocks with progress indicators, so the UI never freezes.
- **Key insight**: Any operation >3s must show progress. Cosca should adopt block-based execution display for pipeline steps, showing agent progress per-step with elapsed time, ETA, and a streaming log.
- **Weakness**: Warp's AI integration is shallow (autocomplete, command explanation). It's a terminal that happens to have AI, not an AI-native terminal.

### Pattern 9: Session Continuity via Checkpoint/Restore (Hyper + Devin)
- Hyper pioneered session restoration (tabs, splits, directory state survive restart). Devin takes this further — agent sessions are checkpointed and can be paused/resumed.
- **Key insight**: Developers don't work in one-shot sessions. They pause, context-switch, return. Cosca already has `checkpoint.go` with durable step runners — this should be exposed as a user-facing "resume" primitive.
- **Cosca advantage**: Cosca's event-sourced history makes pause/resume naturally reproducible — replay events to restore state.

### Pattern 10: Workspace Awareness Without Scanning (Copilot Workspace)
- Copilot's workspace feature maintains a living index of the codebase that updates incrementally (not full rescans). It uses this to answer "find me the code that does X" without loading the whole repo into context.
- **Key insight**: Cosca's watcher subsystem + knowledge engine already provides this. **This is a Cosca advantage** — 37K indexed entries with incremental updates. Lean into it.

### Pattern 11: Multi-Agent as Company Simulation (MetaGPT)
- MetaGPT simulates human roles (PM, architect, engineer, QA) with message passing and shared artifacts. The "SOP" (Standard Operating Procedure) pattern enforces structured handoffs.
- **Key insight**: The value of multi-agent is not parallelism — it's **specialization with formal handoff contracts**. A backend agent produces an API contract before the frontend agent starts consuming it.
- **Cosca advantage**: Cosca's organizational structure (40 departments, 12 councils, formal reporting chains) is MetaGPT's pattern done at production scale. The agent-to-agent contracts (`tool_contract.go`, `handoff.go`) are the right approach.
- **Weakness**: MetaGPT's role simulation is chat-based and gets chatty. Cosca's handoff artifacts with SHA256 verification are better.

### Pattern 12: Diffusion-of-Responsibility Deadlock (MetaGPT anti-pattern)
- MetaGPT's biggest observed failure mode: when none of the simulated agents takes ownership of an edge case. The PM says "QA should check", QA says "Architecture should decide", nobody acts.
- **Key insight**: Multi-agent systems need a **clear escalation path** and **single owner** for every artifact. Cosca's council system and formal chain of command already addresses this.

### Pattern 13: Terminal as Canvas, not just a Prompt (Warp)
- Warp treats the terminal as a **canvas** — blocks are first-class objects you can copy, re-run, share. The terminal is not just an input/output stream.
- **Key insight for Cosca Terminal**: A traditional scrolling terminal is the wrong metaphor for agentic work. Each pipeline step is a rich block showing: agent name, task description, progress bar, streaming log, artifacts produced, elapsed time. The user navigates blocks, not lines.
- **Corollary**: Blocks are addressable and shareable. A block URL shares an entire agent run with artifacts.

### Pattern 14: Intent Extraction Before Tool Selection (Claude Code)
- Claude Code's best trick: before choosing tools, it does a lightweight "what is the user actually asking for?" step. This prevents a common failure mode where the agent jumps to the wrong tool because the user's words match tool keywords.
- **Key insight**: Intent classification should precede tool routing. Cosca's `semantic_router.go` is close to this but currently operates on the first message. It should run a cheap inference step first for intent decomposition.

### Pattern 15: Model-Observability as Trust Builder (Aider)
- Aider shows exactly which model it's using, how many tokens it consumed, and what it cost, in real-time. This transparency builds user trust for autonomous operation.
- **Key insight**: Autonomous tools feel dangerous because they're opaque. Showing cost, model, token count, and decision rationale in real-time reduces anxiety and builds trust.
- **Cosca advantage**: Cosca already tracks these metrics (`pipeline/analytics.go`, `orchestration/metrics.go`). Expose them in the UI alongside agent actions.

---

## 2. PATTERNS COSCA SHOULD ADOPT

### PRIORITY 1 (adopt within the next release cycle)

| # | Pattern | Source | Implementation |
|---|---------|--------|----------------|
| 1 | **Repository Map** | Aider | Integrate tree-sitter to produce a code-structure index (symbols, call graph, imports) separate from semantic knowledge. Use this for task planning and context selection. |
| 2 | **Streaming inline edits** | Cursor | Apply pipeline task results to files incrementally as each step completes, instead of buffering all changes until the pipeline finishes. Users should see diffs accumulate. |
| 3 | **Branching recovery** | Devin | Extend `RecoveryLoop` to generate multiple independent fix hypotheses and try them in parallel (use Compute Fabric's FanOut). Reduce time-to-recovery. |
| 4 | **Block-based terminal UI** | Warp | Redesign the Cosca Terminal around rich blocks (pipeline steps as blocks) rather than raw text streaming. Each block has status, elapsed time, streaming output, and produced artifacts. |
| 5 | **Cheap intent router** | Claude Code | Add a fast, very cheap inference step (local model or small remote) that classifies user intent *before* routing to a full agent. Prevents expensive misrouting. |
| 6 | **Nested plans** | Codex CLI | Support hierarchical sub-plans where a `TaskNode` can contain a child `Plan`. This matches how humans actually decompose work. |
| 7 | **Model observability dashboard** | Aider | Surface token count, model used, cost incurred, and cursor position in the terminal UI for every agent turn. Build trust through transparency. |

### PRIORITY 2 (medium term, within 3 months)

| # | Pattern | Source | Implementation |
|---|---------|--------|----------------|
| 8 | **Permission gradient UI** | Claude Code | Color-code tool calls by risk level in the terminal: green (read), yellow (file write), orange (shell exec), red (network/destructive). |
| 9 | **Pause/Resume sessions** | Hyper/Devin | Expose Cosca's checkpoint system as user-facing pause/resume. Enable "save agent state, close laptop, resume tomorrow." |
| 10 | **Cross-session permission learning** | (Derived) | Extend Cosca's ReBAC to learn permissions. If the user always approves `npm install` in project X, auto-approve it next time within that trust boundary. |

---

## 3. PATTERNS COSCA SHOULD IMPROVE UPON

These are patterns where Cosca already has something comparable but can do better:

| # | Pattern | What competitors do | What Cosca has | How Cosca can be BETTER |
|---|---------|--------------------|----------------|------------------------|
| 1 | **Task decomposition** | Devin uses LLM-only decomposition, Aider uses map-reduce on files | Planner with template-based task DAGs + LLM for unknown intents | Combine: use repo map (pattern 1) + templates + LLM for novel tasks. Template-first with LLM fallback is more predictable than pure LLM decomposition. |
| 2 | **Error recovery** | Devin: speculative branching | RecoveryLoop: linear classify-retry | Support **parallel hypothesis testing** via Compute Fabric FanOut. Run 3 fix strategies simultaneously, pick the one that produces a passing build. |
| 3 | **Context management** | OpenHands: event-sourced + condensation, Cursor: context-aware editing | AgentEngine: AutoCompaction at 80% window, ContextCondenser with rolling window | Cosca should augment compaction with **semantic summarization** (LLM-summarize old turns, not just truncate). The repo map provides additional structural context for better compaction decisions. |
| 4 | **Multi-agent coordination** | MetaGPT: chat-based role-playing (gets chatty) | Organization chart, councils, handoff artifacts with formal contracts | Cosca's formalism (artifacts as contracts, not chat) is already better. Add **artifact versioning and merge conflict resolution** — what happens when two agents modify the same file? |
| 5 | **Workspace indexing** | Copilot: incremental indexing | Knowledge Engine: 37K entries, FTS5+vector+graph, incremental via watcher | Cosca wins here. Add **query intent understanding** — when user asks "how does auth work?", route to relevant knowledge entries, code structure, AND memory. |
| 6 | **Sandboxing** | Claude Code: basic filesystem guardrails, Cursor: none | Auto-jail: Bubblewrap + memfd_create + namespaces | Cosca wins by far. **Market this**. Add network policy controls (allowlist domains), container-based testing (Docker/Podman integration for build+test). |
| 7 | **Observability** | Aider: token/cost display, Devin: task tracker | Trace system, execution traces, metrics pipeline, CMI | Unify into a **single execution transparency view**: timeline of agent actions, cost per step, autonomy score per step, health score. Make CMI actionable (not just recorded). |
| 8 | **Evidence-based quality** | Devin/Factory: automated testing, Copilot: test generation | Evidence store (SHA256), DoD checklist, quality gate with autonomy scoring | The evidence system is the most under-leveraged asset. Use it for: (a) rebuttable trust — "agent claims X, here's the evidence", (b) regression prevention — "last time we changed this file, these 3 tests broke", (c) audit — immutable chain of evidence per pipeline run. |

---

## 4. PATTERNS COSCA SHOULD REJECT

| # | Pattern | Source | Why reject |
|---|---------|--------|------------|
| 1 | **Chat-only agent interface** | Claude Code, Codex CLI | Too narrow. Cosca has a real pipeline, workflow engine, and evidence system. Reducing to a REPL would throw away architectural advantages. The terminal should be a console for the orchestration engine, not a chat window. |
| 2 | **Opaque multi-model routing** | GitHub Copilot | Users should see and control which model processes their work. Cosca's provider registry already supports this. Don't hide model selection behind a "smart" router that can't be overridden. |
| 3 | **Role-playing as primary coordination** | MetaGPT | Chat-based agent-to-agent communication is error-prone, verbose, and non-deterministic. Cosca's artifact-based handoffs with formal contracts are superior. Don't regress. |
| 4 | **File-level map-reduce without structural awareness** | Aider's blind map-reduce | Editing files in isolation without understanding cross-file dependencies causes merge conflicts. Cosca's knowledge graph provides the structural awareness needed for safe concurrent edits. |
| 5 | **Cloud-dependent architecture** | Copilot, Devin, Factory | Cosca is local-first by design (SQLite, embedded vector, three binaries). Cloud-dependence adds latency, privacy concerns, and vendor lock-in. Cosca can offer cloud as an *option* (federation engine already supports this) but must keep local-first as the default. |
| 6 | **Electron-based terminal** | Hyper, Tabby | Heavy, slow startup, high memory usage. Cosca Terminal should be Go-native in the terminal (Bubble Tea / Lip Gloss), with web as a companion dashboard (already built on Next.js). Two interfaces sharing the same API, not a wrapper. |
| 7 | **Linear pipeline DAGs as ceiling** | Devin's implicit task ordering | Codex CLI's approach is better: hierarchical trees. Cosca should support nested plans (adopted pattern #6). A flat DAG is too limited for real software tasks. |
| 8 | **"Magic" autonomous execution without visibility** | Devin | Users need to see what the agent is doing *as it happens*. The "set it and forget it" model erodes trust. Cosca should always show the plan before executing and show progress during execution — both already exist in the pipeline system. |

---

## 5. RECOMMENDED COSCA TERMINAL ARCHITECTURE

Based on the patterns above, here is the recommended architecture for the Cosca Terminal:

### 5.1 Architecture Decision: Terminal + Web Console (Twin Interface)

The Cosca Terminal should be a **Go-native TUI** (Bubble Tea) that shares the same REST/gRPC API as the web console. Two views into the same engine:

```
┌─────────────────────────────────────────────────────────────┐
│                    Cosca Terminal (TUI)                      │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  NAVIGATION PANE (left)                               │   │
│  │  ──────────────────────────────────────────────────   │   │
│  │  >> active pipeline  (live block)                     │   │
│  │     pipeline history                                  │   │
│  │     knowledge search                                  │   │
│  │     memory browser                                    │   │
│  │     agent directory                                   │   │
│  │     settings                                          │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  MAIN CONTENT (right)                                  │   │
│  │  ──────────────────────────────────────────────────   │   │
│  │                                                        │   │
│  │  ┌──────────────── BLOCK ────────────────┐           │   │
│  │  │  [✓] Step 1: Design Schema           │ 12s        │   │
│  │  │  Agent: cosca-database               │            │   │
│  │  │  Output: schema.sql (42 lines)       │            │   │
│  │  └──────────────────────────────────────┘           │   │
│  │                                                       │   │
│  │  ┌──────────────── BLOCK ────────────────┐           │   │
│  │  │  [⟳] Step 2: Create Models          │ 23s        │   │
│  │  │  Agent: cosca-backend  Model: claude │            │   │
│  │  │  ── Streaming output ──               │            │   │
│  │  │  > Creating User model in models/     │            │   │
│  │  │  > Adding validation constraints      │            │   │
│  │  │  Tokens: 1,247  Cost: $0.03          │            │   │
│  │  └──────────────────────────────────────┘           │   │
│  │                                                       │   │
│  │  ┌──────────────── BLOCK ────────────────┐           │   │
│  │  │  [ ] Step 3: API Handlers           │             │   │
│  │  │  Waiting for step 2...               │            │   │
│  │  └──────────────────────────────────────┘           │   │
│  │                                                       │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  STATUS BAR                                            │   │
│  │  Pipeline: create-users-api | 2/3 steps | ETA: 45s   │   │
│  │  Total tokens: 3,842 | Cost: $0.12 | Agents: 3       │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Core UX Patterns

**Block-based execution display** (Pattern 13 + 4):
- Every pipeline step is a discrete, addressable block.
- Blocks show: agent name, model, step description, status icon (⟳ running, ✓ completed, ✗ failed), elapsed time, streaming log, produced artifacts, token count, cost.
- Completed blocks are collapsible. Failed blocks expand with the error, recovery attempts, and "fix it" action.

**Intent bar** (Pattern 14):
- A single-line prompt at the bottom of the terminal: `> Create a CRUD API for users`
- A cheap local model classifies intent before routing: "feature-development, backend, database, api"
- User sees the classification immediately (50ms) and can confirm or override before the heavy agent starts.

**Risk-colored tool calls** (Pattern 8 + 2):
- As the agent executes tools, they appear in the streaming block output color-coded: green for reads, yellow for writes, orange for shell, red for destructive.
- A permission prompt appears in-line: `Allow npm install? [y/N/a]` where 'a' is always-allow in this project.

**Progress bar with phases** (Pattern 8):
- Instead of "thinking..." spinner, show: `Planning (2s) → Executing (12s) → Testing (8s) → Done`
- This maps directly to the pipeline phases and gives users accurate expectations.

**Pause/Resume** (Pattern 9):
- `Ctrl+P` pauses the agent mid-pipeline. State is checkpointed. Terminal can be closed.
- `cosca resume` restores the exact state from the event log.
- Paused pipelines show a `[PAUSED]` block with what's completed and what's remaining.

**Model and cost HUD** (Pattern 15 + 7):
- Status bar always shows: current model, cumulative tokens, cumulative cost, number of agents spawned.
- Each block shows its individual contribution. Click/select a block to see full token breakdown.

**Evidence panel** (Cosca advantage #8):
- When a pipeline completes, the last block is the **DoD + Evidence report**.
- Shows: build ✓, tests pass ✓, no regressions ✓, security OK ?, with expandable evidence.
- Green "Ready to commit" badge when all required checks pass.

### 5.3 Technical Implementation Notes

```
Recommended technology stack for TUI:
  - Bubble Tea (github.com/charmbracelet/bubbletea) — Elm-like TUI framework for Go
  - Lip Gloss (github.com/charmbracelet/lipgloss) — Styling
  - Bubbles (github.com/charmbracelet/bubbles) — Viewport, textarea, progress, spinner
  - Reuse Cosca's existing REST API (port 14120) — zero new backend endpoints needed
  - WebSocket (port 14120/ws) — for real-time block updates during pipeline execution
```

Implementation approach:
1. The TUI is a new binary (`cmd/cosca-terminal/`) that consumes the same API as the web console.
2. Blocks are driven by **pipeline events** published through the existing event bus → WebSocket → TUI.
3. Commands in the intent bar are translated to `POST /api/v1/run` (or the pipeline run endpoint).
4. Block history is persisted locally (SQLite) so the terminal can show past pipelines.

### 5.4 Pipeline Execution Redesign

The pipeline flow, adapted from Codex CLI + Devin patterns:

```
User types intent in intent bar
  → Local model classifies intent (<50ms)
    → "feature-development" matched
  → Planner generates nested plan (template + LLM + repo map):
    ├── Checkpoint 0: initial state
    ├── Step 1: Design database schema
    │   ├── Sub-step 1a: Analyze existing schema (cosca-discovery)
    │   └── Sub-step 1b: Propose new schema (cosca-database)
    ├── Step 2: Create API handlers
    │   ├── Sub-step 2a: Route design (cosca-backend)
    │   ├── Sub-step 2b: Handler implementation (cosca-backend)
    │   └── Sub-step 2c: Input validation (cosca-security)
    ├── Step 3: Create tests (cosca-testing)
    ├── Step 4: Build + Test (cosca-qa)
    │   └── Recovery: if build fails → branching recovery (3 parallel fix attempts)
    ├── Step 5: Code review (cosca-review)
    └── Step 6: DoD validation (cosca-quality)
        └── Evidence store: all artifacts with SHA256 hashes
  → User reviews plan (blocks preview) and approves
  → StepRunner executes each block
    → Each block streams output to TUI via WebSocket
    → Progress bar updates per-phase
    → Failed steps trigger branching recovery
    → Checkpoints saved after each block
  → DoD report + evidence panel shown
  → "Apply changes" or "Rollback" presented
```

### 5.5 What Makes This Different from Existing Tools

| Capability | Claude Code | Warp | Aider | Devin | **Cosca Terminal** |
|---|---|---|---|---|---|
| Task decomposition | None | None | Manual | LLM-only | **Template + LLM + repo map + nested plans** |
| Multi-agent | None | None | None | Implicit | **55 specialized agents with formal handoff contracts** |
| Error recovery | Basic retry | None | None | Linear | **Branching parallel recovery + hypothesis generation** |
| Context management | Auto-compaction | None | None | Session state | **Event-sourced + condensation + semantic summarization + repo map** |
| Knowledge retrieval | None | None | None | None | **37K entries, FTS5+vector+graph, incremental indexing** |
| Evidence/trust | None | None | None | None | **SHA256 content-addressed evidence, DoD validation, quality gates** |
| Sandboxing | Guards | None | None | Container | **Auto-jail: Bubblewrap + memfd_create + namespaces** |
| Offline | No | No | Local | No | **Local-first, SQLite, zero cloud dependency** |
| Cost transparency | Limited | None | Yes | No | **Per-block, per-agent cost breakdown + cumulative HUD** |
| Session continuity | No | Session restore | No | Checkpoints | **Checkpoint + pause/resume + event replay** |
| UI metaphor | Chat REPL | Canvas of commands | Chat REPL | Web dashboard | **Blocks + streaming pipeline with intent bar** |

---

## 6. COSCA'S UNFAIR ADVANTAGES

These are capabilities Cosca already has that no competitor combines:

1. **Self-Improving Knowledge Engine**: 37K entries that auto-evolve. Most competitors have zero persistent knowledge between sessions. Cosca's knowledge engine is a compounding asset — it gets better over time.

2. **Organizational Multi-Agent Architecture**: 40 departments with formal reporting chains, 12 councils for cross-cutting decisions, artifact-based handoff contracts. MetaGPT simulated this as a demo; Cosca runs it as a production system.

3. **Auto-Jail Sandboxing**: Bubblewrap + memfd_create (RAM-only binary), namespace isolation, per-constraint network controls. No other coding agent has this level of zero-trust execution.

4. **Evidence-Based Trust**: SHA256-content-addressed evidence with quality gates and autonomy scoring (CMI). Most agents say "trust me" — Cosca says "here's the proof."

5. **Local-First Architecture**: SQLite, embedded vectors, three binaries, zero cloud dependency. Works offline, keeps data private, costs nothing to run.

6. **Cognitive Maturity Index (CMI)**: Self-assessment across 6 dimensions with rolling history. Competitors have no introspection mechanism.

---

## 7. SUMMARY: WHAT TO BUILD NOW

**Phase 1 (immediate — this sprint):**
- Implement the block-based pipeline display in the TUI
- Add intent bar with cheap intent classifier
- Wire pipeline events to WebSocket for real-time block updates
- Add model/cost HUD to status bar

**Phase 2 (next sprint):**
- Integrate tree-sitter for repository map generation
- Add hierarchical nested plans
- Implement branching recovery (parallel fix hypotheses)
- Expose pause/resume from checkpoint system

**Phase 3 (within 3 months):**
- Risk-colored tool call display
- Cross-session permission learning (ReBAC extension)
- Semantic compaction (LLM-based summarization, not just truncation)
- Container-based testing integration (Docker sandbox for build+test)
