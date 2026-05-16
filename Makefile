.PHONY: up down logs test build tidy website-dev miniapp-dev admin-dev backup restore migrate seed

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

website-dev:
	cd frontend/miniapp && npm install && npm run dev

miniapp-dev:
	$(MAKE) website-dev

admin-dev:
	cd frontend/admin && npm install && npm run dev

backup:
	bash ./scripts/backup.sh ./backups

restore:
	@echo "Usage: make restore BACKUP=./backups/file.sql.gz"
	@test -n "$(BACKUP)" && bash ./scripts/restore.sh "$(BACKUP)" || true

migrate:
	bash ./scripts/migrate.sh ./migrations/000001_init.up.sql

seed:
	bash ./scripts/seed.sh ./scripts/seed_demo.sql
