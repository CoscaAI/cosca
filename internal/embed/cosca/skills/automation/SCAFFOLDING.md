---
name: scaffolding
description: Use when the user asks to scaffold a new module, service, or component following project templates.
---

# Project Scaffolding

> **Version**: 1.0.0 | **Status**: active | **Owner**: Automation Chief | **Last Updated**: 2026-07-27

## Purpose
Generate project structures from Cosca templates with proper conventions.

## Process
1. Select template from .opencode/cosca/templates/ matching project type.
2. Customize: project name, module path, Go version, features to enable.
3. Generate directory structure following Cosca conventions (cmd/, internal/, pkg/, api/, web/).
4. Initialize: go.mod, Makefile, .gitignore, opencode.json, README.md.
5. Run post-generation hooks: go mod tidy, git init, make build.
6. Verify generated project compiles and tests pass.

## Success Criteria
- Generated project compiles without errors
- All template files present with correct substitutions
- Ready to commit after generation
