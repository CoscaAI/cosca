# TEMPLATE: SDK / Client Library

> **Version**: 1.0.0 | **Status**: active | **Owner**: SDK Chief | **Last Updated**: 2026-07-23

## DOMAIN
Software Development Kits (SDKs) and client libraries for consuming platform APIs in multiple programming languages.

## RECOMMENDED STACK
| Layer | TypeScript | Python | Go | Java |
|-------|-----------|--------|-----|------|
| HTTP Client | axios / fetch | httpx / aiohttp | net/http | OkHttp |
| Serialization | JSON (native) | Pydantic | encoding/json | Jackson |
| Auth | JWT / OAuth2 | JWT / OAuth2 | JWT / OAuth2 | JWT / OAuth2 |
| Testing | Vitest | pytest | Go test | JUnit |
| Build | tsup / rollup | setuptools / poetry | Go build | Maven / Gradle |
| Docs | TypeDoc | Sphinx | godoc | Javadoc |
| Registry | npm | PyPI | Go proxy | Maven Central |

## MODULE STRUCTURE (TypeScript example)
```
sdk-name/
├── src/
│   ├── client/                # HTTP client and transport
│   │   ├── http-client.ts
│   │   ├── auth.ts
│   │   └── errors.ts
│   ├── resources/             # API resource wrappers
│   │   ├── users.ts
│   │   ├── projects.ts
│   │   └── deployments.ts
│   ├── types/                 # TypeScript types
│   │   ├── api.ts
│   │   ├── models.ts
│   │   └── requests.ts
│   └── index.ts               # Public API
├── tests/
│   ├── unit/
│   ├── integration/
│   └── fixtures/              # Test fixtures
├── examples/                  # Usage examples
│   ├── basic-usage.ts
│   └── advanced-usage.ts
├── docs/
│   ├── API.md
│   ├── GETTING_STARTED.md
│   └── MIGRATION.md
├── scripts/
│   ├── build.ts
│   └── release.ts
├── package.json
├── tsconfig.json
└── README.md
```

## KEY FEATURES
- Multi-language SDK support (TS, Python, Go, Java)
- Consistent API across all languages
- Auto-generated from OpenAPI specs (optional)
- Comprehensive error handling with typed errors
- Retry with exponential backoff
- Pagination support for list endpoints
- Streaming support for real-time APIs
- Authentication (API key, OAuth2, JWT)
- Telemetry (opt-in, configurable)
- Thorough documentation with runnable examples

## ARCHITECTURE NOTES
- SDK is thin wrapper over HTTP API — no business logic
- All languages follow same naming conventions
- Error types are consistent across languages
- Retry strategy configurable by consumer
- Pagination uses async iterators/generators
- Default timeout of 30s, configurable per request
- Logging uses standard language logging
- SDK is stateless (auth provided per request or via client)

## RELATED
- [SDK Chief](../../departments/sdk/SKILL.md)
- [API Chief](../../departments/api/SKILL.md)
- [CLI Template](../cli/TEMPLATE.md)
- [API Template](../api/TEMPLATE.md)
