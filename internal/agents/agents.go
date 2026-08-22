// Package agents provides the agent management system for Cosca.
// Agents are specialized AI assistants that perform specific tasks
// within the Cosca ecosystem.
package agents

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/embed"
)

// Agent represents an Cosca agent with its capabilities and responsibilities.
type Agent struct {
	Name             string       `json:"name" yaml:"name"`
	Role             string       `json:"role" yaml:"role"`
	Mission          string       `json:"mission" yaml:"mission"`
	Status           string       `json:"status" yaml:"status"`
	Version          string       `json:"version" yaml:"version"`
	Department       string       `json:"department" yaml:"department"`
	ReportsTo        string       `json:"reports_to" yaml:"reports_to"`
	Capabilities     []Capability `json:"capabilities" yaml:"capabilities"`
	Tools            []Tool       `json:"tools" yaml:"tools"`
	Responsibilities []string     `json:"responsibilities" yaml:"responsibilities"`
	Description      string       `json:"description" yaml:"description"`
	Dependencies     []TableRow   `json:"dependencies" yaml:"dependencies"`
	Inputs           []TableRow   `json:"inputs" yaml:"inputs"`
	Outputs          []TableRow   `json:"outputs" yaml:"outputs"`
}

// TableRow represents a row in a markdown table with key and value columns.
type TableRow struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

// Capability describes an agent capability.
type Capability struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// Tool describes a tool available to an agent.
type Tool struct {
	Name     string `json:"name" yaml:"name"`
	Category string `json:"category" yaml:"category"`
	Purpose  string `json:"purpose" yaml:"purpose"`
}

// Manager manages the lifecycle and querying of agents.
type Manager struct {
	agents map[string]*Agent
}

// NewManager creates a new agent manager and scans both the embedded Cosca
// framework assets and the provided Cosca directory (if any) for department
// SKILL.md files to populate the agent registry. Embedded agents are loaded
// first, then overridden by any agents from the local directory.
func NewManager(coscaDir string) *Manager {
	m := &Manager{
		agents: make(map[string]*Agent),
	}
	// Always load embedded Cosca framework agents first.
	m.loadFromEmbed()
	// Then load from the local Cosca directory (may override embedded agents).
	if coscaDir != "" {
		m.loadFromDir(coscaDir)
	}
	return m
}

// loadFromEmbed loads agents from the embedded Cosca framework assets.
// It reads the departments/ directory from the compiled-in Cosca filesystem
// and parses each department's SKILL.md file.
func (m *Manager) loadFromEmbed() {
	entries, err := embed.ReadDir("departments")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// embed.FS paths always use forward slashes on every platform —
		// filepath.Join would produce backslashes on Windows and the lookup
		// would fail. path.Join yields identical results on Linux.
		skillPath := path.Join("departments", entry.Name(), "SKILL.md")
		content, err := embed.ReadString(skillPath)
		if err != nil {
			continue
		}

		agent, err := parseSkillContent(content)
		if err != nil {
			continue
		}
		agent.Department = entry.Name()
		m.agents[agent.Name] = agent
	}
}

// loadFromDir scans the departments directory inside the given Cosca directory
// for SKILL.md files and registers each as an agent.
func (m *Manager) loadFromDir(coscaDir string) {
	deptsDir := filepath.Join(coscaDir, "departments")
	info, err := os.Stat(deptsDir)
	if err != nil || !info.IsDir() {
		return
	}

	entries, err := os.ReadDir(deptsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(deptsDir, entry.Name(), "SKILL.md")
		agent, err := parseSkillFile(skillPath)
		if err != nil {
			continue
		}
		agent.Department = entry.Name()
		m.agents[agent.Name] = agent
	}
}

// parseSkillFile reads a SKILL.md file from disk and extracts agent metadata from it.
func parseSkillFile(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSkillContent(string(data))
}

// parseSkillContent parses a SKILL.md content string and extracts agent metadata from it.
func parseSkillContent(content string) (*Agent, error) {
	lines := strings.Split(content, "\n")

	agent := &Agent{
		Status:           "active",
		Responsibilities: []string{},
		Dependencies:     []TableRow{},
		Inputs:           []TableRow{},
		Outputs:          []TableRow{},
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Extract version from quote block: > **Version**: X.X.X
		if agent.Version == "" && strings.Contains(line, "**Version**:") {
			agent.Version = extractColonValue(line, "**Version**:")
		}

		// Extract status from quote block: > **Status**: value
		if strings.Contains(line, "**Status**:") {
			agent.Status = extractColonValue(line, "**Status**:")
		}

		// Extract reports_to from list or blockquote: - **Reports To**: Name or > **Reports To**: Name
		if agent.ReportsTo == "" && strings.Contains(line, "**Reports To**:") {
			val := extractColonValue(line, "**Reports To**:")
			// Strip leading blockquote marker if present
			val = strings.TrimLeft(val, "> ")
			agent.ReportsTo = strings.TrimSpace(val)
		}

		// Extract heading for name and role: # NAME — Role or # NAME
		// Only use the first heading to avoid being overwritten by later sections.
		if agent.Name == "" && strings.HasPrefix(line, "# ") {
			heading := strings.TrimPrefix(line, "# ")
			heading = strings.TrimSpace(heading)
			if parts := strings.SplitN(heading, "—", 2); len(parts) == 2 {
				agent.Name = strings.TrimSpace(parts[0])
				agent.Role = strings.TrimSpace(parts[1])
			} else if parts := strings.SplitN(heading, "-", 2); len(parts) == 2 {
				agent.Name = strings.TrimSpace(parts[0])
				agent.Role = strings.TrimSpace(parts[1])
			} else {
				agent.Name = heading
				agent.Role = heading
			}
		}

		// Extract PURPOSE / description: text after ## PURPOSE
		if line == "## PURPOSE" && i+1 < len(lines) {
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if next == "" {
					continue
				}
				if strings.HasPrefix(next, "## ") {
					break
				}
				if agent.Description == "" {
					agent.Description = next
				} else {
					agent.Description += " " + next
				}
			}
		}

		// Extract RESPONSIBILITIES: numbered list
		if line == "## RESPONSIBILITIES" {
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "## ") {
					break
				}
				// Match numbered list items: "1. text" or "1. text"
				item := extractListItem(trimmed)
				if item != "" {
					agent.Responsibilities = append(agent.Responsibilities, item)
				}
			}
		}

		// Extract DEPENDENCIES table: | Key | Value |
		if line == "## DEPENDENCIES" {
			rows := extractTableRows(lines[i+1:])
			agent.Dependencies = append(agent.Dependencies, rows...)
		}

		// Extract INPUTS table
		if line == "## INPUTS" {
			rows := extractTableRows(lines[i+1:])
			agent.Inputs = append(agent.Inputs, rows...)
		}

		// Extract OUTPUTS table
		if line == "## OUTPUTS" {
			rows := extractTableRows(lines[i+1:])
			agent.Outputs = append(agent.Outputs, rows...)
		}
	}

	if agent.Name == "" {
		return nil, fmt.Errorf("no name found in SKILL.md content")
	}

	return agent, nil
}

// extractColonValue extracts the value after a key marker like "**Version**:".
func extractColonValue(line, marker string) string {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return ""
	}
	after := line[idx+len(marker):]
	// Split on pipe or end-of-string to handle "| Status: active | ..."
	if pipeIdx := strings.Index(after, "|"); pipeIdx >= 0 {
		after = after[:pipeIdx]
	}
	return strings.TrimSpace(after)
}

// extractListItem extracts the text from a numbered list item like "1. Do something".
func extractListItem(line string) string {
	// Find ". " after digits
	for i := 0; i < len(line); i++ {
		if line[i] >= '0' && line[i] <= '9' {
			continue
		}
		if line[i] == '.' && i+1 < len(line) && line[i+1] == ' ' {
			return strings.TrimSpace(line[i+2:])
		}
		break
	}
	return ""
}

// extractTableRows parses markdown table rows from a slice of lines.
// It skips the header separator row (|---|...|) and extracts | key | value | pairs.
func extractTableRows(lines []string) []TableRow {
	var rows []TableRow
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip the header or separator rows
		if strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "|") {
			if strings.Contains(trimmed, "---") {
				continue
			}
			// Look for the separator row
			if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "---") {
				continue
			}
		}
		// Stop at next heading
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		// Stop at non-table content
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		// Parse: | Key | Value | or | Key | Value | Extra |
		parts := splitTableRow(trimmed)
		if len(parts) >= 2 {
			rows = append(rows, TableRow{
				Key:   strings.TrimSpace(parts[0]),
				Value: strings.TrimSpace(parts[1]),
			})
		}
	}
	return rows
}

// splitTableRow splits a markdown table row like "| Key | Value | Extra |"
// into its cell values, stripping leading/trailing pipes.
func splitTableRow(row string) []string {
	trimmed := strings.TrimSpace(row)
	// Remove leading and trailing pipes
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil
	}
	cells := strings.Split(trimmed, "|")
	var result []string
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}
	return result
}

// List returns all available agents.
func (m *Manager) List() []Agent {
	result := make([]Agent, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, *a)
	}
	return result
}

// Get returns a specific agent by name (case-insensitive).
func (m *Manager) Get(name string) (*Agent, error) {
	// Try exact match first
	if a, ok := m.agents[name]; ok {
		return a, nil
	}
	// Case-insensitive fallback
	lower := strings.ToLower(name)
	for key, a := range m.agents {
		if strings.ToLower(key) == lower {
			return a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found", name)
}

// Search searches for agents by query string (case-insensitive).
func (m *Manager) Search(query string) ([]Agent, error) {
	var result []Agent
	// Normalize query: lowercase, remove hyphens and underscores for fuzzy matching
	normalizedQ := strings.ToLower(query)
	normalizedQ = strings.ReplaceAll(normalizedQ, "-", " ")
	normalizedQ = strings.ReplaceAll(normalizedQ, "_", " ")
	for _, a := range m.agents {
		if containsIgnoreCase(a.Name, normalizedQ) || containsIgnoreCase(a.Role, normalizedQ) || containsIgnoreCase(a.Department, normalizedQ) || containsIgnoreCase(a.Description, normalizedQ) {
			result = append(result, *a)
		}
	}
	return result, nil
}

// Register adds an agent to the manager.
func (m *Manager) Register(a *Agent) {
	m.agents[a.Name] = a
}

// Add adds an agent to the manager. This is an alias for Register.
func (m *Manager) Add(a Agent) {
	m.agents[a.Name] = &a
}

// containsIgnoreCase checks if s (already lowercased) contains substr (already lowercased).
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, substr)
}
