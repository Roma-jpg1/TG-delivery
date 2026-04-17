package order

import "fmt"

type Status string

const (
	StatusDraft             Status = "draft"
	StatusPendingPayment    Status = "pending_payment"
	StatusPaymentProcessing Status = "payment_processing"
	StatusPaid              Status = "paid"
	StatusConfirmed         Status = "confirmed"
	StatusPreparing         Status = "preparing"
	StatusReadyForDelivery  Status = "ready_for_delivery"
	StatusOutForDelivery    Status = "out_for_delivery"
	StatusDelivered         Status = "delivered"
	StatusCancelled         Status = "cancelled"
	StatusRefundPending     Status = "refund_pending"
	StatusRefunded          Status = "refunded"
	StatusPaymentFailed     Status = "payment_failed"
	StatusManualReview      Status = "manual_review"
)

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusDraft: {
		StatusPendingPayment: {},
		StatusCancelled:      {},
	},
	StatusPendingPayment: {
		StatusPaymentProcessing: {},
		StatusPaid:              {},
		StatusPaymentFailed:     {},
		StatusCancelled:         {},
	},
	StatusPaymentProcessing: {
		StatusPaid:          {},
		StatusPaymentFailed: {},
		StatusCancelled:     {},
	},
	StatusPaid: {
		StatusConfirmed:     {},
		StatusRefundPending: {},
		StatusManualReview:  {},
	},
	StatusConfirmed: {
		StatusPreparing: {},
		StatusCancelled: {},
	},
	StatusPreparing: {
		StatusReadyForDelivery: {},
		StatusRefundPending:    {},
	},
	StatusReadyForDelivery: {
		StatusOutForDelivery: {},
	},
	StatusOutForDelivery: {
		StatusDelivered: {},
	},
	StatusRefundPending: {
		StatusRefunded:     {},
		StatusManualReview: {},
	},
	StatusManualReview: {
		StatusConfirmed:     {},
		StatusCancelled:     {},
		StatusRefundPending: {},
		StatusRefunded:      {},
	},
}

func CanTransition(from, to Status) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func ValidateTransition(from, to Status) error {
	if from == to {
		return nil
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid order status transition: %s -> %s", from, to)
	}
	return nil
}
