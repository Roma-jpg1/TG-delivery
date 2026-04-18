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
- metrics endpoint (`/metrics`)
- initial PostgreSQL migration with core business tables
- stop-list API with transactional audit/outbox writes
- public order flow baseline: `menu -> cart -> checkout draft -> payment session`
- webhook inbox ingestion (`inbox_events`) + worker processing to move order/payment states
- Telegram webhook ingestion + worker handling (`/start`, `/orders`, pre-checkout validation)
- manual review admin endpoints
- React Mini App scaffold (`frontend/miniapp`)
- React Admin panel scaffold (`frontend/admin`)
- docker compose for local infrastructure

## Quick start
1. Copy env if needed:
   - `.env.example` is used by compose as default.
2. Start stack:
   - `make up`
3. API health check:
   - `GET http://localhost:18080/health/ready`
4. Metrics:
   - `GET http://localhost:18080/metrics`
5. Frontends:
   - Mini App dev server: `http://localhost:5173`
   - Admin dev server: `http://localhost:5174`

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

## Public flow API
- `GET /api/v1/menu/branches/{branchID}`
- `GET /api/v1/cart?user_id={uuid}&branch_id={uuid}`
- `POST /api/v1/cart/items`
- `DELETE /api/v1/cart/items/{cartItemID}?user_id={uuid}&branch_id={uuid}`
- `POST /api/v1/checkout/draft`
- `POST /api/v1/payments/sessions`
- `POST /api/v1/webhooks/payments/mock` (header `X-Mock-Payment-Secret`)
- `POST /api/v1/webhooks/telegram` (header `X-Telegram-Bot-Api-Secret-Token`)

## Manual review API
- `GET /api/v1/admin/orders/manual-review`
- `POST /api/v1/admin/orders/{orderID}/manual-review/resolve`

Admin endpoints require header `X-Admin-Token`.

## Telegram bot behavior (worker side)
- `/start` sends Mini App button.
- `/orders` sends latest orders summary.
- `pre_checkout_query` is validated against current availability and order amount/currency.
- On order events (`OrderPaid`, manual review resolution), customer notifications are sent when `users.telegram_user_id` exists.

## Backup and restore
- Backup: `make backup`
- Restore: `make restore BACKUP=./backups/file.sql.gz`
- See details: `docs/backup-restore.md`

## Key folders
- `internal/app` — API runtime
- `internal/worker` — worker runtime
- `internal/domain/order` — order state machine rules
- `internal/storage/postgres` — DB connection layer
- `internal/transport/httpapi` — HTTP router/handlers/middleware
- `migrations` — SQL schema migrations
- `docs` — architecture notes and roadmap

## Next implementation milestones
1. Real payment provider adapter (replace mock provider)
2. Delivery/routing module and courier lifecycle
3. Telegram bot command expansion (`/help`, support routing, deep links with campaign params)
4. Full admin domain CRUD (menu/catalog/branches/users/RBAC UI)
5. Production observability dashboards + SLO automation + incident automation
