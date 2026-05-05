package middleware

import "net/http"

// BodyLimit returns a middleware that limits the request body to maxBytes.
// Uses http.MaxBytesReader. Handlers that decode the body must check for
// *http.MaxBytesError and return 413 with ERR_BODY_TOO_LARGE.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
