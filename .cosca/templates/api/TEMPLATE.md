# API TEMPLATE

## Domain
Pure backend API service. REST, GraphQL, or gRPC.

## Recommended Stack
- Language: TypeScript (NestJS), Python (FastAPI), Go (Gin), or Rust (Actix)
- Database: PostgreSQL + Redis
- API Style: REST + OpenAPI, GraphQL, or gRPC
- Auth: JWT with refresh tokens
- Queue: Bull, Celery, or RabbitMQ
- Observability: OpenTelemetry + structured logging

## Module Structure
```
src/
├── modules/
│   ├── auth/           # Authentication & Authorization
│   ├── users/          # User management
│   └── [domain]/       # Domain modules
├── common/
│   ├── guards/         # Auth guards
│   ├── decorators/     # Custom decorators
│   ├── filters/        # Exception filters
│   ├── interceptors/   # Logging, transform
│   ├── pipes/          # Validation pipes
│   ├── middleware/     # Custom middleware
│   └── dto/            # Shared DTOs
├── config/             # Configuration
├── database/           # Database config, migrations
└── main.ts             # Entry point
tests/
├── unit/
├── integration/
└── e2e/
docs/
├── api/                # OpenAPI/Swagger specs
└── adr/                # Architecture decisions
```

## Key Features
- REST API with OpenAPI 3.0 documentation
- JWT authentication with refresh tokens
- Role-based access control
- Input validation
- Rate limiting
- Request/Response logging
- Health checks (liveness, readiness)
- Structured error responses
- Pagination, filtering, sorting
- API versioning
- Rate limiting

## Architecture Notes
- Clean Architecture / Hexagonal Architecture
- Repository pattern for data access
- Service layer for business logic
- Middleware pipeline for cross-cutting concerns
- Environment-based configuration
- Graceful shutdown
- Comprehensive error handling
