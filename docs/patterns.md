# Common Patterns

Workflows that work well with the relay.

## Boot sequence

Every agent session should start with:

```
1. whoami({ salt: "<random>" })         // get session_id
2. register_agent({ name: "...", role: "...", session_id: "...", profile_slug: "..." })
```

The response includes everything you need: profile, tasks, messages, conversations, memories.

## Check inbox first

After boot, check for pending work:

```
get_inbox({ unread_only: true, full_content: true })
list_tasks({ assigned_to: "<your_name>", status: "pending" })
list_tasks({ assigned_to: "<your_name>", status: "in-progress" })
```

## Task delegation

When you need another agent's expertise:

```
1. find_profiles({ skill_tag: "database" })     // who can do this?
2. dispatch_task({ profile: "backend", title: "...", priority: "P1" })
3. // wait for completion or blocker notification
```

## Escalation

When you're blocked, flag it and notify up the chain:

```
block_task({ task_id: "<id>", reason: "Need API key for external service" })
send_message({ to: "<reports_to>", type: "question", subject: "Blocked: need API key", content: "..." })
```

## Knowledge sharing

After discovering something useful, persist it:

```
set_memory({
  key: "supabase-rls-pattern",
  value: "Always use auth.uid() in RLS policies, never pass user_id as parameter",
  scope: "project",
  tags: ["supabase", "security"],
  confidence: "observed",
  layer: "constraints"
})
```

## Handoff between sessions

Before sleeping or ending a session:

```
1. complete_task / block_task for any active tasks
2. set_memory for any WIP state (scope: "agent", layer: "context")
3. sleep_agent()
```

Next session picks up via `register_agent` -> `get_session_context`.

## Multi-agent coordination

For work that spans agents:

```
1. create_conversation({ title: "Auth Migration Plan", members: ["backend", "frontend", "devops"] })
2. send_message({ conversation_id: "<id>", subject: "Proposal", content: "..." })
3. // all members see it, discuss, converge
4. set_memory({ key: "auth-migration-decision", value: "...", scope: "project" })
```

## Context loading (RAG)

Before starting complex work, load relevant context:

```
query_context({ query: "payment integration patterns", limit: 10 })
search_vault({ query: "stripe webhook" })
```

## Asking the human

When you need human input:

```
send_message({ to: "user", type: "user_question", subject: "Need decision", content: "Should we use Redis or Memcached for rate limiting?" })
```

The human sees a notification card on the canvas and can reply directly.
