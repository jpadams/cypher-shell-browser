
A terminal UI for Neo4j — write Cypher queries and explore results also written in Cypher without leaving your shell.

[![Release](https://github.com/jpadams/cypher-shell-browser/actions/workflows/release.yml/badge.svg)](https://github.com/jpadams/cypher-shell-browser/actions/workflows/release.yml)

![demo](https://github.com/user-attachments/assets/c7a583aa-f0ce-40f3-be47-748becc689d6)

```
╭──────────────────────────────────────────────────────────────╮
│                                                              │
│    (Alice)──[:KNOWS]──▶(Bob)                                 │
│       │                                                      │
│   [:WORKS_AT]         MATCH (a:Person)-[:KNOWS]->(b:Person)  │
│       │               WHERE a.name = "Alice"                 │
│       ▼               RETURN a,b                             │
│   (Acme Corp)                                                │
│                                                              │
│            cypher-shell-browser                              │
│            Neo4j in your terminal                            │
╰──────────────────────────────────────────────────────────────╯
```

## Features

- Connect to any Neo4j instance (local or remote) with bolt/neo4j URI schemes
- Write and execute Cypher queries with syntax highlighting and autocomplete
- True Cypher output with hotkey for MERGE/CREATE for subgraph portability
- Navigate nodes and relationships with the keyboard
- Query history persisted across sessions
- Status bar with context-aware key binding hints
- Ephemeral error display — errors clear automatically on next query

## Build & Install

Requires Go 1.25+.

```sh
go build ./cmd/cypher-shell-browser
```

Or install directly:

```sh
go install github.com/jpadams/cypher-shell-browser/cmd/cypher-shell-browser@latest
```

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/jpadams/cypher-shell-browser/releases) page.

## License

MIT
