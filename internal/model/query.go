package model

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeremyadams/cypher-shell-browser/internal/config"
	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

var (
	queryBorderFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86")).
				Padding(0, 1)

	queryBorderBlurredStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)

	queryPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)
)

type QueryModel struct {
	textarea     textarea.Model
	client       *n4j.Client
	history      []string
	histIdx      int
	width        int
	browsing     bool   // true when navigating history
	savedInput   string // saves current input when entering history
	pendingQuery string // query awaiting success/failure before adding to history
	autocomplete AutocompleteModel
}

type queryResultMsg struct {
	result *n4j.QueryResult
}

type queryErrorMsg struct {
	err error
}

type queryCopiedMsg struct{}

func NewQueryModel(client *n4j.Client) QueryModel {
	ta := textarea.New()
	ta.Placeholder = "Enter Cypher query... (Ctrl+E to execute)"
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = "cypher> "
	ta.FocusedStyle.Prompt = queryPromptStyle
	ta.CharLimit = 0

	history := config.LoadHistory()

	return QueryModel{
		textarea:     ta,
		client:       client,
		history:      history,
		histIdx:      len(history),
		autocomplete: NewAutocompleteModel(),
	}
}

func (m *QueryModel) SetWidth(w int) {
	m.width = w
	m.textarea.SetWidth(w - 4)
}

func (m *QueryModel) SetClient(client *n4j.Client) {
	m.client = client
}

func (m *QueryModel) Focus() tea.Cmd {
	return m.textarea.Focus()
}

func (m *QueryModel) Blur() {
	m.textarea.Blur()
	m.autocomplete.Hide()
}

// Height returns the rendered height of the query box (textarea + borders + popup).
func (m QueryModel) Height() int {
	return m.textarea.Height() + 2 + m.autocomplete.PopupHeight() // borders + popup
}

func (m QueryModel) Value() string {
	return m.textarea.Value()
}

func (m QueryModel) Update(msg tea.Msg) (QueryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When autocomplete popup is visible, intercept certain keys
		if m.autocomplete.Visible() {
			switch msg.String() {
			case "tab":
				full, start, end := m.autocomplete.Accept()
				if full != "" {
					// Replace the entire word (before and after cursor) with the completion
					val := m.textarea.Value()
					newVal := val[:start] + full + val[end:]
					newCursor := start + len(full)
					m.textarea.SetValue(newVal)
					m.setCursorToBytePos(newCursor)
				}
				m.autoResize()
				return m, nil
			case "up":
				m.autocomplete.MoveUp()
				return m, nil
			case "down":
				m.autocomplete.MoveDown()
				return m, nil
			case "esc":
				m.autocomplete.Hide()
				return m, nil
			}
			// All other keys fall through to normal handling
		}

		switch msg.String() {
		case "ctrl+y":
			query := m.textarea.Value()
			if query != "" {
				copyToClipboard(query)
			}
			return m, func() tea.Msg { return queryCopiedMsg{} }
		case "ctrl+e":
			query := m.textarea.Value()
			if query == "" {
				return m, nil
			}
			m.pendingQuery = query
			m.browsing = false
			m.autocomplete.Hide()
			return m, m.executeQuery(query)
		case "ctrl+l":
			m.textarea.Reset()
			m.histIdx = len(m.history)
			m.browsing = false
			m.autocomplete.Hide()
			m.autoResize()
			return m, nil
		case "up":
			// Navigate history when cursor is on the first line
			if m.textarea.Line() == 0 && len(m.history) > 0 {
				if !m.browsing {
					m.savedInput = m.textarea.Value()
					m.browsing = true
					m.histIdx = len(m.history)
				}
				if m.histIdx > 0 {
					m.histIdx--
					m.textarea.SetValue(m.history[m.histIdx])
					m.textarea.CursorEnd()
					m.autoResize()
				}
				return m, nil
			}
		case "down":
			// Navigate history when cursor is on the last line
			if m.textarea.Line() == m.textarea.LineCount()-1 && m.browsing {
				if m.histIdx < len(m.history)-1 {
					m.histIdx++
					m.textarea.SetValue(m.history[m.histIdx])
					m.textarea.CursorEnd()
					m.autoResize()
				} else {
					// Restore saved input
					m.histIdx = len(m.history)
					m.browsing = false
					m.textarea.SetValue(m.savedInput)
					m.textarea.CursorEnd()
					m.autoResize()
				}
				return m, nil
			}
		}
	}

	// Any non-arrow key while browsing exits history mode
	if msg, ok := msg.(tea.KeyMsg); ok {
		key := msg.String()
		if m.browsing && key != "up" && key != "down" {
			m.browsing = false
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.autoResize()
	m.updateAutocomplete()
	return m, cmd
}

// updateAutocomplete evaluates the current text and cursor position to
// show or hide the autocomplete popup with filtered suggestions.
func (m *QueryModel) updateAutocomplete() {
	if !m.textarea.Focused() {
		m.autocomplete.Hide()
		return
	}
	text := m.textarea.Value()
	if text == "" {
		m.autocomplete.Hide()
		return
	}

	// Compute byte cursor position from line and column info
	cursorPos := m.cursorBytePos()
	prefix, ctx, wordStart, wordEnd := extractContext(text, cursorPos)

	// Labels and rel types show immediately after (: or [:, keywords need 2+ chars
	if ctx == ctxKeyword && len(prefix) < 2 {
		m.autocomplete.Hide()
		return
	}

	var candidates []string
	switch ctx {
	case ctxLabel:
		candidates = filterCandidates(m.autocomplete.labels, prefix)
	case ctxRelType:
		candidates = filterCandidates(m.autocomplete.relTypes, prefix)
	case ctxKeyword:
		candidates = filterCandidates(cypherKeywords, prefix)
	}

	// Hide if no matches or only exact match
	if len(candidates) == 0 {
		m.autocomplete.Hide()
		return
	}
	fullWord := text[wordStart:wordEnd]
	if len(candidates) == 1 && strings.EqualFold(candidates[0], fullWord) {
		m.autocomplete.Hide()
		return
	}

	// Compute column for popup positioning
	col := 0
	if info := m.textarea.LineInfo(); info.Width > 0 {
		col = info.ColumnOffset
	}

	m.autocomplete.Show(candidates, prefix, ctx, col, wordStart, wordEnd)
}

// cursorBytePos computes the byte offset of the cursor in the full textarea value.
func (m *QueryModel) cursorBytePos() int {
	val := m.textarea.Value()
	line := m.textarea.Line()
	col := 0
	if info := m.textarea.LineInfo(); info.Width > 0 {
		col = info.ColumnOffset
	}

	pos := 0
	currentLine := 0
	for i, ch := range val {
		if currentLine == line {
			if col <= 0 {
				return i
			}
			col--
		}
		if ch == '\n' {
			currentLine++
		}
		pos = i + 1
	}
	// Cursor at the end
	if currentLine == line {
		return pos
	}
	return len(val)
}

// setCursorToBytePos moves the textarea cursor to the given byte offset.
func (m *QueryModel) setCursorToBytePos(pos int) {
	val := m.textarea.Value()
	targetLine := 0
	targetCol := 0
	for i := 0; i < pos && i < len(val); i++ {
		if val[i] == '\n' {
			targetLine++
			targetCol = 0
		} else {
			targetCol++
		}
	}
	// SetValue may leave the cursor anywhere (e.g. at the end); normalize to
	// line 0 first, then descend to the target line.
	for i := m.textarea.Line(); i > 0; i-- {
		m.textarea.CursorUp()
	}
	for i := 0; i < targetLine; i++ {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursor(targetCol)
}

// autoResize adjusts textarea height to fit content so no lines are hidden.
func (m *QueryModel) autoResize() {
	lines := m.textarea.LineCount()
	if lines < 3 {
		lines = 3
	}
	if lines > 8 {
		lines = 8
	}
	m.textarea.SetHeight(lines)
}

// CommitHistory saves the pending query to history (call on successful execution).
func (m *QueryModel) CommitHistory() {
	if m.pendingQuery == "" {
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != m.pendingQuery {
		m.history = append(m.history, m.pendingQuery)
		config.SaveHistory(m.history)
	}
	m.histIdx = len(m.history)
	m.pendingQuery = ""
}

// DiscardPending clears the pending query (call on failed execution).
func (m *QueryModel) DiscardPending() {
	m.pendingQuery = ""
}

func (m QueryModel) executeQuery(cypher string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		result, err := client.Run(context.Background(), cypher, nil)
		if err != nil {
			return queryErrorMsg{err: err}
		}
		return queryResultMsg{result: result}
	}
}

func (m QueryModel) View() string {
	style := queryBorderBlurredStyle
	if m.textarea.Focused() {
		style = queryBorderFocusedStyle
	}
	queryBox := style.Width(m.width - 2).Render(m.textarea.View())
	if m.autocomplete.Visible() {
		return queryBox + "\n" + m.autocomplete.View()
	}
	return queryBox
}

func copyToClipboard(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return
	}
	cmd.Stdin = strings.NewReader(text)
	cmd.Run()
}
