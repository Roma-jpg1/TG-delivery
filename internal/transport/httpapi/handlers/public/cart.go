package public

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/cart"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CartHandler struct {
	service *cart.Service
}

func NewCartHandler(service *cart.Service) *CartHandler {
	return &CartHandler{service: service}
}

type upsertCartItemRequest struct {
	UserID     uuid.UUID `json:"user_id"`
	BranchID   uuid.UUID `json:"branch_id"`
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Quantity   int       `json:"quantity"`
	Comment    string    `json:"comment"`
}

func (h *CartHandler) GetActiveCart(w http.ResponseWriter, r *http.Request) {
	userID, branchID, ok := parseCartQueryIDs(w, r)
	if !ok {
		return
	}

	cartSnapshot, err := h.service.GetActiveCart(r.Context(), userID, branchID)
	if err != nil {
		if errors.Is(err, cart.ErrCartNotFound) {
			handlers.WriteJSON(w, http.StatusOK, map[string]any{"cart": nil})
			return
		}
		handlers.WriteError(w, http.StatusInternalServerError, "failed to get cart")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"cart": cartSnapshot})
}

func (h *CartHandler) UpsertCartItem(w http.ResponseWriter, r *http.Request) {
	var req upsertCartItemRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Quantity < 0 {
		handlers.WriteError(w, http.StatusBadRequest, "quantity must be >= 0")
		return
	}

	cartSnapshot, err := h.service.UpsertItem(r.Context(), cart.UpsertItemInput{
		UserID:     req.UserID,
		BranchID:   req.BranchID,
		MenuItemID: req.MenuItemID,
		Quantity:   req.Quantity,
		Comment:    req.Comment,
	})
	if err != nil {
		switch {
		case errors.Is(err, cart.ErrItemUnavailable):
			handlers.WriteError(w, http.StatusConflict, "item is unavailable")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to upsert cart item")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"cart": cartSnapshot})
}

func (h *CartHandler) DeleteCartItem(w http.ResponseWriter, r *http.Request) {
	userID, branchID, ok := parseCartQueryIDs(w, r)
	if !ok {
		return
	}

	cartItemID, err := uuid.Parse(chi.URLParam(r, "cartItemID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid cartItemID")
		return
	}

	cartSnapshot, err := h.service.RemoveItem(r.Context(), cart.RemoveItemInput{
		UserID:     userID,
		BranchID:   branchID,
		CartItemID: cartItemID,
	})
	if err != nil {
		switch {
		case errors.Is(err, cart.ErrCartNotFound):
			handlers.WriteError(w, http.StatusNotFound, "cart not found")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to delete cart item")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"cart": cartSnapshot})
}

func parseCartQueryIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return uuid.Nil, uuid.Nil, false
	}

	branchID, err := uuid.Parse(r.URL.Query().Get("branch_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branch_id")
		return uuid.Nil, uuid.Nil, false
	}

	return userID, branchID, true
}
