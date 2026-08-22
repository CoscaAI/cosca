# API Reference: Middleware

> **Status**: active | **Owner**: Backend Chief | **Last Updated**: 2026-07-28 | **Version**: 1.4.0-dev

## Middleware Chain

The API server applies middleware in a specific order, forming a layered request pipeline:

```
Request
  │
  ▼
┌─────────────────────────────────┐
│  SecurityHeadersMiddleware       │  ← OUTERMOST (applied last)
│  CSP, HSTS, X-Frame-Options...  │
├─────────────────────────────────┤
│  auth.Middleware                 │  ← Authentication
│  JWT/API Key/Cookie → Claims    │
├─────────────────────────────────┤
│  CSRFMiddleware                  │  ← Double-submit cookie
│  Mutating requests only         │
├─────────────────────────────────┤
│  RateLimitMiddleware             │  ← Token bucket per IP
│  Login: 5 RPM | General: 100 RPM│
├─────────────────────────────────┤
│  CORSMiddleware                  │  ← Cross-origin requests
├─────────────────────────────────┤
│  LoggingMiddleware               │  ← Structured logging
├─────────────────────────────────┤
│  User-registered middleware      │  ← Via s.Use() (optional)
├─────────────────────────────────┤
│  http.ServeMux                   │  ← Route matching
└─────────────────────────────────┘
  │
  ▼
Handler
```

---

## Security Headers

**File:** `api/middleware/security.go`

### Static Headers

| Header | Value |
|--------|-------|
| `X-Frame-Options` | `DENY` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` |
| `X-Permitted-Cross-Domain-Policies` | `none` |

### Content Security Policy (CSP)

```
default-src 'self';
script-src 'self' 'unsafe-inline' 'unsafe-eval';
style-src 'self' 'unsafe-inline';
connect-src 'self' http://localhost:14120 ws://localhost:14120;
frame-src 'none';
object-src 'none'
```

- `connect-src` configurable via `COSCA_CSP_CONNECT_SRC` env var (default: `'self' http://localhost:14120 ws://localhost:14120`)

### HSTS

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

- Applied only when TLS is detected OR `COSCA_FORCE_HSTS=true`
- TLS detection: checks `TLS` field on request or `X-Forwarded-Proto: https`

---

## CORS

**File:** `api/middleware/cors.go`

### Configuration

```go
CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler
```

- Wildcard `"*"` or comma-separated origins (e.g., `"http://localhost:3000,https://app.example.com"`)
- With wildcard: `Access-Control-Allow-Origin: *`
- With specific origins: echoes request origin if in allowed list

### Allowed Headers

```
Content-Type, Authorization, X-Request-ID, X-Cosca-Scope, X-CSRF-Token, Accept, Origin
```

### Allowed Methods

```
GET, POST, PUT, DELETE, PATCH, OPTIONS
```

### Exposed Headers

```
X-Request-ID, X-Response-Time, Content-Length
```

### Preflight

- `OPTIONS` requests return `204 No Content`
- `Access-Control-Max-Age: 86400` (24 hours)

### Credentials

`Access-Control-Allow-Credentials: true` only for non-wildcard origins.

---

## CSRF Protection

**File:** `api/middleware/csrf.go`

### Pattern: Double-Submit Cookie

1. Server sets `csrf_token` cookie (non-HttpOnly, JS-readable, SameSite=Strict)
2. Client reads cookie value and sends it as `X-CSRF-Token` header
3. Server compares cookie value vs header value using constant-time comparison

### Protected Methods

| Method | Checked? |
|--------|----------|
| `POST`, `PUT`, `DELETE`, `PATCH` | ✅ Checked |
| `GET`, `HEAD`, `OPTIONS` | ❌ Skipped |

### Skipped

- Public paths (login, health, etc.)
- API Key requests (authenticated via `X-API-Key` header)
- Non-browser clients (no cookie present)

### Token Generation

```go
GenerateCSRFToken() (string, error)
```

- 32 random bytes → 64 hex characters
- Cookie: `HttpOnly=false`, `SameSite=Strict`, `MaxAge=86400` (24h)
- `Secure` auto-detected from TLS or `X-Forwarded-Proto`

### Validation

Uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks:
```go
if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
    // reject
}
```

---

## Rate Limiting

**File:** `api/middleware/ratelimit.go`

### Algorithm: Token Bucket

Per-IP, in-memory, fractional tokens for smooth refill.

### Dual-Rate Configuration

| Path | Rate | Env Var |
|------|------|---------|
| `/v1/auth/login` | 5 RPM | `COSCA_RATE_LIMIT_LOGIN_RPM` |
| All other paths | 100 RPM | `COSCA_RATE_LIMIT_RPM` |

**Burst:** Defaults to same as rate (`COSCA_RATE_LIMIT_BURST`)

### IP Extraction

1. `X-Forwarded-For` header — leftmost address
2. Fallback: `RemoteAddr` (port stripped)

### 429 Response

```json
{
  "error": "rate limit exceeded",
  "message": "Too many requests. Please try again later."
}
```

Headers: `Retry-After: <seconds>`

### Cleanup

Background goroutine runs every minute, evicting entries with no activity in the last hour.

---

## Logging

**File:** `api/middleware/logging.go`

### Function

```go
LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler
```

### Logged Fields

| Field | Source |
|-------|--------|
| `method` | HTTP method |
| `path` | Request path |
| `status` | Response status code |
| `duration` | Request duration |
| `remote_addr` | Client IP |
| `response_bytes` | Response body size |
| `query` | Query string (if present) |

### Log Level by Status

| Status | Level | Message |
|--------|-------|---------|
| ≥ 500 | ERROR | `server error` |
| ≥ 400 | WARN | `client error` |
| < 400 | INFO | `request completed` |

### Response Writer Wrapping

Uses a custom `responseWriter` that captures status code and bytes written for logging after the handler completes.

---

## Authentication Middleware

**File:** `api/auth/oidc.go` (misnamed — this is the auth middleware, not OIDC)

### Function

```go
Middleware(secret []byte, publicPaths []string, apiKeyStore *APIKeyStore) func(http.Handler) http.Handler
```

### Auth Priority

1. `X-API-Key` header → API key hash lookup → synthetic JWT claims
2. `cosca_access_token` cookie → JWT validation → claims
3. `Authorization: Bearer <token>` header → JWT validation → claims

### Public Path Bypass

These paths skip authentication:
`/health`, `/ready`, `/v1/auth/login`, `/v1/auth/refresh`, `/v1/auth/logout`, `/v1/csrf-token`

---

## RBAC Middleware

**File:** `api/auth/rbac.go`

### Function

```go
RequireRole(role Role) func(http.Handler) http.Handler
```

### Behavior

1. Extract `Claims` from context → **401** if missing
2. If `claims.Role == "admin"` → immediately allow (bypass)
3. Compare ranks: `admin(3) > editor(2) > viewer(1)`
4. Insufficient rank → **403** with JSON error body
5. Sufficient rank → call next handler

### Usage Example

```go
adminOnly := auth.RequireRole(auth.RoleAdmin)
mux.Handle("GET /v1/users", adminOnly(http.HandlerFunc(users.List)))
```

---

## Error Responses

| Status | Condition | Body |
|--------|-----------|------|
| 401 | Missing/invalid token | `{"error": "unauthorized"}` |
| 401 | Expired token | `{"error": "token expired"}` |
| 403 | Insufficient role | `{"error": "forbidden"}` |
| 403 | CSRF mismatch | `{"error": "csrf token mismatch"}` |
| 429 | Rate limit exceeded | `{"error": "rate limit exceeded"}` |
| 429 | Account locked | `{"error": "account locked", "retry_after": <seconds>}` |

---

> **Related**: [Authentication Reference](auth.md) | [API Overview](overview.md) | [Runtime Configuration](../runtime/configuration.md)
