package autonomy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/processutil"
)

// WorkspaceSnapshot é o estado do worktree usado pelo change-detection (prime
// autonomous.ts:374-423): status do git + diff + hash dos untracked. Se NENHUM
// dos componentes mudou, um gate que falhou não é re-executado.
type WorkspaceSnapshot struct {
	// Status é a saída de `git status --porcelain=v1`.
	Status string
	// Diff é a saída de `git diff HEAD` (tracked modifications).
	Diff string
	// UntrackedHash é o hash agregado dos arquivos untracked (conteúdo, não só path).
	UntrackedHash string
}

// equal devolve true quando DOIS snapshots são idênticos (prime
// gitWorktreeSnapshotsEqual): workspace inalterado → gate não re-executa.
func (a WorkspaceSnapshot) equal(b *WorkspaceSnapshot) bool {
	if b == nil {
		return false
	}
	return a.Status == b.Status && a.Diff == b.Diff && a.UntrackedHash == b.UntrackedHash
}

// Equal é a forma EXPORTADA (para o borda/tests) da comparação de snapshots.
func (a WorkspaceSnapshot) Equal(b WorkspaceSnapshot) bool {
	return a.equal(&b)
}

// Snapshotter captura o estado do workspace (change-detection). Injetável para
// mock nos testes; a implementação real é o gitSnapshotter.
type Snapshotter interface {
	Capture(ctx context.Context, dir string) (WorkspaceSnapshot, error)
}

// gitSnapshotter é a implementação REAL via git (stdlib only, usando processutil
// para rodar os subprocessos com tree-kill e timeout).
type gitSnapshotter struct {
	// Pathspec é o caminho/base do snapshot (default "."). Entradas `:(exclude)...`
	// são pathspecs mágicos do git e permitem excluir diretórios de build.
	Pathspec []string
}

// NewGitSnapshotter cria um snapshotter de workspace sobre o worktree dado.
func NewGitSnapshotter() Snapshotter {
	return &gitSnapshotter{Pathspec: []string{"."}}
}

// WithExcludes adiciona pathspecs `:(exclude)<path>` ao snapshotter (ex.:
// excluir "target/", "node_modules/", "*.db-wal") — o change-detection ignora
// artefatos de build que oscilam e voltariam a reexecutar o gate à toa.
func WithExcludes(s Snapshotter, excludes ...string) Snapshotter {
	g, ok := s.(*gitSnapshotter)
	if !ok {
		return s
	}
	ps := append([]string(nil), g.Pathspec...)
	for _, e := range excludes {
		ps = append(ps, ":(exclude)"+e)
	}
	return &gitSnapshotter{Pathspec: ps}
}

// Capture monta o snapshot (status + diff + hash de untracked). Faz 3 subprocessos
// git com timeout; qualquer falha → erro (fail-closed: sem snapshot válido, o
// change-detection não pode garantir "inalterado" e o gate re-executa).
func (g *gitSnapshotter) Capture(ctx context.Context, dir string) (WorkspaceSnapshot, error) {
	if strings.TrimSpace(dir) == "" {
		return WorkspaceSnapshot{}, errors.New("autonomy: diretório de snapshot vazio")
	}
	git := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		res, err := processutil.Run(ctx, cmd, processutil.Config{MaxRuntime: 30 * time.Second})
		if err != nil {
			return "", err
		}
		if res.ExitCode != 0 {
			return "", fmt.Errorf("git %v: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
		}
		if res.Status == processutil.StatusHardTimeout || res.Status == processutil.StatusIdleTimeout {
			return "", errors.New("git snapshot: timeout")
		}
		return res.Stdout, nil
	}

	status, err := git(append([]string{"status", "--porcelain=v1", "--no-renames", "-uall"}, g.pathspecArgs()...)...)
	if err != nil {
		return WorkspaceSnapshot{}, err
	}
	diff, err := git(append([]string{"diff", "--no-ext-diff", "HEAD"}, g.pathspecArgs()...)...)
	if err != nil {
		return WorkspaceSnapshot{}, err
	}
	hash, err := g.hashUntracked(ctx, dir, status)
	if err != nil {
		return WorkspaceSnapshot{}, err
	}
	return WorkspaceSnapshot{Status: status, Diff: diff, UntrackedHash: hash}, nil
}

// pathspecArgs monta `-- <pathspec...>` para o git.
func (g *gitSnapshotter) pathspecArgs() []string {
	if len(g.Pathspec) == 0 {
		return []string{"--", "."}
	}
	args := []string{"--"}
	for _, p := range g.Pathspec {
		args = append(args, p)
	}
	return args
}

// untrackedPaths extrai os paths untracked ("?? ") da saída porcelain v1.
func untrackedPaths(status string) []string {
	var paths []string
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "?? ") {
			paths = append(paths, strings.TrimPrefix(line, "?? "))
		}
	}
	sort.Strings(paths)
	return paths
}

// hashUntracked agrega o conteúdo dos arquivos untracked num hash sha256
// (ordem determinística + conteúdo), espelhando hashUntrackedFiles do prime.
func (g *gitSnapshotter) hashUntracked(ctx context.Context, dir string, status string) (string, error) {
	paths := untrackedPaths(status)
	if len(paths) == 0 {
		return "", nil
	}
	h := sha256.New()
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
		ch, err := hashUntrackedPath(filepath.Join(dir, p))
		if err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(ch))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashUntrackedPath faz o hash sha256 do conteúdo de um arquivo untracked
// (symlink → alvo, dir → metadata determinística — mirror do prime-agent).
func hashUntrackedPath(path string) (string, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return fmt.Sprintf("error:%s", err), nil // path sumiu entre status e hash → não pode garantir
	}
	if st.Mode()&os.ModeSymlink != 0 {
		var target string
		if t, err := os.Readlink(path); err == nil {
			target = t
		}
		return "symlink:" + target, nil
	}
	if !st.Mode().IsRegular() {
		return fmt.Sprintf("other:%d:%d:%d", st.Mode(), st.Size(), st.ModTime().UnixNano()), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error:%s", err), nil
	}
	s := sha256.Sum256(data)
	return "file:" + hex.EncodeToString(s[:]), nil
}
