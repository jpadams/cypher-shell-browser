package model

import (
	"errors"
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

	// discovered is a credentials file found in the working directory,
	// offered as the placeholder for the cred file field.
	discovered string
}

type connectSubmitMsg struct {
	uri      string
	username string
	password string
	database string
}

// credsLoadedMsg reports the outcome of loading a credentials file from the
// connect screen. err is nil on success.
type credsLoadedMsg struct {
	path string
	err  error
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
		discovered: config.DiscoverCredentialsFile("."),
	}
	m.inputs[fieldCredFile] = credInput
	m.inputs[fieldURI] = uriInput
	m.inputs[fieldUsername] = userInput
	m.inputs[fieldPassword] = passInput
	m.inputs[fieldDatabase] = dbInput

	if m.discovered != "" {
		m.inputs[fieldCredFile].Placeholder = filepath.Base(m.discovered)
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
			if m.focused == fieldCredFile && (m.inputs[fieldCredFile].Value() != "" || m.discovered != "") {
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

// loadCreds reads the credentials file named in the cred file field — falling
// back to the file discovered in the working directory — and fills in whatever
// settings it provides.
func (m ConnectModel) loadCreds() (ConnectModel, tea.Cmd) {
	path := strings.TrimSpace(m.inputs[fieldCredFile].Value())
	if path == "" {
		path = m.discovered
	}
	if path == "" {
		return m, func() tea.Msg {
			return credsLoadedMsg{err: errNoCredFile}
		}
	}

	creds, err := config.LoadCredentialsFile(path)
	if err != nil {
		return m, func() tea.Msg {
			return credsLoadedMsg{path: path, err: err}
		}
	}

	m.inputs[fieldCredFile].SetValue(path)
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
		return credsLoadedMsg{path: path}
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
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1).
		Render("Enter to connect • Tab to next field • Ctrl+O to load credentials file")

	block := lipgloss.JoinVertical(lipgloss.Left, title, form, hint)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}
