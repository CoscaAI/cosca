> **Version**: 1.0.0 | **Status**: active | **Owner**: Compliance Chief | **Last Updated**: 2026-07-23

# COMPLIANCE VALIDATION SKILL

## Description
Validate codebases, configurations, and processes against regulatory compliance standards. Covers GDPR, SOC2, HIPAA, PCI-DSS, and LGPD with automated control checks and evidence collection.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| standard | Yes | `gdpr`, `soc2`, `hipaa`, `pci-dss`, `lgpd`, `all` |
| scope_path | Yes | Path to codebase or configuration to validate |
| evidence_path | No | Path to collect compliance evidence |
| previous_findings | No | Previous audit findings for regression check |

## Outputs
| Output | Description |
|--------|-------------|
| Compliance report | Per-control compliance status |
| Gap analysis | Missing controls and risk levels |
| Evidence collected | Documentation of compliance evidence |
| Remediation plan | Prioritized steps to achieve compliance |

## Standards Coverage

### GDPR (General Data Protection Regulation)
- Data inventory and mapping
- Consent management mechanisms
- Data subject access request (DSAR) handling
- Data retention and deletion policies
- Privacy by design in new features
- Data breach notification procedures
- Data Processing Agreement (DPA) with vendors

### SOC2 (Service Organization Control)
- Security: Firewalls, intrusion detection, access control
- Availability: Monitoring, capacity planning, DR
- Processing Integrity: Data validation, error handling
- Confidentiality: Encryption, access controls
- Privacy: PII handling, consent, notice

### HIPAA (Health Insurance Portability and Accountability Act)
- PHI identification and encryption
- Access controls and audit trails
- Breach notification procedures
- Business Associate Agreements (BAA)
- Security risk analysis

### PCI-DSS (Payment Card Industry Data Security Standard)
- Cardholder data encryption
- Access control for payment systems
- Network segmentation
- Vulnerability management
- Security monitoring and testing

## Process
1. Load compliance controls for selected standard
2. Map controls to system components and configurations
3. Collect evidence for each control (configs, logs, policies)
4. Assess control effectiveness and coverage
5. Identify gaps and calculate risk scores
6. Generate compliance report with evidence
7. Create prioritized remediation plan
8. Update risk register with findings

## Success Criteria
- [ ] All applicable controls assessed
- [ ] Evidence collected for each control
- [ ] Gaps identified with risk ratings
- [ ] Remediation plan with owners and dates
- [ ] Compliance score calculated
- [ ] Report ready for auditor review

## Related
- [Compliance Chief](../../departments/compliance/SKILL.md)
- [Security Chief](../../departments/security/SKILL.md)
- [Security Audit](./SECURITY_AUDIT.md)
- [workflows/compliance-audit.md](../../workflows/compliance-audit.md)
- [Governance Chief](../../departments/governance/SKILL.md)
