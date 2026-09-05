package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// semverRe matches a semantic version: X.Y.Z with optional pre-release
// and/or build metadata, allowing an optional leading "v".
var semverRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// validStatuses are the allowed Status metadata values per CONVENTIONS.md.
var validStatuses = map[string]bool{
	"draft":      true,
	"active":     true,
	"deprecated": true,
}

// departmentRequiredSections are the mandatory sections for
// departments/*/SKILL.md files (mirrors validate-conventions.sh).
var departmentRequiredSections = []string{
	"PURPOSE",
	"SCOPE",
	"OUT OF SCOPE",
	"RESPONSIBILITIES",
	"DEPENDENCIES",
	"INPUTS",
	"OUTPUTS",
	"CONSTRAINTS",
	"QUALITY CRITERIA",
	"ESCALATION",
	"FORBIDDEN ACTIONS",
	"RELATED",
	"HISTORY",
}

// workflowRequiredSections are the mandatory sections for
// workflows/*.md files (mirrors validate-conventions.sh).
var workflowRequiredSections = []string{
	"OBJECTIVE",
	"INPUTS",
	"OUTPUTS",
	"PRECONDITIONS",
	"POSTCONDITIONS",
	"STEPS",
	"SUCCESS CRITERIA",
	"ERROR HANDLING",
	"HISTORY",
}

// ValidateConventions checks that framework files comply with
// CONVENTIONS.md: metadata blocks (Version/Status), valid semantic
// versions, mandatory sections (including HISTORY) and valid internal
// cross-references. Department skills and workflows are validated with
// the same section lists used by the legacy validate-conventions.sh.
func ValidateConventions(root string) ([]Issue, error) {
	var issues []Issue

	// Department skills.
	depSkills, _ := filepath.Glob(filepath.Join(root, "departments", "*", "SKILL.md"))
	for _, skill := range depSkills {
		issues = append(issues, validateDepartmentSkill(root, skill)...)
	}

	// Workflows.
	workflows, _ := filepath.Glob(filepath.Join(root, "workflows", "*.md"))
	for _, wf := range workflows {
		issues = append(issues, validateWorkflow(root, wf)...)
	}

	// Cross-references.
	crossRefIssues, err := ValidateCrossReferences(root)
	if err != nil {
		return nil, err
	}
	issues = append(issues, crossRefIssues...)

	return issues, nil
}

// validateDepartmentSkill validates a departments/*/SKILL.md file against
// the CONVENTIONS.md department contract.
func validateDepartmentSkill(root, path string) []Issue {
	var issues []Issue
	rel := relativePath(root, path)

	content, err := os.ReadFile(path)
	if err != nil {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  fmt.Sprintf("unable to read file: %v", err),
		})
		return issues
	}
	text := string(content)

	// Metadata block.
	version, hasVersion := extractMetadata(text, "Version")
	status, hasStatus := extractMetadata(text, "Status")

	if !hasVersion {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  "missing Version metadata",
		})
	} else if !semverRe.MatchString(version) {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  fmt.Sprintf("invalid semantic version %q (expected X.Y.Z)", version),
		})
	}

	if !hasStatus {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  "missing Status metadata",
		})
	} else if !validStatuses[strings.ToLower(strings.TrimSpace(status))] {
		issues = append(issues, Issue{
			Severity: SeverityWarning,
			File:     rel,
			Message:  fmt.Sprintf("status %q is not one of draft/active/deprecated", status),
		})
	}

	// Mandatory sections.
	for _, section := range departmentRequiredSections {
		if !hasSection(text, section) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				File:     rel,
				Message:  fmt.Sprintf("missing required section %q", section),
			})
		}
	}

	return issues
}

// validateWorkflow validates a workflows/*.md file against the
// CONVENTIONS.md workflow contract.
func validateWorkflow(root, path string) []Issue {
	var issues []Issue
	rel := relativePath(root, path)

	content, err := os.ReadFile(path)
	if err != nil {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  fmt.Sprintf("unable to read file: %v", err),
		})
		return issues
	}
	text := string(content)

	// Version metadata is optional for workflows, but must be valid semver
	// when present.
	if version, ok := extractMetadata(text, "Version"); ok && !semverRe.MatchString(version) {
		issues = append(issues, Issue{
			Severity: SeverityError,
			File:     rel,
			Message:  fmt.Sprintf("invalid semantic version %q (expected X.Y.Z)", version),
		})
	}

	for _, section := range workflowRequiredSections {
		if !hasSection(text, section) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				File:     rel,
				Message:  fmt.Sprintf("missing required section %q", section),
			})
		}
	}

	return issues
}

// extractMetadata extracts a metadata value from a file. It first looks for
// the inline metadata block format (`> **Version**: 1.0.0 | ...`), then
// falls back to YAML frontmatter (`version: 1.0.0`).
func extractMetadata(text, key string) (string, bool) {
	inlineRe := regexp.MustCompile(`\*\*` + regexp.QuoteMeta(key) + `\*\*:\s*([^\n|]+)`)
	if m := inlineRe.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1]), true
	}

	fmRe := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(strings.ToLower(key)) + `\s*[:=]\s*["']?([^\s"']+)`)
	if m := fmRe.FindStringSubmatch(text); len(m) > 1 {
		return strings.Trim(strings.TrimSpace(m[1]), `"'`), true
	}
	return "", false
}

// hasSection reports whether a section header appears in the file. Like the
// legacy bash `grep -q`, the section name may appear anywhere (heading,
// table cell, or body text).
func hasSection(text, section string) bool {
	return strings.Contains(text, section)
}
