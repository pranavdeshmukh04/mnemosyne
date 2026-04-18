# Mnemosyne

A fully local, per-project memory system for Claude Code CLI conversations.

Mnemosyne gives each of your projects a persistent knowledge graph that captures the entities you work with (files, functions, decisions, bugs, concepts, people) and the conversations that touched them. New Claude Code sessions automatically load the relevant context, and Claude actively reads and writes the graph during conversations via MCP tools. A local web viewer lets you explore the graph interactively.

Everything stays on disk in your project directory. No network calls, no cloud storage, no third-party services.

## Why

Working on confidential projects means you can't rely on hosted memory solutions. But losing context between Claude Code sessions means re-explaining the same background, decisions, and constraints every time. Mnemosyne fixes that with a graph database that lives next to your code.

## How It Works

1. Run `/mnemosyne` in any Claude Code session to initialize memory for that project.
2. A per-project SQLite database is created at `.mnemosyne/memory.db`, and a `.mcp.json` is written so Claude Code auto-connects to the Mnemosyne MCP server.
3. During conversations, Claude uses MCP tools to:
   - Load project context at session start
   - Remember new entities and link them as they come up
   - Recall relevant prior knowledge when useful
   - Save the transcript at session end
4. Run `mnemosyne view` to explore the graph in a local browser.

## Architecture

Three components share one SQLite file per project:

- **MCP server** (`mnemosyne mcp`) — stdio server launched by Claude Code
- **CLI** (`mnemosyne`) — admin commands: `init`, `view`, `stats`, `export`, `wipe`, `install-command`
- **Web viewer** — local-only HTTP server with an interactive Cytoscape.js graph

All three live in one Go binary.

## Data Model

A hybrid graph where both conversations and entities are first-class nodes:

- **Nodes:** `conversation`, `file`, `function`, `decision`, `bug`, `concept`, `person`, `other`
- **Edges:** `touched`, `decided`, `caused`, `fixed`, `references`, `depends_on`, `related_to`

See [`docs/design.md`](docs/design.md) for the full schema and tool specifications.

## Installation

```bash
go install github.com/<you>/mnemosyne/cmd/mnemosyne@latest
mnemosyne install-command
```

The second command installs the `/mnemosyne` slash command at `~/.claude/commands/mnemosyne.md`, so any Claude Code session can initialize memory with a single command.

## Usage

Inside any project:

```bash
# In Claude Code:
/mnemosyne           # initializes .mnemosyne/memory.db and .mcp.json
                     # restart Claude Code afterward

# In a shell:
mnemosyne view       # open the graph in your browser
mnemosyne stats      # node/edge counts and DB integrity check
mnemosyne export     # dump the graph to JSON
mnemosyne wipe       # delete this project's memory (with confirmation)
```

## Project Layout

```
mnemosyne/
├── cmd/mnemosyne/        # binary entry point (cobra root)
├── internal/
│   ├── cli/              # admin subcommands
│   ├── db/               # schema, migrations, connection
│   ├── graph/            # node/edge CRUD and queries
│   ├── mcpserver/        # MCP tool handlers
│   └── viewer/           # local HTTP server + embedded UI
├── docs/
│   └── design.md         # full design spec
└── tests/
```

## Tech Stack

- Go 1.22+
- `github.com/mark3labs/mcp-go` — MCP protocol
- `modernc.org/sqlite` — pure-Go SQLite (no CGO)
- `github.com/spf13/cobra` — CLI
- Cytoscape.js — graph visualization (embedded via `//go:embed`)

## Confidentiality

- All data lives in your project's `.mnemosyne/` directory.
- The viewer binds to `127.0.0.1` only.
- No external network calls for storage, retrieval, or extraction.
- `mnemosyne wipe` deletes a project's memory completely.

## Status

Early development. See [`docs/design.md`](docs/design.md) for the current design.
