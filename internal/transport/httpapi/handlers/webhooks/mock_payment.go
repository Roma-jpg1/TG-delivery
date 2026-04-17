package webhooks

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/webhooks"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"
)

const mockPaymentSecretHeader = "X-Mock-Payment-Secret"

type MockPaymentHandler struct {
	service *webhooks.MockPaymentService
}

func NewMockPaymentHandler(service *webhooks.MockPaymentService) *MockPaymentHandler {
	return &MockPaymentHandler{service: service}
}

func (h *MockPaymentHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	var event webhooks.MockPaymentEvent
	if err := handlers.DecodeJSON(r, &event); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.Ingest(
		r.Context(),
		r.Header.Get(mockPaymentSecretHeader),
		middleware.RequestIDFromContext(r.Context()),
		event,
	); err != nil {
		switch {
		case errors.Is(err, webhooks.ErrInvalidSecret):
			handlers.WriteError(w, http.StatusUnauthorized, "invalid webhook secret")
		default:
			handlers.WriteError(w, http.StatusBadRequest, "failed to ingest webhook")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"accepted": true,
	})
}
