package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	nodeBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // cyan
	nodeLabelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	nodePropStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // dim
	edgeStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	edgeLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	compactParenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	compactPropStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func RenderGraph(g *Graph, style RenderStyle) string {
	if len(g.Nodes) == 0 {
		return "No graph data to display"
	}

	switch style {
	case StyleCompact:
		return renderCompact(g)
	default:
		return renderDetailed(g)
	}
}

// renderCompact renders the graph as inline Cypher-like paths.
func renderCompact(g *Graph) string {
	if len(g.Edges) == 0 {
		var lines []string
		ids := sortedNodeIDs(g)
		for _, id := range ids {
			lines = append(lines, compactNodeStr(g.Nodes[id]))
		}
		return strings.Join(lines, "\n")
	}

	rendered := make(map[int64]bool)
	var lines []string

	outEdges := make(map[int64][]*GraphEdge)
	for _, e := range g.Edges {
		outEdges[e.StartID] = append(outEdges[e.StartID], e)
	}

	incomingFrom := make(map[int64]bool)
	for _, e := range g.Edges {
		incomingFrom[e.EndID] = true
	}

	seen := make(map[int64]bool)
	var uniqueStarts []int64
	for _, e := range g.Edges {
		if !incomingFrom[e.StartID] && !seen[e.StartID] {
			seen[e.StartID] = true
			uniqueStarts = append(uniqueStarts, e.StartID)
		}
	}
	sort.Slice(uniqueStarts, func(i, j int) bool { return uniqueStarts[i] < uniqueStarts[j] })

	if len(uniqueStarts) == 0 {
		for id := range outEdges {
			uniqueStarts = append(uniqueStarts, id)
		}
		sort.Slice(uniqueStarts, func(i, j int) bool { return uniqueStarts[i] < uniqueStarts[j] })
	}

	for _, startID := range uniqueStarts {
		lines = append(lines, followChain(g, startID, outEdges, rendered)...)
	}

	for _, e := range g.Edges {
		if !rendered[e.ID] {
			rendered[e.ID] = true
			lines = append(lines, compactEdgeLine(g, e))
		}
	}

	connectedNodes := make(map[int64]bool)
	for _, e := range g.Edges {
		connectedNodes[e.StartID] = true
		connectedNodes[e.EndID] = true
	}
	for _, id := range sortedNodeIDs(g) {
		if !connectedNodes[id] {
			lines = append(lines, compactNodeStr(g.Nodes[id]))
		}
	}

	return strings.Join(lines, "\n")
}

func followChain(g *Graph, startID int64, outEdges map[int64][]*GraphEdge, rendered map[int64]bool) []string {
	var lines []string

	currentID := startID
	for {
		var nextEdge *GraphEdge
		for _, e := range outEdges[currentID] {
			if !rendered[e.ID] {
				nextEdge = e
				break
			}
		}
		if nextEdge == nil {
			break
		}

		var sb strings.Builder
		sb.WriteString(compactNodeStr(g.Nodes[currentID]))

		chainID := currentID
		for nextEdge != nil {
			rendered[nextEdge.ID] = true
			sb.WriteString(compactRelStr(nextEdge))
			endNode := g.Nodes[nextEdge.EndID]
			if endNode != nil {
				sb.WriteString(compactNodeStr(endNode))
			}
			chainID = nextEdge.EndID

			nextEdge = nil
			for _, e := range outEdges[chainID] {
				if !rendered[e.ID] {
					nextEdge = e
					break
				}
			}
		}

		lines = append(lines, sb.String())

		hasMore := false
		for _, e := range outEdges[currentID] {
			if !rendered[e.ID] {
				hasMore = true
				break
			}
		}
		if !hasMore {
			break
		}
	}

	return lines
}

func compactNodeStr(node *GraphNode) string {
	var sb strings.Builder
	sb.WriteString(compactParenStyle.Render("("))
	sb.WriteString(nodeLabelStyle.Render(node.DisplayLabel))
	if len(node.DisplayProps) > 0 {
		sb.WriteString(compactPropStyle.Render(" {" + strings.Join(node.DisplayProps, ", ") + "}"))
	}
	sb.WriteString(compactParenStyle.Render(")"))
	return sb.String()
}

func compactRelStr(edge *GraphEdge) string {
	label := fmt.Sprintf("[:%s]", edge.Type)
	return edgeStyle.Render("-") + edgeLabelStyle.Render(label) + edgeStyle.Render("->")
}

func compactEdgeLine(g *Graph, e *GraphEdge) string {
	var sb strings.Builder
	if n := g.Nodes[e.StartID]; n != nil {
		sb.WriteString(compactNodeStr(n))
	}
	sb.WriteString(compactRelStr(e))
	if n := g.Nodes[e.EndID]; n != nil {
		sb.WriteString(compactNodeStr(n))
	}
	return sb.String()
}

// RenderCompactNode renders a single node in compact Cypher style.
func RenderCompactNode(labels []string, props map[string]any) string {
	label := formatNodeLabel(labels)
	fmtProps := formatNodeProps(props)
	var sb strings.Builder
	sb.WriteString(compactParenStyle.Render("("))
	sb.WriteString(nodeLabelStyle.Render(label))
	if len(fmtProps) > 0 {
		sb.WriteString(compactPropStyle.Render(" {" + strings.Join(fmtProps, ", ") + "}"))
	}
	sb.WriteString(compactParenStyle.Render(")"))
	return sb.String()
}

// RenderCompactEdge renders a single edge in compact Cypher style.
func RenderCompactEdge(relType string) string {
	label := fmt.Sprintf("[:%s]", relType)
	return edgeStyle.Render("-") + edgeLabelStyle.Render(label) + edgeStyle.Render("->")
}

// PlainCypherNode returns a plain-text Cypher representation of a node (no ANSI).
func PlainCypherNode(labels []string, props map[string]any) string {
	label := formatNodeLabel(labels)
	fmtProps := formatAllNodeProps(props)
	if len(fmtProps) > 0 {
		return fmt.Sprintf("(%s {%s})", label, strings.Join(fmtProps, ", "))
	}
	return fmt.Sprintf("(%s)", label)
}

// PlainCypherEdge returns a plain-text Cypher representation of an edge (no ANSI).
func PlainCypherEdge(relType string) string {
	return fmt.Sprintf("-[:%s]->", relType)
}

// renderDetailed renders the graph using canvas with Unicode box nodes and edge routing.
func renderDetailed(g *Graph) string {
	Layout(g, StyleDetailed)

	// Calculate canvas size
	maxX, maxY := 0, 0
	for _, node := range g.Nodes {
		right := node.X + node.Width
		bottom := node.Y + node.Height
		if right > maxX {
			maxX = right
		}
		if bottom > maxY {
			maxY = bottom
		}
	}

	canvasW := maxX + 10
	canvasH := maxY + 2

	canvas := NewCanvas(canvasW, canvasH)

	// Draw edges first (behind nodes)
	for _, edge := range g.Edges {
		drawDetailedEdge(canvas, g, edge)
	}

	// Draw nodes on top
	for _, node := range g.Nodes {
		drawDetailedNode(canvas, node)
	}

	return canvas.Render()
}

func drawDetailedNode(c *Canvas, node *GraphNode) {
	x, y := node.X, node.Y
	w := node.Width

	// Top border: ╭────╮
	c.Set(x, y, '╭', nodeBorderStyle)
	for i := 1; i < w-1; i++ {
		c.Set(x+i, y, '─', nodeBorderStyle)
	}
	c.Set(x+w-1, y, '╮', nodeBorderStyle)

	// Label row
	y++
	c.Set(x, y, '│', nodeBorderStyle)
	label := centerString(node.DisplayLabel, w-2)
	c.SetString(x+1, y, label, nodeLabelStyle)
	c.Set(x+w-1, y, '│', nodeBorderStyle)

	// Property rows
	for _, prop := range node.DisplayProps {
		y++
		c.Set(x, y, '│', nodeBorderStyle)
		pstr := padRight(prop, w-2)
		if len(pstr) > w-2 {
			pstr = pstr[:w-2]
		}
		c.SetString(x+1, y, pstr, nodePropStyle)
		c.Set(x+w-1, y, '│', nodeBorderStyle)
	}

	// Bottom border: ╰────╯
	y++
	c.Set(x, y, '╰', nodeBorderStyle)
	for i := 1; i < w-1; i++ {
		c.Set(x+i, y, '─', nodeBorderStyle)
	}
	c.Set(x+w-1, y, '╯', nodeBorderStyle)
}

func drawDetailedEdge(c *Canvas, g *Graph, edge *GraphEdge) {
	startNode := g.Nodes[edge.StartID]
	endNode := g.Nodes[edge.EndID]
	if startNode == nil || endNode == nil {
		return
	}

	label := fmt.Sprintf("[:%s]", edge.Type)

	if startNode.Layer == endNode.Layer {
		// Same layer: horizontal connection from right side of start to left side of end
		var left, right *GraphNode
		if startNode.X < endNode.X {
			left, right = startNode, endNode
		} else {
			left, right = endNode, startNode
		}
		sx := left.X + left.Width
		ex := right.X
		y := left.Y + left.Height/2

		// Draw horizontal line
		for x := sx; x < ex; x++ {
			c.Set(x, y, '─', edgeStyle)
		}
		// Arrowhead pointing toward endNode
		if startNode.X < endNode.X {
			c.Set(ex-1, y, '▶', edgeStyle)
		} else {
			c.Set(sx, y, '◀', edgeStyle)
		}
		// Label at midpoint above the line
		mid := (sx + ex) / 2
		labelStart := mid - len(label)/2
		if labelStart < sx {
			labelStart = sx
		}
		c.SetString(labelStart, y-1, label, edgeLabelStyle)
	} else {
		// Different layers: vertical connection
		// Start from bottom-center of start node, end at top-center of end node
		sx := startNode.X + startNode.Width/2
		sy := startNode.Y + startNode.Height // one below bottom border
		ex := endNode.X + endNode.Width/2
		ey := endNode.Y - 1 // one above top border

		if sy >= ey {
			// Not enough space, just draw arrow
			c.Set(ex, endNode.Y-1, '▼', edgeStyle)
			return
		}

		// Place label in the middle of the vertical span
		midY := (sy + ey) / 2

		if sx == ex {
			// Straight vertical line
			for y := sy; y <= ey; y++ {
				c.Set(sx, y, '│', edgeStyle)
			}
			// Arrowhead at bottom
			c.Set(ex, ey, '▼', edgeStyle)
			// Label to the right of the line at midpoint
			c.SetString(sx+1, midY, label, edgeLabelStyle)
		} else {
			// L-shaped or Z-shaped routing
			// Go down from start to midY, horizontal to ex, then down to end

			// Vertical segment from start down to midY
			for y := sy; y <= midY; y++ {
				c.Set(sx, y, '│', edgeStyle)
			}

			// Horizontal segment from sx to ex at midY
			minX, maxX := sx, ex
			if minX > maxX {
				minX, maxX = maxX, minX
			}
			for x := minX; x <= maxX; x++ {
				c.Set(x, midY, '─', edgeStyle)
			}

			// Corners
			if ex > sx {
				c.Set(sx, midY, '╰', edgeStyle)
				c.Set(ex, midY, '╮', edgeStyle)
			} else {
				c.Set(sx, midY, '╯', edgeStyle)
				c.Set(ex, midY, '╭', edgeStyle)
			}

			// Vertical segment from midY down to end
			for y := midY + 1; y <= ey; y++ {
				c.Set(ex, y, '│', edgeStyle)
			}

			// Arrowhead
			c.Set(ex, ey, '▼', edgeStyle)

			// Label along the horizontal segment
			labelX := minX + 1
			if labelX+len(label) > maxX {
				labelX = maxX - len(label)
			}
			if labelX < minX+1 {
				labelX = minX + 1
			}
			c.SetString(labelX, midY-1, label, edgeLabelStyle)
		}
	}
}

func sortedNodeIDs(g *Graph) []int64 {
	ids := make([]int64, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func centerString(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	result := make([]byte, width)
	for i := range result {
		result[i] = ' '
	}
	copy(result[pad:], s)
	return string(result)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	result := make([]byte, width)
	for i := range result {
		result[i] = ' '
	}
	copy(result, s)
	return string(result)
}
