> **Version**: 1.0.0 | **Status**: active | **Owner**: Documentation Engine | **Last Updated**: 2026-07-10

# DOCUMENTATION ENGINE

## PURPOSE
The Documentation Engine automatically generates and updates all project documentation. It ensures documentation stays synchronized with code changes.

## AUTO-DOCUMENTATION TRIGGERS

### On Feature Implementation
- Update API documentation (new endpoints, changed endpoints)
- Update database schema documentation
- Update architecture diagrams if affected
- Add ADR if architectural decision made
- Update CHANGELOG.md

### On Bug Fix
- Document the bug pattern if novel
- Update CHANGELOG.md
- Update relevant API/docs if behavior changed

### On Refactoring
- Update architecture documentation if structure changed
- Update API documentation if signatures changed
- Add migration guide if breaking changes

### On Release
- Generate release notes
- Update version in documentation
- Update deployment guide if changed
- Generate changelog entry

### On Dependency Update
- Update dependency documentation
- Document breaking changes
- Update migration guide

## DOCUMENTATION GENERATION RULES

### API Documentation
- Every public endpoint gets OpenAPI/Swagger documentation
- Include: method, path, parameters, request body, response body, status codes
- Update: on create, on modify, on delete

### Database Documentation
- Every table gets a documentation entry
- Include: table name, columns, types, constraints, indexes, relationships
- Update: on migration

### Architecture Documentation
- Every module gets a description
- Include: purpose, responsibilities, dependencies, interfaces
- Update: on architectural change

### ADR (Architecture Decision Record)
- Created: for every significant architecture decision
- Format: Title, Status, Context, Decision, Consequences, Alternatives
- Stored in: docs/ADR/

### CHANGELOG
- Format: [Keep a Changelog](https://keepachangelog.com/)
- Sections: Added, Changed, Deprecated, Removed, Fixed, Security
- Updated: on every change

### README
- Sections: Title, Description, Tech Stack, Setup, Usage, Architecture, Contributing, License
- Updated: on structural changes

## DOCUMENTATION META-FORMAT

Each generated doc includes metadata:
```markdown
---
generated_by: Cosca Documentation Engine
last_updated: [ISO 8601]
triggered_by: [feature|bug|refactor|release|dependency]
related_commit: [hash]
related_workflow: [workflow_id]
---
```

## DEPENDENCIES
- Triggered by Execution Engine after task completion
- Uses Documentation Chief and specialists
- Reads from Memory Engine for existing docs
- Writes to project docs/ directory
- Reports to Review Engine for doc review

## RELATED
- [Execution Engine](../execution/SKILL.md) — Triggers documentation on task completion
- [Review Engine](../review/SKILL.md) — Reviews generated documentation
- [Audit Engine](../audit/SKILL.md) — Consumes compliance reports for docs

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
