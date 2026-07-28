package model

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeremyadams/cypher-shell-browser/internal/config"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// collect flattens a command (including tea.Batch trees) into its messages.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// findMsg returns the first message of type T produced by cmd.
func findMsg[T any](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	msgs := collect(cmd)
	for _, msg := range msgs {
		if typed, ok := msg.(T); ok {
			return typed
		}
	}
	var zero T
	t.Fatalf("expected a %T among %v", zero, msgs)
	return zero
}

func writeAuraFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "Neo4j-a1b2c3d4-Created-2026-07-28.txt")
	content := "NEO4J_URI=neo4j+s://a1b2c3d4.databases.neo4j.io\n" +
		"NEO4J_USERNAME=a1b2c3d4\n" +
		"NEO4J_PASSWORD=file-secret\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestConnectLoadsCredsFromTypedPath(t *testing.T) {
	t.Chdir(t.TempDir())
	path := writeAuraFile(t, ".")

	m := NewConnectModel(&config.Config{URI: "neo4j://localhost:7687", Username: "neo4j"})
	m.focused = fieldCredFile
	m.inputs[fieldCredFile].SetValue(path)

	m, cmd := m.Update(key("enter"))

	loaded := findMsg[credsLoadedMsg](t, cmd)
	if loaded.err != nil {
		t.Fatalf("load error: %v", loaded.err)
	}
	if got := m.inputs[fieldURI].Value(); got != "neo4j+s://a1b2c3d4.databases.neo4j.io" {
		t.Errorf("URI = %q", got)
	}
	if got := m.inputs[fieldUsername].Value(); got != "a1b2c3d4" {
		t.Errorf("Username = %q", got)
	}
	if got := m.inputs[fieldPassword].Value(); got != "file-secret" {
		t.Errorf("Password = %q", got)
	}
	if m.focused != fieldDatabase {
		t.Errorf("focused = %d, want the last field so Enter connects", m.focused)
	}
}

func TestConnectCtrlOLoadsDiscoveredFile(t *testing.T) {
	t.Chdir(t.TempDir())
	writeAuraFile(t, ".")

	m := NewConnectModel(&config.Config{URI: "neo4j://localhost:7687", Username: "neo4j"})
	if m.discovered == "" {
		t.Fatal("expected the Aura file to be discovered")
	}

	m, _ = m.Update(key("ctrl+o"))
	if got := m.inputs[fieldPassword].Value(); got != "file-secret" {
		t.Errorf("Password = %q, want the discovered file to be loaded", got)
	}

	// Enter on the last field submits, with the database inferred from the URI.
	_, cmd := m.Update(key("enter"))
	submit := findMsg[connectSubmitMsg](t, cmd)
	if submit.database != "a1b2c3d4" {
		t.Errorf("database = %q, want it inferred from the Aura URI", submit.database)
	}
	if submit.password != "file-secret" {
		t.Errorf("password = %q", submit.password)
	}
}

func TestConnectCtrlOWithNothingToLoad(t *testing.T) {
	t.Chdir(t.TempDir())

	m := NewConnectModel(&config.Config{})
	_, cmd := m.Update(key("ctrl+o"))

	loaded := findMsg[credsLoadedMsg](t, cmd)
	if loaded.err == nil {
		t.Error("expected an error when there is no credentials file")
	}
}

func TestConnectEnterOnEmptyCredFieldAdvances(t *testing.T) {
	t.Chdir(t.TempDir())

	m := NewConnectModel(&config.Config{})
	m.focused = fieldCredFile

	m, _ = m.Update(key("enter"))
	if m.focused != fieldURI {
		t.Errorf("focused = %d, want %d", m.focused, fieldURI)
	}
}
