package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type PermissionRequest struct {
	ToolName    string
	Description string
	RiskLevel   string
	Parameters  map[string]string
}

func RenderPermissionPrompt(req PermissionRequest, width int) string {
	if width < 20 {
		return ""
	}

	var riskStyle lipgloss.Style
	switch req.RiskLevel {
	case "read":
		riskStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	case "write":
		riskStyle = lipgloss.NewStyle().Foreground(colorGold).Bold(true)
	case "exec":
		riskStyle = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	case "destructive":
		riskStyle = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorRed).
			Bold(true).
			Padding(0, 1)
		return riskStyle.Render(fmt.Sprintf(" DESTRUCTIVE: %s ", req.ToolName))
	default:
		riskStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	}

	var b strings.Builder

	riskLabel := riskStyle.Render(fmt.Sprintf(" [%s] ", strings.ToUpper(req.RiskLevel)))
	b.WriteString(permAskLabelStyle.Render("Tool permission required:"))
	b.WriteString("\n")
	b.WriteString(permToolNameStyle.Render(req.ToolName + " "))
	b.WriteString(riskLabel)
	b.WriteString("\n")

	if req.Description != "" {
		b.WriteString(permParamStyle.Render(req.Description))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
	}

	b.WriteString(" ")
	b.WriteString(permAllowBtnStyle.Render(" (A)llow "))
	b.WriteString(" ")
	b.WriteString(permDenyBtnStyle.Render(" (D)eny  "))
	b.WriteString("\n")

	return permContainerStyle.Render(b.String())
}

type PermissionDialog struct {
	Request  PermissionRequest
	Visible  bool
	RiskIcon string
}
