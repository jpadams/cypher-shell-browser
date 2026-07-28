package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Credentials holds the connection settings read from a credentials file.
// A field is empty when the file did not mention it.
type Credentials struct {
	URI      string
	Username string
	Password string
	Database string
}

// Empty reports whether the file yielded nothing usable.
func (c Credentials) Empty() bool {
	return c.URI == "" && c.Username == "" && c.Password == "" && c.Database == ""
}

// LoadCredentialsFile reads a dotenv-style credentials file. Both plain `.env`
// files and the `Neo4j-<instance>-Created-<date>.txt` file that Aura hands out
// on instance creation use the same `KEY=VALUE` layout, so one parser covers
// both.  Comments (`#`), blank lines, a leading `export `, and surrounding
// quotes are all tolerated.
func LoadCredentialsFile(path string) (Credentials, error) {
	var creds Credentials

	expanded, err := ExpandPath(path)
	if err != nil {
		return creds, err
	}

	f, err := os.Open(expanded)
	if err != nil {
		return creds, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := parseCredLine(scanner.Text())
		if !ok {
			continue
		}
		switch key {
		case "NEO4J_URI", "NEO4J_URL", "NEO4J_BOLT_URL":
			creds.URI = value
		case "NEO4J_USERNAME", "NEO4J_USER":
			creds.Username = value
		case "NEO4J_PASSWORD":
			creds.Password = value
		case "NEO4J_DATABASE", "NEO4J_DB":
			creds.Database = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Credentials{}, err
	}

	if creds.Empty() {
		return creds, fmt.Errorf("no NEO4J_* settings found in %s", path)
	}
	return creds, nil
}

// parseCredLine splits a single `KEY=VALUE` line, returning ok=false for
// comments, blank lines, and anything without an `=`.
func parseCredLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}

	key = strings.ToUpper(strings.TrimSpace(line[:eq]))
	value = strings.TrimSpace(line[eq+1:])
	value = unquote(value)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

// unquote strips one layer of matching single or double quotes, and drops any
// trailing ` # comment` from an unquoted value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// DiscoverCredentialsFiles lists candidate credentials files in dir, most
// promising first: Aura's downloaded `Neo4j-*-Created-*.txt` files (newest
// name first), then `.env`, then any other `*.env`.
func DiscoverCredentialsFiles(dir string) []string {
	if dir == "" {
		dir = "."
	}

	aura, _ := filepath.Glob(filepath.Join(dir, "Neo4j-*-Created-*.txt"))
	sort.Sort(sort.Reverse(sort.StringSlice(aura)))

	// `*.env` also matches a bare `.env`, so dedupe while preserving priority.
	others, _ := filepath.Glob(filepath.Join(dir, "*.env"))
	sort.Strings(others)

	candidates := append([]string{}, aura...)
	candidates = append(candidates, filepath.Join(dir, ".env"))
	candidates = append(candidates, others...)

	var found []string
	seen := make(map[string]bool)
	for _, path := range candidates {
		if seen[path] {
			continue
		}
		seen[path] = true
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			continue
		}
		found = append(found, path)
	}
	return found
}

// DiscoverCredentialsFile returns the single best candidate in dir, or "".
func DiscoverCredentialsFile(dir string) string {
	if files := DiscoverCredentialsFiles(dir); len(files) > 0 {
		return files[0]
	}
	return ""
}

// ExpandPath resolves a leading `~` to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}
