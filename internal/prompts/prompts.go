// Package prompts provides the prompt management system for Cosca.
// Prompts are reusable instruction templates that guide AI agents.
package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/embed"
)

// Prompt represents a reusable prompt template.
type Prompt struct {
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description" yaml:"description"`
	Version     string      `json:"version" yaml:"version"`
	Category    string      `json:"category" yaml:"category"`
	Template    string      `json:"template" yaml:"template"`
	Parameters  []Parameter `json:"parameters" yaml:"parameters"`
}

// Parameter defines a parameter for a prompt template.
type Parameter struct {
	Name    string      `json:"name" yaml:"name"`
	Type    string      `json:"type" yaml:"type"`
	Default interface{} `json:"default" yaml:"default"`
}

// CreateOptions defines options for creating a new prompt.
type CreateOptions struct {
	Name        string
	Template    string
	Description string
}

// Manager manages prompts in the Cosca system.
type Manager struct {
	prompts map[string]*Prompt
}

// NewManager creates a new prompt manager and loads prompt files
// from the embedded Cosca framework and from the Cosca prompts/ directory.
func NewManager(coscaDir string) *Manager {
	m := &Manager{
		prompts: make(map[string]*Prompt),
	}
	m.loadFromEmbed()
	if coscaDir != "" {
		m.loadFromDir(coscaDir)
	}
	return m
}

// loadFromEmbed loads prompts from the embedded Cosca framework assets.
// It walks the prompts/ directory in the embedded filesystem, reads all
// .md files, parses them, and registers them with the manager.
// If the prompts directory does not exist in the embedded assets, it is
// silently skipped.
func (m *Manager) loadFromEmbed() {
	files, err := embed.ListFilesRecursive("prompts")
	if err != nil {
		return // prompts directory does not exist in embedded assets
	}
	for _, f := range files {
		content, err := embed.ReadString(f)
		if err != nil {
			continue
		}
		p := parsePromptFile(filepath.Base(f), content)
		if p != nil {
			m.prompts[strings.ToLower(p.Name)] = p
		}
	}
}

// loadFromDir scans the prompts/ subdirectory under dir for .md files
// and loads them into the manager. Non-existent or unreadable directories
// are silently skipped.
func (m *Manager) loadFromDir(dir string) {
	promptsDir := filepath.Join(dir, "prompts")
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return // prompts directory does not exist or is not accessible
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(promptsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		p := parsePromptFile(entry.Name(), string(data))
		if p != nil {
			m.prompts[strings.ToLower(p.Name)] = p
		}
	}
}

// parsePromptFile parses a markdown file into a Prompt.
// It extracts YAML frontmatter (between --- markers) for metadata
// and uses the remainder as the template.
func parsePromptFile(filename, content string) *Prompt {
	p := &Prompt{
		Name:     strings.TrimSuffix(filename, ".md"),
		Category: "general",
		Version:  "1.0",
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return p
	}

	// Try to parse YAML frontmatter (between --- markers)
	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		endIdx := strings.Index(rest, "\n---")
		if endIdx > 0 {
			frontmatter := rest[:endIdx]
			// Template starts after the closing --- (skip the newline)
			templateStart := endIdx + 5 // len("\n---") + possible \n
			if templateStart < len(rest) {
				p.Template = strings.TrimSpace(rest[templateStart:])
			}
			parseFrontmatter(frontmatter, p)
			return p
		}
	}

	// No frontmatter found; entire content is the template
	p.Template = content
	return p
}

// parseFrontmatter extracts metadata fields from a YAML frontmatter string.
// It recognizes name, description, version, and category keys.
func parseFrontmatter(fm string, p *Prompt) {
	lines := strings.Split(fm, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(strings.ToLower(line[:idx]))
			value := strings.TrimSpace(line[idx+1:])
			value = strings.Trim(value, `"'`)
			switch key {
			case "name":
				if value != "" {
					p.Name = value
				}
			case "description":
				p.Description = value
			case "version":
				p.Version = value
			case "category":
				p.Category = value
			}
		}
	}
}

// Add registers a prompt with the manager.
func (m *Manager) Add(p Prompt) {
	m.prompts[strings.ToLower(p.Name)] = &p
}

// List returns all available prompts.
func (m *Manager) List() []Prompt {
	result := make([]Prompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, *p)
	}
	return result
}

// Get returns a specific prompt by name (case-insensitive).
func (m *Manager) Get(name string) (*Prompt, error) {
	p, ok := m.prompts[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("prompt %q not found", name)
	}
	return p, nil
}

// Search searches for prompts by query string (case-insensitive).
func (m *Manager) Search(query string) ([]Prompt, error) {
	var result []Prompt
	for _, p := range m.prompts {
		if contains(p.Name, query) || contains(p.Description, query) || contains(p.Category, query) {
			result = append(result, *p)
		}
	}
	return result, nil
}

// Create creates a new prompt and registers it with the manager.
func (m *Manager) Create(opts CreateOptions) (*Prompt, error) {
	p := &Prompt{
		Name:        opts.Name,
		Description: opts.Description,
		Version:     "1.0",
		Category:    "general",
		Template:    opts.Template,
	}
	m.prompts[strings.ToLower(opts.Name)] = p
	return p, nil
}

// contains checks whether substr appears in s, ignoring case.
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
