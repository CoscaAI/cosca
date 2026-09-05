# Compliance Engine — Seed Data

> **Version**: 1.0.0 | **Status**: active | **Last Updated**: 2026-07-23

## Compliance Policies

### Policy: Data Retention
```yaml
id: POL-DATA-001
name: Data Retention Policy
standard: GDPR, SOC2
status: active
enforced_since: 2026-01-01
rules:
  - data_type: user_profile
    retention_days: 2555  # 7 years after account closure
    deletion_method: soft_delete_30d_then_hard_delete
  - data_type: audit_log
    retention_days: 1095  # 3 years
    deletion_method: archive_then_delete
  - data_type: session_data
    retention_days: 30
    deletion_method: immediate_hard_delete
  - data_type: backup
    retention_days: 90  # daily backups
    deletion_method: automated_pruning
```

### Policy: Access Control
```yaml
id: POL-ACCESS-001
name: Access Control Policy
standard: SOC2, HIPAA
status: active
enforced_since: 2026-03-01
rules:
  - principle: least_privilege
    mandatory: true
  - principle: separation_of_duties
    mandatory: true
  - principle: access_review
    frequency: quarterly
  - mfa_required: true
  - password_policy:
      min_length: 12
      require_special: true
      require_number: true
      max_age_days: 90
```

### Policy: Breach Notification
```yaml
id: POL-BREACH-001
name: Breach Notification Policy
standard: GDPR
status: active
enforced_since: 2026-01-01
rules:
  - notification_to_dpa: within_72_hours
  - notification_to_affected: without_undue_delay
  - documentation_required: true
  - escalation_chain:
      - level_1: security_chief
      - level_2: compliance_chief
      - level_3: ceo
```

### Compliance Controls Status
| Control ID | Description | Standard | Status | Last Verified |
|-----------|-------------|----------|--------|:------------:|
| CC-001 | Data encryption at rest | GDPR, SOC2 | ✅ Compliant | 2026-06-15 |
| CC-002 | Data encryption in transit | GDPR, SOC2 | ✅ Compliant | 2026-06-15 |
| CC-003 | Access logging | SOC2, HIPAA | ✅ Compliant | 2026-06-10 |
| CC-004 | Vulnerability scanning | SOC2, PCI-DSS | ✅ Compliant | 2026-06-20 |
| CC-005 | Penetration testing | SOC2, PCI-DSS | ⚠️ Due in 30 days | 2026-03-15 |
| CC-006 | Data retention enforcement | GDPR | ✅ Compliant | 2026-06-18 |
| CC-007 | Breach notification procedure | GDPR | ✅ Compliant | 2026-06-12 |
| CC-008 | Vendor risk assessment | SOC2 | ❌ Overdue (60 days) | 2025-12-01 |

## Related
- [Compliance Engine](../tools/SKILL.md)
- [Compliance Chief](../../departments/compliance/SKILL.md)
- [Security Chief](../../departments/security/SKILL.md)
