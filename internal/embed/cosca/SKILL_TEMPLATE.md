# SKILL TEMPLATE — New Skill Creation Guide

> **Version**: 1.0.0 | **Status**: active | **Owner**: Skills Engine | **Last Updated**: 2026-07-11

## Purpose
Use this template when creating any new department skill, engine skill, workflow, or template. Follow the [CONVENTIONS.md](CONVENTIONS.md) contract exactly.

All paths in this template use Virtual Path notation. Replace placeholders with actual values. Never use hardcoded paths. See [engines/resource-resolver/SKILL.md](engines/resource-resolver/SKILL.md).

---

## Department Skill Template

```markdown
# DEPARTMENT NAME — Short Description

> **Version**: 1.0.0 | **Status**: draft | **Owner**: [Department] Chief | **Last Updated**: YYYY-MM-DD

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
| Name | Description |

## DEPENDENCIES
| Depends On | Why |
|-----------|-----|
| Department/Engine | Reason |

## INPUTS
| Input | From | Format |
|-------|------|--------|
| Name | Source | Type |

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|
| Name | Consumer | Type |

## CONSTRAINTS
- Constraint or standard that must be followed

## QUALITY CRITERIA
- [ ] Criterion 1
- [ ] Criterion 2

## ESCALATION
| Issue | Escalate To |
|-------|-------------|
| Type of issue | Target department |

## FORBIDDEN ACTIONS
- Action 1
- Action 2

## RELATED
- [Related file](../path/to/file.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Author | Initial version |
```

---

## Engine Skill Template

```markdown
# ENGINE NAME

> **Version**: 1.0.0 | **Status**: draft | **Owner**: [Engine] Engine | **Last Updated**: YYYY-MM-DD

## PURPOSE
One paragraph describing why this engine exists.

## ACTIVATION
When this engine is triggered (events, conditions).

## SCOPE
What this engine covers.

## OUT OF SCOPE
What this engine does NOT cover.

## PROCESS
Step-by-step description of how the engine works.

## INPUTS
| Input | From | Format |
|-------|------|--------|
| Name | Source | Type |

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|
| Name | Consumer | Type |

## DEPENDENCIES
| Engine | Why |
|--------|-----|
| Name | Reason |

## CONSTRAINTS
- Constraint 1

## QUALITY CRITERIA
- [ ] Criterion 1

## RELATED
- [Related file](../path/to/file.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Author | Initial version |
```

---

## Workflow Template

```markdown
# WORKFLOW: workflow-name

> **Version**: 1.0.0 | **Status**: draft | **Category**: [init|feature|bug|refactor|review|deploy|maintenance|security|performance] | **Last Updated**: YYYY-MM-DD

## OBJECTIVE
One paragraph.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|

## PRECONDITIONS
1. Condition

## POSTCONDITIONS
1. Condition

## DEPENDENCIES
| Workflow | Reason |
|----------|--------|

## STEPS
### Step 1: Name
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
|---------|--------|

## RELATED
- [Related file](../path/to/file.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Author | Initial version |
```

---

## Template Scaffold Template

```markdown
# TEMPLATE NAME

> **Version**: 1.0.0 | **Status**: draft | **Last Updated**: YYYY-MM-DD

## DOMAIN
What type of application this template is for.

## RECOMMENDED STACK
| Layer | Technology |
|-------|-----------|

## MODULE STRUCTURE
```
project/
├── src/
├── tests/
└── README.md
```

## KEY FEATURES
- Feature 1

## ARCHITECTURE NOTES
- Note 1

## RELATED
- [Related template](../template-name/TEMPLATE.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Author | Initial version |
```

---

## Checklist Before Submitting

- [ ] Metadata block present with version, status, owner, date
- [ ] All mandatory sections present (per CONVENTIONS.md)
- [ ] No duplicated content with existing files
- [ ] Cross-references use relative paths
- [ ] Tables properly formatted
- [ ] HISTORY section populated
- [ ] Added to COSCA_INDEX.md
- [ ] Added to skills paths in opencode.jsonc (if department/engine)
- [ ] Agent config created in opencode.jsonc (if department with agents)

---

> **Enforced by**: Skills Engine | **Last reviewed**: 2026-07-10
