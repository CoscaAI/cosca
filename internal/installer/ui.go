// Package installer — UI AVANÇADA do provisioner (TUI bubbletea premium).
//
// A visão do professor: "a UI visualiza o que o Provisioner REALMENTE faz".
// Esta TUI premium consome o event stream e renderiza em tempo real com:
//   - spinner animado na etapa em andamento;
//   - barra de progresso dinâmica (estado real, não falsa);
//   - cores por veredito (✓ verde / ⚠ âmbar / ✗ vermelho);
//   - contador de etapas + estado atual;
//   - tela de certificação final (COSCA IS READY).
//
// A animação nunca é falsa: cada frame reflete um evento do orquestrador.
package installer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── estilos (identidade visual do COSCA) ──────────────────────────────────

var (
	styleLogo   = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Bold(true)
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleState  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
)

// uiLogo é a arte ASCII do COSCA (estilo Kernel).
const uiLogo = `
  ██████╗ ██████╗ ███████╗ ██████╗ █████╗
 ██╔════╝██╔═══██╗██╔════╝██╔════╝██╔══██╗
 ██║     ██║   ██║███████╗██║     ███████║
 ██║     ██║   ██║╚════██║██║     ██╔══██║
 ╚██████╗╚██████╔╝███████║╚██████╗██║  ██║
  ╚═════╝ ╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝
`

// tickMsg dispara o próximo frame do spinner (animação real).
type tickMsg timeTick

// timeTick evita colisão de nome com o pacote time.
type timeTick struct{}

// UIModel é o estado da TUI premium do provisioner.
type UIModel struct {
	logo      string
	title     string
	spinner   spinner.Model
	steps     []string
	current   string
	state     State
	progress  int
	done      bool
	certified bool
	mu        sync.Mutex
}

// NewUIModel monta o modelo inicial (com spinner animado).
func NewUIModel() *UIModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleAccent
	return &UIModel{
		logo:    styleLogo.Render(uiLogo),
		title:   styleTitle.Render("COSCA ENVIRONMENT PROVISIONER"),
		spinner: sp,
	}
}

// Emitter devolve o EmitFunc + canal de eventos para o orquestrador.
func (m *UIModel) Emitter() (EmitFunc, chan tea.Msg) {
	ch := make(chan tea.Msg, 128)
	emit := func(e Event) {
		select {
		case ch <- e:
		default: // buffer cheio — descarta (não bloqueia o provisioner)
		}
	}
	return emit, ch
}

// ── bubbletea contract ─────────────────────────────────────────────────────

func (m *UIModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch e := msg.(type) {
	case Event:
		m.applyEvent(e)
		return m, m.spinner.Tick
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if e.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, m.spinner.Tick
}

// applyEvent atualiza o modelo a partir de um evento real do provisioner.
func (m *UIModel) applyEvent(e Event) {
	switch e.Type {
	case EventStepStarted:
		m.current = e.Check
	case EventStepComplete:
		mark := styleOK.Render("✓")
		if e.Result != ResultPass {
			mark = styleWarn.Render("⚠")
		}
		m.steps = append(m.steps, fmt.Sprintf("  %s %-32s %s",
			mark, e.Check, styleDim.Render(string(e.Result))))
		m.current = ""
	case EventStateChanged:
		m.state = e.State
		m.progress = progressFor(e.State)
		m.steps = append(m.steps, styleAccent.Render(
			fmt.Sprintf("  ◆ %s → %s", e.FromState, e.State)))
	case EventError:
		m.steps = append(m.steps, styleFail.Render(
			fmt.Sprintf("  ✗ %s: %s", e.Check, e.Message)))
	case EventCertified:
		m.certified = true
		m.done = true
		m.progress = 100
	}
}

// ── Renderização premium ───────────────────────────────────────────────────

func (m *UIModel) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString(m.logo + "\n")
	b.WriteString(m.title + "\n")

	// Linha de estado + spinner (animação real na etapa em andamento).
	if m.current != "" {
		b.WriteString("\n  " + m.spinner.View() + " " +
			styleState.Render(m.current) + "\n")
	} else if !m.done {
		b.WriteString("\n  " + styleDim.Render("○ aguardando etapa...") + "\n")
	}

	// Estado atual (header com cor).
	if m.state != "" {
		b.WriteString("\n  " + styleAccent.Render("ESTADO: "+string(m.state)) + "\n")
	}

	// Etapas concluídas (últimas 8 para não estourar a tela).
	start := 0
	if len(m.steps) > 8 {
		start = len(m.steps) - 8
	}
	b.WriteString("\n")
	for _, s := range m.steps[start:] {
		b.WriteString(s + "\n")
	}

	// Barra de progresso dinâmica (estado real).
	b.WriteString("\n  " + renderBar(m.progress) + "\n")

	// Contador de etapas.
	b.WriteString("  " + styleDim.Render(fmt.Sprintf("etapas: %d", len(m.steps))) + "\n")

	// Certificação final.
	if m.done && m.certified {
		b.WriteString("\n" + styleAccent.Render("  🧠 COSCA IS READY — CERTIFIED") + "\n")
	} else if m.done {
		b.WriteString("\n" + styleWarn.Render("  Provisionamento interrompido — retomável") + "\n")
	}

	b.WriteString(styleDim.Render("\n  ctrl+c para sair (o estado é persistido)"))
	return b.String()
}

// renderBar desenha a barra de progresso (estado real em %).
func renderBar(pct int) string {
	const width = 40
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return styleAccent.Render("["+bar+"]") + fmt.Sprintf(" %d%%", pct)
}

// progressFor estima o progresso pela posição do estado na ordem canônica.
func progressFor(s State) int {
	idx := stateIndex(s)
	if idx < 0 {
		return 0
	}
	return idx * 100 / (len(Order) - 1)
}
