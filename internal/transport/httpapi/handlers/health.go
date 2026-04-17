package handlers

import (
	"net/http"
	"time"
)

type HealthHandler struct {
	ServiceName string
	CheckDB     func() error
}

func (h HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": h.ServiceName,
		"time":    time.Now().UTC(),
	})
}

func (h HealthHandler) Readiness(w http.ResponseWriter, _ *http.Request) {
	if h.CheckDB != nil {
		if err := h.CheckDB(); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "degraded",
				"error":  err.Error(),
			})
			return
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
	})
}
