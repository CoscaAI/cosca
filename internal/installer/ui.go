// Package installer â€” UI do provisioner (versÃ£o terminal, TUI bubbletea).
//
// A visÃ£o do professor: "a UI visualiza o que o Provisioner REALMENTE faz".
// Esta TUI consome o event stream (EmitFunc) e renderiza o estado real em
// tempo real â€” logo, fases, progresso, certificaÃ§Ã£o. A animaÃ§Ã£o nunca Ã©
// falsa: cada linha reflete o evento emitido pelo orquestrador.
//
// A versÃ£o .exe com partÃ­culas Ã© uma evoluÃ§Ã£o desta mesma UI (mesmo contrato).
package installer

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// â”€â”€ estilos (reusa a identidade visual do COSCA) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

var (
	// colorPurple/colorGold seguem o logo do chat/ui.
	styleLogo = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Bold(true)
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
)

// uiLogo Ã© a arte ASCII do COSCA (simplificada para a TUI).
const uiLogo = `
  â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•— â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•— â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•— â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•— â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•—
 â–ˆâ–ˆâ•”â•â•â•â•â•â–ˆâ–ˆâ•”â•â•â•â–ˆâ–ˆâ•—â–ˆâ–ˆâ•”â•â•â•â•â•â–ˆâ–ˆâ•”â•â•â•â•â•â–ˆâ–ˆâ•”â•â•â–ˆâ–ˆâ•—
 â–ˆâ–ˆâ•‘     â–ˆâ–ˆâ•‘   â–ˆâ–ˆâ•‘â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•—â–ˆâ–ˆâ•‘     â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•‘
 â–ˆâ–ˆâ•‘     â–ˆâ–ˆâ•‘   â–ˆâ–ˆâ•‘â•šâ•â•â•â•â–ˆâ–ˆâ•‘â–ˆâ–ˆâ•‘     â–ˆâ–ˆâ•”â•â•â–ˆâ–ˆâ•‘
 â•šâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•—â•šâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•”â•â–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•‘â•šâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ–ˆâ•—â–ˆâ–ˆâ•‘  â–ˆâ–ˆâ•‘
  â•šâ•â•â•â•â•â• â•šâ•â•â•â•â•â• â•šâ•â•â•â•â•â•â• â•šâ•â•â•â•â•â•â•šâ•â•  â•šâ•â•
`

// UIModel Ã© o estado da TUI do provisioner.
type UIModel struct {
	logo    string
	title   string
	steps   []string // linha por etapa concluÃ­da (mark + check)
	current string   // etapa em andamento
	progress int     // 0..100 (estimado pelas fases)
	state   State    // installation state atual
	done    bool
	certified bool
	mu      sync.Mutex
}

// NewUIModel monta o modelo inicial da TUI do provisioner.
func NewUIModel() *UIModel {
	return &UIModel{
		logo:  styleLogo.Render(uiLogo),
		title: styleTitle.Render("COSCA ENVIRONMENT PROVISIONER"),
	}
}

// â”€â”€ Consumo do event stream â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// UIEmitter devolve um EmitFunc que atualiza a TUI a partir dos eventos.
// A TUI roda em goroutine; o bubbletea Update recebe os eventos via canal.
func (m *UIModel) Emitter() (EmitFunc, chan tea.Msg) {
	ch := make(chan tea.Msg, 64)
	emit := func(e Event) {
		select {
		case ch <- e:
		default: // buffer cheio â€” descarta (nÃ£o bloqueia o provisioner)
		}
	}
	return emit, ch
}

// â”€â”€ bubbletea contract â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (m *UIModel) Init() tea.Cmd { return nil }

func (m *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch e := msg.(type) {
	case Event:
		m.applyEvent(e)
	case tea.KeyMsg:
		if e.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

// applyEvent atualiza o modelo a partir de um evento real do provisioner.
func (m *UIModel) applyEvent(e Event) {
	switch e.Type {
	case EventStepStarted:
		m.current = e.Check
	case EventStepComplete:
		mark := styleOK.Render("âœ“")
		if e.Result != ResultPass {
			mark = styleWarn.Render("âš ")
		}
		m.steps = append(m.steps, fmt.Sprintf("  %s %-30s %s",
			mark, e.Check, styleDim.Render(string(e.Result))))
		m.current = ""
	case EventStateChanged:
		m.state = e.State
		// Progresso estimado: posiÃ§Ã£o do estado na ordem canÃ´nica.
		m.progress = progressFor(e.State)
		m.steps = append(m.steps, styleAccent.Render(
			fmt.Sprintf("  â—† %s â†’ %s", e.FromState, e.State)))
	case EventError:
		m.steps = append(m.steps, styleFail.Render(
			fmt.Sprintf("  âœ— %s: %s", e.Check, e.Message)))
	case EventCertified:
		m.certified = true
		m.done = true
		m.progress = 100
	}
}

// â”€â”€ RenderizaÃ§Ã£o â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (m *UIModel) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString(m.logo + "\n")
	b.WriteString(m.title + "\n\n")

	// Estado atual (header).
	if m.state != "" {
		b.WriteString(styleAccent.Render("  Estado: " + string(m.state)) + "\n")
	}

	// Etapas concluÃ­das.
	for _, s := range m.steps {
		b.WriteString(s + "\n")
	}

	// Etapa em andamento (com animaÃ§Ã£o do spinner simplificada).
	if m.current != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", styleWarn.Render("â—‰"), m.current))
	}

	// Barra de progresso (estado real, nÃ£o falsa).
	b.WriteString("\n  " + renderBar(m.progress) + "\n")

	if m.done && m.certified {
		b.WriteString("\n" + styleAccent.Render("  ðŸ§  COSCA IS READY â€” CERTIFIED") + "\n")
	} else if m.done {
		b.WriteString("\n" + styleWarn.Render("  Provisionamento interrompido â€” retomÃ¡vel") + "\n")
	}

	b.WriteString(styleDim.Render("\n  ctrl+c para sair (o estado Ã© persistido)"))
	return b.String()
}

// renderBar desenha uma barra de progresso simples (estado real em %).
func renderBar(pct int) string {
	const width = 40
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	bar := strings.Repeat("â–ˆ", filled) + strings.Repeat("â–‘", width-filled)
	return styleAccent.Render("[" + bar + "]") + fmt.Sprintf(" %d%%", pct)
}

// progressFor estima o progresso pela posiÃ§Ã£o do estado na ordem canÃ´nica.
func progressFor(s State) int {
	idx := stateIndex(s)
	if idx < 0 {
		return 0
	}
	return idx * 100 / (len(Order) - 1)
}

