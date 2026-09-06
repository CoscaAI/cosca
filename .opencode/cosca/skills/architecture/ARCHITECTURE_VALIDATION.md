---
name: architecture-validation
description: Use when the user asks to validate code against architectural rules, ADRs, and layered/module boundaries.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23
> 
> # ARCHITECTURE VALIDATION SKILL
> 
> ## Description
> Use this skill to validate code against architectural rules, ADRs, and established patterns. Ensures that implementation follows the defined architecture and prevents architecture erosion.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | code_changes | Yes | Code changes to validate |
> | architecture_rules | Yes | Architecture rules from ADRs or standards |
> | module_boundaries | Yes | Defined module boundaries |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Validation report | Architecture compliance report |
> | Violation list | Architecture violations with locations |
> | Pass/fail decision | Overall architecture validation result |
> 
> ## Process
> 1. Load architecture rules and ADRs
> 2. Validate module boundary compliance
> 3. Check dependency direction against rules
> 4. Validate pattern consistency
> 5. Verify ADR compliance
> 6. Generate validation report
> 
> ## Success Criteria
> - [ ] All architecture rules checked
> - [ ] Module boundaries validated
> - [ ] Dependency direction verified
> - [ ] ADR compliance confirmed
> - [ ] Report generated with clear pass/fail
> 
> ## Related
> - [Architecture Chief](../../departments/architecture/SKILL.md)
> - [Architecture Analysis](./ARCHITECTURE_ANALYSIS.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.1
> - [Review Chief](../../departments/review/SKILL.md)
