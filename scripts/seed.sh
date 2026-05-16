#!/usr/bin/env bash
set -euo pipefail

SQL_FILE=${1:-./scripts/seed_demo.sql}
if [[ ! -f "$SQL_FILE" ]]; then
  echo "SQL file not found: $SQL_FILE"
  exit 1
fi

if command -v docker >/dev/null 2>&1 && docker compose ps postgres >/dev/null 2>&1; then
  cat "$SQL_FILE" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d tg_delivery
else
  PGPASSWORD=${PGPASSWORD:-postgres} psql -v ON_ERROR_STOP=1 -h ${PGHOST:-localhost} -p ${PGPORT:-5432} -U ${PGUSER:-postgres} -d ${PGDATABASE:-tg_delivery} -f "$SQL_FILE"
fi

echo "Seed completed from: $SQL_FILE"
