package procgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OpMetric carrega os metadados de performance de uma operação procedural —
// o embrião do Performance Memory (ordem do Professor: métricas nascem junto
// com o kernel, não depois). Tags JSON para telemetria.
type OpMetric struct {
	// Operation é o tipo do nó (ex: "fbm", "math_clamp").
	Operation string `json:"operation"`
	// Width/Height da imagem processada.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Backend de execução — sempre "cpu-go" nesta fase (puro Go, determinístico).
	Backend string `json:"backend"`
	// LatencyMs da operação (medida com time.Now() no executor).
	LatencyMs float64 `json:"latency_ms"`
	// MemoryKB estimado (W*H*4 bytes → KB).
	MemoryKB int64 `json:"memory_kb"`
	// CacheHit: cache por assinatura (§23) é decisão do grafo, não do nó —
	// o nó registra false; o Stats do graph/render carrega CachedHits reais.
	CacheHit bool `json:"cache_hit"`
	// Quality do render (1.0 nesta fase — ainda não há níveis por kernel).
	Quality float64 `json:"quality"`
	// Timestamp UTC para ordenação da série temporal.
	Timestamp string `json:"timestamp"`
}

// MetricsRecorder coleta OpMetric e despeja em JSONL append-only.
type MetricsRecorder struct {
	mu      sync.Mutex
	records []OpMetric
}

// Record acumula uma métrica (thread-safe).
func (m *MetricsRecorder) Record(mm OpMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mm.Timestamp == "" {
		mm.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if mm.Backend == "" {
		mm.Backend = "cpu-go"
	}
	m.records = append(m.records, mm)
}

// Len devolve quantas métricas foram gravadas.
func (m *MetricsRecorder) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

// Dump grava todas as métricas acumuladas em JSONL (uma OpMetric por linha,
// append-only) no caminho dado. Cria os diretórios pais. Falha se um registro
// não serializar.
func (m *MetricsRecorder) Dump(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if path == "" {
		return fmt.Errorf("procgen: metrics dump: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("procgen: metrics dump: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("procgen: metrics dump: open %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range m.records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("procgen: metrics dump: encode: %w", err)
		}
	}
	return nil
}
