package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"TG-delivery/internal/modules/refunds"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RefundsHandler struct {
	service *refunds.Service
}

func NewRefundsHandler(service *refunds.Service) *RefundsHandler {
	return &RefundsHandler{service: service}
}

type requestRefundRequest struct {
	Reason    string `json:"reason"`
	ActorID   string `json:"actor_id"`
	ActorType string `json:"actor_type"`
}

func (h *RefundsHandler) List(w http.ResponseWriter, r *http.Request) {
	var branchID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("branch_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid branch_id")
			return
		}
		branchID = &parsed
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	items, err := h.service.List(r.Context(), refunds.ListFilter{
		BranchID: branchID,
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:    limit,
	})
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list refunds")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (h *RefundsHandler) Request(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid orderID")
		return
	}

	var req requestRefundRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var actorID *uuid.UUID
	if raw := strings.TrimSpace(req.ActorID); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid actor_id")
			return
		}
		actorID = &parsed
	}

	item, err := h.service.Request(r.Context(), refunds.RequestInput{
		OrderID:   orderID,
		Reason:    req.Reason,
		ActorType: strings.TrimSpace(req.ActorType),
		ActorID:   actorID,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		switch {
		case errors.Is(err, refunds.ErrOrderNotFound):
			handlers.WriteError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, refunds.ErrNoPaymentForRefund):
			handlers.WriteError(w, http.StatusConflict, "no successful payment for refund")
		case errors.Is(err, refunds.ErrOrderStatusNotRefundable):
			handlers.WriteError(w, http.StatusConflict, "order status is not refundable")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to request refund")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"refund": item})
}
