#!/usr/bin/env bash
# Webhook playground — end-to-end demo of wrai.th's trigger + webhook system.
#
# Sets up a demo project with 3 profiles + 3 triggers, fires simulated webhooks
# (git commit, stripe payment, cron tick), and shows the resulting spawn chain.
#
# Usage:
#   ./scripts/webhook-playground.sh              # full demo
#   ./scripts/webhook-playground.sh cleanup      # drop demo project + kill children
#   ./scripts/webhook-playground.sh no-spawn     # register triggers without spawn (fast, no claude cost)
#
# Requires: relay running on :8090, curl, sqlite3, python3.

set -euo pipefail

RELAY="${RELAY:-http://localhost:8090}"
PROJECT="${PROJECT:-playground}"
DB="${DB:-$HOME/.agent-relay/relay.db}"

MODE="${1:-full}"

# --- helpers ----------------------------------------------------------------

mcp() {
  # mcp <method> <params-json>
  curl -sS -X POST "$RELAY/mcp?project=$PROJECT" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "$1"
}

rest_post() {
  curl -sS -X POST "$RELAY$1" -H "Content-Type: application/json" -d "$2"
}

rest_get() {
  curl -sS "$RELAY$1"
}

section() {
  echo
  echo "━━━ $1 ━━━"
}

kill_all_children() {
  sqlite3 "$DB" "SELECT id FROM spawn_children WHERE status='running' AND project='$PROJECT';" | while read -r id; do
    [ -n "$id" ] && curl -sS -X POST "$RELAY/api/spawn/children/$id/kill" >/dev/null 2>&1 || true
  done
}

# --- cleanup mode -----------------------------------------------------------

if [ "$MODE" = "cleanup" ]; then
  section "Cleanup: killing spawns + dropping project"
  kill_all_children
  sqlite3 "$DB" "DELETE FROM triggers WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM trigger_history WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM messages WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM deliveries WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM agents WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM profiles WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM spawn_children WHERE project='$PROJECT';"
  sqlite3 "$DB" "DELETE FROM projects WHERE name='$PROJECT';"
  echo "  done"
  exit 0
fi

# --- setup ------------------------------------------------------------------

section "1. Create project + register 3 profiles"

mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"create_project\",\"arguments\":{\"name\":\"$PROJECT\",\"description\":\"webhook demo\"}}}" >/dev/null
echo "  project: $PROJECT"

for slug in code-reviewer billing-reactor nightly-janitor; do
  mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"register_profile\",\"arguments\":{\"project\":\"$PROJECT\",\"slug\":\"$slug\",\"name\":\"$slug\",\"context_pack\":\"You are the $slug. Acknowledge the trigger meta you received, then exit cleanly.\"}}}" >/dev/null
  echo "  profile: $slug"
done

# --- register triggers ------------------------------------------------------

section "2. Register 3 triggers (3 different events, 1 with match_rules)"

# Trigger 1 — fires on ANY git.commit
TR1=$(rest_post /api/triggers "{
  \"project\":\"$PROJECT\",
  \"event\":\"git.commit\",
  \"profile_slug\":\"code-reviewer\",
  \"cycle\":\"respond\",
  \"cooldown_seconds\":0
}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  git.commit → code-reviewer   id=${TR1:0:8}   (match: *)"

# Trigger 2 — fires ONLY on stripe.payment with amount > 1000
TR2=$(rest_post /api/triggers "{
  \"project\":\"$PROJECT\",
  \"event\":\"stripe.payment.succeeded\",
  \"match_rules\":\"{\\\"currency\\\":\\\"usd\\\"}\",
  \"profile_slug\":\"billing-reactor\",
  \"cycle\":\"respond\",
  \"cooldown_seconds\":0
}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  stripe.payment.succeeded → billing-reactor   id=${TR2:0:8}   (match: currency=usd)"

# Trigger 3 — nightly cron
TR3=$(rest_post /api/triggers "{
  \"project\":\"$PROJECT\",
  \"event\":\"cron.nightly\",
  \"profile_slug\":\"nightly-janitor\",
  \"cycle\":\"respond\",
  \"cooldown_seconds\":0
}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  cron.nightly → nightly-janitor   id=${TR3:0:8}"

# --- fire webhooks ----------------------------------------------------------

section "3. Fire 4 webhooks (simulating external services)"

echo "  [3a] git.commit on main → SHOULD fire code-reviewer"
rest_post /api/webhooks/$PROJECT/git.commit '{
  "author":"loic",
  "branch":"main",
  "sha":"abc123",
  "message":"fix auth bug",
  "files_changed":7
}' | python3 -c "import sys,json; r=json.load(sys.stdin); print(f'     fires={len(r[\"fires\"])} skipped={len(r[\"skipped\"])}')"

echo
echo "  [3b] stripe.payment with currency=usd → SHOULD fire billing-reactor"
rest_post /api/webhooks/$PROJECT/stripe.payment.succeeded '{
  "id":"pi_123",
  "amount":5000,
  "currency":"usd",
  "customer":"cus_456"
}' | python3 -c "import sys,json; r=json.load(sys.stdin); print(f'     fires={len(r[\"fires\"])} skipped={len(r[\"skipped\"])}')"

echo
echo "  [3c] stripe.payment with currency=eur → SHOULD be SKIPPED (match_rules fail)"
rest_post /api/webhooks/$PROJECT/stripe.payment.succeeded '{
  "id":"pi_789",
  "amount":5000,
  "currency":"eur",
  "customer":"cus_111"
}' | python3 -c "import sys,json; r=json.load(sys.stdin); print(f'     fires={len(r[\"fires\"])} skipped={len(r[\"skipped\"])}   (rules_mismatch expected)')"

echo
echo "  [3d] cron.nightly → SHOULD fire nightly-janitor"
rest_post /api/webhooks/$PROJECT/cron.nightly '{
  "tick":"2026-04-19T03:00:00Z",
  "job":"cleanup"
}' | python3 -c "import sys,json; r=json.load(sys.stdin); print(f'     fires={len(r[\"fires\"])} skipped={len(r[\"skipped\"])}')"

if [ "$MODE" = "no-spawn" ]; then
  section "4. no-spawn mode: killing any spawned children to save tokens"
  sleep 1
  kill_all_children
  echo "  done (skipped spawn observation)"
  section "5. Final state"
  sqlite3 "$DB" "SELECT event, COALESCE(child_id, 'none'), COALESCE(error, '') FROM trigger_history WHERE project='$PROJECT' ORDER BY fired_at DESC LIMIT 5;" | awk -F'|' '{print "  "$1"   child="substr($2,1,8)"   err="$3}'
  exit 0
fi

# --- observe spawns ---------------------------------------------------------

section "4. Wait for spawns to complete (up to 90s)"

sleep 2
DEADLINE=$(($(date +%s) + 90))
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  RUNNING=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE status='running' AND project='$PROJECT';")
  TOTAL=$(sqlite3 "$DB" "SELECT count(*) FROM spawn_children WHERE project='$PROJECT';")
  echo "  t=$(($(date +%s) - (DEADLINE - 90)))s   running=$RUNNING / total=$TOTAL"
  if [ "$RUNNING" = "0" ] && [ "$TOTAL" -ge "3" ]; then break; fi
  sleep 5
done

section "5. Final state"

echo
echo "  trigger_history:"
sqlite3 "$DB" "SELECT event, COALESCE(substr(child_id,1,8), 'none'), COALESCE(error, '') FROM trigger_history WHERE project='$PROJECT' ORDER BY fired_at DESC;" | awk -F'|' '{print "    "$1"   child="$2"   err="$3}'

echo
echo "  spawn_children:"
sqlite3 "$DB" "SELECT substr(id,1,8), profile, status, exit_code, COALESCE(error,'') FROM spawn_children WHERE project='$PROJECT' ORDER BY started_at;" | awk -F'|' '{print "    "$1"   profile="$2"   status="$3"   exit="$4"   err="$5}'

echo
echo "  cycle tokens:"
rest_get "/api/cycle-history?project=$PROJECT" | python3 -c "
import sys, json
try:
  h = json.load(sys.stdin)
  for e in h:
    print(f'    {e[\"cycle_name\"]}   {e[\"duration_ms\"]}ms   in={e[\"input_tokens\"]}   out={e[\"output_tokens\"]}   cache_read={e[\"cache_read_tokens\"]}')
except Exception as ex:
  print(f'    (error reading cycle history: {ex})')"

echo
echo "  events:"
rest_get "/api/events/recent?project=$PROJECT&limit=10" | python3 -c "
import sys, json
for e in json.load(sys.stdin):
    tgt = e.get('target','')
    print(f'    {e[\"type\"]}.{e[\"action\"]}   by={e[\"agent\"]}   target={tgt}')"

echo
echo "━━━ done ━━━"
echo "Run './scripts/webhook-playground.sh cleanup' to drop the project and kill spawns."
