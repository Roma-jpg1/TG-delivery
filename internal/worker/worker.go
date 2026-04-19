package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/domain/order"
	"TG-delivery/internal/modules/telegrambot"
	"TG-delivery/internal/storage/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	cfg    config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	bot    *telegrambot.Client
}

type mockPaymentEvent struct {
	EventID           string     `json:"event_id"`
	EventType         string     `json:"event_type"`
	PaymentSessionID  string     `json:"payment_session_id"`
	ProviderPaymentID string     `json:"provider_payment_id"`
	OrderID           *uuid.UUID `json:"order_id,omitempty"`
	Status            string     `json:"status"`
}

type inboxEvent struct {
	ID        uuid.UUID
	Source    string
	Payload   []byte
	Processed *time.Time
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Worker, error) {
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	return &Worker{
		cfg:    cfg,
		logger: logger,
		db:     db,
		bot:    telegrambot.NewClient(cfg.Telegram.BotAPIBase, cfg.Telegram.BotToken),
	}, nil
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
			if err := w.reconcileSucceededPayments(ctx, w.cfg.Worker.BatchSize); err != nil {
				w.logger.Error("payments reconciliation failed", "error", err)
			}
			if err := w.processPendingRefunds(ctx, w.cfg.Worker.BatchSize); err != nil {
				w.logger.Error("process pending refunds failed", "error", err)
			}
			if err := w.moveStalePaidOrdersToManualReview(ctx, w.cfg.Worker.BatchSize, w.cfg.Worker.PaidTimeout); err != nil {
				w.logger.Error("move stale paid orders to manual review failed", "error", err)
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
		SELECT id, source, payload, processed_at
		FROM inbox_events
		WHERE processed_at IS NULL
		ORDER BY first_seen_at
		LIMIT $1
	`, batchSize)
	if err != nil {
		return fmt.Errorf("query inbox batch: %w", err)
	}
	defer rows.Close()

	events := make([]inboxEvent, 0, batchSize)
	for rows.Next() {
		var ev inboxEvent
		if err := rows.Scan(&ev.ID, &ev.Source, &ev.Payload, &ev.Processed); err != nil {
			return fmt.Errorf("scan inbox event: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate inbox rows: %w", err)
	}

	for _, ev := range events {
		if err := w.processSingleInboxEvent(ctx, ev); err != nil {
			w.logger.Warn("inbox event processing failed", "inbox_event_id", ev.ID, "source", ev.Source, "error", err)
			if markErr := w.markInboxFailed(ctx, ev.ID, err.Error()); markErr != nil {
				w.logger.Error("mark inbox failed failed", "inbox_event_id", ev.ID, "error", markErr)
			}
		}
	}

	return nil
}

func (w *Worker) processSingleInboxEvent(ctx context.Context, ev inboxEvent) error {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin inbox tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var source string
	var payload []byte
	var processedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT source, payload, processed_at
		FROM inbox_events
		WHERE id = $1
		FOR UPDATE
	`, ev.ID).Scan(&source, &payload, &processedAt)
	if err != nil {
		return fmt.Errorf("load inbox event for processing: %w", err)
	}
	if processedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit already processed inbox tx: %w", err)
		}
		return nil
	}

	switch source {
	case "mock_payment":
		if err := w.handleMockPaymentInbox(ctx, tx, payload); err != nil {
			return err
		}
	case "telegram":
		if err := w.handleTelegramInbox(ctx, tx, payload); err != nil {
			return err
		}
	default:
		// Unknown source: mark as processed to avoid permanent retries.
		w.logger.Warn("unknown inbox source, skipping", "source", source, "inbox_event_id", ev.ID)
	}

	_, err = tx.Exec(ctx, `
		UPDATE inbox_events
		SET status = 'processed',
		    processed_at = now(),
		    attempts = attempts + 1,
		    last_error = NULL,
		    last_seen_at = now()
		WHERE id = $1
	`, ev.ID)
	if err != nil {
		return fmt.Errorf("mark inbox processed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit inbox tx: %w", err)
	}

	return nil
}

func (w *Worker) handleMockPaymentInbox(ctx context.Context, tx pgx.Tx, payloadBytes []byte) error {
	var event mockPaymentEvent
	if err := json.Unmarshal(payloadBytes, &event); err != nil {
		return fmt.Errorf("unmarshal payment inbox payload: %w", err)
	}
	if event.PaymentSessionID == "" {
		return fmt.Errorf("invalid payment inbox payload: missing payment_session_id")
	}

	var paymentID uuid.UUID
	var orderID uuid.UUID
	var paymentStatus string
	err := tx.QueryRow(ctx, `
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

	return nil
}

func (w *Worker) handleTelegramInbox(ctx context.Context, tx pgx.Tx, payloadBytes []byte) error {
	var update telegrambot.Update
	if err := json.Unmarshal(payloadBytes, &update); err != nil {
		return fmt.Errorf("unmarshal telegram update: %w", err)
	}

	if update.Message != nil {
		return w.handleTelegramMessage(ctx, tx, *update.Message)
	}
	if update.PreCheckoutQuery != nil {
		return w.handleTelegramPreCheckout(ctx, tx, *update.PreCheckoutQuery)
	}
	return nil
}

func (w *Worker) handleTelegramMessage(ctx context.Context, tx pgx.Tx, msg telegrambot.Message) error {
	if msg.Chat.ID == 0 {
		return nil
	}

	userID, err := w.ensureTelegramUser(ctx, tx, msg.From)
	if err != nil {
		return err
	}

	cmd := strings.TrimSpace(strings.ToLower(msg.Text))
	switch cmd {
	case "/start", "start":
		if err := w.bot.SendWebAppButton(ctx, msg.Chat.ID, "Открой меню и сделай заказ", "Открыть меню", w.cfg.Telegram.MiniAppURL); err != nil {
			w.logger.Warn("failed to send start message", "chat_id", msg.Chat.ID, "error", err)
		}
	case "/orders", "orders", "мои заказы":
		recent, err := w.buildRecentOrdersMessage(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err := w.bot.SendMessage(ctx, msg.Chat.ID, recent); err != nil {
			w.logger.Warn("failed to send orders message", "chat_id", msg.Chat.ID, "error", err)
		}
	default:
		if err := w.bot.SendMessage(ctx, msg.Chat.ID, "Команды: /start, /orders"); err != nil {
			w.logger.Warn("failed to send help message", "chat_id", msg.Chat.ID, "error", err)
		}
	}

	return nil
}

func (w *Worker) handleTelegramPreCheckout(ctx context.Context, tx pgx.Tx, q telegrambot.PreCheckoutQuery) error {
	orderID, err := parseOrderIDFromInvoicePayload(q.InvoicePayload)
	if err != nil {
		_ = w.bot.AnswerPreCheckoutQuery(ctx, q.ID, false, "Не удалось проверить заказ")
		return nil
	}

	allowed, err := w.validateOrderForPreCheckout(ctx, tx, orderID, q.TotalAmount, q.Currency)
	if err != nil {
		_ = w.bot.AnswerPreCheckoutQuery(ctx, q.ID, false, "Заказ не прошёл проверку")
		return err
	}

	if allowed {
		if err := w.bot.AnswerPreCheckoutQuery(ctx, q.ID, true, ""); err != nil {
			w.logger.Warn("answer pre checkout failed", "query_id", q.ID, "error", err)
		}
		if err := w.moveOrderToPaymentProcessing(ctx, tx, orderID); err != nil {
			return err
		}
	} else {
		if err := w.bot.AnswerPreCheckoutQuery(ctx, q.ID, false, "Некоторые блюда недоступны, обновите корзину"); err != nil {
			w.logger.Warn("answer pre checkout reject failed", "query_id", q.ID, "error", err)
		}
	}

	return nil
}

func (w *Worker) validateOrderForPreCheckout(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, totalAmount int, currency string) (bool, error) {
	var branchID uuid.UUID
	var status order.Status
	var orderTotal int
	var orderCurrency string
	err := tx.QueryRow(ctx, `
		SELECT branch_id, status::text, total, currency
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(&branchID, &status, &orderTotal, &orderCurrency)
	if err != nil {
		return false, fmt.Errorf("load order for pre_checkout validation: %w", err)
	}

	if status != order.StatusPendingPayment && status != order.StatusPaymentProcessing {
		return false, nil
	}
	if orderTotal != totalAmount || !strings.EqualFold(orderCurrency, currency) {
		return false, nil
	}

	var unavailableCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM order_items oi
		LEFT JOIN branch_menu_items bmi
			ON bmi.branch_id = $1 AND bmi.menu_item_id = oi.menu_item_id
		WHERE oi.order_id = $2
		  AND (bmi.id IS NULL OR bmi.status <> 'available')
	`, branchID, orderID).Scan(&unavailableCount)
	if err != nil {
		return false, fmt.Errorf("validate order items availability for pre_checkout: %w", err)
	}

	return unavailableCount == 0, nil
}

func (w *Worker) moveOrderToPaymentProcessing(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	var current order.Status
	err := tx.QueryRow(ctx, `
		SELECT status::text
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(&current)
	if err != nil {
		return fmt.Errorf("load order for payment_processing transition: %w", err)
	}
	if current == order.StatusPaymentProcessing {
		return nil
	}
	if err := order.ValidateTransition(current, order.StatusPaymentProcessing); err != nil {
		return nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $2::order_status,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
	`, orderID, order.StatusPaymentProcessing)
	if err != nil {
		return fmt.Errorf("update order to payment_processing: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (
			order_id, from_status, to_status, reason, actor_type, metadata, created_at
		)
		VALUES ($1, $2::order_status, $3::order_status, 'pre_checkout_approved', 'system', '{}'::jsonb, now())
	`, orderID, current, order.StatusPaymentProcessing)
	if err != nil {
		return fmt.Errorf("insert payment_processing history: %w", err)
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
			jsonb_build_object('order_id', $2::text),
			'{}'::jsonb,
			now()
		)
	`, orderID, orderID.String())
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
		SELECT id, event_type, aggregate_type, aggregate_id, payload
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
		id            uuid.UUID
		eventType     string
		aggregateType string
		aggregateID   uuid.UUID
		payload       []byte
	}
	events := make([]item, 0, batchSize)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.eventType, &it.aggregateType, &it.aggregateID, &it.payload); err != nil {
			return fmt.Errorf("scan outbox row: %w", err)
		}
		events = append(events, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate outbox rows: %w", err)
	}

	for _, ev := range events {
		if err := w.dispatchOutboxEvent(ctx, ev.eventType, ev.aggregateType, ev.aggregateID, ev.payload); err != nil {
			_, _ = w.db.Exec(ctx, `
				UPDATE outbox_events
				SET attempts = attempts + 1,
				    last_error = $2,
				    locked_at = NULL
				WHERE id = $1
			`, ev.id, err.Error())
			continue
		}

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

func (w *Worker) dispatchOutboxEvent(ctx context.Context, eventType, aggregateType string, aggregateID uuid.UUID, payload []byte) error {
	if aggregateType != "order" {
		return nil
	}
	if !w.bot.Enabled() {
		return nil
	}

	var orderID uuid.UUID
	if aggregateID != uuid.Nil {
		orderID = aggregateID
	} else {
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err == nil {
			if oid, ok := parsed["order_id"].(string); ok {
				if u, err := uuid.Parse(oid); err == nil {
					orderID = u
				}
			}
		}
	}
	if orderID == uuid.Nil {
		return nil
	}

	var chatID int64
	var orderNumber int64
	var status string
	err := w.db.QueryRow(ctx, `
		SELECT COALESCE(u.telegram_user_id, 0), o.order_number, o.status::text
		FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $1
	`, orderID).Scan(&chatID, &orderNumber, &status)
	if err != nil {
		return fmt.Errorf("load order for notification: %w", err)
	}
	if chatID == 0 {
		return nil
	}

	text := fmt.Sprintf("Заказ #%d: %s", orderNumber, status)
	switch eventType {
	case "OrderPaid":
		text = fmt.Sprintf("Заказ #%d оплачен. Передаём на кухню.", orderNumber)
	case "ManualReviewResolved":
		text = fmt.Sprintf("Заказ #%d обновлён оператором. Текущий статус: %s", orderNumber, status)
	case "OrderManualReviewRequired":
		text = fmt.Sprintf("Заказ #%d требует ручной проверки оператором.", orderNumber)
	case "RefundSucceeded":
		text = fmt.Sprintf("Возврат по заказу #%d завершён.", orderNumber)
	}

	if err := w.bot.SendMessage(ctx, chatID, text); err != nil {
		return fmt.Errorf("send telegram notification: %w", err)
	}

	return nil
}

func (w *Worker) reconcileSucceededPayments(ctx context.Context, batchSize int) error {
	rows, err := w.db.Query(ctx, `
		SELECT p.order_id
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE p.status = 'succeeded'
		  AND o.status IN ('pending_payment', 'payment_processing')
		ORDER BY p.updated_at
		LIMIT $1
	`, batchSize)
	if err != nil {
		return fmt.Errorf("query reconciliation candidates: %w", err)
	}
	defer rows.Close()

	orderIDs := make([]uuid.UUID, 0, batchSize)
	for rows.Next() {
		var orderID uuid.UUID
		if err := rows.Scan(&orderID); err != nil {
			return fmt.Errorf("scan reconciliation order id: %w", err)
		}
		orderIDs = append(orderIDs, orderID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate reconciliation rows: %w", err)
	}

	for _, orderID := range orderIDs {
		tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin reconciliation tx: %w", err)
		}
		if err := w.moveOrderToPaid(ctx, tx, orderID); err != nil {
			_ = tx.Rollback(ctx)
			w.logger.Warn("reconciliation move to paid failed", "order_id", orderID, "error", err)
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit reconciliation tx: %w", err)
		}
	}

	return nil
}

func (w *Worker) processPendingRefunds(ctx context.Context, batchSize int) error {
	rows, err := w.db.Query(ctx, `
		SELECT id
		FROM refunds
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT $1
	`, batchSize)
	if err != nil {
		return fmt.Errorf("query pending refunds: %w", err)
	}
	defer rows.Close()

	refundIDs := make([]uuid.UUID, 0, batchSize)
	for rows.Next() {
		var refundID uuid.UUID
		if err := rows.Scan(&refundID); err != nil {
			return fmt.Errorf("scan pending refund id: %w", err)
		}
		refundIDs = append(refundIDs, refundID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate pending refunds: %w", err)
	}

	for _, refundID := range refundIDs {
		if err := w.processSinglePendingRefund(ctx, refundID); err != nil {
			w.logger.Warn("pending refund processing failed", "refund_id", refundID, "error", err)
		}
	}

	return nil
}

func (w *Worker) processSinglePendingRefund(ctx context.Context, refundID uuid.UUID) error {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin refund tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		orderID     uuid.UUID
		paymentID   uuid.UUID
		status      string
		orderStatus order.Status
	)

	err = tx.QueryRow(ctx, `
		SELECT r.order_id, r.payment_id, r.status::text, o.status::text
		FROM refunds r
		JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1
		FOR UPDATE
	`, refundID).Scan(&orderID, &paymentID, &status, &orderStatus)
	if err != nil {
		return fmt.Errorf("load refund for processing: %w", err)
	}
	if status != "pending" {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit non-pending refund tx: %w", err)
		}
		return nil
	}

	providerRefundID := fmt.Sprintf("mockrefund_%s", uuid.NewString())
	_, err = tx.Exec(ctx, `
		UPDATE refunds
		SET status = 'succeeded',
		    provider_refund_id = $2,
		    updated_at = now(),
		    raw_payload = jsonb_set(raw_payload, '{processed_by}', '\"worker\"'::jsonb, true)
		WHERE id = $1
	`, refundID, providerRefundID)
	if err != nil {
		return fmt.Errorf("update refund to succeeded: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE payments
		SET status = CASE WHEN status = 'succeeded' THEN 'refunded' ELSE status END,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
	`, paymentID)
	if err != nil {
		return fmt.Errorf("update payment refunded status: %w", err)
	}

	if orderStatus != order.StatusRefunded {
		target := order.StatusRefunded
		if err := order.ValidateTransition(orderStatus, target); err != nil {
			target = order.StatusManualReview
		}

		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET status = $2::order_status,
			    updated_at = now(),
			    version = version + 1
			WHERE id = $1
		`, orderID, target)
		if err != nil {
			return fmt.Errorf("update order after refund: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (
				order_id, from_status, to_status, reason, actor_type, metadata, created_at
			)
			VALUES ($1, $2::order_status, $3::order_status, 'refund_processed_by_worker', 'system', '{}'::jsonb, now())
		`, orderID, orderStatus, target)
		if err != nil {
			return fmt.Errorf("insert refund status history: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			aggregate_type, aggregate_id, event_type, payload, headers, created_at
		)
		VALUES (
			'refund', $1, 'RefundSucceeded',
			jsonb_build_object('refund_id', $1::text, 'order_id', $2::text),
			'{}'::jsonb, now()
		)
	`, refundID, orderID)
	if err != nil {
		return fmt.Errorf("insert refund succeeded outbox event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE saga_instances
		SET status = CASE
			WHEN status IN ('compensating', 'running', 'manual_review') THEN 'compensated'::saga_status
			ELSE status
		END,
		    current_step = 'refund_succeeded',
		    updated_at = now(),
		    finished_at = now()
		WHERE saga_type = 'order_checkout'
		  AND entity_type = 'order'
		  AND entity_id = $1
	`, orderID)
	if err != nil {
		return fmt.Errorf("update saga after refund: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund tx: %w", err)
	}
	return nil
}

func (w *Worker) moveStalePaidOrdersToManualReview(ctx context.Context, batchSize int, paidTimeout time.Duration) error {
	if paidTimeout <= 0 {
		return nil
	}

	rows, err := w.db.Query(ctx, `
		SELECT id
		FROM orders
		WHERE status = 'paid'
		  AND updated_at <= now() - ($1 * interval '1 second')
		ORDER BY updated_at
		LIMIT $2
	`, int(paidTimeout.Seconds()), batchSize)
	if err != nil {
		return fmt.Errorf("query stale paid orders: %w", err)
	}
	defer rows.Close()

	orderIDs := make([]uuid.UUID, 0, batchSize)
	for rows.Next() {
		var orderID uuid.UUID
		if err := rows.Scan(&orderID); err != nil {
			return fmt.Errorf("scan stale paid order id: %w", err)
		}
		orderIDs = append(orderIDs, orderID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate stale paid orders: %w", err)
	}

	for _, orderID := range orderIDs {
		if err := w.markOrderManualReviewByTimeout(ctx, orderID, paidTimeout); err != nil {
			w.logger.Warn("mark order manual review by timeout failed", "order_id", orderID, "error", err)
		}
	}

	return nil
}

func (w *Worker) markOrderManualReviewByTimeout(ctx context.Context, orderID uuid.UUID, timeout time.Duration) error {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin stale-paid tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current order.Status
	err = tx.QueryRow(ctx, `SELECT status::text FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&current)
	if err != nil {
		return fmt.Errorf("load stale paid order: %w", err)
	}
	if current != order.StatusPaid {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit stale-paid noop tx: %w", err)
		}
		return nil
	}

	if err := order.ValidateTransition(current, order.StatusManualReview); err != nil {
		return nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = 'manual_review',
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
	`, orderID)
	if err != nil {
		return fmt.Errorf("update stale paid order to manual_review: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (
			order_id, from_status, to_status, reason, actor_type, metadata, created_at
		)
		VALUES (
			$1, 'paid', 'manual_review', 'paid_timeout_manual_review', 'system',
			jsonb_build_object('timeout', $2::text),
			now()
		)
	`, orderID, timeout.String())
	if err != nil {
		return fmt.Errorf("insert stale paid manual review history: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			aggregate_type, aggregate_id, event_type, payload, headers, created_at
		)
		VALUES (
			'order', $1, 'OrderManualReviewRequired',
			jsonb_build_object('order_id', $1::text, 'reason', 'paid_timeout'),
			'{}'::jsonb, now()
		)
	`, orderID)
	if err != nil {
		return fmt.Errorf("insert order manual review outbox event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO saga_instances (
			saga_type, entity_type, entity_id, status, current_step, last_error, started_at, created_at, updated_at
		)
		VALUES (
			'order_checkout', 'order', $1, 'manual_review', 'paid_timeout',
			'order remained paid without confirmation',
			now(), now(), now()
		)
		ON CONFLICT (saga_type, entity_type, entity_id)
		DO UPDATE SET
			status = 'manual_review',
			current_step = 'paid_timeout',
			last_error = 'order remained paid without confirmation',
			updated_at = now()
	`, orderID)
	if err != nil {
		return fmt.Errorf("upsert saga manual review on timeout: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stale-paid tx: %w", err)
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

func (w *Worker) ensureTelegramUser(ctx context.Context, tx pgx.Tx, from telegrambot.User) (uuid.UUID, error) {
	var userID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO users (telegram_user_id, username, is_active, created_at, updated_at)
		VALUES ($1, NULLIF($2, ''), true, now(), now())
		ON CONFLICT (telegram_user_id)
		DO UPDATE SET username = COALESCE(NULLIF(EXCLUDED.username, ''), users.username), updated_at = now()
		RETURNING id
	`, from.ID, from.Username).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert telegram user: %w", err)
	}
	return userID, nil
}

func (w *Worker) buildRecentOrdersMessage(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (string, error) {
	rows, err := tx.Query(ctx, `
		SELECT order_number, status::text, total, currency
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return "", fmt.Errorf("query recent orders: %w", err)
	}
	defer rows.Close()

	lines := make([]string, 0, 6)
	lines = append(lines, "Последние заказы:")
	count := 0
	for rows.Next() {
		var num int64
		var status string
		var total int
		var currency string
		if err := rows.Scan(&num, &status, &total, &currency); err != nil {
			return "", fmt.Errorf("scan recent order: %w", err)
		}
		count++
		lines = append(lines, fmt.Sprintf("#%d — %s (%d %s)", num, status, total, currency))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate recent orders: %w", err)
	}
	if count == 0 {
		return "У вас пока нет заказов.", nil
	}

	return strings.Join(lines, "\n"), nil
}

func parseOrderIDFromInvoicePayload(payload string) (uuid.UUID, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return uuid.Nil, fmt.Errorf("empty invoice payload")
	}
	if strings.HasPrefix(payload, "order:") {
		payload = strings.TrimPrefix(payload, "order:")
	}
	return uuid.Parse(payload)
}

func normalizePaymentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "success", "paid":
		return "succeeded"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return "failed"
	}
}
