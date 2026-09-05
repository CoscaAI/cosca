# WORKFLOW: compliance-audit

> **Version**: 1.0.0 | **Category**: audit | **Estimated Duration**: 2-5 days | **Status**: active | **Owner**: Compliance Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Conduct comprehensive compliance audits against regulatory standards (GDPR, SOC2, HIPAA, PCI-DSS, LGPD). Identify gaps, document evidence, and create remediation plans.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| standards | String[] | Yes | Compliance standards to audit against |
| scope | String | Yes | Systems, data, and processes in scope |
| previous_audit | Document | No | Previous audit findings and evidence |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Audit report | Document | Compliance status per control |
| Gap analysis | Document | Missing controls and risks |
| Evidence package | Document | Collected compliance evidence |
| Remediation plan | Document | Prioritized remediation items |

## PRECONDITIONS
1. Audit scope defined and approved
2. Stakeholders identified and notified
3. Previous audit reports available (if applicable)

## POSTCONDITIONS
1. Compliance status documented per control
2. Gaps identified and prioritized
3. Evidence collected and organized
4. Remediation plan approved by CTO

## STEPS
### Step 1: Scope Definition
- **Chief**: Compliance Chief
- **Specialists**: Audit Manager
- **Task**: Define audit scope, identify stakeholders, gather requirements
- **Output**: Audit scope document

### Step 2: Control Mapping
- **Chief**: Compliance Chief
- **Specialists**: Compliance Engineer
- **Task**: Map regulatory controls to system components
- **Output**: Control mapping matrix

### Step 3: Evidence Collection
- **Chief**: Compliance Chief
- **Specialists**: Compliance Engineer, All Chiefs
- **Task**: Collect evidence for each control (configs, logs, policies)
- **Output**: Evidence package

### Step 4: Gap Analysis
- **Chief**: Compliance Chief
- **Specialists**: Risk Analyst
- **Task**: Identify missing controls, assess risk levels
- **Output**: Gap analysis report

### Step 5: Remediation Planning
- **Chief**: Compliance Chief
- **Specialists**: All affected Chiefs
- **Task**: Create prioritized remediation plan with owners and timelines
- **Output**: Remediation plan

### Step 6: Report Generation
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Generate compliance audit report for stakeholders
- **Output**: Final audit report

## VALIDATION
1. All controls assessed
2. Evidence collected for all applicable controls
3. Gaps prioritized by severity
4. Remediation owners assigned
5. Audit trail complete

## SUCCESS CRITERIA
- [ ] All controls assessed
- [ ] Evidence collected and organized
- [ ] Gaps identified with risk levels
- [ ] Remediation plan with owners and dates
- [ ] Audit report delivered to stakeholders

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Evidence not available | Document as gap, assess compensatory controls |
| Stakeholder unavailability | Escalate to CTO for resource allocation |
| Conflicting requirements | Escalate to CTO and CEO for decision |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |

## RELATED
- [Compliance Chief](../departments/compliance/SKILL.md)
- [Security Chief](../departments/security/SKILL.md)
- [Compliance Validation skill](../skills/security/COMPLIANCE_VALIDATION.md)
- [Security Audit skill](../skills/security/SECURITY_AUDIT.md)
