#!/usr/bin/env bash
set -euo pipefail

MIGRATION_FILE=${1:-./migrations/000001_init.up.sql}
if [[ ! -f "$MIGRATION_FILE" ]]; then
  echo "Migration file not found: $MIGRATION_FILE"
  exit 1
fi

if command -v docker >/dev/null 2>&1 && docker compose ps postgres >/dev/null 2>&1; then
  cat "$MIGRATION_FILE" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d tg_delivery
else
  PGPASSWORD=${PGPASSWORD:-postgres} psql -v ON_ERROR_STOP=1 -h ${PGHOST:-localhost} -p ${PGPORT:-5432} -U ${PGUSER:-postgres} -d ${PGDATABASE:-tg_delivery} -f "$MIGRATION_FILE"
fi

echo "Migration completed from: $MIGRATION_FILE"
