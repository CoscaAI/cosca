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

// Workflow represents a structured process with defined steps that can be
// executed by the Cosca Runtime.
type Workflow struct {
	// Name is the unique workflow identifier.
	Name string `json:"name"`
	// Description explains the workflow's purpose.
	Description string `json:"description"`
	// Version is the workflow's semantic version.
	Version string `json:"version"`
	// Status is the workflow's status (active, inactive, disabled).
	Status string `json:"status"`
	// Enabled indicates whether the workflow can be executed.
	Enabled bool `json:"enabled"`
	// Steps is the number of steps in the workflow.
	Steps int `json:"steps"`
	// StepList contains the detailed step definitions.
	StepList []WorkflowStep `json:"step_list,omitempty"`
	// Inputs defines the workflow's expected inputs.
	Inputs []WorkflowIO `json:"inputs,omitempty"`
	// Outputs defines the workflow's expected outputs.
	Outputs []WorkflowIO `json:"outputs,omitempty"`
}

// WorkflowStep represents a single step within a workflow.
type WorkflowStep struct {
	// Name is the step's display name.
	Name string `json:"name"`
	// Description explains what the step does.
	Description string `json:"description"`
	// Agent is the agent responsible for this step.
	Agent string `json:"agent"`
	// Timeout is the maximum duration allowed for this step.
	Timeout string `json:"timeout"`
}

// WorkflowIO represents a workflow input or output specification.
type WorkflowIO struct {
	// Name is the parameter name.
	Name string `json:"name"`
	// Type is the parameter data type.
	Type string `json:"type"`
	// Required indicates whether the parameter is mandatory.
	Required bool `json:"required"`
}

// WorkflowResult holds the outcome of a workflow execution.
type WorkflowResult struct {
	// Status is the final status (completed, failed).
	Status string `json:"status"`
	// Duration is the total wall-clock execution time.
	Duration string `json:"duration"`
	// StepsCompleted is how many steps finished successfully.
	StepsCompleted int `json:"steps_completed"`
	// TotalSteps is the total number of steps in the workflow.
	TotalSteps int `json:"total_steps"`
	// Outputs are the workflow's output specifications.
	Outputs []WorkflowIO `json:"outputs,omitempty"`
}

// =============================================================================
// WorkflowsSDK
// =============================================================================

// WorkflowsSDK provides methods for discovering, inspecting, and executing
// Cosca workflows. Workflows define repeatable processes with defined steps
// that agents carry out.
type WorkflowsSDK struct {
	client *Client
}

// List returns all available workflows registered in the runtime.
func (s *WorkflowsSDK) List() ([]Workflow, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/workflows",
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

	var workflows []Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows list: %w", err)
	}

	return workflows, nil
}

// Search finds workflows matching the given query string. The search is
// case-insensitive and matches against name and description fields.
func (s *WorkflowsSDK) Search(query string) ([]Workflow, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/workflows/search?q=%s", query),
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

	var workflows []Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows search results: %w", err)
	}

	return workflows, nil
}

// Get returns a single workflow by name (case-insensitive). Returns an
// error if no workflow matches the given name.
func (s *WorkflowsSDK) Get(name string) (*Workflow, error) {
	if name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/workflows/%s", name),
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

	var workflow Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode workflow: %w", err)
	}

	return &workflow, nil
}

// Run executes a workflow by name. The workflow's steps are executed
// sequentially, and the result contains the completion status, timing,
// and step counts.
func (s *WorkflowsSDK) Run(ctx context.Context, name string) (*WorkflowResult, error) {
	if name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	req, err := s.client.newRequest(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/v1/workflows/%s/run", name),
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

	var result WorkflowResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode workflow result: %w", err)
	}

	return &result, nil
}
