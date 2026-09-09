# WORKFLOW: secrets-rotation

> **Version**: 1.0.0 | **Category**: security | **Estimated Duration**: 30-120 min | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Rotate secrets (API keys, database credentials, certificates, tokens) following security best practices. Ensure zero downtime during rotation and immediate invalidation of old secrets.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| secret_type | String | Yes | API-key, db-credential, certificate, token, ssh-key |
| secret_identifier | String | Yes | Which secret(s) to rotate |
| rotation_reason | String | Yes | scheduled, compromise, compliance, expiry |
| auto_rollback | Boolean | No | Enable auto-rollback on failure (default: true) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Rotation result | Success/Fail | Rotation outcome |
| New secret fingerprint | String | Identification of new secret |
| Verification report | Document | Services verified with new secret |
| Audit log | Log | Complete rotation audit trail |

## PRECONDITIONS
1. Secrets manager (vault) operational
2. All dependent services identified
3. Rollback plan documented
4. Maintenance window approved (if required)

## POSTCONDITIONS
1. Old secret invalidated
2. All services using new secret
3. Rotation logged in audit trail
4. No service disruption

## STEPS
### Step 1: Discovery
- **Chief**: Security Chief
- **Specialists**: Secrets Manager
- **Task**: Identify all services using the secret, assess impact
- **Output**: Dependency map

### Step 2: New Secret Generation
- **Chief**: Security Chief
- **Specialists**: Secrets Manager
- **Task**: Generate new secret in vault
- **Output**: New secret stored

### Step 3: Dual-Lifecycle Activation
- **Chief**: Security Chief
- **Specialists**: DevOps Chief
- **Task**: Deploy new secret alongside old (dual lifecycle)
- **Output**: Dual activation complete

### Step 4: Service Migration
- **Chief**: Backend/DevOps Chiefs
- **Specialists**: Service teams
- **Task**: Update each service to use new secret
- **Output**: Services migrated

### Step 5: Old Secret Invalidation
- **Chief**: Security Chief
- **Specialists**: Secrets Manager
- **Task**: Verify all services on new secret, invalidate old
- **Output**: Old secret revoked

### Step 6: Verification
- **Chief**: Monitoring Chief
- **Specialists**: —
- **Task**: Verify all services operational, no auth errors
- **Output**: Verification report

### Step 7: Audit
- **Chief**: Compliance Chief
- **Specialists**: Compliance Engineer
- **Task**: Log rotation in compliance audit trail
- **Output**: Audit entry

## VALIDATION
1. All services verified with new secret
2. Old secret no longer valid
3. No auth errors post-rotation
4. Rotation logged in audit trail
5. Compliance requirements met

## SUCCESS CRITERIA
- [ ] New secret generated and stored securely
- [ ] All services migrated to new secret
- [ ] Old secret invalidated
- [ ] Zero downtime during rotation
- [ ] Rotation logged in audit trail
- [ ] Compliance check passed

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Service fails to update | Escalate to service owner, auto-rollback (if enabled) |
| New secret compromised | Immediately generate another new secret |
| Service unavailable for rotation | Document, schedule rotation, maintain dual lifecycle |
| Vault outage during rotation | Escalate to Security Chief, manual rotation procedure |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |

## RELATED
- [Security Chief](../departments/security/SKILL.md)
- [Secrets Engine](../engines/secrets/SKILL.md)
- [Secrets Audit skill](../skills/security/SECRETS_AUDIT.md)
- [Compliance Chief](../departments/compliance/SKILL.md)
