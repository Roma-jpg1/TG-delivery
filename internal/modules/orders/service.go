package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"TG-delivery/internal/domain/order"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrInvalidAction = errors.New("invalid manual review action")
)

type ManualReviewOrder struct {
	OrderID     uuid.UUID `json:"order_id"`
	OrderNumber int64     `json:"order_number"`
	BranchID    uuid.UUID `json:"branch_id"`
	UserID      uuid.UUID `json:"user_id"`
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Currency    string    `json:"currency"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ResolveManualReviewInput struct {
	OrderID   uuid.UUID
	Action    string
	Reason    string
	ActorID   *uuid.UUID
	ActorType string
	RequestID string
}

type ResolveResult struct {
	OrderID   uuid.UUID `json:"order_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) ListManualReview(ctx context.Context, branchID *uuid.UUID) ([]ManualReviewOrder, error) {
	var rows pgx.Rows
	var err error
	if branchID != nil {
		rows, err = s.db.Query(ctx, `
			SELECT id, order_number, branch_id, user_id, status::text, total, currency, updated_at
			FROM orders
			WHERE status = 'manual_review' AND branch_id = $1
			ORDER BY updated_at DESC
		`, *branchID)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT id, order_number, branch_id, user_id, status::text, total, currency, updated_at
			FROM orders
			WHERE status = 'manual_review'
			ORDER BY updated_at DESC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("query manual review orders: %w", err)
	}
	defer rows.Close()

	result := make([]ManualReviewOrder, 0, 64)
	for rows.Next() {
		var row ManualReviewOrder
		if err := rows.Scan(&row.OrderID, &row.OrderNumber, &row.BranchID, &row.UserID, &row.Status, &row.Total, &row.Currency, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan manual review order: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manual review orders: %w", err)
	}

	return result, nil
}

func (s *Service) ResolveManualReview(ctx context.Context, in ResolveManualReviewInput) (ResolveResult, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	var target order.Status
	switch action {
	case "confirm":
		target = order.StatusConfirmed
	case "cancel":
		target = order.StatusCancelled
	case "refund":
		target = order.StatusRefundPending
	default:
		return ResolveResult{}, ErrInvalidAction
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ResolveResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fromStatus order.Status
	err = tx.QueryRow(ctx, `
		SELECT status::text
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, in.OrderID).Scan(&fromStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResolveResult{}, ErrOrderNotFound
		}
		return ResolveResult{}, fmt.Errorf("load order for manual review resolution: %w", err)
	}

	if err := order.ValidateTransition(fromStatus, target); err != nil {
		return ResolveResult{}, fmt.Errorf("invalid manual review transition: %w", err)
	}

	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE orders
		SET status = $2::order_status,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
		RETURNING updated_at
	`, in.OrderID, target).Scan(&updatedAt)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("update order status from manual review: %w", err)
	}

	metadata, err := json.Marshal(map[string]any{
		"reason": in.Reason,
		"action": action,
	})
	if err != nil {
		return ResolveResult{}, fmt.Errorf("marshal manual review metadata: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (
			order_id,
			from_status,
			to_status,
			reason,
			actor_type,
			actor_id,
			metadata,
			created_at
		)
		VALUES (
			$1,
			$2::order_status,
			$3::order_status,
			'manual_review_resolution',
			$4,
			$5,
			$6::jsonb,
			now()
		)
	`, in.OrderID, fromStatus, target, in.ActorType, in.ActorID, metadata)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("insert manual review status history: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			headers,
			created_at
		)
		VALUES (
			'order',
			$1,
			$2,
			jsonb_build_object('order_id', $1::text, 'from_status', $3::text, 'to_status', $4::text, 'reason', $5),
			jsonb_build_object('request_id', $6),
			now()
		)
	`, in.OrderID, "ManualReviewResolved", fromStatus, target, in.Reason, in.RequestID)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("insert manual review outbox event: %w", err)
	}

	if target == order.StatusRefundPending {
		var paymentID uuid.UUID
		var amount int
		var currency string
		err = tx.QueryRow(ctx, `
			SELECT id, amount, currency
			FROM payments
			WHERE order_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		`, in.OrderID).Scan(&paymentID, &amount, &currency)
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO refunds (
					order_id,
					payment_id,
					provider,
					idempotency_key,
					amount,
					currency,
					status,
					created_at,
					updated_at
				)
				VALUES (
					$1,
					$2,
					'mock',
					$3,
					$4,
					$5,
					'pending',
					now(),
					now()
				)
				ON CONFLICT (idempotency_key) DO NOTHING
			`, in.OrderID, paymentID, fmt.Sprintf("manual_refund_%s", in.OrderID), amount, currency)
			if err != nil {
				return ResolveResult{}, fmt.Errorf("create pending refund: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ResolveResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return ResolveResult{OrderID: in.OrderID, Status: string(target), UpdatedAt: updatedAt}, nil
}
