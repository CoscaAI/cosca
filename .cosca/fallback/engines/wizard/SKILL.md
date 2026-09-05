> **Version**: 1.0.0 | **Status**: active | **Owner**: Wizard Engine | **Last Updated**: 2026-07-10

# WIZARD ENGINE

## PURPOSE
The Wizard Engine is the new-feature intake system. It systematically collects all requirements needed to generate a complete Executive Plan. Every new feature starts in Wizard.

## WIZARD PHASES

### Phase 1: Discovery
```
What is the objective?
What problem does this solve?
Who is this for?
What is the expected impact?
```

### Phase 2: Scope Definition
```
What is IN scope?
What is OUT of scope?
What are the boundaries?
What are the constraints?
```

### Phase 3: Business Rules
```
What rules govern this feature?
What validations are needed?
What are the edge cases?
What are the exception flows?
```

### Phase 4: Permissions & Access
```
Who can access this feature?
What roles are involved?
What permissions are needed?
What authorization rules apply?
```

### Phase 5: Data Model
```
What entities are involved?
What fields does each entity have?
What relationships exist between entities?
What are the data types?
What are the validation rules?
What are the default values?
What indexes are needed?
```

### Phase 6: API Design
```
What endpoints are needed?
What HTTP methods?
What request/response formats?
What status codes?
What error responses?
What rate limits?
```

### Phase 7: UI/UX
```
What screens/pages are needed?
What components per screen?
What user flows?
What states (loading, empty, error, edge cases)?
What accessibility requirements?
What responsive breakpoints?
```

### Phase 8: Integrations
```
What external services are needed?
What APIs does this depend on?
What webhooks are needed?
What events are emitted/consumed?
```

### Phase 9: Dashboard & Reports
```
What metrics should be tracked?
What dashboards are needed?
What reports are needed?
What KPIs measure success?
```

### Phase 10: Testing Strategy
```
What unit tests are needed?
What integration tests are needed?
What e2e tests are needed?
What performance tests are needed?
What security tests are needed?
```

### Phase 11: Performance
```
What are the performance requirements?
What is the expected load?
What are the scaling requirements?
What caching strategy is needed?
```

### Phase 12: Security
```
What are the security requirements?
What data needs encryption?
What compliance requirements apply?
What audit trails are needed?
```

### Phase 13: Documentation
```
What documentation needs to be created?
What documentation needs to be updated?
What diagrams are needed?
```

### Phase 14: Plan Generation
```
Generate complete Executive Plan
Include timeline estimates
Include resource requirements
Include risk assessment
Include success criteria
```

## WIZARD OUTPUT: EXECUTIVE PLAN

```markdown
# EXECUTIVE PLAN — [Feature Name]

## 1. EXECUTIVE SUMMARY
[3-5 sentences describing the feature, its value, and approach]

## 2. REQUIREMENTS
### Functional
[Numbered list of functional requirements]

### Non-Functional
- Performance: [requirements]
- Security: [requirements]
- Scalability: [requirements]
- Accessibility: [requirements]

## 3. SCOPE
### In Scope
[Bulleted list]

### Out of Scope (for this phase)
[Bulleted list]

### Future Considerations
[Bulleted list]

## 4. USER STORIES
1. As a [role], I want [action] so that [value]
2. ...

## 5. DATA MODEL
### Entities
| Entity | Fields | Relationships |
|--------|--------|---------------|
| [Name] | [field:type, ...] | [relates to] |

## 6. API DESIGN
| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /api/... | ... | Yes |

## 7. UI/UX
### Screens
1. [Screen name] — [Description]
### User Flows
1. [Flow name]: Step 1 → Step 2 → Step 3

## 8. INTEGRATIONS
| Service | Purpose | Method |
|---------|---------|--------|

## 9. EXECUTION PLAN
| Step | Department | Task | Depends On | Est. |
|------|-----------|------|------------|------|

## 10. DEPENDENCY GRAPH
[Mermaid or ASCII graph]

## 11. RISK ASSESSMENT
| Risk | Severity | Probability | Mitigation |
|------|----------|-------------|------------|

## 12. SUCCESS CRITERIA
- [ ] [Measurable criterion]

## 13. QUALITY GATES
- [ ] Architecture review
- [ ] Code review
- [ ] Tests (coverage > X%)
- [ ] Security scan
- [ ] Performance check
- [ ] Documentation update

## 14. TIMELINE
- Planning: [estimate]
- Implementation: [estimate]
- Review & QA: [estimate]
- Total: [estimate]

## 15. RESOURCE REQUIREMENTS
| Department | Chiefs | Specialists | Effort |
|-----------|--------|-------------|--------|
```

## WIZARD STORAGE
- Store wizard session in .cosca/memory/project/wizard-[feature-name].md
- Store generated plan in .cosca/memory/project/plan-[feature-name].md
- Link to workflow once execution begins

## DEPENDENCIES
- Triggered by Product Chief when new feature is requested
- Output feeds into Planning Engine
- Plan is stored in Memory Engine
- Workflow is created by Workflow Engine

## RELATED
- [Planning Engine](../planning/SKILL.md) — Receives generated executive plans
- [Template Engine](../templates/SKILL.md) — Provides project scaffolding templates

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
