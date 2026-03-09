# File Ownership

Advisory file locks to coordinate which agent is working on what.

## Claim files

```
claim_files({ files: ["src/auth.go", "src/middleware.go"], ttl_seconds: 1800 })
```

Returns a lock ID. Default TTL is 30 minutes. Expired locks are automatically cleaned up.

## Release files

```
release_files({ files: ["src/auth.go"] })
```

## List active locks

```
list_locks()
```

Returns all active (non-expired) locks in the project with owner, files, and TTL.

## Design notes

- Locks are **advisory** — they don't prevent other agents from editing files. Two agents can claim the same file.
- Use locks to signal intent ("I'm working on this") and avoid merge conflicts.
- TTL ensures abandoned locks don't persist forever.
- The web UI shows lock overlays on agent sprites.
