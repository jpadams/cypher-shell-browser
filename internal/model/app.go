package model

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeremyadams/cypher-shell-browser/internal/config"
	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

type appState int

const (
	stateConnect appState = iota
	stateQuery
)

type App struct {
	cfg       *config.Config
	client    *n4j.Client
	state     appState
	connect   ConnectModel
	query     QueryModel
	table     TableViewModel
	graph     GraphViewModel
	statusbar StatusBar
	help      HelpModel
	width     int
	height    int
	hasResult  bool
	showHelp   bool
	queryFocus bool // true when query textarea has focus
}

type connectedMsg struct {
	client *n4j.Client
	uri    string
}

type connErrorMsg struct {
	err error
}

type schemaLoadedMsg struct {
	labels   []string
	relTypes []string
}

func NewApp(cfg *config.Config) App {
	app := App{
		cfg:       cfg,
		state:     stateConnect,
		connect:   NewConnectModel(cfg.URI, cfg.Username, cfg.Password, cfg.Database),
		table:     NewTableViewModel(),
		graph:     NewGraphViewModel(),
		statusbar: NewStatusBar(),
		help:      NewHelpModel(),
	}
	return app
}

func (a App) Init() tea.Cmd {
	// If password is provided via flag/env, try auto-connect
	if a.cfg.Password != "" {
		database := a.cfg.Database
		if database == "" {
			database = defaultDatabase(a.cfg.URI, a.cfg.Username)
		}
		return func() tea.Msg {
			return connectSubmitMsg{
				uri:      a.cfg.URI,
				username: a.cfg.Username,
				password: a.cfg.Password,
				database: database,
			}
		}
	}
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.connect.SetSize(msg.Width, msg.Height-1)
		a.statusbar.SetSize(msg.Width)
		a.help.SetSize(msg.Width, msg.Height)
		if a.state == stateQuery {
			a.query.SetWidth(msg.Width)
			a.updateResultSize()
		}
		return a, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "ctrl+c":
			if a.client != nil {
				a.client.Close(context.Background())
			}
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		}

		// Clear errors on any navigation/dismiss key
		if a.statusbar.isError {
			switch msg.String() {
			case "up", "enter", " ", "esc":
				a.statusbar.ClearError()
			}
		}

		if a.showHelp {
			var cmd tea.Cmd
			a.help, cmd = a.help.Update(msg)
			return a, cmd
		}

	case connectSubmitMsg:
		a.statusbar.SetMessage("Connecting...")
		return a, connectCmd(msg.uri, msg.username, msg.password, msg.database)

	case connectedMsg:
		a.client = msg.client
		a.state = stateQuery
		a.queryFocus = true
		a.query = NewQueryModel(a.client)
		a.query.SetWidth(a.width)
		resultH := a.height - 8
		a.table.SetSize(a.width, resultH)
		a.graph.SetSize(a.width, resultH)
		a.statusbar.SetConnected(msg.uri)
		a.statusbar.SetHints(queryInputHints(false))
		return a, tea.Batch(a.query.Focus(), uriTickCmd(), loadSchemaCmd(a.client))

	case schemaLoadedMsg:
		a.query.autocomplete.SetSchema(msg.labels, msg.relTypes)
		return a, nil

	case connErrorMsg:
		cmd := a.statusbar.SetError(fmt.Sprintf("Connection failed: %s", msg.err))
		return a, cmd

	case queryResultMsg:
		a.statusbar.SetLoading(false)
		pendingQuery := a.query.pendingQuery
		a.query.CommitHistory()
		if msg.result.Summary != "" {
			a.statusbar.SetMessage(msg.result.Summary)
		}
		// If no rows and no graph data, clear previous results and keep query focused
		hasData := len(msg.result.Rows) > 0 || len(msg.result.Nodes) > 0
		if !hasData {
			a.hasResult = false
			a.statusbar.SetHints(queryInputHints(false))
			if queryContainsSchemaChange(pendingQuery) {
				return a, loadSchemaCmd(a.client)
			}
			return a, nil
		}
		a.hasResult = true
		a.updateResultSize()
		a.table.SetResult(msg.result)
		a.graph.SetResult(msg.result)
		a.statusbar.SetHints(a.resultHints())
		a.query.Blur()
		a.queryFocus = false
		a.setResultActive(true)
		// Reload schema if the query contained CREATE or MERGE
		if queryContainsSchemaChange(pendingQuery) {
			return a, loadSchemaCmd(a.client)
		}
		return a, nil

	case queryErrorMsg:
		a.statusbar.SetLoading(false)
		a.query.DiscardPending()
		cmd := a.statusbar.SetError(fmt.Sprintf("Query error: %s", msg.err))
		return a, cmd

	case queryCopiedMsg:
		a.statusbar.SetMessage("Copied to clipboard")
		return a, nil

	case uriTickMsg:
		cmd := a.statusbar.Update(msg)
		return a, cmd

	case errScrollTickMsg:
		cmd := a.statusbar.Update(msg)
		return a, cmd
	}

	// Route to sub-models
	switch a.state {
	case stateConnect:
		if a.showHelp {
			return a, nil
		}
		var cmd tea.Cmd
		a.connect, cmd = a.connect.Update(msg)
		return a, cmd

	case stateQuery:
		if a.showHelp {
			return a, nil
		}
		return a.updateQueryState(msg)
	}

	return a, nil
}

func (a *App) resultHints() []StatusHint {
	if a.graph.HasGraph() {
		if a.graph.detailFocus {
			return []StatusHint{
				{Key: "Ctrl+Y", Desc: "copy", Active: true},
				{Key: "←/h", Desc: "lines", Active: true},
				{Key: "Esc", Desc: "query", Active: true},
				{Key: "?", Desc: "help", Active: true},
			}
		}
		return []StatusHint{
			{Key: "Space", Desc: "detail", Active: true},
			{Key: "Ctrl+Y", Desc: "copy", Active: true},
			{Key: "m/c", Desc: "MERGE/CREATE", Active: true},
			{Key: "Esc", Desc: "query", Active: true},
			{Key: "?", Desc: "help", Active: true},
		}
	}
	return []StatusHint{
		{Key: "Esc", Desc: "query", Active: true},
		{Key: "?", Desc: "help", Active: true},
	}
}

func queryInputHints(hasResults bool) []StatusHint {
	hints := []StatusHint{
		{Key: "Ctrl+E", Desc: "execute", Active: true},
	}
	if hasResults {
		hints = append(hints, StatusHint{Key: "Esc", Desc: "results", Active: true})
	}
	hints = append(hints,
		StatusHint{Key: "?", Desc: "help", Active: true},
		StatusHint{Key: "Ctrl+C", Desc: "quit", Active: true},
	)
	return hints
}

func (a App) updateQueryState(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if a.queryFocus && a.hasResult {
				// Return focus to results
				a.queryFocus = false
				a.query.Blur()
				a.setResultActive(true)
				a.statusbar.SetHints(a.resultHints())
				return a, nil
			}
			a.queryFocus = true
			a.setResultActive(false)
			a.graph.ResetPrefix()
			a.statusbar.SetHints(queryInputHints(a.hasResult))
			return a, a.query.Focus()
		case "ctrl+e":
			if a.queryFocus {
				a.statusbar.SetLoading(true)
			}
		}
	}

	if a.queryFocus {
		// Only update query input when it has focus
		var cmd tea.Cmd
		a.query, cmd = a.query.Update(msg)
		a.updateResultSize() // textarea may have auto-resized
		return a, cmd
	}

	// Update the active result view when query is blurred
	var resultCmd tea.Cmd
	if a.hasResult {
		if a.graph.HasGraph() {
			a.graph, resultCmd = a.graph.Update(msg)
			a.statusbar.SetHints(a.resultHints()) // detail focus may have changed
		} else {
			a.table, resultCmd = a.table.Update(msg)
		}
	}

	return a, resultCmd
}

func (a App) View() string {
	if a.showHelp {
		return a.help.View()
	}

	switch a.state {
	case stateConnect:
		return lipgloss.JoinVertical(lipgloss.Left,
			a.connect.View(),
			a.statusbar.View(),
		)

	case stateQuery:
		var topParts []string
		topParts = append(topParts, a.query.View())
		if a.hasResult {
			if a.graph.HasGraph() {
				topParts = append(topParts, a.graph.View())
			} else {
				topParts = append(topParts, a.table.View())
			}
		}
		topContent := lipgloss.JoinVertical(lipgloss.Left, topParts...)
		fillerH := a.height - lipgloss.Height(topContent) - 1
		if fillerH > 0 {
			return lipgloss.JoinVertical(lipgloss.Left,
				topContent,
				strings.Repeat("\n", fillerH-1),
				a.statusbar.View(),
			)
		}
		return lipgloss.JoinVertical(lipgloss.Left, topContent, a.statusbar.View())
	}

	return ""
}

func (a *App) updateResultSize() {
	resultH := a.height - a.query.Height() - 1
	if resultH < 4 {
		resultH = 4
	}
	a.table.SetSize(a.width, resultH)
	a.graph.SetSize(a.width, resultH)
}

func (a *App) setResultActive(active bool) {
	if a.graph.HasGraph() {
		a.graph.active = active
		a.table.active = false
	} else {
		a.table.active = active
		a.graph.active = false
	}
}

func loadSchemaCmd(client *n4j.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		labels := fetchStringList(client, ctx, "CALL db.labels()")
		relTypes := fetchStringList(client, ctx, "CALL db.relationshipTypes()")
		return schemaLoadedMsg{labels: labels, relTypes: relTypes}
	}
}

func fetchStringList(client *n4j.Client, ctx context.Context, cypher string) []string {
	result, err := client.Run(ctx, cypher, nil)
	if err != nil {
		return nil
	}
	var items []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			items = append(items, row[0])
		}
	}
	return items
}

func queryContainsSchemaChange(query string) bool {
	upper := strings.ToUpper(query)
	return strings.Contains(upper, "CREATE") || strings.Contains(upper, "MERGE")
}

func connectCmd(uri, username, password, database string) tea.Cmd {
	return func() tea.Msg {
		client, err := n4j.New(uri, username, password, database)
		if err != nil {
			return connErrorMsg{err: err}
		}
		if err := client.Ping(context.Background()); err != nil {
			return connErrorMsg{err: err}
		}
		return connectedMsg{client: client, uri: uri}
	}
}
