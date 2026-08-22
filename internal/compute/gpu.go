package compute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// GPU Executor — inferência na GPU via backend local (Ollama ROCm/CUDA)
// =============================================================================

// ROCmGfx1031Override é o valor de HSA_OVERRIDE_GFX_VERSION que um servidor
// Ollama/ROCm precisa exportar para executar em GPUs gfx1031 (RX 6700 XT).
// A RX 6700 XT é dropada pelo rocblas (não tem gfx1031 em rocBLAS 6.x), mas o
// override "10.3.0" mapeia o target para gfx1030, que é suportado. Isso é setup
// do SERVIDOR (fora do escopo deste executor) — aqui apenas documentado como
// constante para o operador referenciar.
const ROCmGfx1031Override = "10.3.0"

// GPURequest descreve uma carga de inferência a ser executada na GPU.
type GPURequest struct {
	Model          string        // modelo do servidor Ollama (ex: "llama3.1:8b")
	Prompt         string        // prompt de texto
	VRAMEstimateMB int           // estimativa de VRAM da carga (0 = sem verificação)
	Timeout        time.Duration // timeout da inferência (default 5min)
}

// GPUResult é o resultado de uma inferência executada na GPU.
type GPUResult struct {
	Output       string
	TotalTokens  int
	Duration     time.Duration
	TokensPerSec float64
	VRAMUsedMB   int // opcional (0 se não reportado)
}

// GPUExecutor executa inferência na GPU. A implementação de referência
// (OllamaExecutor) conversa com um servidor Ollama local com backend ROCm/CUDA.
type GPUExecutor interface {
	Name() string
	// Available é true quando o backend de inferência responde (HTTP 2xx em
	// /api/version). A presença REAL de GPU é verificada no Execute — o probe
	// de GPU é lazy/cacheado e nunca roda no construtor nem no Available.
	Available(ctx context.Context) bool
	GPUInfo() GPUInfo
	Execute(ctx context.Context, req GPURequest) (GPUResult, error)
}

// =============================================================================
// OllamaExecutor — servidor Ollama local (backend ROCm/CUDA)
// =============================================================================

// OllamaExecutor é a implementação de referência do GPUExecutor. Ela conversa
// com um servidor Ollama local (http://127.0.0.1:11434 por padrão) cujo backend
// já roda com aceleração ROCm/CUDA. O executor é apenas o cliente HTTP — todo
// o setup de drivers/rocm (incluindo o override HSA_OVERRIDE_GFX_VERSION para
// gfx1031, ver ROCmGfx1031Override) é responsabilidade do servidor.
//
// O probe de GPU (ProbeGPU) é LAZY e CACHEADO: construir o executor é O(1) —
// o probe (caro: invoca rocm-smi/rocminfo) só roda UMA vez, quando o pipeline
// GPU é de fato usado (GPUInfo() ou Execute()). Isso mantém o init do serve em
// ~10ms (só o HTTP check do Available).
type OllamaExecutor struct {
	baseURL   string
	client    *http.Client
	gpu       GPUInfo
	probeOnce sync.Once
	probe     func() GPUInfo // injetável para testes; default ProbeGPU
}

// defaultOllamaBaseURL é o endpoint padrão de um servidor Ollama local.
const defaultOllamaBaseURL = "http://127.0.0.1:11434"

// defaultGPUTimeout é o timeout padrão de uma inferência (5 minutos).
const defaultGPUTimeout = 5 * time.Minute

// NewOllamaExecutor cria um executor Ollama. A baseURL vem de OLLAMA_HOST
// (ex: "http://host:port") quando setado, senão usa o endpoint local padrão.
// NÃO roda ProbeGPU na construção (é O(1)): o probe de GPU é lazy — só dispara
// na primeira consulta real via GPUInfo()/Execute().
func NewOllamaExecutor() *OllamaExecutor {
	baseURL := defaultOllamaBaseURL
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			v = "http://" + v
		}
		baseURL = v
	}
	return &OllamaExecutor{
		baseURL: baseURL,
		client:  &http.Client{},
		gpu:     GPUInfo{},
		probe:   ProbeGPU,
	}
}

// Name retorna o nome do executor.
func (e *OllamaExecutor) Name() string { return "ollama" }

// detectGPU executa o probe de GPU UMA vez (cacheado) e retorna o resultado.
// Thread-safe via sync.Once: chamadas concorrentes rodam o probe uma única vez.
func (e *OllamaExecutor) detectGPU() GPUInfo {
	e.probeOnce.Do(func() { e.gpu = e.probe() })
	return e.gpu
}

// GPUInfo retorna a capacidade de GPU detectada. Dispara o probe lazy na
// primeira chamada (custo pago aqui, não no construtor); nas seguintes devolve
// o valor cacheado.
func (e *OllamaExecutor) GPUInfo() GPUInfo { return e.detectGPU() }

// Available é true quando o servidor Ollama responde em /api/version (2xx).
// A presença real de GPU é verificada apenas no Execute (probe lazy/cacheado) —
// sem GPU o Execute retorna erro claro "GPU não detectada".
func (e *OllamaExecutor) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// ollamaGenerateRequest é o payload do POST /api/generate (stream desabilitado).
type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaGenerateResponse é a resposta do /api/generate com stream=false.
type ollamaGenerateResponse struct {
	Response      string `json:"response"`
	EvalCount     int64  `json:"eval_count"`
	EvalDuration  int64  `json:"eval_duration"` // nanosegundos
	TotalDuration int64  `json:"total_duration"`
}

// Execute roda a inferência no servidor Ollama.
func (e *OllamaExecutor) Execute(ctx context.Context, req GPURequest) (GPUResult, error) {
	gpu := e.detectGPU() // probe lazy/cacheado — custo pago aqui, não no construtor

	// Sem GPU o pipeline não tem o que usar — erro claro em vez de falha silenciosa.
	if gpu.Vendor == GPUNone {
		return GPUResult{}, fmt.Errorf(
			"GPU não detectada (probe: nenhuma GPU encontrada) — pipeline GPU indisponível")
	}

	// Guard de VRAM fail-fast (mesma regra usada no Fabric.SubmitGPU).
	if req.VRAMEstimateMB > 0 && req.VRAMEstimateMB > gpu.VRAMGB*1024 {
		return GPUResult{}, fmt.Errorf(
			"VRAM insuficiente: carga estima %d MB, GPU tem %d GB",
			req.VRAMEstimateMB, gpu.VRAMGB)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultGPUTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(ollamaGenerateRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
		Stream: false,
	})
	if err != nil {
		return GPUResult{}, fmt.Errorf("serializar request de geração: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return GPUResult{}, fmt.Errorf("montar request de geração: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return GPUResult{}, fmt.Errorf("chamar servidor Ollama: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return GPUResult{}, fmt.Errorf("ler resposta do servidor Ollama: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GPUResult{}, fmt.Errorf("servidor Ollama respondeu %s: %s", resp.Status, string(data))
	}

	var gen ollamaGenerateResponse
	if err := json.Unmarshal(data, &gen); err != nil {
		return GPUResult{}, fmt.Errorf("parse da resposta do servidor Ollama: %w", err)
	}

	result := GPUResult{
		Output:      gen.Response,
		TotalTokens: int(gen.EvalCount),
		Duration:    time.Duration(gen.EvalDuration),
	}
	if gen.EvalDuration > 0 {
		result.TokensPerSec = float64(gen.EvalCount) / (float64(gen.EvalDuration) / 1e9)
	}

	return result, nil
}
