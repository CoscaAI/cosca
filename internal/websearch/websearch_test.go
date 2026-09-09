package websearch

import (
	"context"
	"strings"
	"testing"
)

// ── stripHTML (puro, unit-testável) ────────────────────────────────────────

func TestStripHTML(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"removes tags", "<p>oi <b>chefe</b></p>", "oi chefe"},
		{"no tags", "texto limpo", "texto limpo"},
		{"keeps internal spaces", " a \n b \t c ", "a \n b \t c"},
		{"strips entities none", "&amp;", "&amp;"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripHTML(c.in)
			// trimSnippet não é aplicado aqui; stripHTML só remove tags.
			if got != c.want {
				t.Fatalf("stripHTML(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStripHTMLNested(t *testing.T) {
	in := `<div class="x"><a href="/y">link</a> e <span>mais</span></div>`
	want := "link e mais"
	if got := stripHTML(in); got != want {
		t.Fatalf("stripHTML nested = %q, want %q", got, want)
	}
}

// ── trimSnippet (puro) ─────────────────────────────────────────────────────

func TestTrimSnippet(t *testing.T) {
	if got := trimSnippet("um dois tres", 5); !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis on truncation, got %q", got)
	}
	if got := trimSnippet("curto", 10); got != "curto" {
		t.Fatalf("short text should be unchanged, got %q", got)
	}
	if got := trimSnippet("a  b\n  c", 20); got != "a b c" {
		t.Fatalf("whitespace not normalized: %q", got)
	}
}

// ── relevanceScore (puro) ─────────────────────────────────────────────────

func TestRelevanceScore(t *testing.T) {
	q := "busca web"
	titleHit := Result{Title: "como fazer busca web em go", Snippet: "texto"}
	snippetOnly := Result{Title: "outro", Snippet: "fala de busca web"}
	noHit := Result{Title: "nada relacionado", Snippet: "blah"}

	if relevanceScore(titleHit, q) <= relevanceScore(snippetOnly, q) {
		t.Fatalf("title hit should outrank snippet-only hit")
	}
	if relevanceScore(snippetOnly, q) <= relevanceScore(noHit, q) {
		t.Fatalf("snippet hit should outrank no hit")
	}
}

// ── parseRSS (puro, offline) ──────────────────────────────────────────────

func TestParseRSS(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
  <item>
    <title>Notícia 1</title>
    <description><![CDATA[<p>conteúdo</p>]]></description>
    <link>https://ex.com/1</link>
  </item>
  <item>
    <title>Notícia 2</title>
    <description>segunda</description>
    <link>https://ex.com/2</link>
  </item>
</channel></rss>`)

	results, err := parseRSS(xmlData, "G1", 10)
	if err != nil {
		t.Fatalf("parseRSS: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Title != "Notícia 1" {
		t.Fatalf("title = %q", results[0].Title)
	}
	if strings.Contains(results[0].Snippet, "<p>") {
		t.Fatalf("snippet should have HTML stripped: %q", results[0].Snippet)
	}
	if results[0].Source != "G1" {
		t.Fatalf("source = %q", results[0].Source)
	}
}

func TestParseRSSLimit(t *testing.T) {
	xmlData := []byte(`<rss version="2.0"><channel>
	<item><title>a</title><description>d</description><link>l</link></item>
	<item><title>b</title><description>d</description><link>l</link></item>
	<item><title>c</title><description>d</description><link>l</link></item>
	</channel></rss>`)
	results, err := parseRSS(xmlData, "src", 2)
	if err != nil {
		t.Fatalf("parseRSS: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("limit not respected: got %d", len(results))
	}
}

// ── enquadramento de provider (sem rede): New* retorna não-nil ─────────────

func TestProvidersConstruct(t *testing.T) {
	if NewNewsProvider() == nil {
		t.Fatal("news provider nil")
	}
	if NewWikiProvider() == nil {
		t.Fatal("wiki provider nil")
	}
	if NewHNProvider() == nil {
		t.Fatal("hn provider nil")
	}
	if NewGitHubProvider("") == nil || NewGitHubProvider("repos") == nil {
		t.Fatal("github provider nil")
	}
	p := NewWikiProvider().WithLang("")
	if p.lang != "pt" {
		t.Fatalf("empty lang should default to pt, got %q", p.lang)
	}
	if NewWikiProvider().WithLang("en").lang != "en" {
		t.Fatal("WithLang en should stick")
	}
}

// ── smoke test com rede (gated — só roda se a rede estiver disponível) ─────
// Estes são marcados como testes de integração: não devem rodar em CI offline.
// Usamos um flag de short para permitir `go test -short` pular.

func TestWikiLive(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	ctx := context.Background()
	results, err := NewWikiProvider().Search(ctx, "programação", 3)
	if err != nil {
		t.Skipf("wiki indisponível: %v", err)
	}
	if len(results) == 0 {
		t.Skipf("wiki sem resultados")
	}
	if results[0].Source == "" {
		t.Fatalf("wiki result missing source")
	}
}
