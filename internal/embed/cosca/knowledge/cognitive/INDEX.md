# Cognitive Knowledge Domain

> **Category**: Cognitive | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

The cognitive domain captures the AI agent's self-model — how it thinks, feels, and interacts, independent of any specific project, domain, or workspace. This is meta-knowledge about the agent itself.

## Files

| # | File | Description | Type |
|---|------|-------------|------|
| 1 | [`Cognitive_State_Specification.md`](Cognitive_State_Specification.md) | Universal Cognitive State Specification (UCSS) — the formal spec defining the cognitive state model | Specification |
| 2 | [`cognitive-state.md`](cognitive-state.md) | Current cognitive state snapshot — the Kernel's live self-assessment | Runtime State |

## UCSS — Universal Cognitive State Specification

The **[Cognitive State Specification](Cognitive_State_Specification.md)** defines the canonical model for representing an AI agent's internal state. It explicitly excludes domain-specific information (files, repos, branches, builds, tasks) and focuses purely on:

- **Identity** — Persona, role, expertise domains
- **Emotion** — Primary emotion, intensity, stability
- **Energy** — Level, fatigue, cognitive load
- **Confidence** — Overall self-trust, cross-agent validation
- **Meta-cognition** — Self-awareness of knowledge gaps, learning trajectory

## Cognitive State Snapshot

The **[cognitive-state.md](cognitive-state.md)** file is the live instantiation of the UCSS for the Cosca Kernel. It is updated during or after significant tasks and represents the agent's self-assessed state at a point in time.

Key metrics from the current snapshot:
- Persona: Cosca Kernel (Consigliere del Don)
- Primary emotion: focused (intensity: 0.85)
- Energy: 0.82 (fatigue: 0.08)
- Confidence overall: 0.91

## Meta-Learning (Placeholder)

Meta-learning — the agent's ability to reflect on and improve its own cognitive processes — is a planned capability. This section will house:
- Learning trajectory analysis
- Cognitive bias detection
- Self-improvement suggestions
- Cross-session behavioral patterns

## Source of Truth Hierarchy

| Layer | Location | Purpose | Update Frequency |
|-------|----------|---------|:----------------:|
| **Specification** | `knowledge/cognitive/Cognitive_State_Specification.md` | Defines the model — what fields exist, what they mean | Infrequent (breaking changes) |
| **Instanced State** | `knowledge/cognitive/cognitive-state.md` | The Kernel's live self-assessment | Per significant task |
| **Context State** | `memory/context/` | Session-level context (files, repo, task) | Per session |

## Relationship to Memory System

- **`memory/context/`** stores **what** the agent is working on (files, repo, tasks)
- **`knowledge/cognitive/`** stores **how** the agent is thinking (identity, emotion, confidence)
- The UCSS explicitly forbids storing domain information (files, repos, branches) in the cognitive state — those belong to `memory/context/`

---

*The cognitive domain models the agent's internal state. It is the self-awareness layer that enables meta-cognition, confidence tracking, and agent evolution. See [Cognitive_State_Specification.md](Cognitive_State_Specification.md) for the full specification.*
