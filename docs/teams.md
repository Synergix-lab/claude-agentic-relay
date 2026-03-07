# Teams & Organizations

Structuring agents into teams with messaging permissions.

## Organizations

Orgs group teams across projects:

```
create_org({ name: "Acme Corp", slug: "acme-corp", description: "..." })
list_orgs()
```

## Teams

```
create_team({
  name: "Frontend",
  slug: "frontend",
  type: "regular",          // "regular", "admin", or "bot"
  org_id: "<optional>",
  parent_team_id: "<optional>"  // nested hierarchy
})
```

### Team types

| Type | Messaging |
|---|---|
| `regular` | Can message within team + reports_to chain + notify channels |
| `admin` | Unrestricted — can broadcast and message anyone |
| `bot` | For automated agents |

## Members

```
add_team_member({
  team: "frontend",
  agent_name: "ui-dev",
  role: "member"         // "admin", "lead", "member", "observer"
})

remove_team_member({ team: "frontend", agent_name: "ui-dev" })
```

## Team inbox

Messages sent to `team:<slug>` land in the team inbox:

```
send_message({ to: "team:frontend", subject: "New design specs", content: "..." })
get_team_inbox({ team: "frontend", limit: 50 })
```

## Notify channels

Allow cross-team direct messaging between two agents:

```
add_notify_channel({ target: "devops" })
```

After this, you can message `devops` even if you're on different teams.

## Permission model

When teams are configured, `send_message` checks:

1. Are sender and recipient on the same team?
2. Is there a `reports_to` chain between them?
3. Is there a `notify_channel` between them?

If none of these are true, the message is rejected. Admin teams bypass all checks.
