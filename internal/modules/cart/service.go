package cart

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrItemUnavailable = errors.New("menu item is not available")
	ErrCartNotFound    = errors.New("active cart not found")
)

type Item struct {
	ID           uuid.UUID `json:"id"`
	MenuItemID   uuid.UUID `json:"menu_item_id"`
	MenuItemName string    `json:"menu_item_name"`
	Quantity     int       `json:"quantity"`
	UnitPrice    int       `json:"unit_price"`
	LineTotal    int       `json:"line_total"`
	Comment      string    `json:"comment,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Snapshot struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	BranchID  uuid.UUID `json:"branch_id"`
	Currency  string    `json:"currency"`
	Subtotal  int       `json:"subtotal"`
	Total     int       `json:"total"`
	UpdatedAt time.Time `json:"updated_at"`
	Items     []Item    `json:"items"`
}

type UpsertItemInput struct {
	UserID     uuid.UUID
	BranchID   uuid.UUID
	MenuItemID uuid.UUID
	Quantity   int
	Comment    string
}

type RemoveItemInput struct {
	UserID     uuid.UUID
	BranchID   uuid.UUID
	CartItemID uuid.UUID
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) GetActiveCart(ctx context.Context, userID, branchID uuid.UUID) (Snapshot, error) {
	cartID, err := s.findActiveCartID(ctx, s.db, userID, branchID)
	if err != nil {
		return Snapshot{}, err
	}
	if cartID == uuid.Nil {
		return Snapshot{}, ErrCartNotFound
	}
	return s.loadCartSnapshot(ctx, s.db, cartID)
}

func (s *Service) UpsertItem(ctx context.Context, in UpsertItemInput) (Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var branchMenuItemID uuid.UUID
	var price int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT id, price, status::text
		FROM branch_menu_items
		WHERE branch_id = $1 AND menu_item_id = $2
	`, in.BranchID, in.MenuItemID).Scan(&branchMenuItemID, &price, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Snapshot{}, ErrItemUnavailable
		}
		return Snapshot{}, fmt.Errorf("load branch menu item: %w", err)
	}
	if status != "available" {
		return Snapshot{}, ErrItemUnavailable
	}

	cartID, err := s.findOrCreateActiveCart(ctx, tx, in.UserID, in.BranchID)
	if err != nil {
		return Snapshot{}, err
	}

	var existingItemID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM cart_items
		WHERE cart_id = $1 AND menu_item_id = $2
		LIMIT 1
	`, cartID, in.MenuItemID).Scan(&existingItemID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, fmt.Errorf("find existing cart item: %w", err)
	}

	if in.Quantity <= 0 {
		if existingItemID != uuid.Nil {
			if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE id = $1`, existingItemID); err != nil {
				return Snapshot{}, fmt.Errorf("delete cart item: %w", err)
			}
		}
	} else {
		if existingItemID != uuid.Nil {
			_, err = tx.Exec(ctx, `
				UPDATE cart_items
				SET quantity = $2,
				    unit_price = $3,
				    comment = $4,
				    branch_menu_item_id = $5,
				    updated_at = now()
				WHERE id = $1
			`, existingItemID, in.Quantity, price, in.Comment, branchMenuItemID)
		} else {
			_, err = tx.Exec(ctx, `
				INSERT INTO cart_items (
					cart_id,
					menu_item_id,
					branch_menu_item_id,
					quantity,
					unit_price,
					comment,
					created_at,
					updated_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			`, cartID, in.MenuItemID, branchMenuItemID, in.Quantity, price, in.Comment)
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("upsert cart item: %w", err)
		}
	}

	if err := s.recalculateCartTotals(ctx, tx, cartID); err != nil {
		return Snapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, fmt.Errorf("commit transaction: %w", err)
	}

	return s.loadCartSnapshot(ctx, s.db, cartID)
}

func (s *Service) RemoveItem(ctx context.Context, in RemoveItemInput) (Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cartID, err := s.findActiveCartID(ctx, tx, in.UserID, in.BranchID)
	if err != nil {
		return Snapshot{}, err
	}
	if cartID == uuid.Nil {
		return Snapshot{}, ErrCartNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE id = $1 AND cart_id = $2`, in.CartItemID, cartID); err != nil {
		return Snapshot{}, fmt.Errorf("delete cart item: %w", err)
	}

	if err := s.recalculateCartTotals(ctx, tx, cartID); err != nil {
		return Snapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, fmt.Errorf("commit transaction: %w", err)
	}

	return s.loadCartSnapshot(ctx, s.db, cartID)
}

func (s *Service) findOrCreateActiveCart(ctx context.Context, tx pgx.Tx, userID, branchID uuid.UUID) (uuid.UUID, error) {
	cartID, err := s.findActiveCartID(ctx, tx, userID, branchID)
	if err != nil {
		return uuid.Nil, err
	}
	if cartID != uuid.Nil {
		return cartID, nil
	}

	// For API-first onboarding, create a placeholder user row when client uses
	// generated UUIDs before explicit profile registration.
	if _, err := tx.Exec(ctx, `
		INSERT INTO users(id, is_active, created_at, updated_at)
		VALUES ($1, true, now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID); err != nil {
		return uuid.Nil, fmt.Errorf("ensure user exists: %w", err)
	}

	var currency string
	err = tx.QueryRow(ctx, `
		SELECT r.currency
		FROM branches b
		JOIN restaurants r ON r.id = b.restaurant_id
		WHERE b.id = $1
	`, branchID).Scan(&currency)
	if err != nil {
		return uuid.Nil, fmt.Errorf("load branch currency: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO carts (
			user_id,
			branch_id,
			status,
			currency,
			subtotal,
			discount_total,
			delivery_fee,
			total,
			created_at,
			updated_at
		)
		VALUES ($1, $2, 'active', $3, 0, 0, 0, 0, now(), now())
		RETURNING id
	`, userID, branchID, currency).Scan(&cartID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create active cart: %w", err)
	}

	return cartID, nil
}

func (s *Service) findActiveCartID(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID, branchID uuid.UUID) (uuid.UUID, error) {
	var cartID uuid.UUID
	err := querier.QueryRow(ctx, `
		SELECT id
		FROM carts
		WHERE user_id = $1
		  AND branch_id = $2
		  AND status = 'active'
		ORDER BY updated_at DESC
		LIMIT 1
	`, userID, branchID).Scan(&cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("find active cart: %w", err)
	}
	return cartID, nil
}

func (s *Service) recalculateCartTotals(ctx context.Context, tx pgx.Tx, cartID uuid.UUID) error {
	var subtotal int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(unit_price * quantity), 0)
		FROM cart_items
		WHERE cart_id = $1
	`, cartID).Scan(&subtotal); err != nil {
		return fmt.Errorf("calculate cart subtotal: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE carts
		SET subtotal = $2,
		    total = $2 - discount_total + delivery_fee,
		    updated_at = now()
		WHERE id = $1
	`, cartID, subtotal); err != nil {
		return fmt.Errorf("update cart totals: %w", err)
	}

	return nil
}

func (s *Service) loadCartSnapshot(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, cartID uuid.UUID) (Snapshot, error) {
	var cart Snapshot
	if err := querier.QueryRow(ctx, `
		SELECT id, user_id, branch_id, currency, subtotal, total, updated_at
		FROM carts
		WHERE id = $1
	`, cartID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.BranchID,
		&cart.Currency,
		&cart.Subtotal,
		&cart.Total,
		&cart.UpdatedAt,
	); err != nil {
		return Snapshot{}, fmt.Errorf("load cart: %w", err)
	}

	rows, err := querier.Query(ctx, `
		SELECT
			ci.id,
			ci.menu_item_id,
			mi.name,
			ci.quantity,
			ci.unit_price,
			(ci.quantity * ci.unit_price) AS line_total,
			COALESCE(ci.comment, ''),
			ci.updated_at
		FROM cart_items ci
		JOIN menu_items mi ON mi.id = ci.menu_item_id
		WHERE ci.cart_id = $1
		ORDER BY ci.updated_at DESC
	`, cartID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query cart items: %w", err)
	}
	defer rows.Close()

	items := make([]Item, 0, 32)
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.ID,
			&item.MenuItemID,
			&item.MenuItemName,
			&item.Quantity,
			&item.UnitPrice,
			&item.LineTotal,
			&item.Comment,
			&item.UpdatedAt,
		); err != nil {
			return Snapshot{}, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate cart items: %w", err)
	}
	cart.Items = items

	return cart, nil
}
