package model

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeremyadams/cypher-shell-browser/internal/graph"
	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

func treeNode(id int64, label string) n4j.RowPathItem {
	return n4j.RowPathItem{
		IsNode:     true,
		ID:         id,
		Labels:     []string{label},
		Properties: map[string]any{"name": fmt.Sprintf("%s-%d", label, id)},
	}
}

func treeEdge(relType string) n4j.RowPathItem {
	return n4j.RowPathItem{Type: relType}
}

// path builds a row from an alternating label/relType spec: "Video", "HAS_SEGMENT",
// "Segment", ... Node IDs are derived from seq so each row gets distinct
// instances, which is what makes shape collapsing (rather than identity
// collapsing) observable.
func path(seq int64, steps ...string) []n4j.RowPathItem {
	var out []n4j.RowPathItem
	for i, s := range steps {
		if i%2 == 0 {
			out = append(out, treeNode(seq*100+int64(i), s))
		} else {
			out = append(out, treeEdge(s))
		}
	}
	return out
}

// videoRows mirrors the shape mix of `MATCH p=(:Video)-[*]->() RETURN p`.
func videoRows() [][]n4j.RowPathItem {
	var rows [][]n4j.RowPathItem
	seq := int64(0)
	add := func(n int, steps ...string) {
		for i := 0; i < n; i++ {
			seq++
			rows = append(rows, path(seq, steps...))
		}
	}
	add(2, "Video", "HAS_SEGMENT", "Segment")
	add(120, "Video", "HAS_SEGMENT", "Segment", "MENTIONS", "Entity")
	add(88, "Video", "HAS_SEGMENT", "Segment", "ABOUT", "Topic")
	add(38, "Video", "HAS_SEGMENT", "Segment", "NEXT", "Segment")
	add(20, "Video", "HAS_SEGMENT", "Segment", "NEXT", "Segment", "MENTIONS", "Entity")
	add(1, "Video", "HAS_SEGMENT", "Segment", "NEXT", "Segment", "ABOUT", "Topic")
	return rows
}

func plainTreeLines(t *testing.T, rows [][]n4j.RowPathItem) ([]string, [][]int) {
	t.Helper()
	content, lineRows := renderPathTree(rows, graph.VerbosityMedium)
	lines := strings.Split(content, "\n")
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = strings.TrimRight(stripAnsi(l), " ")
	}
	return plain, lineRows
}

func TestPathTreeCollapsesRowsByShape(t *testing.T) {
	rows := videoRows()
	lines, _ := plainTreeLines(t, rows)

	want := []string{
		"(:Video)-[:HAS_SEGMENT]->(:Segment)  ×2",
		"├─[:MENTIONS]->(:Entity)             ×120",
		"├─[:ABOUT]->(:Topic)                 ×88",
		"└─[:NEXT]->(:Segment)                ×38",
		"   ├─[:MENTIONS]->(:Entity)          ×20",
		"   └─[:ABOUT]->(:Topic)              ×1",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i, lines[i], want[i])
		}
	}
}

// Every result row must be accounted for exactly once by the ×N counts.
func TestPathTreeCountsSumToRowTotal(t *testing.T) {
	rows := videoRows()
	lines, _ := plainTreeLines(t, rows)

	sum := 0
	for _, l := range lines {
		if i := strings.LastIndex(l, "×"); i >= 0 {
			var n int
			if _, err := fmt.Sscanf(l[i:], "×%d", &n); err != nil {
				t.Fatalf("parsing count in %q: %v", l, err)
			}
			sum += n
		}
	}
	if sum != len(rows) {
		t.Errorf("counts sum to %d, want %d (every row counted once)", sum, len(rows))
	}
}

// Differing property values must not prevent collapsing; only shape matters.
func TestPathTreeIgnoresProperties(t *testing.T) {
	var rows [][]n4j.RowPathItem
	for i := int64(0); i < 5; i++ {
		p := path(i, "Video", "HAS_SEGMENT", "Segment")
		p[0].Properties = map[string]any{"title": fmt.Sprintf("wildly different %d", i)}
		rows = append(rows, p)
	}

	lines, _ := plainTreeLines(t, rows)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasSuffix(lines[0], "×5") {
		t.Errorf("line = %q, want it to collapse to ×5", lines[0])
	}
	if strings.Contains(lines[0], "{") {
		t.Errorf("line = %q, want no property values in a shape summary", lines[0])
	}
}

// An unbranching run collapses onto one line rather than a staircase.
func TestPathTreeInlinesUnbranchingRuns(t *testing.T) {
	rows := [][]n4j.RowPathItem{
		path(1, "A", "R1", "B", "R2", "C", "R3", "D"),
	}
	lines, _ := plainTreeLines(t, rows)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	want := "(:A)-[:R1]->(:B)-[:R2]->(:C)-[:R3]->(:D)  ×1"
	if lines[0] != want {
		t.Errorf("got %q, want %q", lines[0], want)
	}
}

// Distinct starting shapes each get their own top-level line.
func TestPathTreeForest(t *testing.T) {
	rows := [][]n4j.RowPathItem{
		path(1, "Video", "HAS_SEGMENT", "Segment"),
		path(2, "Video", "HAS_SEGMENT", "Segment"),
		path(3, "Channel", "PUBLISHED", "Video"),
	}
	lines, _ := plainTreeLines(t, rows)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	// Heavier branch first.
	if !strings.HasPrefix(lines[0], "(:Video)") || !strings.HasSuffix(lines[0], "×2") {
		t.Errorf("line 0 = %q, want the (:Video) branch with ×2 first", lines[0])
	}
	if !strings.HasPrefix(lines[1], "(:Channel)") {
		t.Errorf("line 1 = %q, want the (:Channel) branch", lines[1])
	}
}

// Each line points at a row that really has that line's shape, so the detail
// panel and Ctrl+Y show a genuine example.
func TestPathTreeRepresentativeRowsMatchTheirLine(t *testing.T) {
	rows := videoRows()
	_, lineRows := plainTreeLines(t, rows)

	wantLen := []int{3, 5, 5, 5, 7, 7}        // path item counts per tree line
	wantCount := []int{2, 120, 88, 38, 20, 1} // rows behind each tree line
	if len(lineRows) != len(wantLen) {
		t.Fatalf("got %d line row sets, want %d", len(lineRows), len(wantLen))
	}
	for i, set := range lineRows {
		if len(set) != wantCount[i] {
			t.Errorf("line %d carries %d rows, want %d", i, len(set), wantCount[i])
		}
		// Every row behind a line must genuinely have that line's shape.
		for _, rowIdx := range set {
			if rowIdx < 0 || rowIdx >= len(rows) {
				t.Fatalf("line %d: row index %d out of range", i, rowIdx)
			}
			if got := len(rows[rowIdx]); got != wantLen[i] {
				t.Errorf("line %d includes row %d with %d items, want a path of %d",
					i, rowIdx, got, wantLen[i])
			}
		}
	}
}

func TestPathTreeSingleNodeRows(t *testing.T) {
	rows := [][]n4j.RowPathItem{
		{treeNode(1, "Video")},
		{treeNode(2, "Video")},
		{treeNode(3, "Channel")},
	}
	lines, _ := plainTreeLines(t, rows)
	want := []string{"(:Video)    ×2", "(:Channel)  ×1"}
	if len(lines) != 2 || lines[0] != want[0] || lines[1] != want[1] {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestPathTreeEmpty(t *testing.T) {
	content, rows := renderPathTree(nil, graph.VerbosityMedium)
	if content != "No graph data to display" {
		t.Errorf("content = %q", content)
	}
	if len(rows) != 1 || rows[0] != nil {
		t.Errorf("rows = %v, want [nil]", rows)
	}

	// Rows present but all empty paths behave the same way.
	content, _ = renderPathTree([][]n4j.RowPathItem{{}, {}}, graph.VerbosityMedium)
	if content != "No graph data to display" {
		t.Errorf("all-empty content = %q", content)
	}
}

// The `t` key switches views, keeps the cursor on a data line, and restores the
// per-row listing on the way back.
func TestGraphViewTreeToggle(t *testing.T) {
	rows := videoRows()
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = rows
	m.ready = true
	m.renderContent()

	rowLines := len(m.lines)
	if rowLines != len(rows) {
		t.Fatalf("rows view has %d lines, want %d", rowLines, len(rows))
	}

	// Park the cursor deep in the list, past anything the tree will have.
	m.cursor = 200

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if !m.tree {
		t.Fatal("t did not enable the tree view")
	}
	if len(m.lines) != 6 {
		t.Errorf("tree view has %d lines, want 6", len(m.lines))
	}
	if !m.isDataLine(m.cursor) {
		t.Errorf("cursor %d is not on a data line after collapsing", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.tree {
		t.Fatal("t did not return to the rows view")
	}
	if len(m.lines) != rowLines {
		t.Errorf("rows view has %d lines, want %d", len(m.lines), rowLines)
	}
}

// The MERGE/CREATE prefix belongs to copyable per-row Cypher, so the tree view
// clears it and leaves m/c inert rather than mutating hidden state.
func TestGraphViewTreeClearsCypherPrefix(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if m.cypherPrefix != "MERGE " {
		t.Fatalf("cypherPrefix = %q, want MERGE ", m.cypherPrefix)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.cypherPrefix != "" {
		t.Errorf("cypherPrefix = %q, want it cleared on entering the tree", m.cypherPrefix)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if m.cypherPrefix != "" {
		t.Errorf("cypherPrefix = %q, want c inert in the tree view", m.cypherPrefix)
	}
}

// A tree line's detail panel shows the representative row's real properties.
func TestGraphViewTreeDetailUsesRepresentativeRow(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.tree = true
	m.renderContent()

	m.cursor = 1 // the ├─[:MENTIONS]->(:Entity) branch
	m.showDetail = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()

	var headers []string
	for _, e := range m.entries {
		if e.isHeader {
			headers = append(headers, e.label)
		}
	}
	// The detail panel walks the whole representative path, relationships included.
	want := []string{"(:Video)", "[:HAS_SEGMENT]", "(:Segment)", "[:MENTIONS]", "(:Entity)"}
	if len(headers) != len(want) {
		t.Fatalf("detail headers = %v, want %v", headers, want)
	}
	for i := range want {
		if headers[i] != want[i] {
			t.Errorf("header %d = %q, want %q", i, headers[i], want[i])
		}
	}
}

func treeModel(t *testing.T) GraphViewModel {
	t.Helper()
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.tree = true
	m.renderContent()
	return m
}

// Tab steps through every row behind a collapsed line, wrapping at the end, and
// Shift+Tab walks back — so a ×120 line exposes all 120 rows, not just one.
func TestGraphViewTreeCyclesVariants(t *testing.T) {
	m := treeModel(t)
	m.cursor = 1 // the ×120 MENTIONS branch
	m.showDetail = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()

	rows := m.variantRows()
	if len(rows) != 120 {
		t.Fatalf("line carries %d rows, want 120", len(rows))
	}

	seen := map[int]bool{m.currentRow(): true}
	for i := 1; i < len(rows); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if m.variant != i {
			t.Fatalf("after %d tabs variant = %d, want %d", i, m.variant, i)
		}
		row := m.currentRow()
		if seen[row] {
			t.Fatalf("variant %d repeated row %d", i, row)
		}
		seen[row] = true
	}
	if len(seen) != 120 {
		t.Errorf("reached %d distinct rows, want 120", len(seen))
	}

	// Wrap forward, then back.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.variant != 0 {
		t.Errorf("variant = %d after wrapping, want 0", m.variant)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.variant != 119 {
		t.Errorf("variant = %d after shift+tab from 0, want 119", m.variant)
	}
}

// The detail panel and Ctrl+Y follow the selected variant.
func TestGraphViewTreeVariantDrivesDetailAndCopy(t *testing.T) {
	m := treeModel(t)
	m.cursor = 1
	m.showDetail = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()

	first := m.plainCypherRow()
	firstEntries := append([]graphDetailEntry(nil), m.entries...)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	second := m.plainCypherRow()

	if first == "" || second == "" {
		t.Fatal("expected copyable Cypher for both variants")
	}
	if first == second {
		t.Errorf("Ctrl+Y text did not change with the variant: %q", first)
	}
	if len(m.entries) != len(firstEntries) {
		t.Fatalf("entry count changed across variants: %d vs %d", len(m.entries), len(firstEntries))
	}
	same := true
	for i := range m.entries {
		if m.entries[i] != firstEntries[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("detail panel showed identical values for two different variants")
	}
}

// Moving the cursor resets the variant, so a line never opens mid-way through.
func TestGraphViewTreeVariantResets(t *testing.T) {
	m := treeModel(t)
	m.cursor = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.variant == 0 {
		t.Fatal("expected a non-zero variant to start from")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.variant != 0 {
		t.Errorf("variant = %d after moving the cursor, want 0", m.variant)
	}

	m.variant = 3
	m.renderContent()
	if m.variant != 0 {
		t.Errorf("variant = %d after re-render, want 0", m.variant)
	}
}

// A line standing for one row has nothing to cycle, and Tab must not consume the
// key or fake a header.
func TestGraphViewTreeSingleRowLineHasNoVariants(t *testing.T) {
	m := treeModel(t)
	m.cursor = 5 // the ×1 ABOUT leaf

	if n := len(m.variantRows()); n != 1 {
		t.Fatalf("line carries %d rows, want 1", n)
	}
	if h := m.variantHeader(); h != "" {
		t.Errorf("variantHeader = %q, want empty for a single-row line", h)
	}
	if m.cycleVariant(1) {
		t.Error("cycleVariant reported handling a single-row line")
	}
}

// In the rows view every line is already one row, so nothing cycles.
func TestGraphViewRowsViewHasNoVariants(t *testing.T) {
	m := treeModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.tree {
		t.Fatal("expected the rows view")
	}
	if m.lineRows != nil {
		t.Error("lineRows should be nil in the rows view")
	}
	if n := len(m.variantRows()); n != 1 {
		t.Errorf("variantRows = %d, want 1 in the rows view", n)
	}
	if m.cycleVariant(1) {
		t.Error("cycleVariant should be inert in the rows view")
	}
}

// propVariantRows builds three rows of one shape whose (:Entity) property sets
// differ, so an anchored property may be absent from the next variant. Each node
// carries enough properties to force the detail panel to scroll.
func propVariantRows() [][]n4j.RowPathItem {
	bulk := func(prefix string, n int) map[string]any {
		props := map[string]any{"name": prefix + "-name"}
		for i := 0; i < n; i++ {
			props[fmt.Sprintf("%s%d", prefix, i)] = fmt.Sprintf("value-%d", i)
		}
		return props
	}

	entityProps := []map[string]any{
		{"name": "e0", "kind": "Person", "extra": "only-here"},
		{"name": "e1", "kind": "Place"},
		{"name": "e2"},
	}

	var rows [][]n4j.RowPathItem
	for i, ep := range entityProps {
		seq := int64(i + 1)
		p := path(seq, "Video", "HAS_SEGMENT", "Segment", "MENTIONS", "Entity")
		p[0].Properties = bulk("v", 8)
		p[2].Properties = bulk("s", 8)
		p[4].Properties = ep
		rows = append(rows, p)
	}
	return rows
}

func propVariantModel(t *testing.T) GraphViewModel {
	t.Helper()
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = propVariantRows()
	m.ready = true
	m.tree = true
	m.renderContent()
	m.cursor = 0
	m.showDetail = true
	m.detailFocus = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()
	return m
}

// entryAt reports the index of the entry with the given label, and which path
// item (header ordinal) it sits under.
func entryAt(t *testing.T, m GraphViewModel, label string) (idx, item int) {
	t.Helper()
	item = -1
	for i, e := range m.entries {
		if e.isHeader {
			item++
		}
		if e.label == label {
			return i, item
		}
	}
	t.Fatalf("no detail entry labelled %q in %d entries", label, len(m.entries))
	return -1, -1
}

// Cycling holds position on the same node/relationship and property instead of
// snapping back to the top of the panel.
func TestGraphViewVariantKeepsDetailPosition(t *testing.T) {
	m := propVariantModel(t)
	if n := len(m.variantRows()); n != 3 {
		t.Fatalf("line carries %d rows, want 3", n)
	}

	// Park on the (:Entity) property near the bottom, which requires scrolling.
	idx, item := entryAt(t, m, "kind")
	m.propCursor = idx
	m.ensureDetailPropVisible()
	if m.detailScroll == 0 {
		t.Fatalf("expected the panel to have scrolled to reach %q", "kind")
	}
	scrollBefore := m.detailScroll

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.variant != 1 {
		t.Fatalf("variant = %d, want 1", m.variant)
	}
	if m.detailScroll == 0 {
		t.Errorf("detailScroll reset to 0 — position was lost (was %d)", scrollBefore)
	}
	gotIdx, gotItem := entryAt(t, m, "kind")
	if m.propCursor != gotIdx {
		t.Errorf("propCursor = %d, want %d (the same property in the next variant)", m.propCursor, gotIdx)
	}
	if gotItem != item {
		t.Errorf("landed under path item %d, want %d", gotItem, item)
	}
	if m.entries[m.propCursor].value != "Place" {
		t.Errorf("value = %q, want the next variant's value", m.entries[m.propCursor].value)
	}
}

// When the anchored property is missing from the next variant, the panel stays on
// that same node rather than jumping to the top.
func TestGraphViewVariantFallsBackToSameItem(t *testing.T) {
	m := propVariantModel(t)

	idx, item := entryAt(t, m, "extra") // present only in variant 1
	m.propCursor = idx
	m.ensureDetailPropVisible()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.propCursor < 0 || m.propCursor >= len(m.entries) {
		t.Fatalf("propCursor = %d, out of range", m.propCursor)
	}
	// Should be somewhere within the same path item — its header or a property.
	gotItem := -1
	for i := 0; i <= m.propCursor; i++ {
		if m.entries[i].isHeader {
			gotItem++
		}
	}
	if gotItem != item {
		t.Errorf("landed under path item %d, want %d (the anchored node)", gotItem, item)
	}
	if m.propCursor == 0 {
		t.Error("propCursor fell back to the top of the panel")
	}
}

// An expanded value stays expanded across variants, so long values can be
// compared without re-expanding each time.
func TestGraphViewVariantKeepsExpansion(t *testing.T) {
	m := propVariantModel(t)

	idx, _ := entryAt(t, m, "kind")
	m.propCursor = idx
	m.expandedProp = idx
	m.ensureDetailPropVisible()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if m.expandedProp != m.propCursor {
		t.Errorf("expandedProp = %d, propCursor = %d — expansion not carried over",
			m.expandedProp, m.propCursor)
	}
	if got := m.entries[m.propCursor].label; got != "kind" {
		t.Errorf("expanded entry = %q, want kind", got)
	}
}

// Moving the line cursor is a different selection, so the panel resets there —
// anchoring is scoped to variant cycling. The detail pane must be unfocused for
// down to move the line rather than the property cursor.
func TestGraphViewLineMoveResetsDetailPosition(t *testing.T) {
	m := treeModel(t)
	m.cursor = 1
	m.showDetail = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()
	m.detailScroll = 3

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", m.cursor)
	}
	if m.detailScroll != 0 {
		t.Errorf("detailScroll = %d after moving to another line, want 0", m.detailScroll)
	}
}

// paneHasContent reports whether the left pane renders any of the tree, rather
// than an empty window scrolled past the end of the content.
func paneHasContent(m GraphViewModel) bool {
	for _, line := range strings.Split(m.renderCypherPane(), "\n") {
		if strings.Contains(stripAnsi(line), "(:") {
			return true
		}
	}
	return false
}

// Collapsing 269 rows into a 6-line tree while scrolled deep into the list used
// to leave the pane blank: scrollY stayed put while the content shrank beneath
// it. Short content must always show from the top.
func TestGraphViewTreeResetsScrollWhenContentShrinks(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	// Scroll deep into the row list.
	for i := 0; i < 200; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.scrollY == 0 {
		t.Fatal("expected the rows view to have scrolled")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	if len(m.lines) > m.visibleHeight() {
		t.Fatalf("tree is %d lines against a %d-line viewport; fixture no longer exercises the bug",
			len(m.lines), m.visibleHeight())
	}
	if m.scrollY != 0 {
		t.Errorf("scrollY = %d after collapsing, want 0 so the tree starts at the top", m.scrollY)
	}
	if !paneHasContent(m) {
		t.Error("left pane rendered no tree content — scrolled past the end")
	}
	if !m.isDataLine(m.cursor) {
		t.Errorf("cursor %d is not on a data line", m.cursor)
	}
}

// Going back to the long list must not leave the offset past the end either, and
// the cursor must stay in view.
func TestGraphViewRowsViewScrollStaysValid(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.tree = true
	m.renderContent()

	m.cursor = 5
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	if max := len(m.lines) - m.visibleHeight(); m.scrollY > max {
		t.Errorf("scrollY = %d, want at most %d", m.scrollY, max)
	}
	if m.cursor < m.scrollY || m.cursor >= m.scrollY+m.visibleHeight() {
		t.Errorf("cursor %d outside the window [%d, %d)", m.cursor, m.scrollY, m.scrollY+m.visibleHeight())
	}
	if !paneHasContent(m) {
		t.Error("left pane rendered no content")
	}
}

// Shrinking the pane must not scroll the content off screen.
func TestGraphViewShrinkKeepsContentOnScreen(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	for i := 0; i < 200; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	for _, h := range []int{20, 12, 8, 6} {
		m.SetSize(96, h)
		if max := len(m.lines) - m.visibleHeight(); m.visibleHeight() >= 1 && m.scrollY > max {
			t.Errorf("height %d: scrollY = %d, want at most %d", h, m.scrollY, max)
		}
		if !paneHasContent(m) {
			t.Errorf("height %d: left pane rendered no content", h)
		}
	}
}

// Toggling to the tree and back returns you to where you were in the long list.
func TestGraphViewTreeToggleRestoresRowsPosition(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	for i := 0; i < 200; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	wantCursor, wantScroll := m.cursor, m.scrollY
	if wantScroll == 0 {
		t.Fatal("expected the rows view to have scrolled")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.scrollY != 0 {
		t.Fatalf("tree should start at the top, scrollY = %d", m.scrollY)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.cursor != wantCursor {
		t.Errorf("cursor = %d, want %d restored", m.cursor, wantCursor)
	}
	if m.scrollY != wantScroll {
		t.Errorf("scrollY = %d, want %d restored", m.scrollY, wantScroll)
	}
	if !paneHasContent(m) {
		t.Error("left pane rendered no content after returning to the rows view")
	}
}

// The tree's own position is remembered too.
func TestGraphViewTreeToggleRestoresTreePosition(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	wantCursor := m.cursor
	if wantCursor == 0 {
		t.Fatal("expected the tree cursor to have moved")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}) // to rows
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}) // back to tree
	if m.cursor != wantCursor {
		t.Errorf("tree cursor = %d, want %d restored", m.cursor, wantCursor)
	}
}

// Re-running a query starts over at the top: a position held against the previous
// result must not be restored against new rows.
func TestGraphViewNewResultResetsRememberedPositions(t *testing.T) {
	rows := videoRows()
	result := &n4j.QueryResult{
		Columns:  []string{"p"},
		Nodes:    []n4j.ResultNode{{ID: 1, Labels: []string{"Video"}}},
		RowPaths: rows,
	}

	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.SetResult(result)

	for i := 0; i < 200; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	// Same query run again.
	m.SetResult(result)
	if m.cursor != 0 || m.scrollY != 0 {
		t.Errorf("cursor = %d, scrollY = %d after a new result, want 0, 0", m.cursor, m.scrollY)
	}
	if m.rowsPos.valid || m.treePos.valid {
		t.Error("remembered positions survived a new result")
	}

	// Toggling now must not resurrect the old position.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.scrollY != 0 {
		t.Errorf("scrollY = %d after toggling on a fresh result, want 0", m.scrollY)
	}
	if !paneHasContent(m) {
		t.Error("left pane rendered no content")
	}
}

// The detail panel follows the restored cursor rather than showing the row from
// before the toggle.
func TestGraphViewTreeToggleRefreshesDetail(t *testing.T) {
	m := NewGraphViewModel()
	m.SetSize(96, 20)
	m.rowPaths = videoRows()
	m.ready = true
	m.renderContent()

	for i := 0; i < 200; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m.showDetail = true
	m.detailWidth = m.detailPaneWidth()
	m.updateDetail()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	// In the tree the cursor sits on some shape line; the detail panel must
	// describe that line's row, not the row we left behind.
	wantRow := m.currentRow()
	if wantRow < 0 {
		t.Fatal("no current row in the tree view")
	}
	var headers int
	for _, e := range m.entries {
		if e.isHeader {
			headers++
		}
	}
	if headers != len(m.rowPaths[wantRow]) {
		t.Errorf("detail has %d headers, want %d for row %d", headers, len(m.rowPaths[wantRow]), wantRow)
	}
}
