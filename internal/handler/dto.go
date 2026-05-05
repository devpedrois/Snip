package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Hash     string `json:"hash"`
	ShortURL string `json:"short_url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type MaliciousErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
	Report  string `json:"report,omitempty"`
}

type ReportRequest struct {
	Reason string `json:"reason"`
}

type ReportResponse struct {
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("handler: encode response", "err", err)
	}
}

func WriteError(w http.ResponseWriter, status int, msg, code string) {
	WriteJSON(w, status, ErrorResponse{Error: msg, Code: code})
}
