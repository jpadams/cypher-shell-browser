package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	nodeLabelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	edgeStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	edgeLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	compactParenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	compactPropStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func RenderGraph(g *Graph) string {
	if len(g.Nodes) == 0 {
		return "No graph data to display"
	}
	return renderCompact(g)
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

// RenderCompactNode renders a single node in compact Cypher style. At
// VerbosityMinimal only the label is shown; the {…} property block is emitted
// from VerbosityMedium upward.
func RenderCompactNode(labels []string, props map[string]any, v Verbosity) string {
	label := formatNodeLabel(labels)
	var sb strings.Builder
	sb.WriteString(compactParenStyle.Render("("))
	sb.WriteString(nodeLabelStyle.Render(label))
	if v >= VerbosityMedium {
		fmtProps := formatNodeProps(props)
		if len(fmtProps) > 0 {
			sb.WriteString(compactPropStyle.Render(" {" + strings.Join(fmtProps, ", ") + "}"))
		}
	}
	sb.WriteString(compactParenStyle.Render(")"))
	return sb.String()
}

// RenderCompactEdge renders a single edge in compact Cypher style. Edge
// rendering is identical across the current verbosity levels; the parameter
// leaves room for a future relationship-property level.
func RenderCompactEdge(relType string, v Verbosity) string {
	label := fmt.Sprintf("[:%s]", relType)
	return edgeStyle.Render("-") + edgeLabelStyle.Render(label) + edgeStyle.Render("->")
}

// RenderCompactEdgeAfterBranch renders a relationship without its leading dash,
// for use where a tree branch connector already draws the horizontal stroke.
func RenderCompactEdgeAfterBranch(relType string, v Verbosity) string {
	return edgeLabelStyle.Render(fmt.Sprintf("[:%s]", relType)) + edgeStyle.Render("->")
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

func sortedNodeIDs(g *Graph) []int64 {
	ids := make([]int64, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
