package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeremyadams/cypher-shell-browser/internal/config"
)

var (
	connectTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86")).
				MarginBottom(1)

	connectLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Width(12)

	connectFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86"))
)

// Indices into ConnectModel.inputs.
const (
	fieldCredFile = iota
	fieldURI
	fieldUsername
	fieldPassword
	fieldDatabase
	numConnectFields
)

type ConnectModel struct {
	inputs  []textinput.Model
	focused int
	width   int
	height  int

	// candidates are the credentials files found in the working directory,
	// most recently used first.  Ctrl+O walks them in order.
	candidates []string
	nextCand   int
	// filled is the path this model last put into the cred file field, used to
	// tell "still showing what I offered" from "the user typed a path".
	filled string
}

type connectSubmitMsg struct {
	uri      string
	username string
	password string
	database string
}

// credsLoadedMsg reports the outcome of loading a credentials file from the
// connect screen. err is nil on success.  When the load came from cycling
// through discovered files, index/total say which one it was (1-based).
type credsLoadedMsg struct {
	path  string
	err   error
	index int
	total int
}

var errNoCredFile = errors.New("no credentials file given (.env or Neo4j-…-Created-….txt)")

func NewConnectModel(cfg *config.Config) ConnectModel {
	uri, username, password, database := cfg.URI, cfg.Username, cfg.Password, cfg.Database

	credInput := textinput.New()
	credInput.Width = 40
	if cfg.CredFile != "" {
		credInput.SetValue(cfg.CredFile)
	}

	uriInput := textinput.New()
	uriInput.Placeholder = "neo4j://localhost:7687"
	uriInput.Width = 40
	if uri != "" {
		uriInput.SetValue(uri)
	}

	userInput := textinput.New()
	userInput.Placeholder = "neo4j"
	userInput.Width = 40
	if username != "" {
		userInput.SetValue(username)
	}

	passInput := textinput.New()
	passInput.Placeholder = "password"
	passInput.EchoMode = textinput.EchoPassword
	passInput.Width = 40
	if password != "" {
		passInput.SetValue(password)
	}

	dbInput := textinput.New()
	dbInput.Placeholder = "auto-detect from URI"
	dbInput.Width = 40
	if database != "" {
		dbInput.SetValue(database)
	}

	m := ConnectModel{
		inputs:     make([]textinput.Model, numConnectFields),
		focused:    fieldURI,
		candidates: config.DiscoverCredentialsFiles("."),
	}
	m.inputs[fieldCredFile] = credInput
	m.inputs[fieldURI] = uriInput
	m.inputs[fieldUsername] = userInput
	m.inputs[fieldPassword] = passInput
	m.inputs[fieldDatabase] = dbInput

	if len(m.candidates) > 0 {
		m.inputs[fieldCredFile].Placeholder = filepath.Base(m.candidates[0])
	} else {
		m.inputs[fieldCredFile].Placeholder = "path to .env or Neo4j-…-Created-….txt"
	}

	m.inputs[m.focused].Focus()
	return m
}

func (m *ConnectModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ConnectModel) Update(msg tea.Msg) (ConnectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+o":
			return m.loadCreds()
		case "tab", "down":
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.updateFocus()
		case "enter":
			// Enter on the cred file field loads it rather than advancing.
			if m.focused == fieldCredFile && (m.inputs[fieldCredFile].Value() != "" || len(m.candidates) > 0) {
				return m.loadCreds()
			}
			if m.focused < len(m.inputs)-1 {
				m.focused++
				return m, m.updateFocus()
			}
			return m, m.submit()
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m ConnectModel) submit() tea.Cmd {
	uri := m.inputs[fieldURI].Value()
	username := m.inputs[fieldUsername].Value()
	database := m.inputs[fieldDatabase].Value()
	if database == "" {
		database = defaultDatabase(uri, username)
	}
	password := m.inputs[fieldPassword].Value()
	return func() tea.Msg {
		return connectSubmitMsg{
			uri:      uri,
			username: username,
			password: password,
			database: database,
		}
	}
}

// loadCreds reads the credentials file named in the cred file field and fills
// in whatever settings it provides.  When the field is empty — or still holds
// the path this model offered last — it instead steps to the next discovered
// file, so repeated Ctrl+O cycles through everything in the directory.
func (m ConnectModel) loadCreds() (ConnectModel, tea.Cmd) {
	typed := strings.TrimSpace(m.inputs[fieldCredFile].Value())

	path := typed
	var index, total int
	if typed == "" || typed == m.filled {
		if len(m.candidates) == 0 {
			return m, func() tea.Msg {
				return credsLoadedMsg{err: errNoCredFile}
			}
		}
		path = m.candidates[m.nextCand]
		index, total = m.nextCand+1, len(m.candidates)
		m.nextCand = (m.nextCand + 1) % len(m.candidates)
	}

	// Show which file was tried even when it turns out to be unusable, so a
	// failure names its source and the next Ctrl+O moves on.
	m.inputs[fieldCredFile].SetValue(path)
	m.filled = path

	creds, err := config.LoadCredentialsFile(path)
	if err != nil {
		return m, func() tea.Msg {
			return credsLoadedMsg{path: path, err: err, index: index, total: total}
		}
	}

	if creds.URI != "" {
		m.inputs[fieldURI].SetValue(creds.URI)
	}
	if creds.Username != "" {
		m.inputs[fieldUsername].SetValue(creds.Username)
	}
	if creds.Password != "" {
		m.inputs[fieldPassword].SetValue(creds.Password)
	}
	if creds.Database != "" {
		m.inputs[fieldDatabase].SetValue(creds.Database)
	}

	// Park on the last field so Enter connects straight away.
	m.focused = fieldDatabase
	focusCmd := m.updateFocus()

	return m, tea.Batch(focusCmd, func() tea.Msg {
		return credsLoadedMsg{path: path, index: index, total: total}
	})
}

func (m *ConnectModel) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focused {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

// defaultDatabase infers the database name from the URI and username.
// For Aura instances (neo4j+s:// or neo4j+ssc://), the database ID is the
// hostname prefix (which matches the username). For local/bolt/neo4j
// connections, defaults to "neo4j".
func defaultDatabase(uri, username string) string {
	lower := strings.ToLower(uri)
	if strings.HasPrefix(lower, "neo4j+s://") || strings.HasPrefix(lower, "neo4j+ssc://") {
		if username != "" {
			return username
		}
	}
	return "neo4j"
}

func (m ConnectModel) View() string {
	labels := []string{"Cred file:", "URI:", "Username:", "Password:", "Database:"}

	// Show the inferred database name as ghost text when the field is empty
	if m.inputs[fieldDatabase].Value() == "" {
		inferred := defaultDatabase(m.inputs[fieldURI].Value(), m.inputs[fieldUsername].Value())
		m.inputs[fieldDatabase].Placeholder = "auto-detect (" + inferred + ")"
	}

	rows := make([]string, len(m.inputs))
	for i, input := range m.inputs {
		label := connectLabelStyle.Render(labels[i])
		rows[i] = lipgloss.JoinHorizontal(lipgloss.Center, label, input.View())
	}

	form := lipgloss.JoinVertical(lipgloss.Left, rows...)

	title := connectTitleStyle.Render("Connect to Neo4j")

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	parts := []string{title, form}

	switch n := len(m.candidates); {
	case n == 1:
		parts = append(parts, dimStyle.MarginTop(1).Render("1 credentials file here • Ctrl+O to load it"))
	case n > 1:
		parts = append(parts, dimStyle.MarginTop(1).Render(
			fmt.Sprintf("%d credentials files here • Ctrl+O to cycle, most recently used first", n)))
	}

	hintTop := 1
	if len(m.candidates) > 0 {
		hintTop = 0
	}
	parts = append(parts, dimStyle.MarginTop(hintTop).
		Render("Enter to connect • Tab to next field"))

	block := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}
