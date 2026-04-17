.PHONY: up down logs test build tidy

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
