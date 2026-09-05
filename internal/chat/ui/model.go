package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/engine"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// ─── Model ────────────────────────────────────────────────────────────────────

// Model é o modelo de estado da TUI do chat (binário único `cosca chat`).
type Model struct {
	// engine é o AgentEngine usado para processar as mensagens.
	engine *engine.AgentEngine

	// kernelIdentity é a identidade do Kernel (carregada no boot).
	kernelIdentity kernel.Persona

	// ── Sub-componentes bubbletea ──
	viewport viewport.Model  // histórico rolável
	input    textinput.Model // campo de entrada
	picker   providerPicker  // seletor de modelo (Ctrl+P / /model)
	keyInput textinput.Model // campo de entrada de API key
	spinner  spinner.Model   // indicador de trabalho
	menu     slashMenu       // menu de slash commands (autocomplete)

	// ── Estado da entrada de API key ──
	keyEntry    bool   // modo de coleta de API key ativo
	keyProvider string // provider que está sendo configurado

	// ── Estado da conversa ──
	messages []chatMessage // histórico renderizado
	history  []string      // histórico de entradas (↑/↓)
	histIdx  int           // posição no histórico de entradas

	// ── Estado do streaming ──
	streaming  bool // uma resposta está sendo gerada
	busy       bool // o engine está processando (streaming ou aguardando)
	stopCh     chan struct{}
	evtCh      <-chan engine.EngineEvent // canal de eventos do RunStream
	streamDone bool                      // o canal de eventos foi esgotado

	// ── Buffer da resposta em andamento ──
	currentRole string // "assistant" | "tool" | "subagent" | "system"
	currentText strings.Builder

	// ── Métricas ──
	lastTokens   int
	lastTurns    int
	sessionStart time.Time

	// hasProvider indica se há um modelo configurado no boot (avisado na
	// tela de boas-vindas; o Don conecta via Ctrl+P se ausente).
	hasProvider bool

	// ── Estado da janela ──
	width  int
	height int
	ready  bool

	// ── Execução ──
	ctx    context.Context
	cancel context.CancelFunc

	// quit sinaliza que a TUI deve encerrar.
	quit bool

	// showHelp alterna o painel de ajuda inline.
	showHelp bool
}

// chatMessage é uma mensagem do histórico renderizado.
type chatMessage struct {
	role    string // user | assistant | tool | subagent | system | slash | error | info
	content string
}

// ─── Construtor ───────────────────────────────────────────────────────────────

// New cria uma nova TUI ligada ao engine fornecido.
// reg é o registry de providers usado pelo seletor de modelo (Ctrl+P / /model).
// hasProvider indica se há um provider configurado (se false, a TUI avisa
// para o Don conectar um modelo via Ctrl+P).
func New(eng *engine.AgentEngine, persona kernel.Persona, reg *provider.Registry, hasProvider bool) Model {
	ti := textinput.New()
	ti.Placeholder = "Fale com o Kernel… (ou digite /help)"
	ti.Prompt = "❯ "
	ti.PromptStyle = headerSubStyle
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorGold)
	ti.CharLimit = 4096

	// Campo de API key: máscara para proteger a chave na tela.
	ki := textinput.New()
	ki.Placeholder = "sk-…"
	ki.Prompt = "🔑 "
	ki.EchoMode = textinput.EchoPassword
	ki.PromptStyle = headerSubStyle
	ki.Cursor.Style = lipgloss.NewStyle().Foreground(colorGold)
	ki.CharLimit = 512

	ctx, cancel := context.WithCancel(context.Background())

	// Spinner para o estado de trabalho (estilo do Kernel).
	sp := spinner.New()
	sp.Style = headerSubStyle
	sp.Spinner = spinner.Dot

	m := Model{
		engine:         eng,
		kernelIdentity: persona,
		input:          ti,
		keyInput:       ki,
		picker:         newProviderPicker(reg),
		spinner:        sp,
		history:        make([]string, 0),
		histIdx:        -1,
		stopCh:         make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
		sessionStart:   time.Now(),
		hasProvider:    hasProvider,
	}

	// O foco precisa ser aplicado AQUI (no construtor), não no Init():
	// o Init() recebe o modelo por valor e as mutações são descartadas,
	// deixando o input sem foco — o Don digita e nada entra.
	m.input.Focus()

	return m
}

// ─── Init (bubbletea) ─────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// ─── Update (bubbletea) ───────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Janela redimensionada ──
	case tea.WindowSizeMsg:
		headerHeight := 4
		footerHeight := 1
		inputHeight := 3
		viewportHeight := msg.Height - headerHeight - footerHeight - inputHeight - 2
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.Style = historyViewportStyle
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}
		m.input.Width = msg.Width - 6
		m.keyInput.Width = msg.Width - 6
		m.picker.resize(msg.Width, msg.Height)
		return m, nil

	// ── Tick do spinner ──
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ── Teclas ──
	case tea.KeyMsg:
		// Modo de entrada de API key: repassa as teclas para o keyInput.
		if m.keyEntry {
			return m.handleKeyEntry(msg)
		}

		// Seletor de modelo aberto: repassa as teclas para o picker.
		if m.picker.hasVisible() {
			var pickerCmd tea.Cmd
			m.picker, pickerCmd = m.picker.update(msg)
			if pickerCmd != nil {
				return m, pickerCmd
			}
			return m, nil
		}

		// Ctrl+P: abre o seletor de modelo (estilo opencode).
		if msg.String() == "ctrl+p" {
			if m.picker.hasVisible() {
				m.picker.visible = false
			} else {
				m.picker.open(m.width, m.height)
			}
			return m, nil
		}

		// Menu de slash commands aberto: navegação do menu.
		if m.menu.visible && !m.busy {
			return m.handleMenuKey(msg)
		}

		// Ctrl+C durante streaming: cancela a resposta.
		if m.busy && msg.String() == "ctrl+c" {
			return m, func() tea.Msg {
				close(m.stopCh)
				return stopStreamingMsg{}
			}
		}

		// Ctrl+C parado: encerra com limpeza.
		if msg.String() == "ctrl+c" && !m.busy {
			m.quit = true
			m.cancel()
			return m, tea.Quit
		}

		// Ctrl+D: encerra.
		if msg.String() == "ctrl+d" {
			m.quit = true
			m.cancel()
			return m, tea.Quit
		}

		// Enter: envia a mensagem.
		if msg.String() == "enter" {
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			if strings.HasPrefix(prompt, "/") {
				return m.execCommand(prompt)
			}

			// Prompt normal → envia ao engine em streaming.
			m.history = append(m.history, prompt)
			m.histIdx = len(m.history)
			m.input.SetValue("")
			m.appendMessage("user", prompt)
			m.busy = true
			m.streaming = true
			m.currentRole = "assistant"
			m.currentText.Reset()
			m.lastTokens = 0
			m.lastTurns = 0
			userInput := prompt
			return m, m.runStream(userInput)
		}

		// ↑ / ↓: navega pelo histórico de entradas.
		if msg.String() == "up" && !m.busy {
			if m.histIdx > 0 {
				m.histIdx--
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			}
			return m, nil
		}
		if msg.String() == "down" && !m.busy {
			if m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			} else {
				m.histIdx = len(m.history)
				m.input.SetValue("")
			}
			return m, nil
		}

	// ── Eventos do engine ──
	case engineEventMsg:
		return m.handleEngineEvent(msg)

	case engineStreamStartedMsg:
		m.evtCh = msg.events
		m.streamDone = false
		return m, m.consumeNext()

	case stopStreamingMsg:
		m.busy = false
		m.streaming = false
		m.evtCh = nil
		m.streamDone = true
		m.appendMessage("info", "⏹ Geração cancelada pelo Don.")
		m.cancel()
		m.ctx, m.cancel = context.WithCancel(context.Background())
		m.stopCh = make(chan struct{})
		return m, m.viewportCmd()

	case engineDoneMsg:
		m.busy = false
		m.streaming = false
		m.evtCh = nil
		m.streamDone = true
		if m.lastTokens > 0 || m.lastTurns > 0 {
			m.appendMessage("info",
				fmt.Sprintf("— Tokens: %d · Turns: %d", m.lastTokens, m.lastTurns))
		}
		return m, m.viewportCmd()

	case modelChangeMsg:
		// Troca de provider via seletor: aplica no engine e reporta.
		result := m.applyProviderChange(m.picker.registry, msg.name)
		m.appendMessage("slash", result)
		// Atualiza o marcador de ativo no seletor.
		m.picker.active = msg.name
		m.picker.visible = false
		m.kernelIdentity.Model = msg.name
		return m, m.viewportCmd()

	case apiKeyRequestMsg:
		// Abre o campo de entrada de API key para o provider.
		m.keyEntry = true
		m.keyProvider = msg.name
		m.keyInput.SetValue("")
		m.keyInput.Focus()
		m.keyInput.Placeholder = "Cole a API key do " + msg.name + "…"
		return m, textinput.Blink

	case apiKeySubmitMsg:
		// Chave digitada: salva, conecta e ativa.
		m.keyEntry = false
		m.keyInput.Blur()
		result := m.applyAPIKey(msg.name, msg.key)
		m.appendMessage("slash", result)
		m.kernelIdentity.Model = msg.name
		return m, m.viewportCmd()
	}

	// Input focado: repassa as teclas ao textinput.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Abre o menu de slash commands quando o input começa com "/".
	if !m.busy && !m.keyEntry && !m.picker.hasVisible() {
		if v := m.input.Value(); strings.HasPrefix(v, "/") {
			if !m.menu.visible {
				m.menu.visible = true
				m.menu.cursor = 0
				m.menu.hasFocus = true
			}
			m.menu.query = strings.TrimPrefix(v, "/")
			m.menu = m.menu.clampCursor()
		} else if m.menu.visible {
			// O "/" foi apagado → fecha o menu.
			m.menu.visible = false
			m.menu.query = ""
			m.menu.hasFocus = false
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKeyEntry processa teclas durante o modo de coleta de API key.
func (m Model) handleKeyEntry(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		key := strings.TrimSpace(m.keyInput.Value())
		if key == "" {
			return m, nil
		}
		name := m.keyProvider
		m.keyEntry = false
		m.keyInput.Blur()
		m.keyInput.SetValue("")
		return m, func() tea.Msg {
			return apiKeySubmitMsg{name: name, key: key}
		}
	case "esc":
		// Cancela a entrada de chave.
		m.keyEntry = false
		m.keyInput.Blur()
		m.keyInput.SetValue("")
		return m, nil
	}

	var cmd tea.Cmd
	m.keyInput, cmd = m.keyInput.Update(msg)
	return m, cmd
}

// handleMenuKey processa teclas enquanto o menu de slash commands está aberto.
// ↑/↓ navegam, Enter executa, Tab completa, Esc fecha. As demais teclas seguem
// para o input (texto real) e o filtro é derivado do valor dele — como o
// opencode faz.
func (m Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.menu.visible = false
		m.menu.query = ""
		m.menu.hasFocus = false
		return m, nil

	case "up", "down":
		items := m.menu.filtered()
		if len(items) == 0 {
			return m, nil
		}
		if msg.String() == "up" {
			m.menu.cursor--
		} else {
			m.menu.cursor++
		}
		m.menu = m.menu.clampCursor()
		return m, nil

	case "tab":
		// Completa o comando selecionado no input.
		if full := m.menu.completeTo(); full != "" {
			m.input.SetValue(full)
			m.input.CursorEnd()
		}
		return m, nil

	case "enter":
		items := m.menu.filtered()
		if len(items) == 0 {
			m.menu.visible = false
			m.menu.query = ""
			return m, nil
		}
		if m.menu.cursor >= len(items) {
			m.menu.cursor = len(items) - 1
		}
		selected := items[m.menu.cursor].name
		m.menu.visible = false
		m.menu.query = ""
		m.menu.hasFocus = false
		return m.execCommand(selected)
	}

	// Demais teclas: repassa ao input (texto real) e re-deriva o filtro.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.menu.query = strings.TrimPrefix(m.input.Value(), "/")
	m.menu.cursor = 0
	m.menu = m.menu.clampCursor()
	return m, cmd
}

// execCommand executa um comando slash completo (do menu ou digitado).
func (m Model) execCommand(cmd string) (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(cmd)
	if prompt == "" {
		return m, nil
	}
	m.history = append(m.history, prompt)
	m.histIdx = len(m.history)
	m.input.SetValue("")

	result := renderSlashResult(m, prompt)
	if isSlashAction(result) {
		updated, quit := m.handleSlashAction(result)
		if quit {
			updated.quit = true
			updated.cancel()
			return updated, tea.Quit
		}
		return updated, updated.viewportCmd()
	}
	role := "slash"
	if strings.HasPrefix(result, "\x00error") {
		role = "error"
		result = strings.TrimPrefix(result, "\x00error")
	}
	m.appendMessage(role, result)
	return m, m.viewportCmd()
}

// ─── View (bubbletea) ─────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return ""
	}

	// Cabeçalho — identidade do Kernel + status do sistema.
	header := m.renderHeader()

	// Corpo — boas-vindas ou histórico rolável.
	var body strings.Builder
	if len(m.messages) == 0 && !m.busy {
		body.WriteString(m.renderWelcome())
	} else {
		for _, msg := range m.messages {
			body.WriteString(m.renderMessage(msg))
			body.WriteString("\n\n")
		}
	}
	// Resposta em andamento.
	if m.streaming {
		body.WriteString(renderStreaming(m.currentRole, m.currentText.String()))
		body.WriteString("\n")
	}
	// Indicador de trabalho com spinner.
	if m.busy && !m.streaming {
		body.WriteString(m.spinner.View() + " " + infoBubble.Render("processando… (Ctrl+C para cancelar)"))
		body.WriteString("\n")
	}
	if m.showHelp {
		body.WriteString(renderHelp())
		body.WriteString("\n")
	}

	m.viewport.SetContent(body.String())

	// Rodapé.
	footer := m.renderFooter()

	// Modo de entrada de API key: mostra o campo de chave no lugar do input.
	if m.keyEntry {
		keyHint := infoBubble.Render(fmt.Sprintf("Conectando %s — Enter salva · Esc cancela", m.keyProvider))
		return appStyle.Render(
			header + "\n" +
				m.viewport.View() + "\n" +
				inputBoxStyle.Render(m.keyInput.View()) + "\n" +
				keyHint,
		)
	}

	// Seletor de modelo sobreposto (estilo opencode).
	if m.picker.hasVisible() {
		return appStyle.Render(
			header + "\n" +
				m.picker.View() + "\n" +
				inputBoxStyle.Render(m.input.View()) + "\n" +
				footer,
		)
	}

	// Input: moldura dourada quando focado.
	inputStyle := inputBoxStyle
	if m.input.Focused() {
		inputStyle = inputFocusedStyle
	}

	// Menu de slash commands acima do input.
	menuBlock := ""
	if m.menu.visible {
		menuBlock = m.menu.View(m.width) + "\n"
	}

	return appStyle.Render(
		header + "\n" +
			m.viewport.View() + "\n" +
			menuBlock +
			inputStyle.Render(m.input.View()) + "\n" +
			footer,
	)
}

// ─── Renderização de componentes ──────────────────────────────────────────────

// renderHeader monta a barra superior com a identidade do Kernel (linha 1)
// e o status do sistema — modelo ativo, sessão, streaming (linha 2).
func (m Model) renderHeader() string {
	p := m.kernelIdentity

	// Linha 1: identidade.
	left := headerStyle.Render(fmt.Sprintf(" ☯ %s", p.Name))
	right := headerSubStyle.Render(fmt.Sprintf(" %s · %s ", p.Role, p.Version))
	sep := " "
	if sepWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2; sepWidth > 0 {
		sep = strings.Repeat("─", sepWidth)
	}
	line1 := lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)

	// Linha 2: status do sistema.
	model := m.picker.activeName()
	if model == "" {
		model = "engine local"
	}
	status := "pronto"
	statusColor := colorGreen
	if m.busy {
		status = "ocupado"
		statusColor = colorGold
	}
	if m.streaming {
		status = "gerando"
		statusColor = colorCyan
	}
	modelTag := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).
		Render(" model: " + model + " ")
	statusTag := lipgloss.NewStyle().Foreground(colorBackground).Background(statusColor).
		Bold(true).Padding(0, 1).Render(" " + status + " ")
	sessionTag := lipgloss.NewStyle().Foreground(colorGrayLight).
		Render(fmt.Sprintf(" sessão %s ", time.Since(m.sessionStart).Round(time.Second)))

	line2 := lipgloss.JoinHorizontal(lipgloss.Left,
		modelTag, statusTag, sessionTag,
	)

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

// renderWelcome monta a tela de boas-vindas com o logo ASCII e a identidade.
func (m Model) renderWelcome() string {
	p := m.kernelIdentity
	var b strings.Builder

	// Logo ASCII em destaque.
	b.WriteString(renderLogo())
	b.WriteString("\n\n")

	// Identidade e papel.
	b.WriteString(welcomeTitle.Render(p.Name + " — " + p.Role))
	b.WriteString("\n\n")

	if !m.hasProvider {
		b.WriteString(welcomeHint.Render("⚠ Nenhum modelo configurado. Pressione Ctrl+P (ou /model)"))
		b.WriteString("\n")
		b.WriteString(welcomeHint.Render("  para escolher e conectar um provider."))
		b.WriteString("\n\n")
	}

	// Dicas de uso.
	b.WriteString(welcomeHint.Render("Digite / para ver os comandos · Ctrl+P escolhe o modelo"))
	b.WriteString("\n")
	b.WriteString(welcomeHint.Render("↑/↓ histórico · Ctrl+C cancela · Ctrl+D sai"))
	return welcomeBox.Render(b.String())
}

// renderFooter monta a barra inferior com dicas contextuais.
func (m Model) renderFooter() string {
	model := m.picker.activeName()
	if model == "" {
		model = "engine local"
	}
	info := fmt.Sprintf("%d msgs", len(m.messages))
	if m.lastTokens > 0 {
		info += fmt.Sprintf(" · %d tokens", m.lastTokens)
	}
	if m.lastTurns > 0 {
		info += fmt.Sprintf(" · %d turns", m.lastTurns)
	}
	hint := fmt.Sprintf(" /help ajuda · Ctrl+P modelo · ↑/↓ histórico · Ctrl+D sai · %s", info)
	return footerStyle.Render(hint)
}

// renderMessage renderiza uma mensagem do histórico conforme o papel.
func (m Model) renderMessage(msg chatMessage) string {
	ts := timestampStyle.Render(time.Now().Format("15:04"))
	switch msg.role {
	case "user":
		return userBubbleBox.Render(msg.content)
	case "assistant":
		return assistantBubble.Render(msg.content)
	case "tool":
		return toolCallBox.Render("⚡ " + msg.content)
	case "subagent":
		return subagentBubble.Render(msg.content)
	case "system":
		return kernelBadgeStyle.Render(msg.content)
	case "slash":
		return slashBubble.Render(msg.content)
	case "error":
		return errorBubble.Render("✗ " + msg.content)
	case "info":
		return infoBubble.Render(msg.content + "  " + ts)
	default:
		return msg.content
	}
}

// appendMessage adiciona uma mensagem ao histórico renderizado.
func (m *Model) appendMessage(role, content string) {
	m.messages = append(m.messages, chatMessage{role: role, content: content})
}

// viewportCmd garante que o viewport role até o fim após atualizações.
func (m Model) viewportCmd() tea.Cmd {
	return viewport.Sync(m.viewport)
}

// ─── Streaming: envia o prompt ao engine e captura eventos ────────────────────

// runStream inicia a execução do engine em streaming e registra o canal de
// eventos no modelo. O fluxo de eventos é consumido via consumeNext, que
// reencadeia a si mesmo até o canal fechar (engineDoneMsg) ou o Don cancelar
// (stopStreamingMsg).
func (m Model) runStream(prompt string) tea.Cmd {
	return func() tea.Msg {
		events, err := m.engine.RunStream(m.ctx, prompt, nil)
		if err != nil {
			return engineEventMsg{event: &engine.EngineEvent{
				Type:  engine.EngineEventError,
				Error: err,
			}}
		}
		// Armazena o canal no modelo via mensagem de setup.
		return engineStreamStartedMsg{events: events}
	}
}

// engineStreamStartedMsg registra o canal de eventos do RunStream no modelo.
type engineStreamStartedMsg struct {
	events <-chan engine.EngineEvent
}

// consumeNext consome o próximo evento do canal e o reencaminha para o loop.
// Quando o canal fecha, emite engineDoneMsg. Quando o Don cancela, emite
// stopStreamingMsg. Como cada evento reencadeia a si mesmo, o streaming
// avança evento a evento sem bloquear o loop bubbletea.
func (m Model) consumeNext() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.stopCh:
			return stopStreamingMsg{}
		case evt, ok := <-m.evtCh:
			if !ok {
				return engineDoneMsg{}
			}
			return engineEventMsg{event: &evt}
		}
	}
}

// handleEngineEvent processa um evento do engine e emite comandos de continuação.
func (m Model) handleEngineEvent(msg engineEventMsg) (tea.Model, tea.Cmd) {
	evt := msg.event
	if evt == nil {
		return m, m.consumeNext()
	}

	switch evt.Type {
	case engine.EngineEventContent:
		m.currentText.WriteString(evt.Content)
		m.streaming = true
		// Continua consumindo o próximo evento.
		return m, m.consumeNext()

	case engine.EngineEventToolStart:
		// Fecha a resposta em andamento e mostra a tool call.
		m.flushCurrent()
		name := "unknown"
		if evt.ToolCall != nil {
			name = evt.ToolCall.Name
		}
		m.currentRole = "tool"
		m.currentText.Reset()
		m.currentText.WriteString(fmt.Sprintf("⚡ %s", name))
		m.streaming = true
		return m, m.consumeNext()

	case engine.EngineEventToolResult:
		m.flushCurrent()
		status := "done"
		detail := ""
		if evt.ToolResult != nil {
			if evt.ToolResult.Status == "error" {
				status = "falhou"
			}
			if evt.ToolResult.Error != "" {
				detail = ": " + evt.ToolResult.Error
			}
			if out, ok := evt.ToolResult.Output.(string); ok && out != "" && len(out) < 120 {
				detail = ": " + out
			}
		}
		m.appendMessage("info", fmt.Sprintf("   → %s %s", status, detail))
		m.streaming = false
		return m, m.consumeNext()

	case engine.EngineEventSubagentStart:
		m.flushCurrent()
		m.currentRole = "subagent"
		m.currentText.Reset()
		m.currentText.WriteString("⇄ subagente acionado")
		m.streaming = true
		return m, m.consumeNext()

	case engine.EngineEventSubagentResult:
		m.flushCurrent()
		m.streaming = false
		return m, m.consumeNext()

	case engine.EngineEventTurnEnd:
		if evt.TurnRecord != nil {
			m.lastTurns = evt.TurnRecord.TurnNumber
			if evt.TurnRecord.TokensUsed > m.lastTokens {
				m.lastTokens = evt.TurnRecord.TokensUsed
			}
		}
		return m, m.consumeNext()

	case engine.EngineEventDone:
		m.flushCurrent()
		m.streaming = false
		m.busy = false
		return m, m.viewportCmd()

	case engine.EngineEventError:
		m.flushCurrent()
		if evt.Error != nil {
			m.appendMessage("error", evt.Error.Error())
		}
		m.streaming = false
		m.busy = false
		return m, m.viewportCmd()
	}

	return m, m.consumeNext()
}

// flushCurrent move o buffer de resposta em andamento para o histórico.
func (m *Model) flushCurrent() {
	if m.currentText.Len() == 0 {
		m.streaming = false
		return
	}
	text := strings.TrimSpace(m.currentText.String())
	if text != "" {
		m.appendMessage(m.currentRole, text)
	}
	m.currentText.Reset()
	m.streaming = false
}

// ─── Mensagens internas ───────────────────────────────────────────────────────

// engineEventMsg transporta um evento do engine para o loop bubbletea.
type engineEventMsg struct {
	event *engine.EngineEvent
}

// engineDoneMsg sinaliza o fim do fluxo de eventos.
type engineDoneMsg struct{}

// stopStreamingMsg sinaliza que a geração foi cancelada.
type stopStreamingMsg struct{}

// Compile-time checks.
var _ tea.Model = Model{}
var _ = executor.ToolCall{} // manter o import usado por subagentes/tools
