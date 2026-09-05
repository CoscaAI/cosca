package prompts

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	m := &Manager{prompts: make(map[string]*Prompt)}
	if m.prompts == nil {
		t.Error("prompts map should be initialized")
	}
}

func TestAddAndList(t *testing.T) {
	t.Parallel()

	m := &Manager{prompts: make(map[string]*Prompt)}
	m.Add(Prompt{
		Name:        "test-prompt",
		Description: "A test prompt",
		Version:     "1.0",
		Category:    "general",
		Template:    "template text",
	})

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Name != "test-prompt" {
		t.Errorf("Name = %q", list[0].Name)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	m := &Manager{prompts: make(map[string]*Prompt)}
	m.Add(Prompt{Name: "my-prompt"})

	p, err := m.Get("my-prompt")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Name != "my-prompt" {
		t.Errorf("Name = %q", p.Name)
	}

	// Not found
	_, err = m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent prompt")
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	m := &Manager{prompts: make(map[string]*Prompt)}
	m.Add(Prompt{Name: "code-review", Description: "Review code", Category: "review"})
	m.Add(Prompt{Name: "deploy", Description: "Deploy app", Category: "deploy"})

	results, err := m.Search("review")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()

	m := &Manager{prompts: make(map[string]*Prompt)}
	p, err := m.Create(CreateOptions{
		Name:        "new-prompt",
		Description: "New prompt",
		Template:    "template text",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if p.Name != "new-prompt" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Template != "template text" {
		t.Errorf("Template = %q", p.Template)
	}
}

func TestParsePromptFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		content  string
		wantName string
	}{
		{
			name:     "no frontmatter",
			filename: "hello.md",
			content:  "Hello template",
			wantName: "hello",
		},
		{
			name:     "with frontmatter",
			filename: "file.md",
			content:  "---\nname: custom-name\ndescription: desc\nversion: 2.0\ncategory: test\n---\n\nTemplate body",
			wantName: "custom-name",
		},
		{
			name:     "empty content",
			filename: "empty.md",
			content:  "",
			wantName: "empty",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := parsePromptFile(tc.filename, tc.content)
			if p == nil {
				t.Fatal("parsePromptFile returned nil")
			}
			if p.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", p.Name, tc.wantName)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()

	p := &Prompt{}
	parseFrontmatter("name: test\nversion: 1.0", p)
	if p.Name != "test" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Version != "1.0" {
		t.Errorf("Version = %q", p.Version)
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	if !contains("Hello World", "world") {
		t.Error("should match 'world' in 'Hello World'")
	}
	if contains("Hi", "hello") {
		t.Error("should not match")
	}
}
