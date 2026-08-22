package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// CuratedFailure is the sanitized, shareable form of a failure record.
// Only the lesson is exposed — never the internal details (Task, Failed
// Approach, Root Cause, Consequence). Cross-agent learning (P5) happens
// through this sanitized view, not through raw memory access.
type CuratedFailure struct {
	ID               string   `json:"id"`
	Date             string   `json:"date"`
	Name             string   `json:"name"`
	Agent            string   `json:"agent"`
	Domain           string   `json:"domain"`
	Lesson           string   `json:"lesson"`
	AvoidancePattern string   `json:"avoidance_pattern,omitempty"`
	Tags             []string `json:"tags,omitempty"`
}

// CurateFailuresDir walks dir for directories that contain a failures.md and
// returns the sanitized lessons of every failure found. Attribution comes
// from the directory name (the agent), and the domain is derived via
// DepartmentOf.
func CurateFailuresDir(dir string) ([]CuratedFailure, error) {
	var out []CuratedFailure
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || info.Name() != "failures.md" {
			return nil
		}
		agent := filepath.Base(filepath.Dir(path))
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, ParseCuratedFailures(string(content), agent)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ParseCuratedFailures parses a failures.md document (the format verified in
// internal/embed/cosca/memory/agent/*/failures.md) into its sanitized form.
// Malformed entries are skipped gracefully; a parse never fails wholesale.
func ParseCuratedFailures(content, agent string) []CuratedFailure {
	var out []CuratedFailure
	for _, entry := range splitFailureEntries(content) {
		if f := parseFailureEntry(entry, agent); f != nil {
			out = append(out, *f)
		}
	}
	return out
}

// splitFailureEntries splits a document into per-entry line slices, each
// starting at a `### {ID} | {date} | {Name}` header.
func splitFailureEntries(content string) [][]string {
	lines := strings.Split(content, "\n")
	var entries [][]string
	var current []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "### ") {
			if len(current) > 0 {
				entries = append(entries, current)
			}
			current = []string{line}
		} else if len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		entries = append(entries, current)
	}
	return entries
}

// parseFailureEntry parses a single failure entry. It returns nil for
// malformed entries (unparseable header or no lesson — nothing safe to share).
func parseFailureEntry(lines []string, agent string) *CuratedFailure {
	if len(lines) == 0 {
		return nil
	}

	header := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "###"))
	parts := strings.Split(header, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil // malformed header: need at least an ID and a date
	}

	f := &CuratedFailure{
		ID:     parts[0],
		Date:   parts[1],
		Agent:  agent,
		Domain: DepartmentOf(agent),
	}
	if len(parts) > 2 {
		f.Name = strings.Join(parts[2:], "|")
	}

	var prose []string
	for i := 1; i < len(lines); i++ {
		field, rest, ok, clean := failureFieldRow(lines[i])
		if !ok {
			s := strings.TrimSpace(lines[i])
			if s != "" && !strings.HasPrefix(s, "### ") && s != "---" && !strings.HasPrefix(s, ">") {
				prose = append(prose, s)
			}
			continue
		}
		value := rest
		if !clean {
			// Multi-line value: accumulate until the next field row, the next
			// entry, or a horizontal rule ends the entry.
			var b strings.Builder
			b.WriteString(rest)
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if _, _, okNext, _ := failureFieldRow(lines[i+1]); okNext {
					break
				}
				if strings.HasPrefix(next, "### ") || next == "---" {
					break
				}
				if next != "" {
					b.WriteString(" ")
					b.WriteString(next)
				}
				i++
			}
			value = b.String()
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "|"))

		switch strings.ToLower(field) {
		case "lesson":
			f.Lesson = value
		case "avoidance pattern":
			f.AvoidancePattern = value
		case "tags":
			f.Tags = parseFailureTags(value)
		}
	}

	if f.Lesson == "" && len(prose) > 0 {
		f.Lesson = strings.Join(prose, " ")
	}
	if f.Lesson == "" {
		return nil // no lesson → nothing safe to share
	}
	return f
}

// failureFieldRow parses a markdown table row of the form
// `| **Field** | value... |`. It returns the field name, the raw remainder
// after the field (including any trailing pipe when the row is clean), and
// whether the row terminates cleanly with a closing pipe.
func failureFieldRow(line string) (field, rest string, ok bool, clean bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "|") {
		return "", "", false, false
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, "|"))
	if !strings.HasPrefix(s, "**") {
		return "", "", false, false
	}
	end := strings.Index(s[2:], "**")
	if end < 0 {
		return "", "", false, false
	}
	field = s[2 : 2+end]
	rest = strings.TrimSpace(s[2+end+2:])
	if !strings.HasPrefix(rest, "|") {
		return "", "", false, false
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "|"))
	clean = strings.HasSuffix(strings.TrimSpace(line), "|")
	return field, rest, true, clean
}

// parseFailureTags extracts #tag tokens from a Tags value, tolerating stray
// separators such as commas or semicolons.
func parseFailureTags(value string) []string {
	var tags []string
	for _, tok := range strings.Fields(value) {
		if !strings.HasPrefix(tok, "#") {
			continue
		}
		tok = strings.Trim(strings.TrimPrefix(tok, "#"), " ,;")
		if tok != "" {
			tags = append(tags, tok)
		}
	}
	return tags
}
