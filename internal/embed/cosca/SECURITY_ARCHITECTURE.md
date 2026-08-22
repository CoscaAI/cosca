# SECURITY ARCHITECTURE — Enterprise Cybersecurity Framework

> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-12

## PURPOSE
This document is the single source of truth for all cybersecurity policies, standards, and procedures in the Cosca ecosystem. It implements defense-in-depth across 8 security domains. Every agent, engine, and workflow must comply. No exception.

---

## 1. SECURITY PRINCIPLES

| Principle | Rule | Enforcement |
|-----------|------|-------------|
| **Zero Trust** | Never trust, always verify. Every request authenticated and authorized. | Identity Engine |
| **Least Privilege** | Agents get minimum permissions needed. Elevated access requires justification + TTL. | Policy Engine + Identity Engine |
| **Defense in Depth** | Multiple security layers. If one fails, others catch it. | All 8 domains below |
| **Secure by Default** | New projects start with maximum security. Opt-out requires Security Council approval. | Security Baseline (project-init Step 7) |
| **Shift Left** | Security at every stage: design → code → build → test → deploy → monitor. | Quality Gates 0-4 |
| **Assume Breach** | Design for when (not if) a breach occurs. Isolate, detect, respond, recover. | Incident Response + Recovery Engine |
| **Privacy by Design** | PII encrypted at rest and in transit. Data minimization. Right to erasure. | Compliance Engine |
| **Never Trust the Model** | AI output validated before user-facing use. Prompt injection defenses. No secrets in prompts. | AI Council + Secrets Engine |

---

## 2. SECURITY DOMAINS (8)

```
┌─────────────────────────────────────────────────────────────┐
│                    SECURITY DOMAINS                          │
├─────────────────────────────────────────────────────────────┤
│ 1. Identity & Access    → Who are you? What can you do?     │
│ 2. Data Protection      → Encryption, masking, retention    │
│ 3. Application Security → OWASP, SAST, DAST, dependency     │
│ 4. Infrastructure       → Network, containers, cloud        │
│ 5. Supply Chain         → SBOM, signing, provenance         │
│ 6. AI/ML Security       → Prompt injection, model poisoning │
│ 7. Incident Response    → Detect, contain, eradicate, recover│
│ 8. Compliance           → GDPR, SOC2, HIPAA, PCI-DSS        │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. DOMAIN 1: IDENTITY & ACCESS MANAGEMENT

### 3.1 Authentication Standards
| Method | Use Case | Minimum Requirement |
|--------|----------|-------------------|
| API Keys | Service-to-service (agent-to-agent) | 256-bit random, rotated every 90 days |
| JWT (Access) | User/agent sessions | RS256, 15 min TTL, no secrets in payload |
| JWT (Refresh) | Session renewal | 256-bit random, 7 day TTL, single use, rotation on use |
| OAuth 2.0 / OIDC | Third-party integration | PKCE required, state parameter validated |
| mTLS | Critical service communication | Certificate pinning, rotation every 30 days |
| Biometric | Mobile/desktop apps | Platform-native (FaceID, TouchID, Windows Hello) |

### 3.2 Authorization Model (RBAC + ABAC Hybrid)
```
RBAC Layer:
  Role → Permissions
  Admin > Chief > Specialist > ReadOnly

ABAC Layer:
  Subject (who) + Action (what) + Resource (where) + Context (when/why)
  
  Example: "Backend Chief CAN write_file ON src/api/ DURING business hours FROM approved IP"
```

### 3.3 Identity Engine Enforcement
| Check | When | Action on Failure |
|-------|------|------------------|
| Agent authentication | Every spawn | Block spawn, alert Security Chief |
| Token validation | Every request | Return 401, log attempt |
| Permission check | Every tool call | Deny tool, log attempt |
| Tenant isolation | Every cross-tenant access | Block, alert immediately |
| Session timeout | Every 15 min (access token) | Force re-auth, preserve workflow state |
| Anomaly detection | Continuous | Escalate to Security Council |

---

## 4. DOMAIN 2: DATA PROTECTION

### 4.1 Encryption Standards
| Data State | Algorithm | Key Management |
|-----------|-----------|---------------|
| At Rest | AES-256-GCM | Envelope encryption (DEK + KEK), HSM-backed |
| In Transit | TLS 1.3 | Certificate pinning, HSTS, forward secrecy |
| In Memory | In-memory only (no swap) | mlock, encrypted swap disabled |
| In Backups | AES-256-GCM | Separate backup key, offline master key |

### 4.2 Data Classification
| Level | Examples | Storage | Encryption | Retention |
|-------|----------|---------|------------|-----------|
| **P0 - Secrets** | API keys, passwords, tokens | Secrets Engine (vault) | AES-256-GCM + HSM | Rotated per policy |
| **P1 - PII** | Emails, names, addresses, IPs | Encrypted DB column | AES-256-GCM | Per compliance + right-to-erasure |
| **P2 - Confidential** | Source code, architecture docs, ADRs | Encrypted at rest | AES-256-GCM | Project lifetime |
| **P3 - Internal** | Workflow logs, agent metrics | Standard storage | AES-256-GCM (optional) | 12 months |
| **P4 - Public** | README, public docs, changelogs | Standard storage | None | Forever |

### 4.3 Data Privacy Matrix
| Regulation | Requirement | Cosca Enforcement |
|-----------|------------|-----------------|
| GDPR | Right to erasure, data portability, consent | Compliance Engine + Memory pruning |
| CCPA | Right to know, right to delete, opt-out | Compliance Engine |
| HIPAA | PHI encryption, access logging, BAA | Compliance Engine + Audit Engine |
| PCI-DSS | Card data never stored, tokenization | Secrets Engine (PCI scope isolation) |
| SOC2 | Security, availability, confidentiality | Full audit trail + Compliance reports |

---

## 5. DOMAIN 3: APPLICATION SECURITY

### 5.1 OWASP Top 10 (2021) — Cosca Coverage
| # | Vulnerability | Cosca Defense |
|---|-------------|------------|
| A01 | Broken Access Control | Identity Engine + RBAC/ABAC + permission check on every tool call |
| A02 | Cryptographic Failures | Secrets Engine (never hardcoded), TLS 1.3, AES-256-GCM |
| A03 | Injection | Parameterized queries only, input validation on all agent inputs, output encoding |
| A04 | Insecure Design | Architecture Council review, threat modeling per feature |
| A05 | Security Misconfiguration | Security Baseline (project-init Step 7), CSP headers, secure defaults |
| A06 | Vulnerable Components | Dependency audit every 24h, SBOM generation, CVE scanning |
| A07 | Auth Failures | MFA for admin, JWT best practices, brute-force protection |
| A08 | Software & Data Integrity | Signed commits, SBOM verification, CI/CD pipeline integrity |
| A09 | Logging & Monitoring Failures | Audit Engine (every action logged), Observability (anomaly detection) |
| A10 | SSRF | Agent tool restrictions, network egress filtering, URL validation |

### 5.2 Security Scanning Pipeline
```
Commit → Pre-commit hooks (secrets scan, basic lint)
  ↓
PR → SAST (static analysis), Dependency audit (CVE check)
  ↓
Build → Container scan (Trivy), SBOM generation (CycloneDX)
  ↓
Test → DAST (dynamic analysis), Fuzz testing
  ↓
Deploy → IaC scan (tfsec, checkov), Compliance check
  ↓
Production → RASP, WAF, continuous monitoring
```

### 5.3 Secure Coding Standards
| Language | Standard | Enforced By |
|----------|----------|-------------|
| TypeScript/JavaScript | ESLint security plugin, no-eval, CSP headers | Pre-commit + CI |
| Python | Bandit, safety, pip-audit | Pre-commit + CI |
| Go | gosec, nancy | Pre-commit + CI |
| Java/Kotlin | SpotBugs, OWASP Dependency Check | CI |
| Infrastructure | tfsec, checkov, kubesec | CI + Pre-deploy |

---

## 6. DOMAIN 4: INFRASTRUCTURE SECURITY

### 6.1 Network Security
| Layer | Control |
|-------|---------|
| Perimeter | WAF, DDoS protection, IP allowlisting for admin |
| Network | VPC isolation, private subnets, NAT gateways |
| Service | mTLS between services, service mesh (Istio/Linkerd) |
| Container | Non-root user, read-only filesystem, seccomp/AppArmor |
| Egress | Allowlist outbound connections, block crypto mining |

### 6.2 Container Security
```dockerfile
# Secure Dockerfile pattern
FROM node:20-alpine            # Minimal base image, pinned version
RUN addgroup -S app && adduser -S app -G app
USER app                        # Non-root user
COPY --chown=app:app . .
EXPOSE 3000
HEALTHCHECK --interval=30s CMD wget -q http://localhost:3000/health || exit 1
```

### 6.3 Cloud Security Posture
| Provider | Key Controls |
|----------|-------------|
| AWS | IAM least privilege, S3 block public access, CloudTrail enabled, KMS CMK |
| GCP | IAM conditions, VPC Service Controls, Audit Logs, CMEK |
| Azure | RBAC, NSG rules, Key Vault, Defender for Cloud |

---

## 7. DOMAIN 5: SUPPLY CHAIN SECURITY

### 7.1 SBOM (Software Bill of Materials)
```
Every project generates SBOM in CycloneDX format:
  - All direct dependencies with version + hash
  - All transitive dependencies
  - License information per dependency
  - CVE status per dependency
  - Generated on: every build
  - Stored in: .cosca/security/sbom.json
```

### 7.2 Dependency Verification
| Check | Frequency | Action on Failure |
|-------|-----------|------------------|
| CVE scan (critical) | Every commit + daily | Block merge, force upgrade |
| CVE scan (high) | Daily | Block release, schedule fix |
| License compliance | Every build | Warn if copyleft in proprietary project |
| Version pinning | Every commit | Block unpinned versions |
| Provenance verification | Every install | Block unsigned packages |
| Typosquatting detection | Weekly | Alert Security Chief |

### 7.3 Signed Commits
```
All Cosca-generated commits must be:
  - GPG signed (verified by GitHub/GitLab)
  - Have verified author email
  - Follow conventional commits format
  - Include workflow ID in commit message footer
```

---

## 8. DOMAIN 6: AI/ML SECURITY

### 8.1 Prompt Injection Defenses
| Attack Vector | Defense |
|--------------|---------|
| Direct injection ("ignore previous instructions") | Input sanitization, system prompt hardening, output validation |
| Indirect injection (poisoned data in context) | Context validation, sandboxed execution |
| Jailbreaking | Prompt template with role enforcement, input/output guardrails |
| Data exfiltration via prompt | Output filtering, no secrets in context, token limit enforcement |
| Model inversion | Rate limiting, output monitoring, differential privacy |

### 8.2 AI Output Validation
```
Before AI output reaches user:
  1. Validate format (JSON, code, text)
  2. Scan for secrets (regex patterns)
  3. Scan for malicious code (eval, exec, system calls)
  4. Validate against expected schema
  5. Sanitize for XSS if rendered in UI
  6. Log all AI outputs for audit
```

### 8.3 Model Security
| Concern | Mitigation |
|---------|-----------|
| Model poisoning | Use only signed models from trusted sources |
| Training data leakage | Data anonymization before training |
| Adversarial inputs | Input validation, anomaly detection |
| Model theft | API rate limiting, watermarking |
| Cost abuse | Token budgets per agent, cost anomaly alerts |

---

## 9. DOMAIN 7: INCIDENT RESPONSE

### 9.1 Incident Severity Classification
| Severity | Definition | Response Time | Escalation |
|----------|-----------|---------------|------------|
| **P0 - Critical** | Active breach, data exfiltration, system compromise | < 15 min | Executive Council + all hands |
| **P1 - High** | Vulnerability exploitable, service down, secrets leaked | < 1 hour | Security Council |
| **P2 - Medium** | Non-critical CVE, suspicious activity, policy violation | < 4 hours | Security Chief |
| **P3 - Low** | Minor misconfiguration, outdated dependency | < 24 hours | Respective Chief |

### 9.2 Incident Response Playbook
```
DETECT → Alert from Monitoring Engine or Security scan
  ↓
TRIAGE → Security Chief classifies severity (P0-P3)
  ↓
CONTAIN → Isolate affected system, rotate secrets, block attacker IP
  ↓
ERADICATE → Remove root cause, patch vulnerability, verify fix
  ↓
RECOVER → Restore from clean backup, verify integrity, resume service
  ↓
LEARN → Post-mortem documented in knowledge/incidents/
  ↓
IMPROVE → Update policies, add detection rules, harden defenses
```

### 9.3 Incident Response Team
| Role | Primary | Secondary |
|------|---------|-----------|
| Incident Commander | Security Chief | CTO |
| Technical Lead | DevOps Chief | Backend Chief |
| Communications | CEO | Documentation Chief |
| Legal/Compliance | Compliance Engine | Security Chief |
| Forensics | Security Engineer (specialist) | AI Chief |

---

## 10. DOMAIN 8: COMPLIANCE AUTOMATION

### 10.1 Continuous Compliance Monitoring
| Framework | Checks | Frequency | Evidence |
|-----------|--------|-----------|----------|
| SOC2 | Access reviews, change management, risk assessment | Monthly | Audit logs + Compliance Engine reports |
| GDPR | Data inventory, consent records, DSR handling | Monthly | Data map + DSR log |
| HIPAA | PHI access logs, encryption verification, BAA tracking | Weekly | Access logs + encryption status |
| PCI-DSS | Card data scan, network segmentation, access control | Weekly | PCI scan results + network diagram |
| ISO 27001 | ISMS review, control effectiveness, risk treatment | Quarterly | Control matrix + risk register |

### 10.2 Audit Trail Requirements
```
Every security-relevant event must be logged:
  - Who (agent ID)
  - What (action)
  - When (timestamp with timezone)
  - Where (resource, IP)
  - Result (success/failure)
  - Context (workflow ID, session ID)
  
  Logs must be:
  - Immutable (append-only, no deletion)
  - Tamper-evident (hash chain)
  - Retained for minimum 1 year
  - Searchable within 5 seconds
```

---

## 11. SECURITY COUNCIL AUTHORITY

### 11.1 Decisions Requiring Security Council Approval
- New AI provider integration
- Change to encryption standards
- Secrets management policy changes
- Third-party SDK/plugin approval
- Penetration test scope and findings
- Compliance framework adoption
- Incident severity classification (P0/P1)
- Security baseline changes

### 11.2 Security Veto Power
The Security Chief (or Security Council) can veto:
- Any release with critical/high CVEs
- Any deployment to production without security review
- Any code merge with hardcoded secrets
- Any third-party integration without security assessment
- Any architecture change without threat model

---

## 12. SECURITY METRICS & REPORTING

### 12.1 Key Security Metrics
| Metric | Target | Measurement |
|--------|--------|-------------|
| Mean Time to Detect (MTTD) | < 1 hour (P0), < 24 hours (P1) | Incident timestamps |
| Mean Time to Respond (MTTR) | < 4 hours (P0), < 48 hours (P1) | Incident resolution time |
| Vulnerability remediation | Critical: 24h, High: 7d, Medium: 30d | CVE tracking |
| Secrets in code | 0 (zero tolerance) | Pre-commit scan |
| Dependency health | 0 critical/high CVEs | Daily audit |
| Security review coverage | 100% of PRs | Review tracking |
| Penetration test cadence | Quarterly | Test reports |
| Security training | Annual for all agent types | Learning Engine |

### 12.2 Security Dashboard
```
┌─────────────────────────────────────────────────────┐
│              SECURITY POSTURE DASHBOARD               │
├─────────────────────────────────────────────────────┤
│ STATUS: 🟢 SECURE    LAST INCIDENT: 15 days ago      │
│                                                       │
│ VULNERABILITIES                                       │
│ Critical: 0    High: 0    Medium: 3    Low: 12        │
│                                                       │
│ COMPLIANCE                                            │
│ SOC2: ✅    GDPR: ✅    HIPAA: ✅    PCI: ✅            │
│                                                       │
│ ACTIVE DEFENSES                                        │
│ SAST: ✅    DAST: ✅    SCA: ✅    WAF: ✅              │
│ Secrets Scan: ✅    Container Scan: ✅                  │
│                                                       │
│ RECENT EVENTS                                         │
│ ✅ Dependency audit passed — 2 min ago                │
│ ✅ Secrets scan clean — 15 min ago                    │
│ ⚠️ Medium CVE in lodash — patching scheduled          │
│ ✅ Access review completed — 1 day ago                │
└─────────────────────────────────────────────────────┘
```

---

## 13. SECURITY CHECKLIST — Every Project

Before any project goes to production:
- [ ] Security baseline applied (project-init Step 7)
- [ ] .gitignore covers: .env, secrets, credentials, tokens
- [ ] No hardcoded secrets (verified by scan)
- [ ] All dependencies audited (0 critical/high CVEs)
- [ ] Authentication enforced on all endpoints
- [ ] Authorization checked on all protected resources
- [ ] Rate limiting configured
- [ ] CSP headers set
- [ ] HTTPS enforced (HSTS)
- [ ] Input validation on all external inputs
- [ ] Output encoding on all outputs
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention (context-appropriate encoding)
- [ ] CSRF protection on state-changing operations
- [ ] Docker runs as non-root user
- [ ] SBOM generated and verified
- [ ] Threat model documented (for P0/P1 features)
- [ ] Incident response runbook ready
- [ ] Audit trail configured and tested
- [ ] Penetration test passed (quarterly)

---

## RELATED
- [Security Chief](departments/security/SKILL.md) — Security strategy and oversight
- [Secrets Engine](engines/secrets/SKILL.md) — Credential management
- [Identity Engine](engines/identity/SKILL.md) — Auth and access control
- [Compliance Engine](engines/compliance/SKILL.md) — Regulatory compliance
- [Policy Engine](engines/policy/SKILL.md) — Security policies (POL-SEC-*)
- [QUALITY_GATES.md](QUALITY_GATES.md) — Gate 2.3 Security checks
- [COUNCILS.md](councils/COUNCILS.md) — Security Council authority
- [ENTERPRISE_REDUNDANCY.md](ENTERPRISE_REDUNDANCY.md) — Circuit breakers and recovery
- [PROVIDER_INTERFACE.md](PROVIDER_INTERFACE.md) — AI provider security
- [project-init workflow](workflows/project-init.md) — Security Baseline (Step 7)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Complete cybersecurity framework: 8 domains, Zero Trust, defense-in-depth, OWASP coverage, supply chain, AI security, incident response, compliance automation |
