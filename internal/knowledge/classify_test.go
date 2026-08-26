package knowledge

import (
	"testing"
)

// ── (a) ADR / decisão / learning → persistente ───────────────────────────────

func TestClassifyDoc_ADR_FrontmatterType_Persistent(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc("/docs/adr/0001-usar-sqlite.md", map[string]any{"type": "adr"}, "ADR content", "")
	if !cls.Persistent {
		t.Fatalf("ADR por frontmatter type deveria ser persistente, got %+v", cls)
	}
	if cls.Kind != KindADR {
		t.Fatalf("kind should be %q, got %q", KindADR, cls.Kind)
	}
	if cls.Confidence < minClassifyConfidence {
		t.Fatalf("confidence %v should be >= %v", cls.Confidence, minClassifyConfidence)
	}
}

func TestClassifyDoc_Decision_ContentHeading_Persistent(t *testing.T) {
	t.Parallel()
	content := "# Decision\n\nWe choose the SQLite-backed store.\n\n| Status | Accepted |\n"
	cls := ClassifyDoc("/misc/nota.md", map[string]any{}, content, "")
	if !cls.Persistent {
		t.Fatalf("decisão por conteúdo deveria ser persistente, got %+v", cls)
	}
	if cls.Kind != KindDecision {
		t.Fatalf("kind should be %q, got %q", KindDecision, cls.Kind)
	}
}

func TestClassifyDoc_Learning_BlockFormat_Persistent(t *testing.T) {
	t.Parallel()
	// Formato real de um learning block (register.go buildBlock).
	content := "PREV: 000\nID: L17\nLEVEL: 4\n---\n## L17 | 2026-08-25 | Descoberta | Level 4\n\n" +
		"| Field | Value |\n|-------|-------|\n| **Learned** | achado com valor futuro |\n| **Outcome** | success |\n"
	cls := ClassifyDoc("/misc/block.md", map[string]any{}, content, "")
	if !cls.Persistent {
		t.Fatalf("learning block deveria ser persistente, got %+v", cls)
	}
	if cls.Kind != KindLearning {
		t.Fatalf("kind should be %q, got %q", KindLearning, cls.Kind)
	}
}

func TestClassifyDoc_Learning_PathToken_Persistent(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc(".opencode/cosca/memory/agent/cosca-uiux/learnings.md", nil, "conteúdo validado por teste", "cosca-uiux")
	if !cls.Persistent {
		t.Fatalf("aprendizado em diretório learnings deveria ser persistente, got %+v", cls)
	}
	if cls.Kind != KindLearning {
		t.Fatalf("kind should be %q, got %q", KindLearning, cls.Kind)
	}
}

// ── (b) session / log / gold-test / backup → NÃO persistente ────────────────

func TestClassifyDoc_TransientPaths_NotPersistent(t *testing.T) {
	t.Parallel()
	cases := []string{
		"/proj/.opencode/cosca/memory/agent/x/session/abc.md",
		"/proj/docs/logs/out.md",
		"/proj/x/gold-test/case.md",
		"/proj/backups/data.md",
		"/proj/memory/agent/x/traces/t1.md",
		"/proj/tmp/scratch.md",
		"/proj/testdata/fix.md",
		"/proj/snapshots/before.md",
	}
	for _, p := range cases {
		cls := ClassifyDoc(p, map[string]any{"type": "learning"}, "# Decision\n\n| **Learned** | x |", "")
		if cls.Persistent {
			t.Errorf("path %q deveria ser NÃO persistente (transiente), got %+v", p, cls)
		}
	}
}

func TestClassifyDoc_TransientContent_NotPersistent(t *testing.T) {
	t.Parallel()
	corpus := []string{
		"2026-08-25T10:11:12Z INFO start\n2026-08-25T10:11:13Z INFO read\n2026-08-25T10:11:14Z INFO done\n",
		"=== RUN TestFoo\n--- PASS: TestFoo (0.01s)\n",
		"[INFO] boot\n[DEBUG] cfg\n",
		"\x1b[31mERRO\x1b[0m boot\n",
	}
	for _, content := range corpus {
		cls := ClassifyDoc("/misc/x.md", map[string]any{}, content, "")
		if cls.Persistent {
			t.Errorf("conteúdo transitório deveria ser NÃO persistente, got %+v", cls)
		}
	}
}

func TestClassifyDoc_TransientType_NotPersistent(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"session", "log", "trace", "test", "fixture"} {
		cls := ClassifyDoc("/docs/x.md", map[string]any{"type": typ}, "any content", "")
		if cls.Persistent {
			t.Errorf("type %q deveria ser NÃO persistente, got %+v", typ, cls)
		}
	}
}

// ── (c) conteúdo ambíguo / baixa confiança → NÃO persistente (fail-closed) ───

func TestClassifyDoc_Ambiguous_NotPersistent_FailClosed(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc("/misc/notas.md", map[string]any{}, "hello world sem sinal de conhecimento consolidado", "")
	if cls.Persistent {
		t.Fatalf("conteúdo ambíguo deveria ser NÃO persistente (fail-closed), got %+v", cls)
	}
}

func TestClassifyDoc_Ambiguous_ConfidenceBelowThreshold(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc("/misc/x.md", map[string]any{}, "texto aleatório que não casa com nenhum marcador de conhecimento", "")
	if cls.Persistent {
		t.Fatal("candidato não classificado deve cair em fail-closed")
	}
	if cls.Confidence >= minClassifyConfidence {
		t.Fatalf("confidence %v should be below threshold %v", cls.Confidence, minClassifyConfidence)
	}
}

// ── (d) origem projeto → scope=project; origem embed → scope=global ──────────

func TestClassifyDoc_ScopeProject(t *testing.T) {
	t.Parallel()
	cases := []struct{ path, content string }{
		{".opencode/cosca/memory/agent/cosca-uiux/learnings.md", "## L2 | ... | lvl | x\n"},
		{"docs/adr/0001-teste.md", "# ADR-001\n"},
		{"projeto-do-cliente/docs/decision.md", "# Decision\n"},
	}
	for _, c := range cases {
		cls := ClassifyDoc(c.path, map[string]any{}, c.content, "")
		if cls.Scope != ScopeProject {
			t.Errorf("path %q deveria ter scope=project, got %q", c.path, cls.Scope)
		}
	}
}

func TestClassifyDoc_ScopeGlobal_Embed(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc("internal/embed/cosca/knowledge/foo.md", map[string]any{}, "# Decision\n\nDecição embarcada", "")
	if cls.Scope != ScopeGlobal {
		t.Fatalf("path embarcado deveria ter scope=global, got %q", cls.Scope)
	}
}

// `.cosca/fallback/**` é a cópia materializada do cérebro embarcado
// (MaterializeFallback copia internal/embed/cosca/** para .cosca/fallback/**).
// Logo é CONTEÚDO DE FRAMEWORK/global, não conhecimento único do projeto —
// tratá-lo como project poluiria o escopo do projeto com o framework.
func TestClassifyDoc_ScopeGlobal_Fallback(t *testing.T) {
	t.Parallel()
	cls := ClassifyDoc(".cosca/fallback/memory/agent/cosca-kernel/learnings.md", map[string]any{}, "## L3 | ... | lvl | x\n", "")
	if cls.Scope != ScopeGlobal {
		t.Fatalf("copía embarcada (.cosca/fallback) deveria ter scope=global, got %q", cls.Scope)
	}
}

// ── (e) NUNCA scope=global para origem de projeto ────────────────────────────

func TestClassifyDoc_NeverGlobalForProjectOrigin(t *testing.T) {
	t.Parallel()
	// Mesmo com conteúdo/type que lembrem conhecimento "global", a origem de
	// projeto NUNCA vira scope=global (regra de ouro).
	cases := []struct {
		path string
		fm   map[string]any
	}{
		{".opencode/cosca/memory/agent/x/learnings.md", map[string]any{"type": "learning"}},
		{"docs/decision.md", map[string]any{"type": "decision"}},
		{"docs/architecture/x.md", map[string]any{"type": "architecture"}},
	}
	for _, c := range cases {
		cls := ClassifyDoc(c.path, c.fm, "# Decision\n\nconteúdo", "")
		if cls.Scope == ScopeGlobal {
			t.Errorf("origem de projeto %q NUNCA deve ter scope=global, got %+v", c.path, cls)
		}
	}
}
