// Package csnparser reads CodeSearchNet parquet files and extracts
// (docstring, code) pairs to train the intelligence engine.
package csnparser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// CSN PARSER
// ============================================================

// CodePair represents a (docstring, code) pair.
type CodePair struct {
	Language   string `json:"language"`
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	FuncName   string `json:"func_name"`
	Code       string `json:"code"`
	Docstring  string `json:"docstring"`
	URL        string `json:"url"`
}

// ParseResult represents the result of parsing.
type ParseResult struct {
	FilesRead     int            `json:"files_read"`
	RowsRead      int            `json:"rows_read"`
	RowsWithCode  int            `json:"rows_with_code"`
	RowsWithDoc   int            `json:"rows_with_doc"`
	ByLanguage    map[string]int `json:"by_language"`
	SamplePairs   []*CodePair    `json:"sample_pairs"`
	Duration      time.Duration  `json:"duration"`
}

// Parser reads CodeSearchNet parquet files.
type Parser struct {
	Dir string
}

// NewParser creates a new parser.
func NewParser(dir string) *Parser {
	return &Parser{Dir: dir}
}

// Parse reads all parquet files in the directory.
func (p *Parser) Parse(ctx context.Context, maxFiles int) (*ParseResult, error) {
	start := time.Now()

	result := &ParseResult{
		ByLanguage:  make(map[string]int),
		SamplePairs: make([]*CodePair, 0),
	}

	// Find parquet files
	files, err := filepath.Glob(filepath.Join(p.Dir, "*.parquet"))
	if err != nil {
		return nil, err
	}

	if maxFiles > 0 && len(files) > maxFiles {
		files = files[:maxFiles]
	}

	result.FilesRead = len(files)

	for _, file := range files {
		if err := p.parseFile(ctx, file, result); err != nil {
			fmt.Printf("  [warn] %s: %v\n", filepath.Base(file), err)
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// parseFile reads a single parquet file using low-level ReadRows.
func (p *Parser) parseFile(ctx context.Context, file string, result *ParseResult) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// Low-level reader
	reader := parquet.NewReader(f)
	defer reader.Close()

	// Read rows in batches
	rows := make([]parquet.Row, 1000)
	for {
		n, err := reader.ReadRows(rows)
		if err != nil && n == 0 {
			break
		}

		for i := 0; i < n; i++ {
			pair := rowToPair(rows[i])
			result.RowsRead++

			// Track language
			lang := pair.Language
			if lang == "" {
				lang = detectLanguage(pair.Path)
			}
			result.ByLanguage[lang]++

			// Track rows with code
			if strings.TrimSpace(pair.Code) != "" {
				result.RowsWithCode++
			}

			// Track rows with docstring
			if strings.TrimSpace(pair.Docstring) != "" {
				result.RowsWithDoc++
			}

			// Collect sample pairs
			if len(result.SamplePairs) < 10 &&
				strings.TrimSpace(pair.Code) != "" &&
				strings.TrimSpace(pair.Docstring) != "" {
				result.SamplePairs = append(result.SamplePairs, pair)
			}
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// rowToPair converts a raw parquet row to a CodePair.
// Column order (from schema): 0=repo, 1=path, 2=func_name, 3=original_string,
// 4=language, 5=code, 6=code_tokens, 7=docstring, 8=docstring_tokens, 9=score, 10=url
func rowToPair(row parquet.Row) *CodePair {
	pair := &CodePair{}

	for _, value := range row {
		col := value.Column()
		switch col {
		case 0:
			pair.Repo = value.String()
		case 1:
			pair.Path = value.String()
		case 2:
			pair.FuncName = value.String()
		case 3:
			// original_string - fallback if code empty
			if pair.Code == "" {
				pair.Code = value.String()
			}
		case 4:
			pair.Language = value.String()
		case 5:
			pair.Code = value.String()
		case 7:
			pair.Docstring = value.String()
		case 10:
			pair.URL = value.String()
		}
	}

	return pair
}

// filepathGlob returns parquet files in a directory.
func filepathGlob(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, "*.parquet"))
}

// ExtractAllRules reads ALL pairs from all parquet files and extracts rules.
// This is the full-scale training path used by the CLI.
func ExtractAllRules(ctx context.Context, dir string) ([]*intelligence.Rule, *ParseResult, error) {
	parser := &Parser{Dir: dir}
	result, err := parser.Parse(ctx, 0)
	if err != nil {
		return nil, result, err
	}

	// Read all pairs
	var allPairs []*CodePair
	files, err := filepathGlob(dir)
	if err != nil {
		return nil, result, err
	}

	for _, file := range files {
		pairs := readFilePairs(file, ctx)
		allPairs = append(allPairs, pairs...)
	}

	// Extract rules from ALL pairs
	rules := ExtractRules(allPairs)
	return rules, result, nil
}

// readFilePairs reads all pairs from a single parquet file.
func readFilePairs(file string, ctx context.Context) []*CodePair {
	var pairs []*CodePair

	f, err := os.Open(file)
	if err != nil {
		return pairs
	}
	defer f.Close()

	reader := parquet.NewReader(f)
	defer reader.Close()

	rows := make([]parquet.Row, 1000)
	for {
		n, err := reader.ReadRows(rows)
		if err != nil && n == 0 {
			break
		}

		for i := 0; i < n; i++ {
			pair := rowToPair(rows[i])
			if strings.TrimSpace(pair.Code) != "" && strings.TrimSpace(pair.Docstring) != "" {
				pairs = append(pairs, pair)
			}
		}

		select {
		case <-ctx.Done():
			return pairs
		default:
		}
	}

	return pairs
}

// ============================================================
// RULE EXTRACTION
// ============================================================

// ExtractRules converts code pairs into rules.
// Each pair (docstring → code) becomes a rule:
// "if the code does X, the intent is Y"
func ExtractRules(pairs []*CodePair) []*intelligence.Rule {
	var rules []*intelligence.Rule
	now := time.Now()

	for _, pair := range pairs {
		if pair == nil || strings.TrimSpace(pair.Code) == "" {
			continue
		}

		codeLower := strings.ToLower(pair.Code)
		docLower := strings.ToLower(pair.Docstring)

		// SECURITY: code mentions query/exec + docstring mentions injection/security
		if (strings.Contains(codeLower, "query") || strings.Contains(codeLower, "exec")) &&
			containsAny(docLower, []string{"inject", "security", "sanitize", "validate", "paramet"}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "CSN-SEC-" + hashID(pair.Repo+pair.FuncName),
				Domain:      "security",
				Name:        "Security Pattern",
				Description: "CodeSearchNet: " + pair.Docstring,
				Condition: intelligence.Condition{
					Operator: "HAS_PATTERN",
					Field:    "patterns",
					Value:    "sql_injection",
				},
				Action: intelligence.Action{
					Type:     "flag",
					Severity: "critical",
					Message:  "Security: parameterize queries and validate inputs",
					Params:   map[string]interface{}{"source": "CodeSearchNet", "repo": pair.Repo},
				},
				Priority:   90,
				Confidence: 0.85,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// PERFORMANCE: code has loops + docstring mentions performance/optimize
		if (strings.Contains(codeLower, "for ") || strings.Contains(codeLower, "while ")) &&
			containsAny(docLower, []string{"performance", "optimize", "fast", "efficient", "cache"}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "CSN-PERF-" + hashID(pair.Repo+pair.FuncName),
				Domain:      "performance",
				Name:        "Performance Pattern",
				Description: "CodeSearchNet: " + pair.Docstring,
				Condition: intelligence.Condition{
					Operator: "AND",
					Children: []intelligence.Condition{
						{Operator: "CONTAINS", Field: "code", Value: "for "},
						{Operator: "CONTAINS", Field: "code", Value: "query"},
					},
				},
				Action: intelligence.Action{
					Type:     "flag",
					Severity: "warning",
					Message:  "Performance: avoid N+1, batch load",
					Params:   map[string]interface{}{"source": "CodeSearchNet", "repo": pair.Repo},
				},
				Priority:   80,
				Confidence: 0.8,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// ERROR HANDLING: code returns error + docstring mentions error handling
		if strings.Contains(codeLower, "error") &&
			containsAny(docLower, []string{"error", "fail", "exception", "handle"}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "CSN-ERR-" + hashID(pair.Repo+pair.FuncName),
				Domain:      "code_quality",
				Name:        "Error Handling Pattern",
				Description: "CodeSearchNet: " + pair.Docstring,
				Condition: intelligence.Condition{
					Operator: "AND",
					Children: []intelligence.Condition{
						{Operator: "CONTAINS", Field: "code", Value: "error"},
						{Operator: "CONTAINS", Field: "code", Value: "return"},
					},
				},
				Action: intelligence.Action{
					Type:     "suggest",
					Severity: "info",
					Message:  "Error handling: check and propagate errors",
					Params:   map[string]interface{}{"source": "CodeSearchNet", "repo": pair.Repo},
				},
				Priority:   60,
				Confidence: 0.7,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	return rules
}

// ============================================================
// HELPERS
// ============================================================

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".ts":
		return "javascript"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func hashID(s string) string {
	h := 0
	for _, c := range s {
		h = (h*31 + int(c)) & 0x7fffffff
	}
	return strings.ToUpper(toHex(h))
}

func toHex(n int) string {
	const digits = "0123456789ABCDEF"
	if n == 0 {
		return "0"
	}
	var result string
	for n > 0 {
		result = string(digits[n%16]) + result
		n /= 16
	}
	return result
}