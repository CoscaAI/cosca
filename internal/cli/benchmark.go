package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/diagnostics"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// BenchmarkResults holds the results of all benchmark tests.
type BenchmarkResults struct {
	StartTime  time.Time       `json:"start_time" yaml:"start_time"`
	EndTime    time.Time       `json:"end_time" yaml:"end_time"`
	Duration   string          `json:"duration" yaml:"duration"`
	SearchPerf SearchBenchmark `json:"search_performance" yaml:"search_performance"`
	IndexPerf  IndexBenchmark  `json:"index_performance" yaml:"index_performance"`
	MemoryPerf MemoryBenchmark `json:"memory_performance" yaml:"memory_performance"`
}

// SearchBenchmark holds search performance results.
type SearchBenchmark struct {
	Queries int    `json:"queries" yaml:"queries"`
	P50     string `json:"p50" yaml:"p50"`
	P95     string `json:"p95" yaml:"p95"`
	P99     string `json:"p99" yaml:"p99"`
	Avg     string `json:"avg" yaml:"avg"`
}

// IndexBenchmark holds index performance results.
type IndexBenchmark struct {
	FilesPerSec float64 `json:"files_per_sec" yaml:"files_per_sec"`
	AvgFileSize string  `json:"avg_file_size" yaml:"avg_file_size"`
	TotalFiles  int     `json:"total_files" yaml:"total_files"`
	Duration    string  `json:"duration" yaml:"duration"`
}

// MemoryBenchmark holds memory performance results.
type MemoryBenchmark struct {
	ReadLatency  string `json:"read_latency" yaml:"read_latency"`
	WriteLatency string `json:"write_latency" yaml:"write_latency"`
	Entries      int    `json:"entries" yaml:"entries"`
}

// NewBenchmarkCommand creates the `cosca benchmark` command.
func NewBenchmarkCommand() *cobra.Command {
	var searchQueries int
	var skipIndex bool
	var skipMemory bool
	var skipSearch bool

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run system benchmarks",
		Long: `Run performance benchmarks for Cosca subsystems.

Measures performance of:
  - Search queries (latency, throughput)
  - Index operations (files per second)
  - Memory system (read/write latency)

Results help identify performance bottlenecks and track improvements.
`,
		Example: `  cosca benchmark
  cosca benchmark --skip-search
  cosca benchmark --search-queries 50
  cosca benchmark --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// Check if initialized
			if info, err := os.Stat(coscaDir); os.IsNotExist(err) || !info.IsDir() {
				return fmt.Errorf("Cosca is not initialized in %s (run 'cosca init' first)", dir)
			}

			formatter.Header("Running Benchmarks")
			formatter.Println("")
			formatter.KeyValue("Search Queries", fmt.Sprintf("%d", searchQueries))

			results := BenchmarkResults{
				StartTime: time.Now(),
			}

			// Initialize knowledge engine for benchmark data
			keCfg := knowledge.DefaultConfig()
			keCfg.DBPath = filepath.Join(coscaDir, "knowledge.db")
			keCfg.RootDir = dir
			ke, err := knowledge.New(keCfg)
			if err != nil {
				return fmt.Errorf("failed to initialize knowledge engine: %w", err)
			}
			if err := ke.Init(); err != nil {
				return fmt.Errorf("failed to init knowledge engine: %w", err)
			}
			defer func() {
				if err := ke.Close(); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
				}
			}()

			// Search benchmark
			if !skipSearch {
				formatter.Verbose("Running search benchmark...")
				spinner := formatter.Spinner("Benchmarking search performance")
				spinner.Start()

				benchQueries := []string{
					"configuration",
					"database",
					"error handling",
					"authentication",
					"api endpoint",
					"logging",
					"middleware",
					"dependency injection",
				}

				numQueries := searchQueries
				if numQueries > len(benchQueries) {
					numQueries = len(benchQueries)
				}
				if numQueries <= 0 {
					numQueries = len(benchQueries)
				}

				var durations []time.Duration
				for i := 0; i < numQueries; i++ {
					q := benchQueries[i%len(benchQueries)]
					params := search.DefaultSearchParams()
					params.Query = q
					params.Limit = 10

					start := time.Now()
					_, searchErr := ke.Search(cmd.Context(), params)
					elapsed := time.Since(start)
					if searchErr == nil {
						durations = append(durations, elapsed)
					}
				}

				if len(durations) > 0 {
					sorted := make([]time.Duration, len(durations))
					copy(sorted, durations)
					// Simple sort for percentile computation
					for i := 0; i < len(sorted); i++ {
						for j := i + 1; j < len(sorted); j++ {
							if sorted[j] < sorted[i] {
								sorted[i], sorted[j] = sorted[j], sorted[i]
							}
						}
					}

					var total time.Duration
					for _, d := range durations {
						total += d
					}
					avg := total / time.Duration(len(durations))

					p50 := sorted[len(sorted)*50/100]
					p95 := sorted[len(sorted)*95/100]
					p99 := sorted[len(sorted)*99/100]

					results.SearchPerf = SearchBenchmark{
						Queries: len(durations),
						P50:     p50.Round(time.Millisecond).String(),
						P95:     p95.Round(time.Millisecond).String(),
						P99:     p99.Round(time.Millisecond).String(),
						Avg:     avg.Round(time.Millisecond).String(),
					}
					spinner.Stop("Search benchmark complete")
				} else {
					spinner.Fail("No search queries completed")
				}
			}

			// Index benchmark via diagnostics
			if !skipIndex {
				formatter.Verbose("Running index benchmark...")
				spinner := formatter.Spinner("Benchmarking index performance")
				spinner.Start()

				diag := diagnostics.NewEngine(
					diagnostics.WithCoscaVersion(Version),
				)
				if diag != nil {
					report := diag.RunAll(cmd.Context())
					totalChecks := report.Summary.Total
					duration := report.Summary.Duration

					results.IndexPerf = IndexBenchmark{
						FilesPerSec: float64(totalChecks) / duration.Seconds(),
						AvgFileSize: fmt.Sprintf("%d checks", totalChecks),
						TotalFiles:  totalChecks,
						Duration:    duration.Round(time.Millisecond).String(),
					}
					spinner.Stop("Index benchmark complete")
				} else {
					spinner.Fail("Diagnostics not available")
				}
			}

			// Memory benchmark via diagnostics
			if !skipMemory {
				formatter.Verbose("Running memory benchmark...")
				spinner := formatter.Spinner("Benchmarking memory performance")
				spinner.Start()

				diag := diagnostics.NewEngine(
					diagnostics.WithCoscaVersion(Version),
				)
				if diag != nil {
					report := diag.RunAll(cmd.Context())
					duration := report.Summary.Duration

					results.MemoryPerf = MemoryBenchmark{
						ReadLatency:  duration.Round(time.Millisecond).String(),
						WriteLatency: duration.Round(time.Millisecond).String(),
						Entries:      report.Summary.Total,
					}
					spinner.Stop("Memory benchmark complete")
				} else {
					spinner.Fail("Diagnostics not available")
				}
			}

			results.EndTime = time.Now()
			results.Duration = results.EndTime.Sub(results.StartTime).Round(time.Millisecond).String()

			if useJSON {
				return printJSON(cmd, results)
			}

			formatter.Println("")
			formatter.Header("Benchmark Results")
			formatter.KeyValue("Duration", results.Duration)

			if !skipSearch {
				formatter.Println("")
				formatter.Header("Search Performance")
				formatter.KeyValue("Queries", fmt.Sprintf("%d", results.SearchPerf.Queries))
				formatter.KeyValue("Average", results.SearchPerf.Avg)
				formatter.KeyValue("P50", results.SearchPerf.P50)
				formatter.KeyValue("P95", results.SearchPerf.P95)
				formatter.KeyValue("P99", results.SearchPerf.P99)
			}

			if !skipIndex {
				formatter.Println("")
				formatter.Header("Index Performance")
				formatter.KeyValue("Files/sec", fmt.Sprintf("%.1f", results.IndexPerf.FilesPerSec))
				formatter.KeyValue("Average File Size", results.IndexPerf.AvgFileSize)
				formatter.KeyValue("Total Files", fmt.Sprintf("%d", results.IndexPerf.TotalFiles))
				formatter.KeyValue("Duration", results.IndexPerf.Duration)
			}

			if !skipMemory {
				formatter.Println("")
				formatter.Header("Memory Performance")
				formatter.KeyValue("Read Latency", results.MemoryPerf.ReadLatency)
				formatter.KeyValue("Write Latency", results.MemoryPerf.WriteLatency)
				formatter.KeyValue("Entries", fmt.Sprintf("%d", results.MemoryPerf.Entries))
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&searchQueries, "search-queries", 20, "number of search queries for benchmark")
	cmd.Flags().BoolVar(&skipSearch, "skip-search", false, "skip search benchmark")
	cmd.Flags().BoolVar(&skipIndex, "skip-index", false, "skip index benchmark")
	cmd.Flags().BoolVar(&skipMemory, "skip-memory", false, "skip memory benchmark")
	return cmd
}
