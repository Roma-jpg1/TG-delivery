package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/domain/order"
	"TG-delivery/internal/storage/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	cfg    config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
}

type mockPaymentEvent struct {
	EventID           string     `json:"event_id"`
	EventType         string     `json:"event_type"`
	PaymentSessionID  string     `json:"payment_session_id"`
	ProviderPaymentID string     `json:"provider_payment_id"`
	OrderID           *uuid.UUID `json:"order_id,omitempty"`
	Status            string     `json:"status"`
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Worker, error) {
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	return &Worker{cfg: cfg, logger: logger, db: db}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.db.Close()

	w.logger.Info("worker started", "poll_interval", w.cfg.Worker.OutboxPollInterval.String(), "batch_size", w.cfg.Worker.BatchSize)
	ticker := time.NewTicker(w.cfg.Worker.OutboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return nil
		case <-ticker.C:
			if err := w.processInboxBatch(ctx, w.cfg.Worker.BatchSize); err != nil {
				w.logger.Error("process inbox batch failed", "error", err)
			}
			if err := w.processOutboxBatch(ctx, w.cfg.Worker.BatchSize); err != nil {
				w.logger.Error("process outbox batch failed", "error", err)
			}
			if err := w.autoRestoreAvailability(ctx); err != nil {
				w.logger.Error("auto restore availability failed", "error", err)
			}
		}
	}
}

func (w *Worker) processInboxBatch(ctx context.Context, batchSize int) error {
	rows, err := w.db.Query(ctx, `
		SELECT id
		FROM inbox_events
		WHERE source = 'mock_payment'
		  AND processed_at IS NULL
		ORDER BY first_seen_at
		LIMIT $1
	`, batchSize)
	if err != nil {
		return fmt.Errorf("query inbox batch ids: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0, batchSize)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan inbox id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate inbox ids: %w", err)
	}

	for _, id := range ids {
		if err := w.processSingleInboxEvent(ctx, id); err != nil {
			w.logger.Warn("inbox event processing failed", "inbox_event_id", id, "error", err)
			if markErr := w.markInboxFailed(ctx, id, err.Error()); markErr != nil {
				w.logger.Error("mark inbox failed failed", "inbox_event_id", id, "error", markErr)
			}
		}
	}

	return nil
}

func (w *Worker) processSingleInboxEvent(ctx context.Context, inboxID uuid.UUID) error {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin inbox tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var payloadBytes []byte
	var processedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT payload, processed_at
		FROM inbox_events
		WHERE id = $1
		FOR UPDATE
	`, inboxID).Scan(&payloadBytes, &processedAt)
	if err != nil {
		return fmt.Errorf("load inbox event: %w", err)
	}
	if processedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit already processed inbox tx: %w", err)
		}
		return nil
	}

	var event mockPaymentEvent
	if err := json.Unmarshal(payloadBytes, &event); err != nil {
		return fmt.Errorf("unmarshal inbox payload: %w", err)
	}

	if event.PaymentSessionID == "" {
		return fmt.Errorf("invalid inbox payload: missing payment_session_id")
	}

	var paymentID uuid.UUID
	var orderID uuid.UUID
	var paymentStatus string
	err = tx.QueryRow(ctx, `
		SELECT id, order_id, status::text
		FROM payments
		WHERE provider = 'mock' AND provider_session_id = $1
		FOR UPDATE
	`, event.PaymentSessionID).Scan(&paymentID, &orderID, &paymentStatus)
	if err != nil {
		return fmt.Errorf("load payment by session: %w", err)
	}

	normalized := normalizePaymentStatus(event.Status)
	switch normalized {
	case "succeeded":
		if paymentStatus != "succeeded" {
			_, err = tx.Exec(ctx, `
				UPDATE payments
				SET status = 'succeeded',
				    provider_payment_id = COALESCE(NULLIF($2, ''), provider_payment_id),
				    raw_payload = $3::jsonb,
				    updated_at = now(),
				    version = version + 1
				WHERE id = $1
			`, paymentID, event.ProviderPaymentID, payloadBytes)
			if err != nil {
				return fmt.Errorf("mark payment succeeded: %w", err)
			}
		}

		if err := w.moveOrderToPaid(ctx, tx, orderID); err != nil {
			return err
		}

	case "failed", "cancelled":
		if paymentStatus != "failed" && paymentStatus != "cancelled" {
			_, err = tx.Exec(ctx, `
				UPDATE payments
				SET status = $2::payment_status,
				    provider_payment_id = COALESCE(NULLIF($3, ''), provider_payment_id),
				    raw_payload = $4::jsonb,
				    failure_reason = 'webhook_reported_failure',
				    updated_at = now(),
				    version = version + 1
				WHERE id = $1
			`, paymentID, normalized, event.ProviderPaymentID, payloadBytes)
			if err != nil {
				return fmt.Errorf("mark payment failed/cancelled: %w", err)
			}
		}
		if err := w.moveOrderToPaymentFailed(ctx, tx, orderID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported payment status: %s", normalized)
	}

	_, err = tx.Exec(ctx, `
		UPDATE inbox_events
		SET status = 'processed',
		    processed_at = now(),
		    attempts = attempts + 1,
		    last_error = NULL,
		    last_seen_at = now()
		WHERE id = $1
	`, inboxID)
	if err != nil {
		return fmt.Errorf("mark inbox processed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit inbox tx: %w", err)
	}

	return nil
}

func (w *Worker) moveOrderToPaid(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	var currentStatus order.Status
	err := tx.QueryRow(ctx, `
		SELECT status::text
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("load order for paid transition: %w", err)
	}

	if currentStatus == order.StatusPaid {
		return nil
	}
	if err := order.ValidateTransition(currentStatus, order.StatusPaid); err != nil {
		return fmt.Errorf("invalid transition to paid: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $2::order_status,
		    placed_at = COALESCE(placed_at, now()),
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
	`, orderID, order.StatusPaid)
	if err != nil {
		return fmt.Errorf("update order to paid: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (
			order_id,
			from_status,
			to_status,
			reason,
			actor_type,
			metadata,
			created_at
		)
		VALUES ($1, $2::order_status, $3::order_status, 'payment_succeeded_webhook', 'system', '{}'::jsonb, now())
	`, orderID, currentStatus, order.StatusPaid)
	if err != nil {
		return fmt.Errorf("insert order paid history: %w", err)
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
			'OrderPaid',
			jsonb_build_object('order_id', $1::text),
			'{}'::jsonb,
			now()
		)
	`, orderID)
	if err != nil {
		return fmt.Errorf("insert order paid outbox event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO saga_instances (
			saga_type,
			entity_type,
			entity_id,
			status,
			current_step,
			started_at,
			created_at,
			updated_at
		)
		VALUES ('order_checkout', 'order', $1, 'running', 'payment_succeeded', now(), now(), now())
		ON CONFLICT (saga_type, entity_type, entity_id)
		DO NOTHING
	`, orderID)
	if err != nil {
		return fmt.Errorf("upsert saga instance: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO saga_steps (
			saga_instance_id,
			step_name,
			status,
			attempt,
			input_payload,
			output_payload,
			created_at,
			updated_at
		)
		SELECT si.id, 'payment_succeeded', 'completed', 0, '{}'::jsonb, '{}'::jsonb, now(), now()
		FROM saga_instances si
		WHERE si.saga_type = 'order_checkout' AND si.entity_type = 'order' AND si.entity_id = $1
		ON CONFLICT (saga_instance_id, step_name, attempt)
		DO NOTHING
	`, orderID)
	if err != nil {
		return fmt.Errorf("insert saga step: %w", err)
	}

	return nil
}

func (w *Worker) moveOrderToPaymentFailed(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	var currentStatus order.Status
	err := tx.QueryRow(ctx, `
		SELECT status::text
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("load order for payment_failed transition: %w", err)
	}

	if currentStatus == order.StatusPaymentFailed {
		return nil
	}
	if err := order.ValidateTransition(currentStatus, order.StatusPaymentFailed); err != nil {
		return fmt.Errorf("invalid transition to payment_failed: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $2::order_status,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
	`, orderID, order.StatusPaymentFailed)
	if err != nil {
		return fmt.Errorf("update order to payment_failed: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (
			order_id,
			from_status,
			to_status,
			reason,
			actor_type,
			metadata,
			created_at
		)
		VALUES ($1, $2::order_status, $3::order_status, 'payment_failed_webhook', 'system', '{}'::jsonb, now())
	`, orderID, currentStatus, order.StatusPaymentFailed)
	if err != nil {
		return fmt.Errorf("insert payment_failed history: %w", err)
	}

	return nil
}

func (w *Worker) markInboxFailed(ctx context.Context, inboxID uuid.UUID, cause string) error {
	_, err := w.db.Exec(ctx, `
		UPDATE inbox_events
		SET status = 'failed',
		    attempts = attempts + 1,
		    last_error = $2,
		    last_seen_at = now()
		WHERE id = $1
	`, inboxID, cause)
	if err != nil {
		return fmt.Errorf("mark inbox failed: %w", err)
	}
	return nil
}

func (w *Worker) processOutboxBatch(ctx context.Context, batchSize int) error {
	rows, err := w.db.Query(ctx, `
		SELECT id, event_type
		FROM outbox_events
		WHERE processed_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, batchSize)
	if err != nil {
		return fmt.Errorf("query outbox batch: %w", err)
	}
	defer rows.Close()

	type item struct {
		id        uuid.UUID
		eventType string
	}
	events := make([]item, 0, batchSize)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.eventType); err != nil {
			return fmt.Errorf("scan outbox row: %w", err)
		}
		events = append(events, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate outbox rows: %w", err)
	}

	for _, ev := range events {
		_, err := w.db.Exec(ctx, `
			UPDATE outbox_events
			SET processed_at = now(),
			    attempts = attempts + 1,
			    locked_at = NULL,
			    last_error = NULL
			WHERE id = $1 AND processed_at IS NULL
		`, ev.id)
		if err != nil {
			return fmt.Errorf("mark outbox event processed: %w", err)
		}
		w.logger.Debug("outbox event dispatched", "event_id", ev.id, "event_type", ev.eventType)
	}

	return nil
}

func (w *Worker) autoRestoreAvailability(ctx context.Context) error {
	_, err := w.db.Exec(ctx, `
		UPDATE branch_menu_items
		SET status = 'available',
		    available_until = NULL,
		    reason = 'auto_restored_by_worker',
		    updated_at = now(),
		    version = version + 1
		WHERE status = 'out_of_stock'
		  AND available_until IS NOT NULL
		  AND available_until <= now()
	`)
	if err != nil {
		return fmt.Errorf("auto restore availability query failed: %w", err)
	}
	return nil
}

func normalizePaymentStatus(status string) string {
	switch status {
	case "succeeded", "success", "paid":
		return "succeeded"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return "failed"
	}
}
