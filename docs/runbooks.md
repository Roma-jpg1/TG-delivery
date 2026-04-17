# Runbooks

## 1) Payment succeeded, order not progressed
1. Find payment by `provider_payment_id`.
2. Check `orders.status` and latest `order_status_history`.
3. Check `inbox_events` record for webhook and processing status.
4. If webhook exists but order not updated:
   - replay inbox event processor for the event ID;
   - if replay fails repeatedly, move order to `manual_review`.
5. Notify operator and customer with corrected status.

## 2) Kitchen rejected after payment
1. Ensure order status is `paid|confirmed`.
2. Trigger refund workflow (`refund_pending`).
3. Verify `refunds.status` and provider refund id.
4. On refund success set `orders.status=refunded`.
5. On repeated refund failure set `manual_review` and alert finance/operator.

## 3) Webhook delivery issues
1. Check HTTP 2xx rate on webhook endpoint.
2. Check signature validation failures.
3. Inspect `inbox_events` growth and duplicate rate.
4. Temporarily switch to polling reconciliation job if provider supports it.

## 4) Stop-list emergency rollback
1. Fetch recent `menu_item_availability_log` for branch/time window.
2. Bulk restore affected items to `available`.
3. Verify menu read model and cart validation behavior.
4. Record incident in postmortem with actor/request identifiers.
