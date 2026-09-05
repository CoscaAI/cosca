---
type: pattern
key: api-patterns
tags: [pattern, api, rest, design]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: API Chief
category: design
confidence: 0.95
times_used: 20
times_succeeded: 19
---

# API Design Patterns

## Pattern: Pagination with Cursor
- **Context**: List endpoints returning large datasets
- **Solution**: Cursor-based pagination (opaque cursor, limit parameter)
- **Benefits**: Stable under data changes, no offset drift
- **Example**: `GET /api/users?cursor=abc123&limit=20`

## Pattern: Error Envelope
- **Context**: Consistent error responses across all APIs
- **Solution**: Standard error envelope with code, message, details
- **Example**: `{"error": {"code": "VALIDATION_ERROR", "message": "Invalid email", "details": [{"field": "email", "reason": "format"}]}}`

## Pattern: API Versioning (URL)
- **Context**: Backward-incompatible API changes
- **Solution**: Major version in URL path (/v1/, /v2/)
- **Benefits**: Clear, cacheable, discoverable
- **Deprecation**: 6-month overlap, sunset header in responses
