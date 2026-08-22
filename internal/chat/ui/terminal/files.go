package terminal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FileEntry struct {
	Path     string
	Action   string // created, modified, deleted, error
	LinesAdd int
	LinesDel int
	HasError bool
}

type FilesPanel struct {
	Files    []FileEntry
	selected int
}

func (fp *FilesPanel) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if fp.selected > 0 {
			fp.selected--
		}
	case "down", "j":
		if fp.selected < len(fp.Files)-1 {
			fp.selected++
		}
	}
}

func FilesPanelView(files []FileEntry, width, height int, selected int) string {
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

	if len(files) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("No files tracked yet")

		return borderStyle.Height(contentHeight).Render(
			headerStyle.Render("FILES") + "\n" + empty,
		)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("FILES"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	displayCount := contentHeight - 4
	if displayCount < 1 {
		displayCount = 1
	}

	for i, f := range files {
		if i >= displayCount {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Italic(true).
				Render(fmt.Sprintf("  ... %d more", len(files)-displayCount)))
			break
		}

		rowStyle := lipgloss.NewStyle()
		if i == selected {
			rowStyle = lipgloss.NewStyle().
				Background(colorSelection).
				Foreground(colorFg)
		}

		b.WriteString(renderFileRow(f, rowStyle, contentWidth))
		b.WriteString("\n")
	}

	return borderStyle.Height(contentHeight).Render(b.String())
}

func renderFileRow(f FileEntry, style lipgloss.Style, width int) string {
	actionIcon := " "
	actionColor := colorGrayLight

	switch f.Action {
	case "created":
		actionIcon = "+"
		actionColor = colorGreen
	case "modified":
		actionIcon = "~"
		actionColor = colorGold
	case "deleted":
		actionIcon = "-"
		actionColor = colorRed
	case "error":
		actionIcon = "!"
		actionColor = colorRed
	}

	icon := lipgloss.NewStyle().
		Foreground(actionColor).
		Bold(true).
		Render(actionIcon)

	label := truncateFilePath(f.Path, width-12-lipgloss.Width(icon))

	var delta string
	if f.LinesAdd > 0 || f.LinesDel > 0 {
		addPart := ""
		delPart := ""
		if f.LinesAdd > 0 {
			addPart = lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", f.LinesAdd))
		}
		if f.LinesDel > 0 {
			delPart = lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", f.LinesDel))
		}
		if addPart != "" && delPart != "" {
			delta = addPart + " " + delPart
		} else if addPart != "" {
			delta = addPart
		} else {
			delta = delPart
		}
	}

	errMarker := ""
	if f.HasError {
		errMarker = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(" ERR")
	}

	labelStyled := style.Render(label)
	deltaStyled := delta
	if deltaStyled == "" {
		return fmt.Sprintf(" %s %s%s", icon, labelStyled, errMarker)
	}

	return fmt.Sprintf(" %s %s %s%s", icon, labelStyled, deltaStyled, errMarker)
}

func truncateFilePath(path string, maxLen int) string {
	if maxLen < 5 {
		maxLen = 5
	}
	if len(path) <= maxLen {
		return path
	}

	dir, file := filepath.Split(path)
	if len(file) >= maxLen-3 {
		return "..." + file[len(file)-(maxLen-3):]
	}

	prefixLen := maxLen - len(file) - 3
	if prefixLen < 1 {
		return "..." + file[len(file)-(maxLen-3):]
	}

	if len(dir) > prefixLen {
		dir = dir[:prefixLen] + "..."
	}

	return dir + file
}

func ScanModifiedFiles(projectPath string) []FileEntry {
	if projectPath == "" {
		return nil
	}

	entries := fileChangesFromGit(projectPath)
	if entries != nil {
		return entries
	}

	return nil
}

func fileChangesFromGit(projectPath string) []FileEntry {
	cmd := exec.Command("git", "-C", projectPath, "diff", "--name-status", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var entries []FileEntry

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		filePath := parts[1]

		entry := FileEntry{Path: filePath}

		if status == "M" || status == "MM" {
			entry.Action = "modified"
		} else if status == "A" || status == "??" {
			entry.Action = "created"
		} else if status == "D" {
			entry.Action = "deleted"
		} else if status == "R" {
			entry.Action = "renamed"
		} else {
			entry.Action = "modified"
		}

		adds, dels := fileLineChanges(projectPath, filePath)
		entry.LinesAdd = adds
		entry.LinesDel = dels

		entries = append(entries, entry)
	}

	return entries
}

func fileLineChanges(projectPath, file string) (adds, dels int) {
	cmd := exec.Command("git", "-C", projectPath, "diff", "HEAD", "--", file)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			adds++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			dels++
		}
	}

	return adds, dels
}
