package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/service"
)

type AnalyticsHandler struct {
	svc service.AnalyticsService
}

func NewAnalyticsHandler(svc service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")

	stats, err := h.svc.GetStats(r.Context(), hash)
	if err != nil {
		if errors.Is(err, domain.ErrURLNotFound) {
			WriteError(w, http.StatusNotFound, "url not found", "ERR_NOT_FOUND")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}
