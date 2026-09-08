// Package trainer connects the Intelligence Engine to the REAL knowledge base.
// It reads the 118K entries from knowledge.db and distills them into
// deterministic rules that the engine can use.
package trainer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// TRAINER
// ============================================================

// Trainer reads knowledge and trains rules.
type Trainer struct {
	dbPath string
}

// NewTrainer creates a new trainer.
func NewTrainer(dbPath string) *Trainer {
	return &Trainer{dbPath: dbPath}
}

// KnowledgeEntry represents a knowledge base entry.
type KnowledgeEntry struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Domain     string  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

// TrainingResult represents the result of training.
type TrainingResult struct {
	EntriesRead    int            `json:"entries_read"`
	RulesExtracted int            `json:"rules_extracted"`
	RulesByDomain  map[string]int `json:"rules_by_domain"`
	Duration       time.Duration  `json:"duration"`
	SampleRules    []*intelligence.Rule `json:"sample_rules"`
	AllRules       []*intelligence.Rule `json:"all_rules"`
}

// LoadKnowledge reads entries from the knowledge database.
// It reads from knowledge_entries (curated) AND chunks (the bulk).
func (t *Trainer) LoadKnowledge(ctx context.Context, limit int) ([]KnowledgeEntry, error) {
	// Open read-only
	db, err := sql.Open("sqlite", "file:"+t.dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open knowledge db: %w", err)
	}
	defer db.Close()

	var entries []KnowledgeEntry

	// 1. Read curated knowledge_entries (high confidence)
	query := `SELECT id, content, category, sub_category, confidence FROM knowledge_entries`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query knowledge_entries: %w", err)
	}

	for rows.Next() {
		var e KnowledgeEntry
		var subCategory string
		if err := rows.Scan(&e.ID, &e.Content, &e.Category, &subCategory, &e.Confidence); err != nil {
			continue
		}
		e.Domain = e.Category
		if e.Domain == "" {
			e.Domain = subCategory
		}
		entries = append(entries, e)
	}
	rows.Close()

	// 2. Read chunks (the bulk of knowledge - 113K)
	chunkLimit := limit
	if chunkLimit > 0 {
		chunkLimit -= len(entries)
		if chunkLimit < 0 {
			chunkLimit = 0
		}
	}

	if chunkLimit > 0 {
		query = `SELECT c.id, c.content, c.section_type, COALESCE(d.doc_type, 'general'), 0.5
			FROM chunks c
			LEFT JOIN documents d ON d.id = c.document_id
			WHERE c.content != ''`
		query += fmt.Sprintf(" LIMIT %d", chunkLimit)

		chunkRows, err := db.QueryContext(ctx, query)
		if err != nil {
			return entries, nil // chunks are optional - return curated entries
		}

		for chunkRows.Next() {
			var e KnowledgeEntry
			var sectionType string
			if err := chunkRows.Scan(&e.ID, &e.Content, &sectionType, &e.Domain, &e.Confidence); err != nil {
				continue
			}
			e.Category = sectionType
			entries = append(entries, e)
		}
		chunkRows.Close()
	}

	return entries, nil
}

// Train extracts rules from knowledge entries.
func (t *Trainer) Train(ctx context.Context, entries []KnowledgeEntry) *TrainingResult {
	start := time.Now()

result := &TrainingResult{
		RulesByDomain: make(map[string]int),
		SampleRules:   make([]*intelligence.Rule, 0),
		AllRules:      make([]*intelligence.Rule, 0),
	}
	result.EntriesRead = len(entries)

	// Extract rules from each entry
	for _, entry := range entries {
		rule := extractRule(entry)
		if rule != nil {
			result.RulesExtracted++
			result.RulesByDomain[rule.Domain]++
			result.AllRules = append(result.AllRules, rule)
			if len(result.SampleRules) < 10 {
				result.SampleRules = append(result.SampleRules, rule)
			}
		}
	}

	result.Duration = time.Since(start)
	return result
}

// ============================================================
// RULE EXTRACTION
// ============================================================

// extractRule converts a knowledge entry into a deterministic rule.
func extractRule(entry KnowledgeEntry) *intelligence.Rule {
	content := strings.ToLower(entry.Content)
	now := time.Now()

	// SECURITY patterns
	if containsAny(content, []string{
		"sql injection", "injection", "cwe-89", "parameterized query",
		"prepared statement", "sanitize input", "input validation",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-SEC-%s", entry.ID),
			Domain:      "security",
			Name:        "SQL Injection Prevention",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{Operator: "CONTAINS", Field: "code", Value: "query"},
					{Operator: "CONTAINS", Field: "code", Value: "+"},
					{Operator: "CONTAINS", Field: "code", Value: "select"},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "SQL injection: use parameterized queries",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   90,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	if containsAny(content, []string{
		"hardcoded secret", "api key", "secret key", "credential",
		"cwe-798", "password in code",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-SEC-%s", entry.ID),
			Domain:      "security",
			Name:        "Hardcoded Credential",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{
						Operator: "OR",
						Children: []intelligence.Condition{
							{Operator: "CONTAINS", Field: "code", Value: "sk-"},
							{Operator: "CONTAINS", Field: "code", Value: "AKIA"},
							{Operator: "CONTAINS", Field: "code", Value: "eyJ"},
							{Operator: "CONTAINS", Field: "code", Value: "password ="},
						},
					},
					{Operator: "CONTAINS", Field: "code", Value: "="},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "Hardcoded credential detected",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   95,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// PERFORMANCE patterns
	if containsAny(content, []string{
		"n+1", "query loop", "batch load", "performance",
		"lazy loading", "eager loading",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-PERF-%s", entry.ID),
			Domain:      "performance",
			Name:        "N+1 Query Prevention",
			Description: entry.Content,
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
				Message:  "N+1 query pattern - batch load",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   80,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// CODE QUALITY patterns
	if containsAny(content, []string{
		"god object", "too large", "refactor", "split file",
		"single responsibility", "separation of concerns",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-CODE-%s", entry.ID),
			Domain:      "code_quality",
			Name:        "Code Organization",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.lines",
				Value:    500.0,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "File too large - consider splitting",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   70,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	if containsAny(content, []string{
		"todo", "fixme", "hack", "technical debt",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-CODE-%s", entry.ID),
			Domain:      "code_quality",
			Name:        "Technical Debt",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "OR",
				Children: []intelligence.Condition{
					{Operator: "CONTAINS", Field: "code", Value: "TODO"},
					{Operator: "CONTAINS", Field: "code", Value: "FIXME"},
					{Operator: "CONTAINS", Field: "code", Value: "HACK"},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  "Technical debt marker found",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   60,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// ARCHITECTURE patterns
	if containsAny(content, []string{
		"architecture", "design pattern", "coupling", "cohesion",
		"dependency injection", "layered architecture",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-ARCH-%s", entry.ID),
			Domain:      "architecture",
			Name:        "Architecture Best Practice",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.coupling",
				Value:    0.7,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "High coupling - consider decoupling",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   65,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// TESTING patterns
	if containsAny(content, []string{
		"test coverage", "unit test", "integration test", "tdd",
		"test strategy", "test pyramid",
	}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("TRAIN-TEST-%s", entry.ID),
			Domain:      "testing",
			Name:        "Test Coverage",
			Description: entry.Content,
			Condition: intelligence.Condition{
				Operator: "NOT",
				Children: []intelligence.Condition{
					{Operator: "CONTAINS", Field: "file_path", Value: "_test"},
				},
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "info",
				Message:  "File has no tests",
				Params:   map[string]interface{}{"source": "knowledge", "entry_id": entry.ID},
			},
			Priority:   50,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	return nil
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

