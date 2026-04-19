package refunds

import (
	"context"
	"errors"
	"fmt"
	"time"

	"TG-delivery/internal/domain/order"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound            = errors.New("order not found")
	ErrNoPaymentForRefund       = errors.New("no payment available for refund")
	ErrOrderStatusNotRefundable = errors.New("order status is not refundable")
)

type Refund struct {
	ID               uuid.UUID `json:"id"`
	OrderID          uuid.UUID `json:"order_id"`
	PaymentID        uuid.UUID `json:"payment_id"`
	Provider         string    `json:"provider"`
	ProviderRefundID string    `json:"provider_refund_id,omitempty"`
	Amount           int       `json:"amount"`
	Currency         string    `json:"currency"`
	Status           string    `json:"status"`
	FailureReason    string    `json:"failure_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ListFilter struct {
	BranchID *uuid.UUID
	Status   string
	Limit    int
}

type RequestInput struct {
	OrderID   uuid.UUID
	Reason    string
	ActorType string
	ActorID   *uuid.UUID
	RequestID string
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Refund, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	query := `
		SELECT rf.id, rf.order_id, rf.payment_id, rf.provider,
		       COALESCE(rf.provider_refund_id,''), rf.amount, rf.currency,
		       rf.status::text, COALESCE(rf.failure_reason,''), rf.created_at, rf.updated_at
		FROM refunds rf
		JOIN orders o ON o.id = rf.order_id
		WHERE ($1::uuid IS NULL OR o.branch_id = $1)
		  AND ($2::text = '' OR rf.status::text = $2)
		ORDER BY rf.created_at DESC
		LIMIT $3
	`

	rows, err := s.db.Query(ctx, query, filter.BranchID, filter.Status, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("list refunds: %w", err)
	}
	defer rows.Close()

	out := make([]Refund, 0, filter.Limit)
	for rows.Next() {
		var item Refund
		if err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.PaymentID,
			&item.Provider,
			&item.ProviderRefundID,
			&item.Amount,
			&item.Currency,
			&item.Status,
			&item.FailureReason,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan refund: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refunds: %w", err)
	}

	return out, nil
}

func (s *Service) Request(ctx context.Context, in RequestInput) (Refund, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Refund{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus order.Status
	err = tx.QueryRow(ctx, `SELECT status::text FROM orders WHERE id = $1 FOR UPDATE`, in.OrderID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, ErrOrderNotFound
		}
		return Refund{}, fmt.Errorf("load order for refund request: %w", err)
	}

	if currentStatus != order.StatusPaid && currentStatus != order.StatusConfirmed && currentStatus != order.StatusPreparing && currentStatus != order.StatusManualReview {
		if currentStatus != order.StatusRefundPending && currentStatus != order.StatusRefunded {
			return Refund{}, ErrOrderStatusNotRefundable
		}
	}

	var (
		paymentID uuid.UUID
		amount    int
		currency  string
		provider  string
	)
	err = tx.QueryRow(ctx, `
		SELECT id, amount, currency, provider
		FROM payments
		WHERE order_id = $1 AND status IN ('succeeded', 'refunded')
		ORDER BY created_at DESC
		LIMIT 1
	`, in.OrderID).Scan(&paymentID, &amount, &currency, &provider)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, ErrNoPaymentForRefund
		}
		return Refund{}, fmt.Errorf("load payment for refund: %w", err)
	}

	if currentStatus != order.StatusRefundPending {
		if err := order.ValidateTransition(currentStatus, order.StatusRefundPending); err == nil {
			_, err = tx.Exec(ctx, `
				UPDATE orders SET status = 'refund_pending', updated_at = now(), version = version + 1 WHERE id = $1
			`, in.OrderID)
			if err != nil {
				return Refund{}, fmt.Errorf("move order to refund_pending: %w", err)
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO order_status_history(order_id, from_status, to_status, reason, actor_type, actor_id, metadata, created_at)
				VALUES ($1, $2::order_status, 'refund_pending', 'manual_refund_requested', $3, $4, jsonb_build_object('reason', $5::text), now())
			`, in.OrderID, currentStatus, in.ActorType, in.ActorID, in.Reason)
			if err != nil {
				return Refund{}, fmt.Errorf("insert refund_pending history: %w", err)
			}
		}
	}

	idempotencyKey := fmt.Sprintf("refund_%s", in.OrderID)
	var refund Refund
	err = tx.QueryRow(ctx, `
		INSERT INTO refunds(
			order_id, payment_id, provider, idempotency_key, amount, currency, status, raw_payload, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', jsonb_build_object('reason', $7::text), now(), now())
		ON CONFLICT (idempotency_key)
		DO UPDATE SET updated_at = now()
		RETURNING id, order_id, payment_id, provider, COALESCE(provider_refund_id,''), amount, currency, status::text, COALESCE(failure_reason,''), created_at, updated_at
	`, in.OrderID, paymentID, provider, idempotencyKey, amount, currency, in.Reason).Scan(
		&refund.ID,
		&refund.OrderID,
		&refund.PaymentID,
		&refund.Provider,
		&refund.ProviderRefundID,
		&refund.Amount,
		&refund.Currency,
		&refund.Status,
		&refund.FailureReason,
		&refund.CreatedAt,
		&refund.UpdatedAt,
	)
	if err != nil {
		return Refund{}, fmt.Errorf("upsert refund: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events(
			aggregate_type, aggregate_id, event_type, payload, headers, created_at
		)
		VALUES('refund', $1, 'RefundRequested', jsonb_build_object('refund_id',$1::text,'order_id',$2::text), jsonb_build_object('request_id',$3::text), now())
	`, refund.ID, in.OrderID, in.RequestID)
	if err != nil {
		return Refund{}, fmt.Errorf("insert refund outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Refund{}, fmt.Errorf("commit refund request: %w", err)
	}

	return refund, nil
}
