> **Version**: 1.0.0 | **Status**: deprecated — use [documentation/ADR_CREATION](../documentation/ADR_CREATION.md) instead | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23
>
> # ADR GENERATION SKILL
>
> ## Description
> Generate Architecture Decision Records (ADRs) following the standard Cosca format. Captures decisions, context, alternatives, and consequences.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | decision_title | Yes | Title of the architecture decision |
> | context | Yes | Why this decision is needed |
> | alternatives | Yes | Alternatives considered |
> | decision | Yes | The chosen option |
> | consequences | Yes | Expected consequences of the decision |
> | status | No | `proposed`, `accepted`, `deprecated`, `superseded` |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | ADR document | Complete ADR in standard format |
> | ADR metadata | Status, date, deciders, tags |
>
> ## ADR Format
> ```markdown
> # ADR-NNN: [Title]
>
> **Status**: [proposed | accepted | deprecated | superseded]
> **Deciders**: [list of decision-makers]
> **Date**: [ISO date]
> **Tags**: [domain-tags]
>
> ## Context
> [What is the issue motivating this decision?]
>
> ## Decision Drivers
> - [Driver 1]
> - [Driver 2]
>
> ## Considered Options
> - [Option 1]
> - [Option 2]
> - [Option 3]
>
> ## Decision Outcome
> [Chosen option and rationale]
>
> ## Pros and Cons
> [Trade-off analysis]
>
> ## Consequences
> [Positive and negative consequences]
>
> ## Compliance
> [How to verify this decision is followed]
> ```
>
> ## Related
> - [Architecture Chief](../../departments/architecture/SKILL.md)
> - [ADR Creation](../../skills/documentation/ADR_CREATION.md)
> - [Architecture Validation](./ARCHITECTURE_VALIDATION.md)
