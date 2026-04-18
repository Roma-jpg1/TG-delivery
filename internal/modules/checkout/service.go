package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"TG-delivery/internal/modules/delivery"

	"TG-delivery/internal/domain/order"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCartNotFound           = errors.New("active cart not found")
	ErrCartEmpty              = errors.New("cart is empty")
	ErrCartRevalidationFailed = errors.New("cart revalidation failed")
	ErrAddressNotFound        = errors.New("address not found")
	ErrOutOfDeliveryZone      = errors.New("address is outside delivery zone")
	ErrBelowMinOrder          = errors.New("cart total is below minimum order amount")
)

type CreateDraftInput struct {
	UserID          uuid.UUID
	BranchID        uuid.UUID
	AddressID       *uuid.UUID
	CustomerComment string
	RequestID       string
}

type Draft struct {
	OrderID     uuid.UUID `json:"order_id"`
	Status      string    `json:"status"`
	Currency    string    `json:"currency"`
	Subtotal    int       `json:"subtotal"`
	DeliveryFee int       `json:"delivery_fee"`
	Total       int       `json:"total"`
	CreatedAt   time.Time `json:"created_at"`
}

type cartItemRow struct {
	CartItemID      uuid.UUID
	MenuItemID      uuid.UUID
	MenuItemName    string
	MenuItemDesc    string
	Quantity        int
	CurrentPrice    int
	CartUnitPrice   int
	OptionsSnapshot []byte
	BranchStatus    string
}

type Service struct {
	db       *pgxpool.Pool
	delivery *delivery.Service
}

func NewService(db *pgxpool.Pool, deliveryService *delivery.Service) *Service {
	return &Service{db: db, delivery: deliveryService}
}

func (s *Service) CreateOrderDraft(ctx context.Context, in CreateDraftInput) (Draft, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Draft{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cartID uuid.UUID
	var currency string
	err = tx.QueryRow(ctx, `
		SELECT id, currency
		FROM carts
		WHERE user_id = $1 AND branch_id = $2 AND status = 'active'
		ORDER BY updated_at DESC
		LIMIT 1
		FOR UPDATE
	`, in.UserID, in.BranchID).Scan(&cartID, &currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Draft{}, ErrCartNotFound
		}
		return Draft{}, fmt.Errorf("load active cart: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT
			ci.id,
			ci.menu_item_id,
			mi.name,
			COALESCE(mi.description, ''),
			ci.quantity,
			bmi.price,
			ci.unit_price,
			COALESCE(ci.options, '[]'::jsonb),
			bmi.status::text
		FROM cart_items ci
		JOIN menu_items mi ON mi.id = ci.menu_item_id
		JOIN branch_menu_items bmi ON bmi.id = ci.branch_menu_item_id
		WHERE ci.cart_id = $1
		ORDER BY ci.created_at
	`, cartID)
	if err != nil {
		return Draft{}, fmt.Errorf("load cart items: %w", err)
	}
	defer rows.Close()

	items := make([]cartItemRow, 0, 32)
	for rows.Next() {
		var row cartItemRow
		if err := rows.Scan(
			&row.CartItemID,
			&row.MenuItemID,
			&row.MenuItemName,
			&row.MenuItemDesc,
			&row.Quantity,
			&row.CurrentPrice,
			&row.CartUnitPrice,
			&row.OptionsSnapshot,
			&row.BranchStatus,
		); err != nil {
			return Draft{}, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return Draft{}, fmt.Errorf("iterate cart items: %w", err)
	}

	if len(items) == 0 {
		return Draft{}, ErrCartEmpty
	}

	subtotal := 0
	for _, item := range items {
		if item.BranchStatus != "available" {
			return Draft{}, fmt.Errorf("%w: menu_item_id=%s status=%s", ErrCartRevalidationFailed, item.MenuItemID, item.BranchStatus)
		}
		if item.CurrentPrice != item.CartUnitPrice {
			return Draft{}, fmt.Errorf("%w: menu_item_id=%s stale price", ErrCartRevalidationFailed, item.MenuItemID)
		}
		subtotal += item.CurrentPrice * item.Quantity
	}

	deliveryFee := 0
	var deliveryQuote *delivery.Quote
	if in.AddressID != nil {
		if s.delivery == nil {
			return Draft{}, fmt.Errorf("delivery service is not configured")
		}
		quote, err := s.delivery.Quote(ctx, delivery.QuoteInput{
			UserID:       in.UserID,
			BranchID:     in.BranchID,
			AddressID:    *in.AddressID,
			CartSubtotal: subtotal,
		})
		if err != nil {
			switch {
			case errors.Is(err, delivery.ErrAddressNotFound):
				return Draft{}, ErrAddressNotFound
			case errors.Is(err, delivery.ErrOutOfZone):
				return Draft{}, ErrOutOfDeliveryZone
			case errors.Is(err, delivery.ErrBelowMinOrder):
				return Draft{}, ErrBelowMinOrder
			default:
				return Draft{}, fmt.Errorf("calculate delivery quote: %w", err)
			}
		}
		deliveryQuote = &quote
		deliveryFee = quote.DeliveryFee
	}
	total := subtotal + deliveryFee

	var addressSnapshot []byte
	if in.AddressID != nil {
		err = tx.QueryRow(ctx, `
			SELECT jsonb_build_object(
				'id', a.id,
				'label', a.label,
				'city', a.city,
				'street', a.street,
				'house', a.house,
				'apartment', a.apartment,
				'entrance', a.entrance,
				'floor', a.floor,
				'comment', a.comment,
				'latitude', a.latitude,
				'longitude', a.longitude
			)
			FROM addresses a
			WHERE a.id = $1 AND a.user_id = $2
		`, *in.AddressID, in.UserID).Scan(&addressSnapshot)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Draft{}, fmt.Errorf("%w: address not found", ErrCartRevalidationFailed)
			}
			return Draft{}, fmt.Errorf("load address snapshot: %w", err)
		}
	} else {
		addressSnapshot = []byte(`{}`)
	}

	pricingSnapshotMap := map[string]any{
		"subtotal":     subtotal,
		"delivery_fee": deliveryFee,
		"total":        total,
	}
	if deliveryQuote != nil {
		pricingSnapshotMap["delivery_quote"] = deliveryQuote
	}

	pricingSnapshot, err := json.Marshal(pricingSnapshotMap)
	if err != nil {
		return Draft{}, fmt.Errorf("marshal pricing snapshot: %w", err)
	}

	var orderID uuid.UUID
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			user_id,
			branch_id,
			cart_id,
			status,
			currency,
			subtotal,
			discount_total,
			delivery_fee,
			total,
			customer_comment,
			delivery_address_snapshot,
			pricing_snapshot,
			request_id,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4::order_status, $5, $6, 0, $7, $8, $9,
			$10::jsonb, $11::jsonb, NULLIF($12,''), now(), now()
		)
		RETURNING id, created_at
	`, in.UserID, in.BranchID, cartID, order.StatusDraft, currency, subtotal, deliveryFee, total, in.CustomerComment, addressSnapshot, pricingSnapshot, in.RequestID).Scan(&orderID, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && in.RequestID != "" {
			err = tx.QueryRow(ctx, `
				SELECT id, created_at, status::text, currency, subtotal, delivery_fee, total
				FROM orders
				WHERE branch_id = $1 AND request_id = $2
				LIMIT 1
			`, in.BranchID, in.RequestID).Scan(&orderID, &createdAt, new(string), new(string), new(int), new(int), new(int))
			if err != nil {
				return Draft{}, fmt.Errorf("load idempotent order: %w", err)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return Draft{}, fmt.Errorf("commit idempotent transaction: %w", commitErr)
			}
			return s.loadDraft(ctx, orderID)
		}
		return Draft{}, fmt.Errorf("create order draft: %w", err)
	}

	for _, item := range items {
		lineTotal := item.CurrentPrice * item.Quantity
		_, err := tx.Exec(ctx, `
			INSERT INTO order_items (
				order_id,
				menu_item_id,
				name_snapshot,
				description_snapshot,
				price_snapshot,
				quantity,
				options_snapshot,
				line_total,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, now())
		`, orderID, item.MenuItemID, item.MenuItemName, item.MenuItemDesc, item.CurrentPrice, item.Quantity, item.OptionsSnapshot, lineTotal)
		if err != nil {
			return Draft{}, fmt.Errorf("insert order item snapshot: %w", err)
		}
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
		VALUES ($1, NULL, $2::order_status, 'draft_created', 'system', '{}'::jsonb, now())
	`, orderID, order.StatusDraft)
	if err != nil {
		return Draft{}, fmt.Errorf("insert order history: %w", err)
	}

	outboxPayload, err := json.Marshal(map[string]any{
		"order_id": orderID,
		"status":   order.StatusDraft,
		"total":    total,
		"currency": currency,
	})
	if err != nil {
		return Draft{}, fmt.Errorf("marshal outbox payload: %w", err)
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
		VALUES ('order', $1, 'OrderDraftCreated', $2::jsonb, jsonb_build_object('request_id', $3), now())
	`, orderID, outboxPayload, in.RequestID)
	if err != nil {
		return Draft{}, fmt.Errorf("insert order draft outbox event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE carts
		SET status = 'submitted',
		    updated_at = now()
		WHERE id = $1
	`, cartID)
	if err != nil {
		return Draft{}, fmt.Errorf("mark cart submitted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Draft{}, fmt.Errorf("commit transaction: %w", err)
	}

	return Draft{
		OrderID:     orderID,
		Status:      string(order.StatusDraft),
		Currency:    currency,
		Subtotal:    subtotal,
		DeliveryFee: deliveryFee,
		Total:       total,
		CreatedAt:   createdAt,
	}, nil
}

func (s *Service) loadDraft(ctx context.Context, orderID uuid.UUID) (Draft, error) {
	var d Draft
	err := s.db.QueryRow(ctx, `
		SELECT id, status::text, currency, subtotal, delivery_fee, total, created_at
		FROM orders
		WHERE id = $1
	`, orderID).Scan(&d.OrderID, &d.Status, &d.Currency, &d.Subtotal, &d.DeliveryFee, &d.Total, &d.CreatedAt)
	if err != nil {
		return Draft{}, fmt.Errorf("load order draft: %w", err)
	}
	return d, nil
}
