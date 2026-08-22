package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	// Valid frontmatter.
	content := `---
name: test-skill
description: A test skill
version: "2.1.0"
category: coding
---
# SKILL: test-skill
Body here
`
	fm := parseFrontmatter(content)
	if fm.Name != "test-skill" || fm.Description != "A test skill" || fm.Version != "2.1.0" || fm.Category != "coding" {
		t.Fatalf("frontmatter: %+v", fm)
	}

	// No frontmatter → zero struct.
	if fm := parseFrontmatter("# just a title\n"); fm.Name != "" {
		t.Fatalf("no-frontmatter: %+v", fm)
	}
	// Unterminated frontmatter → zero.
	if fm := parseFrontmatter("---\nname: x\n"); fm.Name != "" {
		t.Fatalf("unterminated: %+v", fm)
	}
	// Invalid YAML → zero.
	if fm := parseFrontmatter("---\n: : : bad\n---\n"); fm.Name != "" {
		t.Fatalf("invalid yaml: %+v", fm)
	}
}

func TestExtractTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"# WORKFLOW: code-review", "code-review"},
		{"# SKILL: api-design", "api-design"},
		{"# UNIT TESTING SKILL", "UNIT TESTING"},
		{"# AI CHIEF", "AI CHIEF"},
		{"# Backend AGENT", "Backend"},
		{"no heading here", ""},
	}
	for _, tc := range cases {
		if got := extractTitle(tc.in); got != tc.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractDescription(t *testing.T) {
	content := "# X\n\n## Description\nThis is the desc.\nSecond line.\n\n## Other\nignored"
	got := extractDescription(content)
	if !strings.Contains(got, "This is the desc.") || !strings.Contains(got, "Second line.") {
		t.Fatalf("desc = %q", got)
	}
	// No description section.
	if got := extractDescription("# X\n\n## Other\n"); got != "" {
		t.Fatalf("no desc: %q", got)
	}
}

func TestExtractVersionStatusCategory(t *testing.T) {
	content := "> **Version**: 1.2.3 | x\n> **Status**: active | y\n> **Category**: backend | z\n"
	if got := extractVersion(content); got != "1.2.3" {
		t.Fatalf("version = %q", got)
	}
	if got := extractStatus(content); got != "active" {
		t.Fatalf("status = %q", got)
	}
	if got := extractCategory(content); got != "backend" {
		t.Fatalf("category = %q", got)
	}
	// Status defaults to active.
	if got := extractStatus("no status here"); got != "active" {
		t.Fatalf("status default = %q", got)
	}
}

func TestExtractPurposeRoleReportsTo(t *testing.T) {
	content := "# BACKEND CHIEF\n\n## PURPOSE\nHandle all backend work.\n\n## RESPONSIBILITIES\n- x\n"
	if got := extractPurpose(content); !strings.Contains(got, "Handle all backend work.") {
		t.Fatalf("purpose = %q", got)
	}
	if got := extractRole(content); got != "BACKEND CHIEF" {
		t.Fatalf("role = %q", got)
	}
	content2 := "- **Reports To**: cosca-cto\n"
	if got := extractReportsTo(content2); got != "cosca-cto" {
		t.Fatalf("reports to = %q", got)
	}
}

func TestExtractTemplateType(t *testing.T) {
	if got := extractTemplateType("# TEMPLATE: saas\n"); got != "saas" {
		t.Fatalf("template type = %q", got)
	}
	if got := extractTemplateType("# NoType\n"); got != "" {
		t.Fatalf("no type = %q", got)
	}
}

func TestExtractInstructions(t *testing.T) {
	content := "# X\nBody"
	if got := extractInstructions(content); got != content {
		t.Fatalf("instructions = %q", got)
	}
}

func TestParseToolsFromContent(t *testing.T) {
	content := `## Inputs
| Name | Type | Required | Description |
|------|------|----------|-------------|
| read_file | string | yes | Read files |
| read_file | string | yes | dup |
| write_file | string | no | Write files |
## Next
`
	tools := parseToolsFromContent(content)
	if len(tools) != 2 {
		t.Fatalf("tools = %v", tools)
	}
	if tools[0].Name != "read_file" || tools[1].Name != "write_file" {
		t.Fatalf("tool names: %v", tools)
	}
}

func TestParseWorkflowSteps(t *testing.T) {
	content := `# WORKFLOW: review

## STEPS
### Step 1: Analyze
- **Chief**: cosca-architecture
- **Description**: analyze the code

### Step 2: Verify
- **Chief**: cosca-testing

## NEXT SECTION
`
	steps := parseWorkflowSteps(content)
	if len(steps) != 2 {
		t.Fatalf("steps = %v", steps)
	}
	if steps[0].Name != "1: Analyze" || steps[0].Agent != "cosca-architecture" || steps[0].Description != "analyze the code" {
		t.Fatalf("step0: %+v", steps[0])
	}
	if steps[1].Agent != "cosca-testing" {
		t.Fatalf("step1: %+v", steps[1])
	}
}

func TestParseWorkflowIO(t *testing.T) {
	content := `## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| prompt | string | yes | input prompt |
| context | map | no | extra |

## OUTPUTS
| Name | Type |
|------|------|
| result | string |
`
	inputs := parseWorkflowIO(content, "INPUTS")
	if len(inputs) != 2 {
		t.Fatalf("inputs = %v", inputs)
	}
	if inputs[0].Name != "prompt" || inputs[0].Type != "string" || !inputs[0].Required {
		t.Fatalf("input0: %+v", inputs[0])
	}
	if inputs[1].Required {
		t.Fatalf("input1 required: %+v", inputs[1])
	}
	outputs := parseWorkflowIO(content, "OUTPUTS")
	if len(outputs) != 1 || outputs[0].Name != "result" {
		t.Fatalf("outputs = %v", outputs)
	}
}

func TestParseCapabilities(t *testing.T) {
	content := "- Orchestrate agents\n- **Metadata**: skip\n- Route tasks\n"
	caps := parseCapabilities(content)
	if len(caps) != 2 {
		t.Fatalf("caps = %v", caps)
	}
	if caps[0].Name != "Orchestrate agents" {
		t.Fatalf("cap0: %+v", caps[0])
	}
}

func TestParseAgentTools(t *testing.T) {
	content := "- Backend API endpoints → Backend Chief\n- Frontend UI -> Frontend Chief\n- Backend API endpoints → dup\n"
	tools := parseAgentTools(content)
	if len(tools) != 2 {
		t.Fatalf("tools = %v", tools)
	}
	if tools[0].Name != "Backend API endpoints" || tools[0].Purpose != "Backend Chief" {
		t.Fatalf("tool0: %+v", tools[0])
	}
}

func TestParseResponsibilities(t *testing.T) {
	content := "## RESPONSIBILITIES\n- Handle routing\n1. Manage memory\n- **skip me**\n## NEXT\n"
	resp := parseResponsibilities(content)
	// The parser is intentionally naive: it does not filter bold metadata
	// lines, so "**skip me**" is included.
	if len(resp) != 3 {
		t.Fatalf("resp = %v", resp)
	}
	if resp[0] != "Handle routing" || resp[1] != "Manage memory" {
		t.Fatalf("resp: %v", resp)
	}
}

func TestParsePromptParameters(t *testing.T) {
	content := `## Inputs
| Name | Type | Default | Description |
|------|------|---------|-------------|
| goal | string | none | the goal |
| limit | int | 10 | max |
`
	params := parsePromptParameters(content)
	if len(params) != 2 {
		t.Fatalf("params = %v", params)
	}
	if params[0].Name != "goal" || params[0].Type != "string" {
		t.Fatalf("param0: %+v", params[0])
	}
}

func TestUtilities(t *testing.T) {
	if firstNonEmpty("", "  ", "x", "y") != "x" {
		t.Fatal("firstNonEmpty")
	}
	if firstNonEmpty() != "" {
		t.Fatal("firstNonEmpty empty")
	}
	if nameFromPath("/a/b/cool-skill.md") != "cool-skill" {
		t.Fatal("nameFromPath")
	}
	if inferCategoryFromPath("/root/skills/api/thing.md") != "api" {
		t.Fatal("inferCategory")
	}
	if inferCategoryFromPath("/root/skills/thing.md") != "general" {
		t.Fatal("inferCategory general")
	}
	if !isHeaderRow("Name", []string{"Name"}) || !isHeaderRow("INPUT", nil) {
		t.Fatal("isHeaderRow")
	}
	if isHeaderRow("read_file", nil) {
		t.Fatal("isHeaderRow false")
	}
	if !isSeparatorRow([]string{"---", ":---:"}) {
		t.Fatal("isSeparatorRow")
	}
	if isSeparatorRow([]string{"---", "x"}) {
		t.Fatal("isSeparatorRow false")
	}
	if !isAllDashes("--:-") || isAllDashes("") || isAllDashes("-x") {
		t.Fatal("isAllDashes")
	}
	parts := splitPipe("| a | b | c |")
	if len(parts) != 3 || parts[0] != "a" || parts[2] != "c" {
		t.Fatalf("splitPipe = %v", parts)
	}
}

// ── AOSFramework Discover* with temp framework dirs ───────────────────

func writeFrameworkFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverSkills(t *testing.T) {
	root := t.TempDir()
	writeFrameworkFile(t, root, "skills/coding/api-design.md",
		"---\nname: api-design\ndescription: Design APIs\nversion: \"1.0.0\"\ncategory: coding\n---\n# SKILL: api-design\n\n## Inputs\n| Name | Type | Description |\n|------|------|-------------|\n| read_file | string | reads |\n")
	writeFrameworkFile(t, root, "skills/SKILLS_CATALOG.md", "# catalog — must be skipped")

	fw := &AOSFramework{Root: root}
	skills, err := fw.DiscoverSkills()
	if err != nil {
		t.Fatalf("DiscoverSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("skills = %d, want 1 (catalog must be skipped)", len(skills))
	}
	if skills[0].Name != "api-design" || skills[0].Category != "coding" {
		t.Fatalf("skill: %+v", skills[0])
	}
	if len(skills[0].Tools) != 1 {
		t.Fatalf("tools: %+v", skills[0].Tools)
	}
}

func TestDiscoverSkillsMissingDir(t *testing.T) {
	fw := &AOSFramework{Root: t.TempDir()}
	skills, err := fw.DiscoverSkills()
	if err != nil || len(skills) != 0 {
		t.Fatalf("missing skills dir: %v, %d", err, len(skills))
	}
}

func TestDiscoverWorkflows(t *testing.T) {
	root := t.TempDir()
	writeFrameworkFile(t, root, "workflows/review.md",
		"# WORKFLOW: review\n\n## STEPS\n### Step 1: Analyze\n- **Chief**: cosca-architecture\n")
	fw := &AOSFramework{Root: root}
	wfs, err := fw.DiscoverWorkflows()
	if err != nil || len(wfs) != 1 {
		t.Fatalf("workflows: %v, %d", err, len(wfs))
	}
	if wfs[0].Name != "review" || len(wfs[0].StepList) != 1 {
		t.Fatalf("wf: %+v", wfs[0])
	}
}

func TestDiscoverAgents(t *testing.T) {
	root := t.TempDir()
	writeFrameworkFile(t, root, "departments/backend/SKILL.md",
		"---\nname: cosca-backend\nrole: Backend Chief\n---\n# BACKEND CHIEF\n\n## RESPONSIBILITIES\n- Build APIs\n")
	fw := &AOSFramework{Root: root}
	agents, err := fw.DiscoverAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("agents: %v, %d", err, len(agents))
	}
	if agents[0].Name != "cosca-backend" || agents[0].Role != "Backend Chief" {
		t.Fatalf("agent: %+v", agents[0])
	}
	if len(agents[0].Responsibilities) != 1 {
		t.Fatalf("responsibilities: %+v", agents[0].Responsibilities)
	}
}

func TestDiscoverTemplatesAndPrompts(t *testing.T) {
	root := t.TempDir()
	writeFrameworkFile(t, root, "templates/saas/project.md",
		"# TEMPLATE: saas\n\n# PROJECT\n")
	writeFrameworkFile(t, root, "prompts/review.md",
		"---\nname: review-prompt\ncategory: qa\n---\n# PROMPT: review-prompt\n")

	fw := &AOSFramework{Root: root}
	tmpls, err := fw.DiscoverTemplates()
	if err != nil || len(tmpls) != 1 {
		t.Fatalf("templates: %v, %d", err, len(tmpls))
	}
	if tmpls[0].Type != "saas" {
		t.Fatalf("template type: %+v", tmpls[0])
	}

	prompts, err := fw.DiscoverPrompts()
	if err != nil || len(prompts) != 1 {
		t.Fatalf("prompts: %v, %d", err, len(prompts))
	}
	if prompts[0].Name != "review-prompt" || prompts[0].Category != "qa" {
		t.Fatalf("prompt: %+v", prompts[0])
	}
}

func TestNewAOSFrameworkEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COSCA_HOME", dir)
	fw, err := NewAOSFramework()
	if err != nil {
		t.Fatalf("NewAOSFramework: %v", err)
	}
	if fw.Root != dir {
		t.Fatalf("root = %q, want %q", fw.Root, dir)
	}
}
