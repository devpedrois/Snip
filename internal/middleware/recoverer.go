package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns a middleware that recovers from panics.
// In dev mode (isDev=true), the response includes the stack trace.
// In production (isDev=false), only a generic error is returned; the stack trace is logged internally.
func Recovery(isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					slog.Error("panic recovered", "panic", rec, "stack", string(stack))

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					if isDev {
						_ = json.NewEncoder(w).Encode(struct {
							Error string `json:"error"`
							Code  string `json:"code"`
							Stack string `json:"stack,omitempty"`
						}{
							Error: "internal server error",
							Code:  "ERR_INTERNAL",
							Stack: string(stack),
						})
					} else {
						_ = json.NewEncoder(w).Encode(struct {
							Error string `json:"error"`
							Code  string `json:"code"`
						}{
							Error: "internal server error",
							Code:  "ERR_INTERNAL",
						})
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
