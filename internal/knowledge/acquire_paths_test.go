package knowledge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquirePackagePaths(t *testing.T) {
	kbDir := t.TempDir()
	dir := filepath.Join(kbDir, "pkgs")
	s := NewPackageStore(dir)

	ctx := context.Background()

	// 1. Pacote inexistente → erro.
	if _, err := AcquirePackage(ctx, s, "ghost-pkg", kbDir, true, false); err == nil {
		t.Fatal("pacote inexistente deve dar erro")
	}

	// 2. Pacote sem repository → erro claro.
	if err := s.Add(KnowledgePackage{ID: "no-repo", Kind: PackageKindLibrary, Ecosystem: "go", Status: "none"}); err != nil {
		t.Fatalf("Add no-repo: %v", err)
	}
	if _, err := AcquirePackage(ctx, s, "no-repo", kbDir, true, false); err == nil {
		t.Fatal("pacote sem repository deve dar erro")
	} else if !strings.Contains(err.Error(), "no repository set") {
		t.Fatalf("erro = %v", err)
	}

	// 3. Pacote já adquirido → already_acquired (sem rede).
	if err := s.Add(KnowledgePackage{ID: "done", Kind: PackageKindLibrary, Ecosystem: "go", Status: PackageStatusValidated}); err != nil {
		t.Fatalf("Add done: %v", err)
	}
	res, err := AcquirePackage(ctx, s, "done", kbDir, true, false)
	if err != nil {
		t.Fatalf("already-acquired: %v", err)
	}
	if res.Status != "already_acquired" {
		t.Fatalf("status = %q", res.Status)
	}
}

func TestVerifyGitHubRepoMalformed(t *testing.T) {
	// Repo sem "/" → erro imediato, SEM rede.
	if _, err := verifyGitHubRepo(context.Background(), "not-a-repo"); err == nil {
		t.Fatal("repo malformado deve dar erro sem rede")
	}
}
