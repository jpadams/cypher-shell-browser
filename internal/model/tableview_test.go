package model

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

func manyRowResult(n int) *n4j.QueryResult {
	rows := make([][]string, n)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("row-%d", i)}
	}
	return &n4j.QueryResult{Columns: []string{"n"}, Rows: rows}
}

// A scrolled table that then gets squeezed into a tiny result area used to
// panic inside bubbles' viewport with "slice bounds out of range".  This
// happens in practice when the query textarea grows to several lines while
// table results are on screen.
func TestTableViewSurvivesShrinkAfterScrolling(t *testing.T) {
	m := NewTableViewModel()
	m.SetSize(80, 24)
	m.SetResult(manyRowResult(50))

	for i := 0; i < 40; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	for h := 24; h >= 0; h-- {
		m.SetSize(80, h)
		_ = m.View()
	}
}

func TestTableViewSurvivesTinyWidth(t *testing.T) {
	m := NewTableViewModel()
	m.SetSize(80, 24)
	m.SetResult(manyRowResult(20))

	for w := 80; w >= 0; w-- {
		m.SetSize(w, 24)
		_ = m.View()
	}
}

// Results arriving while the view is already tiny must not panic either.
func TestTableViewSetResultAtTinySize(t *testing.T) {
	m := NewTableViewModel()
	m.SetSize(4, 1)
	m.SetResult(manyRowResult(50))
	_ = m.View()
}

// The graph view renders by hand rather than through bubbles' viewport, but it
// shares the same shrinking-result-area path, so check it survives too.
func TestGraphViewSurvivesShrinking(t *testing.T) {
	result := &n4j.QueryResult{
		Columns: []string{"a", "r", "b"},
		Nodes: []n4j.ResultNode{
			{ID: 1, Labels: []string{"Person"}, Properties: map[string]any{"name": "Alice", "city": "Malmö"}},
			{ID: 2, Labels: []string{"Person"}, Properties: map[string]any{"name": "Bob"}},
			{ID: 3, Labels: []string{"Company"}, Properties: map[string]any{"name": "Acme Corp"}},
		},
		Edges: []n4j.ResultEdge{
			{ID: 10, Type: "KNOWS", StartID: 1, EndID: 2},
			{ID: 11, Type: "WORKS_AT", StartID: 1, EndID: 3},
		},
	}

	m := NewGraphViewModel()
	m.SetSize(80, 24)
	m.SetResult(result)
	m.active = true

	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	for h := 24; h >= 0; h-- {
		for _, w := range []int{80, 20, 6, 0} {
			m.SetSize(w, h)
			_ = m.View()
		}
	}
}
