#!/usr/bin/env bash
# scripts/diagnose-errors.sh
#
# Snapshot of all recent failing requests across the relay, ready to paste
# into a chat for triage. Reads from request_logs (status='error') + tails
# the backend container logs over the same window.
#
# Read-only: only SELECTs + docker logs.
#
# Usage:
#   ./scripts/diagnose-errors.sh [window]
#   ./scripts/diagnose-errors.sh 1h
#   ./scripts/diagnose-errors.sh 24h
#
# Args:
#   window         Postgres interval string (default: 1h). e.g. 30min, 6h, 2days.
#
# Env overrides:
#   COMPOSE="docker compose"   Compose v1 users: COMPOSE=docker-compose
#
# Run from the repo root (where docker-compose*.yml lives).
# Output is teed to /tmp/relay-errors-<timestamp>.txt.

set -u

WINDOW="${1:-1h}"

if [[ -n "${COMPOSE:-}" ]]; then
    read -r -a CP <<< "$COMPOSE"
else
    CP=(docker compose)
fi

if [[ -f docker-compose.prod.yml ]]; then
    CP+=(-f docker-compose.prod.yml)
elif [[ ! -f docker-compose.yml ]]; then
    echo "ERROR: run from repo root (no docker-compose*.yml found)." >&2
    exit 1
fi

OUT="/tmp/relay-errors-$(date +%Y%m%d-%H%M%S).txt"
exec > >(tee "$OUT") 2>&1

line() { printf '\n════════ %s ════════\n' "$*"; }

PSQL=("${CP[@]}" exec -T postgres psql -U relay -d relay -P pager=off -A -F$'\t' -c)

# Detect whether the v2 error-diagnostics columns exist on this DB.
HAS_V2=$("${CP[@]}" exec -T postgres psql -U relay -d relay -tA -c "
    SELECT COUNT(*) FROM information_schema.columns
    WHERE table_name='request_logs'
      AND column_name IN ('upstream_status','upstream_error','error_stage');
" 2>/dev/null | tr -d '[:space:]')

line "Context"
echo "window      : last $WINDOW"
echo "schema      : $([[ "$HAS_V2" == "3" ]] && echo 'v2 (has error_stage/upstream_*)' || echo 'legacy (no error_stage column — run AutoMigrate or ALTER TABLE)')"
echo "now (db)    : $("${PSQL[@]}" "SELECT NOW();" 2>/dev/null | tail -1)"

line "Volume · status counts in window"
"${PSQL[@]}" "
    SELECT status, COUNT(*) AS n
    FROM request_logs
    WHERE created_at > NOW() - INTERVAL '$WINDOW'
    GROUP BY status
    ORDER BY n DESC;
"

line "Errors per hour (last 24h, regardless of window arg)"
"${PSQL[@]}" "
    SELECT date_trunc('hour', created_at) AS hour, COUNT(*) AS n
    FROM request_logs
    WHERE status='error' AND created_at > NOW() - INTERVAL '24 hours'
    GROUP BY 1 ORDER BY 1 DESC;
"

if [[ "$HAS_V2" == "3" ]]; then
    line "Aggregate by error_stage × upstream_status"
    "${PSQL[@]}" "
        SELECT COALESCE(NULLIF(error_stage,''), '(unset)') AS stage,
               upstream_status,
               COUNT(*) AS n,
               MAX(created_at) AS last_seen
        FROM request_logs
        WHERE status='error' AND created_at > NOW() - INTERVAL '$WINDOW'
        GROUP BY 1,2 ORDER BY n DESC;
    "
fi

line "Top failing models in window"
"${PSQL[@]}" "
    SELECT model, COUNT(*) AS n,
           ROUND(AVG(duration_ms)) AS avg_ms,
           MAX(created_at) AS last_seen
    FROM request_logs
    WHERE status='error' AND created_at > NOW() - INTERVAL '$WINDOW'
    GROUP BY model ORDER BY n DESC LIMIT 20;
"

line "Top failing channels in window (channel_id → name/base_url)"
"${PSQL[@]}" "
    SELECT rl.channel_id,
           COALESCE(c.name, '(deleted)') AS channel_name,
           COALESCE(c.base_url, '') AS base_url,
           COUNT(*) AS n,
           MAX(rl.created_at) AS last_seen
    FROM request_logs rl
    LEFT JOIN channels c ON c.id = rl.channel_id
    WHERE rl.status='error' AND rl.created_at > NOW() - INTERVAL '$WINDOW'
    GROUP BY 1,2,3 ORDER BY n DESC LIMIT 20;
"

line "Top failing users / keys in window"
"${PSQL[@]}" "
    SELECT rl.user_id,
           COALESCE(u.email,'(deleted)') AS email,
           rl.api_key_id,
           COUNT(*) AS n,
           MAX(rl.created_at) AS last_seen
    FROM request_logs rl
    LEFT JOIN users u ON u.id = rl.user_id
    WHERE rl.status='error' AND rl.created_at > NOW() - INTERVAL '$WINDOW'
    GROUP BY 1,2,3 ORDER BY n DESC LIMIT 15;
"

if [[ "$HAS_V2" == "3" ]]; then
    line "Error clusters (every distinct error in window, deduped by signature)"
    # Cluster signature = (error_stage, upstream_status, first 200 chars of body).
    # Picks one full sample per cluster via DISTINCT ON; ARRAY_AGG keeps the
    # affected models/channels visible without exploding the row count.
    "${PSQL[@]}" "
        WITH clustered AS (
            SELECT id, created_at, user_id, channel_id, model,
                   error_stage, upstream_status,
                   COALESCE(upstream_error,'') AS upstream_error,
                   COALESCE(error_stage,'') || '|' ||
                       upstream_status::text || '|' ||
                       LEFT(COALESCE(upstream_error,''), 200) AS sig
            FROM request_logs
            WHERE status='error' AND created_at > NOW() - INTERVAL '$WINDOW'
        ),
        agg AS (
            SELECT sig, COUNT(*) AS n, MAX(created_at) AS last_seen,
                   ARRAY_AGG(DISTINCT model ORDER BY model) AS models,
                   ARRAY_AGG(DISTINCT channel_id ORDER BY channel_id) AS channel_ids
            FROM clustered GROUP BY sig
        ),
        sample AS (
            SELECT DISTINCT ON (sig) sig, error_stage, upstream_status,
                   upstream_error, id AS sample_id
            FROM clustered ORDER BY sig, created_at DESC
        )
        SELECT a.n,
               a.last_seen,
               s.error_stage,
               s.upstream_status,
               a.models,
               a.channel_ids,
               s.sample_id,
               LEFT(s.upstream_error, 1500) AS upstream_error
        FROM agg a JOIN sample s USING (sig)
        ORDER BY a.n DESC;
    "

    line "Auto-classification (heuristic counts within window)"
    "${PSQL[@]}" "
        SELECT
          CASE
            WHEN upstream_status = 401 OR upstream_error ILIKE '%invalid%api%key%'
              OR upstream_error ILIKE '%authentication%' OR upstream_error ILIKE '%unauthorized%'
              THEN '01_upstream_auth (invalid/expired upstream key — fix channel.api_key)'
            WHEN upstream_status = 403 OR upstream_error ILIKE '%blocked%' OR upstream_error ILIKE '%forbidden%'
              THEN '02_upstream_forbidden (CF/WAF or account block on upstream)'
            WHEN upstream_status = 429 OR upstream_error ILIKE '%rate%limit%' OR upstream_error ILIKE '%quota%'
              THEN '03_upstream_rate_limited (429 — back off or rotate channel)'
            WHEN upstream_status BETWEEN 500 AND 599
              THEN '04_upstream_5xx (upstream broken — try fallback channel)'
            WHEN error_stage = 'no_channel' OR upstream_error ILIKE '%no_channel%' OR upstream_error ILIKE '%no active channel%'
              THEN '05_no_channel (no channel matches model — check channels.models)'
            WHEN error_stage = 'auth' OR upstream_error ILIKE '%insufficient%balance%' OR upstream_error ILIKE '%balance%'
              THEN '06_relay_auth_or_balance (relay-side reject: bad sk-key, disabled, or out of balance)'
            WHEN upstream_error ILIKE '%timeout%' OR upstream_error ILIKE '%context deadline%'
              THEN '07_timeout (upstream slow / network)'
            WHEN upstream_error ILIKE '%context canceled%' OR upstream_error ILIKE '%client disconnect%' OR upstream_error ILIKE '%broken pipe%'
              THEN '08_client_aborted (client cut the connection — usually benign)'
            WHEN error_stage = 'convert' OR upstream_error ILIKE '%convert%' OR upstream_error ILIKE '%marshal%' OR upstream_error ILIKE '%unmarshal%'
              THEN '09_payload_convert (request/response shape mismatch — adapter bug?)'
            WHEN upstream_status = 0 AND upstream_error <> ''
              THEN '10_transport (TCP/TLS/DNS — never reached upstream)'
            ELSE '99_other'
          END AS category,
          COUNT(*) AS n,
          MAX(created_at) AS last_seen
        FROM request_logs
        WHERE status='error' AND created_at > NOW() - INTERVAL '$WINDOW'
        GROUP BY 1 ORDER BY 1;
    "
else
    line "Full error rows in window (legacy schema — no upstream_error available)"
    "${PSQL[@]}" "
        SELECT id, created_at, user_id, channel_id, model, duration_ms, ip_address
        FROM request_logs
        WHERE status='error' AND created_at > NOW() - INTERVAL '$WINDOW'
        ORDER BY id DESC;
    "
    echo
    echo "(legacy schema — restart backend to AutoMigrate the v2 columns,"
    echo " or run: ALTER TABLE request_logs ADD COLUMN upstream_status int DEFAULT 0,"
    echo "         ADD COLUMN upstream_error varchar(2048), ADD COLUMN error_stage varchar(50);)"
fi

line "Backend container logs (window: $WINDOW)"
# Compose's --since wants Go-duration syntax (1h, 30m, 24h), which is the
# common case here. If WINDOW uses Postgres-style "30min"/"2days", fall back
# to a 1h tail so this section still produces something.
LOG_SINCE="$WINDOW"
case "$LOG_SINCE" in
    *min|*minutes|*day|*days|*hour|*hours) LOG_SINCE="1h" ;;
esac
"${CP[@]}" logs --since "$LOG_SINCE" backend 2>&1 \
    | grep -viE "GET /health|GET /metrics" \
    | grep -iE "error|panic|fail|stage=|upstream|level=warn" \
    | tail -n 200 || echo "(no matching log lines)"

line "Done"
echo "Saved to: $OUT"
echo "Paste the contents back into chat for analysis."
