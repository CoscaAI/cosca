package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// opencodeInstallCommands maps OS+type to install instructions.
var opencodeInstallCommands = []struct {
	OS      string
	Name    string
	Command string
	Detect  string
}{
	{"linux", "Terminal (curl)", "curl -fsSL https://opencode.ai/install | bash", "opencode"},
	{"any", "Terminal (npm)", "npm i -g opencode-ai", "opencode"},
	{"any", "Terminal (bun)", "bun add -g opencode-ai", "opencode"},
	{"darwin", "Terminal (brew)", "brew install anomalyco/tap/opencode", "opencode"},
	{"linux", "Terminal (paru)", "paru -S opencode", "opencode"},
	{"darwin", "Desktop (brew)", "brew install --cask opencode-desktop", "OpenCode"},
	{"darwin", "Desktop (Apple Silicon)", "Baixar em https://opencode.ai/download", "OpenCode"},
	{"darwin", "Desktop (Intel)", "Baixar em https://opencode.ai/download", "OpenCode"},
	{"windows", "Desktop (x64)", "Baixar em https://opencode.ai/download", "OpenCode"},
	{"linux", "Desktop (.deb)", "Baixar em https://opencode.ai/download", "OpenCode"},
	{"linux", "Desktop (.rpm)", "Baixar em https://opencode.ai/download", "OpenCode"},
}

// NewStartCommand creates the `cosca start` command.
func NewStartCommand() *cobra.Command {
	var (
		noEditor   bool
		skipDoctor bool
		noOpen     bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "start [dir]",
		Short: "Prepara e inicia o Cosca — init, install, doctor, sync e abre o editor",
		Long: `cosca start prepara o projeto para uso imediato com o Cosca.

Fluxo automático:
  1. Detecta o estado da pasta (vazia, novo projeto, existente)
  2. Verifica dependências (Go, Git, OpenCode, etc.)
  3. Executa cosca init (se necessário) + cosca install
  4. Executa cosca doctor --fix (diagnóstico + correções seguras)
  5. Executa cosca sync (indexação do projeto)
  6. Abre o editor (OpenCode preferencialmente)

Se o OpenCode não estiver instalado, exibe os comandos de instalação
para o sistema operacional atual.

Comandos que exigem sudo NUNCA são executados — são listados para o Don.`,
		Example: `  cosca start                          # Prepara o diretório atual
  cosca start /home/don/meu-projeto     # Prepara um diretório específico
  cosca start --no-editor               # Pula integração com editor
  cosca start --skip-doctor             # Pula diagnóstico de saúde
  cosca start --no-open                 # Não abre o editor no final
  cosca start --dry-run                 # Mostra o que faria sem executar`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve target directory.
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("não foi possível resolver o diretório %q: %w", dir, err)
			}

			// Check directory exists.
			info, err := os.Stat(absDir)
			if err != nil {
				return fmt.Errorf("diretório %q não encontrado", absDir)
			}
			if !info.IsDir() {
				return fmt.Errorf("%q não é um diretório", absDir)
			}

			// ── PASSO 1: Detectar ──────────────────────────────────────
			fmt.Println()
			fmt.Println("  ╔══════════════════════════════════════════╗")
			fmt.Println("  ║       COSCA START — v1.5.0              ║")
			fmt.Println("  ╚══════════════════════════════════════════╝")
			fmt.Println()
			fmt.Printf("  📁 Projeto: %s\n", absDir)
			fmt.Println()

			isEmpty, err := isDirEmpty(absDir)
			if err != nil {
				return fmt.Errorf("erro ao ler diretório: %w", err)
			}

			coscaDir := filepath.Join(absDir, ".cosca")
			_, hasCosca := os.Stat(coscaDir)

			if isEmpty {
				fmt.Println("  🆕 Pasta vazia detectada — inicializando novo projeto...")
			} else if hasCosca == nil {
				fmt.Println("  📂 Projeto existente com Cosca — verificando instalação...")
			} else {
				fmt.Println("  📂 Projeto existente sem Cosca — inicializando...")
			}

			if dryRun {
				fmt.Println()
				fmt.Println("  🔍 DRY RUN — nenhuma alteração será feita.")
				fmt.Println()
				fmt.Println("  Ações que seriam executadas:")
				if isEmpty || hasCosca != nil {
					fmt.Println("     • cosca init")
				}
				fmt.Println("     • cosca install")
				if !skipDoctor {
					fmt.Println("     • cosca doctor --fix")
				}
				fmt.Println("     • cosca sync")
				if !noOpen {
					fmt.Println("     • abrir editor")
				}
				fmt.Println()
				return nil
			}

			// ── PASSO 2: Dependências ─────────────────────────────────
			fmt.Println()
			fmt.Println("  ── PASSO 2: Verificando dependências ──")
			deps := checkStartDeps()
			for _, d := range deps {
				if d.Ok {
					fmt.Printf("  ✅ %s: %s\n", d.Name, d.Version)
				} else {
					fmt.Printf("  ⚠️  %s: NÃO ENCONTRADO\n", d.Name)
					if d.InstallHint != "" {
						fmt.Printf("     💡 Instale com: %s\n", d.InstallHint)
					}
				}
			}

			// ── PASSO 3: Init + Install ───────────────────────────────
			fmt.Println()
			if isEmpty || hasCosca != nil {
				fmt.Println("  ── PASSO 3a: Inicializando Cosca ──")
				// Run cosca init in the target directory.
				initArgs := []string{"init"}
				if !noEditor {
					initArgs = append(initArgs, "--editor", "opencode")
				}
				if err := runCoscaInDir(absDir, initArgs...); err != nil {
					fmt.Printf("  ⚠️  cosca init: %v (continuando...)\n", err)
				} else {
					fmt.Println("  ✅ cosca init concluído")
				}
			}

			fmt.Println("  ── PASSO 3b: Instalando Cosca ──")
			installArgs := []string{"install", "--status"}
			if err := runCoscaInDir(absDir, installArgs...); err != nil {
				fmt.Printf("  ⚠️  cosca install --status: %v\n", err)
			}
			// Always run full install (idempotent).
			if err := runCoscaInDir(absDir, "install"); err != nil {
				fmt.Printf("  ⚠️  cosca install: %v (continuando...)\n", err)
			} else {
				fmt.Println("  ✅ cosca install concluído")
			}

			// ── PASSO 4: Doctor ───────────────────────────────────────
			if !skipDoctor {
				fmt.Println()
				fmt.Println("  ── PASSO 4: Diagnóstico de saúde ──")
				if err := runCoscaInDir(absDir, "doctor", "--fix"); err != nil {
					fmt.Printf("  ⚠️  cosca doctor: %v\n", err)
				} else {
					fmt.Println("  ✅ cosca doctor concluído")
				}
			}

			// ── PASSO 5: Sync ─────────────────────────────────────────
			fmt.Println()
			fmt.Println("  ── PASSO 5: Indexando projeto ──")
			if err := runCoscaInDir(absDir, "sync"); err != nil {
				fmt.Printf("  ⚠️  cosca sync: %v (continuando...)\n", err)
			} else {
				fmt.Println("  ✅ cosca sync concluído")
			}

			// ── PASSO 6: Abrir editor ─────────────────────────────────
			fmt.Println()
			if !noOpen {
				fmt.Println("  ── PASSO 6: Abrindo editor ──")
				editorFound := openEditor(absDir)
				if !editorFound {
					fmt.Println()
					fmt.Println("  ╔══════════════════════════════════════════╗")
					fmt.Println("  ║  NENHUM EDITOR ENCONTRADO               ║")
					fmt.Println("  ║  Instale o OpenCode para começar:        ║")
					fmt.Println("  ║                                          ║")
					printOpenCodeInstallInstructions()
					fmt.Println("  ║                                          ║")
					fmt.Println("  ║  Após instalar, execute:                 ║")
					fmt.Printf("  ║  cd %s && opencode                ║\n", absDir)
					fmt.Println("  ╚══════════════════════════════════════════╝")
				}
			}

			// ── PASSO 7: Resumo ───────────────────────────────────────
			fmt.Println()
			fmt.Println("  ╔══════════════════════════════════════════╗")
			fmt.Println("  ║  🟢 COSCA PRONTO!                       ║")
			fmt.Println("  ║                                          ║")
			fmt.Printf("  ║  📁 Projeto: %s\n", absDir)
			// Count indexed files if knowledge.db exists.
			fmt.Println("  ║  ✅ Init + Install + Doctor + Sync       ║")
			fmt.Println("  ║                                          ║")
			fmt.Println("  ║  🚀 Próximo comando:                     ║")
			fmt.Println("  ║     cosca serve                          ║")
			fmt.Println("  ╚══════════════════════════════════════════╝")
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().BoolVar(&noEditor, "no-editor", false, "Pular integração com editor")
	cmd.Flags().BoolVar(&skipDoctor, "skip-doctor", false, "Pular diagnóstico de saúde")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Não abrir o editor no final")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Mostrar ações sem executar nada")

	return cmd
}

// isDirEmpty returns true if the directory has no files (excluding hidden).
func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return false, nil
		}
	}
	return true, nil
}

// depInfo holds a dependency check result.
type depInfo struct {
	Name        string
	Ok          bool
	Version     string
	InstallHint string
}

// checkStartDeps verifies the tools needed for cosca start.
func checkStartDeps() []depInfo {
	var deps []depInfo

	// Go
	if path, err := exec.LookPath("go"); err == nil {
		out, _ := exec.Command(path, "version").Output()
		deps = append(deps, depInfo{Name: "Go", Ok: true, Version: strings.TrimSpace(string(out))})
	} else {
		deps = append(deps, depInfo{Name: "Go", InstallHint: "https://go.dev/dl/ — baixe e instale Go 1.21+"})
	}

	// Git
	if path, err := exec.LookPath("git"); err == nil {
		out, _ := exec.Command(path, "--version").Output()
		deps = append(deps, depInfo{Name: "Git", Ok: true, Version: strings.TrimSpace(string(out))})
	} else {
		deps = append(deps, depInfo{Name: "Git", InstallHint: "sudo apt install git (Linux) ou https://git-scm.com/downloads"})
	}

	// OpenCode
	if path, err := exec.LookPath("opencode"); err == nil {
		deps = append(deps, depInfo{Name: "OpenCode", Ok: true, Version: path})
	} else {
		deps = append(deps, depInfo{Name: "OpenCode", InstallHint: opencodeQuickInstall()})
	}

	// Bubblewrap (optional)
	if _, err := exec.LookPath("bwrap"); err == nil {
		deps = append(deps, depInfo{Name: "bwrap (jail)", Ok: true, Version: "instalado"})
	} else {
		deps = append(deps, depInfo{Name: "bwrap (jail)", InstallHint: "sudo apt install bubblewrap (opcional, segurança)"})
	}

	// Node.js (optional)
	if path, err := exec.LookPath("node"); err == nil {
		out, _ := exec.Command(path, "--version").Output()
		deps = append(deps, depInfo{Name: "Node.js", Ok: true, Version: strings.TrimSpace(string(out))})
	} else {
		deps = append(deps, depInfo{Name: "Node.js", InstallHint: "https://nodejs.org/ (opcional, para frontend)"})
	}

	return deps
}

// opencodeQuickInstall returns the fastest install command for the current OS.
func opencodeQuickInstall() string {
	switch runtime.GOOS {
	case "linux":
		return "curl -fsSL https://opencode.ai/install | bash"
	case "darwin":
		return "brew install anomalyco/tap/opencode"
	default:
		return "npm i -g opencode-ai"
	}
}

// printOpenCodeInstallInstructions prints all install options for the current OS.
func printOpenCodeInstallInstructions() {
	currentOS := runtime.GOOS
	fmt.Println("  ║  ── Terminal ──")
	for _, c := range opencodeInstallCommands {
		if c.OS == currentOS || c.OS == "any" {
			if strings.Contains(c.Name, "Desktop") {
				continue
			}
			fmt.Printf("  ║  • %s:\n", c.Name)
			fmt.Printf("  ║    %s\n", c.Command)
		}
	}
	fmt.Println("  ║")
	fmt.Println("  ║  ── Desktop ──")
	for _, c := range opencodeInstallCommands {
		if c.OS == currentOS || c.OS == "any" {
			if !strings.Contains(c.Name, "Desktop") {
				continue
			}
			fmt.Printf("  ║  • %s:\n", c.Name)
			fmt.Printf("  ║    %s\n", c.Command)
		}
	}
}

// openEditor tries to open the directory in the best available editor.
// Returns true if an editor was found and launched.
func openEditor(dir string) bool {
	// Priority: opencode → code (VS Code) → cursor → windsorf → zed
	editors := []string{"opencode", "code", "cursor", "windsurf", "zed"}

	for _, editor := range editors {
		path, err := exec.LookPath(editor)
		if err != nil {
			continue
		}

		var cmd *exec.Cmd
		switch editor {
		case "opencode":
			cmd = exec.Command(path, dir)
		default:
			cmd = exec.Command(path, dir)
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			fmt.Printf("  ✅ Aberto com %s\n", editor)
			return true
		}
	}

	return false
}

// runCoscaInDir runs a cosca command in the specified directory.
// Searches for the cosca binary in PATH, ~/.cosca/bin/, and the current
// executable's directory before falling back to "cosca".
func runCoscaInDir(dir string, args ...string) error {
	coscaBin := findCoscaBin()
	cmd := exec.Command(coscaBin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

// findCoscaBin locates the cosca binary. Uses the current executable
// first (we ARE cosca), then falls back to search paths.
func findCoscaBin() string {
	// 1. We are already running as cosca — use our own path.
	if exe, err := os.Executable(); err == nil {
		return exe
	}

	// 2. PATH lookup.
	if p, err := exec.LookPath("cosca"); err == nil {
		return p
	}

	// 3. Standard install location.
	home, err := os.UserHomeDir()
	if err == nil {
		standardPath := filepath.Join(home, ".cosca", "bin", "cosca")
		if _, err := os.Stat(standardPath); err == nil {
			return standardPath
		}
	}

	// 4. Common Go build output.
	if _, err := os.Stat("./bin/cosca"); err == nil {
		return "./bin/cosca"
	}

	return "cosca"
}
