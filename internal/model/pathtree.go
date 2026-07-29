package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeremyadams/cypher-shell-browser/internal/graph"
	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

var (
	treeBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	treeCountStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

const (
	// treeAlignLimit caps the count column: past this width the counts are
	// pushed so far right that aligning them just hides them off-screen.
	treeAlignLimit = 76
	treeCountGap   = 2
)

// pathTreeNode is one node of a result path, collapsed by *shape* — its labels
// and the relationship type that reached it — with properties ignored.  Rows
// sharing a shape prefix therefore share a branch, so a result made of many
// near-identical paths folds down to the handful of distinct shapes it holds.
type pathTreeNode struct {
	relType  string // relationship that led here; "" at the start of a path
	labels   []string
	children []*pathTreeNode
	index    map[string]*pathTreeNode

	// rowIdxs are the result rows whose path ends exactly here — every variant
	// behind this shape, so the detail panel can step through them all.
	rowIdxs  []int
	subtotal int // rows ending here or anywhere below
	firstRow int // a row passing through here, for purely structural nodes
}

func newPathTreeNode(relType string, labels []string) *pathTreeNode {
	return &pathTreeNode{
		relType: relType,
		labels:  labels,
		index:   make(map[string]*pathTreeNode),
	}
}

// buildPathTree folds result rows into a shape tree.  The returned node is a
// synthetic root whose children are the distinct path starts.
func buildPathTree(rowPaths [][]n4j.RowPathItem) *pathTreeNode {
	root := newPathTreeNode("", nil)

	for rowIdx, path := range rowPaths {
		if len(path) == 0 {
			continue
		}
		cur := root
		relType := ""
		for _, item := range path {
			if !item.IsNode {
				relType = item.Type
				continue
			}
			cur = cur.step(relType, item.Labels, rowIdx)
			relType = ""
		}
		cur.rowIdxs = append(cur.rowIdxs, rowIdx)
	}

	root.finalize()
	return root
}

// step descends to the child matching relType/labels, creating it if needed.
func (n *pathTreeNode) step(relType string, labels []string, rowIdx int) *pathTreeNode {
	key := relType + "\x00" + strings.Join(labels, "\x00")
	if existing, ok := n.index[key]; ok {
		return existing
	}
	child := newPathTreeNode(relType, labels)
	child.firstRow = rowIdx
	n.index[key] = child
	n.children = append(n.children, child)
	return child
}

// finalize computes subtree totals and orders children by weight, so the shapes
// that account for most of the result come first.  Equal weights keep their
// order of first appearance in the results.
func (n *pathTreeNode) finalize() int {
	n.subtotal = len(n.rowIdxs)
	for _, c := range n.children {
		n.subtotal += c.finalize()
	}
	sort.SliceStable(n.children, func(i, j int) bool {
		return n.children[i].subtotal > n.children[j].subtotal
	})
	return n.subtotal
}

// treeLine is one rendered line plus the result rows it stands for.
type treeLine struct {
	content string // styled, without the count column
	count   int    // rows ending on this line; 0 renders no count
	rows    []int  // rows the detail panel can step through; never empty
}

// renderPathTree renders rows as a collapsed shape tree.  It returns the styled
// text and, per line, every result row that line stands for — the detail panel
// steps through them, so a ×120 line does not hide 119 of its rows.
func renderPathTree(rowPaths [][]n4j.RowPathItem, v graph.Verbosity) (string, [][]int) {
	root := buildPathTree(rowPaths)
	if len(root.children) == 0 {
		return noGraphDataMessage, [][]int{nil}
	}

	var collected []treeLine
	for i, child := range root.children {
		child.appendLines(&collected, "", i == len(root.children)-1, true, v)
	}

	// Align the count column, unless the content is wide enough that doing so
	// would just push the counts out of view.
	width := 0
	for _, l := range collected {
		if l.count == 0 {
			continue
		}
		if w := lipgloss.Width(l.content); w > width {
			width = w
		}
	}
	if width > treeAlignLimit {
		width = 0
	}

	lines := make([]string, len(collected))
	rows := make([][]int, len(collected))
	for i, l := range collected {
		text := l.content
		if l.count > 0 {
			pad := treeCountGap
			if width > 0 {
				pad = width - lipgloss.Width(text) + treeCountGap
			}
			text += strings.Repeat(" ", pad) + treeCountStyle.Render(fmt.Sprintf("×%d", l.count))
		}
		lines[i] = text
		rows[i] = l.rows
	}
	return strings.Join(lines, "\n"), rows
}

// appendLines emits this node's line and recurses into its children.  A node
// with a single child and no rows ending on it is inlined onto the same line,
// so an unbranching run of steps reads as one path rather than a staircase.
func (n *pathTreeNode) appendLines(out *[]treeLine, prefix string, isLast, isTop bool, v graph.Verbosity) {
	connector := ""
	childPrefix := prefix
	if !isTop {
		if isLast {
			connector = treeBranchStyle.Render("└─")
			childPrefix = prefix + "   "
		} else {
			connector = treeBranchStyle.Render("├─")
			childPrefix = prefix + treeBranchStyle.Render("│") + "  "
		}
	}

	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(connector)
	// The connector already draws the relationship's leading dash.
	sb.WriteString(n.render(v, !isTop))

	// Collapse an unbranching run into this line.
	tail := n
	for len(tail.children) == 1 && len(tail.rowIdxs) == 0 {
		tail = tail.children[0]
		sb.WriteString(tail.render(v, false))
	}

	// A purely structural line has no rows of its own; offer one passing
	// through so the detail panel still shows something concrete.
	rows := tail.rowIdxs
	if len(rows) == 0 {
		rows = []int{tail.firstRow}
	}
	*out = append(*out, treeLine{content: sb.String(), count: len(tail.rowIdxs), rows: rows})

	for i, child := range tail.children {
		child.appendLines(out, childPrefix, i == len(tail.children)-1, false, v)
	}
}

// render draws this node's step — the relationship that reached it, if any,
// followed by the node itself — reusing the compact Cypher styling.  Properties
// are dropped: the tree groups by shape, and a shape has no property values.
func (n *pathTreeNode) render(v graph.Verbosity, afterBranch bool) string {
	var sb strings.Builder
	if n.relType != "" {
		if afterBranch {
			sb.WriteString(graph.RenderCompactEdgeAfterBranch(n.relType, v))
		} else {
			sb.WriteString(graph.RenderCompactEdge(n.relType, v))
		}
	}
	sb.WriteString(graph.RenderCompactNode(n.labels, nil, v))
	return sb.String()
}
