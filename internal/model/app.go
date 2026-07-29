package model

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	cfg        *config.Config
	client     *n4j.Client
	state      appState
	connect    ConnectModel
	query      QueryModel
	table      TableViewModel
	graph      GraphViewModel
	statusbar  StatusBar
	help       HelpModel
	width      int
	height     int
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
	labels       []string
	relTypes     []string
	labelProps   map[string][]string
	relTypeProps map[string][]string
	procedures   []string
}

func NewApp(cfg *config.Config) App {
	app := App{
		cfg:       cfg,
		state:     stateConnect,
		connect:   NewConnectModel(cfg),
		table:     NewTableViewModel(),
		graph:     NewGraphViewModel(),
		statusbar: NewStatusBar(),
		help:      NewHelpModel(),
	}
	if cfg.CredFile != "" {
		app.statusbar.SetMessage("Loaded credentials from " + filepath.Base(cfg.CredFile))
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
		a.setResultSize(a.height - 8)
		a.statusbar.SetConnected(msg.uri)
		a.statusbar.SetHints(queryInputHints(false))
		return a, tea.Batch(a.query.Focus(), uriTickCmd(), loadSchemaCmd(a.client), schemaTickCmd())

	case schemaLoadedMsg:
		a.query.autocomplete.SetSchema(msg.labels, msg.relTypes, msg.labelProps, msg.relTypeProps, msg.procedures)
		return a, nil

	case connErrorMsg:
		cmd := a.statusbar.SetError(fmt.Sprintf("Connection failed: %s", msg.err))
		return a, cmd

	case credsLoadedMsg:
		if msg.err != nil {
			return a, a.statusbar.SetError(fmt.Sprintf("Credentials file: %s", msg.err))
		}
		a.statusbar.SetMessage("Filled from " + credsSourceLabel(msg))
		return a, nil

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
		text := msg.text
		if text == "" {
			text = "Copied to clipboard"
		}
		a.statusbar.SetMessage(text)
		return a, nil

	case uriTickMsg:
		cmd := a.statusbar.Update(msg)
		return a, cmd

	case errScrollTickMsg:
		cmd := a.statusbar.Update(msg)
		return a, cmd

	case schemaTickMsg:
		if a.queryFocus && a.client != nil {
			return a, tea.Batch(loadSchemaCmd(a.client), schemaTickCmd())
		}
		return a, nil
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

// credsSourceLabel names the file the connection fields were filled from, plus
// its position when there is more than one candidate to cycle through.
func credsSourceLabel(msg credsLoadedMsg) string {
	name := filepath.Base(msg.path)
	if msg.total > 1 {
		return fmt.Sprintf("%s (%d of %d) — Ctrl+O for next, Enter to connect", name, msg.index, msg.total)
	}
	return name + " — Enter to connect"
}

func (a *App) resultHints() []StatusHint {
	if a.graph.HasGraph() {
		if a.graph.detailFocus {
			return []StatusHint{
				{Key: "v", Desc: "internals", Active: true},
				{Key: "Ctrl+Y", Desc: "copy", Active: true},
				{Key: "←/h", Desc: "lines", Active: true},
				{Key: "Esc", Desc: "query", Active: true},
				{Key: "?", Desc: "help", Active: true},
			}
		}
		hints := []StatusHint{
			{Key: "Space", Desc: "detail", Active: true},
		}
		if a.graph.showDetail {
			hints = append(hints, StatusHint{Key: "v", Desc: "internals", Active: true})
		} else {
			hints = append(hints, StatusHint{Key: "v", Desc: "verbosity", Active: true})
		}
		if a.graph.tree {
			hints = append(hints, StatusHint{Key: "t", Desc: "rows", Active: true})
			if n := len(a.graph.variantRows()); n > 1 {
				hints = append(hints, StatusHint{
					Key:    "Tab",
					Desc:   fmt.Sprintf("variant %d/%d", a.graph.variant+1, n),
					Active: true,
				})
			}
		} else {
			hints = append(hints, StatusHint{Key: "t", Desc: "tree", Active: true})
		}
		hints = append(hints,
			StatusHint{Key: "Ctrl+Y", Desc: "copy row", Active: true},
			StatusHint{Key: "Ctrl+A", Desc: "copy all", Active: true},
		)
		if !a.graph.tree {
			hints = append(hints, StatusHint{Key: "m/c", Desc: "MERGE/CREATE", Active: true})
		}
		hints = append(hints,
			StatusHint{Key: "Esc", Desc: "query", Active: true},
			StatusHint{Key: "?", Desc: "help", Active: true},
		)
		return hints
	}
	return []StatusHint{
		{Key: "Esc", Desc: "query", Active: true},
		{Key: "?", Desc: "help", Active: true},
	}
}

func queryInputHints(hasResults bool) []StatusHint {
	hints := []StatusHint{
		{Key: "Ctrl+R", Desc: "run query", Active: true},
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
			return a, tea.Batch(a.query.Focus(), loadSchemaCmd(a.client), schemaTickCmd())
		case "ctrl+r":
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
	a.setResultSize(a.height - a.query.Height() - 1)
}

// setResultSize sizes both result views, never handing them a height too small
// to render a row.
func (a *App) setResultSize(h int) {
	if h < minTableHeight {
		h = minTableHeight
	}
	a.table.SetSize(a.width, h)
	a.graph.SetSize(a.width, h)
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

type schemaTickMsg struct{}

func schemaTickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return schemaTickMsg{}
	})
}

func loadSchemaCmd(client *n4j.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		labels := fetchStringList(client, ctx, "CALL db.labels()")
		relTypes := fetchStringList(client, ctx, "CALL db.relationshipTypes()")
		labelProps := fetchEntityProps(client, ctx,
			"CALL db.schema.nodeTypeProperties() YIELD nodeLabels, propertyName "+
				"UNWIND nodeLabels AS label RETURN DISTINCT label, propertyName")
		relTypeProps := fetchEntityProps(client, ctx,
			"CALL db.schema.relTypeProperties() YIELD relType, propertyName "+
				"RETURN DISTINCT relType, propertyName")
		procedures := fetchStringList(client, ctx, "CALL dbms.procedures() YIELD name RETURN name ORDER BY name")
		if len(procedures) == 0 {
			// Neo4j 5.x alternative
			procedures = fetchStringList(client, ctx, "SHOW PROCEDURES YIELD name RETURN name ORDER BY name")
		}
		return schemaLoadedMsg{labels: labels, relTypes: relTypes, labelProps: labelProps, relTypeProps: relTypeProps, procedures: procedures}
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

// fetchEntityProps runs a query that returns (entity, propertyName) rows and
// builds a map of entity → []propertyName.  The entity column may contain
// Neo4j's internal relType format (e.g. `:"KNOWS"`), which is sanitised to
// a plain identifier.
func fetchEntityProps(client *n4j.Client, ctx context.Context, cypher string) map[string][]string {
	result, err := client.Run(ctx, cypher, nil)
	if err != nil {
		return nil
	}
	props := make(map[string][]string)
	seen := make(map[string]map[string]bool)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		entity := sanitizeEntityName(row[0])
		prop := row[1]
		if entity == "" || prop == "" {
			continue
		}
		if seen[entity] == nil {
			seen[entity] = make(map[string]bool)
		}
		if !seen[entity][prop] {
			seen[entity][prop] = true
			props[entity] = append(props[entity], prop)
		}
	}
	return props
}

// sanitizeEntityName extracts the identifier from Neo4j's internal type
// representations such as `:"KNOWS"` → "KNOWS".
func sanitizeEntityName(s string) string {
	start := 0
	for start < len(s) && !isIdentChar(s[start]) {
		start++
	}
	end := start
	for end < len(s) && isIdentChar(s[end]) {
		end++
	}
	return s[start:end]
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
