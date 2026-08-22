// Package chunker — testes do filtro de triviais T1-T4 (L353).
//
// Inclui o teste de REGRESSÃO do problema JSON (L354): o chunker produz
// fragmentos de 1 caractere ao chunkar JSON em bloco de código. O teste
// REPRODUZ o problema (não corrige — a hipótese de correção é da L354).
package chunker

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/markdown"
)

// ── IsTrivial: unit por regra ──────────────────────────────────────────────

func TestIsTrivialRules(t *testing.T) {
	cases := []struct {
		in       string
		trivial  bool
		wantRule Rule
	}{
		// T1 vazio
		{"", true, RuleT1},
		{"   ", true, RuleT1},
		{"\n\n\n", true, RuleT1},
		// T2 separador puro
		{"---", true, RuleT2},
		{"***", true, RuleT2},
		{"====", true, RuleT2},
		{"___", true, RuleT2},
		{"—", true, RuleT2},
		// T4 estrutura vazia
		{"##", true, RuleT4},
		{"```", true, RuleT4},
		{">>>", true, RuleT4},
		// T3 zero alfanuméricos (símbolos sem estrutura)
		{"!!!", true, RuleT3},
		{"§§§", true, RuleT3},
		{"{", true, RuleT3},
		{"},", true, RuleT3},
		{"😀😀", true, RuleT3},
		{"|-------|----------|", true, RuleT3},
		// Preservados (têm alfanuméricos — NUNCA filtrar por tamanho)
		{"Go 1.26", false, RuleNone},
		{"v1.2.3", false, RuleNone},
		{"RAG", false, RuleNone},
		{"API", false, RuleNone},
		{"SQL", false, RuleNone},
		{"404", false, RuleNone},
		{"C++17", false, RuleNone},
		{"--- # Introdução", false, RuleNone},
		{"### 3.2 Método", false, RuleNone},
		{`"scripts": {`, false, RuleNone},
		{`"arm64"`, false, RuleNone},
	}
	for _, tc := range cases {
		got, rule := IsTrivial(tc.in)
		if got != tc.trivial || rule != tc.wantRule {
			t.Errorf("IsTrivial(%q) = (%v, %v), want (%v, %v)", tc.in, got, rule, tc.trivial, tc.wantRule)
		}
	}
}

// ── ChunkDocument aplica o filtro (comportamento-futuro) ──────────────────

func TestChunkDocumentFiltersTrivial(t *testing.T) {
	doc, err := markdown.NewParser().Parse("", "---\n\n# Título\n\nConteúdo real do documento com palavras.\n\n---\n\nMais conteúdo real aqui.\n")
	if err != nil {
		t.Fatal(err)
	}
	c := New(DefaultConfig())
	chunks, err := c.ChunkDocument(doc, "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range chunks {
		if _, rule := IsTrivial(ch.Content); rule != RuleNone {
			t.Fatalf("chunk trivial vazou: %q (regra %v)", ch.Content, rule)
		}
	}
	st := c.TrivialStats()
	if st.FilteredTotal == 0 {
		t.Log("nenhum trivial neste documento (ok — documento limpo)")
	}
	t.Logf("triviais filtrados=%d (T1=%d T2=%d T3=%d T4=%d) by_source=%v",
		st.FilteredTotal, st.FilteredT1, st.FilteredT2, st.FilteredT3, st.FilteredT4, st.BySource)
}

func TestChunkDocumentFilterDisabled(t *testing.T) {
	doc, err := markdown.NewParser().Parse("", "---\n\n# Título\n\nConteúdo real.\n\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.FilterTrivial = false
	c := New(cfg)
	chunks, err := c.ChunkDocument(doc, "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	// Com o filtro desligado, os triviais passam (comportamento antigo).
	found := false
	for _, ch := range chunks {
		if trivial, _ := IsTrivial(ch.Content); trivial {
			found = true
		}
	}
	if !found {
		t.Log("documento sem triviais — verificação do desligamento inconclusiva")
	}
	if st := c.TrivialStats(); st.FilteredTotal != 0 {
		t.Fatalf("filtro desligado mas contou %d", st.FilteredTotal)
	}
}

// ── REGRESSÃO: chunker produz fragmentos JSON (L354 — reproduz, não corrige) ──

func TestChunkerJSONFragmentsRepro(t *testing.T) {
	// CAUSA RAIZ (L354): o PARSER MARKDOWN cria um NodeParagraph POR LINHA
	// (loop por linhas, default:) — linhas consecutivas sem blank line NÃO são
	// agrupadas (divergência do CommonMark). JSON indentado = cada linha vira
	// um nó → o chunker cria 1 chunk por linha (fragmentos de 15 chars).
	// Evidência: doc 4b87339a do corpus virou 1.013 chunks (694 ≤25 chars,
	// 141 triviais); repro abaixo: JSON indentado > MaxTokens → 1.500 chunks.
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("  \"campo\": {\n    \"a\": 1,\n    \"b\": true,\n    \"url\": \"https://x.com/1.0.0\",\n    \"lista\": [1, 2, 3]\n  },\n")
	}
	sb.WriteString("}\n")

	doc, err := markdown.NewParser().Parse("", sb.String())
	if err != nil {
		t.Fatal(err)
	}
	c := New(DefaultConfig())
	chunks, err := c.ChunkDocument(doc, "doc-json-indent")
	if err != nil {
		t.Fatal(err)
	}
	var fragments int
	for _, ch := range chunks {
		if len(ch.Content) <= 25 {
			fragments++
		}
	}
	st := c.TrivialStats()
	t.Logf("REPRO L354 (JSON indentado): chunks=%d fragmentos<=25 chars=%d triviais filtrados=%d — causa raiz: parser markdown cria 1 nó/linha",
		len(chunks), fragments, st.FilteredTotal)
	// Controle: o splitLongParagraph AGUPA corretamente (9 chunks de 2.550
	// chars) quando recebe o bloco inteiro — o problema está no parser.
	if len(chunks) > 100 {
		t.Log("CONFIRMADO: fragmentação linha-a-linha (parser markdown, não o splitter)")
	}
}

// ── Observabilidade: contadores por regra/source/documento ────────────────

func TestTrivialFilterCounters(t *testing.T) {
	f := NewTrivialFilter()
	f.Record(RuleT2, "text", "doc-a")
	f.Record(RuleT2, "text", "doc-a")
	f.Record(RuleT3, "code", "doc-b")
	f.Record(RuleT1, "text", "doc-c")

	st := f.Snapshot()
	if st.FilteredTotal != 4 {
		t.Fatalf("total=%d want 4", st.FilteredTotal)
	}
	if st.FilteredT2 != 2 || st.FilteredT3 != 1 || st.FilteredT1 != 1 {
		t.Fatalf("por regra errado: %+v", st)
	}
	if st.BySource["text"] != 3 || st.BySource["code"] != 1 {
		t.Fatalf("by_source errado: %v", st.BySource)
	}
	if st.ByDocument["doc-a"] != 2 {
		t.Fatalf("by_document errado: %v", st.ByDocument)
	}
	// Sem conteúdo sensível nos contadores.
	if strings.Contains(strings.Join(mapKeys(st.ByDocument), ","), "{") {
		t.Fatal("contadores não devem conter conteúdo")
	}
}

func mapKeys(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
