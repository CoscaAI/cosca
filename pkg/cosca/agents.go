package cosca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// Agent represents an Cosca agent with its capabilities and metadata.
type Agent struct {
	// Name is the unique agent identifier.
	Name string `json:"name"`
	// Role describes the agent's functional role.
	Role string `json:"role"`
	// Mission describes the agent's high-level mission.
	Mission string `json:"mission,omitempty"`
	// Status is the agent's current status (active, inactive, etc.).
	Status string `json:"status"`
	// Version is the agent's version string.
	Version string `json:"version"`
	// Department is the agent's organisational department.
	Department string `json:"department"`
	// ReportsTo indicates the agent's direct report.
	ReportsTo string `json:"reports_to,omitempty"`
	// Description provides a longer description of the agent.
	Description string `json:"description,omitempty"`
	// Responsibilities lists the agent's key responsibilities.
	Responsibilities []string `json:"responsibilities,omitempty"`
	// Dependencies are the agent's declared dependencies.
	Dependencies []AgentTableRow `json:"dependencies,omitempty"`
	// Inputs are the agent's expected inputs.
	Inputs []AgentTableRow `json:"inputs,omitempty"`
	// Outputs are the agent's expected outputs.
	Outputs []AgentTableRow `json:"outputs,omitempty"`
}

// AgentTableRow represents a key-value row in the agent's metadata tables.
type AgentTableRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// =============================================================================
// AgentsSDK
// =============================================================================

// AgentsSDK provides methods for discovering and inspecting Cosca agents.
// Agents are specialised AI assistants that perform specific tasks within
// the Cosca ecosystem.
type AgentsSDK struct {
	client *Client
}

// List returns all available agents registered in the runtime.
func (s *AgentsSDK) List() ([]Agent, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/agents",
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

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to decode agents list: %w", err)
	}

	return agents, nil
}

// Search finds agents matching the given query string. The search is
// case-insensitive and matches against name, role, department, and
// description fields.
func (s *AgentsSDK) Search(query string) ([]Agent, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/agents/search?q=%s", query),
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

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to decode agents search results: %w", err)
	}

	return agents, nil
}

// Get returns a single agent by name (case-insensitive). Returns an error
// if no agent matches the given name.
func (s *AgentsSDK) Get(name string) (*Agent, error) {
	if name == "" {
		return nil, fmt.Errorf("agent name is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/agents/%s", name),
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

	var agent Agent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent: %w", err)
	}

	return &agent, nil
}
