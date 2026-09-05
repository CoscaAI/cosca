# Security Chief — Evolution Example

## How the Security Chief Auto-Evolves

### Task 1: Basic Security Audit (Level 1)
```
RETRIEVE → "OWASP Top 10 Baseline" (Level 1)
APPLY   → Manual code review using OWASP checklist
RESULT  → Found 3 medium issues, 0 critical
LEARN   → Recorded: "OWASP Top 10 Baseline" technique works, 
          but manual review missed CSP header config
NEXT    → Learn CSP header auditing for Level 2
```

### Task 5: PR Security Review (Level 2)
```
RETRIEVE → "OWASP Baseline" (L1) + "govulncheck Integration" (L2) 
           + "CSP Header Audit" (L2 — learned in Task 3)
APPLY   → Automated dependency scan + manual CSP review + OWASP checklist
RESULT  → Found CSP misconfiguration (medium), caught CVE in dependency (high)
LEARN   → Combining automated + manual yields 2x more findings
NEXT    → Learn threat modeling for Level 3
```

### Task 15: Architecture Security Review (Level 3)
```
RETRIEVE → "STRIDE Threat Modeling" (L3) + "Attack Chain Analysis" (L3)
           + "Automated CVE Scanning" (L2) + "CSP/Header Audit" (L2)
APPLY   → STRIDE per subsystem → attack chain mapping → automated scan → manual review
RESULT  → Identified 3 attack chains, 1 novel (API key leakage via error messages)
LEARN   → Error message enumeration is an overlooked vector in REST APIs
NEXT    → Develop "Error Message Attack Surface Mapping" technique (Level 4)
```

### Task 30: Novel Vulnerability Research (Level 4)
```
RETRIEVE → All L3 techniques + "Error Message Attack Surface" (L4)
APPLY   → Novel technique: fuzzing error responses across all 36 endpoints
RESULT  → Discovered information disclosure pattern in 4 endpoints
LEARN   → Error response fuzzing is effective against REST APIs
NEXT    → Document as new OWASP technique, contribute to framework (Level 5)
```

### Semantic Memory After 30 Tasks
```
learnings.md: 30 entries, FTS5-indexed
tags: #owasp #xss #csp #csrf #jwt #cve #stride #fuzzing #api-security #error-leakage
level: 4 (Expert)
patterns.md: 5 reusable patterns discovered
evolution.md: progression timeline 1→2→3→4

Next task → instant semantic retrieval → applies Level 4 techniques immediately
```
