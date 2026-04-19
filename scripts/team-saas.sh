#!/usr/bin/env bash
# Full SaaS team — autonomous product discovery + design + build + ship.
#
# Team hierarchy (all reports_to CEO, since CEO is the sole executive):
#   ceo  →  product-mgr  →  designer  →  [backend-dev, frontend-dev, infra-dev]  →  qa  →  ceo (verdict)
#
# Cascade is driven by the exit_prompt on each profile: when an agent finishes,
# its exit_prompt tells it to dispatch_task for the next profile BEFORE exit.
# No manual re-dispatch from this script.
#
# Usage: ./scripts/team-saas.sh          — full run
#        ./scripts/team-saas.sh cleanup  — drop project + kill children + wipe workspace

set -euo pipefail

RELAY="${RELAY:-http://localhost:8090}"
PROJECT="${PROJECT:-saas-team}"
DB="${DB:-$HOME/.agent-relay/relay.db}"
WORKSPACE="${WORKSPACE:-/Users/loic/Projects/agent-relay/demo-saas}"
MODE="${1:-full}"

rest_post() { curl -sS -X POST "$RELAY$1" -H "Content-Type: application/json" -d "$2"; }
rest_get()  { curl -sS "$RELAY$1"; }
section()   { echo; echo "━━━ $1 ━━━"; }

# --- cleanup ----------------------------------------------------------------
if [ "$MODE" = "cleanup" ]; then
  section "Cleanup"
  sqlite3 "$DB" "SELECT id FROM spawn_children WHERE status='running' AND project='$PROJECT';" | while read -r id; do
    [ -n "$id" ] && curl -sS -X POST "$RELAY/api/spawn/children/$id/kill" >/dev/null 2>&1 || true
  done
  pkill -f "$WORKSPACE/backend" 2>/dev/null || true
  for t in triggers trigger_history messages deliveries agents profiles tasks goals boards spawn_children memories team_members teams; do
    sqlite3 "$DB" "DELETE FROM $t WHERE project='$PROJECT';" 2>/dev/null || true
  done
  sqlite3 "$DB" "DELETE FROM projects WHERE name='$PROJECT';"
  rm -rf "$WORKSPACE"
  echo "  done"
  exit 0
fi

# --- setup ------------------------------------------------------------------

section "1. Workspace + project + CEO"
rm -rf "$WORKSPACE"
mkdir -p "$WORKSPACE/backend" "$WORKSPACE/frontend"
echo "  workspace: $WORKSPACE"

curl -sS -X POST "$RELAY/mcp?project=$PROJECT" \
  -H "Content-Type: application/json" -H "Accept: application/json, text/event-stream" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"create_project\",\"arguments\":{\"name\":\"$PROJECT\"}}}" >/dev/null

curl -sS -X POST "$RELAY/mcp?project=$PROJECT" \
  -H "Content-Type: application/json" -H "Accept: application/json, text/event-stream" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"register_agent\",\"arguments\":{\"name\":\"ceo\",\"project\":\"$PROJECT\",\"is_executive\":true,\"role\":\"CEO — kicks off, receives final report\"}}}" >/dev/null
echo "  agent: ceo (sole executive → everyone auto-reports)"

# --- profiles registered via a single Python helper so we avoid bash heredoc
# escaping pain (the exit_prompts mention shell syntax which fights heredoc)

section "2. Profiles with exit_prompt cascade"

PROJECT="$PROJECT" WORKSPACE="$WORKSPACE" RELAY="$RELAY" python3 <<'PY'
import os, json, urllib.request

RELAY = os.environ["RELAY"]
PROJECT = os.environ["PROJECT"]
WORKSPACE = os.environ["WORKSPACE"]

def mcp_call(tool, args):
    body = {"jsonrpc":"2.0","id":1,"method":"tools/call",
            "params":{"name": tool, "arguments": args}}
    req = urllib.request.Request(
        f"{RELAY}/mcp?project={PROJECT}",
        data=json.dumps(body).encode(),
        headers={"Content-Type":"application/json","Accept":"application/json, text/event-stream"})
    raw = urllib.request.urlopen(req, timeout=10).read().decode()
    start = raw.find("{")
    obj = json.loads(raw[start:])
    return obj

def register_profile(slug, name, ctx, exit_prompt, tools):
    r = mcp_call("register_profile", {
        "project": PROJECT, "slug": slug, "name": name,
        "role": name, "context_pack": ctx, "exit_prompt": exit_prompt,
        "allowed_tools": json.dumps(tools),
    })
    if "result" in r and not r["result"].get("isError"):
        print(f"  ✓ {slug}")
    else:
        print(f"  ✗ {slug}: {r}")

# -------------------------- product-mgr ----------------------------
product_ctx = f"""You are the PRODUCT MANAGER of a tiny startup.

Your one job RIGHT NOW: pick a micro-SaaS idea and write a crisp product spec.

Constraints on the idea:
- Solves a real, specific pain (not 'another TODO app')
- Implementable as: 1 Go backend + 1 static HTML page, under 300 LOC total
- No external services, no database beyond in-memory
- Quirky but useful

Write {WORKSPACE}/PRODUCT.md (~40-60 lines) with:
  # <Product Name>
  ## The problem
  ## The solution
  ## Target user (one persona)
  ## Core feature list (3-5 bullets)
  ## Non-features (what we explicitly do NOT build)
  ## Success metric (one number we track)
  ## API surface (endpoints, one-line each — the backend dev will implement these)

For each task dispatched:
1. claim_task then start_task
2. Write PRODUCT.md via the Write tool
3. complete_task with result stating the product name
4. Your exit_prompt cascades the next step.
"""

product_exit = f"""Before exit: dispatch the design task.
Call dispatch_task with: project={PROJECT!r}, as='product-mgr', profile='designer',
title='design', priority='P1',
description='Read {WORKSPACE}/PRODUCT.md and produce {WORKSPACE}/DESIGN.md with: UI components, user flow, data model (Go structs sketch), endpoint list with request/response shapes.'
Then set_memory and exit.
"""

register_profile("product-mgr", "Product Manager", product_ctx, product_exit,
                 ["Write","Read","mcp__agent-relay__*"])

# -------------------------- designer ----------------------------
designer_ctx = f"""You are the DESIGNER. Translate the product spec into a build brief.

When dispatched:
1. claim_task then start_task
2. Read {WORKSPACE}/PRODUCT.md
3. Write {WORKSPACE}/DESIGN.md (~50-80 lines) with:
   ## Data model (Go structs with fields + JSON tags)
   ## API endpoints (method + path + request body + response shape)
   ## UI layout (sections, components, user flow sketch)
   ## Styling direction (colors, vibe — one paragraph)
   ## Build constraints (port, stdlib only, file structure under {WORKSPACE})
4. complete_task with result='design written'
"""

designer_exit = f"""Before exit: dispatch 3 parallel build tasks.

Call dispatch_task THREE times with:
  1) profile='backend-dev', title='build-backend', priority='P1',
     description='Read {WORKSPACE}/PRODUCT.md and {WORKSPACE}/DESIGN.md. Implement the Go REST API at {WORKSPACE}/backend/main.go + go.mod. Stdlib only. Follow the design exactly.'
  2) profile='frontend-dev', title='build-frontend', priority='P1',
     description='Read {WORKSPACE}/DESIGN.md. Build index.html + app.js + styles.css at {WORKSPACE}/frontend/. Vanilla JS, fetch the API endpoints listed in the design.'
  3) profile='infra-dev', title='build-infra', priority='P1',
     description='Read {WORKSPACE}/DESIGN.md. Write {WORKSPACE}/Makefile (build, run, stop, clean, test), {WORKSPACE}/Dockerfile (multi-stage), {WORKSPACE}/README.md (quickstart).'

All with project={PROJECT!r}, as='designer'. Then set_memory, exit.
"""

register_profile("designer", "Designer", designer_ctx, designer_exit,
                 ["Write","Read","mcp__agent-relay__*"])

# -------------------------- backend-dev ----------------------------
backend_ctx = f"""You are the BACKEND-DEV.

When dispatched:
1. claim_task then start_task
2. Read {WORKSPACE}/PRODUCT.md and {WORKSPACE}/DESIGN.md
3. Write {WORKSPACE}/backend/go.mod (module based on product name, go 1.21)
4. Write {WORKSPACE}/backend/main.go implementing the API exactly as designed:
   - Stdlib only (no deps)
   - In-memory store with sync.Mutex
   - CORS headers (Access-Control-Allow-Origin: *)
   - Serve {WORKSPACE}/frontend/ static files from /
   - Listen on the port specified in DESIGN.md
   - Keep under 200 LOC
5. complete_task with result='backend written, listens on <port>'
"""

backend_exit = f"""Before exit, dispatch QA:
dispatch_task with project={PROJECT!r}, as='backend-dev', profile='qa',
title='qa-integration', priority='P1',
description='The 3 build tasks should be done. Run make build, start the server, curl the main endpoint, verify it responds. Send PASS/FAIL to CEO.'
Then set_memory and exit.

(QA trigger has 10s cooldown, so if frontend or infra also tries to dispatch qa, duplicates are harmless.)
"""

register_profile("backend-dev", "Backend Dev", backend_ctx, backend_exit,
                 ["Write","Read","Edit","mcp__agent-relay__*"])

# -------------------------- frontend-dev ----------------------------
frontend_ctx = f"""You are the FRONTEND-DEV.

When dispatched:
1. claim_task then start_task
2. Read {WORKSPACE}/DESIGN.md
3. Write 3 files at {WORKSPACE}/frontend/:
   - index.html (semantic, minimal markup)
   - app.js (vanilla, fetch() to the API endpoints listed in the design)
   - styles.css (match the vibe described in DESIGN.md)
4. Keep it polished but minimal. No frameworks.
5. complete_task with result='frontend written'
"""

register_profile("frontend-dev", "Frontend Dev", frontend_ctx, "",
                 ["Write","Read","Edit","mcp__agent-relay__*"])

# -------------------------- infra-dev ----------------------------
infra_ctx = f"""You are the INFRA-DEV.

When dispatched:
1. claim_task then start_task
2. Read {WORKSPACE}/DESIGN.md for port and binary name
3. Write these files at {WORKSPACE}/:
   - Makefile with targets: build, run, stop, clean, test
     build: cd backend and go build -o <bin-name> .
     run:   cd backend and ./<bin-name> &
     stop:  pkill -f <bin-name>
     test:  curl the main endpoint to verify
   - Dockerfile: multi-stage golang:1.21 -> alpine, copy the binary + frontend static files, EXPOSE the port, CMD to run the binary
   - README.md (~30 lines): product name, what it does, quickstart using make, project structure tree
4. complete_task with result='infra written'
"""

register_profile("infra-dev", "Infra Dev", infra_ctx, "",
                 ["Write","Read","mcp__agent-relay__*"])

# -------------------------- qa ----------------------------
qa_ctx = f"""You are the QA engineer — final integration check.

When dispatched:
1. claim_task then start_task
2. Run Bash: cd {WORKSPACE} and make build, pipe to head -20. Check for errors.
3. If build failed: send P1 message to 'ceo' with subject='FAIL: build' and the error. complete_task with result='FAIL: build'. Exit.
4. If build OK:
   a. Run Bash: cd {WORKSPACE}/backend and run the binary in the background (binary name from DESIGN.md)
   b. Wait 1 second (sleep 1)
   c. Run Bash: curl against the main endpoint (from DESIGN.md). Capture the response.
   d. Run Bash: pkill -f the binary to stop the server
5. Write {WORKSPACE}/QA_REPORT.md with: commands run, responses captured, PASS/FAIL verdict.
6. Send P1 message to 'ceo' with subject='PASS' (or 'FAIL: <reason>') and a 1-sentence summary.
7. complete_task with the verdict string.
"""

register_profile("qa", "QA", qa_ctx, "",
                 ["Bash","Read","Write","mcp__agent-relay__*"])

print("  (all 6 profiles registered)")
PY

# --- triggers ---------------------------------------------------------------

section "3. Triggers (one per profile, match on profile in task.dispatched meta)"

for slug in product-mgr designer backend-dev frontend-dev infra-dev qa; do
  rest_post /api/triggers "{\"project\":\"$PROJECT\",\"event\":\"task.dispatched\",\"match_rules\":\"{\\\"profile\\\":\\\"$slug\\\"}\",\"profile_slug\":\"$slug\",\"cycle\":\"respond\",\"cooldown_seconds\":10,\"max_duration\":\"5m\"}" >/dev/null
  echo "  ✓ task.dispatched {profile=$slug} → spawn $slug"
done

# --- browser hint + seed dispatch -------------------------------------------

section "4. ▶▶▶  OPEN http://localhost:8090 → colony '$PROJECT'"
echo "  Watch: ceo at top, then product-mgr → designer → 3 parallel → qa → ceo"
echo "  Starting in 8s..."
for i in 8 7 6 5 4 3 2 1; do printf "\r  T-minus %ds " "$i"; sleep 1; done
echo

section "5. CEO dispatches the initial 'find-idea' task"

PROJECT="$PROJECT" WORKSPACE="$WORKSPACE" RELAY="$RELAY" python3 <<'PY'
import os, json, urllib.request
RELAY = os.environ["RELAY"]
PROJECT = os.environ["PROJECT"]
WORKSPACE = os.environ["WORKSPACE"]
body = {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{
    "name":"dispatch_task","arguments":{
        "project": PROJECT, "as": "ceo", "profile": "product-mgr",
        "title": "find-idea", "priority": "P1",
        "description": f"Find a micro-SaaS idea to build today. Write the product spec at {WORKSPACE}/PRODUCT.md. Specific and useful. Your exit_prompt cascades the rest."
    }}}
req = urllib.request.Request(f"{RELAY}/mcp?project={PROJECT}",
    data=json.dumps(body).encode(),
    headers={"Content-Type":"application/json","Accept":"application/json, text/event-stream"})
raw = urllib.request.urlopen(req).read().decode()
obj = json.loads(raw[raw.find("{"):])
task = json.loads(obj["result"]["content"][0]["text"])
print(f"  dispatched find-idea → product-mgr   task={task['task']['id'][:8]}")
PY

# --- watch ------------------------------------------------------------------

section "6. Watching the cascade (up to 12 min)"

# Relax set -e for the watch loop — transient sqlite3 reads (DB locked, empty
# result) shouldn't abort the whole pipeline.
set +e

DEADLINE=$(($(date +%s) + 720))
LAST=0
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  NOW=$(date +%s)
  TASKS_DONE=$(sqlite3 "$DB" "SELECT count(*) FROM tasks WHERE project='$PROJECT' AND status='done';" 2>/dev/null)
  TASKS_DONE=${TASKS_DONE:-0}
  TASKS_TOTAL=$(sqlite3 "$DB" "SELECT count(*) FROM tasks WHERE project='$PROJECT';" 2>/dev/null)
  TASKS_TOTAL=${TASKS_TOTAL:-0}
  RUNNING=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT' AND status='running';" 2>/dev/null)
  RUNNING=${RUNNING:-0}
  SPAWNS=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT';" 2>/dev/null)
  SPAWNS=${SPAWNS:-0}
  if [ "$((NOW - LAST))" -ge 12 ]; then
    CURRENT=$(sqlite3 "$DB" "SELECT profile_slug FROM tasks WHERE project='$PROJECT' AND status IN ('pending','in-progress','accepted') ORDER BY dispatched_at DESC LIMIT 1;" 2>/dev/null)
    FILES_ROOT=$(ls "$WORKSPACE"/*.md 2>/dev/null | wc -l | tr -d ' ')
    FILES_BACK=$(ls "$WORKSPACE/backend" 2>/dev/null | wc -l | tr -d ' ')
    FILES_FRONT=$(ls "$WORKSPACE/frontend" 2>/dev/null | wc -l | tr -d ' ')
    echo "  t=$((NOW - (DEADLINE - 720)))s  tasks=$TASKS_DONE/$TASKS_TOTAL  children=$RUNNING/$SPAWNS  working_on=${CURRENT:-idle}  files=md:$FILES_ROOT back:$FILES_BACK front:$FILES_FRONT"
    LAST=$NOW
  fi
  CEO_VERDICT=$(sqlite3 "$DB" "SELECT count(*) FROM messages WHERE project='$PROJECT' AND to_agent='ceo' AND (subject LIKE 'PASS%' OR subject LIKE 'FAIL%') AND created_at > datetime('now','-20 minutes');" 2>/dev/null)
  CEO_VERDICT=${CEO_VERDICT:-0}
  if [ "$CEO_VERDICT" -ge 1 ] && [ "$RUNNING" = "0" ]; then
    echo "  ✓ CEO received QA verdict — pipeline complete"
    break
  fi
  sleep 5
done

set -e

# --- result -----------------------------------------------------------------

section "7. Workspace structure"
if command -v tree >/dev/null; then tree "$WORKSPACE" -L 2 2>/dev/null; fi
ls -la "$WORKSPACE" | tail -n +2
for d in backend frontend; do
  echo "  $d/:"
  ls -la "$WORKSPACE/$d" 2>/dev/null | tail -n +2 | awk '{printf "    %s (%s b)\n", $9, $5}'
done

section "8. The idea (PRODUCT.md — first 30 lines)"
if [ -f "$WORKSPACE/PRODUCT.md" ]; then head -30 "$WORKSPACE/PRODUCT.md"; else echo "  (missing)"; fi

section "9. Timeline"
echo
echo "  tasks:"
sqlite3 "$DB" "SELECT substr(dispatched_at,12,8), title, profile_slug, status, COALESCE(assigned_to,'-') FROM tasks WHERE project='$PROJECT' ORDER BY dispatched_at;" | awk -F'|' '{printf "    %s  %-18s %-14s %-12s by=%s\n", $1, $2, $3, $4, $5}'
echo
echo "  messages (last 15):"
sqlite3 "$DB" "SELECT substr(created_at,12,8), from_agent, to_agent, priority, substr(subject,1,30) FROM messages WHERE project='$PROJECT' ORDER BY created_at DESC LIMIT 15;" | tail -r | awk -F'|' '{printf "    %s  %-27s → %-15s [%s] %s\n", $1, $2, $3, $4, $5}'
echo
echo "  spawns:"
sqlite3 "$DB" "SELECT substr(id,1,8), profile, status, exit_code, substr(started_at,12,8), substr(COALESCE(finished_at,''),12,8) FROM spawn_children WHERE project='$PROJECT' ORDER BY started_at;" | awk -F'|' '{printf "    %s  %-13s  %-10s exit=%s  %s → %s\n", $1, $2, $3, $4, $5, $6}'
echo
echo "  token totals:"
rest_get "/api/cycle-history?project=$PROJECT" | python3 -c "
import sys, json
tot_out = tot_cache = 0
entries = []
for e in json.load(sys.stdin):
  if 'spawn' in e.get('cycle_name',''):
    tot_out += e['output_tokens']
    tot_cache += e['cache_read_tokens']
    entries.append((e['cycle_name'], e['duration_ms'], e['output_tokens'], e['cache_read_tokens']))
for name, dur, out, cache in entries:
  print(f'    {name:<26} {dur:>6}ms  out={out:>5}  cache={cache}')
print(f'    {\"TOTAL\":<26} ---  out={tot_out}  cache={tot_cache}')"
echo
echo "  org tree:"
rest_get "/api/org?project=$PROJECT" | python3 -c "
import sys, json
def walk(a, d=0):
  print('    '+'  '*d+a['name']+('*' if a.get('is_executive') else ''))
  for r in a.get('reports',[]): walk(r, d+1)
for a in json.load(sys.stdin): walk(a)"

section "10. Can we run it?"
export PATH="/opt/homebrew/bin:$PATH"
if [ -f "$WORKSPACE/Makefile" ] && [ -f "$WORKSPACE/backend/main.go" ]; then
  echo "  make build:"
  (cd "$WORKSPACE" && make build 2>&1 | head -8) || echo "    build failed"
  [ -f "$WORKSPACE/QA_REPORT.md" ] && { echo; echo "  QA_REPORT.md (first 25 lines):"; head -25 "$WORKSPACE/QA_REPORT.md" | awk '{print "    " $0}'; }
fi

pkill -f "$WORKSPACE/backend/" 2>/dev/null || true
sqlite3 "$DB" "SELECT id FROM spawn_children WHERE status='running';" | while read -r id; do
  [ -n "$id" ] && curl -sS -X POST "$RELAY/api/spawn/children/$id/kill" >/dev/null 2>&1
done

echo
echo "━━━ done ━━━"
echo "Workspace: $WORKSPACE   |   Cleanup: ./scripts/team-saas.sh cleanup"
