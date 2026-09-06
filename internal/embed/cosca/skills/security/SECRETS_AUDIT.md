---
name: secrets-audit
description: Use when the user asks to scan code, config, git history, or infrastructure manifests for hardcoded secrets and credentials.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-23

# SECRETS AUDIT SKILL

## Description
Scan codebases, configuration files, environment files, git history, and infrastructure manifests for hardcoded secrets, credentials, API keys, tokens, certificates, and other sensitive data exposure.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| scan_path | Yes | Path to directory or repository to scan |
| scan_depth | No | `quick` (current files only), `deep` (entire git history), `full` (git + files + configs) |
| custom_patterns | No | Additional regex patterns for proprietary secret formats |
| exclude_paths | No | Paths to exclude from scan (node_modules, vendor, etc.) |

## Outputs
| Output | Description |
|--------|-------------|
| Secrets report | All findings with file paths and line numbers |
| Risk assessment | Severity per exposed secret (critical, high, medium, low) |
| Remediation guide | Step-by-step: rotate, revoke, remove from history |

## Secret Detection Patterns
- API keys (AWS, GCP, Azure, OpenAI, Stripe, etc.)
- Database connection strings
- JWT tokens and signing secrets
- Private SSH keys and certificates
- OAuth client secrets and tokens
- Password strings in configuration
- Encryption keys and salts
- Environment variable files (.env, .env.prod)
- Hardcoded credentials in code
- Secrets in Dockerfiles and docker-compose
- Secrets in CI/CD configuration
- Secrets in Terraform/IaC state files

## Process
1. Configure scan scope and exclusions
2. Run regex-based secret pattern matching on files
3. Analyze git history for committed secrets (full depth)
4. Scan environment and configuration files
5. Check CI/CD pipeline variables
6. Validate findings against entropy analysis
7. Remove false positives (test tokens, documentation examples)
8. Categorize findings by severity and type
9. Generate remediation report with rotation steps
10. Create audit trail entry for compliance

## Success Criteria
- [ ] All targeted paths scanned
- [ ] Git history analyzed for committed secrets
- [ ] Findings categorized by severity
- [ ] False positives identified and documented
- [ ] Remediation steps provided for each finding
- [ ] Audit log generated for compliance

## Related
- [Security Chief](../../departments/security/SKILL.md)
- [Security Audit](./SECURITY_AUDIT.md)
- [Vulnerability Assessment](./VULNERABILITY_ASSESSMENT.md)
- [Compliance Validation](./COMPLIANCE_VALIDATION.md)
- [Secrets Engine](../../engines/secrets/SKILL.md)
- [workflows/secrets-rotation.md](../../workflows/secrets-rotation.md)
