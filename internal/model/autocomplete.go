package model

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type completionContext int

const (
	ctxKeyword completionContext = iota
	ctxLabel
	ctxRelType
	ctxPropKey // inside { } map literal: {na| or {name: "x", ag|
	ctxDotProp // after identifier dot: n.na|
)

const maxVisible = 8

var (
	popupBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86")).
				Padding(0, 1)

	popupItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	popupSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("86")).
				Bold(true)
)

type AutocompleteModel struct {
	visible  bool
	items    []string
	selected int
	prefix   string
	context  completionContext
	labels      []string
	relTypes    []string
	labelProps  map[string][]string
	relTypeProps map[string][]string
	// cursorCol tracks the column offset for popup positioning
	cursorCol int
	// wordStart and wordEnd are byte offsets of the full word being completed
	wordStart int
	wordEnd   int
}

func NewAutocompleteModel() AutocompleteModel {
	return AutocompleteModel{}
}

func (m *AutocompleteModel) SetSchema(labels, relTypes []string, labelProps, relTypeProps map[string][]string) {
	m.labels = labels
	m.relTypes = relTypes
	m.labelProps = labelProps
	m.relTypeProps = relTypeProps
}

// propsFor returns property names for a node label or relationship type.
func (m *AutocompleteModel) propsFor(entity string) []string {
	if props, ok := m.labelProps[entity]; ok {
		return props
	}
	return m.relTypeProps[entity]
}

func (m *AutocompleteModel) Show(items []string, prefix string, ctx completionContext, col int, wordStart, wordEnd int) {
	m.visible = true
	m.items = items
	m.prefix = prefix
	m.context = ctx
	m.cursorCol = col
	m.wordStart = wordStart
	m.wordEnd = wordEnd
	if m.selected >= len(items) {
		m.selected = 0
	}
}

func (m *AutocompleteModel) Hide() {
	m.visible = false
	m.items = nil
	m.selected = 0
	m.prefix = ""
}

func (m *AutocompleteModel) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *AutocompleteModel) MoveDown() {
	if m.selected < len(m.items)-1 {
		m.selected++
	}
}

// Accept returns the full completion text and the byte range to replace.
func (m *AutocompleteModel) Accept() (full string, start, end int) {
	if !m.visible || len(m.items) == 0 {
		return "", 0, 0
	}
	item := m.items[m.selected]
	s, e := m.wordStart, m.wordEnd
	m.Hide()
	return item, s, e
}

// Visible returns whether the popup is showing.
func (m AutocompleteModel) Visible() bool {
	return m.visible
}

// PopupHeight returns the rendered height of the popup including borders.
func (m AutocompleteModel) PopupHeight() int {
	if !m.visible {
		return 0
	}
	n := len(m.items)
	if n > maxVisible {
		n = maxVisible
	}
	return n + 2 // border top + bottom
}

// PopupIndent returns the horizontal offset for the popup.
func (m AutocompleteModel) PopupIndent() int {
	// Account for query box border (1) + padding (1) + prompt "cypher> " (8)
	indent := m.cursorCol + 10 - len(m.prefix)
	if indent < 0 {
		indent = 0
	}
	return indent
}

func (m AutocompleteModel) View() string {
	if !m.visible || len(m.items) == 0 {
		return ""
	}

	// Determine visible window
	start := 0
	end := len(m.items)
	if end > maxVisible {
		// Keep selected item visible
		if m.selected >= maxVisible {
			start = m.selected - maxVisible + 1
		}
		end = start + maxVisible
		if end > len(m.items) {
			end = len(m.items)
			start = end - maxVisible
		}
	}

	// Find max width for consistent item widths
	maxW := 0
	for _, item := range m.items[start:end] {
		if len(item) > maxW {
			maxW = len(item)
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		padded := m.items[i] + strings.Repeat(" ", maxW-len(m.items[i]))
		if i == m.selected {
			lines = append(lines, popupSelectedStyle.Render(padded))
		} else {
			lines = append(lines, popupItemStyle.Render(padded))
		}
	}

	content := strings.Join(lines, "\n")
	popup := popupBorderStyle.Render(content)

	// Indent popup to align with cursor
	indent := m.PopupIndent()
	if indent > 0 {
		indented := strings.Repeat(" ", indent)
		var result []string
		for _, line := range strings.Split(popup, "\n") {
			result = append(result, indented+line)
		}
		return strings.Join(result, "\n")
	}

	return popup
}

// extractContext analyzes the text at the cursor position to determine
// what kind of completions to offer, the prefix typed so far, and the
// byte range of the full word (including any characters after the cursor).
func extractContext(text string, cursorPos int) (prefix string, ctx completionContext, wordStart int, wordEnd int) {
	if cursorPos > len(text) {
		cursorPos = len(text)
	}
	before := text[:cursorPos]

	// Helper: find end of identifier/word from cursorPos forward
	findWordEnd := func() int {
		e := cursorPos
		for e < len(text) && isIdentChar(text[e]) {
			e++
		}
		return e
	}

	// Pass A: dot-notation (ctxDotProp)
	{
		ps := cursorPos
		for ps > 0 && isIdentChar(before[ps-1]) {
			ps--
		}
		if ps > 0 && before[ps-1] == '.' {
			dotIdx := ps - 1
			if dotIdx == 0 || !isDigit(before[dotIdx-1]) { // exclude float literals
				return before[ps:cursorPos], ctxDotProp, ps, findWordEnd()
			}
		}
	}

	// Pass B: map literal brace (ctxPropKey)
	{
		if ok, pfx, ps := scanForBraceContext(before, cursorPos); ok {
			return pfx, ctxPropKey, ps, findWordEnd()
		}
	}

	// Scan backwards from cursor to find context
	// Look for `:` preceded by `(` or `[` (possibly with variable name between)
	colonIdx := -1
	for i := len(before) - 1; i >= 0; i-- {
		ch := before[i]
		if ch == ' ' || ch == '\n' || ch == '\t' || ch == ')' || ch == ']' || ch == '{' || ch == '}' {
			break
		}
		if ch == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx >= 0 {
		labelStart := colonIdx + 1
		wEnd := findWordEnd()

		// Check what's before the colon — scan for ( or [
		for j := colonIdx - 1; j >= 0; j-- {
			ch := before[j]
			if ch == '(' {
				return before[labelStart:], ctxLabel, labelStart, wEnd
			}
			if ch == '[' {
				return before[labelStart:], ctxRelType, labelStart, wEnd
			}
			if !isIdentChar(ch) {
				break
			}
		}
		// Colon right after ( or [ with nothing between
		if colonIdx > 0 {
			ch := before[colonIdx-1]
			if ch == '(' {
				return before[labelStart:], ctxLabel, labelStart, wEnd
			}
			if ch == '[' {
				return before[labelStart:], ctxRelType, labelStart, wEnd
			}
		}
	}

	// Default: keyword context — extract current partial word
	wStart := cursorPos
	for wStart > 0 && isIdentChar(before[wStart-1]) {
		wStart--
	}
	wEnd := findWordEnd()
	return before[wStart:cursorPos], ctxKeyword, wStart, wEnd
}

// extractVarBindings scans a query for pattern variable→label/type bindings,
// e.g. (n:Person) → {"n": "Person"}, [r:KNOWS] → {"r": "KNOWS"}.
func extractVarBindings(query string) map[string]string {
	bindings := make(map[string]string)
	i := 0
	for i < len(query) {
		ch := query[i]
		// skip string literals
		if ch == '"' || ch == '\'' {
			quote := ch
			i++
			for i < len(query) && query[i] != quote {
				if query[i] == '\\' {
					i++
				}
				i++
			}
			i++
			continue
		}
		if ch == '(' || ch == '[' {
			i++
			for i < len(query) && (query[i] == ' ' || query[i] == '\t') {
				i++
			}
			varStart := i
			for i < len(query) && isIdentChar(query[i]) {
				i++
			}
			varName := query[varStart:i]
			if i < len(query) && query[i] == ':' {
				i++
				labelStart := i
				for i < len(query) && isIdentChar(query[i]) {
					i++
				}
				label := query[labelStart:i]
				if varName != "" && label != "" {
					if _, exists := bindings[varName]; !exists {
						bindings[varName] = label
					}
				}
			}
			continue
		}
		i++
	}
	return bindings
}

// findBraceEntityLabel scans backward from propStart to find the label or
// relationship type of the enclosing node/relationship pattern.
// Returns "" when no label is determinable (anonymous or no-label pattern).
func findBraceEntityLabel(text string, propStart int) string {
	// Step 1: find the opening { that contains propStart
	braceIdx := -1
	i := propStart - 1
	depth := 0
	for i >= 0 {
		ch := text[i]
		if ch == '"' || ch == '\'' {
			quote := ch
			i--
			for i >= 0 && text[i] != quote {
				i--
			}
		} else if ch == '}' {
			depth++
		} else if ch == '{' {
			if depth == 0 {
				braceIdx = i
				break
			}
			depth--
		}
		i--
	}
	if braceIdx < 0 {
		return ""
	}

	// Step 2: scan backward from { to find the enclosing ( or [
	j := braceIdx - 1
	for j >= 0 {
		ch := text[j]
		if ch == '(' || ch == '[' {
			break
		}
		if ch == ')' || ch == ']' {
			closer := ch
			opener := byte('(')
			if closer == ']' {
				opener = '['
			}
			j--
			d := 1
			for j >= 0 && d > 0 {
				if text[j] == closer {
					d++
				}
				if text[j] == opener {
					d--
				}
				j--
			}
			continue
		}
		j--
	}
	if j < 0 {
		return ""
	}

	// Step 3: extract label from text[j+1:braceIdx], e.g. "n:Person " or ":Person" or "r:KNOWS"
	inner := strings.TrimSpace(text[j+1 : braceIdx])
	colonIdx := strings.IndexByte(inner, ':')
	if colonIdx < 0 {
		return ""
	}
	after := inner[colonIdx+1:]
	end := 0
	for end < len(after) && isIdentChar(after[end]) {
		end++
	}
	return after[:end]
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func scanForBraceContext(before string, cursorPos int) (isPropKey bool, propPrefix string, propStart int) {
	ps := cursorPos
	for ps > 0 && isIdentChar(before[ps-1]) {
		ps--
	}
	prefix := before[ps:cursorPos]

	depth := 0
	foundComma := false
	i := ps - 1
	for i >= 0 {
		ch := before[i]
		switch {
		case ch == '"' || ch == '\'':
			quote := ch
			i--
			for i >= 0 && before[i] != quote {
				i--
			}
			i--
		case ch == '}':
			depth++
			i--
		case ch == '{':
			depth--
			if depth < 0 {
				return true, prefix, ps
			}
			i--
		case ch == ',' && depth == 0:
			foundComma = true
			i--
		case ch == ':' && depth == 0 && !foundComma:
			return false, "", 0 // cursor is on value side
		case (ch == '(' || ch == '[') && depth == 0:
			return false, "", 0
		default:
			i--
		}
	}
	return false, "", 0
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// filterCandidates returns candidates that match the prefix (case-insensitive).
// Order is preserved from the source list (keywords are ranked by frequency).
// If prefix is empty, all candidates are returned.
func filterCandidates(candidates []string, prefix string) []string {
	if prefix == "" {
		return candidates
	}
	upper := strings.ToUpper(prefix)
	var matches []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToUpper(c), upper) {
			matches = append(matches, c)
		}
	}
	return matches
}
