#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR=${1:-./backups}
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
mkdir -p "$OUTPUT_DIR"

BACKUP_FILE="$OUTPUT_DIR/tg_delivery_${TIMESTAMP}.sql.gz"

if command -v docker >/dev/null 2>&1 && docker compose ps postgres >/dev/null 2>&1; then
  docker compose exec -T postgres pg_dump -U postgres -d tg_delivery | gzip > "$BACKUP_FILE"
else
  PGPASSWORD=${PGPASSWORD:-postgres} pg_dump -h ${PGHOST:-localhost} -p ${PGPORT:-5432} -U ${PGUSER:-postgres} -d ${PGDATABASE:-tg_delivery} | gzip > "$BACKUP_FILE"
fi

echo "Backup created: $BACKUP_FILE"
