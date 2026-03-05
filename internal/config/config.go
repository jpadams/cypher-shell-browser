package config

import (
	"flag"
	"os"
)

type Config struct {
	URI      string
	Username string
	Password string
	Database string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.URI, "uri", envOrDefault("NEO4J_URI", "neo4j://localhost:7687"), "Neo4j connection URI")
	flag.StringVar(&cfg.Username, "username", envOrDefault("NEO4J_USERNAME", "neo4j"), "Neo4j username")
	flag.StringVar(&cfg.Password, "password", envOrDefault("NEO4J_PASSWORD", ""), "Neo4j password")
	flag.StringVar(&cfg.Database, "database", envOrDefault("NEO4J_DATABASE", ""), "Neo4j database name (auto-detected from URI if not set)")
	flag.Parse()

	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
