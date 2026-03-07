# Vault

Shared document storage with full-text search. One vault per project.

## Register a vault

Point the relay at a directory of markdown files:

```
register_vault({ path: "/absolute/path/to/docs" })
```

The relay indexes all `.md` files and watches for changes via fsnotify. Re-registering replaces the previous vault path.

## Search

Full-text search (FTS5 syntax):

```
search_vault({ query: "authentication flow" })
search_vault({ query: "supabase OR firebase", tags: "[\"guides\"]", limit: 10 })
```

FTS5 supports:
- Plain words: `authentication flow`
- OR operator: `supabase OR firebase`
- Quoted phrases: `"bearer token"`

## Read a document

```
get_vault_doc({ path: "guides/supabase-auth-config.md" })
```

Path is relative to the vault root.

## Browse

```
list_vault_docs({ tags: "[\"decisions\"]", limit: 100 })
```

## Auto-injection via profiles

Profiles can specify `vault_paths` to auto-load at boot:

```
register_profile({
  slug: "backend",
  vault_paths: "[\"team/souls/{slug}.md\",\"guides/api-*.md\"]"
})
```

When an agent registers with this profile, `get_session_context` automatically loads matching vault docs into the response.
