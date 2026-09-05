package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// Provider represents an AI/LLM provider registered in the Cosca Runtime.
type Provider struct {
	// Name is the unique provider identifier (e.g. "openai", "anthropic").
	Name string `json:"name"`
	// Status is the provider's connection status (available, configured, etc.).
	Status string `json:"status"`
	// Model is the default model for this provider.
	Model string `json:"model"`
	// Active indicates whether this is the currently active provider.
	Active bool `json:"active"`
	// Configured indicates whether the provider has valid credentials.
	Configured bool `json:"configured"`
	// BaseURL is the provider's API base URL.
	BaseURL string `json:"base_url"`
	// APIVersion is the provider's API version.
	APIVersion string `json:"api_version"`
	// Models lists the available models for this provider.
	Models []string `json:"models,omitempty"`
	// Capabilities lists the provider's capabilities (chat, embeddings, etc.).
	Capabilities []string `json:"capabilities,omitempty"`
}

// TestResult holds the outcome of a provider connectivity test.
type TestResult struct {
	// ResponseTime is the round-trip time of the test request.
	ResponseTime string `json:"response_time"`
	// Model is the model tested against.
	Model string `json:"model"`
	// Status is the test result (reachable, configured, timeout, etc.).
	Status string `json:"status"`
}

// ProviderStatus represents the overall status of the provider subsystem.
type ProviderStatus struct {
	// Active is the name of the currently active provider.
	Active string `json:"active"`
	// Configured is the count of providers with valid credentials.
	Configured int `json:"configured"`
	// Available is the total count of available providers.
	Available int `json:"available"`
	// Statuses are per-provider status strings.
	Statuses []string `json:"statuses"`
}

// setActiveRequest is the body for setting the active provider.
type setActiveRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
}

// =============================================================================
// ProvidersSDK
// =============================================================================

// ProvidersSDK provides methods for managing AI/LLM providers in the Cosca
// Runtime. It supports listing, inspecting, testing, and activating
// providers such as OpenAI, Anthropic, Ollama, etc.
type ProvidersSDK struct {
	client *Client
}

// List returns all available AI providers with their current status and
// configuration.
func (s *ProvidersSDK) List() ([]Provider, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/providers",
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var providers []Provider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, fmt.Errorf("failed to decode providers list: %w", err)
	}

	return providers, nil
}

// Get returns detailed information about a specific provider by name.
func (s *ProvidersSDK) Get(name string) (*Provider, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/providers/%s", name),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var provider Provider
	if err := json.NewDecoder(resp.Body).Decode(&provider); err != nil {
		return nil, fmt.Errorf("failed to decode provider: %w", err)
	}

	return &provider, nil
}

// Test performs a connectivity test against the specified provider. It
// verifies the provider is reachable and returns timing and status
// information.
func (s *ProvidersSDK) Test(name string) (*TestResult, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		fmt.Sprintf("/v1/providers/%s/test", name),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var result TestResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode test result: %w", err)
	}

	return &result, nil
}

// SetActive sets the specified provider (and optional model) as the active
// AI provider for the runtime. Returns the updated provider subsystem
// status.
func (s *ProvidersSDK) SetActive(name string, model string) (*ProviderStatus, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	body := setActiveRequest{Provider: name, Model: model}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPut,
		"/v1/providers/active",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var status ProviderStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode provider status: %w", err)
	}

	return &status, nil
}
