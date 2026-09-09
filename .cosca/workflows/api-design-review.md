# WORKFLOW: api-design-review

> **Version**: 1.0.0 | **Category**: review | **Estimated Duration**: 30-60 min | **Status**: active | **Owner**: API Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Review API designs for consistency, correctness, security, and adherence to organizational standards before implementation begins. API-first approach ensures contracts are validated before code is written.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| api_spec | OpenAPI/GraphQL/Protobuf | Yes | API contract specification |
| use_cases | Document | Yes | Intended use cases and consumers |
| architecture_context | ADR | No | Related architecture decisions |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Review decision | Pass/Fail/Changes | API design review outcome |
| Review report | Document | Detailed findings and recommendations |
| Approved spec | OpenAPI/GraphQL/Protobuf | Final approved API contract |

## PRECONDITIONS
1. API contract draft exists in standard format
2. Use cases and consumers identified
3. Related architecture decisions documented

## POSTCONDITIONS
1. API contract approved and versioned
2. Review findings documented in memory
3. Breaking changes flagged for migration plan

## DEPENDENCIES
| Workflow | Reason |
|----------|--------|
| feature-development | API design feeds feature implementation |

## STEPS
### Step 1: Architecture Alignment
- **Chief**: Architecture Chief
- **Specialists**: Solutions Architect
- **Task**: Validate API follows architecture patterns and ADRs
- **Output**: Architecture alignment check

### Step 2: Contract Review
- **Chief**: API Chief
- **Specialists**: API Designer
- **Task**: Review OpenAPI/GraphQL/Protobuf for completeness, consistency, RESTful conventions
- **Output**: Contract review findings

### Step 3: Security Review
- **Chief**: Security Chief
- **Specialists**: Security Engineer
- **Task**: Review authentication, authorization, input validation, rate limiting
- **Output**: Security review findings

### Step 4: Consumer Review
- **Chief**: Backend Chief, Frontend Chief
- **Specialists**: API Consumer representatives
- **Task**: Validate API meets consumer needs and is implementable
- **Output**: Consumer feedback

### Step 5: Final Decision
- **Chief**: API Chief
- **Specialists**: —
- **Task**: Consolidate feedback, approve or request changes
- **Output**: Final review decision

## VALIDATION
1. OpenAPI spec passes validation tools
2. No breaking changes without migration plan
3. Authentication and authorization defined for all endpoints
4. Error responses documented for all endpoints
5. Rate limiting and pagination considered for list endpoints

## SUCCESS CRITERIA
- [ ] API contract approved and versioned
- [ ] Security review passed
- [ ] Architecture alignment confirmed
- [ ] Consumer feedback incorporated
- [ ] Breaking changes documented with migration plan

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Architecture misalignment | Escalate to Architecture Chief for ADR |
| Security concerns | Escalate to Security Chief for exceptions |
| Consumer requirements mismatch | Return to design phase with feedback |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |

## RELATED
- [API Chief](../departments/api/SKILL.md)
- [API Audit skill](../skills/api/API_AUDIT.md)
- [OpenAPI Validation skill](../skills/api/OPENAPI_VALIDATION.md)
- [feature-development.md](./feature-development.md)
