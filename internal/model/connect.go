package model

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type ConnectModel struct {
	inputs  []textinput.Model
	focused int
	width   int
	height  int
}

type connectSubmitMsg struct {
	uri      string
	username string
	password string
	database string
}

func NewConnectModel(uri, username, password, database string) ConnectModel {
	uriInput := textinput.New()
	uriInput.Placeholder = "neo4j://localhost:7687"
	uriInput.Focus()
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

	return ConnectModel{
		inputs:  []textinput.Model{uriInput, userInput, passInput, dbInput},
		focused: 0,
	}
}

func (m *ConnectModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ConnectModel) Update(msg tea.Msg) (ConnectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.updateFocus()
		case "enter":
			if m.focused < len(m.inputs)-1 {
				m.focused++
				return m, m.updateFocus()
			}
			uri := m.inputs[0].Value()
			username := m.inputs[1].Value()
			database := m.inputs[3].Value()
			if database == "" {
				database = defaultDatabase(uri, username)
			}
			return m, func() tea.Msg {
				return connectSubmitMsg{
					uri:      uri,
					username: username,
					password: m.inputs[2].Value(),
					database: database,
				}
			}
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
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
	labels := []string{"URI:", "Username:", "Password:", "Database:"}

	rows := make([]string, len(m.inputs))
	for i, input := range m.inputs {
		label := connectLabelStyle.Render(labels[i])
		rows[i] = lipgloss.JoinHorizontal(lipgloss.Center, label, input.View())
	}

	form := lipgloss.JoinVertical(lipgloss.Left, rows...)

	title := connectTitleStyle.Render("Connect to Neo4j")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1).Render("Enter to connect • Tab to next field")

	block := lipgloss.JoinVertical(lipgloss.Left, title, form, hint)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}
