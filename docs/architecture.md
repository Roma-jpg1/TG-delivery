# TG Delivery Architecture (MVP)

## Core principles
1. PostgreSQL is the source of truth.
2. Order lifecycle is a strict state machine.
3. Payments are decoupled from order lifecycle via saga orchestration.
4. External side effects run via outbox/inbox pattern.
5. Webhooks and retries are idempotent.
6. Stop-list is checked at least twice (cart and pre-payment validation).
7. Paid order must either progress or be compensated (refund/manual review).

## Runtime components
- Telegram Bot (entry + notifications)
- Telegram Mini App (menu + cart + checkout)
- Go API service
- Go worker service (outbox/saga/webhook processing)
- PostgreSQL
- Admin panel via API
- Mini App frontend (React/Vite)
- Admin frontend (React/Vite)

## Implemented baseline flow
1. `GET /api/v1/menu/branches/{branchID}`
2. `POST /api/v1/cart/items`
3. `POST /api/v1/checkout/draft`
4. `POST /api/v1/payments/sessions`
5. `POST /api/v1/webhooks/payments/mock`
6. worker processes inbox event and transitions order/payment states
7. `POST /api/v1/webhooks/telegram` and worker handles `/start`, `/orders`, `pre_checkout_query`

## Backend module boundaries
- users
- restaurants / branches
- menu / availability / stop-list
- cart / checkout
- orders (state machine)
- payments / refunds / payment_webhooks / reconciliation
- saga orchestration
- outbox / inbox
- audit / observability

## Reliability
- Worker-based asynchronous processing.
- Dedicated `outbox_events` and `inbox_events` tables.
- Idempotency keys for payment and refund operations.
- Manual review path for ambiguous failures.
