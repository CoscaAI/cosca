package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// ─── Palette ─────────────────────────────────────────────────────────────────
// Identidade visual do Cosca: preto/grafite + dourado (Don) + verde (família).
// Tokyo Night adaptado: fundo profundo, contraste alto, acentos dourados.
var (
	// Cores base
	colorBackground    = lipgloss.Color("#1a1b26")
	colorBackgroundAlt = lipgloss.Color("#16161e")
	colorForeground    = lipgloss.Color("#c0caf5")
	colorGold          = lipgloss.Color("#e0af68") // o Don — dourado
	colorGoldDim       = lipgloss.Color("#b08a4e")
	colorGreen         = lipgloss.Color("#9ece6a") // a família — verde
	colorRed           = lipgloss.Color("#f7768e") // erros
	colorBlue          = lipgloss.Color("#7aa2f7") // informações
	colorPurple        = lipgloss.Color("#bb9af7") // kernel / subagentes
	colorCyan          = lipgloss.Color("#7dcfff") // tool calls
	colorGray          = lipgloss.Color("#565f89") // secundário
	colorGrayLight     = lipgloss.Color("#787c99") // terciário
	colorBorder        = lipgloss.Color("#3b4261") // bordas de painéis
)

// ─── Layout base ──────────────────────────────────────────────────────────────
var (
	// appStyle é a moldura geral do aplicativo.
	appStyle = lipgloss.NewStyle().
			Background(colorBackground).
			Foreground(colorForeground)

	// headerStyle renderiza o painel do cabeçalho com a identidade do Kernel.
	headerStyle = lipgloss.NewStyle().
			Background(colorGold).
			Foreground(colorBackground).
			Bold(true).
			Padding(0, 1)

	headerSubStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true)

	kernelBadgeStyle = lipgloss.NewStyle().
				Foreground(colorPurple).
				Bold(true)

	// statusPanelStyle é o box do header com o modelo ativo e o status.
	statusPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Foreground(colorForeground).
				Padding(0, 1)

	// historyViewportStyle envolve a área rolável de histórico.
	historyViewportStyle = lipgloss.NewStyle().
				Padding(0, 1)

	// inputBoxStyle é a moldura do campo de entrada.
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGoldDim).
			Padding(0, 1)

	// inputFocusedStyle é a moldura do campo de entrada quando ativo.
	inputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorGold).
				Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Padding(0, 1)

	// separatorStyle é o divisor fino entre blocos da conversa.
	separatorStyle = lipgloss.NewStyle().
			Foreground(colorBorder)
)

// ─── Bolhas de mensagem ───────────────────────────────────────────────────────

// assistantBubble renderiza a resposta do modelo em uma bolha com borda.
var assistantBubble = lipgloss.NewStyle().
	Foreground(colorForeground).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 1).
	MaxWidth(100)

// userBubbleBox é a bolha do Don — fundo dourado sólido, texto escuro.
var userBubbleBox = lipgloss.NewStyle().
	Foreground(colorBackground).
	Background(colorGold).
	Bold(true).
	Padding(0, 1).
	MaxWidth(100)

// toolBubble renderiza uma tool call em execução.
var toolBubble = lipgloss.NewStyle().
	Foreground(colorCyan).
	Bold(true)

// toolCallBox é a caixa compacta de uma tool call (estilo opencode).
var toolCallBox = lipgloss.NewStyle().
	Foreground(colorCyan).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 1).
	MaxWidth(100)

// toolOKBubble renderiza o resultado bem-sucedido de uma tool.
var toolOKBubble = lipgloss.NewStyle().
	Foreground(colorGreen)

// toolFailBubble renderiza a falha de uma tool.
var toolFailBubble = lipgloss.NewStyle().
	Foreground(colorRed)

// subagentBubble renderiza um subagente sendo acionado.
var subagentBubble = lipgloss.NewStyle().
	Foreground(colorPurple).
	Bold(true)

// errorBubble renderiza erros terminais.
var errorBubble = lipgloss.NewStyle().
	Foreground(colorRed).
	Bold(true)

// slashBubble renderiza o resultado de um comando slash.
var slashBubble = lipgloss.NewStyle().
	Foreground(colorBlue)

// infoBubble renderiza informações do sistema (sessão, tokens).
var infoBubble = lipgloss.NewStyle().
	Foreground(colorGrayLight).
	Italic(true)

// helpBubble renderiza o painel de ajuda.
var helpBubble = lipgloss.NewStyle().
	Foreground(colorCyan)

// welcomeTitle renderiza o título da tela de boas-vindas.
var welcomeTitle = lipgloss.NewStyle().
	Foreground(colorGold).
	Bold(true)

// welcomeBody renderiza o corpo da tela de boas-vindas.
var welcomeBody = lipgloss.NewStyle().
	Foreground(colorForeground)

// welcomeHint renderiza as dicas da tela de boas-vindas.
var welcomeHint = lipgloss.NewStyle().
	Foreground(colorGrayLight)

// welcomeBox emoldura a tela de boas-vindas.
var welcomeBox = lipgloss.NewStyle().
	Border(lipgloss.DoubleBorder()).
	BorderForeground(colorGold).
	Padding(1, 2).
	Width(72)

// timestampStyle renderiza o horário de uma mensagem.
var timestampStyle = lipgloss.NewStyle().
	Foreground(colorGray).
	Italic(true)
