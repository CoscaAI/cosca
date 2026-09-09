---
type: pattern
key: go-provider-pattern
tags: [go, provider, interface, plugin, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Architecture Chief
category: design
confidence: 0.95
times_used: 10
---

# Pattern: Go Provider Interface

## Context
Cosca needs to support 10+ LLM providers (OpenAI, Anthropic, Ollama, etc.) through a unified interface. Each provider has different APIs, authentication, and response formats.

## Solution
Define a `Provider` interface and implement it per-provider.

```go
// internal/providers/providers.go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Health(ctx context.Context) error
    Models() []string
}

type ChatRequest struct {
    Model    string
    Messages []Message
    Options  ChatOptions
}

type ChatResponse struct {
    Content   string
    Usage     Usage
    Model     string
}
```

## Structure
```
internal/providers/
├── providers.go        ← Interface + registry
├── providers_test.go   ← Interface conformance tests
├── common.go           ← Shared types
├── transport.go        ← HTTP transport
├── ratelimit.go        ← Rate limiting
├── openai/             ← Implementation
├── anthropic/          ← Implementation
├── ollama/             ← Implementation
├── mistral/            ← Implementation
├── groq/               ← Implementation
├── deepseek/           ← Implementation
├── google/             ← Implementation
├── azure/              ← Implementation
├── bedrock/            ← Implementation
├── local/              ← Local models
└── openaicompat/       ← OpenAI-compatible
```

## Benefits
- Add new provider without changing any other code
- Test each provider in isolation
- Provider registry with lazy loading
- Health checks per provider
- Rate limiting per provider

## Application in Cosca
All 10+ providers implement this interface. The registry auto-discovers providers at startup. CLI commands (`cosca provider list`, `cosca provider test`) use the interface generically.
