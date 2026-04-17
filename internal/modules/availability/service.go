package availability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"TG-delivery/internal/domain/menu"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("branch menu item not found")
	ErrInvalidStatus = errors.New("invalid branch menu item status")
)

type UpdateAvailabilityInput struct {
	BranchID       uuid.UUID
	MenuItemID     uuid.UUID
	Status         menu.BranchItemStatus
	Reason         string
	AvailableUntil *time.Time
	ActorType      string
	ActorID        *uuid.UUID
	RequestID      string
	IPAddress      string
	UserAgent      string
}

type UpdateAvailabilityResult struct {
	BranchMenuItemID uuid.UUID             `json:"branch_menu_item_id"`
	BranchID         uuid.UUID             `json:"branch_id"`
	MenuItemID       uuid.UUID             `json:"menu_item_id"`
	OldStatus        menu.BranchItemStatus `json:"old_status"`
	NewStatus        menu.BranchItemStatus `json:"new_status"`
	Reason           string                `json:"reason,omitempty"`
	AvailableUntil   *time.Time            `json:"available_until,omitempty"`
	Version          int                   `json:"version"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type StopListItem struct {
	BranchMenuItemID uuid.UUID             `json:"branch_menu_item_id"`
	MenuItemID       uuid.UUID             `json:"menu_item_id"`
	MenuItemName     string                `json:"menu_item_name"`
	Status           menu.BranchItemStatus `json:"status"`
	AvailableUntil   *time.Time            `json:"available_until,omitempty"`
	Reason           string                `json:"reason,omitempty"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type ListStopListInput struct {
	BranchID uuid.UUID
	Status   *menu.BranchItemStatus
}

type Repository interface {
	UpdateAvailability(ctx context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error)
	ListStopList(ctx context.Context, in ListStopListInput) ([]StopListItem, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpdateAvailability(ctx context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error) {
	if !menu.IsValidStatus(in.Status) {
		return UpdateAvailabilityResult{}, ErrInvalidStatus
	}

	if in.ActorType == "" {
		in.ActorType = "system"
	}
	in.ActorType = strings.TrimSpace(strings.ToLower(in.ActorType))
	if in.ActorType == "" {
		in.ActorType = "system"
	}

	if in.Status == menu.StatusAvailable {
		in.AvailableUntil = nil
	}

	result, err := s.repo.UpdateAvailability(ctx, in)
	if err != nil {
		return UpdateAvailabilityResult{}, err
	}

	return result, nil
}

func (s *Service) ListStopList(ctx context.Context, in ListStopListInput) ([]StopListItem, error) {
	if in.Status != nil && !menu.IsValidStatus(*in.Status) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, *in.Status)
	}

	return s.repo.ListStopList(ctx, in)
}
