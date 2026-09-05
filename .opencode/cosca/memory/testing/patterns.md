---
type: testing
key: test-patterns
tags: [testing, patterns, go]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Cosca — Test Patterns

## AAA Pattern (Arrange, Act, Assert)
```go
func TestSomething(t *testing.T) {
    // Arrange
    ctx := context.Background()
    engine := NewEngine(testConfig())

    // Act
    result, err := engine.DoSomething(ctx, input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## Table-Driven Tests
```go
func TestProvider(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{...}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Provider(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

## Mock Pattern
```go
type mockProvider struct {
    models []string
    err    error
}
func (m *mockProvider) Models() ([]string, error) {
    return m.models, m.err
}
```

## TempDir Pattern (corrected)
```go
func TestWithTempDir(t *testing.T) {
    dir := t.TempDir()  // auto-cleanup, project-local
    // use dir...
}
```

## Race Detection
```bash
go test -race ./...  # always in CI
```

## Key Rules
- Never use `/tmp` — use `t.TempDir()` or `./tmp/`
- Never `panic()` in tests — use `t.Fatal()` or `require`
- Always use `require` for preconditions, `assert` for results
- Mock at interface boundaries, not implementation details
- Tests must pass with `-count=10` (no flakiness)
