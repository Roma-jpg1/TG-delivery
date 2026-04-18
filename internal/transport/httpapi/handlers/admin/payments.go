package admin

import (
	"net/http"
	"strconv"
	"strings"

	"TG-delivery/internal/modules/payments"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/google/uuid"
)

type PaymentsHandler struct {
	service *payments.Service
}

func NewPaymentsHandler(service *payments.Service) *PaymentsHandler {
	return &PaymentsHandler{service: service}
}

func (h *PaymentsHandler) List(w http.ResponseWriter, r *http.Request) {
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

	items, err := h.service.List(r.Context(), payments.ListFilter{
		BranchID: branchID,
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:    limit,
	})
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list payments")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}
