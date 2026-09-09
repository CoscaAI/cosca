# GO BENCHMARKING & PROFILING — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-27

## Description
Systematic Go benchmarking and profiling for Cosca. Covers micro-benchmarks (testing.B), macro-benchmarks (end-to-end timing), CPU profiling (pprof), memory profiling, trace analysis, and continuous benchmark tracking in CI. Existing benchmarks at: internal/runtime/runtime_bench_test.go, internal/search/search_bench_test.go, etc.

## Benchmark Types

### Micro-Benchmarks (per function)
```go
// internal/search/search_bench_test.go
func BenchmarkSearch_FTS5_Small(b *testing.B) {
    engine := setupTestEngine(b, 1000) // 1K documents
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.Search(context.Background(), SearchQuery{Query: "test"})
    }
}

func BenchmarkSearch_Vector_Small(b *testing.B) {
    engine := setupTestEngine(b, 1000)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.VectorSearch(context.Background(), VectorQuery{Embedding: testVector})
    }
}

func BenchmarkSearch_Hybrid_Large(b *testing.B) {
    engine := setupTestEngine(b, 100000) // 100K documents
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.HybridSearch(context.Background(), SearchQuery{Query: "complex query"})
    }
}
```

### Comparative Benchmarks (A vs B)
```go
func BenchmarkProviderLatency(b *testing.B) {
    providers := []string{"openai", "anthropic", "ollama", "local"}
    for _, p := range providers {
        b.Run(p, func(b *testing.B) {
            provider := getProvider(p)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                provider.Chat(context.Background(), simpleRequest)
            }
        })
    }
}
```

## Running Benchmarks
```bash
# Run all benchmarks
go test -bench=. ./... -benchmem

# Run specific benchmark with count
go test -bench=BenchmarkSearch -benchmem -count=10 ./internal/search/

# Compare two implementations (benchstat)
go test -bench=BenchmarkSearch -count=10 ./internal/search/ > old.txt
# ... make changes ...
go test -bench=BenchmarkSearch -count=10 ./internal/search/ > new.txt
benchstat old.txt new.txt

# Benchmark with CPU profiling
go test -bench=BenchmarkSearch -cpuprofile=cpu.prof ./internal/search/
go tool pprof -http=:8080 cpu.prof
```

## Profiling

### CPU Profile
```bash
# Profile running server
curl -o cpu.prof http://localhost:14120/debug/pprof/profile?seconds=30
go tool pprof -http=:8080 cpu.prof

# In code
import _ "net/http/pprof"
go func() { http.ListenAndServe(":6060", nil) }()
```

### Memory Profile
```bash
curl -o mem.prof http://localhost:14120/debug/pprof/heap
go tool pprof -http=:8080 mem.prof

# Look for:
# - inuse_space: current memory usage
# - alloc_space: total allocated (find leaks)
```

### Goroutine Profile
```bash
curl http://localhost:14120/debug/pprof/goroutine?debug=2 | head -50
# Look for goroutine count growth over time (leaks)
```

### Trace (concurrency analysis)
```bash
curl -o trace.out http://localhost:14120/debug/pprof/trace?seconds=5
go tool trace trace.out
```

## Cosca Benchmark Targets

### Critical Paths (must benchmark)
| Component | Benchmark | Target |
|-----------|-----------|--------|
| Search FTS5 | 1K docs | < 5ms p50, < 20ms p99 |
| Search FTS5 | 100K docs | < 50ms p50, < 200ms p99 |
| Vector search | 1K vectors | < 10ms p50 |
| Hybrid search | 10K docs | < 30ms p50 |
| Memory record read | 1K records | < 5ms p50 |
| Indexing | 1K files | < 5s total |
| Startup cold | Clean state | < 500ms |
| Startup warm | Existing .cosca/ | < 100ms |
| Provider latency (local) | Simple query | < 50ms |
| Plugin load (Go) | Native | < 10ms |
| Plugin load (WASM) | wazero | < 50ms |

### CI Benchmark Tracking
```yaml
# .github/workflows/benchmark.yml
- name: Run benchmarks
  run: |
    go test -bench=. -benchmem -count=5 ./... | tee benchmark.txt

- name: Compare with baseline
  uses: benchmark-action/github-action-benchmark@v2
  with:
    tool: 'go'
    output-file-path: benchmark.txt
    github-token: ${{ secrets.GITHUB_TOKEN }}
    auto-push: true
    alert-threshold: '130%'  # Alert if >30% regression
```

## Key Rules
1. **b.ResetTimer()** after setup (exclude setup from measurement)
2. **b.StopTimer() / b.StartTimer()** for setup inside loop
3. **-benchmem** always (track allocations)
4. **-count=10** minimum for statistical significance
5. **benchstat** for comparing before/after
6. **Never benchmark in CI with -short** (production benchmarks separate)

## Anti-Patterns
- ❌ Benchmarking with compiler optimizations eliminating dead code
- ❌ Not resetting timer after setup
- ❌ Single run (always use -count=N)
- ❌ Running benchmarks on shared CI runner (noisy neighbors)
- ❌ Comparing benchmarks across different machines
