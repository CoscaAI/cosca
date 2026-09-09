# WORKFLOW: security-audit

> **Version**: 1.0.0 | **Status**: active | **Category**: security | **Last Updated**: 2026-07-10

## OBJECTIVE
Comprehensive security audit of the codebase including dependency scanning, static analysis, secret detection, and OWASP Top 10 compliance review.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| scope | string | No | full (entire codebase) or diff (recent changes only) |
| include_dependencies | boolean | No | Whether to scan dependencies (default: true) |
| include_secrets | boolean | No | Whether to scan for secrets (default: true) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| vulnerability_report | object | List of vulnerabilities by severity |
| dependency_audit | object | Dependency vulnerability scan results |
| secret_scan | object | Secret detection results |
| compliance_report | object | OWASP Top 10 compliance status |
| recommendations | array | Remediation recommendations |

## PRECONDITIONS
1. Codebase accessible
2. Dependency files present

## POSTCONDITIONS
1. Complete audit report generated
2. Critical/High vulnerabilities identified
3. Remediation plan created (if issues found)
4. Audit stored in memory for trend analysis

## DEPENDENCIES
None

## STEPS

### Step 1: Dependency Scan
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Scan all dependencies for known vulnerabilities (CVEs)
- **Output**: Dependency vulnerability report

### Step 2: Static Analysis (SAST)
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Run static code analysis for security patterns
- **Output**: SAST findings report

### Step 3: Secret Detection
- **Chief**: Security
- **Specialists**: Secrets Manager
- **Task**: Scan codebase for hardcoded secrets, keys, tokens
- **Output**: Secret scan report

### Step 4: OWASP Review
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Manual/automated review against OWASP Top 10
- **Output**: OWASP compliance matrix

### Step 5: Configuration Review
- **Chief**: Security
- **Specialists**: Compliance Engineer
- **Task**: Review CORS, CSP, security headers, HTTPS config
- **Output**: Configuration security report

### Step 6: Threat Model Update
- **Chief**: Security
- **Specialists**: Penetration Tester
- **Task**: Review threat model for new attack surfaces
- **Output**: Updated threat model

### Step 7: Report Generation
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Compile all findings into comprehensive audit report
- **Output**: Security audit report with priorities and recommendations

### Step 8: Remediation Planning
- **Chief**: Security
- **Specialists**: Compliance Engineer
- **Task**: Create prioritized remediation plan for findings
- **Output**: Remediation plan with timelines and owners

## VALIDATION
1. All scan tools executed successfully
2. Report covers all scoped areas
3. Findings classified by severity
4. Remediation steps actionable
5. Audit stored for trend comparison

## SUCCESS CRITERIA
- [ ] Complete audit report generated
- [ ] All scans executed (deps, SAST, secrets, OWASP, config)
- [ ] Findings prioritized (critical → low)
- [ ] Remediation plan created
- [ ] Audit stored in memory

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Scan tool failure | Retry with alternative tool |
| Incomplete scan | Report partial results with scope gaps noted |
| Zero findings | Confirm scan coverage, not just absence |

## RELATED
- [Security Chief](../departments/security/SKILL.md)
- [QUALITY_GATES.md](../identidade/QUALITY_GATES.md)
- [Dependency Update Workflow](dependency-update.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Initial security-audit workflow |
