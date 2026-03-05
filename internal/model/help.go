package model

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Width(16)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("226")).
				MarginTop(1).
				MarginBottom(0)
)

type HelpModel struct {
	width  int
	height int
}

func NewHelpModel() HelpModel {
	return HelpModel{}
}

func (m *HelpModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "?" || keyMsg.String() == "esc" {
			// Parent handles toggling
		}
	}
	return m, nil
}

func (m HelpModel) View() string {
	title := helpTitleStyle.Render("Cypher Shell Browser — Keybindings")

	sections := []string{
		title,
		helpSectionStyle.Render("Global"),
		helpLine("Ctrl+C", "Quit"),
		helpLine("?", "Toggle this help"),
		helpLine("Ctrl+L", "Clear / new query"),

		helpSectionStyle.Render("Query Input"),
		helpLine("Ctrl+E", "Execute query"),
		helpLine("Ctrl+Y", "Copy query to clipboard"),
		helpLine("↑/↓", "Browse query history"),
		helpLine("Tab", "Accept autocomplete suggestion"),

		helpSectionStyle.Render("Results"),
		helpLine("Esc", "Toggle query input / results"),
		helpLine("↑/↓ or j/k", "Move row cursor"),
		helpLine("Space", "Toggle detail panel"),
		helpLine("→/l or Enter", "Focus detail panel"),
		helpLine("←/h", "Focus Cypher lines"),
		helpLine("m/c", "Toggle MERGE/CREATE prefix"),
		helpLine("Ctrl+Y", "Copy Cypher to clipboard"),

		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Press ? or Esc to close"),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func helpLine(key, desc string) string {
	return helpKeyStyle.Render(key) + helpDescStyle.Render(desc)
}
