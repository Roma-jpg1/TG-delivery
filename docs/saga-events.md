# SAGA Events and Flows

## Happy path
1. `OrderDraftCreated`
2. `PaymentRequested`
3. `PaymentSessionCreated`
4. `PaymentSucceeded`
5. `OrderPaid`
6. `KitchenAccepted`
7. `OrderConfirmed`
8. `PreparingStarted`
9. `CourierAssigned`
10. `Delivered`

## Compensation path
1. `PaymentSucceeded`
2. `KitchenRejected`
3. `RefundRequested`
4. `RefundSucceeded`
5. `OrderRefunded`

## Reliability rules
- Every external webhook first lands in `inbox_events` (`UNIQUE(source, external_event_id)`).
- Every side effect after business commit is produced via `outbox_events`.
- Worker retries idempotently with backoff.
- Inconsistent states route to `manual_review`.
