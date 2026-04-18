# API Contracts (Baseline)

## Public API (`/api/v1`)
- `GET /menu/branches/{branchID}`: branch menu with availability and prices.
- `GET /cart?user_id={uuid}`: active cart.
- `POST /cart/items`: add/update item in active cart.
- `DELETE /cart/items/{cartItemID}?user_id={uuid}`: remove item.
- `POST /checkout/draft`: create order draft from cart with server revalidation.
- `POST /payments/sessions`: move order to pending payment and create payment session.
- `POST /webhooks/payments/mock`: webhook ingestion endpoint (inbox dedup).
- `POST /webhooks/telegram`: Telegram update ingestion endpoint (inbox dedup).

## Admin API (`/api/v1/admin`)
- `GET /branches/{branchID}/stop-list`: stop-list list (optional `status`).
- `PUT /branches/{branchID}/menu-items/{menuItemID}/availability`: set item availability.
- `GET /orders/manual-review`: fetch problematic orders.
- `POST /orders/{orderID}/manual-review/resolve`: resolve manual-review decision.

## Request consistency
- Every request returns/accepts `X-Request-ID`.
- All write operations include audit trail metadata in DB (`audit_log`).
- Service metrics are exposed at `GET /metrics`.
