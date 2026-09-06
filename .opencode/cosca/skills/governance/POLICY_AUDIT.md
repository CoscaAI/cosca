---
name: policy-audit
description: Use when the user asks to audit policies for coverage, consistency, and enforcement gaps.
---

# Policy Audit

> **Version**: 1.0.0 | **Status**: active | **Owner**: Governance Chief | **Last Updated**: 2026-07-27

## Purpose
Audit project against Cosca governance policies, conventions, and lifecycle rules.

## Process
1. Load governance documents: CONVENTIONS.md, GOVERNANCE.md, QUALITY_GATES.md.
2. Check agent compliance: every agent has version, status, owner, AGENT_DNA fields.
3. Check skill compliance: every skill follows SKILL_TEMPLATE.md format with Process section.
4. Check deprecation policy: no references to deprecated agents or skills from active code.
5. Check versioning policy: semver used consistently, no breaking changes without major bump.
6. Generate compliance report with violations grouped by severity.

## Success Criteria
- All active agents compliant with AGENT_DNA (23 fields)
- All active skills compliant with SKILL_TEMPLATE.md
- Zero references to deprecated resources from active code
