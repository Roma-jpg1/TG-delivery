# TG Delivery

MVP foundation for a Telegram-first restaurant ordering system:
- Telegram Bot + Mini App as client channels
- Go backend API (modular monolith)
- Go worker for async processing (outbox/saga/webhooks)
- PostgreSQL as source of truth

## Current state
This repository contains:
- service runtime skeleton (API + worker modes)
- structured logging
- request correlation (`X-Request-ID`)
- health endpoints (`/health/live`, `/health/ready`)
- initial PostgreSQL migration with core business tables
- stop-list API with transactional audit/outbox writes
- docker compose for local infrastructure

## Quick start
1. Copy env if needed:
   - `.env.example` is used by compose as default.
2. Start stack:
   - `make up`
3. API health check:
   - `GET http://localhost:18080/health/ready`

## Run modes
Single binary supports two roles via `APP_ROLE`:
- `api` (default)
- `worker`

## Admin stop-list API
- `GET /api/v1/admin/branches/{branchID}/stop-list?status=out_of_stock`
  - returns current non-available items for a branch
- `PUT /api/v1/admin/branches/{branchID}/menu-items/{menuItemID}/availability`
  - updates branch availability status and writes:
    - `menu_item_availability_log`
    - `audit_log`
    - `outbox_events`

## Key folders
- `internal/app` — API runtime
- `internal/worker` — worker runtime
- `internal/domain/order` — order state machine rules
- `internal/storage/postgres` — DB connection layer
- `internal/transport/httpapi` — HTTP router/handlers/middleware
- `migrations` — SQL schema migrations
- `docs` — architecture notes and roadmap

## Next implementation milestones
1. Menu CRUD + branch pricing API
2. Cart/checkout server-side revalidation
3. Payment provider integration + webhook inbox
4. Saga orchestrator + compensation flow
5. Telegram bot and Mini App contracts
