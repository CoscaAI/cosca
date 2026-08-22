package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// writeSkill writes a SKILL.md file inside root/departments/dept/.
func writeSkill(t *testing.T, root, dept, content string) string {
	t.Helper()
	dir := filepath.Join(root, "departments", dept)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	writeTestFile(t, path, content)
	return path
}

// sampleSkill returns a realistic SKILL.md fixture exercising every parser
// feature: version/status quote block, reports-to, em-dash heading, purpose,
// responsibilities, and dependency/input/output tables.
func sampleSkill() string {
	return `> **Version**: 2.3.0 | **Status**: active | **Last Updated**: 2026-01-01

# DATA ENGINEER — Data Pipeline Specialist

## METADATA
- **Version**: 2.3.0
- **Status**: active
- **Owner**: Data Engineer
- **Reports To**: Analytics Chief

## PURPOSE
Builds data pipelines.
Handles ETL workloads.

## RESPONSIBILITIES
1. Design pipelines
2. Write ETL jobs
3. Monitor data quality

## DEPENDENCIES
| Depends On | Why |
|-----------|-----|
| Analytics Chief | Data requirements |

## INPUTS
| Input | From | Format |
|-------|------|--------|
| Raw data | Upstream | CSV |

## OUTPUTS
| Output | To | Format |
|--------|-----|--------|
| Clean data | Warehouse | Parquet |
`
}

// TestNewManagerEmptyCoscaDir verifies NewManager with an empty directory
// still loads the embedded Cosca framework agents.
func TestNewManagerEmptyCoscaDir(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	list := m.List()
	if len(list) == 0 {
		t.Fatal("expected embedded agents to be loaded")
	}
	// Well-known embedded agents must be present and resolvable
	// case-insensitively.
	for _, name := range []string{"CEO", "CTO", "AUTOMATION CHIEF", "testing chief"} {
		a, err := m.Get(name)
		if err != nil {
			t.Errorf("Get(%q): expected embedded agent, got error: %v", name, err)
			continue
		}
		if a.Name == "" {
			t.Errorf("Get(%q): agent name should not be empty", name)
		}
		if a.Department == "" {
			t.Errorf("Get(%q): expected department to be set", name)
		}
	}
}

// TestNewManagerLocalDirOverridesEmbedded verifies that an agent loaded from
// the local Cosca directory replaces the embedded agent with the same name.
func TestNewManagerLocalDirOverridesEmbedded(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "ceo", "# CEO\n\n## PURPOSE\nLocal override description.\n")

	m := NewManager(root)

	a, err := m.Get("CEO")
	if err != nil {
		t.Fatalf("expected CEO to be present, got error: %v", err)
	}
	if !strings.Contains(a.Description, "Local override") {
		t.Errorf("expected local description to override embedded, got %q", a.Description)
	}
}

// TestNewManagerLocalDirAddsNewAgents verifies local-only agents are added
// alongside the embedded ones.
func TestNewManagerLocalDirAddsNewAgents(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "custom", "# CUSTOM LEAD — Custom Lead Role\n\n## PURPOSE\nHandles custom work.\n")

	m := NewManager(root)

	a, err := m.Get("CUSTOM LEAD")
	if err != nil {
		t.Fatalf("expected local agent to be registered, got error: %v", err)
	}
	if a.Department != "custom" {
		t.Errorf("expected department 'custom', got %q", a.Department)
	}
	if a.Role != "Custom Lead Role" {
		t.Errorf("expected role 'Custom Lead Role', got %q", a.Role)
	}
	// Embedded agents must still be present.
	if _, err := m.Get("CEO"); err != nil {
		t.Errorf("expected embedded CEO to remain, got error: %v", err)
	}
}

// TestNewManagerMissingCoscaDir verifies a nonexistent directory is ignored.
func TestNewManagerMissingCoscaDir(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "does-not-exist"))
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.List()) == 0 {
		t.Fatal("expected embedded agents to still load")
	}
}

// TestNewManagerCoscaDirWithoutDepartments verifies a Cosca directory without
// a departments/ subdirectory is handled gracefully.
func TestNewManagerCoscaDirWithoutDepartments(t *testing.T) {
	m := NewManager(t.TempDir())
	if len(m.List()) == 0 {
		t.Fatal("expected embedded agents to still load")
	}
}

// TestLoadFromDir verifies loadFromDir registers one agent per department
// directory with a parseable SKILL.md, skipping files, directories without
// SKILL.md, and invalid SKILL.md files.
func TestLoadFromDir(t *testing.T) {
	root := t.TempDir()

	// Valid department with a full SKILL.md.
	writeSkill(t, root, "alpha", sampleSkill())

	// Department without a SKILL.md — must be skipped.
	if err := os.MkdirAll(filepath.Join(root, "departments", "beta"), 0o755); err != nil {
		t.Fatalf("failed to mkdir beta: %v", err)
	}

	// Non-directory entry — must be skipped.
	writeTestFile(t, filepath.Join(root, "departments", "gamma"), "not a directory")

	// Department whose SKILL.md has no name — must be skipped.
	writeSkill(t, root, "delta", "no heading and no name here")

	m := &Manager{agents: make(map[string]*Agent)}
	m.loadFromDir(root)

	if a, err := m.Get("DATA ENGINEER"); err != nil {
		t.Errorf("expected alpha agent to load, got error: %v", err)
	} else if a.Department != "alpha" {
		t.Errorf("expected department 'alpha', got %q", a.Department)
	}

	if _, err := m.Get("beta"); err == nil {
		t.Error("expected no agent for department without SKILL.md")
	}
	if _, err := m.Get("delta"); err == nil {
		t.Error("expected no agent for SKILL.md without a name")
	}
}

// TestLoadFromDirMissingRoot verifies loadFromDir tolerates a nonexistent
// departments directory.
func TestLoadFromDirMissingRoot(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.loadFromDir(filepath.Join(t.TempDir(), "nope"))
	if len(m.agents) != 0 {
		t.Errorf("expected no agents, got %d", len(m.agents))
	}
}

// TestLoadFromDirDepartmentsIsFile verifies loadFromDir tolerates a
// departments entry that is a regular file rather than a directory.
func TestLoadFromDirDepartmentsIsFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "departments"), "not a directory")

	m := &Manager{agents: make(map[string]*Agent)}
	m.loadFromDir(root)
	if len(m.agents) != 0 {
		t.Errorf("expected no agents, got %d", len(m.agents))
	}
}

// TestListEmpty verifies List on an empty manager returns an empty slice.
func TestListEmpty(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	if got := m.List(); len(got) != 0 {
		t.Errorf("expected empty list, got %d entries", len(got))
	}
}

// TestList verifies List returns every registered agent.
func TestList(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "A", Department: "a"})
	m.Register(&Agent{Name: "B", Department: "b"})
	m.Register(&Agent{Name: "C", Department: "c"})

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(list))
	}
	seen := make(map[string]bool)
	for _, a := range list {
		seen[a.Name] = true
	}
	for _, want := range []string{"A", "B", "C"} {
		if !seen[want] {
			t.Errorf("expected agent %q in list, missing", want)
		}
	}
}

// TestListReturnsCopies verifies List returns value copies so callers cannot
// mutate the manager's internal agents.
func TestListReturnsCopies(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "ORIGINAL"})

	list := m.List()
	list[0].Name = "MUTATED"

	a, err := m.Get("ORIGINAL")
	if err != nil {
		t.Fatalf("expected original agent intact, got error: %v", err)
	}
	if a.Name != "ORIGINAL" {
		t.Errorf("expected stored name 'ORIGINAL', got %q", a.Name)
	}
}

// TestGet verifies Get returns exact and case-insensitive matches and an
// error for unknown names.
func TestGet(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "AUTOMATION CHIEF", Role: "Automation Chief"})

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"exact match", "AUTOMATION CHIEF", false},
		{"case-insensitive", "automation chief", false},
		{"mixed case", "Automation Chief", false},
		{"unknown", "UNKNOWN", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := m.Get(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Get(%q): expected error, got agent %v", tt.query, a)
				}
				if !strings.Contains(err.Error(), tt.query) {
					t.Errorf("Get(%q): error should mention query, got %q", tt.query, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Get(%q): unexpected error: %v", tt.query, err)
			}
			if a.Name != "AUTOMATION CHIEF" {
				t.Errorf("Get(%q): expected AUTOMATION CHIEF, got %q", tt.query, a.Name)
			}
		})
	}
}

// TestSearch verifies Search matches name, role, department, and description
// case-insensitively and normalizes hyphens/underscores in the query.
func TestSearch(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "AUTOMATION CHIEF", Role: "Automation Chief", Department: "automation", Description: "Builds automation tooling"})
	m.Register(&Agent{Name: "SEMANTIC MEMORY CHIEF", Role: "Semantic Memory Chief", Department: "semantic-memory", Description: "Vector embeddings"})
	m.Register(&Agent{Name: "BACKEND CHIEF", Role: "Backend Chief", Department: "backend", Description: "Leads backend development"})

	tests := []struct {
		name    string
		query   string
		wantNum int
	}{
		{"by name exact", "AUTOMATION CHIEF", 1},
		{"by name lowercase", "automation chief", 1},
		{"by role", "semantic", 1},
		{"by department", "backend", 1},
		{"by description", "vector", 1},
		{"normalizes hyphen", "semantic-memory", 1},
		{"normalizes underscore", "semantic_memory", 1},
		{"common term matches multiple", "chief", 3},
		{"no match", "quantum", 0},
		{"empty query returns all", "", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Search(tt.query)
			if err != nil {
				t.Fatalf("Search(%q): unexpected error: %v", tt.query, err)
			}
			if len(got) != tt.wantNum {
				t.Errorf("Search(%q): expected %d results, got %d", tt.query, tt.wantNum, len(got))
			}
		})
	}
}

// TestSearchNormalizedNames verifies searching with hyphen/underscore
// separators finds agents whose stored names use spaces.
func TestSearchNormalizedNames(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "DATA ENGINEER", Role: "Data Engineer", Department: "data"})

	for _, query := range []string{"data-engineer", "data_engineer"} {
		got, err := m.Search(query)
		if err != nil {
			t.Fatalf("Search(%q): unexpected error: %v", query, err)
		}
		if len(got) != 1 {
			t.Errorf("Search(%q): expected 1 result, got %d", query, len(got))
		}
		if got[0].Name != "DATA ENGINEER" {
			t.Errorf("Search(%q): expected 'DATA ENGINEER', got %q", query, got[0].Name)
		}
	}
}

// TestRegister verifies Register adds a new agent and overwrites an existing
// one with the same name.
func TestRegister(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}

	m.Register(&Agent{Name: "FIRST", Role: "First Role"})
	if len(m.agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(m.agents))
	}
	if a, err := m.Get("FIRST"); err != nil || a.Role != "First Role" {
		t.Errorf("expected FIRST with role 'First Role', got %v, err=%v", a, err)
	}

	// Overwrite by same name.
	m.Register(&Agent{Name: "FIRST", Role: "Updated Role"})
	if len(m.agents) != 1 {
		t.Fatalf("expected 1 agent after overwrite, got %d", len(m.agents))
	}
	if a, err := m.Get("FIRST"); err != nil || a.Role != "Updated Role" {
		t.Errorf("expected role 'Updated Role', got %q, err=%v", a.Role, err)
	}
}

// TestAdd verifies Add stores a copy of the value, so later mutations of the
// caller's value do not affect the stored agent.
func TestAddStoresCopy(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}

	orig := Agent{Name: "COPY ME", Role: "Original Role"}
	m.Add(orig)
	orig.Role = "Mutated Role"

	a, err := m.Get("COPY ME")
	if err != nil {
		t.Fatalf("expected agent after Add, got error: %v", err)
	}
	if a.Role != "Original Role" {
		t.Errorf("expected stored role 'Original Role', got %q (Add should copy)", a.Role)
	}
}

// TestParseSkillContentFull verifies full parsing of a synthetic SKILL.md.
func TestParseSkillContentFull(t *testing.T) {
	agent, err := parseSkillContent(sampleSkill())
	if err != nil {
		t.Fatalf("parseSkillContent failed: %v", err)
	}

	if agent.Name != "DATA ENGINEER" {
		t.Errorf("expected name 'DATA ENGINEER', got %q", agent.Name)
	}
	if agent.Role != "Data Pipeline Specialist" {
		t.Errorf("expected role 'Data Pipeline Specialist', got %q", agent.Role)
	}
	if agent.Version != "2.3.0" {
		t.Errorf("expected version '2.3.0', got %q", agent.Version)
	}
	if agent.Status != "active" {
		t.Errorf("expected status 'active', got %q", agent.Status)
	}
	if agent.ReportsTo != "Analytics Chief" {
		t.Errorf("expected reports_to 'Analytics Chief', got %q", agent.ReportsTo)
	}
	if agent.Description != "Builds data pipelines. Handles ETL workloads." {
		t.Errorf("unexpected description %q", agent.Description)
	}
	if len(agent.Responsibilities) != 3 {
		t.Fatalf("expected 3 responsibilities, got %d", len(agent.Responsibilities))
	}
	if agent.Responsibilities[0] != "Design pipelines" {
		t.Errorf("expected first responsibility 'Design pipelines', got %q", agent.Responsibilities[0])
	}
	if len(agent.Dependencies) != 2 {
		t.Errorf("expected 2 dependency rows (header + data), got %d", len(agent.Dependencies))
	}
	if agent.Dependencies[1] != (TableRow{Key: "Analytics Chief", Value: "Data requirements"}) {
		t.Errorf("unexpected dependency row %+v", agent.Dependencies[1])
	}
	if len(agent.Inputs) != 2 {
		t.Errorf("expected 2 input rows, got %d", len(agent.Inputs))
	}
	if len(agent.Outputs) != 2 {
		t.Errorf("expected 2 output rows, got %d", len(agent.Outputs))
	}
	if agent.Outputs[1] != (TableRow{Key: "Clean data", Value: "Warehouse"}) {
		t.Errorf("unexpected output row %+v", agent.Outputs[1])
	}
}

// TestParseSkillContentHeadingVariants verifies the three heading formats:
// em-dash, hyphen, and bare name.
func TestParseSkillContentHeadingVariants(t *testing.T) {
	tests := []struct {
		name     string
		heading  string
		wantName string
		wantRole string
	}{
		{"em dash", "# ALPHA — The Role", "ALPHA", "The Role"},
		{"hyphen", "# BETA - The Role", "BETA", "The Role"},
		{"bare", "# GAMMA", "GAMMA", "GAMMA"},
		{"no heading", "just a paragraph", "no name", "no name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.heading + "\n\n## PURPOSE\nSome purpose.\n"
			agent, err := parseSkillContent(content)
			if tt.wantName == "no name" {
				if err == nil {
					t.Fatal("expected error for content without a name")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSkillContent failed: %v", err)
			}
			if agent.Name != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, agent.Name)
			}
			if agent.Role != tt.wantRole {
				t.Errorf("expected role %q, got %q", tt.wantRole, agent.Role)
			}
		})
	}
}

// TestParseSkillContentReportsToVariants verifies both list and blockquote
// forms of the Reports To field.
func TestParseSkillContentReportsToVariants(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"blockquote", "> **Reports To**: QA Chief", "QA Chief"},
		{"list item", "- **Reports To**: QA Chief", "QA Chief"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "# SOME AGENT\n\n" + tt.line + "\n"
			agent, err := parseSkillContent(content)
			if err != nil {
				t.Fatalf("parseSkillContent failed: %v", err)
			}
			if agent.ReportsTo != tt.want {
				t.Errorf("expected reports_to %q, got %q", tt.want, agent.ReportsTo)
			}
		})
	}
}

// TestParseSkillContentEmptySections verifies defaults are applied when
// optional sections are absent.
func TestParseSkillContentEmptySections(t *testing.T) {
	agent, err := parseSkillContent("# SOLO AGENT\n")
	if err != nil {
		t.Fatalf("parseSkillContent failed: %v", err)
	}
	if agent.Status != "active" {
		t.Errorf("expected default status 'active', got %q", agent.Status)
	}
	if agent.Version != "" {
		t.Errorf("expected empty version, got %q", agent.Version)
	}
	if len(agent.Responsibilities) != 0 {
		t.Errorf("expected empty responsibilities, got %d", len(agent.Responsibilities))
	}
	if len(agent.Dependencies) != 0 || len(agent.Inputs) != 0 || len(agent.Outputs) != 0 {
		t.Error("expected empty table sections")
	}
}

// TestParseSkillContentNoName verifies an error is returned when no name can
// be extracted.
func TestParseSkillContentNoName(t *testing.T) {
	if _, err := parseSkillContent("no heading anywhere\n## PURPOSE\nnothing\n"); err == nil {
		t.Fatal("expected error for content without a name")
	}
}

// TestParseSkillContentVersionLastWins verifies the version is captured from
// the quote block but not overwritten by a later plain occurrence.
func TestParseSkillContentVersion(t *testing.T) {
	content := "> **Version**: 9.9.9\n\n# AGENT X — Role Y\n\n- **Version**: 1.0.0\n"
	agent, err := parseSkillContent(content)
	if err != nil {
		t.Fatalf("parseSkillContent failed: %v", err)
	}
	if agent.Version != "9.9.9" {
		t.Errorf("expected version '9.9.9', got %q", agent.Version)
	}
}

// TestParseSkillFile verifies reading a SKILL.md from disk.
func TestParseSkillFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	writeTestFile(t, path, sampleSkill())

	agent, err := parseSkillFile(path)
	if err != nil {
		t.Fatalf("parseSkillFile failed: %v", err)
	}
	if agent.Name != "DATA ENGINEER" {
		t.Errorf("expected name 'DATA ENGINEER', got %q", agent.Name)
	}
}

// TestParseSkillFileMissing verifies an error for a nonexistent file.
func TestParseSkillFileMissing(t *testing.T) {
	if _, err := parseSkillFile(filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestExtractColonValue verifies value extraction after a marker, including
// pipe truncation and missing markers.
func TestExtractColonValue(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		marker string
		want   string
	}{
		{"plain", "**Version**: 1.2.3", "**Version**:", "1.2.3"},
		{"pipe truncation", "> **Version**: 1.2.3 | **Status**: active", "**Version**:", "1.2.3"},
		{"missing marker", "**Status**: active", "**Version**:", ""},
		{"whitespace padding", "**Reports To**:   CTO  ", "**Reports To**:", "CTO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractColonValue(tt.line, tt.marker); got != tt.want {
				t.Errorf("extractColonValue(%q, %q): expected %q, got %q", tt.line, tt.marker, tt.want, got)
			}
		})
	}
}

// TestExtractListItem verifies numbered list item extraction.
func TestExtractListItem(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"1. Do something", "Do something"},
		{"9. Ninth item", "Ninth item"},
		{"1.2.3", ""},
		{"1.", ""},
		{" 1. leading space", ""},
		{"## HEADING", ""},
		{"plain text", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := extractListItem(tt.line); got != tt.want {
				t.Errorf("extractListItem(%q): expected %q, got %q", tt.line, tt.want, got)
			}
		})
	}
}

// TestExtractTableRows verifies markdown table parsing, including header
// rows, separators, heading termination, and non-table lines.
func TestExtractTableRows(t *testing.T) {
	lines := []string{
		"| Key | Value |",
		"|---|-----|",
		"| A | B |",
		"| C | D |",
		"## NEXT SECTION",
		"| Z | should not appear |",
	}
	rows := extractTableRows(lines)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0] != (TableRow{Key: "Key", Value: "Value"}) {
		t.Errorf("unexpected first row %+v", rows[0])
	}
	if rows[1] != (TableRow{Key: "A", Value: "B"}) {
		t.Errorf("unexpected second row %+v", rows[1])
	}
	if rows[2] != (TableRow{Key: "C", Value: "D"}) {
		t.Errorf("unexpected third row %+v", rows[2])
	}
}

// TestExtractTableRowsSkipsNonTableContent verifies non-table lines are
// ignored while subsequent table rows are still collected.
func TestExtractTableRowsSkipsNonTableContent(t *testing.T) {
	lines := []string{
		"some prose line",
		"",
		"| A | B |",
		"| C | D |",
	}
	rows := extractTableRows(lines)
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d: %+v", len(rows), rows)
	}
}

// TestExtractTableRowsEmpty verifies empty input yields no rows.
func TestExtractTableRowsEmpty(t *testing.T) {
	if rows := extractTableRows(nil); len(rows) != 0 {
		t.Errorf("expected no rows, got %d", len(rows))
	}
}

// TestSplitTableRow verifies markdown row splitting into cells.
func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		row  string
		want []string
	}{
		{"| A | B |", []string{"A", "B"}},
		{"| A | B | C |", []string{"A", "B", "C"}},
		{"|   X   | Y |", []string{"X", "Y"}},
		{"| only one |", []string{"only one"}},
		{"|", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.row, func(t *testing.T) {
			got := splitTableRow(tt.row)
			if len(got) != len(tt.want) {
				t.Fatalf("splitTableRow(%q): expected %v, got %v", tt.row, tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitTableRow(%q): cell %d expected %q, got %q", tt.row, i, tt.want[i], got[i])
				}
			}
		})
	}
}

// TestContainsIgnoreCase verifies case-insensitive substring matching.
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"HELLO WORLD", "world", true},
		{"MiXeD CaSe", "ixed", true},
		{"abc", "d", false},
		{"abc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"/"+tt.substr, func(t *testing.T) {
			if got := containsIgnoreCase(tt.s, tt.substr); got != tt.want {
				t.Errorf("containsIgnoreCase(%q, %q): expected %v, got %v", tt.s, tt.substr, tt.want, got)
			}
		})
	}
}
