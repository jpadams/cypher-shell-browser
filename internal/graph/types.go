package graph

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type GraphNode struct {
	ID           int64
	Labels       []string
	Properties   map[string]any
	DisplayLabel string
	DisplayProps []string
	// Layout coords (grid)
	Layer int
	Order int
	// Character coords
	X, Y   int
	Width  int
	Height int
}

type GraphEdge struct {
	ID         int64
	Type       string
	StartID    int64
	EndID      int64
	Properties map[string]any
}

type Graph struct {
	Nodes map[int64]*GraphNode
	Edges []*GraphEdge
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[int64]*GraphNode),
	}
}

type RenderStyle int

const (
	StyleDetailed RenderStyle = iota
	StyleCompact
)

type StyledCell struct {
	Char     rune
	Style    lipgloss.Style
	HasStyle bool
}

type Canvas struct {
	Width  int
	Height int
	Cells  [][]StyledCell
}

func NewCanvas(width, height int) *Canvas {
	cells := make([][]StyledCell, height)
	for y := range cells {
		cells[y] = make([]StyledCell, width)
		for x := range cells[y] {
			cells[y][x] = StyledCell{Char: ' '}
		}
	}
	return &Canvas{Width: width, Height: height, Cells: cells}
}

func (c *Canvas) Set(x, y int, ch rune, style lipgloss.Style) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.Cells[y][x] = StyledCell{Char: ch, Style: style, HasStyle: true}
	}
}

func (c *Canvas) SetString(x, y int, s string, style lipgloss.Style) {
	for i, ch := range s {
		c.Set(x+i, y, ch, style)
	}
}

func (c *Canvas) Render() string {
	var sb strings.Builder
	for y, row := range c.Cells {
		if y > 0 {
			sb.WriteByte('\n')
		}
		for _, cell := range row {
			if cell.HasStyle {
				sb.WriteString(cell.Style.Render(string(cell.Char)))
			} else {
				sb.WriteRune(cell.Char)
			}
		}
	}
	return sb.String()
}
