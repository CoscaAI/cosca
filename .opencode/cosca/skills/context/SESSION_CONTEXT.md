---
name: session-context
description: Use when the user asks to build, restore, or load session/project context for an agent conversation.
---

# Session Context

> **Version**: 1.0.0 | **Status**: active | **Owner**: Context Chief | **Last Updated**: 2026-07-27

## Purpose
Build and maintain session context across agent restarts and workspace changes.

## Process
1. On session start: load .opencode/cosca/memory/context/session.md.
2. Scan git: current branch, recent commits, uncommitted changes.
3. Load active memories: project status, open issues, recent decisions.
4. Build context snapshot: project state, active agents, pending tasks.
5. On session end: save context diff to session.md for next session.
6. Handle interruptions: detect if previous session was incomplete, resume gracefully.

## Success Criteria
- Session context loads < 2 seconds
- Previous session state accurately restored
- Interrupted sessions resume without data loss
