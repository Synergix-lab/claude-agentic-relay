# Markdown Rendering Test

All supported elements in one document.

## Headings

### H3 — Section Title
#### H4 — Subsection
##### H5 — Minor heading

## Paragraphs

This is a normal paragraph with **bold text**, *italic text*, and `inline code`. You can also have **bold with `code` inside** and *italic with `code` inside*.

This is a second paragraph separated by a blank line. It should have proper spacing from the one above.

## Links

Check the [relay documentation](https://github.com/Synergix-lab/WRAI.TH) for more info. Also see [MCP Protocol](https://modelcontextprotocol.io).

## Lists

### Unordered list

- First item
- Second item with `inline code`
- Third item with **bold** and *italic*
- Fourth item

### Ordered list

1. Step one
2. Step two
3. Step three

### Nested content in lists

- `register_agent` — register/update agent identity
- `send_message` — send to agent, team (`team:slug`), or broadcast (`*`)
- `get_inbox` — get messages (unread_only, limit, full_content)
- `get_session_context` — everything in one call

### Checkbox lists

- [x] Task completed
- [x] Another done task
- [ ] Pending task
- [ ] Another pending

## Blockquotes

> This is a blockquote. It should have a left border and subtle background.

> Multi-line blockquote
> continues here on the second line.

## Code Blocks

### Inline code

Use `register_agent` to start. Pass `session_id` from `whoami`. The `as` parameter is required.

### Fenced code — no language

```
get_session_context()
get_inbox(unread_only: true)
mark_read(message_ids: ["abc123"])
```

### Fenced code — JavaScript

```javascript
const relay = new MCPClient("http://localhost:8090/mcp");
await relay.call("register_agent", {
  name: "backend",
  role: "FastAPI developer",
  reports_to: "tech-lead"
});
```

### Fenced code — Go

```go
func (r *Relay) HandleRegisterAgent(req RegisterAgentRequest) (*AgentResponse, error) {
    agent, err := r.DB.UpsertAgent(req.Name, req.Role)
    if err != nil {
        return nil, fmt.Errorf("register agent: %w", err)
    }
    return &AgentResponse{Agent: agent}, nil
}
```

### Fenced code — JSON

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

### Fenced code — Bash

```bash
curl -fsSL https://raw.githubusercontent.com/Synergix-lab/WRAI.TH/main/install.sh | bash
./agent-relay serve
```

### Fenced code — Python

```python
import httpx

async def register_agent(name: str, role: str):
    async with httpx.AsyncClient() as client:
        resp = await client.post("http://localhost:8090/mcp", json={
            "method": "register_agent",
            "params": {"name": name, "role": role}
        })
        return resp.json()
```

## Tables

### Simple table

| Tool | Description |
|------|-------------|
| `register_agent` | Register/update agent identity |
| `send_message` | Send to agent or team |
| `get_inbox` | Get unread messages |
| `mark_read` | Mark messages as read |

### Table with alignment

| Feature | Status | Priority |
|:--------|:------:|----------:|
| Messaging | Done | P0 |
| Memory | Done | P0 |
| Goals | Done | P1 |
| Vault | Done | P1 |
| Activity | Beta | P2 |

### Wide table

| Scope | Cascade Order | Visibility | Use Case |
|-------|--------------|------------|----------|
| `agent` | First (highest priority) | Private to agent | Personal notes, session state |
| `project` | Second | All agents in project | Shared conventions, architecture |
| `global` | Third (lowest priority) | All agents everywhere | Universal rules, company standards |

## Horizontal Rules

Content above the rule.

---

Content below the rule.

***

Another section after a second rule.

## Mixed Content

### API endpoint with table and code

The `dispatch_task` tool creates a task for a profile:

```
dispatch_task({
  profile_slug: "backend",
  title: "Add rate limiting",
  priority: "P1",
  board_id: "<board-id>"
})
```

State machine transitions:

| From | To | Trigger |
|------|----|---------|
| `pending` | `accepted` | `claim_task` |
| `accepted` | `in-progress` | `start_task` |
| `in-progress` | `done` | `complete_task` |
| `in-progress` | `blocked` | `block_task` |
| any | `cancelled` | `cancel_task` |

### List with code blocks between

- First configure the MCP server
- Then register your agent:

```
register_agent({ name: "backend", role: "Go developer" })
```

- Check your inbox
- Start working on tasks

## Special Characters

Ampersand: R&D department. Less than: 5 < 10. Greater than: 10 > 5.
Pipes in text: use `team:slug` format | not raw pipes.
Backticks: use \`code\` for inline code.

## Empty Section

## End

That's all the markdown elements. If something looks wrong, tell me which section.
