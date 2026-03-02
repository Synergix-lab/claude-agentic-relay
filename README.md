<div align="center">

# Claude Agentic Relay

**Make your Claude Code agents talk to each other.**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-Streamable_HTTP-8A2BE2?style=flat-square)](https://modelcontextprotocol.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

Running Claude Code on your backend *and* your frontend at the same time?<br>
Right now they're blind to each other. This fixes that.

[Quick Start](#quick-start) · [How It Works](#how-it-works) · [MCP Tools](#mcp-tools) · [Claude Code Skill](#claude-code-skill)

</div>

---

## The Problem

You're building a full-stack app. You have Claude Code running on your API, another on your frontend, maybe one on infra. They each make decisions the others should know about — API contracts change, types get renamed, endpoints move. But they can't communicate. So you end up being the messenger, copy-pasting context between terminals.

## The Fix

Claude Agentic Relay is a single-binary MCP server that gives your agents a shared message bus. Any Claude Code instance can send messages, ask questions, and get answers from any other — in real time.

```
 ┌─────────────┐                        ┌─────────────┐
 │ Claude Code  │───── MCP/HTTP ────────│ Claude Code  │
 │  (backend)   │          │            │  (frontend)  │
 └─────────────┘           │            └─────────────┘
                    ┌──────┴──────┐
                    │    Relay    │
                    │   :8090    │
                    │   SQLite   │
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │ Claude Code  │
                    │   (infra)   │
                    └─────────────┘
```

## Quick Start

### 1. Build

```bash
git clone https://github.com/Synergix-lab/claude-agentic-relay.git
cd claude-agentic-relay
go build -o agent-relay .
./agent-relay
```

### 2. Connect your agents

Add this to `.mcp.json` in each of your project roots:

**Backend project:**
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

**Frontend project:**
```json
{
  "mcpServers": {
    "agent-relay": {
      "type": "http",
      "url": "http://localhost:8090/mcp?agent=frontend"
    }
  }
}
```

The `?agent=` parameter is how each instance identifies itself. Use any name you want.

### 3. They can now talk

Your backend agent can ask your frontend agent a question:

```
backend  →  "What fields do you need for UserProfile?"
frontend →  "name, email, avatar_url, role"
backend  →  builds the endpoint with the right contract
```

No copy-paste. No context switching. They just figure it out.

## How It Works

The relay is a [Model Context Protocol](https://modelcontextprotocol.io) server using Streamable HTTP transport. Each Claude Code instance connects as a client with a unique agent name extracted from the URL query string.

Messages are persisted in **SQLite** (WAL mode) so nothing is lost if the relay restarts. Threads are tracked via `reply_to` references and reconstructed with recursive queries. When a message arrives for a connected agent, a **push notification** is sent over the MCP session so the agent knows to check its inbox.

**Zero external dependencies** — one binary, one SQLite file at `~/.agent-relay/relay.db`.

## MCP Tools

| Tool | What it does |
|------|-------------|
| `register_agent` | Announce yourself — name, role, what you're working on |
| `send_message` | Send to a specific agent, or `*` to broadcast to everyone |
| `get_inbox` | Pull your messages (unread filter, limit) |
| `get_thread` | Get a full conversation thread from any message in it |
| `list_agents` | See who's connected and when they were last active |
| `mark_read` | Mark messages as read |

### Message Types

| Type | Use case |
|------|----------|
| `question` | Ask another agent something |
| `response` | Reply to a question |
| `notification` | FYI — "I just changed the auth middleware" |
| `code-snippet` | Share a piece of code |
| `task` | Assign work to another agent |

### Example Conversation

```
backend  → send_message(to="frontend", type="question",
             subject="API contract",
             content="What fields do you need for UserProfile?")

frontend → get_inbox() → sees the question

frontend → send_message(to="backend", type="response",
             reply_to="<msg-id>",
             content="Need: name, email, avatar_url, role")

backend  → get_thread("<msg-id>") → full conversation in order
```

## Claude Code Skill

Install the `/relay` command for a human-friendly interface:

```bash
cp skill/relay.md ~/.claude/commands/relay.md
```

Then in any Claude Code session:

| Command | Action |
|---------|--------|
| `/relay` | Check inbox, show unread messages |
| `/relay send frontend <message>` | Send a message |
| `/relay agents` | List who's connected |
| `/relay thread <id>` | View a conversation |
| `/relay read` | Mark everything as read |

## Run as a Service (macOS)

Keep the relay running permanently:

```bash
go build -o agent-relay .
cp agent-relay /usr/local/bin/
cp com.agent-relay.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.agent-relay.plist
```

Starts on login, restarts on crash. Logs at `/tmp/agent-relay.log`.

## Project Structure

```
main.go                     # Entry point, graceful shutdown
internal/
  db/                       # SQLite layer (WAL, migrations, queries)
  relay/                    # MCP server, tools, handlers, push notifications
  models/                   # Agent & Message structs
skill/
  relay.md                  # Claude Code /relay command
com.agent-relay.plist       # macOS launchd config
```

Built with [mcp-go](https://github.com/mark3labs/mcp-go) · [go-sqlite3](https://github.com/mattn/go-sqlite3) · [google/uuid](https://github.com/google/uuid)

## Contributing

PRs welcome. If you're adding a feature, open an issue first so we can discuss the approach.

## License

[MIT](LICENSE)
