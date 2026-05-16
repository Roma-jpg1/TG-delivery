# Delivery Website

MVP foundation for a restaurant ordering system:
- Web storefront as the primary client channel
- Optional Telegram Bot integration
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
- addresses API + delivery quote API
- admin payment/refund APIs + manual refund request
- worker reconciliation for paid orders and pending refunds
- React delivery website scaffold (`frontend/miniapp`)
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
   - Website dev server: `http://localhost:5173`
   - Admin dev server: `http://localhost:5174`
6. Seed demo data:
   - `make seed`

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
- `GET /api/v1/addresses?user_id={uuid}`
- `POST /api/v1/addresses`
- `DELETE /api/v1/addresses/{addressID}?user_id={uuid}`
- `POST /api/v1/delivery/quote`
- `POST /api/v1/webhooks/payments/mock` (header `X-Mock-Payment-Secret`)
- `POST /api/v1/webhooks/telegram` (header `X-Telegram-Bot-Api-Secret-Token`)

## Manual review API
- `GET /api/v1/admin/orders/manual-review`
- `POST /api/v1/admin/orders/{orderID}/manual-review/resolve`
- `GET /api/v1/admin/payments`
- `GET /api/v1/admin/refunds`
- `POST /api/v1/admin/orders/{orderID}/refunds`

Admin endpoints require header `X-Admin-Token`.

## Telegram bot behavior (worker side)
- `/start` sends website order button.
- `/orders` sends latest orders summary.
- `pre_checkout_query` is validated against current availability and order amount/currency.
- On order events (`OrderPaid`, manual review resolution), customer notifications are sent when `users.telegram_user_id` exists.

## Backup and restore
- Backup: `make backup`
- Restore: `make restore BACKUP=./backups/file.sql.gz`
- See details: `docs/backup-restore.md`

## Demo IDs
- Branch: `11111111-1111-1111-1111-111111111111`
- User: `22222222-2222-2222-2222-222222222222`
- Seed script: `scripts/seed_demo.sql`

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
2. Courier assignment lifecycle and external delivery providers
3. Telegram bot command expansion (`/help`, support routing, deep links with campaign params)
4. Full admin domain CRUD (menu/catalog/branches/users/RBAC UI)
5. Production observability dashboards + SLO automation + incident automation
