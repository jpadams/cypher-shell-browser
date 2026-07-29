package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

// collectDeadline bounds how long a single command may take to produce its
// message. Focusing an input returns bubbles' cursor-blink command, which is a
// tea.Tick that would otherwise block for its whole blink interval — several
// hundred milliseconds per keypress. The messages these tests assert on are
// plain closures that return immediately.
const collectDeadline = 50 * time.Millisecond

// collect flattens a command (including tea.Batch trees) into its messages,
// skipping any command that does not deliver within collectDeadline.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			return []tea.Msg{msg}
		}
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	case <-time.After(collectDeadline):
		return nil // a timer command; carries nothing these tests need
	}
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
	if len(m.candidates) == 0 {
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

// Repeated Ctrl+O walks every discovered file in recency order and wraps
// around, reporting which one filled the fields each time.
func TestConnectCtrlOCyclesThroughCandidates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	files := []struct {
		name     string
		password string
		age      time.Duration
	}{
		{"Neo4j-aaaa1111-Created-2026-07-28.txt", "newest", 0},
		{".env", "middle", 24 * time.Hour},
		{"Neo4j-zzzz9999-Created-2020-01-01.txt", "oldest", 72 * time.Hour},
	}
	for _, f := range files {
		body := "NEO4J_URI=neo4j+s://x.databases.neo4j.io\nNEO4J_PASSWORD=" + f.password + "\n"
		if err := os.WriteFile(f.name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		when := base.Add(-f.age)
		if err := os.Chtimes(f.name, when, when); err != nil {
			t.Fatal(err)
		}
	}

	m := NewConnectModel(&config.Config{})
	if len(m.candidates) != 3 {
		t.Fatalf("discovered %d files, want 3: %v", len(m.candidates), m.candidates)
	}

	// Two full laps, to prove it wraps rather than sticking on the last file.
	for lap := 0; lap < 2; lap++ {
		for i, f := range files {
			var cmd tea.Cmd
			m, cmd = m.Update(key("ctrl+o"))
			loaded := findMsg[credsLoadedMsg](t, cmd)

			if loaded.err != nil {
				t.Fatalf("lap %d step %d: %v", lap, i, loaded.err)
			}
			if filepath.Base(loaded.path) != f.name {
				t.Errorf("lap %d step %d: loaded %s, want %s", lap, i, filepath.Base(loaded.path), f.name)
			}
			if loaded.index != i+1 || loaded.total != 3 {
				t.Errorf("lap %d step %d: reported %d of %d, want %d of 3", lap, i, loaded.index, loaded.total, i+1)
			}
			if got := m.inputs[fieldPassword].Value(); got != f.password {
				t.Errorf("lap %d step %d: password = %q, want %q", lap, i, got, f.password)
			}
			if got := m.inputs[fieldCredFile].Value(); filepath.Base(got) != f.name {
				t.Errorf("lap %d step %d: cred field = %q, want it to name %s", lap, i, got, f.name)
			}
		}
	}
}

// A path the user typed is honoured instead of resuming the cycle.
func TestConnectTypedPathBeatsCycling(t *testing.T) {
	t.Chdir(t.TempDir())
	writeAuraFile(t, ".")
	if err := os.WriteFile("chosen.env", []byte("NEO4J_PASSWORD=typed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := NewConnectModel(&config.Config{})
	m.inputs[fieldCredFile].SetValue("chosen.env")

	m, cmd := m.Update(key("ctrl+o"))
	loaded := findMsg[credsLoadedMsg](t, cmd)
	if loaded.err != nil {
		t.Fatalf("load error: %v", loaded.err)
	}
	if loaded.path != "chosen.env" {
		t.Errorf("loaded %q, want the typed path", loaded.path)
	}
	if loaded.total != 0 {
		t.Errorf("total = %d, want 0 for a typed path (no cycle position)", loaded.total)
	}
	if got := m.inputs[fieldPassword].Value(); got != "typed" {
		t.Errorf("password = %q", got)
	}
}

// An unusable file names itself in the error and does not stall the cycle.
func TestConnectCyclePastUnusableFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if err := os.WriteFile("broken.env", []byte("# nothing useful here\nFOO=bar\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("good.env", []byte("NEO4J_PASSWORD=works\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes("broken.env", base, base)
	os.Chtimes("good.env", base.Add(-time.Hour), base.Add(-time.Hour))

	m := NewConnectModel(&config.Config{})

	m, cmd := m.Update(key("ctrl+o"))
	first := findMsg[credsLoadedMsg](t, cmd)
	if first.err == nil {
		t.Fatal("expected the file with no NEO4J_* settings to fail")
	}
	if filepath.Base(first.path) != "broken.env" {
		t.Errorf("failure names %q, want broken.env", first.path)
	}

	m, cmd = m.Update(key("ctrl+o"))
	second := findMsg[credsLoadedMsg](t, cmd)
	if second.err != nil {
		t.Fatalf("cycle stalled on the bad file: %v", second.err)
	}
	if got := m.inputs[fieldPassword].Value(); got != "works" {
		t.Errorf("password = %q, want the next file to load", got)
	}
}
