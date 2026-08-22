# Compliance Assessments

> **Category**: Best Practices → Compliance | **Version**: 1.0.0 | **Owner**: Cosca Compliance Chief | **Last Updated**: 2026-07-29

## Purpose

Regulatory compliance assessments for data protection and privacy frameworks. These assessments evaluate Cosca's conformance to legal requirements and identify remediation gaps.

## Assessments

| # | File | Regulation | Jurisdiction | Date | Status |
|---|------|------------|-------------|------|:------:|
| 1 | [`gdpr-lgpd-assessment.md`](gdpr-lgpd-assessment.md) | GDPR + LGPD | EU + Brazil | 2026-07-28 | Complete |

## Assessment Details

### GDPR/LGPD Assessment (2026-07-28)

**Scope**: Combined assessment covering both GDPR (EU General Data Protection Regulation) and LGPD (Brazilian Lei Geral de Proteção de Dados).

**Key Areas Evaluated**:
- Data collection and processing practices
- User consent mechanisms
- Data storage and retention policies
- Cross-border data transfer compliance
- Data subject rights (access, rectification, erasure)
- Data Protection Officer (DPO) requirements

**Relevance**: Cosca processes user data through agent interactions, memory storage, and provider integrations. Compliance evaluation is critical for production deployment.

## Related

- **[Playbook: Compliance Audit GDPR](../playbooks/playbook-compliance-audit.md)** — Step-by-step GDPR audit guide
- **[Risk Registry](../../failures/risks/RISK_REGISTRY.md)** — Risk R8 (documentation drift) directly impacts compliance documentation accuracy
- **[Heuristic H-009](../../heuristics/H-009-memory-drift-fiction.yaml)** — Memory/knowledge drift (GDPR fabrication was a real incident)

---

*Heuristic H-009 was partially derived from a GDPR fabrication incident where documentation falsely claimed compliance dates. Always verify compliance claims against actual implementation.*
