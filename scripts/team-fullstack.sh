#!/usr/bin/env bash
# Fullstack team demo — 4 agents with hierarchy building a real taskboard app.
#
# Team:
#   cto (executive) — dispatches, coordinates
#   ├── backend-dev    → Go REST API in backend/
#   ├── frontend-dev   → index.html + JS + CSS in frontend/
#   └── infra-dev      → Dockerfile + Makefile + run.sh at repo root
#
# App: "taskboard" — a tiny kanban-style app.
#   Backend:  POST /tasks, GET /tasks, PATCH /tasks/:id, DELETE /tasks/:id (in-memory)
#   Frontend: index.html fetches /tasks, renders cards with inline status edit
#   Infra:    Dockerfile, Makefile (build/run/stop), README.md with quickstart
#
# All 3 devs are dispatched in parallel, each triggers an auto-spawn,
# each works in its own subdir — no file conflicts.
#
# Watch it live: http://localhost:8090 → Galaxy → click 'fullstack-team'
#
# Usage: ./scripts/team-fullstack.sh            — full run
#        ./scripts/team-fullstack.sh cleanup    — drop project + wipe workspace

set -euo pipefail

RELAY="${RELAY:-http://localhost:8090}"
PROJECT="${PROJECT:-fullstack-team}"
DB="${DB:-$HOME/.agent-relay/relay.db}"
WORKSPACE="${WORKSPACE:-/tmp/taskboard}"
MODE="${1:-full}"

mcp() {
  curl -sS -X POST "$RELAY/mcp?project=$PROJECT" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "$1"
}

rest_post() {
  curl -sS -X POST "$RELAY$1" -H "Content-Type: application/json" -d "$2"
}

section() {
  echo
  echo "━━━ $1 ━━━"
}

# --- cleanup ----------------------------------------------------------------
if [ "$MODE" = "cleanup" ]; then
  section "Cleanup"
  sqlite3 "$DB" "SELECT id FROM spawn_children WHERE status='running' AND project='$PROJECT';" | while read -r id; do
    [ -n "$id" ] && curl -sS -X POST "$RELAY/api/spawn/children/$id/kill" >/dev/null 2>&1 || true
  done
  pkill -f "$WORKSPACE/backend/taskboard" 2>/dev/null || true
  for t in triggers trigger_history messages deliveries agents profiles tasks goals boards spawn_children memories team_members teams; do
    sqlite3 "$DB" "DELETE FROM $t WHERE project='$PROJECT';" 2>/dev/null || true
  done
  sqlite3 "$DB" "DELETE FROM projects WHERE name='$PROJECT';"
  rm -rf "$WORKSPACE"
  echo "  done"
  exit 0
fi

# --- setup ------------------------------------------------------------------

section "1. Workspace + project"
rm -rf "$WORKSPACE"
mkdir -p "$WORKSPACE/backend" "$WORKSPACE/frontend"
echo "  workspace: $WORKSPACE"

mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_project","arguments":{"name":"'"$PROJECT"'","description":"Fullstack taskboard built by autonomous team"}}}' >/dev/null
echo "  project: $PROJECT"

# --- CTO ---
mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_agent","arguments":{"name":"cto","project":"'"$PROJECT"'","is_executive":true,"role":"CTO — plans and dispatches"}}}' >/dev/null
echo "  agent: cto (executive, top of hierarchy)"

# --- profiles ---
section "2. Register 3 profiles"

register_profile() {
  local slug="$1"
  local ctx="$2"
  local tools="$3"
  local ctx_esc
  ctx_esc=$(python3 -c "import json,sys; print(json.dumps(sys.stdin.read()))" <<< "$ctx")
  mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_profile","arguments":{"project":"'"$PROJECT"'","slug":"'"$slug"'","name":"'"$slug"'","context_pack":'"$ctx_esc"',"allowed_tools":"'"$tools"'"}}}' >/dev/null
  echo "  profile: $slug"
}

BACKEND_CTX=$(cat <<EOF
You are the BACKEND-DEV of the fullstack team. Reports to CTO.

Task: build a Go REST API for a taskboard at $WORKSPACE/backend/

API spec (stdlib only, no deps):
  POST /tasks         {"title","status"}           → 201 + {"id","title","status","created_at"}
  GET  /tasks                                      → 200 + [...tasks]
  PATCH /tasks/:id    {"title"?,"status"?}         → 200 + updated task
  DELETE /tasks/:id                                → 204
  Static files under /: serve $WORKSPACE/frontend if the file exists

Listen on :8788. In-memory store with sync.Mutex. UUIDs for IDs.
Add CORS headers (Access-Control-Allow-Origin: *) so the frontend can fetch.

Files to write:
  $WORKSPACE/backend/go.mod        (module taskboard, go 1.21)
  $WORKSPACE/backend/main.go       (one file, ~150 lines)

For each task dispatched to you:
1. claim_task → start_task
2. Write the files via Write tool
3. complete_task with result='wrote backend — main.go + go.mod'
4. Exit

Do NOT run go build — infra-dev will handle that.
EOF
)

FRONTEND_CTX=$(cat <<EOF
You are the FRONTEND-DEV of the fullstack team. Reports to CTO.

Task: build a plain HTML + vanilla JS UI for the taskboard at $WORKSPACE/frontend/

Spec:
  Single index.html with 3 columns (todo/doing/done)
  A form at top: input for title + select for status + submit button
  On load: fetch('/tasks') from same origin, render cards
  Card actions: click "done" button to PATCH status, click "x" to DELETE
  Minimal CSS, dark theme, no frameworks

Files to write:
  $WORKSPACE/frontend/index.html
  $WORKSPACE/frontend/app.js
  $WORKSPACE/frontend/styles.css

API endpoints to use (same-origin, backend serves frontend):
  POST /tasks {"title":"...","status":"todo"}
  GET  /tasks
  PATCH /tasks/:id {"status":"..."}
  DELETE /tasks/:id

For each task dispatched:
1. claim_task → start_task
2. Write the 3 files via Write tool
3. complete_task with result='wrote frontend — index.html + app.js + styles.css'
4. Exit

Keep it clean and working, not fancy.
EOF
)

INFRA_CTX=$(cat <<EOF
You are the INFRA-DEV of the fullstack team. Reports to CTO.

Task: package the taskboard app at $WORKSPACE/

Files to write:
  $WORKSPACE/Makefile        — targets: build, run, stop, clean, test
                               build: cd backend && go build -o taskboard .
                               run: cd backend && ./taskboard &
                               stop: pkill -f backend/taskboard
                               test: curl POST /tasks then GET /tasks
  $WORKSPACE/Dockerfile      — multi-stage: golang:1.21 build, alpine runtime,
                               copy backend binary + frontend static files,
                               EXPOSE 8788, CMD ["./taskboard"]
  $WORKSPACE/README.md       — ~30 lines:
                               - Title + description
                               - Architecture (backend Go, frontend vanilla JS, port 8788)
                               - Quickstart (make build && make run, curl examples)
                               - Project structure tree (backend/, frontend/)

For each task dispatched:
1. claim_task → start_task
2. Write the 3 files via Write tool
3. complete_task with result='wrote infra — Makefile + Dockerfile + README'
4. Exit

You do NOT need to run commands, just write the files. The CTO will run the
Makefile at the end.
EOF
)

register_profile "backend-dev"  "$BACKEND_CTX"  '[\"Write\",\"Read\",\"Edit\",\"mcp__agent-relay__*\"]'
register_profile "frontend-dev" "$FRONTEND_CTX" '[\"Write\",\"Read\",\"Edit\",\"mcp__agent-relay__*\"]'
register_profile "infra-dev"    "$INFRA_CTX"    '[\"Write\",\"Read\",\"Edit\",\"mcp__agent-relay__*\"]'

# --- hierarchy: reports_to ---
section "3. Hierarchy — 3 devs report to CTO"

for slug in backend-dev frontend-dev infra-dev; do
  # Pre-register with reports_to so they appear under cto in the org tree.
  # Will be auto-replaced when the spawned child actually registers.
  mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_agent","arguments":{"name":"'"$slug"'","project":"'"$PROJECT"'","profile_slug":"'"$slug"'","reports_to":"cto","role":"'"$slug"'"}}}' >/dev/null
  echo "  $slug reports_to cto"
done

# --- triggers ---
section "4. Triggers: one per profile (parallel dispatch)"

for slug in backend-dev frontend-dev infra-dev; do
  TR=$(rest_post /api/triggers '{
    "project":"'"$PROJECT"'",
    "event":"task.dispatched",
    "match_rules":"{\"profile\":\"'"$slug"'\"}",
    "profile_slug":"'"$slug"'",
    "cycle":"respond",
    "cooldown_seconds":0,
    "max_duration":"5m"
  }' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
  echo "  task.dispatched (profile=$slug) → spawn $slug   ${TR:0:8}"
done

# --- browser hint ---
section "5. ▶▶▶  OPEN http://localhost:8090 IN YOUR BROWSER"
echo
echo "  Galaxy → click 'fullstack-team' planet → Colony view"
echo "  You'll see: cto (crown) + 3 dev sprites linked by hierarchy arcs"
echo "  Watch: Kanban column 'pending' → 'in-progress' → 'done' for each task"
echo "         Message orbs animating between agents + relay-os"
echo "         Token widget counting up"
echo
echo "  Starting dispatch in 12s..."
for i in 12 11 10 9 8 7 6 5 4 3 2 1; do
  printf "\r  T-minus %2ds " "$i"
  sleep 1
done
echo

# --- dispatch 3 parallel tasks ---
section "6. CTO dispatches 3 parallel tasks"

dispatch_to() {
  local slug="$1" title="$2" desc="$3"
  local id
  id=$(SLUG="$slug" TITLE="$title" DESC="$desc" PROJ="$PROJECT" python3 -c '
import json, os, urllib.request
args = {"project": os.environ["PROJ"], "as": "cto", "profile": os.environ["SLUG"],
        "title": os.environ["TITLE"], "description": os.environ["DESC"], "priority": "P1"}
body = {"jsonrpc":"2.0","id":1,"method":"tools/call",
        "params":{"name":"dispatch_task","arguments":args}}
req = urllib.request.Request(
    "'"$RELAY"'/mcp?project=" + os.environ["PROJ"],
    data=json.dumps(body).encode(),
    headers={"Content-Type":"application/json","Accept":"application/json, text/event-stream"}
)
raw = urllib.request.urlopen(req).read().decode()
start = raw.find("{")
obj = json.loads(raw[start:])
t = obj["result"]["content"][0]["text"]
print(json.loads(t)["task"]["id"])')
  echo "  → $slug: $title   id=${id:0:8}"
  echo "$id"
}

BACK_ID=$(dispatch_to "backend-dev"  "build-backend"  "Build the Go REST API for the taskboard per your profile spec. Write go.mod + main.go in $WORKSPACE/backend/. Listen on :8788, stdlib only, sync.Mutex store, CORS headers, UUIDs." | tail -1)
FRONT_ID=$(dispatch_to "frontend-dev" "build-frontend" "Build the vanilla HTML/JS UI per your profile spec. Write index.html + app.js + styles.css in $WORKSPACE/frontend/. Fetch /tasks from same origin, 3 columns kanban, add/update/delete tasks." | tail -1)
INFRA_ID=$(dispatch_to "infra-dev"    "build-infra"    "Write Makefile + Dockerfile + README.md at $WORKSPACE/ per your profile spec. Makefile: build/run/stop/clean/test targets. Dockerfile: multi-stage. README: quickstart + structure." | tail -1)

# --- watch ---
section "7. Watching the pipeline (up to 8 min for 3 parallel spawns)"

sleep 2
DEADLINE=$(($(date +%s) + 480))
LAST=0
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  NOW=$(date +%s)
  DONE=$(sqlite3 "$DB" "SELECT count(*) FROM tasks WHERE project='$PROJECT' AND status='done';")
  RUNNING=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT' AND status='running';")
  TOTAL=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT';")
  if [ "$((NOW - LAST))" -ge 8 ]; then
    BACK_FILES=$(ls "$WORKSPACE/backend" 2>/dev/null | wc -l | tr -d ' ')
    FRONT_FILES=$(ls "$WORKSPACE/frontend" 2>/dev/null | wc -l | tr -d ' ')
    ROOT_FILES=$(ls "$WORKSPACE" 2>/dev/null | grep -v '^backend$\|^frontend$' | wc -l | tr -d ' ')
    echo "  t=$((NOW - (DEADLINE - 480)))s  tasks_done=$DONE/3  children=$RUNNING/$TOTAL  files: backend=$BACK_FILES frontend=$FRONT_FILES root=$ROOT_FILES"
    LAST=$NOW
  fi
  if [ "$DONE" = "3" ] && [ "$RUNNING" = "0" ]; then break; fi
  sleep 4
done

# --- result ---
section "8. Workspace structure"
if command -v tree >/dev/null; then
  tree "$WORKSPACE" -L 2
else
  ls -la "$WORKSPACE" | tail -n +2
  for d in backend frontend; do
    echo "  $d/:"
    ls -la "$WORKSPACE/$d" 2>/dev/null | tail -n +2 | awk '{print "    " $9, "(" $5 "b)"}'
  done
fi

section "9. CTO runs the build (as the final integration step)"

if [ -f "$WORKSPACE/Makefile" ]; then
  echo "  make build:"
  export PATH="/opt/homebrew/bin:$PATH"
  (cd "$WORKSPACE" && make build 2>&1 | head -10) || echo "  build FAILED"
  if [ -f "$WORKSPACE/backend/taskboard" ]; then
    echo "  binary: $(ls -la "$WORKSPACE/backend/taskboard" | awk '{print $5}') bytes"

    echo
    echo "  integration test: start server + curl POST/GET/DELETE"
    (cd "$WORKSPACE/backend" && ./taskboard &) 2>/dev/null
    until curl -sS http://localhost:8788/tasks >/dev/null 2>&1; do sleep 0.3; done
    echo "    POST /tasks:"
    CREATE=$(curl -sS -X POST http://localhost:8788/tasks -H "Content-Type: application/json" -d '{"title":"integration test","status":"todo"}')
    echo "      $CREATE"
    TID=$(echo "$CREATE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
    echo "    GET /tasks:"
    curl -sS http://localhost:8788/tasks | python3 -c "import sys,json; print('      count=' + str(len(json.load(sys.stdin))))"
    if [ -n "$TID" ]; then
      echo "    DELETE /tasks/$TID:"
      curl -sS -o /dev/null -w "      HTTP %{http_code}\n" -X DELETE "http://localhost:8788/tasks/$TID"
    fi
    pkill -f "$WORKSPACE/backend/taskboard" 2>/dev/null || true
    sleep 1
  fi
fi

# --- telemetry ---
section "10. Telemetry"
echo
echo "  tasks:"
sqlite3 "$DB" "SELECT title, status, assigned_to, substr(COALESCE(result,''),1,50) FROM tasks WHERE project='$PROJECT' ORDER BY dispatched_at;" | awk -F'|' '{printf "    %-18s %-8s %-35s %s\n", $1, $2, $3, $4}'

echo
echo "  children:"
sqlite3 "$DB" "SELECT substr(id,1,8), profile, status, exit_code FROM spawn_children WHERE project='$PROJECT' ORDER BY started_at;" | awk -F'|' '{printf "    %s  %-14s %-10s exit=%s\n", $1, $2, $3, $4}'

echo
echo "  cycle tokens:"
curl -sS "$RELAY/api/cycle-history?project=$PROJECT" | python3 -c "
import sys, json
for e in json.load(sys.stdin):
  if 'spawn' in e.get('cycle_name',''):
    print(f'    {e[\"cycle_name\"]:<26} {e[\"duration_ms\"]}ms  out={e[\"output_tokens\"]}  cache_read={e[\"cache_read_tokens\"]}')"

echo
echo "  org tree:"
curl -sS "$RELAY/api/org?project=$PROJECT" | python3 -c "
import sys, json
def walk(a, d=0):
  print('    ' + '  '*d + a['name'] + (' (exec)' if a.get('is_executive') else '') + '  ' + a.get('role',''))
  for r in a.get('reports', []): walk(r, d+1)
for a in json.load(sys.stdin): walk(a)"

echo
echo "━━━ done ━━━"
echo "Explore: $WORKSPACE"
echo "Run manually: cd $WORKSPACE && make run  (then open http://localhost:8788)"
echo "Cleanup: ./scripts/team-fullstack.sh cleanup"
