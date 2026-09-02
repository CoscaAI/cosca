package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/trace"
)

// ─── Panel (Tab) System ───────────────────────────────────────────────────────

type PanelID int

const (
	PanelChat PanelID = iota
	PanelTasks
	PanelAgents
	PanelOperations
	PanelFiles
	PanelDiff
	PanelSystem
)

var panelNames = map[PanelID]string{
	PanelChat:       "Chat",
	PanelTasks:      "Tasks",
	PanelAgents:     "Agents",
	PanelOperations: "Operations",
	PanelFiles:      "Files",
	PanelDiff:       "Diff",
	PanelSystem:     "System",
}

var panelKeys = map[string]PanelID{
	"1": PanelChat,
	"2": PanelTasks,
	"3": PanelAgents,
	"4": PanelOperations,
	"5": PanelFiles,
	"6": PanelSystem,
}

// ─── Message ───────────────────────────────────────────────────────────────────

type Message struct {
	Role    string
	Content string
	Time    time.Time
}

// ─── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	chatViewport viewport.Model
	chatInput    textinput.Model
	messages     []Message

	tasks        []TaskInfo
	taskViewport viewport.Model

	files       []FileEntry
	fileEntries []FileEntry

	diffs       []DiffEntry
	diffEntries []DiffEntry

	hud HUDInfo

	runner       pipeline.Runner
	planner      *pipeline.Planner
	stepRunner   *pipeline.StepRunner
	recoveryLoop *pipeline.RecoveryLoop

	sessionCtx *pipeline.TerminalContext

	advancedMode bool

	width  int
	height int
	ready  bool

	currentAgent string
	currentModel string

	streaming bool
	streamCh  <-chan pipeline.RunEvent

	operations     *OperationsPanel
	frame          int
	background     bool
	currentTraceID string
	pendingPrompts []string

	fabric *compute.Fabric

	progress float64

	spinner        spinner.Model
	busy           bool
	currentRole    string
	currentText    strings.Builder
	activeTaskName string

	history []string
	histIdx int

	ctx    context.Context
	cancel context.CancelFunc
	quit   bool

	activePanel   PanelID
	panels        []PanelID
	filesSelected int
	diffSelected  int
	diffDirty     bool

	breadcrumbs []string

	sessionStart time.Time
	totalCost    float64
	branch       string

	palette              PaletteModel
	paletteOpen          bool
	showPermissionDialog bool
	pendingToolCall      interface{}
	themeName            string
	modeIndicator        string
	multiLineMode        bool
	fileSuggestions      []string
}

// ─── Constructor ───────────────────────────────────────────────────────────────

// ModelConfig holds optional constructor parameters.
type ModelConfig struct {
	AdvancedMode bool
	CurrentModel string
	CurrentAgent string
	Output       io.Writer
	Fabric       *compute.Fabric
}

// configureColorProfile binds Lipgloss to the writer Bubble Tea renders to.
// The default termenv output can have cached a non-TTY profile before the
// program is started, so using its EnvColorProfile here can permanently strip
// colors from a TTY session.
func configureColorProfile(output io.Writer) {
	if output == nil {
		output = os.Stdout
	}

	profile := termenv.Ascii
	if !colorDisabled() && (isTerminalWriter(output) || colorForced()) {
		profile = environmentColorProfile()
	}

	renderer := lipgloss.DefaultRenderer()
	renderer.SetOutput(termenv.NewOutput(output, termenv.WithProfile(profile)))
	renderer.SetColorProfile(profile)
}

func isTerminalWriter(output io.Writer) bool {
	file, ok := output.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func colorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return os.Getenv("CLICOLOR") == "0" && !colorForced()
}

func colorForced() bool {
	forced := os.Getenv("CLICOLOR_FORCE")
	return forced != "" && forced != "0"
}

func environmentColorProfile() termenv.Profile {
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	switch colorTerm {
	case "truecolor", "24bit":
		return termenv.TrueColor
	case "true", "yes":
		return termenv.ANSI256
	}

	termName := strings.ToLower(os.Getenv("TERM"))
	switch {
	case termName == "dumb":
		return termenv.Ascii
	case strings.Contains(termName, "256color"):
		return termenv.ANSI256
	case strings.Contains(termName, "color"), strings.Contains(termName, "ansi"), termName == "xterm":
		return termenv.ANSI
	default:
		// A TTY without a useful TERM value still supports basic ANSI color.
		return termenv.ANSI
	}
}

func fillTrailingBackground(s string, background lipgloss.Color) string {
	trimmed := strings.TrimRight(s, " ")
	if len(trimmed) == len(s) {
		return s
	}
	return trimmed + lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", len(s)-len(trimmed)))
}

func New(
	runner pipeline.Runner,
	planner *pipeline.Planner,
	stepRunner *pipeline.StepRunner,
	recoveryLoop *pipeline.RecoveryLoop,
	termCtx *pipeline.TerminalContext,
	cfg ModelConfig,
) Model {
	configureColorProfile(cfg.Output)

	ti := textinput.New()
	ti.Placeholder = "cosca> type a task or / for commands or : for palette..."
	ti.Prompt = "❯ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextMuted).Background(th.InputFocusedBackground)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground)
	ti.CharLimit = 4096

	sp := spinner.New()
	sp.Style = titleSubStyle
	sp.Spinner = spinner.Dot

	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		runner:       runner,
		planner:      planner,
		stepRunner:   stepRunner,
		recoveryLoop: recoveryLoop,
		sessionCtx:   termCtx,
		chatInput:    ti,
		spinner:      sp,
		operations:   NewOperationsPanel(),
		messages:     make([]Message, 0),
		tasks:        make([]TaskInfo, 0),
		history:      make([]string, 0),
		histIdx:      -1,
		ctx:          ctx,
		cancel:       cancel,
		advancedMode: cfg.AdvancedMode,
		currentModel: cfg.CurrentModel,
		currentAgent: cfg.CurrentAgent,
		fabric:       cfg.Fabric,
		sessionStart: time.Now(),

		activePanel: PanelChat,
		panels:      []PanelID{PanelChat, PanelTasks, PanelOperations, PanelFiles, PanelDiff},
		breadcrumbs: []string{"Chat"},
		branch:      "",

		palette:       NewPalette(),
		paletteOpen:   false,
		themeName:     "cosca",
		modeIndicator: "PLAN",
		multiLineMode: false,
	}

	if termCtx != nil {
		m.branch = gitBranch(termCtx.ProjectPath)
	}

	m.chatInput.Focus()
	return m
}

// ─── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// frameTick drives the animation frame counter for the operations panel.
func frameTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return frameTickMsg{}
	})
}

// ─── Update ─────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		titleHeight := 2
		tabHeight := 1
		breadcrumbHeight := 1
		hudHeight := 1
		inputHeight := 3
		layoutSeparators := 5

		viewportHeight := msg.Height - titleHeight - tabHeight - breadcrumbHeight - hudHeight - inputHeight - layoutSeparators
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		if !m.ready {
			m.chatViewport = viewport.New(msg.Width, viewportHeight)
			m.chatViewport.Style = chatViewportStyle
			m.taskViewport = viewport.New(30, viewportHeight)
			m.ready = true
		} else {
			m.chatViewport.Width = msg.Width
			m.chatViewport.Height = viewportHeight
			m.taskViewport.Width = 30
			m.taskViewport.Height = viewportHeight
		}

		m.chatInput.Width = msg.Width - 6

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case frameTickMsg:
		if m.busy || m.background {
			m.frame++
			m.operations.SetFrame(m.frame)
			return m, frameTick()
		}
		return m, nil

	case tea.KeyMsg:
		if m.quit {
			return m, nil
		}

		if msg.String() == "ctrl+p" {
			if m.paletteOpen {
				m.palette = m.palette.Close()
				m.paletteOpen = false
				m.chatInput.Focus()
			} else {
				m.palette = m.palette.Open()
				m.paletteOpen = true
			}
			return m, nil
		}

		if msg.String() == "ctrl+k" {
			if m.modeIndicator == "PLAN" {
				m.modeIndicator = "BUILD"
			} else {
				m.modeIndicator = "PLAN"
			}
			return m, nil
		}

		if msg.String() == "ctrl+e" {
			if m.busy {
				return m, nil
			}
			m.multiLineMode = !m.multiLineMode
			if m.multiLineMode {
				m.appendMessage("info", "Multi-line mode enabled. Press Ctrl+E again to disable.")
			} else {
				m.appendMessage("info", "Multi-line mode disabled.")
			}
			return m, m.viewportCmd()
		}

		if m.paletteOpen {
			switch msg.String() {
			case "esc":
				m.palette = m.palette.Close()
				m.paletteOpen = false
				m.chatInput.Focus()
				return m, nil
			case "enter":
				selected := m.palette.SelectedCommand()
				if selected != nil {
					m.palette = m.palette.Close()
					m.paletteOpen = false
					m.chatInput.Focus()
					return m.handlePaletteAction(selected.ID)
				}
				return m, nil
			case "up":
				if m.palette.cursor > 0 {
					m.palette.cursor--
				}
				return m, nil
			case "down":
				if m.palette.cursor < len(m.palette.filtered)-1 {
					m.palette.cursor++
				}
				return m, nil
			}
			var pc tea.Cmd
			m.palette, pc = m.palette.Update(msg)
			if pc != nil {
				return m, pc
			}
			return m, nil
		}

		if msg.String() == "ctrl+c" && m.busy {
			return m, func() tea.Msg { return stopStreamingMsg{} }
		}

		if msg.String() == "ctrl+c" && !m.busy {
			m.quit = true
			m.cancel()
			return m, tea.Quit
		}

		if msg.String() == "ctrl+d" && m.busy {
			return m, func() tea.Msg { return stopStreamingMsg{} }
		}

		if msg.String() == "ctrl+d" && !m.busy {
			m.activePanel = PanelDiff
			m.breadcrumbs = []string{"Diff"}
			if m.sessionCtx != nil && len(m.diffEntries) == 0 {
				m.diffEntries = GetGitDiffs(m.sessionCtx.ProjectPath)
			}
			m.diffDirty = false
			return m, nil
		}

		if msg.String() == "ctrl+f" {
			m.activePanel = PanelFiles
			m.breadcrumbs = []string{"Files"}
			if m.sessionCtx != nil && len(m.fileEntries) == 0 {
				m.fileEntries = ScanModifiedFiles(m.sessionCtx.ProjectPath)
			}
			m.diffDirty = false
			return m, nil
		}

		if msg.String() == "ctrl+q" {
			m.quit = true
			m.cancel()
			return m, tea.Quit
		}

		if msg.String() == "ctrl+t" {
			if m.activePanel == PanelTasks {
				m.activePanel = PanelChat
				m.breadcrumbs = []string{"Chat"}
			} else {
				m.activePanel = PanelTasks
				m.breadcrumbs = []string{"Tasks"}
			}
			return m, nil
		}

		if msg.String() == "ctrl+o" {
			if m.activePanel == PanelOperations {
				m.activePanel = PanelChat
				m.breadcrumbs = []string{"Chat"}
			} else {
				m.activePanel = PanelOperations
				m.breadcrumbs = []string{panelNames[PanelOperations]}
			}
			return m, nil
		}

		// Tab cycling (allowed while busy so the user can watch operations)
		if msg.String() == "tab" {
			idx := indexOfPanel(m.panels, m.activePanel)
			idx = (idx + 1) % len(m.panels)
			m.activePanel = m.panels[idx]
			m.breadcrumbs = []string{panelNames[m.activePanel]}
			return m, nil
		}

		// Shift+Tab = reverse tab cycling
		if msg.String() == "shift+tab" {
			idx := indexOfPanel(m.panels, m.activePanel)
			idx--
			if idx < 0 {
				idx = len(m.panels) - 1
			}
			m.activePanel = m.panels[idx]
			m.breadcrumbs = []string{panelNames[m.activePanel]}
			return m, nil
		}

		// Number keys for direct panel access
		if p, ok := panelKeys[msg.String()]; ok {
			if containsPanel(m.panels, p) {
				m.activePanel = p
				m.breadcrumbs = []string{panelNames[p]}
				return m, nil
			}
		}

		// Operations tree navigation
		if m.activePanel == PanelOperations {
			switch msg.String() {
			case "up", "k":
				m.operations.Tree().MoveUp()
				return m, nil
			case "down", "j":
				m.operations.Tree().MoveDown()
				return m, nil
			case "enter", "right", "l":
				m.operations.Tree().ToggleExpand()
				return m, nil
			case "left", "h":
				if m.operations.Tree().Selected() != nil {
					m.operations.Tree().ToggleExpand()
				}
				return m, nil
			}
		}

		if (m.activePanel == PanelFiles || m.activePanel == PanelDiff) && !m.busy {
			switch msg.String() {
			case "up", "k", "down", "j":
				if m.activePanel == PanelDiff {
					if msg.String() == "up" || msg.String() == "k" {
						if m.diffSelected > 0 {
							m.diffSelected--
						}
					} else if m.diffSelected < len(m.diffEntries)-1 {
						m.diffSelected++
					}
				} else {
					if msg.String() == "up" || msg.String() == "k" {
						if m.filesSelected > 0 {
							m.filesSelected--
						}
					} else if m.filesSelected < len(m.fileEntries)-1 {
						m.filesSelected++
					}
				}
				return m, nil
			}
		}

		if msg.String() == "enter" {
			input := strings.TrimSpace(m.chatInput.Value())
			if input == "" {
				return m, nil
			}

			if strings.HasPrefix(input, "/") {
				return m.execSlashCommand(input)
			}

			if strings.HasPrefix(input, ":") {
				return m.execColonCommand(input)
			}

			m.history = append(m.history, input)
			m.histIdx = len(m.history)
			m.chatInput.SetValue("")
			m.appendMessage("user", input)

			// The input stays focused the whole time work runs; if a task is
			// already executing, the new prompt is queued and starts when the
			// current run finishes (background execution).
			if m.busy {
				m.pendingPrompts = append(m.pendingPrompts, input)
				m.appendMessage("info", "Queued — will run when the current task finishes (Ctrl+C to cancel).")
				return m, nil
			}

			m.busy = true
			m.streaming = true
			m.background = true
			m.currentRole = "assistant"
			m.currentText.Reset()
			m.currentTraceID = trace.NewID().String()
			m.hud.Phase = PhasePlanning
			m.hud.PhasePct = 0.1

			return m, tea.Batch(m.runPipelineStream(input), frameTick())
		}

		if msg.String() == "up" && !m.busy {
			if m.histIdx > 0 {
				m.histIdx--
				m.chatInput.SetValue(m.history[m.histIdx])
				m.chatInput.CursorEnd()
			}
			return m, nil
		}

		if msg.String() == "down" && !m.busy {
			if m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.chatInput.SetValue(m.history[m.histIdx])
				m.chatInput.CursorEnd()
			} else {
				m.histIdx = len(m.history)
				m.chatInput.SetValue("")
			}
			return m, nil
		}

	case pipelineEventMsg:
		return m.handlePipelineEvent(msg.event)

	case pipelineStreamStartedMsg:
		m.streamCh = msg.events
		return m, m.consumeNextEvent()

	case stopStreamingMsg:
		m.busy = false
		m.streaming = false
		m.background = false
		m.streamCh = nil
		m.activeTaskName = ""
		m.hud.Phase = PhaseIdle
		m.pendingPrompts = nil
		m.appendMessage("info", "Cancelled.")
		return m, m.viewportCmd()

	case pipelineDoneMsg:
		m.busy = false
		m.streaming = false
		m.background = false
		m.streamCh = nil
		m.activeTaskName = ""
		m.hud.Phase = PhaseDone

		if len(m.pendingPrompts) > 0 {
			next := m.pendingPrompts[0]
			m.pendingPrompts = m.pendingPrompts[1:]
			m.busy = true
			m.streaming = true
			m.background = true
			m.currentRole = "assistant"
			m.currentText.Reset()
			m.currentTraceID = trace.NewID().String()
			m.hud.Phase = PhasePlanning
			m.hud.PhasePct = 0.1
			m.appendMessage("info", "Running next queued task...")
			return m, tea.Batch(m.runPipelineStream(next), frameTick())
		}

		return m, m.viewportCmd()
	}

	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return ""
	}

	header := m.renderHeader()
	tabBar := m.renderTabBar()
	breadcrumbs := m.renderBreadcrumbs()

	var body strings.Builder
	if len(m.messages) == 0 && !m.busy {
		body.WriteString(m.renderWelcome())
	} else {
		for _, msg := range m.messages {
			body.WriteString(m.renderMessage(msg))
			body.WriteString("\n\n")
		}
	}

	if m.streaming {
		body.WriteString(renderStreamingContent(m.currentRole, m.currentText.String()))
	}

	if m.busy && !m.streaming {
		body.WriteString(m.spinner.View() + " " + infoBubble.Render("processing... (Ctrl+C to cancel)"))
	}

	m.chatViewport.SetContent(body.String())

	inputStyle := inputBoxStyle
	if m.chatInput.Focused() {
		inputStyle = inputFocusedStyle
	}
	inputBackground := th.InputBackground
	if m.chatInput.Focused() {
		inputBackground = th.InputFocusedBackground
	}
	inputView := fillTrailingBackground(m.chatInput.View(), inputBackground)

	// Update HUD before rendering.
	m.hud.Model = m.currentModel
	m.hud.Agent = m.currentAgent
	if m.busy {
		m.hud.Status = "busy"
	} else if m.streaming {
		m.hud.Status = "streaming"
	} else {
		m.hud.Status = "ready"
	}
	m.hud.ActiveTask = m.activeTaskName
	m.hud.Elapsed = time.Since(m.sessionStart)
	m.hud.Branch = m.branch

	sidebarW := 30

	// Palette is a centered overlay: the viewport shrinks while it is open so
	// the palette never pushes the HUD below the fold.
	paletteView := ""
	vpHeight := m.chatViewport.Height
	if m.paletteOpen {
		pv := m.palette.View()
		ph := lipgloss.Height(pv)
		if vpHeight > ph {
			vpHeight -= ph
		}
		if vpHeight < 5 {
			vpHeight = 5
		}
		padL := (m.width - lipgloss.Width(pv)) / 2
		if padL < 0 {
			padL = 0
		}
		padR := m.width - lipgloss.Width(pv) - padL
		if padR < 0 {
			padR = 0
		}
		paletteView = lipgloss.NewStyle().
			Background(th.BackgroundPanel).
			PaddingLeft(padL).
			PaddingRight(padR).
			Render(pv)
	}

	chatW := m.width - sidebarW
	if chatW < 20 {
		chatW = m.width
	}

	rightPanel := ""
	switch m.activePanel {
	case PanelFiles:
		rightPanel = FilesPanelView(m.fileEntries, sidebarW, vpHeight, m.filesSelected)
	case PanelDiff:
		rightPanel = DiffPanelView(m.diffEntries, sidebarW, vpHeight, m.diffSelected)
	case PanelTasks:
		rightPanel = TaskPanelView(m.tasks, sidebarW, vpHeight)
	case PanelOperations:
		rightPanel = m.operations.Render(sidebarW, vpHeight)
	}

	vpCopy := m.chatViewport
	vpCopy.Width = chatW
	vpCopy.Height = vpHeight
	vpCopy.Style = chatViewportStyle

	paddedVP := chatViewportStyle.Width(chatW).Render(vpCopy.View())

	var mainArea string
	if rightPanel != "" {
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, paddedVP, rightPanel)
	} else {
		mainArea = paddedVP
	}

	return appStyle.Width(m.width).Height(m.height).Render(
		header + "\n" +
			tabBar + "\n" +
			breadcrumbs + "\n" +
			mainArea + "\n" +
			paletteView + "\n" +
			inputStyle.Render(inputView) + "\n" +
			HudView(m.hud, m.width, m.frame),
	)
}

// ─── Tab Bar ───────────────────────────────────────────────────────────────────

func (m Model) renderTabBar() string {
	var tabs []string
	for _, p := range m.panels {
		name := panelNames[p]
		if p == PanelDiff && m.diffDirty {
			name = "• " + name
		}
		if p == m.activePanel {
			tabs = append(tabs, tabActiveStyle.Render(" "+name+" "))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render(" "+name+" "))
		}
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	fillWidth := m.width - lipgloss.Width(joined)
	if fillWidth < 0 {
		fillWidth = 0
	}

	fill := lipgloss.NewStyle().Background(th.BgAlt).Render(strings.Repeat(" ", fillWidth))
	return tabBarStyle.Width(m.width).Render(joined + fill)
}

// ─── Breadcrumbs ───────────────────────────────────────────────────────────────

func (m Model) renderBreadcrumbs() string {
	if len(m.breadcrumbs) == 0 {
		return ""
	}

	var parts []string
	for i, crumb := range m.breadcrumbs {
		if i == len(m.breadcrumbs)-1 {
			parts = append(parts, breadcrumbActiveStyle.Render(crumb))
		} else {
			parts = append(parts, breadcrumbStyle.Render(crumb))
		}
		if i < len(m.breadcrumbs)-1 {
			parts = append(parts, breadcrumbStyle.Render(" > "))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// ─── Header ────────────────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	left := titleStyle.Render(" ◉ COSCA TERMINAL ")
	agent := m.currentAgent
	if agent == "" {
		agent = "kernel"
	}
	right := titleSubStyle.Render(fmt.Sprintf(" %s · %s ", agent, m.currentModel))

	sepW := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if sepW < 0 {
		sepW = 0
	}
	sep := titleDividerStyle.Render(strings.Repeat("─", sepW))
	line1 := lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)

	status := "ready"
	statusColor := colorGreen
	if m.busy {
		status = "processing"
		statusColor = colorGold
	}
	if m.streaming {
		status = "generating"
		statusColor = colorCyan
	}
	modeTag := "simple"
	if m.advancedMode {
		modeTag = "advanced"
	}

	statusTag := lipgloss.NewStyle().
		Foreground(colorBg).
		Background(statusColor).
		Bold(true).
		Padding(0, 1).
		Render(" ● " + status + " ")

	modeStyle := lipgloss.NewStyle().
		Foreground(colorGrayLight).
		Render(" mode:" + modeTag + " ")

	modeIndicatorTag := ""
	if m.modeIndicator == "BUILD" {
		modeIndicatorTag = modeIndicatorBuildStyle.Render(" BUILD ")
	} else {
		modeIndicatorTag = modeIndicatorStyle.Render(" PLAN ")
	}

	modelTag := lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true).
		Render(" model:" + m.currentModel + " ")

	keybindHint := lipgloss.NewStyle().
		Foreground(colorGray).
		Render(" Ctrl+P:palette · Ctrl+F:files · Ctrl+D:diff · Ctrl+T:tasks ")

	line2 := lipgloss.JoinHorizontal(lipgloss.Left, statusTag, modeIndicatorTag, modeStyle, modelTag, keybindHint)

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

// ─── Welcome ───────────────────────────────────────────────────────────────────

func (m Model) renderWelcome() string {
	var b strings.Builder
	b.WriteString(renderCoscaASCII())
	b.WriteString("\n\n")

	subtitle := lipgloss.JoinHorizontal(lipgloss.Top,
		welcomeBody.Render("AI Orchestration Terminal "),
		lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("v2"),
		welcomeHint.Render("  ·  theme: "+m.themeName),
	)
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	b.WriteString(renderShortcutGrid([][2]string{
		{"Enter", "run task"},
		{"/", "slash commands"},
		{":", "palette"},
		{"Ctrl+T", "tasks panel"},
		{"Ctrl+F", "files panel"},
		{"Ctrl+D", "diff panel"},
		{"Ctrl+O", "operations"},
		{"Tab", "cycle panels"},
		{"Ctrl+K", "plan / build"},
		{"Ctrl+C", "cancel / quit"},
	}))
	b.WriteString("\n")

	b.WriteString(m.renderSystemStatus())
	b.WriteString("\n\n")

	b.WriteString(welcomeHint.Render("Type a task to execute — input stays live while work runs in the background."))
	b.WriteString("\n")
	b.WriteString(welcomeHint.Render("Type /help for commands or : for the palette."))
	b.WriteString("\n")

	welcome := welcomeBox.Render(b.String())
	background := lipgloss.NewStyle().Background(th.BackgroundPanel)
	lines := strings.Split(welcome, "\n")
	for i, line := range lines {
		if gap := m.width - lipgloss.Width(line); gap > 0 {
			lines[i] = line + background.Render(strings.Repeat(" ", gap))
		}
	}
	return strings.Join(lines, "\n")
}

// renderCoscaASCII returns the COSCA ASCII wordmark.
func renderCoscaASCII() string {
	art := []string{
		` ██████╗ ██████╗ ███████╗ ██████╗ █████╗ `,
		`██╔════╝██╔═══██╗██╔════╝██╔════╝██╔══██╗`,
		`██║     ██║   ██║███████╗██║     ███████║`,
		`██║     ██║   ██║╚════██║██║     ██╔══██║`,
		`╚██████╗╚██████╔╝███████║╚██████╗██║  ██║`,
		` ╚═════╝ ╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝`,
	}
	var sb strings.Builder
	for _, line := range art {
		sb.WriteString(welcomeTitle.Render(line))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// renderShortcutGrid lays shortcut pairs out in two columns.
func renderShortcutGrid(shortcuts [][2]string) string {
	var rows []string
	for i := 0; i < len(shortcuts); i += 2 {
		left := renderShortcutCell(shortcuts[i][0], shortcuts[i][1])
		var right string
		if i+1 < len(shortcuts) {
			right = renderShortcutCell(shortcuts[i+1][0], shortcuts[i+1][1])
		}
		if right == "" {
			rows = append(rows, left)
		} else {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func renderShortcutCell(key, desc string) string {
	keyStyle := lipgloss.NewStyle().Foreground(colorGold).Bold(true)
	cell := keyStyle.Render("  "+key) + lipgloss.NewStyle().Foreground(colorGrayLight).Render(" "+desc)
	return lipgloss.NewStyle().Width(40).Render(cell)
}

// renderSystemStatus renders the model/agent/providers/fabric status row.
func (m Model) renderSystemStatus() string {
	model := m.currentModel
	if model == "" {
		model = "default"
	}
	agent := m.currentAgent
	if agent == "" {
		agent = "kernel"
	}
	fabric := "off"
	if m.fabric != nil {
		fabric = "ready"
	}

	modelTag := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(" model:" + model + " ")
	agentTag := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("agent:" + agent + " ")
	providerTag := lipgloss.NewStyle().Foreground(colorBlue).Render("providers:1 ")
	fabricTag := lipgloss.NewStyle().Foreground(colorGold).Render("fabric:" + fabric + " ")

	return lipgloss.JoinHorizontal(lipgloss.Left, modelTag, agentTag, providerTag, fabricTag)
}

// ─── Message rendering ─────────────────────────────────────────────────────────

// msgHeader renders a subtle per-bubble header: role label + timestamp.
func msgHeader(label string, labelStyle lipgloss.Style, t time.Time) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render(label),
		timestampStyle.Render(" "+t.Format("15:04")+" "),
	)
}

func (m Model) renderMessage(msg Message) string {
	switch msg.Role {
	case "user":
		return userBubbleBox.Render(msgHeader("you", userBubbleLabel, msg.Time) + "\n" + msg.Content)
	case "assistant":
		rendered := strings.Trim(RenderMarkdown(msg.Content, m.width-8), "\n")
		return assistantBubble.Render(msgHeader("cosca", assistantBubbleLabel, msg.Time) + "\n" + rendered)
	case "tool":
		risk := toolRiskLevel(msg.Content)
		icon, style := toolRiskStyle(risk)
		return style.Render(icon + " " + msg.Content)
	case "error":
		return errorBubble.Render("✗ " + msg.Content)
	case "slash":
		return slashBubble.Render(msg.Content)
	case "info":
		return infoBubble.Render(msg.Content + "  " + timestampStyle.Render(msg.Time.Format("15:04")))
	default:
		return msg.Content
	}
}

// toolRiskLevel determines the risk category of a tool call from its name.
func toolRiskLevel(content string) string {
	lower := strings.ToLower(content)
	destructive := []string{"rm ", "delete", "drop ", "truncate", "destroy", "purge", "wipe", "format", "mkfs"}
	execOps := []string{"bash", "exec ", "sh ", "run ", "sudo ", " systemctl", "kill", "docker run", "docker exec", "kubectl apply", "kubectl delete"}
	writeOps := []string{"write", "edit", "save", "put", "post", "patch", "mv ", "cp ", "create", "touch", "mkdir", "sed ", "echo ", "tee ", "chmod", "chown"}

	for _, kw := range destructive {
		if strings.Contains(lower, kw) {
			return "destructive"
		}
	}

	for _, kw := range execOps {
		if strings.Contains(lower, kw) {
			return "exec"
		}
	}

	for _, kw := range writeOps {
		if strings.Contains(lower, kw) {
			return "write"
		}
	}

	return "read"
}

// toolRiskStyle returns a distinct icon + box style per tool risk class.
func toolRiskStyle(level string) (string, lipgloss.Style) {
	switch level {
	case "destructive":
		return "☠", toolDestructiveStyle
	case "exec":
		return "⚡", toolExecStyle
	case "write":
		return "✎", toolWriteStyle
	default:
		return "◎", toolReadStyle
	}
}

func (m *Model) appendMessage(role, content string) {
	m.messages = append(m.messages, Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
}

// ─── Streaming ─────────────────────────────────────────────────────────────────

func renderStreamingContent(role, text string) string {
	if text == "" {
		return ""
	}
	switch role {
	case "assistant":
		return assistantBubble.Render(text) + titleSubStyle.Render("▍")
	case "tool":
		risk := toolRiskLevel(text)
		icon, style := toolRiskStyle(risk)
		return style.Render(icon+" "+text) + titleSubStyle.Render("▍")
	default:
		return text + titleSubStyle.Render("▍")
	}
}

func (m Model) viewportCmd() tea.Cmd {
	return viewport.Sync(m.chatViewport)
}

// runPipelineStream runs the pipeline in a goroutine so the tea message loop
// never blocks: the returned command spawns the runner, forwards events into a
// local channel, and returns immediately. The input stays focused and typable
// for the entire run.
func (m Model) runPipelineStream(input string) tea.Cmd {
	return func() tea.Msg {
		if m.fabric != nil {
			return m.runPipelineViaFabric(input)
		}
		return m.startPipelineStream(input)
	}
}

func (m Model) startPipelineStream(input string) tea.Msg {
	ch := make(chan pipeline.RunEvent, 32)
	go func() {
		defer close(ch)
		events, err := m.runner.RunStream(m.ctx, pipeline.RunRequest{
			Prompt: input,
			Agent:  m.currentAgent,
		})
		if err != nil {
			ch <- pipeline.RunEvent{
				Type: pipeline.EventError,
				Data: err,
			}
			return
		}
		for ev := range events {
			select {
			case ch <- ev:
			case <-m.ctx.Done():
				return
			}
		}
	}()
	return pipelineStreamStartedMsg{events: ch}
}

// runPipelineViaFabric submits the whole run as a single fabric task. The
// worker pool executes it in the background while the terminal stays live.
func (m Model) runPipelineViaFabric(input string) tea.Msg {
	ch := make(chan pipeline.RunEvent, 32)
	task := compute.Task{
		ID:     "terminal:" + m.currentTraceID,
		Weight: 4,
		Fn: func(ctx context.Context) (interface{}, error) {
			// The producer owns the channel: it must be closed here, not by
			// the submitter goroutine below. Closing it there races with the
			// sends in this function ("send on closed channel" panic when
			// m.ctx is cancelled while the pool keeps streaming).
			defer close(ch)

			events, err := m.runner.RunStream(ctx, pipeline.RunRequest{
				Prompt: input,
				Agent:  m.currentAgent,
			})
			if err != nil {
				select {
				case ch <- pipeline.RunEvent{Type: pipeline.EventError, Data: err}:
				case <-ctx.Done():
				}
				return nil, err
			}
			for ev := range events {
				select {
				case ch <- ev:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, nil
		},
	}
	go func() {
		// This goroutine only closes the channel when the task was never
		// accepted by the pool (Submit error / nil fabric) — in that case the
		// producer above never runs, so there is no race on close.
		if m.fabric == nil {
			select {
			case ch <- pipeline.RunEvent{Type: pipeline.EventError, Data: fmt.Errorf("fabric not available")}:
			default:
			}
			close(ch)
			return
		}
		if _, err := m.fabric.Submit(m.ctx, "agent", task); err != nil {
			select {
			case ch <- pipeline.RunEvent{Type: pipeline.EventError, Data: err}:
			default:
			}
			close(ch)
		}
	}()
	return pipelineStreamStartedMsg{events: ch}
}

func (m Model) consumeNextEvent() tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-m.streamCh
		if !ok {
			return pipelineDoneMsg{}
		}
		return pipelineEventMsg{event: evt}
	}
}

func (m Model) handlePipelineEvent(evt pipeline.RunEvent) (tea.Model, tea.Cmd) {
	// Feed the operations tree with real pipeline events and drive real HUD
	// progress from step counts (not fabricated numbers).
	tr := MapPipelineEvent(string(evt.Type), m.currentAgent, eventDetail(evt))
	if tr.TraceID != "" && m.currentTraceID != "" {
		tr.TraceID = m.currentTraceID
	}
	m.operations.AddEvent(tr)
	if m.operations.events > 0 {
		m.hud.Progress = float64(m.operations.events-m.operations.active) / float64(m.operations.events)
	}

	switch evt.Type {
	case pipeline.EventContent:
		if s, ok := evt.Data.(string); ok {
			m.currentText.WriteString(s)
		}
		m.streaming = true
		return m, m.consumeNextEvent()

	case pipeline.EventToolStart:
		m.flushCurrentText()
		m.currentRole = "tool"
		m.currentText.Reset()
		if s, ok := evt.Data.(string); ok {
			m.activeTaskName = s
			m.currentText.WriteString(s)
		}
		m.streaming = true
		m.hud.PhasePct = 0.3
		return m, m.consumeNextEvent()

	case pipeline.EventToolResult:
		m.flushCurrentText()
		if s, ok := evt.Data.(string); ok {
			m.appendMessage("info", "   → "+s)
			// File-touching tools (write/edit/destructive) change the working
			// tree: refresh the side panels live and flag the Diff tab so the
			// user notices new changes without being yanked out of the chat.
			if m.sessionCtx != nil && (toolRiskLevel(s) == "write" || toolRiskLevel(s) == "destructive") {
				if diffs := GetGitDiffs(m.sessionCtx.ProjectPath); diffs != nil {
					m.diffEntries = diffs
					m.diffDirty = true
				}
				if files := ScanModifiedFiles(m.sessionCtx.ProjectPath); files != nil {
					m.fileEntries = files
				}
			}
		}
		m.streaming = false
		m.hud.PhasePct = 0.7
		return m, m.consumeNextEvent()

	case pipeline.EventBuildStart:
		m.appendMessage("info", "Building...")
		m.hud.Phase = PhaseBuilding
		m.hud.PhasePct = 0.0
		return m, m.consumeNextEvent()

	case pipeline.EventBuildEnd:
		m.hud.Phase = PhaseExecuting
		m.hud.PhasePct = 0.8
		return m, m.consumeNextEvent()

	case pipeline.EventTestStart:
		m.appendMessage("info", "Running tests...")
		m.hud.Phase = PhaseTesting
		m.hud.PhasePct = 0.0
		return m, m.consumeNextEvent()

	case pipeline.EventTestEnd:
		m.hud.Phase = PhaseExecuting
		m.hud.PhasePct = 0.9
		return m, m.consumeNextEvent()

	case pipeline.EventError:
		m.flushCurrentText()
		if s, ok := evt.Data.(error); ok {
			m.appendMessage("error", s.Error())
		} else if s, ok := evt.Data.(string); ok {
			m.appendMessage("error", s)
		}
		m.streaming = false
		m.busy = false
		m.background = false
		m.hud.Status = "error"
		m.hud.Phase = PhaseIdle
		m.activeTaskName = ""
		return m, m.viewportCmd()

	case pipeline.EventDone:
		m.flushCurrentText()
		m.streaming = false
		m.busy = false
		m.background = false
		m.hud.Phase = PhaseDone
		m.activeTaskName = ""
		return m, m.viewportCmd()
	}

	return m, m.consumeNextEvent()
}

// eventDetail extracts a human-readable detail string from a RunEvent payload.
func eventDetail(evt pipeline.RunEvent) string {
	switch d := evt.Data.(type) {
	case nil:
		return ""
	case string:
		return d
	case error:
		return d.Error()
	default:
		return fmt.Sprintf("%v", d)
	}
}

func (m *Model) flushCurrentText() {
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

// ─── Slash commands ────────────────────────────────────────────────────────────

func (m Model) handlePaletteAction(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "theme-cosca", "cosca", "theme-default":
		SetTheme("cosca")
		m.themeName = "cosca"
		m.refreshThemeStyles()
		m.appendMessage("info", "Switched to Cosca dark theme.")
		return m, m.viewportCmd()
	case "theme-opencode":
		SetTheme("opencode")
		m.themeName = "opencode"
		m.refreshThemeStyles()
		m.appendMessage("info", "Switched to OpenCode theme.")
		return m, m.viewportCmd()
	case "theme-tokyonight":
		SetTheme("tokyonight")
		m.themeName = "tokyonight"
		m.refreshThemeStyles()
		m.appendMessage("info", "Switched to TokyoNight theme.")
		return m, m.viewportCmd()
	case "theme-petrol", "petrol":
		SetTheme("petrol")
		m.themeName = "petrol"
		m.refreshThemeStyles()
		m.appendMessage("info", "Switched to Petrol theme.")
		return m, m.viewportCmd()
	case "mode-advanced":
		m.advancedMode = true
		m.appendMessage("info", "Advanced mode enabled.")
		return m, m.viewportCmd()
	case "mode-simple":
		m.advancedMode = false
		m.appendMessage("info", "Simple mode enabled.")
		return m, m.viewportCmd()
	case "clear-chat":
		m.messages = make([]Message, 0)
		m.appendMessage("info", "Chat cleared.")
		return m, m.viewportCmd()
	case "session-info":
		if m.sessionCtx != nil {
			m.appendMessage("info", m.sessionCtx.ContextSummary())
		} else {
			m.appendMessage("info", "No session context available.")
		}
		return m, m.viewportCmd()
	case "keyboard-shortcuts":
		m.appendMessage("slash", keyboardShortcutsHelp())
		return m, m.viewportCmd()
	case "panel-chat":
		m.activePanel = PanelChat
		m.breadcrumbs = []string{"Chat"}
		return m, nil
	case "panel-tasks":
		m.activePanel = PanelTasks
		m.breadcrumbs = []string{"Tasks"}
		return m, nil
	case "panel-files":
		m.activePanel = PanelFiles
		m.breadcrumbs = []string{"Files"}
		if m.sessionCtx != nil && len(m.fileEntries) == 0 {
			m.fileEntries = ScanModifiedFiles(m.sessionCtx.ProjectPath)
		}
		m.diffDirty = false
		return m, nil
	case "panel-diff":
		m.activePanel = PanelDiff
		m.breadcrumbs = []string{"Diff"}
		if m.sessionCtx != nil && len(m.diffEntries) == 0 {
			m.diffEntries = GetGitDiffs(m.sessionCtx.ProjectPath)
		}
		m.diffDirty = false
		if len(m.diffEntries) > 0 {
			m.appendMessage("info", "Showing git diffs in Diff panel.")
		} else {
			m.appendMessage("info", "No diffs available.")
		}
		return m, m.viewportCmd()
	case "panel-system":
		if m.sessionCtx != nil {
			m.appendMessage("info", m.sessionCtx.ContextSummary())
		} else {
			m.appendMessage("info", "No system info available.")
		}
		return m, m.viewportCmd()
	case "quit":
		m.quit = true
		m.cancel()
		return m, tea.Quit
	case "help-slash":
		m.appendMessage("slash", "Type /help for slash commands list.")
		return m, m.viewportCmd()
	case "help-colon":
		m.appendMessage("slash", "Type :help for colon commands list.")
		return m, m.viewportCmd()
	default:
		m.appendMessage("info", "Unknown command: "+id)
		return m, m.viewportCmd()
	}
}

func (m *Model) refreshThemeStyles() {
	m.chatViewport.Style = chatViewportStyle
	m.taskViewport.Style = taskPanelStyle
	m.chatInput.PromptStyle = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground).Bold(true)
	m.chatInput.TextStyle = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground)
	m.chatInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextMuted).Background(th.InputFocusedBackground)
	m.chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(colorFg).Background(th.InputFocusedBackground)
	m.spinner.Style = titleSubStyle
	m.palette.input.PromptStyle = lipgloss.NewStyle().Foreground(colorFg).Bold(true)
	m.palette.input.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	m.palette.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextMuted)
	m.palette.input.Cursor.Style = lipgloss.NewStyle().Foreground(colorFg)
}

func keyboardShortcutsHelp() string {
	return `Keyboard Shortcuts:
  Ctrl+P     Command Palette
  Ctrl+K     Toggle PLAN/BUILD mode
  Ctrl+E     Toggle multi-line mode
  Ctrl+T     Toggle Tasks panel
  Ctrl+F     Toggle Files panel
  Ctrl+D     Show diffs
  Ctrl+C     Cancel/Exit (cancel stream when busy)
  Ctrl+Q     Quit
  Tab        Cycle panels
  Shift+Tab  Reverse cycle panels
  1-3        Direct panel access
  Up/Down    Navigate history
  Enter      Submit prompt
  /          Slash commands
  :          Colon commands
`
}

func (m Model) execSlashCommand(input string) (tea.Model, tea.Cmd) {
	cmd := strings.ToLower(input)

	if cmd == "/exit" || cmd == "/quit" || cmd == "/q" {
		m.quit = true
		m.cancel()
		return m, tea.Quit
	}

	mc := &CommandContext{
		TermCtx:      m.sessionCtx,
		Tasks:        m.tasks,
		Models:       []string{m.currentModel},
		Agents:       []string{},
		ActiveModel:  m.currentModel,
		ActiveAgent:  m.currentAgent,
		AdvancedMode: m.advancedMode,
	}

	result := ExecSlashCommand(cmd, mc)

	if result.IsAction {
		switch result.ActionID {
		case "simple":
			m.advancedMode = false
		case "advanced":
			m.advancedMode = true
		}
		m.appendMessage("info", result.Text)
		return m, m.viewportCmd()
	}

	if result.IsError {
		m.appendMessage("error", result.Text)
	} else {
		m.appendMessage(result.Role, result.Text)
	}

	m.history = append(m.history, input)
	m.histIdx = len(m.history)
	m.chatInput.SetValue("")
	return m, m.viewportCmd()
}

// ─── Colon commands (Command Palette) ──────────────────────────────────────────

func (m Model) execColonCommand(input string) (tea.Model, tea.Cmd) {
	mc := &CommandContext{
		TermCtx:      m.sessionCtx,
		Tasks:        m.tasks,
		Models:       []string{m.currentModel},
		Agents:       []string{},
		ActiveModel:  m.currentModel,
		ActiveAgent:  m.currentAgent,
		AdvancedMode: m.advancedMode,
	}

	result := ExecColonCommand(input, mc)

	if result.IsAction {
		if result.ShouldQuit {
			m.quit = true
			m.cancel()
			return m, tea.Quit
		}
		if result.NewAgent != "" {
			m.currentAgent = result.NewAgent
		}
		if result.NewModel != "" {
			m.currentModel = result.NewModel
		}
		m.appendMessage("info", result.Text)
		return m, m.viewportCmd()
	}

	if result.IsError {
		m.appendMessage("error", result.Text)
	} else {
		m.appendMessage("slash", result.Text)
	}

	m.history = append(m.history, input)
	m.histIdx = len(m.history)
	m.chatInput.SetValue("")
	return m, m.viewportCmd()
}

// ─── Panel helpers ─────────────────────────────────────────────────────────────

func indexOfPanel(panels []PanelID, target PanelID) int {
	for i, p := range panels {
		if p == target {
			return i
		}
	}
	return 0
}

func containsPanel(panels []PanelID, target PanelID) bool {
	for _, p := range panels {
		if p == target {
			return true
		}
	}
	return false
}

// ─── Internal messages ─────────────────────────────────────────────────────────

type pipelineEventMsg struct {
	event pipeline.RunEvent
}

type pipelineStreamStartedMsg struct {
	events <-chan pipeline.RunEvent
}

type pipelineDoneMsg struct{}

type stopStreamingMsg struct{}

type frameTickMsg struct{}
