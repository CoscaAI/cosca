# Go API Implementation — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Backend Chief | **Stack**: Go 1.23+, net/http, chi | **Complexity**: advanced

## Purpose

Production-grade REST API implementation in Go. Covers the full lifecycle: project structure, handlers, middleware, database access, testing, and deployment. Follows Go standard library patterns with minimal dependencies.

## Project Structure

```
project/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── handler/                 # HTTP handlers (one file per domain)
│   │   ├── handler.go           # Shared response helpers
│   │   ├── users.go
│   │   └── health.go
│   ├── middleware/              # HTTP middleware
│   │   ├── auth.go              # JWT/OAuth2
│   │   ├── logging.go           # Request logging
│   │   ├── cors.go              # CORS configuration
│   │   └── ratelimit.go         # Rate limiting
│   ├── model/                   # Domain types
│   ├── store/                   # Data access (SQL, NoSQL)
│   └── service/                 # Business logic
├── api/
│   └── openapi.yaml             # API specification
└── go.mod
```

## Handler Pattern

```go
// internal/handler/handler.go
package handler

import (
    "encoding/json"
    "net/http"
)

type APIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, APIError{Code: status, Message: message})
}
```

```go
// internal/handler/users.go
type UserHandler struct {
    store *store.UserStore
}

func NewUserHandler(store *store.UserStore) *UserHandler {
    return &UserHandler{store: store}
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "id is required")
        return
    }

    user, err := h.store.Get(r.Context(), id)
    if err != nil {
        writeError(w, http.StatusNotFound, "user not found")
        return
    }

    writeJSON(w, http.StatusOK, user)
}
```

## Middleware Chain

```go
// cmd/server/main.go
func main() {
    r := chi.NewRouter()

    // Order matters: outer → inner
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)       // structured logging
    r.Use(middleware.Recoverer)    // panic recovery
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(corsMiddleware.Handler)  // CORS before auth
    r.Use(authMiddleware.JWT)      // authentication
    r.Use(ratelimitMiddleware.IP)  // rate limiting

    r.Get("/health", healthHandler.Check)
    r.Get("/api/v1/users/{id}", userHandler.Get)
    r.Post("/api/v1/users", userHandler.Create)

    srv := &http.Server{
        Addr:         ":8080",
        Handler:      r,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Graceful shutdown
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        <-sigCh
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        srv.Shutdown(ctx)
    }()

    log.Printf("server listening on :8080")
    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

## SQL Database Access

```go
// internal/store/users.go
type UserStore struct {
    db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
    return &UserStore{db: db}
}

func (s *UserStore) Get(ctx context.Context, id string) (*model.User, error) {
    // Always use parameterized queries — never string concatenation
    row := s.db.QueryRowContext(ctx,
        "SELECT id, name, email, created_at FROM users WHERE id = ?", id)

    var u model.User
    err := row.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound
    }
    return &u, err
}

func (s *UserStore) Create(ctx context.Context, u *model.User) error {
    result, err := s.db.ExecContext(ctx,
        `INSERT INTO users (id, name, email, created_at)
         VALUES (?, ?, ?, ?)`, u.ID, u.Name, u.Email, time.Now())
    if err != nil {
        return fmt.Errorf("insert user: %w", err)
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return ErrNotCreated
    }
    return nil
}
```

## Testing

```go
func TestUserHandler_Get(t *testing.T) {
    tests := []struct {
        name       string
        id         string
        setupStore func() *UserStore
        wantStatus int
    }{
        {
            name: "valid user",
            id:   "usr_123",
            setupStore: func() *UserStore {
                db := setupTestDB(t)
                db.Exec("INSERT INTO users (id, name) VALUES ('usr_123', 'Test')")
                return NewUserStore(db)
            },
            wantStatus: http.StatusOK,
        },
        {
            name:       "missing id",
            id:         "",
            setupStore: func() *UserStore { return NewUserStore(setupTestDB(t)) },
            wantStatus: http.StatusBadRequest,
        },
        {
            name:       "not found",
            id:         "nonexistent",
            setupStore: func() *UserStore { return NewUserStore(setupTestDB(t)) },
            wantStatus: http.StatusNotFound,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            h := NewUserHandler(tt.setupStore())
            w := httptest.NewRecorder()
            r := httptest.NewRequest("GET", "/api/v1/users/"+tt.id, nil)
            r.SetPathValue("id", tt.id)

            h.Get(w, r)

            if w.Code != tt.wantStatus {
                t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
            }
        })
    }
}
```

## Build & Run

```bash
# Development
go run ./cmd/server

# Build
CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/server ./cmd/server

# Test
go test -race -count=1 ./...

# Lint
golangci-lint run ./...

# Vulnerability scan
govulncheck ./...
```

## Cosca Integration

Before implementing, verify knowledge readiness:
```bash
cosca knowledge readiness --stack "chi,sqlite,jwt"
cosca knowledge search "Go chi middleware pattern"
```

## Security Checklist

- [ ] All queries parameterized (`?`, not string concat)
- [ ] JWT with HS256/RS256, 15min access + 7d refresh
- [ ] CORS origins explicit (not `*`)
- [ ] Rate limiting on auth endpoints (5 req/s per IP)
- [ ] Request timeout on all handlers (30s default)
- [ ] Panic recovery middleware
- [ ] No stack traces in error responses
- [ ] Graceful shutdown with draining
- [ ] govulncheck clean before deploy
