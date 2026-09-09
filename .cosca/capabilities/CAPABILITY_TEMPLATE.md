# CAPABILITY TEMPLATE — Standard Capability Contract

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-12

## PURPOSE
Every Capability in the Cosca ecosystem must follow this standardized contract. Capabilities are the atomic units of responsibility — they define WHAT can be done, not WHO does it. Departments and Engines consume Capabilities.

## CAPABILITY CONTRACT (Mandatory Fields)

```markdown
# CAPABILITY: capability-name

> **Version**: X.Y.Z | **Status**: draft|active|deprecated | **Owner**: Department/Engine | **Last Updated**: YYYY-MM-DD

## PURPOSE
One paragraph describing what this capability enables.

## CATEGORY
[architecture|engineering|quality|security|infrastructure|ai|data|platform|governance|product|operations|integration]

## INPUTS
| Input | Type | Required | Description |
|-------|------|----------|-------------|

## OUTPUTS
| Output | Type | Description |
|--------|------|-------------|

## CONTRACT
| Operation | Input | Output | Errors | SLA |
|-----------|-------|--------|--------|-----|

## PROVIDERS
| Provider Type | Department | Engine | Priority |
|--------------|------------|--------|----------|

## DEPENDENCIES
| Capability | Why |
|-----------|-----|

## CONSTRAINTS
- Constraint 1

## QUALITY CRITERIA
- [ ] Criterion 1

## METRICS
| Metric | Target | Measurement |
|--------|--------|-------------|

## VERSION HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | YYYY-MM-DD | Author | Initial capability |
```

## CATEGORIES

| Category | Description | Example Capabilities |
|----------|-------------|---------------------|
| architecture | System design, patterns, boundaries | API Design, Event-Driven Architecture, CQRS |
| engineering | Implementation, code, services | Backend Development, Frontend Development, Database |
| quality | Testing, review, standards | Unit Testing, Code Review, Performance Testing |
| security | Protection, compliance, secrets | Authentication, Authorization, Vulnerability Scanning |
| infrastructure | Cloud, networking, scaling | Container Orchestration, Auto-Scaling, CDN |
| ai | ML, LLM, prompts, RAG | Prompt Engineering, Embeddings, Vector Search |
| data | Storage, caching, messaging | Relational Database, Caching, Message Queue |
| platform | Runtime, SDK, plugins, marketplace | Plugin Management, SDK Generation, Provider Management |
| governance | Policies, versioning, lifecycle | Decision Records, Compliance Audits, Deprecation |
| product | Requirements, scope, UX | Feature Specification, User Research, Prototyping |
| operations | Deploy, monitor, recover | CI/CD, Deployment, Disaster Recovery |
| integration | External APIs, webhooks | Third-Party API Integration, Webhook Management |

## RELATED
- [CAPABILITY_CATALOG.md](CAPABILITY_CATALOG.md) — Complete catalog of all capabilities
- [AGENT_DNA.md](../AGENT_DNA.md) — Agent contract standard
- [CONVENTIONS.md](../CONVENTIONS.md) — File format standards
- [GOVERNANCE.md](../GOVERNANCE.md) — Versioning and lifecycle

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial capability template — 12 sections, 12 categories |
