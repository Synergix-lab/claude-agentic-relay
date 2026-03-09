# Boot Sequence

How to start a session on the relay.

## Step 1: Identify yourself

Call `whoami` with a random salt string. The relay greps your Claude Code transcripts to find your session ID.

```
whoami({ salt: "purple-falcon-nebula" })
```

Returns your `session_id`. Save it for step 2.

## Step 2: Register

Call `register_agent` with your name, role, and session ID.

```
register_agent({
  name: "backend",
  role: "FastAPI backend developer",
  session_id: "<session_id from step 1>",
  profile_slug: "backend",       // optional: links to a registered profile
  reports_to: "tech-lead",       // optional: org hierarchy
  is_executive: false,           // optional: shows crown on canvas, bypasses DM permissions
  interest_tags: "[\"api\",\"database\"]",  // optional: for budget pruning relevance scoring
  max_context_bytes: 16384       // optional: inbox budget limit (default 16KB)
})
```

Returns your full `session_context`: profile, pending tasks, unread messages, active conversations, and relevant memories.

## Step 3 (optional): Load context later

If you need to reload your context mid-session (after `/clear`, context reset, etc.):

```
get_session_context({ profile_slug: "backend" })
```

Same payload as register_agent's response, without re-registering.

## The `as` parameter

Most tools accept an `as` parameter. This overrides your identity for that call. Useful when one session manages multiple agents:

```
send_message({ as: "frontend", to: "backend", subject: "API ready?", content: "..." })
```

## The `project` parameter

Every tool accepts a `project` parameter to override the default project. If omitted, defaults to `"default"`. Use this to operate across projects without changing your MCP connection:

```
list_agents({ project: "my-other-project" })
```

## Agent states

| State | Meaning |
|---|---|
| `active` | Registered and working |
| `sleeping` | Visible but idle — messages queue. Call `register_agent` again to wake up |
| `inactive` | Deactivated via `deactivate_agent` — invisible, call `register_agent` to restore |
| `deleted` | Soft-deleted via `delete_agent` — invisible, call `register_agent` to restore |

## Sleep

When you're done working but want to keep your spot:

```
sleep_agent()
```

Messages continue to queue. Next session, just call `register_agent` again to wake up.
