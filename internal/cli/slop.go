package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/tools"
	"github.com/spf13/cobra"
)

// slopToolRoot é a fonte do slopguard (produto da casa). Overridable via
// COSCA_SLOPGUARD_ROOT para máquinas com checkout diferente.
func slopToolRoot() string {
	if p := os.Getenv("COSCA_SLOPGUARD_ROOT"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "projects", "slopguard")
}

// slopInstalledPath é onde o binário de produção fica (~/.cosca/bin/slopguard).
func slopInstalledPath() string {
	return tools.InstalledPath("slop")
}

// NewSlopCommand cria o `cosca slop` — o fiscal do Padrão COSCA §4 (Output).
// Auto-builda o slopguard se os fontes estiverem mais novos que o binário
// instalado, instala em ~/.cosca/bin e delega os argumentos ao binário
// (uma-ferramenta-um-comando — L52).
//
// SECURITY: apenas delega a um binário Go da própria casa; nenhum argumento é
// interpretado pelo kernel. O exit code do slopguard é propagado (0=limpo,
// 1=slop encontrado) para uso em CI.
func NewSlopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "slop [check args...]",
		Short: "Fiscal do Padrão COSCA — anti-AI-slop (slopguard)",
		Long: `Fiscal do Padrão COSCA §4 (Output): detecta padrões de AI slop
(palavras banidas, frases vazias, em-dash) em arquivos ou diretórios.

  cosca slop check README.md docs/        # fiscaliza arquivos/dirs
  cosca slop check . --skip-patterns      # pula dados de padrão/fixtures

O binário é construído automaticamente quando os fontes do slopguard
(~/Documents/projects/slopguard) estão mais novos que o instalado, e
instalado em ~/.cosca/bin. Exit code: 0 = limpo, 1 = slop encontrado.`,
		Example: `  cosca slop check README.md
  cosca slop check . --skip-patterns`,
		Args: cobra.ArbitraryArgs,
		RunE: runSlop,
	}
}

func runSlop(cmd *cobra.Command, args []string) error {
	if err := ensureSlopBinary(); err != nil {
		return err
	}

	bin := slopInstalledPath()
	c := exec.Command(bin, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// Propaga o exit code do slopguard (1 = slop encontrado).
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

// ensureSlopBinary rebuilda e instala o slopguard se os fontes estiverem mais
// novos que o binário instalado (auto-build, padrão L51).
func ensureSlopBinary() error {
	installed := slopInstalledPath()
	root := slopToolRoot()

	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("fonte do slopguard não encontrada em %s (defina COSCA_SLOPGUARD_ROOT): %w", root, err)
	}

	if !slopIsStale(installed, root) {
		return nil
	}

	fmt.Fprintf(os.Stderr, "  >  slopguard desatualizado — rebuildando…\n")
	built := filepath.Join(root, "build", "slopguard")
	if err := buildSlop(root, built); err != nil {
		return err
	}
	if err := installSlopBinary(built, installed); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  ✓  slopguard instalado em %s\n", installed)
	return nil
}

// slopIsStale reporta se o binário instalado está ausente ou mais velho que
// o fonte Go mais recente do projeto.
func slopIsStale(installed, root string) bool {
	info, err := os.Stat(installed)
	if err != nil {
		return true // ausente → precisa buildar
	}
	installedTime := info.ModTime()

	latest := time.Time{}
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".go" && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
		return nil
	})
	return latest.After(installedTime)
}

// buildSlop compila o slopguard em um path temporário dentro do repo.
func buildSlop(root, out string) error {
	cmd := exec.Command("go", "build", "-o", out, "./cmd/slopguard")
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build do slopguard falhou: %w", err)
	}
	return nil
}

// installSlopBinary copia o binário compilado para ~/.cosca/bin/slopguard.
func installSlopBinary(built, installed string) error {
	data, err := os.ReadFile(built)
	if err != nil {
		return fmt.Errorf("ler binário compilado: %w", err)
	}
	if err := os.WriteFile(installed, data, 0o755); err != nil {
		return fmt.Errorf("instalar slopguard: %w", err)
	}
	return nil
}
