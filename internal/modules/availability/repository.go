package availability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"TG-delivery/internal/domain/menu"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) UpdateAvailability(ctx context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var branchMenuItemID uuid.UUID
	var oldStatus menu.BranchItemStatus
	err = tx.QueryRow(ctx, `
		SELECT id, status::text
		FROM branch_menu_items
		WHERE branch_id = $1 AND menu_item_id = $2
		FOR UPDATE
	`, in.BranchID, in.MenuItemID).Scan(&branchMenuItemID, &oldStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UpdateAvailabilityResult{}, ErrNotFound
		}
		return UpdateAvailabilityResult{}, fmt.Errorf("load branch menu item: %w", err)
	}

	var version int
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE branch_menu_items
		SET status = $2::branch_menu_item_status,
		    available_until = $3,
		    reason = $4,
		    updated_by = $5,
		    version = version + 1,
		    updated_at = now()
		WHERE id = $1
		RETURNING version, updated_at
	`, branchMenuItemID, in.Status, in.AvailableUntil, in.Reason, in.ActorID).Scan(&version, &updatedAt)
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("update branch menu item status: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO menu_item_availability_log (
			branch_id,
			menu_item_id,
			old_status,
			new_status,
			reason,
			actor_type,
			actor_id,
			changed_at
		)
		VALUES ($1, $2, $3::branch_menu_item_status, $4::branch_menu_item_status, $5, $6, $7, now())
	`, in.BranchID, in.MenuItemID, oldStatus, in.Status, in.Reason, in.ActorType, in.ActorID)
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("insert availability log: %w", err)
	}

	oldValues, err := json.Marshal(map[string]any{
		"status": oldStatus,
	})
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("marshal old audit values: %w", err)
	}
	newValues, err := json.Marshal(map[string]any{
		"status":          in.Status,
		"reason":          in.Reason,
		"available_until": in.AvailableUntil,
		"version":         version,
	})
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("marshal new audit values: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (
			actor_type,
			actor_id,
			action,
			entity_type,
			entity_id,
			branch_id,
			request_id,
			ip_address,
			user_agent,
			old_values,
			new_values,
			metadata,
			created_at
		)
		VALUES (
			$1, $2, 'availability.updated', 'branch_menu_item', $3, $4, $5, $6, $7,
			$8::jsonb, $9::jsonb,
			jsonb_build_object('menu_item_id', $10::text),
			now()
		)
	`, in.ActorType, in.ActorID, branchMenuItemID, in.BranchID, in.RequestID, in.IPAddress, in.UserAgent, oldValues, newValues, in.MenuItemID)
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("insert audit log: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"branch_menu_item_id": branchMenuItemID,
		"branch_id":           in.BranchID,
		"menu_item_id":        in.MenuItemID,
		"old_status":          oldStatus,
		"new_status":          in.Status,
		"reason":              in.Reason,
		"available_until":     in.AvailableUntil,
		"version":             version,
		"updated_at":          updatedAt,
	})
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("marshal outbox payload: %w", err)
	}

	headers, err := json.Marshal(map[string]any{
		"request_id": in.RequestID,
	})
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("marshal outbox headers: %w", err)
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
		VALUES ('branch_menu_item', $1, 'menu_item.availability_changed', $2::jsonb, $3::jsonb, now())
	`, branchMenuItemID, payload, headers)
	if err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return UpdateAvailabilityResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return UpdateAvailabilityResult{
		BranchMenuItemID: branchMenuItemID,
		BranchID:         in.BranchID,
		MenuItemID:       in.MenuItemID,
		OldStatus:        oldStatus,
		NewStatus:        in.Status,
		Reason:           in.Reason,
		AvailableUntil:   in.AvailableUntil,
		Version:          version,
		UpdatedAt:        updatedAt,
	}, nil
}

func (r *PostgresRepository) ListStopList(ctx context.Context, in ListStopListInput) ([]StopListItem, error) {
	statusFilter := ""
	if in.Status != nil {
		statusFilter = string(*in.Status)
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			bmi.id,
			bmi.menu_item_id,
			mi.name,
			bmi.status::text,
			bmi.available_until,
			COALESCE(bmi.reason, ''),
			bmi.updated_at
		FROM branch_menu_items bmi
		JOIN menu_items mi ON mi.id = bmi.menu_item_id
		WHERE bmi.branch_id = $1
		  AND bmi.status <> 'available'
		  AND ($2 = '' OR bmi.status = $2::branch_menu_item_status)
		ORDER BY bmi.updated_at DESC
	`, in.BranchID, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("list stop-list items: %w", err)
	}
	defer rows.Close()

	items := make([]StopListItem, 0)
	for rows.Next() {
		var item StopListItem
		if err := rows.Scan(
			&item.BranchMenuItemID,
			&item.MenuItemID,
			&item.MenuItemName,
			&item.Status,
			&item.AvailableUntil,
			&item.Reason,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stop-list item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stop-list rows: %w", err)
	}

	return items, nil
}
