package gitmgr

import (
	"context"
	"errors"
	"fmt"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// ─── Separação de operações (professor) ────────────────────────────────────
//
// O COSCA Git Manager separa as operações em três classes:
//
//   READ       — leitura/informação. Sempre permitidas (status, log, diff,
//                inspect, brances, current branch).
//
//   WRITE      — escrita não-destrutiva. Exigem que o caller passe pelo
//                gate de confirmação do Don (via CLI), mas NÃO apagam nada.
//                (branch create, commit, pull, push normal).
//
//   DESTRUCTIVE— potencialmente destrutivas/resquisas. Exigem GATE EXPLÍCITO
//                (DestructiveGateError se não autorizado). (reset --hard,
//                branch delete, force push, destructive merge).
//
// O gate é FAIL-CLOSED: operação destrutiva SEM autorização retorna
// DestructiveGateError e NUNCA executa.

// ErrDestructiveBlocked indica que uma operação destrutiva foi bloqueada.
var ErrDestructiveBlocked = errors.New("gitmgr: operação destrutiva bloqueada pelo gate")

// DestructiveGate é o token de autorização explícita para operações
// destrutivas. O CLI só o fornece após o Don confirmar (prompt/flag --force).
type DestructiveGate struct {
	Authorized bool
	Reason     string
}

// authorized confirma que o gate foi autorizado explicitamente.
func (g *DestructiveGate) authorized() error {
	if g == nil || !g.Authorized {
		return &DestructiveGateError{Op: "destructive", Reason: "sem autorização explícita (gate fail-closed)"}
	}
	return nil
}

// ResetHard faz `reset --hard` a um commit. DESTRUTIVO: descarta mudanças do
// working tree. Exige gate explícito.
func (m *Manager) ResetHard(ctx context.Context, to string, gate *DestructiveGate) error {
	if err := gate.authorized(); err != nil {
		return err
	}
	wt, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("gitmgr: worktree: %w", err)
	}
	h, err := m.repo.ResolveRevision(plumbing.Revision(to))
	if err != nil {
		return fmt.Errorf("gitmgr: resolver %q: %w", to, err)
	}
	if err := wt.Reset(&git.ResetOptions{Commit: *h, Mode: git.HardReset}); err != nil {
		return fmt.Errorf("gitmgr: reset --hard: %w", err)
	}
	return nil
}

// DeleteBranch apaga um branch local. DESTRUTIVO: exige gate explícito.
func (m *Manager) DeleteBranch(ctx context.Context, name string, gate *DestructiveGate) error {
	if err := gate.authorized(); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("gitmgr: nome de branch vazio")
	}
	err := m.repo.DeleteBranch(name)
	if err != nil {
		return fmt.Errorf("gitmgr: deletar branch %q: %w", name, err)
	}
	return nil
}

// ForcePush faz um push forçado (sobrescreve o remoto). DESTRUTIVO: o mais
// crítico de todos — exige gate explícito. Não implementado no núcleo v0
// (a decisão de força é sempre do Don), mas o método existe para sinalizar
// o contrato de segurança.
func (m *Manager) ForcePush(ctx context.Context, gate *DestructiveGate) error {
	if err := gate.authorized(); err != nil {
		return err
	}
	return fmt.Errorf("gitmgr: force push NÃO implementado no v0 — o Don decide manualmente")
}

// RefList expõe operações READ (informação).
type RefList struct {
	Head    string   `json:"head"`
	Branches []string `json:"branches"`
}

// Inspect devolve um snapshot READ-ONLY do repositório (status + branch +
// branches + head). Nunca altera nada.
func (m *Manager) Inspect(ctx context.Context) (*RefList, error) {
	head, err := m.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("gitmgr: head: %w", err)
	}
	branches, err := m.Branches(ctx)
	if err != nil {
		return nil, err
	}
	return &RefList{Head: head.Name().Short(), Branches: branches}, nil
}

// Diff devolve um resumo das mudanças (adicionado/modificado) no working tree.
// READ-ONLY.
func (m *Manager) Diff(ctx context.Context) (string, error) {
	wt, err := m.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("gitmgr: worktree: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("gitmgr: status: %w", err)
	}
	var b strings.Builder
	for path, s := range st {
		b.WriteString(fmt.Sprintf("%c  %s\n", s.Worktree, path))
	}
	return b.String(), nil
}
