#!/usr/bin/env bash
# scripts/cache-hit-rate.sh
#
# Snapshot of prompt-cache hit rate across the relay. Aggregates the cache
# token columns on request_logs four ways:
#   1. Overall (window).
#   2. Per channel — surfaces whether weighted-random channel rotation is
#      splintering caches across upstream keys.
#   3. Per model — different models have separate caches.
#   4. Per user — find out which user is seeing the low hit rate.
#   5. Per hour trend — last 24h, regardless of window arg.
#
# Hit-rate formula:
#   cache_read / (input + cache_read + cache_write)
# i.e. of all input-side tokens, what fraction was served from cache.
#
# Read-only: only SELECTs.
#
# Usage:
#   ./scripts/cache-hit-rate.sh [window]
#   ./scripts/cache-hit-rate.sh 1h
#   ./scripts/cache-hit-rate.sh 24h
#   ./scripts/cache-hit-rate.sh 7days
#
# Args:
#   window         Postgres interval string (default: 24h). e.g. 30min, 6h, 7days.
#
# Env overrides:
#   COMPOSE="docker compose"   Compose v1 users: COMPOSE=docker-compose
#   MIN_REQS=10                Hide groups with fewer than N requests (default 10).
#
# Run from the repo root (where docker-compose*.yml lives).
# Output is teed to /tmp/relay-cache-<timestamp>.txt.

set -u

WINDOW="${1:-24h}"
MIN_REQS="${MIN_REQS:-10}"

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

OUT="/tmp/relay-cache-$(date +%Y%m%d-%H%M%S).txt"
exec > >(tee "$OUT") 2>&1

line() { printf '\n════════ %s ════════\n' "$*"; }

PSQL=("${CP[@]}" exec -T postgres psql -U relay -d relay -P pager=off -c)

line "Context"
echo "window      : last $WINDOW"
echo "min reqs    : $MIN_REQS  (groups with fewer reqs hidden in per-X tables)"
echo "now (db)    : $("${CP[@]}" exec -T postgres psql -U relay -d relay -tA -c 'SELECT NOW();' 2>/dev/null | tr -d '[:space:]')"
echo "formula     : hit% = cache_read_tokens / prompt_tokens"
echo "              (prompt_tokens already = fresh_input + cache_read + cache_write,"
echo "               see backend/internal/adapter/claude.go:219)"

line "Overall hit rate (window)"
"${PSQL[@]}" "
    SELECT
      COUNT(*)                              AS reqs,
      COALESCE(SUM(prompt_tokens
                   - cache_read_tokens
                   - cache_write_tokens), 0) AS fresh_input,
      COALESCE(SUM(cache_read_tokens), 0)   AS cache_hit,
      COALESCE(SUM(cache_write_tokens), 0)  AS cache_write,
      COALESCE(SUM(completion_tokens), 0)   AS output,
      ROUND(100.0 * SUM(cache_read_tokens)::numeric
            / NULLIF(SUM(prompt_tokens), 0), 2)
            AS hit_rate_pct
    FROM request_logs
    WHERE created_at > NOW() - INTERVAL '$WINDOW' AND status='success';
"

line "Per channel (window)  — uneven rates here = channel rotation splitting caches"
"${PSQL[@]}" "
    SELECT
      rl.channel_id,
      COALESCE(c.name, '(deleted)') AS channel,
      COUNT(*) AS reqs,
      ROUND(100.0 * SUM(rl.cache_read_tokens)::numeric
            / NULLIF(SUM(rl.prompt_tokens), 0), 2)
            AS hit_rate_pct,
      COALESCE(SUM(rl.cache_read_tokens), 0)  AS cache_hit_tokens,
      COALESCE(SUM(rl.cache_write_tokens), 0) AS cache_write_tokens
    FROM request_logs rl
    LEFT JOIN channels c ON c.id = rl.channel_id
    WHERE rl.created_at > NOW() - INTERVAL '$WINDOW' AND rl.status='success'
    GROUP BY 1,2
    HAVING COUNT(*) >= $MIN_REQS
    ORDER BY hit_rate_pct NULLS LAST;
"

line "Per model (window)"
"${PSQL[@]}" "
    SELECT
      model,
      COUNT(*) AS reqs,
      ROUND(100.0 * SUM(cache_read_tokens)::numeric
            / NULLIF(SUM(prompt_tokens), 0), 2)
            AS hit_rate_pct
    FROM request_logs
    WHERE created_at > NOW() - INTERVAL '$WINDOW' AND status='success'
    GROUP BY model
    HAVING COUNT(*) >= $MIN_REQS
    ORDER BY reqs DESC;
"

line "Top 20 users by request volume (window)"
"${PSQL[@]}" "
    SELECT
      rl.user_id,
      COALESCE(u.email, '(deleted)') AS email,
      COUNT(*) AS reqs,
      ROUND(100.0 * SUM(rl.cache_read_tokens)::numeric
            / NULLIF(SUM(rl.prompt_tokens), 0), 2)
            AS hit_rate_pct
    FROM request_logs rl
    LEFT JOIN users u ON u.id = rl.user_id
    WHERE rl.created_at > NOW() - INTERVAL '$WINDOW' AND rl.status='success'
    GROUP BY 1,2
    HAVING COUNT(*) >= $MIN_REQS
    ORDER BY reqs DESC
    LIMIT 20;
"

line "Hourly trend (last 24h, regardless of window arg)"
"${PSQL[@]}" "
    SELECT
      date_trunc('hour', created_at) AS hour,
      COUNT(*) AS reqs,
      ROUND(100.0 * SUM(cache_read_tokens)::numeric
            / NULLIF(SUM(prompt_tokens), 0), 2)
            AS hit_rate_pct
    FROM request_logs
    WHERE created_at > NOW() - INTERVAL '24 hours' AND status='success'
    GROUP BY 1
    ORDER BY 1 DESC;
"

line "Live load (right now)"
"${PSQL[@]}" "
    SELECT
      (SELECT COUNT(*) FROM request_logs
        WHERE created_at >= NOW() - INTERVAL '1 minute')                AS rpm_last_1min,
      (SELECT COALESCE(SUM(total_tokens), 0) FROM request_logs
        WHERE created_at >= NOW() - INTERVAL '1 minute'
          AND status='success')                                         AS tpm_last_1min,
      (SELECT COUNT(*) FROM request_logs
        WHERE status='pending'
          AND created_at >= NOW() - INTERVAL '5 minute')                AS in_flight_estimate;
"

line "Done"
echo "Saved to: $OUT"
echo
echo "Reading the output:"
echo "  • Overall hit_rate_pct < 30%   → likely missing cache_control on client side"
echo "                                   or short prompts under the 1024-token min."
echo "  • Per-channel rates very uneven → channel rotation is fragmenting caches;"
echo "                                    consider sticky routing or single top-priority channel."
echo "  • Per-user low rate            → that user's client isn't sending cache_control,"
echo "                                    or their prompts change every turn."
