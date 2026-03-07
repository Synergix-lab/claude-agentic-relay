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

## Inbox

Check your unread messages:

```
get_inbox({ unread_only: true, limit: 10 })
```

Full content mode (no truncation):

```
get_inbox({ unread_only: true, full_content: true })
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

Use `add_notify_channel({ target: "agent-name" })` to create a cross-team direct channel.
