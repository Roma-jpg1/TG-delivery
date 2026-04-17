package webhooks

import (
	"errors"
	"io"
	"net/http"

	"TG-delivery/internal/modules/webhooks"
	"TG-delivery/internal/transport/httpapi/handlers"
	"TG-delivery/internal/transport/httpapi/middleware"
)

const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

type TelegramHandler struct {
	service *webhooks.TelegramService
}

func NewTelegramHandler(service *webhooks.TelegramService) *TelegramHandler {
	return &TelegramHandler{service: service}
}

func (h *TelegramHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	if err := h.service.Ingest(r.Context(), r.Header.Get(telegramSecretHeader), middleware.RequestIDFromContext(r.Context()), body); err != nil {
		switch {
		case errors.Is(err, webhooks.ErrInvalidTelegramSecret):
			handlers.WriteError(w, http.StatusUnauthorized, "invalid telegram webhook secret")
		default:
			handlers.WriteError(w, http.StatusBadRequest, "failed to ingest telegram update")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"accepted": true})
}
