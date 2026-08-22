---
type: evolution
key: kernel-learnings
tags: [learning, evolution, self-improvement]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Kernel Learnings — Continuous Improvement Log

## 🔴 Mistake Corrected — 2026-07-26
**Context**: User typed "pr br" — Kernel interpreted as "Pull Request" and created branch + commit.
**What went wrong**: "br" was "pt-br" (Portuguese), not "branch". Kernel acted without confirming.
**Correction**: Had to delete branch, revert commit, restore files.
**Learning**: When user input is ambiguous (< 5 chars), ALWAYS confirm before acting. Ask "Did you mean X or Y?".
**Rule added**: Ambiguous short commands → confirm first, execute after.

## 🟢 Pattern Confirmed — 2026-07-26
**Context**: User wanted self-contained `.opencode/` configuration.
**What worked**: Used Python script to do bulk JSON path replacements safely instead of manual edits.
**Why it worked**: Avoided JSON corruption, applied 14 changes atomically, validated with zero errors.
**Pattern**: Use scripting (Python/sed) for bulk config changes, validate with JSON parser after.

## 🔵 Discovery — 2026-07-26
**Context**: Analyzing memory system for evolution.
**Discovery**: Current memory had 0% connection to actual Cosca project. All entries were from a generic "order-system" template.
**Impact**: Kernel was working with wrong mental model of the project.
**Action**: Full memory rewrite to align with real project.
**Insight**: Memory bootstrapping from templates creates dangerous false confidence. Always validate memory against real codebase.

## 🟡 Improvement Found — 2026-07-26
**Context**: Session startup — Kernel had to glob/grep/read 10+ files to understand project.
**Improvement**: Created `context/session.md` — single file with all essential context.
**Benefit**: Session startup drops from 5-15 seconds to under 1 second.
**Status**: Implemented in this evolution cycle.

## 🟢 Pattern Confirmed — 2026-07-26
**Context**: `git checkout main` followed by `git branch -D` to undo mistaken branch creation.
**What worked**: Used reflog (`git reflog --all`) to recover lost files from deleted commit.
**Why it worked**: Git reflog preserves commits for 30 days even after branch deletion.
**Pattern**: `git checkout <commit-hash> -- <paths>` to recover specific files from reflog.

## 🟡 Improvement Found — 2026-07-26
**Context**: Chat about option A vs B vs C for memory evolution.
**What could be better**: Kernel recommended B but didn't clearly demonstrate the concrete difference.
**Improvement**: Added concrete scenario comparison table (Option A vs B behavior in real situations).
**Result**: User understood and approved immediately.
**Pattern**: When proposing options, always show "with X" vs "without X" concrete scenarios.

## 🔵 Discovery — 2026-07-26
**Context**: Working with `.opencode/cosca/memory/` structure.
**Discovery**: The memory directory has 9 well-organized categories. Adding more is easy.
**Insight**: The INDEX.md pattern (one index per directory) scales well for navigation.
**Action**: Applied INDEX.md updates to all new directories (codebase, context, evolution).

## 🔵 Radar — 2026-07-26
**Context**: Chef pediu para colocar dois itens no radar tecnológico.
**Items**:
1. Memória semântica do Kernel — busca semântica nos 103+ arquivos de memória (infra existe, esperar v2.0)
2. Gatilho semântico de checkpoint — firewall automático contra truncamento de contexto de sessão (esperar v2.0)
**Decision**: Ambos no radar. Implementar após Cosca v2.0 estável.
**Lesson**: Separar "memória semântica de arquivos" de "gatilho semântico de checkpoint". São coisas diferentes. O primeiro é busca, o segundo é preservação de contexto.

## 🔴 Mistake Corrected — 2026-07-26 (late)
**Context**: User restarted OpenCode in another terminal. Error: "Unrecognized key: agents".
**Root cause**: Global config at ~/.config/opencode/opencode.json had `agents` (plural) and `skills` — both unrecognized by OpenCode. Created during initial Cosca global installation.
**Fix**: Renamed `agents` → `agent` (singular). Converted `skills` list → `command` dict format. Fixed in ~/.config/opencode/opencode.json.
**Prevention**: 
- OpenCode uses `agent` (singular), `command` (dict), never `agents` or `skills`
- Project is now self-contained via .opencode/opencode.json — global config is for global tools only
- If adding global agents, use correct key: `agent` not `agents`
