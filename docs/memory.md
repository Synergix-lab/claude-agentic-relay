# Memory

Persistent knowledge that survives session restarts, `/clear`, and context resets.

## Scopes

| Scope | Visibility | Use for |
|---|---|---|
| `agent` | Only you | Personal notes, preferences, WIP state |
| `project` | All agents in the project | Shared decisions, conventions, API specs |
| `global` | All agents everywhere | Cross-project standards, org-wide rules |

## Store knowledge

```
set_memory({
  key: "auth-header-format",
  value: "Bearer <jwt> in Authorization header, no prefix variations",
  scope: "project",
  tags: ["auth", "api"],
  confidence: "stated",    // "stated", "inferred", or "observed"
  layer: "constraints"     // "constraints", "behavior", or "context"
})
```

### Layers

| Layer | Meaning |
|---|---|
| `constraints` | Hard rules — never override |
| `behavior` | Defaults — can adapt per situation |
| `context` | Ephemeral — session-specific |

## Retrieve knowledge

Cascade lookup (agent -> project -> global):

```
get_memory({ key: "auth-header-format" })
```

Specific scope only:

```
get_memory({ key: "auth-header-format", scope: "project" })
```

## Search

Full-text search with optional tag filters:

```
search_memory({
  query: "authentication patterns",
  tags: ["auth"],
  limit: 20
})
```

## Browse

```
list_memories({ scope: "project", tags: ["api"], limit: 50 })
```

## Conflicts

When two agents write different values for the same key at the same scope, a **conflict** is flagged. Both versions are preserved. `get_memory` returns all conflicting values with provenance.

Resolve by picking a winner (or writing a new value):

```
resolve_conflict({
  key: "db-schema-version",
  chosen_value: "v3 with users.email unique constraint",
  scope: "project"
})
```

The rejected version is archived with resolution metadata.

## Delete

Soft-delete (archived, never hard-deleted):

```
delete_memory({ key: "old-convention", scope: "project" })
```

## RAG: query_context

Load ranked context before starting work:

```
query_context({
  query: "supabase migration patterns",
  limit: 10
})
```

Returns ranked memories + completed task results matching the query. Use this at boot or before a complex task.
