package model

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

var (
	tableBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	tableBorderFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86"))
)

type TableViewModel struct {
	table    table.Model
	rowPaths [][]n4j.RowPathItem
	ready    bool
	active   bool // true when table view has focus (not query input)
	width    int
	height   int
}

func NewTableViewModel() TableViewModel {
	return TableViewModel{}
}

func (m *TableViewModel) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width = w
	m.height = h
	if m.ready {
		m.table.SetWidth(w - 2)
		m.table.SetHeight(h - 3) // -2 border, -1 header separator line
	}
}

func (m *TableViewModel) SetResult(result *n4j.QueryResult) {
	if len(result.Columns) == 0 {
		m.ready = false
		return
	}

	m.rowPaths = result.RowPaths

	// Distribute column widths to fill the border content area exactly.
	// Each cell has Padding(0,1) = 2 chars of padding per column.
	nCols := len(result.Columns)
	contentW := m.width - 2 - nCols*2 // inside border, minus cell padding
	colWidth := contentW / nCols
	if colWidth < 10 {
		colWidth = 10
	}
	remainder := contentW - colWidth*nCols

	cols := make([]table.Column, nCols)
	for i, c := range result.Columns {
		w := colWidth
		if i < remainder {
			w++ // spread extra pixels across the first columns
		}
		cols[i] = table.Column{Title: c, Width: w}
	}

	rows := make([]table.Row, len(result.Rows))
	for i, r := range result.Rows {
		row := make(table.Row, len(r))
		copy(row, r)
		rows[i] = row
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(m.height-3),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("86"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("62")).
		Bold(false)
	t.SetStyles(s)

	t.SetWidth(m.width - 2)
	m.table = t
	m.ready = true
}

func (m TableViewModel) Update(msg tea.Msg) (TableViewModel, tea.Cmd) {
	if !m.ready {
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m TableViewModel) View() string {
	if !m.ready {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render("No results to display")
	}

	tStyle := tableBorderStyle
	if m.active {
		tStyle = tableBorderFocusedStyle
	}

	return tStyle.Render(m.table.View())
}
