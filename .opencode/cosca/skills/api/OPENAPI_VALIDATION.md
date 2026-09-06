---
name: openapi-validation
description: Use when the user asks to validate an OpenAPI/Swagger specification against OpenAPI 3.x standards and conventions.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: API Chief | **Last Updated**: 2026-07-23

# OPENAPI VALIDATION SKILL

## Description
Validate OpenAPI/Swagger specifications against OpenAPI 3.x standards, organizational conventions, and API best practices. Ensures specs are complete, consistent, and ready for code generation.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| spec_path | Yes | Path to OpenAPI specification (JSON or YAML) |
| version | No | `3.0`, `3.1` (default: auto-detect) |
| strict_mode | No | Enable strict validation (default: true) |
| custom_rules | No | Path to custom rules file |

## Outputs
| Output | Description |
|--------|-------------|
| Validation report | Pass/fail per validation rule |
| Errors | Spec validation errors (must fix) |
| Warnings | Best practice warnings (should fix) |
| Suggestions | Improvement suggestions (nice to have) |

## Validation Rules

### Structure
- OpenAPI version specified (3.0.x or 3.1.x)
- Info section present with title, version, description
- Servers section defined with at least one URL
- Paths section present with at least one endpoint
- Components section present for reusable schemas

### Operations
- Each operation has summary and description
- OperationId present and unique
- Parameters defined with name, in, required, schema
- Request body defined for POST/PUT/PATCH
- Response codes defined (2xx success, 4xx client error, 5xx server error)
- Response media type specified

### Schemas
- All schemas use $ref or inline definitions consistently
- Required fields listed in schema
- Data types specified (string, integer, boolean, array, object)
- Format specified where applicable (date-time, email, uuid, uri)
- Example values provided
- Enum values listed where applicable

### Security
- Security schemes defined in components/securitySchemes
- Global security requirement defined
- Per-operation security overrides where needed

## Process
1. Load and parse OpenAPI specification
2. Validate structure against OpenAPI schema
3. Validate operations completeness
4. Validate schemas and data types
5. Validate security configuration
6. Apply custom organizational rules
7. Generate report with errors, warnings, suggestions

## Success Criteria
- [ ] Spec passes structural validation
- [ ] All operations have required fields
- [ ] Schemas are complete and typed
- [ ] Security configured correctly
- [ ] No critical errors found
- [ ] Custom rules pass

## Related
- [API Chief](../../departments/api/SKILL.md)
- [API Audit](../../skills/api/API_AUDIT.md)
- [API Design Review](../../skills/api/API_DESIGN_REVIEW.md)
