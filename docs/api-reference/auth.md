# API Reference: Authentication

> **Status**: active | **Owner**: Backend Chief | **Last Updated**: 2026-07-28 | **Version**: 1.4.0-dev

## Overview

Cosca uses a multi-method authentication system supporting three auth sources, with RBAC authorization and brute-force protection. Tokens are HMAC-SHA256 signed (no external JWT library), passwords are bcrypt-hashed (cost 12), and API keys are SHA-256 hashed for storage.

---

## Auth Methods

Three authentication methods are supported, checked in priority order:

| Priority | Method | Source | Use Case |
|----------|--------|--------|----------|
| 1 | API Key | `X-API-Key` header | Programmatic/automated access, CI/CD |
| 2 | Cookie | `cosca_access_token` httpOnly cookie | Browser clients (XSS-resistant) |
| 3 | Bearer Token | `Authorization: Bearer <token>` header | API clients, curl, SDKs |

The first matching method authenticates the request. All three result in JWT claims stored in the request context.

---

## JWT Tokens

### Algorithm

- **HMAC-SHA256** (custom implementation, no `jwt-go` dependency)
- Token format: `header.payload.signature` (base64url, no padding)
- Signature verification uses constant-time comparison (`crypto/hmac.Equal`)

### Claims

```json
{
  "sub": "user-id",
  "username": "admin",
  "role": "admin",
  "type": "access",
  "iat": 1690000000,
  "exp": 1690086400
}
```

| Claim | Description |
|-------|-------------|
| `sub` | User ID (or `apikey:<key-id>` for API key auth) |
| `username` | Human-readable identifier |
| `role` | RBAC role: `admin`, `editor`, or `viewer` |
| `type` | `access` or `refresh` |
| `iat` | Issued-at timestamp |
| `exp` | Expiration timestamp |

### Token Lifetimes

| Token | TTL | Cookie Path |
|-------|-----|-------------|
| Access Token | 24 hours | `/` |
| Refresh Token | 7 days | `/v1/auth/refresh` |

### Validation

1. Split on `.` — reject if not exactly 3 parts
2. Decode signature; compute HMAC-SHA256 of `header.payload`
3. Constant-time comparison of computed vs provided signature
4. Decode claims JSON
5. Reject if `exp <= 0` or `exp < time.Now().Unix()`

---

## Endpoints

### `POST /v1/auth/login` — Authenticate

**Request:**
```json
{
  "username": "admin",
  "password": "secret"
}
```

**Response (200):**
```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 86400
}
```

**Cookies set:**
- `cosca_access_token` — HttpOnly, Path `/`, MaxAge 24h
- `cosca_refresh_token` — HttpOnly, Path `/v1/auth/refresh`, MaxAge 7d
- `cosca_auth_state=true` — Non-HttpOnly (JS-readable, signals auth state)

**Account lockout:** 5 consecutive failed attempts → locked for 15 minutes (returns 429).

### `POST /v1/auth/refresh` — Refresh Token

Reads refresh token from `cosca_refresh_token` cookie or JSON body. Validates token type must be `refresh` (access tokens rejected). Issues new access + refresh pair (full rotation).

### `POST /v1/auth/logout` — Logout

Clears all 3 cookies (`MaxAge=-1`). No request body needed.

### `GET /v1/auth/me` — Current User

Returns authenticated user profile (password hash stripped).

### `GET /v1/csrf-token` — CSRF Token

Returns a fresh CSRF token for the double-submit pattern. Sets `csrf_token` cookie (HttpOnly=false, SameSite=Strict, 24h).

---

## Brute-Force Protection

- **5 consecutive failed attempts** → account locked for **15 minutes**
- Lock check happens **before** bcrypt comparison (saves CPU on locked accounts)
- Successful login resets `FailedAttempts` counter and clears `LockedUntil`
- Expired locks auto-reset on next authentication attempt

---

## RBAC Authorization

### Role Hierarchy

| Role | Rank | Capabilities |
|------|------|-------------|
| `admin` | 3 | Full access — bypasses all role checks |
| `editor` | 2 | Read + write access to all non-admin resources |
| `viewer` | 1 | Read-only access |

### Admin-Only Endpoints

All `/v1/users`, `/v1/api-keys`, `/v1/audit/logs`, `/v1/secrets`, and `/v1/skills/{name}/install` are protected with `RequireRole(RoleAdmin)`.

### Middleware

```go
// Auth middleware (JWT/API Key/Cookie → Claims in context)
handler = auth.Middleware(jwtSecret, publicPaths, apiKeyStore)(handler)

// Role-based authorization
adminOnly := auth.RequireRole(auth.RoleAdmin)
mux.Handle("GET /v1/users", adminOnly(http.HandlerFunc(users.List)))
```

### Context Access

```go
claims, ok := auth.ClaimsFromContext(ctx)
if !ok {
    // Unauthenticated
}
// claims.Sub, claims.Username, claims.Role
```

---

## API Keys

### Key Format

```
cosca_sk_<64 hex characters>
```

Example: `cosca_sk_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2`

### Storage

- Only **SHA-256 hash** is stored
- Plaintext returned **once** at creation time
- `List()` and `GetByID()` strip the `Hash` field
- Revoked keys remain for audit trail

### Lifecycle

| State | Description |
|-------|-------------|
| `active` | Valid and usable |
| `revoked` | Permanently disabled (audit trail preserved) |

### Expiration

Optional `expires_in_days` on creation. Keys with expired `ExpiresAt` are rejected at validation time.

### Endpoints (Admin only)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/api-keys` | List all keys (hashes stripped) |
| `POST` | `/v1/api-keys` | Generate new key — returns `{id, name, prefix, key, ...}` (key shown once) |
| `DELETE` | `/v1/api-keys/{id}` | Revoke key |

---

## User Management (Admin only)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/users` | List users (password hashes stripped) |
| `POST` | `/v1/users` | Create user (`username`, `password`, `role`, `email?`) |
| `DELETE` | `/v1/users/{id}` | Delete user |
| `PUT` | `/v1/users/{id}/role` | Update role |

### Validation

| Field | Max Length |
|-------|-----------|
| `username` | 50 |
| `password` | 128 |
| `email` | 254 |

### Password Hashing

- **bcrypt** cost factor **12** (`$2a$` format)
- Salt included automatically
- Password hashes stripped from all list/get responses

### Default Admin

In dev mode (`COSCA_DEV_MODE=true`), auto-creates `admin`/`admin`.

---

## Public Paths

These endpoints bypass authentication entirely:

| Path | Purpose |
|------|---------|
| `/health` | Health check |
| `/ready` | Readiness probe |
| `/v1/auth/login` | Authentication |
| `/v1/auth/refresh` | Token refresh |
| `/v1/auth/logout` | Logout |
| `/v1/csrf-token` | CSRF token |

---

## Persistence

- **Users**: `users.json` (atomic writes: temp file + rename)
- **API Keys**: `api-keys.json` (atomic writes: temp file + rename)
- **Passwords**: bcrypt hashes in users.json
- **API Key secrets**: SHA-256 hashes in api-keys.json

---

## Security Summary

| Property | Implementation |
|----------|---------------|
| Token signing | HMAC-SHA256, no external JWT library |
| Token TTLs | Access 24h, Refresh 7d |
| Password hashing | bcrypt cost 12 |
| Brute force | 5 fails → 15min lockout per account |
| Lockout check | Before bcrypt (CPU-saving) |
| API key storage | SHA-256 hash (not reversible) |
| Token on refresh | Full rotation (new access + refresh pair) |
| Cookie security | HttpOnly, SameSite=Strict, Secure on TLS |
| Context claims | Typed private key, safe extraction |
| Persistence | Atomic writes (temp file + rename) |

> **Note**: OIDC/OAuth2 third-party identity provider integration is **not implemented** in this version. Authentication is entirely local with the built-in user store.

---

> **Related**: [Middleware Reference](middleware.md) | [API Overview](overview.md) | [Runtime Configuration](../runtime/configuration.md)
