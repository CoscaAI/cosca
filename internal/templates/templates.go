// Package templates provides the template management system for Cosca.
// Templates are re-usable blueprints for prompts, skills, workflows, and more.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/embed"
	"gopkg.in/yaml.v3"
)

// Template represents a re-usable blueprint.
type Template struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
	Version     string `json:"version" yaml:"version"`
	Content     string `json:"content" yaml:"content"`
}

// CreateOptions defines options for creating a new template.
type CreateOptions struct {
	Name        string
	Type        string
	Description string
	Content     string
}

// Manager manages templates in the Cosca system.
type Manager struct {
	templates map[string]*Template
}

// NewManager creates a new template manager and discovers templates
// from embedded Cosca framework + optional local directory.
func NewManager(coscaDir string) *Manager {
	m := &Manager{
		templates: make(map[string]*Template),
	}
	// Always load from embedded Cosca framework first
	m.loadFromEmbed()
	// Then override with local .cosca if it exists
	if coscaDir != "" {
		_ = m.loadFromDir(coscaDir)
	}
	return m
}

// Add registers a template directly.
func (m *Manager) Add(tmpl Template) {
	m.templates[tmpl.Name] = &tmpl
}

// List returns all available templates.
func (m *Manager) List() []Template {
	result := make([]Template, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result
}

// Get returns a specific template by name.
func (m *Manager) Get(name string) (*Template, error) {
	// Try exact match first
	if t, ok := m.templates[name]; ok {
		return t, nil
	}
	// Try case-insensitive match
	lower := strings.ToLower(name)
	for _, t := range m.templates {
		if strings.ToLower(t.Name) == lower {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}

// Search searches for templates by query string.
func (m *Manager) Search(query string) ([]Template, error) {
	q := strings.ToLower(query)
	var result []Template
	for _, t := range m.templates {
		if containsIgnoreCase(t.Name, q) || containsIgnoreCase(t.Description, q) || containsIgnoreCase(t.Type, q) {
			result = append(result, *t)
		}
	}
	return result, nil
}

// Create creates a new template from options.
func (m *Manager) Create(opts CreateOptions) (*Template, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("template name is required")
	}
	tmpl := &Template{
		Name:        opts.Name,
		Type:        opts.Type,
		Description: opts.Description,
		Content:     opts.Content,
	}
	m.templates[tmpl.Name] = tmpl
	return tmpl, nil
}

// loadFromEmbed loads templates from the embedded Cosca framework.
func (m *Manager) loadFromEmbed() {
	entries, err := embed.ReadDir("templates")
	if err != nil {
		return // templates dir is optional
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		content, err := embed.ReadString(filepath.Join("templates", entry.Name(), "TEMPLATE.md"))
		if err != nil {
			continue
		}
		tmpl := parseTemplateContent(content, entry.Name())
		if tmpl != nil {
			m.templates[tmpl.Name] = tmpl
		}
	}
}

// loadFromDir scans the templates/ subdirectory of the given directory
// for TEMPLATE.md files and registers each discovered template.
func (m *Manager) loadFromDir(dir string) error {
	templatesDir := filepath.Join(dir, "templates")
	info, err := os.Stat(templatesDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return fmt.Errorf("reading templates directory %s: %w", templatesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tmplDir := filepath.Join(templatesDir, entry.Name())
		tmplFile := findTemplateFile(tmplDir)
		if tmplFile == "" {
			continue
		}
		tmpl, err := parseTemplateFile(tmplFile)
		if err != nil {
			continue
		}
		m.templates[tmpl.Name] = tmpl
	}
	return nil
}

// findTemplateFile looks for a TEMPLATE.md file in the given directory.
func findTemplateFile(dir string) string {
	candidates := []string{
		filepath.Join(dir, "TEMPLATE.md"),
		filepath.Join(dir, "template.md"),
		filepath.Join(dir, "Template.md"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), "TEMPLATE.md") {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

// parseTemplateFile reads a TEMPLATE.md file from disk and extracts metadata.
func parseTemplateFile(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template file %s: %w", path, err)
	}
	return parseTemplateContent(string(data), filepath.Base(filepath.Dir(path))), nil
}

// parseTemplateContent parses template metadata from markdown content.
func parseTemplateContent(content string, fallbackName string) *Template {
	tmpl := &Template{
		Name:    fallbackName,
		Version: "1.0",
		Content: content,
	}

	// Extract YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			fm := strings.TrimSpace(parts[1])
			body := strings.TrimSpace(parts[2])
			tmpl.Content = body

			var fmData struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
				Type        string `yaml:"type"`
				Version     string `yaml:"version"`
			}
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				if fmData.Name != "" {
					tmpl.Name = fmData.Name
				}
				tmpl.Description = fmData.Description
				tmpl.Type = fmData.Type
				if fmData.Version != "" {
					tmpl.Version = fmData.Version
				}
			}
		}
	}

	// Fallback: extract description from first heading
	if tmpl.Description == "" {
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				tmpl.Description = strings.TrimPrefix(trimmed, "# ")
				if idx := strings.Index(tmpl.Description, "—"); idx > 0 {
					tmpl.Description = strings.TrimSpace(tmpl.Description[:idx])
				}
				break
			}
		}
	}

	return tmpl
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, substr)
}
