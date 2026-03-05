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
	labels   []string
	relTypes []string
	// cursorCol tracks the column offset for popup positioning
	cursorCol int
	// wordStart and wordEnd are byte offsets of the full word being completed
	wordStart int
	wordEnd   int
}

func NewAutocompleteModel() AutocompleteModel {
	return AutocompleteModel{}
}

func (m *AutocompleteModel) SetSchema(labels, relTypes []string) {
	m.labels = labels
	m.relTypes = relTypes
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

	// Helper: find end of identifier/word from cursorPos forward
	findWordEnd := func() int {
		e := cursorPos
		for e < len(text) && isIdentChar(text[e]) {
			e++
		}
		return e
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
