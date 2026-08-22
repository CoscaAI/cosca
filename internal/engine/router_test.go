package engine

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

func setupRouterTest(t *testing.T) (*AgentRegistry, *Router) {
	t.Helper()
	dir := t.TempDir()

	writeAgentFile(t, dir, "database.md",
		"name: cosca-database\ncapabilities: [database, sql, migration, schema]\ndescription: Expert in database design and SQL queries",
		"Database expert prompt")

	writeAgentFile(t, dir, "architecture.md",
		"name: cosca-architecture\ncapabilities: [architecture, design, patterns, system]\ndescription: Expert in software architecture and design patterns",
		"Architecture prompt")

	writeAgentFile(t, dir, "general.md",
		"name: cosca-general\ncapabilities: [general, help]\ndescription: General purpose assistant",
		"General prompt")

	registry := NewAgentRegistry(dir)
	router := NewRouter(registry)
	return registry, router
}

func TestRouteMention(t *testing.T) {
	t.Run("@mention routes to correct agent", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "Hey @cosca-database, write a query", nil)
		if result.Agent != "cosca-database" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-database")
		}
		if result.Confidence != 1.0 {
			t.Errorf("Confidence = %f, want 1.0", result.Confidence)
		}
		if result.Method != methodMention {
			t.Errorf("Method = %q, want %q", result.Method, methodMention)
		}
	})

	t.Run("/agent command routes to correct agent", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "/agent cosca-architecture design this system", nil)
		if result.Agent != "cosca-architecture" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-architecture")
		}
		if result.Confidence != 1.0 {
			t.Errorf("Confidence = %f, want 1.0", result.Confidence)
		}
		if result.Method != methodMention {
			t.Errorf("Method = %q, want %q", result.Method, methodMention)
		}
	})

	t.Run("@mention of unknown agent falls through to keyword", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "@unknown-agent help me", nil)
		// Should fall through to keyword or fallback
		if result.Agent == "unknown-agent" {
			t.Error("should not route to unknown @mention agent")
		}
	})
}

func TestRouteKeyword(t *testing.T) {
	t.Run("keyword match routes to correct agent", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "I need help with a database migration", nil)
		if result.Agent != "cosca-database" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-database")
		}
		if result.Method != methodKeyword {
			t.Errorf("Method = %q, want %q", result.Method, methodKeyword)
		}
		if result.Confidence > 0.9 {
			t.Errorf("Confidence %f should be capped at 0.9", result.Confidence)
		}
	})

	t.Run("architecture keyword routes to architecture agent", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "design the system architecture", nil)
		if result.Agent != "cosca-architecture" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-architecture")
		}
	})

	t.Run("partial word does not match short words", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		// "sql" is 3 chars, should match
		result := router.Route(ctx, "sql query help", nil)
		if result.Agent != "cosca-database" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-database")
		}
	})

	t.Run("low confidence keyword falls back to general", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		// No matching keywords
		result := router.Route(ctx, "hello how are you", nil)
		if result.Agent != defaultAgent {
			t.Errorf("Agent = %q, want %q", result.Agent, defaultAgent)
		}
		if result.Method != methodFallback {
			t.Errorf("Method = %q, want %q", result.Method, methodFallback)
		}
		if result.Confidence != 0.3 {
			t.Errorf("Confidence = %f, want 0.3", result.Confidence)
		}
	})
}

func TestRouteFallback(t *testing.T) {
	t.Run("unknown input falls back to general agent", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "xyzzy flurbo garblex", nil)
		if result.Agent != defaultAgent {
			t.Errorf("Agent = %q, want %q", result.Agent, defaultAgent)
		}
		if result.Method != methodFallback {
			t.Errorf("Method = %q, want %q", result.Method, methodFallback)
		}
	})

	t.Run("garbage input falls back", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "xyzzx flurbo garblex", nil)
		if result.Agent != defaultAgent {
			t.Errorf("Agent = %q, want %q", result.Agent, defaultAgent)
		}
	})
}

func TestRouteResultFields(t *testing.T) {
	t.Run("RouteResult has correct fields for mention", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "@cosca-database query", nil)
		if result.Agent == "" {
			t.Error("Agent should not be empty")
		}
		if result.Confidence < 0 || result.Confidence > 1.0 {
			t.Errorf("Confidence out of range: %f", result.Confidence)
		}
		if result.Method == "" {
			t.Error("Method should not be empty")
		}
	})

	t.Run("RouteResult has correct fields for fallback", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		result := router.Route(ctx, "zzz unknown input", nil)
		if result.Agent == "" {
			t.Error("Agent should not be empty even for fallback")
		}
		if result.Confidence != 0.3 {
			t.Errorf("Confidence = %f, want 0.3", result.Confidence)
		}
	})
}

func TestExtractAgentMention(t *testing.T) {
	t.Run("extracts @mention", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("Hello @cosca-database help")
		if name != "cosca-database" {
			t.Errorf("got %q, want %q", name, "cosca-database")
		}
	})

	t.Run("extracts @mention at start of input", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("@cosca-database write a query")
		if name != "cosca-database" {
			t.Errorf("got %q, want %q", name, "cosca-database")
		}
	})

	t.Run("extracts /agent command", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("/agent cosca-architecture design")
		if name != "cosca-architecture" {
			t.Errorf("got %q, want %q", name, "cosca-architecture")
		}
	})

	t.Run("returns empty for normal text", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("Hello, how are you?")
		if name != "" {
			t.Errorf("got %q, want empty", name)
		}
	})

	t.Run("returns empty for empty string", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("")
		if name != "" {
			t.Errorf("got %q, want empty", name)
		}
	})

	t.Run("@mention with trailing punctuation", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("Hey @cosca-database!")
		if name != "cosca-database" {
			t.Errorf("got %q, want %q", name, "cosca-database")
		}
	})

	t.Run("/agent with trailing text", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("/agent cosca-database please help")
		if name != "cosca-database" {
			t.Errorf("got %q, want %q", name, "cosca-database")
		}
	})

	t.Run("multiple @mentions extracts first", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("@cosca-database and @cosca-architecture")
		if name != "cosca-database" {
			t.Errorf("got %q, want %q (first mention)", name, "cosca-database")
		}
	})

	t.Run("@mention with no following text", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("@")
		if name != "" {
			t.Errorf("got %q, want empty", name)
		}
	})

	t.Run("/agent with no following name", func(t *testing.T) {
		_, router := setupRouterTest(t)

		name := router.ExtractAgentMention("/agent ")
		if name != "" {
			t.Errorf("got %q, want empty", name)
		}
	})
}

func TestRouterWithHistory(t *testing.T) {
	t.Run("history parameter is accepted but not used in routing", func(t *testing.T) {
		_, router := setupRouterTest(t)
		ctx := context.Background()

		history := []chat.Message{
			{Role: chat.RoleUser, Content: "Earlier question"},
			{Role: chat.RoleAssistant, Content: "Earlier answer"},
		}
		result := router.Route(ctx, "database query", history)
		if result.Agent != "cosca-database" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-database")
		}
	})
}

func TestEmptyRegistryRouter(t *testing.T) {
	t.Run("router with empty registry falls back to general", func(t *testing.T) {
		registry := NewAgentRegistry()
		router := NewRouter(registry)
		ctx := context.Background()

		result := router.Route(ctx, "hello", nil)
		if result.Agent != defaultAgent {
			t.Errorf("Agent = %q, want %q", result.Agent, defaultAgent)
		}
		if result.Method != methodFallback {
			t.Errorf("Method = %q", result.Method)
		}
	})

	t.Run("@mention with empty registry falls back", func(t *testing.T) {
		registry := NewAgentRegistry()
		router := NewRouter(registry)
		ctx := context.Background()

		result := router.Route(ctx, "@cosca-database help", nil)
		// The mention is extracted but agent doesn't exist, so falls through
		if result.Method != methodFallback {
			t.Errorf("Method = %q, want fallback when agent not in registry", result.Method)
		}
	})
}
