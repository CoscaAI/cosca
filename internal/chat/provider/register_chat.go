package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/rs/zerolog/log"
)

// Default model identifiers used when neither configuration nor environment
// variables specify one. Kept in sync with the constructor defaults in this
// package so a bare RegisterChatProviders call yields usable providers.
//
// LEI DO COFRE (fail-closed): o Cosca NUNCA chama um provider de nuvem sem
// uma API key explícita presente no ambiente. As factories de providers de
// nuvem podem estar registradas, mas retornam erro quando a chave está
// ausente — o que faz o Select() pular esses providers e recuar para os
// LOCAIS. Somente providers locais (ollama/gpu) e "none" são selecionáveis
// sem chave; "none" é o estado determinístico de ausência de IA.
const (
	defaultOllamaModel = "llama3"
)

// RegisterChatProviders registers the chat provider factories into the given
// ChatRegistry so that Select() can instantiate them. Reads configuration
// from env vars (propagated by cmd/cosca main.go) and the cosca config.
//
// Provider priorities (lower = higher): deepseek=10, openai=20, anthropic=30,
// ollama=40, none=50. Factories for API-key-backed providers return an error
// when the matching API key env var is absent, which makes Select() skip them
// during auto-detection. Ollama needs no API key and is always registered as a
// local fallback — it requires no key, only a running server at request time.
// "none" is the final always-available fallback: it never fails and represents
// the legitimate absence of a cognitive capability (deterministic mode).
//
// The optional cfg map may carry a shared "model" entry applied to every
// provider, or per-provider entries named "<name>_model" and
// "<name>_base_url". Environment variables (COSCA_<NAME>_MODEL and
// <NAME>_BASE_URL / OLLAMA_HOST) are read at factory-invocation time, so a
// hot-reload re-selection picks up key/model changes without a restart.
//
// The helper is robust by design: it never panics, tolerates a nil registry
// (returning an error instead), and only logs at debug level.
func RegisterChatProviders(reg *chat.ChatRegistry, cfg map[string]interface{}) error {
	if reg == nil {
		return fmt.Errorf("register chat providers: nil registry")
	}

	register := func(name, description string, priority int, factory chat.ChatProviderFactory) {
		reg.Register(name, factory, description, priority)
		log.Debug().Str("provider", name).Int("priority", priority).Msg("registered chat provider factory")
	}

	// DeepSeek is an OpenAI-compatible cloud provider. Per the LEI DO COFRE
	// (fail-closed) its factory is registered here as a candidate, but it
	// REFUSES to instantiate when DEEPSEEK_API_KEY is absent — so Select()
	// skips it and falls back to a local provider. The Cosca never hits a
	// cloud endpoint without an explicit key: the guard below is the
	// fail-closed boundary. Priority 10 puts it above the local fallbacks but
	// below the GPU fabric (5), mirroring the original "cloud above local
	// default, GPU first" ordering.
	register("deepseek", "DeepSeek chat provider (OpenAI-compatible)", 10,
		func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
			apiKey := os.Getenv("DEEPSEEK_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("deepseek: API key DEEPSEEK_API_KEY not set (fail-closed: refusing cloud call)")
			}
			model := resolveModel(cfg, "deepseek", "COSCA_DEEPSEEK_MODEL", "deepseek-v4-flash")
			baseURL := resolveBaseURL(cfg, "deepseek", "DEEPSEEK_BASE_URL")
			return NewProviderAdapter(NewDeepSeekWithBaseURL(apiKey, model, baseURL), model), nil
		})

	// Ollama is a local provider: it has no API key, so the factory always
	// succeeds. It acts as a last-resort fallback during auto-detection and
	// only fails at request time when no local server is reachable.
	register("ollama", "Ollama local chat provider", 40,
		func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
			model := resolveModel(cfg, "ollama", "COSCA_OLLAMA_MODEL", defaultOllamaModel)
			baseURL := resolveBaseURL(cfg, "ollama", "OLLAMA_BASE_URL")
			if baseURL == "" {
				baseURL = os.Getenv("OLLAMA_HOST")
				if baseURL != "" && !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
					baseURL = "http://" + baseURL
				}
			}
			return NewProviderAdapter(NewOllama(model, baseURL), model), nil
		})

	// GPU-accelerated provider: wraps the compute fabric's GPU executor
	// (Ollama with ROCm/CUDA backend). Registered at priority 5 (above
	// cloud providers) so local GPU inference is preferred when available.
	// The fabric is passed through the cfg map by bootstrap or serve.
	register("gpu", "Cosca GPU-accelerated chat provider (Ollama ROCm/CUDA)", 5,
		func(_ context.Context, cfg map[string]interface{}) (chat.ChatProvider, error) {
			fabric, _ := cfg["fabric"].(*compute.Fabric)
			if fabric == nil {
				return nil, fmt.Errorf("gpu: no compute fabric configured")
			}
			exec := fabric.GPUExecutor()
			if exec == nil {
				return nil, fmt.Errorf("gpu: no GPU executor configured on fabric")
			}
			gpuInfo := exec.GPUInfo()
			if gpuInfo.Vendor == compute.GPUNone {
				return nil, fmt.Errorf("gpu: no GPU detected")
			}
			model := resolveModel(cfg, "gpu", "COSCA_GPU_MODEL", "llama3.1:8b")
			return NewProviderAdapter(NewGPUChatProvider(exec, model), model), nil
		})

	// None is the deterministic mode: the platform running without any AI
	// capability. It is always available (the absence of a cognitive
	// capability is a valid state, not an outage) and acts as the final
	// fallback, so a bare registry always yields a selectable provider. Its
	// Chat emits a clear, actionable error routing callers to deterministic
	// operations (knowledge, laws, audit, plan).
	register("none", "Null provider (deterministic mode, no AI)", 50,
		func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
			return NewProviderAdapter(NewNullProvider(), "none"), nil
		})

	log.Debug().Strs("providers", reg.List()).Msg("chat provider factories registered")
	return nil
}

// resolveModel returns the model identifier for a provider, preferring (in
// order): a provider-specific config entry (<name>_model), a shared
// cfg["model"] entry, the COSCA_<NAME>_MODEL environment variable, and
// finally the built-in default.
func resolveModel(cfg map[string]interface{}, name, envVar, fallback string) string {
	if v := cfgString(cfg, name+"_model"); v != "" {
		return v
	}
	if v := cfgString(cfg, "model"); v != "" {
		return v
	}
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return fallback
}

// resolveBaseURL returns the base URL override for a provider from a
// provider-specific config entry or environment variable. An empty result
// means the provider's built-in default endpoint is used.
func resolveBaseURL(cfg map[string]interface{}, name, envVar string) string {
	if v := cfgString(cfg, name+"_base_url"); v != "" {
		return v
	}
	return os.Getenv(envVar)
}

// cfgString returns the string value of key in cfg, or "" when cfg is nil,
// the key is missing, or the value is not a string.
func cfgString(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg[key].(string)
	if !ok {
		return ""
	}
	return v
}
