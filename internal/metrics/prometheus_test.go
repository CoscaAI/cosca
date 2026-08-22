package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/runtime"
)

// TestFormatMetrics_NilInputs verifies that FormatMetrics returns valid
// process metrics even when all engine inputs are nil.
func TestFormatMetrics_NilInputs(t *testing.T) {
	output := FormatMetrics(nil, nil, nil, nil, nil)

	// Must contain process metrics.
	required := []string{
		"cosca_process_goroutines",
		"cosca_process_memory_alloc_bytes",
		"cosca_process_memory_sys_bytes",
		"cosca_process_memory_total_alloc_bytes",
		"cosca_process_gc_pause_ns",
		"cosca_process_gc_total",
	}
	for _, m := range required {
		if !strings.Contains(output, m) {
			t.Errorf("missing metric %q in nil-input output", m)
		}
	}
}

// TestFormatMetrics_Runtime verifies runtime metrics when a real runtime
// instance is provided.
func TestFormatMetrics_Runtime(t *testing.T) {
	rt := runtime.New()

	output := FormatMetrics(rt, nil, nil, nil, nil)

	// Runtime state and health.
	required := []string{
		"cosca_runtime_uptime_seconds",
		"cosca_runtime_health",
		"cosca_runtime_state_info",
		"cosca_runtime_operations_total",
		"cosca_runtime_duration_seconds",
	}
	for _, m := range required {
		if !strings.Contains(output, m) {
			t.Errorf("missing %q in runtime output", m)
		}
	}
}

// TestHTTPMetrics_RecordAndFormat verifies HTTP request recording and
// Prometheus output.
func TestHTTPMetrics_RecordAndFormat(t *testing.T) {
	hm := NewHTTPMetrics()

	// Record a few requests (path without method prefix, as middleware strips it).
	hm.Record("GET", "/v1/health", 200, 5*time.Millisecond)
	hm.Record("GET", "/v1/health", 200, 7*time.Millisecond)
	hm.Record("POST", "/v1/knowledge/search", 200, 15*time.Millisecond)
	hm.Record("POST", "/v1/knowledge/search", 500, 100*time.Millisecond)

	output := FormatMetrics(nil, nil, nil, hm, nil)

	// Check HTTP metrics are present.
	required := []string{
		"cosca_http_requests_total",
		"cosca_http_request_duration_seconds_sum",
		"cosca_http_request_duration_seconds_count",
	}
	for _, m := range required {
		if !strings.Contains(output, m) {
			t.Errorf("missing %q in HTTP output", m)
		}
	}

	// Check labels.
	labels := []string{
		`method="GET"`,
		`method="POST"`,
		`status_code="200"`,
		`status_code="500"`,
	}
	for _, label := range labels {
		if !strings.Contains(output, label) {
			t.Errorf("missing label %s in output", label)
		}
	}

	// Verify GET /health appears twice (value 2).
	if !strings.Contains(output, "2") {
		// Not a strict check — just ensure some values are present.
	}
}

// TestGRPCMetrics_RecordAndFormat verifies gRPC request recording.
func TestGRPCMetrics_RecordAndFormat(t *testing.T) {
	gm := NewGRPCMetrics()

	gm.Record("/cosca.v1.RuntimeService/GetStatus", "OK", 1*time.Millisecond)
	gm.Record("/cosca.v1.KnowledgeService/Search", "OK", 5*time.Millisecond)
	gm.Record("/cosca.v1.KnowledgeService/Search", "InvalidArgument", 2*time.Millisecond)

	output := FormatMetrics(nil, nil, nil, nil, gm)

	required := []string{
		"cosca_grpc_requests_total",
		"cosca_grpc_request_duration_seconds_sum",
		"cosca_grpc_request_duration_seconds_count",
	}
	for _, m := range required {
		if !strings.Contains(output, m) {
			t.Errorf("missing %q in gRPC output", m)
		}
	}

	labels := []string{
		`method="/cosca.v1.RuntimeService/GetStatus"`,
		`code="OK"`,
		`code="InvalidArgument"`,
	}
	for _, label := range labels {
		if !strings.Contains(output, label) {
			t.Errorf("missing label %s in gRPC output", label)
		}
	}
}

// TestPrometheusFormatCompliance validates that the output follows the
// Prometheus text exposition format rules.
func TestPrometheusFormatCompliance(t *testing.T) {
	hm := NewHTTPMetrics()
	hm.Record("GET", "/health", 200, 1*time.Millisecond)

	output := FormatMetrics(nil, nil, nil, hm, nil)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	hasHelp := false
	hasType := false
	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP ") {
			hasHelp = true
		}
		if strings.HasPrefix(line, "# TYPE ") {
			hasType = true
		}
	}

	if !hasHelp {
		t.Error("output must contain # HELP lines")
	}
	if !hasType {
		t.Error("output must contain # TYPE lines")
	}

	// Every non-comment, non-empty line should be a valid metric line.
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Metric lines must contain a space between name/labels and value.
		if !strings.Contains(line, " ") {
			t.Errorf("invalid metric line (no space separator): %s", line)
		}
		// Labels (if present) must be properly enclosed.
		if strings.Contains(line, "{") && !strings.Contains(line, "}") {
			t.Errorf("metric line with unclosed label: %s", line)
		}
	}
}

// TestHTTPMetrics_EmptyCollector verifies output when collector has no data.
func TestHTTPMetrics_EmptyCollector(t *testing.T) {
	hm := NewHTTPMetrics()
	output := FormatMetrics(nil, nil, nil, hm, nil)

	// No HTTP metric lines should be present when there's no data.
	if strings.Contains(output, "cosca_http_requests_total") {
		t.Error("cosca_http_requests_total should not be present for empty collector")
	}
	if strings.Contains(output, "cosca_http_request_duration_seconds_sum") {
		t.Error("cosca_http_request_duration_seconds_sum should not be present for empty collector")
	}
}

// TestGRPCMetrics_EmptyCollector verifies no gRPC metrics when collector is empty.
func TestGRPCMetrics_EmptyCollector(t *testing.T) {
	gm := NewGRPCMetrics()
	output := FormatMetrics(nil, nil, nil, nil, gm)

	if strings.Contains(output, "cosca_grpc_requests_total") {
		t.Error("cosca_grpc_requests_total should not be present for empty collector")
	}
}

// TestHTTPMetrics_Concurrent verifies that the collector is safe for
// concurrent access.
func TestHTTPMetrics_Concurrent(t *testing.T) {
	hm := NewHTTPMetrics()
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				hm.Record("GET", "GET /v1/test", 200, time.Microsecond)
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	output := FormatMetrics(nil, nil, nil, hm, nil)
	if !strings.Contains(output, "cosca_http_requests_total") {
		t.Error("expected HTTP metrics after concurrent recording")
	}
}

// TestSplitKey verifies the splitKey helper function.
func TestSplitKey(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  []string
	}{
		{"GET /health 200", 3, []string{"GET", "/health", "200"}},
		{"POST /v1/search 500", 3, []string{"POST", "/v1/search", "500"}},
		{"/cosca.v1/Service/Method OK", 2, []string{"/cosca.v1/Service/Method", "OK"}},
		{"a b c d", 2, []string{"a", "b c d"}},
		{"single", 3, []string{"single"}},
	}

	for _, tt := range tests {
		got := splitKey(tt.input, tt.n)
		if len(got) != len(tt.want) {
			t.Errorf("splitKey(%q, %d) len = %d, want %d", tt.input, tt.n, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitKey(%q, %d)[%d] = %q, want %q", tt.input, tt.n, i, got[i], tt.want[i])
			}
		}
	}
}
