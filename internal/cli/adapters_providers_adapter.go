package cli

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/providers"
)

// Providers Adapter
// =============================================================================

// providerManagerAdapter wraps providers.Manager to match CLI expected API.
type providerManagerAdapter struct {
	inner *providers.Manager
}

// newProviderManagerAdapter creates a provider manager adapter.
func newProviderManagerAdapter() *providerManagerAdapter {
	return &providerManagerAdapter{inner: providers.NewManager()}
}

// List returns the list of providers. CLI expects this method.
func (a *providerManagerAdapter) List() []ProviderInfoEx {
	if a.inner == nil {
		return nil
	}
	list := a.inner.List()
	result := make([]ProviderInfoEx, len(list))
	for i, p := range list {
		result[i] = ProviderInfoEx{
			Name:         p.Name,
			Status:       p.Status,
			Model:        p.Model,
			Active:       p.Active,
			Configured:   p.Configured,
			BaseURL:      p.BaseURL,
			Models:       p.Models,
			Capabilities: p.Capabilities,
		}
	}
	return result
}

// SetActive sets the active provider. CLI expects (name, model) params.
func (a *providerManagerAdapter) SetActive(name, model string) error {
	if a.inner == nil {
		return fmt.Errorf("provider manager not available")
	}
	return a.inner.SetActive(name, model)
}

// Test tests a provider connection. CLI expects (result, error).
func (a *providerManagerAdapter) Test(name string) (ProviderTestResult, error) {
	if a.inner == nil {
		return ProviderTestResult{
			ResponseTime: "0s",
			Model:        name,
			Status:       "unavailable",
		}, fmt.Errorf("provider manager not available")
	}
	result, err := a.inner.Test(name)
	if err != nil {
		return ProviderTestResult{
			ResponseTime: "0s",
			Model:        name,
			Status:       "error",
		}, err
	}
	return ProviderTestResult{
		ResponseTime: result.ResponseTime,
		Model:        result.Model,
		Status:       result.Status,
	}, nil
}

// Info returns provider info. CLI expects (info, error).
func (a *providerManagerAdapter) Info(name string) (ProviderInfoEx, error) {
	if a.inner == nil {
		return ProviderInfoEx{Name: name}, fmt.Errorf("provider manager not available")
	}
	info, err := a.inner.Info(name)
	if err != nil {
		return ProviderInfoEx{Name: name}, err
	}
	return ProviderInfoEx{
		Name:         info.Name,
		Status:       info.Status,
		Model:        info.Model,
		Active:       info.Active,
		Configured:   info.Configured,
		BaseURL:      info.BaseURL,
		Models:       info.Models,
		Capabilities: info.Capabilities,
	}, nil
}

// ProviderInfoEx holds provider info.
type ProviderInfoEx struct {
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

// ProviderTestResult holds provider test results.
type ProviderTestResult struct {
	ResponseTime string `json:"response_time"`
	Model        string `json:"model"`
	Status       string `json:"status"`
}

// =============================================================================
