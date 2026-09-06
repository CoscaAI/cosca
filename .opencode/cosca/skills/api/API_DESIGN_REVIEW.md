---
name: api-design-review
description: Use when the user asks to review an API design before implementation for consistency, organizational standards, and backward compatibility.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: API Chief | **Last Updated**: 2026-07-23

# API DESIGN REVIEW SKILL

## Description
Use this skill to review API designs before implementation. Ensures API contracts are consistent, follow organizational standards, are secure, and meet consumer needs. API-first approach validates contracts before code is written.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| api_spec_path | Yes | Path to API specification (OpenAPI, GraphQL SDL, Protobuf) |
| review_scope | Yes | `full`, `contract-only`, `security-only`, `consumer-impact` |
| consumers | No | Known consumers of the API (frontend, mobile, third-party) |
| architecture_adrs | No | Related ADRs for architecture context |

## Outputs
| Output | Description |
|--------|-------------|
| Review report | Complete API design review with findings |
| Decision | Approved, Changes-Required, or Rejected |
| Recommendations | Specific improvements with rationale |

## Review Dimensions

### Contract Completeness
- All endpoints documented with request/response schemas
- All error responses documented (4xx, 5xx)
- Pagination defined for list endpoints
- Rate limiting documented
- Authentication/authorization defined

### RESTful Design
- Resource naming follows conventions (plural nouns, kebab-case)
- HTTP methods used correctly (GET for read, POST for create, PUT/PATCH for update, DELETE for delete)
- Status codes appropriate for each operation
- HATEOAS links where applicable
- Idempotency for PUT and DELETE

### Security
- Authentication required (no anonymous write endpoints)
- Authorization scopes/permissions defined
- Input validation documented
- CORS configuration specified
- Rate limiting and throttling defined

### Consumer Experience
- Consistent error format across all endpoints
- Meaningful error messages with error codes
- Request/response examples provided
- Deprecation policy documented
- Versioning strategy clear

## Process
1. Load API specification and parse contracts
2. Validate OpenAPI 3.x compliance (or GraphSDL/Protobuf)
3. Review RESTful naming and HTTP method usage
4. Assess security completeness
5. Evaluate consumer experience and consistency
6. Check versioning and deprecation strategy
7. Generate review report with findings and recommendation
8. Document decision in ADR if breaking changes

## Success Criteria
- [ ] Contract validated for completeness
- [ ] RESTful conventions followed
- [ ] Security patterns implemented
- [ ] Consumer experience considered
- [ ] Versioning strategy clear
- [ ] Review decision documented

## Related
- [API Chief](../../departments/api/SKILL.md)
- [API Audit](../../skills/api/API_AUDIT.md)
- [OpenAPI Validation](../../skills/api/OPENAPI_VALIDATION.md)
- [workflows/api-design-review.md](../../workflows/api-design-review.md)
- [Backend Chief](../../departments/backend/SKILL.md)
