package skilleval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EvalDirName is the directory (relative to .cosca/evals/) that holds the
// skill benchmark results and their immutable history.
const EvalDirName = "skills"

// HistoryEntry is a single, immutable row of a skill's evaluation history.
// History is append-only: rows are never edited or removed. `Parent` points at
// the version this entry was derived from (the immediately preceding appended
// version), `IsCurrentBest` marks the single row that is the current best run.
type HistoryEntry struct {
	Version       string  `json:"version"`
	Parent        string  `json:"parent,omitempty"`
	Date          string  `json:"date"`
	PassRate      float64 `json:"pass_rate"`
	Note          string  `json:"note,omitempty"`
	IsCurrentBest bool    `json:"is_current_best"`
}

// evalSkillsDir returns the `.cosca/evals/skills/` directory below the project
// root. Ownership of the directory is granted before use (files are written
// 0600), so a directory that must exist is created with owner-only access.
func evalSkillsDir(root string) string {
	return filepath.Join(root, ".cosca", "evals", EvalDirName)
}

// SaveBenchmark persists a SkillBenchmark to `<name>.benchmark.json` inside
// `.cosca/evals/skills/` with 0600 permissions, creating the directory tree as
// needed. The write is deterministic: the file is a single snapshot per skill
// name (overwritten by each run), so LoadLatestBenchmark is an O(1) read.
func SaveBenchmark(root string, sk *SkillBenchmark) error {
	if sk == nil {
		return fmt.Errorf("skilleval: cannot save a nil benchmark")
	}
	name := sk.Skill
	if name == "" {
		name = "unknown"
	}

	dir := evalSkillsDir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("skilleval: create eval dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(sk, "", "  ")
	if err != nil {
		return fmt.Errorf("skilleval: marshal benchmark for %q: %w", name, err)
	}

	path := filepath.Join(dir, sanitizeName(name)+".benchmark.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("skilleval: write benchmark %q: %w", name, err)
	}
	return nil
}

// LoadLatestBenchmark loads the most recently persisted benchmark for a skill
// from `.cosca/evals/skills/<name>.benchmark.json`. It returns an error only
// when the file is absent or unreadable — the CLI surfaces that as "no
// benchmark yet for this skill".
func LoadLatestBenchmark(root, name string) (*SkillBenchmark, error) {
	path := filepath.Join(evalSkillsDir(root), sanitizeName(name)+".benchmark.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skilleval: read benchmark %q: %w", name, err)
	}
	var sk SkillBenchmark
	if err := json.Unmarshal(data, &sk); err != nil {
		return nil, fmt.Errorf("skilleval: parse benchmark %q: %w", name, err)
	}
	if sk.Skill == "" {
		sk.Skill = name
	}
	return &sk, nil
}

// AppendHistory appends one immutable HistoryEntry to `<name>.history.json`.
//
// Semantics:
//   - Append-only: the existing rows are never edited, only a new row is added.
//   - `parent`: derived automatically as the Version of the last appended row
//     ("" for the first entry), so the chain is self-describing.
//   - `is_current_best`: when true, every previously-marked best row is flipped
//     to false, guaranteeing at most one row claims the best run. When false,
//     the previous best is left untouched.
//   - Permissions are 0600 and the directory tree is created as needed.
//
// Returning the new row lets the caller confirm what was actually persisted.
func AppendHistory(root, name string, version string, passRate float64, note string, isCurrentBest bool) (*HistoryEntry, error) {
	dir := evalSkillsDir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("skilleval: create eval dir %s: %w", dir, err)
	}

	entries, err := loadHistoryDir(dir, name)
	if err != nil {
		return nil, err
	}

	parent := ""
	if len(entries) > 0 {
		parent = entries[len(entries)-1].Version
	}

	entry := HistoryEntry{
		Version:       version,
		Parent:        parent,
		Date:          time.Now().UTC().Format(time.RFC3339),
		PassRate:      passRate,
		Note:          note,
		IsCurrentBest: isCurrentBest,
	}

	// At most one row may claim the current-best run. If the new entry is the
	// best, demote the previous best before appending.
	if isCurrentBest {
		for i := range entries {
			entries[i].IsCurrentBest = false
		}
	}

	entries = append(entries, entry)
	if err := writeHistoryDir(dir, name, entries); err != nil {
		return nil, err
	}
	return &entry, nil
}

// LoadHistory loads the full append-only history for a skill. A missing file
// yields an empty slice (nil error), which is convenient for callers that
// append on first use.
func LoadHistory(root, name string) ([]HistoryEntry, error) {
	return loadHistoryDir(evalSkillsDir(root), name)
}

// loadHistoryDir reads and parses `<name>.history.json` inside dir. A missing
// file is treated as empty — never an error — so appending to a fresh skill
// just creates the file.
func loadHistoryDir(dir, name string) ([]HistoryEntry, error) {
	var entries []HistoryEntry
	path := filepath.Join(dir, sanitizeName(name)+".history.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, fmt.Errorf("skilleval: read history %q: %w", name, err)
	}
	if len(data) == 0 {
		return entries, nil
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("skilleval: parse history %q: %w", name, err)
	}
	return entries, nil
}

// writeHistoryDir persists the history rows to `<name>.history.json` (0600).
func writeHistoryDir(dir, name string, entries []HistoryEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("skilleval: marshal history %q: %w", name, err)
	}
	path := filepath.Join(dir, sanitizeName(name)+".history.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("skilleval: write history %q: %w", name, err)
	}
	return nil
}

// sanitizeName keeps only filename-safe characters in a skill name so a name
// can never escape the eval directory. It mirrors the conservative approach of
// internal/evals without importing it (the rule is a single trivial mapping).
func sanitizeName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "skill"
	}
	return string(out)
}
