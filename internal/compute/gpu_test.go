package compute

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// OllamaExecutor — Available
// =============================================================================

// newOllamaExecutorForTest monta um executor apontando para um baseURL fake,
// com o probe de GPU injetado (nunca o probe real) e estado zero.
func newOllamaExecutorForTest(baseURL string, probe func() GPUInfo) *OllamaExecutor {
	return &OllamaExecutor{
		baseURL: baseURL,
		client:  &http.Client{},
		gpu:     GPUInfo{},
		probe:   probe,
	}
}

func TestOllamaExecutor_Available_ServerUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"version":"0.32.5"}`)
	}))
	defer srv.Close()

	e := newOllamaExecutorForTest(srv.URL, func() GPUInfo { return GPUInfo{Vendor: GPUAmd, VRAMGB: 12} })
	if !e.Available(context.Background()) {
		t.Error("Available should be true when server responds")
	}
}

func TestOllamaExecutor_Available_ServerDown(t *testing.T) {
	e := newOllamaExecutorForTest("http://127.0.0.1:1", func() GPUInfo { return GPUInfo{Vendor: GPUAmd} })
	if e.Available(context.Background()) {
		t.Error("Available should be false when server is unreachable")
	}
}

func TestOllamaExecutor_Available_NoGPU(t *testing.T) {
	// Available é HTTP-only: não consulta o probe de GPU. Com servidor de pé,
	// retorna true mesmo com gpu não probada (Vendor zero). A indisponibilidade
	// por falta de GPU vive no Execute ("GPU não detectada").
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"version":"0.32.5"}`)
	}))
	defer srv.Close()

	e := newOllamaExecutorForTest(srv.URL, func() GPUInfo { return GPUInfo{Vendor: GPUNone} })
	if !e.Available(context.Background()) {
		t.Error("Available should be true regardless of GPU probe (HTTP-only)")
	}
}

// =============================================================================
// OllamaExecutor — Execute
// =============================================================================

func TestOllamaExecutor_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		fmt.Fprint(w, `{"response":"olá","eval_count":42,"eval_duration":2100000000}`)
	}))
	defer srv.Close()

	e := newOllamaExecutorForTest(srv.URL, func() GPUInfo {
		return GPUInfo{Vendor: GPUAmd, VRAMGB: 12}
	})

	result, err := e.Execute(context.Background(), GPURequest{
		Model:  "llama3.1:8b",
		Prompt: "oi",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Output != "olá" {
		t.Errorf("Output = %q, want %q", result.Output, "olá")
	}
	if result.TotalTokens != 42 {
		t.Errorf("TotalTokens = %d, want 42", result.TotalTokens)
	}
	// 42 tokens / 2.1s ≈ 20 tok/s.
	if result.TokensPerSec < 19 || result.TokensPerSec > 21 {
		t.Errorf("TokensPerSec = %.2f, want ≈20", result.TokensPerSec)
	}
	if result.Duration != 2100000000*time.Nanosecond {
		t.Errorf("Duration = %v, want 2.1s", result.Duration)
	}
}

func TestOllamaExecutor_Execute_NoGPU(t *testing.T) {
	e := newOllamaExecutorForTest("http://127.0.0.1:1", func() GPUInfo {
		return GPUInfo{Vendor: GPUNone}
	})

	_, err := e.Execute(context.Background(), GPURequest{Model: "m", Prompt: "oi"})
	if err == nil {
		t.Fatal("Execute should fail when no GPU is detected")
	}
	if !strings.Contains(err.Error(), "GPU não detectada") {
		t.Errorf("error should mention GPU não detectada, got: %v", err)
	}
}

func TestOllamaExecutor_Execute_VRAMGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response":"x"}`)
	}))
	defer srv.Close()

	e := newOllamaExecutorForTest(srv.URL, func() GPUInfo {
		return GPUInfo{Vendor: GPUAmd, VRAMGB: 12}
	})

	_, err := e.Execute(context.Background(), GPURequest{
		Model:          "llama3.1:8b",
		Prompt:         "oi",
		VRAMEstimateMB: 20000,
	})
	if err == nil {
		t.Fatal("Execute should fail when VRAM estimate exceeds GPU VRAM")
	}
	if !strings.Contains(err.Error(), "VRAM") {
		t.Errorf("error should mention VRAM, got: %v", err)
	}
}

func TestOllamaExecutor_Execute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := newOllamaExecutorForTest(srv.URL, func() GPUInfo {
		return GPUInfo{Vendor: GPUAmd, VRAMGB: 12}
	})

	_, err := e.Execute(context.Background(), GPURequest{
		Model:  "llama3.1:8b",
		Prompt: "oi",
	})
	if err == nil {
		t.Fatal("Execute should fail on HTTP 500")
	}
}

func TestOllamaExecutor_ProbeOnce(t *testing.T) {
	var probes int
	e := newOllamaExecutorForTest("http://127.0.0.1:1", func() GPUInfo {
		probes++
		return GPUInfo{Vendor: GPUAmd, VRAMGB: 12}
	})

	// GPUInfo() várias vezes → o probe roda UMA única vez (sync.Once).
	for i := 0; i < 3; i++ {
		if got := e.GPUInfo(); got.Vendor != GPUAmd {
			t.Fatalf("GPUInfo() = %v, want GPUAmd", got)
		}
	}
	if probes != 1 {
		t.Fatalf("probe chamado %d vezes após 3 GPUInfo(), want 1 (cacheado)", probes)
	}

	// Execute() também usa o valor cacheado — o probe não roda de novo.
	// (Vendor=GPUAmd, então passa no guard; o erro vem do HTTP indisponível.)
	_, err := e.Execute(context.Background(), GPURequest{Model: "m", Prompt: "oi"})
	if err == nil {
		t.Fatal("Execute should fail with unreachable server")
	}
	if probes != 1 {
		t.Fatalf("probe chamado %d vezes após Execute(), want 1 (cacheado)", probes)
	}
}

// =============================================================================
// Fabric — SubmitGPU
// =============================================================================

// fakeGPUExecutor é uma implementação determinística do GPUExecutor para testes.
type fakeGPUExecutor struct {
	name      string
	available bool
	gpu       GPUInfo
	result    GPUResult
	err       error
}

func (f *fakeGPUExecutor) Name() string {
	if f.name == "" {
		return "fake"
	}
	return f.name
}

func (f *fakeGPUExecutor) Available(ctx context.Context) bool { return f.available }

func (f *fakeGPUExecutor) GPUInfo() GPUInfo { return f.gpu }

func (f *fakeGPUExecutor) Execute(ctx context.Context, req GPURequest) (GPUResult, error) {
	if f.err != nil {
		return GPUResult{}, f.err
	}
	res := f.result
	if res.Output == "" {
		res.Output = "fake:" + req.Prompt
		res.TotalTokens = len(req.Prompt)
	}
	return res, nil
}

func TestFabric_SubmitGPU_NoExecutor(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	_, err := f.SubmitGPU(context.Background(), GPURequest{Model: "m", Prompt: "oi"})
	if err == nil {
		t.Fatal("SubmitGPU should error without a configured executor")
	}
	if !strings.Contains(err.Error(), "SetGPUExecutor") {
		t.Errorf("error should mention SetGPUExecutor, got: %v", err)
	}
}

func TestFabric_SubmitGPU_Success(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	f.SetGPUExecutor(&fakeGPUExecutor{
		available: true,
		gpu:       GPUInfo{Vendor: GPUAmd, VRAMGB: 12},
		result:    GPUResult{Output: "olá do fake", TotalTokens: 7, Duration: time.Second, TokensPerSec: 7},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = f.Stop(context.Background())
	})

	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// O pool gpu existe (criado no NewFabric) e foi ativado no Start.
	if f.Pool("gpu") == nil {
		t.Fatal("gpu pool should exist")
	}
	if got := f.Pool("gpu").cfg.MaxWorkers; got != 1 {
		t.Errorf("gpu MaxWorkers = %d, want 1 (executor available)", got)
	}

	result, err := f.SubmitGPU(ctx, GPURequest{Model: "llama3.1:8b", Prompt: "oi", VRAMEstimateMB: 2048})
	if err != nil {
		t.Fatalf("SubmitGPU: %v", err)
	}
	if result.Output != "olá do fake" {
		t.Errorf("Output = %q, want %q", result.Output, "olá do fake")
	}
	if result.TotalTokens != 7 {
		t.Errorf("TotalTokens = %d, want 7", result.TotalTokens)
	}
}

func TestFabric_SubmitGPU_VRAMGuard(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	f.SetGPUExecutor(&fakeGPUExecutor{
		available: true,
		gpu:       GPUInfo{Vendor: GPUAmd, VRAMGB: 12},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = f.Stop(context.Background())
	})
	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	_, err := f.SubmitGPU(ctx, GPURequest{Model: "m", Prompt: "oi", VRAMEstimateMB: 20000})
	if err == nil {
		t.Fatal("SubmitGPU should fail when VRAM estimate exceeds GPU VRAM")
	}
	if !strings.Contains(err.Error(), "VRAM") {
		t.Errorf("error should mention VRAM, got: %v", err)
	}
}

// =============================================================================
// Fabric — StatusReport com GPU
// =============================================================================

func TestFabric_StatusReport_WithGPU(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Injeta GPU no snapshot do probe (o jail pode não expor /sys/class/drm).
	f.probe.mu.Lock()
	f.probe.snapshot.GPU = GPUInfo{
		Vendor:  GPUAmd,
		Model:   "AMD Radeon RX 6700 XT",
		VRAMGB:  12,
		Compute: "gfx1031",
	}
	f.probe.mu.Unlock()

	report := f.StatusReport()
	if !strings.Contains(report, "GPU:") {
		t.Errorf("StatusReport should contain GPU line:\n%s", report)
	}
	if !strings.Contains(report, "gpu") {
		t.Errorf("StatusReport should list the gpu pool:\n%s", report)
	}
}

func TestFabric_StatusReport_NoGPU(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	f.probe.mu.Lock()
	f.probe.snapshot.GPU = GPUInfo{Vendor: GPUNone}
	f.probe.mu.Unlock()

	report := f.StatusReport()
	if strings.Contains(report, "GPU:") {
		t.Errorf("StatusReport should NOT contain GPU line without GPU:\n%s", report)
	}
	if !strings.Contains(report, "gpu") {
		t.Errorf("StatusReport should still list the gpu pool:\n%s", report)
	}
}
