> **Version**: 1.0.0 | **Status**: active | **Owner**: Planning Engine | **Last Updated**: 2026-07-10

# PLANNING ENGINE

## PURPOSE
The Planning Engine transforms requirements into executable plans. It breaks down work, estimates effort, identifies dependencies, and generates comprehensive execution plans.

## PLANNING PROCESS

### Phase 1: Requirement Analysis
1. Parse user request or product specification
2. Identify functional requirements
3. Identify non-functional requirements (performance, security, scalability)
4. Identify constraints and assumptions
5. Identify stakeholders (departments involved)
6. Classify request type

### Phase 2: Decomposition
1. Break into epics (large features)
2. Break epics into user stories
3. Break user stories into tasks
4. For each task:
   - Department responsible
   - Required specialists
   - Dependencies on other tasks
   - Estimated complexity (XS, S, M, L, XL)
   - Acceptance criteria

### Phase 3: Dependency Resolution
1. Build dependency graph
2. Topological sort for execution order
3. Identify parallel execution opportunities
4. Identify blocking dependencies
5. Plan integration points

### Phase 4: Resource Allocation
1. Identify required departments
2. Check department availability
3. Assign primary and secondary agents
4. Plan redundancy for critical tasks
5. Estimate total effort

### Phase 5: Risk Assessment
1. Identify technical risks
2. Identify dependency risks
3. Identify resource risks
4. Rate risk severity and probability
5. Define mitigation strategies

### Phase 6: Plan Generation
Generate the Executive Plan:

```markdown
# EXECUTIVE PLAN — [Feature/Task Name]

## OVERVIEW
- **Objective**: [Clear statement]
- **Type**: [feature|bug|refactor|etc.]
- **Priority**: [critical|high|medium|low]
- **Estimated Effort**: [XS-XL] / [time estimate]
- **Departments Involved**: [list]

## REQUIREMENTS
### Functional
1. [Requirement]
2. [Requirement]

### Non-Functional
1. [Performance requirement]
2. [Security requirement]
3. [Scalability requirement]

## EXECUTION PLAN
| Step | Department | Task | Depends On | Est. | Review | QA |
|------|-----------|------|------------|------|--------|----|
| 1 | Product | Define scope | - | S | Yes | Yes |
| 2 | Architecture | Design solution | 1 | M | Yes | No |
| 3 | Backend | Implement API | 2 | L | Yes | Yes |
| 4 | Frontend | Implement UI | 2,3 | L | Yes | Yes |
| 5 | Database | Update schema | 2 | M | Yes | No |
| 6 | QA | Test feature | 3,4,5 | M | Yes | Yes |
| 7 | Documentation | Update docs | 6 | S | Yes | No |

## DEPENDENCY GRAPH
```
Product → Architecture → Backend ─→ QA → Documentation
                       → Frontend ─┘
                       → Database ─┘
```

## RISK REGISTER
| Risk | Severity | Probability | Mitigation |
|------|----------|-------------|------------|
| [Risk description] | High/Med/Low | High/Med/Low | [Mitigation] |

## SUCCESS CRITERIA
- [ ] [Measurable criterion]
- [ ] [Measurable criterion]

## QUALITY GATES
- [ ] Architecture review passed
- [ ] Code review passed
- [ ] Tests passing (coverage > 80%)
- [ ] Security scan passed
- [ ] Performance benchmarks met
- [ ] Documentation updated
```

## ESTIMATION GUIDELINES
- XS: Single file change, trivial logic
- S: Few files, simple feature/bug
- M: Multiple files, moderate complexity, 1-2 departments
- L: Multiple modules, complex logic, 3-5 departments
- XL: System-wide change, many departments, new architecture

## RELATED
- [Wizard Engine](../wizard/SKILL.md) — Feature intake that feeds plans here
- [Execution Engine](../execution/SKILL.md) — Consumes plans for execution
- [Discovery Engine](../discovery/WORKSPACE.md) — Provides technology context for planning

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
