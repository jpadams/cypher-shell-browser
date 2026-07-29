
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

You can also load a file from the connect screen. Type or paste a path into the
**Cred file** field and press Enter, or press **Ctrl+O** to work through the
credentials files in the current directory — Aura `.txt` downloads and `.env`
files alike, **most recently used first**, wrapping around at the end. Each
press names the file it filled the fields from in the status bar, along with its
position (`2 of 3`), and leaves the path in the **Cred file** field so you can
always see which credentials are loaded. A file that turns out to be unusable
reports why and the next press moves on.

Recency means the later of a file's access and modification time, so the
instance you just downloaded or the `.env` you just edited comes up first.
Neither kind is preferred over the other — which is current is a matter of what
you touched last, not what it is named. On `noatime`/`relatime` mounts, and on
Windows, reads may not be recorded and the modification time is used.

Precedence, highest first: command-line flags → credentials file → environment
variables → defaults. So `--env-file creds.txt --database movies` uses the URI,
username, and password from the file with your chosen database.

## License

MIT
