package cosca

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Heartbeat Test Helpers
// =============================================================================

// newHeartbeatTestClient creates a Client backed by an httptest server with
// a configurable heartbeat interval and timeout. Useful for heartbeatLoop tests
// that need a short tick interval.
func newHeartbeatTestClient(t *testing.T, handler http.HandlerFunc, interval, timeout time.Duration) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	addr := strings.TrimPrefix(server.URL, "http://")

	client, err := NewClient(ClientConfig{
		RuntimeAddr:       addr,
		MaxRetries:        0,
		HeartbeatInterval: interval,
		Timeout:           timeout,
	})
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}
	return client
}

// =============================================================================
// sendHeartbeat Tests
// =============================================================================

func TestSendHeartbeat_Success(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/health")
		w.WriteHeader(http.StatusOK)
	})

	err := c.sendHeartbeat()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendHeartbeat_ServerError500(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := c.sendHeartbeat()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "unexpected heartbeat status: 500") {
		t.Errorf("expected 'unexpected heartbeat status: 500', got %q", err.Error())
	}
}

func TestSendHeartbeat_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.sendHeartbeat()
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "unexpected heartbeat status: 404") {
		t.Errorf("expected 'unexpected heartbeat status: 404', got %q", err.Error())
	}
}

func TestSendHeartbeat_Timeout(t *testing.T) {
	t.Parallel()

	// Server handler that sleeps longer than the client timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		MaxRetries:  0,
		Timeout:     20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.sendHeartbeat()
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSendHeartbeat_WithAPIKey(t *testing.T) {
	t.Parallel()

	const apiKey = "secret-api-key-12345"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + apiKey
		if auth != expectedAuth {
			t.Errorf("expected Authorization %q, got %q", expectedAuth, auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		MaxRetries:  0,
		APIKey:      apiKey,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.sendHeartbeat()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendHeartbeat_WithoutAPIKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("expected no Authorization header, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.sendHeartbeat()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendHeartbeat_UserAgentHeader(t *testing.T) {
	t.Parallel()

	// Default User-Agent is set by setDefaults.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != "Cosca-SDK/1.0" {
			t.Errorf("expected User-Agent %q, got %q", "Cosca-SDK/1.0", ua)
		}
		w.WriteHeader(http.StatusOK)
	})

	err := c.sendHeartbeat()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendHeartbeat_CustomUserAgentHeader(t *testing.T) {
	t.Parallel()

	const customUA = "Custom-Client/2.0"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != customUA {
			t.Errorf("expected User-Agent %q, got %q", customUA, ua)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr: addr,
		MaxRetries:  0,
		UserAgent:   customUA,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.sendHeartbeat()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendHeartbeat_Non200Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantMsg    string
	}{
		{
			name:       "bad request 400",
			statusCode: http.StatusBadRequest,
			wantMsg:    "unexpected heartbeat status: 400",
		},
		{
			name:       "service unavailable 503",
			statusCode: http.StatusServiceUnavailable,
			wantMsg:    "unexpected heartbeat status: 503",
		},
		{
			name:       "created 201",
			statusCode: http.StatusCreated,
			wantMsg:    "unexpected heartbeat status: 201",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			err := c.sendHeartbeat()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.wantMsg, err.Error())
			}
		})
	}
}

// =============================================================================
// heartbeatLoop Tests
// =============================================================================

func TestHeartbeatLoop_PeriodicHeartbeats(t *testing.T) {
	// Not parallel — uses goroutines and timing that may conflict with
	// other heartbeat tests.

	var (
		mu              sync.Mutex
		count           int
		heartbeatSignal = make(chan struct{}, 20)
	)

	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			count++
			mu.Unlock()
			heartbeatSignal <- struct{}{}
			w.WriteHeader(http.StatusOK)
		},
		20*time.Millisecond, // heartbeat interval
		30*time.Second,      // timeout
	)

	const expectedMin = 3

	// Start the heartbeat loop.
	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// Wait for at least expectedMin heartbeats.
	for i := 0; i < expectedMin; i++ {
		select {
		case <-heartbeatSignal:
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for heartbeat %d", i+1)
		}
	}

	// Stop the heartbeat.
	client.stopHeartbeat()

	mu.Lock()
	finalCount := count
	mu.Unlock()

	if finalCount < expectedMin {
		t.Errorf("expected at least %d heartbeats, got %d", expectedMin, finalCount)
	}
	t.Logf("heartbeats sent: %d", finalCount)
}

func TestHeartbeatLoop_StopsOnSignal(t *testing.T) {
	// Not parallel for same reason as above.

	var mu sync.Mutex
	sentBeforeStop := 0
	stopped := make(chan struct{})

	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			sentBeforeStop++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		},
		10*time.Millisecond,
		30*time.Second,
	)

	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// Let a tick or two fire so we know it is running.
	time.Sleep(35 * time.Millisecond)

	// Signal stop and wait for the loop to finish.
	go func() {
		client.stopHeartbeat()
		close(stopped)
	}()

	select {
	case <-stopped:
		// stopHeartbeat returned without hanging → loop exited cleanly.
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: heartbeat loop did not stop")
	}

	mu.Lock()
	c := sentBeforeStop
	mu.Unlock()

	t.Logf("heartbeats sent before stop: %d", c)
	if c == 0 {
		t.Log("no heartbeats were sent before stop signal (timing)")
	}
}

func TestHeartbeatLoop_WaitGroupDecrementedOnStop(t *testing.T) {
	// Verify that hbtWg.Wait() unblocks after the loop is stopped.
	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		10*time.Millisecond,
		30*time.Second,
	)

	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// Let it run briefly.
	time.Sleep(30 * time.Millisecond)

	client.stopHeartbeat()

	// If stopHeartbeat returned, hbtWg was decremented internally.
	// We verify by calling Wait() again — it should return immediately.
	waitDone := make(chan struct{})
	go func() {
		client.hbtWg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// hbtWg counter is 0 — Done() was called.
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: hbtWg was not properly decremented")
	}
}

func TestHeartbeatLoop_ReconnectOnFailure(t *testing.T) {
	// When sendHeartbeat fails, heartbeatLoop spawns a Reconnect goroutine
	// which calls Connect() → makes another /v1/health request.
	var (
		mu        sync.Mutex
		reqCount  int
		reqSignal = make(chan struct{}, 1000) // generous buffer: the reconnected heartbeat loop keeps ticking
	)

	// First request → 500 (heartbeat fails). Subsequent → 200 (reconnect succeeds).
	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			reqCount++
			n := reqCount
			mu.Unlock()

			reqSignal <- struct{}{}

			if n == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		},
		10*time.Millisecond,
		30*time.Second,
	)

	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// Wait for at least 2 requests (failed heartbeat + reconnect health-check).
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := reqCount
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-reqSignal:
		case <-deadline:
			mu.Lock()
			r := reqCount
			mu.Unlock()
			t.Fatalf("timeout waiting for reconnect; received %d requests", r)
		}
	}

	// Give the reconnect goroutine time to finish.
	time.Sleep(200 * time.Millisecond)

	client.Close()

	mu.Lock()
	finalCount := reqCount
	mu.Unlock()

	if finalCount < 2 {
		t.Errorf("expected at least 2 requests (heartbeat + reconnect), got %d", finalCount)
	}
	t.Logf("total requests: %d", finalCount)
}

func TestHeartbeatLoop_WaitGroupDecrementedOnFailure(t *testing.T) {
	// When heartbeatLoop exits due to error, its defer calls hbtWg.Done().
	// Verify that hbtWg.Wait() unblocks after the error exit.

	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			// Always fail → loop exits on first tick.
			w.WriteHeader(http.StatusInternalServerError)
		},
		10*time.Millisecond,
		30*time.Second,
	)

	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// The loop should fail on the first tick and call Done().
	// Use a goroutine + timeout to verify hbtWg reaches 0.
	done := make(chan struct{})
	go func() {
		client.hbtWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// hbtWg was properly decremented.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: hbtWg.Wait() did not return after heartbeat failure")
	}

	// Let reconnect goroutine finish to avoid background noise.
	time.Sleep(100 * time.Millisecond)
	client.Close()
}

func TestHeartbeatLoop_NoHeartbeatWhenStoppedBeforeTick(t *testing.T) {
	// If stopHbt is closed before the first tick, the loop should exit
	// without sending any heartbeat.

	var mu sync.Mutex
	sent := false

	client := newHeartbeatTestClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			sent = true
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		},
		500*time.Millisecond, // long interval so tick never fires
		30*time.Second,
	)

	client.hbtWg.Add(1)
	go client.heartbeatLoop()

	// Immediately stop before the ticker fires.
	client.stopHeartbeat()

	mu.Lock()
	wasSent := sent
	mu.Unlock()

	if wasSent {
		t.Error("heartbeat was sent after stop signal")
	}
}
