// Package storage provides JSONL persistence for intelligence rules.
// JSONL (JSON Lines) is chosen for high-speed sequential reads,
// versionability, and zero schema overhead.
package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// JSONL STORAGE
// ============================================================

// RuleFile is the JSONL file format for rules.
// Each line is a JSON object with the rule plus metadata.
type RuleFile struct {
	Version   int       `json:"version"`
	Source    string    `json:"source"`
	SavedAt   time.Time `json:"saved_at"`
	RuleCount int       `json:"rule_count"`
	Rule      *intelligence.Rule `json:"rule"`
}

// Storage persists rules to JSONL files.
type Storage struct {
	Dir string
}

// NewStorage creates a new storage.
func NewStorage(dir string) *Storage {
	return &Storage{Dir: dir}
}

// DefaultDir returns the default rules directory.
func DefaultDir() string {
	return filepath.Join("dsms-data", "rules")
}

// ============================================================
// WRITE
// ============================================================

// SaveRules writes rules to a JSONL file (append-friendly).
func (s *Storage) SaveRules(source string, rules []*intelligence.Rule) (string, error) {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return "", err
	}

	// Filename: rules-{source}-{timestamp}.jsonl
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("rules-%s-%s.jsonl", sanitize(source), ts)
	path := filepath.Join(s.Dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	encoder := json.NewEncoder(w)

	count := 0
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		entry := RuleFile{
			Version:   1,
			Source:    source,
			SavedAt:   time.Now(),
			RuleCount: 1,
			Rule:      rule,
		}
		if err := encoder.Encode(entry); err != nil {
			return "", err
		}
		count++
	}

	if err := w.Flush(); err != nil {
		return "", err
	}

	return path, nil
}

// ============================================================
// READ
// ============================================================

// LoadRules reads all rules from all JSONL files in the directory.
func (s *Storage) LoadRules() ([]*intelligence.Rule, error) {
	files, err := filepath.Glob(filepath.Join(s.Dir, "rules-*.jsonl"))
	if err != nil {
		return nil, err
	}

	var all []*intelligence.Rule
	for _, file := range files {
		rules, err := LoadFile(file)
		if err != nil {
			return nil, err
		}
		all = append(all, rules...)
	}

	return all, nil
}

// LoadFile reads rules from a single JSONL file.
func LoadFile(path string) ([]*intelligence.Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rules []*intelligence.Rule
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry RuleFile
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip corrupt lines
		}

		if entry.Rule != nil {
			rules = append(rules, entry.Rule)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

// ============================================================
// HELPERS
// ============================================================

func sanitize(s string) string {
	var sb strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('-')
		}
	}
	return sb.String()
}