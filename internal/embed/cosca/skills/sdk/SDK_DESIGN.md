---
name: sdk-design
description: Use when the user asks to design the public SDK API surface for a Go or TypeScript client.
---

# SDK Design

> **Version**: 1.0.0 | **Status**: active | **Owner**: SDK Chief | **Last Updated**: 2026-07-27

## Purpose
Design idiomatic, type-safe SDKs for Go and TypeScript that mirror the Cosca REST API.

## Process
1. Study REST API endpoints from api/rest/ and OpenAPI spec.
2. Define SDK surface: client constructor, methods per resource, error types.
3. Implement Go SDK: use net/http, typed request/response structs, context support.
4. Implement TypeScript SDK: use fetch API, typed interfaces, async/await.
5. Add retry logic: exponential backoff, circuit breaker for 5xx errors.
6. Generate docs from code comments (Go: godoc, TS: typedoc).
7. Write examples for every public method.

## Success Criteria
- SDK methods cover 100% of REST API endpoints
- Type safety: zero `any` types in TypeScript, zero `interface{}` in Go
- Examples compile and run
