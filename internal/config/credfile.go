package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// credFilePatterns are the globs searched for credentials files. `*.env` also
// matches a bare `.env`, but the explicit pattern is kept so the common case
// still works if a platform's Glob ever skips dotfiles.
var credFilePatterns = []string{
	"Neo4j-*-Created-*.txt", // downloaded from Aura on instance creation
	"*.env",
	".env",
}

// DiscoverCredentialsFiles lists candidate credentials files in dir, most
// recently used first, so the file you last downloaded or edited comes up
// before older ones.  Aura `.txt` downloads and `.env` files compete purely on
// recency — neither kind is inherently preferred, since which one is current
// is a matter of what you touched last, not what it is named.  Ties break on
// path so the order is stable.
func DiscoverCredentialsFiles(dir string) []string {
	if dir == "" {
		dir = "."
	}

	type candidate struct {
		path string
		used time.Time
	}

	var found []candidate
	seen := make(map[string]bool)
	for _, pattern := range credFilePatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, path := range matches {
			if seen[path] {
				continue
			}
			seen[path] = true
			fi, err := os.Stat(path)
			if err != nil || fi.IsDir() {
				continue
			}
			found = append(found, candidate{path: path, used: lastUsed(fi)})
		}
	}

	sort.SliceStable(found, func(i, j int) bool {
		if !found[i].used.Equal(found[j].used) {
			return found[i].used.After(found[j].used)
		}
		return found[i].path < found[j].path
	})

	paths := make([]string, len(found))
	for i, c := range found {
		paths[i] = c.path
	}
	return paths
}

// lastUsed reports when a file was last read or written, whichever is later.
// Access time is used where the platform exposes it; mounts with `noatime` or
// `relatime` may not record reads, and Windows is not supported at all, so the
// modification time is the floor.
func lastUsed(fi os.FileInfo) time.Time {
	modified := fi.ModTime()
	if accessed, ok := accessTime(fi); ok && accessed.After(modified) {
		return accessed
	}
	return modified
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
