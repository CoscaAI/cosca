package terminal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DiffEntry struct {
	FilePath  string
	Hunks     []DiffHunk
	Additions int
	Deletions int
	Content   []DiffLine
}

type DiffHunk struct {
	Header string
	Lines  []DiffLine
}

type DiffLine struct {
	Type      string
	LineNoOld int
	LineNoNew int
	Content   string
}

type DiffPanel struct {
	Diffs    []DiffEntry
	selected int
}

var (
	diffAddStyle     = lipgloss.NewStyle().Foreground(colorGreen)
	diffDelStyle     = lipgloss.NewStyle().Foreground(colorRed)
	diffHunkStyle    = lipgloss.NewStyle().Foreground(colorCyan)
	diffContextStyle = lipgloss.NewStyle().Foreground(colorGrayLight)
	diffFileStyle    = lipgloss.NewStyle().Foreground(colorGold).Bold(true)
)

func (dp *DiffPanel) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if dp.selected > 0 {
			dp.selected--
		}
	case "down", "j":
		if dp.selected < len(dp.Diffs)-1 {
			dp.selected++
		}
	}
}

func countVisibleLines(lines []DiffLine) int {
	count := 0
	for _, l := range lines {
		stripped := strings.TrimSpace(l.Content)
		if stripped == "" {
			continue
		}
		count++
	}
	return count
}

func renderDiffLine(line DiffLine, width int) string {
	content := truncateStr(line.Content, width)

	switch line.Type {
	case "add":
		return diffAddStyle.Render(content)
	case "del":
		return diffDelStyle.Render(content)
	case "hunk":
		return diffHunkStyle.Render(content)
	default:
		return diffContextStyle.Render(content)
	}
}

func GetGitDiffs(projectPath string) []DiffEntry {
	if projectPath == "" {
		return nil
	}

	cmd := exec.Command("git", "-C", projectPath, "diff", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	diffOutput := string(out)
	if strings.TrimSpace(diffOutput) == "" {
		return nil
	}

	return parseGitDiff(diffOutput)
}

func parseGitDiff(raw string) []DiffEntry {
	lines := strings.Split(raw, "\n")
	var diffs []DiffEntry
	var current *DiffEntry
	var currentHunk *DiffHunk

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			if current != nil {
				if len(current.Content) > 0 {
					diffs = append(diffs, *current)
				}
			}

			parts := strings.Fields(line)
			filePath := ""
			if len(parts) >= 4 {
				raw := parts[3]
				filePath = strings.TrimPrefix(raw, "b/")
			}

			current = &DiffEntry{
				FilePath: filePath,
				Content:  []DiffLine{},
				Hunks:    []DiffHunk{},
			}
			currentHunk = nil
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			current.Content = append(current.Content, DiffLine{
				Type:    "context",
				Content: line,
			})
		} else if strings.HasPrefix(line, "@@") {
			if currentHunk != nil && len(currentHunk.Lines) > 0 {
				current.Hunks = append(current.Hunks, *currentHunk)
			}
			currentHunk = &DiffHunk{
				Header: line,
				Lines:  []DiffLine{},
			}
			current.Content = append(current.Content, DiffLine{
				Type:    "hunk",
				Content: line,
			})
		} else if strings.HasPrefix(line, "+") {
			dl := DiffLine{
				Type:    "add",
				Content: line,
			}
			current.Content = append(current.Content, dl)
			current.Additions++
			if currentHunk != nil {
				currentHunk.Lines = append(currentHunk.Lines, dl)
			}
		} else if strings.HasPrefix(line, "-") {
			dl := DiffLine{
				Type:    "del",
				Content: line,
			}
			current.Content = append(current.Content, dl)
			current.Deletions++
			if currentHunk != nil {
				currentHunk.Lines = append(currentHunk.Lines, dl)
			}
		} else if strings.HasPrefix(line, " ") {
			dl := DiffLine{
				Type:    "context",
				Content: line,
			}
			current.Content = append(current.Content, dl)
			if currentHunk != nil {
				currentHunk.Lines = append(currentHunk.Lines, dl)
			}
		}
	}

	if currentHunk != nil && len(currentHunk.Lines) > 0 {
		current.Hunks = append(current.Hunks, *currentHunk)
	}

	if current != nil && len(current.Content) > 0 {
		diffs = append(diffs, *current)
	}

	return diffs
}

func DiffPanelView(diffs []DiffEntry, width, height, selected int) string {
	panelW := width
	if panelW < 20 {
		panelW = 40
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderSubtle).
		Background(th.BackgroundPanel).
		Foreground(th.Text).
		Padding(0, 1)
	styleWidth := panelW - borderStyle.GetHorizontalBorderSize()
	contentWidth := styleWidth - borderStyle.GetHorizontalPadding()
	borderStyle = borderStyle.Width(styleWidth)
	contentHeight := height - borderStyle.GetVerticalFrameSize()
	if contentHeight < 1 {
		contentHeight = 1
	}

	if len(diffs) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("No diffs available")

		return borderStyle.Height(contentHeight).Render(
			headerStyle.Render("DIFF") + "\n" + empty,
		)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("DIFF"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	remaining := contentHeight - 5
	fileIdx := 0

	for _, d := range diffs {
		if fileIdx > selected && remaining <= 0 {
			break
		}

		needed := countVisibleLines(d.Content) + 2
		if fileIdx < selected {
			fileIdx++
			continue
		}

		if remaining-needed < 0 && fileIdx > selected {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Italic(true).
				Render(fmt.Sprintf("  ... %d more files", len(diffs)-fileIdx)))
			break
		}

		shortPath := filepath.Base(d.FilePath)

		addLabel := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", d.Additions))
		delLabel := lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", d.Deletions))

		b.WriteString(diffFileStyle.Render(" " + shortPath + " "))
		b.WriteString(addLabel + " " + delLabel)
		b.WriteString("\n")

		for i, line := range d.Content {
			if remaining <= 0 {
				break
			}

			var lineNum string
			switch line.Type {
			case "add", "del", "context":
				lineNum = diffLineNumberStyle.Render(fmt.Sprintf("%4d", i))
			default:
				lineNum = strings.Repeat(" ", 4)
			}

			b.WriteString(lineNum + " " + renderDiffLineStyled(line, contentWidth-6))
			b.WriteString("\n")
			remaining--
		}
		b.WriteString("\n")
		remaining -= 2
		fileIdx++
	}

	return borderStyle.Height(contentHeight).Render(b.String())
}

func renderDiffLineStyled(line DiffLine, width int) string {
	content := truncateStr(strings.TrimPrefix(line.Content, " "), width)

	switch line.Type {
	case "add":
		prefix := lipgloss.NewStyle().Foreground(colorGreen).Render("+")
		return prefix + diffAddedLineStyle.Render(content)
	case "del":
		prefix := lipgloss.NewStyle().Foreground(colorRed).Render("-")
		return prefix + diffRemovedLineStyle.Render(content)
	case "hunk":
		return diffHunkHeaderStyle.Render(line.Content)
	default:
		return diffContextLineStyle.Render(content)
	}
}
