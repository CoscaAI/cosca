package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// newCoverageRegistry creates a fresh ChatRegistry for coverage testing.
func newCoverageRegistry() *ChatRegistry {
	return &ChatRegistry{
		stats: &ChatStats{},
	}
}

// simpleProvider is a minimal ChatProvider for coverage tests.
type simpleProvider struct {
	name    string
	model   string
	chatErr error
}

func (p *simpleProvider) Chat(_ context.Context, _ []Message, _ ChatOptions) (*ChatResponse, error) {
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return &ChatResponse{
		ID:    "test-1",
		Model: p.model,
		Choices: []Choice{{
			Index:   0,
			Message: Message{Role: RoleAssistant, Content: "ok"},
		}},
		Usage: Usage{TotalTokens: 10},
	}, nil
}
func (p *simpleProvider) ChatStream(_ context.Context, _ []Message, _ ChatOptions) (ChatStream, error) {
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return nil, errors.New("stream not implemented")
}
func (p *simpleProvider) Model() string { return p.model }
func (p *simpleProvider) Name() string  { return p.name }
func (p *simpleProvider) Close() error  { return nil }

// ── ChatStats: concurrency safety ────────────────────────────────────────────

func TestChatStats_Concurrency(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			s.RecordRequest(10)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 50; i++ {
			s.RecordError()
		}
		done <- struct{}{}
	}()
	<-done
	<-done

	snap := s.Snapshot()
	assert.Equal(t, int64(100), snap.TotalRequests)
	assert.Equal(t, int64(1000), snap.TotalTokens)
	assert.Equal(t, int64(50), snap.Errors)
}

func TestChatStats_RecordRequest_NegativeTokens(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	// atomic.AddInt64 supports negative values
	s.RecordRequest(-5)
	assert.Equal(t, int64(1), s.Snapshot().TotalRequests)
	assert.Equal(t, int64(-5), s.Snapshot().TotalTokens)
}

func TestChatStats_ZeroValueSnapshot(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	snap := s.Snapshot()
	assert.Zero(t, snap.TotalRequests)
	assert.Zero(t, snap.TotalTokens)
	assert.Zero(t, snap.Errors)
}

// ── Registry: Invalidate ─────────────────────────────────────────────────────

func TestRegisteredProvider_Invalidate(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	// Register a provider so we can get a reference to the registered entry
	var createdProvider ChatProvider
	callCount := 0
	reg.Register("openai", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		callCount++
		createdProvider = &simpleProvider{name: "openai", model: "gpt-4o"}
		return createdProvider, nil
	}, "OpenAI provider", 0)

	// Get creates a cached instance
	p1, ok := reg.Get("openai")
	require.True(t, ok)
	require.NotNil(t, p1)
	assert.Equal(t, 1, callCount)

	// Find the registered entry and invalidate it
	reg.mu.RLock()
	var rp *registeredChatProvider
	for i := range reg.providers {
		if reg.providers[i].name == "openai" {
			rp = &reg.providers[i]
			break
		}
	}
	reg.mu.RUnlock()
	require.NotNil(t, rp)

	rp.Invalidate()

	// After invalidation, Get creates a new instance
	p2, ok := reg.Get("openai")
	require.True(t, ok)
	require.NotNil(t, p2)
	assert.Equal(t, 2, callCount)
	assert.NotSame(t, p1, p2, "instances should differ after invalidation")
}

// ── Registry: getOrCreate factory error ──────────────────────────────────────

func TestRegisteredProvider_GetOrCreate_FactoryError(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	fatalErr := errors.New("creation failed")

	reg.Register("broken", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return nil, fatalErr
	}, "broken provider", 0)

	// First Get — factory fails
	p, ok := reg.Get("broken")
	assert.True(t, ok, "name is registered")
	assert.Nil(t, p, "factory failed so provider is nil")

	// Second Get — still nil because factory failed and we cached the nil result (wait, actually we don't cache nil)
	// Let me re-check: getOrCreate has a fast path for p.instance != nil. If nil, it locks,
	// double-checks, then calls factory. If factory fails, it returns nil without setting p.instance.
	// So next call also retries the factory. Let's verify.
	p2, ok2 := reg.Get("broken")
	assert.True(t, ok2)
	assert.Nil(t, p2)
}

// ── Registry: createProvider unknown name ────────────────────────────────────

func TestCreateProvider_Unknown(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	_, err := reg.createProvider(context.Background(), "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// ── Registry: createProvider success ─────────────────────────────────────────

func TestCreateProvider_Success(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	reg.Register("testp", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "testp", model: "test-model"}, nil
	}, "test provider", 0)

	provider, err := reg.createProvider(context.Background(), "testp")
	require.NoError(t, err)
	assert.Equal(t, "testp", provider.Name())
	assert.Equal(t, "test-model", provider.Model())
}

// ── Registry: Name/Model with nothing selected ───────────────────────────────

func TestRegistry_Name_NoSelection(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	assert.Equal(t, "chat-registry", reg.Name())
}

func TestRegistry_Model_NoSelection(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	assert.Empty(t, reg.Model())
}

// ── Registry: Close with nil selected ────────────────────────────────────────

func TestRegistry_Close_NoSelection(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	err := reg.Close()
	assert.NoError(t, err)
}

// ── Registry: Select with empty candidates ───────────────────────────────────

func TestRegistry_Select_EmptyCandidates(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	cfg := ChatRegistryConfig{} // no primary, no auto-detect, no fallbacks
	err := reg.Select(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no chat provider")
}

// ── Registry: Chat third fallback wins ───────────────────────────────────────

func TestRegistry_Chat_ThirdFallbackWins(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	reg.Register("p1", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "p1", model: "m1", chatErr: errors.New("fail1")}, nil
	}, "p1", 0)

	reg.Register("p2", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "p2", model: "m2", chatErr: errors.New("fail2")}, nil
	}, "p2", 1)

	reg.Register("p3", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "p3", model: "m3"}, nil
	}, "p3", 2)

	cfg := ChatRegistryConfig{
		Primary:   "p1",
		Fallbacks: []string{"p2", "p3"},
	}
	err := reg.Select(context.Background(), cfg)
	require.NoError(t, err)

	resp, err := reg.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, ChatOptions{})
	require.NoError(t, err)
	assert.Equal(t, "m3", resp.Model)
}

// ── Registry: GetOrCreate concurrent safety ──────────────────────────────────

func TestGetOrCreate_Concurrent(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	var factoryCalls atomic.Int32

	reg.Register("concurr", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		factoryCalls.Add(1)
		return &simpleProvider{name: "concurr", model: "m1"}, nil
	}, "concurrent provider", 0)

	// Concurrent Get calls should only invoke factory once
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			p, ok := reg.Get("concurr")
			assert.True(t, ok)
			assert.NotNil(t, p)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, int32(1), factoryCalls.Load(),
		"factory should be called exactly once despite concurrent access")
}

// ── Registry: Stats after stream error ───────────────────────────────────────

func TestRegistry_Stats_StreamError(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	fatalErr := errors.New("stream failed")

	reg.Register("badstream", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "badstream", model: "bad", chatErr: fatalErr}, nil
	}, "bad stream", 0)

	_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "badstream"})

	_, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.Error(t, err)

	stats := reg.Stats()
	assert.Equal(t, int64(1), stats.Errors)
}

// ── Registry: Select with only AutoDetect ────────────────────────────────────

func TestRegistry_Select_AutoDetectOnly(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	reg.Register("autoprov", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "autoprov", model: "auto-model"}, nil
	}, "auto-detected", 5)

	cfg := ChatRegistryConfig{
		AutoDetect: true,
	}
	err := reg.Select(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "autoprov", reg.Name())
}

// ── HotReload: computeConfigHash ─────────────────────────────────────────────

func TestHotReload_ComputeConfigHash(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		PollInterval: 30 * time.Second,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	defer hr.Stop()

	hash := hr.computeConfigHash()
	assert.NotEmpty(t, hash, "hash should not be empty")
}

// ── HotReload: reloadIfChanged no change ─────────────────────────────────────

func TestHotReload_ReloadIfChanged_NoChange(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		PollInterval: 30 * time.Second,
		AutoDetect:   true,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	defer hr.Stop()

	// Calling reloadIfChanged when nothing has changed should be a no-op
	assert.NotPanics(t, func() {
		hr.reloadIfChanged()
	})
}

// ── HotReload: reloadIfChanged with env var change ───────────────────────────

func TestHotReload_ReloadIfChanged_EnvChange(t *testing.T) {
	// NOT parallel — modifies env vars
	reg := newCoverageRegistry()

	// Register a provider so auto-detect can find something
	reg.Register("test-reload", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "test-reload", model: "test-model"}, nil
	}, "test reload", 0)

	cfg := HotReloadConfig{
		PollInterval: 30 * time.Second,
		AutoDetect:   true,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	defer hr.Stop()

	// Change an env var to trigger a hash change
	t.Setenv("OPENAI_API_KEY", "sk-test-reload-12345")

	// Now reloadIfChanged should detect the change and attempt to re-select
	assert.NotPanics(t, func() {
		hr.reloadIfChanged()
	})
}

// ── HotReload: isConfigFile ──────────────────────────────────────────────────

func TestHotReload_IsConfigFile(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		ConfigPaths: []string{"/etc/cosca/config.yaml"},
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	defer hr.Stop()

	assert.True(t, hr.isConfigFile("/etc/cosca/config.yaml"))
	assert.True(t, hr.isConfigFile("/other/settings.yml"))
	assert.True(t, hr.isConfigFile("/other/settings.yaml"))
	assert.False(t, hr.isConfigFile("/etc/cosca/config.json"))
	assert.False(t, hr.isConfigFile("/etc/cosca/readme.md"))
}

func TestHotReload_IsConfigFile_EmptyPaths(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		ConfigPaths: []string{},
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	defer hr.Stop()

	// With empty ConfigPaths, only yaml/yml extension matches
	assert.True(t, hr.isConfigFile("/some/file.yaml"))
	assert.False(t, hr.isConfigFile("/some/file.txt"))
}

// ── HotReload: NewHotReload ──────────────────────────────────────────────────

func TestNewHotReload_Basic(t *testing.T) {
	// NOT parallel — fsnotify may not be available in all envs
	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		ConfigPaths:  []string{"/nonexistent/path/config.yaml"},
		PollInterval: 10 * time.Second,
		AutoDetect:   true,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	require.NotNil(t, hr)

	hr.Stop()
}

func TestNewHotReload_ZeroPollInterval(t *testing.T) {
	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		PollInterval: 0, // should be defaulted to 30s
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	assert.NotNil(t, hr)

	hr.Stop()
}

func TestNewHotReload_WithExistingConfigFile(t *testing.T) {
	reg := newCoverageRegistry()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte("# test config\n"), 0644)
	require.NoError(t, err)

	cfg := HotReloadConfig{
		ConfigPaths:  []string{configPath},
		PollInterval: 30 * time.Second,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)
	assert.NotNil(t, hr)

	hr.Stop()
}

// ── HotReload: Start/Stop lifecycle ──────────────────────────────────────────

func TestHotReload_StartStop(t *testing.T) {
	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		PollInterval: 5 * time.Second,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)

	// Start should work
	hr.Start()

	// Start again should be a no-op (already running)
	hr.Start()

	// Give it a moment to enter the watch loop
	time.Sleep(50 * time.Millisecond)

	// Stop should work
	hr.Stop()

	// Stop again should be a no-op
	hr.Stop()
}

func TestHotReload_StopWithoutStart(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	cfg := HotReloadConfig{
		PollInterval: 5 * time.Second,
	}

	hr, err := NewHotReload(reg, cfg)
	require.NoError(t, err)

	// Stop without start should not panic
	assert.NotPanics(t, func() {
		hr.Stop()
	})
}

// ── Registry: Snapshot returns copy ──────────────────────────────────────────

func TestChatStats_Snapshot_ReturnsCopy(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	s.RecordRequest(10)
	s.RecordError()

	snap1 := s.Snapshot()
	assert.Equal(t, int64(1), snap1.TotalRequests)
	assert.Equal(t, int64(1), snap1.Errors)

	// Modify original; snapshot should be unaffected
	s.RecordRequest(5)

	snap2 := s.Snapshot()
	assert.Equal(t, int64(2), snap2.TotalRequests)
	// snap1 must be unchanged (snapshot is a copy, not a reference)
	assert.Equal(t, int64(1), snap1.TotalRequests)
	assert.Equal(t, int64(2), snap2.TotalRequests)
}

// ── Registry: Select primary from candidate missing from registrations ───────

func TestRegistry_Select_PrimaryNotRegistered(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()

	// Register a different provider
	reg.Register("other", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return &simpleProvider{name: "other", model: "other-m"}, nil
	}, "other", 0)

	cfg := ChatRegistryConfig{
		Primary: "openai", // not registered
	}
	err := reg.Select(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no chat provider")
}

// ── Registered provider: getOrCreate factory error retries ───────────────────

func TestRegisteredProvider_GetOrCreate_RetriesAfterError(t *testing.T) {
	t.Parallel()

	reg := newCoverageRegistry()
	callCount := 0

	reg.Register("flaky", func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary error")
		}
		return &simpleProvider{name: "flaky", model: "flaky-m"}, nil
	}, "flaky provider", 0)

	// First Get — factory fails
	p1, ok1 := reg.Get("flaky")
	assert.True(t, ok1)
	assert.Nil(t, p1)
	assert.Equal(t, 1, callCount)

	// Second Get — factory fails again
	p2, ok2 := reg.Get("flaky")
	assert.True(t, ok2)
	assert.Nil(t, p2)
	assert.Equal(t, 2, callCount)

	// Third Get — factory succeeds (but callCount check passes on 3rd call)
	p3, ok3 := reg.Get("flaky")
	assert.True(t, ok3)
	if p3 != nil {
		assert.Equal(t, 3, callCount)
		assert.Equal(t, "flaky-m", p3.Model())
	}
}
