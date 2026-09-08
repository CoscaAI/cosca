// Package scanner provides a real-world scanner that analyzes
// all Go files in the project and reports findings.
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
	"cosca/internal/dsms/intelligence/expert"
)

// ============================================================
// SCANNER
// ============================================================

// FileResult represents analysis results for a single file.
type FileResult struct {
	FilePath    string                       `json:"file_path"`
	Lines       int                          `json:"lines"`
	Functions   int                          `json:"functions"`
	Findings    []*intelligence.RuleResult   `json:"findings"`
	Analysis    *codeanalyzer.AnalysisResult `json:"analysis"`
	Error       string                       `json:"error,omitempty"`
	AnalyzedAt  time.Time                    `json:"analyzed_at"`
}

// ScanResult represents the overall scan results.
type ScanResult struct {
	FilesScanned    int                       `json:"files_scanned"`
	FilesAnalyzed   int                       `json:"files_analyzed"`
	FilesWithIssues int                       `json:"files_with_issues"`
	TotalFindings   int                       `json:"total_findings"`
	TotalLines      int                       `json:"total_lines"`
	StartTime       time.Time                 `json:"start_time"`
	EndTime         time.Time                 `json:"end_time"`
	Duration        time.Duration             `json:"duration"`
	FindingsBySeverity map[string]int         `json:"findings_by_severity"`
	FindingsByDomain   map[string]int         `json:"findings_by_domain"`
	TopFindings        []*FindingSummary     `json:"top_findings"`
	Files              []*FileResult          `json:"files"`
}

// FindingSummary summarizes a finding across all files.
type FindingSummary struct {
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	Severity    string `json:"severity"`
	Domain      string `json:"domain"`
	Count       int    `json:"count"`
	Files       int    `json:"files"`
	ExamplePath string `json:"example_path"`
}

// Scanner scans Go files in a directory.
type Scanner struct {
	rootDir string
	workers int
}

// NewScanner creates a new scanner.
func NewScanner(rootDir string) *Scanner {
	return &Scanner{
		rootDir: rootDir,
		workers: 8,
	}
}

// Scan scans all Go files in the directory tree.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		StartTime:        time.Now(),
		FindingsBySeverity: make(map[string]int),
		FindingsByDomain:   make(map[string]int),
		Files:              make([]*FileResult, 0),
	}

	// Find all Go files
	var goFiles []string
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip vendor, node_modules, .git, dsms-test
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" || base == ".git" ||
				base == "dsms-test" || base == ".opencode" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.FilesScanned = len(goFiles)

	// Analyze files in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, s.workers)

	for _, file := range goFiles {
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()

			res := s.analyzeFile(path)
			mu.Lock()
			result.Files = append(result.Files, res)
			mu.Unlock()
		}(file)
	}

	wg.Wait()

	// Aggregate results
	for _, file := range result.Files {
		if file.Error == "" {
			result.FilesAnalyzed++
			result.TotalLines += file.Lines
		}

		if len(file.Findings) > 0 {
			result.FilesWithIssues++
			result.TotalFindings += len(file.Findings)

			for _, f := range file.Findings {
				result.FindingsBySeverity[f.Severity]++
				result.FindingsByDomain[f.Domain]++
			}
		}
	}

	// Build top findings
	result.TopFindings = s.buildTopFindings(result.Files)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// analyzeFile analyzes a single Go file.
func (s *Scanner) analyzeFile(path string) *FileResult {
	res := &FileResult{
		FilePath:   path,
		Findings:   make([]*intelligence.RuleResult, 0),
		AnalyzedAt: time.Now(),
	}

	// Read file
	code, err := os.ReadFile(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// Analyze with AST
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, err := analyzer.AnalyzeGo(path, string(code))
	if err != nil {
		res.Error = err.Error()
		return res
	}

	res.Analysis = analysis
	res.Lines = analysis.Lines
	res.Functions = analysis.FunctionCount

	// Build context
	ctx := &intelligence.Context{
		Language: analysis.Language,
		FilePath: path,
		Code:     string(code),
		Metrics:  analysis.Metrics,
		Patterns: extractPatterns(analysis),
	}

	// Run expert systems
	registry := expert.DefaultRegistry()
	for _, sys := range registry.All() {
		findings := sys.Analyze(ctx)
		res.Findings = append(res.Findings, findings...)
	}

	return res
}

// extractPatterns extracts pattern names from analysis results.
func extractPatterns(analysis *codeanalyzer.AnalysisResult) []string {
	if analysis == nil {
		return nil
	}
	var patterns []string
	for _, p := range analysis.Patterns {
		patterns = append(patterns, p.Pattern)
	}
	return patterns
}

// buildTopFindings aggregates findings across files.
func (s *Scanner) buildTopFindings(files []*FileResult) []*FindingSummary {
	byRule := make(map[string]*FindingSummary)

	for _, file := range files {
		for _, f := range file.Findings {
			key := f.RuleID
			if _, ok := byRule[key]; !ok {
				byRule[key] = &FindingSummary{
					RuleID:      f.RuleID,
					RuleName:    f.RuleName,
					Severity:    f.Severity,
					Domain:      f.Domain,
					ExamplePath: file.FilePath,
				}
			}
			byRule[key].Count++
			if byRule[key].ExamplePath == "" {
				byRule[key].ExamplePath = file.FilePath
			}
		}
	}

	// Count unique files per rule
	for _, file := range files {
		seen := make(map[string]bool)
		for _, f := range file.Findings {
			if !seen[f.RuleID] {
				seen[f.RuleID] = true
				if summary, ok := byRule[f.RuleID]; ok {
					summary.Files++
				}
			}
		}
	}

	// Sort by count descending
	var summaries []*FindingSummary
	for _, s := range byRule {
		summaries = append(summaries, s)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Count > summaries[j].Count
	})

	return summaries
}

// ============================================================
// REPORT
// ============================================================

// Report generates a human-readable scan report.
func Report(result *ScanResult) string {
	var sb strings.Builder

	sb.WriteString("\n════════════════════════════════════════════════════════════\n")
	sb.WriteString("  SCAN DO PROJETO — INTELLIGENCE ENGINE (zero LLM)\n")
	sb.WriteString("════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  Arquivos Go: %d\n", result.FilesScanned))
	sb.WriteString(fmt.Sprintf("  Analisados: %d\n", result.FilesAnalyzed))
	sb.WriteString(fmt.Sprintf("  Com problemas: %d\n", result.FilesWithIssues))
	sb.WriteString(fmt.Sprintf("  Total findings: %d\n", result.TotalFindings))
	sb.WriteString(fmt.Sprintf("  Total linhas: %d\n", result.TotalLines))
	sb.WriteString(fmt.Sprintf("  Tempo: %v\n", result.Duration))
	sb.WriteString("────────────────────────────────────────────────────────\n")

	// Severity breakdown
	sb.WriteString("\n── POR SEVERIDADE ──\n")
	severities := []string{"critical", "warning", "info"}
	for _, sev := range severities {
		count := result.FindingsBySeverity[sev]
		icon := "ℹ"
		switch sev {
		case "critical":
			icon = "✗"
		case "warning":
			icon = "!"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %d\n", icon, sev, count))
	}

	// Domain breakdown
	sb.WriteString("\n── POR DOMÍNIO ──\n")
	var domains []string
	for domain := range result.FindingsByDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", domain, result.FindingsByDomain[domain]))
	}

	// Top findings
	sb.WriteString("\n── TOP FINDINGS ──\n")
	for i, f := range result.TopFindings {
		if i >= 15 {
			break
		}
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s (%s): %d ocorrências em %d arquivos\n",
			i+1, f.Severity, f.RuleName, f.Domain, f.Count, f.Files))
		sb.WriteString(fmt.Sprintf("     Exemplo: %s\n", f.ExamplePath))
	}

	sb.WriteString("════════════════════════════════════════════════════════════\n")

	return sb.String()
}
