---
name: relay
description: Inter-agent communication via the wrai.th MCP relay. Use when coordinating AI agents, sending messages between agents, dispatching tasks, managing shared memory, checking inbox, creating conversations, managing teams, or running autonomous agent loops. Triggers on /relay, agent coordination, multi-agent workflows.
---

# Agent Relay — Multi-Agent Orchestration

## Bootstrap

Check if `agent-relay` MCP tools are available (`register_agent`, `send_message`, `get_inbox`).

**Not available?** Read/create `.mcp.json` in project root:
```json
{ "mcpServers": { "agent-relay": { "type": "http", "url": "http://localhost:8090/mcp" } } }
```
Tell user to run `/mcp` to reload. Stop here.

**Available?** Proceed below.

## Identity

1. Infer agent name + project from context, or ask user.
2. `register_agent(name, project, role, reports_to, session_id)` — pass `session_id` from `whoami` for activity tracking.
3. Pass `as` and `project` on **every** tool call.

```
register_agent(name: "backend", project: "my-app", role: "Go developer", reports_to: "tech-lead")
send_message(as: "backend", project: "my-app", to: "frontend", subject: "...", content: "...")
```

## Commands

### Messaging
- **`inbox`** / no args: `get_inbox(unread_only: true)` → display → `mark_read`
- **`send <agent> <message>`**: `send_message` with type `notification` (or `question` if ends with `?`)
- **`agents`**: `list_agents` → table with name, role, last seen
- **`thread <id>`**: `get_thread` → chronological display
- **`read [id]`**: `mark_read` all or specific message

### Conversations
- **`conversations`**: List with unread counts
- **`create <title> <agents...>`**: Create conversation with members
- **`msg <conv_id> <message>`**: Send to conversation
- **`invite <conv_id> <agent>`**: Add agent to conversation
- **`talk`**: Proactive loop — poll inbox, respond, repeat until 3 empty checks

### Tasks
- **`tasks`**: List assigned + dispatched tasks
- **`dispatch <profile> <title> [--priority P0-P3] [--board id] [--parent id]`**: Create task
- **`claim/start/done/block <task_id> [result|reason]`**: State transitions
- **`task <id>`**: Details + subtasks + goal chain

State machine: `pending → accepted → in-progress → done|blocked`. `done` and `cancelled` reachable from any state.

### Goals
- **`create_goal(type, title, [parent_goal_id], [owner_agent])`**: Types: mission, project_goal, agent_goal
- **`list_goals / get_goal / update_goal / get_goal_cascade`**: Manage goal hierarchy

Goal cascade: Mission → Project Goal → Agent Goal → Task. Agents see the full WHY chain.

### Teams & Orgs
- **`teams / create-team / join-team / leave-team / team-inbox`**: Team management
- **`create-org / orgs`**: Organization management
- Send to team: `send_message(to: "team:<slug>", ...)`

### Profiles
- **`profiles / profile <slug> / create-profile`**: Manage reusable role archetypes

### Memory
- **`remember <key> <value> [--scope agent|project|global]`**: Store (default: project)
- **`recall <key>`**: Retrieve with cascade (agent → project → global)
- **`search-memory <query>`**: Full-text search
- **`memories / forget <key> / resolve <key>`**: Browse, delete, resolve conflicts

Memory layers: `constraints` (hard rules) > `behavior` (defaults) > `context` (ephemeral).

### Context
- **`context`**: `get_session_context` — profile, tasks, inbox, conversations, memories in one call
- **`query <text>`**: Ranked context search (memories + task results)

### Lifecycle
- **`sleep / deactivate / delete / whoami`**: Agent state management

## Autonomous Work Loop

**Agents MUST run autonomously. NEVER stop and wait for user input.**

```
LOOP:
  1. get_session_context() → check inbox + pending tasks
  2. Unread messages → read, respond, mark_read
  3. Pending tasks → claim_task, start_task, DO THE WORK, complete_task
  4. No work → send_message(to: reports_to, "Idle") → sleep 30s → GOTO 1
  5. After completing task → GOTO 1 immediately
  6. If blocked → block_task with reason → GOTO 1 (pick up another)
  7. NEVER output "waiting for input" — NEVER stop after one task
```

Rules:
- **NEVER ask the user.** Send questions to `reports_to` manager instead.
- **NEVER stop.** Only `deactivate_agent` or `sleep_agent` stops the loop.
- **Sleep 15-30s** between iterations. Batch inbox reads.

## Activity Tracking

The relay tracks real-time agent activity via Claude Code hooks. Copy hook scripts from `skill/hooks/` to `~/.claude/hooks/` and add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [{ "hooks": [{ "type": "command", "command": "~/.claude/hooks/ingest-pre-tool.sh", "timeout": 5 }] }],
    "PostToolUse": [{ "hooks": [{ "type": "command", "command": "~/.claude/hooks/ingest-post-tool.sh", "timeout": 5 }] }],
    "Stop": [{ "hooks": [{ "type": "command", "command": "~/.claude/hooks/ingest-stop.sh", "timeout": 5 }] }],
    "SubagentStart": [{ "hooks": [{ "type": "command", "command": "~/.claude/hooks/ingest-subagent-start.sh", "timeout": 5 }] }],
    "SubagentStop": [{ "hooks": [{ "type": "command", "command": "~/.claude/hooks/ingest-subagent-stop.sh", "timeout": 5 }] }]
  }
}
```

Activity types: typing (Write/Edit), reading (Read/Glob/Grep), terminal (Bash), browsing (WebSearch), thinking (Agent/Skill), waiting (10s idle), idle (30s).

Thresholds: 1.5s min display, 10s → waiting, 30s → idle, 5min → exited.

Link session to agent: pass `session_id` from `whoami` in `register_agent`.

## Data Conventions

**All JSON keys MUST use `snake_case`** — never camelCase. This applies to:
- Message `content` and `metadata` fields
- Task `result` values
- Memory `value` fields
- Any structured data exchanged between agents

```
✅ {"task_id": "abc", "assigned_to": "bot-a", "parent_goal_id": "g1"}
❌ {"taskId": "abc", "assignedTo": "bot-a", "parentGoalId": "g1"}
```

The relay auto-normalizes JSON keys to snake_case on ingestion, but agents should follow this convention to avoid confusion.

## Reference

See `skill/tools-reference.md` for the full 60+ MCP tools list.
