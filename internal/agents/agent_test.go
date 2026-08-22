package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager_LoadsAgentsFromCosca(t *testing.T) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	coscaDir := filepath.Join(homedir, ".config", "opencode", "cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "departments")); os.IsNotExist(err) {
		t.Skip("Cosca departments directory not found")
	}

	m := NewManager(coscaDir)
	list := m.List()

	if len(list) == 0 {
		t.Fatal("expected at least one agent to be loaded")
	}

	t.Logf("Loaded %d agents from Cosca framework", len(list))

	// Verify key agents exist
	expected := []struct {
		name    string
		role    string
		dept    string
		reports string
	}{
		{"CEO", "Chief Executive Officer", "ceo", "User"},
		{"CTO", "", "cto", "CEO"},
		{"AUTOMATION CHIEF", "", "automation", "CTO"},
		{"BACKEND CHIEF", "", "backend", "CTO"},
		{"ARCHITECTURE CHIEF", "", "architecture", "CTO"},
	}

	for _, exp := range expected {
		agent, err := m.Get(exp.name)
		if err != nil {
			t.Errorf("expected agent %q to be loaded, got error: %v", exp.name, err)
			continue
		}
		if agent.Name != exp.name {
			t.Errorf("expected name %q, got %q", exp.name, agent.Name)
		}
		if exp.role != "" && agent.Role != exp.role {
			t.Errorf("agent %q: expected role %q, got %q", exp.name, exp.role, agent.Role)
		}
		if agent.Department != exp.dept {
			t.Errorf("agent %q: expected department %q, got %q", exp.name, exp.dept, agent.Department)
		}
		if exp.reports != "" && agent.ReportsTo != exp.reports {
			t.Errorf("agent %q: expected reports_to %q, got %q", exp.name, exp.reports, agent.ReportsTo)
		}
	}
}

func TestGet_CaseInsensitive(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "AUTOMATION CHIEF", Role: "Automation Chief"})

	tests := []struct {
		query string
		found bool
	}{
		{"AUTOMATION CHIEF", true},
		{"automation chief", true},
		{"Automation Chief", true},
		{"AUTOMATION", false},
		{"chief", false},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		_, err := m.Get(tt.query)
		if tt.found && err != nil {
			t.Errorf("Get(%q): expected found, got error: %v", tt.query, err)
		}
		if !tt.found && err == nil {
			t.Errorf("Get(%q): expected not found, but got agent", tt.query)
		}
	}
}

func TestAdd_Method(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Add(Agent{Name: "test-agent", Role: "Tester", Department: "testing"})

	agent, err := m.Get("test-agent")
	if err != nil {
		t.Fatalf("expected agent to be found after Add, got error: %v", err)
	}
	if agent.Name != "test-agent" {
		t.Errorf("expected name 'test-agent', got %q", agent.Name)
	}
	if agent.Role != "Tester" {
		t.Errorf("expected role 'Tester', got %q", agent.Role)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	m := &Manager{agents: make(map[string]*Agent)}
	m.Register(&Agent{Name: "AUTOMATION CHIEF", Role: "Automation Chief", Department: "automation", Description: "Builds automation tools"})
	m.Register(&Agent{Name: "BACKEND CHIEF", Role: "Backend Chief", Department: "backend", Description: "Leads backend development"})

	tests := []struct {
		query   string
		results int
	}{
		{"automation", 1},
		{"AUTOMATION", 1},
		{"chief", 2},
		{"CHIEF", 2},
		{"backend", 1},
		{"Backend", 1},
	}

	for _, tt := range tests {
		results, err := m.Search(tt.query)
		if err != nil {
			t.Errorf("Search(%q): unexpected error: %v", tt.query, err)
			continue
		}
		if len(results) != tt.results {
			t.Errorf("Search(%q): expected %d results, got %d", tt.query, tt.results, len(results))
		}
	}
}

func TestParseSkillFile_Automation(t *testing.T) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	skillPath := filepath.Join(homedir, ".config", "opencode", "cosca", "departments", "automation", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Skip("automation SKILL.md not found")
	}

	agent, err := parseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("parseSkillFile failed: %v", err)
	}

	if agent.Name != "AUTOMATION CHIEF" {
		t.Errorf("expected name 'AUTOMATION CHIEF', got %q", agent.Name)
	}
	if agent.Version == "" {
		t.Error("expected version to be non-empty")
	}
	if agent.Status != "active" {
		t.Errorf("expected status 'active', got %q", agent.Status)
	}
	if agent.ReportsTo != "CTO" {
		t.Errorf("expected reports_to 'CTO', got %q", agent.ReportsTo)
	}
	if agent.Description == "" {
		t.Error("expected description to be non-empty")
	}
	if len(agent.Responsibilities) == 0 {
		t.Error("expected at least one responsibility")
	}
	if len(agent.Dependencies) == 0 {
		t.Error("expected at least one dependency")
	}
	if len(agent.Outputs) == 0 {
		t.Error("expected at least one output")
	}
}

func TestParseSkillFile_Backend_DualFormat(t *testing.T) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	skillPath := filepath.Join(homedir, ".config", "opencode", "cosca", "departments", "backend", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Skip("backend SKILL.md not found (dual-format test)")
	}

	agent, err := parseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("parseSkillFile failed: %v", err)
	}

	// Name should be "BACKEND CHIEF" not "HISTORY" (dual-format regression)
	if agent.Name != "BACKEND CHIEF" {
		t.Errorf("expected name 'BACKEND CHIEF', got %q (dual-format regression)", agent.Name)
	}
	if agent.ReportsTo == "" {
		t.Error("expected reports_to to be non-empty")
	}
	if agent.ReportsTo == "> **Reports To**: CTO" {
		t.Error("reports_to contains raw blockquote marker")
	}
	if agent.Description == "" {
		t.Error("expected description to be non-empty")
	}
}

func TestParseSkillFile_CEO(t *testing.T) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	skillPath := filepath.Join(homedir, ".config", "opencode", "cosca", "departments", "ceo", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Skip("ceo SKILL.md not found")
	}

	agent, err := parseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("parseSkillFile failed: %v", err)
	}

	if agent.Name != "CEO" {
		t.Errorf("expected name 'CEO', got %q", agent.Name)
	}
	if agent.Role != "Chief Executive Officer" {
		t.Errorf("expected role 'Chief Executive Officer', got %q", agent.Role)
	}
	if agent.ReportsTo != "User" {
		t.Errorf("expected reports_to 'User', got %q", agent.ReportsTo)
	}
	if len(agent.Responsibilities) != 7 {
		t.Errorf("expected 7 responsibilities, got %d", len(agent.Responsibilities))
	}
}
