package admin

import (
	"errors"
	"net/http"
	"strings"

	"TG-delivery/internal/modules/orders"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrdersHandler struct {
	service *orders.Service
}

func NewOrdersHandler(service *orders.Service) *OrdersHandler {
	return &OrdersHandler{service: service}
}

type resolveManualReviewRequest struct {
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	ActorID   string `json:"actor_id"`
	ActorType string `json:"actor_type"`
}

func (h *OrdersHandler) ListManualReview(w http.ResponseWriter, r *http.Request) {
	var branchID *uuid.UUID
	if rawBranchID := strings.TrimSpace(r.URL.Query().Get("branch_id")); rawBranchID != "" {
		parsed, err := uuid.Parse(rawBranchID)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid branch_id")
			return
		}
		branchID = &parsed
	}

	items, err := h.service.ListManualReview(r.Context(), branchID)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to load manual review orders")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func (h *OrdersHandler) ResolveManualReview(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid orderID")
		return
	}

	var req resolveManualReviewRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
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

	result, err := h.service.ResolveManualReview(r.Context(), orders.ResolveManualReviewInput{
		OrderID:   orderID,
		Action:    req.Action,
		Reason:    req.Reason,
		ActorID:   actorID,
		ActorType: strings.TrimSpace(req.ActorType),
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrOrderNotFound):
			handlers.WriteError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, orders.ErrInvalidAction):
			handlers.WriteError(w, http.StatusBadRequest, "invalid action")
		default:
			handlers.WriteError(w, http.StatusConflict, "cannot resolve manual review")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"result": result,
	})
}
