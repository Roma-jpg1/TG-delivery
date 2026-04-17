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
- public order flow baseline: `menu -> cart -> checkout draft -> payment session`
- webhook inbox ingestion (`inbox_events`) + worker processing to move order/payment states
- manual review admin endpoints
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

## Public flow API
- `GET /api/v1/menu/branches/{branchID}`
- `GET /api/v1/cart?user_id={uuid}&branch_id={uuid}`
- `POST /api/v1/cart/items`
- `DELETE /api/v1/cart/items/{cartItemID}?user_id={uuid}&branch_id={uuid}`
- `POST /api/v1/checkout/draft`
- `POST /api/v1/payments/sessions`
- `POST /api/v1/webhooks/payments/mock` (header `X-Mock-Payment-Secret`)

## Manual review API
- `GET /api/v1/admin/orders/manual-review`
- `POST /api/v1/admin/orders/{orderID}/manual-review/resolve`

Admin endpoints require header `X-Admin-Token`.

## Key folders
- `internal/app` — API runtime
- `internal/worker` — worker runtime
- `internal/domain/order` — order state machine rules
- `internal/storage/postgres` — DB connection layer
- `internal/transport/httpapi` — HTTP router/handlers/middleware
- `migrations` — SQL schema migrations
- `docs` — architecture notes and roadmap

## Next implementation milestones
1. Telegram Bot integration (`/start`, deep links, order notifications)
2. Mini App + Admin panel frontend
3. Real payment provider adapter (replace mock provider)
4. Delivery/routing module and courier lifecycle
5. Production observability dashboards + SLO automation
