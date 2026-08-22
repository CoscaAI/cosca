package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/middleware"
)

// captureLog runs the LoggingMiddleware around a trivial handler and returns
// the rendered log output for the given request.
func captureLog(t *testing.T, req *http.Request) string {
	t.Helper()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	mw := middleware.LoggingMiddleware(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return buf.String()
}

// TestLogging_RedactsToken verifies that a JWT passed via ?token= in the
// query string is replaced by token=REDACTED in the log output — the raw
// token value must never appear in the logs.
func TestLogging_RedactsToken(t *testing.T) {
	const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEifQ.signature-secret-value"

	req := httptest.NewRequest("GET", "/v1/ws?token="+jwt+"&topic=chat", nil)
	output := captureLog(t, req)

	if strings.Contains(output, jwt) {
		t.Fatalf("log output leaked the raw JWT: %s", output)
	}
	if !strings.Contains(output, "token=REDACTED") {
		t.Errorf("expected 'token=REDACTED' in log output, got: %s", output)
	}
	// The remaining query parameters must still be visible.
	if !strings.Contains(output, "topic=chat") {
		t.Errorf("expected non-sensitive param 'topic=chat' to be preserved, got: %s", output)
	}
}

// TestLogging_RedactsMultipleSensitive verifies that every sensitive query
// parameter (token, access_token, refresh_token, api_key, key, password) is
// redacted while benign parameters keep their values.
func TestLogging_RedactsMultipleSensitive(t *testing.T) {
	rawQuery := "token=jwt&access_token=at&refresh_token=rt&api_key=ak&key=secret-key&password=p4ss&page=2&q=hello+world"
	req := httptest.NewRequest("GET", "/v1/search?"+rawQuery, nil)
	output := captureLog(t, req)

	for _, leak := range []string{"jwt", "at&", "rt&", "ak&", "secret-key", "p4ss"} {
		if strings.Contains(output, leak) {
			t.Errorf("log output leaked sensitive value %q: %s", leak, output)
		}
	}

	for _, want := range []string{
		"token=REDACTED",
		"access_token=REDACTED",
		"refresh_token=REDACTED",
		"api_key=REDACTED",
		"key=REDACTED",
		"password=REDACTED",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in log output, got: %s", want, output)
		}
	}

	// Benign parameters are preserved as-is.
	if !strings.Contains(output, "page=2") || !strings.Contains(output, "q=hello+world") {
		t.Errorf("expected benign params to be preserved, got: %s", output)
	}
}

// TestLogging_NoSensitiveParamsKeepsQuery verifies that a query string with
// no sensitive parameters is logged unchanged.
func TestLogging_NoSensitiveParamsKeepsQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/search?q=golang&page=3", nil)
	output := captureLog(t, req)

	if !strings.Contains(output, "q=golang&page=3") {
		t.Errorf("expected unredacted query in log output, got: %s", output)
	}
}

// TestLogging_TokenWithoutValueIsRedacted verifies that a bare "token"
// parameter (no value) is still redacted rather than logged raw.
func TestLogging_TokenWithoutValueIsRedacted(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/ws?token", nil)
	output := captureLog(t, req)

	if strings.Contains(output, "query=\"token\"") {
		t.Errorf("expected bare token param to be redacted, got: %s", output)
	}
	if !strings.Contains(output, "token=REDACTED") {
		t.Errorf("expected 'token=REDACTED' in log output, got: %s", output)
	}
}
