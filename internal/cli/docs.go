package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

// NewDocsCommand creates the `cosca docs` command.
func NewDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Open Cosca documentation",
		Long:  `Open the Cosca documentation in the default web browser.`,
		Example: `  cosca docs
  cosca docs --offline`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			var url string

			offline, _ := cmd.Flags().GetBool("offline")

			if offline {
				// Check for local docs
				localDocs := findLocalDocs()
				if localDocs != "" {
					url = localDocs
				} else {
					url = "https://cosca.enterprise/docs"
					formatter.Warning("Local documentation not found, opening online docs")
				}
			} else {
				url = "https://cosca.enterprise/docs"
			}

			formatter.Verbose(fmt.Sprintf("Opening %s", url))

			if err := openBrowser(url); err != nil {
				return fmt.Errorf("failed to open browser: %w", err)
			}

			formatter.Success(fmt.Sprintf("Documentation opened: %s", url))
			return nil
		},
	}

	cmd.Flags().Bool("offline", false, "open local documentation if available")
	return cmd
}

// openBrowser é uma variável de pacote para permitir stub nos testes (não abrir
// janela real ao rodar `go test`). Em produção usa defaultOpenBrowser.
var openBrowser = defaultOpenBrowser

// defaultOpenBrowser abre uma URL no navegador padrão da plataforma.
func defaultOpenBrowser(url string) error {
	if url == "" {
		return fmt.Errorf("cannot open empty URL")
	}
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// findLocalDocs looks for local documentation files.
func findLocalDocs() string {
	// Check common locations for local docs
	checkPaths := []string{
		"/usr/share/doc/cosca/index.html",
		"/usr/local/share/doc/cosca/index.html",
		"/opt/cosca/docs/index.html",
	}

	for _, path := range checkPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
