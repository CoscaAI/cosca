package templates

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	if m.templates == nil {
		t.Error("templates map should be initialized")
	}
}

func TestAddAndList(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	m.Add(Template{
		Name:        "test-template",
		Description: "A test",
		Type:        "skill",
		Version:     "1.0",
	})

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Name != "test-template" {
		t.Errorf("Name = %q", list[0].Name)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	m.Add(Template{Name: "my-template"})

	tmpl, err := m.Get("my-template")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if tmpl.Name != "my-template" {
		t.Errorf("Name = %q", tmpl.Name)
	}

	// Case insensitive
	tmpl, err = m.Get("MY-TEMPLATE")
	if err != nil {
		t.Fatalf("Get (case-insensitive) error: %v", err)
	}
	if tmpl.Name != "my-template" {
		t.Errorf("Name = %q", tmpl.Name)
	}

	// Not found
	_, err = m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	m.Add(Template{Name: "react-app", Description: "React application template", Type: "frontend"})
	m.Add(Template{Name: "go-api", Description: "Go API template", Type: "backend"})

	results, err := m.Search("react")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	tmpl, err := m.Create(CreateOptions{
		Name:        "new-template",
		Type:        "skill",
		Description: "A new template",
		Content:     "template content",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if tmpl.Name != "new-template" {
		t.Errorf("Name = %q", tmpl.Name)
	}
	if tmpl.Type != "skill" {
		t.Errorf("Type = %q", tmpl.Type)
	}
}

func TestCreateEmptyName(t *testing.T) {
	t.Parallel()

	m := &Manager{templates: make(map[string]*Template)}
	_, err := m.Create(CreateOptions{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestParseTemplateContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		content        string
		fallback       string
		wantName       string
		hasFrontmatter bool
	}{
		{
			name:     "no frontmatter",
			content:  "# My Template\n\nContent here",
			fallback: "my-fallback",
			wantName: "my-fallback",
		},
		{
			name: "with frontmatter",
			content: `---
name: custom-name
description: My description
type: skill
version: 2.0
---
# Content`,
			fallback:       "fallback-name",
			wantName:       "custom-name",
			hasFrontmatter: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl := parseTemplateContent(tc.content, tc.fallback)
			if tmpl == nil {
				t.Fatal("parseTemplateContent returned nil")
			}
			if tmpl.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", tmpl.Name, tc.wantName)
			}
		})
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	t.Parallel()

	if !containsIgnoreCase("Hello World", "hello") {
		t.Error("should match 'hello' in 'Hello World'")
	}
	if containsIgnoreCase("Hello World", "xyz") {
		t.Error("should not match 'xyz'")
	}
}

func TestTemplateStruct(t *testing.T) {
	t.Parallel()

	tmpl := Template{
		Name:        "t1",
		Description: "desc",
		Type:        "type1",
		Version:     "1.0",
		Content:     "content",
	}

	if tmpl.Name != "t1" {
		t.Errorf("Name = %q", tmpl.Name)
	}
}
