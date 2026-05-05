package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

type unsupportedMediaTypeError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// RequireJSON rejects requests whose Content-Type does not include "application/json".
// Returns 415 with {"error":"unsupported media type","code":"ERR_UNSUPPORTED_MEDIA_TYPE"}.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(unsupportedMediaTypeError{
				Error: "unsupported media type",
				Code:  "ERR_UNSUPPORTED_MEDIA_TYPE",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
