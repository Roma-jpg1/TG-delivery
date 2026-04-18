.PHONY: up down logs test build tidy miniapp-dev admin-dev backup restore

up:
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f api worker postgres

test:
	go test ./...

build:
	go build ./...

tidy:
	go mod tidy

miniapp-dev:
	cd frontend/miniapp && npm install && npm run dev

admin-dev:
	cd frontend/admin && npm install && npm run dev

backup:
	./scripts/backup.sh ./backups

restore:
	@echo "Usage: make restore BACKUP=./backups/file.sql.gz"
	@test -n "$(BACKUP)" && ./scripts/restore.sh "$(BACKUP)" || true
