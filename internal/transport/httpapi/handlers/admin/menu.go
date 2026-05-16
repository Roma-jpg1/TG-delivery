package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"

	menumodule "TG-delivery/internal/modules/menu"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MenuService interface {
	ListBranchMenu(ctx context.Context, branchID uuid.UUID, includeUnavailable bool) ([]menumodule.Item, error)
	UpdateBranchMenuItem(ctx context.Context, in menumodule.UpdateItemInput) (menumodule.Item, error)
}

type MenuHandler struct {
	service MenuService
}

func NewMenuHandler(service MenuService) *MenuHandler {
	return &MenuHandler{service: service}
}

type updateMenuItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PhotoURL    string `json:"photo_url"`
	Price       int    `json:"price"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

func (h *MenuHandler) ListBranchMenu(w http.ResponseWriter, r *http.Request) {
	branchID, err := uuid.Parse(chi.URLParam(r, "branchID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branchID")
		return
	}

	items, err := h.service.ListBranchMenu(r.Context(), branchID, true)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list menu")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (h *MenuHandler) UpdateBranchMenuItem(w http.ResponseWriter, r *http.Request) {
	branchID, err := uuid.Parse(chi.URLParam(r, "branchID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid branchID")
		return
	}

	menuItemID, err := uuid.Parse(chi.URLParam(r, "menuItemID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid menuItemID")
		return
	}

	var req updateMenuItemRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		handlers.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Price < 0 {
		handlers.WriteError(w, http.StatusBadRequest, "price must be non-negative")
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "available"
	}

	item, err := h.service.UpdateBranchMenuItem(r.Context(), menumodule.UpdateItemInput{
		BranchID:    branchID,
		MenuItemID:  menuItemID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		PhotoURL:    strings.TrimSpace(req.PhotoURL),
		Price:       req.Price,
		Status:      strings.TrimSpace(req.Status),
		Reason:      strings.TrimSpace(req.Reason),
	})
	if err != nil {
		if errors.Is(err, menumodule.ErrNotFound) {
			handlers.WriteError(w, http.StatusNotFound, "menu item not found")
			return
		}
		handlers.WriteError(w, http.StatusInternalServerError, "failed to update menu item")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}
