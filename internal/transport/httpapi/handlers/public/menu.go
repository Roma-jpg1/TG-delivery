package public

import (
	"net/http"
	"strconv"

	"TG-delivery/internal/modules/menu"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MenuHandler struct {
	service *menu.Service
}

func NewMenuHandler(service *menu.Service) *MenuHandler {
	return &MenuHandler{service: service}
}

func (h *MenuHandler) ListBranchMenu(w http.ResponseWriter, r *http.Request) {
	branchID, err := uuid.Parse(chi.URLParam(r, "branchID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branchID")
		return
	}

	includeUnavailable := false
	if raw := r.URL.Query().Get("include_unavailable"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			handlers.WriteError(w, http.StatusBadRequest, "invalid include_unavailable")
			return
		}
		includeUnavailable = parsed
	}

	items, err := h.service.ListBranchMenu(r.Context(), branchID, includeUnavailable)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list menu")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}
