---
name: documentation-update
description: Use when the user asks to update project documentation to match the current codebase and conventions.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Documentation Chief | **Last Updated**: 2026-07-23
> 
> # DOCUMENTATION UPDATE SKILL
> 
> ## Description
> Use this skill to update project documentation after code changes. Updates README, API docs, changelog, ADRs, and architecture docs to reflect changes made.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | change_description | Yes | Description of what changed |
> | changed_files | Yes | List of files that were modified |
> | change_type | Yes | `feature`, `bugfix`, `refactor`, `architecture`, `docs`, `config` |
> | doc_types | No | `readme`, `api`, `changelog`, `adr`, `architecture` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Updated documentation | All affected docs updated |
> | Documentation diff | What changed and why |
> | Review checklist | Documentation quality check |
> 
> ## Process
> 1. Identify all documentation that references changed code
> 2. Determine required updates per change type
> 3. Update README if project structure or setup changed
> 4. Update API docs for endpoint/contract changes
> 5. Update CHANGELOG with change entry
> 6. Create ADR if architecture decision was made
> 7. Update architecture docs for structural changes
> 8. Verify internal links remain valid
> 9. Run documentation quality checks
> 
> ## Documentation Quality Gates
> - [ ] API docs updated for all endpoint changes
> - [ ] ADR created for architecture decisions
> - [ ] README updated if setup/usage changed
> - [ ] CHANGELOG entry added
> - [ ] Code comments explain "why", not "what"
> - [ ] No broken internal links
> - [ ] No TODOs without issue reference
> 
> ## Related
> - [Documentation Chief](../../departments/documentation/SKILL.md)
> - [ADR Creation](./ADR_CREATION.md)
> - [API Documentation](./API_DOCUMENTATION.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.6 Documentation
