---
name: api-documentation
description: Use when the user asks to generate API documentation from OpenAPI specs, code annotations, or contract definitions.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Documentation Chief | **Last Updated**: 2026-07-23

# API DOCUMENTATION SKILL

## Description
Generate comprehensive API documentation from OpenAPI specs, code annotations, or contract definitions. Creates developer-friendly docs with examples, guides, and reference sections.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| spec_source | Yes | OpenAPI spec, GraphQL schema, or code annotations path |
| doc_format | Yes | `markdown`, `html`, `openapi-ui`, `graphql-playground` |
| include_examples | No | Generate code examples in target languages (default: true) |
| target_languages | No | Languages for examples: `curl`, `python`, `typescript`, `go`, `java` |

## Outputs
| Output | Description |
|--------|-------------|
| API documentation | Complete API reference docs |
| Getting started guide | Quickstart with authentication |
| Code examples | Examples in target languages |
| Changelog | API version history |

## Documentation Sections
- API overview and base URL
- Authentication (API key, OAuth, JWT)
- Endpoint reference with request/response schemas
- Error codes and error response format
- Rate limiting and throttling
- Pagination and filtering
- Code examples in multiple languages
- SDK/client library references
- Migration guides for version upgrades
- FAQ and troubleshooting

## Process
1. Parse API specification
2. Extract endpoints, schemas, and security definitions
3. Generate endpoint reference documentation
4. Create request/response examples
5. Generate code snippets for target languages
6. Build getting started guide
7. Compile changelog from spec version history
8. Review for completeness and accuracy
9. Publish to documentation portal

## Success Criteria
- [ ] All endpoints documented with request/response schemas
- [ ] Authentication documentation complete
- [ ] Code examples in requested languages
- [ ] Error codes documented
- [ ] Rate limiting documented
- [ ] Changelog generated
- [ ] Getting started guide created

## Related
- [Documentation Chief](../../departments/documentation/SKILL.md)
- [API Chief](../../departments/api/SKILL.md)
- [OpenAPI Validation](../../skills/api/OPENAPI_VALIDATION.md)
- [Documentation Update](./DOCUMENTATION_UPDATE.md)
- [SDK Chief](../../departments/sdk/SKILL.md)
