# Backup and Restore

## Create backup
```bash
./scripts/backup.sh ./backups
```

## Restore backup
```bash
./scripts/restore.sh ./backups/tg_delivery_YYYYMMDD_HHMMSS.sql.gz
```

## Production recommendations
- Keep encrypted backups in remote object storage.
- Run backup integrity checks and periodic restore drills.
- Track backup age and restore duration as SLO indicators.
