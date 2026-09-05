/*
 */

// Package cosca provides the public SDK for interacting with the Cosca Runtime
// programmatically. It is the primary entry point for both internal CLI
// components and external plugin developers / integrators.
//
// The SDK is organised into sub-SDKs covering each major subsystem:
//   - Knowledge  (indexing, searching, knowledge-base management)
//   - Memory     (agent memory storage, retrieval, snapshots)
//   - Context    (build, query, and manage runtime context)
//   - Runtime    (start, stop, health, status)
//   - Plugin     (install, uninstall, list, inspect plugins)
//   - Discovery  (project, workspace, editor auto-detection)
//   - Graph      (entity relations, graph stats, exports)
//
// Basic usage:
//
//	client, err := cosca.NewClient(cosca.ClientConfig{
//	    RuntimeAddr: "localhost:9090",
//	    APIKey:      "my-api-key",
//	    Timeout:     30 * time.Second,
//	    MaxRetries:  3,
//	})
//	if err != nil { /* handle */ }
//	defer client.Close()
//
//	// Use individual sub-SDKs
//	results, err := client.Knowledge.Search("query", cosca.SearchOptions{Limit: 10})
//	snap, err  := client.Memory.CreateSnapshot("pre-deploy")
//	status, err := client.Runtime.Status()
package cosca

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog"
)

// =============================================================================
// Client Configuration
// =============================================================================

// ClientConfig defines the configuration for creating a new Cosca SDK Client.
type ClientConfig struct {
	// RuntimeAddr is the address of the Cosca Runtime in host:port form.
	// Defaults to "localhost:14120" (porta real do servidor REST — api/rest).
	RuntimeAddr string `json:"runtimeAddr" yaml:"runtimeAddr"`

	// APIKey is the authentication key for the runtime API.
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Timeout is the default timeout for SDK operations.
	// Defaults to 30 seconds.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxRetries is the maximum number of retries for transient failures.
	// Defaults to 3.
	MaxRetries int `json:"maxRetries" yaml:"maxRetries"`

	// TLSConfig is an optional TLS configuration for secure connections.
	TLSConfig *tls.Config `json:"-" yaml:"-"`

	// Logger is an optional zerolog logger. If nil, a no-op logger is used.
	Logger *zerolog.Logger `json:"-" yaml:"-"`

	// HeartbeatInterval defines how often the client sends heartbeats.
	// Defaults to 15 seconds. Set to 0 to disable.
	HeartbeatInterval time.Duration `json:"heartbeatInterval" yaml:"heartbeatInterval"`

	// UserAgent is the HTTP User-Agent header value sent with requests.
	UserAgent string `json:"userAgent,omitempty" yaml:"userAgent,omitempty"`
}

// setDefaults applies sensible defaults for missing configuration fields.
func (c *ClientConfig) setDefaults() {
	if c.RuntimeAddr == "" {
		c.RuntimeAddr = "localhost:14120"
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 15 * time.Second
	}
	if c.UserAgent == "" {
		c.UserAgent = "Cosca-SDK/1.0"
	}
}

// Validate checks that the configuration is valid.
func (c *ClientConfig) Validate() error {
	var errs []string

	if c.RuntimeAddr == "" {
		errs = append(errs, "runtime address is required")
	}
	if _, _, err := net.SplitHostPort(c.RuntimeAddr); err != nil {
		errs = append(errs, fmt.Sprintf("invalid runtime address %q: %v", c.RuntimeAddr, err))
	}
	if c.Timeout < 0 {
		errs = append(errs, "timeout must be non-negative")
	}
	if c.MaxRetries < 0 {
		errs = append(errs, "max retries must be non-negative")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid client config: %s", strings.Join(errs, "; "))
	}
	return nil
}

// =============================================================================
// Connection State
// =============================================================================

// connState tracks the lifecycle of the client connection.
type connState int

const (
	connStateDisconnected connState = iota
	connStateConnecting
	connStateConnected
	connStateReconnecting
	connStateClosed
)

func (s connState) String() string {
	switch s {
	case connStateDisconnected:
		return "disconnected"
	case connStateConnecting:
		return "connecting"
	case connStateConnected:
		return "connected"
	case connStateReconnecting:
		return "reconnecting"
	case connStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// =============================================================================
// CoscaError
// =============================================================================

// CoscaError represents a structured error returned by the Cosca Runtime API.
// It captures the HTTP status code, an application error code, and any
// additional details the server may include.
type CoscaError struct {
	// StatusCode is the HTTP status code returned by the server.
	StatusCode int `json:"-"`
	// Code is the application-level error code (e.g. "NOT_FOUND", "INVALID_INPUT").
	Code string `json:"code"`
	// Message is a human-readable description of the error.
	Message string `json:"message"`
	// Details contains additional structured error information.
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *CoscaError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Cosca API error (HTTP %d): [%s] %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("Cosca API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// decodeError reads an error response body and returns an CoscaError.
// This is the shared error decoding helper used by all sub-SDKs.
func (c *Client) decodeError(resp *http.Response) *CoscaError {
	ae := &CoscaError{
		StatusCode: resp.StatusCode,
	}
	if err := json.NewDecoder(resp.Body).Decode(ae); err != nil {
		ae.Code = "UNKNOWN"
		ae.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return ae
}

// =============================================================================
// Client
// =============================================================================

// Client is the main entry point for the Cosca SDK. It holds the configuration
// and provides access to all sub-SDKs.
type Client struct {
	config  ClientConfig
	mu      sync.RWMutex
	state   connState
	baseURL string
	httpCli *http.Client
	log     zerolog.Logger
	stopHbt chan struct{} // signals heartbeat goroutine to stop
	hbtWg   sync.WaitGroup

	// Sub-SDKs initialised by NewClient.
	Knowledge     *KnowledgeSDK
	Memory        *MemorySDK
	Context       *ContextSDK
	Runtime       *RuntimeSDK
	Plugin        *PluginSDK
	Discovery     *DiscoverySDK
	Graph         *GraphSDK
	Agents        *AgentsSDK
	Skills        *SkillsSDK
	Providers     *ProvidersSDK
	Workflows     *WorkflowsSDK
	Orchestration *OrchestrationSDK
}

// NewClient creates a new Cosca SDK Client with the given configuration.
// It validates the config, applies defaults, and initialises all sub-SDKs.
// Call Close() when the client is no longer needed to release resources.
func NewClient(config ClientConfig) (*Client, error) {
	config.setDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	log := zerolog.Nop()
	if config.Logger != nil {
		log = *config.Logger
	}

	transport := &http.Transport{
		TLSClientConfig: config.TLSConfig,
	}

	httpCli := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	// Determine the base URL from the runtime address.
	baseURL := fmt.Sprintf("http://%s", config.RuntimeAddr)
	if config.TLSConfig != nil {
		baseURL = fmt.Sprintf("https://%s", config.RuntimeAddr)
	}

	c := &Client{
		config:  config,
		state:   connStateDisconnected,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpCli: httpCli,
		log:     log,
		stopHbt: make(chan struct{}),
	}

	// Initialise sub-SDKs.
	c.Knowledge = &KnowledgeSDK{client: c}
	c.Memory = &MemorySDK{client: c}
	c.Context = &ContextSDK{client: c}
	c.Runtime = &RuntimeSDK{client: c}
	c.Plugin = &PluginSDK{client: c}
	c.Discovery = &DiscoverySDK{client: c}
	c.Graph = &GraphSDK{client: c}
	c.Agents = &AgentsSDK{client: c}
	c.Skills = &SkillsSDK{client: c}
	c.Providers = &ProvidersSDK{client: c}
	c.Workflows = &WorkflowsSDK{client: c}
	c.Orchestration = &OrchestrationSDK{client: c}

	return c, nil
}

// =============================================================================
// Connection Management
// =============================================================================

// Connect establishes a connection to the Cosca Runtime. It sends a health-check
// ping to verify connectivity and starts the heartbeat goroutine.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.state == connStateConnected {
		c.mu.Unlock()
		return nil
	}
	c.state = connStateConnecting
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.state == connStateConnecting {
			c.state = connStateDisconnected
		}
		c.mu.Unlock()
	}()

	// Verify connectivity with a health check.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health-check request: %w", err)
	}
	c.setAuthHeaders(req)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to runtime at %s: %w", c.baseURL, err)
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("runtime health check failed (HTTP %d): %s",
			resp.StatusCode, string(body))
	}

	c.mu.Lock()
	c.state = connStateConnected
	c.mu.Unlock()

	// Start heartbeat if interval > 0.
	if c.config.HeartbeatInterval > 0 {
		c.mu.Lock()
		if c.state == connStateClosed {
			c.mu.Unlock()
			return fmt.Errorf("cannot start heartbeat on closed client")
		}
		// A fresh stop channel: a previous stopHeartbeat (e.g. during a
		// reconnect) closed the old one, which would kill the new loop
		// immediately if reused.
		c.stopHbt = make(chan struct{})
		c.mu.Unlock()
		c.hbtWg.Add(1)
		go c.heartbeatLoop()
	}

	c.log.Info().Str("addr", c.config.RuntimeAddr).Msg("connected to Cosca runtime")
	return nil
}

// Reconnect force-disconnects and re-establishes the connection.
func (c *Client) Reconnect(ctx context.Context) error {
	c.stopHeartbeat()
	c.mu.Lock()
	c.state = connStateReconnecting
	c.mu.Unlock()

	c.log.Info().Msg("reconnecting to Cosca runtime...")
	return c.Connect(ctx)
}

// Close tears down the client, stops the heartbeat, and releases resources.
func (c *Client) Close() error {
	c.stopHeartbeat()
	c.mu.Lock()
	c.state = connStateClosed
	c.mu.Unlock()

	// Release idle keep-alive connections held by the transport so
	// servers/clients can shut down promptly.
	if c.httpCli != nil {
		c.httpCli.CloseIdleConnections()
	}

	c.log.Info().Msg("Cosca SDK client closed")
	return nil
}

// ConnectionState returns the current connection state.
func (c *Client) ConnectionState() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.String()
}

// IsConnected reports whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == connStateConnected
}

// =============================================================================
// Heartbeat
// =============================================================================

// heartbeatLoop sends periodic heartbeats to the runtime.
func (c *Client) heartbeatLoop() {
	defer c.hbtWg.Done()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.sendHeartbeat(); err != nil {
				c.log.Warn().Err(err).Msg("heartbeat failed")
				// On heartbeat failure, attempt reconnect in the background.
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := c.Reconnect(ctx); err != nil {
						c.log.Error().Err(err).Msg("reconnect after heartbeat failure")
					}
				}()
				return
			}
		case <-c.stopHbt:
			return
		}
	}
}

// sendHeartbeat sends a single heartbeat request.
func (c *Client) sendHeartbeat() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/health", nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return err
	}
	// Drain the body before closing so the keep-alive connection returns to
	// the idle pool instead of being held "active" by the server.
	_, _ = io.Copy(io.Discard, resp.Body)
	safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected heartbeat status: %d", resp.StatusCode)
	}
	return nil
}

// stopHeartbeat signals the heartbeat goroutine to stop.
// The close is guarded by c.mu so concurrent callers (Close() racing the
// reconnect goroutine spawned on heartbeat failure) cannot double-close the
// channel, which would panic.
func (c *Client) stopHeartbeat() {
	c.mu.Lock()
	select {
	case <-c.stopHbt:
		// Already closed.
	default:
		close(c.stopHbt)
	}
	c.mu.Unlock()
	c.hbtWg.Wait()
}

// =============================================================================
// HTTP Helpers
// =============================================================================

// newRequest creates an http.Request with the SDK's default headers.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuthHeaders(req)
	return req, nil
}

// setAuthHeaders applies authentication and user-agent headers.
func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
}

// doRequest performs an HTTP request with automatic retry logic.
// The request body is re-created on every attempt via GetBody so that retries
// work for requests with a body (the first attempt consumes the original body).
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Restore the body on every attempt after the first. Without this,
		// retrying a POST fails with "ContentLength=N with Body length 0"
		// because http.Client consumes the request body on the first Do.
		if attempt > 0 && req.Body != nil && req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to reset request body for retry: %w", err)
			}
		}

		resp, err = c.httpCli.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Close the response body on failure to prevent leaks.
		if resp != nil {
			safe.Close(resp.Body)
		}

		// If this was the last attempt, return the error.
		if attempt == c.config.MaxRetries {
			if err != nil {
				return nil, fmt.Errorf("request failed after %d retries: %w",
					c.config.MaxRetries, err)
			}
			return nil, fmt.Errorf("request failed with status %d after %d retries",
				resp.StatusCode, c.config.MaxRetries)
		}

		// Exponential backoff.
		backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}

		c.log.Debug().
			Int("attempt", attempt+1).
			Int("maxRetries", c.config.MaxRetries).
			Dur("backoff", backoff).
			Msg("retrying request")

		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("request failed: %w", err)
}

// Config returns a copy of the client's configuration.
func (c *Client) Config() ClientConfig {
	return c.config
}

// BaseURL returns the base URL of the runtime.
func (c *Client) BaseURL() string {
	return c.baseURL
}
