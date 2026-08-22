// Package cosca provides the public SDK for interacting with the Cosca Runtime.
package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// ContextScope defines the visibility scope of a context entry.
type ContextScope string

// Predefined context scopes.
const (
	ContextScopeGlobal  ContextScope = "global"
	ContextScopeProject ContextScope = "project"
	ContextScopeSession ContextScope = "session"
)

// ContextEntry represents a single context data point.
type ContextEntry struct {
	Key       string       `json:"key"`
	Value     string       `json:"value"`
	Scope     ContextScope `json:"scope"`
	UpdatedAt string       `json:"updatedAt"`
}

// ContextSDK provides context management operations.
type ContextSDK struct {
	client *Client
}

// Get retrieves a context entry by key.
func (c *ContextSDK) Get(ctx context.Context, key string) (*ContextEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/context/%s", c.client.BaseURL(), key), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get context: %s", resp.Status)
	}
	var entry ContextEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode context: %w", err)
	}
	return &entry, nil
}

// Set stores a context entry.
func (c *ContextSDK) Set(ctx context.Context, key, value string, scope ContextScope) error {
	body, err := json.Marshal(map[string]string{
		"key":   key,
		"value": value,
		"scope": string(scope),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/context", c.client.BaseURL()), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("set context: %s", resp.Status)
	}
	return nil
}

// List returns all context entries for a given scope.
func (c *ContextSDK) List(ctx context.Context, scope ContextScope) ([]ContextEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/context?scope=%s", c.client.BaseURL(), scope), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list context: %s", resp.Status)
	}
	var entries []ContextEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode context: %w", err)
	}
	return entries, nil
}

// Delete removes a context entry.
func (c *ContextSDK) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/v1/context/%s", c.client.BaseURL(), key), nil)
	if err != nil {
		return err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete context: %s", resp.Status)
	}
	return nil
}

// Build triggers a context rebuild.
func (c *ContextSDK) Build(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/context/build", c.client.BaseURL()), nil)
	if err != nil {
		return err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("build context: %s", resp.Status)
	}
	return nil
}

// GetCurrent returns the current active context for the session or project.
func (c *ContextSDK) GetCurrent(ctx context.Context) (*ContextEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/context/current", c.client.BaseURL()), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get current context: %s", resp.Status)
	}
	var entry ContextEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode context: %w", err)
	}
	return &entry, nil
}

// Clear removes all context entries for the session scope, effectively
// resetting the current context.
func (c *ContextSDK) Clear(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/v1/context", c.client.BaseURL()), nil)
	if err != nil {
		return err
	}
	resp, err := c.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clear context: %s", resp.Status)
	}
	return nil
}

// CreateBuilder returns a context builder configuration.
func (c *ContextSDK) CreateBuilder(_ context.Context, name string) (*ContextBuilder, error) {
	return &ContextBuilder{Name: name, CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

// ContextBuilder is a placeholder for the context builder model.
type ContextBuilder struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}
