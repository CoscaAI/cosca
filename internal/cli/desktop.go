package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/tools"
	"github.com/spf13/cobra"
)

// desktopToolRoot is the cosca-desktop source repository. Overridable via
// COSCA_DESKTOP_ROOT so the same command works on machines with a different
// checkout location.
func desktopToolRoot() string {
	if p := os.Getenv("COSCA_DESKTOP_ROOT"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "projects", "cosca-desktop")
}

// desktopInstalledPath is where the production desktop binary is installed
// globally, next to the cosca binaries (~/.cosca/bin/cosca-desktop). The path
// is registered in internal/tools (the "one command per tool" registry).
func desktopInstalledPath() string {
	return tools.InstalledPath("desktop")
}

// NewDesktopCommand creates the `cosca desktop` command: builds the
// cosca-desktop tool if its sources are newer than the installed binary,
// installs it globally into ~/.cosca/bin, and launches it bound to the target
// directory — the current working directory by default, or the directory given
// as an argument (like `code .` opens the current folder). `~` and relative
// paths are expanded.
//
// With --dev the desktop runs under `wails dev` (hot-reload + WebKit
// devtools) instead of the production binary — the developer workflow, no
// 40s rebuild per change.
//
// SECURITY: the desktop runs OUTSIDE the agent jail (it is an admin launcher
// for a GUI app, like `cosca terminal`). It never executes untrusted agent
// code itself; the desktop's own engine wires the pipeline with its normal
// sandboxing.
func NewDesktopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop [diretorio]",
		Short: "Abrir o COSCA Desktop na pasta atual (ou na pasta informada)",
		Long: `Abrir o COSCA Desktop — o cockpit enterprise (Wails + React).

Sem argumento, abre na pasta atual (como "code ."). Com um diretorio, abre
nessa pasta (expande ~ e relativos):

  cosca desktop                # abre no diretorio atual
  cosca desktop ~/Documents/bruno
  cosca desktop ./projeto

O binario e construido automaticamente quando os fontes do cosca-desktop
estao mais novos que o instalado (auto-build) e instalado em ~/.cosca/bin,
ficando disponivel para execucao de qualquer pasta.

Com --dev roda o modo desenvolvimento (wails dev): hot-reload + devtools
do WebKit — edite o codigo e veja a mudanca na hora, sem rebuild manual.`,
		Example: `  cosca desktop
  cosca desktop ~/Documents/bruno
  cosca desktop --dev           # modo dev: hot-reload + devtools`,
		Args: cobra.MaximumNArgs(1),
		RunE: runDesktop,
	}
	cmd.Flags().Bool("dev", false, "rodar em modo desenvolvimento (wails dev — hot-reload + devtools)")
	return cmd
}

// runDesktop resolves the target directory and either launches the production
// binary (default) or runs the dev server (--dev).
func runDesktop(cmd *cobra.Command, args []string) error {
	devMode, _ := cmd.Flags().GetBool("dev")
	target, err := resolveDesktopDir(args)
	if err != nil {
		return err
	}

	if devMode {
		return runDesktopDev(cmd, target)
	}

	if err := ensureDesktopBinary(); err != nil {
		return err
	}

	bin := desktopInstalledPath()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("cosca-desktop nao encontrado em %s: %w", bin, err)
	}

	proc := exec.Command(bin)
	proc.Dir = target
	proc.Env = append(os.Environ(), "COSCA_WORKDIR="+target)
	// Desanexa o desktop do terminal (setsid no Unix; no-op no Windows).
	detachDesktopProcess(proc)
	if err := proc.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar cosca-desktop: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ cosca-desktop aberto em %s\n", target)
	return nil
}

// runDesktopDev runs `wails dev -tags webkit2_41` in the desktop repo in the
// foreground (Ctrl+C to stop). Hot-reload recompiles on every source change,
// so the developer never waits for a full production build.
func runDesktopDev(cmd *cobra.Command, target string) error {
	root := desktopToolRoot()
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("fonte do cosca-desktop nao encontrada em %s (defina COSCA_DESKTOP_ROOT): %w", root, err)
	}
	wailsBin := resolveWails()
	if wailsBin == "" {
		return fmt.Errorf("wails CLI nao encontrado — instale com: go install github.com/wailsapp/wails/v2/cmd/wails@latest")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "modo dev: hot-reload ativo em %s (Ctrl+C para parar)\n", target)
	dev := exec.Command(wailsBin, "dev", "-tags", "webkit2_41")
	dev.Dir = root
	dev.Env = append(os.Environ(), "COSCA_WORKDIR="+target)
	dev.Stdout = os.Stderr
	dev.Stderr = os.Stderr
	dev.Stdin = os.Stdin
	if err := dev.Run(); err != nil {
		return fmt.Errorf("wails dev encerrado: %w", err)
	}
	return nil
}

// resolveDesktopDir expands ~, absolutizes, and validates the target directory
// (default: the current working directory).
func resolveDesktopDir(args []string) (string, error) {
	raw := "."
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		raw = strings.TrimSpace(args[0])
	}
	expanded, err := expandDesktopPath(raw)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolver diretorio %q: %w", raw, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("diretorio %q: %w", raw, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q nao e um diretorio", raw)
	}
	return abs, nil
}

// expandDesktopPath expands a leading ~ to the user home directory.
func expandDesktopPath(p string) (string, error) {
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expandir ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expandir ~: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// ensureDesktopBinary rebuilds the desktop when its sources are newer than the
// installed binary (or when the binary is missing), then atomically installs
// the fresh build into ~/.cosca/bin. It is a no-op when the installed binary
// is already fresh.
func ensureDesktopBinary() error {
	installed := desktopInstalledPath()
	root := desktopToolRoot()

	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("fonte do cosca-desktop nao encontrada em %s (defina COSCA_DESKTOP_ROOT): %w", root, err)
	}

	if !desktopIsStale(installed, desktopSources(root)) {
		return nil
	}

	fmt.Fprintf(os.Stderr, "  >  cosca-desktop desatualizado — rebuildando…\n")
	if err := buildDesktop(root); err != nil {
		return err
	}

	built := filepath.Join(root, "build", "bin", "cosca-desktop")
	if _, err := os.Stat(built); err != nil {
		return fmt.Errorf("build nao produziu %s", built)
	}
	if err := installDesktopBinary(built, installed); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  ✓  cosca-desktop instalado em %s\n", installed)
	return nil
}

// desktopSources walks the desktop repo collecting Go sources (root *.go) and
// frontend sources (frontend/src/**) — the inputs that, when newer than the
// installed binary, make it stale.
func desktopSources(root string) []string {
	var files []string
	roots := []string{
		filepath.Join(root, "frontend", "src"),
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".tsx") || strings.HasSuffix(p, ".ts") ||
			strings.HasSuffix(p, ".css") {
			files = append(files, p)
		}
		return nil
	})
	_ = roots // (frontend/src já é coberto pelo walk geral)
	sort.Strings(files)
	return files
}

// desktopIsStale reports whether the installed binary is missing or older than
// any of the given sources.
func desktopIsStale(installed string, sources []string) bool {
	inst, err := os.Stat(installed)
	if err != nil {
		return true
	}
	for _, s := range sources {
		si, err := os.Stat(s)
		if err != nil {
			continue
		}
		if si.ModTime().After(inst.ModTime()) {
			return true
		}
	}
	return false
}

// buildDesktop runs `wails build -tags webkit2_41` in the desktop repo.
func buildDesktop(root string) error {
	wailsBin := resolveWails()
	if wailsBin == "" {
		return fmt.Errorf("wails CLI nao encontrado — instale com: go install github.com/wailsapp/wails/v2/cmd/wails@latest")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, wailsBin, "build", "-tags", "webkit2_41")
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wails build falhou: %w", err)
	}
	return nil
}

// resolveWails locates the wails CLI: $HOME/go/bin/wails first (the common
// no-root install), then the PATH.
func resolveWails() string {
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "go", "bin", "wails")
		if fi, err := os.Stat(candidate); err == nil && fi.Mode().IsRegular() {
			return candidate
		}
	}
	if p, err := exec.LookPath("wails"); err == nil {
		return p
	}
	return ""
}

// installDesktopBinary atomically copies the built binary into the global bin
// dir (temp + chmod + rename) so a running instance is never corrupted.
func installDesktopBinary(built, installed string) error {
	dir := filepath.Dir(installed)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("criar %s: %w", dir, err)
	}
	tmp := installed + ".new"
	if err := copyFile(built, tmp); err != nil {
		return fmt.Errorf("copiar binario: %w", err)
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp, installed); err != nil {
		return fmt.Errorf("instalar binario: %w", err)
	}
	return nil
}

// copyFile copies src to dst (regular file, 0644 before chmod).
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
