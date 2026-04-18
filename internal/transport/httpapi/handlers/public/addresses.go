package public

import (
	"errors"
	"net/http"

	"TG-delivery/internal/modules/addresses"
	"TG-delivery/internal/transport/httpapi/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AddressesHandler struct {
	service *addresses.Service
}

func NewAddressesHandler(service *addresses.Service) *AddressesHandler {
	return &AddressesHandler{service: service}
}

type upsertAddressRequest struct {
	AddressID  *uuid.UUID `json:"address_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Label      string     `json:"label"`
	City       string     `json:"city"`
	Street     string     `json:"street"`
	House      string     `json:"house"`
	Apartment  string     `json:"apartment"`
	Entrance   string     `json:"entrance"`
	Floor      string     `json:"floor"`
	Comment    string     `json:"comment"`
	Latitude   float64    `json:"latitude"`
	Longitude  float64    `json:"longitude"`
	SetDefault bool       `json:"set_default"`
}

func (h *AddressesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	items, err := h.service.List(r.Context(), userID)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, "failed to list addresses")
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (h *AddressesHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req upsertAddressRequest
	if err := handlers.DecodeJSON(r, &req); err != nil {
		handlers.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.service.Upsert(r.Context(), addresses.UpsertInput{
		AddressID:  req.AddressID,
		UserID:     req.UserID,
		Label:      req.Label,
		City:       req.City,
		Street:     req.Street,
		House:      req.House,
		Apartment:  req.Apartment,
		Entrance:   req.Entrance,
		Floor:      req.Floor,
		Comment:    req.Comment,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		SetDefault: req.SetDefault,
	})
	if err != nil {
		switch {
		case errors.Is(err, addresses.ErrAddressNotFound):
			handlers.WriteError(w, http.StatusNotFound, "address not found")
		case errors.Is(err, addresses.ErrInvalidCoordinate):
			handlers.WriteError(w, http.StatusBadRequest, "invalid coordinates")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to save address")
		}
		return
	}

	handlers.WriteJSON(w, http.StatusOK, map[string]any{"address": item})
}

func (h *AddressesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	addressID, err := uuid.Parse(chi.URLParam(r, "addressID"))
	if err != nil {
		handlers.WriteError(w, http.StatusBadRequest, "invalid addressID")
		return
	}

	if err := h.service.Delete(r.Context(), userID, addressID); err != nil {
		switch {
		case errors.Is(err, addresses.ErrAddressNotFound):
			handlers.WriteError(w, http.StatusNotFound, "address not found")
		default:
			handlers.WriteError(w, http.StatusInternalServerError, "failed to delete address")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
