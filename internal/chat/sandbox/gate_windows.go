//go:build windows

package sandbox

// gate_windows.go — SANDBOX NATIVO WINDOWS (ADR-034, Onda 2 — Job Object).
//
// No Windows o bubblewrap não existe. O backend nativo de isolamento de
// processo é o JOB OBJECT: o executor compartilhado (processutil.Run, ver
// internal/processutil/processutil_tree_windows.go) já cria um Job Object com
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE e derruba a árvore inteira ao término —
// o análogo do --die-with-parent. O Gate aqui valida, monta o comando dentro
// do contrato (env allowlist, workspace, fail-closed) e delega ao executor.
//
// A detecção de disponibilidade (jobObjectAvailable) é automática: se o SO
// criar o Job Object, o sandbox nativo está operacional — sem flag manual.
//
// FAIL-CLOSED: sem Job Object disponível, os modos SandboxReadOnly/
// SandboxWorkspace NEGAM a execução com erro claro — nunca caem em execução
// direta silenciosa. Apenas SandboxFull (equivalente ao execDirect explícito)
// exige COSCA_ALLOW_NO_ROOT=1.
//
// LIMITAÇÃO HONESTA (fase atual da ADR-034): o Job Object contém processo/
// árvore — os limites de MEMÓRIA (JOB_OBJECT_LIMIT_PROCESS_MEMORY, análogo do
// RLIMIT_AS) e de PROCESSOS (anti fork-bomb) são o próximo incremento, junto
// com o AppContainer (isolamento FS/rede por SID, não exposto no x/sys v0.47.0).
// O Rails (validação de caminho) permanece como contenção de caminho. Ver
// docs/adr/ADR-034-native-windows-sandbox.md §3.2/§4.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/processutil"
)

// nativeSandboxAvailable reporta se o backend nativo (Job Object) está
// disponível — o sandbox do Windows está operacional quando o job cria.
func nativeSandboxAvailable() bool { return jobObjectAvailable() }

// findBwrap retorna "" — bwrap não existe no Windows.
func findBwrap() string { return "" }

// bwrapCommand nunca é chamado no Windows (findBwrap() == "").
func (g *Gate) bwrapCommand(_ context.Context, _ chat.Command, _ chat.SandboxMode) (*exec.Cmd, error) {
	return nil, errors.New("sandbox: bwrap is not supported on Windows")
}

// execBwrap nunca é chamado no Windows (findBwrap() == "").
func (g *Gate) execBwrap(_ context.Context, _ chat.Command, _ chat.SandboxMode) (*chat.SandboxResult, error) {
	return nil, errors.New("sandbox: bwrap is not supported on Windows")
}

// execNative executa um comando sob o sandbox nativo do Windows.
//
// O executor compartilhado (processutil.Run) é o dono da execução: ele inicia
// o processo, cria o Job Object com KILL_ON_JOB_CLOSE, monitora idle/timeout e
// derruba a árvore inteira ao fim. Aqui montamos o comando com o contrato de
// segurança (env allowlist, workspace como cwd default) e garantimos o
// fail-closed antes de qualquer execução.
func (g *Gate) execNative(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("sandbox: empty command arguments")
	}
	if !jobObjectAvailable() {
		return nil, fmt.Errorf("sandbox unavailable: no native sandbox backend (Job Object) on this system; refusing to run without isolation")
	}
	if err := validateEnv(cmd.Env); err != nil {
		return nil, err
	}
	if err := g.validateWorkspace(); err != nil {
		return nil, err
	}

	execCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
	// Env restrito (allowlist) — nunca herda credenciais do host.
	execCmd.Env = safeEnv(cmd.Env)
	execCmd.Dir = resolveWorkDir(g, cmd)
	// stdin conectado para comandos interativos que precisam dele.
	execCmd.Stdin = os.Stdin

	// O modo ReadOnly é respeitado no nível do executor (Rails/validação de
	// caminho rejeitam escrita fora do workspace); a distinção FS/rede real por
	// SID é o AppContainer (fase seguinte da ADR-034).
	_ = mode

	// Roda através do executor compartilhado: stream de saída, idle timeout,
	// teto de runtime e término da árvore inteira (Job Object + taskkill).
	res, err := processutil.Run(ctx, execCmd, processutil.Config{
		IdleTimeout: cmd.IdleTimeout,
		MaxRuntime:  cmd.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox: command execution failed: %w", err)
	}

	return &chat.SandboxResult{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		ExitCode: res.ExitCode,
		Duration: res.Duration,
		Status:   string(res.Status),
		IdleFor:  res.IdleFor,
	}, nil
}

// resolveWorkDir resolve o diretório de trabalho do comando (default:
// workspace do gate).
func resolveWorkDir(g *Gate, cmd chat.Command) string {
	if cmd.WorkDir != "" {
		abs, err := filepath.Abs(cmd.WorkDir)
		if err == nil {
			return abs
		}
		return cmd.WorkDir
	}
	return g.workspace
}
