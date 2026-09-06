---
name: project-scanning
description: Use when the user asks to scan or discover a workspace, detecting stack, framework, language, and architecture.
---

# Project Scanning

> **Version**: 1.0.0 | **Status**: active | **Owner**: Discovery Chief | **Last Updated**: 2026-07-27

## Purpose
Detect technology stack, framework, database, and architecture from a workspace directory.

## Process
1. Scan root directory for config files: go.mod (Go), package.json (Node), requirements.txt (Python).
2. Detect framework: Next.js, React, Vue from package.json dependencies.
3. Detect database: SQLite (modernc.org/sqlite), PostgreSQL (lib/pq), MySQL from go.mod.
4. Analyze directory structure to infer architecture: cmd/internal/pkg (Go standard), src/features (React).
5. Classify project type: CLI, REST API, Web App, Mobile, Library, Monorepo.
6. Generate project context report with confidence scores per detection.
7. Feed results to cosca-bootstrap for agent activation decisions.

## Success Criteria
- Language detected with > 99% accuracy
- Framework detected with > 95% accuracy
- Scan completes < 5 seconds for projects < 1000 files
