# CONTEXT COMPRESSION ENGINE — Cognitive State Compression

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28
>
> **Constitutional authority**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implements G2 (Transparência total) and the principle of efficient context management.
> **Approved by**: Don — "Não guardar toda conversa. Guardar estado atual do projeto."

---

## Purpose

Every session starts with context loading. Current verbose format (session.md + current.md) costs ~5k bytes of prose but could explode to 9.7k+ tokens when including conversation history, memory indexes, and framework context.

The Context Compression Engine reduces the "cognitive state" to a compact, structured format that captures only what matters for the next session to be productive immediately — not the entire conversation, but the essential state.

**Target:** 9.7k tokens → **≤ 2,000 tokens** (80% reduction) without losing actionable context.

---

## Compression Philosophy

### What to KEEP (high signal)
- Current project state (git, build, tests, lint)
- Active decisions (last 3-5 that changed direction)
- Pending work (what was in progress when session ended)
- Known risks (what could break)
- Next actions (what to do first)

### What to DISCARD (low signal)
- Conversation history (70% is noise — clarifications, iterations, corrections)
- Completed work details (archived in memory, not needed for next session)
- Exploration paths that were abandoned
- Agent internal reasoning (stays in agent memory)

### The Rule of 5
Each section has a hard limit of 5 items. If there are more than 5 decisions, only the top 5 make it. More than 5 pending items? Top 5. This forces curation at compression time.

---

## Compressed Format: `.cosca/memory/context/cognitive-state.md`

```yaml
# COGNITIVE STATE — Cosca v1.4.0-dev
# Compressed: {timestamp} | Session: {session_key} | Tokens: ~{count}/2000

ARCHITECTURE:
  stack: {lang} {version} + {framework} + {database}
  module: {go_module_path}
  agents: {count} ({breakdown})
  skills: {count}
  engines: {count}
  workflows: {count}
  key_files: [{path1}, {path2}, {path3}]  # max 3, the ones you'll need first

STATE:
  git: {branch}, {ahead/behind}, {clean/dirty}
  build: {pass/fail}
  tests: {pass/fail} ({package_count} pkgs)
  lint: {pass/fail} ({warning_count} warnings)
  memory: {file_count} files, {health} health
  dashboard_cis: {score}/100 ({classification})
  dashboard_success: {rate}%
  dashboard_confidence: {avg}

DECISIONS:  # Last 5, most recent first
  - [{date}] {decision_summary} → {outcome}
  - [{date}] {decision_summary} → {outcome}

PENDING:  # Top 5 by priority
  - [{priority}] {item} ({status}, {blocker})
  - [{priority}] {item} ({status}, {blocker})

RISKS:  # Top 5 by severity
  - [{severity}] {risk_description}
  - [{severity}] {risk_description}

NEXT:  # Exact 3 — what to do when session resumes
  - [{priority}] {action} ({estimated_effort})
  - [{priority}] {action} ({estimated_effort})
  - [{priority}] {action} ({estimated_effort})

RECENT_COMMITS:  # Last 3, for context
  - {short_hash} {message_summary}
  - {short_hash} {message_summary}
  - {short_hash} {message_summary}
```

---

## Compression Rules

### Rule 1 — Aggressive Truncation
```
ARCHITECTURE → facts only, no prose. "Go 1.25+Next.js 15+SQLite" not "The project is built with..."
DECISIONS → one line each. Date + what + outcome. No justification (that's in ADRs).
PENDING → priority + item + status + blocker. No narrative.
RISKS → severity + what. No mitigation (that's in bug registry).
NEXT → exact 3. Priority + action + effort. Executable, not aspirational.
```

### Rule 2 — Implicit Knowledge is NOT Stored
```
❌ "The project uses Go modules" — implicit from stack
✅ Agents now live in .cosca/agents/{name}/PROMPT.md — explicit, not implicit
❌ "Kernel routes tasks" — implicit from framework
✅ Only store what CHANGED or what's UNUSUAL
```

### Rule 3 — Completed Work is Archived, Not Compressed
```
When a task is completed:
  → Details go to memory/{category}/ (permanent)
  → Only the DECISION stays in cognitive state
  → Marked as completed in PENDING, then removed next compression

Example:
  PENDING: [#4 Causalidade] → completed
  DECISIONS: [+2026-07-28 Bug Registry → Causality Tree v3.0]
  PENDING: [removed on next compression]
```

### Rule 4 — Token Budget Enforcement
```
Hard budget: 2,000 tokens
Soft target: 1,500 tokens (leaves 500 for session-specific additions)

If compression exceeds 2,000 tokens:
  1. Reduce DECISIONS from 5 → 3
  2. Reduce PENDING from 5 → 3
  3. Reduce RISKS from 5 → 3
  4. If still exceeds: truncate RECENT_COMMITS to 1
  5. If STILL exceeds: alert — cognitive state too complex, needs human curation
```

### Rule 5 — Freshness Indicator
```
Each entry has an implicit or explicit age:
  ARCHITECTURE: changes rarely → no freshness needed
  STATE: always current → "as of {timestamp}"
  DECISIONS: date-stamped → freshness visible
  PENDING: status indicates staleness
  RISKS: severity may escalate over time → auto-escalate if > 14 days

Stale entries (> 30 days in PENDING without update):
  → Flag with [STALE] prefix
  → Auto-demote priority by 1 level
```

---

## Compression Pipeline

```
COMPRESSION CYCLE (triggered at session end or on demand):

1. COLLECT
   ├── Read current session.md, current.md
   ├── Read last 5 decisions from memory/decisions/
   ├── Read active bugs from memory/bug/INDEX.md
   ├── Read roadmap PENDING items
   └── Read last 3 git commits

2. EXTRACT (signal from noise)
   ├── Architecture facts → from project/overview.md (static)
   ├── State facts → from git, build, test, lint commands
   ├── Decisions → last 5 from memory/decisions/
   ├── Pending → from roadmap + active session
   ├── Risks → from bug registry + confidence alerts
   └── Next → from pending items sorted by priority

3. TRUNCATE (Rule of 5)
   ├── Each section: keep top 5, discard rest
   └── If section has < 5 items: keep all (don't pad)

4. COMPRESS (Rule 1)
   ├── Convert prose → structured YAML-like format
   ├── Remove articles, filler words, implicit knowledge
   └── One line per item maximum

5. VALIDATE
   ├── Count tokens (approximate: chars/4 for English, chars/2 for Portuguese)
   ├── If > 2000: apply budget enforcement (Rule 4)
   ├── Verify all sections present
   └── Check RECENT_COMMITS against git log

6. WRITE
   └── Overwrite .cosca/memory/context/cognitive-state.md

7. REPORT
   └── Log compression ratio: {before_tokens} → {after_tokens} = {ratio}% reduction
```

---

## Example: Current State Compressed

### BEFORE (verbose, ~5,019 bytes / ~1,250 tokens for two files)

See: `context/session.md` (58 lines) + `sessions/active/current.md` (51 lines) = 109 lines of prose.

### AFTER (compressed, ~1,600 bytes / ~400 tokens)

```yaml
# COGNITIVE STATE — Cosca v1.4.0-dev
# Compressed: 2026-07-28T22:00:00Z | Session: evolution-marathon-2026-07-28 | Tokens: ~400/2000

ARCHITECTURE:
  stack: Go 1.25 + Next.js 15 + SQLite (modernc.org)
  module: github.com/CoscaAI/cosca
  agents: 51 (41 chiefs + 9 specialists + 1 kernel)
  skills: 71 (28 categories)
  engines: 32 (incl. Memory Decay, Evidence, Curation)
  workflows: 27 (incl. Metacognition Pipeline)
  key_files: [.cosca/CONSTITUTION.md, .cosca/AGENT_DNA.md, docs/adr/]

STATE:
  git: main, 13 ahead, clean
  build: pass
  tests: pass (50+ pkgs)
  lint: zero
  memory: 405 files, healthy (0 broken, 0 orphans)
  dashboard_cis: 42.80/100 (Em desenvolvimento)
  dashboard_success: 100%
  dashboard_confidence: 0.48

DECISIONS:
  - [2026-07-28] 10 melhorias do Don → roadmap Categoria A/B/C/D
  - [2026-07-28] Bug Registry → Causality Tree v3.0 (4 níveis)
  - [2026-07-28] Memory Curation → Decay Engine (CurationScore v2.0)
  - [2026-07-28] Dashboard v2.0 → Cosca Intelligence Score (5 dims)
  - [2026-07-28] Agent DNA v3.0 → 28 fields (metacognition layer)

PENDING:
  - [P0] #9 Context Compression (in_progress)
  - [P0] Completar 4 itens Categoria A do roadmap
  - [P1] gRPC server (proto done)
  - [P1] Ativar Onda 2 agents (41 sem execução)
  - [P2] Performance benchmarking suite

RISKS:
  - [HIGH] 41/51 agents com seed data (sem execução real)
  - [MEDIUM] Confidence médio 0.48 (meta: 0.70)
  - [MEDIUM] Sem soak test (>24h) no CI
  - [LOW] TypeScript SDK bloqueado por OpenAPI spec
  - [LOW] Helm chart pendente de CI/CD

NEXT:
  - [P0] Finalizar #9 Context Compression (30min)
  - [P0] Commit + apresentar Don (5min)
  - [P1] Verificar se Don quer iniciar Categoria B

RECENT_COMMITS:
  - d90e5ce feat(#6): cosca intelligence score — dashboard v2.0.0
  - 9511fc4 feat(#2): memory decay engine — CurationScore v2.0
  - 183c833 feat(#4): arvore de causalidade — bug registry v3.0
```

**Compression ratio:** ~1,250 tokens → ~400 tokens = **68% reduction** for just these two files. When including conversation history and framework context (9.7k total), the compression is even more dramatic.

---

## When to Compress

| Trigger | Description |
|---------|-------------|
| **Session end** | Automatic — runs as session cleanup |
| **Manual** | `cosca context compress` (Don or Kernel) |
| **Threshold** | When session context exceeds 5,000 tokens |
| **Pre-commit** | Before committing session memory |
| **Weekly** | Part of dashboard generation cycle |

---

## When to DECOMPRESS (Expand)

Compression is lossy by design. When full context is needed:

| Scenario | Recovery Method |
|----------|----------------|
| Need decision rationale | Read ADR from `memory/decisions/` |
| Need bug details | Read bug file from `memory/bug/` |
| Need task history | Read session archive from `memory/sessions/archive/` |
| Need architecture details | Read `memory/architecture/` |
| Need conversation | Read raw session log (if available) |

The cognitive state is the INDEX, not the ARCHIVE. It tells you WHERE to look, not WHAT you'll find.

---

## Integration Points

| System | How it integrates |
|--------|------------------|
| **Bootstrap Engine** | Loads cognitive-state.md first (fast), then expands on demand |
| **Memory Chief** | Compressed state stored as `memory/context/cognitive-state.md` |
| **Curation Engine** | R1 scoring includes compression freshness |
| **Dashboard** | Token savings reported as M2 (Eficiência de Tokens) |
| **Session Cleanup** | Compression runs as final step before session archival |

---

## Token Savings Tracking

| Metric | Before Compression | After Compression | Savings |
|--------|-------------------|-------------------|---------|
| Session context (2 files) | ~1,250 tokens | ~400 tokens | 68% |
| Full session load (est.) | ~9,700 tokens | ~2,000 tokens | 79% |
| Monthly token savings (20 sessions) | 194,000 tokens | 40,000 tokens | 154,000 tokens |

**Cost impact (GPT-4 class):** ~$1.50/session saved → ~$30/month for 20 sessions.

---

> **Related**: [MEMORY_CURATION_ENGINE.md](../memory-curation/MEMORY_CURATION_ENGINE.md) — also reduces memory footprint | [platform-evolution-v1.4.0.md](../../memory/roadmap/platform-evolution-v1.4.0.md) — roadmap item #9 | [session.md](../../memory/context/session.md) — current verbose format being compressed
