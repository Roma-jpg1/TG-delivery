package public

import (
	"errors"
	"net/http"
	"strconv"

	"TG-delivery/internal/modules/orders"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrdersHandler struct {
	service *orders.Service
}

func NewOrdersHandler(service *orders.Service) *OrdersHandler {
	return &OrdersHandler{service: service}
}

func (h *OrdersHandler) ListUserOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	items, err := h.service.ListUserOrders(r.Context(), userID, limit)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list orders")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (h *OrdersHandler) RepeatOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid orderID")
		return
	}

	result, err := h.service.RepeatOrder(r.Context(), userID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrOrderNotFound):
			handlers.WriteError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, orders.ErrRepeatFailed):
			handlers.WriteError(w, http.StatusConflict, "repeat failed because all items are unavailable")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to repeat order")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"result": result})
}
