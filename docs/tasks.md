# Tasks & Goals

How work gets assigned, tracked, and completed.

## Task lifecycle

```
pending -> accepted -> in-progress -> done
                                   -> blocked
                                   -> cancelled
```

## Dispatch a task

Tasks are dispatched to a **profile**, not to a specific agent. Any agent running that profile can claim it.

```
dispatch_task({
  profile: "backend",
  title: "Add rate limiting to /api/auth",
  description: "Use sliding window, 100 req/min per IP",
  priority: "P1",
  board_id: "<optional>",
  goal_id: "<optional>"
})
```

### Priorities

| Priority | Meaning |
|---|---|
| `P0` | Critical — drop everything |
| `P1` | High — do next |
| `P2` | Normal |
| `P3` | Low — when you get to it |

## Claim and work

```
claim_task({ task_id: "<id>" })      // pending -> accepted
start_task({ task_id: "<id>" })      // accepted -> in-progress (can skip accepted)
complete_task({ task_id: "<id>", result: "Added sliding window rate limiter..." })
```

## Block or cancel

```
block_task({ task_id: "<id>", reason: "Need Redis for sliding window" })
cancel_task({ task_id: "<id>", reason: "No longer needed" })
```

Blocking triggers a push notification to the dispatcher. If the task has a parent, the parent's dispatcher is notified too.

## Query tasks

```
list_tasks({ status: "pending", profile: "backend", priority: "P1" })
list_tasks({ assigned_to: "backend", status: "in-progress" })
list_tasks({ board_id: "<id>" })
get_task({ task_id: "<id>", include_subtasks: true })  // subtask chain (max depth 3)
```

## Subtasks

Pass `parent_task_id` when dispatching to create subtasks:

```
dispatch_task({
  profile: "backend",
  title: "Write rate limiter middleware",
  parent_task_id: "<parent_id>"
})
```

## Archive

Clean up completed/cancelled tasks:

```
archive_tasks({ status: "done" })                    // archive all done tasks
archive_tasks({ status: "cancelled", board_id: "<id>" })  // archive cancelled on a board
```

## Boards

Boards group tasks into sprints or workstreams.

```
create_board({ name: "Sprint 1", slug: "sprint-1", description: "MVP features" })
list_boards()
archive_board({ board_id: "<id>" })     // hides board + tasks from listings
delete_board({ board_id: "<id>" })      // permanent delete (must archive first)
```

## Goal cascade

Goals flow top-down:

```
mission
  +-- project_goal
        +-- agent_goal
              +-- tasks
```

Progress rolls up automatically from tasks to goals.

```
create_goal({
  type: "mission",
  title: "Launch MVP by Q2"
})

create_goal({
  type: "project_goal",
  title: "Complete authentication system",
  parent_goal_id: "<mission_id>"
})

create_goal({
  type: "agent_goal",
  title: "Implement JWT refresh flow",
  parent_goal_id: "<project_goal_id>",
  owner_agent: "backend"
})
```

Link tasks to goals:

```
dispatch_task({ ..., goal_id: "<agent_goal_id>" })
```

View the full tree:

```
get_goal_cascade()
```

Update goal status:

```
update_goal({ goal_id: "<id>", status: "completed" })  // "active", "completed", "paused"
```
