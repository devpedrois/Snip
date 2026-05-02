package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/service"
)

type ShortenHandler struct {
	svc     service.ShortenerService
	baseURL string
}

func NewShortenHandler(svc service.ShortenerService, baseURL string) *ShortenHandler {
	return &ShortenHandler{svc: svc, baseURL: baseURL}
}

func (h *ShortenHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "ERR_INVALID_BODY")
		return
	}

	u, err := h.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidURL) {
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_INVALID_URL")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		return
	}

	WriteJSON(w, http.StatusCreated, ShortenResponse{
		Hash:     u.Hash,
		ShortURL: fmt.Sprintf("%s/%s", h.baseURL, u.Hash),
	})
}
