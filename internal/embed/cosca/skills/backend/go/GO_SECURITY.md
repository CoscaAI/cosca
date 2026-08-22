# Go Security — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief | **Stack**: Go 1.23+

## Purpose

Hardened security patterns for Go applications. Covers authentication, authorization, input validation, cryptography, secret management, and dependency auditing. Based on OWASP Go guidelines and real attack vectors.

## Authentication — JWT

```go
// internal/auth/jwt.go
import (
    "crypto/rand"
    "encoding/base64"
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    jwt.RegisteredClaims
    UserID string   `json:"sub"`
    Roles  []string `json:"roles"`
}

const (
    AccessTokenDuration  = 15 * time.Minute
    RefreshTokenDuration = 7 * 24 * time.Hour
)

// GenerateSecureKey creates a cryptographically random 256-bit key.
// NEVER hardcode keys. Load from environment or vault.
func GenerateSecureKey() (string, error) {
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(key), nil
}

// NewAccessToken creates a short-lived access token.
func NewAccessToken(userID string, roles []string, secret []byte) (string, error) {
    now := time.Now()
    claims := Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
            ID:        generateJTI(), // unique token ID for revocation
        },
        Roles: roles,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(secret)
}

// ValidateToken verifies and parses a token. Returns claims or error.
func ValidateToken(tokenStr string, secret []byte) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{},
        func(t *jwt.Token) (any, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
            }
            return secret, nil
        },
    )
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid claims")
    }
    return claims, nil
}
```

## Auth Middleware

```go
// internal/middleware/auth.go
func JWT(secret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            header := r.Header.Get("Authorization")
            if header == "" || !strings.HasPrefix(header, "Bearer ") {
                writeError(w, http.StatusUnauthorized, "missing authorization header")
                return
            }

            claims, err := ValidateToken(header[7:], secret)
            if err != nil {
                writeError(w, http.StatusUnauthorized, "invalid or expired token")
                return
            }

            // Inject claims into context for downstream handlers.
            ctx := context.WithValue(r.Context(), ClaimsKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

## Input Validation

```go
// internal/validate/validate.go
func SanitizeString(s string, maxLen int) (string, error) {
    s = strings.TrimSpace(s)
    if len(s) == 0 {
        return "", fmt.Errorf("string is empty")
    }
    if len(s) > maxLen {
        return "", fmt.Errorf("string exceeds max length of %d", maxLen)
    }
    // Trim control characters and null bytes.
    s = strings.Map(func(r rune) rune {
        if r < 32 || r == 127 {
            return -1 // drop
        }
        return r
    }, s)
    return s, nil
}

func ValidateEmail(email string) error {
    if len(email) > 254 {
        return fmt.Errorf("email too long")
    }
    // Use net/mail for RFC-compliant parsing.
    _, err := mail.ParseAddress(email)
    return err
}

type CreateUserInput struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func (i *CreateUserInput) Validate() error {
    var errs []string

    if name, err := SanitizeString(i.Name, 100); err != nil {
        errs = append(errs, "name: "+err.Error())
    } else {
        i.Name = name
    }

    if err := ValidateEmail(i.Email); err != nil {
        errs = append(errs, "email: "+err.Error())
    }

    if len(errs) > 0 {
        return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

## Rate Limiting

```go
// internal/middleware/ratelimit.go
// Token bucket per IP, stored in memory. Use Redis for distributed deployments.
func IPRateLimiter(rate int, burst int) func(http.Handler) http.Handler {
    limiter := make(map[string]*rateLimiterEntry)
    var mu sync.Mutex

    // Cleanup goroutine — prevent memory leak from stale entries.
    go func() {
        for {
            time.Sleep(10 * time.Minute)
            mu.Lock()
            for ip, e := range limiter {
                if time.Since(e.lastSeen) > 30*time.Minute {
                    delete(limiter, ip)
                }
            }
            mu.Unlock()
        }
    }()

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip, _, _ := net.SplitHostPort(r.RemoteAddr)

            mu.Lock()
            entry, ok := limiter[ip]
            if !ok {
                entry = &rateLimiterEntry{
                    tokens:    burst,
                    lastSeen:  time.Now(),
                    burst:     burst,
                    lastReset: time.Now(),
                }
                limiter[ip] = entry
            }
            mu.Unlock()

            if !entry.allow() {
                writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## Secret Management

```go
// NEVER in code: apiKey := "sk-abc123" — this is an instant security failure.

// Use environment variables with validation.
func MustLoadSecret(name string) string {
    val := os.Getenv(name)
    if val == "" {
        log.Fatalf("required secret %s not set", name)
    }
    return val
}

// At startup:
// dbPassword := MustLoadSecret("DB_PASSWORD")
// jwtSecret := MustLoadSecret("JWT_SECRET")
// apiKey := MustLoadSecret("PROVIDER_API_KEY")
```

## Security Commands

```bash
# Static analysis
golangci-lint run --enable gosec ./...

# Vulnerability scan (mandatory before deploy)
govulncheck ./...

# Check for secrets in code (pre-commit hook)
gitleaks detect --source .

# Dependency audit
go mod verify
go list -m -u all  # check for updates

# Binary analysis
go tool objdump bin/server | strings | grep -i "secret\|password\|key"
```

## Security Checklist — Every PR Must Pass

- [ ] No hardcoded secrets (`rg 'sk-|api.?key|password\s*=' --type go | grep -v test`)
- [ ] JWT with short expiry (15min access, 7d refresh)
- [ ] JWT uses unique `jti` for revocation support
- [ ] Input validated BEFORE any processing
- [ ] SQL parameterized — zero string concatenation
- [ ] CORS origins explicit, not wildcard
- [ ] Rate limiting on all auth endpoints
- [ ] Content-Type header explicitly set on all responses
- [ ] No stack traces in error responses
- [ ] govulncheck returns 0 findings
- [ ] gitleaks clean (zero secrets committed)
