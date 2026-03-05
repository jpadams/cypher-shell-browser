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

	cypherPrefixStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("226"))
)

// graphDetailEntry is a single navigable item in the detail panel.
type graphDetailEntry struct {
	isHeader bool
	label    string
	value    string
}

type GraphViewModel struct {
	graph        *graph.Graph
	style        graph.RenderStyle
	warning      string
	ready        bool
	active       bool
	width        int
	height       int
	scrollX      int
	maxLineLen   int
	rowPaths     [][]n4j.RowPathItem
	cypherPrefix string

	// Line-based rendering
	lines     []string // rendered Cypher lines (styled)
	lineToRow []int    // maps line index -> rowPaths index (-1 = non-data line)
	cursor    int      // selected line index
	scrollY   int      // vertical scroll offset

	// Detail panel
	showDetail   bool
	detailFocus  bool
	detailWidth  int
	entries      []graphDetailEntry
	propCursor   int
	expandedProp int
	detailScroll int
}

func NewGraphViewModel() GraphViewModel {
	return GraphViewModel{
		style:        graph.StyleCompact,
		propCursor:   -1,
		expandedProp: -1,
	}
}

func (m *GraphViewModel) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width = w
	m.height = h
}

func (m *GraphViewModel) SetResult(result *n4j.QueryResult) {
	g, warning := graph.ExtractGraph(result)
	m.graph = g
	m.warning = warning
	m.rowPaths = result.RowPaths
	m.ready = len(g.Nodes) > 0
	m.scrollX = 0
	m.cursor = 0
	m.scrollY = 0
	m.showDetail = false
	m.detailFocus = false
	m.propCursor = -1
	m.expandedProp = -1
	m.detailScroll = 0
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

func (m *GraphViewModel) ToggleStyle() {
	if m.style == graph.StyleDetailed {
		m.style = graph.StyleCompact
	} else {
		m.style = graph.StyleDetailed
		m.cypherPrefix = ""
	}
	m.scrollX = 0
	if m.ready {
		m.renderContent()
	}
}

func (m *GraphViewModel) renderContent() {
	var content string
	if m.style == graph.StyleCompact && len(m.rowPaths) > 0 {
		content = renderCompactFromRows(m.rowPaths, m.cypherPrefix)
	} else {
		content = graph.RenderGraph(m.graph, m.style)
	}

	m.lines = strings.Split(content, "\n")

	// Build lineToRow mapping: in compact mode each rendered line maps
	// 1:1 to the non-empty rowPaths entries. Other lines are -1.
	m.lineToRow = make([]int, len(m.lines))
	if m.style == graph.StyleCompact && len(m.rowPaths) > 0 {
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
	} else {
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

// moveCursor moves the cursor by delta, skipping non-data lines.
func (m *GraphViewModel) moveCursor(delta int) {
	cur := m.cursor + delta
	for cur >= 0 && cur < len(m.lines) {
		if m.isDataLine(cur) {
			m.cursor = cur
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
			text := m.plainCypherText()
			if text != "" {
				copyToClipboard(text)
			}
			return m, func() tea.Msg { return queryCopiedMsg{} }

		case "m":
			if !m.detailFocus {
				if m.cypherPrefix == "MERGE " {
					m.cypherPrefix = ""
				} else {
					m.cypherPrefix = "MERGE "
				}
				m.renderContent()
				return m, nil
			}

		case "c":
			if !m.detailFocus {
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
			Render("No graph data to display")
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

	if !m.isDataLine(m.cursor) {
		return
	}
	rowIdx := m.lineToRow[m.cursor]
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
			m.addDetailProps(item.Properties)
		} else {
			m.entries = append(m.entries, graphDetailEntry{isHeader: true, label: "[:" + item.Type + "]"})
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
	return m.height - 4
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
	return strings.Join(visible, "\n")
}

func (m GraphViewModel) renderDetailPropEntry(idx int, e graphDetailEntry, availWidth int) []string {
	isSelected := m.detailFocus && idx == m.propCursor
	isExpanded := idx == m.expandedProp

	keyStyle := graphDetailKeyStyle
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

func (m GraphViewModel) plainCypherText() string {
	if len(m.rowPaths) == 0 {
		return ""
	}
	var lines []string
	for _, path := range m.rowPaths {
		if len(path) == 0 {
			continue
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
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

func renderCompactFromRows(rowPaths [][]n4j.RowPathItem, prefix string) string {
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
				sb.WriteString(graph.RenderCompactNode(item.Labels, item.Properties))
			} else {
				sb.WriteString(graph.RenderCompactEdge(item.Type))
			}
		}
		lines = append(lines, sb.String())
	}
	if len(lines) == 0 {
		return "No graph data to display"
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
