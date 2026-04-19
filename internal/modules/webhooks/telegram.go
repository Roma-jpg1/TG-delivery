package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidTelegramSecret = errors.New("invalid telegram webhook secret")

type TelegramUpdate struct {
	UpdateID int64           `json:"update_id"`
	Raw      json.RawMessage `json:"-"`
}

type TelegramService struct {
	db     *pgxpool.Pool
	secret string
}

func NewTelegramService(db *pgxpool.Pool, secret string) *TelegramService {
	return &TelegramService{db: db, secret: secret}
}

func (s *TelegramService) Ingest(ctx context.Context, providedSecret, requestID string, body []byte) error {
	if providedSecret != s.secret {
		return ErrInvalidTelegramSecret
	}

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return fmt.Errorf("decode telegram update: %w", err)
	}
	if update.UpdateID == 0 {
		return fmt.Errorf("telegram update_id is required")
	}

	_, err := s.db.Exec(ctx, `
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
			'telegram',
			$1,
			'update',
			$2::jsonb,
			jsonb_build_object('request_id', $3::text),
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
	`, fmt.Sprintf("%d", update.UpdateID), body, requestID)
	if err != nil {
		return fmt.Errorf("store telegram inbox event: %w", err)
	}

	return nil
}
