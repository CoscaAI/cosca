package cosca

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Test Helpers
// =============================================================================

// testPortBase is the base for fixed per-test ports. We avoid the OS ephemeral
// port range (32768-60999) entirely: under heavy parallel load the kernel
// recycles ephemeral ports quickly, so a request from test A can dial into the
// listener that test B just got assigned on the same recycled port, yielding
// flaky "connection refused" / EOF. Fixed, unique ports BELOW the ephemeral
// range are immune to that race. The range below must not collide with the
// runtime (14120-14125), ollama (11434) or the vite dev server (5173).
const (
	testPortBase = 20000
	testPortSpan = 5000 // 20000..24999
)

var testPortCounter uint32 // atomic counter for unique per-test ports

// newTestClient creates a Client backed by the given httptest handler.
// Retries are disabled so tests are hermetic (no retry backoff under load),
// and each test listens on a unique fixed port to avoid ephemeral-port
// recycling races between parallel tests.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	// Grab a unique fixed port. Retry on the rare collision (e.g. a leftover
	// socket in TIME_WAIT without SO_REUSEADDR visible on this host).
	// Note: we build the httptest.Server directly instead of using
	// NewUnstartedServer so no internal listener is created and leaked when
	// we override s.Listener (with -count=N, leaked fds would accumulate
	// across the N runs of the same test binary).
	var (
		server *httptest.Server
		addr   string
	)
	for i := 0; ; i++ {
		n := atomic.AddUint32(&testPortCounter, 1)
		addr = fmt.Sprintf("127.0.0.1:%d", testPortBase+(int(n)%testPortSpan))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			server = &httptest.Server{
				Listener: ln,
				Config:   &http.Server{Handler: handler},
			}
			break
		}
		if i > 100 {
			t.Fatalf("failed to allocate a test port for httptest server: %v", err)
		}
	}
	server.Start()
	t.Cleanup(server.Close)

	// Extract host:port from the server URL for ClientConfig validation.
	addr = strings.TrimPrefix(server.URL, "http://")

	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	// The package-internal defaults turn MaxRetries<=0 into 3; tests must not
	// retry (no backoff under parallel load). Disable it for real.
	client.config.MaxRetries = 0

	return client
}

// assertMethodPath is a helper that checks the HTTP method and path of a request.
func assertMethodPath(t *testing.T, r *http.Request, method, path string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("expected method %q, got %q", method, r.Method)
	}
	if r.URL.Path != path {
		t.Errorf("expected path %q, got %q", path, r.URL.Path)
	}
}

// writeJSON writes v as JSON to the response writer. Fails the test on error.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("failed to write JSON response: %v", err)
	}
}

// writeErrorJSON writes an CoscaError-like JSON body with the given status.
func writeErrorJSON(t *testing.T, w http.ResponseWriter, status int, code, message string) {
	t.Helper()
	writeJSON(t, w, status, map[string]string{"code": code, "message": message})
}

// =============================================================================
// Client / Config Tests
// =============================================================================

func TestNewClient_ValidConfig(t *testing.T) {

	client, err := NewClient(ClientConfig{
		RuntimeAddr: "localhost:9090",
		MaxRetries:  1,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://localhost:9090" {
		t.Errorf("expected baseURL %q, got %q", "http://localhost:9090", client.baseURL)
	}

	// Verify all sub-SDKs are initialized.
	if client.Agents == nil {
		t.Error("expected Agents SDK to be initialized")
	}
	if client.Skills == nil {
		t.Error("expected Skills SDK to be initialized")
	}
	if client.Providers == nil {
		t.Error("expected Providers SDK to be initialized")
	}
	if client.Workflows == nil {
		t.Error("expected Workflows SDK to be initialized")
	}
	if client.Orchestration == nil {
		t.Error("expected Orchestration SDK to be initialized")
	}
	if client.Knowledge == nil {
		t.Error("expected Knowledge SDK to be initialized")
	}
	if client.Memory == nil {
		t.Error("expected Memory SDK to be initialized")
	}
	if client.Runtime == nil {
		t.Error("expected Runtime SDK to be initialized")
	}
	if client.Context == nil {
		t.Error("expected Context SDK to be initialized")
	}
	if client.Plugin == nil {
		t.Error("expected Plugin SDK to be initialized")
	}
	if client.Discovery == nil {
		t.Error("expected Discovery SDK to be initialized")
	}
	if client.Graph == nil {
		t.Error("expected Graph SDK to be initialized")
	}

	// Defaults should be applied.
	cfg := client.Config()
	if cfg.UserAgent != "Cosca-SDK/1.0" {
		t.Errorf("expected UserAgent %q, got %q", "Cosca-SDK/1.0", cfg.UserAgent)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("expected HeartbeatInterval %v, got %v", 15*time.Second, cfg.HeartbeatInterval)
	}
}

func TestNewClient_InvalidConfig(t *testing.T) {

	tests := []struct {
		name    string
		config  ClientConfig
		wantErr string
	}{
		{
			name: "invalid host:port format",
			config: ClientConfig{
				RuntimeAddr: "not-valid",
			},
			wantErr: "invalid client config",
		},
		{
			name: "negative timeout",
			config: ClientConfig{
				RuntimeAddr: "localhost:9090",
				Timeout:     -1 * time.Second,
			},
			wantErr: "invalid client config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error to contain %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestClientConfig_SetDefaults(t *testing.T) {

	cfg := ClientConfig{}
	cfg.setDefaults()

	if cfg.RuntimeAddr != "localhost:9090" {
		t.Errorf("expected RuntimeAddr %q, got %q", "localhost:9090", cfg.RuntimeAddr)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout %v, got %v", 30*time.Second, cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries %d, got %d", 3, cfg.MaxRetries)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("expected HeartbeatInterval %v, got %v", 15*time.Second, cfg.HeartbeatInterval)
	}
	if cfg.UserAgent != "Cosca-SDK/1.0" {
		t.Errorf("expected UserAgent %q, got %q", "Cosca-SDK/1.0", cfg.UserAgent)
	}
}

func TestAosError_Error(t *testing.T) {

	tests := []struct {
		name     string
		err      CoscaError
		contains string
	}{
		{
			name:     "with code",
			err:      CoscaError{StatusCode: 404, Code: "NOT_FOUND", Message: "agent not found"},
			contains: "Cosca API error (HTTP 404): [NOT_FOUND] agent not found",
		},
		{
			name:     "without code",
			err:      CoscaError{StatusCode: 500, Message: "internal server error"},
			contains: "Cosca API error (HTTP 500): internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.contains {
				t.Errorf("expected %q, got %q", tt.contains, tt.err.Error())
			}
		})
	}
}

func TestClient_ConnectionState(t *testing.T) {

	client, err := NewClient(ClientConfig{RuntimeAddr: "localhost:9090"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.ConnectionState() != "disconnected" {
		t.Errorf("expected disconnected, got %q", client.ConnectionState())
	}
	if client.IsConnected() {
		t.Error("expected not connected")
	}
}

// =============================================================================
// AgentsSDK Tests
// =============================================================================

func TestAgentsSDK_List(t *testing.T) {

	expected := []Agent{
		{
			Name:             "test-agent",
			Role:             "tester",
			Mission:          "test things",
			Status:           "active",
			Version:          "1.0.0",
			Department:       "qa",
			ReportsTo:        "cto",
			Description:      "A test agent",
			Responsibilities: []string{"testing", "reporting"},
			Dependencies:     []AgentTableRow{{Key: "runtime", Value: ">=1.0"}},
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/agents")
		writeJSON(t, w, http.StatusOK, expected)
	})

	agents, err := c.Agents.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Name != "test-agent" {
		t.Errorf("expected Name %q, got %q", "test-agent", a.Name)
	}
	if a.Role != "tester" {
		t.Errorf("expected Role %q, got %q", "tester", a.Role)
	}
	if a.Mission != "test things" {
		t.Errorf("expected Mission %q, got %q", "test things", a.Mission)
	}
	if a.Status != "active" {
		t.Errorf("expected Status %q, got %q", "active", a.Status)
	}
	if a.Version != "1.0.0" {
		t.Errorf("expected Version %q, got %q", "1.0.0", a.Version)
	}
	if a.Department != "qa" {
		t.Errorf("expected Department %q, got %q", "qa", a.Department)
	}
	if a.ReportsTo != "cto" {
		t.Errorf("expected ReportsTo %q, got %q", "cto", a.ReportsTo)
	}
	if a.Description != "A test agent" {
		t.Errorf("expected Description %q, got %q", "A test agent", a.Description)
	}
	if len(a.Responsibilities) != 2 {
		t.Errorf("expected 2 responsibilities, got %d", len(a.Responsibilities))
	}
}

func TestAgentsSDK_List_Empty(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/agents")
		writeJSON(t, w, http.StatusOK, []Agent{})
	})

	agents, err := c.Agents.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected empty list, got %d agents", len(agents))
	}
}

func TestAgentsSDK_List_Errors(t *testing.T) {

	tests := []struct {
		name         string
		statusCode   int
		body         string
		wantCoscaErr bool
		errText      string
	}{
		{
			name:         "not found",
			statusCode:   http.StatusNotFound,
			body:         `{"code":"NOT_FOUND","message":"no agents available"}`,
			wantCoscaErr: true,
			errText:      "[NOT_FOUND]",
		},
		{
			name:         "forbidden",
			statusCode:   http.StatusForbidden,
			body:         ``,
			wantCoscaErr: true,
			errText:      "HTTP 403",
		},
		{
			name:         "server error",
			statusCode:   http.StatusInternalServerError,
			body:         `{"code":"INTERNAL","message":"boom"}`,
			wantCoscaErr: false,
			errText:      "request failed with status 500",
		},
		{
			name:         "invalid json body",
			statusCode:   http.StatusOK,
			body:         `{invalid}`,
			wantCoscaErr: false,
			errText:      "failed to decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.body != "" {
					w.Write([]byte(tt.body))
				}
			})

			_, err := c.Agents.List()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tt.wantCoscaErr {
				coscaErr, ok := err.(*CoscaError)
				if !ok {
					t.Fatalf("expected *CoscaError, got %T: %v", err, err)
				}
				if !strings.Contains(coscaErr.Error(), tt.errText) {
					t.Errorf("expected error to contain %q, got %q", tt.errText, coscaErr.Error())
				}
			} else {
				if _, ok := err.(*CoscaError); ok {
					t.Errorf("expected non-CoscaError, got *CoscaError: %v", err)
				}
				if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("expected error to contain %q, got %q", tt.errText, err.Error())
				}
			}
		})
	}
}

func TestAgentsSDK_Search(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/agents/search")
		if r.URL.Query().Get("q") == "" {
			t.Error("expected non-empty 'q' query parameter")
		}
		writeJSON(t, w, http.StatusOK, []Agent{{Name: "result-agent", Role: "searcher"}})
	})

	agents, err := c.Agents.Search("test-query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "result-agent" {
		t.Errorf("expected Name %q, got %q", "result-agent", agents[0].Name)
	}
}

func TestAgentsSDK_Search_EmptyQuery(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for empty query")
	})

	_, err := c.Agents.Search("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "search query is required") {
		t.Errorf("expected 'search query is required', got %q", err.Error())
	}
}

func TestAgentsSDK_Get(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/agents/my-agent")
		writeJSON(t, w, http.StatusOK, Agent{Name: "my-agent", Role: "worker"})
	})

	agent, err := c.Agents.Get("my-agent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if agent.Name != "my-agent" {
		t.Errorf("expected Name %q, got %q", "my-agent", agent.Name)
	}
	if agent.Role != "worker" {
		t.Errorf("expected Role %q, got %q", "worker", agent.Role)
	}
}

func TestAgentsSDK_Get_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for empty name")
	})

	_, err := c.Agents.Get("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "agent name is required") {
		t.Errorf("expected 'agent name is required', got %q", err.Error())
	}
}

func TestAgentsSDK_Get_NotFound(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "agent 'ghost' not found")
	})

	_, err := c.Agents.Get("ghost")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code NOT_FOUND, got %q", coscaErr.Code)
	}
}

// =============================================================================
// SkillsSDK Tests
// =============================================================================

func TestSkillsSDK_List(t *testing.T) {

	expected := []Skill{
		{
			Name:         "code-review",
			Description:  "Reviews code for quality",
			Version:      "2.0.0",
			Category:     "development",
			Instructions: "Review the code and provide feedback.",
			Tools:        []SkillTool{{Name: "lint", Description: "Runs linter"}},
			Source:       "file://skills/code-review.md",
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/skills")
		writeJSON(t, w, http.StatusOK, expected)
	})

	skills, err := c.Skills.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "code-review" {
		t.Errorf("expected Name %q, got %q", "code-review", s.Name)
	}
	if s.Description != "Reviews code for quality" {
		t.Errorf("expected Description mismatch")
	}
	if s.Version != "2.0.0" {
		t.Errorf("expected Version %q, got %q", "2.0.0", s.Version)
	}
	if s.Category != "development" {
		t.Errorf("expected Category %q, got %q", "development", s.Category)
	}
	if len(s.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(s.Tools))
	}
	if s.Tools[0].Name != "lint" {
		t.Errorf("expected tool Name %q, got %q", "lint", s.Tools[0].Name)
	}
	if s.Source != "file://skills/code-review.md" {
		t.Errorf("expected Source %q, got %q", "file://skills/code-review.md", s.Source)
	}
}

func TestSkillsSDK_List_Empty(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []Skill{})
	})

	skills, err := c.Skills.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected empty list, got %d skills", len(skills))
	}
}

func TestSkillsSDK_Search(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/skills/search")
		if r.URL.Query().Get("q") != "deploy" {
			t.Errorf("expected query 'deploy', got %q", r.URL.Query().Get("q"))
		}
		writeJSON(t, w, http.StatusOK, []Skill{{Name: "deploy-skill", Category: "ops"}})
	})

	skills, err := c.Skills.Search("deploy")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "deploy-skill" {
		t.Errorf("expected Name %q, got %q", "deploy-skill", skills[0].Name)
	}
}

func TestSkillsSDK_Search_EmptyQuery(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Skills.Search("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSkillsSDK_Get(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/skills/my-skill")
		writeJSON(t, w, http.StatusOK, Skill{Name: "my-skill", Version: "1.0.0"})
	})

	skill, err := c.Skills.Get("my-skill")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if skill.Name != "my-skill" {
		t.Errorf("expected Name %q, got %q", "my-skill", skill.Name)
	}
}

func TestSkillsSDK_Get_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Skills.Get("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSkillsSDK_Install(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/skills/new-skill/install")

		// Verify request body.
		var req installSkillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			return
		}
		if req.Source != "https://example.com/skill.md" {
			t.Errorf("expected Source %q, got %q", "https://example.com/skill.md", req.Source)
		}

		writeJSON(t, w, http.StatusOK, Skill{
			Name:    "new-skill",
			Version: "1.0.0",
			Source:  "https://example.com/skill.md",
		})
	})

	skill, err := c.Skills.Install("new-skill", "https://example.com/skill.md")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if skill.Name != "new-skill" {
		t.Errorf("expected Name %q, got %q", "new-skill", skill.Name)
	}
	if skill.Source != "https://example.com/skill.md" {
		t.Errorf("expected Source %q, got %q", "https://example.com/skill.md", skill.Source)
	}
}

func TestSkillsSDK_Install_Created(t *testing.T) {

	// Verify that HTTP 201 Created is also accepted.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusCreated, Skill{Name: "created-skill", Version: "0.1.0"})
	})

	skill, err := c.Skills.Install("created-skill", "file://local.md")
	if err != nil {
		t.Fatalf("expected no error for 201, got %v", err)
	}
	if skill.Name != "created-skill" {
		t.Errorf("expected Name %q, got %q", "created-skill", skill.Name)
	}
}

func TestSkillsSDK_Install_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Skills.Install("", "source")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSkillsSDK_Install_EmptySource(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Skills.Install("valid-name", "")
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "source is required") {
		t.Errorf("expected 'source is required', got %q", err.Error())
	}
}

func TestSkillsSDK_Install_BadRequest(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_SOURCE", "source cannot be empty")
	})

	_, err := c.Skills.Install("skill", "invalid://")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coscaErr.StatusCode)
	}
}

// =============================================================================
// ProvidersSDK Tests
// =============================================================================

func TestProvidersSDK_List(t *testing.T) {

	expected := []Provider{
		{
			Name:         "openai",
			Status:       "configured",
			Model:        "gpt-4o",
			Active:       true,
			Configured:   true,
			BaseURL:      "https://api.openai.com/v1",
			APIVersion:   "2024-01-01",
			Models:       []string{"gpt-4o", "gpt-4-turbo"},
			Capabilities: []string{"chat", "embeddings"},
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/providers")
		writeJSON(t, w, http.StatusOK, expected)
	})

	providers, err := c.Providers.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	p := providers[0]
	if p.Name != "openai" {
		t.Errorf("expected Name %q, got %q", "openai", p.Name)
	}
	if !p.Active {
		t.Error("expected Active to be true")
	}
	if !p.Configured {
		t.Error("expected Configured to be true")
	}
	if len(p.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(p.Models))
	}
	if len(p.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(p.Capabilities))
	}
}

func TestProvidersSDK_List_Empty(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []Provider{})
	})

	providers, err := c.Providers.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected empty list, got %d providers", len(providers))
	}
}

func TestProvidersSDK_Get(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/providers/anthropic")
		writeJSON(t, w, http.StatusOK, Provider{Name: "anthropic", Status: "available"})
	})

	provider, err := c.Providers.Get("anthropic")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if provider.Name != "anthropic" {
		t.Errorf("expected Name %q, got %q", "anthropic", provider.Name)
	}
}

func TestProvidersSDK_Get_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Providers.Get("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestProvidersSDK_Get_NotFound(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "provider not found")
	})

	_, err := c.Providers.Get("unknown")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestProvidersSDK_Test(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/providers/openai/test")
		writeJSON(t, w, http.StatusOK, TestResult{
			ResponseTime: "245ms",
			Model:        "gpt-4o",
			Status:       "reachable",
		})
	})

	result, err := c.Providers.Test("openai")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != "reachable" {
		t.Errorf("expected Status %q, got %q", "reachable", result.Status)
	}
	if result.Model != "gpt-4o" {
		t.Errorf("expected Model %q, got %q", "gpt-4o", result.Model)
	}
	if result.ResponseTime != "245ms" {
		t.Errorf("expected ResponseTime %q, got %q", "245ms", result.ResponseTime)
	}
}

func TestProvidersSDK_Test_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Providers.Test("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestProvidersSDK_Test_ServerError(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Providers.Test("down-provider")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	// 5xx from doRequest returns a plain error, not CoscaError.
	if _, ok := err.(*CoscaError); ok {
		t.Errorf("expected non-CoscaError for 5xx, got *CoscaError")
	}
}

func TestProvidersSDK_SetActive(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPut, "/v1/providers/active")

		var req setActiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Provider != "openai" {
			t.Errorf("expected Provider %q, got %q", "openai", req.Provider)
		}
		if req.Model != "gpt-4o" {
			t.Errorf("expected Model %q, got %q", "gpt-4o", req.Model)
		}

		writeJSON(t, w, http.StatusOK, ProviderStatus{
			Active:     "openai",
			Configured: 3,
			Available:  5,
			Statuses:   []string{"openai:active"},
		})
	})

	status, err := c.Providers.SetActive("openai", "gpt-4o")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Active != "openai" {
		t.Errorf("expected Active %q, got %q", "openai", status.Active)
	}
	if status.Configured != 3 {
		t.Errorf("expected Configured %d, got %d", 3, status.Configured)
	}
	if status.Available != 5 {
		t.Errorf("expected Available %d, got %d", 5, status.Available)
	}
}

func TestProvidersSDK_SetActive_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Providers.SetActive("", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestProvidersSDK_SetActive_BadRequest(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_PROVIDER", "provider does not exist")
	})

	_, err := c.Providers.SetActive("invalid", "")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coscaErr.StatusCode)
	}
}

// =============================================================================
// WorkflowsSDK Tests
// =============================================================================

func TestWorkflowsSDK_List(t *testing.T) {

	expected := []Workflow{
		{
			Name:        "deploy",
			Description: "Deploy to production",
			Version:     "1.0.0",
			Status:      "active",
			Enabled:     true,
			Steps:       3,
			StepList: []WorkflowStep{
				{Name: "build", Description: "Build the project", Agent: "builder", Timeout: "5m"},
				{Name: "test", Description: "Run tests", Agent: "tester", Timeout: "10m"},
			},
			Inputs:  []WorkflowIO{{Name: "branch", Type: "string", Required: true}},
			Outputs: []WorkflowIO{{Name: "version", Type: "string", Required: false}},
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/workflows")
		writeJSON(t, w, http.StatusOK, expected)
	})

	workflows, err := c.Workflows.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}

	wf := workflows[0]
	if wf.Name != "deploy" {
		t.Errorf("expected Name %q, got %q", "deploy", wf.Name)
	}
	if !wf.Enabled {
		t.Error("expected Enabled to be true")
	}
	if wf.Steps != 3 {
		t.Errorf("expected Steps %d, got %d", 3, wf.Steps)
	}
	if len(wf.StepList) != 2 {
		t.Errorf("expected 2 step list entries, got %d", len(wf.StepList))
	}
	if len(wf.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(wf.Inputs))
	}
	if len(wf.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(wf.Outputs))
	}
}

func TestWorkflowsSDK_List_Empty(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []Workflow{})
	})

	workflows, err := c.Workflows.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(workflows) != 0 {
		t.Errorf("expected empty list, got %d workflows", len(workflows))
	}
}

func TestWorkflowsSDK_Search(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/workflows/search")
		if r.URL.Query().Get("q") != "ci" {
			t.Errorf("expected query 'ci', got %q", r.URL.Query().Get("q"))
		}
		writeJSON(t, w, http.StatusOK, []Workflow{{Name: "ci-pipeline", Enabled: true}})
	})

	workflows, err := c.Workflows.Search("ci")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
}

func TestWorkflowsSDK_Search_EmptyQuery(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Workflows.Search("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestWorkflowsSDK_Get(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/workflows/deploy")
		writeJSON(t, w, http.StatusOK, Workflow{Name: "deploy", Status: "active"})
	})

	wf, err := c.Workflows.Get("deploy")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if wf.Name != "deploy" {
		t.Errorf("expected Name %q, got %q", "deploy", wf.Name)
	}
}

func TestWorkflowsSDK_Get_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Workflows.Get("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestWorkflowsSDK_Get_NotFound(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
	})

	_, err := c.Workflows.Get("missing")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestWorkflowsSDK_Run(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/workflows/deploy/run")
		writeJSON(t, w, http.StatusOK, WorkflowResult{
			Status:         "completed",
			Duration:       "15.2s",
			StepsCompleted: 3,
			TotalSteps:     3,
			Outputs:        []WorkflowIO{{Name: "version", Type: "string", Required: false}},
		})
	})

	result, err := c.Workflows.Run(context.Background(), "deploy")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected Status %q, got %q", "completed", result.Status)
	}
	if result.Duration != "15.2s" {
		t.Errorf("expected Duration %q, got %q", "15.2s", result.Duration)
	}
	if result.StepsCompleted != 3 {
		t.Errorf("expected StepsCompleted %d, got %d", 3, result.StepsCompleted)
	}
	if result.TotalSteps != 3 {
		t.Errorf("expected TotalSteps %d, got %d", 3, result.TotalSteps)
	}
}

func TestWorkflowsSDK_Run_EmptyName(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Workflows.Run(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestWorkflowsSDK_Run_Conflict(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "ALREADY_RUNNING", "workflow is already in progress")
	})

	_, err := c.Workflows.Run(context.Background(), "busy-workflow")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 409 {
		t.Errorf("expected status 409, got %d", coscaErr.StatusCode)
	}
}

// =============================================================================
// OrchestrationSDK Tests
// =============================================================================

func TestOrchestrationSDK_Run(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/run")

		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Prompt != "hello world" {
			t.Errorf("expected Prompt %q, got %q", "hello world", req.Prompt)
		}

		writeJSON(t, w, http.StatusOK, RunResult{
			Response:   "Hello! How can I help?",
			Agent:      "assistant",
			SkillsUsed: []string{"chat", "greeting"},
			DurationMs: 1234,
			MemoryID:   "mem-001",
		})
	})

	result, err := c.Orchestration.Run(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Response != "Hello! How can I help?" {
		t.Errorf("expected Response %q, got %q", "Hello! How can I help?", result.Response)
	}
	if result.Agent != "assistant" {
		t.Errorf("expected Agent %q, got %q", "assistant", result.Agent)
	}
	if len(result.SkillsUsed) != 2 {
		t.Errorf("expected 2 skills used, got %d", len(result.SkillsUsed))
	}
	if result.DurationMs != 1234 {
		t.Errorf("expected DurationMs %d, got %d", 1234, result.DurationMs)
	}
	if result.MemoryID != "mem-001" {
		t.Errorf("expected MemoryID %q, got %q", "mem-001", result.MemoryID)
	}
}

func TestOrchestrationSDK_Run_WithOptions(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Agent != "code-reviewer" {
			t.Errorf("expected Agent %q, got %q", "code-reviewer", req.Agent)
		}
		if req.Provider != "anthropic" {
			t.Errorf("expected Provider %q, got %q", "anthropic", req.Provider)
		}

		writeJSON(t, w, http.StatusOK, RunResult{Response: "Code looks good!"})
	})

	result, err := c.Orchestration.Run(
		context.Background(),
		"review this code",
		WithAgent("code-reviewer"),
		WithProvider("anthropic"),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Response != "Code looks good!" {
		t.Errorf("expected Response %q, got %q", "Code looks good!", result.Response)
	}
}

func TestOrchestrationSDK_Run_EmptyPrompt(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Orchestration.Run(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "prompt is required") {
		t.Errorf("expected 'prompt is required', got %q", err.Error())
	}
}

func TestOrchestrationSDK_Run_BadRequest(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_PROMPT", "prompt too short")
	})

	_, err := c.Orchestration.Run(context.Background(), "hi")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coscaErr.StatusCode)
	}
}

func TestOrchestrationSDK_Stream(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/run/stream")
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("expected Accept header %q, got %q", "text/event-stream", r.Header.Get("Accept"))
		}

		// Write SSE events.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected ResponseWriter to implement Flusher")
			return
		}

		events := []string{
			`data: {"type":"thinking","content":"Let me think..."}`,
			`data: {"type":"response","content":"Hello from stream!"}`,
			`data: {"type":"done","content":"","duration_ms":999}`,
		}
		for _, e := range events {
			fmt.Fprintf(w, "%s\n\n", e)
			flusher.Flush()
		}
	})

	ctx := context.Background()
	events, err := c.Orchestration.Stream(ctx, "stream this")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var received []StreamEvent
	for evt := range events {
		received = append(received, evt)
	}

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
	if received[0].Type != "thinking" {
		t.Errorf("expected first event type 'thinking', got %q", received[0].Type)
	}
	if received[1].Type != "response" {
		t.Errorf("expected second event type 'response', got %q", received[1].Type)
	}
	if received[2].Type != "done" {
		t.Errorf("expected third event type 'done', got %q", received[2].Type)
	}
	if received[2].DurationMs != 999 {
		t.Errorf("expected DurationMs %d, got %d", 999, received[2].DurationMs)
	}
}

func TestOrchestrationSDK_Stream_EmptyPrompt(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Orchestration.Stream(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestOrchestrationSDK_Stream_ErrorResponse(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "stream endpoint not available")
	})

	_, err := c.Orchestration.Stream(context.Background(), "test")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

// =============================================================================
// KnowledgeSDK Tests
// =============================================================================

func TestKnowledgeSDK_Search(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/search")

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Query != "golang testing" {
			t.Errorf("expected Query %q, got %q", "golang testing", req.Query)
		}
		if req.Type != "keyword" {
			t.Errorf("expected Type 'keyword', got %q", req.Type)
		}

		writeJSON(t, w, http.StatusOK, searchResponse{
			Results: []SearchResult{
				{ID: "1", Score: 0.95, Content: "Testing in Go using the testing package"},
				{ID: "2", Score: 0.80, Title: "Go Test Best Practices"},
			},
			Total:   2,
			TookMs:  45,
			HasMore: false,
		})
	})

	results, err := c.Knowledge.Search("golang testing", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "1" {
		t.Errorf("expected ID %q, got %q", "1", results[0].ID)
	}
	if results[0].Score != 0.95 {
		t.Errorf("expected Score %f, got %f", 0.95, results[0].Score)
	}
	if results[1].Title != "Go Test Best Practices" {
		t.Errorf("expected Title %q, got %q", "Go Test Best Practices", results[1].Title)
	}
}

func TestKnowledgeSDK_Search_EmptyQuery(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.Search("", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "search query is required") {
		t.Errorf("expected 'search query is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_Search_ServerError(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Knowledge.Search("test", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestKnowledgeSDK_Search_InvalidJSON(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	})

	_, err := c.Knowledge.Search("test", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

// =============================================================================
// MemorySDK Tests
// =============================================================================

func TestMemorySDK_Store(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/memory/store")

		var req storeMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Key != "session-data" {
			t.Errorf("expected Key %q, got %q", "session-data", req.Key)
		}
		if req.Value != "some-important-value" {
			t.Errorf("expected Value %q, got %q", "some-important-value", req.Value)
		}
		if req.Type != MemoryTypeWorking {
			t.Errorf("expected Type %q, got %q", MemoryTypeWorking, req.Type)
		}

		w.WriteHeader(http.StatusCreated)
	})

	err := c.Memory.Store(MemoryRecord{
		Key:   "session-data",
		Value: "some-important-value",
		Type:  MemoryTypeWorking,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_Store_EmptyKey(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Store(MemoryRecord{Key: "", Value: "value"})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "memory record key is required") {
		t.Errorf("expected 'memory record key is required', got %q", err.Error())
	}
}

func TestMemorySDK_Store_NilValue(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Store(MemoryRecord{Key: "key", Value: nil})
	if err == nil {
		t.Fatal("expected error for nil value")
	}
	if !strings.Contains(err.Error(), "memory record value is required") {
		t.Errorf("expected 'memory record value is required', got %q", err.Error())
	}
}

func TestMemorySDK_Store_BadRequest(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_RECORD", "duplicate key")
	})

	err := c.Memory.Store(MemoryRecord{Key: "dup", Value: "test"})
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coscaErr.StatusCode)
	}
}

func TestMemorySDK_Search(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/search")

		if r.URL.Query().Get("query") != "important" {
			t.Errorf("expected Query %q, got %q", "important", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("layer") != "working" {
			t.Errorf("expected Layer %q, got %q", "working", r.URL.Query().Get("layer"))
		}

		writeJSON(t, w, http.StatusOK, memorySearchResponse{
			Records: []MemoryRecord{
				{ID: "mem-1", Key: "task-1", Value: "do the thing", Score: 0.95},
				{ID: "mem-2", Key: "task-2", Value: "remember this", Score: 0.75},
			},
			Total: 2,
		})
	})

	records, err := c.Memory.Search("important", "working")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].ID != "mem-1" {
		t.Errorf("expected ID %q, got %q", "mem-1", records[0].ID)
	}
	if records[0].Score != 0.95 {
		t.Errorf("expected Score %f, got %f", 0.95, records[0].Score)
	}
}

func TestMemorySDK_Search_EmptyQuery(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Memory.Search("", "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestMemorySDK_Search_EmptyResults(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, memorySearchResponse{
			Records: []MemoryRecord{},
			Total:   0,
		})
	})

	records, err := c.Memory.Search("nonexistent", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

// =============================================================================
// RuntimeSDK Tests (basic sanity)
// =============================================================================

func TestRuntimeSDK_Status(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/status")
		writeJSON(t, w, http.StatusOK, RuntimeStatus{
			State:        "running",
			Version:      "2.0.0",
			ActiveAgents: 5,
			Mode:         "development",
		})
	})

	status, err := c.Runtime.Status()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected State %q, got %q", "running", status.State)
	}
	if status.Version != "2.0.0" {
		t.Errorf("expected Version %q, got %q", "2.0.0", status.Version)
	}
	if status.ActiveAgents != 5 {
		t.Errorf("expected ActiveAgents %d, got %d", 5, status.ActiveAgents)
	}
	if status.Mode != "development" {
		t.Errorf("expected Mode %q, got %q", "development", status.Mode)
	}
}

func TestRuntimeSDK_Health(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/health")
		writeJSON(t, w, http.StatusOK, HealthReport{
			Status:  "healthy",
			Version: "2.0.0",
			Checks:  []HealthCheck{{Name: "db", Status: "pass"}},
		})
	})

	report, err := c.Runtime.Health()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Status != "healthy" {
		t.Errorf("expected Status %q, got %q", "healthy", report.Status)
	}
	if len(report.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(report.Checks))
	}
}

// =============================================================================
// ContextSDK Tests (basic sanity)
// =============================================================================

func TestContextSDK_Get(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/context/my-key")
		writeJSON(t, w, http.StatusOK, ContextEntry{
			Key:       "my-key",
			Value:     "my-value",
			Scope:     ContextScopeSession,
			UpdatedAt: "2024-01-01T00:00:00Z",
		})
	})

	entry, err := c.Context.Get(context.Background(), "my-key")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entry.Key != "my-key" {
		t.Errorf("expected Key %q, got %q", "my-key", entry.Key)
	}
	if entry.Value != "my-value" {
		t.Errorf("expected Value %q, got %q", "my-value", entry.Value)
	}
	if entry.Scope != ContextScopeSession {
		t.Errorf("expected Scope %q, got %q", ContextScopeSession, entry.Scope)
	}
}

func TestContextSDK_Set(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/context")
		w.WriteHeader(http.StatusCreated)
	})

	err := c.Context.Set(context.Background(), "key", "value", ContextScopeSession)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestContextSDK_List(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != string(ContextScopeProject) {
			t.Errorf("expected scope %q, got %q", ContextScopeProject, r.URL.Query().Get("scope"))
		}
		writeJSON(t, w, http.StatusOK, []ContextEntry{
			{Key: "k1", Value: "v1", Scope: ContextScopeProject},
			{Key: "k2", Value: "v2", Scope: ContextScopeProject},
		})
	})

	entries, err := c.Context.List(context.Background(), ContextScopeProject)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestContextSDK_Delete(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodDelete, "/v1/context/to-delete")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Context.Delete(context.Background(), "to-delete")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestContextSDK_Build(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/context/build")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Context.Build(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestContextSDK_GetCurrent(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/context/current")
		writeJSON(t, w, http.StatusOK, ContextEntry{Key: "current", Value: "active"})
	})

	entry, err := c.Context.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entry.Key != "current" {
		t.Errorf("expected Key %q, got %q", "current", entry.Key)
	}
}

func TestContextSDK_Clear(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodDelete, "/v1/context")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Context.Clear(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestContextSDK_CreateBuilder(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("CreateBuilder should not call the server")
	})

	builder, err := c.Context.CreateBuilder(context.Background(), "my-builder")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if builder.Name != "my-builder" {
		t.Errorf("expected Name %q, got %q", "my-builder", builder.Name)
	}
	if builder.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
}

// =============================================================================
// Client Integration / Edge Cases
// =============================================================================

func TestClient_NetworkError(t *testing.T) {

	// Create a client pointing to an invalid address.
	client, err := NewClient(ClientConfig{
		RuntimeAddr: "127.0.0.1:1", // non-routable, should fail fast
		MaxRetries:  0,
		Timeout:     50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.Agents.List()
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestClient_UserAgentHeader(t *testing.T) {

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != "Cosca-SDK/1.0" {
			t.Errorf("expected User-Agent %q, got %q", "Cosca-SDK/1.0", ua)
		}
		writeJSON(t, w, http.StatusOK, []Agent{})
	})

	_, err := c.Agents.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_APIKeyHeader(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-key-123" {
			t.Errorf("expected Authorization %q, got %q", "Bearer secret-key-123", auth)
		}
		writeJSON(t, w, http.StatusOK, []Agent{})
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		APIKey:      "secret-key-123",
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.Agents.List()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWithAgent_Option(t *testing.T) {

	cfg := &runConfig{}
	WithAgent("code-reviewer")(cfg)
	if cfg.Agent != "code-reviewer" {
		t.Errorf("expected Agent %q, got %q", "code-reviewer", cfg.Agent)
	}
}

func TestWithProvider_Option(t *testing.T) {

	cfg := &runConfig{}
	WithProvider("openai")(cfg)
	if cfg.Provider != "openai" {
		t.Errorf("expected Provider %q, got %q", "openai", cfg.Provider)
	}
}
