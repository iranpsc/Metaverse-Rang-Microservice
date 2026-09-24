#!/bin/sh
# Wait for MySQL, then run shared/cmd/migrate (default: up).
set -eu

MIGRATIONS_PATH="${MIGRATIONS_PATH:-/migrations}"
DB_HOST="${DB_HOST:-mysql}"
DB_PORT="${DB_PORT:-3306}"
WAIT_SECONDS="${MYSQL_WAIT_SECONDS:-90}"

if [ "${SKIP_MIGRATE:-0}" = "1" ] || [ "${SKIP_MIGRATE:-0}" = "true" ]; then
  echo "⏭️  SKIP_MIGRATE=${SKIP_MIGRATE}: skipping migrations"
  exit 0
fi

echo "⏳ Waiting for MySQL at ${DB_HOST}:${DB_PORT} (up to ${WAIT_SECONDS}s)..."
deadline=$(( $(date +%s) + WAIT_SECONDS ))
while true; do
  if nc -z -w 1 "${DB_HOST}" "${DB_PORT}" >/dev/null 2>&1; then
    echo "✅ MySQL port is open"
    break
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "❌ MySQL did not become ready within ${WAIT_SECONDS}s" >&2
    exit 1
  fi
  sleep 2
done

cmd="up"
if [ "$#" -gt 0 ]; then
  cmd="$1"
  shift
fi

force_args=""
if [ "${APP_ENV:-}" = "production" ] || [ "${MIGRATE_FORCE:-0}" = "1" ] || [ "${MIGRATE_FORCE:-0}" = "true" ]; then
  force_args="-force"
fi

echo "▶️  Running: migrate ${cmd} -path=${MIGRATIONS_PATH} ${force_args} $*"
# shellcheck disable=SC2086
exec migrate "${cmd}" -path="${MIGRATIONS_PATH}" ${force_args} "$@"
