// Package providers provides the provider management system for Cosca.
// It manages AI/LLM providers such as OpenAI, Anthropic, Ollama, etc.
package providers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// ProviderInfo holds information about a single provider.
type ProviderInfo struct {
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Model        string   `json:"model"`
	Active       bool     `json:"active"`
	Configured   bool     `json:"configured"`
	BaseURL      string   `json:"base_url"`
	APIVersion   string   `json:"api_version"`
	Models       []string `json:"models,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// TestResult holds the result of a provider connection test.
type TestResult struct {
	ResponseTime string `json:"response_time"`
	Model        string `json:"model"`
	Status       string `json:"status"`
}

// Manager manages AI providers.
type Manager struct {
	active      string
	activeModel string
	registry    ProviderRegistry // optional: when set, List enriches entries with registration data
}

// ProviderRegistry provides information about providers registered in the
// chat subsystem. Implementations are expected to come from the
// internal/chat package (ChatRegistry satisfies this interface).
type ProviderRegistry interface {
	// IsRegistered returns true when the named provider is registered.
	IsRegistered(name string) bool

	// List returns all registered provider names.
	List() []string
}

// ProviderStatus represents the status of provider management.
type ProviderStatus struct {
	Active     string   `json:"active"`
	Configured int      `json:"configured"`
	Available  int      `json:"available"`
	Statuses   []string `json:"statuses"`
}

// NewManager creates a new provider manager.
func NewManager() *Manager {
	return &Manager{}
}

// SetRegistry attaches a ProviderRegistry for dynamic provider discovery.
// When set, List enriches the built-in provider catalogue with registration
// data from the chat subsystem. Pass nil to revert to static-only mode.
func (m *Manager) SetRegistry(registry ProviderRegistry) {
	m.registry = registry
}

// Status returns the current provider status.
func (m *Manager) Status() ProviderStatus {
	providers := m.List()
	configured := 0
	available := 0
	var statuses []string
	statuses = make([]string, 0, len(providers))
	for _, p := range providers {
		if p.Configured {
			configured++
		}
		available++
		statuses = append(statuses, fmt.Sprintf("%s: %s", p.Name, p.Status))
	}
	return ProviderStatus{
		Active:     m.active,
		Configured: configured,
		Available:  available,
		Statuses:   statuses,
	}
}

// List returns all available providers with their status.
// It starts from a built-in catalogue and enriches entries with registration
// data when a ProviderRegistry has been set via SetRegistry.
func (m *Manager) List() []ProviderInfo {
	providers := m.buildDefaultProviders()

	if m.registry != nil {
		providers = m.enhanceWithRegistry(providers)
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})

	return providers
}

// buildDefaultProviders returns the hardcoded provider catalogue. This is
// the fallback used when no ProviderRegistry is available.
func (m *Manager) buildDefaultProviders() []ProviderInfo {
	var providers []ProviderInfo
	providers = make([]ProviderInfo, 0, 10)

	// OpenAI
	providers = append(providers, ProviderInfo{
		Name:         "openai",
		Status:       detectStatus("openai"),
		Model:        "gpt-4o",
		Active:       m.active == "openai",
		Configured:   os.Getenv("OPENAI_API_KEY") != "",
		BaseURL:      "https://api.openai.com/v1",
		Models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		Capabilities: []string{"chat", "embeddings", "vision"},
	})

	// Anthropic
	providers = append(providers, ProviderInfo{
		Name:         "anthropic",
		Status:       detectStatus("anthropic"),
		Model:        "claude-3-5-sonnet",
		Active:       m.active == "anthropic",
		Configured:   os.Getenv("ANTHROPIC_API_KEY") != "",
		BaseURL:      "https://api.anthropic.com/v1",
		Models:       []string{"claude-3-5-sonnet", "claude-3-opus", "claude-3-haiku"},
		Capabilities: []string{"chat", "vision"},
	})

	// Google/Gemini
	providers = append(providers, ProviderInfo{
		Name:         "google",
		Status:       detectStatus("google"),
		Model:        "gemini-pro",
		Active:       m.active == "google",
		Configured:   os.Getenv("GOOGLE_API_KEY") != "",
		BaseURL:      "https://generativelanguage.googleapis.com/v1",
		Models:       []string{"gemini-pro", "gemini-flash", "gemini-ultra"},
		Capabilities: []string{"chat", "embeddings", "vision"},
	})

	// Azure
	providers = append(providers, ProviderInfo{
		Name:         "azure",
		Status:       detectStatus("azure"),
		Model:        "gpt-4o",
		Active:       m.active == "azure",
		Configured:   os.Getenv("AZURE_OPENAI_API_KEY") != "",
		BaseURL:      "",
		Models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4"},
		Capabilities: []string{"chat", "embeddings", "vision"},
	})

	// DeepSeek
	providers = append(providers, ProviderInfo{
		Name:         "deepseek",
		Status:       detectStatus("deepseek"),
		Model:        "deepseek-v4-flash",
		Active:       m.active == "deepseek",
		Configured:   os.Getenv("DEEPSEEK_API_KEY") != "",
		BaseURL:      "https://api.deepseek.com/v1",
		Models:       []string{"deepseek-v4-flash", "deepseek-chat", "deepseek-reasoner"},
		Capabilities: []string{"chat"},
	})

	// Ollama (local)
	providers = append(providers, ProviderInfo{
		Name:         "ollama",
		Status:       detectStatus("ollama"),
		Model:        "llama3",
		Active:       m.active == "ollama",
		Configured:   true, // local, always available if running
		BaseURL:      "http://localhost:11434",
		Models:       []string{"llama3", "mistral", "codellama", "phi3"},
		Capabilities: []string{"chat", "embeddings"},
	})

	// Groq
	providers = append(providers, ProviderInfo{
		Name:         "groq",
		Status:       detectStatus("groq"),
		Model:        "llama-3.1-70b",
		Active:       m.active == "groq",
		Configured:   os.Getenv("GROQ_API_KEY") != "",
		BaseURL:      "https://api.groq.com/openai/v1",
		Models:       []string{"llama-3.1-70b", "mixtral-8x7b", "gemma2-9b"},
		Capabilities: []string{"chat"},
	})

	// Mistral
	providers = append(providers, ProviderInfo{
		Name:         "mistral",
		Status:       detectStatus("mistral"),
		Model:        "mistral-large",
		Active:       m.active == "mistral",
		Configured:   os.Getenv("MISTRAL_API_KEY") != "",
		BaseURL:      "https://api.mistral.ai/v1",
		Models:       []string{"mistral-large", "mistral-medium", "mistral-small"},
		Capabilities: []string{"chat", "embeddings"},
	})

	// Bedrock
	providers = append(providers, ProviderInfo{
		Name:         "bedrock",
		Status:       detectStatus("bedrock"),
		Model:        "anthropic.claude-v2",
		Active:       m.active == "bedrock",
		Configured:   os.Getenv("AWS_ACCESS_KEY_ID") != "",
		BaseURL:      "",
		Models:       []string{"anthropic.claude-v2", "amazon.titan", "meta.llama2"},
		Capabilities: []string{"chat", "embeddings"},
	})

	// Local/TF-IDF (always available)
	providers = append(providers, ProviderInfo{
		Name:         "local",
		Status:       "available",
		Model:        "tf-idf",
		Active:       m.active == "local",
		Configured:   true,
		BaseURL:      "local",
		Models:       []string{"tf-idf"},
		Capabilities: []string{"embeddings"},
	})

	return providers
}

// enhanceWithRegistry merges registration data from the chat registry into
// the provider catalogue. Hardcoded entries that are also registered get
// their status and Configured flag updated; registry-only names are added
// as new basic entries.
func (m *Manager) enhanceWithRegistry(providers []ProviderInfo) []ProviderInfo {
	registeredNames := m.registry.List()
	registeredSet := make(map[string]bool, len(registeredNames))
	for _, name := range registeredNames {
		registeredSet[name] = true
	}

	// Enhance existing providers with registration data.
	for i := range providers {
		if registeredSet[providers[i].Name] {
			if providers[i].Status == "configured" || providers[i].Status == "available" {
				providers[i].Status = "registered"
			}
			providers[i].Configured = true
		}
	}

	// Add registry-only providers not present in the hardcoded catalogue.
	for _, name := range registeredNames {
		if !hasProviderByName(providers, name) && name != "" {
			providers = append(providers, ProviderInfo{
				Name:       name,
				Status:     "registered",
				Configured: true,
				Active:     m.active == name,
			})
		}
	}

	return providers
}

// hasProviderByName checks whether a provider with the given name exists
// in the slice (case-sensitive exact match).
func hasProviderByName(providers []ProviderInfo, name string) bool {
	for _, p := range providers {
		if p.Name == name {
			return true
		}
	}
	return false
}

// SetActive sets the active provider and optional model.
func (m *Manager) SetActive(provider, model string) error {
	providers := m.List()
	found := false
	for _, p := range providers {
		if p.Name == provider {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("provider %q not found", provider)
	}

	m.active = provider
	if model != "" {
		m.activeModel = model
	}
	return nil
}

// Test tests connectivity to a provider and returns the result.
func (m *Manager) Test(provider string) (TestResult, error) {
	providers := m.List()
	var target ProviderInfo
	found := false
	for _, p := range providers {
		if p.Name == provider {
			target = p
			found = true
			break
		}
	}
	if !found {
		return TestResult{}, fmt.Errorf("provider %q not found", provider)
	}

	start := time.Now()
	status := "not_configured"

	switch provider {
	case "ollama":
		status = testOllama()
	default:
		if target.Configured && target.BaseURL != "" {
			status = testHTTP(target.BaseURL)
		} else if target.Configured {
			status = "configured"
		}
	}

	elapsed := time.Since(start)
	return TestResult{
		ResponseTime: elapsed.Round(time.Millisecond).String(),
		Model:        target.Model,
		Status:       status,
	}, nil
}

// Info returns detailed information about a specific provider.
func (m *Manager) Info(provider string) (ProviderInfo, error) {
	providers := m.List()
	for _, p := range providers {
		if p.Name == provider {
			return p, nil
		}
	}
	return ProviderInfo{Name: provider}, fmt.Errorf("provider %q not found", provider)
}

// ── Helpers ────────────────────────────────────────────────────────────────

// detectStatus checks the connectivity status of a provider.
func detectStatus(provider string) string {
	switch provider {
	case "ollama":
		return testOllama()
	case "openai":
		if os.Getenv("OPENAI_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "anthropic":
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "google":
		if os.Getenv("GOOGLE_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "azure":
		if os.Getenv("AZURE_OPENAI_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "deepseek":
		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "groq":
		if os.Getenv("GROQ_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "mistral":
		if os.Getenv("MISTRAL_API_KEY") != "" {
			return "configured"
		}
		return "no_key"
	case "bedrock":
		if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
			return "configured"
		}
		return "no_credentials"
	default:
		return "unknown"
	}
}

// testOllama checks if Ollama is running locally.
func testOllama() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return "not_running"
	}
	defer safe.Close(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return "available"
	}
	return "error"
}

// testHTTP checks if an HTTP endpoint is reachable.
func testHTTP(baseURL string) string {
	client := &http.Client{Timeout: 3 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL, nil)
	if err != nil {
		return "configured"
	}

	resp, err := client.Do(req)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "timeout"
		}
		return "configured" // can't reach but configured
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return "reachable"
	}
	return "configured"
}
