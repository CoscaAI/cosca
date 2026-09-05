package ui

import (
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/provider"
)

// ─── Catálogo de Providers Conhecidos ─────────────────────────────────────────
// Define quais providers existem, se precisam de API key, os modelos padrão e
// o construtor usado para conectar. O seletor de modelo (picker) lista TODOS
// os providers conhecidos — mesmo os que ainda não têm chave — e o Don pode
// conectar escolhendo um e digitando a API key na TUI.

// knownProvider descreve um provider conhecido pelo Cosca.
type knownProvider struct {
	name         string
	needsKey     bool
	defaultModel string
	defaultURL   string
	// build constrói o provider com a chave (e URL) fornecidas.
	build func(apiKey, model, baseURL string) chat.Provider
}

// knownProviders é o catálogo estático de providers.
var knownProviders = []knownProvider{
	{
		name:         "deepseek",
		needsKey:     true,
		defaultModel: "deepseek-v4-flash",
		defaultURL:   "https://api.deepseek.com/v1",
		build: func(apiKey, model, baseURL string) chat.Provider {
			return provider.NewDeepSeek(apiKey, model)
		},
	},
	{
		name:         "openai",
		needsKey:     true,
		defaultModel: "gpt-4o",
		defaultURL:   "https://api.openai.com/v1",
		build: func(apiKey, model, baseURL string) chat.Provider {
			return provider.NewOpenAI(apiKey, model, baseURL)
		},
	},
	{
		name:         "anthropic",
		needsKey:     true,
		defaultModel: "claude-sonnet-4-20250514",
		defaultURL:   "https://api.anthropic.com",
		build: func(apiKey, model, baseURL string) chat.Provider {
			return provider.NewAnthropic(apiKey, model, baseURL)
		},
	},
	{
		name:         "ollama",
		needsKey:     false,
		defaultModel: "llama3",
		defaultURL:   "http://localhost:11434",
		build: func(apiKey, model, baseURL string) chat.Provider {
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			return provider.NewOllama(model, baseURL)
		},
	},
}

// knownProviderByName localiza um provider conhecido pelo nome.
func knownProviderByName(name string) (knownProvider, bool) {
	for _, kp := range knownProviders {
		if kp.name == name {
			return kp, true
		}
	}
	return knownProvider{}, false
}

// providerNeedsKey reporta se um provider conhecido exige API key.
func providerNeedsKey(name string) bool {
	if kp, ok := knownProviderByName(name); ok {
		return kp.needsKey
	}
	return false
}
