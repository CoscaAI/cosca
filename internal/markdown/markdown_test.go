package markdown

import (
	"testing"
)

func TestNewParser(t *testing.T) {
	t.Parallel()

	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
	if p.MaxSize != 10*1024*1024 {
		t.Errorf("MaxSize = %d, want %d", p.MaxSize, 10*1024*1024)
	}
}

func TestParserDefaults(t *testing.T) {
	t.Parallel()

	p := NewParser()
	if p.IncludeRawHTML != false {
		t.Error("IncludeRawHTML should be false")
	}
}

func TestParseSimpleDocument(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "# Title\n\nSome paragraph content."
	doc, err := p.Parse("/test/file.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title != "Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Title")
	}
	if doc.Path != "/test/file.md" {
		t.Errorf("Path = %q", doc.Path)
	}
	if doc.TokenCount <= 0 {
		t.Errorf("TokenCount = %d, want > 0", doc.TokenCount)
	}
}

func TestParseWithHeadings(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "# Main\n\n## Section 1\n\n### Subsection"
	doc, err := p.Parse("/test.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Headings) != 3 {
		t.Fatalf("got %d headings, want 3", len(doc.Headings))
	}
	if doc.Headings[0].Text != "Main" {
		t.Errorf("heading[0].Text = %q", doc.Headings[0].Text)
	}
}

func TestParseWithCodeBlock(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "# Doc\n\n```go\npackage main\n```"
	doc, err := p.Parse("/test.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.CodeBlocks) != 1 {
		t.Fatalf("got %d code blocks, want 1", len(doc.CodeBlocks))
	}
	if doc.CodeBlocks[0].Language != "go" {
		t.Errorf("language = %q, want go", doc.CodeBlocks[0].Language)
	}
}

func TestParseWithFrontmatter(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "---\ntitle: My Doc\nversion: 1.0\n---\n\n# Content"
	doc, err := p.Parse("/test.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Frontmatter.Data) == 0 {
		t.Fatal("frontmatter data should not be empty")
	}
	if doc.Frontmatter.Data["title"] != "My Doc" {
		t.Errorf("title = %v", doc.Frontmatter.Data["title"])
	}
}

func TestParseEmptyContent(t *testing.T) {
	t.Parallel()

	p := NewParser()
	doc, err := p.Parse("/empty.md", "")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc == nil {
		t.Fatal("doc should not be nil")
	}
}

func TestParseLargeFileError(t *testing.T) {
	t.Parallel()

	p := NewParser()
	p.MaxSize = 10
	content := "this content is definitely more than ten bytes long for sure"
	_, err := p.Parse("/large.md", content)
	if err == nil {
		t.Error("expected error for file too large")
	}
}

func TestCountTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"abcdefgh", 2},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := CountTokens(tc.text)
			if got != tc.want {
				t.Errorf("CountTokens(%q) = %d, want %d", tc.text, got, tc.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Go Language", "go-language"},
		{"  spaces  ", "spaces"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := slugify(tc.input)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetHeadingLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want int
	}{
		{"# Title", 1},
		{"## Section", 2},
		{"### Sub", 3},
		{"#### Subsub", 4},
		{"##### Deep", 5},
		{"###### Deepest", 6},
		{"not a heading", 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := GetHeadingLevel(tc.line)
			if got != tc.want {
				t.Errorf("GetHeadingLevel(%q) = %d, want %d", tc.line, got, tc.want)
			}
		})
	}
}

func TestIsCodeBlockAt(t *testing.T) {
	t.Parallel()

	lines := []string{
		"hello",
		"```go",
		"code",
		"```",
	}

	if IsCodeBlockAt(lines, 0) {
		t.Error("line 0 should not be a code block")
	}
	if !IsCodeBlockAt(lines, 1) {
		t.Error("line 1 should be a code block")
	}
	if IsCodeBlockAt(lines, -1) {
		t.Error("negative index should not be a code block")
	}
}

func TestDocumentSummary(t *testing.T) {
	t.Parallel()

	p := NewParser()
	doc, _ := p.Parse("/test.md", "# Title\n\nContent")
	summary := doc.DocumentSummary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestToJSON(t *testing.T) {
	t.Parallel()

	p := NewParser()
	doc, _ := p.Parse("/test.md", "# Title\n\nContent")
	json, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if json == "" {
		t.Error("JSON should not be empty")
	}
}

func TestParseTable(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "# Test\n\n| Col1 | Col2 |\n| --- | --- |\n| A | B |\n| C | D |"
	doc, err := p.Parse("/test.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(doc.Tables[0].Rows))
	}
}

func TestParseList(t *testing.T) {
	t.Parallel()

	p := NewParser()
	content := "# List\n\n- item1\n- item2\n- item3"
	doc, err := p.Parse("/test.md", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Lists) != 1 {
		t.Fatalf("got %d lists, want 1", len(doc.Lists))
	}
	if len(doc.Lists[0].Items) != 3 {
		t.Errorf("got %d items, want 3", len(doc.Lists[0].Items))
	}
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()

	links := extractLinks("check [this link](https://example.com) out", 0)
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if links[0].Text != "this link" {
		t.Errorf("link text = %q", links[0].Text)
	}
	if links[0].URL != "https://example.com" {
		t.Errorf("URL = %q", links[0].URL)
	}
}

func TestExtractReferences(t *testing.T) {
	t.Parallel()

	refs := extractReferences("[ref1]: https://example.com\n[ref2]: https://test.com \"Title\"")
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2", len(refs))
	}
}

func TestParseSimpleFrontmatter(t *testing.T) {
	t.Parallel()

	data := parseSimpleFrontmatter("key1: value1\nkey2: value2")
	if data["key1"] != "value1" {
		t.Errorf("key1 = %v", data["key1"])
	}
}

func TestParseSimpleTOML(t *testing.T) {
	t.Parallel()

	data := make(map[string]interface{})
	parseSimpleTOML(data, "key = \"value\"")
	if data["key"] != "value" {
		t.Errorf("key = %v", data["key"])
	}
}
