package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/scanner"
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req ShortenRequest
	if err := decoder.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, http.StatusRequestEntityTooLarge, "request body too large", "ERR_BODY_TOO_LARGE")
			return
		}
		// encoding/json does not export a sentinel type for unknown-field errors;
		// string inspection is the only available mechanism.
		if strings.Contains(err.Error(), "unknown field") {
			WriteError(w, http.StatusBadRequest, "unknown fields in request body", "ERR_UNKNOWN_FIELDS")
			return
		}
		WriteError(w, http.StatusBadRequest, "invalid request body", "ERR_INVALID_BODY")
		return
	}

	u, result, err := h.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrURLMalicious):
			details := fmt.Sprintf("detected by %d/%d security engines", result.Positives, result.Total)
			WriteJSON(w, http.StatusUnprocessableEntity, MaliciousErrorResponse{
				Error:   "url flagged as malicious",
				Code:    "ERR_URL_MALICIOUS",
				Details: details,
				Report:  result.Permalink,
			})
		case errors.Is(err, domain.ErrInvalidURL):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_INVALID_URL")
		case errors.Is(err, domain.ErrURLHasCredentials):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_URL_CREDENTIALS")
		case errors.Is(err, domain.ErrURLPrivateIP):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_URL_PRIVATE_IP")
		case errors.Is(err, domain.ErrURLSelfRef):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_URL_SELF_REF")
		case errors.Is(err, domain.ErrURLBlocked):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_URL_BLOCKED")
		case errors.Is(err, domain.ErrURLHomograph):
			WriteError(w, http.StatusBadRequest, err.Error(), "ERR_URL_HOMOGRAPH")
		default:
			WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		}
		return
	}

	if result.Status == scanner.ScanUnverified {
		w.Header().Set("X-Scan-Status", "unverified")
	}

	WriteJSON(w, http.StatusCreated, ShortenResponse{
		Hash:     u.Hash,
		ShortURL: fmt.Sprintf("%s/%s", h.baseURL, u.Hash),
	})
}
