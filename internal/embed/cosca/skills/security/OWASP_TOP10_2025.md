# OWASP Top 10 2025 — Defense per Language

> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief

## A01 — Broken Access Control

| Language | Defense |
|----------|---------|
| **Go** | Middleware check on every handler. `r.PathValue` validated before use. RBAC with context injection. |
| **Rust** | Actix guards + extractors. Axum `FromRequest` for auth extraction. |
| **Python** | FastAPI dependencies (`Depends(get_current_user)`). Django `@permission_required`. |
| **Node** | NestJS guards. Express middleware with role check. |
| **Java** | Spring Security `@PreAuthorize`. Method-level `@RolesAllowed`. |
| **C#** | ASP.NET `[Authorize(Roles = "...")]`. Policy-based authorization. |
| **Swift** | Keychain for tokens. `@MainActor` view model guards. |
| **Kotlin** | EncryptedSharedPrefs. Repository-level auth check. |

```go
// Go — access control middleware
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, _ := r.Context().Value(ClaimsKey).(*Claims)
            for _, required := range roles {
                for _, has := range claims.Roles {
                    if has == required {
                        next.ServeHTTP(w, r)
                        return
                    }
                }
            }
            http.Error(w, "forbidden", http.StatusForbidden)
        })
    }
}
```

## A02 — Cryptographic Failures

- **NEVER** invent your own crypto. Use standard libraries.
- **Go**: `crypto/rand` (not `math/rand`), `golang.org/x/crypto/argon2` for passwords
- **All**: AES-256-GCM for encryption at rest, TLS 1.3 for transit
- **Tokens**: JWT with HS256 minimum (RS256 for distributed systems)
- **Passwords**: bcrypt cost 12+ or argon2id

## A03 — Injection

```go
// ALWAYS parameterized — never string concatenation
db.Query("SELECT * FROM users WHERE id = ?", userInput) // ✅
db.Query("SELECT * FROM users WHERE id = '" + userInput + "'") // ❌ SQL Injection

// For dynamic ORDER BY / GROUP BY: whitelist
var allowedColumns = map[string]bool{"name": true, "created_at": true}
if !allowedColumns[sortBy] {
    return errors.New("invalid sort column")
}
```

## A04 — Insecure Design

- Threat model every feature BEFORE implementation
- Limit file uploads: max size, allowed MIME types, scan for malware
- Rate limit all public endpoints
- Never trust client-side validation — server must re-validate

## A05 — Security Misconfiguration

```bash
# Go
go build -ldflags="-w -s"  # strip debug info
# Docker
FROM scratch  # minimal image, no shell
# All
# Remove default accounts, disable directory listing, set secure headers
```

## A06 — Vulnerable Components

```bash
# Go: govulncheck ./...
# Node: pnpm audit
# Python: pip-audit / safety check
# Rust: cargo audit
# Java: ./gradlew dependencyCheckAnalyze
# C#: dotnet list package --vulnerable
```

## A07 — Auth Failures

- JWT: short expiry (15min access, 7d refresh), unique `jti`, revocation list
- MFA required for admin accounts
- Account lockout after 5 failed attempts
- Password requirements: 12+ chars, not in breach database (HaveIBeenPwned API)

## A08 — Software & Data Integrity

- Verify checksums of all downloaded dependencies
- Use lockfiles (`go.sum`, `pnpm-lock.yaml`, `Cargo.lock`, `Pipfile.lock`)
- Sign releases with cosign/sigstore
- CI pipeline integrity: require branch protection, signed commits

## A09 — Logging & Monitoring

```go
// Structured logging — NEVER log secrets
log.Info().
    Str("user_id", userID).
    Str("action", "login").
    Msg("user authenticated")
// ❌ log.Info("password: " + password)  <-- NEVER

// Alert on: 5 failed logins, 10 403s/min, SQL errors, panic recovery
```

## A10 — SSRF (Server-Side Request Forgery)

```go
var ssrfBlocklist = []string{
    "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
    "169.254.0.0/16", "::1", "fc00::/7",
}

func isSSRF(urlStr string) bool {
    u, _ := url.Parse(urlStr)
    ip := net.ParseIP(u.Hostname())
    for _, cidr := range ssrfBlocklist {
        _, network, _ := net.ParseCIDR(cidr)
        if network.Contains(ip) { return true }
    }
    return false
}
```

## Cosca Security Commands

```bash
cosca knowledge verify                # check knowledge integrity
cosca doctor                           # full diagnostic
cosca knowledge search "SQL injection" # find known patterns
gitleaks detect --source .             # pre-commit secret scan
```
