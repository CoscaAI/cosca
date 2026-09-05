// Package metrics provides Prometheus-compatible metrics exposition for the
// Cosca platform. It collects HTTP and gRPC request counters and formats
// runtime, knowledge, memory, process, and request metrics into the
// Prometheus text exposition format (text/plain; version=0.0.4).
//
// All collectors use mutex-protected maps for thread safety. The format
// function uses only the standard library — no external Prometheus client
// dependency.
package metrics

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	coscaRuntime "github.com/CoscaAI/cosca/internal/runtime"
)

// =============================================================================
// HTTP Metrics Collector
// =============================================================================

// HTTPMetrics collects HTTP request metrics. It is safe for concurrent use.
type HTTPMetrics struct {
	mu             sync.Mutex
	requestCount   map[string]int64 // "METHOD PATH STATUS" -> count
	requestTotalNs map[string]int64 // "METHOD PATH STATUS" -> total ns
}

// NewHTTPMetrics creates a new HTTP metrics collector.
func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		requestCount:   make(map[string]int64),
		requestTotalNs: make(map[string]int64),
	}
}

// Record records an HTTP request with the given method, path, status code,
// and duration. The path should be the route pattern (e.g. "GET /v1/memory/search").
func (m *HTTPMetrics) Record(method, path string, status int, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build key: gracefully handle missing method/path by falling back to
	// request values. The path must be the route pattern for proper grouping.
	if method == "" {
		method = "GET"
	}
	if path == "" {
		path = "/"
	}

	key := method + " " + path + " " + strconv.Itoa(status)
	m.requestCount[key]++
	m.requestTotalNs[key] += int64(d)
}

// snapshot returns a copy of the current counters. Callers must hold m.mu.
func (m *HTTPMetrics) snapshot() (keys []string, counts, totalNs map[string]int64) {
	keys = make([]string, 0, len(m.requestCount))
	counts = make(map[string]int64, len(m.requestCount))
	totalNs = make(map[string]int64, len(m.requestTotalNs))

	for k, v := range m.requestCount {
		keys = append(keys, k)
		counts[k] = v
	}
	for k, v := range m.requestTotalNs {
		totalNs[k] = v
	}
	sort.Strings(keys)
	return
}

// =============================================================================
// gRPC Metrics Collector
// =============================================================================

// GRPCMetrics collects gRPC request metrics. It is safe for concurrent use.
type GRPCMetrics struct {
	mu             sync.Mutex
	requestCount   map[string]int64 // "FULL_METHOD CODE" -> count
	requestTotalNs map[string]int64 // "FULL_METHOD CODE" -> total ns
}

// NewGRPCMetrics creates a new gRPC metrics collector.
func NewGRPCMetrics() *GRPCMetrics {
	return &GRPCMetrics{
		requestCount:   make(map[string]int64),
		requestTotalNs: make(map[string]int64),
	}
}

// Record records a gRPC call with the given full method name, status code,
// and duration.
func (m *GRPCMetrics) Record(method, code string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := method + " " + code
	m.requestCount[key]++
	m.requestTotalNs[key] += int64(d)
}

// snapshot returns a copy of the current counters. Callers must hold m.mu.
func (m *GRPCMetrics) snapshot() (keys []string, counts, totalNs map[string]int64) {
	keys = make([]string, 0, len(m.requestCount))
	counts = make(map[string]int64, len(m.requestCount))
	totalNs = make(map[string]int64, len(m.requestTotalNs))

	for k, v := range m.requestCount {
		keys = append(keys, k)
		counts[k] = v
	}
	for k, v := range m.requestTotalNs {
		totalNs[k] = v
	}
	sort.Strings(keys)
	return
}

// =============================================================================
// Prometheus Text Format — FormatMetrics
// =============================================================================

// FormatMetrics returns a Prometheus text/plain exposition payload with all
// Cosca metrics. Parameters may be nil if the corresponding subsystem is
// not available.
func FormatMetrics(
	rt *coscaRuntime.Runtime,
	ke *knowledge.Engine,
	mem *memory.MemoryEngine,
	httpMetrics *HTTPMetrics,
	grpcMetrics *GRPCMetrics,
) string {
	var b strings.Builder

	// ── Process metrics ────────────────────────────────────────────────
	writeProcessMetrics(&b)

	// ── Runtime metrics ────────────────────────────────────────────────
	if rt != nil {
		writeRuntimeMetrics(&b, rt)
	}

	// ── Knowledge engine metrics ───────────────────────────────────────
	if ke != nil {
		writeKnowledgeMetrics(&b, ke)
	}

	// ── Memory engine metrics ──────────────────────────────────────────
	if mem != nil {
		writeMemoryMetrics(&b, mem)
	}

	// ── HTTP request metrics ───────────────────────────────────────────
	if httpMetrics != nil {
		writeHTTPMetrics(&b, httpMetrics)
	}

	// ── gRPC request metrics ───────────────────────────────────────────
	if grpcMetrics != nil {
		writeGRPCMetrics(&b, grpcMetrics)
	}

	return b.String()
}

// =============================================================================
// Process metrics
// =============================================================================

func writeProcessMetrics(b *strings.Builder) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	b.WriteString("# HELP cosca_process_goroutines Number of goroutines currently running.\n")
	b.WriteString("# TYPE cosca_process_goroutines gauge\n")
	fmt.Fprintf(b, "cosca_process_goroutines %d\n", runtime.NumGoroutine())

	b.WriteString("# HELP cosca_process_memory_alloc_bytes Heap memory currently allocated.\n")
	b.WriteString("# TYPE cosca_process_memory_alloc_bytes gauge\n")
	fmt.Fprintf(b, "cosca_process_memory_alloc_bytes %d\n", m.Alloc)

	b.WriteString("# HELP cosca_process_memory_sys_bytes Total OS memory obtained from the system.\n")
	b.WriteString("# TYPE cosca_process_memory_sys_bytes gauge\n")
	fmt.Fprintf(b, "cosca_process_memory_sys_bytes %d\n", m.Sys)

	b.WriteString("# HELP cosca_process_memory_total_alloc_bytes Total bytes allocated over process lifetime.\n")
	b.WriteString("# TYPE cosca_process_memory_total_alloc_bytes counter\n")
	fmt.Fprintf(b, "cosca_process_memory_total_alloc_bytes %d\n", m.TotalAlloc)

	b.WriteString("# HELP cosca_process_gc_pause_ns Last GC pause duration in nanoseconds.\n")
	b.WriteString("# TYPE cosca_process_gc_pause_ns gauge\n")
	fmt.Fprintf(b, "cosca_process_gc_pause_ns %d\n", m.PauseNs[(m.NumGC+255)%256])

	b.WriteString("# HELP cosca_process_gc_total Total number of GC cycles.\n")
	b.WriteString("# TYPE cosca_process_gc_total counter\n")
	fmt.Fprintf(b, "cosca_process_gc_total %d\n", m.NumGC)
}

// =============================================================================
// Runtime metrics
// =============================================================================

func writeRuntimeMetrics(b *strings.Builder, rt *coscaRuntime.Runtime) {
	state := rt.State().Current().String()
	health := rt.Health()
	uptime := rt.Metrics().Uptime().Seconds()
	snap := rt.Metrics().Snapshot()

	// Uptime
	b.WriteString("# HELP cosca_runtime_uptime_seconds Runtime uptime in seconds.\n")
	b.WriteString("# TYPE cosca_runtime_uptime_seconds gauge\n")
	fmt.Fprintf(b, "cosca_runtime_uptime_seconds %.2f\n", uptime)

	// Health
	b.WriteString("# HELP cosca_runtime_health Runtime health status (1=healthy, 0=unhealthy).\n")
	b.WriteString("# TYPE cosca_runtime_health gauge\n")
	if health == coscaRuntime.StatusHealthy || health == coscaRuntime.StatusUnknown {
		b.WriteString("cosca_runtime_health 1\n")
	} else {
		b.WriteString("cosca_runtime_health 0\n")
	}

	// State info
	b.WriteString("# HELP cosca_runtime_state_info Runtime state info.\n")
	b.WriteString("# TYPE cosca_runtime_state_info gauge\n")
	fmt.Fprintf(b, "cosca_runtime_state_info{state=\"%s\"} 1\n", state)

	// Operation counters
	b.WriteString("# HELP cosca_runtime_operations_total Total operations by type.\n")
	b.WriteString("# TYPE cosca_runtime_operations_total counter\n")
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"index\"} %d\n", snap.IndexCount)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"search\"} %d\n", snap.SearchCount)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"context_build\"} %d\n", snap.ContextBuilds)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"memory_store\"} %d\n", snap.MemoryStores)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"plugin_call\"} %d\n", snap.PluginCalls)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"error\"} %d\n", snap.ErrorCount)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"sync\"} %d\n", snap.SyncCount)
	fmt.Fprintf(b, "cosca_runtime_operations_total{operation=\"event\"} %d\n", snap.EventCount)

	// Duration percentiles for index operations
	b.WriteString("# HELP cosca_runtime_duration_seconds Operation duration percentiles.\n")
	b.WriteString("# TYPE cosca_runtime_duration_seconds gauge\n")
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"index\",quantile=\"0.5\"} %.6f\n", snap.IndexDurationP50.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"index\",quantile=\"0.95\"} %.6f\n", snap.IndexDurationP95.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"index\",quantile=\"0.99\"} %.6f\n", snap.IndexDurationP99.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"search\",quantile=\"0.5\"} %.6f\n", snap.SearchDurationP50.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"search\",quantile=\"0.95\"} %.6f\n", snap.SearchDurationP95.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"search\",quantile=\"0.99\"} %.6f\n", snap.SearchDurationP99.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"context\",quantile=\"0.5\"} %.6f\n", snap.ContextDurationP50.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"context\",quantile=\"0.95\"} %.6f\n", snap.ContextDurationP95.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"context\",quantile=\"0.99\"} %.6f\n", snap.ContextDurationP99.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"memory\",quantile=\"0.5\"} %.6f\n", snap.MemoryDurationP50.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"memory\",quantile=\"0.95\"} %.6f\n", snap.MemoryDurationP95.Seconds())
	fmt.Fprintf(b, "cosca_runtime_duration_seconds{operation=\"memory\",quantile=\"0.99\"} %.6f\n", snap.MemoryDurationP99.Seconds())

	// Goroutine stats
	b.WriteString("# HELP cosca_runtime_goroutines Current goroutine count (sampled).\n")
	b.WriteString("# TYPE cosca_runtime_goroutines gauge\n")
	fmt.Fprintf(b, "cosca_runtime_goroutines %d\n", snap.Goroutines)

	// Component health
	b.WriteString("# HELP cosca_runtime_component_health Component health status (1=healthy, 0=unhealthy).\n")
	b.WriteString("# TYPE cosca_runtime_component_health gauge\n")
	for name, status := range snap.ComponentHealth {
		val := 0
		if status == coscaRuntime.StatusHealthy {
			val = 1
		}
		fmt.Fprintf(b, "cosca_runtime_component_health{component=\"%s\"} %d\n", name, val)
	}
}

// =============================================================================
// Knowledge engine metrics
// =============================================================================

func writeKnowledgeMetrics(b *strings.Builder, ke *knowledge.Engine) {
	stats, err := ke.GetStats()
	if err != nil {
		return
	}

	b.WriteString("# HELP cosca_knowledge_documents_total Total indexed documents.\n")
	b.WriteString("# TYPE cosca_knowledge_documents_total gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_documents_total %d\n", stats.DocumentCount)

	b.WriteString("# HELP cosca_knowledge_chunks_total Total indexed chunks.\n")
	b.WriteString("# TYPE cosca_knowledge_chunks_total gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_chunks_total %d\n", stats.ChunkCount)

	b.WriteString("# HELP cosca_knowledge_entities_total Total extracted entities.\n")
	b.WriteString("# TYPE cosca_knowledge_entities_total gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_entities_total %d\n", stats.EntityCount)

	b.WriteString("# HELP cosca_knowledge_vectors_total Total vector embeddings.\n")
	b.WriteString("# TYPE cosca_knowledge_vectors_total gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_vectors_total %d\n", stats.VectorCount)

	b.WriteString("# HELP cosca_knowledge_graph_nodes Graph nodes count.\n")
	b.WriteString("# TYPE cosca_knowledge_graph_nodes gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_graph_nodes %d\n", stats.GraphStats.Nodes)

	b.WriteString("# HELP cosca_knowledge_db_size_bytes Database file size in bytes.\n")
	b.WriteString("# TYPE cosca_knowledge_db_size_bytes gauge\n")
	fmt.Fprintf(b, "cosca_knowledge_db_size_bytes %d\n", stats.DBSize)

	if stats.EmbeddingStats != nil {
		b.WriteString("# HELP cosca_knowledge_embedding_requests_total Total embedding API requests.\n")
		b.WriteString("# TYPE cosca_knowledge_embedding_requests_total counter\n")
		fmt.Fprintf(b, "cosca_knowledge_embedding_requests_total %d\n", stats.EmbeddingStats.TotalRequests)

		b.WriteString("# HELP cosca_knowledge_embedding_tokens_total Total tokens processed by embedding API.\n")
		b.WriteString("# TYPE cosca_knowledge_embedding_tokens_total counter\n")
		fmt.Fprintf(b, "cosca_knowledge_embedding_tokens_total %d\n", stats.EmbeddingStats.TotalTokens)
	}

	// Cache stats
	if len(stats.CacheStats) > 0 {
		b.WriteString("# HELP cosca_knowledge_cache_entries Cache entries by tier.\n")
		b.WriteString("# TYPE cosca_knowledge_cache_entries gauge\n")
		if v, ok := stats.CacheStats["memory_entries"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_entries{tier=\"memory\"} %d\n", v)
		}
		if v, ok := stats.CacheStats["sqlite_entries"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_entries{tier=\"sqlite\"} %d\n", v)
		}
		if v, ok := stats.CacheStats["file_entries"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_entries{tier=\"file\"} %d\n", v)
		}

		b.WriteString("# HELP cosca_knowledge_cache_hits_total Cache hits by tier.\n")
		b.WriteString("# TYPE cosca_knowledge_cache_hits_total counter\n")
		if v, ok := stats.CacheStats["memory_hits"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_hits_total{tier=\"memory\"} %d\n", v)
		}
		if v, ok := stats.CacheStats["sqlite_hits"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_hits_total{tier=\"sqlite\"} %d\n", v)
		}
		if v, ok := stats.CacheStats["file_hits"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_hits_total{tier=\"file\"} %d\n", v)
		}

		b.WriteString("# HELP cosca_knowledge_cache_misses_total Cache misses.\n")
		b.WriteString("# TYPE cosca_knowledge_cache_misses_total counter\n")
		if v, ok := stats.CacheStats["misses"]; ok {
			fmt.Fprintf(b, "cosca_knowledge_cache_misses_total %d\n", v)
		}
	}
}

// =============================================================================
// Memory engine metrics
// =============================================================================

func writeMemoryMetrics(b *strings.Builder, mem *memory.MemoryEngine) {
	layerStats := mem.GetLayerStats(context.Background())

	totalRecords := 0
	totalSize := 0
	for _, ls := range layerStats {
		totalRecords += ls.Count
		totalSize += ls.TotalSize
	}

	b.WriteString("# HELP cosca_memory_records_total Total memory records.\n")
	b.WriteString("# TYPE cosca_memory_records_total gauge\n")
	fmt.Fprintf(b, "cosca_memory_records_total %d\n", totalRecords)

	b.WriteString("# HELP cosca_memory_records_size_bytes Total size of memory records in bytes.\n")
	b.WriteString("# TYPE cosca_memory_records_size_bytes gauge\n")
	fmt.Fprintf(b, "cosca_memory_records_size_bytes %d\n", totalSize)

	b.WriteString("# HELP cosca_memory_layer_records Records per memory layer.\n")
	b.WriteString("# TYPE cosca_memory_layer_records gauge\n")
	for layer, ls := range layerStats {
		fmt.Fprintf(b, "cosca_memory_layer_records{layer=\"%s\"} %d\n", string(layer), ls.Count)
	}

	b.WriteString("# HELP cosca_memory_layer_size_bytes Size per memory layer in bytes.\n")
	b.WriteString("# TYPE cosca_memory_layer_size_bytes gauge\n")
	for layer, ls := range layerStats {
		fmt.Fprintf(b, "cosca_memory_layer_size_bytes{layer=\"%s\"} %d\n", string(layer), ls.TotalSize)
	}
}

// =============================================================================
// Helpers
// =============================================================================
// HTTP request metrics
// =============================================================================

func writeHTTPMetrics(b *strings.Builder, m *HTTPMetrics) {
	m.mu.Lock()
	keys, counts, totalNs := m.snapshot()
	m.mu.Unlock()

	if len(keys) == 0 {
		return
	}

	b.WriteString("# HELP cosca_http_requests_total Total HTTP requests.\n")
	b.WriteString("# TYPE cosca_http_requests_total counter\n")
	for _, key := range keys {
		// key format: "METHOD PATH STATUS"
		parts := splitKey(key, 3)
		if len(parts) != 3 {
			continue
		}
		method, path, status := parts[0], parts[1], parts[2]
		fmt.Fprintf(b, "cosca_http_requests_total{method=\"%s\",path=\"%s\",status_code=\"%s\"} %d\n",
			method, path, status, counts[key])
	}

	b.WriteString("# HELP cosca_http_request_duration_seconds_sum Sum of HTTP request durations.\n")
	b.WriteString("# TYPE cosca_http_request_duration_seconds_sum counter\n")
	for _, key := range keys {
		parts := splitKey(key, 3)
		if len(parts) != 3 {
			continue
		}
		method, path, status := parts[0], parts[1], parts[2]
		fmt.Fprintf(b, "cosca_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\",status_code=\"%s\"} %.6f\n",
			method, path, status, float64(totalNs[key])/1e9)
	}

	b.WriteString("# HELP cosca_http_request_duration_seconds_count Count of HTTP requests for duration metric.\n")
	b.WriteString("# TYPE cosca_http_request_duration_seconds_count counter\n")
	for _, key := range keys {
		parts := splitKey(key, 3)
		if len(parts) != 3 {
			continue
		}
		method, path, status := parts[0], parts[1], parts[2]
		fmt.Fprintf(b, "cosca_http_request_duration_seconds_count{method=\"%s\",path=\"%s\",status_code=\"%s\"} %d\n",
			method, path, status, counts[key])
	}
}

// =============================================================================
// gRPC request metrics
// =============================================================================

func writeGRPCMetrics(b *strings.Builder, m *GRPCMetrics) {
	m.mu.Lock()
	keys, counts, totalNs := m.snapshot()
	m.mu.Unlock()

	if len(keys) == 0 {
		return
	}

	b.WriteString("# HELP cosca_grpc_requests_total Total gRPC requests.\n")
	b.WriteString("# TYPE cosca_grpc_requests_total counter\n")
	for _, key := range keys {
		// key format: "FULL_METHOD CODE"
		parts := splitKey(key, 2)
		if len(parts) != 2 {
			continue
		}
		method, code := parts[0], parts[1]
		fmt.Fprintf(b, "cosca_grpc_requests_total{method=\"%s\",code=\"%s\"} %d\n",
			method, code, counts[key])
	}

	b.WriteString("# HELP cosca_grpc_request_duration_seconds_sum Sum of gRPC request durations.\n")
	b.WriteString("# TYPE cosca_grpc_request_duration_seconds_sum counter\n")
	for _, key := range keys {
		parts := splitKey(key, 2)
		if len(parts) != 2 {
			continue
		}
		method, code := parts[0], parts[1]
		fmt.Fprintf(b, "cosca_grpc_request_duration_seconds_sum{method=\"%s\",code=\"%s\"} %.6f\n",
			method, code, float64(totalNs[key])/1e9)
	}

	b.WriteString("# HELP cosca_grpc_request_duration_seconds_count Count of gRPC requests for duration metric.\n")
	b.WriteString("# TYPE cosca_grpc_request_duration_seconds_count counter\n")
	for _, key := range keys {
		parts := splitKey(key, 2)
		if len(parts) != 2 {
			continue
		}
		method, code := parts[0], parts[1]
		fmt.Fprintf(b, "cosca_grpc_request_duration_seconds_count{method=\"%s\",code=\"%s\"} %d\n",
			method, code, counts[key])
	}
}

// =============================================================================
// Helpers
// =============================================================================

// splitKey splits a string by spaces into at most n parts, consuming all
// remaining tokens into the last part. This ensures that a key like
// "GET /v1/memory/search 200" is split into ["GET", "/v1/memory/search", "200"].
func splitKey(s string, n int) []string {
	if n <= 1 {
		return []string{s}
	}

	parts := make([]string, 0, n)
	remaining := s
	for i := 0; i < n-1; i++ {
		idx := strings.Index(remaining, " ")
		if idx < 0 {
			parts = append(parts, remaining)
			return parts
		}
		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+1:]
	}
	parts = append(parts, remaining)
	return parts
}
