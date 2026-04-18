#!/usr/bin/env bash
set -euo pipefail

BACKUP_FILE=${1:-}
if [[ -z "$BACKUP_FILE" || ! -f "$BACKUP_FILE" ]]; then
  echo "Usage: $0 <backup.sql.gz>"
  exit 1
fi

if command -v docker >/dev/null 2>&1 && docker compose ps postgres >/dev/null 2>&1; then
  gunzip -c "$BACKUP_FILE" | docker compose exec -T postgres psql -U postgres -d tg_delivery
else
  PGPASSWORD=${PGPASSWORD:-postgres} gunzip -c "$BACKUP_FILE" | psql -h ${PGHOST:-localhost} -p ${PGPORT:-5432} -U ${PGUSER:-postgres} -d ${PGDATABASE:-tg_delivery}
fi

echo "Restore completed from: $BACKUP_FILE"
