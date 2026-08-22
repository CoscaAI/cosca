package cosca

import (
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// RuntimeSDK Extended Tests
// =============================================================================

func TestRuntimeSDK_Start(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/runtime/start")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Runtime.Start()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRuntimeSDK_Start_Accepted(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	err := c.Runtime.Start()
	if err != nil {
		t.Fatalf("expected no error for 202, got %v", err)
	}
}

func TestRuntimeSDK_Start_AlreadyRunning(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "ALREADY_RUNNING", "runtime is already running")
	})

	err := c.Runtime.Start()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 409 {
		t.Errorf("expected status 409, got %d", coscaErr.StatusCode)
	}
}

func TestRuntimeSDK_Start_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := c.Runtime.Start()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestRuntimeSDK_Stop(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/runtime/stop")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Runtime.Stop()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRuntimeSDK_Stop_Accepted(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	err := c.Runtime.Stop()
	if err != nil {
		t.Fatalf("expected no error for 202, got %v", err)
	}
}

func TestRuntimeSDK_Stop_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "NOT_RUNNING", "runtime is not running")
	})

	err := c.Runtime.Stop()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "NOT_RUNNING" {
		t.Errorf("expected Code %q, got %q", "NOT_RUNNING", coscaErr.Code)
	}
}

func TestRuntimeSDK_Stop_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := c.Runtime.Stop()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestRuntimeSDK_Status_Error(t *testing.T) {
	t.Parallel()

	// Use a 4xx error to get CoscaError (5xx are retried by doRequest and return plain error).
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "RUNTIME_DOWN", "runtime is not available")
	})

	_, err := c.Runtime.Status()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestRuntimeSDK_Status_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	})

	_, err := c.Runtime.Status()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode runtime status") {
		t.Errorf("expected 'failed to decode runtime status', got %q", err.Error())
	}
}

func TestRuntimeSDK_Health_Error(t *testing.T) {
	t.Parallel()

	// Use a 4xx error to get CoscaError (5xx are retried by doRequest and return plain error).
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "HEALTH_CHECK_FAILED", "database unreachable")
	})

	_, err := c.Runtime.Health()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "HEALTH_CHECK_FAILED" {
		t.Errorf("expected Code %q, got %q", "HEALTH_CHECK_FAILED", coscaErr.Code)
	}
}

func TestRuntimeSDK_Health_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	})

	_, err := c.Runtime.Health()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode health report") {
		t.Errorf("expected 'failed to decode health report', got %q", err.Error())
	}
}

func TestRuntimeSDK_Health_FullReport(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, HealthReport{
			Status:  "degraded",
			Version: "2.0.0",
			Checks: []HealthCheck{
				{Name: "db", Status: "pass", Message: "connected", Duration: 5000000},
				{Name: "knowledge", Status: "warn", Message: "slow response", Duration: 250000000},
			},
			Warnings: []string{"knowledge index response time above 200ms"},
			Subsystems: map[string]SubsystemHealth{
				"database":  {Status: "healthy", Latency: 3000000},
				"knowledge": {Status: "degraded", Message: "high latency", Latency: 250000000},
			},
		})
	})

	report, err := c.Runtime.Health()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Status != "degraded" {
		t.Errorf("expected Status %q, got %q", "degraded", report.Status)
	}
	if len(report.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(report.Checks))
	}
	if len(report.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(report.Warnings))
	}
	if len(report.Subsystems) != 2 {
		t.Errorf("expected 2 subsystems, got %d", len(report.Subsystems))
	}
}
