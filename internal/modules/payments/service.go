package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"TG-delivery/internal/domain/order"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderInvalidStatus = errors.New("order status does not allow payment session")
)

type CreateSessionInput struct {
	OrderID        uuid.UUID
	Provider       string
	IdempotencyKey string
	RequestID      string
}

type Session struct {
	PaymentID         uuid.UUID `json:"payment_id"`
	Provider          string    `json:"provider"`
	ProviderSessionID string    `json:"provider_session_id"`
	CheckoutURL       string    `json:"checkout_url"`
	Amount            int       `json:"amount"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) CreateSession(ctx context.Context, in CreateSessionInput) (Session, error) {
	if in.Provider == "" {
		in.Provider = "mock"
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = uuid.NewString()
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existing Session
	err = tx.QueryRow(ctx, `
		SELECT id, provider, provider_session_id, amount, currency, status::text, created_at
		FROM payments
		WHERE idempotency_key = $1
	`, in.IdempotencyKey).Scan(
		&existing.PaymentID,
		&existing.Provider,
		&existing.ProviderSessionID,
		&existing.Amount,
		&existing.Currency,
		&existing.Status,
		&existing.CreatedAt,
	)
	if err == nil {
		existing.CheckoutURL = fmt.Sprintf("https://pay.mock.local/session/%s", existing.ProviderSessionID)
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return Session{}, fmt.Errorf("commit idempotent transaction: %w", commitErr)
		}
		return existing, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, fmt.Errorf("check idempotent payment: %w", err)
	}

	var orderStatus order.Status
	var amount int
	var currency string
	err = tx.QueryRow(ctx, `
		SELECT status::text, total, currency
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, in.OrderID).Scan(&orderStatus, &amount, &currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrOrderNotFound
		}
		return Session{}, fmt.Errorf("load order for payment: %w", err)
	}

	if orderStatus != order.StatusDraft && orderStatus != order.StatusPendingPayment {
		return Session{}, fmt.Errorf("%w: %s", ErrOrderInvalidStatus, orderStatus)
	}

	if orderStatus == order.StatusDraft {
		if err := order.ValidateTransition(order.StatusDraft, order.StatusPendingPayment); err != nil {
			return Session{}, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET status = $2::order_status,
			    updated_at = now(),
			    version = version + 1
			WHERE id = $1
		`, in.OrderID, order.StatusPendingPayment)
		if err != nil {
			return Session{}, fmt.Errorf("move order to pending_payment: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (
				order_id, from_status, to_status, reason, actor_type, metadata, created_at
			)
			VALUES ($1, $2::order_status, $3::order_status, 'payment_session_requested', 'system', '{}'::jsonb, now())
		`, in.OrderID, order.StatusDraft, order.StatusPendingPayment)
		if err != nil {
			return Session{}, fmt.Errorf("insert order pending_payment history: %w", err)
		}
	}

	providerSessionID := uuid.NewString()
	providerPaymentID := fmt.Sprintf("mockpay_%s", uuid.NewString())

	var paymentID uuid.UUID
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (
			order_id,
			provider,
			provider_payment_id,
			provider_session_id,
			idempotency_key,
			amount,
			currency,
			status,
			request_payload,
			response_payload,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, 'pending',
			jsonb_build_object('request_id', $8),
			jsonb_build_object('checkout_url', $9),
			now(), now()
		)
		RETURNING id, created_at
	`, in.OrderID, in.Provider, providerPaymentID, providerSessionID, in.IdempotencyKey, amount, currency, in.RequestID, fmt.Sprintf("https://pay.mock.local/session/%s", providerSessionID)).Scan(&paymentID, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Session{}, fmt.Errorf("idempotency violation: %w", err)
		}
		return Session{}, fmt.Errorf("insert payment session: %w", err)
	}

	paymentRequestedPayload, err := json.Marshal(map[string]any{
		"order_id":            in.OrderID,
		"payment_id":          paymentID,
		"provider":            in.Provider,
		"provider_session_id": providerSessionID,
		"amount":              amount,
		"currency":            currency,
	})
	if err != nil {
		return Session{}, fmt.Errorf("marshal payment outbox payload: %w", err)
	}

	for _, eventType := range []string{"PaymentRequested", "PaymentSessionCreated"} {
		_, err = tx.Exec(ctx, `
			INSERT INTO outbox_events (
				aggregate_type,
				aggregate_id,
				event_type,
				payload,
				headers,
				created_at
			)
			VALUES ('payment', $1, $2, $3::jsonb, jsonb_build_object('request_id', $4), now())
		`, paymentID, eventType, paymentRequestedPayload, in.RequestID)
		if err != nil {
			return Session{}, fmt.Errorf("insert payment outbox event %s: %w", eventType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit transaction: %w", err)
	}

	return Session{
		PaymentID:         paymentID,
		Provider:          in.Provider,
		ProviderSessionID: providerSessionID,
		CheckoutURL:       fmt.Sprintf("https://pay.mock.local/session/%s", providerSessionID),
		Amount:            amount,
		Currency:          currency,
		Status:            "pending",
		CreatedAt:         createdAt,
	}, nil
}
