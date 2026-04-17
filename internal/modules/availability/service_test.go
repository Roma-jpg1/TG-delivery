package availability

import (
	"context"
	"errors"
	"testing"
	"time"

	"TG-delivery/internal/domain/menu"

	"github.com/google/uuid"
)

type repoStub struct {
	updateFn func(ctx context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error)
	listFn   func(ctx context.Context, in ListStopListInput) ([]StopListItem, error)
}

func (r repoStub) UpdateAvailability(ctx context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error) {
	if r.updateFn == nil {
		return UpdateAvailabilityResult{}, nil
	}
	return r.updateFn(ctx, in)
}

func (r repoStub) ListStopList(ctx context.Context, in ListStopListInput) ([]StopListItem, error) {
	if r.listFn == nil {
		return nil, nil
	}
	return r.listFn(ctx, in)
}

func TestServiceUpdateAvailability_ValidatesStatus(t *testing.T) {
	s := NewService(repoStub{})

	_, err := s.UpdateAvailability(context.Background(), UpdateAvailabilityInput{
		BranchID:   uuid.New(),
		MenuItemID: uuid.New(),
		Status:     "broken",
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestServiceUpdateAvailability_DefaultsActorTypeAndClearsUntilForAvailable(t *testing.T) {
	called := false
	now := time.Now().UTC().Add(2 * time.Hour)
	s := NewService(repoStub{
		updateFn: func(_ context.Context, in UpdateAvailabilityInput) (UpdateAvailabilityResult, error) {
			called = true
			if in.ActorType != "system" {
				t.Fatalf("expected actor_type system, got %q", in.ActorType)
			}
			if in.AvailableUntil != nil {
				t.Fatalf("expected available_until nil for available status")
			}
			return UpdateAvailabilityResult{}, nil
		},
	})

	_, err := s.UpdateAvailability(context.Background(), UpdateAvailabilityInput{
		BranchID:       uuid.New(),
		MenuItemID:     uuid.New(),
		Status:         menu.StatusAvailable,
		AvailableUntil: &now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected repository call")
	}
}

func TestServiceListStopList_ValidatesFilter(t *testing.T) {
	s := NewService(repoStub{})
	invalid := menu.BranchItemStatus("bad")

	_, err := s.ListStopList(context.Background(), ListStopListInput{
		BranchID: uuid.New(),
		Status:   &invalid,
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}
