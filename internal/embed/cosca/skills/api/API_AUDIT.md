---
name: api-audit
description: Use when the user asks to audit an API contract or its implementation for consistency, correctness, and standard compliance.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: API Chief | **Last Updated**: 2026-07-23
> 
> # API AUDIT SKILL
> 
> ## Description
> Use this skill to audit API contracts and implementations. Validates OpenAPI specifications, checks API consistency, ensures versioning compliance, and verifies security patterns.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | api_spec_path | Yes | Path to OpenAPI/Swagger/GraphQL spec |
> | api_code_path | Yes | Path to API implementation code |
> | audit_scope | Yes | `contract`, `implementation`, `security`, `full` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Audit report | API compliance report |
> | Violations list | Contract-implementation mismatches |
> | Security findings | API security vulnerabilities |
> | Recommendations | API improvement suggestions |
> 
> ## Audit Checks
> 
> ### Contract Validation
> - OpenAPI 3.x compliance
> - All endpoints documented
> - Request/response schemas defined
> - Error responses documented
> - API versioning strategy clear
> 
> ### Implementation Validation
> - Contract matches implementation
> - Status codes match specification
> - Response formats match schemas
> - Validation logic exists for required fields
> 
> ### API Security
> - Authentication required
> - Authorization checks present
> - Rate limiting configured
> - Input validation implemented
> - CORS configured properly
> - HTTPS enforced
> 
> ### API Design Quality
> - RESTful naming conventions
> - Consistent error format
> - Pagination for list endpoints
> - HATEOAS links (where applicable)
> - Idempotent PUT/DELETE operations
> 
> ## Success Criteria
> - [ ] Contract validated against OpenAPI standard
> - [ ] Implementation matches contract
> - [ ] Security patterns verified
> - [ ] Design quality assessed
> - [ ] Recommendations provided
> 
> ## Related
> - [API Chief](../../departments/api/SKILL.md)
> - [OpenAPI Validation](./OPENAPI_VALIDATION.md)
> - [API Design Review](./API_DESIGN_REVIEW.md)
> - [Backend Chief](../../departments/backend/SKILL.md)

## Process
1. **Endpoint Inventory**: Enumerate all REST endpoints from route registration (api/rest/server.go).
2. **Contract Validation**: Verify OpenAPI spec (api/rest/openapi.yaml) matches actual handler implementations.
3. **Status Code Audit**: Check every handler returns appropriate HTTP status codes (200, 201, 400, 401, 403, 404, 500).
4. **Error Format Check**: Verify all errors use consistent envelope (`{"error": "message"}`) via writeError.
5. **Auth Enforcement**: Confirm every protected endpoint has auth middleware. No unprotected state-changing operations.
6. **Input Validation**: Test boundary conditions for all query params, path params, and request bodies.
7. **Rate Limiting**: Check for rate limit headers and proper 429 responses on protected endpoints.
8. **Documentation Sync**: Ensure api-reference docs match actual endpoint behavior.
