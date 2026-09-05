// Package markdown — L356: validação da correção A (agrupamento de parágrafos
// consecutivos, CommonMark) — OLD (flag off) vs NEW (flag on).
//
// Critérios de sucesso (ordem do Don): nenhum conteúdo perdido; fronteiras
// estruturais preservadas; markdown normal sem regressão relevante; JSON
// deixa de fragmentar por linha; redução de fragmentação confirmada.
package markdown

import (
	"strings"
	"testing"
)

// normalizeLine remove marcadores de markdown para verificação de conteúdo
// semântico (o parser normaliza: "# Heading" → "Heading", fences removidos).
func normalizeLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "### ")
	line = strings.TrimPrefix(line, "## ")
	line = strings.TrimPrefix(line, "# ")
	if strings.HasPrefix(line, "```") {
		return "" // fence (com ou sem linguagem) é marcador puro
	}
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimPrefix(line, "+ ")
	line = strings.TrimPrefix(line, "1. ")
	line = strings.TrimPrefix(line, "> ")
	return line
}

func parseWith(t *testing.T, content string, group bool) *Document {
	t.Helper()
	p := NewParser()
	p.GroupConsecutiveParagraphLines = group
	doc, err := p.Parse("", content)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func countParagraphNodes(doc *Document) int {
	n := 0
	for _, c := range doc.AST.Children {
		if c.Type == NodeParagraph {
			n++
		}
	}
	return n
}

func countAllNodes(doc *Document) int {
	return len(doc.AST.Children)
}

// ── Casos de segurança ─────────────────────────────────────────────────────

func TestL356SafetyOLDvsNEW(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"markdown normal", "# Heading\nparagraph line 1\nparagraph line 2\n"},
		{"blank lines", "paragraph A\n\nparagraph B\n"},
		{"heading+text", "# Title\ntext under heading\n"},
		{"code block", "```json\n{\n  \"a\": 1\n}\n```\n"},
		{"lista", "- item 1\n- item 2\n"},
		{"json indentado", "{\n\"a\": {\n\"b\": 1\n}\n}\n"},
		{"json minificado", "{\"a\":{\"b\":1}}\n"},
		{"texto tecnico", "Go 1.26\nv1.2.3\nRAG\nAPI\nSQL\n404\nC++17\n"},
		{"urls", "https://example.com/a/b\n"},
		{"code antes de texto", "```go\nfunc main() {}\n```\nDepois do código.\n"},
		{"texto antes de lista", "Introdução.\n- a\n- b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldDoc := parseWith(t, tc.content, false)
			newDoc := parseWith(t, tc.content, true)

			// 1. NENHUM conteúdo perdido (o texto completo deve estar
			// representado nos nós do NEW).
			allNew := ""
			for _, c := range newDoc.AST.Children {
				allNew += c.Content + "\n"
			}
			// O conteúdo pode ter \n colapsados em " " ou "\n" — verificar
			// que cada linha original está presente em algum nó do NEW.
			for _, line := range strings.Split(tc.content, "\n") {
				norm := normalizeLine(line)
				if norm == "" {
					continue
				}
				if !strings.Contains(allNew, norm) {
					t.Errorf("NEW perdeu a linha %q", line)
				}
			}

			// 2. Fronteiras estruturais preservadas: tipos de nó do OLD
			// devem continuar existindo no NEW (heading/code/list).
			oldTypes := map[NodeType]bool{}
			for _, c := range oldDoc.AST.Children {
				oldTypes[c.Type] = true
			}
			for _, c := range newDoc.AST.Children {
				delete(oldTypes, c.Type)
			}
			for typ := range oldTypes {
				if typ != NodeParagraph { // parágrafos agrupam — esperado
					t.Errorf("NEW perdeu o tipo de nó %q (fronteira destruída)", typ)
				}
			}

			t.Logf("OLD: %d nós (%d parágrafos)  NEW: %d nós (%d parágrafos)",
				countAllNodes(oldDoc), countParagraphNodes(oldDoc),
				countAllNodes(newDoc), countParagraphNodes(newDoc))
		})
	}
}

// ── Caso crítico: JSON 30KB indentado ─────────────────────────────────────

func TestL356CriticalJSON(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("  \"campo\": {\n    \"a\": 1,\n    \"b\": true,\n    \"url\": \"https://x.com/1.0.0\",\n    \"lista\": [1, 2, 3]\n  },\n")
	}
	sb.WriteString("}\n")
	content := sb.String()

	oldDoc := parseWith(t, content, false)
	newDoc := parseWith(t, content, true)
	t.Logf("caso crítico JSON 30KB: OLD=%d nós  NEW=%d nós (esperado ~4: 1 por parágrafo do JSON)",
		countAllNodes(oldDoc), countAllNodes(newDoc))

	// O NEW deve ter POUCOS parágrafos (o JSON inteiro é 1 parágrafo).
	if countParagraphNodes(newDoc) > 5 {
		t.Errorf("NEW fragmentou o JSON: %d parágrafos", countParagraphNodes(newDoc))
	}
	// Conteúdo íntegro: cada linha do JSON presente.
	allNew := ""
	for _, c := range newDoc.AST.Children {
		allNew += c.Content + "\n"
	}
	missing := 0
	for _, line := range strings.Split(content, "\n") {
		norm := normalizeLine(line)
		if norm != "" && !strings.Contains(allNew, norm) {
			missing++
		}
	}
	if missing > 0 {
		t.Errorf("NEW perdeu %d linhas do JSON", missing)
	}
}

// ── Regressão real: amostra de documentos do repositório ──────────────────

func TestL356RealDocs(t *testing.T) {
	// Amostra representativa: markdown normal, markdown com estruturas,
	// JSON indentado grande, JSON minificado, texto técnico.
	docs := map[string]string{
		"md-paragraphs": "# Título\n\nPrimeira linha do parágrafo.\nSegunda linha sem blank line.\nTerceira também.\n\nOutro parágrafo.\n\n## Seção\n\nTexto da seção com mais conteúdo para o teste.\n",
		"md-misto":      "# Guia\n\nTexto introdutório.\nContinuando na mesma linha lógica.\n\n```go\nfunc f() {}\n```\n\n- item 1\n- item 2\n\nFinal com texto.\n",
		"json-indent":   "{\n  \"name\": \"cosca\",\n  \"version\": \"1.26.0\",\n  \"deps\": {\n    \"a\": \"1.0.0\",\n    \"b\": \"2.3.4\"\n  }\n}\n",
		"texto-tecnico": "Go 1.26\nv1.2.3\nRAG\nAPI\nSQL\n404\nC++17\nhttps://example.com/a/b\n",
	}
	t.Logf("── L356 OLD vs NEW (amostra real) ──")
	for name, content := range docs {
		oldDoc := parseWith(t, content, false)
		newDoc := parseWith(t, content, true)
		// conteúdo íntegro no NEW
		allNew := ""
		for _, c := range newDoc.AST.Children {
			allNew += c.Content + "\n"
		}
		missing := 0
		for _, line := range strings.Split(content, "\n") {
			norm := normalizeLine(line)
			if norm != "" && !strings.Contains(allNew, norm) {
				missing++
			}
		}
		// tipos preservados
		oldTypes := map[NodeType]bool{}
		for _, c := range oldDoc.AST.Children {
			oldTypes[c.Type] = true
		}
		loss := []string{}
		for _, c := range newDoc.AST.Children {
			delete(oldTypes, c.Type)
		}
		for typ := range oldTypes {
			if typ != NodeParagraph {
				loss = append(loss, string(typ))
			}
		}
		t.Logf("%-14s OLD=%d nós  NEW=%d nós  missing=%d  tipos-perdidos=%v",
			name, countAllNodes(oldDoc), countAllNodes(newDoc), missing, loss)
		if missing > 0 {
			t.Errorf("%s: %d linhas perdidas no NEW", name, missing)
		}
		if len(loss) > 0 {
			t.Errorf("%s: tipos de nó perdidos no NEW: %v", name, loss)
		}
	}
}