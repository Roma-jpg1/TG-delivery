package public

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/payments"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/google/uuid"
)

type PaymentsHandler struct {
	service *payments.Service
}

func NewPaymentsHandler(service *payments.Service) *PaymentsHandler {
	return &PaymentsHandler{service: service}
}

type createSessionRequest struct {
	OrderID        uuid.UUID `json:"order_id"`
	Provider       string    `json:"provider"`
	IdempotencyKey string    `json:"idempotency_key"`
}

func (h *PaymentsHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := h.service.CreateSession(r.Context(), payments.CreateSessionInput{
		OrderID:        req.OrderID,
		Provider:       req.Provider,
		IdempotencyKey: req.IdempotencyKey,
		RequestID:      middleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		switch {
		case errors.Is(err, payments.ErrOrderNotFound):
			handlers.WriteError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, payments.ErrOrderInvalidStatus):
			handlers.WriteError(w, http.StatusConflict, "order status does not allow payment session")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to create payment session")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"session": session,
	})
}
