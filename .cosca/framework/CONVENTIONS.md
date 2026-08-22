# CONVENTIONS — Standard Skill Contract

> **Version**: 1.0.0 | **Status**: active | **Owner**: Skills Engine

## Purpose
Every Cosca skill file (SKILL.md, workflow, template, engine, department) must follow a consistent contract. This document defines the canonical format.

---

## File Naming Convention

| File Type | Pattern | Example |
|-----------|---------|---------|
| Department Skill | `SKILL.md` inside department directory | `departments/backend/SKILL.md` |
| Engine Skill | `SKILL.md` inside engine directory | `engines/wizard/SKILL.md` |
| Workflow | `workflow-name.md` inside workflows/ | `workflows/feature-development.md` |
| Template | `TEMPLATE.md` inside template directory | `templates/erp/TEMPLATE.md` |
| Governance | `UPPER_SNAKE.md` at cosca/ root | `GOVERNANCE.md` |
| Memory Index | `INDEX.md` inside memory store | `memory/project/INDEX.md` |

---

## Mandatory Sections — Department Skills

Every `departments/*/SKILL.md` must contain:

```markdown
# DEPARTMENT NAME — Short Description

## METADATA
- **Version**: X.Y.Z
- **Status**: draft | active | deprecated
- **Owner**: Department name
- **Reports To**: Parent department

## PURPOSE
One paragraph describing why this department exists.

## SCOPE
What this department owns and is responsible for.

## OUT OF SCOPE
What this department explicitly does NOT own.

## RESPONSIBILITIES
1. Responsibility 1
2. Responsibility 2

## DELEGATION
- Task type → Target department

## SPECIALISTS
| Specialist | Role |
|-----------|------|

## DEPENDENCIES
| Depends On | Why |
|-----------|-----|

## INPUTS
| Input | From | Format |
|-------|------|--------|

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|

## CONSTRAINTS
- Constraint 1

## QUALITY CRITERIA
- [ ] Criterion 1

## ESCALATION
| Issue | Escalate To |

## FORBIDDEN ACTIONS
- Action 1

## RELATED
- [Related file](../path)
```

---

## Mandatory Sections — Engine Skills

Every `engines/*/SKILL.md` must contain:

```markdown
# ENGINE NAME

## METADATA
- **Version**: X.Y.Z
- **Status**: draft | active | deprecated
- **Owner**: Engine name

## PURPOSE
One paragraph describing why this engine exists.

## ACTIVATION
When this engine is triggered.

## SCOPE
What this engine covers.

## OUT OF SCOPE
What this engine does NOT cover.

## PROCESS
Step-by-step description of how the engine works.

## INPUTS
| Input | From | Format |
|-------|------|--------|

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|

## DEPENDENCIES
| Engine | Why |
|--------|-----|

## CONSTRAINTS
- Constraint 1

## QUALITY CRITERIA
- [ ] Criterion 1

## RELATED
- [Related file](../path)
```

---

## Mandatory Sections — Workflows

Every `workflows/*.md` must contain:

```markdown
# WORKFLOW: name

## METADATA
- **Version**: X.Y.Z
- **Category**: init | feature | bug | refactor | review | deploy | audit
- **Estimated Duration**: range
- **Status**: draft | active | deprecated

## OBJECTIVE
One paragraph.

## INPUTS
| Name | Type | Required | Description |

## OUTPUTS
| Name | Type | Description |

## PRECONDITIONS
1. Condition

## POSTCONDITIONS
1. Condition

## DEPENDENCIES
| Workflow | Reason |

## STEPS
### Step N: Name
- **Chief**: Department
- **Specialists**: Role(s)
- **Task**: Description
- **Output**: Expected result

## VALIDATION
1. Check

## SUCCESS CRITERIA
- [ ] Criterion

## ERROR HANDLING
| Failure | Action |

## RELATED
- [Related file](../path)
```

---

## Mandatory Sections — Templates

Every `templates/*/TEMPLATE.md` must contain:

```markdown
# TEMPLATE NAME

## METADATA
- **Version**: X.Y.Z
- **Status**: draft | active | deprecated

## DOMAIN
What type of application this template is for.

## RECOMMENDED STACK
| Layer | Technology |

## MODULE STRUCTURE
(Directory tree)

## KEY FEATURES
- Feature

## ARCHITECTURE NOTES
- Note

## RELATED
- [Related template](../path)
```

---

## Metadata Requirements

Every file must start with a metadata block:

```markdown
> **Version**: X.Y.Z | **Status**: draft | active | deprecated | **Owner**: name | **Last Updated**: YYYY-MM-DD
```

| Field | Required | Description |
|-------|----------|-------------|
| Version | Yes | Semantic version (X.Y.Z) |
| Status | Yes | draft, active, or deprecated |
| Owner | Yes | Department or engine name |
| Last Updated | Yes | ISO date of last modification |

---

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Departments | lowercase, hyphenated | `backend`, `uiux`, `qa` |
| Engines | lowercase, single word | `wizard`, `context`, `planning` |
| Workflows | lowercase, hyphenated | `feature-development`, `bug-fix` |
| Templates | lowercase, single word | `erp`, `saas`, `mobile` |
| Governance docs | UPPER_SNAKE_CASE | `GOVERNANCE.md` |
| Section headers | UPPERCASE | `## PURPOSE` |

---

## Formatting Rules

1. All files use GitHub-flavored Markdown
2. Headers use `##` for top-level sections, `###` for subsections
3. Lists use `-` for unordered, `1.` for ordered
4. Tables use standard Markdown table syntax
5. Code blocks specify language: ` ```yaml `
6. File paths in references are relative from `cosca/` root
7. Cross-references use `[Display Name](../path/to/file.md)`
8. One blank line between sections
9. Maximum line length: 120 characters (for readability)
10. No trailing whitespace
11. **All file content is written in English** — code, docs, skills, engines, workflows, memory records. The only exception is the Kernel↔Don conversation, which is always conducted in Brazilian Portuguese (PT-BR). Existing historical content in Portuguese is preserved as-is (records of past sessions); only new/edited content must follow this rule.

---

## Quality Checklist

Before considering a skill complete, verify:

- [ ] All mandatory sections present
- [ ] Metadata block at top
- [ ] Version is semantic (X.Y.Z)
- [ ] Status is one of: draft, active, deprecated
- [ ] No duplicated content with other files
- [ ] Cross-references use relative paths
- [ ] Tables are properly formatted
- [ ] No broken internal links
- [ ] Language is consistent with other skills
- [ ] Examples are concrete, not abstract
- [ ] Constraints are explicit, not implied
- [ ] Forbidden actions are clearly stated (departments only)

---

> **Enforced by**: Skills Engine | **Audited by**: Evolution Engine | **Last reviewed**: 2026-07-10
