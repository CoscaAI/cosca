// Package expander scans E:\cosca-tmp and E:\cosca-tmp-miner for
// knowledge to train the intelligence engine with real-world content.
package expander

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// EXPANDER
// ============================================================

// Expander scans directories for knowledge content.
type Expander struct {
	Roots []string
}

// NewExpander creates an expander with default roots.
func NewExpander() *Expander {
	return &Expander{
		Roots: []string{
			`E:\cosca-tmp`,
			`E:\cosca-tmp-miner`,
		},
	}
}

// KnowledgeDoc represents a knowledge document found on disk.
type KnowledgeDoc struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
}

// ScanResult represents the result of scanning.
type ScanResult struct {
	RootsScanned    int              `json:"roots_scanned"`
	FilesFound      int              `json:"files_found"`
	DocsFound       int              `json:"docs_found"`
	TotalBytes      int64            `json:"total_bytes"`
	ByExtension     map[string]int   `json:"by_extension"`
	SampleDocs      []*KnowledgeDoc  `json:"sample_docs"`
	Duration        time.Duration    `json:"duration"`
}

// Scan scans the roots for knowledge files.
func (e *Expander) Scan(ctx context.Context, maxFiles int) (*ScanResult, error) {
	start := time.Now()

	result := &ScanResult{
		ByExtension: make(map[string]int),
		SampleDocs:  make([]*KnowledgeDoc, 0),
	}

	// Knowledge extensions (text-based)
	knowledgeExts := map[string]bool{
		".md": true, ".mdx": true, ".go": true, ".ts": true, ".tsx": true,
		".js": true, ".py": true, ".rs": true, ".lua": true, ".luau": true,
		".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".cpp": true, ".h": true, ".rb": true, ".sh": true, ".sql": true,
	}

	for _, root := range e.Roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}

		result.RootsScanned++

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip heavy/vendored dirs
				base := strings.ToLower(info.Name())
				if base == ".git" || base == "node_modules" || base == "vendor" ||
					base == "dist" || base == "build" || base == ".venv" ||
					base == "__pycache__" || base == "target" || base == ".next" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !knowledgeExts[ext] {
				return nil
			}

			// Respect max files
			if maxFiles > 0 && result.FilesFound >= maxFiles {
				return filepath.SkipAll
			}

			result.FilesFound++
			result.TotalBytes += info.Size()
			result.ByExtension[ext]++

			// Read small files (up to 200KB) as knowledge docs
			if info.Size() > 0 && info.Size() < 200*1024 && len(result.SampleDocs) < 200 {
				content, err := os.ReadFile(path)
				if err == nil {
					result.DocsFound++
					result.SampleDocs = append(result.SampleDocs, &KnowledgeDoc{
						Path:     path,
						Content:  string(content),
						Language: strings.TrimPrefix(ext, "."),
						Size:     info.Size(),
					})
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ============================================================
// RULE EXTRACTION FROM DOCS
// ============================================================

// ExtractRules converts knowledge docs into rules.
func ExtractRules(docs []*KnowledgeDoc) []*intelligence.Rule {
	var rules []*intelligence.Rule
	now := time.Now()

	for _, doc := range docs {
		content := strings.ToLower(doc.Content)

		// SECURITY patterns
		if containsAny(content, []string{
			"sql injection", "injection", "cwe-89", "parameterized query",
			"prepared statement", "sanitize input", "input validation",
			"never trust", "server authority", "client never",
		}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "EXT-SEC-" + hashID(doc.Path),
				Domain:      "security",
				Name:        "Security Pattern",
				Description: "Extracted from: " + doc.Path,
				Condition: intelligence.Condition{
					Operator: "HAS_PATTERN",
					Field:    "patterns",
					Value:    "sql_injection",
				},
				Action: intelligence.Action{
					Type:     "flag",
					Severity: "critical",
					Message:  "Security pattern from knowledge: validate inputs, use parameterized queries",
					Params:   map[string]interface{}{"source": doc.Path},
				},
				Priority:   90,
				Confidence: 0.85,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// PERFORMANCE patterns
		if containsAny(content, []string{
			"n+1", "query loop", "batch load", "performance",
			"lazy loading", "eager loading", "rate limit", "quota",
		}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "EXT-PERF-" + hashID(doc.Path),
				Domain:      "performance",
				Name:        "Performance Pattern",
				Description: "Extracted from: " + doc.Path,
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
					Message:  "Performance pattern: avoid N+1 queries, batch load",
					Params:   map[string]interface{}{"source": doc.Path},
				},
				Priority:   80,
				Confidence: 0.8,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// ARCHITECTURE patterns
		if containsAny(content, []string{
			"architecture", "design pattern", "coupling", "cohesion",
			"dependency injection", "layered architecture", "server authority",
			"client never", "state management",
		}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "EXT-ARCH-" + hashID(doc.Path),
				Domain:      "architecture",
				Name:        "Architecture Pattern",
				Description: "Extracted from: " + doc.Path,
				Condition: intelligence.Condition{
					Operator: "GT",
					Field:    "metrics.coupling",
					Value:    0.7,
				},
				Action: intelligence.Action{
					Type:     "suggest",
					Severity: "warning",
					Message:  "Architecture pattern: keep coupling low, server as authority",
					Params:   map[string]interface{}{"source": doc.Path},
				},
				Priority:   70,
				Confidence: 0.75,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// CODE QUALITY patterns
		if containsAny(content, []string{
			"god object", "too large", "refactor", "split file",
			"single responsibility", "separation of concerns",
			"code review", "best practice", "clean code",
		}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "EXT-CODE-" + hashID(doc.Path),
				Domain:      "code_quality",
				Name:        "Code Quality Pattern",
				Description: "Extracted from: " + doc.Path,
				Condition: intelligence.Condition{
					Operator: "GT",
					Field:    "metrics.lines",
					Value:    500.0,
				},
				Action: intelligence.Action{
					Type:     "suggest",
					Severity: "warning",
					Message:  "Code quality: split large files, single responsibility",
					Params:   map[string]interface{}{"source": doc.Path},
				},
				Priority:   65,
				Confidence: 0.7,
				Version:    1,
				Enabled:    true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		// TESTING patterns
		if containsAny(content, []string{
			"test coverage", "unit test", "integration test", "tdd",
			"test strategy", "test pyramid", "testing",
		}) {
			rules = append(rules, &intelligence.Rule{
				ID:          "EXT-TEST-" + hashID(doc.Path),
				Domain:      "testing",
				Name:        "Testing Pattern",
				Description: "Extracted from: " + doc.Path,
				Condition: intelligence.Condition{
					Operator: "NOT",
					Children: []intelligence.Condition{
						{Operator: "CONTAINS", Field: "file_path", Value: "_test"},
					},
				},
				Action: intelligence.Action{
					Type:     "suggest",
					Severity: "info",
					Message:  "Testing: add test coverage",
					Params:   map[string]interface{}{"source": doc.Path},
				},
				Priority:   50,
				Confidence: 0.65,
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

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hashID creates a simple hash-based ID from a path.
func hashID(path string) string {
	h := 0
	for _, c := range path {
		h = (h*31 + int(c)) & 0x7fffffff
	}
	return strings.ToUpper(strings.ReplaceAll(toHex(h), "0x", ""))
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