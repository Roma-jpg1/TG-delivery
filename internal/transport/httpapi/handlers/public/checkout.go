package public

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/checkout"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/google/uuid"
)

type CheckoutHandler struct {
	service *checkout.Service
}

func NewCheckoutHandler(service *checkout.Service) *CheckoutHandler {
	return &CheckoutHandler{service: service}
}

type createDraftRequest struct {
	UserID          uuid.UUID  `json:"user_id"`
	BranchID        uuid.UUID  `json:"branch_id"`
	AddressID       *uuid.UUID `json:"address_id"`
	CustomerComment string     `json:"customer_comment"`
}

func (h *CheckoutHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	var req createDraftRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	draft, err := h.service.CreateOrderDraft(r.Context(), checkout.CreateDraftInput{
		UserID:          req.UserID,
		BranchID:        req.BranchID,
		AddressID:       req.AddressID,
		CustomerComment: req.CustomerComment,
		RequestID:       middleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		switch {
		case errors.Is(err, checkout.ErrCartNotFound):
			handlers.WriteError(w, http.StatusNotFound, "active cart not found")
		case errors.Is(err, checkout.ErrCartEmpty):
			handlers.WriteError(w, http.StatusConflict, "cart is empty")
		case errors.Is(err, checkout.ErrCartRevalidationFailed):
			handlers.WriteError(w, http.StatusConflict, "cart revalidation failed")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to create checkout draft")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}
