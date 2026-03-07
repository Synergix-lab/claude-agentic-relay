# Profiles

Reusable role definitions that persist across sessions.

## What a profile is

A profile defines an agent archetype: role, skills, personality (soul), and what docs to auto-load at boot. When an agent registers with `profile_slug: "backend"`, it receives the profile's context pack automatically.

## Register a profile

```
register_profile({
  slug: "backend",
  name: "Backend Developer",
  role: "FastAPI backend, database migrations, API design",
  context_pack: "You are a backend developer focused on clean API design...",
  skills: "[{\"id\":\"fastapi\",\"name\":\"FastAPI\",\"tags\":[\"python\",\"api\"]},{\"id\":\"postgres\",\"name\":\"PostgreSQL\",\"tags\":[\"database\"]}]",
  soul_keys: "[\"coding-style\",\"error-handling-convention\"]",
  vault_paths: "[\"team/souls/backend.md\",\"guides/api-*.md\"]"
})
```

### Fields

| Field | Purpose |
|---|---|
| `slug` | Unique ID, used in `register_agent` and `dispatch_task` |
| `name` | Display name |
| `role` | Role description |
| `context_pack` | Markdown blob: personality, working style, instructions |
| `skills` | JSON array of skill objects with tags — used by `find_profiles` |
| `soul_keys` | JSON array of memory keys to auto-load at boot |
| `vault_paths` | JSON array of vault doc paths to auto-inject. Supports globs. `{slug}` resolves to the profile slug |

## Retrieve profiles

```
get_profile({ slug: "backend" })
list_profiles()
```

## Find by skill

Find which profiles can handle a task:

```
find_profiles({ skill_tag: "database" })
```

Returns all profiles with a skill tagged `database`. Use this before `dispatch_task` to pick the right profile.

## How profiles connect to tasks

Tasks are dispatched to profiles, not agents:

```
dispatch_task({ profile: "backend", title: "Fix auth bug", ... })
```

Any agent running the `backend` profile can `claim_task` it.
