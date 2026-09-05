package gitmgr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// helper: cria um repo git temporário com um commit.
func newTestRepo(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	// Inicializa o repo go-git
	initRepo(t, dir)
	m, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return m, dir
}

// TestDestructiveGateFailClosed valida a regra de OURO: uma operação
// destrutiva SEM autorização explícita é BLOQUEADA (retorna erro e NÃO executa).
func TestDestructiveGateFailClosed(t *testing.T) {
	m, _ := newTestRepo(t)
	ctx := context.Background()

	// ResetHard sem gate → deve falhar (fail-closed).
	if err := m.ResetHard(ctx, "HEAD", nil); err == nil {
		t.Fatal("ResetHard sem gate deveria ser bloqueado")
	} else {
		var de *DestructiveGateError
		if !errorsAs(err, &de) {
			t.Fatalf("esperava DestructiveGateError, got %T: %v", err, err)
		}
	}

	// DeleteBranch sem gate → deve falhar.
	if err := m.DeleteBranch(ctx, "foo", nil); err == nil {
		t.Fatal("DeleteBranch sem gate deveria ser bloqueado")
	}
}

// TestDestructiveGateWithAuthorization valida que COM autorização explícita
// a operação destrutiva pode prosseguir (quando bem-formada).
func TestDestructiveGateWithAuthorization(t *testing.T) {
	m, dir := newTestRepo(t)
	ctx := context.Background()

	// Cria um arquivo não-commitado e um branch para testar o delete.
	_ = os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("x"), 0o644)
	if err := m.CreateBranch(ctx, "feature-x"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	gate := &DestructiveGate{Authorized: true, Reason: "Don aprovou"}
	if err := m.DeleteBranch(ctx, "feature-x", gate); err != nil {
		t.Fatalf("DeleteBranch com gate deveria funcionar: %v", err)
	}
}

// TestProvenanceWrite valida que o registro de proveniência é gravado como
// JSON em .cosca/provenance/.
func TestProvenanceWrite(t *testing.T) {
	m, _ := newTestRepo(t)
	ctx := context.Background()

	p := Provenance{
		CampaignID: "campaign-002",
		Dataset:    "dataset-train-002.jsonl",
		LoRA:       "cosca-qwen3-4b-lora-002",
		Base:       "Qwen/Qwen3-4B",
		Model:      "qwen3:4b",
		Repo:       "cosca",
		Commit:     "abc1234",
		Branch:     "main",
		Actor:      "cosca-kernel",
		Op:         "train",
		GoldenResult: "0.88 -> 0.94",
		Promotion:  "PASS",
	}
	path, err := m.WriteProvenance(ctx, p)
	if err != nil {
		t.Fatalf("WriteProvenance: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("provenance não criado: %v", err)
	}
	if filepath.Base(path) != "campaign-002.json" {
		t.Fatalf("nome esperado campaign-002.json, got %s", filepath.Base(path))
	}
	t.Logf("provenance escrito em %s", path)
}
