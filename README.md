# Agent Relay

A lightweight MCP server that enables real-time communication between AI coding agents. Built for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) multi-agent workflows.

When you run multiple Claude Code instances on different parts of a project (backend, frontend, infra...), they can't talk to each other. Agent Relay fixes that with a shared message bus exposed as MCP tools.

## How it works

```
┌──────────────┐     MCP/HTTP      ┌──────────────┐     MCP/HTTP      ┌──────────────┐
│  Claude Code  │◄────────────────►│              │◄────────────────►│  Claude Code  │
│   (backend)   │                  │ Agent Relay  │                  │  (frontend)   │
└──────────────┘                   │   :8090      │                  └──────────────┘
                                   │              │
┌──────────────┐     MCP/HTTP      │   SQLite     │
│  Claude Code  │◄────────────────►│   + Push     │
│    (infra)    │                  └──────────────┘
└──────────────┘
```

Each agent connects via Streamable HTTP with a unique identity (`?agent=backend`). Messages are persisted in SQLite with threading, broadcast, and read tracking.

## Features

- **6 MCP tools** - register, send, inbox, thread, list agents, mark read
- **Threading** - reply chains with recursive thread reconstruction
- **Broadcast** - send to `*` to reach all connected agents
- **Push notifications** - real-time MCP notifications when messages arrive
- **Persistent** - SQLite with WAL mode, survives restarts
- **Zero config** - single binary, no external dependencies
- **Claude Code skill** - `/relay` command for human-friendly usage

## Quick Start

### Build & Run

```bash
git clone https://github.com/Synergix-lab/go-realy.git
cd go-realy
go build -o agent-relay .
./agent-relay
```

Listens on `:8090` by default (`PORT` env var to override).

### Configure your project

Add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "agent-relay": {
      "type": "http",
      "url": "http://localhost:8090/mcp?agent=backend"
    }
  }
}
```

Change `?agent=backend` to whatever identifies this instance (`frontend`, `infra`, `mobile`, etc.).

### Install the Claude Code skill (optional)

```bash
cp skill/relay.md ~/.claude/commands/relay.md
```

Then use `/relay` in any Claude Code session:
- `/relay` - check inbox
- `/relay send frontend Need the UserProfile fields` - send a message
- `/relay agents` - list connected agents
- `/relay thread <id>` - view conversation thread

## MCP Tools

| Tool | Description |
|------|-------------|
| `register_agent` | Announce your identity (name, role, description) |
| `send_message` | Send to a specific agent or `*` for broadcast |
| `get_inbox` | Fetch messages (with unread filter) |
| `get_thread` | Reconstruct full conversation from any message ID |
| `list_agents` | See all registered agents and last activity |
| `mark_read` | Mark messages as read by ID |

### Message types

`question` | `response` | `notification` | `code-snippet` | `task`

### Example flow

```
backend  → send_message(to="frontend", type="question", subject="API contract",
             content="What fields do you need for UserProfile?")

frontend → get_inbox() → sees the question
frontend → send_message(to="backend", type="response", reply_to="<msg-id>",
             content="Need: name, email, avatar_url, role")

backend  → get_thread("<msg-id>") → sees full conversation
```

## Architecture

```
main.go                  # Entry point: DB init + HTTP server
internal/
  db/
    db.go                # SQLite init, WAL mode, migrations
    agents.go            # Agent CRUD
    messages.go          # Message CRUD, inbox queries, threading
  relay/
    relay.go             # Wires MCP server + tools
    tools.go             # 6 MCP tool definitions
    handlers.go          # Tool handler implementations
    context.go           # Agent identity from ?agent= query param
    notify.go            # Push notifications to connected sessions
  models/
    agent.go             # Agent struct
    message.go           # Message struct
skill/
  relay.md               # Claude Code /relay command
```

**Dependencies**: [mcp-go](https://github.com/mark3labs/mcp-go), [go-sqlite3](https://github.com/mattn/go-sqlite3), [google/uuid](https://github.com/google/uuid)

## Run as a service (macOS)

```bash
go build -o agent-relay .
cp agent-relay /usr/local/bin/
cp com.agent-relay.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.agent-relay.plist
```

The relay will start automatically on login and restart if it crashes.

## Data

SQLite database at `~/.agent-relay/relay.db`. WAL mode with single writer connection for reliability. Delete the file to reset all state.

## License

MIT
