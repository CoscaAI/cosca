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
	PanelMemory
	PanelPermissions
	PanelMissionControl
	PanelGraph
)

var panelNames = map[PanelID]string{
	PanelChat:          "Chat",
	PanelTasks:         "Tasks",
	PanelAgents:        "Agents",
	PanelOperations:    "Operations",
	PanelFiles:         "Files",
	PanelDiff:          "Diff",
	PanelSystem:        "System",
	PanelMemory:        "Memory",
	PanelPermissions:   "Permissions",
	PanelMissionControl: "Mission",
	PanelGraph:         "Graph",
}

// workspaceKeys maps Alt+1..9 to the workspace panels of the Mission Control
// vision. Since Fase 6 ALL panels are real — the Alt+1..9 workspace is
// complete. Agents (Alt+3) since Fase 3; Memory (Alt+6) since Fase 4;
// Permissions (Alt+7) since Fase 5; Mission Control (Alt+8) and Graph (Alt+9)
// since Fase 6.
//
// NOTE: bubbletea v1.3.10 does NOT track Ctrl for character keys (Ctrl+1
// arrives identical to plain 1), so workspace switching uses Alt+1..9 which
// is reliably reported on both Windows and Unix terminals.
var workspaceKeys = map[string]PanelID{
	"alt+1": PanelChat,
	"alt+2": PanelFiles,
	"alt+3": PanelAgents,
	"alt+4": PanelOperations,
	"alt+5": PanelTasks,
	"alt+6": PanelMemory,
	"alt+7": PanelPermissions,
	"alt+8": PanelMissionControl,
	"alt+9": PanelGraph,
}

// panelPhase returns the roadmap phase for workspace panels that are still
// placeholders. Since Fase 6 every panel is implemented, so it always returns
// empty. It is retained for API compatibility.
func panelPhase(p PanelID) string {
	return ""
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
	usedAgents    []string
	memorySelected int
	permissionSelected int

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
	showHUD              bool
	inspectorOpen        bool
	verificationOpen     bool
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
		usedAgents:  nil,

		palette:       NewPalette(),
		paletteOpen:   false,
		themeName:     "cosca",
		modeIndicator: "PLAN",
		multiLineMode: false,
		showHUD:       true,
	}

	if termCtx != nil {
		m.branch = gitBranch(termCtx.ProjectPath)
	}
	if cfg.CurrentAgent != "" {
		m.usedAgents = []string{cfg.CurrentAgent}
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

		appBarHeight := 1 // single thin app bar (R1)
		hudHeight := 1
		inputHeight := 3
		layoutSeparators := 4

		viewportHeight := msg.Height - appBarHeight - hudHeight - inputHeight - layoutSeparators
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

		// Context Inspector (P16): Alt+I toggles the centered overlay.
		if msg.String() == "alt+i" {
			m.inspectorOpen = !m.inspectorOpen
			return m, nil
		}

		if msg.String() == "esc" && m.inspectorOpen {
			m.inspectorOpen = false
			return m, nil
		}

		// Verification Mode (P24): Alt+V toggles the verification overlay.
		if msg.String() == "alt+v" {
			m.verificationOpen = !m.verificationOpen
			return m, nil
		}

		if msg.String() == "esc" && m.verificationOpen {
			m.verificationOpen = false
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

		// Workspace panel access: Alt+1..9 (openCode-style). Plain digits
		// stay available for typing in the input.
		if p, ok := workspaceKeys[msg.String()]; ok {
			m.switchPanel(p)
			return m, nil
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

		// Memory panel navigation (P15): ↑↓ moves, Enter toggles detail.
		if m.activePanel == PanelMemory && !m.busy {
			memCount := len(buildMemoryEntries(&m))
			switch msg.String() {
			case "up", "k":
				if m.memorySelected > 0 {
					m.memorySelected--
				}
				return m, nil
			case "down", "j":
				if m.memorySelected < memCount-1 {
					m.memorySelected++
				}
				return m, nil
			}
		}

		// Permissions panel navigation (P19): ↑↓ moves.
		if m.activePanel == PanelPermissions && !m.busy {
			permCount := len(buildPermissionRules(&m))
			switch msg.String() {
			case "up", "k":
				if m.permissionSelected > 0 {
					m.permissionSelected--
				}
				return m, nil
			case "down", "j":
				if m.permissionSelected < permCount-1 {
					m.permissionSelected++
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

	appBar := m.renderAppBar()

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
	railW := 14
	const (
		minSidebarWidth = 100 // >= this the right context panel appears
		wideRailWidth   = 160 // >= this the left agents rail appears
		minChatWidth    = 24
	)

	// Right context panel appears when there is room. On the Chat panel the
	// sidebar shows session info (OpenCode-style); on other panels it shows
	// the panel content.
	useSide := m.width >= minSidebarWidth
	useRail := m.width >= wideRailWidth

	// The viewport height is the full content area (app bar + input + hud
	// consume the rest). Overlays float on top and do NOT shrink the viewport.
	vpHeight := m.chatViewport.Height

	chatW := m.width
	if useRail {
		chatW -= railW
	}
	if useSide {
		chatW -= sidebarW
	}
	// Narrow-terminal fallback: drop the rail first, then the sidebar, so the
	// chat never gets crushed below a usable width.
	if chatW < minChatWidth && useRail {
		useRail = false
		chatW += railW
	}
	if chatW < minChatWidth && useSide {
		useSide = false
		chatW += sidebarW
	}
	if chatW < 10 {
		chatW = m.width
	}

	rightPanel := ""
	if useSide {
		switch m.activePanel {
		case PanelChat:
			rightPanel = renderSessionInfoSidebar(m, sidebarW, vpHeight)
		case PanelFiles:
			rightPanel = FilesPanelView(m.fileEntries, sidebarW, vpHeight, m.filesSelected)
		case PanelDiff:
			rightPanel = DiffPanelView(m.diffEntries, sidebarW, vpHeight, m.diffSelected)
		case PanelTasks:
			rightPanel = TaskPanelView(m.tasks, sidebarW, vpHeight)
		case PanelOperations:
			rightPanel = m.operations.Render(sidebarW, vpHeight)
		case PanelAgents:
			rightPanel = AgentsPanelView(m.usedAgents, m.currentAgent, sidebarW, vpHeight)
		case PanelMemory:
			rightPanel = MemoryPanelView(buildMemoryEntries(&m), m.memorySelected, sidebarW, vpHeight)
		case PanelPermissions:
			rightPanel = PermissionsPanelView(buildPermissionRules(&m), m.permissionSelected, sidebarW, vpHeight)
		case PanelMissionControl:
			rightPanel = MissionControlView(&m, sidebarW, vpHeight)
		case PanelGraph:
			rightPanel = GraphPanelView(&m, sidebarW, vpHeight)
		}
	}

	vpCopy := m.chatViewport
	vpCopy.Width = chatW
	vpCopy.Height = vpHeight
	vpCopy.Style = chatViewportStyle

	paddedVP := chatViewportStyle.Width(chatW).Render(vpCopy.View())

	cols := []string{}
	if useRail {
		cols = append(cols, renderAgentsRail(m.usedAgents, m.currentAgent, railW, vpHeight))
	}
	cols = append(cols, paddedVP)
	if rightPanel != "" {
		cols = append(cols, rightPanel)
	}
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	hudLine := ""
	if m.showHUD {
		hudLine = "\n" + HudView(m.hud, m.width, m.frame)
	}

	// Base layout: app bar + content + input + hud.
	base := appBar + "\n" +
		mainArea + "\n" +
		inputStyle.Render(inputView) +
		hudLine

	// R2: Overlays FLOAT on top — they replace the content area (not the
	// input/hud) instead of being injected into the vertical flow. This keeps
	// the layout stable and the overlay visually centered.
	if m.paletteOpen {
		return appStyle.Width(m.width).Height(m.height).Render(
			appBar + "\n" +
				m.renderCenteredOverlay(m.palette.View()) + "\n" +
				inputStyle.Render(inputView) +
				hudLine,
		)
	}
	if m.inspectorOpen {
		return appStyle.Width(m.width).Height(m.height).Render(
			appBar + "\n" +
				m.renderCenteredOverlay(ContextInspectorView(&m)) + "\n" +
				inputStyle.Render(inputView) +
				hudLine,
		)
	}
	if m.verificationOpen {
		return appStyle.Width(m.width).Height(m.height).Render(
			appBar + "\n" +
				m.renderCenteredOverlay(VerificationView(&m)) + "\n" +
				inputStyle.Render(inputView) +
				hudLine,
		)
	}

	return appStyle.Width(m.width).Height(m.height).Render(base)
}

// renderCenteredOverlay centers an overlay box horizontally within the content
// area, with a dimmed background so it reads as floating above the content.
func (m Model) renderCenteredOverlay(content string) string {
	contentW := lipgloss.Width(content)
	padL := (m.width - contentW) / 2
	if padL < 0 {
		padL = 0
	}
	padR := m.width - contentW - padL
	if padR < 0 {
		padR = 0
	}
	return lipgloss.NewStyle().
		Background(th.BackgroundPanel).
		PaddingLeft(padL).
		PaddingRight(padR).
		Render(content)
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
		{"Ctrl+P", "command palette"},
		{"Alt+1..9", "workspace panels"},
		{"Ctrl+T", "tasks panel"},
		{"Ctrl+F", "files panel"},
		{"Ctrl+D", "diff panel"},
		{"Ctrl+O", "operations"},
		{"Tab", "cycle panels"},
		{"Ctrl+K", "plan / build"},
		{"Ctrl+C", "cancel / quit"},
		{":", "colon commands"},
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

// ─── Session Info Sidebar (OpenCode-style) ──────────────────────────────────

// renderSessionInfoSidebar renders the right sidebar when the Chat panel is
// active, showing session context: title, model, agent, tokens, branch, etc.
func renderSessionInfoSidebar(m Model, width, height int) string {
	panelW := width
	if panelW < 10 {
		panelW = 30
	}
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderSubtle).
		Background(th.BackgroundPanel).
		Foreground(th.Text).
		Padding(0, 1)
	styleWidth := panelW - borderStyle.GetHorizontalBorderSize()
	contentWidth := styleWidth - borderStyle.GetHorizontalPadding()
	if contentWidth < 8 {
		contentWidth = 8
	}
	borderStyle = borderStyle.Width(styleWidth)
	contentHeight := height - borderStyle.GetVerticalFrameSize()
	if contentHeight < 3 {
		contentHeight = 3
	}

	var b strings.Builder

	// Title.
	b.WriteString(sessionInfoTitle.Render("SESSION"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	// Model.
	model := m.currentModel
	if model == "" {
		model = "default"
	}
	b.WriteString(sessionInfoLabel.Render("Model"))
	b.WriteString("\n")
	b.WriteString(sessionInfoValue.Render(truncateStr(model, contentWidth)))
	b.WriteString("\n\n")

	// Agent.
	agent := m.currentAgent
	if agent == "" {
		agent = "kernel"
	}
	b.WriteString(sessionInfoLabel.Render("Agent"))
	b.WriteString("\n")
	b.WriteString(sessionInfoValue.Render(truncateStr(agent, contentWidth)))
	b.WriteString("\n\n")

	// Status.
	status := "ready"
	if m.busy {
		status = "busy"
	} else if m.streaming {
		status = "streaming"
	}
	b.WriteString(sessionInfoLabel.Render("Status"))
	b.WriteString("\n")
	statusDot := "●"
	statusColor := th.Success
	if m.busy {
		statusColor = th.Warning
		statusDot = "⠋"
	} else if m.streaming {
		statusColor = th.Accent2
		statusDot = "⠁"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(statusDot+" "+status))
	b.WriteString("\n\n")

	// Branch.
	if m.branch != "" {
		b.WriteString(sessionInfoLabel.Render("Branch"))
		b.WriteString("\n")
		b.WriteString(sessionInfoValue.Render(truncateStr(m.branch, contentWidth)))
		b.WriteString("\n\n")
	}

	// Messages count.
	b.WriteString(sessionInfoLabel.Render("Messages"))
	b.WriteString("\n")
	b.WriteString(sessionInfoValue.Render(fmt.Sprintf("%d", len(m.messages))))
	b.WriteString("\n\n")

	// Mode.
	b.WriteString(sessionInfoLabel.Render("Mode"))
	b.WriteString("\n")
	modeLabel := "PLAN"
	if m.advancedMode {
		modeLabel = "BUILD"
	}
	b.WriteString(sessionInfoValue.Render(modeLabel))
	b.WriteString("\n\n")

	// Agents used.
	if len(m.usedAgents) > 0 {
		b.WriteString(sessionInfoLabel.Render("Agents Used"))
		b.WriteString("\n")
		for i, a := range m.usedAgents {
			if i >= 5 {
				b.WriteString(sessionInfoMuted.Render(fmt.Sprintf("  ... %d more", len(m.usedAgents)-5)))
				b.WriteString("\n")
				break
			}
			dot := "○"
			dotStyle := lipgloss.NewStyle().Foreground(colorGray)
			if a == m.currentAgent {
				dot = "●"
				dotStyle = lipgloss.NewStyle().Foreground(th.Accent)
			}
			b.WriteString(dotStyle.Render(dot) + " " + sessionInfoValue.Render(truncateStr(a, contentWidth-2)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Version footer (bottom-aligned via remaining space).
	versionStr := "Cosca Terminal v2"
	b.WriteString(sessionInfoMuted.Render(versionStr))

	return borderStyle.Height(contentHeight).Render(b.String())
}
// Palette `status` action).
func (m Model) statusLine() string {
	model := m.currentModel
	if model == "" {
		model = "default"
	}
	agent := m.currentAgent
	if agent == "" {
		agent = "kernel"
	}
	state := "ready"
	if m.busy {
		state = "busy"
	} else if m.streaming {
		state = "streaming"
	}
	part := func(k, v string) string { return k + ":" + v }
	return strings.Join([]string{
		part("status", state),
		part("model", model),
		part("agent", agent),
		part("theme", m.themeName),
		part("mode", modeLabel(m.advancedMode)),
		part("branch", func() string { if m.branch == "" { return "-" }; return m.branch }()),
		part("messages", fmt.Sprintf("%d", len(m.messages))),
		part("agents-used", fmt.Sprintf("%d", len(m.usedAgents))),
	}, " · ")
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
		content := msgHeader("you", userBubbleLabel, msg.Time) + "\n" + msg.Content
		return userMessageStyle.Render(content)
	case "assistant":
		rendered := strings.Trim(RenderMarkdown(msg.Content, m.width-8), "\n")
		return assistantBubble.Render(msgHeader("cosca", assistantBubbleLabel, msg.Time) + "\n" + rendered)
	case "tool":
		risk := toolRiskLevel(msg.Content)
		icon, style := toolRiskStyle(risk)
		// Parse tool name from content (first word before any space/paren).
		toolName := msg.Content
		if idx := strings.IndexAny(msg.Content, " ("); idx > 0 {
			toolName = msg.Content[:idx]
		}
		header := toolCallNameStyle.Render(icon+" "+toolName)
		detail := ""
		if idx := strings.IndexAny(msg.Content, " ("); idx > 0 && idx < len(msg.Content) {
			detail = " " + msg.Content[idx:]
		}
		return style.Render(header + toolCallResultStyle.Render(detail))
	case "error":
		return errorBubble.Render("✗ " + msg.Content)
	case "slash":
		return slashBubble.Render(msg.Content)
	case "info":
		return infoBubble.Render(msg.Content + "  " + timestampStyle.Render(msg.Time.Format("15:04")))
	case "reasoning":
		return reasoningStyle.Render("  thinking... " + msg.Content)
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
	m.rememberAgent(tr.Actor)
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
	case "clear-context":
		m.messages = make([]Message, 0)
		m.pendingPrompts = nil
		m.usedAgents = nil
		m.branch = ""
		if m.sessionCtx != nil {
			m.branch = gitBranch(m.sessionCtx.ProjectPath)
		}
		m.appendMessage("info", "Context cleared.")
		return m, m.viewportCmd()
	case "toggle-hud":
		m.showHUD = !m.showHUD
		state := "hidden"
		if m.showHUD {
			state = "visible"
		}
		m.appendMessage("info", "Status bar " + state + ".")
		return m, m.viewportCmd()
	case "inspect-context", "context":
		m.inspectorOpen = true
		return m, nil
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
	case "panel-operations":
		m.activePanel = PanelOperations
		m.breadcrumbs = []string{panelNames[PanelOperations]}
		return m, nil
	case "panel-agents":
		m.switchPanel(PanelAgents)
		return m, nil
	case "panel-memory":
		m.switchPanel(PanelMemory)
		return m, nil
	case "panel-permissions", "panel-git":
		m.switchPanel(PanelPermissions)
		return m, nil
	case "computer":
		m.switchPanel(PanelPermissions)
		m.appendMessage("info", "Computer Mode — capabilities shown in Permissions panel.")
		return m, m.viewportCmd()
	case "verify", "verification":
		m.verificationOpen = true
		return m, nil
	case "panel-mission", "mission-control", "overview", "panel-deploy":
		m.switchPanel(PanelMissionControl)
		return m, nil
	case "panel-graph", "graph":
		m.switchPanel(PanelGraph)
		return m, nil
	case "status":
		m.appendMessage("info", m.statusLine())
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
  Ctrl+O     Operations panel
  Alt+1     Chat
  Alt+2     Files
  Alt+3     Agents (hierarchy)
  Alt+4     Operations
  Alt+5     Tasks
  Alt+6     Memory (explorer)
  Alt+I     Context Inspector
  Alt+7     Permissions (center)
  Alt+8     Mission Control (cockpit)
  Alt+9     Graph (relations)
  Alt+V     Verification
  Ctrl+C     Cancel/Exit (cancel stream when busy)
  Ctrl+Q     Quit
  Tab        Cycle panels
  Shift+Tab  Reverse cycle panels
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
			m.rememberAgent(result.NewAgent)
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

// switchPanel activates a workspace panel and keeps its side data warm.
func (m *Model) switchPanel(p PanelID) {
	m.activePanel = p
	m.breadcrumbs = []string{panelNames[p]}
	switch p {
	case PanelFiles:
		if m.sessionCtx != nil && len(m.fileEntries) == 0 {
			m.fileEntries = ScanModifiedFiles(m.sessionCtx.ProjectPath)
		}
		m.diffDirty = false
	case PanelDiff:
		if m.sessionCtx != nil && len(m.diffEntries) == 0 {
			m.diffEntries = GetGitDiffs(m.sessionCtx.ProjectPath)
		}
		m.diffDirty = false
	}
}

// rememberAgent records an agent that worked in this session (drives the left
// agents rail on wide terminals).
func (m *Model) rememberAgent(agent string) {
	if agent == "" {
		return
	}
	for _, a := range m.usedAgents {
		if a == agent {
			return
		}
	}
	m.usedAgents = append(m.usedAgents, agent)
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
