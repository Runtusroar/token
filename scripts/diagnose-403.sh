#!/usr/bin/env bash
# scripts/diagnose-403.sh
#
# Localize the source of a 403 "Your request was blocked" error hitting the
# ai-relay gateway. Probes three layers in order and prints a diff:
#   1. DB state for the offending API key / model / request logs.
#   2. Public URL (traverses Cloudflare, frontend, backend, upstream).
#   3. Backend direct from inside the docker network (bypasses CF + frontend).
#
# Read-only: only SELECTs + outbound probes; nothing is mutated.
#
# Usage:
#   ./scripts/diagnose-403.sh <sk-key> [model]
#
# Env overrides:
#   DOMAIN=api.juezhou.org     Public host for Step 4.
#   COMPOSE="docker compose"   Compose v1 users: COMPOSE=docker-compose
#
# Run from the repo root (where docker-compose*.yml lives).

set -u
IFS=$'\n\t'

SK_KEY="${1:-}"
MODEL="${2:-claude-opus-4-7}"
DOMAIN="${DOMAIN:-api.juezhou.org}"

if [[ -z "$SK_KEY" || "$SK_KEY" == "-h" || "$SK_KEY" == "--help" ]]; then
    cat <<EOF
Usage: $0 <sk-key> [model]
  sk-key   Relay API key that is failing (e.g. sk-48508...c8ed)
  model    Model name (default: claude-opus-4-7)

Environment:
  DOMAIN   Public hostname (default: api.juezhou.org)
  COMPOSE  Compose command (default: 'docker compose')
EOF
    exit 1
fi

if [[ ! "$SK_KEY" =~ ^sk-[a-zA-Z0-9]+$ ]]; then
    echo "ERROR: key does not look like sk-<hex>" >&2
    exit 1
fi

if [[ -f docker-compose.prod.yml ]]; then
    CF_FILE="-f docker-compose.prod.yml"
elif [[ -f docker-compose.yml ]]; then
    CF_FILE=""
else
    echo "ERROR: run this from the repo root (no docker-compose*.yml found)." >&2
    exit 1
fi

COMPOSE_CMD="${COMPOSE:-docker compose}"
CP="$COMPOSE_CMD $CF_FILE"

PAYLOAD='{"model":"'"$MODEL"'","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}'

line() { printf '\n\033[36m════════ %s ════════\033[0m\n' "$*"; }

line "Step 1/6  DB · api_keys + user for this key"
$CP exec -T postgres psql -U relay -d relay -P pager=off -c "
    SELECT ak.id AS key_id,
           ak.status AS key_status,
           ak.last_used_at,
           u.id  AS user_id,
           u.email,
           u.status AS user_status,
           u.balance
    FROM api_keys ak
    JOIN users u ON u.id = ak.user_id
    WHERE ak.key = '$SK_KEY';
" || echo "(db query failed; check if postgres service name differs)"

line "Step 2/6  DB · active channels that include '$MODEL'"
$CP exec -T postgres psql -U relay -d relay -P pager=off -c "
    SELECT id, name, type, status, priority, weight, base_url
    FROM channels
    WHERE status = 'active'
      AND models::text LIKE '%$MODEL%'
    ORDER BY priority, id;
" || true

line "Step 3/6  DB · last 10 request_logs for this key"
$CP exec -T postgres psql -U relay -d relay -P pager=off -c "
    SELECT rl.id, rl.created_at, rl.channel_id, rl.model,
           rl.status, rl.duration_ms, rl.ip_address
    FROM request_logs rl
    JOIN api_keys ak ON ak.id = rl.api_key_id
    WHERE ak.key = '$SK_KEY'
    ORDER BY rl.created_at DESC
    LIMIT 10;
" || true

line "Step 4/6  Public · POST https://$DOMAIN/v1/chat/completions (through Cloudflare)"
TMP_HDR=$(mktemp); TMP_BODY=$(mktemp)
trap 'rm -f "$TMP_HDR" "$TMP_BODY"' EXIT
if curl -sS -D "$TMP_HDR" -o "$TMP_BODY" \
        --max-time 30 \
        -X POST "https://$DOMAIN/v1/chat/completions" \
        -H "Authorization: Bearer $SK_KEY" \
        -H "Content-Type: application/json" \
        --data "$PAYLOAD"; then
    :
else
    echo "(curl exit=$?)"
fi
echo "-- response headers --"
head -n 40 "$TMP_HDR"
echo "-- response body (first 1500 chars) --"
head -c 1500 "$TMP_BODY"; echo

line "Step 5/6  Origin-direct · same request to the backend container (bypass CF + frontend)"
# wget is in busybox on alpine; no curl. -S prints server headers to stderr.
$CP exec -T -e SK="$SK_KEY" -e BODY="$PAYLOAD" backend sh -c '
    wget -S -O - \
         --header="Authorization: Bearer $SK" \
         --header="Content-Type: application/json" \
         --post-data="$BODY" \
         --timeout=30 \
         http://localhost:8080/v1/chat/completions 2>&1 | head -c 3000
    echo
' || echo "(exec exit=$?)"

line "Step 6/6  Backend logs · last 60 seconds"
$CP logs --since 60s backend 2>&1 | tail -n 80 || true

line "Interpretation"
cat <<'EOF'
Compare Step 4 (public) vs Step 5 (direct-to-backend):

  A) Step 4 = 403 "Your request was blocked"
     Step 5 = 200 (or structured JSON)
     → Cloudflare (yours, in front of juezhou) blocked Step 4.
       Action: check CF dashboard → Security → Events for a matching Ray-ID
       (look at "cf-ray" in the Step 4 headers); inspect WAF Managed Rules,
       Bot Fight Mode, Rate Limit, and any custom Firewall Rules.

  B) Step 4 = 403 "Your request was blocked"
     Step 5 = 403 "Your request was blocked"    (identical)
     → The 403 originates upstream. Your server forwarded it unchanged
       (adapter/claude.go:87-94). Action: identify the upstream from Step 2
       (base_url), curl that /v1/messages directly with its real key to
       reproduce. Almost certainly upstream's own WAF / account block.

  C) Step 4 = 403 but with a STRUCTURED JSON body
       {"error":{"type":"authentication_error",...}}
     → That came from THIS backend's auth middleware — your key/account is
       not active. Step 1/Step 3 should confirm.

  D) Step 1 returns zero rows
     → the sk- key does not exist in your DB. Source confirmed: auth reject
       (but then you would see 401, not 403; a 403 plain-text means B).

  E) Step 2 returns zero rows
     → no active channel for this model. The backend returns 503 "no_channel",
       not 403 — so this is not the cause, but worth fixing anyway.

Headers to inspect in Step 4:
  - server: cloudflare   → CF is the responder
  - cf-ray: xxxxxxxxxxx  → grep this Ray-ID in CF dashboard
  - content-type         → text/html = CF block page; application/json = your app
EOF
