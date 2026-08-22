package parser

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/markdown"
)

func TestNewEntityParser(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	if ep == nil {
		t.Fatal("NewEntityParser returned nil")
	}
}

func TestEntityTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		entityType EntityType
		want       string
	}{
		{EntityAgent, "agent"},
		{EntitySkill, "skill"},
		{EntityPrompt, "prompt"},
		{EntityWorkflow, "workflow"},
		{EntityTemplate, "template"},
		{EntityProvider, "provider"},
		{EntityPlugin, "plugin"},
		{EntityADR, "adr"},
		{EntityCapability, "capability"},
		{EntityMemoryRecord, "memory_record"},
		{EntityPattern, "pattern"},
		{EntityPlaybook, "playbook"},
		{EntityRunbook, "runbook"},
		{EntityIncident, "incident"},
		{EntityBenchmark, "benchmark"},
		{EntityReferenceArch, "reference_architecture"},
		{EntityConfig, "config"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if string(tc.entityType) != tc.want {
				t.Errorf("EntityType = %q, want %q", tc.entityType, tc.want)
			}
		})
	}
}

func TestRelationshipTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		relType RelationshipType
		want    string
	}{
		{RelDependsOn, "depends_on"},
		{RelExtends, "extends"},
		{RelImplements, "implements"},
		{RelInvokes, "invokes"},
		{RelReferences, "references"},
		{RelDefines, "defines"},
		{RelContains, "contains"},
		{RelRelatedTo, "related_to"},
		{RelImports, "imports"},
		{RelReportsTo, "reports_to"},
		{RelTriggers, "triggers"},
		{RelDocuments, "documents"},
		{RelSupersedes, "supersedes"},
		{RelConsumes, "consumes"},
		{RelProduces, "produces"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if string(tc.relType) != tc.want {
				t.Errorf("RelationshipType = %q, want %q", tc.relType, tc.want)
			}
		})
	}
}

func TestEntityBasics(t *testing.T) {
	t.Parallel()

	e := Entity{
		ID:   "test-id",
		Type: EntityAgent,
		Name: "test-agent",
		Path: "/path/to/agent.md",
	}

	if e.ID != "test-id" {
		t.Errorf("ID = %q", e.ID)
	}
	if e.Type != EntityAgent {
		t.Errorf("Type = %v", e.Type)
	}
	if e.Name != "test-agent" {
		t.Errorf("Name = %q", e.Name)
	}
}

// ── detectEntityType ──────────────────────────────────────────────────────

func TestDetectEntityType(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	tests := []struct {
		name string
		path string
		want EntityType
	}{
		{"skill in skills dir", "/path/to/skills/test-skill.md", EntitySkill},
		{"workflow dir", "/path/to/workflows/deploy.yaml", EntityWorkflow},
		{"template dir", "/path/to/templates/react.md", EntityTemplate},
		{"provider dir", "/path/to/providers/openai.md", EntityProvider},
		{"plugin dir", "/path/to/plugins/my-plugin.md", EntityPlugin},
		{"adr by name", "/path/to/adr/001-decision.md", EntityADR},
		{"decision in path", "/decision/123-decision.md", EntityADR},
		{"capability dir", "/path/to/capabilities/auth.md", EntityCapability},
		{"config yaml", "/path/to/config.yaml", EntityConfig},
		{"config json", "/path/to/config.json", EntityConfig},
		{"default skill", "/path/to/some-file.md", EntitySkill},
		{"agent dna", "/path/to/agent_dna/chief.md", EntityAgent},
		{"dna in base", "/path/entity-dna.md", EntityAgent},
		{"pattern dir", "/path/to/patterns/singleton.md", EntityPattern},
		{"playbook", "/path/to/playbook-deploy.md", EntityPlaybook},
		{"runbook", "/path/to/runbook-failure.md", EntityRunbook},
		{"incident dir", "/path/to/incidents/001.md", EntityIncident},
		{"benchmark", "/path/to/benchmark-perf.md", EntityBenchmark},
		{"config toml", "/path/to/config.toml", EntityConfig},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ep.detectEntityType(tc.path, nil)
			if got != tc.want {
				t.Errorf("detectEntityType(%q) = %s, want %s", tc.path, got, tc.want)
			}
		})
	}
}

func TestDetectEntityType_FromFrontmatter(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Frontmatter: markdown.Frontmatter{
			Data: map[string]interface{}{
				"type": "agent",
			},
		},
	}

	got := ep.detectEntityType("some/file.md", doc)
	if got != EntityAgent {
		t.Errorf("detectEntityType with frontmatter type = %s, want agent", got)
	}
}

func TestDetectEntityType_DepartmentDir(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	// Department dir with enough headings for agent detection
	doc := &markdown.Document{
		Headings: make([]markdown.Heading, 25),
	}

	got := ep.detectEntityType("/department/engineering/test.md", doc)
	if got != EntityAgent {
		t.Errorf("detectEntityType for department dir = %s, want agent", got)
	}
}

func TestDetectEntityType_SkillMD(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	got := ep.detectEntityType("/path/skill.md", nil)
	if got != EntitySkill {
		t.Errorf("detectEntityType for skill.md = %s, want skill", got)
	}
}

// ── parseTableRow ──────────────────────────────────────────────────────────

func TestParseTableRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want []string
	}{
		{"| a | b | c |", []string{"a", "b", "c"}},
		{"| x |", []string{"x"}},
		{"", []string{""}},
		{"no pipes here", []string{"no pipes here"}},
		{"  | spaces  |  around  | ", []string{"", "spaces", "around", ""}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := parseTableRow(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d cells, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("cell[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ── extractFirstParagraph ──────────────────────────────────────────────────

func TestExtractFirstParagraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"after heading", "# Title\n\nHello world", "Hello world"},
		{"plain text", "Just text here", "Just text here"},
		{"empty", "", ""},
		{"only heading", "# Title", ""},
		{"skip blockquote", "> quote\nreal text", "real text"},
		{"skip code fence", "```\ncode\n```\nreal text", "code"},
		{"leading empty lines", "\n\n\nactual text", "actual text"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractFirstParagraph(tc.content)
			if got != tc.want {
				t.Errorf("extractFirstParagraph = %q, want %q", got, tc.want)
			}
		})
	}
}

// ── extractSection ──────────────────────────────────────────────────────────

func TestExtractSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		heading string
		want    string
	}{
		{
			"simple section",
			"## Section1\nsome content here\n\n## Section2\nother content",
			"## Section1",
			"some content here",
		},
		{
			"heading not found",
			"## Section1\ncontent",
			"## Missing",
			"",
		},
		{
			"last section",
			"## Section1\ncontent\n\n## Last\nfinal content",
			"## Last",
			"final content",
		},
		{
			"empty content after heading",
			"## Empty\n",
			"## Empty",
			"",
		},
		{
			"multiline content",
			"## Multi\nline one\nline two\n\n## Next",
			"## Multi",
			"line one\nline two",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractSection(tc.content, tc.heading)
			if got != tc.want {
				t.Errorf("extractSection = %q, want %q", got, tc.want)
			}
		})
	}
}

// ── extractListAfterHeading ─────────────────────────────────────────────────

func TestExtractListAfterHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		heading string
		want    []string
	}{
		{
			"unordered list with dash",
			"## Items\n- item one\n- item two\n\n## Next",
			"## Items",
			[]string{"item one", "item two"},
		},
		{
			"unordered list with star",
			"## Items\n* star one\n* star two",
			"## Items",
			[]string{"star one", "star two"},
		},
		{
			"ordered list",
			"## Steps\n1. first\n2. second\n3. third",
			"## Steps",
			[]string{"first", "second", "third"},
		},
		{
			"heading not found",
			"## Other\n- item",
			"## Missing",
			nil,
		},
		{
			"empty list section",
			"## Empty\n\n## Next",
			"## Empty",
			nil,
		},
		{
			"mixed list types",
			"## Mixed\n- dash item\n* star item\n1. ordered",
			"## Mixed",
			[]string{"dash item", "star item", "ordered"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractListAfterHeading(tc.content, tc.heading)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("item[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ── extractTableAfterHeading ────────────────────────────────────────────────

func TestExtractTableAfterHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		heading string
		wantLen int
		wantCol string
	}{
		{
			"simple table",
			"## Data\n| name | age |\n|------|-----|\n| alice | 30 |\n| bob | 25 |\n\n## Next",
			"## Data",
			2,
			"name",
		},
		{
			"heading not found",
			"## Other\n| a | b |\n|---|---|\n| 1 | 2 |",
			"## Missing",
			0,
			"",
		},
		{
			"no table under heading",
			"## Data\njust text here",
			"## Data",
			0,
			"",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractTableAfterHeading(tc.content, tc.heading)
			if len(got) != tc.wantLen {
				t.Fatalf("got %d rows, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen > 0 && len(got[0]) == 0 {
				t.Error("expected at least one column in table")
			}
			if tc.wantCol != "" && len(got) > 0 {
				if _, ok := got[0][tc.wantCol]; !ok {
					t.Errorf("missing column %q", tc.wantCol)
				}
			}
		})
	}
}

// ── extractSteps ────────────────────────────────────────────────────────────

func TestExtractSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    int
		first   string
	}{
		{
			"two steps",
			"### Step 1: Setup\ndo setup things\n### Step 2: Run\nrun the process",
			2,
			"Setup",
		},
		{
			"no steps",
			"just plain text here",
			0,
			"",
		},
		{
			"single step",
			"### Step 1: Initialize\ninit details here",
			1,
			"Initialize",
		},
		{
			"three steps",
			"### Step 1: A\nstep a\n### Step 2: B\nstep b\n### Step 3: C\nstep c",
			3,
			"A",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := extractSteps(tc.content)
			if len(steps) != tc.want {
				t.Fatalf("got %d steps, want %d", len(steps), tc.want)
			}
			if tc.want > 0 && steps[0]["name"] != tc.first {
				t.Errorf("step 0 name = %q, want %q", steps[0]["name"], tc.first)
			}
		})
	}
}

// ── extractRelationships ────────────────────────────────────────────────────

func TestExtractRelationships(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	t.Run("cross references", func(t *testing.T) {
		t.Parallel()
		content := "See [the doc](../path/to/doc.md) for more info."
		rels := ep.extractRelationships(content, nil)
		if len(rels) != 1 {
			t.Fatalf("got %d relationships, want 1", len(rels))
		}
		if rels[0].Type != RelReferences {
			t.Errorf("Type = %s, want references", rels[0].Type)
		}
		if rels[0].Weight != 1.0 {
			t.Errorf("Weight = %f, want 1.0", rels[0].Weight)
		}
	})

	t.Run("depends on relationship", func(t *testing.T) {
		t.Parallel()
		content := "depends_on: other-agent\nsome text"
		rels := ep.extractRelationships(content, nil)
		found := false
		for _, r := range rels {
			if r.Type == RelDependsOn {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected depends_on relationship")
		}
	})

	t.Run("extends relationship", func(t *testing.T) {
		t.Parallel()
		content := "extends: base-agent\ntext"
		rels := ep.extractRelationships(content, nil)
		found := false
		for _, r := range rels {
			if r.Type == RelExtends {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected extends relationship")
		}
	})

	t.Run("reports_to relationship", func(t *testing.T) {
		t.Parallel()
		content := "reports_to: CTO\nmore stuff"
		rels := ep.extractRelationships(content, nil)
		found := false
		for _, r := range rels {
			if r.Type == RelReportsTo {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected reports_to relationship")
		}
	})

	t.Run("no relationships", func(t *testing.T) {
		t.Parallel()
		rels := ep.extractRelationships("just plain text", nil)
		if len(rels) != 0 {
			t.Errorf("got %d relationships, want 0", len(rels))
		}
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()
		rels := ep.extractRelationships("", nil)
		if len(rels) != 0 {
			t.Errorf("got %d relationships, want 0", len(rels))
		}
	})

	t.Run("multiple cross references", func(t *testing.T) {
		t.Parallel()
		content := "[a](../a.md) and [b](../b.md)"
		rels := ep.extractRelationships(content, nil)
		refCount := 0
		for _, r := range rels {
			if r.Type == RelReferences {
				refCount++
			}
		}
		if refCount != 2 {
			t.Errorf("got %d reference rels, want 2", refCount)
		}
	})
}

// ── parseAgent ──────────────────────────────────────────────────────────────

func TestParseAgentBasic(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	content := "# AGENT: test-agent\n\n## 1. ROLE\nTest role description"
	entity, err := ep.parseAgent("/tmp/test-agent/agent.md", content, nil)
	if err != nil {
		t.Fatalf("parseAgent error: %v", err)
	}
	if entity.Name != "test-agent" {
		t.Errorf("agent name = %q", entity.Name)
	}
}

func TestParseAgent_Full(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	// Build a full agent document
	doc := &markdown.Document{
		Title: "AGENT: chief-architect",
		Frontmatter: markdown.Frontmatter{
			Data: map[string]interface{}{
				"type":    "agent",
				"version": "1.2.3",
			},
		},
	}

	content := strings.Join([]string{
		"# AGENT: chief-architect",
		"**Version**: 1.2.3",
		"**Status**: active",
		"**Owner**: John Doe",
		"## 1. ROLE",
		"Architecture decision maker",
		"## 3. RESPONSIBILITIES",
		"- Design system architecture",
		"- Review ADRs",
		"## 7. DEPENDENCIES",
		"| name | version |",
		"|------|---------|",
		"| kernel | 1.0 |",
		"## 6. TOOLS",
		"| tool | purpose |",
		"|------|---------|",
		"| planner | planning |",
		"## 4. INPUTS",
		"| input | type |",
		"|-------|------|",
		"| spec | doc |",
		"## 5. OUTPUTS",
		"| output | type |",
		"|--------|------|",
		"| design | doc |",
	}, "\n")

	entity, err := ep.parseAgent("/path/to/agent.md", content, doc)
	if err != nil {
		t.Fatalf("parseAgent error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityAgent {
		t.Errorf("Type = %s, want agent", entity.Type)
	}
	if entity.Name != "chief-architect" {
		t.Errorf("Name = %q, want chief-architect", entity.Name)
	}
	if entity.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", entity.Version)
	}
	if entity.Status != "active" {
		t.Errorf("Status = %q, want active", entity.Status)
	}
	if entity.Owner != "John Doe" {
		t.Errorf("Owner = %q, want John Doe", entity.Owner)
	}
	if _, ok := entity.Metadata["responsibilities"]; !ok {
		t.Error("expected responsibilities in metadata")
	}
	if _, ok := entity.Metadata["dependencies"]; !ok {
		t.Error("expected dependencies in metadata")
	}
	if _, ok := entity.Metadata["tools"]; !ok {
		t.Error("expected tools in metadata")
	}
}

func TestParseAgent_AGENTPrefixVariants(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	tests := []struct {
		name string
		doc  *markdown.Document
		path string
		want string
	}{
		{
			"colon prefix",
			&markdown.Document{Title: "AGENT: my-agent"},
			"/agent_dna/chief.md",
			"my-agent",
		},
		{
			"space prefix",
			&markdown.Document{Title: "AGENT my-agent"},
			"/agent_dna/chief.md",
			"my-agent",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entity, err := ep.parseAgent(tc.path, "# Heading\ntext", tc.doc)
			if err != nil {
				t.Fatalf("parseAgent error: %v", err)
			}
			if entity.Name != tc.want {
				t.Errorf("Name = %q, want %q", entity.Name, tc.want)
			}
		})
	}
}

func TestParseAgent_NoNameFallback(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	entity, err := ep.parseAgent("/departments/engineering/agent.md", "", nil)
	if err != nil {
		t.Fatalf("parseAgent error: %v", err)
	}
	// Should fallback to directory name
	if entity.Name != "engineering" {
		t.Errorf("Name = %q, want engineering", entity.Name)
	}
}

// ── parseSkill ──────────────────────────────────────────────────────────────

func TestParseSkill(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "My Skill",
	}

	content := strings.Join([]string{
		"# My Skill",
		"**Version**: 1.0.0",
		"**Status**: stable",
		"**Owner**: team-alpha",
		"## PURPOSE",
		"Does something useful",
		"## SCOPE",
		"Applies to all modules",
		"## DEPENDENCIES",
		"| dep | version |",
		"|-----|---------|",
		"| lib | 2.0 |",
	}, "\n")

	entity, err := ep.parseSkill("/skills/my-skill.md", content, doc)
	if err != nil {
		t.Fatalf("parseSkill error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntitySkill {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "My Skill" {
		t.Errorf("Name = %q", entity.Name)
	}
	if entity.Version != "1.0.0" {
		t.Errorf("Version = %q", entity.Version)
	}
	if entity.Owner != "team-alpha" {
		t.Errorf("Owner = %q", entity.Owner)
	}
	if entity.Status != "stable" {
		t.Errorf("Status = %q", entity.Status)
	}
	if _, ok := entity.Metadata["category"]; !ok {
		t.Error("expected category in metadata")
	}
	if _, ok := entity.Metadata["dependencies"]; !ok {
		t.Error("expected dependencies in metadata")
	}
}

func TestParseSkill_NilDoc(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	entity, err := ep.parseSkill("/skills/noskill.md", "some content", nil)
	if err != nil {
		t.Fatalf("parseSkill error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Name != "" {
		t.Errorf("Name = %q, want empty", entity.Name)
	}
}

// ── parseWorkflow ───────────────────────────────────────────────────────────

func TestParseWorkflow(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "Deploy Workflow",
	}

	content := strings.Join([]string{
		"# Deploy Workflow",
		"**Version**: 2.0",
		"**Category**: deploy",
		"## OBJECTIVE",
		"Deploy the application",
		"### Step 1: Build",
		"Run build",
		"### Step 2: Test",
		"Run tests",
		"## DEPENDENCIES",
		"| dep | ver |",
		"|-----|-----|",
		"| ci  | 1.0 |",
		"## INPUTS",
		"| input | type |",
		"|-------|------|",
		"| code | src |",
		"## OUTPUTS",
		"| output | type |",
		"|--------|------|",
		"| binary | exe |",
	}, "\n")

	entity, err := ep.parseWorkflow("/workflows/deploy.md", content, doc)
	if err != nil {
		t.Fatalf("parseWorkflow error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityWorkflow {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "Deploy Workflow" {
		t.Errorf("Name = %q", entity.Name)
	}
	if entity.Version != "2.0" {
		t.Errorf("Version = %q", entity.Version)
	}
	if _, ok := entity.Metadata["category"]; !ok {
		t.Error("expected category in metadata")
	}
	if _, ok := entity.Metadata["steps"]; !ok {
		t.Error("expected steps in metadata")
	} else {
		steps, _ := entity.Metadata["steps"].([]map[string]string)
		if len(steps) != 2 {
			t.Errorf("got %d steps, want 2", len(steps))
		}
	}
}

// ── parseTemplate ───────────────────────────────────────────────────────────

func TestParseTemplate(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "React Template",
	}

	content := strings.Join([]string{
		"# React Template",
		"**Version**: 1.0",
		"## DOMAIN",
		"Frontend web applications",
		"## RECOMMENDED STACK",
		"| tech | version |",
		"|------|---------|",
		"| react | 18 |",
		"## KEY FEATURES",
		"- SSR",
		"- Routing",
	}, "\n")

	entity, err := ep.parseTemplate("/templates/react.md", content, doc)
	if err != nil {
		t.Fatalf("parseTemplate error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityTemplate {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "React Template" {
		t.Errorf("Name = %q", entity.Name)
	}
	if _, ok := entity.Metadata["stack"]; !ok {
		t.Error("expected stack in metadata")
	}
	if _, ok := entity.Metadata["features"]; !ok {
		t.Error("expected features in metadata")
	}
}

// ── parseProvider ───────────────────────────────────────────────────────────

func TestParseProvider_YAML(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	content := `id: openai
name: OpenAI
models:
  - gpt-4
  - gpt-3.5-turbo
capabilities:
  - chat
  - embedding
`

	entity, err := ep.parseProvider("/providers/openai.md", content, nil)
	if err != nil {
		t.Fatalf("parseProvider error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityProvider {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "openai" {
		t.Errorf("Name = %q, want openai", entity.Name)
	}
	if _, ok := entity.Metadata["display_name"]; !ok {
		t.Error("expected display_name in metadata")
	}
}

func TestParseProvider_WithDoc(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "Provider Title",
	}

	entity, err := ep.parseProvider("/providers/test.md", "", doc)
	if err != nil {
		t.Fatalf("parseProvider error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Name != "Provider Title" {
		t.Errorf("Name = %q", entity.Name)
	}
}

// ── parsePlugin ─────────────────────────────────────────────────────────────

func TestParsePlugin_JSON(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	content := `{
		"id": "my-plugin",
		"version": "1.0.0",
		"description": "A test plugin",
		"permissions": ["read", "write"],
		"lifecycle": "active"
	}`

	entity, err := ep.parsePlugin("/plugins/my-plugin.md", content, nil)
	if err != nil {
		t.Fatalf("parsePlugin error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityPlugin {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "my-plugin" {
		t.Errorf("Name = %q", entity.Name)
	}
	if entity.Version != "1.0.0" {
		t.Errorf("Version = %q", entity.Version)
	}
	if entity.Description != "A test plugin" {
		t.Errorf("Description = %q", entity.Description)
	}
	if _, ok := entity.Metadata["permissions"]; !ok {
		t.Error("expected permissions in metadata")
	}
	if _, ok := entity.Metadata["lifecycle"]; !ok {
		t.Error("expected lifecycle in metadata")
	}
}

func TestParsePlugin_NonJSON(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "Plugin Title",
	}

	entity, err := ep.parsePlugin("/plugins/test.md", "not json content", doc)
	if err != nil {
		t.Fatalf("parsePlugin error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Name != "Plugin Title" {
		t.Errorf("Name = %q", entity.Name)
	}
}

// ── parseADR ────────────────────────────────────────────────────────────────

func TestParseADR(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "ADR-001: Use Go",
	}

	content := strings.Join([]string{
		"# ADR-001: Use Go",
		"## Status",
		"Accepted",
		"## Context",
		"We needed a language for CLI tools.",
		"## Decision",
		"Use Go for all CLI development.",
		"## Consequences",
		"Fast builds, static binaries.",
	}, "\n")

	entity, err := ep.parseADR("/adr/001-use-go.md", content, doc)
	if err != nil {
		t.Fatalf("parseADR error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityADR {
		t.Errorf("Type = %s", entity.Type)
	}
	if entity.Name != "ADR-001: Use Go" {
		t.Errorf("Name = %q", entity.Name)
	}
	if _, ok := entity.Metadata["adr_number"]; !ok {
		t.Error("expected adr_number in metadata")
	}
	if entity.Metadata["adr_number"] != "001" {
		t.Errorf("adr_number = %v", entity.Metadata["adr_number"])
	}
	if _, ok := entity.Metadata["status"]; !ok {
		t.Error("expected status in metadata")
	}
	if _, ok := entity.Metadata["decision"]; !ok {
		t.Error("expected decision in metadata")
	}
}

// ── parseCapability ─────────────────────────────────────────────────────────

func TestParseCapability_YAML(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	content := `id: auth-capability
category: security
dependencies:
  - oauth2
  - jwt
`

	entity, err := ep.parseCapability("/capabilities/auth.md", content, nil)
	if err != nil {
		t.Fatalf("parseCapability error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Type != EntityCapability {
		t.Errorf("Type = %s", entity.Type)
	}
	if _, ok := entity.Metadata["capability_id"]; !ok {
		t.Error("expected capability_id in metadata")
	}
}

func TestParseCapability_NonYAML(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	doc := &markdown.Document{
		Title: "Capability Name",
	}

	entity, err := ep.parseCapability("/capabilities/test.md", "# Not YAML", doc)
	if err != nil {
		t.Fatalf("parseCapability error: %v", err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Name != "Capability Name" {
		t.Errorf("Name = %q", entity.Name)
	}
}

// ── parseGeneric ────────────────────────────────────────────────────────────

func TestParseGeneric(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	doc := &markdown.Document{
		Title: "My Document",
		Frontmatter: markdown.Frontmatter{
			Data: map[string]interface{}{
				"key1": "val1",
			},
		},
	}

	entity := ep.parseGeneric("/path/file.md", "# My Document\n\nSome description", doc, EntitySkill)
	if entity == nil {
		t.Fatal("parseGeneric returned nil")
	}
	if entity.Type != EntitySkill {
		t.Errorf("Type = %v", entity.Type)
	}
	if entity.Name != "My Document" {
		t.Errorf("Name = %q", entity.Name)
	}
	if _, ok := entity.Metadata["key1"]; !ok {
		t.Error("expected frontmatter key1 in metadata")
	}
}

func TestParseGeneric_NoTitle(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	// When doc is nil, parseGeneric leaves Name empty (falls through in detectEntityType)
	entity := ep.parseGeneric("/path/file.md", "just content\nno title", nil, EntityConfig)
	if entity == nil {
		t.Fatal("entity is nil")
	}
	// With nil doc, name is empty since there's no title to fall back on
	if entity.Name != "" {
		t.Errorf("Name = %q, want empty", entity.Name)
	}
}

func TestParseGeneric_EmptyContent(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()
	entity := ep.parseGeneric("/path/empty.md", "", nil, EntitySkill)
	if entity == nil {
		t.Fatal("entity is nil")
	}
	if entity.Description != "" {
		t.Errorf("Description = %q, want empty", entity.Description)
	}
}

// ── ParseFile ───────────────────────────────────────────────────────────────

func TestParseFile_Skill(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	content := strings.Join([]string{
		"# My Skill",
		"**Version**: 1.0",
		"**Status**: active",
		"## PURPOSE",
		"Does work",
	}, "\n")

	entities, err := ep.ParseFile("/skills/my-skill.md", content)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(entities))
	}
	if entities[0].Type != EntitySkill {
		t.Errorf("Type = %s", entities[0].Type)
	}
}

func TestParseFile_Generic(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	entities, err := ep.ParseFile("/unknown/basic.md", "# Title\n\nSimple content")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(entities))
	}
	if entities[0].Name != "Title" {
		t.Errorf("Name = %q", entities[0].Name)
	}
}

func TestParseFile_WorksWithRelationships(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	content := "# My Skill\nSee [the doc](../path/to/doc.md)\ndepends_on: external-service"

	entities, err := ep.ParseFile("/skills/test.md", content)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(entities))
	}
	if len(entities[0].Relationships) == 0 {
		t.Error("expected relationships on entity")
	}
}

func TestParseFile_EmptyContent(t *testing.T) {
	t.Parallel()

	ep := NewEntityParser()

	entities, err := ep.ParseFile("/path/empty.md", "")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(entities))
	}
}

// ── empty input edge cases ─────────────────────────────────────────────────

func TestExtractSection_EmptyInput(t *testing.T) {
	t.Parallel()

	got := extractSection("", "## Missing")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractListAfterHeading_EmptyInput(t *testing.T) {
	t.Parallel()

	got := extractListAfterHeading("", "## Missing")
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestExtractTableAfterHeading_EmptyInput(t *testing.T) {
	t.Parallel()

	got := extractTableAfterHeading("", "## Missing")
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestExtractSteps_EmptyInput(t *testing.T) {
	t.Parallel()

	got := extractSteps("")
	if len(got) != 0 {
		t.Errorf("got %d steps, want 0", len(got))
	}
}

// ── parseTableRow edge case ────────────────────────────────────────────────

func TestParseTableRow_SeparatorLine(t *testing.T) {
	t.Parallel()

	// A separator line like |---|----| is stripped to empty cell strings
	got := parseTableRow("|---|----|")
	if len(got) != 2 {
		t.Fatalf("got %d cells, want 2", len(got))
	}
	// Separator line dashes are treated as cell content
	if got[0] != "---" {
		t.Errorf("cell[0] = %q, want ---", got[0])
	}
}
