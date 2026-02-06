package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/go-chi/chi/v5"
)

type OrderService interface {
	GetOrder(ctx context.Context, orderUID string) (*model.Order, error)
}

type Handler struct {
	service OrderService
	l       *slog.Logger
}

func New(l *slog.Logger, service OrderService) *Handler {
	return &Handler{
		service: service,
		l:       l,
	}
}

func (h *Handler) GetOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		order, err := h.service.GetOrder(r.Context(), id)
		if err != nil {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(order); err != nil {
			h.l.Error("failed to encode response", "error", err)
			return
		}
	}
}
