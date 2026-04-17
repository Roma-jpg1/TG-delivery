package menu

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	MenuItemID       uuid.UUID  `json:"menu_item_id"`
	CategoryID       *uuid.UUID `json:"category_id,omitempty"`
	CategoryName     string     `json:"category_name,omitempty"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	PhotoURL         string     `json:"photo_url,omitempty"`
	Price            int        `json:"price"`
	Status           string     `json:"status"`
	AvailableUntil   *time.Time `json:"available_until,omitempty"`
	AvailabilityNote string     `json:"availability_note,omitempty"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) ListBranchMenu(ctx context.Context, branchID uuid.UUID, includeUnavailable bool) ([]Item, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			mi.id,
			c.id,
			COALESCE(c.name, ''),
			mi.name,
			COALESCE(mi.description, ''),
			COALESCE(mi.photo_url, ''),
			bmi.price,
			bmi.status::text,
			bmi.available_until,
			COALESCE(bmi.reason, '')
		FROM branch_menu_items bmi
		JOIN menu_items mi ON mi.id = bmi.menu_item_id
		LEFT JOIN categories c ON c.id = mi.category_id
		WHERE bmi.branch_id = $1
		  AND mi.is_deleted = false
		  AND ($2 = true OR bmi.status = 'available')
		ORDER BY c.sort_order NULLS LAST, mi.name
	`, branchID, includeUnavailable)
	if err != nil {
		return nil, fmt.Errorf("query branch menu: %w", err)
	}
	defer rows.Close()

	items := make([]Item, 0, 128)
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.MenuItemID,
			&item.CategoryID,
			&item.CategoryName,
			&item.Name,
			&item.Description,
			&item.PhotoURL,
			&item.Price,
			&item.Status,
			&item.AvailableUntil,
			&item.AvailabilityNote,
		); err != nil {
			return nil, fmt.Errorf("scan branch menu item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate branch menu rows: %w", err)
	}

	return items, nil
}
