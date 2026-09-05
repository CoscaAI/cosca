---
name: secrets
description: Provides centralized, secure credential management - encryption, rotation, audit, and least privilege.
level: 3
---

# SECRETS MANAGER ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Secrets Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Secrets Manager Engine provides centralized, secure credential management for all Cosca agents and workflows. No agent shall ever hardcode or log secrets. All credential access flows through this engine with encryption, rotation, audit, and least-privilege enforcement.

## ACTIVATION
- **On agent spawn**: Inject required secrets into agent context (never written to disk)
- **On credential request**: Validate, retrieve, and inject at runtime
- **On rotation schedule**: Rotate credentials automatically per policy
- **On security audit**: Verify no leaked secrets in code, logs, or memory stores

## SCOPE
- Secure storage and retrieval of all credential types
- Automatic credential rotation with zero-downtime cutover
- Runtime injection (never persisted in agent context files)
- Audit logging of all credential access (who, when, for what)
- Leak detection (scan code, logs, memory stores for exposed secrets)
- Least-privilege enforcement (agent X can only access secrets in scope Y)
- Integration with external vaults (HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager, Doppler, SOPS)

## OUT OF SCOPE
- Authorization logic (delegate to Security Chief)
- API key generation for external services (delegate to Integrations Chief)
- Encryption algorithm implementation (delegate to Security Chief)
- Physical hardware security (HSM, TPM — delegate to Infrastructure Chief)

## PROCESS

### 1. Secret Registration
```
Agent/Chief registers credential type:
  → Secrets Engine validates format
  → Encrypts with envelope encryption (data key + master key)
  → Stores in vault backend
  → Returns secret_id (never the secret value)
```

### 2. Runtime Injection
```
Agent requests secret:
  → Validates agent identity and scope
  → Checks access policy (least-privilege)
  → Decrypts from vault
  → Injects as environment variable in agent sandbox
  → Logs access in audit trail (who, when, secret_id, NOT the value)
  → Secret never written to disk
```

### 3. Rotation
```
Trigger: schedule (e.g., 90 days) OR compromise event
  → Generate new credential
  → Store new version in vault (versioned)
  → Grace period: old + new both valid for N minutes
  → Notify dependent services
  → Revoke old credential after grace period
  → Log rotation event
```

### 4. Leak Detection
```
Scan triggers: pre-commit, code review, periodic audit
  → Scan codebase for patterns (private keys, tokens, passwords)
  → Scan logs for credential patterns
  → Scan memory stores for exposed secrets
  → On detection: block commit, alert Security Chief, initiate rotation
```

## SECRET TYPES

| Type | Examples | Rotation | Vault Backend |
|------|----------|----------|---------------|
| API Keys | OpenAI, Anthropic, Stripe, SendGrid | 90 days | HashiCorp Vault |
| Database Credentials | PostgreSQL, MongoDB, Redis URLs | 30 days | Vault Dynamic Secrets |
| Cloud Credentials | AWS IAM, GCP SA, Azure SP | 90 days | Native cloud vault |
| TLS Certificates | SSL/TLS certs, private keys | 90 days (auto-renew) | Vault PKI |
| OAuth Tokens | GitHub, Google, OIDC tokens | Per-provider policy | Encrypted store |
| Webhook Secrets | Stripe webhook, GitHub webhook | On rotation schedule | Vault KV v2 |
| Environment Variables | Non-secret config | Never | cosca.config.yaml |
| Agent Tokens | Agent-to-agent auth tokens | 7 days | Vault Transit |

## SECURITY POLICIES

### Access Control Matrix
| Role | Can Read | Can Write | Can Rotate | Can Delete | Can Audit |
|------|----------|-----------|------------|------------|-----------|
| Kernel | ✅ (inject only) | ❌ | ❌ | ❌ | ❌ |
| Security Chief | ⚠️ (audit only) | ✅ | ✅ | ✅ | ✅ |
| DevOps Chief | ❌ | ✅ (register) | ✅ | ❌ | ❌ |
| Backend Chief | ❌ | ❌ | ❌ | ❌ | ❌ |
| Any Specialist | ❌ | ❌ | ❌ | ❌ | ❌ |
| Integrations Chief | ❌ | ✅ (register) | ❌ | ❌ | ❌ |

### Leak Prevention Rules
```
1. Secrets NEVER in code → enforced by pre-commit hook
2. Secrets NEVER in logs → redaction middleware
3. Secrets NEVER in memory stores → scan on write
4. Secrets NEVER in agent context files → injected at runtime only
5. Secrets NEVER in git → .gitignore + pre-commit + audit
6. Secrets NEVER in environment dumps → masked in /status output
```

## INTEGRATION

### Vault Backend Interface
```yaml
backend:
  type: hashicorp_vault | aws_secrets_manager | gcp_secret_manager | doppler | sops | env_file
  config:
    endpoint: https://vault.internal:8200
    auth_method: kubernetes | iam | token | app_role
    namespace: cosca/production
  fallback:
    type: env_file
    config:
      path: /etc/cosca/secrets.env
      encrypted: true
```

### Agent Injection Protocol
```
1. Agent requests spawn
2. Kernel identifies required secrets from task context
3. Kernel requests secrets from Secrets Engine
4. Secrets Engine validates access, decrypts, returns temporary handle
5. Kernel injects handle as environment variable (COSCA_SECRET_HANDLE)
6. Agent uses handle to access secrets at runtime
7. Handle expires when agent session ends
8. Secret value never in agent's memory dump
```

## INPUTS

| Input | From | Format |
|-------|------|--------|
| Secret registration request | DevOps Chief, Integrations Chief | Registration payload |
| Agent spawn request (with scope) | Kernel | Agent context |
| Rotation trigger | Scheduler or manual | Rotation event |
| Leak scan trigger | Pre-commit hook, Review Engine | Scan event |

## OUTPUTS

| Output | To | Format |
|--------|-----|--------|
| Secret handle (temporary) | Kernel → Agent sandbox | Environment variable |
| Leak detection report | Security Chief, Review Chief | Scan report |
| Rotation confirmation | DevOps Chief | Rotation log |
| Access audit trail | Audit Engine | Audit events |
| Vault health status | Monitoring Chief | Health metrics |

## DEPENDENCIES

| Depends On | Why |
|-----------|-----|
| Security Chief | Security policies, encryption standards, access rules |
| DevOps Chief | Vault infrastructure, deployment configuration |
| Audit Engine | All secret access events logged for compliance |
| Monitoring Chief | Vault health, rotation status, leak alerts |
| Review Engine | Pre-commit leak detection integration |
| Kernel | Agent spawn → secret injection flow |
| Resource Resolver | Vault endpoint and config path resolution |

## CONSTRAINTS

- Secrets must never be logged, committed, or stored unencrypted
- All secret values encrypted at rest with AES-256-GCM
- All secret access must be authenticated and authorized
- Rotation must support grace period (old + new valid simultaneously)
- Leak detection must block commits (not just warn)
- Vault backend must support HA (no single point of failure)
- Secret handles must expire with agent session (TTL-based)
- Maximum secret age: 90 days (auto-rotation required)

## QUALITY CRITERIA

- [ ] All secrets encrypted at rest (AES-256-GCM minimum)
- [ ] No secrets found in codebase (0 tolerated)
- [ ] No secrets found in logs (redaction verified)
- [ ] No secrets found in memory stores (scan passing)
- [ ] Rotation completed for all secrets older than 90 days
- [ ] Vault backend health check passing
- [ ] Leak detection running on every commit
- [ ] Audit trail complete for all access events
- [ ] Grace period works (old + new valid during rotation window)

## ESCALATION

| Issue | Escalate To |
|-------|-------------|
| Credential leak detected | Security Chief (IMMEDIATE) |
| Vault backend unavailable | DevOps Chief |
| Rotation failure | DevOps Chief + Security Chief |
| Unauthorized access attempt | Security Chief |
| Encryption key compromise | CTO + Security Chief (CRITICAL) |

## RELATED

- [Security Chief](../../departments/security/SKILL.md) — Security policies and encryption standards
- [DevOps Chief](../../departments/devops/SKILL.md) — Vault infrastructure and deployment
- [Audit Engine](../audit/SKILL.md) — Secret access audit trail
- [Review Engine](../review/SKILL.md) — Pre-commit leak detection
- [Integrations Chief](../../departments/integrations/SKILL.md) — Third-party API credential registration
- [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md) — AI provider credential management
- [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.3 Security checks

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Secrets Manager Engine — centralized credential management, rotation, leak detection |
