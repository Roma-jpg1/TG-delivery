# Monitoring and Alerts

## Key metrics
- `orders_created_total`
- `orders_paid_total`
- `checkout_duration_ms`
- `orders_pending_payment_total`
- `orders_paid_unconfirmed_total`
- `refund_pending_total`
- `refund_failed_total`
- `outbox_unprocessed_total`
- `inbox_duplicates_total`
- `stop_list_items_total`
- `manual_review_orders_total`

## Alert rules
- High `orders_pending_payment_total` for > 10m.
- `orders_paid_unconfirmed_total` above threshold for > 5m.
- `outbox_unprocessed_total` continuously increasing.
- Webhook signature failures spike.
- Checkout 5xx rate above baseline.
- Paid conversion drop (`cart -> paid`) beyond SLO budget.
