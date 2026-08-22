package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// syncTimeout bounds a single models.dev catalog download.
const syncTimeout = 30 * time.Second

// SyncFromAPI fetches the models.dev catalog from apiURL, parses it into a
// Cache, and writes it atomically to dest. It returns the populated cache.
//
// The upstream api.json document is keyed by provider name; each provider
// carries a "models" map of model metadata. Parsing is tolerant of the
// legacy layout (context_length / pricing.* as decimal strings / top_provider)
// so older snapshots and the provider-agnostic models.json keep working.
//
// On any network, HTTP, parse, or write failure an error is returned so the
// CLI can surface a clear message; the existing cache is left untouched.
func SyncFromAPI(ctx context.Context, apiURL, dest string) (*Cache, error) {
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}

	reqCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create models.dev request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cosca-models/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models.dev catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MB cap
	if err != nil {
		return nil, fmt.Errorf("read models.dev catalog: %w", err)
	}

	models, err := parseCatalog(body)
	if err != nil {
		return nil, err
	}

	c := NewCache(dest)
	c.UpdatedAt = time.Now().UTC()
	c.Models = models

	if err := SaveCache(dest, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ─── Parsing ────────────────────────────────────────────────────────────────

// parseCatalog detects the shape of a models.dev document and returns a map
// keyed by canonical model ID.
//
// Supported shapes:
//   - api.json:     { "deepseek": { ..., "models": { "deepseek-v4-flash": {...} } } }
//   - models.json:  { "deepseek/deepseek-v4-flash": {...} }
//   - catalog.json: { "models": {...}, "providers": {...} }
func parseCatalog(data []byte) (map[string]Model, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse models.dev catalog: %w", err)
	}
	if len(probe) == 0 {
		return map[string]Model{}, nil
	}

	out := make(map[string]Model)

	// catalog.json: {"models": {...}, "providers": {...}}.
	if raw, ok := probe["models"]; ok {
		if _, hasProviders := probe["providers"]; hasProviders {
			if err := parseFlatModels(raw, out); err != nil {
				return nil, err
			}
			return out, nil
		}
	}

	// Provider-keyed api.json: values carry a "models" submap.
	if isProviderKeyed(probe) {
		// Deterministic iteration order so the same catalog always produces
		// the same cache.
		providerKeys := make([]string, 0, len(probe))
		for providerID := range probe {
			providerKeys = append(providerKeys, providerID)
		}
		sort.Strings(providerKeys)

		// owner tracks which top-level provider key won a model ID, so alias
		// providers (which resell a model under the same canonical ID) never
		// clobber the canonical entry.
		owner := make(map[string]string)

		for _, providerID := range providerKeys {
			var p struct {
				Name   string                     `json:"name"`
				Models map[string]json.RawMessage `json:"models"`
			}
			if err := json.Unmarshal(probe[providerID], &p); err != nil {
				return nil, fmt.Errorf("parse provider %q: %w", providerID, err)
			}
			providerName := p.Name
			if providerName == "" {
				providerName = providerID
			}
			for modelID, mRaw := range p.Models {
				m, err := parseModel(mRaw)
				if err != nil {
					return nil, fmt.Errorf("parse model %q: %w", modelID, err)
				}
				m.ID = canonicalID(providerID, modelID, m.ID)
				if m.Provider == "" {
					m.Provider = providerName
				}
				prefix, _, _ := strings.Cut(m.ID, "/")
				prev, exists := out[m.ID]
				if !exists {
					out[m.ID] = m
					owner[m.ID] = providerID
					continue
				}
				// Collision: the provider whose key owns the model's ID
				// prefix is canonical and wins over reseller aliases.
				if providerID == prefix && owner[m.ID] != prefix {
					out[m.ID] = m
					owner[m.ID] = providerID
					continue
				}
				// Keep the existing entry; ensure the canonical owner keeps a
				// name even if its own entry came first without one.
				if owner[m.ID] == prefix && prev.Provider == "" {
					prev.Provider = providerName
					out[m.ID] = prev
				}
			}
		}
		return out, nil
	}

	// Flat models.json: keyed directly by model ID.
	if err := parseFlatModels(data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// isProviderKeyed reports whether the top-level entries look like providers,
// i.e. their values contain a "models" submap (current api.json).
func isProviderKeyed(probe map[string]json.RawMessage) bool {
	sampled := 0
	for _, raw := range probe {
		var p struct {
			Models map[string]json.RawMessage `json:"models"`
		}
		if json.Unmarshal(raw, &p) == nil && len(p.Models) > 0 {
			return true
		}
		sampled++
		if sampled >= 8 {
			break
		}
	}
	return false
}

// parseFlatModels parses a document keyed directly by model ID (models.json
// layout, or the "models" half of catalog.json).
func parseFlatModels(data []byte, out map[string]Model) error {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse flat models: %w", err)
	}
	for id, raw := range entries {
		m, err := parseModel(raw)
		if err != nil {
			return fmt.Errorf("parse model %q: %w", id, err)
		}
		m.ID = canonicalID("", id, m.ID)
		if m.Provider == "" {
			m.Provider = providerFromID(m.ID)
		}
		out[m.ID] = m
	}
	return nil
}

// canonicalID resolves the model's canonical ID, preferring the explicit map
// key (which for provider-keyed documents is prefixed with the provider name).
func canonicalID(providerKey, mapKey, declaredID string) string {
	if declaredID != "" && strings.Contains(declaredID, "/") {
		return declaredID
	}
	id := mapKey
	if id == "" {
		id = declaredID
	}
	if providerKey != "" && !strings.Contains(id, "/") {
		id = providerKey + "/" + id
	}
	return id
}

// providerFromID extracts the provider name from a canonical "provider/model" ID.
func providerFromID(id string) string {
	provider, _, _ := strings.Cut(id, "/")
	return provider
}

// parseModel parses a single model entry, accepting both the current layout
// (limit.context, cost.*, modalities.*) and the legacy layout (context_length,
// pricing.* as decimal strings, architecture.*, top_provider).
func parseModel(raw json.RawMessage) (Model, error) {
	var m struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		ContextLength int64  `json:"context_length"`
		Limit         struct {
			Context int64 `json:"context"`
		} `json:"limit"`
		Modalities struct {
			Input  []string `json:"input"`
			Output []string `json:"output"`
		} `json:"modalities"`
		Architecture struct {
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
		Cost struct {
			Input      float64 `json:"input"`
			Output     float64 `json:"output"`
			CacheRead  float64 `json:"cache_read"`
			CacheWrite float64 `json:"cache_write"`
		} `json:"cost"`
		Pricing struct {
			Prompt          string `json:"prompt"`
			Completion      string `json:"completion"`
			InputCacheRead  string `json:"input_cache_read"`
			InputCacheWrite string `json:"input_cache_write"`
		} `json:"pricing"`
		SupportedParameters []string `json:"supported_parameters"`
		TopProvider         string   `json:"top_provider"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return Model{}, err
	}

	ctxLen := m.Limit.Context
	if ctxLen <= 0 {
		ctxLen = m.ContextLength
	}

	in := m.Modalities.Input
	if len(in) == 0 {
		in = m.Architecture.InputModalities
	}
	outMod := m.Modalities.Output
	if len(outMod) == 0 {
		outMod = m.Architecture.OutputModalities
	}

	model := Model{
		ID:                  m.ID,
		Name:                m.Name,
		ContextLength:       ctxLen,
		InputModalities:     in,
		OutputModalities:    outMod,
		SupportedParameters: m.SupportedParameters,
		Provider:            m.TopProvider,
		Pricing: ModelPricing{
			Prompt:          m.Cost.Input,
			Completion:      m.Cost.Output,
			InputCacheRead:  m.Cost.CacheRead,
			InputCacheWrite: m.Cost.CacheWrite,
		},
	}

	// Legacy string-based pricing overrides zero values when present.
	if s := strings.TrimSpace(m.Pricing.Prompt); s != "" {
		if f, err := parsePrice(s); err == nil {
			model.Pricing.Prompt = f
		}
	}
	if s := strings.TrimSpace(m.Pricing.Completion); s != "" {
		if f, err := parsePrice(s); err == nil {
			model.Pricing.Completion = f
		}
	}
	if s := strings.TrimSpace(m.Pricing.InputCacheRead); s != "" {
		if f, err := parsePrice(s); err == nil {
			model.Pricing.InputCacheRead = f
		}
	}
	if s := strings.TrimSpace(m.Pricing.InputCacheWrite); s != "" {
		if f, err := parsePrice(s); err == nil {
			model.Pricing.InputCacheWrite = f
		}
	}

	return model, nil
}

// parsePrice converts a decimal price string like "0.00003" to a float64.
func parsePrice(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
