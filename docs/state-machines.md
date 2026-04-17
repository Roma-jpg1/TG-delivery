# State Machines

## Order status
- `draft`
- `pending_payment`
- `payment_processing`
- `paid`
- `confirmed`
- `preparing`
- `ready_for_delivery`
- `out_for_delivery`
- `delivered`
- `cancelled`
- `refund_pending`
- `refunded`
- `payment_failed`
- `manual_review`

## Allowed transitions
- `draft -> pending_payment | cancelled`
- `pending_payment -> payment_processing | paid | payment_failed | cancelled`
- `payment_processing -> paid | payment_failed | cancelled`
- `paid -> confirmed | refund_pending | manual_review`
- `confirmed -> preparing | cancelled`
- `preparing -> ready_for_delivery | refund_pending`
- `ready_for_delivery -> out_for_delivery`
- `out_for_delivery -> delivered`
- `refund_pending -> refunded | manual_review`

## Payment status
- `created -> pending -> succeeded | failed | cancelled`
- `succeeded -> refunded` (full refund path)
- any ambiguous failure case -> `manual_review`

## Refund status
- `pending -> succeeded | failed | manual_review`
