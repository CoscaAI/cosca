package cosca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// RuntimeStatus represents the current state of the Cosca Runtime.
type RuntimeStatus struct {
	// State is the runtime state (running, stopped, starting, error, etc.).
	State string `json:"state"`
	// Version is the runtime version string.
	Version string `json:"version"`
	// Uptime is how long the runtime has been running.
	Uptime time.Duration `json:"uptime"`
	// StartedAt is when the runtime was started.
	StartedAt time.Time `json:"startedAt"`
	// ActiveAgents is the number of currently active agents.
	ActiveAgents int `json:"activeAgents"`
	// ActiveWorkflows is the number of running workflows.
	ActiveWorkflows int `json:"activeWorkflows"`
	// LoadedPlugins is the number of loaded plugins.
	LoadedPlugins int `json:"loadedPlugins"`
	// MemoryUsage is the current memory usage in bytes.
	MemoryUsage int64 `json:"memoryUsage"`
	// Goroutines is the number of goroutines in the runtime process.
	Goroutines int `json:"goroutines"`
	// Mode is the runtime mode (development, production, etc.).
	Mode string `json:"mode"`
	// Metadata holds additional runtime metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HealthReport provides a detailed health snapshot of the runtime and its
// subsystems.
type HealthReport struct {
	// Status is the overall health status (healthy, degraded, unhealthy).
	Status string `json:"status"`
	// Timestamp is when the health check was performed.
	Timestamp time.Time `json:"timestamp"`
	// Version is the runtime version.
	Version string `json:"version"`
	// Uptime is the runtime uptime duration.
	Uptime time.Duration `json:"uptime"`

	// Subsystems holds health information for each subsystem.
	Subsystems map[string]SubsystemHealth `json:"subsystems"`

	// Checks holds individual health check results.
	Checks []HealthCheck `json:"checks"`

	// Warnings list non-critical issues.
	Warnings []string `json:"warnings,omitempty"`
}

// SubsystemHealth reports the health of a single subsystem.
type SubsystemHealth struct {
	// Status is the subsystem status (healthy, degraded, unhealthy).
	Status string `json:"status"`
	// Message provides additional context.
	Message string `json:"message,omitempty"`
	// Latency is the response time of the subsystem check.
	Latency time.Duration `json:"latency,omitempty"`
	// Error is the error message if the subsystem is unhealthy.
	Error string `json:"error,omitempty"`
}

// HealthCheck is a single health check probe result.
type HealthCheck struct {
	// Name is the check name.
	Name string `json:"name"`
	// Status is the check status (pass, warn, fail).
	Status string `json:"status"`
	// Message provides detail about the check.
	Message string `json:"message,omitempty"`
	// Duration is how long the check took.
	Duration time.Duration `json:"duration,omitempty"`
}

// =============================================================================
// RuntimeSDK
// =============================================================================

// RuntimeSDK provides methods for controlling and monitoring the Cosca Runtime.
// It supports starting and stopping the runtime, querying status, and
// performing detailed health checks.
type RuntimeSDK struct {
	client *Client
}

// Start initialises and starts the Cosca Runtime. If the runtime is already
// running, this is a no-op.
func (s *RuntimeSDK) Start() error {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/runtime/start",
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return s.decodeError(resp)
	}

	return nil
}

// Stop gracefully shuts down the Cosca Runtime and all its subsystems.
func (s *RuntimeSDK) Stop() error {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/runtime/stop",
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return s.decodeError(resp)
	}

	return nil
}

// Status returns the current runtime status including version, uptime,
// active agents, workflows, and memory usage.
func (s *RuntimeSDK) Status() (RuntimeStatus, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/status",
		nil,
	)
	if err != nil {
		return RuntimeStatus{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return RuntimeStatus{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return RuntimeStatus{}, s.decodeError(resp)
	}

	var status RuntimeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return RuntimeStatus{}, fmt.Errorf("failed to decode runtime status: %w", err)
	}

	return status, nil
}

// Health performs a comprehensive health check against the runtime and all
// its subsystems (database, knowledge index, providers, plugins, etc.).
func (s *RuntimeSDK) Health() (HealthReport, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/health",
		nil,
	)
	if err != nil {
		return HealthReport{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return HealthReport{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return HealthReport{}, s.decodeError(resp)
	}

	var report HealthReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return HealthReport{}, fmt.Errorf("failed to decode health report: %w", err)
	}

	return report, nil
}

// decodeError reads an error response body and returns it as a formatted error.
func (s *RuntimeSDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
