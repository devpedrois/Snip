package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// NewCORS returns a CORS middleware configured for the API.
// allowedOrigins is a comma-separated list (e.g. "https://example.com,https://app.com").
// Pass "*" for development.
func NewCORS(allowedOrigins string) func(http.Handler) http.Handler {
	var origins []string
	for _, o := range strings.Split(allowedOrigins, ",") {
		if s := strings.TrimSpace(o); s != "" {
			origins = append(origins, s)
		}
	}
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "X-Request-ID"},
		MaxAge:         300,
	})
}
