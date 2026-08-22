package terminal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/chat/ui/theme"
)

var (
	th               = theme.Petrol
	currentThemeName = "petrol"
)

// SetTheme switches between available themes at runtime.
func SetTheme(name string) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tokyonight", "tokyo", "theme-tokyonight":
		th = theme.TokyoNight
		currentThemeName = "tokyonight"
	case "opencode", "theme-opencode":
		th = theme.OpenCode
		currentThemeName = "opencode"
	case "petrol", "theme-petrol":
		th = theme.Petrol
		currentThemeName = "petrol"
	default:
		th = theme.Petrol
		currentThemeName = "petrol"
	}
	applyThemeStyles(th)
}

// ThemeName returns the current theme name.
func ThemeName() string {
	return currentThemeName
}

// ─── Theme Palette ──────────────────────────────────────────────────────────

var (
	colorBg        = th.Surface // always dark — for text on colored backgrounds
	colorBgAlt     = th.BgAlt
	colorFg        = th.Foreground
	colorGold      = th.Primary
	colorGoldDim   = th.PrimaryDim
	colorGreen     = th.Success
	colorRed       = th.Error
	colorBlue      = th.Info
	colorPurple    = th.Accent
	colorCyan      = th.Accent2
	colorGray      = th.Muted
	colorGrayLight = th.MutedLight
	colorBorder    = th.Border
	colorOrange    = th.Warning
	colorTextMuted = th.TextMuted
	colorSelection = th.Selection
)

// ─── App shell ──────────────────────────────────────────────────────────────

var (
	appStyle = th.App()

	titleStyle    = th.Header()
	titleSubStyle = th.HeaderSub()
)

// ─── Panel layout ───────────────────────────────────────────────────────────

var (
	panelStyle = th.Panel()

	chatViewportStyle = lipgloss.NewStyle().
				Background(th.BackgroundPanel).
				Foreground(th.Text).
				Padding(0, 1)

	taskPanelStyle = th.Panel().
			Padding(0, 1).
			BorderLeft(true)
)

// ─── Input ──────────────────────────────────────────────────────────────────

var (
	inputBoxStyle     = th.InputBox()
	inputFocusedStyle = th.InputFocused()
)

// ─── Tab bar ────────────────────────────────────────────────────────────────

var (
	tabBarStyle      = th.TabBar()
	tabActiveStyle   = th.TabActive()
	tabInactiveStyle = th.TabInactive()
)

// ─── Breadcrumbs ────────────────────────────────────────────────────────────

var (
	breadcrumbStyle       = th.Breadcrumb()
	breadcrumbActiveStyle = th.BreadcrumbActive()
)

// ─── HUD / Status bar ───────────────────────────────────────────────────────

var (
	hudStyle = th.StatusBar()

	hudLabelStyle = lipgloss.NewStyle().
			Foreground(colorGrayLight)

	hudValueStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			Bold(true)

	hudAgentStyle = lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true)

	hudModelStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	hudPhaseLabel = lipgloss.NewStyle().
			Foreground(colorGrayLight)

	hudPhaseFill = lipgloss.NewStyle().
			Foreground(colorCyan)

	hudPhaseTrack = lipgloss.NewStyle().
			Foreground(colorGray)
)

// ─── Progress bar ───────────────────────────────────────────────────────────

var (
	progressTrackStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	progressFillStyle = lipgloss.NewStyle().
				Foreground(colorGold)
)

// ─── Message bubbles ────────────────────────────────────────────────────────

var (
	assistantBubble = lipgloss.NewStyle().
			Foreground(th.Text).
			Background(th.BackgroundPanel).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(th.BorderSubtle).
			Padding(0, 1).
			MaxWidth(96)

	userBubbleBox = lipgloss.NewStyle().
			Foreground(colorFg).
			Background(th.InputBackground).
			Bold(true).
			Padding(0, 1).
			MaxWidth(96)

	toolCallBox = lipgloss.NewStyle().
			Foreground(colorCyan).
			Background(th.BackgroundElement).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(th.BorderSubtle).
			Padding(0, 1).
			MaxWidth(96)

	toolReadStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Background(th.BackgroundElement).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(th.BorderSubtle).
			Padding(0, 1).
			MaxWidth(96)

	toolWriteStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Background(th.BackgroundElement).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGoldDim).
			Padding(0, 1).
			MaxWidth(96)

	toolExecStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Background(th.BackgroundElement).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOrange).
			Padding(0, 1).
			MaxWidth(96)

	toolDestructiveStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Background(th.DiffRemovedBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorRed).
				Padding(0, 1).
				MaxWidth(96)

	errorBubble = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	slashBubble = lipgloss.NewStyle().
			Foreground(colorBlue)

	infoBubble = lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true)

	welcomeTitle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true)

	welcomeBody = lipgloss.NewStyle().
			Foreground(th.Text)

	welcomeHint = lipgloss.NewStyle().
			Foreground(colorGrayLight)

	welcomeBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(th.BorderActive).
			Background(th.BackgroundPanel).
			Padding(1, 2).
			Width(72)

	timestampStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)
)

// ─── Task rendering ─────────────────────────────────────────────────────────

var (
	taskPendingStyle = lipgloss.NewStyle().Foreground(colorGrayLight)
	taskRunningStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	taskDoneStyle    = lipgloss.NewStyle().Foreground(colorGreen)
	taskFailedStyle  = lipgloss.NewStyle().Foreground(colorRed)
	taskAgentStyle   = lipgloss.NewStyle().Foreground(colorPurple)
)

// ─── Diff rendering ─────────────────────────────────────────────────────────

var (
	diffAddedLineStyle = lipgloss.NewStyle().
				Foreground(th.DiffAdded).
				Background(th.DiffAddedBg).
				Width(40)

	diffRemovedLineStyle = lipgloss.NewStyle().
				Foreground(th.DiffRemoved).
				Background(th.DiffRemovedBg).
				Width(40)

	diffContextLineStyle = lipgloss.NewStyle().
				Foreground(th.DiffContext).
				Background(th.DiffContextBg)

	diffHunkHeaderStyle = lipgloss.NewStyle().
				Foreground(th.DiffHunkHeader).
				Bold(true)

	diffLineNumberStyle = lipgloss.NewStyle().
				Foreground(th.DiffLineNumber).
				Width(4)

	diffFileHeaderStyle = lipgloss.NewStyle().
				Foreground(th.Primary).
				Bold(true).
				Padding(0, 1)
)

// ─── Command palette ────────────────────────────────────────────────────────

var (
	paletteOverlayStyle = lipgloss.NewStyle().
				Background(th.BackgroundPanel).
				Foreground(th.Text).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(th.BorderActive).
				Width(80).
				Padding(1, 1).
				MaxHeight(20)

	paletteInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(th.BorderActive).
				Background(th.InputFocusedBackground).
				Foreground(th.Foreground).
				Padding(0, 1).
				Width(78)

	paletteItemStyle = lipgloss.NewStyle().
				Foreground(th.Text).
				Padding(0, 1)

	paletteItemSelectedStyle = lipgloss.NewStyle().
					Foreground(colorBg).
					Background(th.Primary).
					Bold(true).
					Padding(0, 1).
					Width(76)

	paletteCategoryStyle = lipgloss.NewStyle().
				Foreground(th.MutedLight).
				Bold(true).
				Padding(0, 1)

	paletteHintStyle = lipgloss.NewStyle().
				Foreground(colorGray).
				Italic(true).
				Padding(0, 1)
)

// ─── Permission prompts ─────────────────────────────────────────────────────

var (
	permContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorOrange).
				Background(th.BackgroundPanel).
				Foreground(th.Text).
				Padding(1, 2).
				Width(60)

	permAllowBtnStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorGreen).
				Bold(true).
				Padding(0, 1)

	permDenyBtnStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorRed).
				Bold(true).
				Padding(0, 1)

	permAskLabelStyle = lipgloss.NewStyle().
				Foreground(colorFg).
				Bold(true)

	permToolNameStyle = lipgloss.NewStyle().
				Foreground(colorPurple).
				Bold(true)

	permParamStyle = lipgloss.NewStyle().
			Foreground(colorGrayLight)
)

// ─── Code block styles ──────────────────────────────────────────────────────

var (
	codeBlockBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(th.BorderSubtle).
				Background(th.BackgroundElement).
				Padding(0, 1)

	codeBlockHeaderStyle = lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Bold(true).
				Padding(0, 1)

	codeBlockContentStyle = lipgloss.NewStyle().
				Foreground(th.Text).
				Padding(0, 1)
)

// ─── Status indicators ──────────────────────────────────────────────────────

var (
	modeIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorPurple).
				Bold(true).
				Padding(0, 1)

	modeIndicatorBuildStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorOrange).
				Bold(true).
				Padding(0, 1)

	statusDotStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	statusDotBusyStyle = lipgloss.NewStyle().
				Foreground(colorOrange)

	statusDotStreamingStyle = lipgloss.NewStyle().
				Foreground(colorCyan)
)

// ─── File reference styles ──────────────────────────────────────────────────

var (
	fileRefStyle = lipgloss.NewStyle().
		Foreground(colorBlue).
		Background(th.BackgroundElement).
		Underline(true).
		Padding(0, 1)
)

func init() {
	applyThemeStyles(theme.Petrol)
}

// applyThemeStyles keeps the package-level styles in sync with the selected
// theme. Lipgloss styles are values, so changing th alone would leave every
// already-built style on the previous palette.
func applyThemeStyles(t theme.Theme) {
	th = t

	colorBg = t.Surface
	colorBgAlt = t.BgAlt
	colorFg = t.Foreground
	colorGold = t.Primary
	colorGoldDim = t.PrimaryDim
	colorGreen = t.Success
	colorRed = t.Error
	colorBlue = t.Info
	colorPurple = t.Accent
	colorCyan = t.Accent2
	colorGray = t.Muted
	colorGrayLight = t.MutedLight
	colorBorder = t.Border
	colorOrange = t.Warning
	colorTextMuted = t.TextMuted
	colorSelection = t.Selection

	appStyle = t.App()
	titleStyle = t.Header()
	titleSubStyle = t.HeaderSub()

	panelStyle = t.Panel()
	chatViewportStyle = lipgloss.NewStyle().
		Background(t.BackgroundPanel).
		Foreground(t.Text).
		Padding(0, 1)
	taskPanelStyle = t.Panel().
		Padding(0, 1).
		BorderLeft(true)

	inputBoxStyle = t.InputBox()
	inputFocusedStyle = t.InputFocused()

	tabBarStyle = t.TabBar()
	tabActiveStyle = t.TabActive()
	tabInactiveStyle = t.TabInactive()

	breadcrumbStyle = t.Breadcrumb()
	breadcrumbActiveStyle = t.BreadcrumbActive()

	hudStyle = t.StatusBar()
	hudLabelStyle = lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.BgAlt)
	hudValueStyle = lipgloss.NewStyle().Foreground(t.Foreground).Background(t.BgAlt).Bold(true)
	hudAgentStyle = lipgloss.NewStyle().Foreground(t.Accent).Background(t.BgAlt).Bold(true)
	hudModelStyle = lipgloss.NewStyle().Foreground(t.Accent2).Background(t.BgAlt)
	hudPhaseLabel = lipgloss.NewStyle().Foreground(t.MutedLight).Background(t.BgAlt)
	hudPhaseFill = lipgloss.NewStyle().Foreground(t.Accent2).Background(t.BgAlt)
	hudPhaseTrack = lipgloss.NewStyle().Foreground(t.Muted).Background(t.BgAlt)

	progressTrackStyle = lipgloss.NewStyle().Foreground(t.Muted).Background(t.BgAlt)
	progressFillStyle = lipgloss.NewStyle().Foreground(t.Primary).Background(t.BgAlt)

	assistantBubble = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.BackgroundPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderSubtle).
		Padding(0, 1).
		MaxWidth(96)
	userBubbleBox = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Background(t.InputBackground).
		Bold(true).
		Padding(0, 1).
		MaxWidth(96)
	toolCallBox = lipgloss.NewStyle().
		Foreground(t.Accent2).
		Background(t.BackgroundElement).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderSubtle).
		Padding(0, 1).
		MaxWidth(96)
	toolReadStyle = lipgloss.NewStyle().
		Foreground(t.Accent2).
		Background(t.BackgroundElement).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderSubtle).
		Padding(0, 1).
		MaxWidth(96)
	toolWriteStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(t.BackgroundElement).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.PrimaryDim).
		Padding(0, 1).
		MaxWidth(96)
	toolExecStyle = lipgloss.NewStyle().
		Foreground(t.Warning).
		Background(t.BackgroundElement).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Warning).
		Padding(0, 1).
		MaxWidth(96)
	toolDestructiveStyle = lipgloss.NewStyle().
		Foreground(t.Error).
		Background(t.DiffRemovedBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Error).
		Padding(0, 1).
		MaxWidth(96)
	errorBubble = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	slashBubble = lipgloss.NewStyle().Foreground(t.Info)
	infoBubble = lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	welcomeTitle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	welcomeBody = lipgloss.NewStyle().Foreground(t.Text)
	welcomeHint = lipgloss.NewStyle().Foreground(t.TextMuted)
	welcomeBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.BorderActive).
		Background(t.BackgroundPanel).
		Padding(1, 2).
		Width(72)
	timestampStyle = lipgloss.NewStyle().Foreground(t.Muted).Italic(true)

	taskPendingStyle = lipgloss.NewStyle().Foreground(t.MutedLight)
	taskRunningStyle = lipgloss.NewStyle().Foreground(t.Accent2).Bold(true)
	taskDoneStyle = lipgloss.NewStyle().Foreground(t.Success)
	taskFailedStyle = lipgloss.NewStyle().Foreground(t.Error)
	taskAgentStyle = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)

	diffAddedLineStyle = lipgloss.NewStyle().
		Foreground(t.DiffAdded).
		Background(t.DiffAddedBg).
		Width(40)
	diffRemovedLineStyle = lipgloss.NewStyle().
		Foreground(t.DiffRemoved).
		Background(t.DiffRemovedBg).
		Width(40)
	diffContextLineStyle = lipgloss.NewStyle().
		Foreground(t.DiffContext).
		Background(t.DiffContextBg)
	diffHunkHeaderStyle = lipgloss.NewStyle().
		Foreground(t.DiffHunkHeader).
		Bold(true)
	diffLineNumberStyle = lipgloss.NewStyle().
		Foreground(t.DiffLineNumber).
		Width(4)
	diffFileHeaderStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(0, 1)
	diffAddStyle = lipgloss.NewStyle().Foreground(t.Success)
	diffDelStyle = lipgloss.NewStyle().Foreground(t.Error)
	diffHunkStyle = lipgloss.NewStyle().Foreground(t.Accent2)
	diffContextStyle = lipgloss.NewStyle().Foreground(t.MutedLight)
	diffFileStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)

	paletteOverlayStyle = lipgloss.NewStyle().
		Background(t.BackgroundPanel).
		Foreground(t.Text).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Width(80).
		Padding(1, 1).
		MaxHeight(20)
	paletteInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Background(t.InputFocusedBackground).
		Foreground(t.Foreground).
		Padding(0, 1).
		Width(78)
	paletteItemStyle = lipgloss.NewStyle().
		Foreground(t.Text).
		Padding(0, 1)
	paletteItemSelectedStyle = lipgloss.NewStyle().
		Foreground(t.Surface).
		Background(t.Primary).
		Bold(true).
		Padding(0, 1).
		Width(76)
	paletteCategoryStyle = lipgloss.NewStyle().
		Foreground(t.MutedLight).
		Bold(true).
		Padding(0, 1)
	paletteHintStyle = lipgloss.NewStyle().
		Foreground(t.Muted).
		Italic(true).
		Padding(0, 1)

	permContainerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Warning).
		Background(t.BackgroundPanel).
		Foreground(t.Text).
		Padding(1, 2).
		Width(60)
	permAllowBtnStyle = lipgloss.NewStyle().
		Foreground(t.Surface).
		Background(t.Success).
		Bold(true).
		Padding(0, 1)
	permDenyBtnStyle = lipgloss.NewStyle().
		Foreground(t.Surface).
		Background(t.Error).
		Bold(true).
		Padding(0, 1)
	permAskLabelStyle = lipgloss.NewStyle().Foreground(t.Foreground).Bold(true)
	permToolNameStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	permParamStyle = lipgloss.NewStyle().Foreground(t.MutedLight)

	codeBlockBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderSubtle).
		Background(t.BackgroundElement).
		Padding(0, 1)
	codeBlockHeaderStyle = lipgloss.NewStyle().
		Foreground(t.MutedLight).
		Bold(true).
		Padding(0, 1)
	codeBlockContentStyle = lipgloss.NewStyle().
		Foreground(t.Text).
		Padding(0, 1)

	modeIndicatorStyle = lipgloss.NewStyle().
		Foreground(t.Surface).
		Background(t.Accent).
		Bold(true).
		Padding(0, 1)
	modeIndicatorBuildStyle = lipgloss.NewStyle().
		Foreground(t.Surface).
		Background(t.Warning).
		Bold(true).
		Padding(0, 1)
	statusDotStyle = lipgloss.NewStyle().Foreground(t.Success)
	statusDotBusyStyle = lipgloss.NewStyle().Foreground(t.Warning)
	statusDotStreamingStyle = lipgloss.NewStyle().Foreground(t.Accent2)

	fileRefStyle = lipgloss.NewStyle().
		Foreground(t.Info).
		Background(t.BackgroundElement).
		Underline(true).
		Padding(0, 1)
}
