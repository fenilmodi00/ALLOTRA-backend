#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
DB_CONTAINER="${DB_CONTAINER:-ipo-backend-db-1}"
DB_NAME="${DB_NAME:-ipo_db}"
DB_USER="${DB_USER:-postgres}"
WAIT_SECONDS="${WAIT_SECONDS:-240}"
STRICT_WAIT="${STRICT_WAIT:-false}"

log() {
  printf '[bootstrap] %s\n' "$1"
}

sql() {
  local query="$1"
  docker exec "$DB_CONTAINER" psql -X -q -A -t -U "$DB_USER" -d "$DB_NAME" -c "$query"
}

require_tools() {
  command -v curl >/dev/null 2>&1 || { echo "curl is required"; exit 1; }
  command -v docker >/dev/null 2>&1 || { echo "docker is required"; exit 1; }
}

ensure_job_dispatch_schema() {
  log "Ensuring job_dispatch schema matches poller expectations"
  sql "ALTER TABLE job_dispatch ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;" >/dev/null
  sql "ALTER TABLE job_dispatch ADD COLUMN IF NOT EXISTS picked_up_at TIMESTAMP;" >/dev/null
  sql "ALTER TABLE job_dispatch ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 0;" >/dev/null
  sql "ALTER TABLE job_dispatch ADD COLUMN IF NOT EXISTS target_ipo_id UUID;" >/dev/null
}

cleanup_stale_running_jobs() {
  log "Cleaning stale running jobs (if any)"
  sql "
    UPDATE job_dispatch
    SET status = 'failed',
        completed_at = NOW(),
        updated_at = NOW(),
        error_message = COALESCE(error_message, '') || ' [stale run reset by bootstrap]'
    WHERE status = 'running'
      AND (picked_up_at IS NULL OR picked_up_at < NOW() - INTERVAL '15 minutes');
  " >/dev/null
}

wait_server() {
  local i=0
  while [ "$i" -lt 30 ]; do
    if curl -fsS "$BASE_URL/ready" >/dev/null 2>&1; then
      log "Server is ready at $BASE_URL"
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  echo "Server is not ready at $BASE_URL/ready"
  exit 1
}

enqueue_job() {
  local job_type="$1"
  local payload="$2"
  local priority="${3:-10}"
  sql "INSERT INTO job_dispatch (job_type, payload, status, priority, created_at, updated_at) VALUES ('$job_type', '$payload'::jsonb, 'pending', $priority, NOW(), NOW()) RETURNING id;" | head -n 1 | tr -d '[:space:]'
}

wait_job() {
  local job_id="$1"
  local waited=0
  while [ "$waited" -lt "$WAIT_SECONDS" ]; do
    local status
    status="$(sql "SELECT status FROM job_dispatch WHERE id = '$job_id';" | tr -d '[:space:]')"
    if [ "$status" = "completed" ]; then
      log "Job $job_id completed"
      return 0
    fi
    if [ "$status" = "failed" ]; then
      local err
      err="$(sql "SELECT COALESCE(error_message, '') FROM job_dispatch WHERE id = '$job_id';")"
      echo "Job $job_id failed: $err"
      return 1
    fi
    sleep 5
    waited=$((waited + 5))
  done
  echo "Job $job_id timed out after ${WAIT_SECONDS}s"
  return 1
}

trigger_gmp_history() {
  if [ -n "$ADMIN_TOKEN" ]; then
    log "Triggering GMP history via admin endpoint"
    curl -fsS -X POST "$BASE_URL/api/v2/admin/gmp-history/update" -H "X-Admin-Token: $ADMIN_TOKEN" >/dev/null
    return 0
  fi

  log "ADMIN_TOKEN not set, enqueueing gmp_history_update directly"
  local job_id
  job_id="$(enqueue_job "gmp_history_update" "{}" 20)"
  if [ "$STRICT_WAIT" != "true" ]; then
    log "Queued gmp_history_update job $job_id (non-strict mode, not waiting)"
    return 0
  fi
  if ! wait_job "$job_id"; then
    return 1
  fi
}

enqueue_registrar_fetch_jobs() {
  log "Enqueueing fetch_registrar_company_code jobs for today's IPOs"

  local inserted
  inserted="$(sql "
WITH candidates AS (
  SELECT
    i.id,
    i.name,
    CASE
      WHEN UPPER(i.registrar) LIKE '%KFIN%' THEN 'KFIN'
      WHEN UPPER(i.registrar) LIKE '%BIGSHARE%' THEN 'BIGSHARE'
      WHEN UPPER(i.registrar) LIKE '%MUFG%' THEN 'MUFG'
      ELSE NULL
    END AS registrar_short_code
  FROM ipo_list i
  WHERE DATE(i.result_date) = CURRENT_DATE
), unresolved AS (
  SELECT c.*
  FROM candidates c
  LEFT JOIN ipo_registrar_codes rc
    ON rc.ipo_id = c.id AND rc.registrar_short_code = c.registrar_short_code
  WHERE c.registrar_short_code IS NOT NULL
    AND (rc.id IS NULL OR rc.is_resolved = FALSE)
)
INSERT INTO job_dispatch (job_type, payload, status, priority, target_ipo_id, created_at, updated_at)
SELECT
  'fetch_registrar_company_code',
  jsonb_build_object(
    'ipo_id', u.id,
    'registrar_short_code', u.registrar_short_code,
    'ipo_name', u.name
  ),
  'pending',
  50,
  u.id,
  NOW(),
  NOW()
FROM unresolved u
RETURNING id;
")"

  if [ -z "${inserted// }" ]; then
    log "No new registrar fetch jobs were needed"
    return 0
  fi

  while IFS= read -r job_id; do
    job_id="$(echo "$job_id" | tr -d '[:space:]')"
    [ -z "$job_id" ] && continue
    wait_job "$job_id"
  done <<< "$inserted"
}

summary() {
  log "Summary"
  sql "SELECT job_type, status, COUNT(*) FROM job_dispatch GROUP BY job_type, status ORDER BY job_type, status;"
  sql "SELECT ipo_id, registrar_short_code, registrar_company_code, match_score, is_resolved FROM ipo_registrar_codes ORDER BY updated_at DESC LIMIT 10;"
}

main() {
  require_tools
  ensure_job_dispatch_schema
  cleanup_stale_running_jobs
  wait_server

  log "Step 1/4: enqueue daily_ipo_update (Chittorgarh + Groww pipeline)"
  local daily_job_id
  daily_job_id="$(enqueue_job "daily_ipo_update" "{}" 30)"
  wait_job "$daily_job_id"

  log "Step 2/4: enqueue gmp_update"
  local gmp_job_id
  gmp_job_id="$(enqueue_job "gmp_update" "{}" 25)"
  wait_job "$gmp_job_id"

  log "Step 3/4: trigger gmp_history_update"
  trigger_gmp_history

  log "Step 4/4: enqueue fetch_registrar_company_code"
  enqueue_registrar_fetch_jobs

  summary
  log "Bootstrap completed"
}

main "$@"
