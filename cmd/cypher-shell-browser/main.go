package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeremyadams/cypher-shell-browser/internal/config"
	"github.com/jeremyadams/cypher-shell-browser/internal/model"
)

func main() {
	cfg := config.Load()

	app := model.NewApp(cfg)

	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
