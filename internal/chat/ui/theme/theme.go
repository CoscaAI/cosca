package theme

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme is a shared colour palette used by all Cosca TUIs.
// App uses BackgroundPanel as a safe fallback when Background is empty.
type Theme struct {
	Background             lipgloss.Color
	Foreground             lipgloss.Color
	BgAlt                  lipgloss.Color
	Surface                lipgloss.Color // contrasting surface for text on Primary/colored backgrounds
	InputBackground        lipgloss.Color
	InputFocusedBackground lipgloss.Color
	Primary                lipgloss.Color
	PrimaryDim             lipgloss.Color
	Success                lipgloss.Color
	Error                  lipgloss.Color
	Info                   lipgloss.Color
	Accent                 lipgloss.Color
	Accent2                lipgloss.Color
	Muted                  lipgloss.Color
	MutedLight             lipgloss.Color
	Border                 lipgloss.Color

	Text      lipgloss.Color
	TextMuted lipgloss.Color

	BackgroundPanel   lipgloss.Color
	BackgroundElement lipgloss.Color

	BorderActive lipgloss.Color
	BorderSubtle lipgloss.Color

	DiffAdded            lipgloss.Color
	DiffRemoved          lipgloss.Color
	DiffContext          lipgloss.Color
	DiffHunkHeader       lipgloss.Color
	DiffHighlightAdded   lipgloss.Color
	DiffHighlightRemoved lipgloss.Color
	DiffAddedBg          lipgloss.Color
	DiffRemovedBg        lipgloss.Color
	DiffContextBg        lipgloss.Color
	DiffLineNumber       lipgloss.Color

	MarkdownText           lipgloss.Color
	MarkdownHeading        lipgloss.Color
	MarkdownLink           lipgloss.Color
	MarkdownCode           lipgloss.Color
	MarkdownBlockQuote     lipgloss.Color
	MarkdownEmph           lipgloss.Color
	MarkdownStrong         lipgloss.Color
	MarkdownHorizontalRule lipgloss.Color
	MarkdownListItem       lipgloss.Color
	MarkdownCodeBlock      lipgloss.Color

	SyntaxComment     lipgloss.Color
	SyntaxKeyword     lipgloss.Color
	SyntaxFunction    lipgloss.Color
	SyntaxVariable    lipgloss.Color
	SyntaxString      lipgloss.Color
	SyntaxNumber      lipgloss.Color
	SyntaxType        lipgloss.Color
	SyntaxOperator    lipgloss.Color
	SyntaxPunctuation lipgloss.Color

	BackgroundMenu lipgloss.Color // overlay menu background (command palette, popups)

	Warning   lipgloss.Color
	Selection lipgloss.Color
}

// Petrol is Cosca's high-contrast petroleum, black, gray, and white palette.
// It intentionally uses a solid background so the terminal emulator's theme
// cannot show through the full-screen application.
var Petrol = Theme{
	Background:             "#092F33",
	Foreground:             "#F4F7F7",
	BgAlt:                  "#0D3A3E",
	Surface:                "#05090A",
	InputBackground:        "#4A5557",
	InputFocusedBackground: "#596568",
	Primary:                "#72C9C6",
	PrimaryDim:             "#2D7778",
	Success:                "#78D6A0",
	Error:                  "#FF7777",
	Info:                   "#83C9D0",
	Accent:                 "#72C9C6",
	Accent2:                "#83C9D0",
	Muted:                  "#647477",
	MutedLight:             "#B8C4C5",
	Border:                 "#34585A",

	Text:      "#D5DEDF",
	TextMuted: "#9BA9AA",

	BackgroundPanel:   "#05090A",
	BackgroundElement: "#11191B",

	BorderActive: "#8ADBD6",
	BorderSubtle: "#1C393B",

	DiffAdded:            "#78D6A0",
	DiffRemoved:          "#FF7777",
	DiffContext:          "#B8C4C5",
	DiffHunkHeader:       "#83C9D0",
	DiffHighlightAdded:   "#9BE3B8",
	DiffHighlightRemoved: "#FF9C9C",
	DiffAddedBg:          "#123A32",
	DiffRemovedBg:        "#3B2022",
	DiffContextBg:        "#05090A",
	DiffLineNumber:       "#647477",

	MarkdownText:           "#D5DEDF",
	MarkdownHeading:        "#8ADBD6",
	MarkdownLink:           "#83C9D0",
	MarkdownCode:           "#B7E1CF",
	MarkdownBlockQuote:     "#9BA9AA",
	MarkdownEmph:           "#D5DEDF",
	MarkdownStrong:         "#F4F7F7",
	MarkdownHorizontalRule: "#647477",
	MarkdownListItem:       "#83C9D0",
	MarkdownCodeBlock:      "#B7E1CF",

	SyntaxComment:     "#647477",
	SyntaxKeyword:     "#8ADBD6",
	SyntaxFunction:    "#83C9D0",
	SyntaxVariable:    "#F4F7F7",
	SyntaxString:      "#78D6A0",
	SyntaxNumber:      "#E6C36A",
	SyntaxType:        "#B7E1CF",
	SyntaxOperator:    "#72C9C6",
	SyntaxPunctuation: "#D5DEDF",

	BackgroundMenu: "#0D1919",

	Warning:   "#E6C36A",
	Selection: "#276568",
}

var TokyoNight = Theme{
	Background:             "#1a1b26",
	Foreground:             "#c0caf5",
	BgAlt:                  "#16161e",
	Surface:                "#1a1b26", // dark — used as text color on Primary backgrounds
	InputBackground:        "#30343F",
	InputFocusedBackground: "#3B4261",
	Primary:                "#e0af68",
	PrimaryDim:             "#b08a4e",
	Success:                "#9ece6a",
	Error:                  "#f7768e",
	Info:                   "#7aa2f7",
	Accent:                 "#bb9af7",
	Accent2:                "#7dcfff",
	Muted:                  "#565f89",
	MutedLight:             "#787c99",
	Border:                 "#3b4261",

	Text:      "#c0caf5",
	TextMuted: "#787c99",

	BackgroundPanel:   "#1a1b26",
	BackgroundElement: "#16161e",

	BorderActive: "#e0af68",
	BorderSubtle: "#3b4261",

	DiffAdded:            "#9ece6a",
	DiffRemoved:          "#f7768e",
	DiffContext:          "#787c99",
	DiffHunkHeader:       "#7dcfff",
	DiffHighlightAdded:   "#73daca",
	DiffHighlightRemoved: "#f7768e",
	DiffAddedBg:          "#1a3a2a",
	DiffRemovedBg:        "#3a1a2a",
	DiffContextBg:        "#1a1b26",
	DiffLineNumber:       "#565f89",

	MarkdownText:           "#c0caf5",
	MarkdownHeading:        "#e0af68",
	MarkdownLink:           "#7aa2f7",
	MarkdownCode:           "#bb9af7",
	MarkdownBlockQuote:     "#787c99",
	MarkdownEmph:           "#c0caf5",
	MarkdownStrong:         "#c0caf5",
	MarkdownHorizontalRule: "#565f89",
	MarkdownListItem:       "#7dcfff",
	MarkdownCodeBlock:      "#bb9af7",

	SyntaxComment:     "#565f89",
	SyntaxKeyword:     "#bb9af7",
	SyntaxFunction:    "#7aa2f7",
	SyntaxVariable:    "#c0caf5",
	SyntaxString:      "#9ece6a",
	SyntaxNumber:      "#e0af68",
	SyntaxType:        "#7dcfff",
	SyntaxOperator:    "#bb9af7",
	SyntaxPunctuation: "#c0caf5",

	BackgroundMenu: "#1a1b26",

	Warning:   "#ff9e64",
	Selection: "#565f89",
}

var OpenCode = Theme{
	Background:             "#0D1117",
	Foreground:             "#e6edf3",
	BgAlt:                  "#161b22",
	Surface:                "#0d1117", // dark contrasting surface for text on Primary/colored bgs
	InputBackground:        "#3B424A",
	InputFocusedBackground: "#4A535D",
	Primary:                "#58a6ff",
	PrimaryDim:             "#388bfd",
	Success:                "#3fb950",
	Error:                  "#f85149",
	Info:                   "#58a6ff",
	Accent:                 "#bc8cff",
	Accent2:                "#79c0ff",
	Muted:                  "#484f58",
	MutedLight:             "#8b949e",
	Border:                 "#30363d",

	Text:      "#e6edf3",
	TextMuted: "#8b949e",

	BackgroundPanel:   "#161b22",
	BackgroundElement: "#0d1117",

	BorderActive: "#58a6ff",
	BorderSubtle: "#21262d",

	DiffAdded:            "#3fb950",
	DiffRemoved:          "#f85149",
	DiffContext:          "#8b949e",
	DiffHunkHeader:       "#79c0ff",
	DiffHighlightAdded:   "#56d364",
	DiffHighlightRemoved: "#f85149",
	DiffAddedBg:          "#12261e",
	DiffRemovedBg:        "#2d1520",
	DiffContextBg:        "#161b22",
	DiffLineNumber:       "#484f58",

	MarkdownText:           "#e6edf3",
	MarkdownHeading:        "#58a6ff",
	MarkdownLink:           "#58a6ff",
	MarkdownCode:           "#bc8cff",
	MarkdownBlockQuote:     "#8b949e",
	MarkdownEmph:           "#e6edf3",
	MarkdownStrong:         "#e6edf3",
	MarkdownHorizontalRule: "#30363d",
	MarkdownListItem:       "#79c0ff",
	MarkdownCodeBlock:      "#bc8cff",

	SyntaxComment:     "#8b949e",
	SyntaxKeyword:     "#ff7b72",
	SyntaxFunction:    "#d2a8ff",
	SyntaxVariable:    "#ffa657",
	SyntaxString:      "#a5d6ff",
	SyntaxNumber:      "#a5d6ff",
	SyntaxType:        "#ffa657",
	SyntaxOperator:    "#ff7b72",
	SyntaxPunctuation: "#e6edf3",

	BackgroundMenu: "#0D1117",

	Warning:   "#d29922",
	Selection: "#1f6feb",
}

// Cosca is Cosca's default premium dark theme: near-black GitHub-dark-style
// neutrals (opencode look) with the house teal/cyan accent. It is the terminal
// default — Petrol/TokyoNight/OpenCode remain selectable.
var Cosca = Theme{
	Background:             "#0D1117",
	Foreground:             "#E6EDF3",
	BgAlt:                  "#161B22",
	Surface:                "#05090A", // dark text on bright accent surfaces
	InputBackground:        "#21262D",
	InputFocusedBackground: "#2D333B",
	Primary:                "#2DD4BF", // cosca ciano
	PrimaryDim:             "#178F84",
	Success:                "#16C784", // cosca verde
	Error:                  "#F85149",
	Info:                   "#58A6FF",
	Accent:                 "#2DD4BF",
	Accent2:                "#79C0FF",
	Muted:                  "#484F58",
	MutedLight:             "#8B949E",
	Border:                 "#30363D",

	Text:      "#E6EDF3",
	TextMuted: "#8B949E",

	BackgroundPanel:   "#161B22",
	BackgroundElement: "#0D1117",

	BorderActive: "#2DD4BF",
	BorderSubtle: "#21262D",

	DiffAdded:            "#3FB950",
	DiffRemoved:          "#F85149",
	DiffContext:          "#8B949E",
	DiffHunkHeader:       "#79C0FF",
	DiffHighlightAdded:   "#56D364",
	DiffHighlightRemoved: "#FF7B72",
	DiffAddedBg:          "#132A1F",
	DiffRemovedBg:        "#3A1D1D",
	DiffContextBg:        "#161B22",
	DiffLineNumber:       "#484F58",

	MarkdownText:           "#E6EDF3",
	MarkdownHeading:        "#2DD4BF",
	MarkdownLink:           "#58A6FF",
	MarkdownCode:           "#7EE0D6",
	MarkdownBlockQuote:     "#8B949E",
	MarkdownEmph:           "#E6EDF3",
	MarkdownStrong:         "#F0F6FC",
	MarkdownHorizontalRule: "#30363D",
	MarkdownListItem:       "#79C0FF",
	MarkdownCodeBlock:      "#79C0FF",

	SyntaxComment:     "#7D8590",
	SyntaxKeyword:     "#FF7B72",
	SyntaxFunction:    "#D2A8FF",
	SyntaxVariable:    "#FFA657",
	SyntaxString:      "#A5D6FF",
	SyntaxNumber:      "#A5D6FF",
	SyntaxType:        "#FFA657",
	SyntaxOperator:    "#FF7B72",
	SyntaxPunctuation: "#C9D1D9",

	BackgroundMenu: "#0D1117",

	Warning:   "#D29922",
	Selection: "#16524B",
}

// Panel returns a bordered panel style.
func (t Theme) Panel() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderSubtle).
		Background(t.BackgroundPanel).
		Foreground(t.Text)
}

// Header returns the top-level header style (primary bg, contrasting text).
func (t Theme) Header() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Surface).
		Bold(true).
		Padding(0, 1)
}

// HeaderSub is a secondary header label in the theme's primary accent.
func (t Theme) HeaderSub() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
}

// TabBar returns the tab-bar container style.
func (t Theme) TabBar() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Foreground(t.MutedLight)
}

// TabActive returns the style for the active tab.
func (t Theme) TabActive() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Surface).
		Bold(true).
		Padding(0, 1)
}

// TabInactive returns the style for an inactive tab.
func (t Theme) TabInactive() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Foreground(t.MutedLight).
		Padding(0, 1)
}

// StatusBar returns the bottom status bar style.
func (t Theme) StatusBar() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Foreground(t.MutedLight).
		Padding(0, 1)
}

// Selected returns a highlighted selection style.
func (t Theme) Selected() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Selection).
		Foreground(t.Foreground)
}

// RiskLevel returns a style coloured by tool risk level.
func (t Theme) RiskLevel(level string) lipgloss.Style {
	switch level {
	case "write":
		return lipgloss.NewStyle().Foreground(t.Primary)
	case "exec":
		return lipgloss.NewStyle().Foreground(t.Warning)
	case "destructive":
		return lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(t.Accent2)
	}
}

// App returns the full-screen app style with a solid background.
func (t Theme) App() lipgloss.Style {
	background := t.Background
	if background == "" {
		background = t.BackgroundPanel
		if background == "" {
			background = t.Surface
		}
	}
	return lipgloss.NewStyle().
		Background(background).
		Foreground(t.Foreground)
}

// InputBox returns an un-focused input border.
func (t Theme) InputBox() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "│"}).
		BorderForeground(t.PrimaryDim).
		Background(t.InputBackground).
		Foreground(t.Foreground).
		Padding(0, 1)
}

// InputFocused returns a focused input border.
func (t Theme) InputFocused() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.Border{Left: "│"}).
		BorderForeground(t.Primary).
		Background(t.InputFocusedBackground).
		Foreground(t.Foreground).
		Padding(0, 1)
}

// Breadcrumb returns the breadcrumb trail style.
func (t Theme) Breadcrumb() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.MutedLight).
		Padding(0, 1)
}

// BreadcrumbActive returns the active breadcrumb segment style.
func (t Theme) BreadcrumbActive() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
}

// MarkdownStyle returns a glamour renderer config matching the theme.
func (t Theme) MarkdownStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       stringPtr(string(t.Text)),
			},
			Margin: uintPtr(0),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.Text)),
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownHeading)),
				Bold:  boolPtr(true),
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(string(t.MarkdownBlockQuote)),
				Italic: boolPtr(true),
			},
			Indent: uintPtr(2),
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr(string(t.MarkdownCodeBlock)),
				},
			},
			// Reference the per-theme registered chroma style instead of
			// CodeBlock.Chroma: glamour registers a Chroma-based style once
			// under a fixed name, which would freeze the first palette seen.
			Theme: t.SyntaxChromaName(),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownCode)),
			},
		},
		Emph: ansi.StylePrimitive{
			Color:  stringPtr(string(t.MarkdownEmph)),
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Color: stringPtr(string(t.MarkdownStrong)),
			Bold:  boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr(string(t.MarkdownHorizontalRule)),
			Prefix: "\n",
			Suffix: "\n",
		},
		Item: ansi.StylePrimitive{
			Color: stringPtr(string(t.MarkdownListItem)),
		},
		Enumeration: ansi.StylePrimitive{
			Color: stringPtr(string(t.MarkdownListItem)),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.MarkdownListItem)),
			},
			Ticked:   "[x]",
			Unticked: "[ ]",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(string(t.MarkdownLink)),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(string(t.MarkdownLink)),
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr(string(t.Text)),
				},
			},
		},
	}
}

// chromaFormatterName returns a chroma formatter matching the active colour
// profile so code blocks never output more colour depth than the terminal
// supports (and render plain when colours are disabled).
func (t Theme) chromaFormatterName() string {
	switch lipgloss.ColorProfile() {
	case termenv.TrueColor:
		return "terminal16m"
	case termenv.ANSI256:
		return "terminal256"
	case termenv.ANSI:
		return "terminal"
	default:
		return "noop"
	}
}

// MarkdownRenderer returns a glamour TermRenderer configured with the theme.
func (t Theme) MarkdownRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStyles(t.MarkdownStyle()),
		glamour.WithWordWrap(width),
		glamour.WithColorProfile(lipgloss.ColorProfile()),
		glamour.WithChromaFormatter(t.chromaFormatterName()),
	)
}

// DiffAddedLine returns a style for added diff lines.
func (t Theme) DiffAddedLine() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.DiffAdded).
		Background(t.DiffAddedBg).
		Width(40)
}

// DiffRemovedLine returns a style for removed diff lines.
func (t Theme) DiffRemovedLine() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.DiffRemoved).
		Background(t.DiffRemovedBg).
		Width(40)
}

// DiffContextLine returns a style for context diff lines.
func (t Theme) DiffContextLine() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.DiffContext).
		Background(t.DiffContextBg).
		Width(40)
}

// DiffHunkLine returns a style for hunk header lines.
func (t Theme) DiffHunkLine() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.DiffHunkHeader).
		Bold(true)
}

// DiffLineNumberStyle returns a style for diff line numbers.
func (t Theme) DiffLineNumberStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.DiffLineNumber).
		Width(4)
}

// SyntaxChromaName returns a stable, theme-unique chroma style name used by
// glamour code blocks. It is derived from the theme background and primary
// colour so every selectable palette resolves its own syntax highlight style
// (glamour registers a code-block theme only once, keyed by name — a shared
// name would freeze the first palette forever).
func (t Theme) SyntaxChromaName() string {
	bg := strings.TrimPrefix(string(t.Background), "#")
	if bg == "" {
		bg = "default"
	}
	accent := strings.TrimPrefix(string(t.Primary), "#")
	if accent == "" {
		accent = "base"
	}
	return "cosca-" + bg + "-" + accent
}

// SyntaxBlock returns a chroma style built from the theme's syntax tokens.
// The returned style is named SyntaxChromaName so it can be registered once
// per palette and referenced by glamour's CodeBlock.Theme.
func (t Theme) SyntaxBlock() *chroma.Style {
	return t.syntaxBlockNamed(t.SyntaxChromaName())
}

// syntaxBlockNamed builds a github-dark derived chroma style, overriding the
// token categories that carry the theme's syntax palette.
func (t Theme) syntaxBlockNamed(name string) *chroma.Style {
	cs := styles.Get("github-dark")
	if cs == nil {
		cs = styles.Fallback
	}

	b := cs.Builder()

	b.Add(chroma.Comment, string(t.SyntaxComment))
	b.Add(chroma.Keyword, string(t.SyntaxKeyword))
	b.Add(chroma.KeywordConstant, string(t.SyntaxKeyword))
	b.Add(chroma.KeywordNamespace, string(t.SyntaxKeyword))
	b.Add(chroma.KeywordType, string(t.SyntaxType))
	b.Add(chroma.NameFunction, string(t.SyntaxFunction))
	b.Add(chroma.NameVariable, string(t.SyntaxVariable))
	b.Add(chroma.NameBuiltin, string(t.SyntaxFunction))
	b.Add(chroma.NameClass, string(t.SyntaxType))
	b.Add(chroma.LiteralString, string(t.SyntaxString))
	b.Add(chroma.LiteralStringAffix, string(t.SyntaxString))
	b.Add(chroma.LiteralNumber, string(t.SyntaxNumber))
	b.Add(chroma.NameTag, string(t.SyntaxType))
	b.Add(chroma.NameAttribute, string(t.SyntaxFunction))
	b.Add(chroma.Operator, string(t.SyntaxOperator))
	b.Add(chroma.Punctuation, string(t.SyntaxPunctuation))

	formatted, err := b.Build()
	if err != nil {
		return cs
	}
	formatted.Name = name
	return formatted
}

// registerSyntaxStyles registers one chroma style per selectable palette under
// its SyntaxChromaName. Registration happens once at init so glamour's code
// block rendering never races a runtime registration.
func registerSyntaxStyles() {
	for _, t := range []Theme{Cosca, Petrol, TokyoNight, OpenCode} {
		if cs := t.syntaxBlockNamed(t.SyntaxChromaName()); cs.Name == t.SyntaxChromaName() {
			styles.Register(cs)
		}
	}
}

func init() {
	registerSyntaxStyles()
}

// ─── Helper functions ──────────────────────────────────────────────────────

func stringPtr(s string) *string {
	return &s
}

func uintPtr(u uint) *uint {
	return &u
}

func boolPtr(b bool) *bool {
	return &b
}
