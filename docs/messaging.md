# Messaging

How agents communicate through the relay.

## Addressing modes

| Mode | `to` value | Example |
|---|---|---|
| Direct | agent name | `"backend"` |
| Broadcast | `*` | `"*"` — all agents in the project |
| Team | `team:<slug>` | `"team:frontend"` |
| User | `"user"` | sends to the human watching the canvas |
| Conversation | set `conversation_id` | `to` is ignored |

## Send a message

```
send_message({
  to: "backend",
  type: "question",
  subject: "Database schema",
  content: "What's the current users table structure?"
})
```

### Message types

| Type | When to use |
|---|---|
| `question` | You need an answer |
| `response` | You're answering a question |
| `notification` | FYI — no reply expected |
| `code-snippet` | Sharing code |
| `task` | Task-related update |
| `user_question` | Asking the human user |

## Threading

Reply to a message by passing `reply_to`:

```
send_message({
  to: "backend",
  type: "response",
  subject: "Re: Database schema",
  content: "Here's the schema...",
  reply_to: "<message_id>"
})
```

Get the full thread:

```
get_thread({ message_id: "<any_message_id_in_thread>" })
```

## Conversations (group threads)

Create a multi-agent conversation:

```
create_conversation({
  title: "API Design Review",
  members: ["backend", "frontend", "tech-lead"]
})
```

Send to the conversation (all members see it):

```
send_message({
  conversation_id: "<conversation_id>",
  type: "notification",
  subject: "Review update",
  content: "..."
})
```

Add someone mid-thread:

```
invite_to_conversation({ conversation_id: "<id>", agent_name: "devops" })
```

## Priority

| Priority | Alias | Meaning |
|---|---|---|
| `P0` | `interrupt` | Critical — drop everything |
| `P1` | `steering` | Important — do next |
| `P2` | `advisory` | Normal (default) |
| `P3` | `info` | Low — when you get to it |

Both short form (`P0`) and MACP aliases (`interrupt`) are accepted:

```
send_message({ to: "backend", priority: "P0", ... })
send_message({ to: "backend", priority: "interrupt", ... })  // same
```

## TTL (Time-to-Live)

Messages expire after `ttl_seconds` (default: 3600 = 1h). Set `0` for never expires:

```
send_message({ to: "backend", ttl_seconds: 300, ... })   // expires in 5 min
send_message({ to: "backend", ttl_seconds: 0, ... })     // never expires
```

Expired messages are excluded from `get_inbox`.

## Delivery Tracking

Each message creates a **delivery record** per recipient with states:

```
queued → surfaced → acknowledged
```

- `queued`: message sent, not yet seen by recipient
- `surfaced`: recipient called `get_inbox` and saw it
- `acknowledged`: recipient explicitly confirmed receipt

Acknowledge a delivery:

```
ack_delivery({ delivery_id: "<id>" })
```

The `delivery_id` is returned in each inbox message.

## Inbox

Check your unread messages:

```
get_inbox({ unread_only: true, limit: 10 })
```

Full content mode (no truncation):

```
get_inbox({ unread_only: true, full_content: true })
```

### Budget pruning

For agents with limited context, apply budget pruning to fit inbox within `max_context_bytes`:

```
get_inbox({ apply_budget: true })
```

Budget prunes by: priority first (P0 kept, P3 cut first), then Jaccard scoring against `interest_tags`. Configure at registration:

```
register_agent({ ..., max_context_bytes: 8192, interest_tags: "[\"database\",\"auth\"]" })
```

Mark as read:

```
mark_read({ message_ids: ["<id1>", "<id2>"] })
// or mark an entire conversation:
mark_read({ conversation_id: "<id>" })
```

## Conversation history

```
get_conversation_messages({
  conversation_id: "<id>",
  format: "digest",    // "full", "compact", or "digest"
  limit: 50
})
```

- `full` — complete content
- `compact` — metadata only (id, from, type, subject, timestamp)
- `digest` — metadata + first 200 chars

## Permissions

When teams are configured, you can only message:
- Agents in your team
- Agents in your `reports_to` chain (up or down)
- Agents you have a `notify_channel` with

Use `add_notify_channel({ target: "agent-name" })` to create a cross-team direct channel. Note: notify channels are **unidirectional** (source→target only).

Executives (`is_executive: true`) bypass all permission checks.

## Cross-project isolation

Messages are strictly namespaced by project. An agent in project A cannot see or send messages to agents in project B. Use the `project` parameter to operate in a different namespace:

```
send_message({ as: "bot", project: "other-project", to: "backend", ... })
```
