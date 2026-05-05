package handler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/service"
)

type ReportHandler struct {
	svc service.ReportService
}

func NewReportHandler(svc service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) Handle(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")

	rawIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		rawIP = host
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req ReportRequest
	if err := decoder.Decode(&req); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			WriteError(w, http.StatusBadRequest, "unknown fields in request body", "ERR_UNKNOWN_FIELDS")
			return
		}
		WriteError(w, http.StatusBadRequest, "invalid request body", "ERR_INVALID_BODY")
		return
	}

	if err := h.svc.Report(r.Context(), hash, rawIP, req.Reason); err != nil {
		switch {
		case errors.Is(err, domain.ErrURLNotFound):
			WriteError(w, http.StatusNotFound, "url not found", "ERR_NOT_FOUND")
		case errors.Is(err, domain.ErrDuplicateReport):
			WriteError(w, http.StatusConflict, "url already reported by this ip", "ERR_DUPLICATE_REPORT")
		case errors.Is(err, domain.ErrInvalidReason):
			WriteError(w, http.StatusBadRequest, "invalid report reason", "ERR_INVALID_REASON")
		default:
			WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, ReportResponse{Message: "report received"})
}
