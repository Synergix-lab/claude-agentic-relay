# Changelog

All notable changes to wrai.th are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Versions follow [Semantic Versioning](https://semver.org/).

## [0.5.0] — 2026-03-11

### Added
- **`/health` REST endpoint** — uptime, version, DB row counts for monitoring
- **`move_task` MCP tool** — move task to different board/goal with prefix resolution
- **`batch_complete_tasks` MCP tool** — complete multiple tasks in one call
- **`batch_dispatch_tasks` MCP tool** — dispatch multiple tasks in one call
- **`list_tasks` filters** — `status: "active"` excludes done/cancelled, `include_archived` boolean
- **Auto-notifications** — dispatching a task sends an inbox message to agents running the target profile
- **Inline checklist** — interactive checkboxes on kanban cards, toggle items without opening the edit form

### Changed
- Default message TTL raised from 1h to **4h** (14400s)

### Fixed
- `GetAgentTasks` and `GetUnackedTasks` now exclude archived tasks
- `apiError` uses `json.Marshal` for proper JSON escaping
- Board dropdown in edit form showed empty names (`b.title` → `b.name`)
- `assigned_to` field value was never saved in edit form

## [0.4.0] — 2026-03-10

### Added
- **Kanban board** — full task management UI with drag-and-drop, board/goal columns, edit form with checklist and larger textarea
- **Pixel holo UI assets** — 9-slice panels, buttons, dividers, loading wheel, icon sets
- **Cascade delete** — deleting a project removes all related agents, tasks, messages, memories, boards, goals, conversations

### Fixed
- Vault `indexFile` errors now logged instead of silently swallowed during full reindex
- Duplicate `ZOOM_STEPS` and scale-btn declarations removed
- CRLF line endings in installer scripts
- Installer wrapped in block for `curl | sh` pipe compatibility
- Nil pointer in `DB.Close()` when opened read-only
- Release workflow made idempotent with `--clobber` uploads

## [0.3.0] — 2026-03-09

### Added
- **`create_project` MCP tool** — one-command colony setup with 8-phase onboarding prompt (CTO + adaptive worker profiles, auto/interactive modes, sprint planning)
- **`agent-relay update` CLI command** — self-update via GitHub Releases API (source build or prebuilt binary, launchd/systemd restart, `--force` flag)
- **Smart Messaging** — priority-based routing, conversations (group chats), delivery tracking, SSE real-time stream
- **Context budget pruning** — `get_inbox({ apply_budget: true })` scores messages by `0.7×priority + 0.2×tagRelevance + 0.1×freshness` and greedily selects the highest-value subset that fits the agent's byte budget. P0 messages always bypass the budget
- **Message orbs** — animated projectiles between agents on canvas (team, direct, broadcast)
- **Cancel button** on task notification cards — founder can reject agent tasks directly
- **Markdown rendering** in task notification cards (via marked.js)
- **Zoom controls** — +/- buttons and keyboard shortcuts for UI font scaling (localStorage persistent)
- **install.sh dependency audit** — checks curl (required), go, cc, git, jq, python3 with clear warnings
- **`.mcp.json` protection** — backup before merge, never overwrite existing config
- Auto-normalize JSON keys to snake_case
- Comprehensive MCP handler and REST API test coverage
- Reverse proxy docs, TLS troubleshooting, platform notes

### Fixed
- **Human task regression** — agents dispatching to `"human"` profile now trigger notification cards, kanban highlights, and My Tasks filter
- Repo URL corrected from `claude-agentic-relay` to `WRAI.TH` across all files
- Hook scripts guard for jq availability

### Changed
- `list_tasks` truncates descriptions/results to 200 chars (~70% token savings)
- `cancelled` status added to REST task transition endpoint

### Performance
- SQLite optimizations for concurrent agent workloads (WAL, busy timeout, connection pooling)

## [0.2.1] — 2026-03-08

### Changed
- Redesigned colony agent selection with Civilization-style macro→micro navigation

## [0.2.0] — 2026-03-08

### Added
- Opt-in authentication, CORS, rate limiting, body size limits

### Changed
- License switched from MIT to AGPL-3.0

## [0.1.1] — 2026-03-08

Initial public release — MCP relay server with SQLite persistence, canvas UI, pixel art galaxy/colony views, vault indexing, CI/CD with cross-platform binary builds.

[0.5.0]: https://github.com/Synergix-lab/WRAI.TH/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Synergix-lab/WRAI.TH/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Synergix-lab/WRAI.TH/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/Synergix-lab/WRAI.TH/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Synergix-lab/WRAI.TH/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/Synergix-lab/WRAI.TH/releases/tag/v0.1.1
