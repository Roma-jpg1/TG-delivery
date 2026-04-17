package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidSecret = errors.New("invalid webhook secret")

type MockPaymentEvent struct {
	EventID           string     `json:"event_id"`
	EventType         string     `json:"event_type"`
	PaymentSessionID  string     `json:"payment_session_id"`
	ProviderPaymentID string     `json:"provider_payment_id"`
	OrderID           *uuid.UUID `json:"order_id,omitempty"`
	Status            string     `json:"status"`
	OccurredAt        *time.Time `json:"occurred_at,omitempty"`
	Raw               any        `json:"raw,omitempty"`
}

type MockPaymentService struct {
	db     *pgxpool.Pool
	secret string
}

func NewMockPaymentService(db *pgxpool.Pool, secret string) *MockPaymentService {
	return &MockPaymentService{db: db, secret: secret}
}

func (s *MockPaymentService) Ingest(ctx context.Context, providedSecret, requestID string, event MockPaymentEvent) error {
	if providedSecret != s.secret {
		return ErrInvalidSecret
	}
	if event.EventID == "" || event.PaymentSessionID == "" || event.Status == "" {
		return fmt.Errorf("invalid webhook payload")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO inbox_events (
			source,
			external_event_id,
			event_type,
			payload,
			headers,
			signature_valid,
			status,
			attempts,
			first_seen_at,
			last_seen_at
		)
		VALUES (
			'mock_payment',
			$1,
			$2,
			$3::jsonb,
			jsonb_build_object('request_id', $4),
			true,
			'received',
			0,
			now(),
			now()
		)
		ON CONFLICT (source, external_event_id)
		DO UPDATE SET
			last_seen_at = now(),
			attempts = inbox_events.attempts + 1
	`, event.EventID, event.EventType, payload, requestID)
	if err != nil {
		return fmt.Errorf("insert inbox event: %w", err)
	}

	return nil
}
