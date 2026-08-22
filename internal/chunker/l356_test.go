// Package chunker — L356: validação de regressão do pipeline completo após a
// correção A (agrupamento de parágrafos no parser markdown, CommonMark).
// Caso crítico: JSON 30KB indentado — OLD ~1.500 chunks → NEW ~9 chunks.
package chunker

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/markdown"
)

func TestL356CriticalJSONPipeline(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("  \"campo\": {\n    \"a\": 1,\n    \"b\": true,\n    \"url\": \"https://x.com/1.0.0\",\n    \"lista\": [1, 2, 3]\n  },\n")
	}
	sb.WriteString("}\n")
	content := sb.String()

	// OLD: flag desligada (comportamento antigo).
	oldP := markdown.NewParser()
	oldP.GroupConsecutiveParagraphLines = false
	oldDoc, err := oldP.Parse("", content)
	if err != nil {
		t.Fatal(err)
	}
	cOld := New(DefaultConfig())
	oldChunks, err := cOld.ChunkDocument(oldDoc, "d")
	if err != nil {
		t.Fatal(err)
	}
	le25old := 0
	for _, ch := range oldChunks {
		if len(ch.Content) <= 25 {
			le25old++
		}
	}

	// NEW: flag ligada (default do parser).
	newDoc, err := markdown.NewParser().Parse("", content)
	if err != nil {
		t.Fatal(err)
	}
	cNew := New(DefaultConfig())
	newChunks, err := cNew.ChunkDocument(newDoc, "d")
	if err != nil {
		t.Fatal(err)
	}
	le25new := 0
	for _, ch := range newChunks {
		if len(ch.Content) <= 25 {
			le25new++
		}
	}
	t.Logf("CHUNKER caso crítico JSON 30KB: OLD=%d chunks (≤25:%d)  NEW=%d chunks (≤25:%d)",
		len(oldChunks), le25old, len(newChunks), le25new)

	// Critério (L355): NEW ≈ 9 chunks, sem fragmentos ≤25.
	if len(newChunks) > 20 {
		t.Errorf("NEW ainda fragmenta: %d chunks (esperado ~9)", len(newChunks))
	}
	if le25new > 0 {
		t.Errorf("NEW ainda tem %d fragmentos ≤25 chars", le25new)
	}
	// Redução de fragmentação confirmada.
	if len(newChunks) >= len(oldChunks) {
		t.Errorf("sem redução: NEW=%d >= OLD=%d", len(newChunks), len(oldChunks))
	}
}

// TestL356MarkdownNoRegression — markdown normal não pode regredir.
func TestL356MarkdownNoRegression(t *testing.T) {
	content := "# Título\n\nParágrafo um com conteúdo real.\n\n## Seção\n\nTexto da seção.\n\n- item 1\n- item 2\n\n```go\nfunc f() {}\n```\n\nFinal.\n"
	doc, err := markdown.NewParser().Parse("", content)
	if err != nil {
		t.Fatal(err)
	}
	c := New(DefaultConfig())
	chunks, err := c.ChunkDocument(doc, "d")
	if err != nil {
		t.Fatal(err)
	}
	// Estruturas preservadas: 1 heading, 1 code, 1 lista.
	headings, codes, lists := 0, 0, 0
	for _, ch := range chunks {
		switch ch.SectionType {
		case "heading":
			headings++
		case "code":
			codes++
		case "list":
			lists++
		}
	}
	if headings < 1 || codes < 1 || lists < 1 {
		t.Errorf("fronteiras perdidas: h=%d c=%d l=%d", headings, codes, lists)
	}
	// Nenhum conteúdo essencial perdido.
	all := ""
	for _, ch := range chunks {
		all += ch.Content + "\n"
	}
	for _, keep := range []string{"Título", "Parágrafo um", "Texto da seção", "item 1", "func f() {}"} {
		if !strings.Contains(all, keep) {
			t.Errorf("conteúdo perdido: %q", keep)
		}
	}
	t.Logf("markdown normal: %d chunks (h=%d c=%d l=%d) — sem regressão", len(chunks), headings, codes, lists)
}