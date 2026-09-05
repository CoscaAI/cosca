package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Logo ASCII (estilo opencode) ─────────────────────────────────────────────
// Identidade visual do Cosca na abertura: arte ASCII do ☯ com a marca.

// coscaLogo é a arte ASCII renderizada na tela de boas-vindas.
const coscaLogo = `
      ██████╗  ██████╗ ███████╗ ██████╗ █████╗
     ██╔════╝ ██╔═══██╗██╔════╝██╔════╝██╔══██╗
     ██║      ██║   ██║███████╗██║     ███████║
     ██║      ██║   ██║╚════██║██║     ██╔══██║
     ╚██████╗ ╚██████╔╝███████║╚██████╗██║  ██║
      ╚═════╝  ╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝`

// logoStyle renderiza o logo com a cor do Kernel.
var logoStyle = lipgloss.NewStyle().
	Foreground(colorPurple).
	Bold(true)

// logoAccentStyle renderiza o ☯ central com dourado.
var logoAccentStyle = lipgloss.NewStyle().
	Foreground(colorGold).
	Bold(true)

// renderLogo monta o logo centralizado com o símbolo ☯.
func renderLogo() string {
	// Extrai as linhas do logo e aplica a cor.
	var b strings.Builder
	for _, line := range strings.Split(strings.Trim(coscaLogo, "\n"), "\n") {
		b.WriteString(logoStyle.Render(strings.TrimRight(line, " ")))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
