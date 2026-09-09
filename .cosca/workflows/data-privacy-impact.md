# WORKFLOW: data-privacy-impact

> **Version**: 1.0.0 | **Category**: compliance | **Estimated Duration**: 2-5 days | **Status**: active | **Owner**: Compliance Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Conduct Data Privacy Impact Assessments (DPIA) as required by GDPR and other privacy regulations. Identify privacy risks in new features or processes and define mitigations before implementation.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| feature_description | String | Yes | Description of the feature or process |
| data_categories | String[] | Yes | Types of personal data processed |
| data_subjects | String[] | Yes | Categories of data subjects affected |
| processing_purpose | String | Yes | Purpose of data processing |
| third_parties | String[] | No | Third parties with data access |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| DPIA report | Document | Completed DPIA with risk assessment |
| Risk register | Document | Identified privacy risks and mitigations |
| Approval decision | Document | DPIA approved, conditionally approved, or rejected |
| Data flow diagram | Document | Personal data flow map |

## STEPS
### Step 1: Screening
- **Chief**: Compliance Chief
- **Specialists**: Privacy Engineer
- **Task**: Determine if DPIA is required (high-risk processing assessment)
- **Output**: DPIA screening decision

### Step 2: Data Mapping
- **Chief**: Compliance Chief
- **Specialists**: Privacy Engineer, Database Chief
- **Task**: Map personal data flows, collection, storage, processing, sharing, deletion
- **Output**: Data flow diagram

### Step 3: Risk Identification
- **Chief**: Compliance Chief
- **Specialists**: Risk Analyst
- **Task**: Identify privacy risks to data subjects
- **Output**: Risk register

### Step 4: Risk Assessment
- **Chief**: Compliance Chief
- **Specialists**: Risk Analyst, Security Chief
- **Task**: Assess likelihood and severity of each risk
- **Output**: Risk assessment matrix

### Step 5: Mitigation Planning
- **Chief**: Compliance Chief
- **Specialists**: Security Chief, Backend Chief, Architecture Chief
- **Task**: Define mitigations for each identified risk
- **Output**: Mitigation plan

### Step 6: DPIA Documentation
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Compile complete DPIA report
- **Output**: DPIA report

### Step 7: Approval
- **Chief**: Compliance Chief
- **Specialists**: CEO (if high-risk)
- **Task**: Review and approve DPIA
- **Output**: Approval decision

## SUCCESS CRITERIA
- [ ] All personal data flows mapped
- [ ] Privacy risks identified and assessed
- [ ] Mitigations defined for all high/medium risks
- [ ] DPIA report complete and documented
- [ ] Approval obtained before feature implementation

## RELATED
- [Compliance Chief](../departments/compliance/SKILL.md)
- [Security Chief](../departments/security/SKILL.md)
- [skills/security/COMPLIANCE_VALIDATION.md](../skills/security/COMPLIANCE_VALIDATION.md)
- [workflows/compliance-audit.md](./compliance-audit.md)

## PRECONDITIONS
1. Feature description and data categories documented
2. Data subjects identified and processing purpose defined
3. Third parties with data access identified

## POSTCONDITIONS
1. DPIA report completed and stored in memory/architecture
2. Risk register updated with all identified privacy risks
3. Approval decision documented (approved/conditional/rejected)
4. Data flow diagram created for audit trail

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| High-risk processing identified without mitigations | Escalate to CEO, postpone feature |
| Data mapping incomplete | Extend discovery phase |
| Third-party compliance unclear | Request DPA or replace vendor |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
