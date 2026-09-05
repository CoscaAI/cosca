package autonomy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// haveGit salta o teste quando git não está no PATH (o change-detection real
// precisa de git — é o escopo "git/workspace real").
func haveGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH — teste de integração do snapshot precisa de git")
	}
}

// TestGitSnapshotter_RealChangeDetection — o snapshotter REAL (git) detecta:
// workspace inalterado → snapshot IGUAL; alteração tracked (diff) ou untracked →
// snapshot DIFERENTE. É a base do "gate não re-executa quando nada mudou".
func TestGitSnapshotter_RealChangeDetection(t *testing.T) {
	haveGit(t)
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "cosca@local")
	run("config", "user.name", "cosca")
	f := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	f("main.go", "package main\n")
	f("go.mod", "module x\n")
	run("add", ".")
	run("commit", "-q", "-m", "init")

	ctx := context.Background()
	s := NewGitSnapshotter()

	// Linha de base.
	snapA, err := s.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	// Workspace INALTERADO → snapshot IGUAL.
	snapB, err := s.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture inalterado: %v", err)
	}
	if !snapA.Equal(snapB) {
		t.Fatalf("workspace inalterado deveria dar snapshot igual\nA=%+v\nB=%+v", snapA, snapB)
	}

	// Arquivo TRACKED mudou (diff) → snapshot DIFERENTE.
	f("main.go", "package main\n// mudou\n")
	snapC, err := s.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture tracked-changed: %v", err)
	}
	if snapA.Equal(snapC) {
		t.Fatalf("diff deveria tornar o snapshot diferente")
	}

	// Arquivo UNTRACKED criado → snapshot DIFERENTE.
	f("note.txt", "todo\n")
	snapD, err := s.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture untracked: %v", err)
	}
	if snapA.Equal(snapD) {
		t.Fatalf("untracked deveria tornar o snapshot diferente")
	}
}

// TestGitSnapshotter_WithExcludes_IgnoresExcluded — um diretório EXCLUÍDO
// (ex.: target/) não entra no change-detection: tocar um arquivo dentro dele
// NÃO muda o snapshot (o gate não re-executa por artefato de build).
func TestGitSnapshotter_WithExcludes_IgnoresExcluded(t *testing.T) {
	haveGit(t)
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "cosca@local")
	run("config", "user.name", "cosca")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target", "build.log"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")

	ctx := context.Background()
	s := NewGitSnapshotter()
	g, ok := s.(*gitSnapshotter)
	if !ok {
		t.Fatal("tipo inesperado")
	}
	scoped := WithExcludes(g, "target")

	base, err := scoped.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture base: %v", err)
	}
	// Mudar arquivo DENTRO do diretório excluído → snapshot IGUAL.
	if err := os.WriteFile(filepath.Join(dir, "target", "build.log"), []byte("v2 changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := scoped.Capture(ctx, dir)
	if err != nil {
		t.Fatalf("capture after: %v", err)
	}
	if !base.Equal(after) {
		t.Fatalf("mudança em diretório EXCLUÍDO não deveria mudar o snapshot\nbase=%+v\nafter=%+v", base, after)
	}
}
