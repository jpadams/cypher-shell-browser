package model

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeremyadams/cypher-shell-browser/internal/graph"
	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

var (
	graphBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86"))

	graphBorderDimStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	graphDetailBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	graphDetailBorderFocusedStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("86"))

	graphSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("62"))

	graphDetailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	graphDetailKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86"))

	graphDetailKeySelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("255")).
					Background(lipgloss.Color("62"))

	graphDetailValTruncStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("245"))

	graphDetailValStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	graphDetailDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	graphDetailInternalKeyStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("243")).
					Italic(true)

	graphDetailInternalValStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("243")).
					Italic(true)

	cypherPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("226"))
)

// noGraphDataMessage is shown when a result has nodes but no path rows to draw
// Cypher lines from.
const noGraphDataMessage = "No graph data to display"

// viewPos is a remembered scroll position in the left pane.
type viewPos struct {
	valid  bool
	cursor int
	scroll int
}

// graphDetailEntry is a single navigable item in the detail panel.
type graphDetailEntry struct {
	isHeader   bool
	isInternal bool // true for <id> / <elementId> metadata fields
	label      string
	value      string
}

type GraphViewModel struct {
	ready        bool
	active       bool
	width        int
	height       int
	scrollX      int
	maxLineLen   int
	rowPaths     [][]n4j.RowPathItem
	cypherPrefix string
	tree         bool // collapse rows into a shape tree instead of one row per line

	// Line-based rendering
	lines     []string // rendered Cypher lines (styled)
	lineToRow []int    // maps line index -> rowPaths index (-1 = non-data line)
	cursor    int      // selected line index
	scrollY   int      // vertical scroll offset

	// Tree view: one line stands for many rows, so each line carries the full
	// set and `variant` selects which of them the detail panel and copy show.
	lineRows [][]int
	variant  int

	// Remembered position of each view, so toggling between them returns you to
	// where you were instead of the top.  Cleared by SetResult: a position from
	// the previous result means nothing against new rows.
	rowsPos viewPos
	treePos viewPos

	// Detail panel
	showDetail   bool
	detailFocus  bool
	detailWidth  int
	entries      []graphDetailEntry
	propCursor   int
	expandedProp int
	detailScroll int
	showInternal bool            // whether to show <id>/<elementId> internal fields
	verbosity    graph.Verbosity // results-pane property verbosity (persists across queries)
}

func NewGraphViewModel() GraphViewModel {
	return GraphViewModel{
		propCursor:   -1,
		expandedProp: -1,
		verbosity:    graph.VerbosityMedium,
	}
}

func (m *GraphViewModel) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width = w
	m.height = h
	// A shorter pane can leave the old offset past the end of the content.
	m.clampScroll()
	m.ensureCursorVisible()
}

func (m *GraphViewModel) SetResult(result *n4j.QueryResult) {
	m.rowPaths = result.RowPaths
	m.ready = len(result.Nodes) > 0
	m.scrollX = 0
	m.cursor = 0
	m.scrollY = 0
	// A fresh result starts at the top, and positions remembered against the
	// previous one no longer mean anything.
	m.rowsPos = viewPos{}
	m.treePos = viewPos{}
	m.variant = 0
	m.showDetail = false
	m.detailFocus = false
	m.propCursor = -1
	m.expandedProp = -1
	m.detailScroll = 0
	m.showInternal = false
	if m.ready {
		m.renderContent()
		m.updateDetail()
	}
}

func (m *GraphViewModel) ResetPrefix() {
	if m.cypherPrefix != "" {
		m.cypherPrefix = ""
		if m.ready {
			m.renderContent()
		}
	}
}

func (m *GraphViewModel) renderContent() {
	hasRows := len(m.rowPaths) > 0

	var content string
	var treeRows [][]int
	switch {
	case hasRows && m.tree:
		// Each tree line stands for many rows, so the renderer supplies the
		// rows per line rather than the 1:1 mapping below.
		content, treeRows = renderPathTree(m.rowPaths, m.verbosity)
	case hasRows:
		content = renderCompactFromRows(m.rowPaths, m.cypherPrefix, m.verbosity)
	default:
		content = noGraphDataMessage
	}

	m.lines = strings.Split(content, "\n")

	// Build lineToRow mapping: in compact mode each rendered line maps
	// 1:1 to the non-empty rowPaths entries. Other lines are -1.
	m.lineToRow = make([]int, len(m.lines))
	m.lineRows = treeRows
	m.variant = 0
	switch {
	case treeRows != nil:
		// lineToRow keeps the first row of each line, so everything built on it
		// (cursor validity, copy fallbacks) behaves as in the rows view.
		for i := range m.lines {
			m.lineToRow[i] = -1
			if i < len(treeRows) && len(treeRows[i]) > 0 {
				m.lineToRow[i] = treeRows[i][0]
			}
		}
	case hasRows:
		rowIdx := 0
		for i := range m.lines {
			// renderCompactFromRows skips empty paths, so advance rowIdx
			// past empty ones to find the matching rowPath.
			for rowIdx < len(m.rowPaths) && len(m.rowPaths[rowIdx]) == 0 {
				rowIdx++
			}
			if rowIdx < len(m.rowPaths) {
				m.lineToRow[i] = rowIdx
				rowIdx++
			} else {
				m.lineToRow[i] = -1
			}
		}
	default:
		for i := range m.lines {
			m.lineToRow[i] = -1
		}
	}

	// Measure longest line for scroll clamping
	m.maxLineLen = 0
	for _, line := range m.lines {
		w := lipgloss.Width(line)
		if w > m.maxLineLen {
			m.maxLineLen = w
		}
	}

	// Clamp cursor to a valid data line
	m.clampCursor()
	// Switching views changes the line count wholesale — a scroll offset from
	// the previous view means nothing here, so bring the content back on screen.
	m.clampScroll()
	m.ensureCursorVisible()
}

// toggleTree switches between the row list and the shape tree, restoring the
// position you last held in the view being entered.  Without this, collapsing
// and expanding again would dump you back at the top of a long result.
func (m *GraphViewModel) toggleTree() {
	here := viewPos{valid: true, cursor: m.cursor, scroll: m.scrollY}
	var there viewPos
	if m.tree {
		m.treePos, there = here, m.rowsPos
	} else {
		m.rowsPos, there = here, m.treePos
	}

	m.tree = !m.tree
	// The MERGE/CREATE prefix belongs to copyable per-row Cypher, not to a
	// shape summary.
	if m.tree {
		m.cypherPrefix = ""
	}
	m.scrollX = 0
	m.renderContent()

	if there.valid {
		m.cursor = there.cursor
		m.scrollY = there.scroll
		// Re-clamp: the result may have fewer lines than when we left.
		m.clampCursor()
		m.clampScroll()
		m.ensureCursorVisible()
	}
	if m.showDetail {
		m.updateDetail()
	}
}

// clampScroll keeps the rendered content on screen. Content that fits the
// viewport always starts at the top; taller content is bounded so the last line
// can be reached but not scrolled past.
func (m *GraphViewModel) clampScroll() {
	vis := m.visibleHeight()
	if vis < 1 || len(m.lines) <= vis {
		m.scrollY = 0
		return
	}
	if max := len(m.lines) - vis; m.scrollY > max {
		m.scrollY = max
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

func (m *GraphViewModel) HasGraph() bool {
	return m.ready
}

// isDataLine returns true if the line at idx corresponds to a rowPath entry.
func (m *GraphViewModel) isDataLine(idx int) bool {
	if idx < 0 || idx >= len(m.lineToRow) {
		return false
	}
	return m.lineToRow[idx] >= 0
}

// clampCursor ensures the cursor is on a valid data line.
func (m *GraphViewModel) clampCursor() {
	if len(m.lines) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.lines) {
		m.cursor = len(m.lines) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	// If cursor is on a non-data line, move to the nearest data line below, then above.
	if !m.isDataLine(m.cursor) {
		for i := m.cursor; i < len(m.lines); i++ {
			if m.isDataLine(i) {
				m.cursor = i
				return
			}
		}
		for i := m.cursor; i >= 0; i-- {
			if m.isDataLine(i) {
				m.cursor = i
				return
			}
		}
	}
}

// variantRows returns the result rows the cursor's line stands for.  In the
// rows view that is the single row on the line; in the tree view it is every
// row sharing that line's shape.
func (m GraphViewModel) variantRows() []int {
	if !m.isDataLine(m.cursor) {
		return nil
	}
	if m.lineRows != nil && m.cursor < len(m.lineRows) {
		return m.lineRows[m.cursor]
	}
	return []int{m.lineToRow[m.cursor]}
}

// currentRow is the result row the detail panel and copy act on, honouring the
// selected variant.
func (m GraphViewModel) currentRow() int {
	rows := m.variantRows()
	if len(rows) == 0 {
		return -1
	}
	idx := m.variant
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	return rows[idx]
}

// cycleVariant steps to another row behind the current line.
func (m *GraphViewModel) cycleVariant(delta int) bool {
	n := len(m.variantRows())
	if n <= 1 {
		return false
	}
	// Cycling is for comparing the same thing across variants, so hold the
	// panel's position instead of snapping back to the top.
	anchor := m.captureDetailAnchor()
	m.variant = ((m.variant+delta)%n + n) % n
	if m.showDetail {
		m.updateDetail()
		m.restoreDetailAnchor(anchor)
	}
	return true
}

// moveCursor moves the cursor by delta, skipping non-data lines.
func (m *GraphViewModel) moveCursor(delta int) {
	cur := m.cursor + delta
	for cur >= 0 && cur < len(m.lines) {
		if m.isDataLine(cur) {
			m.cursor = cur
			m.variant = 0
			m.ensureCursorVisible()
			if m.showDetail {
				m.updateDetail()
			}
			return
		}
		cur += delta
	}
}

// visibleHeight returns the number of content lines visible inside the border.
func (m *GraphViewModel) visibleHeight() int {
	return m.height - 2 // top + bottom border
}

// cypherPaneWidth returns the width available for Cypher lines.
func (m *GraphViewModel) cypherPaneWidth() int {
	if m.showDetail {
		return m.width - m.detailPaneWidth()
	}
	return m.width
}

func (m *GraphViewModel) detailPaneWidth() int {
	return m.width * 45 / 100
}

func (m *GraphViewModel) ensureCursorVisible() {
	vis := m.visibleHeight()
	if vis <= 0 {
		return
	}
	if m.cursor < m.scrollY {
		m.scrollY = m.cursor
	}
	if m.cursor >= m.scrollY+vis {
		m.scrollY = m.cursor - vis + 1
	}
}

func (m GraphViewModel) Update(msg tea.Msg) (GraphViewModel, tea.Cmd) {
	if !m.ready {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+y":
			text := m.plainCypherRow()
			if text != "" {
				copyToClipboard(text)
			}
			return m, func() tea.Msg { return queryCopiedMsg{text: "Copied row to clipboard"} }

		case "ctrl+a":
			text := m.plainCypherText()
			if text != "" {
				copyToClipboard(text)
			}
			return m, func() tea.Msg {
				return queryCopiedMsg{text: fmt.Sprintf("Copied %d rows to clipboard", m.rowCount())}
			}

		case "m":
			if !m.detailFocus && !m.tree {
				if m.cypherPrefix == "MERGE " {
					m.cypherPrefix = ""
				} else {
					m.cypherPrefix = "MERGE "
				}
				m.renderContent()
				return m, nil
			}

		case "c":
			if !m.detailFocus && !m.tree {
				if m.cypherPrefix == "CREATE " {
					m.cypherPrefix = ""
				} else {
					m.cypherPrefix = "CREATE "
				}
				m.renderContent()
				return m, nil
			}

		case " ":
			if !m.detailFocus {
				m.showDetail = !m.showDetail
				if m.showDetail {
					m.detailWidth = m.detailPaneWidth()
					m.updateDetail()
				}
				return m, nil
			}
			// In detail focus, space toggles expand
			if m.propCursor >= 0 && m.propCursor < len(m.entries) && !m.entries[m.propCursor].isHeader {
				if m.expandedProp == m.propCursor {
					m.expandedProp = -1
				} else {
					m.expandedProp = m.propCursor
				}
				m.ensureDetailPropVisible()
			}
			return m, nil

		case "tab":
			if m.cycleVariant(1) {
				return m, nil
			}

		case "shift+tab":
			if m.cycleVariant(-1) {
				return m, nil
			}

		case "t":
			if !m.detailFocus {
				m.toggleTree()
				return m, nil
			}

		case "v", "V":
			if m.showDetail {
				m.showInternal = !m.showInternal
				m.updateDetail()
			} else {
				if m.verbosity == graph.VerbosityMedium {
					m.verbosity = graph.VerbosityMinimal
				} else {
					m.verbosity = graph.VerbosityMedium
				}
				m.scrollX = 0
				m.renderContent()
			}
			return m, nil

		case "enter":
			if !m.detailFocus && m.showDetail {
				m.detailFocus = true
				return m, nil
			}
			if m.detailFocus {
				// Toggle expand on enter
				if m.propCursor >= 0 && m.propCursor < len(m.entries) && !m.entries[m.propCursor].isHeader {
					if m.expandedProp == m.propCursor {
						m.expandedProp = -1
					} else {
						m.expandedProp = m.propCursor
					}
					m.ensureDetailPropVisible()
				}
				return m, nil
			}

		case "right", "l":
			if m.detailFocus {
				return m, nil
			}
			if m.showDetail && !m.detailFocus {
				m.detailFocus = true
				return m, nil
			}
			// Horizontal scroll
			viewW := m.cypherPaneWidth() - 2
			maxScroll := m.maxLineLen - viewW
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollX < maxScroll {
				m.scrollX += 4
				if m.scrollX > maxScroll {
					m.scrollX = maxScroll
				}
			}
			return m, nil

		case "left", "h":
			if m.detailFocus {
				m.detailFocus = false
				m.expandedProp = -1
				return m, nil
			}
			if m.scrollX > 0 {
				m.scrollX -= 4
				if m.scrollX < 0 {
					m.scrollX = 0
				}
			}
			return m, nil

		case "up", "k":
			if m.detailFocus {
				if m.scrollDetailExpandedProp(-1) {
					return m, nil
				}
				m.moveDetailPropCursor(-1)
				return m, nil
			}
			m.moveCursor(-1)
			return m, nil

		case "down", "j":
			if m.detailFocus {
				if m.scrollDetailExpandedProp(1) {
					return m, nil
				}
				m.moveDetailPropCursor(1)
				return m, nil
			}
			m.moveCursor(1)
			return m, nil
		}
	}

	return m, nil
}

func (m GraphViewModel) View() string {
	if !m.ready {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render(noGraphDataMessage)
	}

	cypherPane := m.renderCypherPane()

	if !m.showDetail {
		return cypherPane
	}

	detailPane := m.renderDetailPane()
	return lipgloss.JoinHorizontal(lipgloss.Top, cypherPane, detailPane)
}

func (m GraphViewModel) renderCypherPane() string {
	vis := m.visibleHeight()
	paneW := m.cypherPaneWidth() - 2 // subtract borders

	// Determine visible lines slice
	start := m.scrollY
	if start > len(m.lines) {
		start = len(m.lines)
	}
	end := start + vis
	if end > len(m.lines) {
		end = len(m.lines)
	}

	var rendered []string
	for i := start; i < end; i++ {
		line := m.lines[i]
		if m.scrollX > 0 {
			line = truncateLeft(line, m.scrollX)
		}
		// Truncate to pane width to prevent wrapping
		line = truncateToWidth(line, paneW)
		if i == m.cursor {
			// Render selected line: pad to pane width and apply highlight
			plain := stripAnsi(line)
			w := lipgloss.Width(plain)
			padded := plain
			if w < paneW {
				padded = padded + strings.Repeat(" ", paneW-w)
			}
			line = graphSelectedStyle.Render(padded)
		}
		rendered = append(rendered, line)
	}

	// Pad remaining lines to fill viewport
	for len(rendered) < vis {
		rendered = append(rendered, "")
	}

	content := strings.Join(rendered, "\n")

	style := graphBorderStyle
	if !m.active || m.detailFocus {
		style = graphBorderDimStyle
	}

	return style.Width(paneW).Render(content)
}

func (m GraphViewModel) renderDetailPane() string {
	style := graphDetailBorderStyle
	if m.active && m.detailFocus {
		style = graphDetailBorderFocusedStyle
	}

	dw := m.detailPaneWidth()
	vis := m.visibleHeight()
	content := m.renderDetailContent()

	// Pad content to match the left pane height so both borders align
	lines := strings.Split(content, "\n")
	for len(lines) < vis {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	return style.Width(dw - 2).Render(content)
}

// --- Detail panel logic (mirroring tableview.go) ---

func (m *GraphViewModel) updateDetail() {
	m.propCursor = -1
	m.expandedProp = -1
	m.detailScroll = 0
	m.entries = nil

	rowIdx := m.currentRow()
	if rowIdx < 0 || rowIdx >= len(m.rowPaths) || len(m.rowPaths[rowIdx]) == 0 {
		return
	}

	path := m.rowPaths[rowIdx]
	for _, item := range path {
		if item.IsNode {
			label := strings.Join(item.Labels, ":")
			if label == "" {
				label = "Node"
			}
			m.entries = append(m.entries, graphDetailEntry{isHeader: true, label: "(:" + label + ")"})
			if m.showInternal {
				m.entries = append(m.entries, graphDetailEntry{isInternal: true, label: "<id>", value: fmt.Sprintf("%d", item.ID)})
				m.entries = append(m.entries, graphDetailEntry{isInternal: true, label: "<elementId>", value: item.ElementID})
			}
			m.addDetailProps(item.Properties)
		} else {
			m.entries = append(m.entries, graphDetailEntry{isHeader: true, label: "[:" + item.Type + "]"})
			if m.showInternal {
				m.entries = append(m.entries, graphDetailEntry{isInternal: true, label: "<id>", value: fmt.Sprintf("%d", item.ID)})
				m.entries = append(m.entries, graphDetailEntry{isInternal: true, label: "<elementId>", value: item.ElementID})
			}
			m.addDetailProps(item.Properties)
		}
	}

	// Set cursor to first property
	for i, e := range m.entries {
		if !e.isHeader && e.label != "(no properties)" {
			m.propCursor = i
			break
		}
	}
}

// detailAnchor marks a place in the detail panel by what it refers to — which
// node or relationship of the path, and which property within it — rather than
// by line offset.  Variants of the same shape can carry different property sets,
// so a raw offset would drift; naming the position keeps you on the same
// relationship or property while cycling.
type detailAnchor struct {
	itemIdx  int    // ordinal of the node/relationship header, -1 if none
	propName string // property label under the cursor; "" means the header line
	expanded bool   // whether that entry was expanded
	scroll   int    // fallback offset when there is nothing to anchor to
}

// captureDetailAnchor records where the detail panel is currently pointing.
func (m GraphViewModel) captureDetailAnchor() detailAnchor {
	anchor := detailAnchor{itemIdx: -1, scroll: m.detailScroll}
	if m.propCursor < 0 || m.propCursor >= len(m.entries) {
		return anchor
	}

	item := -1
	for i := 0; i <= m.propCursor; i++ {
		if m.entries[i].isHeader {
			item++
		}
	}
	anchor.itemIdx = item
	if !m.entries[m.propCursor].isHeader {
		anchor.propName = m.entries[m.propCursor].label
	}
	anchor.expanded = m.expandedProp == m.propCursor
	return anchor
}

// restoreDetailAnchor moves the detail panel back to an anchored position after
// the entries have been rebuilt.  It falls back to the same path item's header,
// then to the top, when the anchored property is absent from this variant.
func (m *GraphViewModel) restoreDetailAnchor(anchor detailAnchor) {
	if anchor.itemIdx < 0 || len(m.entries) == 0 {
		m.detailScroll = anchor.scroll
		m.clampDetailScroll()
		return
	}

	start, item := -1, -1
	for i, e := range m.entries {
		if !e.isHeader {
			continue
		}
		item++
		if item == anchor.itemIdx {
			start = i
			break
		}
	}
	if start < 0 {
		return // this variant has no such item; leave the fresh position alone
	}

	target := start
	if anchor.propName != "" {
		for i := start + 1; i < len(m.entries) && !m.entries[i].isHeader; i++ {
			if m.entries[i].label == anchor.propName {
				target = i
				break
			}
		}
	}

	m.propCursor = target
	if anchor.expanded && !m.entries[target].isHeader {
		m.expandedProp = target
	}
	m.ensureDetailPropVisible()
}

// clampDetailScroll keeps the scroll offset within the rendered content.
func (m *GraphViewModel) clampDetailScroll() {
	total := 0
	for i := range m.entries {
		total += m.detailEntryLineCount(i)
	}
	max := total - m.detailVisibleLines()
	if max < 0 {
		max = 0
	}
	if m.detailScroll > max {
		m.detailScroll = max
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
}

func (m *GraphViewModel) addDetailProps(props map[string]any) {
	if len(props) == 0 {
		m.entries = append(m.entries, graphDetailEntry{isHeader: false, label: "(no properties)", value: ""})
		return
	}

	written := make(map[string]bool)
	for _, key := range []string{"name", "title", "id"} {
		if v, ok := props[key]; ok {
			m.entries = append(m.entries, graphDetailEntry{
				label: key,
				value: graphFormatPropValue(v),
			})
			written[key] = true
		}
	}

	keys := make([]string, 0, len(props))
	for k := range props {
		if !written[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		m.entries = append(m.entries, graphDetailEntry{
			label: k,
			value: graphFormatPropValue(props[k]),
		})
	}
}

func graphFormatPropValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case []any:
		items := make([]string, len(val))
		for i, item := range val {
			items[i] = fmt.Sprintf("%v", item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case string:
		if val == "" {
			return ""
		}
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (m *GraphViewModel) detailVisibleLines() int {
	vis := m.height - 4
	if m.variantHeader() != "" {
		vis-- // the variant header is pinned and does not scroll
	}
	return vis
}

// variantHeader labels which of a line's rows the detail panel is showing, and
// is empty when the line stands for a single row.
func (m GraphViewModel) variantHeader() string {
	rows := m.variantRows()
	if len(rows) <= 1 {
		return ""
	}
	idx := m.variant
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	return graphDetailDimStyle.Render(fmt.Sprintf("variant %d of %d · Tab", idx+1, len(rows)))
}

func (m *GraphViewModel) ensureDetailPropVisible() {
	vis := m.detailVisibleLines()
	if vis <= 0 {
		return
	}

	linesBefore := 0
	for i := 0; i < m.propCursor && i < len(m.entries); i++ {
		linesBefore += m.detailEntryLineCount(i)
	}

	if linesBefore < m.detailScroll {
		m.detailScroll = linesBefore
	}

	linesThrough := linesBefore + m.detailEntryLineCount(m.propCursor)
	if linesThrough > m.detailScroll+vis {
		m.detailScroll = linesThrough - vis
	}

	if linesBefore < m.detailScroll {
		m.detailScroll = linesBefore
	}
}

func (m *GraphViewModel) detailEntryLineCount(idx int) int {
	if idx < 0 || idx >= len(m.entries) {
		return 1
	}
	e := m.entries[idx]
	if e.isHeader {
		return 1
	}
	dw := m.detailPaneWidth()
	if idx == m.expandedProp && len(e.value) > 0 {
		totalAvail := dw - 4
		prefixPlain := len(e.label) + 4
		firstLineAvail := totalAvail - prefixPlain
		if firstLineAvail < 10 {
			firstLineAvail = 10
		}
		if len(e.value) <= firstLineAvail {
			return 1
		}
		remaining := len(e.value) - firstLineAvail
		wrapWidth := totalAvail - 4
		if wrapWidth < 10 {
			wrapWidth = 10
		}
		return 1 + (remaining+wrapWidth-1)/wrapWidth
	}
	return 1
}

func (m *GraphViewModel) scrollDetailExpandedProp(delta int) bool {
	if m.expandedProp < 0 || m.expandedProp != m.propCursor {
		return false
	}
	vis := m.detailVisibleLines()
	entryLines := m.detailEntryLineCount(m.expandedProp)
	if entryLines <= vis {
		return false
	}

	linesBefore := 0
	for i := 0; i < m.expandedProp && i < len(m.entries); i++ {
		linesBefore += m.detailEntryLineCount(i)
	}
	entryEnd := linesBefore + entryLines

	newScroll := m.detailScroll + delta
	if newScroll < linesBefore {
		return false
	}
	if newScroll+vis > entryEnd {
		return false
	}
	m.detailScroll = newScroll
	return true
}

func (m *GraphViewModel) moveDetailPropCursor(delta int) {
	if len(m.entries) == 0 {
		return
	}
	cur := m.propCursor + delta
	for cur >= 0 && cur < len(m.entries) {
		if !m.entries[cur].isHeader && m.entries[cur].label != "(no properties)" {
			m.propCursor = cur
			m.ensureDetailPropVisible()
			return
		}
		cur += delta
	}
}

func (m GraphViewModel) renderDetailContent() string {
	if len(m.entries) == 0 {
		return graphDetailDimStyle.Render("No entity details")
	}

	dw := m.detailPaneWidth()
	availWidth := dw - 4
	if availWidth < 10 {
		availWidth = 10
	}

	var allLines []string
	for i, e := range m.entries {
		if e.isHeader {
			allLines = append(allLines, graphDetailTitleStyle.Render(e.label))
		} else {
			allLines = append(allLines, m.renderDetailPropEntry(i, e, availWidth)...)
		}
	}

	vis := m.detailVisibleLines()
	if vis <= 0 {
		vis = 1
	}
	start := m.detailScroll
	if start > len(allLines) {
		start = len(allLines)
	}
	end := start + vis
	if end > len(allLines) {
		end = len(allLines)
	}

	visible := allLines[start:end]
	if header := m.variantHeader(); header != "" {
		visible = append([]string{header}, visible...)
	}
	return strings.Join(visible, "\n")
}

func (m GraphViewModel) renderDetailPropEntry(idx int, e graphDetailEntry, availWidth int) []string {
	isSelected := m.detailFocus && idx == m.propCursor
	isExpanded := idx == m.expandedProp

	keyStyle := graphDetailKeyStyle
	if e.isInternal {
		keyStyle = graphDetailInternalKeyStyle
	}
	if isSelected {
		keyStyle = graphDetailKeySelectedStyle
	}

	if e.label == "(no properties)" {
		return []string{"  " + graphDetailDimStyle.Render(e.label)}
	}

	if e.value == "" {
		return []string{"  " + keyStyle.Render(e.label+":")}
	}

	prefix := "  " + keyStyle.Render(e.label+": ")
	prefixPlain := len(e.label) + 4
	valAvail := availWidth - prefixPlain
	if valAvail < 10 {
		valAvail = 10
	}

	valStr := e.value
	valStyle := graphDetailValTruncStyle
	if e.isInternal {
		valStyle = graphDetailInternalValStyle
	}
	if isSelected {
		valStyle = graphDetailKeySelectedStyle
	}

	if !isExpanded {
		if len(valStr) > valAvail {
			valStr = valStr[:valAvail-1] + "…"
		}
		return []string{prefix + valStyle.Render(valStr)}
	}

	expandValStyle := graphDetailValStyle
	if e.isInternal {
		expandValStyle = graphDetailInternalValStyle
	}
	if isSelected {
		expandValStyle = graphDetailKeySelectedStyle
	}
	if len(valStr) <= valAvail {
		return []string{prefix + expandValStyle.Render(valStr)}
	}

	const wrapIndent = 4
	wrapWidth := availWidth - wrapIndent
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	var lines []string
	indent := strings.Repeat(" ", wrapIndent)
	remaining := valStr

	firstChunk := valAvail
	if firstChunk > len(remaining) {
		firstChunk = len(remaining)
	}
	lines = append(lines, prefix+expandValStyle.Render(remaining[:firstChunk]))
	remaining = remaining[firstChunk:]

	for len(remaining) > 0 {
		chunk := wrapWidth
		if chunk > len(remaining) {
			chunk = len(remaining)
		}
		lines = append(lines, indent+expandValStyle.Render(remaining[:chunk]))
		remaining = remaining[chunk:]
	}
	return lines
}

// --- Existing helpers ---

// plainCypherText returns every result row as plain Cypher, one per line.
func (m GraphViewModel) plainCypherText() string {
	if len(m.rowPaths) == 0 {
		return ""
	}
	var lines []string
	for _, path := range m.rowPaths {
		if line := m.plainCypherPath(path); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// plainCypherRow returns the plain Cypher for the row under the cursor.
func (m GraphViewModel) plainCypherRow() string {
	rowIdx := m.currentRow()
	if rowIdx < 0 || rowIdx >= len(m.rowPaths) {
		return ""
	}
	return m.plainCypherPath(m.rowPaths[rowIdx])
}

// plainCypherPath renders a single row path as plain Cypher (with the active
// MERGE/CREATE prefix). Returns "" for an empty path.
func (m GraphViewModel) plainCypherPath(path []n4j.RowPathItem) string {
	if len(path) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(m.cypherPrefix)
	for _, item := range path {
		if item.IsNode {
			sb.WriteString(graph.PlainCypherNode(item.Labels, item.Properties))
		} else {
			sb.WriteString(graph.PlainCypherEdge(item.Type))
		}
	}
	return sb.String()
}

// rowCount returns the number of non-empty result rows.
func (m GraphViewModel) rowCount() int {
	n := 0
	for _, path := range m.rowPaths {
		if len(path) > 0 {
			n++
		}
	}
	return n
}

func renderCompactFromRows(rowPaths [][]n4j.RowPathItem, prefix string, v graph.Verbosity) string {
	var lines []string
	styledPrefix := ""
	if prefix != "" {
		styledPrefix = cypherPrefixStyle.Render(prefix)
	}
	for _, path := range rowPaths {
		if len(path) == 0 {
			continue
		}
		var sb strings.Builder
		sb.WriteString(styledPrefix)
		for _, item := range path {
			if item.IsNode {
				sb.WriteString(graph.RenderCompactNode(item.Labels, item.Properties, v))
			} else {
				sb.WriteString(graph.RenderCompactEdge(item.Type, v))
			}
		}
		lines = append(lines, sb.String())
	}
	if len(lines) == 0 {
		return noGraphDataMessage
	}
	return strings.Join(lines, "\n")
}

// truncateLeft removes the first n visible characters from a string,
// handling ANSI escape sequences correctly.
func truncateLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	runes := []rune(s)
	visibleSkipped := 0
	i := 0
	inEscape := false

	for i < len(runes) && visibleSkipped < n {
		if runes[i] == '\x1b' {
			inEscape = true
			i++
			continue
		}
		if inEscape {
			if (runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') {
				inEscape = false
			}
			i++
			continue
		}
		visibleSkipped++
		i++
	}
	for i < len(runes) && runes[i] == '\x1b' {
		for i < len(runes) {
			ch := runes[i]
			i++
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				break
			}
		}
	}
	return string(runes[i:])
}

// truncateToWidth truncates a string (which may contain ANSI escapes) to
// at most maxWidth visible terminal cells, using lipgloss.Width for accurate
// emoji/unicode measurement (handles variation selectors correctly).
func truncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	// Binary search for the right cut point: find the largest prefix
	// whose lipgloss.Width <= maxWidth.
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lipgloss.Width(string(runes[:mid])) <= maxWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo])
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	runes := []rune(s)
	var out []rune
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			i++
			for i < len(runes) {
				ch := runes[i]
				i++
				if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
					break
				}
			}
			continue
		}
		out = append(out, runes[i])
		i++
	}
	return string(out)
}
