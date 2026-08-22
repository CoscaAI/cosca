//
// Unit tests for the internal/framework package.

package framework

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file (and its parent directories) with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// =============================================================================
// ValidateCrossReferences
// =============================================================================

func TestValidateCrossReferences(t *testing.T) {
	root := t.TempDir()

	// docs/a.md has a valid relative link, a broken link and external links.
	writeFile(t, filepath.Join(root, "docs", "a.md"), `# A

[valid](b.md)
[broken](missing.md)
[external](https://example.com/x.md)
[anchor](#section)
[mail](mailto:test@example.com)
`)
	// docs/b.md links back — cross-directory + anchor stripped.
	writeFile(t, filepath.Join(root, "docs", "b.md"), `# B

[back](a.md)
[up](../readme.md#intro)
`)
	writeFile(t, filepath.Join(root, "readme.md"), `# Readme

[go to docs](docs/a.md)
`)

	issues, err := ValidateCrossReferences(root)
	if err != nil {
		t.Fatalf("ValidateCrossReferences returned error: %v", err)
	}

	// Exactly one broken link: missing.md.
	var broken []Issue
	for _, iss := range issues {
		if strings.Contains(iss.Message, "missing.md") {
			broken = append(broken, iss)
		}
	}
	if len(broken) != 1 {
		t.Errorf("expected 1 issue for missing.md, got %d: %+v", len(broken), issues)
	}
	if len(broken) > 0 {
		if broken[0].Severity != SeverityError {
			t.Errorf("broken link severity = %q, want %q", broken[0].Severity, SeverityError)
		}
		if broken[0].File != "docs/a.md" {
			t.Errorf("broken link file = %q, want %q", broken[0].File, "docs/a.md")
		}
	}

	// Valid links, external links, anchors and mailto must NOT be flagged.
	for _, iss := range issues {
		for _, ok := range []string{"example.com", "mailto:", "#section"} {
			if strings.Contains(iss.Message, ok) {
				t.Errorf("unexpected issue for external/anchor link: %+v", iss)
			}
		}
		if strings.Contains(iss.Message, `"b.md"`) || strings.Contains(iss.Message, `"a.md"`) {
			t.Errorf("unexpected issue for valid link: %+v", iss)
		}
	}
}

// =============================================================================
// ValidateConventions
// =============================================================================

const validDepartmentSkill = `> **Version**: 1.2.3 | **Status**: active

# ALPHA DEPARTMENT

## PURPOSE
Purpose paragraph.

## SCOPE
Scope paragraph.

## OUT OF SCOPE
Out of scope paragraph.

## RESPONSIBILITIES
- Responsibility one.

## DEPENDENCIES
| Depends On | Why |
|-----------|-----|
| CTO | Direction |

## INPUTS
| Input | From | Format |
|-------|------|--------|
| Spec | CTO | md |

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|
| API | Frontend | json |

## CONSTRAINTS
- Constraint one.

## QUALITY CRITERIA
- [ ] Criterion one.

## ESCALATION
| Issue | Escalate To |
|-------|-------------|
| X | CTO |

## FORBIDDEN ACTIONS
- Never do X.

## RELATED
- [CTO](../cto/SKILL.md)

## HISTORY
- 2026-07-01: created.
`

const invalidDepartmentSkill = `> **Status**: weird

# BETA DEPARTMENT

## PURPOSE
Purpose.

## RESPONSIBILITIES
- One.
`

func TestValidateConventions(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "departments", "alpha", "SKILL.md"), validDepartmentSkill)
	writeFile(t, filepath.Join(root, "departments", "beta", "SKILL.md"), invalidDepartmentSkill)
	// Target of the RELATED cross-reference in the valid skill.
	writeFile(t, filepath.Join(root, "departments", "cto", "SKILL.md"), validDepartmentSkill)
	// A compliant workflow.
	writeFile(t, filepath.Join(root, "workflows", "flow.md"), `> **Version**: 0.1.0

# WORKFLOW: flow

## OBJECTIVE
Do a thing.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| input | string | yes | The input |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| output | string | Result |

## PRECONDITIONS
1. Ready.

## POSTCONDITIONS
1. Done.

## STEPS
### Step 1: Do it
- **Chief**: Backend
- **Task**: Execute
- **Output**: Result

## SUCCESS CRITERIA
- [ ] Done.

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Err | Fix |

## HISTORY
- 2026-07-01: created.
`)

	issues, err := ValidateConventions(root)
	if err != nil {
		t.Fatalf("ValidateConventions returned error: %v", err)
	}

	// Group issues by file.
	byFile := make(map[string][]Issue)
	for _, iss := range issues {
		byFile[iss.File] = append(byFile[iss.File], iss)
	}

	// The valid department skill must produce no issues.
	if list := byFile["departments/alpha/SKILL.md"]; len(list) != 0 {
		t.Errorf("valid department skill flagged: %+v", list)
	}

	// The invalid department skill must report missing Version,
	// invalid Status and missing sections.
	beta := byFile["departments/beta/SKILL.md"]
	if len(beta) == 0 {
		t.Fatal("invalid department skill produced no issues")
	}
	foundMissingVersion := false
	foundBadStatus := false
	foundMissingHistory := false
	for _, iss := range beta {
		if strings.Contains(iss.Message, "Version") {
			foundMissingVersion = true
		}
		if strings.Contains(iss.Message, `status "weird"`) {
			foundBadStatus = true
		}
		if strings.Contains(iss.Message, `"HISTORY"`) {
			foundMissingHistory = true
		}
	}
	if !foundMissingVersion {
		t.Errorf("expected missing Version metadata issue, got: %+v", beta)
	}
	if !foundBadStatus {
		t.Errorf("expected invalid Status issue, got: %+v", beta)
	}
	if !foundMissingHistory {
		t.Errorf("expected missing HISTORY section issue, got: %+v", beta)
	}

	// The compliant workflow must produce no issues.
	if list := byFile["workflows/flow.md"]; len(list) != 0 {
		t.Errorf("valid workflow flagged: %+v", list)
	}
}

func TestValidateConventions_InvalidSemver(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "departments", "gamma", "SKILL.md"), `> **Version**: not-a-version | **Status**: active

## PURPOSE
Purpose.

## SCOPE
Scope.

## OUT OF SCOPE
Oos.

## RESPONSIBILITIES
- One.

## DEPENDENCIES
- None.

## INPUTS
- None.

## OUTPUTS
- None.

## CONSTRAINTS
- None.

## QUALITY CRITERIA
- [ ] One.

## ESCALATION
- None.

## FORBIDDEN ACTIONS
- None.

## RELATED
- None.

## HISTORY
- 2026-07-01: created.
`)

	issues, err := ValidateConventions(root)
	if err != nil {
		t.Fatalf("ValidateConventions returned error: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.File == "departments/gamma/SKILL.md" && strings.Contains(iss.Message, "semantic version") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid semantic version issue, got: %+v", issues)
	}
}

// =============================================================================
// DetectOrphans
// =============================================================================

func TestDetectOrphans(t *testing.T) {
	root := t.TempDir()

	// guide.md and reference.md reference each other by basename.
	writeFile(t, filepath.Join(root, "guide.md"), "# guide\n\nSee [reference](reference.md) for details.\n")
	writeFile(t, filepath.Join(root, "reference.md"), "# reference\n\nThis file is referenced from the guide.\n")
	// unlinked.md is never referenced.
	writeFile(t, filepath.Join(root, "unlinked.md"), "# unlinked\n\nNothing points here.\n")
	// INDEX.md is excluded from the candidate set.
	writeFile(t, filepath.Join(root, "INDEX.md"), "# Index\n")
	// memory/ is excluded from the candidate set but still a reference source.
	writeFile(t, filepath.Join(root, "memory", "note.md"), "# memory note\n")

	orphans, err := DetectOrphans(root)
	if err != nil {
		t.Fatalf("DetectOrphans returned error: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %v", len(orphans), orphans)
	}
	if orphans[0] != "unlinked.md" {
		t.Errorf("orphan = %q, want %q", orphans[0], "unlinked.md")
	}
}

func TestDetectOrphans_ExcludesIndexAndMemory(t *testing.T) {
	root := t.TempDir()
	// INDEX.md and memory/ files reference each other but must not appear
	// as orphans (and must not mask real orphans).
	writeFile(t, filepath.Join(root, "INDEX.md"), "# Index\n")
	writeFile(t, filepath.Join(root, "memory", "one.md"), "# one\n")
	writeFile(t, filepath.Join(root, "orphan.md"), "# orphan\n")

	orphans, err := DetectOrphans(root)
	if err != nil {
		t.Fatalf("DetectOrphans returned error: %v", err)
	}
	if len(orphans) != 1 || orphans[0] != "orphan.md" {
		t.Errorf("orphans = %v, want [orphan.md]", orphans)
	}
}

// =============================================================================
// Report
// =============================================================================

func TestReport(t *testing.T) {
	root := t.TempDir()

	// Departments, engines, workflows, skills, templates, agents, memory, ADRs.
	writeFile(t, filepath.Join(root, "departments", "alpha", "SKILL.md"), "# alpha SKILL\n")
	writeFile(t, filepath.Join(root, "engines", "omega", "SKILL.md"), "# omega SKILL\n")
	writeFile(t, filepath.Join(root, "workflows", "flow.md"), "# flow\n\nSee the guide at guide.md and the catalog at skills/SKILLS_CATALOG.md\n")
	writeFile(t, filepath.Join(root, "skills", "one.md"), "# one\n\nTEMPLATE files live in templates\n")
	writeFile(t, filepath.Join(root, "skills", "SKILLS_CATALOG.md"), "# SKILLS_CATALOG index\n")
	writeFile(t, filepath.Join(root, "templates", "erp", "TEMPLATE.md"), "# erp TEMPLATE\n")
	writeFile(t, filepath.Join(root, "agents", "cosca-alpha", "PROMPT.md"), "# PROMPT\n")
	writeFile(t, filepath.Join(root, "agents", "cosca-alpha", "INDEX.md"), "# index\n\nContains PROMPT.md\n")
	writeFile(t, filepath.Join(root, "memory", "note.md"), "# note one\n")
	writeFile(t, filepath.Join(root, "memory", "architecture", "adr", "adr-001.md"), "# adr-001\n")

	// Docs with one valid link and one broken link.
	writeFile(t, filepath.Join(root, "guide.md"), "# guide\n\nSee [reference](docs/reference.md) and flow.md\n\nBroken: [missing](docs/nope.md)\n")
	writeFile(t, filepath.Join(root, "docs", "reference.md"), "# reference\n")
	// An unreferenced file.
	writeFile(t, filepath.Join(root, "docs", "unlinked.md"), "# unlinked\n")

	report, err := Report(root)
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}

	if report.Root != root {
		t.Errorf("Root = %q, want %q", report.Root, root)
	}
	if report.TotalMarkdownFiles != 13 {
		t.Errorf("TotalMarkdownFiles = %d, want 13", report.TotalMarkdownFiles)
	}
	if report.Departments != 1 {
		t.Errorf("Departments = %d, want 1", report.Departments)
	}
	if report.Engines != 1 {
		t.Errorf("Engines = %d, want 1", report.Engines)
	}
	if report.Workflows != 1 {
		t.Errorf("Workflows = %d, want 1", report.Workflows)
	}
	if report.Skills != 1 {
		t.Errorf("Skills = %d, want 1 (SKILLS_CATALOG.md excluded)", report.Skills)
	}
	if report.Templates != 1 {
		t.Errorf("Templates = %d, want 1", report.Templates)
	}
	if report.Agents != 1 {
		t.Errorf("Agents = %d, want 1", report.Agents)
	}
	if report.MemoryRecords != 2 {
		t.Errorf("MemoryRecords = %d, want 2", report.MemoryRecords)
	}
	if report.ADRs != 1 {
		t.Errorf("ADRs = %d, want 1", report.ADRs)
	}
	if report.BrokenLinks != 1 {
		t.Errorf("BrokenLinks = %d, want 1", report.BrokenLinks)
	}
	if report.OrphanCount != 1 {
		t.Errorf("OrphanCount = %d, want 1", report.OrphanCount)
	}
}
