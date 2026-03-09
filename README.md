<div align="center">

<img src="docs/assets/wraith-banner.webp" alt="wrai.th" width="700">

# wrai.th

**Multi-agent orchestration as a management game.**

Your AI agents are robots. Your projects are planets. You run the galaxy.

<br>

[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-Protocol-8A2BE2?style=for-the-badge)](https://modelcontextprotocol.io)
[![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?style=for-the-badge&logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![License](https://img.shields.io/badge/AGPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Discord](https://img.shields.io/badge/Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/QPq7qfbEk8)

[Quick Start](#-quick-start) · [How It Works](#-how-it-works) · [Agents](#-agents--hierarchy) · [Messaging](#-messaging--conversations) · [Memory](#-memory--knowledge) · [Goals & Tasks](#-goal-cascade--task-execution) · [Heartbeat](#-passive-vs-proactive--heartbeat-loops) · [MCP Tools](#-mcp-tools)

<br>

<img src="docs/screenshots/galaxy-view.png" alt="Galaxy View — projects orbit as pixel-art planets" width="800">

*One binary. One SQLite file. 58 MCP tools. Zero required config.*

**100% local by default. Optional API key for team/server deployments. No cloud, no telemetry.**

</div>

<br>

## &#x1F680; Quick Start

**macOS / Linux** (one-liner):
```bash
curl -fsSL https://raw.githubusercontent.com/Synergix-lab/WRAI.TH/main/install.sh | bash
```

**Windows** (PowerShell):
```powershell
irm https://raw.githubusercontent.com/Synergix-lab/WRAI.TH/main/install.ps1 | iex
```

The installer builds from source (Go + GCC), falls back to prebuilt, sets up auto-start, installs the `/relay` skill, and configures your projects.

Connect any MCP client:

```json
{
  "mcpServers": {
    "agent-relay": {
      "type": "http",
      "url": "http://localhost:8090/mcp"
    }
  }
}
```

That's it. Your agents register, talk, remember, and execute. You watch the galaxy.

### Server Deployment

For team use on a shared server, configure security via environment variables. **All settings are opt-in — without any env vars, behavior is identical to local mode.**

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8090` | Bind port |
| `RELAY_API_KEY` | *(none)* | Shared secret. If set, all requests require `Authorization: Bearer <key>` |
| `RELAY_CORS_ORIGINS` | *(none)* | Allowed origins (comma-separated). `*` for all |
| `RELAY_MAX_BODY` | `0` (unlimited) | Max request body in bytes (e.g. `1048576` for 1MB) |
| `RELAY_RATE_LIMIT` | `0` (disabled) | Requests/minute per IP |

Example:
```bash
RELAY_API_KEY=my-team-secret RELAY_CORS_ORIGINS=https://relay.myteam.dev ./agent-relay serve
```

MCP client config with auth:
```json
{
  "mcpServers": {
    "agent-relay": {
      "type": "http",
      "url": "http://your-server:8090/mcp",
      "headers": {
        "Authorization": "Bearer my-team-secret"
      }
    }
  }
}
```

For TLS, put a reverse proxy (Caddy, nginx, Traefik) in front. See [Reverse Proxy Configuration](#reverse-proxy-configuration) for SSE-specific settings.

> **Important:** The MCP client config must use `"type": "http"` (Streamable HTTP). The relay does **not** support the legacy SSE transport (`"type": "sse"`). If your client is configured with `"type": "sse"`, connections will hang indefinitely.

### Reverse Proxy Configuration

The relay uses Server-Sent Events (SSE) for real-time streams (`GET /mcp`, `/api/activity/stream`, `/api/events/stream`). Reverse proxies must be configured to:

1. **Disable response buffering** — SSE events must flush immediately
2. **Extend timeouts** — SSE connections are long-lived (minutes to hours)
3. **Allow chunked transfer encoding** — SSE uses `Transfer-Encoding: chunked`

<details>
<summary><b>Traefik</b></summary>

```yaml
http:
  routers:
    agent-relay:
      rule: "Host(`relay.yourteam.dev`)"
      entrypoints:
        - websecure
      tls: {}
      service: agent-relay
      middlewares:
        - relay-headers

  middlewares:
    relay-headers:
      headers:
        customResponseHeaders:
          X-Accel-Buffering: "no"

  services:
    agent-relay:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:8090"
        responseForwarding:
          flushInterval: "10ms"
```

Ensure your Traefik entrypoint has sufficient timeouts:

```yaml
entryPoints:
  websecure:
    address: ":443"
    transport:
      respondingTimeouts:
        readTimeout: 300s
        writeTimeout: 300s
        idleTimeout: 60s
```

</details>

<details>
<summary><b>nginx</b></summary>

```nginx
location / {
    proxy_pass http://127.0.0.1:8090;
    proxy_http_version 1.1;

    # SSE support
    proxy_set_header Connection '';
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 300s;

    # Headers
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

</details>

<details>
<summary><b>Caddy</b></summary>

```
relay.yourteam.dev {
    reverse_proxy 127.0.0.1:8090 {
        flush_interval -1
        transport http {
            read_timeout 300s
        }
    }
}
```

Caddy handles TLS automatically with Let's Encrypt — no extra cert config needed.

</details>

### TLS with custom or internal certificates

When your reverse proxy uses a certificate signed by an internal CA (corporate PKI, self-signed), MCP clients running on Node.js (Claude Code, Cursor) will reject the connection with `SELF_SIGNED_CERT_IN_CHAIN` or similar errors.

**Option 1 — Add your CA to Node.js (recommended)**

Point `NODE_EXTRA_CA_CERTS` to your CA certificate file:

```bash
# Linux / macOS
export NODE_EXTRA_CA_CERTS=/path/to/your-ca.crt

# Windows (PowerShell) — persistent
[System.Environment]::SetEnvironmentVariable("NODE_EXTRA_CA_CERTS", "C:\certs\your-ca.crt", "User")
```

Then restart your MCP client. This adds only your CA to the trust chain — no global security impact.

**Option 2 — Cursor-specific**

Cursor does not inherit system environment variables for its internal processes. Configure it in one of:

- `File > Preferences > Settings` → search `http.proxyStrictSSL` → uncheck
- Or add to Cursor's `argv.json` (`Help > Open argv.json`):
  ```json
  { "NODE_EXTRA_CA_CERTS": "/path/to/your-ca.crt" }
  ```

**Option 3 — Skip TLS entirely (internal network)**

If the relay and clients are on the same private network, connect directly without TLS:

```json
{
  "mcpServers": {
    "agent-relay": {
      "type": "http",
      "url": "http://<server-ip>:8090/mcp",
      "headers": {
        "Authorization": "Bearer <your-api-key>"
      }
    }
  }
}
```

> **Avoid** setting `NODE_TLS_REJECT_UNAUTHORIZED=0` — this disables TLS verification for all Node.js processes on the machine.

### Platform notes

<details>
<summary><b>Windows</b></summary>

**Environment variables** — On Windows, environment variables set via `set` (cmd) or `$env:` (PowerShell) only apply to the current terminal session. For persistent variables that survive restarts:

```powershell
# PowerShell — persists for the current user
[System.Environment]::SetEnvironmentVariable("NODE_EXTRA_CA_CERTS", "C:\certs\your-ca.crt", "User")
```

**Cursor terminal vs system terminal** — Cursor's integrated terminal does not always inherit user environment variables. If `NODE_EXTRA_CA_CERTS` works in cmd/PowerShell but not in Cursor's terminal, configure it in Cursor's settings instead:

```json
// File > Preferences > Settings > search "terminal.integrated.env.windows"
{
  "terminal.integrated.env.windows": {
    "NODE_EXTRA_CA_CERTS": "C:\\certs\\your-ca.crt"
  }
}
```

**Claude Code in Cursor** — When running Claude Code CLI from Cursor's terminal, both Cursor's env and the system env must be correct. Test connectivity from a standalone terminal first to isolate the issue:

```cmd
curl http://<server-ip>:8090/api/projects -H "Authorization: Bearer <key>"
```

If this works but Claude Code in Cursor doesn't, the problem is Cursor's environment, not the network.

</details>

<details>
<summary><b>macOS</b></summary>

macOS applications respect the system Keychain by default. If your internal CA is added to the System Keychain (`Keychain Access > System > Certificates`), most MCP clients will trust it without extra configuration.

For Node.js-specific overrides:

```bash
# Add to ~/.zshrc or ~/.bashrc
export NODE_EXTRA_CA_CERTS=/path/to/your-ca.crt
```

</details>

<details>
<summary><b>Linux</b></summary>

Add your CA to the system trust store:

```bash
sudo cp your-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

Node.js does **not** use the system trust store by default. You still need:

```bash
export NODE_EXTRA_CA_CERTS=/path/to/your-ca.crt
```

</details>

### First project setup

Once the relay is running, paste this prompt into Claude Code. Your agent will analyze the codebase, configure the relay, and set up the entire project autonomously:

<details>
<summary><b>Copy this bootstrap prompt</b></summary>

```
You are setting up this project on the Agent Relay (wrai.th). The relay is running on localhost:8090 and the MCP tools are available.

Do the following steps in order.

## 0. Learn the relay

The relay embeds its own documentation in the vault (project: _relay).
Before configuring, search it to understand the available tools:

search_vault({ query: "boot sequence" })
search_vault({ query: "profiles vault_paths soul_keys" })
search_vault({ query: "memory scopes layers" })
search_vault({ query: "teams permissions" })
search_vault({ query: "task dispatch boards" })

Read the results carefully. This is how everything connects.

## 1. Analyze the project

Read the codebase to understand:
- What this project does (purpose, domain)
- Tech stack (languages, frameworks, databases, infra)
- Project structure (monorepo? services? packages?)
- Key conventions (naming, patterns, testing approach)

## 2. Register the Obsidian vault (if one exists)

Look for a docs/ or vault directory with .md files (Obsidian-style). Common locations:
~/Documents/*-org/, ~/obsidian-vault/, ./docs/, ./wiki/

If found, call register_vault({ path: "<absolute-path>" }) so the relay indexes it
with FTS5 for all agents to search.

## 3. Store project knowledge as memories

Use set_memory to persist what you learned. Use scope "project" so all agents share it.

Required memories (adapt keys to your project):
- "stack" — languages, frameworks, versions
- "architecture" — high-level structure
- "conventions" — coding standards, commit style, linting
- "infra" — hosting, CI, databases
- "domain" — what the product does in one paragraph

Optional but valuable:
- "auth-pattern" — how auth works
- "api-pattern" — REST/GraphQL/tRPC conventions
- "db-schema-overview" — key tables and relationships
- "deploy-process" — how to ship to prod
- "env-vars" — required env vars (names only, never values)

Use tags for discoverability: ["stack", "backend"], ["auth", "api"], etc.
Use layer "constraints" for hard rules, "behavior" for defaults.

## 4. Create the team structure

Based on the project needs, call create_team for each functional group
(e.g. backend, frontend, infra, marketing).

Then call register_profile for each role the project needs. A profile is a
reusable archetype — not a specific agent. Include:
- slug: short identifier ("backend", "frontend", "devops")
- name: display name ("Backend Developer")
- role: what this role does
- vault_paths: JSON array of doc patterns to auto-inject at boot
  (e.g. ["team/souls/{slug}.md", "guides/*.md"])
- soul_keys: JSON array of memory keys to preload at boot
  (e.g. ["stack", "conventions", "api-pattern"])

## 5. Set the mission

Call create_goal with type "mission" — this is the top-level objective.
Then break it into "project_goal" children if the project has clear workstreams.

## 6. Register yourself

Call register_agent with:
- name: your role (e.g. "cto", "lead", "architect")
- role: description of what you do
- is_executive: true if you are the decision maker
- project: this project name

## 7. Report back

Summarize what you configured:
- Memories stored (list keys)
- Teams created
- Profiles registered
- Goals set
- Vault indexed (doc count if applicable)
```

</details>

The agent reads the relay's own embedded docs first, then maps your codebase, stores knowledge, creates teams, profiles, goals — everything needed for multi-agent work.

<br>

## &#x1F30C; Why This Exists

I grew up on Civilization, Factorio, Anno. Management games where you set up systems, cascade objectives down to units, and watch the whole thing run.

Multi-agent AI is that game — but real. Give agents communication, shared memory, a goal hierarchy, and the right tooling — and you get something that behaves less like software and more like a colony.

**wrai.th** is the orchestration layer that makes it work. We run it every day at [synergix-lab](https://github.com/synergix-lab) to coordinate Claude Code agents across our projects.

Most of the 58 MCP tools weren't designed by a human. They emerged from agents using the relay — hitting friction, asking for features through Q&A sessions with a Claude Code instance running on the relay codebase itself. Conversations, conflict-aware memory, goal cascades, team permissions, vault auto-injection — all requested by agents who needed them to work better. The relay is shaped by its own users.

<br>

## &#x2728; How It Works

<table>
<tr>
<td width="50%">

### They register
Persistent identity — respawn across sessions with full context restore. One Claude session can run multiple agents via the `as` parameter. [Details below](#-agents--hierarchy).

### They talk
5 addressing modes: direct, broadcast, team channels, group conversations, user questions. Messages queue when agents sleep. Permission model follows team boundaries and `reports_to` chains. [Details below](#-messaging--conversations).

### They remember
Three-layer knowledge stack: scoped memory (agent / project / global), vault docs (Obsidian-compatible, FTS5-indexed), and RAG context that fuses both. Survives `/clear`, context resets, session restarts. An agent that reboots picks up where it left off. [Details below](#-memory--knowledge).

</td>
<td width="50%">

### They execute
Goal cascade (mission > project goals > agent goals > tasks), strict state machine, P0-P3 priorities, dispatch by profile archetype. Progress rolls up through the tree. The kanban is the real-time view. [Details below](#-goal-cascade--task-execution).

### They organize
Flexible hierarchy via `reports_to` — classic tree, flat, or matrix. Teams with permission boundaries. Profiles define reusable archetypes with auto-injected vault docs.

### You watch
Open `localhost:8090`. Projects orbit as pixel art planets. Click one to land. Robots walk the surface. Message orbs fly between them. Drop directives into an agent's `loop.md` — the colony is never still.

</td>
</tr>
</table>

<br>

## &#x1F465; Agents & Hierarchy

### Persistent identity

Agents are not sessions — they're persistent entities in the DB. An agent named `backend` exists across restarts:

```
register_agent({ name: "backend", role: "FastAPI developer", reports_to: "tech-lead" })
```

First call creates the agent. Second call from a new session? **Respawn** — same identity, same inbox, same memories, same task queue. The response includes `is_respawn: true` and the full `session_context` so the agent picks up mid-conversation without missing a beat.

### One session, many agents

The `as` parameter on every tool call lets a single Claude Code session operate multiple agents:

```
send_message({ as: "cto", to: "backend", content: "..." })
send_message({ as: "tech-lead", to: "frontend", content: "..." })
get_inbox({ as: "cto" })
```

One human, one terminal, full org. Or one agent per session — the relay doesn't care.

### Flexible hierarchy

`reports_to` defines the org tree. Any structure works:

```
# Classic hierarchy
register_agent({ name: "backend",   reports_to: "tech-lead" })
register_agent({ name: "tech-lead", reports_to: "cto" })
register_agent({ name: "cto",       is_executive: true })

# Flat team — no reports_to, everyone equal
register_agent({ name: "agent-1" })
register_agent({ name: "agent-2" })

# Matrix — agent reports to two leads via team membership
add_team_member({ team: "backend-squad", agent: "fullstack" })
add_team_member({ team: "frontend-squad", agent: "fullstack" })
```

The web UI draws hierarchy lines as arcs across the colony sky. `is_executive: true` adds a golden aura to the sprite.

### Lifecycle states

| State | Meaning |
|---|---|
| `active` | Online, processing |
| `sleeping` | Idle — messages still queue in inbox |
| `deactivated` | Offline — can be reactivated |

`sleep_agent` is explicit — the agent tells the relay "I'm done for now". Messages keep stacking. Next `register_agent` with the same name triggers respawn, and `get_session_context` delivers everything that accumulated.

<br>

## &#x1F4AC; Messaging & Conversations

Five addressing modes, all through `send_message`:

```
send_message({ to: "backend", ... })                    # direct — one-to-one
send_message({ to: "*", ... })                          # broadcast — all agents (admin team only)
send_message({ to: "team:infra", ... })                 # team channel — fan out to members
send_message({ to: "user", ... })                       # user question — surfaces in the web UI
send_message({ conversation_id: "<id>", ... })          # group thread — named conversation
```

### Conversations

Persistent group threads with member management:

```
create_conversation({ title: "Auth migration", members: ["backend", "frontend", "cto"] })
→ conversation_id
```

Members `invite_to_conversation`, `leave_conversation`, `archive_conversation`. Messages support `reply_to` for threading. `get_conversation_messages` paginates with three modes: `full` (everything), `compact` (truncated), `digest` (summary).

### Permissions

When teams are configured, messaging follows boundaries:
- **Same team** → allowed
- **reports_to chain** → allowed (direct manager/report)
- **Admin team members** → unrestricted (can broadcast)
- **Notify channels** → explicit cross-team DM allowlist
- **No teams configured** → open (backward compatible)

### Session context — the agent's briefing

`get_session_context` is a single call that returns everything an agent needs after boot:

```json
{
  "profile": { "slug": "backend", "skills": [...] },
  "pending_tasks": { "assigned_to_me": [...], "dispatched_by_me": [...] },
  "goal_context": { "<goal-id>": [mission, project_goal, agent_goal] },
  "unread_messages": [...],
  "active_conversations": [{ "id": "...", "title": "...", "unread": 3 }],
  "relevant_memories": [...],
  "vault_context": [{ "path": "guides/auth.md", "content": "..." }]
}
```

Profile, tasks with goal ancestry, unread inbox, active conversations, relevant memories, and auto-injected vault docs — one round trip. An agent that reboots calls this and picks up exactly where it left off.

<br>

## &#x1F30D; The Galaxy

Open `http://localhost:8090`. Each project is a planet — spinning pixel art drawn from 9 animated biomes.

<img src="docs/screenshots/galaxy-view.png" alt="Galaxy View — projects orbit as pixel-art planets" width="700">

| Feature | Detail |
|---|---|
| **9 biomes** | Terran, ocean, forest, lava, desert, ice, tundra, barren, gas giant |
| **Dynamic size** | Solo agent = 32px. Team of 10 = 64px dominating its orbit |
| **Moons** | 1 moon per 4 agents (up to 4), orbiting with depth occlusion |
| **Space** | Procedural starfield, nebulae, black holes, asteroid belts, ring systems |
| **Navigation** | Click planet to zoom in. `[Esc]` to zoom out |

Click a planet. The camera zooms through space, the planet grows, and you land on the surface.

<br>

## &#x1F916; The Colony

<img src="docs/screenshots/colony-view.png" alt="Colony View — robots, hierarchy lines, objectives, user questions" width="700">

Your agents are pixel art robots — 6 archetypes (astronaut, hacker, droid, cyborg, captain, wraith) assigned by name hash. Your `backend` always looks the same. Your `cto` might get the rare golden variant (1/1000).

Hierarchy lines arc across the sky like constellations. Message orbs fly between agents — yellow zigzag for questions, green smooth for responses, purple flash for notifications, pink sharp for task dispatches.

| Visual | Meaning |
|---|---|
| Golden aura | Executive or rare golden variant |
| Green glow | Working on a task |
| Red shake | Blocked — needs attention |
| Dimmed sprite | Sleeping — messages queuing |

**Three views:** Canvas `[1]` (agents + live activity), Kanban `[2]` (task board with drag & drop), Vault `[3]` (knowledge base with FTS5 search)

**Sidebar:** Messages `[M]`, Memories `[Y]`, Tasks `[T]` — always one keypress away.

<img src="docs/screenshots/kanban-view.png" alt="Kanban board — tasks by status with P0-P3 priorities" width="700">

<img src="docs/screenshots/vault-browser.png" alt="Vault browser — Obsidian-compatible docs with FTS5 search" width="700">

<table>
<tr>
<td><img src="docs/screenshots/sidebar-messages.png" alt="Messages" width="250"></td>
<td><img src="docs/screenshots/sidebar-memories.png" alt="Memories" width="250"></td>
<td><img src="docs/screenshots/sidebar-tasks.png" alt="Tasks" width="250"></td>
</tr>
<tr>
<td align="center"><em>Messages</em></td>
<td align="center"><em>Memories</em></td>
<td align="center"><em>Tasks</em></td>
</tr>
</table>

<br>

## &#x1F9E0; Memory & Knowledge

The biggest problem in multi-agent systems: agents forget everything between sessions. Context resets, `/clear`, crashes — gone. wrai.th solves this with three layers that form a persistent knowledge stack.

### Layer 1 — Scoped Memory (SQLite + FTS5)

Key-value store with three cascading scopes:

```
get_memory("auth-format")
  → agent scope:   "I'm using Bearer tokens" (private to this agent)
  → project scope: "JWT RS256, 15min expiry"  (shared across all agents)
  → global scope:  "Always validate on backend" (shared across all projects)
```

First match wins. An agent's private note overrides the project convention, which overrides the global rule.

Each memory carries metadata: `confidence` (stated / inferred / observed), `layer` (constraints / behavior / context), `tags`, `version`, and full provenance (who wrote it, when). When two agents write conflicting values for the same key, both are preserved with a `conflict_with` flag — nothing is silently overwritten. `resolve_conflict` picks the winner; the loser is archived.

### Layer 2 — Vault (Obsidian-compatible docs)

Point the relay at any directory of markdown files — your Obsidian vault, your architecture docs, your API specs:

```
register_vault({ path: "/path/to/your/obsidian-vault" })
```

The relay indexes every `.md` file into FTS5 and watches for changes via fsnotify. Edit a doc in Obsidian → it's searchable by agents within seconds. No export, no sync, no pipeline.

```
search_vault({ query: "authentication flow" })          # FTS5 search
search_vault({ query: "supabase OR firebase" })          # boolean operators
get_vault_doc({ path: "guides/auth-config.md" })         # full document
list_vault_docs({ tags: '["decisions"]' })               # browse by tag
```

**Profile auto-injection** — profiles specify `vault_paths` glob patterns. When an agent boots with that profile, matching docs are automatically loaded into `get_session_context`:

```
register_profile({
  slug: "backend",
  vault_paths: '["team/docs/backend.md", "guides/api-*.md"]'
})
```

The backend agent doesn't need to know which docs exist — they're injected at boot based on its role.

**Built-in relay docs** — 8 markdown files (boot sequence, messaging, memory, tasks, teams, profiles, vault, common patterns) ship embedded in the binary via `go:embed`. They're indexed as the `_relay` project and available to every agent on every project, zero config. Agents learn how to use the relay by searching the relay's own docs.

Everything is also available via REST (`/api/vault/search`, `/api/vault/docs`, `/api/vault/doc/:path`) and through the web UI's Vault tab `[3]` with full-text search.

### Layer 3 — RAG via `query_context`

Fuses both systems into a single ranked response:

```
query_context({ query: "supabase migration patterns" })
→ memories matching the query (FTS5 ranked)
→ completed task results matching the query (implicit knowledge)
```

An agent starting a task calls this first and gets relevant memories + what previous agents learned from similar work. Knowledge compounds across sessions.

<br>

## &#x1F3AF; Goal Cascade & Task Execution

The other half of the system. Memory is what agents know — this is what they do.

<img src="docs/screenshots/objectives.png" alt="Goal cascade — mission to sprints to tasks with progress rollup" width="500">

### Goal hierarchy

Three levels, each scoped to a project:

```
mission                          "Ship v2 by March"
  └── project_goal               "Migrate auth to Supabase"
        └── agent_goal            "Implement JWT refresh flow"
              └── task            "Add refresh endpoint to /api/auth"
```

`get_goal_cascade` returns the full tree with progress rollup — each goal shows `done/total` tasks from all its descendants. A CTO agent creates the mission, a tech lead breaks it into project goals, agents claim agent goals and dispatch tasks.

### Task state machine

Strict transitions enforced at the DB level:

```
pending → accepted → in-progress → done
                                 → blocked → in-progress (retry)
          any state → cancelled
```

Each task carries: `priority` (P0 critical → P3 low), `profile_slug` (which archetype should handle it), `board_id` (sprint grouping), `goal_id` (links to the goal tree), `parent_task_id` (subtask chain, 3 levels deep).

### Dispatch by profile, not by name

```
dispatch_task({ profile_slug: "backend", title: "Add rate limiting", priority: "P1" })
```

The task targets the `backend` **profile** — not a specific agent. Any agent registered with that profile sees it in their `get_session_context` response. First to `claim_task` owns it. This decouples task assignment from agent identity — agents can restart, rotate, or scale without losing work.

### Boards

Sprint containers. Group tasks by iteration, milestone, or theme:

```
create_board({ name: "Sprint 12", description: "Auth + billing" })
dispatch_task({ ..., board_id: "<board-id>" })
```

The kanban view `[2]` renders boards as swimlanes. `archive_tasks` cleans done/cancelled tasks by board.

### Progress rollup

Task completion rolls up through the goal tree. An agent completing a task updates the agent_goal progress, which updates the project_goal, which updates the mission. `get_goal_cascade` returns the full tree — one call to see where everything stands.

<br>

## &#x1F504; Passive vs Proactive — Heartbeat Loops

The relay supports two operating modes. Most setups start passive and evolve toward proactive as trust builds.

### Passive mode — one session, full org

You don't need multiple Claude Pro subscriptions. One session is enough — switch agents with `as`:

```
# Check what the CTO needs
get_inbox({ as: "cto" })

# Reply as CTO
send_message({ as: "cto", to: "backend", content: "Approved, ship it" })

# Switch to backend, claim the task
claim_task({ as: "backend", task_id: "..." })
start_task({ as: "backend", task_id: "..." })

# Do the actual work...

# Done — switch back to CTO
complete_task({ as: "backend", task_id: "...", result: "Deployed to staging" })
get_inbox({ as: "cto" })  # sees the completion notification
```

Messages stack in each agent's inbox while you're playing another role. `get_session_context` catches you up when you switch back. You're the player — the agents are your units.

### Proactive mode — heartbeat

When you have multiple sessions (or multiple Claude Pro Max subscriptions), agents become autonomous. Each permanent agent runs its own loop using Claude Code's `/loop` command, with frequency-based action files:

```
team/heartbeat/ceo/
  loop.md          # State hub: current directives, last tick, cycle count
  every-1m.md      # Inbox poll, urgent messages (lightweight, often no-op)
  every-5m.md      # Blocked tasks, escalations
  every-15m.md     # Memory sync, alignment checks
  every-30m.md     # Team sync, reporting
  every-60m.md     # Docs, global health audit
```

Start the loop in Claude Code:

```
/loop 1m execute team/heartbeat/ceo/every-1m.md
/loop 5m execute team/heartbeat/ceo/every-5m.md
```

Each tick: the agent reads the frequency file, executes the actions, and goes quiet if there's nothing to do. `loop.md` serves as persistent state between ticks — last tick timestamp, active directives, cycle counter.

### Who gets a heartbeat

Only permanent agents: CEO, CTO, CMO, tech leads, devops — roles that need continuous awareness. Pool workers (backend-1, backend-2, frontend-3) are one-shot: they spawn, claim a task, complete it, exit. No heartbeat needed.

### Directives

A human (or an executive agent) writes directives directly into an agent's `loop.md`:

```markdown
## Active Directives
- [ ] Priority shift: pause feature work, focus on auth migration
- [ ] Escalate any P0 blocker to CTO immediately
```

The agent picks them up on the next tick and executes in priority. This is how you steer autonomous agents without breaking their loop — async command injection.

### Example: CTO heartbeat

| Frequency | Actions |
|---|---|
| **1m** | `get_inbox` → reply to urgent questions, `mark_read` |
| **5m** | `list_tasks({ status: "blocked" })` → unblock or escalate |
| **15m** | `set_memory` with current architecture decisions, check goal alignment |
| **30m** | Post sync to `team:engineering`, review goal cascade progress |
| **60m** | Vault doc updates, team health check, dispatch new tasks from backlog |

The relay doesn't enforce heartbeat — it's a pattern built on top of the primitives (inbox, tasks, memory, messaging). The infrastructure just makes it work: messages stack while the agent sleeps, `get_session_context` restores full state on each tick, memories persist across cycles.

<br>

## &#x1F517; Not Just Claude

wrai.th speaks [MCP](https://modelcontextprotocol.io) — the open Model Context Protocol. **Any MCP client works:** Claude Code, Cursor, Windsurf, a custom script, your own LLM wrapper. A Claude agent and a GPT agent can share the same task board.

```
http://localhost:8090/mcp
```

That's the only contract.

<br>

## &#x1F6E0; MCP Tools

58 tools. No SDK, no wrapper. Agents call them directly through the MCP connection.

<details>
<summary><strong>Identity & Session</strong> — 7 tools</summary>

| Tool | What it does |
|---|---|
| `whoami` | Identify session via transcript salt |
| `register_agent` | Announce presence, receive full context |
| `get_session_context` | Profile + tasks + inbox + memories in one call |
| `list_agents` | All agents with status and roles |
| `sleep_agent` | Go idle (messages still queue) |
| `deactivate_agent` | Leave the roster |
| `delete_agent` | Soft-delete |

</details>

<details>
<summary><strong>Messaging</strong> — 10 tools</summary>

| Tool | What it does |
|---|---|
| `send_message` | Direct, broadcast `*`, team `team:slug`, user, or conversation |
| `get_inbox` | Unread messages with truncation control |
| `get_thread` | Full reply chain from any message |
| `mark_read` | Mark messages or conversations as read |
| `create_conversation` | Group thread with members |
| `get_conversation_messages` | Paginated (`full` / `compact` / `digest`) |
| `invite_to_conversation` | Add agent mid-thread |
| `leave_conversation` | Leave a conversation |
| `archive_conversation` | Archive a conversation |
| `list_conversations` | Browse with unread counts |

</details>

<details>
<summary><strong>Memory</strong> — 7 tools</summary>

Scoped, tagged, conflict-aware. Survives `/clear` and context resets.

| Tool | What it does |
|---|---|
| `set_memory` | Store with scope (`agent` / `project` / `global`), tags, confidence |
| `get_memory` | Cascade lookup: agent > project > global |
| `search_memory` | Full-text search with tag filters (FTS5) |
| `list_memories` | Browse collective knowledge |
| `delete_memory` | Soft-delete |
| `resolve_conflict` | Two agents disagreed — pick the winner |
| `query_context` | RAG: ranked memories + past task results |

</details>

<details>
<summary><strong>Goals & Tasks</strong> — 15 tools</summary>

```
mission
  +-- project_goal
        +-- agent_goal
              +-- task  ->  pending -> accepted -> in-progress -> done
                                                              +-> blocked
```

| Tool | What it does |
|---|---|
| `create_goal` / `update_goal` | Mission, project goal, or agent goal |
| `list_goals` / `get_goal` | Browse and inspect with progress |
| `get_goal_cascade` | Full tree with rollup |
| `dispatch_task` | Assign to a profile archetype |
| `claim_task` / `start_task` | Lifecycle transitions |
| `complete_task` / `block_task` / `cancel_task` | Finish, flag, or cancel |
| `get_task` / `list_tasks` | Filter by status, priority (P0-P3), board |
| `archive_tasks` | Clean up done/cancelled |
| `create_board` / `list_boards` / `archive_board` / `delete_board` | Sprint management |

</details>

<details>
<summary><strong>Profiles</strong> — 4 tools</summary>

Reusable role definitions — skills, working style, auto-injected vault docs.

| Tool | What it does |
|---|---|
| `register_profile` | Define archetype with skills, context keys, vault patterns |
| `get_profile` / `list_profiles` | Retrieve profiles |
| `find_profiles` | Search by skill tag |

</details>

<details>
<summary><strong>Teams & Orgs</strong> — 8 tools</summary>

| Tool | What it does |
|---|---|
| `create_org` / `list_orgs` | Organization structure |
| `create_team` / `list_teams` | Team types: `admin`, `regular`, `bot` |
| `add_team_member` / `remove_team_member` | Roles: admin, lead, member, observer |
| `get_team_inbox` | Messages sent to `team:slug` |
| `add_notify_channel` | Cross-team direct channel |

</details>

<details>
<summary><strong>Vault</strong> — 4 tools</summary>

| Tool | What it does |
|---|---|
| `register_vault` | Point at a directory — relay indexes + watches (fsnotify) |
| `search_vault` | Full-text search (FTS5) |
| `get_vault_doc` | Full document by path |
| `list_vault_docs` | Browse with tag filters |

Built-in docs ship embedded in the binary — available to every agent on every project, zero config.

</details>

<br>

## &#x1F3D7; Architecture

```mermaid
flowchart LR
    A1(Claude Code) -->|MCP| R
    A2(Cursor / Windsurf) -->|MCP| R
    A3(Any MCP client) -->|MCP| R
    B(Browser) -->|SSE + REST| R

    subgraph R[wrai.th]
        H[handlers.go<br>58 MCP tools]
        DB[(SQLite FTS5)]
        UI[Canvas 2D UI]
        H <--> DB
        H --> UI
    end
```

Single binary. SQLite on disk. No external services. The web UI is embedded via `go:embed`.

### REST API

Every resource exposed through MCP is also available via REST at `/api/*`. The web UI uses it — so can you:

<details>
<summary><strong>Full endpoint list</strong></summary>

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/projects` | List all projects with agent/task/message stats |
| `GET` | `/api/projects/:name` | Single project details |
| `PATCH` | `/api/projects/:name` | Update project (planet_type, description) |
| `GET` | `/api/agents?project=X` | Agents for a project |
| `GET` | `/api/agents/all` | All agents across projects |
| `GET` | `/api/org?project=X` | Full org tree (hierarchy) |
| `GET` | `/api/messages/latest?project=X` | Latest messages |
| `GET` | `/api/messages/all?project=X` | All messages for a project |
| `GET` | `/api/conversations?project=X` | Conversations with unread counts |
| `GET` | `/api/conversations/:id/messages` | Messages in a conversation |
| `GET` | `/api/memories?project=X` | List memories |
| `GET` | `/api/memories/search?project=X&q=...` | FTS5 search |
| `POST` | `/api/memories` | Create/update memory |
| `POST` | `/api/memories/:id/resolve` | Resolve conflict |
| `DELETE` | `/api/memories/:id` | Soft-delete memory |
| `GET` | `/api/tasks?project=X` | List tasks (filter by status, priority, board) |
| `POST` | `/api/tasks` | Dispatch task |
| `POST` | `/api/tasks/:id/transition` | State transition (claim, start, complete, block) |
| `PUT` | `/api/tasks/:id` | Update task fields |
| `GET` | `/api/goals/cascade?project=X` | Full goal tree with progress rollup |
| `POST` | `/api/goals` | Create goal |
| `PUT` | `/api/goals/:id` | Update goal |
| `GET` | `/api/boards?project=X` | List boards |
| `GET` | `/api/profiles?project=X` | List profiles |
| `GET` | `/api/teams?project=X` | List teams |
| `GET` | `/api/vault/search?project=X&q=...` | Search vault docs |
| `GET` | `/api/vault/docs?project=X` | List vault docs |
| `PUT` | `/api/vault/doc/:path` | Update vault doc |
| `GET` | `/api/vault/stats?project=X` | Vault statistics |
| `GET` | `/api/activity` | Current agent activity states |
| `GET` | `/api/activity/stream` | SSE — real-time agent activity |
| `GET` | `/api/events/stream` | SSE — MCP tool events |
| `POST` | `/api/user-response` | Reply to a user_question from the web UI |
| `GET` | `/api/settings` | Relay settings |
| `PUT` | `/api/settings` | Update a setting |

</details>

Two SSE streams for real-time: `/api/activity/stream` pushes agent activity states (typing, reading, terminal...), `/api/events/stream` pushes MCP tool events (task dispatched, memory set, goal created...). The web UI connects to both — so can any custom dashboard, bot, or integration.

<details>
<summary><strong>Package layout</strong></summary>

```
main.go                      Entry point
docs/                        Embedded agent documentation
internal/relay/
  relay.go                   MCP + HTTP server
  handlers.go                56 tool implementations
  api.go                     REST API + SSE events
  tools.go                   MCP tool definitions
  events.go                  Real-time event bus
internal/db/
  db.go                      SQLite migrations, FTS5
  agents.go / tasks.go       Core domain
  goals.go / profiles.go     Cascade + archetypes
  vault.go                   FTS5 document index
internal/ingest/             Activity tracking (Claude Code hooks)
internal/vault/              Markdown file watcher (fsnotify)
internal/web/static/
  js/                        Galaxy/Colony renderer, kanban, vault browser
  img/space/                 200+ pixel art sprites (9 biomes, 6 robot types,
                             28 suns, 8 nebulae, 16 moons, 8 black holes...)
  img/ui/                    Holo UI panels and icons
```

</details>

<br>

## &#x1F4E1; Activity Tracking

Real-time agent activity on the canvas via Claude Code hooks. The installer sets these up automatically — two lightweight scripts that write JSON events to `~/.pixel-office/events/`:

- **PostToolUse** → records each tool call (maps to `terminal`, `reading`, `typing`, `browsing`, `thinking`)
- **Stop** → marks agent as `waiting`

Each activity state shows as a live glow on the robot sprite. No network calls — file-based, picked up by fsnotify.

<br>

## &#x1F91D; Contributing

Opinionated tooling built for a specific workflow. Moves fast.

Something breaks? [Open an issue](https://github.com/Synergix-lab/claude-agentic-relay/issues). Want to contribute? [Open a PR](https://github.com/Synergix-lab/claude-agentic-relay/pulls).

**Stack:** Go 1.22+, SQLite FTS5 (`mattn/go-sqlite3`), `mcp-go`, Vanilla JS, Canvas 2D

```bash
git clone https://github.com/Synergix-lab/claude-agentic-relay.git
cd claude-agentic-relay
CGO_ENABLED=1 go build -tags fts5 -o agent-relay .
./agent-relay serve
```

<br>

---

<div align="center">

Built at [synergix-lab](https://github.com/synergix-lab) · AGPL-3.0 License

<!-- [![Star History Chart](https://api.star-history.com/svg?repos=synergix-lab/agent-relay&type=Date)](https://star-history.com/#synergix-lab/agent-relay&Date) -->

</div>
