package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	URI      string
	Username string
	Password string
	Database string

	// CredFile is the credentials file that was loaded, if any.
	CredFile string
}

func Load() *Config {
	cfg := &Config{}

	var credFile string
	flag.StringVar(&credFile, "env-file", os.Getenv("NEO4J_ENV_FILE"),
		"Path to a credentials file (.env or Neo4j-<instance>-Created-<date>.txt)")
	flag.StringVar(&cfg.URI, "uri", envOrDefault("NEO4J_URI", "neo4j://localhost:7687"), "Neo4j connection URI")
	flag.StringVar(&cfg.Username, "username", envOrDefault("NEO4J_USERNAME", "neo4j"), "Neo4j username")
	flag.StringVar(&cfg.Password, "password", envOrDefault("NEO4J_PASSWORD", ""), "Neo4j password")
	flag.StringVar(&cfg.Database, "database", envOrDefault("NEO4J_DATABASE", ""), "Neo4j database name (auto-detected from URI if not set)")
	flag.Parse()

	if credFile != "" {
		creds, err := LoadCredentialsFile(credFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading credentials file: %v\n", err)
			os.Exit(1)
		}
		cfg.CredFile = credFile
		// Explicit command-line flags win over the file; the file wins over
		// environment variables and built-in defaults.
		cfg.Apply(creds, explicitFlags())
	}

	return cfg
}

// Apply copies non-empty credential fields onto the config, skipping any field
// named in override (fields the user set explicitly on the command line).
func (c *Config) Apply(creds Credentials, override map[string]bool) {
	if creds.URI != "" && !override["uri"] {
		c.URI = creds.URI
	}
	if creds.Username != "" && !override["username"] {
		c.Username = creds.Username
	}
	if creds.Password != "" && !override["password"] {
		c.Password = creds.Password
	}
	if creds.Database != "" && !override["database"] {
		c.Database = creds.Database
	}
}

// explicitFlags reports which flags were actually passed on the command line.
func explicitFlags() map[string]bool {
	set := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	return set
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
