package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat/sandbox"
)

// ── P0-3: Security boundary — path traversal + symlink escape ──────────
//
// Usa o Rails REAL (resolve symlinks via EvalSymlinks) — não o mock. Verifica
// que os tools respeitam o validator quando o workspace contém armadilhas:
// symlink para fora, path traversal com "..", diretório bloqueado (.git).

func newWorkspaceWithSymlink(t *testing.T) (workspace, outside string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires unix")
	}
	workspace = t.TempDir()
	outside = t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Symlink DENTRO do workspace apontando para fora.
	if err := os.Symlink(secret, filepath.Join(workspace, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	return workspace, outside
}

func TestSecurity_ReadToolBlocksSymlinkEscape(t *testing.T) {
	ws, _ := newWorkspaceWithSymlink(t)
	rails := sandbox.NewRails(ws)
	rt := NewReadTool(ws, rails)

	// O Rails resolve o symlink e detecta que o alvo está FORA do workspace.
	res, err := rt.Execute(context.Background(), json.RawMessage(`{"path":"escape.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" || strings.Contains(res.Output, "TOP SECRET") {
		t.Fatalf("symlink escape not blocked: error=%q output=%q", res.Error, res.Output)
	}
}

func TestSecurity_ReadToolBlocksPathTraversal(t *testing.T) {
	ws := t.TempDir()
	// Arquivo fora do workspace.
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("S"), 0o644)

	rails := sandbox.NewRails(ws)
	rt := NewReadTool(ws, rails)

	res, err := rt.Execute(context.Background(), json.RawMessage(`{"path":"../secret.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Fatal("path traversal not blocked")
	}
}

func TestSecurity_GlobToolBlocksEscape(t *testing.T) {
	ws := t.TempDir()
	_ = os.WriteFile(filepath.Join(ws, "ok.go"), []byte("x"), 0o644)

	rails := sandbox.NewRails(ws)
	gt := NewGlobTool(ws, rails)

	// Pattern que tenta escapar do workspace.
	res, err := gt.Execute(context.Background(), json.RawMessage(`{"pattern":"../../etc/*"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" || strings.Contains(res.Output, "etc") {
		t.Fatalf("glob escape not blocked: %q", res.Output)
	}

	// Pattern absoluto também é rejeitado. No Windows, "/etc/passwd" não é um
	// path absoluto (falta o drive), então a semântica POSIX de path absoluto
	// não se aplica — a asserção vale apenas em sistemas Unix.
	if runtime.GOOS != "windows" {
		res, _ = gt.Execute(context.Background(), json.RawMessage(`{"pattern":"/etc/passwd"}`))
		if res.Error == "" {
			t.Fatal("absolute pattern not blocked")
		}
	}
}

func TestSecurity_ReadToolBlocksBlockedDir(t *testing.T) {
	ws := t.TempDir()
	// .git é bloqueado pelo Rails (IsBlocked).
	_ = os.MkdirAll(filepath.Join(ws, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(ws, ".git", "config"), []byte("x"), 0o644)

	rails := sandbox.NewRails(ws)
	rt := NewReadTool(ws, rails)

	res, err := rt.Execute(context.Background(), json.RawMessage(`{"path":".git/config"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Fatal("blocked directory not rejected")
	}
}

// TestSecurity_ValidPathStillWorks é o controle: com Rails real, um caminho
// legítimo continua funcionando (o watchdog de segurança não quebra o uso).
func TestSecurity_ValidPathStillWorks(t *testing.T) {
	ws := t.TempDir()
	_ = os.WriteFile(filepath.Join(ws, "readme.md"), []byte("hello"), 0o644)

	rails := sandbox.NewRails(ws)
	rt := NewReadTool(ws, rails)

	res, err := rt.Execute(context.Background(), json.RawMessage(`{"path":"readme.md"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" || !strings.Contains(res.Output, "hello") {
		t.Fatalf("valid path broken: error=%q output=%q", res.Error, res.Output)
	}
}
