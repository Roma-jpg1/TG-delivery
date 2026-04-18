package public

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/delivery"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/google/uuid"
)

type DeliveryHandler struct {
	service *delivery.Service
}

func NewDeliveryHandler(service *delivery.Service) *DeliveryHandler {
	return &DeliveryHandler{service: service}
}

type quoteRequest struct {
	UserID       uuid.UUID `json:"user_id"`
	BranchID     uuid.UUID `json:"branch_id"`
	AddressID    uuid.UUID `json:"address_id"`
	CartSubtotal int       `json:"cart_subtotal"`
}

func (h *DeliveryHandler) Quote(w http.ResponseWriter, r *http.Request) {
	var req quoteRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	quote, err := h.service.Quote(r.Context(), delivery.QuoteInput{
		UserID:       req.UserID,
		BranchID:     req.BranchID,
		AddressID:    req.AddressID,
		CartSubtotal: req.CartSubtotal,
	})
	if err != nil {
		switch {
		case errors.Is(err, delivery.ErrAddressNotFound):
			handlers.WriteError(w, http.StatusNotFound, "address not found")
		case errors.Is(err, delivery.ErrOutOfZone):
			handlers.WriteError(w, http.StatusConflict, "address is outside delivery zone")
		case errors.Is(err, delivery.ErrBelowMinOrder):
			handlers.WriteError(w, http.StatusConflict, "cart total is below minimum order amount")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to calculate delivery quote")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"quote": quote})
}
