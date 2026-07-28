package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoadCredentialsFileAura(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "Neo4j-a1b2c3d4-Created-2026-07-28.txt", `# Wait 60 seconds before connecting using these details, or login to https://console.neo4j.io to validate the Aura Instance is available
NEO4J_URI=neo4j+s://a1b2c3d4.databases.neo4j.io
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=sUp3r-s3cret_pa55
AURA_INSTANCEID=a1b2c3d4
AURA_INSTANCENAME=Instance01
`)

	creds, err := LoadCredentialsFile(path)
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if creds.URI != "neo4j+s://a1b2c3d4.databases.neo4j.io" {
		t.Errorf("URI = %q", creds.URI)
	}
	if creds.Username != "neo4j" {
		t.Errorf("Username = %q", creds.Username)
	}
	if creds.Password != "sUp3r-s3cret_pa55" {
		t.Errorf("Password = %q", creds.Password)
	}
	if creds.Database != "" {
		t.Errorf("Database = %q, want empty", creds.Database)
	}
}

func TestLoadCredentialsFileDotenv(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".env", `
# local dev
export NEO4J_URI="bolt://localhost:7687"
NEO4J_USER='neo4j'
NEO4J_PASSWORD=letmein   # inline comment
neo4j_database=movies
OTHER_THING=ignored
`)

	creds, err := LoadCredentialsFile(path)
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	want := Credentials{
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
		Password: "letmein",
		Database: "movies",
	}
	if creds != want {
		t.Errorf("got %+v, want %+v", creds, want)
	}
}

func TestLoadCredentialsFilePreservesHashInQuotedValue(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".env", "NEO4J_PASSWORD=\"pa55 # word\"\n")

	creds, err := LoadCredentialsFile(path)
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if creds.Password != "pa55 # word" {
		t.Errorf("Password = %q", creds.Password)
	}
}

func TestLoadCredentialsFileErrors(t *testing.T) {
	dir := t.TempDir()

	if _, err := LoadCredentialsFile(filepath.Join(dir, "missing.env")); err == nil {
		t.Error("expected error for missing file")
	}

	empty := writeFile(t, dir, "nothing.env", "# just a comment\nFOO=bar\n")
	if _, err := LoadCredentialsFile(empty); err == nil {
		t.Error("expected error for file with no NEO4J_* settings")
	}
}

func TestDiscoverCredentialsFilePrefersAura(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "NEO4J_PASSWORD=x\n")
	writeFile(t, dir, "Neo4j-aaaa1111-Created-2026-01-01.txt", "NEO4J_PASSWORD=old\n")
	newest := writeFile(t, dir, "Neo4j-bbbb2222-Created-2026-07-28.txt", "NEO4J_PASSWORD=new\n")

	if got := DiscoverCredentialsFile(dir); got != newest {
		t.Errorf("DiscoverCredentialsFile = %q, want %q", got, newest)
	}
}

func TestDiscoverCredentialsFileFallsBackToDotenv(t *testing.T) {
	dir := t.TempDir()
	dotenv := writeFile(t, dir, ".env", "NEO4J_PASSWORD=x\n")
	writeFile(t, dir, "staging.env", "NEO4J_PASSWORD=y\n")

	if got := DiscoverCredentialsFile(dir); got != dotenv {
		t.Errorf("DiscoverCredentialsFile = %q, want %q", got, dotenv)
	}

	files := DiscoverCredentialsFiles(dir)
	if len(files) != 2 || files[1] != filepath.Join(dir, "staging.env") {
		t.Errorf("DiscoverCredentialsFiles = %v", files)
	}
}

func TestDiscoverCredentialsFileNone(t *testing.T) {
	if got := DiscoverCredentialsFile(t.TempDir()); got != "" {
		t.Errorf("DiscoverCredentialsFile = %q, want empty", got)
	}
}

func TestApplyRespectsExplicitFlags(t *testing.T) {
	cfg := &Config{URI: "neo4j://localhost:7687", Username: "neo4j", Password: "", Database: ""}
	creds := Credentials{
		URI:      "neo4j+s://abc.databases.neo4j.io",
		Username: "abc",
		Password: "secret",
		Database: "abc",
	}

	cfg.Apply(creds, map[string]bool{"uri": true})

	if cfg.URI != "neo4j://localhost:7687" {
		t.Errorf("URI = %q, want the explicit flag value to win", cfg.URI)
	}
	if cfg.Username != "abc" || cfg.Password != "secret" || cfg.Database != "abc" {
		t.Errorf("unset fields should come from the file: %+v", cfg)
	}
}
