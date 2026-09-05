// Package models syncs the models.dev catalog (anomalyco, MIT, open-source
// database of AI models) into a local cache and exposes lookups so Cosca can
// use real per-model context_length and pricing instead of hardcoded defaults.
//
// The cache is strictly an enhancement: when it is missing, stale, or offline
// the rest of Cosca keeps its current behavior (hardcoded fallbacks).
package models

// DefaultAPIURL is the public models.dev endpoint. It carries both provider
// endpoints and per-model metadata and requires no authentication.
const DefaultAPIURL = "https://models.dev/api.json"

// ModelPricing holds per-token pricing in USD.
type ModelPricing struct {
	// Prompt is the price per 1M input tokens.
	Prompt float64 `json:"prompt"`
	// Completion is the price per 1M output tokens.
	Completion float64 `json:"completion"`
	// InputCacheRead is the price per 1M cached input tokens.
	InputCacheRead float64 `json:"input_cache_read"`
	// InputCacheWrite is the price per 1M cached input tokens written.
	InputCacheWrite float64 `json:"input_cache_write"`
}

// Model is a single AI model from the models.dev catalog.
type Model struct {
	// ID is the canonical identifier, e.g. "deepseek/deepseek-v4-flash".
	ID string `json:"id"`
	// Name is the human-readable model name.
	Name string `json:"name"`
	// ContextLength is the model's context window in tokens (0 = unknown).
	ContextLength int64 `json:"context_length"`
	// InputModalities lists the input types the model accepts (text, image, ...).
	InputModalities []string `json:"input_modalities,omitempty"`
	// OutputModalities lists the output types the model produces.
	OutputModalities []string `json:"output_modalities,omitempty"`
	// Pricing holds per-token pricing in USD.
	Pricing ModelPricing `json:"pricing"`
	// SupportedParameters lists optional parameters the model accepts.
	SupportedParameters []string `json:"supported_parameters,omitempty"`
	// Provider is the top provider name (e.g. "deepseek", "openai").
	Provider string `json:"provider,omitempty"`
}
