#!/usr/bin/env bash
# Team demo — autonomous agents collaborating to build a real mini-app.
#
# Scenario: a "docs-team" project with 3 doc-writer agents building README
# sections in parallel. The CTO dispatches 3 tasks; triggers auto-spawn a
# claude child per task; each child claim/start/write/complete autonomously;
# we observe the result on disk.
#
# Usage:
#   ./scripts/team-demo.sh                    # run the demo (spawns real claude children)
#   ./scripts/team-demo.sh cleanup            # drop project + kill children + wipe workspace
#
# Requires: relay running on :8090, claude CLI in PATH.

set -euo pipefail

RELAY="${RELAY:-http://localhost:8090}"
PROJECT="${PROJECT:-docs-team}"
DB="${DB:-$HOME/.agent-relay/relay.db}"
WORKSPACE="${WORKSPACE:-/tmp/docs-team-demo}"
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
  for t in triggers trigger_history messages deliveries agents profiles tasks goals boards spawn_children memories; do
    sqlite3 "$DB" "DELETE FROM $t WHERE project='$PROJECT';" 2>/dev/null || true
  done
  sqlite3 "$DB" "DELETE FROM projects WHERE name='$PROJECT';"
  rm -rf "$WORKSPACE"
  echo "  done — project dropped, workspace wiped"
  exit 0
fi

# --- setup ------------------------------------------------------------------

section "1. Workspace"
rm -rf "$WORKSPACE"
mkdir -p "$WORKSPACE"
echo "  $WORKSPACE"

section "2. Project + doc-writer profile + auto-spawn trigger"

mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"create_project\",\"arguments\":{\"name\":\"$PROJECT\"}}}" >/dev/null
echo "  project: $PROJECT"

# The profile prompt is the agent's soul — must be precise about what to do.
PROFILE_CONTEXT=$(cat <<EOF
You are a technical documentation writer in a multi-agent team.

For each task dispatched to you:
1. claim_task(task_id: <id>, as: <your name>, project: $PROJECT)
2. start_task(task_id: <id>, as: <your name>, project: $PROJECT)
3. Read the task description carefully — it tells you what to write and which file to write it to (an absolute path in $WORKSPACE)
4. Use the Write tool to write the markdown content to the exact file path specified
5. complete_task(task_id: <id>, as: <your name>, project: $PROJECT, result: "wrote <file>")
6. Exit immediately after complete_task returns

Rules:
- The content you write must be valid markdown, 5-10 lines, concrete and useful
- Do NOT write anything outside the requested file
- Do NOT create extra files
- Do NOT run Bash commands (you only need Write and the relay MCP tools)

Your output should be ONE Write call and ONE complete_task call. Nothing else.
EOF
)

PROFILE_CONTEXT_ESC=$(python3 -c "import json,sys; print(json.dumps(sys.stdin.read()))" <<< "$PROFILE_CONTEXT")

mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"register_profile\",\"arguments\":{\"project\":\"$PROJECT\",\"slug\":\"doc-writer\",\"name\":\"Doc Writer\",\"context_pack\":$PROFILE_CONTEXT_ESC,\"allowed_tools\":\"[\\\"Write\\\",\\\"Read\\\",\\\"mcp__agent-relay__*\\\"]\"}}}" >/dev/null
echo "  profile: doc-writer (tools: Write, Read, relay MCP)"

# Register a CTO agent (executive) who will dispatch
mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"register_agent\",\"arguments\":{\"name\":\"cto\",\"project\":\"$PROJECT\",\"is_executive\":true}}}" >/dev/null
echo "  agent: cto (executive)"

# Trigger: any task dispatched → spawn a doc-writer
TR=$(rest_post /api/triggers "{
  \"project\":\"$PROJECT\",
  \"event\":\"task.dispatched\",
  \"profile_slug\":\"doc-writer\",
  \"cycle\":\"respond\",
  \"cooldown_seconds\":0,
  \"max_duration\":\"3m\"
}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  trigger: task.dispatched → spawn doc-writer   id=${TR:0:8}"

# --- dispatch 3 tasks -------------------------------------------------------

section "3. CTO dispatches 3 parallel tasks"

declare -a TASK_IDS

dispatch_task() {
  # $1 = title, $2 = description
  local title="$1"
  local desc="$2"
  # Build the JSON payload via python to get reliable escaping of the description
  local payload
  payload=$(TITLE="$title" DESC="$desc" PROJ="$PROJECT" python3 -c '
import json, os
args = {"project": os.environ["PROJ"], "as": "cto", "profile": "doc-writer",
        "title": os.environ["TITLE"], "description": os.environ["DESC"], "priority": "P1"}
body = {"jsonrpc":"2.0","id":1,"method":"tools/call",
        "params":{"name":"dispatch_task","arguments":args}}
print(json.dumps(body))')
  local id
  id=$(mcp "$payload" | python3 -c "import sys,json; t=json.load(sys.stdin)['result']['content'][0]['text']; print(json.loads(t)['task']['id'])")
  echo "  dispatched: $title   id=${id:0:8}"
  TASK_IDS+=("$id")
}

dispatch_task \
  "write-features" \
  "Write the Features section of a README for a Go URL-shortener (mini project named 'shortly'). Output FILE: $WORKSPACE/01-features.md. Include 4-6 bullet points listing realistic features (POST /shorten, GET /:code redirects, in-memory storage, per-link expiry, QR code endpoint). Markdown only, start with '## Features'."

dispatch_task \
  "write-install" \
  "Write the Installation section of a README for shortly (Go URL-shortener). Output FILE: $WORKSPACE/02-install.md. Show curl-install and 'go install' commands, include prerequisites (Go 1.21+). Start with '## Installation'."

dispatch_task \
  "write-usage" \
  "Write the Usage section of a README for shortly. Output FILE: $WORKSPACE/03-usage.md. Include a concrete curl example for POST /shorten and GET /:code. Start with '## Usage'."

# --- observe ----------------------------------------------------------------

section "4. Observe: triggers fire → children spawn → tasks complete"

sleep 2
DEADLINE=$(($(date +%s) + 240))   # up to 4 min
LAST_REPORT=0
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  NOW=$(date +%s)
  DONE=$(sqlite3 "$DB" "SELECT count(*) FROM tasks WHERE project='$PROJECT' AND status='done';")
  RUNNING=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT' AND status='running';")
  TOTAL=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT';")
  FILES=$(ls "$WORKSPACE" 2>/dev/null | wc -l | tr -d ' ')

  if [ "$((NOW - LAST_REPORT))" -ge 5 ] || [ "$DONE" = "3" ]; then
    echo "  t=$((NOW - (DEADLINE - 240)))s   tasks_done=$DONE/3   children_running=$RUNNING/$TOTAL   files_written=$FILES"
    LAST_REPORT=$NOW
  fi

  if [ "$DONE" = "3" ] && [ "$RUNNING" = "0" ]; then break; fi
  sleep 3
done

# --- show result ------------------------------------------------------------

section "5. Result: workspace contents"
ls -la "$WORKSPACE" | tail -n +2
echo

for f in "$WORKSPACE"/*.md; do
  [ -e "$f" ] || continue
  echo "────── $(basename "$f") ──────"
  cat "$f"
  echo
done

section "6. Telemetry"

echo "  tasks:"
sqlite3 "$DB" "SELECT title, status, COALESCE(assigned_to,'-'), COALESCE(substr(result,1,40),'') FROM tasks WHERE project='$PROJECT' ORDER BY dispatched_at;" | awk -F'|' '{print "    "$1"   status="$2"   by="$3"   result="$4}'

echo
echo "  cycle history (tokens per spawn):"
curl -sS "$RELAY/api/cycle-history?project=$PROJECT" | python3 -c "
import sys, json
try:
  h = json.load(sys.stdin)
  for e in h:
    if 'spawn' in e.get('cycle_name',''):
      print(f'    {e[\"cycle_name\"]}   {e[\"duration_ms\"]}ms   in={e[\"input_tokens\"]}   out={e[\"output_tokens\"]}   cache_read={e[\"cache_read_tokens\"]}')
except Exception as ex:
  print(f'    (cycle-history error: {ex})')
"

echo
echo "  memory (agents persisted learnings):"
sqlite3 "$DB" "SELECT key, substr(value,1,60), agent_name FROM memories WHERE project='$PROJECT' AND archived_at IS NULL;" | awk -F'|' '{print "    "$1"   ="$2"   by="$3}'

echo
echo "  stray claude processes:"
ps aux | grep -E "claude --allow" | grep -v grep | wc -l | awk '{print "    "$0}'

echo
echo "━━━ done ━━━"
echo "Workspace: $WORKSPACE"
echo "Run './scripts/team-demo.sh cleanup' to wipe."
