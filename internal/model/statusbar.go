package model

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	uriFixedLen  = 18 // always-visible prefix length
	uriCycleLen  = 5  // cycling suffix window size
	uriThreshold = uriFixedLen + uriCycleLen // 23; only cycle if URI > this
)

var (
	statusBarBg = lipgloss.Color("236")

	statusBarStyle = lipgloss.NewStyle().
			Background(statusBarBg).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	statusErrorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("196")).
				Foreground(lipgloss.Color("255")).
				Padding(0, 1).
				Bold(true)

	statusConnectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("22")).
				Foreground(lipgloss.Color("255")).
				Padding(0, 1)

	statusHintActiveStyle = lipgloss.NewStyle().
				Background(statusBarBg).
				Foreground(lipgloss.Color("252"))

	statusHintDimStyle = lipgloss.NewStyle().
				Background(statusBarBg).
				Foreground(lipgloss.Color("243"))

	statusSepStyle = lipgloss.NewStyle().
			Background(statusBarBg).
			Foreground(lipgloss.Color("240"))
)

// StatusHint is a single key binding hint shown in the status bar.
type StatusHint struct {
	Key    string
	Desc   string
	Active bool // bright when true, dim when false
}

type uriTickMsg struct{}
type errScrollTickMsg struct{}

type uriPhase int

const (
	uriPhaseEllipsis uriPhase = iota // show "...."
	uriPhaseLast4                    // show last 4 chars of tail
	uriPhaseScroll                   // scroll from end toward start
	uriPhaseFirst4                   // show first 4 chars of tail (chars 19-22)
)

type StatusBar struct {
	width     int
	connected bool
	uri       string
	message   string
	isError   bool
	hints     []StatusHint
	loading   bool

	// URI cycling state
	phase     uriPhase
	tailOff   int // current offset into tail for the 4-char window

	// Error scroll state
	errOffset int  // current scroll offset into error message
	errTicking bool // whether the error scroll ticker is active
}

func NewStatusBar() StatusBar {
	return StatusBar{
		hints: []StatusHint{{Key: "Ctrl+C", Desc: "quit", Active: false}},
	}
}

func (s *StatusBar) SetSize(width int) {
	s.width = width
}

func (s *StatusBar) SetConnected(uri string) {
	s.connected = true
	s.uri = uri
	s.isError = false
	s.message = "Connected"
	s.phase = uriPhaseEllipsis
	s.tailOff = 0
}

func (s *StatusBar) SetError(msg string) tea.Cmd {
	s.isError = true
	// Flatten to single line so the status bar never grows vertically
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", "")
	s.message = msg
	s.errOffset = 0
	s.errTicking = true
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return errScrollTickMsg{}
	})
}

func (s *StatusBar) ClearError() {
	if s.isError {
		s.isError = false
		s.message = ""
		s.errOffset = 0
		s.errTicking = false
	}
}

func (s *StatusBar) SetMessage(msg string) {
	s.isError = false
	s.message = msg
}

func (s *StatusBar) SetHints(hints []StatusHint) {
	s.hints = hints
}

func (s *StatusBar) SetLoading(loading bool) {
	s.loading = loading
}

func (s *StatusBar) uriTailLen() int {
	return len(s.uri) - uriFixedLen
}

// Update handles tick messages for URI cycling and error scrolling.
func (s *StatusBar) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(errScrollTickMsg); ok {
		return s.updateErrScroll()
	}
	if _, ok := msg.(uriTickMsg); !ok {
		return nil
	}
	if !s.connected || len(s.uri) <= uriThreshold {
		return nil
	}

	tailLen := s.uriTailLen()

	switch s.phase {
	case uriPhaseEllipsis:
		// Transition: show last 4 chars of tail
		s.phase = uriPhaseLast4
		s.tailOff = tailLen - uriCycleLen
		return tickAfter(5 * time.Second)

	case uriPhaseLast4:
		// Transition: start scrolling toward the beginning
		s.phase = uriPhaseScroll
		s.tailOff = tailLen - uriCycleLen - 1
		return tickAfter(300 * time.Millisecond)

	case uriPhaseScroll:
		if s.tailOff <= 0 {
			// Reached the beginning — show first 4 of tail
			s.phase = uriPhaseFirst4
			s.tailOff = 0
			return tickAfter(3 * time.Second)
		}
		s.tailOff--
		return tickAfter(300 * time.Millisecond)

	case uriPhaseFirst4:
		// Back to ellipsis
		s.phase = uriPhaseEllipsis
		s.tailOff = 0
		return tickAfter(10 * time.Second)
	}

	return nil
}

func tickAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return uriTickMsg{}
	})
}

// uriTickCmd starts the first tick (initial ellipsis hold).
func uriTickCmd() tea.Cmd {
	return tickAfter(10 * time.Second)
}

func (s *StatusBar) updateErrScroll() tea.Cmd {
	if !s.isError || !s.errTicking {
		return nil
	}
	maxW := s.errMaxWidth()
	if len(s.message) <= maxW {
		// Fits already, no scrolling needed
		return nil
	}
	// Advance scroll offset
	s.errOffset++
	if s.errOffset > len(s.message)-maxW {
		// Reached the end, stop scrolling
		s.errTicking = false
		return nil
	}
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return errScrollTickMsg{}
	})
}

func (s StatusBar) errMaxWidth() int {
	maxW := s.width - 4 // padding/border
	if maxW < 10 {
		maxW = 10
	}
	return maxW
}

func (s StatusBar) displayError() string {
	maxW := s.errMaxWidth()
	msg := s.message
	if len(msg) <= maxW {
		return msg
	}
	end := s.errOffset + maxW
	if end > len(msg) {
		end = len(msg)
	}
	return msg[s.errOffset:end]
}

func (s StatusBar) displayURI() string {
	if len(s.uri) <= uriThreshold {
		return s.uri
	}

	prefix := s.uri[:uriFixedLen]

	if s.phase == uriPhaseEllipsis {
		return prefix + "....."
	}

	tail := s.uri[uriFixedLen:]
	end := s.tailOff + uriCycleLen
	if end > len(tail) {
		end = len(tail)
	}
	return prefix + tail[s.tailOff:end]
}

func (s StatusBar) View() string {
	sep := statusSepStyle.Render(" │ ")

	// Build left section
	left := ""
	if s.isError {
		left = statusErrorStyle.Render(s.displayError())
	} else if s.loading {
		left = statusBarStyle.Render("⏳ Running query...")
	} else if s.connected {
		left = statusConnectedStyle.Render("● " + s.displayURI())
		if s.message != "" && s.message != "Connected" {
			left += sep + statusBarStyle.Render(s.message)
		}
	} else if s.message != "" {
		left = statusBarStyle.Render(s.message)
	}

	// Build hints section
	hintsStr := ""
	for i, h := range s.hints {
		style := statusHintDimStyle
		if h.Active {
			style = statusHintActiveStyle
		}
		if i > 0 {
			hintsStr += sep
		}
		hintsStr += style.Render(h.Key + ": " + h.Desc)
	}

	// Join left and hints with separator, fill remaining space
	content := left
	if hintsStr != "" {
		content += sep + hintsStr
	}

	contentW := lipgloss.Width(content)
	gap := s.width - contentW
	if gap < 0 {
		gap = 0
	}

	filler := lipgloss.NewStyle().
		Background(statusBarBg).
		Render(repeat(' ', gap))

	return content + filler
}

func repeat(ch rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}
