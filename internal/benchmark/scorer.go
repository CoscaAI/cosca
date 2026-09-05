package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Scorer persists and aggregates benchmark results.
type Scorer struct {
	outputDir string
}

// NewScorer creates a scorer that writes to the given directory.
func NewScorer(outputDir string) *Scorer {
	os.MkdirAll(outputDir, 0o755)
	return &Scorer{outputDir: outputDir}
}

// Save persists a bench result as JSON.
func (s *Scorer) Save(result *BenchResult) error {
	filename := filepath.Join(s.outputDir,
		fmt.Sprintf("%s_%s_%s.json",
			result.Benchmark, result.Agent, result.CompletedAt.Format("20060102-150405")))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return os.WriteFile(filename, data, 0o644)
}

// FormatReport returns a human-readable summary.
func (s *Scorer) FormatReport(result *BenchResult) string {
	return fmt.Sprintf(`Benchmark: %s
Agent:     %s
Model:     %s
Total:     %d
Correct:   %d
Accuracy:  %.1f%%
Avg Score: %.2f
Duration:  %dms
Tokens:    %d
`,
		result.Benchmark, result.Agent, result.Model,
		result.Total, result.Correct,
		result.Accuracy*100, result.AvgScore,
		result.DurationMs, result.TokensUsed,
	)
}

// Load loads a bench result from a JSON file.
func (s *Scorer) Load(filename string) (*BenchResult, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var result BenchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns all result files in the output directory.
func (s *Scorer) List() ([]string, error) {
	entries, err := os.ReadDir(s.outputDir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, filepath.Join(s.outputDir, e.Name()))
		}
	}
	return files, nil
}

// Compare loads two results and returns a diff summary.
func (s *Scorer) Compare(a, b *BenchResult) string {
	delta := a.Accuracy - b.Accuracy
	dir := "↓"
	if delta > 0 {
		dir = "↑"
	}
	return fmt.Sprintf("%s: %.1f%% → %.1f%% (%s%.1f%%)",
		a.Agent, a.Accuracy*100, b.Accuracy*100, dir, delta*100)
}

// ensure context import
var _ context.Context
