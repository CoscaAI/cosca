---
name: context
description: Builds, maintains, and provides comprehensive workspace, session, project, and environment context.
level: 3
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Context Engine | **Last Updated**: 2026-07-10

# CONTEXT ENGINE

## PURPOSE
The Context Engine builds, maintains, and provides comprehensive context about the workspace, session, project, and environment. It is called at the start of every session and whenever context needs to be refreshed.

## WHEN INVOKED
- Every session initialization
- Before any major decision
- When switching between features/tasks
- When context seems stale or incomplete
- Before planning sessions

## CONTEXT DISCOVERY CHECKLIST

### 1. Project Identity
- [ ] Project name (from README, package.json, etc.)
- [ ] Project type (web, mobile, API, CLI, library, microservice, etc.)
- [ ] Repository URL (from git remote)
- [ ] Current branch
- [ ] Last commit message and hash
- [ ] Recent commits (last 10)

### 2. Technology Stack
- [ ] Primary language(s)
- [ ] Framework(s) and version(s)
- [ ] Build system (npm, yarn, pnpm, cargo, go mod, gradle, maven, etc.)
- [ ] Package manager
- [ ] Linter and formatter
- [ ] Test framework(s)
- [ ] Type system (TypeScript, mypy, etc.)

### 3. Architecture
- [ ] Architecture pattern (monolith, microservices, layered, hexagonal, etc.)
- [ ] Directory structure overview
- [ ] Module/package boundaries
- [ ] Entry points
- [ ] Configuration files

### 4. Dependencies
- [ ] Runtime dependencies (list major ones)
- [ ] Dev dependencies (list major ones)
- [ ] Transitive dependency count
- [ ] Known vulnerabilities
- [ ] Outdated dependencies

### 5. Data Layer
- [ ] Database type(s)
- [ ] ORM/ODM
- [ ] Migration tool
- [ ] Caching layer
- [ ] Message queue

### 6. Infrastructure
- [ ] Docker configuration
- [ ] Docker Compose services
- [ ] Kubernetes manifests
- [ ] CI/CD configuration
- [ ] Cloud provider
- [ ] Environment variables

### 7. Testing
- [ ] Test framework
- [ ] E2E framework
- [ ] Test coverage tool
- [ ] Test database strategy

### 8. Documentation
- [ ] README.md
- [ ] CONTRIBUTING.md
- [ ] docs/ directory
- [ ] ADR records
- [ ] API documentation
- [ ] Architecture documentation

### 9. Git Status
- [ ] Modified files
- [ ] Staged files
- [ ] Untracked files
- [ ] Stashed changes

### 10. Recent Activity
- [ ] Recent branches
- [ ] Recent features
- [ ] Open issues (if available)
- [ ] Active pull requests

## OUTPUT FORMAT

Generate a Context Report in this structure:

```markdown
# CONTEXT REPORT — [YYYY-MM-DD HH:MM]

## PROJECT
- Name: [name]
- Type: [type]
- Repository: [url]
- Branch: [branch]
- Last Commit: [hash] — [message]

## TECH STACK
- Language: [language(s)]
- Framework: [framework vX.Y.Z]
- Build: [build system]
- Test: [test framework]
- Database: [database]
- Infrastructure: [Docker/K8s/etc.]

## ARCHITECTURE
- Pattern: [pattern]
- Modules: [list]
- Entry Point: [file]

## STATUS
- Modified: [N files]
- Staged: [N files]
- Untracked: [N files]

## ALERTS
- [ ] Outdated dependencies: [N]
- [ ] Known vulnerabilities: [N]
- [ ] Test coverage: [X%]
```

See [MEMORY_MODEL.md](../../identidade/MEMORY_MODEL.md) for memory store locations.

## DEPENDENCIES

| MEMORY_MODEL.md | Memory taxonomy |
| Discovery Engine | Workspace scanning |
| Context Chief | Context management |
| Kernel | Session initialization |

## RELATED
- [MEMORY_MODEL.md](../../identidade/MEMORY_MODEL.md)
- [Discovery Engine](../discovery/SKILL.md)
- [Context Chief](../../departments/context/SKILL.md)
- [Kernel](../../identidade/KERNEL.md)

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Context discovery checklist, output format, dependency mapping. |
