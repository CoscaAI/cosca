# API Client Generation

> **Version**: 1.0.0 | **Status**: active | **Owner**: SDK Chief | **Last Updated**: 2026-07-27

## Purpose
Generate typed API clients from the Cosca OpenAPI specification.

## Process
1. Load OpenAPI spec from api/rest/openapi.yaml.
2. Validate spec: all endpoints have request/response schemas, error responses defined.
3. Generate Go client: typed structs, client methods, error handling.
4. Generate TypeScript client: typed interfaces, async methods, error classes.
5. Add authentication: API key header injection, JWT token management.
6. Run generated tests against live server to verify correctness.

## Success Criteria
- Generated code compiles without manual edits
- All endpoints covered with typed interfaces
- Auth works: API key and JWT flows tested
