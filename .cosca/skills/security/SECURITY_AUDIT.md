> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-23
> 
> # SECURITY AUDIT SKILL
> 
> ## Description
> Use this skill to perform comprehensive security audits. Covers OWASP Top 10, dependency vulnerabilities, secrets exposure, authentication/authorization, and security configuration.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | codebase_path | Yes | Path to codebase to audit |
> | audit_scope | Yes | `full`, `quick`, `dependency-only`, `secrets-only` |
> | compliance_standard | No | `owasp`, `gdpr`, `hipaa`, `pci-dss`, `soc2` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Audit report | Complete security findings |
> | Vulnerability inventory | All vulnerabilities with severity |
> | Remediation plan | Prioritized fixes |
> | Compliance report | Compliance status per standard |
> 
> ## Audit Dimensions
> 
> ### Code Security (OWASP Top 10)
> - Injection (SQL, NoSQL, OS, LDAP)
> - Broken authentication
> - Sensitive data exposure
> - XML External Entities (XXE)
> - Broken access control
> - Security misconfiguration
> - Cross-Site Scripting (XSS)
> - Insecure deserialization
> - Known vulnerabilities
> - Insufficient logging/monitoring
> 
> ### Infrastructure Security
> - Network security groups
> - TLS/SSL configuration
> - Secret management
> - Container security
> - IAM permissions
> 
> ### Dependency Security
> - Known CVEs in dependencies
> - Outdated packages
> - Unused dependencies
> - License compliance
> 
> ## Success Criteria
> - [ ] All audit dimensions covered
> - [ ] Vulnerabilities categorized by severity
> - [ ] Remediation plan prioritized
> - [ ] Compliance status documented
> - [ ] 0 critical/high findings for gate pass
> 
> ## Related
> - [Security Chief](../../departments/security/SKILL.md)
> - [Compliance Chief](../../departments/compliance/SKILL.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.3 Security
> - [SECURITY_ARCHITECTURE.md](../../SECURITY_ARCHITECTURE.md)
> - [workflows/security-audit.md](../../workflows/security-audit.md)

## Process
1. **Scope Definition**: Identify audit boundary (which packages, APIs, subsystems).
2. **Dependency Scan**: Run `govulncheck ./...` for Go, check `npm audit` for frontend dependencies. Flag any CVEs.
3. **Secrets Scan**: Search codebase for hardcoded secrets (API keys, passwords, tokens). Use regex patterns: `(= *"[A-Za-z0-9+/]{20,}")`.
4. **Auth Review**: Verify JWT implementation (algorithm, expiry, rotation), RBAC enforcement at middleware level, API key scopes.
5. **Input Validation**: Check all user-input paths for validation, sanitization, and proper error responses.
6. **Infrastructure Scan**: Review Dockerfiles, K8s manifests, terraform for security misconfigurations.
7. **Compliance Check**: Map findings to OWASP Top 10 categories and compliance standards (GDPR, LGPD).
8. **Generate Report**: Output prioritized vulnerability list with CVSS scores and remediation steps.
