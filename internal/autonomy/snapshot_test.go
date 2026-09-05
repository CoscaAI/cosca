package autonomy

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestUntrackedPaths_Parse — extrai os paths untracked ("?? ") da saída porcelain v1,
// ordena deterministicamente (base do hash de untracked).
func TestUntrackedPaths_Parse(t *testing.T) {
	status := "?? go.mod\n?? dir/new.go\nM  main.go\n?? note.txt\n"
	got := untrackedPaths(status)
	want := []string{"dir/new.go", "go.mod", "note.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("untrackedPaths() = %v, want %v", got, want)
	}
	// Sem untracked → vazio.
	if p := untrackedPaths("M  main.go\nA  go.mod\n"); len(p) != 0 {
		t.Fatalf("esperava vazio, got %v", p)
	}
}

// TestWorkspaceSnapshot_Components — os TRÊS componentes (status/diff/untracked)
// independem entre si: qualquer um mudando torna o snapshot diferente.
func TestWorkspaceSnapshot_Components(t *testing.T) {
	base := WorkspaceSnapshot{Status: "S", Diff: "D", UntrackedHash: "U"}
	for name, alt := range map[string]WorkspaceSnapshot{
		"status": {Status: "S2", Diff: "D", UntrackedHash: "U"},
		"diff":   {Status: "S", Diff: "D2", UntrackedHash: "U"},
		"untrack": {Status: "S", Diff: "D", UntrackedHash: "U2"},
	} {
		if base.Equal(alt) {
			t.Fatalf("mudança em %s deveria tornar o snapshot diferente", name)
		}
	}
}

// TestHashUntrackedPath_Deterministic — o hash de um arquivo untracked é
// determinístico (mesmo conteúdo → mesmo hash), e difere por conteúdo.
func TestHashUntrackedPath_Deterministic(t *testing.T) {
	dir := t.TempDir()
	import_ := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		h, err := hashUntrackedPath(p)
		if err != nil {
			t.Fatalf("hashUntrackedPath(%s): %v", name, err)
		}
		return h
	}
	a1 := import_("a.txt", "hello")
	a2 := import_("a.txt", "hello") // mesmo conteúdo
	b := import_("b.txt", "world")
	if a1 != a2 {
		t.Fatalf("conteúdo igual → hash igual, got %q vs %q", a1, a2)
	}
	if a1 == b {
		t.Fatalf("conteúdo diferente → hash diferente, got %q", a1)
	}
}

// TestWithExcludes_AddsMagicPathspec — WithExcludes adiciona pathspecs mágicos
// `:(exclude)<path>` para o change-detection ignorar artefatos de build que
// oscilam (evita re-executar o gate à toa).
func TestWithExcludes_AddsMagicPathspec(t *testing.T) {
	s := NewGitSnapshotter()
	g, ok := s.(*gitSnapshotter)
	if !ok {
		t.Fatal("NewGitSnapshotter não devolveu *gitSnapshotter")
	}
	ex := WithExcludes(g, "target", "node_modules")
	gx, ok := ex.(*gitSnapshotter)
	if !ok {
		t.Fatal("WithExcludes não devolveu *gitSnapshotter")
	}
	want := []string{".", ":(exclude)target", ":(exclude)node_modules"}
	if !reflect.DeepEqual(gx.Pathspec, want) {
		t.Fatalf("Pathspec = %v, want %v", gx.Pathspec, want)
	}
	args := gx.pathspecArgs()
	if !reflect.DeepEqual(args, []string{"--", ".", ":(exclude)target", ":(exclude)node_modules"}) {
		t.Fatalf("pathspecArgs() = %v", args)
	}
}
