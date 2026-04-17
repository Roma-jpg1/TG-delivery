package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"TG-delivery/internal/domain/menu"
	"TG-delivery/internal/modules/availability"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AvailabilityService interface {
	UpdateAvailability(ctx context.Context, in availability.UpdateAvailabilityInput) (availability.UpdateAvailabilityResult, error)
	ListStopList(ctx context.Context, in availability.ListStopListInput) ([]availability.StopListItem, error)
}

type AvailabilityHandler struct {
	service AvailabilityService
}

func NewAvailabilityHandler(service AvailabilityService) *AvailabilityHandler {
	return &AvailabilityHandler{service: service}
}

type updateAvailabilityRequest struct {
	Status         menu.BranchItemStatus `json:"status"`
	Reason         string                `json:"reason"`
	AvailableUntil *time.Time            `json:"available_until"`
	ActorType      string                `json:"actor_type"`
	ActorID        string                `json:"actor_id"`
}

func (h *AvailabilityHandler) UpdateAvailability(w http.ResponseWriter, r *http.Request) {
	branchID, err := uuid.Parse(chi.URLParam(r, "branchID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branchID")
		return
	}

	menuItemID, err := uuid.Parse(chi.URLParam(r, "menuItemID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid menuItemID")
		return
	}

	var req updateAvailabilityRequest
	if err := decodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var actorID *uuid.UUID
	if strings.TrimSpace(req.ActorID) != "" {
		parsed, err := uuid.Parse(req.ActorID)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid actor_id")
			return
		}
		actorID = &parsed
	}

	ipAddress := extractIP(r.RemoteAddr)
	result, err := h.service.UpdateAvailability(r.Context(), availability.UpdateAvailabilityInput{
		BranchID:       branchID,
		MenuItemID:     menuItemID,
		Status:         req.Status,
		Reason:         strings.TrimSpace(req.Reason),
		AvailableUntil: req.AvailableUntil,
		ActorType:      strings.TrimSpace(req.ActorType),
		ActorID:        actorID,
		RequestID:      middleware.RequestIDFromContext(r.Context()),
		IPAddress:      ipAddress,
		UserAgent:      r.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, availability.ErrNotFound):
			handlers.WriteError(w, http.StatusNotFound, "branch menu item not found")
		case errors.Is(err, availability.ErrInvalidStatus):
			handlers.WriteError(w, http.StatusBadRequest, "invalid status")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to update availability")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"result": result,
	})
}

func (h *AvailabilityHandler) ListStopList(w http.ResponseWriter, r *http.Request) {
	branchID, err := uuid.Parse(chi.URLParam(r, "branchID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branchID")
		return
	}

	var status *menu.BranchItemStatus
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		parsed := menu.BranchItemStatus(rawStatus)
		status = &parsed
	}

	items, err := h.service.ListStopList(r.Context(), availability.ListStopListInput{
		BranchID: branchID,
		Status:   status,
	})
	if err != nil {
		if errors.Is(err, availability.ErrInvalidStatus) {
			handlers.WriteError(w, http.StatusBadRequest, "invalid status filter")
			return
		}
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list stop-list")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func decodeJSON(r *http.Request, dest any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return errors.New("invalid JSON payload")
	}
	return nil
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
