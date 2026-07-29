package graph

import (
	"fmt"
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
