
A terminal UI for Neo4j — write Cypher queries and explore results also written in Cypher without leaving your shell.

[![Release](https://github.com/jpadams/cypher-shell-browser/actions/workflows/release.yml/badge.svg)](https://github.com/jpadams/cypher-shell-browser/actions/workflows/release.yml)

![demo-new](https://github.com/user-attachments/assets/46e54934-9988-4f81-b53f-3a9734c75e5b)


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
- Load credentials from a `.env` file or the `Neo4j-…-Created-….txt` Aura hands you
- Write and run Cypher queries with syntax highlighting and autocomplete
- True Cypher output with hotkey for MERGE/CREATE for subgraph portability
- Copy Cypher to the clipboard — the highlighted row (`Ctrl+Y`) or all results (`Ctrl+A`)
- Toggle results verbosity (`v`) to show or hide node properties for scanning shape
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

## Credentials

Connection settings can come from flags, environment variables, or a credentials file:

```sh
cypher-shell-browser --uri neo4j+s://xxxxxxxx.databases.neo4j.io --username neo4j --password secret

NEO4J_URI=... NEO4J_USERNAME=... NEO4J_PASSWORD=... cypher-shell-browser

cypher-shell-browser --env-file .env
cypher-shell-browser --env-file ~/Downloads/Neo4j-a1b2c3d4-Created-2026-07-28.txt
```

A credentials file is any `KEY=VALUE` file — a `.env` you wrote yourself or the
`Neo4j-<instance>-Created-<date>.txt` that Aura downloads when you create an
instance. Both use the same layout, so no conversion is needed:

```
NEO4J_URI=neo4j+s://a1b2c3d4.databases.neo4j.io
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=your-generated-password
NEO4J_DATABASE=neo4j
```

Comments, blank lines, a leading `export `, and quoted values are all fine, and
everything but `NEO4J_*` is ignored. `NEO4J_USER`, `NEO4J_URL`, and
`NEO4J_BOLT_URL` are accepted as aliases. `NEO4J_DATABASE` is optional — it is
inferred from the URI when absent.

You can also load a file from the connect screen: type or paste a path into the
**Cred file** field and press Enter, or press **Ctrl+O** to load the file the
current directory offers (an `Neo4j-…-Created-….txt` if present, otherwise
`.env`), whose name is shown as the field's placeholder.

Precedence, highest first: command-line flags → credentials file → environment
variables → defaults. So `--env-file creds.txt --database movies` uses the URI,
username, and password from the file with your chosen database.

## License

MIT
