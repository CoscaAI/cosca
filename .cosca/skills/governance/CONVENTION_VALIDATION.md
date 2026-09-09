> **Version**: 1.0.0 | **Status**: active | **Owner**: Governance Chief | **Last Updated**: 2026-07-23
> 
> # CONVENTION VALIDATION SKILL
> 
> ## Description
> Use this skill to validate files against Cosca CONVENTIONS.md standards. Ensures all framework files follow the canonical format, metadata requirements, naming conventions, and cross-reference standards.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | file_path | Yes | Path to file(s) to validate |
> | file_type | Yes | `department`, `engine`, `workflow`, `template`, `skill`, `governance` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Compliance report | CONVENTIONS compliance status |
> | Violations list | Specific violations with locations |
> | Compliance score | Percentage score (> 80% required) |
> 
> ## Validation Checklist
> 
> ### Metadata (all files)
> - [ ] Version field present with SemVer (X.Y.Z)
> - [ ] Status field: draft | active | deprecated | retired
> - [ ] Owner field present
> - [ ] Last Updated field with ISO date
> 
> ### Department Skills (additional)
> - [ ] PURPOSE section present
> - [ ] SCOPE section present
> - [ ] OUT OF SCOPE section present
> - [ ] RESPONSIBILITIES (8-12 items)
> - [ ] DELEGATION defined
> - [ ] SPECIALISTS listed
> - [ ] DEPENDENCIES with table
> - [ ] INPUTS with table
> - [ ] OUTPUTS with table
> - [ ] CONSTRAINTS listed
> - [ ] QUALITY CRITERIA (checklist)
> - [ ] ESCALATION with table
> - [ ] FORBIDDEN ACTIONS listed
> - [ ] RELATED with cross-references
> - [ ] HISTORY section present
> 
> ### Naming Conventions
> - [ ] Department names: lowercase, hyphenated
> - [ ] Workflow names: lowercase, hyphenated
> - [ ] Governance docs: UPPER_SNAKE_CASE
> - [ ] Section headers: UPPERCASE
> 
> ### Cross-references
> - [ ] All internal links valid
> - [ ] Relative paths used (not absolute)
> - [ ] No broken references
> 
> ## Success Criteria
> - [ ] All mandatory sections present
> - [ ] Metadata block valid
> - [ ] Naming conventions followed
> - [ ] Cross-references valid
> - [ ] Compliance score > 80%
> 
> ## Related
> - [Governance Chief](../../departments/governance/SKILL.md)
> - [CONVENTIONS.md](../../CONVENTIONS.md) — Standard skill contract
> - [AGENT_DNA.md](../../AGENT_DNA.md) — Agent contract standard
> - [GOVERNANCE.md](../../GOVERNANCE.md) — Governance policies
