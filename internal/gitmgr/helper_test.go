package gitmgr

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initRepo cria um repositório git vazio com um commit inicial (necessário
// para as operações de branch/reset funcionarem).
func initRepo(t *testing.T, dir string) {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	// Escreve um arquivo inicial (o repo vazio não pode commitar working tree
	// limpo) e adiciona + commita.
	if err := os.WriteFile(filepath.Join(dir, "inicial.txt"), []byte("inicial\n"), 0o644); err != nil {
		t.Fatalf("write inicial: %v", err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@cosca"},
	}); err != nil {
		t.Fatalf("commit inicial: %v", err)
	}
}

// errorsAs é um wrapper de errors.As para os testes.
func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}
